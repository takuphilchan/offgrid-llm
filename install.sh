#!/bin/bash
# OffGrid LLM Installation Script
# Comprehensive installation with GPU support, llama.cpp compilation, and systemd integration

set -eu

# Installation options
FORCE_CPU_ONLY=false
FORCE_GPU=false

# Lock file to prevent concurrent installations
LOCK_FILE="/tmp/offgrid-install.lock"

# Cleanup function
cleanup() {
    rm -f "$LOCK_FILE"
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Color definitions (matching CLI brand colors)
BRAND_PRIMARY='\033[38;5;45m'      # Bright cyan (#00d4ff)
BRAND_SECONDARY='\033[38;5;141m'   # Purple (#af87ff)
BRAND_ACCENT='\033[38;5;226m'      # Yellow (#ffff00)
BRAND_SUCCESS='\033[38;5;78m'      # Green (#5fd787)
BRAND_ERROR='\033[38;5;196m'       # Red (#ff005f)
BRAND_MUTED='\033[38;5;240m'       # Gray (#585858)
RESET='\033[0m'
BOLD='\033[1m'
DIM='\033[2m'

# Progress tracking
TOTAL_STEPS=14
CURRENT_STEP=0
START_TIME=$(date +%s)

# ASCII Art Banner
print_banner() {
    echo -e "${BRAND_PRIMARY}${BOLD}"
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
    ║                  E D G E   I N F E R E N C E                  ║
    ║                                                               ║
    ╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${RESET}"
    echo -e "${BRAND_MUTED}    Offline-first AI for edge environments${RESET}"
    echo ""
}

# Print Functions
print_header() {
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}$1${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo ""
}

print_success() { echo -e "${BRAND_SUCCESS}✓${RESET} $1"; }
print_error() { echo -e "${BRAND_ERROR}✗${RESET} $1" >&2; }
print_info() { echo -e "${BRAND_PRIMARY}→${RESET} $1"; }
print_warning() { echo -e "${BRAND_ACCENT}⚡${RESET} $1"; }
print_step() { echo -e "${BOLD}${BRAND_PRIMARY}▸${RESET} $1"; }
print_divider() { echo -e "${BRAND_MUTED}$(printf '━%.0s' {1..70})${RESET}"; }
print_dim() { echo -e "${DIM}$1${RESET}"; }

# Progress tracking functions
get_elapsed_time() {
    local now=$(date +%s)
    local elapsed=$((now - START_TIME))
    printf "%02d:%02d" $((elapsed / 60)) $((elapsed % 60))
}

print_progress() {
    CURRENT_STEP=$((CURRENT_STEP + 1))
    local percent=$((CURRENT_STEP * 100 / TOTAL_STEPS))
    local elapsed=$(get_elapsed_time)
    
    echo ""
    echo -e "${BRAND_MUTED}╭────────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_MUTED}│${RESET} ${BOLD}Step ${CURRENT_STEP}/${TOTAL_STEPS}${RESET} ${BRAND_MUTED}[${BRAND_SUCCESS}$(printf '█%.0s' $(seq 1 $((percent / 5))))$(printf '░%.0s' $(seq 1 $((20 - percent / 5))))${BRAND_MUTED}] ${BRAND_ACCENT}${percent}%${RESET} ${BRAND_MUTED}│ ${DIM}Elapsed: ${elapsed}${RESET}"
    echo -e "${BRAND_MUTED}╰────────────────────────────────────────────────────────────────────╯${RESET}"
    
    print_header "$1"
    if [ -n "$2" ]; then
        echo -e "${DIM}  Estimated time: $2${RESET}"
        echo ""
    fi
}

# Usage/Help
usage() {
    echo -e "${BRAND_MUTED}$(printf '━%.0s' {1..70})${RESET}"
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}Usage${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo "  ./install.sh [OPTIONS]"
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}Options${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo "  --cpu-only          Force CPU-only mode (skip GPU detection)"
    echo "  --gpu               Force GPU mode (fail if no GPU detected)"
    echo "  --help, -h          Show this help message"
    echo ""
    echo -e "${BRAND_PRIMARY}◆${RESET} ${BOLD}Examples${RESET}"
    echo -e "${BRAND_MUTED}$(printf '─%.0s' {1..60})${RESET}"
    echo -e "  ${BRAND_MUTED}\$${RESET} ./install.sh                    # Auto-detect GPU"
    echo -e "  ${BRAND_MUTED}\$${RESET} ./install.sh --cpu-only         # CPU-only mode"
    echo -e "  ${BRAND_MUTED}\$${RESET} ./install.sh --gpu              # Require GPU"
    echo ""
    echo -e "${BRAND_MUTED}$(printf '━%.0s' {1..70})${RESET}"
    echo ""
    exit 0
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --cpu-only)
                FORCE_CPU_ONLY=true
                print_info "CPU-only mode enabled"
                shift
                ;;
            --gpu)
                FORCE_GPU=true
                print_info "GPU mode required"
                shift
                ;;
            --help|-h)
                usage
                ;;
            *)
                print_error "Unknown option: $1"
                usage
                ;;
        esac
    done

    # Validate conflicting options
    if [ "$FORCE_CPU_ONLY" = true ] && [ "$FORCE_GPU" = true ]; then
        print_error "Cannot use --cpu-only and --gpu together"
        exit 1
    fi
}

# Error Handler
handle_error() {
    print_error "Installation failed at: $1"
    print_info "Check the error message above for details"
    exit 1
}

trap 'handle_error "line $LINENO"' ERR

# Dependency Checks
check_dependencies() {
    print_progress "Checking System Dependencies" "~30 seconds"
    
    local missing=()
    local total=7
    local checked=0
    
    for cmd in curl awk grep sed tee xargs git; do
        checked=$((checked + 1))
        if ! command -v $cmd &> /dev/null; then
            missing+=($cmd)
            print_warning "[$checked/$total] $cmd not found"
        else
            print_success "[$checked/$total] $cmd is available"
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        echo ""
        print_error "Missing ${#missing[@]} required dependencies: ${missing[*]}"
        print_info "Installing missing dependencies..."
        echo ""
        
        if command -v apt-get &> /dev/null; then
            sudo apt-get update -qq
            sudo apt-get install -y -qq "${missing[@]}" snapd 2>&1 | grep -v "^Selecting\|^Preparing\|^Unpacking" || true
        elif command -v dnf &> /dev/null; then
            sudo dnf install -y -q "${missing[@]}" snapd
        elif command -v yum &> /dev/null; then
            sudo yum install -y -q "${missing[@]}" snapd
        else
            print_error "Unable to install dependencies automatically"
            print_info "Please install: ${missing[*]} snapd"
            exit 1
        fi
        
        print_success "All dependencies installed"
    else
        print_success "All dependencies are available"
    fi
}

# Architecture Detection
detect_architecture() {
    print_progress "Detecting System Architecture" "~5 seconds"
    
    local arch=$(uname -m)
    case $arch in
        x86_64|amd64)
            ARCH="amd64"
            print_success "Architecture: x86_64 (amd64)"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            print_success "Architecture: aarch64 (arm64)"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# OS Detection
