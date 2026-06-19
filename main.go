package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
    
    "golang.org/x/term"
)

// Константы telnet протокола
const (
    IAC  = 255 // Interpret As Command
    DONT = 254
    DO   = 253
    WONT = 252
    WILL = 251
    SB   = 250 // Subnegotiation Begin
    SE   = 240 // Subnegotiation End
)

// Структура для хранения пары логин/пароль
type Credential struct {
	Username string
	Password string
}

// Загрузка кредов из INI (поддерживаем свой формат)
func loadCredentials() []Credential {
	var creds []Credential
	
	// Читаем файл построчно
	data, err := os.ReadFile("config.ini")
	if err != nil {
		fmt.Println("Ошибка чтения config.ini:", err)
		fmt.Println("Создайте файл с секцией [credentials]")
		os.Exit(1)
	}
	
	lines := strings.Split(string(data), "\n")
	inCredentials := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Пропускаем пустые и комментарии
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Проверяем секцию
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inCredentials = strings.ToLower(line) == "[credentials]"
			continue
		}
		
		if !inCredentials {
			continue
		}
				
		// Поддерживаем несколько пар через запятую
		pairs := strings.Split(line, ",")
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			
			credsParts := strings.SplitN(pair, ":", 2)
			if len(credsParts) == 2 {
				creds = append(creds, Credential{
					Username: strings.TrimSpace(credsParts[0]),
					Password: strings.TrimSpace(credsParts[1]),
				})
			}
		}
	}
	
	return creds
}

// Попытка подключения с конкретными кредами
func tryLogin(host string, cred Credential) (net.Conn, error) {
    if !strings.Contains(host, ":") {
        host = host + ":23"
    }

    conn, err := net.DialTimeout("tcp", host, 5*time.Second)
    if err != nil {
        return nil, fmt.Errorf("ошибка подключения: %v", err)
    }

    // readUntil – читает данные, пока не встретится одна из подстрок (без учёта регистра)
    readUntil := func(prompts []string, timeout time.Duration) (string, error) {
        buf := make([]byte, 4096)
        var acc []byte
        deadline := time.Now().Add(timeout)
        for time.Now().Before(deadline) {
            conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
            n, err := conn.Read(buf)
            if n > 0 {
                // Фильтруем телнет-команды сразу
                clean := filterTelnetCommands(buf[:n])
                acc = append(acc, clean...)
                text := string(acc)
                for _, p := range prompts {
                    if strings.Contains(strings.ToLower(text), strings.ToLower(p)) {
                        return text, nil
                    }
                }
            }
            if err != nil {
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    continue
                }
                return string(acc), err
            }
        }
        return string(acc), fmt.Errorf("таймаут ожидания одного из: %v", prompts)
    }

    // Этап 1: ждём запрос логина
    _, err = readUntil([]string{"login:", "username:", "user:"}, 8*time.Second)
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("не получен запрос логина: %v", err)
    }
    // Отправляем логин
    conn.Write([]byte(cred.Username + "\r\n"))

    // Этап 2: ждём запрос пароля
    _, err = readUntil([]string{"password:", "pass:"}, 8*time.Second)
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("не получен запрос пароля: %v", err)
    }
    // Отправляем пароль
    conn.Write([]byte(cred.Password + "\r\n"))

    // Этап 3: ждём приглашение после входа (успех)
    prompt, err := readUntil([]string{"$", ">", "#", "(config)>", "~$", "]"}, 8*time.Second)
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("не получено приглашение после входа: %v", err)
    }

    // Дополнительная проверка на отказ
    if strings.Contains(strings.ToLower(prompt), "incorrect") ||
        strings.Contains(strings.ToLower(prompt), "failed") ||
        strings.Contains(strings.ToLower(prompt), "denied") {
        conn.Close()
        return nil, fmt.Errorf("неверный логин/пароль")
    }

    return conn, nil
}

