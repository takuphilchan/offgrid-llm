#!/bin/bash
# OffGrid LLM Universal Installer
# Works on Linux, macOS, and Windows (Git Bash/WSL)
# Usage: curl -fsSL https://offgrid.dev/install | bash
#    or: curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
#    or: AUTOSTART=yes bash install.sh  # Auto-enable and start services

set -e

# Configuration
REPO="takuphilchan/offgrid-llm"
GITHUB_URL="https://github.com/${REPO}"
INSTALL_DIR="/usr/local/bin"
VERSION="${VERSION:-latest}"  # Can be overridden with VERSION=v0.1.0 ./install.sh
AUTOSTART="${AUTOSTART:-ask}"  # Options: yes, no, ask

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

# Detect CPU instruction set support
detect_cpu_features() {
    local cpu_variant="avx2"
    
    if [ "$(uname -s)" = "Linux" ]; then
        if grep -q "avx512" /proc/cpuinfo 2>/dev/null; then
            cpu_variant="avx512"
        elif grep -q "avx2" /proc/cpuinfo 2>/dev/null; then
            cpu_variant="avx2"
        else
            cpu_variant="basic"
        fi
    elif [ "$(uname -s)" = "Darwin" ]; then
        # macOS - check with sysctl
        if sysctl machdep.cpu.features machdep.cpu.leaf7_features 2>/dev/null | grep -qi "avx512"; then
            cpu_variant="avx512"
        else
            # Most Macs have at least AVX2
            cpu_variant="avx2"
        fi
    fi
    
    echo "$cpu_variant"
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
    local cpu_features="$5"
    
    # Construct bundle name with CPU features
    local bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
    local ext=".tar.gz"
    [ "$os" = "windows" ] && ext=".zip"
    
    local download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
    local tmp_dir="/tmp/offgrid-install-$$"
    
    log_info "Downloading: ${bundle_name}${ext}"
    
    mkdir -p "$tmp_dir"
    cd "$tmp_dir"
    
    # Download with fallback logic
    if ! curl -fsSL -o "bundle${ext}" "$download_url"; then
        log_warn "Download failed: ${bundle_name}${ext}"
        
        # Try CPU variant fallback if GPU version failed
        if [ "$variant" != "cpu" ]; then
            log_warn "Trying CPU version instead..."
            variant="cpu"
            bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
            download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
            
            if ! curl -fsSL -o "bundle${ext}" "$download_url"; then
                log_warn "CPU-${cpu_features} version not available"
                
                # Try AVX2 fallback if AVX-512 failed
                if [ "$cpu_features" = "avx512" ]; then
                    log_warn "Trying AVX2 version (compatible with most CPUs)..."
                    cpu_features="avx2"
                    bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
                    download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
                    
                    if ! curl -fsSL -o "bundle${ext}" "$download_url"; then
                        log_error "All download attempts failed"
                        exit 1
                    fi
                else
                    exit 1
                fi
            fi
        else
            # Try AVX2 fallback for CPU variant too
            if [ "$cpu_features" = "avx512" ]; then
                log_warn "Trying AVX2 version (compatible with most CPUs)..."
                cpu_features="avx2"
                bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
                download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
                
                if ! curl -fsSL -o "bundle${ext}" "$download_url"; then
                    log_error "All download attempts failed"
                    exit 1
                fi
            else
                exit 1
            fi
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
    
    # Stop services if running (to release file locks)
    if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet offgrid@$USER 2>/dev/null; then
        log_info "Stopping offgrid service..."
        $use_sudo systemctl stop offgrid@$USER 2>/dev/null || true
    fi
    
    # Kill any running processes using the binaries and wait for them to exit
    if pgrep -x llama-server >/dev/null 2>&1; then
        log_info "Stopping running llama-server processes..."
        pkill -x llama-server 2>/dev/null || true
        # Wait up to 5 seconds for processes to exit
        for i in {1..10}; do
            pgrep -x llama-server >/dev/null 2>&1 || break
            sleep 0.5
        done
        # Force kill if still running
        if pgrep -x llama-server >/dev/null 2>&1; then
            log_warn "Force killing llama-server..."
            pkill -9 -x llama-server 2>/dev/null || true
            sleep 1
        fi
    fi
    
    # Same for offgrid
    if pgrep -x offgrid >/dev/null 2>&1; then
        log_info "Stopping running offgrid processes..."
        pkill -x offgrid 2>/dev/null || true
        for i in {1..10}; do
            pgrep -x offgrid >/dev/null 2>&1 || break
            sleep 0.5
        done
        if pgrep -x offgrid >/dev/null 2>&1; then
            pkill -9 -x offgrid 2>/dev/null || true
            sleep 1
        fi
    fi
    
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

# Setup systemd service (Linux only)
setup_systemd() {
    if [ "$(detect_os)" != "linux" ]; then
        echo "0"
        return
    fi
    
    if ! command -v systemctl >/dev/null 2>&1; then
        log_warn "systemd not available - you'll need to start services manually"
        echo "0"
        return
    fi
    
    log_info "Setting up systemd services for auto-start..."
    
    # Create offgrid service (offgrid manages llama-server internally)
    sudo tee /etc/systemd/system/offgrid@.service >/dev/null << 'EOF'
[Unit]
Description=OffGrid LLM API Server for %i
After=network.target

[Service]
Type=simple
User=%i
WorkingDirectory=/home/%i
Environment="HOME=/home/%i"
ExecStart=/usr/local/bin/offgrid serve
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    
    # Reload systemd
    sudo systemctl daemon-reload
    
    # Configure llama-server port
    sudo mkdir -p /etc/offgrid >/dev/null 2>&1
    echo "42382" | sudo tee /etc/offgrid/llama-port >/dev/null 2>&1
    
    # Determine what to do based on AUTOSTART variable
    local enable_services="no"
    local start_services="no"
    
    if [ "$AUTOSTART" = "yes" ]; then
        enable_services="yes"
        start_services="yes"
    elif [ "$AUTOSTART" = "no" ]; then
        enable_services="no"
        start_services="no"
    else
        # Interactive mode
        echo "" >&2
        echo -e "${CYAN}${BOLD}Service Configuration${NC}" >&2
        echo "" >&2
        read -t 30 -p "Enable services to start on boot? [Y/n] " -n 1 -r 2>/dev/null || REPLY="Y"
        echo >&2
        
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            enable_services="yes"
        fi
        
        echo "" >&2
        read -t 30 -p "Start services now? [Y/n] " -n 1 -r 2>/dev/null || REPLY="Y"
        echo >&2
        
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            start_services="yes"
        fi
    fi
    
    # Enable services if requested
    if [ "$enable_services" = "yes" ]; then
        sudo systemctl enable offgrid@${USER}.service >/dev/null 2>&1
        log_success "Service enabled for auto-start on boot"
    fi
    
    # Start services if requested
    if [ "$start_services" = "yes" ]; then
        log_info "Starting offgrid service..."
        
        # Start offgrid (it will manage llama-server internally)
        sudo systemctl start offgrid@${USER}.service 2>/dev/null
        
        # Wait for offgrid to be ready (up to 10 seconds)
        local retries=20
        local offgrid_ready=false
        for i in $(seq 1 $retries); do
            if systemctl is-active --quiet offgrid@${USER}.service 2>/dev/null; then
                # Try to hit health endpoint
                if curl -sf http://localhost:11611/health >/dev/null 2>&1; then
                    offgrid_ready=true
                    break
                fi
            fi
            sleep 0.5
        done
        
        # Check final status
        if [ "$offgrid_ready" = true ]; then
            log_success "Service started and ready!"
            echo ""
            echo -e "${GREEN}✓${NC} offgrid server running on port 11611" >&2
            echo -e "${CYAN}ℹ${NC} llama-server will start automatically when you run a model" >&2
            echo "1"  # Signal that services are running
            return
        elif systemctl is-active --quiet offgrid@${USER}.service 2>/dev/null; then
            log_success "Service started!"
            echo ""
            echo -e "${GREEN}✓${NC} offgrid server running on port 11611" >&2
            echo -e "${CYAN}ℹ${NC} llama-server will start automatically when you run a model" >&2
            log_warn "Service may take a moment to fully initialize..."
            echo "1"  # Signal that services are running
            return
        else
            log_warn "Service may need a moment to start"
            echo "   Check status with: systemctl status offgrid@${USER}.service" >&2
        fi
    fi
    
    echo "0"  # Services not started
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
    local services_running="$1"
    
    echo ""
    echo -e "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}${BOLD}  ✓ Installation Complete!${NC}"
    echo -e "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    
    if [ "$services_running" = "1" ]; then
        echo -e "${GREEN}${BOLD}🚀 Service is running and ready!${NC}"
        echo ""
        echo -e "  ${GREEN}✓${NC} Web UI:  ${CYAN}http://localhost:11611/ui${NC}"
        echo -e "  ${GREEN}✓${NC} API:     ${CYAN}http://localhost:11611${NC}"
        echo ""
        echo -e "${BOLD}Quick Start:${NC}"
        echo -e "  ${CYAN}offgrid search llama --limit 5${NC}"
        echo -e "  ${CYAN}offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF --file Llama-3.2-3B-Instruct-Q4_K_M.gguf${NC}"
        echo -e "  ${CYAN}offgrid run Llama-3.2-3B-Instruct-Q4_K_M${NC}"
        echo ""
        echo -e "${BOLD}Manage Service:${NC}"
        echo -e "  ${CYAN}sudo systemctl status offgrid@${USER}${NC}     # Check status"
        echo -e "  ${CYAN}sudo systemctl stop offgrid@${USER}${NC}       # Stop server"
        echo -e "  ${CYAN}sudo systemctl restart offgrid@${USER}${NC}    # Restart server"
    else
        echo -e "${YELLOW}⚠${NC}  ${BOLD}Service not started - manual setup required${NC}"
        echo ""
        echo -e "${BOLD}Start service manually:${NC}"
        
        if command -v systemctl >/dev/null 2>&1; then
            echo -e "  ${CYAN}sudo systemctl start offgrid@${USER}${NC}"
            echo ""
            echo -e "${BOLD}Or run directly:${NC}"
        fi
        
        echo -e "  ${CYAN}offgrid serve &${NC}"
        echo ""
        echo -e "${BOLD}Then try:${NC}"
        echo -e "  ${CYAN}offgrid search llama --limit 5${NC}"
        echo -e "  ${CYAN}offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF --file Llama-3.2-3B-Instruct-Q4_K_M.gguf${NC}"
        echo -e "  ${CYAN}offgrid run Llama-3.2-3B-Instruct-Q4_K_M${NC}"
    fi
    
    echo ""
    echo -e "${BOLD}Documentation:${NC} ${CYAN}https://github.com/${REPO}${NC}"
    echo -e "${GREEN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
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
    CPU_FEATURES=$(detect_cpu_features)
    
    log_success "Platform: $OS-$ARCH-$VARIANT-$CPU_FEATURES"
    
    # Get version
    if [ "$VERSION" = "latest" ]; then
        log_info "Fetching latest version..."
        VERSION=$(get_latest_version)
        log_success "Latest version: $VERSION"
    fi
    
    # Download bundle
    BUNDLE_DIR=$(download_bundle "$OS" "$ARCH" "$VARIANT" "$VERSION" "$CPU_FEATURES")
    
    # Verify checksums
    verify_checksums "$BUNDLE_DIR"
    
    # Install binaries
    install_binaries "$BUNDLE_DIR" "$OS"
    
    # Setup systemd (outputs "1" if services started, "0" if not)
    SERVICES_RUNNING=$(setup_systemd)
    
    # Print success
    print_success "$SERVICES_RUNNING"
    
    # Exit successfully
    exit 0
}

# Run main
main "$@"
