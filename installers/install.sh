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
                LLAMACPP_FILE="llama-{VERSION}-bin-ubuntu-x64.zip"
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

    print_success "Detected: $OS-$ARCH"
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
        print_error "Download failed: $LLAMACPP_URL"
        print_warning "Install manually from: https://github.com/ggml-org/llama.cpp/releases"
        exit 1
    fi
    
    # Extract
    print_step "Extracting llama.cpp..."
    unzip -q "$TMPDIR/llama.zip" -d "$TMPDIR"
    
    # Find binaries - they're usually in build/bin/ or bin/
    LLAMA_SERVER=""
    for search_path in "$TMPDIR/build/bin/llama-server" "$TMPDIR/bin/llama-server" "$TMPDIR/llama-server"; do
        if [ -f "$search_path" ]; then
            LLAMA_SERVER="$search_path"
            break
        fi
    done
    
    if [ -z "$LLAMA_SERVER" ]; then
        print_error "Could not find llama-server binary in archive"
        print_warning "Install manually from: https://github.com/ggml-org/llama.cpp/releases"
        exit 1
    fi
    
    # Install to /usr/local/bin
    INSTALL_DIR="/usr/local/bin"
    print_step "Installing llama-server to $INSTALL_DIR..."
    
    if [ -w "$INSTALL_DIR" ]; then
        cp "$LLAMA_SERVER" "$INSTALL_DIR/llama-server"
        chmod +x "$INSTALL_DIR/llama-server"
    else
        sudo cp "$LLAMA_SERVER" "$INSTALL_DIR/llama-server"
        sudo chmod +x "$INSTALL_DIR/llama-server"
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
    print_step "  STEP 3/3: Install OffGrid LLM"
    print_step "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    install_offgrid
    
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
    echo "    • llama.cpp       (/usr/local/bin/llama-server)"
    echo ""
    echo "  Get Started:"
    echo "    offgrid version           # Check version"
    echo "    offgrid server start      # Start API server"
    echo "    offgrid chat              # Interactive chat"
    echo ""
    echo "  Documentation:"
    echo "    https://github.com/takuphilchan/offgrid-llm"
    echo ""
}

# Run
main
