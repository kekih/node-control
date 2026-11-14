#!/usr/bin/env bash
set -e

echo "=================================================="
echo "   Сборка проекта Node (Client & VPS Agent)      "
echo "=================================================="

mkdir -p bin

echo "[1/3] Сборка сервера-агента для Linux (bin/node-agent)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/node-agent-linux-amd64 ./cmd/server

echo "[2/3] Сборка клиента (bin/node-client)..."
go build -ldflags="-s -w" -o bin/node-client ./cmd/client

echo "[3/3] Сборка агента для текущей ОС..."
go build -ldflags="-s -w" -o bin/node-agent ./cmd/server

echo "=================================================="
echo "[✓] Сборка успешно завершена!"
echo "  • bin/node-client"
echo "  • bin/node-agent-linux-amd64 (Для VPS)"
echo "=================================================="