// Основная функция подключения с перебором кредов
func telnetConnectWithRetry(host string, creds []Credential) error {
	var lastErr error
	
	for i, cred := range creds {
		fmt.Printf("Попытка %d/%d: %s:%s ... ", 
			i+1, len(creds), cred.Username, cred.Password)
		
		conn, err := tryLogin(host, cred)
		if err == nil {
			fmt.Println("✅ Успешно!")
			fmt.Printf("\n--- Сессия открыта под %s ---\n", cred.Username)
			fmt.Println("Вводите команды (Ctrl+C для выхода)")
			conn.Write([]byte("\r\n"))

			// Настройка терминала в raw mode для интерактивного ввода
			oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				return fmt.Errorf("не удалось перевести терминал в raw mode: %w", err)
			}
			defer term.Restore(int(os.Stdin.Fd()), oldState)

			// Создаем канал для синхронизации
			done := make(chan struct{})
			defer close(done)

			// Чтение из telnet соединения и вывод на экран
			go func() {
				defer func() {
					done <- struct{}{}
				}()

				reader := bufio.NewReader(conn)
				writer := bufio.NewWriter(os.Stdout)
				
				buf := make([]byte, 4096)
				
				for {
					select {
					case <-done:
						return
					default:
						// Устанавливаем таймаут для неблокирующего чтения
						conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
						
						n, err := reader.Read(buf)
						if n > 0 {
							// Обработка телнет команд (IAC - Interpret As Command)
							data := buf[:n]
							// Фильтруем телнет escape-последовательности
							cleanData := filterTelnetCommands(data)
							
							if len(cleanData) > 0 {
								writer.Write(cleanData)
								writer.Flush()
							}
						}
						
						if err != nil {
							if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
								continue
							}
							return
						}
					}
				}
			}()

			// Чтение пользовательского ввода и отправка в telnet
			go func() {
				defer func() {
					done <- struct{}{}
				}()

				reader := bufio.NewReader(os.Stdin)
				
				for {
					select {
					case <-done:
						return
					default:
						char, err := reader.ReadByte()
						if err != nil {
							return
						}
						
						// Отправляем каждый символ в telnet соединение
						conn.Write([]byte{char})
						
						// Обработка специальных клавиш
						switch char {
						case 3: // Ctrl+C
							return
						case 13: // Enter
							conn.Write([]byte{'\n'})
						}
					}
				}
			}()

			// Ожидаем завершения работы
			<-done
			
			// Восстанавливаем терминал
			term.Restore(int(os.Stdin.Fd()), oldState)
			fmt.Println("\nСессия завершена")
			return nil
		}
		
		fmt.Println("❌ Ошибка:", err)
		lastErr = err
	}
	
	return fmt.Errorf("все попытки входа не удались. Последняя ошибка: %v", lastErr)
}

// Функция для фильтрации telnet команд
func filterTelnetCommands(data []byte) []byte {
	result := make([]byte, 0, len(data))
	i := 0
	
	for i < len(data) {
		if data[i] == IAC { // Interpret As Command (255)
			// Пропускаем telnet команды
			if i+1 < len(data) {
				switch data[i+1] {
				case WILL, WONT, DO, DONT:
					// Пропускаем опции с параметром
					i += 3
				case SB:
					// Пропускаем до конца subnegotiation
					for i < len(data) {
						if i+1 < len(data) && data[i] == IAC && data[i+1] == SE {
							i += 2
							break
						}
						i++
					}
				default:
					// Пропускаем другие команды
					i += 2
				}
			} else {
				i++
			}
		} else {
			result = append(result, data[i])
			i++
		}
	}
	
	return result
}


// Парсинг telnet:// ссылок
func parseTelnetURL(url string) string {
	re := regexp.MustCompile(`telnet://([^:/]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return url
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Использование:")
		fmt.Println("  brutel.exe 192.168.0.1")
		fmt.Println("  brutel.exe telnet://192.168.0.1")
		fmt.Println("  brutel.exe 10.0.0.5:2323")
		os.Exit(1)
	}
	
	// Загружаем креды
	creds := loadCredentials()
	if len(creds) == 0 {
		fmt.Println("Ошибка: в config.ini нет логинов в секции [credentials]")
		fmt.Println("Пример:")
		fmt.Println("[credentials]")
		fmt.Println("admin:admin123")
		fmt.Println("root:toor")
		fmt.Println("admin:admin,user:user,cisco:cisco")
		os.Exit(1)
	}
	
	fmt.Printf("Загружено %d логинов для перебора\n", len(creds))
	
	// Парсим аргумент
	arg := os.Args[1]
	host := parseTelnetURL(arg)
	
	// Подключаемся с перебором
	if err := telnetConnectWithRetry(host, creds); err != nil {
		fmt.Println("\n❌", err)
		os.Exit(1)
	}
}