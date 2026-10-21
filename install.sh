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
LEGACY_INSTALL="${OFFGRID_INSTALL_LEGACY:-no}"
# The root URL keeps its existing no-service default. The published legacy URL
# supplies its former default through OFFGRID_INSTALL_LEGACY.
if [ "$LEGACY_INSTALL" = "yes" ]; then
    AUTOSTART="${AUTOSTART:-ask}"
else
    AUTOSTART="${AUTOSTART:-none}"
fi

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
    printf "\r\033[K  ${CYAN}${SPINNER[$SPIN_IDX]}${NC} %s" "$1" >&2
    SPIN_IDX=$(( (SPIN_IDX + 1) % ${#SPINNER[@]} ))
}

spin_done() {
    printf "\r\033[K  ${GREEN}✓${NC} %s\n" "$1" >&2
}

spin_fail() {
    printf "\r\033[K  ${RED}✗${NC} %s\n" "$1" >&2
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

detect_gpu_variant() {
    local os="$1"
    if [ "$os" = "linux" ]; then
        if command -v vulkaninfo >/dev/null 2>&1 && vulkaninfo --summary >/dev/null 2>&1; then
            echo "vulkan"
        elif command -v nvidia-smi >/dev/null 2>&1; then
            echo "vulkan"
        elif [ -d /sys/class/drm ] && ls /sys/class/drm/card*/device/vendor 2>/dev/null | xargs cat | grep -q '0x1002'; then
            echo "vulkan"
        else
            echo "cpu"
        fi
    elif [ "$os" = "darwin" ] && [ "$(uname -m)" = "arm64" ]; then
        echo "metal"
    else
        echo "cpu"
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
    local variant="${6:-cpu}"
    
    local bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
    local ext=".tar.gz"
    [ "$os" = "windows" ] && ext=".zip"
    
    local download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
    
    local downloaded="no"
    if download_with_progress "$download_url" "${tmp_dir}/bundle${ext}" "Downloading OffGrid..."; then
        downloaded="yes"
        spin_done "Downloaded OffGrid"
    else
        # Keep the legacy GPU -> CPU fallback before feature fallback.
        if [ "$variant" != "cpu" ]; then
            rm -f "${tmp_dir}/bundle${ext}"
            variant="cpu"
            bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
            download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
            if download_with_progress "$download_url" "${tmp_dir}/bundle${ext}" "Trying CPU build..."; then
                downloaded="yes"
                spin_done "Downloaded (CPU)"
            fi
        fi
        # Fallback to AVX2 for unavailable AVX-512 release assets.
        if [ "$downloaded" = "no" ] && [ "$cpu_features" = "avx512" ]; then
            rm -f "${tmp_dir}/bundle${ext}"
            variant="cpu"
            cpu_features="avx2"
            bundle_name="offgrid-${version}-${os}-${arch}-${variant}-${cpu_features}"
            download_url="${GITHUB_URL}/releases/download/${version}/${bundle_name}${ext}"
            if download_with_progress "$download_url" "${tmp_dir}/bundle${ext}" "Trying AVX2 build..."; then
                downloaded="yes"
                spin_done "Downloaded (AVX2)"
            fi
        fi
        if [ "$downloaded" = "no" ]; then
            spin_fail "Download failed"
            return 1
        fi
    fi
    
    # Extract
    spin "Extracting..."
    cd "$tmp_dir" || return 1
    if [ "$os" = "windows" ]; then
        if ! unzip -q "bundle${ext}" 2>/dev/null; then
            spin_fail "Extraction failed"
            return 1
        fi
    else
        if ! tar -xzf "bundle${ext}" 2>/dev/null; then
            spin_fail "Extraction failed"
            return 1
        fi
    fi
    spin_done "Extracted bundle"
    cd - >/dev/null
    
    echo "$bundle_name"
}

verify_legacy_checksums() {
    local bundle_dir="$1"
    [ "$LEGACY_INSTALL" = "yes" ] || return 0
    if [ ! -f "$bundle_dir/checksums.sha256" ]; then
        warn "Checksums file not found, skipping verification"
        return 0
    fi
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$bundle_dir" && sha256sum -c checksums.sha256 >/dev/null) || {
            error "Checksum verification failed"
            return 1
        }
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$bundle_dir" && shasum -a 256 -c checksums.sha256 >/dev/null) || {
            error "Checksum verification failed"
            return 1
        }
    else
        error "No SHA-256 checksum verifier is available"
        return 1
    fi
    ok "Checksums verified"
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
        spin_done "Downloaded Desktop app"
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
    if [ "$LEGACY_INSTALL" = "yes" ]; then
        # The published installer targeted exact binary names, not every
        # process whose command line mentions OffGrid (including this wrapper).
        local name i
        for name in llama-server offgrid; do
            if pgrep -x "$name" >/dev/null 2>&1; then
                pkill -TERM -x "$name" 2>/dev/null || true
                for i in {1..10}; do
                    pgrep -x "$name" >/dev/null 2>&1 || break
                    sleep 0.5
                done
                if pgrep -x "$name" >/dev/null 2>&1; then
                    pkill -9 -x "$name" 2>/dev/null || true
                fi
            fi
        done
        return 0
    fi
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

    if [ "$LEGACY_INSTALL" = "yes" ] && command -v systemctl >/dev/null 2>&1 && \
       systemctl is-active --quiet "offgrid@${USER}.service" 2>/dev/null; then
        $use_sudo systemctl stop "offgrid@${USER}.service"
    fi
    
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

setup_legacy_systemd() {
    local os="$1"
    [ "$os" = "linux" ] || return 0
    [ "$AUTOSTART" != "none" ] || return 0
    if ! command -v systemctl >/dev/null 2>&1; then
        warn "systemd not available; start OffGrid manually"
        return 0
    fi

    local use_sudo=""
    [ "$(id -u)" -ne 0 ] && use_sudo="sudo"
    $use_sudo tee /etc/systemd/system/offgrid@.service >/dev/null <<'UNIT' || return 1
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
UNIT
    $use_sudo systemctl daemon-reload || { error "systemd reload failed"; return 1; }
    $use_sudo mkdir -p /etc/offgrid || return 1
    printf '%s\n' 42382 | $use_sudo tee /etc/offgrid/llama-port >/dev/null || return 1

    local enable_service="no"
    local start_service="no"
    case "$AUTOSTART" in
        yes) enable_service="yes"; start_service="yes" ;;
        no) ;;
        ask)
            local reply="Y"
            read -t 30 -p "Enable auto-start on boot? [Y/n] " -n 1 -r reply 2>/dev/null || reply="Y"
            printf '\n' >&2
            [[ "$reply" =~ ^[Nn]$ ]] || enable_service="yes"
            reply="Y"
            read -t 30 -p "Start services now? [Y/n] " -n 1 -r reply 2>/dev/null || reply="Y"
            printf '\n' >&2
            [[ "$reply" =~ ^[Nn]$ ]] || start_service="yes"
            ;;
        *) error "AUTOSTART must be yes, no, or ask"; return 1 ;;
    esac
    if [ "$enable_service" = "yes" ]; then
        $use_sudo systemctl enable "offgrid@${USER}.service" || {
            error "Could not enable offgrid@${USER}.service"
            return 1
        }
    fi
    if [ "$start_service" = "yes" ]; then
        $use_sudo systemctl start "offgrid@${USER}.service" || {
            error "Could not start offgrid@${USER}.service"
            return 1
        }
        ok "Service started: offgrid@${USER}.service"
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
        spin_done "Installed Web UI"
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
                spin_done "Installed Desktop app"
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
                spin_done "Installed Desktop app → /Applications"
            fi
            ;;
        windows)
            cp "$app_path" "$USERPROFILE/Desktop/" 2>/dev/null || cp "$app_path" "$HOME/Desktop/" 2>/dev/null
            spin_done "Saved installer to Desktop"
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
        spin_done "Whisper ready (Speech-to-Text)"
    fi
    
    if [ -d "$bundle_dir/audio/piper" ]; then
        spin "Setting up Piper (Text-to-Speech)..."
        cp -r "$bundle_dir/audio/piper/"* "$AUDIO_DIR/piper/" 2>/dev/null || true
        chmod +x "$AUDIO_DIR/piper/"* 2>/dev/null || true
        spin_done "Piper ready (Text-to-Speech)"
    elif [ "$LEGACY_INSTALL" != "yes" ] && [ "$(uname -s)" = "Linux" ]; then
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
        spin_done "Piper ready (Text-to-Speech)"
    fi
    
    rm -rf "$tmp_dir"
}

