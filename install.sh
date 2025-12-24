#!/bin/bash
# OffGrid LLM - Universal Installer
# One command to install everything: CLI, Desktop, Voice
#
# Usage:
#   curl -fsSL https://offgrid.run/install.sh | bash
#
# Options (environment variables):
#   NONINTERACTIVE=yes   Skip all prompts, use defaults
#   VERSION=v0.x.x       Install specific version (default: latest)

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

# Progress spinner
SPINNER=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
SPIN_IDX=0

# Helper Functions
ok()      { printf "  ${GREEN}✓${NC} %s\n" "$1" >&2; }
error()   { printf "  ${RED}✗${NC} %s\n" "$1" >&2; }
warn()    { printf "  ${YELLOW}○${NC} %s\n" "$1" >&2; }
info()    { printf "  ${CYAN}→${NC} %s\n" "$1" >&2; }
dim()     { printf "    ${DIM}%s${NC}\n" "$1" >&2; }
section() { printf "\n  ${BOLD}%s${NC}\n" "$1" >&2; }

spin() {
    printf "\r  ${CYAN}${SPINNER[$SPIN_IDX]}${NC} %s" "$1" >&2
    SPIN_IDX=$(( (SPIN_IDX + 1) % ${#SPINNER[@]} ))
}

spin_done() {
    printf "\r  ${GREEN}✓${NC} %s\n" "$1" >&2
}

spin_fail() {
    printf "\r  ${RED}✗${NC} %s\n" "$1" >&2
}

print_banner() {
    echo "" >&2
    printf "  ${CYAN}◈${NC} ${BOLD}OffGrid LLM${NC}\n" >&2
    printf "  ${DIM}Run AI locally, completely offline${NC}\n" >&2
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
        *) error "Unsupported OS: $os"; exit 1 ;;
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
    if [ "$(uname -s)" = "Linux" ]; then
        if grep -q "avx512" /proc/cpuinfo 2>/dev/null; then
            echo "avx512"
        elif grep -q "avx2" /proc/cpuinfo 2>/dev/null; then
            echo "avx2"
        else
            echo "basic"
        fi
    elif [ "$(uname -s)" = "Darwin" ]; then
        if sysctl machdep.cpu.features machdep.cpu.leaf7_features 2>/dev/null | grep -qi "avx512"; then
            echo "avx512"
        else
            echo "avx2"
        fi
    else
        echo "avx2"
    fi
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

# ═══════════════════════════════════════════════════════════════
# Download Functions
# ═══════════════════════════════════════════════════════════════
download_with_progress() {
    local url="$1"
    local output="$2"
    local label="$3"
    
    # Start background download
    curl -fsSL -o "$output" "$url" 2>/dev/null &
    local pid=$!
    
    # Show spinner while downloading
    while kill -0 $pid 2>/dev/null; do
        spin "$label"
        sleep 0.1
    done
    
    # Check result
    wait $pid
    return $?
}

download_cli_bundle() {
    local os="$1"
    local arch="$2"
    local version="$3"
    local cpu_features="$4"
    local tmp_dir="$5"
    
    local variant="cpu"
    local bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
    local ext=".tar.gz"
    [ "$os" = "windows" ] && ext=".zip"
    
    local download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
    
    if download_with_progress "$download_url" "${tmp_dir}/bundle${ext}" "Downloading OffGrid..."; then
        spin_done "Downloaded"
    else
        # Fallback to AVX2
        if [ "$cpu_features" = "avx512" ]; then
            cpu_features="avx2"
            bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
            download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
            
            if download_with_progress "$download_url" "${tmp_dir}/bundle${ext}" "Trying AVX2 build..."; then
                spin_done "Downloaded (AVX2)"
            else
                spin_fail "Download failed"
                return 1
            fi
        else
            spin_fail "Download failed"
            return 1
        fi
    fi
    
    # Extract
    spin "Extracting..."
    cd "$tmp_dir"
    if [ "$os" = "windows" ]; then
        unzip -q "bundle${ext}" 2>/dev/null
    else
        tar -xzf "bundle${ext}" 2>/dev/null
    fi
    spin_done "Extracted"
    cd - >/dev/null
    
    echo "$bundle_name"
}

download_desktop_app() {
    local os="$1"
    local arch="$2"
    local version="$3"
    local tmp_dir="$4"
    
    local app_name=""
    local version_num="${version#v}"
    
    case "$os" in
        linux)
            [ "$arch" = "amd64" ] && app_name="OffGrid.LLM.Desktop-${version_num}-x86_64.AppImage"
            [ "$arch" = "arm64" ] && app_name="OffGrid.LLM.Desktop-${version_num}-arm64.AppImage"
            ;;
        darwin)
            [ "$arch" = "arm64" ] && app_name="OffGrid.LLM.Desktop-${version_num}-arm64.zip"
            [ "$arch" = "amd64" ] && app_name="OffGrid.LLM.Desktop-${version_num}-x64.zip"
            ;;
        windows)
            app_name="OffGrid.LLM.Desktop-Setup-${version_num}.exe"
            ;;
    esac
    
    [ -z "$app_name" ] && return 0
    
    local download_url="${GITHUB_URL}/releases/download/${version}/${app_name}"
    
    if download_with_progress "$download_url" "${tmp_dir}/${app_name}" "Downloading Desktop app..."; then
        spin_done "Desktop app ready"
        echo "${tmp_dir}/${app_name}"
    else
        # Desktop is optional, don't fail
        dim "Desktop app not available for this platform"
        echo ""
    fi
}

