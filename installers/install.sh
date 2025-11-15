#!/bin/bash
# OffGrid LLM Easy Installer
# Automatically installs llama.cpp + OffGrid LLM in one command
# Usage: curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash

set -e

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Functions
print_banner() {
    echo ""
    echo -e "${CYAN}"
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║     ██████╗ ███████╗███████╗ ██████╗ ██████╗ ██╗██████╗      ║
║    ██╔═══██╗██╔════╝██╔════╝██╔════╝ ██╔══██╗██║██╔══██╗     ║
║    ██║   ██║█████╗  █████╗  ██║  ███╗██████╔╝██║██║  ██║     ║
║    ██║   ██║██╔══╝  ██╔══╝  ██║   ██║██╔══██╗██║██║  ██║     ║
║    ╚██████╔╝██║     ██║     ╚██████╔╝██║  ██║██║██████╔╝     ║
║     ╚═════╝ ╚═╝     ╚═╝      ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝      ║
║                                                               ║
║               E A S Y   I N S T A L L E R                     ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

print_step() { echo -e "${CYAN}[$(date +%H:%M:%S)]${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1" >&2; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }

# Detect platform
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            LLAMACPP_ARCH="x64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            LLAMACPP_ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)
            LLAMACPP_VARIANT="ubuntu"
            if [ "$ARCH" = "arm64" ]; then
                LLAMACPP_FILE="llama-{VERSION}-bin-ubuntu-arm64.zip"
            else
                # Detect GPU for x64 Linux
                detect_gpu_linux
                # Build filename based on GPU variant
                if [ "$GPU_VARIANT" = "vulkan" ]; then
                    LLAMACPP_FILE="llama-{VERSION}-bin-ubuntu-vulkan-x64.zip"
                else
                    LLAMACPP_FILE="llama-{VERSION}-bin-ubuntu-x64.zip"
                fi
            fi
            ;;
        darwin)
            LLAMACPP_VARIANT="macos"
            if [ "$ARCH" = "arm64" ]; then
                LLAMACPP_FILE="llama-{VERSION}-bin-macos-arm64.zip"
            else
                LLAMACPP_FILE="llama-{VERSION}-bin-macos-x64.zip"
            fi
            ;;
        *)
            print_error "Unsupported OS: $OS (use install-windows.ps1 for Windows)"
            exit 1
            ;;
    esac

    print_success "Detected: $OS-$ARCH${GPU_INFO}"
}

# Detect GPU for Linux x64
detect_gpu_linux() {
    GPU_VARIANT="cpu"
    GPU_INFO=""
    
    # Check for NVIDIA GPU (Vulkan is best for pre-built binaries)
    if command -v nvidia-smi &> /dev/null && nvidia-smi &> /dev/null; then
        GPU_NAME=$(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -1)
        print_success "NVIDIA GPU detected: $GPU_NAME"
        
        # Check if Vulkan is available
        if command -v vulkaninfo &> /dev/null || [ -f /usr/lib/x86_64-linux-gnu/libvulkan.so.1 ]; then
            GPU_VARIANT="vulkan"
            GPU_INFO=" (Vulkan GPU)"
            print_success "Using Vulkan-accelerated binary"
        else
            print_warning "Vulkan not found. Installing CPU-only binary."
            print_warning "For GPU acceleration, install vulkan-tools:"
            echo "  sudo apt-get install vulkan-tools libvulkan1"
            GPU_INFO=" (CPU-only - install vulkan-tools for GPU)"
        fi
        return
    fi
    
    # Check for AMD GPU
    if command -v rocm-smi &> /dev/null && rocm-smi &> /dev/null; then
        GPU_NAME=$(rocm-smi --showproductname 2>/dev/null | grep "GPU" | head -1)
        print_success "AMD GPU detected: $GPU_NAME"
        
        # Check if Vulkan is available
        if command -v vulkaninfo &> /dev/null || [ -f /usr/lib/x86_64-linux-gnu/libvulkan.so.1 ]; then
            GPU_VARIANT="vulkan"
            GPU_INFO=" (Vulkan GPU)"
            print_success "Using Vulkan-accelerated binary"
        else
            print_warning "Vulkan not found. Installing CPU-only binary."
            print_warning "For GPU acceleration, install vulkan-tools:"
            echo "  sudo apt-get install mesa-vulkan-drivers vulkan-tools"
            GPU_INFO=" (CPU-only - install vulkan-tools for GPU)"
        fi
        return
    fi
    
    # Check for Intel GPU with Vulkan
    if [ -f /sys/class/drm/card0/device/vendor ] && grep -q "0x8086" /sys/class/drm/card0/device/vendor 2>/dev/null; then
        if command -v vulkaninfo &> /dev/null || [ -f /usr/lib/x86_64-linux-gnu/libvulkan.so.1 ]; then
            GPU_VARIANT="vulkan"
            GPU_INFO=" (Vulkan GPU - Intel)"
            print_success "Intel GPU detected, using Vulkan binary"
            return
        fi
    fi
    
    # No GPU detected or no Vulkan support
    GPU_INFO=" (CPU-only)"
    print_step "No GPU detected or Vulkan not available - using CPU-only binary"
}

