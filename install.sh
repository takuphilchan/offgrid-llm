#!/bin/bash
# OffGrid LLM Universal Installer
# Works on Linux, macOS, and Windows (Git Bash/WSL)
# Usage: curl -fsSL https://offgrid.dev/install | bash
#    or: curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash

set -e

# Configuration
REPO="takuphilchan/offgrid-llm"
GITHUB_URL="https://github.com/${REPO}"
INSTALL_DIR="/usr/local/bin"
VERSION="${VERSION:-latest}"  # Can be overridden with VERSION=v0.1.0 ./install.sh

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m'

# Logging functions (all output to stderr to avoid contaminating variable captures)
log_info() { echo -e "${CYAN}▶${NC} $1" >&2; }
log_success() { echo -e "${GREEN}✓${NC} $1" >&2; }
log_error() { echo -e "${RED}✗${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}⚠${NC} $1" >&2; }

# Print banner
print_banner() {
    echo ""
    echo -e "${CYAN}${BOLD}"
    cat << 'EOF'
╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║    ██████╗ ███████╗███████╗ ██████╗ ██████╗ ██╗██████╗   ║
║   ██╔═══██╗██╔════╝██╔════╝██╔════╝ ██╔══██╗██║██╔══██╗  ║
║   ██║   ██║█████╗  █████╗  ██║  ███╗██████╔╝██║██║  ██║  ║
║   ██║   ██║██╔══╝  ██╔══╝  ██║   ██║██╔══██╗██║██║  ██║  ║
║   ╚██████╔╝██║     ██║     ╚██████╔╝██║  ██║██║██████╔╝  ║
║    ╚═════╝ ╚═╝     ╚═╝      ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝   ║
║                                                           ║
║          U N I V E R S A L   I N S T A L L E R            ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

# Detect operating system
detect_os() {
    local os
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    
    case "$os" in
        linux*)
            echo "linux"
            ;;
        darwin*)
            echo "darwin"
            ;;
        mingw*|msys*|cygwin*)
            echo "windows"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch="$(uname -m)"
    
    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# Detect GPU support
detect_gpu() {
    local os="$1"
    local variant="cpu"
    
    if [ "$os" = "linux" ]; then
        # Check for Vulkan (most compatible)
        if command -v vulkaninfo >/dev/null 2>&1 && vulkaninfo --summary >/dev/null 2>&1; then
            log_info "Vulkan GPU detected"
            variant="vulkan"
        # Check for NVIDIA
        elif command -v nvidia-smi >/dev/null 2>&1; then
            log_info "NVIDIA GPU detected"
            variant="vulkan"  # Use Vulkan even for NVIDIA (more compatible than CUDA)
        # Check for AMD
        elif [ -d "/sys/class/drm" ] && ls /sys/class/drm/card*/device/vendor 2>/dev/null | xargs cat | grep -q "0x1002"; then
            log_info "AMD GPU detected"
            variant="vulkan"
        else
            log_warn "No GPU detected, using CPU-only version"
        fi
    elif [ "$os" = "darwin" ]; then
        # macOS: Check if Apple Silicon
        if [ "$(uname -m)" = "arm64" ]; then
            log_info "Apple Silicon detected - Metal support enabled"
            variant="metal"
        else
            log_warn "Intel Mac - using CPU-only version"
            variant="cpu"
        fi
    fi
    
    echo "$variant"
}

# Get latest release version
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
        log_error "Failed to fetch latest version"
        exit 1
    fi
    
    echo "$version"
}

# Download and extract bundle
download_bundle() {
    local os="$1"
    local arch="$2"
    local variant="$3"
    local version="$4"
    
    # Construct bundle name
    local bundle_name="offgrid-${version}-${os}-${arch}-${variant}"
    local ext=".tar.gz"
    [ "$os" = "windows" ] && ext=".zip"
    
    local download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
    local tmp_dir="/tmp/offgrid-install-$$"
    
    log_info "Downloading: ${bundle_name}${ext}"
    
    mkdir -p "$tmp_dir"
    cd "$tmp_dir"
    
    # Download
    if ! curl -fsSL -o "bundle${ext}" "$download_url"; then
        log_error "Download failed. URL: $download_url"
        
        # Try CPU fallback if GPU version failed
        if [ "$variant" != "cpu" ]; then
            log_warn "GPU version not available, trying CPU version..."
            variant="cpu"
            bundle_name="offgrid-${version}-${os}-${arch}-${variant}"
            download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
            
            if ! curl -fsSL -o "bundle${ext}" "$download_url"; then
                log_error "CPU version download also failed"
                exit 1
            fi
        else
            exit 1
        fi
    fi
    
    log_success "Downloaded successfully"
    
    # Extract
    log_info "Extracting bundle..."
    if [ "$os" = "windows" ]; then
        unzip -q "bundle${ext}"
    else
        tar -xzf "bundle${ext}"
    fi
    
    # Find extracted directory
    local extracted_dir=$(find . -maxdepth 1 -type d -name "offgrid-*" | head -1)
    
    if [ -z "$extracted_dir" ]; then
        log_error "Failed to find extracted directory"
        exit 1
    fi
    
    echo "$tmp_dir/$extracted_dir"
}

