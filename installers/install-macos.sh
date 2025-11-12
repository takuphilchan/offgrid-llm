#!/bin/bash
# OffGrid LLM macOS Installation Script
# Simplified installer for macOS systems

set -e

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

# Functions
print_header() {
    echo ""
    echo -e "${CYAN}╭────────────────────────────────────────────────────────────────────╮${NC}"
    echo -e "${CYAN}│ $1${NC}"
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
    ║                m a c O S   I N S T A L L E R                  ║
    ║                                                               ║
    ╚═══════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

print_header "Installing OffGrid LLM for macOS"

# Check if running on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    print_error "This script is for macOS only"
    exit 1
fi

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "arm64" ]]; then
    print_info "Detected: Apple Silicon (ARM64)"
    BINARY_SUFFIX="darwin-arm64"
elif [[ "$ARCH" == "x86_64" ]]; then
    print_info "Detected: Intel (x86_64)"
    BINARY_SUFFIX="darwin-amd64"
else
    print_error "Unsupported architecture: $ARCH"
    exit 1
fi

# Installation paths
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/Library/Application Support/OffGrid"

# Check if binaries exist in current directory
if [[ ! -f "offgrid" ]]; then
    print_error "offgrid binary not found in current directory"
    print_info "Please extract the downloaded archive first"
    exit 1
fi

# Install binaries
print_info "Installing binaries to $INSTALL_DIR..."

if [[ -w "$INSTALL_DIR" ]]; then
    cp offgrid "$INSTALL_DIR/offgrid"
    [[ -f "llama-server" ]] && cp llama-server "$INSTALL_DIR/llama-server"
    chmod +x "$INSTALL_DIR/offgrid"
    [[ -f "$INSTALL_DIR/llama-server" ]] && chmod +x "$INSTALL_DIR/llama-server"
else
    sudo cp offgrid "$INSTALL_DIR/offgrid"
    [[ -f "llama-server" ]] && sudo cp llama-server "$INSTALL_DIR/llama-server"
    sudo chmod +x "$INSTALL_DIR/offgrid"
    [[ -f "$INSTALL_DIR/llama-server" ]] && sudo chmod +x "$INSTALL_DIR/llama-server"
fi

print_success "Installed offgrid to $INSTALL_DIR"
[[ -f "$INSTALL_DIR/llama-server" ]] && print_success "Installed llama-server to $INSTALL_DIR"

# Create config directory
print_info "Creating configuration directory..."
mkdir -p "$CONFIG_DIR"
print_success "Created: $CONFIG_DIR"

# Check if llama.cpp is installed via Homebrew
print_info "Checking for llama.cpp..."
if command -v llama-server &> /dev/null; then
    print_success "llama.cpp is already installed"
elif command -v brew &> /dev/null; then
    print_warning "llama.cpp not found"
    read -p "Install llama.cpp via Homebrew? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        brew install llama.cpp
        print_success "Installed llama.cpp"
    fi
else
    print_warning "llama.cpp not found and Homebrew not installed"
    print_info "You can install it later with: brew install llama.cpp"
fi

echo ""
print_header "Installation Complete!"

echo -e "${GRAY}Installation Details:${NC}"
echo -e "  Install Path:  ${GRAY}$INSTALL_DIR${NC}"
echo -e "  Config Path:   ${GRAY}$CONFIG_DIR${NC}"
echo -e "  Architecture:  ${GRAY}$ARCH${NC}"
echo ""

echo -e "${CYAN}Next Steps:${NC}"
echo -e "  ${GRAY}1. Verify installation:${NC}"
echo -e "     ${GRAY}offgrid --version${NC}"
echo ""
echo -e "  ${GRAY}2. Get started:${NC}"
echo -e "     ${GRAY}offgrid --help${NC}"
echo -e "     ${GRAY}offgrid server start${NC}"
echo ""

echo -e "${YELLOW}To uninstall:${NC}"
echo -e "  ${GRAY}sudo rm $INSTALL_DIR/offgrid${NC}"
echo -e "  ${GRAY}sudo rm $INSTALL_DIR/llama-server${NC}"
echo -e "  ${GRAY}rm -rf \"$CONFIG_DIR\"${NC}"
echo ""