# ═══════════════════════════════════════════════════════════════
# Main Installation Flow
# ═══════════════════════════════════════════════════════════════
main() {
    case "$AUTOSTART" in
        none|yes|no|ask) ;;
        *) error "AUTOSTART must be yes, no, or ask"; return 1 ;;
    esac
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
    spin_done "Found version $VERSION"
    
    # Create temp directory
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    
    local install_start=$(date +%s)
    
    section "Downloading"
    
    # Download CLI bundle
    local variant="cpu"
    [ "$LEGACY_INSTALL" = "yes" ] && variant="$(detect_gpu_variant "$os")"
    local bundle_name=$(download_cli_bundle "$os" "$arch" "$VERSION" "$cpu_features" "$tmp_dir" "$variant")
    
    if [ -z "$bundle_name" ]; then
        error "Failed to download"
        exit 1
    fi
    
    local bundle_dir="$tmp_dir/$bundle_name"
    verify_legacy_checksums "$bundle_dir"
    
    # Download Desktop app
    local app_path=""
    if [ "$LEGACY_INSTALL" != "yes" ]; then
        app_path=$(download_desktop_app "$os" "$arch" "$VERSION" "$tmp_dir")
    fi
    
    section "Installing"
    
    install_cli "$bundle_dir" "$os"
    install_webui "$bundle_dir"
    install_audio "$bundle_dir"
    if [ "$LEGACY_INSTALL" != "yes" ]; then
        install_desktop "$app_path" "$os"
    fi
    setup_legacy_systemd "$os"
    
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

# Help is safe for local and streamed compatibility checks.
case "${1:-}" in
    --help|-h)
        printf '%s\n' 'OffGrid installer' \
            'Usage: bash install.sh [--help]' \
            'VERSION=vX.Y.Z selects a release (default: latest).' \
            'AUTOSTART=yes|no|ask controls offgrid@USER.service when selected.' \
            'NONINTERACTIVE=yes is reserved for noninteractive installation.'
        exit 0
        ;;
esac

# Run main
main "$@"