# ═══════════════════════════════════════════════════════════════
# Installation Functions
# ═══════════════════════════════════════════════════════════════
stop_running_processes() {
    pkill -TERM -f "offgrid serve" 2>/dev/null || true
    pkill -TERM -x "llama-server" 2>/dev/null || true
    sleep 0.5
    pkill -9 -f "offgrid" 2>/dev/null || true
    pkill -9 -x "llama-server" 2>/dev/null || true
}

install_cli() {
    local bundle_dir="$1"
    local os="$2"
    
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"
    
    local use_sudo=""
    [ "$os" != "windows" ] && [ ! -w "$INSTALL_DIR" ] && use_sudo="sudo"
    
    stop_running_processes
    
    spin "Installing CLI..."
    
    if $use_sudo cp -f "$bundle_dir/offgrid${ext}" "$INSTALL_DIR/" 2>/dev/null && \
       $use_sudo cp -f "$bundle_dir/llama-server${ext}" "$INSTALL_DIR/" 2>/dev/null; then
        $use_sudo chmod +x "$INSTALL_DIR/offgrid${ext}" "$INSTALL_DIR/llama-server${ext}"
        spin_done "CLI installed → $INSTALL_DIR/offgrid"
        return 0
    else
        spin_fail "Failed to install CLI"
        return 1
    fi
}

install_webui() {
    local bundle_dir="$1"
    
    local WEB_DIR="/var/lib/offgrid/web/ui"
    local use_sudo=""
    [ ! -w "/var/lib" ] 2>/dev/null && use_sudo="sudo"
    
    if [ -d "$bundle_dir/web/ui" ]; then
        spin "Installing Web UI..."
        $use_sudo mkdir -p "$WEB_DIR" 2>/dev/null
        $use_sudo cp -r "$bundle_dir/web/ui/"* "$WEB_DIR/" 2>/dev/null
        spin_done "Web UI installed"
    fi
}

install_desktop() {
    local app_path="$1"
    local os="$2"
    
    [ -z "$app_path" ] || [ ! -f "$app_path" ] && return 0
    
    spin "Installing Desktop app..."
    
    case "$os" in
        linux)
            local app_dir="$HOME/.local/share/offgrid-desktop"
            local bin_dir="$HOME/.local/bin"
            
            rm -rf "$app_dir" 2>/dev/null
            mkdir -p "$app_dir" "$bin_dir"
            
            chmod +x "$app_path"
            cd "$app_dir"
            "$app_path" --appimage-extract >/dev/null 2>&1 || true
            cd - >/dev/null
            
            if [ -d "$app_dir/squashfs-root" ]; then
                cat > "$bin_dir/offgrid-desktop" << 'LAUNCHER'
#!/bin/bash
cd "$HOME/.local/share/offgrid-desktop/squashfs-root"
exec ./AppRun "$@"
LAUNCHER
                chmod +x "$bin_dir/offgrid-desktop"
                
                mkdir -p "$HOME/.local/share/applications"
                cat > "$HOME/.local/share/applications/offgrid-llm.desktop" << EOF
[Desktop Entry]
Name=OffGrid LLM
Comment=Local AI Assistant
Exec=$bin_dir/offgrid-desktop
Type=Application
Categories=Utility;Development;
Terminal=false
EOF
                spin_done "Desktop app installed"
            else
                spin_fail "Desktop extraction failed"
            fi
            ;;
        darwin)
            if [ "${app_path##*.}" = "zip" ]; then
                local tmp_extract="/tmp/offgrid-app-$$"
                mkdir -p "$tmp_extract"
                unzip -q "$app_path" -d "$tmp_extract"
                cp -R "$tmp_extract/"*.app /Applications/ 2>/dev/null
                rm -rf "$tmp_extract"
                spin_done "Desktop app installed → /Applications"
            fi
            ;;
        windows)
            cp "$app_path" "$USERPROFILE/Desktop/" 2>/dev/null || cp "$app_path" "$HOME/Desktop/" 2>/dev/null
            spin_done "Installer saved to Desktop"
            ;;
    esac
}

