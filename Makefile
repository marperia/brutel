APP_NAME = brutel
VERSION = 1.0.0

# Windows
build-win:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(APP_NAME)_$(VERSION)_windows_amd64.exe main.go
	GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o $(APP_NAME)_$(VERSION)_windows_386.exe main.go

# Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(APP_NAME)_$(VERSION)_linux_amd64 main.go
	GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o $(APP_NAME)_$(VERSION)_linux_armv7 main.go

# macOS
build-mac:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(APP_NAME)_$(VERSION)_macos_amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(APP_NAME)_$(VERSION)_macos_arm64 main.go

# Все платформы
build-all: build-win build-linux build-mac
	@echo "✅ Собрано для всех платформ"

# Только для текущей ОС
build:
	go build -ldflags="-s -w" -o $(APP_NAME) main.go

# Очистка
clean:
	rm -f $(APP_NAME) $(APP_NAME)_*.exe $(APP_NAME)_*