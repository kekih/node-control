@echo off
chcp 65001 > nul
echo ==================================================
echo    Сборка проекта Node (Client ^& VPS Agent)
echo ==================================================

if not exist bin mkdir bin

echo [1/3] Сборка локального клиента для Windows (bin\node-client.exe)...
go build -ldflags="-s -w" -o bin\node-client.exe ./cmd/client
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Не удалось собрать node-client.exe
    exit /b 1
)

echo [2/3] Сборка сервера-агента для Windows (bin\node-agent.exe)...
go build -ldflags="-s -w" -o bin\node-agent.exe ./cmd/server
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Не удалось собрать node-agent.exe
    exit /b 1
)

echo [3/3] Кросс-компиляция сервера-агента для Linux VPS (bin\node-agent-linux-amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\node-agent-linux-amd64 ./cmd/server
set GOOS=windows
set GOARCH=amd64

echo ==================================================
echo [✓] Сборка успешно завершена!
echo   • bin\node-client.exe (Запуск клиента управления)
echo   • bin\node-agent.exe (Запуск агента на Windows)
echo   • bin\node-agent-linux-amd64 (Загрузить на Linux VPS)
echo ==================================================
