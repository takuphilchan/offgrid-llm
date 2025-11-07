#!/bin/bash
# OffGrid LLM Installation Script
# Comprehensive installation with GPU support, llama.cpp compilation, and systemd integration

set -eu

# Color and Formatting Setup
BOLD='\033[1m'
CYAN='\033[36m'
GREEN='\033[32m'
RED='\033[31m'
YELLOW='\033[33m'
RESET='\033[0m'

# Print Functions
print_header() {
    echo -e "\n${BOLD}${CYAN}┌────────────────────────────────────────────────────────────────┐${RESET}"
    echo -e "${BOLD}${CYAN}│  $1${RESET}"
    echo -e "${BOLD}${CYAN}└────────────────────────────────────────────────────────────────┘${RESET}"
}

print_success() { echo -e "${GREEN}✓${RESET} $1"; }
print_error() { echo -e "${RED}✗${RESET} $1" >&2; }
print_info() { echo -e "${CYAN}ℹ${RESET} $1"; }
print_warning() { echo -e "${YELLOW}⚠${RESET} $1"; }
print_step() { echo -e "${BOLD}➜${RESET} $1"; }

# Error Handler
handle_error() {
    print_error "Installation failed at: $1"
    print_info "Check the error message above for details"
    exit 1
}

trap 'handle_error "line $LINENO"' ERR

# Dependency Checks
check_dependencies() {
    print_header "Checking Dependencies"
    
    local missing=()
    for cmd in curl awk grep sed tee xargs git snap; do
        if ! command -v $cmd &> /dev/null; then
            missing+=($cmd)
        else
            print_success "$cmd is available"
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        print_error "Missing required dependencies: ${missing[*]}"
        print_info "Installing missing dependencies..."
        
        if command -v apt-get &> /dev/null; then
            sudo apt-get update
            sudo apt-get install -y "${missing[@]}" snapd
        elif command -v dnf &> /dev/null; then
            sudo dnf install -y "${missing[@]}" snapd
        elif command -v yum &> /dev/null; then
            sudo yum install -y "${missing[@]}" snapd
        else
            print_error "Unable to install dependencies automatically"
            print_info "Please install: ${missing[*]} snapd"
            exit 1
        fi
    fi
}

# Architecture Detection
detect_architecture() {
    print_header "Detecting System Architecture"
    
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
    print_header "Detecting Operating System"
    
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
    print_header "Detecting GPU Hardware"
    
    GPU_TYPE="none"
    
    # Check for NVIDIA GPU (vendor ID: 10de)
    if lspci 2>/dev/null | grep -i 'vga.*nvidia\|3d.*nvidia\|display.*nvidia' &> /dev/null || \
       lspci -n 2>/dev/null | grep -E '(0300|0302):.*10de:' &> /dev/null; then
        GPU_TYPE="nvidia"
        GPU_INFO=$(lspci | grep -i 'vga.*nvidia\|3d.*nvidia\|display.*nvidia' | head -n1)
        print_success "NVIDIA GPU detected: $GPU_INFO"
    
    # Check for AMD GPU (vendor ID: 1002)
    elif lspci 2>/dev/null | grep -i 'vga.*amd\|vga.*ati\|3d.*amd' &> /dev/null || \
         lspci -n 2>/dev/null | grep -E '(0300|0302):.*1002:' &> /dev/null; then
        GPU_TYPE="amd"
        GPU_INFO=$(lspci | grep -i 'vga.*amd\|vga.*ati\|3d.*amd' | head -n1)
        print_success "AMD GPU detected: $GPU_INFO"
    
    else
        print_info "No dedicated GPU detected - will use CPU inference"
    fi
}