# Install binaries
install_binaries() {
    local bundle_dir="$1"
    local os="$2"
    
    log_info "Installing binaries to $INSTALL_DIR..."
    
    # Check if we need sudo
    local use_sudo=""
    if [ ! -w "$INSTALL_DIR" ]; then
        if command -v sudo >/dev/null 2>&1; then
            use_sudo="sudo"
        else
            log_error "No write permission to $INSTALL_DIR and sudo not available"
            log_info "Try running as root or install to a user directory"
            exit 1
        fi
    fi
    
    # Copy binaries
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"
    
    $use_sudo cp "$bundle_dir/offgrid${ext}" "$INSTALL_DIR/"
    $use_sudo cp "$bundle_dir/llama-server${ext}" "$INSTALL_DIR/"
    $use_sudo chmod +x "$INSTALL_DIR/offgrid${ext}" "$INSTALL_DIR/llama-server${ext}"
    
    log_success "Binaries installed successfully"
}

# Verify checksums
verify_checksums() {
    local bundle_dir="$1"
    
    if [ ! -f "$bundle_dir/checksums.sha256" ]; then
        log_warn "Checksums file not found, skipping verification"
        return
    fi
    
    log_info "Verifying checksums..."
    
    cd "$bundle_dir"
    if sha256sum -c checksums.sha256 >/dev/null 2>&1; then
        log_success "Checksums verified"
    else
        log_error "Checksum verification failed!"
        exit 1
    fi
}

# Setup systemd service (Linux only, optional)
setup_systemd() {
    if [ "$(detect_os)" != "linux" ]; then
        return
    fi
    
    if ! command -v systemctl >/dev/null 2>&1; then
        return
    fi
    
    echo ""
    read -p "Setup auto-start on boot? (systemd) [y/N] " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        return
    fi
    
    log_info "Setting up systemd services..."
    
    # Create llama-server service
    sudo tee /etc/systemd/system/llama-server@.service >/dev/null << 'EOF'
[Unit]
Description=llama.cpp Inference Server for %i
After=network.target

[Service]
Type=simple
User=%i
ExecStart=/usr/local/bin/llama-server --port 48081 --host 127.0.0.1
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    
    # Create offgrid service
    sudo tee /etc/systemd/system/offgrid@.service >/dev/null << 'EOF'
[Unit]
Description=OffGrid LLM API Server for %i
After=network.target llama-server@%i.service
Wants=llama-server@%i.service

[Service]
Type=simple
User=%i
WorkingDirectory=/home/%i
ExecStart=/usr/local/bin/offgrid serve
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    
    # Enable and start services
    sudo systemctl daemon-reload
    sudo systemctl enable llama-server@${USER}.service
    sudo systemctl enable offgrid@${USER}.service
    sudo systemctl start llama-server@${USER}.service
    sudo systemctl start offgrid@${USER}.service
    
    log_success "Systemd services configured and started"
    log_info "Manage with: systemctl {start|stop|status} offgrid@${USER}.service"
}

# Cleanup
cleanup() {
    if [ -n "$tmp_dir" ] && [ -d "$tmp_dir" ]; then
        rm -rf "$tmp_dir"
    fi
}

trap cleanup EXIT

# Print success message
print_success() {
    echo ""
    echo -e "${GREEN}${BOLD}✓ Installation complete!${NC}"
    echo ""
    echo "Get started:"
    echo -e "  ${CYAN}offgrid --version${NC}"
    echo -e "  ${CYAN}offgrid search llama --limit 5${NC}"
    echo -e "  ${CYAN}offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF --file Llama-3.2-3B-Instruct-Q4_K_M.gguf${NC}"
    echo -e "  ${CYAN}offgrid run Llama-3.2-3B-Instruct-Q4_K_M${NC}"
    echo ""
    echo "Web UI: ${CYAN}http://localhost:11611/ui${NC}"
    echo "Docs: ${CYAN}https://github.com/${REPO}${NC}"
    echo ""
}

# Main installation flow
main() {
    print_banner
    
    # Check for required tools
    for cmd in curl tar; do
        if ! command -v $cmd >/dev/null 2>&1; then
            log_error "Required command not found: $cmd"
            exit 1
        fi
    done
    
    # Detect platform
    log_info "Detecting platform..."
    OS=$(detect_os)
    ARCH=$(detect_arch)
    VARIANT=$(detect_gpu "$OS")
    
    log_success "Platform: $OS-$ARCH-$VARIANT"
    
    # Get version
    if [ "$VERSION" = "latest" ]; then
        log_info "Fetching latest version..."
        VERSION=$(get_latest_version)
        log_success "Latest version: $VERSION"
    fi
    
    # Download bundle
    BUNDLE_DIR=$(download_bundle "$OS" "$ARCH" "$VARIANT" "$VERSION")
    
    # Verify checksums
    verify_checksums "$BUNDLE_DIR"
    
    # Install binaries
    install_binaries "$BUNDLE_DIR" "$OS"
    
    # Setup systemd (optional, Linux only)
    setup_systemd
    
    # Print success
    print_success
}

# Run main
main "$@"
