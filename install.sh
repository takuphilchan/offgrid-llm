#!/bin/bash
# OffGrid LLM - Universal Installer
# One command to install everything: CLI, Desktop, Audio (Whisper/Piper)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
#
# Options (environment variables):
#   CLI=yes|no         Install CLI tools (default: yes)
#   DESKTOP=yes|no     Install Desktop app (default: yes)
#   AUDIO=yes|no       Install Audio - Whisper STT + Piper TTS (default: yes)
#   AUTOSTART=yes|no   Auto-start service (default: ask)
#   NONINTERACTIVE=yes Skip all prompts, use defaults
#   GPU=cpu|vulkan     CPU (default) or Vulkan GPU acceleration
#
# Requirements:
#   Linux: GLIBC 2.35+ (Ubuntu 22.04+, Debian 12+)
#   macOS: 12.0+ (Monterey) - Apple Silicon uses Metal automatically
#   Windows: Windows 10+

set -e

# ═══════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════
REPO="takuphilchan/offgrid-llm"
GITHUB_URL="https://github.com/${REPO}"
INSTALL_DIR="/usr/local/bin"
VERSION="${VERSION:-latest}"

# Colors (disable if not a terminal)
if [ -t 1 ]; then
    BOLD='\033[1m'
    DIM='\033[2m'
    GREEN='\033[32m'
    RED='\033[31m'
    YELLOW='\033[33m'
    CYAN='\033[36m'
    NC='\033[0m'
else
    BOLD='' DIM='' GREEN='' RED='' YELLOW='' CYAN='' NC=''
fi

# Helper Functions (matching CLI style)
ok()      { printf "  ${GREEN}\xE2\x9C\x93${NC} %s\n" "$1" >&2; }
error()   { printf "  ${RED}\xE2\x9C\x97${NC} %s\n" "$1" >&2; }
warn()    { printf "  ${YELLOW}\xE2\x97\xA6${NC} %s\n" "$1" >&2; }
info()    { printf "  ${CYAN}\xE2\x86\x92${NC} %s\n" "$1" >&2; }
step()    { printf "  ${CYAN}\xE2\x87\xA3${NC} %s\n" "$1" >&2; }
dim()     { printf "    ${DIM}%s${NC}\n" "$1" >&2; }
section() { printf "\n  ${CYAN}\xE2\x97\x88${NC} ${BOLD}%s${NC}\n" "$1" >&2; }

print_banner() {
    echo "" >&2
    printf "  ${CYAN}\xE2\x97\x88${NC} ${BOLD}OffGrid LLM${NC} Installer\n" >&2
    printf "  ${DIM}Universal installer for CLI, Desktop & Audio${NC}\n" >&2
    echo "" >&2
}

# ═══════════════════════════════════════════════════════════════
# System Detection
# ═══════════════════════════════════════════════════════════════
detect_os() {
    local os
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    
    case "$os" in
        linux*) echo "linux" ;;
        darwin*) echo "darwin" ;;
        mingw*|msys*|cygwin*) echo "windows" ;;
        *) error "Unsupported operating system: $os"; exit 1 ;;
    esac
}

detect_arch() {
    local arch
    arch="$(uname -m)"
    
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) error "Unsupported architecture: $arch"; exit 1 ;;
    esac
}

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
        if sysctl machdep.cpu.features machdep.cpu.leaf7_features 2>/dev/null | grep -qi "avx512"; then
            cpu_variant="avx512"
        else
            cpu_variant="avx2"
        fi
    fi
    
    echo "$cpu_variant"
}

detect_gpu() {
    local os="$1"
    local variant="cpu"
    
    # Auto-detect GPU for acceleration
    if [ "$os" = "linux" ]; then
        # Check for Vulkan-capable GPU
        if command -v vulkaninfo >/dev/null 2>&1 && vulkaninfo --summary >/dev/null 2>&1; then
            variant="vulkan"
        elif command -v nvidia-smi >/dev/null 2>&1; then
            variant="vulkan"
        elif [ -d "/sys/class/drm" ] && ls /sys/class/drm/card*/device/vendor 2>/dev/null | xargs cat 2>/dev/null | grep -q "0x1002"; then
            # AMD GPU detected
            variant="vulkan"
        fi
    elif [ "$os" = "darwin" ]; then
        # macOS Apple Silicon uses Metal
        if [ "$(uname -m)" = "arm64" ]; then
            variant="metal"
        fi
    fi
    
    echo "$variant"
}

get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
        error "Failed to fetch latest version"
        exit 1
    fi
    
    echo "$version"
}

# Check GLIBC version (for Linux binary compatibility)
# Returns: "compatible" if >= required, "incompatible" otherwise
check_glibc_version() {
    local required_major="${1:-2}"
    local required_minor="${2:-38}"
    
    if [ "$(uname -s)" != "Linux" ]; then
        echo "compatible"
        return
    fi
    
    local glibc_version
    glibc_version=$(ldd --version 2>&1 | head -1 | grep -oE '[0-9]+\.[0-9]+' | head -1)
    
    if [ -z "$glibc_version" ]; then
        echo "unknown"
        return
    fi
    
    local major minor
    major=$(echo "$glibc_version" | cut -d. -f1)
    minor=$(echo "$glibc_version" | cut -d. -f2)
    
    if [ "$major" -gt "$required_major" ] || \
       ([ "$major" -eq "$required_major" ] && [ "$minor" -ge "$required_minor" ]); then
        echo "compatible"
    else
        echo "incompatible:$glibc_version"
    fi
}

