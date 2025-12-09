#!/bin/bash
# OffGrid LLM - Universal Installer
# One command to install everything: CLI, Desktop, Audio (Whisper/Piper)
#
# Usage:
#   curl -fsSL https://offgrid.dev/install | bash
#   curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
#
# Options (environment variables):
#   CLI=yes|no         Install CLI tools (default: yes)
#   DESKTOP=yes|no     Install Desktop app (default: yes)
#   AUDIO=yes|no       Install Audio - Whisper STT + Piper TTS (default: yes)
#   AUTOSTART=yes|no   Auto-start service (default: ask)
#   NONINTERACTIVE=yes Skip all prompts, use defaults (installs everything)

set -e

# ═══════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════
REPO="takuphilchan/offgrid-llm"
GITHUB_URL="https://github.com/${REPO}"
INSTALL_DIR="/usr/local/bin"
VERSION="${VERSION:-latest}"

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# ═══════════════════════════════════════════════════════════════
# Helper Functions
# ═══════════════════════════════════════════════════════════════
log_info() { echo -e "${CYAN}▶${NC} $1" >&2; }
log_success() { echo -e "${GREEN}✓${NC} $1" >&2; }
log_error() { echo -e "${RED}✗${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}⚠${NC} $1" >&2; }
log_dim() { echo -e "${DIM}  $1${NC}" >&2; }

print_banner() {
    echo "" >&2
    echo -e "${CYAN}${BOLD}" >&2
    cat << "EOF" >&2
    ╔═══════════════════════════════════════════════════════════╗
    ║                                                           ║
    ║     ██████╗ ███████╗███████╗ ██████╗ ██████╗ ██╗██████╗  ║
    ║    ██╔═══██╗██╔════╝██╔════╝██╔════╝ ██╔══██╗██║██╔══██╗ ║
    ║    ██║   ██║█████╗  █████╗  ██║  ███╗██████╔╝██║██║  ██║ ║
    ║    ██║   ██║██╔══╝  ██╔══╝  ██║   ██║██╔══██╗██║██║  ██║ ║
    ║    ╚██████╔╝██║     ██║     ╚██████╔╝██║  ██║██║██████╔╝ ║
    ║     ╚═════╝ ╚═╝     ╚═╝      ╚═════╝ ╚═╝  ╚═╝╚═╝╚═════╝  ║
    ║                                                           ║
    ║              Universal Installer                          ║
    ║                                                           ║
    ╚═══════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}" >&2
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
        *) log_error "Unsupported operating system: $os"; exit 1 ;;
    esac
}

detect_arch() {
    local arch
    arch="$(uname -m)"
    
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) log_error "Unsupported architecture: $arch"; exit 1 ;;
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
    
    if [ "$os" = "linux" ]; then
        if command -v vulkaninfo >/dev/null 2>&1 && vulkaninfo --summary >/dev/null 2>&1; then
            variant="vulkan"
        elif command -v nvidia-smi >/dev/null 2>&1; then
            variant="vulkan"
        elif [ -d "/sys/class/drm" ] && ls /sys/class/drm/card*/device/vendor 2>/dev/null | xargs cat 2>/dev/null | grep -q "0x1002"; then
            variant="vulkan"
        fi
    elif [ "$os" = "darwin" ]; then
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
        log_error "Failed to fetch latest version"
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
    
    echo "" >&2
    echo -e "${BOLD}What would you like to install?${NC}" >&2
    echo "" >&2
    echo -e "  ${GREEN}1)${NC} ${BOLD}Full Installation${NC} ${GREEN}(Recommended)${NC}" >&2
    echo -e "     ${DIM}CLI + Desktop App + Voice Assistant (Speech-to-Text & Text-to-Speech)${NC}" >&2
    echo "" >&2
    echo -e "  ${GREEN}2)${NC} ${BOLD}CLI + Voice${NC}" >&2
    echo -e "     ${DIM}Command-line with Voice Assistant support${NC}" >&2
    echo "" >&2
    echo -e "  ${GREEN}3)${NC} ${BOLD}CLI Only${NC}" >&2
    echo -e "     ${DIM}Minimal installation (no voice features)${NC}" >&2
    echo "" >&2
    echo -e "  ${GREEN}4)${NC} ${BOLD}Custom${NC}" >&2
    echo -e "     ${DIM}Choose individual components${NC}" >&2
    echo "" >&2
    
    read -p "Enter your choice [1-4] (default: 1): " choice
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
            custom_menu
            ;;
        *)
            INSTALL_CLI="yes"
            INSTALL_DESKTOP="yes"
            INSTALL_AUDIO="yes"
            ;;
    esac
}