# Check dependencies
check_dependencies() {
    MISSING=()
    
    for cmd in curl tar unzip; do
        if ! command -v $cmd &> /dev/null; then
            MISSING+=($cmd)
        fi
    done
    
    if [ ${#MISSING[@]} -ne 0 ]; then
        print_error "Missing required tools: ${MISSING[*]}"
        print_step "Install them with:"
        if [ "$OS" = "darwin" ]; then
            echo "  brew install ${MISSING[*]}"
        else
            echo "  sudo apt-get install ${MISSING[*]}  # Debian/Ubuntu"
            echo "  sudo yum install ${MISSING[*]}      # RHEL/CentOS"
        fi
        exit 1
    fi
}

# Install llama.cpp
install_llamacpp() {
    print_step "Checking llama.cpp installation..."
    
    if command -v llama-server &> /dev/null; then
        VERSION_INFO=$(llama-server --version 2>&1 | head -n1 || echo "unknown")
        print_success "llama.cpp already installed: $VERSION_INFO"
        return 0
    fi
    
    print_step "Installing llama.cpp (inference engine)..."
    
    # Get latest release
    print_step "Fetching latest llama.cpp release..."
    LLAMACPP_API=$(curl -sL "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest")
    LLAMACPP_VERSION=$(echo "$LLAMACPP_API" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$LLAMACPP_VERSION" ]; then
        print_error "Failed to fetch llama.cpp version"
        print_warning "You can install manually from: https://github.com/ggml-org/llama.cpp/releases"
        exit 1
    fi
    
    print_success "Latest llama.cpp: $LLAMACPP_VERSION"
    
    # Build download URL
    LLAMACPP_FILE="${LLAMACPP_FILE/\{VERSION\}/$LLAMACPP_VERSION}"
    LLAMACPP_URL="https://github.com/ggml-org/llama.cpp/releases/download/${LLAMACPP_VERSION}/${LLAMACPP_FILE}"
    
    # Download
    TMPDIR=$(mktemp -d)
    trap "rm -rf $TMPDIR" EXIT
    
    print_step "Downloading llama.cpp..."
    if ! curl -fsSL -o "$TMPDIR/llama.zip" "$LLAMACPP_URL"; then
        # If GPU binary download failed, try CPU fallback
        if [ "$GPU_VARIANT" != "cpu" ]; then
            print_warning "GPU binary download failed, trying CPU-only version..."
            LLAMACPP_FILE_CPU=$(echo "$LLAMACPP_FILE" | sed 's/-vulkan//')
            LLAMACPP_URL_CPU="https://github.com/ggml-org/llama.cpp/releases/download/${LLAMACPP_VERSION}/${LLAMACPP_FILE_CPU}"
            
            if curl -fsSL -o "$TMPDIR/llama.zip" "$LLAMACPP_URL_CPU"; then
                print_success "Downloaded CPU-only version as fallback"
                GPU_INFO=" (CPU-only - GPU binary unavailable)"
            else
                print_error "Download failed: $LLAMACPP_URL_CPU"
                print_warning "Install manually from: https://github.com/ggml-org/llama.cpp/releases"
                exit 1
            fi
        else
            print_error "Download failed: $LLAMACPP_URL"
            print_warning "Install manually from: https://github.com/ggml-org/llama.cpp/releases"
            exit 1
        fi
    fi
    
    # Extract
    print_step "Extracting llama.cpp..."
    unzip -q "$TMPDIR/llama.zip" -d "$TMPDIR"
    
    # Find binaries - they're usually in build/bin/ or bin/
    LLAMA_BIN_DIR=""
    for search_path in "$TMPDIR/build/bin" "$TMPDIR/bin" "$TMPDIR"; do
        if [ -f "$search_path/llama-server" ]; then
            LLAMA_BIN_DIR="$search_path"
            break
        fi
    done
    
    if [ -z "$LLAMA_BIN_DIR" ] || [ ! -f "$LLAMA_BIN_DIR/llama-server" ]; then
        print_error "Could not find llama-server binary in archive"
        print_warning "Install manually from: https://github.com/ggml-org/llama.cpp/releases"
        exit 1
    fi
    
    # Install binaries and libraries
    INSTALL_BIN="/usr/local/bin"
    INSTALL_LIB="/usr/local/lib"
    
    print_step "Installing llama-server to $INSTALL_BIN..."
    if [ -w "$INSTALL_BIN" ]; then
        cp "$LLAMA_BIN_DIR/llama-server" "$INSTALL_BIN/llama-server"
        chmod +x "$INSTALL_BIN/llama-server"
    else
        sudo cp "$LLAMA_BIN_DIR/llama-server" "$INSTALL_BIN/llama-server"
        sudo chmod +x "$INSTALL_BIN/llama-server"
    fi
    
    # Install shared libraries (including GPU backends)
    print_step "Installing shared libraries..."
    for lib in "$LLAMA_BIN_DIR"/lib*.so*; do
        if [ -f "$lib" ]; then
            lib_name=$(basename "$lib")
            if [ -w "$INSTALL_LIB" ]; then
                cp "$lib" "$INSTALL_LIB/$lib_name"
            else
                sudo cp "$lib" "$INSTALL_LIB/$lib_name"
            fi
        fi
    done
    
    # Update library cache
    if [ -w "/etc/ld.so.cache" ]; then
        ldconfig
    else
        sudo ldconfig 2>/dev/null || true
    fi
    
    print_success "llama.cpp installed successfully!"
}

# Install OffGrid
install_offgrid() {
    print_step "Installing OffGrid LLM..."
    
    # Get latest release
    print_step "Fetching latest OffGrid release..."
    OFFGRID_API=$(curl -sL "https://api.github.com/repos/takuphilchan/offgrid-llm/releases/latest")
    OFFGRID_VERSION=$(echo "$OFFGRID_API" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$OFFGRID_VERSION" ]; then
        print_error "Failed to fetch OffGrid version"
        exit 1
    fi
    
    print_success "Latest OffGrid: $OFFGRID_VERSION"
    
    # Build download URL
    OFFGRID_FILE="offgrid-${OS}-${ARCH}.tar.gz"
    OFFGRID_URL="https://github.com/takuphilchan/offgrid-llm/releases/download/${OFFGRID_VERSION}/${OFFGRID_FILE}"
    
    # Download
    TMPDIR=$(mktemp -d)
    trap "rm -rf $TMPDIR" EXIT
    
    print_step "Downloading OffGrid..."
    if ! curl -fsSL -o "$TMPDIR/offgrid.tar.gz" "$OFFGRID_URL"; then
        print_error "Download failed: $OFFGRID_URL"
        exit 1
    fi
    
    # Extract
    print_step "Extracting OffGrid..."
    tar -xzf "$TMPDIR/offgrid.tar.gz" -C "$TMPDIR"
    
    # Find binary
    OFFGRID_BIN=""
    for search_path in "$TMPDIR/offgrid-${OS}-${ARCH}" "$TMPDIR/offgrid"; do
        if [ -f "$search_path" ]; then
            OFFGRID_BIN="$search_path"
            break
        fi
    done
    
    if [ -z "$OFFGRID_BIN" ]; then
        print_error "Could not find offgrid binary in archive"
        exit 1
    fi
    
    # Install
    INSTALL_DIR="/usr/local/bin"
    print_step "Installing offgrid to $INSTALL_DIR..."
    
    # Check if offgrid is currently running
    if pgrep -x offgrid > /dev/null 2>&1; then
        print_warning "OffGrid is currently running. Stopping it first..."
        # Try without sudo first, then with sudo if needed
        if ! pkill -x offgrid 2>/dev/null; then
            sudo pkill -x offgrid 2>/dev/null || true
        fi
        sleep 1
    fi
    
    # Remove old binary if it exists (handles "Text file busy" error)
    if [ -f "$INSTALL_DIR/offgrid" ]; then
        print_step "Removing old version..."
        if [ -w "$INSTALL_DIR" ]; then
            rm -f "$INSTALL_DIR/offgrid" 2>/dev/null || {
                print_warning "Cannot remove old binary (may be in use)"
                print_step "Trying with sudo..."
                sudo rm -f "$INSTALL_DIR/offgrid"
            }
        else
            sudo rm -f "$INSTALL_DIR/offgrid"
        fi
    fi
    
    # Install new binary
    if [ -w "$INSTALL_DIR" ]; then
        cp "$OFFGRID_BIN" "$INSTALL_DIR/offgrid"
        chmod +x "$INSTALL_DIR/offgrid"
    else
        sudo cp "$OFFGRID_BIN" "$INSTALL_DIR/offgrid"
        sudo chmod +x "$INSTALL_DIR/offgrid"
    fi
    
    # Create config directory
    CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/offgrid"
    mkdir -p "$CONFIG_DIR"
    
    print_success "OffGrid installed successfully!"
    
    # Verify
    if command -v offgrid &> /dev/null; then
        INSTALLED_VERSION=$(offgrid version 2>/dev/null | grep -oP 'v[0-9.]+' || echo "$OFFGRID_VERSION")
        print_success "Installation verified: $INSTALLED_VERSION"
    fi
}

# Setup llama-server systemd service (auto-start)
setup_llama_service() {
    # Only for Linux with systemd
    if [ "$OS" != "linux" ] || ! command -v systemctl &> /dev/null; then
        return 0
    fi
    
    print_step "Setting up llama-server auto-start service..."
    
    # Create startup script
    SCRIPT_DIR="/usr/local/bin"
    SCRIPT_PATH="$SCRIPT_DIR/llama-server-start.sh"
    
    print_step "Creating llama-server startup script..."
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
    echo "Please download a model first with: offgrid download <model-id>"
    exit 1
fi

# Find first available GGUF model (smallest first)
MODEL_FILE=$(find "$MODELS_DIR" -name "*.gguf" -type f | sort -h | head -1)

if [ -z "$MODEL_FILE" ]; then
    echo "No GGUF models found in $MODELS_DIR"
    echo "Please download a model first with: offgrid download <model-id>"
    exit 1
fi

echo "Starting llama-server with model: $(basename "$MODEL_FILE")"
echo "Port: $PORT"

# Start llama-server
exec llama-server \
    --model "$MODEL_FILE" \
    --port "$PORT" \
    --host 127.0.0.1 \
    -c 4096 \
    --threads 4 \
    --metrics
SCRIPT_EOF
    
    sudo chmod +x "$SCRIPT_PATH"
    print_success "Created startup script: $SCRIPT_PATH"
    
    # Create systemd service
    SERVICE_FILE="/etc/systemd/system/llama-server@.service"
    
    print_step "Creating systemd service..."
    sudo tee "$SERVICE_FILE" > /dev/null << 'SERVICE_EOF'
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
    
    print_success "Created systemd service: $SERVICE_FILE"
    
    # Create port config directory
    sudo mkdir -p /etc/offgrid
    echo "42382" | sudo tee /etc/offgrid/llama-port > /dev/null
    print_success "Configured llama-server port: 42382"
    
    # Enable and start service for current user
    CURRENT_USER=$(whoami)
    
    print_step "Enabling llama-server service for user: $CURRENT_USER"
    sudo systemctl daemon-reload
    sudo systemctl enable "llama-server@$CURRENT_USER"
    
    print_success "llama-server auto-start enabled!"
    print_warning "Note: llama-server will start on next boot"
    print_warning "To start now: sudo systemctl start llama-server@$CURRENT_USER"
    print_warning "You need to download a model first: offgrid download <model-id>"
}

# Main
main() {
    print_banner
    
    echo ""
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    print_step "  STEP 1/3: System Check"
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    detect_platform
    check_dependencies
    
    echo ""
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    print_step "  STEP 2/3: Install llama.cpp (Inference Engine)"
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    install_llamacpp
    
    echo ""
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    print_step "  STEP 3/4: Install OffGrid LLM"
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    install_offgrid
    
    echo ""
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    print_step "  STEP 4/4: Setup Auto-Start Service"
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    setup_llama_service
    
    echo ""
    echo -e "${GREEN}"
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║              🎉  INSTALLATION COMPLETE!  🎉                   ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
    
    echo ""
    print_success "All components installed successfully!"
    echo ""
    echo "  Installed:"
    echo "    • OffGrid LLM     (/usr/local/bin/offgrid)"
    echo "    • llama.cpp       (/usr/local/bin/llama-server)${GPU_INFO}"
    if [ "$OS" = "linux" ] && command -v systemctl &> /dev/null; then
        echo "    • Auto-start      (systemd service enabled)"
    fi
    echo ""
    
    # Show GPU info if applicable
    if [ "$GPU_VARIANT" = "vulkan" ]; then
        echo "  GPU Acceleration:"
        echo "    • Vulkan-accelerated inference enabled"
        echo "    • Use --gpu-layers flag to offload layers to GPU"
        echo ""
    elif command -v nvidia-smi &> /dev/null && nvidia-smi &> /dev/null; then
        echo "  GPU Detected but not enabled:"
        echo "    • Install vulkan-tools for GPU acceleration:"
        echo "      sudo apt-get install vulkan-tools libvulkan1"
        echo "    • Then reinstall: curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash"
        echo ""
    fi
    
    echo "  Get Started:"
    echo "    offgrid version           # Check version"
    echo "    offgrid server start      # Start API server"
    echo "    offgrid chat              # Interactive chat"
    echo ""
    
    if [ "$OS" = "linux" ] && command -v systemctl &> /dev/null; then
        CURRENT_USER=$(whoami)
        echo "  Service Management:"
        echo "    sudo systemctl start llama-server@$CURRENT_USER    # Start now"
        echo "    sudo systemctl status llama-server@$CURRENT_USER   # Check status"
        echo "    sudo journalctl -u llama-server@$CURRENT_USER      # View logs"
        echo ""
    fi
    
    echo "  Documentation:"
    echo "    https://github.com/takuphilchan/offgrid-llm"
    echo ""
}

# Run
main
