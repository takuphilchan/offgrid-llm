#!/usr/bin/env bash
# Source-build dependency, hardware, and compilation helpers for scripts/build/install-from-source.sh.

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

# Install a supported, security-patched Go toolchain
install_go() {
    print_progress "Installing Go Programming Language" "~1-2 minutes"

    local REQUIRED_GO_VERSION="1.26.6"
    local GO_VERSION="1.26.6"
    local GO_TARBALL="go${GO_VERSION}.linux-${ARCH}.tar.gz"
    local GO_URL="https://go.dev/dl/${GO_TARBALL}"
    local GO_SHA256

    case "$ARCH" in
        amd64)
            GO_SHA256="708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89"
            ;;
        arm64)
            GO_SHA256="d0507e9e9d7fe012aae570108cbd76c15de879e17130ab8cb90d4d7445cb1f2e"
            ;;
        *)
            print_error "No trusted Go checksum configured for architecture: $ARCH"
            return 1
            ;;
    esac

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

    print_step "Verifying Go archive checksum..."
    if ! printf '%s  %s\n' "$GO_SHA256" "$GO_TARBALL" | sha256sum -c -; then
        print_error "Go archive checksum verification failed"
        rm -f "$GO_TARBALL"
        return 1
    fi

    # Avoid overlaying a new toolchain onto files from an older release.
    sudo rm -rf /usr/local/go

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
    # Note: BUILD_SHARED_LIBS=OFF ensures backends are statically linked into llama-server
    # This avoids "no backends are loaded" errors when running as a service
    CMAKE_ARGS="-DBUILD_SHARED_LIBS=OFF -DCMAKE_C_COMPILER=/usr/bin/gcc -DCMAKE_CXX_COMPILER=/usr/bin/g++ -DLLAMA_CURL=OFF"

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

    # Use the supported Go toolchain if available - check known locations
    if [ -f "/usr/local/go/bin/go" ]; then
        GO_CMD="/usr/local/go/bin/go"
        print_info "Using installed Go at /usr/local/go/bin/go"
    elif [ -f "$HOME/go1.26.6/bin/go" ]; then
        GO_CMD="$HOME/go1.26.6/bin/go"
        print_info "Using installed Go at $HOME/go1.26.6/bin/go"
    else
        # Try to find a supported Go version in PATH
        if command -v go &> /dev/null; then
            GO_VERSION_CHECK=$(go version | awk '{print $3}' | sed 's/go//')
            if [ "$(printf '%s\n' "1.26.6" "$GO_VERSION_CHECK" | sort -V | head -n1)" = "1.26.6" ]; then
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
    GO_VERSION_NUM=$(echo $GO_VERSION | sed 's/go//')
    MIN_GO_VERSION="1.26.6"
    if [ "$(printf '%s\n' "$MIN_GO_VERSION" "$GO_VERSION_NUM" | sort -V | head -n1)" != "$MIN_GO_VERSION" ]; then
        print_warning "Go version $GO_VERSION_NUM is older than required $MIN_GO_VERSION"
        print_warning "Attempting to use system Go anyway..."
    fi

    print_step "Downloading Go dependencies..."
    GOTOOLCHAIN=local $GO_CMD mod download 2>&1 | grep -E "go: downloading|error" || true

    # Note: OffGrid uses HTTP-based communication with llama-server for inference
    # This provides better stability and compatibility than direct CGO bindings
    # Architecture: OffGrid (Go) ⟷ HTTP ⟷ llama-server (C++)

    print_step "Building OffGrid LLM server..."
    print_info "Using HTTP-based llama.cpp integration for maximum compatibility"

    if GOTOOLCHAIN=local $GO_CMD build -o offgrid ./cmd/offgrid 2>&1 | tee /tmp/go_build.log | tail -n 10; then
        if [ -f offgrid ] && [ -x offgrid ]; then
            print_success "Built successfully"
            print_info "Real inference via llama-server HTTP backend"
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
