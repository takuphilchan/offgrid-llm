#!/bin/bash
# Quick reinstall script for testing

set -e

echo "🧹 Cleaning up existing installation..."

# Stop services
sudo systemctl stop offgrid-llm.service llama-server.service 2>/dev/null || true

# Kill any running processes
pkill -f llama-server || true
pkill -f offgrid || true

# Remove systemd services
sudo rm -f /etc/systemd/system/llama-server.service
sudo rm -f /etc/systemd/system/offgrid-llm.service
sudo systemctl daemon-reload

# Remove config and binaries
sudo rm -rf /etc/offgrid
sudo rm -f /usr/local/bin/offgrid
sudo rm -f /usr/local/bin/llama-server

# Remove llama.cpp build (optional - comment out to keep)
# rm -rf ~/llama.cpp/build

echo "✓ Cleanup complete"
echo ""
echo "🚀 Starting fresh installation..."
echo ""

# Run installation
./install.sh