# Install Build Dependencies
install_build_deps() {
    print_header "Installing Build Dependencies"
    
    local packages=()
    
    # Common build tools
    packages+=(build-essential gcc g++ make cmake git)
    
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
    
    print_step "Installing: ${packages[*]}"
    
    if [ "$PKG_MANAGER" = "apt-get" ]; then
        print_info "Updating package lists..."
        sudo apt-get update -qq || print_warning "apt-get update had issues, continuing..."
        
        print_info "Installing packages (this may take a few minutes)..."
        sudo apt-get install -y -qq "${packages[@]}" 2>&1 | grep -v "^Selecting\|^Preparing\|^Unpacking" || {
            print_warning "Some packages failed to install, continuing..."
        }
    elif [ "$PKG_MANAGER" = "dnf" ]; then
        sudo dnf install -y -q "${packages[@]}"
    elif [ "$PKG_MANAGER" = "yum" ]; then
        sudo yum install -y -q "${packages[@]}"
    fi
    
    print_success "Build dependencies installed"
}

# Install/Verify NVIDIA Drivers
install_nvidia_drivers() {
    if [ "$GPU_TYPE" != "nvidia" ]; then
        return 0
    fi
    
    print_header "Configuring NVIDIA GPU Support"
    
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
    print_header "Building llama.cpp Inference Engine"
    
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
        git pull -q || print_warning "Could not update llama.cpp, using existing version"
    else
        print_step "Cloning llama.cpp repository..."
        git clone --depth 1 -q https://github.com/ggerganov/llama.cpp.git "$LLAMA_DIR" 2>&1 | tail -n 2
        cd "$LLAMA_DIR"
    fi
    
    print_step "Configuring build with CMake..."
    mkdir -p build
    cd build
    
    # Clean environment to avoid MinGW conflicts in WSL
    unset CPATH C_INCLUDE_PATH CPLUS_INCLUDE_PATH
    export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin
    
    # Configure CMake based on GPU type
    CMAKE_ARGS="-DBUILD_SHARED_LIBS=ON -DCMAKE_C_COMPILER=/usr/bin/gcc -DCMAKE_CXX_COMPILER=/usr/bin/g++ -DLLAMA_CURL=OFF"
    
    if [ "$GPU_TYPE" = "nvidia" ]; then
        print_info "Configuring for NVIDIA CUDA acceleration..."
        CMAKE_ARGS="$CMAKE_ARGS -DLLAMA_CUBLAS=ON"
    elif [ "$GPU_TYPE" = "amd" ]; then
        print_info "Configuring for AMD ROCm acceleration..."
        CMAKE_ARGS="$CMAKE_ARGS -DLLAMA_HIPBLAS=ON"
    else
        print_info "Configuring for CPU-only inference..."
    fi
    
    cmake .. $CMAKE_ARGS 2>&1 | grep -E "Build files|llama.cpp|CUDA|HIP|OpenBLAS|Accelerate|compiler" || true
    
    print_step "Building llama.cpp (this may take 5-10 minutes)..."
    print_info "Building with $(nproc) CPU cores..."
    
    # Build and check for success
    if timeout 600 cmake --build . --config Release -j$(nproc) 2>&1 | tee /tmp/llama_build.log | grep -v "warning:" | grep -E "Built target|error:|Error|FAILED"; then
        # Check if critical libraries were built
        if [ -f "src/libllama.so" ] || [ -f "libllama.so" ] || [ -f "ggml/src/libggml.so" ] || ls *.so &>/dev/null; then
            print_success "Build completed successfully"
        else
            print_warning "Build completed but some libraries may be missing"
            print_info "Checking what was built..."
            ls -la *.so src/*.so ggml/src/*.so 2>/dev/null | head -n 5 || true
        fi
    else
        print_error "Build failed or timed out"
        print_warning "Will build OffGrid LLM in mock mode"
        cd "$ORIGINAL_DIR"
        return 1
    fi
    
    # Install libraries
    print_step "Installing llama.cpp libraries..."
    sudo cmake --install . 2>&1 | grep -E "Install|Up-to-date" || {
        print_warning "Failed to install llama.cpp system-wide"
        print_info "Will use local build"
    }
    
    print_success "llama.cpp built successfully at $LLAMA_DIR/build"
    
    # Export paths for Go build
    export C_INCLUDE_PATH="$LLAMA_DIR:${C_INCLUDE_PATH:-}"
    export LIBRARY_PATH="$LLAMA_DIR/build:${LIBRARY_PATH:-}"
    export LD_LIBRARY_PATH="$LLAMA_DIR/build:${LD_LIBRARY_PATH:-}"
    
    # Return to original directory
    cd "$ORIGINAL_DIR"
}

# Build OffGrid LLM
build_offgrid() {
    print_header "Building OffGrid LLM"
    
    local BUILD_DIR=$(pwd)
    
    # Verify Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}')
    print_info "Using Go: $GO_VERSION"
    
    print_step "Downloading Go dependencies..."
    go mod download 2>&1 | grep -E "go: downloading|error" || true
    
    # Check if llama.cpp libraries exist
    LLAMA_AVAILABLE=false
    if [ -f "$HOME/llama.cpp/build/src/libllama.so" ] || \
       [ -f "$HOME/llama.cpp/build/libllama.so" ] || \
       [ -f "$HOME/llama.cpp/libllama.so" ] || \
       [ -f "/usr/local/lib/libggml.so" ] || \
       [ -f "$HOME/llama.cpp/build/ggml/src/libggml.so" ]; then
        LLAMA_AVAILABLE=true
        print_info "llama.cpp libraries found, building with real inference support"
        
        # Set CGO environment for llama.cpp
        export CGO_ENABLED=1
        export C_INCLUDE_PATH="$HOME/llama.cpp:$HOME/llama.cpp/include:$HOME/llama.cpp/ggml/include:${C_INCLUDE_PATH:-}"
        export LIBRARY_PATH="$HOME/llama.cpp/build:$HOME/llama.cpp/build/src:$HOME/llama.cpp/build/ggml/src:$HOME/llama.cpp:/usr/local/lib:${LIBRARY_PATH:-}"
        export LD_LIBRARY_PATH="$HOME/llama.cpp/build:$HOME/llama.cpp/build/src:$HOME/llama.cpp/build/ggml/src:$HOME/llama.cpp:/usr/local/lib:${LD_LIBRARY_PATH:-}"
        
        print_step "Building with llama.cpp integration..."
        if go build -tags llama -o offgrid ./cmd/offgrid 2>&1 | tee /tmp/go_build.log | tail -n 20; then
            if [ -f offgrid ] && [ -x offgrid ]; then
                print_success "Built with llama.cpp support"
            else
                print_error "Build command succeeded but binary not created"
                LLAMA_AVAILABLE=false
            fi
        else
            print_warning "Failed to build with llama support"
            print_info "Check /tmp/go_build.log for details"
            LLAMA_AVAILABLE=false
        fi
    fi
    
    # Fallback to mock mode if llama build failed
    if [ "$LLAMA_AVAILABLE" = false ]; then
        print_step "Building in mock mode (no real inference)..."
        if go build -o offgrid ./cmd/offgrid 2>&1 | tail -n 10; then
            if [ -f offgrid ] && [ -x offgrid ]; then
                print_success "Built in mock mode"
                print_warning "Real inference not available - download a model and it will use mock responses"
            else
                print_error "Failed to build binary"
                exit 1
            fi
        else
            print_error "Build failed"
            exit 1
        fi
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
    print_header "Installing OffGrid LLM"
    
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
    print_header "Setting Up Service User"
    
    # Create offgrid user if it doesn't exist
    if ! id offgrid &> /dev/null; then
        print_step "Creating offgrid system user..."
        sudo useradd -r -s /bin/false -U -m -d /var/lib/offgrid offgrid
        print_success "User 'offgrid' created"
    else
        print_info "User 'offgrid' already exists"
    fi
    
    # Add to video and render groups for GPU access
    if [ "$GPU_TYPE" != "none" ]; then
        print_step "Adding offgrid user to GPU groups..."
        sudo usermod -aG video offgrid 2>/dev/null || true
        sudo usermod -aG render offgrid 2>/dev/null || true
    fi
}

# Create Systemd Service
setup_systemd_service() {
    print_header "Setting Up Systemd Service"
    
    local SERVICE_FILE="/etc/systemd/system/offgrid-llm.service"
    
    print_step "Creating service file at $SERVICE_FILE..."
    
    sudo tee "$SERVICE_FILE" > /dev/null <<'SERVICE_EOF'
[Unit]
Description=OffGrid LLM - Offline AI Inference Engine
Documentation=https://github.com/yourusername/offgrid-llm
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=offgrid
Group=offgrid
WorkingDirectory=/var/lib/offgrid
ExecStart=/usr/local/bin/offgrid serve
Restart=always
RestartSec=3
Environment="OFFGRID_PORT=11611"
Environment="OFFGRID_MODELS_DIR=/var/lib/offgrid/models"

# Security settings
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
    print_header "Setting Up Configuration"
    
    local CONFIG_DIR="/var/lib/offgrid"
    local MODELS_DIR="$CONFIG_DIR/models"
    local WEB_DIR="$CONFIG_DIR/web/ui"
    
    print_step "Creating directories..."
    sudo mkdir -p "$MODELS_DIR"
    sudo mkdir -p "$WEB_DIR"
    
    print_step "Copying web UI files..."
    if [ -f "web/ui/index.html" ]; then
        sudo cp -r web/ui/* "$WEB_DIR/"
        print_success "Web UI files copied"
    else
        print_warning "Web UI files not found in current directory"
    fi
    
    sudo chown -R offgrid:offgrid "$CONFIG_DIR"
    
    print_success "Configuration directories created"
}

# Start Service
start_service() {
    print_header "Starting OffGrid LLM Service"
    
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
    print_header "Installation Complete! 🎉"
    
    echo ""
    print_success "OffGrid LLM has been installed successfully"
    echo ""
    
    echo -e "${BOLD}System Information:${RESET}"
    echo -e "  Architecture: ${CYAN}$ARCH${RESET}"
    echo -e "  OS: ${CYAN}$NAME $VERSION${RESET}"
    echo -e "  GPU: ${CYAN}${GPU_TYPE}${RESET}"
    if [ "$GPU_TYPE" != "none" ]; then
        echo -e "  GPU Info: ${CYAN}${GPU_INFO}${RESET}"
    fi
    echo ""
    
    echo -e "${BOLD}Service Information:${RESET}"
    echo -e "  Status: ${GREEN}Running${RESET}"
    echo -e "  Port: ${CYAN}11611${RESET}"
    echo -e "  Web UI: ${CYAN}http://localhost:11611/ui${RESET}"
    echo -e "  API: ${CYAN}http://localhost:11611${RESET}"
    echo ""
    
    echo -e "${BOLD}Useful Commands:${RESET}"
    echo -e "  ${CYAN}offgrid serve${RESET}          - Start server manually"
    echo -e "  ${CYAN}offgrid list${RESET}           - List available models"
    echo -e "  ${CYAN}offgrid download <model>${RESET} - Download a model"
    echo -e "  ${CYAN}sudo systemctl status offgrid-llm${RESET}  - Check service status"
    echo -e "  ${CYAN}sudo journalctl -u offgrid-llm -f${RESET}  - View live logs"
    echo ""
    
    echo -e "${BOLD}Next Steps:${RESET}"
    echo -e "  1. Visit ${CYAN}http://localhost:11611/ui${RESET} in your browser"
    echo -e "  2. Download a model: ${CYAN}offgrid download tinyllama${RESET}"
    echo -e "  3. Start chatting with your offline AI!"
    echo ""
}

# Main Installation Flow
main() {
    print_header "OffGrid LLM Installation"
    echo -e "${CYAN}Offline AI inference for the edge${RESET}"
    echo ""
    
    # Pre-flight checks
    check_dependencies
    detect_architecture
    detect_os
    detect_gpu
    
    # Build and install
    install_build_deps
    install_nvidia_drivers
    build_llama_cpp
    build_offgrid
    install_binary
    
    # System setup
    setup_user
    setup_config
    setup_systemd_service
    start_service
    
    # Summary
    display_summary
}

# Run main installation
main "$@"