custom_menu() {
    echo "" >&2
    echo -e "${BOLD}Custom Installation${NC}" >&2
    echo "" >&2
    
    # CLI (always yes for custom, needed for everything)
    INSTALL_CLI="yes"
    log_info "CLI tools will be installed (required)"
    
    # Desktop
    read -p "Install Desktop app? [Y/n]: " desktop_choice
    desktop_choice="${desktop_choice:-Y}"
    if [[ "$desktop_choice" =~ ^[Yy] ]]; then
        INSTALL_DESKTOP="yes"
    else
        INSTALL_DESKTOP="no"
    fi
    
    # Audio (Voice Assistant)
    read -p "Install Voice Assistant (Whisper STT + Piper TTS)? [Y/n]: " audio_choice
    audio_choice="${audio_choice:-Y}"
    if [[ "$audio_choice" =~ ^[Yy] ]]; then
        INSTALL_AUDIO="yes"
    else
        INSTALL_AUDIO="no"
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
    
    log_info "Downloading CLI bundle: ${bundle_name}${ext}"
    
    if ! curl -fsSL -o "${tmp_dir}/bundle${ext}" "$download_url" 2>/dev/null; then
        # Fallback to CPU variant
        if [ "$variant" != "cpu" ]; then
            log_warn "GPU variant not available, trying CPU..."
            variant="cpu"
            bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
            download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
            
            if ! curl -fsSL -o "${tmp_dir}/bundle${ext}" "$download_url" 2>/dev/null; then
                # Fallback to AVX2
                if [ "$cpu_features" = "avx512" ]; then
                    log_warn "Trying AVX2 version..."
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
    
    # Extract
    cd "$tmp_dir"
    if [ "$os" = "windows" ]; then
        unzip -q "bundle${ext}"
    else
        tar -xzf "bundle${ext}"
    fi
    
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
                app_name="OffGrid.LLM.Desktop-${version_num}-arm64.dmg"
            else
                app_name="OffGrid.LLM.Desktop-${version_num}-x64.dmg"
            fi
            download_url="${GITHUB_URL}/releases/download/${version}/${app_name}"
            ;;
        windows)
            app_name="OffGrid.LLM.Desktop-Setup-${version_num}.exe"
            download_url="${GITHUB_URL}/releases/download/${version}/${app_name}"
            ;;
    esac
    
    log_info "Downloading Desktop app: ${app_name}"
    
    if curl -fsSL -o "${tmp_dir}/${app_name}" "$download_url" 2>/dev/null; then
        echo "${tmp_dir}/${app_name}"
    else
        log_warn "Desktop app not available for this platform"
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
        log_warn "Missing build tools:$missing_tools"
        log_info "Installing build dependencies..."
        
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
            log_error "Cannot install build tools automatically. Please install: git cmake g++ make"
            return 1
        fi
    fi
    
    # Clone and build
    rm -rf "$build_dir"
    mkdir -p "$build_dir"
    
    log_dim "Cloning whisper.cpp..."
    if ! git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git "$build_dir/whisper.cpp" 2>/dev/null; then
        log_error "Failed to clone whisper.cpp"
        rm -rf "$build_dir"
        return 1
    fi
    
    cd "$build_dir/whisper.cpp"
    
    log_dim "Building whisper.cpp (this may take a few minutes)..."
    # Build with static linking to avoid LD_LIBRARY_PATH issues
    if ! cmake -B build -DCMAKE_BUILD_TYPE=Release -DGGML_CCACHE=OFF -DBUILD_SHARED_LIBS=OFF >/dev/null 2>&1; then
        log_error "CMake configuration failed"
        cd - >/dev/null
        rm -rf "$build_dir"
        return 1
    fi
    
    local num_cores
    num_cores=$(nproc 2>/dev/null || echo 4)
    
    if ! cmake --build build --config Release -j"$num_cores" >/dev/null 2>&1; then
        log_error "Build failed"
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
        log_error "whisper-cli binary not found after build"
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
    
    log_info "Installing CLI tools..."
    
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"
    
    # Determine if we need sudo
    local use_sudo=""
    if [ "$os" != "windows" ] && [ ! -w "$INSTALL_DIR" ]; then
        use_sudo="sudo"
    fi
    
    # Stop running processes
    if [ "$os" != "windows" ]; then
        pkill -x offgrid 2>/dev/null || true
        pkill -x llama-server 2>/dev/null || true
        sleep 1
    fi
    
    # Copy binaries
    $use_sudo cp "$bundle_dir/offgrid${ext}" "$INSTALL_DIR/"
    $use_sudo cp "$bundle_dir/llama-server${ext}" "$INSTALL_DIR/"
    $use_sudo chmod +x "$INSTALL_DIR/offgrid${ext}" "$INSTALL_DIR/llama-server${ext}"
    
    log_success "CLI installed to $INSTALL_DIR"
}

