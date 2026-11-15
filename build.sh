#!/bin/bash
set -e

echo "=================================================="
echo "    Сборка проекта Node (Client & VPS Agent)      "
echo "=================================================="

mkdir -p bin

echo "[1/4] Сборка нативного клиента для Linux (bin/node-client-linux-amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/node-client-linux-amd64 ./cmd/client

echo "[2/4] Сборка нативного клиента для Windows (bin/node-client.exe)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/node-client.exe ./cmd/client

echo "[3/4] Сборка сервера-агента для Linux VPS (bin/node-agent-linux-amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/node-agent-linux-amd64 ./cmd/server

echo "[4/4] Сборка сервера-агента для Windows (bin/node-agent.exe)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/node-agent.exe ./cmd/server

echo "=================================================="
echo "[✓] Сборка завершена в папке bin/"
echo "=================================================="