install_audio() {
    local bundle_dir="$1"
    
    local AUDIO_DIR="$HOME/.offgrid-llm/audio"
    mkdir -p "$AUDIO_DIR/whisper" "$AUDIO_DIR/piper"
    
    if [ -d "$bundle_dir/audio/whisper" ]; then
        spin "Setting up Whisper (Speech-to-Text)..."
        cp -r "$bundle_dir/audio/whisper/"* "$AUDIO_DIR/whisper/" 2>/dev/null || true
        chmod +x "$AUDIO_DIR/whisper/"* 2>/dev/null || true
        spin_done "Whisper ready"
    fi
    
    if [ -d "$bundle_dir/audio/piper" ]; then
        spin "Setting up Piper (Text-to-Speech)..."
        cp -r "$bundle_dir/audio/piper/"* "$AUDIO_DIR/piper/" 2>/dev/null || true
        chmod +x "$AUDIO_DIR/piper/"* 2>/dev/null || true
        spin_done "Piper ready"
    elif [ "$(uname -s)" = "Linux" ]; then
        download_piper "$AUDIO_DIR/piper"
    fi
}

download_piper() {
    local install_dir="$1"
    local arch=$(uname -m)
    local piper_url=""
    
    case "$arch" in
        x86_64|amd64) piper_url="https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz" ;;
        aarch64|arm64) piper_url="https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_aarch64.tar.gz" ;;
        *) return 1 ;;
    esac
    
    local tmp_dir="/tmp/piper-$$"
    mkdir -p "$tmp_dir"
    
    if download_with_progress "$piper_url" "$tmp_dir/piper.tar.gz" "Downloading Piper (TTS)..."; then
        tar -xzf "$tmp_dir/piper.tar.gz" -C "$tmp_dir" 2>/dev/null
        rm -rf "$install_dir"
        mkdir -p "$install_dir"
        [ -d "$tmp_dir/piper" ] && cp -r "$tmp_dir/piper/"* "$install_dir/"
        chmod +x "$install_dir/piper" 2>/dev/null || true
        spin_done "Piper ready"
    fi
    
    rm -rf "$tmp_dir"
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
    
    section "System"
    dim "$os / $arch / $cpu_features"
    
    # Get version
    echo "" >&2
    spin "Checking latest version..."
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(get_latest_version)
    fi
    spin_done "Version $VERSION"
    
    # Create temp directory
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    
    local install_start=$(date +%s)
    
    section "Downloading"
    
    # Download CLI bundle
    local bundle_name=$(download_cli_bundle "$os" "$arch" "$VERSION" "$cpu_features" "$tmp_dir")
    
    if [ -z "$bundle_name" ]; then
        error "Failed to download"
        exit 1
    fi
    
    local bundle_dir="$tmp_dir/$bundle_name"
    
    # Download Desktop app
    local app_path=$(download_desktop_app "$os" "$arch" "$VERSION" "$tmp_dir")
    
    section "Installing"
    
    install_cli "$bundle_dir" "$os"
    install_webui "$bundle_dir"
    install_audio "$bundle_dir"
    install_desktop "$app_path" "$os"
    
    # Done
    local install_end=$(date +%s)
    local install_time=$((install_end - install_start))
    
    echo "" >&2
    ok "Installation complete! (${install_time}s)"
    
    section "Get Started"
    echo "" >&2
    printf "    ${CYAN}\$${NC} offgrid init          ${DIM}# Download your first model${NC}\n" >&2
    printf "    ${CYAN}\$${NC} offgrid serve         ${DIM}# Start the server${NC}\n" >&2
    printf "    ${CYAN}\$${NC} offgrid run <model>   ${DIM}# Chat with a model${NC}\n" >&2
    echo "" >&2
    printf "    ${DIM}Web UI:${NC}  http://localhost:11611/ui\n" >&2
    
    if [ -n "$app_path" ] && [ -f "$app_path" ]; then
        case "$os" in
            linux) printf "    ${DIM}Desktop:${NC} offgrid-desktop\n" >&2 ;;
            darwin) printf "    ${DIM}Desktop:${NC} Open 'OffGrid LLM' from Applications\n" >&2 ;;
        esac
    fi
    
    echo "" >&2
}

# Run main
main "$@"