# Get GLIBC version string
get_glibc_version() {
    ldd --version 2>&1 | head -1 | grep -oE '[0-9]+\.[0-9]+' | head -1
}

# ═══════════════════════════════════════════════════════════════
# Installation Menu
# ═══════════════════════════════════════════════════════════════
show_menu() {
    local os="$1"
    local has_gpu="$2"
    
    section "Installation Options"
    echo "" >&2
    printf "    ${GREEN}1.${NC} Full ${DIM}(recommended)${NC}\n" >&2
    printf "       ${DIM}CLI + Desktop + Voice${NC}\n" >&2
    printf "    ${GREEN}2.${NC} CLI + Voice\n" >&2
    printf "       ${DIM}Command-line with speech recognition${NC}\n" >&2
    printf "    ${GREEN}3.${NC} CLI only\n" >&2
    printf "       ${DIM}Minimal install (~50MB)${NC}\n" >&2
    printf "    ${GREEN}4.${NC} Custom\n" >&2
    printf "       ${DIM}Choose individual components${NC}\n" >&2
    echo "" >&2
    
    read -p "  Enter choice [1-4] (default: 1): " choice
    choice="${choice:-1}"
    
    case "$choice" in
        1)
            INSTALL_CLI="yes"
            INSTALL_DESKTOP="yes"
            INSTALL_AUDIO="yes"
            ;;
        2)
            INSTALL_CLI="yes"
            INSTALL_DESKTOP="no"
            INSTALL_AUDIO="yes"
            ;;
        3)
            INSTALL_CLI="yes"
            INSTALL_DESKTOP="no"
            INSTALL_AUDIO="no"
            ;;
        4)
            custom_menu "$has_gpu"
            ;;
        *)
            INSTALL_CLI="yes"
            INSTALL_DESKTOP="yes"
            INSTALL_AUDIO="yes"
            ;;
    esac
}

custom_menu() {
    local has_gpu="$1"
    
    echo "" >&2
    echo "  Custom Installation" >&2
    echo "" >&2
    
    # CLI (always yes for custom, needed for everything)
    INSTALL_CLI="yes"
    info "CLI tools will be installed (required)"
    
    # Desktop
    read -p "  Install Desktop app? [Y/n]: " desktop_choice
    desktop_choice="${desktop_choice:-Y}"
    if [[ "$desktop_choice" =~ ^[Yy] ]]; then
        INSTALL_DESKTOP="yes"
    else
        INSTALL_DESKTOP="no"
    fi
    
    # Audio (Voice Assistant)
    read -p "  Install Voice Assistant (Whisper + Piper)? [Y/n]: " audio_choice
    audio_choice="${audio_choice:-Y}"
    if [[ "$audio_choice" =~ ^[Yy] ]]; then
        INSTALL_AUDIO="yes"
    else
        INSTALL_AUDIO="no"
    fi
    
    # GPU acceleration (if available)
    if [ "$has_gpu" = "true" ]; then
        echo "" >&2
        read -p "  Enable GPU acceleration (Vulkan)? [y/N]: " gpu_choice
        gpu_choice="${gpu_choice:-N}"
        if [[ "$gpu_choice" =~ ^[Yy] ]]; then
            ENABLE_GPU="yes"
        else
            ENABLE_GPU="no"
        fi
    fi
}

# ═══════════════════════════════════════════════════════════════
# Download Functions
# ═══════════════════════════════════════════════════════════════
download_cli_bundle() {
    local os="$1"
    local arch="$2"
    local version="$3"
    local variant="$4"
    local cpu_features="$5"
    local tmp_dir="$6"
    
    local bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
    local ext=".tar.gz"
    [ "$os" = "windows" ] && ext=".zip"
    
    local download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
    
    section "Downloading"
    step "CLI Bundle"
    dim "${bundle_name}${ext}"
    
    local start_time=$(date +%s)
    
    if ! curl -fsSL -o "${tmp_dir}/bundle${ext}" "$download_url" 2>/dev/null; then
        # Fallback to CPU variant
        if [ "$variant" != "cpu" ]; then
            dim "GPU variant unavailable, trying CPU..."
            variant="cpu"
            bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
            download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
            
            if ! curl -fsSL -o "${tmp_dir}/bundle${ext}" "$download_url" 2>/dev/null; then
                # Fallback to AVX2
                if [ "$cpu_features" = "avx512" ]; then
                    dim "Trying AVX2 version..."
                    cpu_features="avx2"
                    bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
                    download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
                    curl -fsSL -o "${tmp_dir}/bundle${ext}" "$download_url" || return 1
                else
                    return 1
                fi
            fi
        else
            return 1
        fi
    fi
    
    local end_time=$(date +%s)
    local elapsed=$((end_time - start_time))
    local size=$(du -h "${tmp_dir}/bundle${ext}" 2>/dev/null | cut -f1)
    ok "Downloaded ${size} in ${elapsed}s"
    
    # Extract
    dim "Extracting..."
    cd "$tmp_dir"
    if [ "$os" = "windows" ]; then
        unzip -q "bundle${ext}"
    else
        tar -xzf "bundle${ext}"
    fi
    ok "Extracted"
    
    echo "$bundle_name"
}