install_audio() {
    local bundle_dir="$1"
    local interactive="$2"
    
    log_info "Installing Audio components..."
    
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
            echo "" >&2
            log_warn "GLIBC Compatibility Notice"
            echo -e "${DIM}  Your system has GLIBC $glibc_version${NC}" >&2
            echo -e "${DIM}  Pre-built audio binaries require GLIBC 2.38+${NC}" >&2
            echo "" >&2
            
            if [ "$interactive" = "yes" ]; then
                echo -e "${BOLD}Audio Installation Options:${NC}" >&2
                echo "  1) Build from source  (Recommended - works on any system, takes ~5 min)" >&2
                echo "  2) Skip audio         (Install CLI only, no voice features)" >&2
                echo "" >&2
                
                local audio_choice
                read -r -p "Enter choice [1-2] (default: 1): " audio_choice </dev/tty
                audio_choice="${audio_choice:-1}"
                
                case "$audio_choice" in
                    2)
                        log_info "Skipping audio installation"
                        log_dim "You can install audio later with: offgrid audio setup"
                        return 0
                        ;;
                    *)
                        # Continue with build from source
                        ;;
                esac
            else
                log_info "Non-interactive mode: Building audio from source"
            fi
        else
            log_dim "GLIBC $glibc_version detected - compatible with pre-built binaries"
        fi
    fi
    
    # Install Whisper (Speech-to-Text)
    local whisper_installed=false
    
    if [ "$os" = "Linux" ]; then
        if [[ "$glibc_status" == "compatible" ]] && [ -d "$bundle_dir/audio/whisper" ]; then
            # Use pre-built binaries
            log_info "Installing pre-built Whisper..."
            cp -r "$bundle_dir/audio/whisper/"* "$AUDIO_DIR/whisper/" 2>/dev/null || true
            chmod +x "$AUDIO_DIR/whisper/"* 2>/dev/null || true
            
            # Test if it works
            if "$AUDIO_DIR/whisper/whisper-cli" --help >/dev/null 2>&1; then
                whisper_installed=true
                log_success "Whisper (Speech-to-Text) installed"
            else
                log_warn "Pre-built Whisper failed, building from source..."
                rm -rf "$AUDIO_DIR/whisper"/*
            fi
        fi
        
        if [ "$whisper_installed" = "false" ]; then
            # Build from source
            log_info "Building Whisper from source (this may take a few minutes)..."
            if build_whisper_from_source "$AUDIO_DIR/whisper"; then
                whisper_installed=true
                log_success "Whisper (Speech-to-Text) built and installed"
            else
                log_warn "Whisper build failed - voice input will not be available"
                log_dim "You can try later with: offgrid audio setup whisper"
            fi
        fi
    elif [ -d "$bundle_dir/audio/whisper" ]; then
        # macOS/Windows: use pre-built binaries
        cp -r "$bundle_dir/audio/whisper/"* "$AUDIO_DIR/whisper/" 2>/dev/null || true
        chmod +x "$AUDIO_DIR/whisper/"* 2>/dev/null || true
        whisper_installed=true
        log_success "Whisper (Speech-to-Text) installed"
    else
        log_warn "Whisper binaries not in bundle - will build on first use"
    fi
    
    # Install Piper (Text-to-Speech)
    local piper_installed=false
    
    if [ "$os" = "Linux" ]; then
        log_info "Downloading Piper from official releases..."
        if download_piper "$AUDIO_DIR/piper"; then
            # Test if it works
            if LD_LIBRARY_PATH="$AUDIO_DIR/piper:$LD_LIBRARY_PATH" "$AUDIO_DIR/piper/piper" --help >/dev/null 2>&1; then
                piper_installed=true
                log_success "Piper (Text-to-Speech) installed"
            else
                log_warn "Piper binary not compatible with your system"
                log_dim "Text-to-speech will not be available"
                rm -rf "$AUDIO_DIR/piper"/*
            fi
        else
            log_warn "Piper download failed - voice output will not be available"
        fi
    elif [ -d "$bundle_dir/audio/piper" ]; then
        cp -r "$bundle_dir/audio/piper/"* "$AUDIO_DIR/piper/" 2>/dev/null || true
        chmod +x "$AUDIO_DIR/piper/"* 2>/dev/null || true
        piper_installed=true
        log_success "Piper (Text-to-Speech) installed"
    else
        log_warn "Piper binaries not in bundle - will download on first use"
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
            log_warn "Unsupported architecture for Piper: $arch"
            return 1
            ;;
    esac
    
    local tmp_dir="/tmp/piper-download-$$"
    mkdir -p "$tmp_dir"
    
    log_dim "Downloading Piper..."
    if ! curl -fsSL -o "$tmp_dir/piper.tar.gz" "$piper_url" 2>/dev/null; then
        rm -rf "$tmp_dir"
        return 1
    fi
    
    log_dim "Extracting Piper..."
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
    
    log_info "Installing Web UI..."
    
    local WEB_DIR="/var/lib/offgrid/web/ui"
    
    # Determine if we need sudo
    local use_sudo=""
    if [ ! -w "/var/lib" ] 2>/dev/null; then
        use_sudo="sudo"
    fi
    
    $use_sudo mkdir -p "$WEB_DIR"
    
    if [ -d "$bundle_dir/web/ui" ]; then
        $use_sudo cp -r "$bundle_dir/web/ui/"* "$WEB_DIR/"
        log_success "Web UI installed"
    else
        # Download from GitHub
        log_info "Downloading Web UI from GitHub..."
        local ui_tmp="/tmp/offgrid-ui-$$"
        mkdir -p "$ui_tmp"
        
        if curl -fsSL "${GITHUB_URL}/archive/refs/heads/main.tar.gz" | tar -xz -C "$ui_tmp" --strip-components=2 "offgrid-llm-main/web/ui" 2>/dev/null; then
            $use_sudo cp -r "$ui_tmp/"* "$WEB_DIR/"
            rm -rf "$ui_tmp"
            log_success "Web UI installed"
        else
            rm -rf "$ui_tmp"
            log_warn "Web UI download failed"
        fi
    fi
}

install_desktop() {
    local app_path="$1"
    local os="$2"
    
    if [ -z "$app_path" ] || [ ! -f "$app_path" ]; then
        log_warn "Desktop app not available"
        return
    fi
    
    log_info "Installing Desktop app..."
    
    case "$os" in
        linux)
            # Install AppImage - always extract to avoid FUSE requirement
            # FUSE is often not available, especially on WSL, containers, minimal installs
            local app_dir="$HOME/.local/share/offgrid-desktop"
            local bin_dir="$HOME/.local/bin"
            
            # Clean previous installation
            rm -rf "$app_dir"
            mkdir -p "$app_dir" "$bin_dir"
            
            log_dim "Extracting AppImage..."
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
                log_dim "Extraction failed, installing AppImage directly (requires FUSE)..."
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
            log_success "Desktop app installed to $bin_dir"
            ;;
        darwin)
            # Mount DMG and copy app
            local mount_point="/Volumes/OffGrid-LLM"
            hdiutil attach "$app_path" -quiet
            cp -R "${mount_point}/OffGrid LLM.app" /Applications/
            hdiutil detach "$mount_point" -quiet
            log_success "Desktop app installed to /Applications"
            ;;
        windows)
            # Just save the installer
            local app_dir="$HOME/Desktop"
            cp "$app_path" "$app_dir/"
            log_success "Desktop installer saved to $app_dir"
            log_info "Run the installer to complete Desktop installation"
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
    local gpu_variant=$(detect_gpu "$os")
    
    log_info "System detected: $os-$arch ($gpu_variant, $cpu_features)"
    
    # Get version
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(get_latest_version)
    fi
    log_info "Version: $VERSION"
    
    # Check if running interactively (stdin is a terminal)
    local is_interactive="no"
    if [ -t 0 ] && [ "${NONINTERACTIVE:-}" != "yes" ]; then
        is_interactive="yes"
    fi
    
    # Show menu if interactive
    if [ "$is_interactive" = "yes" ] && [ -z "${CLI:-}" ] && [ -z "${DESKTOP:-}" ] && [ -z "${AUDIO:-}" ]; then
        show_menu "$os"
    else
        # Use defaults or environment variables
        # Default: Full installation (CLI + Desktop + Audio)
        INSTALL_CLI="${CLI:-yes}"
        INSTALL_DESKTOP="${DESKTOP:-yes}"
        INSTALL_AUDIO="${AUDIO:-yes}"
        
        if [ "$is_interactive" != "yes" ]; then
            log_info "Non-interactive mode: Installing full system (CLI + Desktop + Audio)"
        fi
    fi
    
    # Summary
    echo "" >&2
    echo -e "${BOLD}Installation Summary:${NC}" >&2
    echo -e "  CLI Tools:    ${INSTALL_CLI}" >&2
    echo -e "  Desktop App:  ${INSTALL_DESKTOP}" >&2
    echo -e "  Audio (STT/TTS): ${INSTALL_AUDIO}" >&2
    echo "" >&2
    
    if [ "$is_interactive" = "yes" ]; then
        read -p "Proceed with installation? [Y/n]: " confirm
        confirm="${confirm:-Y}"
        if [[ ! "$confirm" =~ ^[Yy] ]]; then
            log_info "Installation cancelled"
            exit 0
        fi
    fi
    
    # Create temp directory
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    
    # Download and install CLI bundle
    if [ "$INSTALL_CLI" = "yes" ]; then
        local bundle_name=$(download_cli_bundle "$os" "$arch" "$VERSION" "$gpu_variant" "$cpu_features" "$tmp_dir")
        
        if [ -z "$bundle_name" ]; then
            log_error "Failed to download CLI bundle"
            exit 1
        fi
        
        local bundle_dir="$tmp_dir/$bundle_name"
        
        install_cli "$bundle_dir" "$os"
        install_webui "$bundle_dir"
        
        if [ "$INSTALL_AUDIO" = "yes" ]; then
            install_audio "$bundle_dir" "$is_interactive"
        fi
    fi
    
    # Download and install Desktop app
    if [ "$INSTALL_DESKTOP" = "yes" ]; then
        local app_path=$(download_desktop_app "$os" "$arch" "$VERSION" "$tmp_dir")
        install_desktop "$app_path" "$os"
    fi
    
    # Success message
    echo ""
    echo -e "${GREEN}${BOLD}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}${BOLD}║           Installation Complete!                          ║${NC}"
    echo -e "${GREEN}${BOLD}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    if [ "$INSTALL_CLI" = "yes" ]; then
        echo -e "${BOLD}Get Started:${NC}"
        echo ""
        echo -e "  ${CYAN}offgrid --version${NC}          Check installation"
        echo -e "  ${CYAN}offgrid serve${NC}              Start server with Web UI"
        echo -e "  ${CYAN}offgrid search llama${NC}       Search for models"
        echo ""
        echo -e "${BOLD}Web UI:${NC} http://localhost:11611/ui"
        echo ""
    fi
    
    if [ "$INSTALL_DESKTOP" = "yes" ]; then
        echo -e "${BOLD}Desktop App:${NC}"
        case "$os" in
            linux) echo -e "  Run: ${CYAN}offgrid-desktop${NC} or find in app menu" ;;
            darwin) echo -e "  Open: ${CYAN}OffGrid LLM${NC} from Applications" ;;
            windows) echo -e "  Run the installer on your Desktop" ;;
        esac
        echo ""
    fi
    
    if [ "$INSTALL_AUDIO" = "yes" ]; then
        echo -e "${BOLD}Audio Features:${NC}"
        echo -e "  Speech-to-Text: Whisper.cpp installed"
        echo -e "  Text-to-Speech: Piper installed"
        echo -e "  ${DIM}Download voice models: offgrid audio setup whisper --model base.en${NC}"
        echo ""
    fi
    
    echo -e "${BOLD}Documentation:${NC} https://github.com/takuphilchan/offgrid-llm"
    echo ""
}

# Run main
main "$@"