detect_os() {
    print_progress "Detecting Operating System" "~5 seconds"
    
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_ID=$ID
        OS_VERSION=$VERSION_ID
        print_success "OS: $NAME $VERSION"
        
        case $OS_ID in
            ubuntu|debian|pop|linuxmint)
                PKG_MANAGER="apt-get"
                ;;
            fedora|rhel|centos|rocky|almalinux)
                PKG_MANAGER="dnf"
                ;;
            amzn)
                PKG_MANAGER="yum"
                ;;
            *)
                print_warning "Unknown distribution: $OS_ID (will attempt to continue)"
                PKG_MANAGER="apt-get"
                ;;
        esac
    else
        print_error "/etc/os-release not found - cannot detect OS"
        exit 1
    fi
}

# GPU Detection
detect_gpu() {
    print_progress "Detecting GPU Hardware" "~10 seconds"
    
    # Check if CPU-only mode is forced
    if [ "$FORCE_CPU_ONLY" = true ]; then
        GPU_TYPE="none"
        print_info "CPU-only mode forced (skipping GPU detection)"
        return
    fi
    
    GPU_TYPE="none"
    
    # Method 1: Check nvidia-smi (most reliable for NVIDIA)
    if command -v nvidia-smi &> /dev/null && nvidia-smi &> /dev/null; then
        GPU_TYPE="nvidia"
        GPU_INFO=$(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -n1)
        DRIVER_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -n1)
        CUDA_VERSION=$(nvidia-smi --query-gpu=compute_cap --format=csv,noheader 2>/dev/null | head -n1)
        print_success "NVIDIA GPU detected: $GPU_INFO"
        print_info "  Driver: $DRIVER_VERSION | Compute Capability: $CUDA_VERSION"
    
    # Method 2: Check lspci for NVIDIA GPU (vendor ID: 10de)
    elif lspci 2>/dev/null | grep -i 'vga.*nvidia\|3d.*nvidia\|display.*nvidia' &> /dev/null || \
         lspci -n 2>/dev/null | grep -E '(0300|0302):.*10de:' &> /dev/null; then
        GPU_TYPE="nvidia"
        GPU_INFO=$(lspci | grep -i 'vga.*nvidia\|3d.*nvidia\|display.*nvidia' | head -n1)
        print_success "NVIDIA GPU detected: $GPU_INFO"
        print_warning "  nvidia-smi not working - drivers may need to be installed/loaded"
    
    # Method 3: Check for AMD GPU (vendor ID: 1002)
    elif lspci 2>/dev/null | grep -i 'vga.*amd\|vga.*ati\|3d.*amd' &> /dev/null || \
         lspci -n 2>/dev/null | grep -E '(0300|0302):.*1002:' &> /dev/null; then
        GPU_TYPE="amd"
        GPU_INFO=$(lspci | grep -i 'vga.*amd\|vga.*ati\|3d.*amd' | head -n1)
        print_success "AMD GPU detected: $GPU_INFO"
        
        # Check for ROCm
        if command -v rocm-smi &> /dev/null; then
            ROCM_VERSION=$(rocm-smi --showdriverversion 2>/dev/null | grep -oP 'ROCm version: \K[0-9.]+' || echo "unknown")
            print_info "  ROCm version: $ROCM_VERSION"
        else
            print_warning "  rocm-smi not found - ROCm drivers may need to be installed"
        fi
    
    # Method 4: Check /proc/driver/nvidia for NVIDIA in WSL
    elif [ -d "/proc/driver/nvidia" ]; then
        GPU_TYPE="nvidia"
        GPU_INFO="NVIDIA GPU (detected via /proc/driver/nvidia)"
        print_success "$GPU_INFO"
        print_warning "  nvidia-smi not available - may be running in WSL or container"
    
    else
        print_info "No dedicated GPU detected - will use CPU inference"
        print_dim "  Checked: nvidia-smi, lspci, /proc/driver/nvidia"
        
        # If GPU mode was forced, fail
        if [ "$FORCE_GPU" = true ]; then
            print_error "GPU mode was required (--gpu) but no GPU detected"
            exit 1
        fi
    fi
}