download_desktop_app() {
    local os="$1"
    local arch="$2"
    local version="$3"
    local tmp_dir="$4"
    
    local app_name=""
    local download_url=""
    local version_num="${version#v}"  # Remove 'v' prefix
    
    case "$os" in
        linux)
            if [ "$arch" = "amd64" ]; then
                app_name="OffGrid.LLM.Desktop-${version_num}-x86_64.AppImage"
            else
                app_name="OffGrid.LLM.Desktop-${version_num}-arm64.AppImage"
            fi
            download_url="${GITHUB_URL}/releases/download/${version}/${app_name}"
            ;;
        darwin)
            if [ "$arch" = "arm64" ]; then
                app_name="OffGrid.LLM.Desktop-${version_num}-arm64.zip"
            else
                app_name="OffGrid.LLM.Desktop-${version_num}-x64.zip"
            fi
            download_url="${GITHUB_URL}/releases/download/${version}/${app_name}"
            ;;
        windows)
            app_name="OffGrid.LLM.Desktop-Setup-${version_num}.exe"
            download_url="${GITHUB_URL}/releases/download/${version}/${app_name}"
            ;;
    esac
    
    step "Desktop App"
    dim "$app_name"
    
    local start_time=$(date +%s)
    
    if curl -fsSL -o "${tmp_dir}/${app_name}" "$download_url" 2>/dev/null; then
        local end_time=$(date +%s)
        local elapsed=$((end_time - start_time))
        local size=$(du -h "${tmp_dir}/${app_name}" 2>/dev/null | cut -f1)
        ok "Downloaded ${size} in ${elapsed}s"
        echo "${tmp_dir}/${app_name}"
    else
        warn "Desktop app not available for this platform"
        echo ""
    fi
}

# ═══════════════════════════════════════════════════════════════
# Build Whisper from Source (for GLIBC compatibility)
# ═══════════════════════════════════════════════════════════════
build_whisper_from_source() {
    local install_dir="$1"
    local build_dir="/tmp/whisper-build-$$"
    
    # Check for required build tools
    local missing_tools=""
    command -v git >/dev/null 2>&1 || missing_tools="$missing_tools git"
    command -v cmake >/dev/null 2>&1 || missing_tools="$missing_tools cmake"
    command -v make >/dev/null 2>&1 || missing_tools="$missing_tools make"
    (command -v g++ >/dev/null 2>&1 || command -v clang++ >/dev/null 2>&1) || missing_tools="$missing_tools g++"
    
    if [ -n "$missing_tools" ]; then
        warn "Missing build tools:$missing_tools"
        info "Installing build dependencies..."
        
        # Try to install missing tools
        if command -v apt-get >/dev/null 2>&1; then
            sudo apt-get update -qq
            sudo apt-get install -y -qq git cmake build-essential
        elif command -v dnf >/dev/null 2>&1; then
            sudo dnf install -y git cmake gcc-c++ make
        elif command -v yum >/dev/null 2>&1; then
            sudo yum install -y git cmake gcc-c++ make
        elif command -v pacman >/dev/null 2>&1; then
            sudo pacman -S --noconfirm git cmake base-devel
        else
            error "Cannot install build tools automatically. Please install: git cmake g++ make"
            return 1
        fi
    fi
    
    # Clone and build
    rm -rf "$build_dir"
    mkdir -p "$build_dir"
    
    dim "Cloning whisper.cpp..."
    if ! git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git "$build_dir/whisper.cpp" 2>/dev/null; then
        error "Failed to clone whisper.cpp"
        rm -rf "$build_dir"
        return 1
    fi
    
    cd "$build_dir/whisper.cpp"
    
    dim "Building whisper.cpp (this may take a few minutes)..."
    # Build with static linking to avoid LD_LIBRARY_PATH issues
    if ! cmake -B build -DCMAKE_BUILD_TYPE=Release -DGGML_CCACHE=OFF -DBUILD_SHARED_LIBS=OFF >/dev/null 2>&1; then
        error "CMake configuration failed"
        cd - >/dev/null
        rm -rf "$build_dir"
        return 1
    fi
    
    local num_cores
    num_cores=$(nproc 2>/dev/null || echo 4)
    
    if ! cmake --build build --config Release -j"$num_cores" >/dev/null 2>&1; then
        error "Build failed"
        cd - >/dev/null
        rm -rf "$build_dir"
        return 1
    fi
    
    # Install binaries
    mkdir -p "$install_dir"
    
    # Copy the main binary
    if [ -f "build/bin/whisper-cli" ]; then
        cp "build/bin/whisper-cli" "$install_dir/"
    elif [ -f "build/bin/main" ]; then
        cp "build/bin/main" "$install_dir/whisper-cli"
    else
        error "whisper-cli binary not found after build"
        cd - >/dev/null
        rm -rf "$build_dir"
        return 1
    fi
    
    chmod +x "$install_dir/whisper-cli"
    
    # Copy shared libraries if they exist
    find build -name "*.so*" -exec cp {} "$install_dir/" \; 2>/dev/null || true
    
    cd - >/dev/null
    rm -rf "$build_dir"
    
    return 0
}

