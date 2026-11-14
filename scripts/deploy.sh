#!/usr/bin/env bash
# Quick deployment script for Linux VPS
set -e

PORT=${1:-9090}
SHARE_PORT=${2:-9091}
SECRET=${3:-""}

echo "=== Установка Node Agent на VPS ==="

# Create directory
mkdir -p /opt/node-agent
mkdir -p /opt/node-agent/workdir

# Copy binary if present in current dir
if [ -f "node-agent-linux-amd64" ]; then
    cp node-agent-linux-amd64 /opt/node-agent/node-agent
elif [ -f "node-agent" ]; then
    cp node-agent /opt/node-agent/node-agent
fi

chmod +x /opt/node-agent/node-agent

# Create systemd service
cat <<EOF > /etc/systemd/system/node-agent.service
[Unit]
Description=Node Control Agent Daemon
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/node-agent/workdir
ExecStart=/opt/node-agent/node-agent -port ${PORT} -share-port ${SHARE_PORT} -secret "${SECRET}" -dir /opt/node-agent/workdir
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable node-agent
systemctl restart node-agent

echo "=== Node Agent успешно установлен и запущен! ==="
echo "Порт управления (Tailscale): ${PORT}"
echo "Порт раздачи файлов (Public): ${SHARE_PORT}"
echo "Статус сервиса: systemctl status node-agent"
