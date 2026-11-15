@echo off
chcp 65001 > nul
echo ==================================================
echo    Сборка проекта Node (Client ^& VPS Agent)
echo ==================================================

if not exist bin mkdir bin

echo [1/4] Сборка нативного клиента для Windows (bin\node-client.exe)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\node-client.exe ./cmd/client
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Не удалось собрать node-client.exe
    exit /b 1
)

echo [2/4] Сборка нативного клиента для Linux (bin\node-client-linux-amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\node-client-linux-amd64 ./cmd/client
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Не удалось собрать node-client-linux-amd64
    exit /b 1
)

echo [3/4] Сборка сервера-агента для Windows (bin\node-agent.exe)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\node-agent.exe ./cmd/server
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Не удалось собрать node-agent.exe
    exit /b 1
)

echo [4/4] Кросс-компиляция сервера-агента для Linux VPS (bin\node-agent-linux-amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\node-agent-linux-amd64 ./cmd/server
set GOOS=windows
set GOARCH=amd64

echo ==================================================
echo [✓] Сборка успешно завершена!
echo   • bin\node-client.exe (Нативный клиент Windows)
echo   • bin\node-client-linux-amd64 (Нативный клиент Linux)
echo   • bin\node-agent.exe (Сервер/агент для Windows)
echo   • bin\node-agent-linux-amd64 (Сервер/агент для Linux VPS)
echo ==================================================