# Install Build Dependencies
install_build_deps() {
    print_progress "Installing Build Dependencies" "~2-3 minutes"
    
    local packages=()
    
    # Common build tools
    packages+=(build-essential gcc g++ make cmake git unzip curl wget)
    
    # Go language
    if ! command -v go &> /dev/null; then
        packages+=(golang-go)
    fi
    
    # GPU-specific packages
    if [ "$GPU_TYPE" = "nvidia" ]; then
        print_step "Adding NVIDIA CUDA dependencies..."
        packages+=(nvidia-cuda-toolkit nvidia-cuda-dev)
    elif [ "$GPU_TYPE" = "amd" ]; then
        print_step "Adding AMD ROCm dependencies..."
        if [ "$PKG_MANAGER" = "apt-get" ]; then
            packages+=(rocm-dev rocm-libs)
        fi
    fi
    
    print_step "Installing: ${#packages[@]} packages"
    print_dim "  ${packages[*]}"
    echo ""
    
    if [ "$PKG_MANAGER" = "apt-get" ]; then
        print_info "Updating package lists..."
        echo ""
        
        # Retry apt-get update up to 3 times
        local retry=0
        local max_retries=3
        while [ $retry -lt $max_retries ]; do
            if sudo apt-get update -qq 2>&1 | tail -1; then
                print_success "Package lists updated"
                break
            else
                retry=$((retry + 1))
                if [ $retry -lt $max_retries ]; then
                    print_warning "Update failed, retrying ($retry/$max_retries)..."
                    sleep 2
                else
                    print_warning "Update had issues after $max_retries attempts, continuing..."
                fi
            fi
        done
        
        echo ""
        print_info "Installing packages (this may take a few minutes)..."
        echo ""
        
        # Try to install packages, but don't fail if some are not available
        if sudo apt-get install -y -qq "${packages[@]}" 2>&1 | grep -E "^Setting up|^Processing|^Unpacking" | while read line; do
            echo -ne "${DIM}  ${line}${RESET}\r"
        done; then
            echo ""
            print_success "All packages installed successfully"
        else
            print_warning "Some packages failed to install, trying individually..."
            
            # Try installing packages one by one
            local failed_packages=()
            local installed=0
            for pkg in "${packages[@]}"; do
                if sudo apt-get install -y -qq "$pkg" 2>&1 | grep -v "^Selecting\|^Preparing\|^Unpacking"; then
                    installed=$((installed + 1))
                    print_success "[$installed/${#packages[@]}] $pkg installed"
                else
                    failed_packages+=("$pkg")
                    print_warning "[$installed/${#packages[@]}] $pkg failed"
                fi
            done
            
            if [ ${#failed_packages[@]} -gt 0 ]; then
                print_warning "Failed to install: ${failed_packages[*]}"
                print_info "Installation will continue with available packages"
            fi
        fi
    elif [ "$PKG_MANAGER" = "dnf" ]; then
        sudo dnf install -y -q "${packages[@]}"
    elif [ "$PKG_MANAGER" = "yum" ]; then
        sudo yum install -y -q "${packages[@]}"
    fi
    
    echo ""
    print_success "Build dependencies installed"
}

# Install Go 1.21+
install_go() {
    print_progress "Installing Go Programming Language" "~1-2 minutes"
    
    local REQUIRED_GO_VERSION="1.21"
    local GO_VERSION="1.21.13"
    local GO_TARBALL="go${GO_VERSION}.linux-${ARCH}.tar.gz"
    local GO_URL="https://go.dev/dl/${GO_TARBALL}"
    
    # Check if Go is installed and version is sufficient
    if command -v go &> /dev/null; then
        CURRENT_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        print_info "Found Go version: $CURRENT_VERSION"
        
        # Compare versions
        if [ "$(printf '%s\n' "$REQUIRED_GO_VERSION" "$CURRENT_VERSION" | sort -V | head -n1)" = "$REQUIRED_GO_VERSION" ]; then
            print_success "Go $CURRENT_VERSION is sufficient (>= $REQUIRED_GO_VERSION)"
            return 0
        else
            print_warning "Go $CURRENT_VERSION is too old, upgrading to $GO_VERSION"
            sudo rm -rf /usr/local/go
        fi
    fi
    
    print_step "Downloading Go $GO_VERSION..."
    cd /tmp
    if ! curl -sL "$GO_URL" -o "$GO_TARBALL"; then
        print_error "Failed to download Go"
        return 1
    fi
    
    print_step "Installing Go to /usr/local/go..."
    sudo tar -C /usr/local -xzf "$GO_TARBALL"
    rm "$GO_TARBALL"
    
    # Configure Go PATH for system-wide persistence across reboots
    print_step "Configuring Go PATH for persistent access..."
    
    # 1. Add to /etc/profile.d/ (system-wide for login shells)
    if ! grep -q "/usr/local/go/bin" /etc/profile.d/go.sh 2>/dev/null; then
        sudo tee /etc/profile.d/go.sh > /dev/null << 'EOF'
# Go Programming Language Environment
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
EOF
        sudo chmod +x /etc/profile.d/go.sh
        print_success "Added Go to /etc/profile.d/go.sh (system-wide)"
    fi
    
    # 2. Add to /etc/environment (system-wide for all sessions including non-login)
    if ! grep -q "/usr/local/go/bin" /etc/environment 2>/dev/null; then
        # Read current PATH from /etc/environment or use default
        CURRENT_PATH=$(grep "^PATH=" /etc/environment 2>/dev/null | sed 's/^PATH=//' | sed 's/"//g')
        if [ -z "$CURRENT_PATH" ]; then
            CURRENT_PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
        fi
        
        # Only add if not already present
        if ! echo "$CURRENT_PATH" | grep -q "/usr/local/go/bin"; then
            NEW_PATH="${CURRENT_PATH}:/usr/local/go/bin"
            sudo sed -i '/^PATH=/d' /etc/environment 2>/dev/null || true
            echo "PATH=\"${NEW_PATH}\"" | sudo tee -a /etc/environment > /dev/null
            print_success "Added Go to /etc/environment (persistent across all sessions)"
        fi
    fi
    
    # 3. Add to current user's profile files for immediate availability
    for profile_file in "$HOME/.bashrc" "$HOME/.profile" "$HOME/.zshrc"; do
        if [ -f "$profile_file" ]; then
            if ! grep -q "/usr/local/go/bin" "$profile_file" 2>/dev/null; then
                echo "" >> "$profile_file"
                echo "# Go Programming Language" >> "$profile_file"
                echo 'export PATH=$PATH:/usr/local/go/bin' >> "$profile_file"
                echo 'export GOPATH=$HOME/go' >> "$profile_file"
                echo 'export PATH=$PATH:$GOPATH/bin' >> "$profile_file"
                print_success "Added Go to $(basename $profile_file)"
            fi
        fi
    done
    
    # 4. Export for current session
    export PATH=$PATH:/usr/local/go/bin
    export GOPATH=$HOME/go
    export PATH=$PATH:$GOPATH/bin
    
    # 5. Create GOPATH directory if it doesn't exist
    mkdir -p "$HOME/go/bin" 2>/dev/null || true
    
    # Verify installation
    if command -v go &> /dev/null; then
        INSTALLED_VERSION=$(go version | awk '{print $3}')
        print_success "Go $INSTALLED_VERSION installed successfully"
        print_success "Go PATH configured for persistent access (survives reboots)"
    else
        print_error "Go installation failed"
        return 1
    fi
    
    cd - > /dev/null
}

# Install/Verify NVIDIA Drivers
install_nvidia_drivers() {
    if [ "$GPU_TYPE" != "nvidia" ]; then
        return 0
    fi
    
    print_progress "Configuring NVIDIA GPU Support" "~1 minute"
    
    # Check if nvidia-smi exists and works
    if command -v nvidia-smi &> /dev/null && nvidia-smi &> /dev/null; then
        DRIVER_VERSION=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -n1)
        print_success "NVIDIA drivers already installed: $DRIVER_VERSION"
    else
        print_step "Installing NVIDIA drivers..."
        
        if [ "$PKG_MANAGER" = "apt-get" ]; then
            sudo apt-get update
            sudo apt-get install -y nvidia-driver-535 || {
                print_warning "Failed to install NVIDIA drivers automatically"
                print_info "Please install NVIDIA drivers manually from: https://www.nvidia.com/Download/index.aspx"
            }
        else
            print_warning "Automatic NVIDIA driver installation not supported on this OS"
            print_info "Please install NVIDIA drivers manually from: https://www.nvidia.com/Download/index.aspx"
        fi
    fi
    
    # Load nvidia modules
    print_step "Loading NVIDIA kernel modules..."
    sudo modprobe nvidia 2>/dev/null || print_warning "Failed to load nvidia module"
    sudo modprobe nvidia_uvm 2>/dev/null || print_warning "Failed to load nvidia_uvm module"
}

# Build llama.cpp
build_llama_cpp() {
    print_progress "Building llama.cpp Inference Engine" "~5-10 minutes"
    
    local LLAMA_DIR="$HOME/llama.cpp"
    local ORIGINAL_DIR="$(pwd)"
    
    # Check if llama.cpp is already built
    if [ -f "$LLAMA_DIR/build/libllama.so" ]; then
        print_success "llama.cpp already built at $LLAMA_DIR/build"
        export C_INCLUDE_PATH="$LLAMA_DIR:${C_INCLUDE_PATH:-}"
        export LIBRARY_PATH="$LLAMA_DIR/build:${LIBRARY_PATH:-}"
        export LD_LIBRARY_PATH="$LLAMA_DIR/build:${LD_LIBRARY_PATH:-}"
        return 0
    fi
    
    # Clone or update llama.cpp
    if [ -d "$LLAMA_DIR/.git" ]; then
        print_step "Updating existing llama.cpp repository..."
        cd "$LLAMA_DIR"
        # Clean any existing Makefile or build artifacts that might have old configuration
        make clean 2>/dev/null || true
        git pull -q || print_warning "Could not update llama.cpp, using existing version"
    elif [ -d "$LLAMA_DIR" ]; then
        print_step "Using existing llama.cpp directory (not a git repo)..."
        cd "$LLAMA_DIR"
        # Clean any existing Makefile or build artifacts
        make clean 2>/dev/null || true
    else
        print_step "Downloading llama.cpp repository..."
        
        # Try git clone first
        if timeout 60 git clone --depth 1 https://github.com/ggerganov/llama.cpp.git "$LLAMA_DIR" 2>&1 | tail -n 5; then
            print_success "Git clone successful"
            cd "$LLAMA_DIR"
        else
            # Fallback to downloading zip archive
            print_warning "Git clone timed out, trying zip download..."
            local TMP_DIR=$(mktemp -d)
            cd "$TMP_DIR"
            
            if curl -L --max-time 120 -o llama.cpp.zip "https://github.com/ggerganov/llama.cpp/archive/refs/heads/master.zip" 2>&1 | grep -E "Downloaded|failed"; then
                print_step "Extracting archive..."
                unzip -q llama.cpp.zip || {
                    print_error "Failed to extract llama.cpp"
                    rm -rf "$TMP_DIR"
                    return 1
                }
                mv llama.cpp-master "$LLAMA_DIR"
                cd "$LLAMA_DIR"
                rm -rf "$TMP_DIR"
                print_success "Downloaded and extracted llama.cpp"
            else
                print_error "Failed to download llama.cpp"
                rm -rf "$TMP_DIR"
                return 1
            fi
        fi
    fi
    
    print_step "Configuring build with CMake..."
    # Clean any existing build files to avoid cached configuration issues
    if [ -d "build" ]; then
        print_info "Cleaning previous build configuration..."
        rm -rf build
    fi
    mkdir -p build
    cd build
    
    # Clean environment to avoid MinGW conflicts in WSL
    unset CPATH C_INCLUDE_PATH CPLUS_INCLUDE_PATH
    export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin
    
    # Configure CMake based on GPU type
    CMAKE_ARGS="-DBUILD_SHARED_LIBS=ON -DCMAKE_C_COMPILER=/usr/bin/gcc -DCMAKE_CXX_COMPILER=/usr/bin/g++ -DLLAMA_CURL=OFF"
    
    if [ "$GPU_TYPE" = "nvidia" ]; then
        print_info "Configuring for NVIDIA CUDA acceleration..."
        
        # Check for CUDA toolkit in multiple locations
        NVCC_PATH=""
        CUDA_PATH=""
        
        if command -v nvcc &> /dev/null; then
            NVCC_PATH=$(which nvcc)
            CUDA_PATH=$(dirname $(dirname "$NVCC_PATH"))
        elif [ -f "/usr/local/cuda/bin/nvcc" ]; then
            NVCC_PATH="/usr/local/cuda/bin/nvcc"
            CUDA_PATH="/usr/local/cuda"
        elif [ -f "/usr/lib/cuda/bin/nvcc" ]; then
            NVCC_PATH="/usr/lib/cuda/bin/nvcc"
            CUDA_PATH="/usr/lib/cuda"
        else
            # Search for CUDA installations
            for cuda_dir in /usr/local/cuda-* /opt/cuda /usr/cuda; do
                if [ -f "$cuda_dir/bin/nvcc" ]; then
                    NVCC_PATH="$cuda_dir/bin/nvcc"
                    CUDA_PATH="$cuda_dir"
                    break
                fi
            done
        fi
        
        if [ -n "$NVCC_PATH" ] && [ -f "$NVCC_PATH" ]; then
            CUDA_VERSION=$("$NVCC_PATH" --version 2>/dev/null | grep "release" | awk '{print $5}' | tr -d ',')
            print_info "  Found CUDA toolkit: $CUDA_VERSION at $CUDA_PATH"
            CMAKE_ARGS="$CMAKE_ARGS -DGGML_CUDA=ON -DCMAKE_CUDA_COMPILER=$NVCC_PATH"
            
            # Add CUDA to PATH for the build
            export PATH="$CUDA_PATH/bin:$PATH"
            export LD_LIBRARY_PATH="$CUDA_PATH/lib64:${LD_LIBRARY_PATH:-}"
        else
            print_warning "  CUDA toolkit (nvcc) not found - building CPU version"
            print_info "  Your NVIDIA GPU is detected but CUDA toolkit is not installed"
            print_info "  For GPU acceleration, install CUDA: https://developer.nvidia.com/cuda-downloads"
            print_dim "  Checked: PATH, /usr/local/cuda*, /usr/lib/cuda, /opt/cuda"
            GPU_TYPE="none"  # Fallback to CPU
        fi
    elif [ "$GPU_TYPE" = "amd" ]; then
        print_info "Configuring for AMD ROCm acceleration..."
        CMAKE_ARGS="$CMAKE_ARGS -DGGML_HIPBLAS=ON"
    else
        if [ "$FORCE_CPU_ONLY" = true ]; then
            print_info "Configuring for CPU-only inference (forced by --cpu-only flag)..."
        else
            print_info "Configuring for CPU-only inference (no GPU detected)..."
        fi
    fi
    
    cmake .. $CMAKE_ARGS 2>&1 | grep -E "Build files|llama.cpp|CUDA|HIP|OpenBLAS|Accelerate|compiler|Configuring done|Generating done" || true
    
    if [ ! -f "Makefile" ] && [ ! -f "build.ninja" ]; then
        print_error "CMake configuration failed - no build files generated"
        cat CMakeCache.txt 2>/dev/null | grep -i error || true
        return 1
    fi
    
    print_step "Building llama.cpp (this may take 5-10 minutes)..."
    print_info "Building with $(nproc) CPU cores..."
    
    # Build llama-server specifically with timeout and progress monitoring
    BUILD_LOG="/tmp/llama_build_$$.log"
    
    if cmake --build . --config Release --target llama-server -j$(nproc) > "$BUILD_LOG" 2>&1; then
        if [ -f "bin/llama-server" ]; then
            print_success "llama-server built successfully"
            
            # Verify the binary works
            if ldd bin/llama-server > /dev/null 2>&1; then
                print_step "Installing llama-server and shared libraries..."
                
                # Install the main binary
                sudo install -o0 -g0 -m755 bin/llama-server /usr/local/bin/llama-server
                
                # Install shared libraries to /usr/local/lib
                print_step "Installing shared libraries..."
                for lib in bin/*.so*; do
                    if [ -f "$lib" ]; then
                        sudo install -o0 -g0 -m755 "$lib" /usr/local/lib/
                        print_info "  Installed $(basename $lib)"
                    fi
                done
                
                # Update library cache
                print_step "Updating library cache..."
                sudo ldconfig
                
                # Verify installation
                print_step "Verifying installation..."
                if /usr/local/bin/llama-server --version > /dev/null 2>&1; then
                    print_success "llama-server installed and verified"
                else
                    print_warning "llama-server installed but may have runtime issues"
                    print_info "Checking dependencies:"
                    ldd /usr/local/bin/llama-server | grep -E "not found|=>" | head -10
                fi
            else
                print_error "llama-server has missing dependencies:"
                ldd bin/llama-server | grep "not found" || true
                return 1
            fi
        else
            print_error "llama-server binary not found after build"
            print_info "Checking build directory..."
            find . -name "llama-server" -o -name "llama-server.exe" 2>/dev/null || true
            tail -20 "$BUILD_LOG"
            return 1
        fi
    else
        BUILD_EXIT=$?
        print_error "Build failed with exit code $BUILD_EXIT"
        print_info "Last 30 lines of build log:"
        tail -30 "$BUILD_LOG"
        
        # Check for common errors
        if grep -q "No CMAKE_CUDA_COMPILER" "$BUILD_LOG"; then
            print_error "CUDA compiler not found - check CUDA installation"
        elif grep -q "nvcc.*not found" "$BUILD_LOG"; then
            print_error "nvcc not in PATH - add CUDA bin directory to PATH"
        elif grep -q "undefined reference" "$BUILD_LOG"; then
            print_error "Linker error - missing libraries or incompatible CUDA version"
        fi
        
        print_warning "Will build OffGrid LLM in mock mode"
        cd "$ORIGINAL_DIR"
        return 1
    fi
    
    # Install libraries
    print_step "Installing llama.cpp libraries..."
    if sudo cmake --install . 2>&1 | grep -E "Install|Up-to-date"; then
        print_success "Libraries installed system-wide"
    else
        print_warning "Failed to install llama.cpp system-wide, will use local build"
    fi
    
    # Verify shared libraries exist
    if [ -f "$LLAMA_DIR/build/ggml/src/libggml.so" ]; then
        print_step "Copying shared libraries to system path..."
        sudo cp -v "$LLAMA_DIR/build/ggml/src/libggml*.so" /usr/local/lib/ 2>/dev/null || true
        sudo ldconfig
    fi
    
    print_success "llama.cpp built successfully at $LLAMA_DIR/build"
    
    # Export paths for Go build
    export C_INCLUDE_PATH="$LLAMA_DIR:${C_INCLUDE_PATH:-}"
    export LIBRARY_PATH="$LLAMA_DIR/build:${LIBRARY_PATH:-}"
    export LD_LIBRARY_PATH="$LLAMA_DIR/build:/usr/local/lib:${LD_LIBRARY_PATH:-}"
    
    # Clean up build log
    rm -f "$BUILD_LOG"
    
    # Return to original directory
    cd "$ORIGINAL_DIR"
}

# Build OffGrid LLM
build_offgrid() {
    print_progress "Building OffGrid LLM" "~2-3 minutes"
    
    local BUILD_DIR=$(pwd)
    local GO_CMD="go"
    
    # Use Go 1.21+ if available - check multiple possible locations
    if [ -f "/usr/local/go/bin/go" ]; then
        GO_CMD="/usr/local/go/bin/go"
        print_info "Using installed Go at /usr/local/go/bin/go"
    elif [ -f "$HOME/go1.21.5/bin/go" ]; then
        GO_CMD="$HOME/go1.21.5/bin/go"
        print_info "Using installed Go at $HOME/go1.21.5/bin/go"
    elif [ -f "$HOME/go1.21.13/bin/go" ]; then
        GO_CMD="$HOME/go1.21.13/bin/go"
        print_info "Using installed Go at $HOME/go1.21.13/bin/go"
    else
        # Try to find Go 1.21+ in PATH
        if command -v go &> /dev/null; then
            GO_VERSION_CHECK=$(go version | awk '{print $3}' | sed 's/go//' | cut -d. -f1-2)
            if [ "$(printf '%s\n' "1.21" "$GO_VERSION_CHECK" | sort -V | head -n1)" = "1.21" ]; then
                GO_CMD="go"
                print_info "Using system Go (version check passed)"
            fi
        fi
    fi
    
    # Verify Go is installed
    if ! command -v "$GO_CMD" &> /dev/null; then
        print_error "Go is not installed"
        exit 1
    fi
    
    GO_VERSION=$($GO_CMD version | awk '{print $3}')
    print_info "Using Go: $GO_VERSION"
    
    # Check Go version meets minimum requirement
    GO_VERSION_NUM=$(echo $GO_VERSION | sed 's/go//' | cut -d. -f1-2)
    MIN_GO_VERSION="1.21"
    if [ "$(printf '%s\n' "$MIN_GO_VERSION" "$GO_VERSION_NUM" | sort -V | head -n1)" != "$MIN_GO_VERSION" ]; then
        print_warning "Go version $GO_VERSION_NUM is older than required $MIN_GO_VERSION"
        print_warning "Attempting to use system Go anyway..."
    fi
    
    print_step "Downloading Go dependencies..."
    GOTOOLCHAIN=local $GO_CMD mod download 2>&1 | grep -E "go: downloading|error" || true
    
    # TODO: Real llama.cpp inference integration
    # Currently disabled due to go-llama.cpp version incompatibility with latest llama.cpp
    # The go-skynet/go-llama.cpp binding from March 2024 expects older llama.cpp with grammar-parser.h
    # We need to either:
    # 1. Pin to older llama.cpp version matching the binding
    # 2. Switch to a more modern Go binding (llama-cpp-python wrapper, or direct CGO)
    # 3. Fork and update go-llama.cpp to work with latest llama.cpp
    #
    # For now, building in mock mode to ensure reliable installation
    LLAMA_AVAILABLE=false
    
    print_step "Building in mock mode..."
    print_info "Note: Real inference coming soon - version compatibility work in progress"
    
    if GOTOOLCHAIN=local $GO_CMD build -o offgrid ./cmd/offgrid 2>&1 | tee /tmp/go_build.log | tail -n 10; then
        if [ -f offgrid ] && [ -x offgrid ]; then
            print_success "Built successfully"
            print_warning "Currently using mock mode - real inference integration coming in next update"
        else
            print_error "Failed to build binary"
            exit 1
        fi
    else
        print_error "Build failed - check /tmp/go_build.log for details"
        exit 1
    fi
    
    # Verify binary was created
    if [ ! -f offgrid ]; then
        print_error "Build failed - binary not created"
        exit 1
    fi
    
    print_success "OffGrid LLM binary built successfully"
}

# Install Binary
install_binary() {
    print_progress "Installing OffGrid LLM Binary" "~10 seconds"
    
    local INSTALL_DIR="/usr/local/bin"
    
    print_step "Installing binary to $INSTALL_DIR/offgrid..."
    sudo install -o0 -g0 -m755 offgrid "$INSTALL_DIR/offgrid"
    
    print_success "Binary installed to $INSTALL_DIR/offgrid"
    
    # Verify installation
    if command -v offgrid &> /dev/null; then
        INSTALLED_VERSION=$(offgrid --version 2>/dev/null || echo "unknown")
        print_success "Installation verified: $INSTALLED_VERSION"
    fi
}

# Create User and Groups
setup_user() {
    print_progress "Setting Up Service User" "~10 seconds"
    
    # Create offgrid user if it doesn't exist
    if ! id offgrid &> /dev/null; then
        print_step "Creating offgrid system user..."
        sudo useradd -r -s /bin/false -U -m -d /var/lib/offgrid offgrid
        print_success "User 'offgrid' created"
    else
        print_info "User 'offgrid' already exists"
    fi
    
    # Ensure /var/lib/offgrid is readable by all users (needed for model access)
    sudo chmod 755 /var/lib/offgrid
    
    # Add to video and render groups for GPU access
    if [ "$GPU_TYPE" != "none" ]; then
        print_step "Adding offgrid user to GPU groups..."
        sudo usermod -aG video offgrid 2>/dev/null || true
        sudo usermod -aG render offgrid 2>/dev/null || true
    fi
}

# Create Systemd Service
# Setup llama-server systemd service
setup_llama_server_service() {
    print_progress "Setting Up llama-server Service" "~30 seconds"
    
    # Verify llama-server binary exists
    if [ ! -f "/usr/local/bin/llama-server" ]; then
        print_error "llama-server binary not found at /usr/local/bin/llama-server"
        print_warning "Skipping llama-server service setup"
        print_info "OffGrid LLM will run in mock mode"
        return 1
    fi
    
    # Verify llama-server can run
    if ! /usr/local/bin/llama-server --version &> /dev/null; then
        print_warning "llama-server binary exists but may have issues"
        print_info "Checking dependencies..."
        ldd /usr/local/bin/llama-server | grep "not found" || true
    fi
    
    local SERVICE_FILE="/etc/systemd/system/llama-server.service"
    
    # Generate a random high port for internal use only (49152-65535 dynamic/private range)
    # Using a specific seed based on hostname for consistency across restarts
    local RANDOM_PORT=$((49152 + $(hostname | md5sum | cut -c1-4 | xargs -I{} printf "%d" 0x{}) % 16384))
    print_info "Using internal port: $RANDOM_PORT (localhost-only, not externally accessible)"
    
    # Find a model to use (prefer tinyllama)
    local MODEL_PATH=""
    local MODELS_DIR="/var/lib/offgrid/models"
    
    # Create models directory if it doesn't exist
    sudo mkdir -p "$MODELS_DIR"
    sudo chown offgrid:offgrid "$MODELS_DIR"
    
    # Make models directory readable by all, writable by group
    # Use setgid so new files inherit the offgrid group
    sudo chmod 2775 "$MODELS_DIR"  # 2 = setgid bit
    
    # Create marker file to indicate system directory is ready
    sudo touch "$MODELS_DIR/.offgrid"
    sudo chown offgrid:offgrid "$MODELS_DIR/.offgrid"
    
    # Add current user to offgrid group for write access
    if ! groups | grep -q offgrid; then
        print_info "Adding current user to 'offgrid' group for model management..."
        sudo usermod -a -G offgrid "$USER"
        print_info "Group membership will be active in new terminal sessions"
    fi
    
    if [ -f "$MODELS_DIR/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf" ]; then
        MODEL_PATH="$MODELS_DIR/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf"
    elif [ -f "$MODELS_DIR/tinyllama-1.1b-chat.Q4_K_M.gguf" ]; then
        MODEL_PATH="$MODELS_DIR/tinyllama-1.1b-chat.Q4_K_M.gguf"
    elif [ -f "$MODELS_DIR/tinyllama.gguf" ]; then
        MODEL_PATH="$MODELS_DIR/tinyllama.gguf"
    else
        # Use the first .gguf file found
        MODEL_PATH=$(sudo find "$MODELS_DIR" -name "*.gguf" -type f 2>/dev/null | head -n 1)
    fi
    
    if [ -z "$MODEL_PATH" ]; then
        print_warning "No model found in $MODELS_DIR"
        print_info "You can download a model with:"
        print_dim "  wget https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf"
        print_dim "  sudo mv tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf $MODELS_DIR/"
        print_dim "  sudo chown offgrid:offgrid $MODELS_DIR/*.gguf"
        print_dim "  sudo systemctl restart llama-server"
        MODEL_PATH="$MODELS_DIR/model.gguf"
    else
        print_info "Using model: $(basename $MODEL_PATH)"
        # Fix permissions on model files
        sudo chmod 644 "$MODELS_DIR"/*.gguf 2>/dev/null || true
        sudo chown offgrid:offgrid "$MODELS_DIR"/*.gguf 2>/dev/null || true
    fi
    
    print_step "Creating llama-server service file at $SERVICE_FILE..."
    
    # Determine GPU layers based on GPU type
    local GPU_LAYERS="0"
    local EXTRA_ARGS=""
    if [ "$GPU_TYPE" = "nvidia" ]; then
        GPU_LAYERS="99"  # Offload all layers to GPU
        EXTRA_ARGS="--n-gpu-layers 99"
        print_info "Configured for NVIDIA GPU acceleration (99 layers offloaded)"
    elif [ "$GPU_TYPE" = "amd" ]; then
        GPU_LAYERS="99"
        EXTRA_ARGS="--n-gpu-layers 99"
        print_info "Configured for AMD GPU acceleration (99 layers offloaded)"
    else
        print_info "Configured for CPU-only inference"
    fi
    
    # Create startup script that reads active model from config
    print_step "Creating llama-server startup script..."
    local START_SCRIPT="/usr/local/bin/llama-server-start.sh"
    sudo tee "$START_SCRIPT" > /dev/null <<'START_SCRIPT_EOF'
#!/bin/bash
# llama-server startup script with dynamic model loading

ACTIVE_MODEL_FILE="/etc/offgrid/active-model"
MODELS_DIR="/var/lib/offgrid/models"
DEFAULT_MODEL=""
LLAMA_PORT_FILE="/etc/offgrid/llama-port"

# Find the first available model if no active model is set
if [ -f "$ACTIVE_MODEL_FILE" ]; then
    MODEL_PATH=$(cat "$ACTIVE_MODEL_FILE")
    if [ ! -f "$MODEL_PATH" ]; then
        echo "Warning: Configured model not found: $MODEL_PATH" >&2
        MODEL_PATH=""
    fi
fi

# If no active model or it doesn't exist, find the first .gguf file
if [ -z "$MODEL_PATH" ]; then
    MODEL_PATH=$(find "$MODELS_DIR" -maxdepth 1 -name "*.gguf" -type f | head -n 1)
    if [ -z "$MODEL_PATH" ]; then
        echo "Error: No .gguf models found in $MODELS_DIR" >&2
        exit 1
    fi
    echo "Using first available model: $MODEL_PATH" >&2
    # Save it as the active model for next time
    echo "$MODEL_PATH" > "$ACTIVE_MODEL_FILE"
fi

# Read port from config
if [ -f "$LLAMA_PORT_FILE" ]; then
    LLAMA_PORT=$(cat "$LLAMA_PORT_FILE")
else
    LLAMA_PORT=49233
fi

echo "Starting llama-server with model: $(basename $MODEL_PATH)" >&2
echo "Port: $LLAMA_PORT" >&2

# Execute llama-server with the selected model
exec /usr/local/bin/llama-server -m "$MODEL_PATH" --port "$LLAMA_PORT" --host 127.0.0.1 -c 2048 EXTRA_ARGS_PLACEHOLDER
START_SCRIPT_EOF
    
    # Replace EXTRA_ARGS_PLACEHOLDER with actual GPU args
    sudo sed -i "s/EXTRA_ARGS_PLACEHOLDER/$EXTRA_ARGS/" "$START_SCRIPT"
    sudo chmod +x "$START_SCRIPT"
    
    print_success "Created startup script at $START_SCRIPT"
    
    # Create initial active model config
    print_step "Setting up active model configuration..."
    sudo mkdir -p /etc/offgrid
    echo "$MODEL_PATH" | sudo tee /etc/offgrid/active-model > /dev/null
    sudo chmod 644 /etc/offgrid/active-model
    print_info "Active model: $(basename $MODEL_PATH)"
    
    sudo tee "$SERVICE_FILE" > /dev/null <<SERVICE_EOF
[Unit]
Description=llama.cpp Inference Server (Internal)
Documentation=https://github.com/ggerganov/llama.cpp
After=network-online.target
Wants=network-online.target
Before=offgrid-llm.service

[Service]
Type=simple
User=offgrid
Group=offgrid
WorkingDirectory=/var/lib/offgrid
ExecStart=/usr/local/bin/llama-server-start.sh
Restart=always
RestartSec=3

# Environment variables
Environment="PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
Environment="LD_LIBRARY_PATH=/usr/local/lib"
Environment="LLAMA_SERVER_INTERNAL=1"

# Security hardening - localhost-only binding for internal IPC
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/offgrid
ReadWritePaths=/etc/offgrid
# Network isolation - only localhost
IPAddressDeny=any
IPAddressAllow=localhost

[Install]
WantedBy=multi-user.target
SERVICE_EOF
    
    # Store the port in a config file for OffGrid to read
    print_step "Saving internal port configuration..."
    sudo mkdir -p /etc/offgrid
    echo "$RANDOM_PORT" | sudo tee /etc/offgrid/llama-port > /dev/null
    sudo chmod 644 /etc/offgrid/llama-port
    
    print_success "llama-server service created on internal port $RANDOM_PORT"
    
    print_step "Reloading systemd daemon..."
    sudo systemctl daemon-reload
    
    print_step "Enabling llama-server service..."
    sudo systemctl enable llama-server.service
    
    print_success "llama-server service enabled (will start on boot)"
    
    # Start the service now
    print_step "Starting llama-server service..."
    sudo systemctl start llama-server.service
    
    sleep 2
    
    if sudo systemctl is-active --quiet llama-server.service; then
        print_success "llama-server is running on internal port $RANDOM_PORT!"
        
        # Wait for server to be ready
        print_step "Waiting for llama-server to be ready..."
        for i in {1..10}; do
            if curl -s http://127.0.0.1:${RANDOM_PORT}/health | grep -q "ok"; then
                print_success "llama-server health check passed"
                print_info "Internal endpoint: http://127.0.0.1:${RANDOM_PORT} (not accessible externally)"
                return 0
            fi
            sleep 1
        done
        print_warning "llama-server is running but health check timed out"
    else
        print_error "Failed to start llama-server service"
        print_info "Check logs with: sudo journalctl -u llama-server.service -n 50"
    fi
}

setup_systemd_service() {
    print_progress "Setting Up OffGrid Systemd Service" "~20 seconds"
    
    local SERVICE_FILE="/etc/systemd/system/offgrid-llm.service"
    
    # Read the internal llama-server port
    local LLAMA_PORT=8081
    if [ -f "/etc/offgrid/llama-port" ]; then
        LLAMA_PORT=$(cat /etc/offgrid/llama-port)
        print_info "Configuring OffGrid to connect to llama-server on port $LLAMA_PORT"
    fi
    
    print_step "Creating service file at $SERVICE_FILE..."
    
    sudo tee "$SERVICE_FILE" > /dev/null <<SERVICE_EOF
[Unit]
Description=OffGrid LLM - Offline AI Inference Engine
Documentation=https://github.com/yourusername/offgrid-llm
After=network-online.target llama-server.service
Wants=network-online.target
Requires=llama-server.service

[Service]
Type=simple
User=offgrid
Group=offgrid
WorkingDirectory=/var/lib/offgrid
ExecStart=/usr/local/bin/offgrid serve
Restart=always
RestartSec=3

# Environment variables
Environment="PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
Environment="OFFGRID_PORT=11611"
Environment="OFFGRID_MODELS_DIR=/var/lib/offgrid/models"
Environment="LLAMA_SERVER_URL=http://127.0.0.1:${LLAMA_PORT}"

# Security settings - only expose port 11611 externally
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/offgrid

[Install]
WantedBy=multi-user.target
SERVICE_EOF
    
    print_success "Systemd service created"
    
    print_step "Reloading systemd daemon..."
    sudo systemctl daemon-reload
    
    print_step "Enabling offgrid-llm service..."
    sudo systemctl enable offgrid-llm.service
    
    print_success "Service enabled (will start on boot)"
}

# Setup Configuration
setup_config() {
    print_progress "Setting Up Configuration" "~10 seconds"
    
    local CONFIG_DIR="/var/lib/offgrid"
    local MODELS_DIR="$CONFIG_DIR/models"
    local WEB_DIR="$CONFIG_DIR/web/ui"
    
    print_step "Creating directories..."
    sudo mkdir -p "$MODELS_DIR"
    sudo mkdir -p "$WEB_DIR"
    
    print_step "Copying web UI..."
    if [ -f "web/ui/index.html" ]; then
        sudo cp -r web/ui/* "$WEB_DIR/"
        print_success "Web UI files copied"
    else
        print_warning "No UI files found"
    fi
    
    sudo chown -R offgrid:offgrid "$CONFIG_DIR"
    
    print_success "Configuration directories created"
}

# Install Shell Completions
install_completions() {
    print_progress "Installing Shell Completions" "~10 seconds"
    
    # Detect user's shell
    USER_SHELL=$(basename "$SHELL")
    
    case "$USER_SHELL" in
        bash)
            print_step "Installing Bash completions..."
            
            # Try user installation (safer, doesn't require sudo)
            mkdir -p "$HOME/.bash_completion.d"
            /usr/local/bin/offgrid completions bash > "$HOME/.bash_completion.d/offgrid" 2>/dev/null || {
                print_warning "Could not generate completions (offgrid binary might not be ready yet)"
                return
            }
            
            # Add sourcing to .bashrc if not already there
            if ! grep -q "\.bash_completion\.d/offgrid" "$HOME/.bashrc" 2>/dev/null; then
                echo "" >> "$HOME/.bashrc"
                echo "# OffGrid completions" >> "$HOME/.bashrc"
                echo "[ -f ~/.bash_completion.d/offgrid ] && source ~/.bash_completion.d/offgrid" >> "$HOME/.bashrc"
            fi
            
            print_success "Bash completions installed to ~/.bash_completion.d/offgrid"
            print_info "Restart your shell or run: source ~/.bashrc"
            ;;
            
        zsh)
            print_step "Installing Zsh completions..."
            
            ZSH_COMPLETIONS="${XDG_DATA_HOME:-$HOME/.local/share}/zsh/site-functions"
            mkdir -p "$ZSH_COMPLETIONS"
            
            /usr/local/bin/offgrid completions zsh > "$ZSH_COMPLETIONS/_offgrid" 2>/dev/null || {
                print_warning "Could not generate completions"
                return
            }
            
            print_success "Zsh completions installed to $ZSH_COMPLETIONS/_offgrid"
            print_info "Restart your shell or run: exec zsh"
            ;;
            
        fish)
            print_step "Installing Fish completions..."
            
            mkdir -p "$HOME/.config/fish/completions"
            /usr/local/bin/offgrid completions fish > "$HOME/.config/fish/completions/offgrid.fish" 2>/dev/null || {
                print_warning "Could not generate completions"
                return
            }
            
            print_success "Fish completions installed (active immediately)"
            ;;
            
        *)
            print_info "Shell: $USER_SHELL (manual completion setup required)"
            print_info "Run: offgrid completions <bash|zsh|fish> to generate"
            ;;
    esac
}

# Start Service
start_service() {
    print_progress "Starting OffGrid LLM Service" "~15 seconds"
    
    print_step "Starting offgrid-llm service..."
    sudo systemctl start offgrid-llm.service
    
    sleep 2
    
    if sudo systemctl is-active --quiet offgrid-llm.service; then
        print_success "Service started successfully"
        
        print_info "Service status:"
        sudo systemctl status offgrid-llm.service --no-pager -l | head -n 10
    else
        print_error "Service failed to start"
        print_info "Check logs with: sudo journalctl -u offgrid-llm.service -n 50"
        exit 1
    fi
}

# Display Summary
display_summary() {
    local total_time=$(get_elapsed_time)
    
    echo ""
    echo ""
    print_divider
    echo -e "${BRAND_SUCCESS}${BOLD}"
    cat << "EOF"
    ╔═══════════════════════════════════════════════════════════════╗
    ║                                                               ║
    ║              ✓  INSTALLATION COMPLETE  ✓                      ║
    ║                                                               ║
    ╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${RESET}"
    print_divider
    
    echo ""
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SYSTEM INFORMATION${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Architecture     ${BRAND_SECONDARY}$ARCH${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Operating System ${BRAND_SECONDARY}$NAME $VERSION${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  GPU Type         ${BRAND_SECONDARY}${GPU_TYPE}${RESET}"
    if [ "$GPU_TYPE" != "none" ]; then
        echo -e "${BRAND_PRIMARY}│${RESET}  GPU Info         ${BRAND_SECONDARY}${GPU_INFO}${RESET}"
    fi
    echo -e "${BRAND_PRIMARY}│${RESET}  Inference Mode   ${BRAND_SUCCESS}REAL LLM${RESET} ${DIM}(via llama.cpp)${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Install Time     ${BRAND_ACCENT}${total_time}${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    
    echo ""
    
    # Get internal port
    local INTERNAL_PORT="(random)"
    if [ -f "/etc/offgrid/llama-port" ]; then
        INTERNAL_PORT=$(cat /etc/offgrid/llama-port)
    fi
    
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SERVICE ENDPOINTS${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  llama-server     ${BRAND_MUTED}http://127.0.0.1:${INTERNAL_PORT}${RESET} ${DIM}(internal only)${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  OffGrid API      ${BRAND_SECONDARY}http://localhost:11611${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  Web UI           ${BRAND_SECONDARY}http://localhost:11611/ui${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    
    echo ""
    
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SECURITY${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} llama-server bound to 127.0.0.1 only (internal IPC)"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Random high port ${INTERNAL_PORT} not exposed externally"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Only OffGrid port 11611 is publicly accessible"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Same architecture as Ollama for security and isolation"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    
    echo ""
    
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}QUICK START COMMANDS${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Start interactive chat${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid run <model>${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# List available models${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid list${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Download a model${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid download tinyllama${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Start with session auto-save${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}offgrid run <model> --save my-session${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    
    echo ""
    
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}SERVICE MANAGEMENT${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Check service status${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo systemctl status offgrid-llm${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo systemctl status llama-server${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# View logs${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo journalctl -u offgrid-llm -f${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo journalctl -u llama-server -f${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}# Restart services${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}sudo systemctl restart offgrid-llm${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    
    echo ""
    
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}HEALTH CHECK${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SECONDARY}curl http://localhost:11611/health${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    
    echo ""
    
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}NEXT STEPS${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_ACCENT}1.${RESET} Visit ${BRAND_SECONDARY}http://localhost:11611/ui${RESET} in your browser"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_ACCENT}2.${RESET} Test the health endpoint"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_ACCENT}3.${RESET} Try a chat completion"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    
    echo ""
    print_divider
    echo ""
    echo -e "${BRAND_SUCCESS}${BOLD}  🚀 OffGrid LLM is ready to use!${RESET}"
    echo -e "${DIM}     Installed in ${total_time} • Real LLM inference enabled${RESET}"
    echo ""
    print_divider
    echo ""
}

# Main Installation Flow
main() {
    # Parse command line arguments first
    parse_args "$@"
    
    # Show banner
    clear
    print_banner
    
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}OffGrid LLM Installation${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  This installer will:"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Check system dependencies"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Detect GPU hardware"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Install Go 1.21+"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Build llama.cpp inference engine"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Build and install OffGrid LLM"
    echo -e "${BRAND_PRIMARY}│${RESET}    • Configure systemd services"
    echo -e "${BRAND_PRIMARY}│${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${DIM}Estimated time: 10-15 minutes${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    echo ""
    
    # Check if running as root (not recommended but check dependencies will need sudo)
    if [ "$EUID" -eq 0 ]; then
        print_warning "Running as root - this is not recommended"
        print_info "Please run as a regular user with sudo access"
    fi
    
    # Check if another instance is running using lock file
    if [ -f "$LOCK_FILE" ]; then
        LOCK_PID=$(cat "$LOCK_FILE" 2>/dev/null || echo "")
        if [ -n "$LOCK_PID" ] && kill -0 "$LOCK_PID" 2>/dev/null; then
            print_error "Another installation appears to be running"
            print_info "PID: $LOCK_PID"
            print_info "If this is incorrect, remove $LOCK_FILE and try again"
            exit 1
        else
            print_info "Removing stale lock file"
            rm -f "$LOCK_FILE"
        fi
    fi
    
    # Create lock file with current PID
    echo $$ > "$LOCK_FILE"
    
    echo ""
    print_info "Starting installation..."
    sleep 1
    
    # Pre-flight checks
    check_dependencies
    detect_architecture
    detect_os
    detect_gpu
    
    # Show pre-flight summary
    echo ""
    echo -e "${BRAND_PRIMARY}╭─────────────────────────────────────────────────────────────────╮${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET} ${BOLD}PRE-FLIGHT CHECK COMPLETE${RESET}"
    echo -e "${BRAND_PRIMARY}├─────────────────────────────────────────────────────────────────┤${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} System dependencies verified"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Architecture: ${BRAND_SECONDARY}${ARCH}${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} Operating System: ${BRAND_SECONDARY}${NAME}${RESET}"
    echo -e "${BRAND_PRIMARY}│${RESET}  ${BRAND_SUCCESS}✓${RESET} GPU: ${BRAND_SECONDARY}${GPU_TYPE}${RESET}"
    echo -e "${BRAND_PRIMARY}╰─────────────────────────────────────────────────────────────────╯${RESET}"
    echo ""
    
    print_info "Proceeding with installation..."
    sleep 2
    
    # Build and install
    install_build_deps
    install_go
    install_nvidia_drivers
    build_llama_cpp
    build_offgrid
    install_binary
    
    # System setup
    setup_user
    setup_config
    setup_llama_server_service
    setup_systemd_service
    start_service
    
    # Install shell completions
    install_completions
    
    # Summary
    display_summary
}

# Run main installation
main "$@"
