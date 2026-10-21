#!/bin/bash
# USB Distribution Package Creator
# Creates a complete offline installation package on USB drive

set -e

echo "📦 OffGrid LLM - USB Distribution Package Creator"
echo "=================================================="
echo

# Check arguments
if [ $# -lt 1 ]; then
    echo "Usage: $0 <usb-mount-point> [model-ids...]"
    echo
    echo "Examples:"
    echo "  $0 /media/usb"
    echo "  $0 /media/usb tinyllama-1.1b-chat llama-2-7b-chat"
    echo
    echo "This will create a complete offline installation package with:"
    echo "  - Binary for Linux, Windows, macOS"
    echo "  - Selected models (or all if none specified)"
    echo "  - Installation scripts"
    echo "  - Documentation"
    exit 1
fi

USB_PATH="$1"
shift
MODEL_IDS="$@"

# Verify USB path exists
if [ ! -d "$USB_PATH" ]; then
    echo "❌ USB path not found: $USB_PATH"
    exit 1
fi

# Create package structure
PACKAGE_DIR="$USB_PATH/offgrid-llm-package"
echo "📁 Creating package structure at $PACKAGE_DIR"

mkdir -p "$PACKAGE_DIR"/{bin,models,docs,scripts}

# Build binaries for multiple platforms
echo
echo "🔨 Building binaries..."

# Linux AMD64
echo "  Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o "$PACKAGE_DIR/bin/offgrid-linux-amd64" ./cmd/offgrid

# Linux ARM64 (Raspberry Pi, etc.)
echo "  Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -o "$PACKAGE_DIR/bin/offgrid-linux-arm64" ./cmd/offgrid

# Windows
echo "  Building for Windows..."
GOOS=windows GOARCH=amd64 go build -o "$PACKAGE_DIR/bin/offgrid-windows.exe" ./cmd/offgrid

# macOS
echo "  Building for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -o "$PACKAGE_DIR/bin/offgrid-macos-amd64" ./cmd/offgrid

# macOS Apple Silicon
echo "  Building for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o "$PACKAGE_DIR/bin/offgrid-macos-arm64" ./cmd/offgrid

# Copy models
echo
echo "📥 Copying models..."

MODELS_DIR="${HOME}/.offgrid-llm/models"

if [ -z "$MODEL_IDS" ]; then
    # Copy all models
    if [ -d "$MODELS_DIR" ] && [ "$(ls -A $MODELS_DIR/*.gguf 2>/dev/null)" ]; then
        cp "$MODELS_DIR"/*.gguf "$PACKAGE_DIR/models/" 2>/dev/null || true
        MODEL_COUNT=$(ls -1 "$PACKAGE_DIR/models"/*.gguf 2>/dev/null | wc -l)
        echo "  Copied $MODEL_COUNT model(s)"
    else
        echo "  ⚠️  No models found in $MODELS_DIR"
        echo "  Run 'offgrid download <model-id>' to download models first"
    fi
else
    # Copy specific models
    for model_id in $MODEL_IDS; do
        # Find matching model files
        found=false
        for file in "$MODELS_DIR"/${model_id}*.gguf; do
            if [ -f "$file" ]; then
                cp "$file" "$PACKAGE_DIR/models/"
                echo "  ✓ $(basename $file)"
                found=true
            fi
        done
        
        if [ "$found" = false ]; then
            echo "  ⚠️  Model not found: $model_id"
            echo "     Download it first: offgrid download $model_id"
        fi
    done
fi

# Copy documentation
echo
echo "📚 Copying documentation..."
cp README.md "$PACKAGE_DIR/"
cp -r docs "$PACKAGE_DIR/" 2>/dev/null || true
echo "  ✓ Documentation copied"

# Create installation script for Linux/Mac
cat > "$PACKAGE_DIR/install.sh" << 'EOF'
#!/bin/bash
set -e

echo "🌐 OffGrid LLM - Offline Installation"
echo "====================================="
echo

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "❌ Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

BINARY="offgrid-${OS}-${ARCH}"

if [ ! -f "bin/$BINARY" ]; then
    echo "❌ Binary not found for your platform: $BINARY"
    echo "Available binaries:"
    ls -1 bin/
    exit 1
fi

# Installation directory
INSTALL_DIR="${HOME}/.local/bin"
MODELS_DIR="${HOME}/.offgrid-llm/models"

echo "Installing to: $INSTALL_DIR"
echo "Models to: $MODELS_DIR"
echo

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$MODELS_DIR"

# Copy binary
echo "📦 Installing binary..."
cp "bin/$BINARY" "$INSTALL_DIR/offgrid"
chmod +x "$INSTALL_DIR/offgrid"

# Copy models
echo "📥 Installing models..."
if [ -d "models" ] && [ "$(ls -A models/*.gguf 2>/dev/null)" ]; then
    cp models/*.gguf "$MODELS_DIR/"
    MODEL_COUNT=$(ls -1 "$MODELS_DIR"/*.gguf 2>/dev/null | wc -l)
    echo "  Installed $MODEL_COUNT model(s)"
else
    echo "  No models to install"
fi

# Setup systemd service (Linux only)
if [ "$OS" = "linux" ] && command -v systemctl &> /dev/null; then
    echo
    echo "🔧 Setting up auto-start service..."
    
    # Create startup script
    SCRIPT_PATH="/usr/local/bin/llama-server-start.sh"
    
    sudo tee "$SCRIPT_PATH" > /dev/null << 'SCRIPT_EOF'
#!/bin/bash
# Auto-start script for llama-server
set -e

# Read port from config, default to 42382
PORT=42382
if [ -f /etc/offgrid/llama-port ]; then
    PORT=$(cat /etc/offgrid/llama-port)
fi

# Find models directory
MODELS_DIR="${HOME}/.offgrid-llm/models"

if [ ! -d "$MODELS_DIR" ]; then
    echo "Models directory not found: $MODELS_DIR"
    exit 1
fi

# Find first available GGUF model (smallest first)
MODEL_FILE=$(find "$MODELS_DIR" -name "*.gguf" -type f | sort -h | head -1)

if [ -z "$MODEL_FILE" ]; then
    echo "No GGUF models found in $MODELS_DIR"
    exit 1
fi

echo "Starting llama-server with model: $(basename "$MODEL_FILE")"
echo "Port: $PORT"

# Start llama-server
exec llama-server \\
    --model "$MODEL_FILE" \\
    --port "$PORT" \\
    --host 127.0.0.1 \\
    -c 4096 \\
    --threads 4 \\
    --metrics
SCRIPT_EOF
    
    sudo chmod +x "$SCRIPT_PATH"
    
    # Create systemd service
    sudo tee "/etc/systemd/system/llama-server@.service" > /dev/null << 'SERVICE_EOF'
[Unit]
Description=Llama.cpp HTTP Server for OffGrid LLM
After=network.target

[Service]
Type=simple
User=%i
Environment="HOME=/home/%i"
ExecStart=/usr/local/bin/llama-server-start.sh
Restart=always
RestartSec=5s
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
SERVICE_EOF
    
    # Create port config
    sudo mkdir -p /etc/offgrid
    echo "42382" | sudo tee /etc/offgrid/llama-port > /dev/null
    
    # Enable service
    CURRENT_USER=$(whoami)
    sudo systemctl daemon-reload
    sudo systemctl enable "llama-server@$CURRENT_USER"
    
    echo "  ✅ Auto-start service enabled"
fi

# Check if binary is in PATH
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo
    echo "⚠️  $INSTALL_DIR is not in your PATH"
    echo "Add this to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo
fi

echo
echo "✅ Installation complete!"
echo
echo "To start the server:"
echo "  offgrid"
echo
echo "Or if not in PATH:"
echo "  $INSTALL_DIR/offgrid"
echo
if [ "$OS" = "linux" ] && command -v systemctl &> /dev/null; then
    CURRENT_USER=$(whoami)
    echo "Service commands:"
    echo "  sudo systemctl start llama-server@$CURRENT_USER    # Start llama-server now"
    echo "  sudo systemctl status llama-server@$CURRENT_USER   # Check status"
    echo
fi
echo "For help:"
echo "  offgrid help"
echo
EOF

chmod +x "$PACKAGE_DIR/install.sh"

# Create installation script for Windows
cat > "$PACKAGE_DIR/install.bat" << 'EOF'
@echo off
echo OffGrid LLM - Windows Installation
echo ==================================
echo.

set INSTALL_DIR=%USERPROFILE%\.offgrid-llm
set MODELS_DIR=%USERPROFILE%\.offgrid-llm\models

echo Installing to: %INSTALL_DIR%
echo.

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%MODELS_DIR%" mkdir "%MODELS_DIR%"

echo Installing binary...
copy /Y bin\offgrid-windows.exe "%INSTALL_DIR%\offgrid.exe"

echo Installing models...
if exist "models\*.gguf" (
    copy /Y models\*.gguf "%MODELS_DIR%\"
    echo Models installed
) else (
    echo No models to install
)

echo.
echo Installation complete!
echo.
echo To start the server, run:
echo   %INSTALL_DIR%\offgrid.exe
echo.
echo Consider adding %INSTALL_DIR% to your PATH for easier access.
echo.
pause
EOF

# Create README for the package
cat > "$PACKAGE_DIR/README.txt" << 'EOF'
OffGrid LLM - Offline Installation Package
===========================================

This package contains everything needed to run OffGrid LLM completely offline.

Contents:
---------
  bin/       - Binaries for different platforms
  models/    - Pre-downloaded AI models
  docs/      - Documentation
  install.sh - Linux/Mac installation script
  install.bat- Windows installation script

Installation:
-------------

Linux/Mac:
  1. Open terminal in this directory
  2. Run: ./install.sh
  3. Start server: offgrid

Windows:
  1. Double-click install.bat
  2. Run: %USERPROFILE%\.offgrid-llm\offgrid.exe

Manual Installation:
-------------------
  1. Copy the binary for your platform to a directory in your PATH
  2. Copy models/*.gguf to ~/.offgrid-llm/models/
  3. Run: offgrid

Usage:
------
  offgrid           - Start the server
  offgrid catalog   - Show available models
  offgrid list      - List installed models
  offgrid help      - Show help

Documentation:
--------------
  See docs/ folder or README.md for full documentation

Support:
--------
  For issues and updates, visit:
  https://github.com/takuphilchan/offgrid-llm
EOF

# Calculate package size
echo
echo "📊 Package Summary"
echo "=================="

TOTAL_SIZE=$(du -sh "$PACKAGE_DIR" | cut -f1)
BIN_COUNT=$(ls -1 "$PACKAGE_DIR/bin" | wc -l)
MODEL_COUNT=$(ls -1 "$PACKAGE_DIR/models"/*.gguf 2>/dev/null | wc -l || echo "0")

echo "  Location: $PACKAGE_DIR"
echo "  Total size: $TOTAL_SIZE"
echo "  Binaries: $BIN_COUNT platforms"
echo "  Models: $MODEL_COUNT"
echo

# Create checksum file
echo "🔐 Creating checksums..."
cd "$PACKAGE_DIR"
find . -type f -exec sha256sum {} \; > CHECKSUMS.txt
cd - > /dev/null

echo
echo "✅ USB distribution package created successfully!"
echo
echo "Next steps:"
echo "  1. Safely eject USB drive"
echo "  2. Transport to offline location"
echo "  3. Run install script on target machine"
echo
echo "Package location: $PACKAGE_DIR"
echo