# ═══════════════════════════════════════════════════════════════
# Installation Functions
# ═══════════════════════════════════════════════════════════════
install_cli() {
    local bundle_dir="$1"
    local os="$2"
    
    dim "Installing CLI tools..."
    
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"
    
    # Determine if we need sudo
    local use_sudo=""
    if [ "$os" != "windows" ] && [ ! -w "$INSTALL_DIR" ]; then
        use_sudo="sudo"
    fi
    
    # Stop any running offgrid processes before installing
    if [ "$os" != "windows" ]; then
        local was_running=false
        
        # Check if offgrid is running
        if pgrep -f "offgrid serve\|offgrid run\|llama-server" >/dev/null 2>&1; then
            was_running=true
            dim "Stopping running OffGrid processes..."
            
            # Try graceful stop first
            pkill -f "offgrid serve" 2>/dev/null || true
            pkill -f "offgrid run" 2>/dev/null || true
            pkill -x "llama-server" 2>/dev/null || true
            sleep 2
            
            # Force kill if still running
            if pgrep -f "offgrid serve\|offgrid run\|llama-server" >/dev/null 2>&1; then
                pkill -9 -f "offgrid serve" 2>/dev/null || true
                pkill -9 -f "offgrid run" 2>/dev/null || true
                pkill -9 -x "llama-server" 2>/dev/null || true
                sleep 1
            fi
        fi
        
        # Also stop systemd service if running
        if systemctl --user is-active offgrid >/dev/null 2>&1; then
            dim "Stopping offgrid service..."
            systemctl --user stop offgrid 2>/dev/null || true
            sleep 1
        fi
    fi
    
    # Copy binaries (with retry for "Text file busy")
    local retries=3
    local copied=false
    
    for i in $(seq 1 $retries); do
        if $use_sudo cp "$bundle_dir/offgrid${ext}" "$INSTALL_DIR/" 2>/dev/null && \
           $use_sudo cp "$bundle_dir/llama-server${ext}" "$INSTALL_DIR/" 2>/dev/null; then
            copied=true
            break
        fi
        
        if [ $i -lt $retries ]; then
            dim "Waiting for file lock to release..."
            sleep 2
        fi
    done
    
    if [ "$copied" != "true" ]; then
        error "Failed to copy binaries. Please stop any running offgrid processes and try again."
        return 1
    fi
    
    $use_sudo chmod +x "$INSTALL_DIR/offgrid${ext}" "$INSTALL_DIR/llama-server${ext}"
    
    ok "CLI installed"
}

