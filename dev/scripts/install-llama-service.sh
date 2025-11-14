#!/bin/bash
# Install llama-server systemd service

set -e

CURRENT_USER="${USER}"
SERVICE_FILE="/etc/systemd/system/llama-server@.service"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Installing llama-server systemd service..."

# Copy service file
sudo cp "$SCRIPT_DIR/llama-server@.service" "$SERVICE_FILE"

# Reload systemd
sudo systemctl daemon-reload

# Enable and start service for current user
sudo systemctl enable "llama-server@${CURRENT_USER}.service"
sudo systemctl start "llama-server@${CURRENT_USER}.service"

echo "✓ llama-server service installed and started"
echo ""
echo "Usage:"
echo "  sudo systemctl status llama-server@${CURRENT_USER}"
echo "  sudo systemctl stop llama-server@${CURRENT_USER}"
echo "  sudo systemctl restart llama-server@${CURRENT_USER}"
echo "  sudo journalctl -u llama-server@${CURRENT_USER} -f"
echo ""
echo "The service will automatically start on boot."
