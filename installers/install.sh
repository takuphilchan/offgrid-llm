#!/bin/bash
# OffGrid LLM Universal Installer
# Auto-detects platform and installs the appropriate version
# Usage: curl -fsSL https://offgrid-llm.io/install.sh | bash

set -e

# Configuration
REPO="takuphilchan/offgrid-llm"
VERSION="${OFFGRID_VERSION:-latest}"
INSTALL_DIR="${OFFGRID_INSTALL_DIR:-/usr/local/bin}"

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
GRAY='\033[0;90m'
NC='\033[0m'

# Functions
print_header() {
    echo ""
    echo -e "${CYAN}╭────────────────────────────────────────────────────────────────────╮${NC}"
    echo -e "${CYAN}│ $1"
    echo -e "${CYAN}╰────────────────────────────────────────────────────────────────────╯${NC}"
    echo ""
}

print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1" >&2; }
print_info() { echo -e "${CYAN}→${NC} $1"; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }

# Banner
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
    ║            U N I V E R S A L   I N S T A L L E R              ║
    ║                                                               ║
    ╚═══════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

print_header "Detecting System..."

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Normalize architecture names
case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        print_error "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Determine platform
case "$OS" in
    linux)
        PLATFORM="linux"
        FILE_EXT=""
        ARCHIVE_EXT="tar.gz"
        ;;
    darwin)
        PLATFORM="darwin"
        FILE_EXT=""
        ARCHIVE_EXT="tar.gz"
        ;;
    mingw*|msys*|cygwin*)
        PLATFORM="windows"
        FILE_EXT=".exe"
        ARCHIVE_EXT="zip"
        ;;
    *)
        print_error "Unsupported operating system: $OS"
        exit 1
        ;;
esac

print_success "Detected: $PLATFORM ($ARCH)"

# Get latest version if not specified
if [ "$VERSION" = "latest" ]; then
    print_info "Fetching latest version..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        print_error "Failed to fetch latest version"
        exit 1
    fi
    print_success "Latest version: $VERSION"
fi

# Construct download URL (matches GitHub release artifact names)
# Workflow creates: offgrid-linux-amd64.tar.gz (not offgrid-v0.1.0-linux-amd64.tar.gz)
FILENAME="offgrid-${PLATFORM}-${ARCH}.${ARCHIVE_EXT}"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

print_header "Downloading OffGrid LLM"
print_info "Version: $VERSION"
print_info "Platform: $PLATFORM-$ARCH"
print_info "URL: $DOWNLOAD_URL"

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

cd "$TMP_DIR"

# Download with progress
print_info "Downloading..."
if command -v wget &> /dev/null; then
    wget --progress=bar:force -O "$FILENAME" "$DOWNLOAD_URL" 2>&1 | \
        grep --line-buffered -oP '\d+%' | \
        while read -r percent; do
            echo -ne "\r${CYAN}→${NC} Downloading... ${GREEN}$percent${NC}"
        done
    echo ""
elif command -v curl &> /dev/null; then
    curl -L --progress-bar -o "$FILENAME" "$DOWNLOAD_URL"
else
    print_error "Neither wget nor curl is available"
    exit 1
fi

print_success "Downloaded: $FILENAME"

# Extract archive
print_info "Extracting..."
case "$ARCHIVE_EXT" in
    tar.gz)
        tar -xzf "$FILENAME"
        ;;
    zip)
        unzip -q "$FILENAME"
        ;;
esac

print_success "Extracted"

# Run platform-specific installer
print_header "Installing OffGrid LLM"

case "$PLATFORM" in
    linux|darwin)
        # Check for install script in archive
        if [ -f "install.sh" ]; then
            chmod +x install.sh
            ./install.sh
        else
            # Manual installation
            print_info "Installing binaries to $INSTALL_DIR..."
            
            # Check write permissions
            if [ -w "$INSTALL_DIR" ]; then
                SUDO=""
            else
                SUDO="sudo"
                print_warning "Requires sudo for installation to $INSTALL_DIR"
            fi
            
            # Install offgrid binary (check various possible names)
            BINARY_NAME=""
            if [ -f "bin/offgrid" ]; then
                BINARY_NAME="bin/offgrid"
            elif [ -f "offgrid" ]; then
                BINARY_NAME="offgrid"
            elif [ -f "offgrid-${PLATFORM}-${ARCH}" ]; then
                BINARY_NAME="offgrid-${PLATFORM}-${ARCH}"
            else
                print_error "offgrid binary not found in archive"
                exit 1
            fi
            
            $SUDO install -m 755 "$BINARY_NAME" "$INSTALL_DIR/offgrid"
            print_success "Installed: offgrid"
            
            # Install llama-server if present
            if [ -f "bin/llama-server" ]; then
                $SUDO install -m 755 bin/llama-server "$INSTALL_DIR/llama-server"
                print_success "Installed: llama-server"
            elif [ -f "llama-server" ]; then
                $SUDO install -m 755 llama-server "$INSTALL_DIR/llama-server"
                print_success "Installed: llama-server"
            fi
            
            # Create config directory
            if [ "$PLATFORM" = "darwin" ]; then
                CONFIG_DIR="$HOME/Library/Application Support/OffGrid"
            else
                CONFIG_DIR="$HOME/.config/offgrid"
            fi
            
            mkdir -p "$CONFIG_DIR"
            print_success "Created config directory: $CONFIG_DIR"
        fi
        ;;
    
    windows)
        print_warning "Windows detected"
        print_info "For Windows, please download the installer from:"
        print_info "  https://github.com/$REPO/releases/download/$VERSION/OffGridSetup-${VERSION}.exe"
        print_info ""
        print_info "Or use PowerShell to install:"
        print_info "  powershell -ExecutionPolicy Bypass -File install.ps1"
        exit 0
        ;;
esac

# Verify installation
print_header "Verifying Installation"

if command -v offgrid &> /dev/null; then
    INSTALLED_VERSION=$(offgrid --version 2>&1 | grep -oP 'v\d+\.\d+\.\d+(-\w+)?' || echo "unknown")
    print_success "OffGrid LLM installed successfully!"
    print_info "Version: $INSTALLED_VERSION"
else
    print_warning "offgrid command not found in PATH"
    print_info "You may need to add $INSTALL_DIR to your PATH"
    print_info "Or restart your terminal"
fi

echo ""
echo -e "${GREEN}╭────────────────────────────────────────────────────────────────────╮${NC}"
echo -e "${GREEN}│ Installation Complete!                                              │${NC}"
echo -e "${GREEN}╰────────────────────────────────────────────────────────────────────╯${NC}"
echo ""

echo -e "${CYAN}Quick Start:${NC}"
echo -e "  ${GRAY}# Check version${NC}"
echo -e "  offgrid --version"
echo ""
echo -e "  ${GRAY}# Start the server${NC}"
echo -e "  offgrid server start"
echo ""
echo -e "  ${GRAY}# Get help${NC}"
echo -e "  offgrid --help"
echo ""

echo -e "${CYAN}Documentation:${NC}"
echo -e "  https://github.com/$REPO"
echo ""

echo -e "${YELLOW}Tip:${NC} Run 'offgrid doctor' to check system requirements and dependencies"
echo ""