install_audio() {
    local bundle_dir="$1"
    local interactive="$2"
    
    section "Audio Setup"
    
    local AUDIO_DIR="$HOME/.offgrid-llm/audio"
    mkdir -p "$AUDIO_DIR/whisper" "$AUDIO_DIR/piper"
    
    local os
    os="$(uname -s)"
    
    # Check GLIBC compatibility on Linux
    local glibc_status="compatible"
    local glibc_version=""
    
    if [ "$os" = "Linux" ]; then
        glibc_status=$(check_glibc_version 2 38)
        glibc_version=$(get_glibc_version)
        
        if [[ "$glibc_status" == incompatible* ]]; then
            warn "GLIBC $glibc_version detected (2.38+ required for binaries)"
            echo "" >&2
            
            if [ "$interactive" = "yes" ]; then
                echo "  Audio options:" >&2
                echo "    1) Build from source (recommended, ~5 min)" >&2
                echo "    2) Skip (install later)" >&2
                echo "" >&2
                
                local audio_choice
                read -r -p "  Enter choice [1-2] (default: 1): " audio_choice </dev/tty
                audio_choice="${audio_choice:-1}"
                
                case "$audio_choice" in
                    2)
                        dim "Skipped. Run 'offgrid audio setup' later."
                        return 0
                        ;;
                    *)
                        # Continue with build from source
                        ;;
                esac
            else
                dim "Building audio from source..."
            fi
        else
            dim "GLIBC $glibc_version — compatible with binaries"
        fi
    fi
    
    # Install Whisper (Speech-to-Text)
    local whisper_installed=false
    step "Audio Components"
    
    if [ "$os" = "Linux" ]; then
        if [[ "$glibc_status" == "compatible" ]] && [ -d "$bundle_dir/audio/whisper" ]; then
            # Use pre-built binaries
            dim "Installing Whisper (Speech-to-Text)..."
            cp -r "$bundle_dir/audio/whisper/"* "$AUDIO_DIR/whisper/" 2>/dev/null || true
            chmod +x "$AUDIO_DIR/whisper/"* 2>/dev/null || true
            
            # Test if it works
            if "$AUDIO_DIR/whisper/whisper-cli" --help >/dev/null 2>&1; then
                whisper_installed=true
                ok "Whisper ready"
            else
                warn "Pre-built Whisper failed, building from source..."
                rm -rf "$AUDIO_DIR/whisper"/*
            fi
        fi
        
        if [ "$whisper_installed" = "false" ]; then
            # Build from source
            dim "Building Whisper from source (~2-5 min)..."
            local build_start=$(date +%s)
            if build_whisper_from_source "$AUDIO_DIR/whisper"; then
                local build_end=$(date +%s)
                local build_time=$((build_end - build_start))
                whisper_installed=true
                ok "Whisper built in ${build_time}s"
            else
                warn "Whisper build failed - voice input will not be available"
                dim "You can try later with: offgrid audio setup whisper"
            fi
        fi
    elif [ -d "$bundle_dir/audio/whisper" ]; then
        # macOS/Windows: use pre-built binaries
        dim "Installing Whisper (Speech-to-Text)..."
        cp -r "$bundle_dir/audio/whisper/"* "$AUDIO_DIR/whisper/" 2>/dev/null || true
        chmod +x "$AUDIO_DIR/whisper/"* 2>/dev/null || true
        whisper_installed=true
        ok "Whisper ready"
    else
        warn "Whisper binaries not in bundle - will build on first use"
    fi
    
    # Install Piper (Text-to-Speech)
    local piper_installed=false
    
    if [ "$os" = "Linux" ]; then
        dim "Downloading Piper (Text-to-Speech)..."
        if download_piper "$AUDIO_DIR/piper"; then
            # Test if it works
            if LD_LIBRARY_PATH="$AUDIO_DIR/piper:$LD_LIBRARY_PATH" "$AUDIO_DIR/piper/piper" --help >/dev/null 2>&1; then
                piper_installed=true
                ok "Piper ready"
            else
                warn "Piper binary not compatible with your system"
                dim "Text-to-speech will not be available"
                rm -rf "$AUDIO_DIR/piper"/*
            fi
        else
            warn "Piper download failed - voice output will not be available"
        fi
    elif [ -d "$bundle_dir/audio/piper" ]; then
        dim "Installing Piper (Text-to-Speech)..."
        cp -r "$bundle_dir/audio/piper/"* "$AUDIO_DIR/piper/" 2>/dev/null || true
        chmod +x "$AUDIO_DIR/piper/"* 2>/dev/null || true
        piper_installed=true
        ok "Piper ready"
    else
        warn "Piper binaries not in bundle - will download on first use"
    fi
}

# Download Piper from official releases
download_piper() {
    local install_dir="$1"
    local arch
    arch="$(uname -m)"
    
    local piper_url=""
    case "$arch" in
        x86_64|amd64)
            piper_url="https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz"
            ;;
        aarch64|arm64)
            piper_url="https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_aarch64.tar.gz"
            ;;
        *)
            warn "Unsupported architecture for Piper: $arch"
            return 1
            ;;
    esac
    
    local tmp_dir="/tmp/piper-download-$$"
    mkdir -p "$tmp_dir"
    
    dim "Downloading Piper..."
    if ! curl -fsSL -o "$tmp_dir/piper.tar.gz" "$piper_url" 2>/dev/null; then
        rm -rf "$tmp_dir"
        return 1
    fi
    
    dim "Extracting Piper..."
    if ! tar -xzf "$tmp_dir/piper.tar.gz" -C "$tmp_dir" 2>/dev/null; then
        rm -rf "$tmp_dir"
        return 1
    fi
    
    # Clean existing piper installation to avoid conflicts
    rm -rf "$install_dir"
    mkdir -p "$install_dir"
    
    # Copy piper files (tarball extracts to piper/ subdirectory)
    if [ -d "$tmp_dir/piper" ]; then
        cp -r "$tmp_dir/piper/"* "$install_dir/"
    fi
    
    chmod +x "$install_dir/piper" 2>/dev/null || true
    
    # Create lib symlinks
    cd "$install_dir"
    [ -f "libpiper_phonemize.so.1.2.0" ] && ln -sf "libpiper_phonemize.so.1.2.0" "libpiper_phonemize.so.1" 2>/dev/null && ln -sf "libpiper_phonemize.so.1" "libpiper_phonemize.so" 2>/dev/null
    [ -f "libonnxruntime.so.1.14.1" ] && ln -sf "libonnxruntime.so.1.14.1" "libonnxruntime.so.1" 2>/dev/null && ln -sf "libonnxruntime.so.1" "libonnxruntime.so" 2>/dev/null
    [ -f "libespeak-ng.so.1.52.0.1" ] && ln -sf "libespeak-ng.so.1.52.0.1" "libespeak-ng.so.1" 2>/dev/null && ln -sf "libespeak-ng.so.1" "libespeak-ng.so" 2>/dev/null
    cd - >/dev/null
    
    rm -rf "$tmp_dir"
    return 0
}

install_webui() {
    local bundle_dir="$1"
    
    dim "Installing Web UI..."
    
    local WEB_DIR="/var/lib/offgrid/web/ui"
    
    # Determine if we need sudo
    local use_sudo=""
    if [ ! -w "/var/lib" ] 2>/dev/null; then
        use_sudo="sudo"
    fi
    
    $use_sudo mkdir -p "$WEB_DIR"
    
    if [ -d "$bundle_dir/web/ui" ]; then
        $use_sudo cp -r "$bundle_dir/web/ui/"* "$WEB_DIR/"
        ok "Web UI ready"
    else
        # Download from GitHub
        dim "Fetching latest Web UI..."
        local ui_tmp="/tmp/offgrid-ui-$$"
        mkdir -p "$ui_tmp"
        
        if curl -fsSL "${GITHUB_URL}/archive/refs/heads/main.tar.gz" | tar -xz -C "$ui_tmp" --strip-components=2 "offgrid-llm-main/web/ui" 2>/dev/null; then
            $use_sudo cp -r "$ui_tmp/"* "$WEB_DIR/"
            rm -rf "$ui_tmp"
            ok "Web UI ready"
        else
            rm -rf "$ui_tmp"
            warn "Web UI download failed"
        fi
    fi
}
install_desktop() {
    local app_path="$1"
    local os="$2"
    
    if [ -z "$app_path" ] || [ ! -f "$app_path" ]; then
        warn "Desktop app not available"
        return
    fi
    
    info "Installing Desktop app..."
    
    case "$os" in
        linux)
            # Install AppImage - always extract to avoid FUSE requirement
            # FUSE is often not available, especially on WSL, containers, minimal installs
            local app_dir="$HOME/.local/share/offgrid-desktop"
            local bin_dir="$HOME/.local/bin"
            
            # Clean previous installation
            rm -rf "$app_dir"
            mkdir -p "$app_dir" "$bin_dir"
            
            dim "Extracting AppImage..."
            chmod +x "$app_path"
            cd "$app_dir"
            "$app_path" --appimage-extract >/dev/null 2>&1 || true
            cd - >/dev/null
            
            # Create launcher script
            if [ -d "$app_dir/squashfs-root" ]; then
                cat > "$bin_dir/offgrid-desktop" << 'LAUNCHER'
#!/bin/bash
cd "$HOME/.local/share/offgrid-desktop/squashfs-root"
exec ./AppRun "$@"
LAUNCHER
                chmod +x "$bin_dir/offgrid-desktop"
            else
                # Extraction failed - try copying AppImage directly (requires FUSE)
                dim "Using AppImage directly (requires FUSE)..."
                cp "$app_path" "$bin_dir/offgrid-desktop"
                chmod +x "$bin_dir/offgrid-desktop"
            fi
            
            # Create desktop entry
            local desktop_dir="$HOME/.local/share/applications"
            mkdir -p "$desktop_dir"
            cat > "$desktop_dir/offgrid-llm.desktop" << EOF
[Desktop Entry]
Name=OffGrid LLM
Comment=Local AI Assistant
Exec=$bin_dir/offgrid-desktop
Icon=offgrid-llm
Type=Application
Categories=Utility;Development;
Terminal=false
EOF
            ok "Desktop app installed"
            ;;
        darwin)
            # Mount DMG and copy app
            local mount_point="/Volumes/OffGrid-LLM"
            hdiutil attach "$app_path" -quiet
            cp -R "${mount_point}/OffGrid LLM.app" /Applications/
            hdiutil detach "$mount_point" -quiet
            ok "Desktop app installed to /Applications"
            ;;
        windows)
            # Just save the installer
            local app_dir="$HOME/Desktop"
            cp "$app_path" "$app_dir/"
            ok "Desktop installer saved to Desktop"
            dim "Run the installer to complete setup"
            ;;
    esac
}

# ═══════════════════════════════════════════════════════════════
# Auto-start Service Setup
# ═══════════════════════════════════════════════════════════════
setup_autostart() {
    local os="$1"
    
    echo "" >&2
    section "Auto-start"
    
    # Ask user
    printf "    Install as system service? (starts on boot)\n" >&2
    printf "    ${DIM}This lets OffGrid run in background automatically${NC}\n" >&2
    echo "" >&2
    
    local install_service="no"
    printf "    Install service? [y/N] " >&2
    read -r response < /dev/tty 2>/dev/null || response="n"
    case "$response" in
        [yY]|[yY][eE][sS]) install_service="yes" ;;
    esac
    
    if [ "$install_service" != "yes" ]; then
        dim "Skipped. Run 'offgrid serve' manually when needed."
        return 0
    fi
    
    case "$os" in
        linux)
            setup_systemd_service
            ;;
        darwin)
            setup_launchd_service
            ;;
        *)
            warn "Auto-start not supported on $os"
            dim "Run 'offgrid serve' manually"
            ;;
    esac
}

setup_systemd_service() {
    local service_dir=""
    local service_file=""
    local user_mode=""
    local offgrid_path=$(which offgrid 2>/dev/null || echo "/usr/local/bin/offgrid")
    local data_dir="${OFFGRID_DATA:-$HOME/.offgrid}"
    
    # Prefer user service (no sudo required)
    if [ -n "${XDG_CONFIG_HOME:-}" ]; then
        service_dir="${XDG_CONFIG_HOME}/systemd/user"
    else
        service_dir="$HOME/.config/systemd/user"
    fi
    service_file="$service_dir/offgrid.service"
    user_mode="--user"
    
    # Create service directory
    mkdir -p "$service_dir"
    
    # Generate service file
    cat > "$service_file" << EOF
[Unit]
Description=OffGrid LLM Server
Documentation=https://github.com/safetorun/offgrid-llm
After=network.target

[Service]
Type=simple
ExecStart=$offgrid_path serve
Environment=OFFGRID_DATA=$data_dir
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF
    
    step "Service file created"
    
    # Reload systemd and enable service
    if systemctl $user_mode daemon-reload 2>/dev/null; then
        step "Systemd reloaded"
    fi
    
    if systemctl $user_mode enable offgrid 2>/dev/null; then
        step "Service enabled (starts on login)"
    fi
    
    # Ask to start now
    printf "    Start OffGrid now? [Y/n] " >&2
    read -r start_now < /dev/tty 2>/dev/null || start_now="y"
    case "$start_now" in
        [nN]|[nN][oO]) 
            dim "Service will start on next login"
            ;;
        *)
            if systemctl $user_mode start offgrid 2>/dev/null; then
                ok "OffGrid is running"
                dim "Check status: systemctl $user_mode status offgrid"
            else
                warn "Failed to start service"
                dim "Try: systemctl $user_mode start offgrid"
            fi
            ;;
    esac
    
    # Enable lingering so user services start at boot (not just login)
    if command -v loginctl >/dev/null 2>&1; then
        loginctl enable-linger "$(whoami)" 2>/dev/null || true
    fi
}

setup_launchd_service() {
    local plist_dir="$HOME/Library/LaunchAgents"
    local plist_file="$plist_dir/com.offgrid.llm.plist"
    local offgrid_path=$(which offgrid 2>/dev/null || echo "/usr/local/bin/offgrid")
    local data_dir="${OFFGRID_DATA:-$HOME/.offgrid}"
    local log_dir="$data_dir/logs"
    
    # Create directories
    mkdir -p "$plist_dir"
    mkdir -p "$log_dir"
    
    # Generate plist file
    cat > "$plist_file" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.offgrid.llm</string>
    <key>ProgramArguments</key>
    <array>
        <string>$offgrid_path</string>
        <string>serve</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>OFFGRID_DATA</key>
        <string>$data_dir</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$log_dir/offgrid.log</string>
    <key>StandardErrorPath</key>
    <string>$log_dir/offgrid.err</string>
</dict>
</plist>
EOF
    
    step "LaunchAgent created"
    
    # Ask to start now
    printf "    Start OffGrid now? [Y/n] " >&2
    read -r start_now < /dev/tty 2>/dev/null || start_now="y"
    case "$start_now" in
        [nN]|[nN][oO]) 
            ok "Service will start on next login"
            ;;
        *)
            # Unload first (in case already loaded)
            launchctl unload "$plist_file" 2>/dev/null || true
            
            if launchctl load "$plist_file" 2>/dev/null; then
                ok "OffGrid is running"
                dim "Logs: $log_dir/offgrid.log"
            else
                warn "Failed to start service"
                dim "Try: launchctl load $plist_file"
            fi
            ;;
    esac
}

# ═══════════════════════════════════════════════════════════════
# Main Installation Flow
# ═══════════════════════════════════════════════════════════════
main() {
    print_banner
    
    # Detect system
    local os=$(detect_os)
    local arch=$(detect_arch)
    local cpu_features=$(detect_cpu_features)
    
    # GPU variant: CPU is default, user can opt-in to GPU
    # Use: GPU=vulkan to enable Vulkan acceleration
    local gpu_variant="cpu"
    local has_gpu=false
    local detected_gpu=""
    
    # Check if GPU is available (for menu display)
    if [ "$os" = "linux" ]; then
        if command -v vulkaninfo >/dev/null 2>&1 && vulkaninfo --summary >/dev/null 2>&1; then
            has_gpu=true
            detected_gpu="vulkan"
        elif command -v nvidia-smi >/dev/null 2>&1; then
            has_gpu=true
            detected_gpu="vulkan"
        elif [ -d "/sys/class/drm" ] && ls /sys/class/drm/card*/device/vendor 2>/dev/null | xargs cat 2>/dev/null | grep -q "0x1002"; then
            has_gpu=true
            detected_gpu="vulkan"
        fi
    elif [ "$os" = "darwin" ]; then
        # macOS Apple Silicon always uses Metal (handled automatically)
        if [ "$arch" = "arm64" ]; then
            gpu_variant="metal"
            detected_gpu="metal"
        fi
    fi
    
    # Environment variable override
    if [ -n "${GPU:-}" ]; then
        gpu_variant="$GPU"
        dim "GPU variant override: $gpu_variant"
    fi
    
    # Check GLIBC for Vulkan builds on Linux (requires 2.38+)
    if [ "$os" = "linux" ] && [ "$gpu_variant" = "vulkan" ]; then
        local glibc_check=$(check_glibc_version 2 38)
        if [ "$glibc_check" != "compatible" ]; then
            local current_glibc=$(ldd --version 2>/dev/null | head -n1 | grep -oE '[0-9]+\.[0-9]+$' || echo "unknown")
            warn "GLIBC $current_glibc < 2.38 - Vulkan requires Ubuntu 24.04+"
            info "Falling back to CPU build"
            gpu_variant="cpu"
            has_gpu=false
        fi
    fi
    
    section "System"
    printf "    ${DIM}OS${NC}        %s\n" "$os/$arch" >&2
    printf "    ${DIM}CPU${NC}       %s\n" "$cpu_features" >&2
    
    # Get version
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(get_latest_version)
    fi
    printf "    ${DIM}Version${NC}   %s\n" "$VERSION" >&2
    
    # Check if running interactively (stdin is a terminal)
    local is_interactive="no"
    if [ -t 0 ] && [ "${NONINTERACTIVE:-}" != "yes" ]; then
        is_interactive="yes"
    fi
    
    # Initialize GPU choice
    ENABLE_GPU="no"
    
    # Show menu if interactive
    if [ "$is_interactive" = "yes" ] && [ -z "${CLI:-}" ] && [ -z "${DESKTOP:-}" ] && [ -z "${AUDIO:-}" ]; then
        show_menu "$os" "$has_gpu"
    else
        # Use defaults or environment variables
        # Default: Full installation (CLI + Desktop + Audio) with CPU
        INSTALL_CLI="${CLI:-yes}"
        INSTALL_DESKTOP="${DESKTOP:-yes}"
        INSTALL_AUDIO="${AUDIO:-yes}"
        
        if [ "$is_interactive" != "yes" ]; then
            dim "Non-interactive mode, using defaults"
        fi
    fi
    
    # Apply GPU choice if user selected it
    if [ "$ENABLE_GPU" = "yes" ] && [ "$has_gpu" = "true" ]; then
        gpu_variant="vulkan"
    fi
    
    # Summary
    section "Components"
    if [ "$INSTALL_CLI" = "yes" ]; then
        printf "    ${GREEN}\xE2\x9C\x93${NC} CLI Tools\n" >&2
    else
        printf "    ${DIM}\xE2\x97\xA6 CLI Tools${NC}\n" >&2
    fi
    if [ "$INSTALL_DESKTOP" = "yes" ]; then
        printf "    ${GREEN}\xE2\x9C\x93${NC} Desktop App\n" >&2
    else
        printf "    ${DIM}\xE2\x97\xA6 Desktop App${NC}\n" >&2
    fi
    if [ "$INSTALL_AUDIO" = "yes" ]; then
        printf "    ${GREEN}\xE2\x9C\x93${NC} Audio (STT/TTS)\n" >&2
    else
        printf "    ${DIM}\xE2\x97\xA6 Audio (STT/TTS)${NC}\n" >&2
    fi
    printf "    ${DIM}Backend${NC}   %s\n" "$gpu_variant" >&2
    echo "" >&2
    
    if [ "$is_interactive" = "yes" ]; then
        read -p "  Proceed? [Y/n]: " confirm
        confirm="${confirm:-Y}"
        if [[ ! "$confirm" =~ ^[Yy] ]]; then
            info "Cancelled"
            exit 0
        fi
    fi
    
    echo "" >&2
    
    # Create temp directory
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    
    # Track installation start time
    local install_start=$(date +%s)
    local components_installed=0
    
    # Download and install CLI bundle
    if [ "$INSTALL_CLI" = "yes" ]; then
        local bundle_name=$(download_cli_bundle "$os" "$arch" "$VERSION" "$gpu_variant" "$cpu_features" "$tmp_dir")
        
        if [ -z "$bundle_name" ]; then
            error "Failed to download CLI bundle"
            exit 1
        fi
        
        local bundle_dir="$tmp_dir/$bundle_name"
        
        install_cli "$bundle_dir" "$os"
        install_webui "$bundle_dir"
        components_installed=$((components_installed + 1))
        
        if [ "$INSTALL_AUDIO" = "yes" ]; then
            install_audio "$bundle_dir" "$is_interactive"
            components_installed=$((components_installed + 1))
        fi
    fi
    
    # Download and install Desktop app
    if [ "$INSTALL_DESKTOP" = "yes" ]; then
        local app_path=$(download_desktop_app "$os" "$arch" "$VERSION" "$tmp_dir")
        install_desktop "$app_path" "$os"
        components_installed=$((components_installed + 1))
    fi
    
    # Calculate total time
    local install_end=$(date +%s)
    local install_time=$((install_end - install_start))
    
    # Success message
    echo "" >&2
    ok "Installation complete (${install_time}s)"
    
    # Ask about auto-start service (only for CLI installs)
    if [ "$INSTALL_CLI" = "yes" ] && [ "$is_interactive" = "yes" ]; then
        setup_autostart "$os"
    fi
    
    if [ "$INSTALL_CLI" = "yes" ]; then
        section "Next Steps"
        printf "    ${CYAN}\$${NC} offgrid init          ${DIM}# Download your first model${NC}\n" >&2
        printf "    ${CYAN}\$${NC} offgrid serve         ${DIM}# Start the server${NC}\n" >&2
        printf "    ${CYAN}\$${NC} offgrid run <model>   ${DIM}# Interactive chat${NC}\n" >&2
        echo "" >&2
        printf "    ${DIM}Web UI${NC}  http://localhost:11611/ui\n" >&2
    fi
    
    if [ "$INSTALL_DESKTOP" = "yes" ]; then
        echo "" >&2
        case "$os" in
            linux) dim "Desktop: Run 'offgrid-desktop' or find in app menu" ;;
            darwin) dim "Desktop: Open 'OffGrid LLM' from Applications" ;;
            windows) dim "Desktop: Run the installer saved to Desktop" ;;
        esac
    fi
    
    if [ "$INSTALL_AUDIO" = "yes" ]; then
        echo "" >&2
        dim "Voice: Run 'offgrid audio setup' to download models"
    fi
    
    echo "" >&2
}

# Run main
main "$@"
