#!/bin/bash
# OffGrid - Universal Installer
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
    printf "  ${CYAN}◈${NC} ${BOLD}OffGrid${NC}\n" >&2
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

# Keep these two functions identical in installers/desktop.sh: both entry points
# can be downloaded alone. Inspect all instruction families enabled by GGML.
cpu_features_from_flags() {
    local flags feature
    flags=" $(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/avx1\.0/avx/g; s/avx2\.0/avx2/g; s/sse4\.2/sse4_2/g') "
    for feature in sse4_2 avx avx2 f16c fma bmi2; do
        case "$flags" in *" $feature "*) ;; *) echo basic; return ;; esac
    done
    for feature in avx512f avx512dq avx512bw avx512vl; do
        case "$flags" in *" $feature "*) ;; *) echo avx2; return ;; esac
    done
    echo avx512
}

detect_cpu_features() {
    local os arch flags
    os=$(uname -s); arch=$(uname -m)
    case "$os:$arch" in
        Darwin:arm64) echo apple-silicon; return ;;
        Linux:aarch64|Linux:arm64) echo neon; return ;;
    esac
    case "$os" in
        Linux) flags=$(awk -F ': ' '/^flags[[:space:]]*:/ { print $2; exit }' /proc/cpuinfo) || return 1 ;;
        Darwin) flags=$(sysctl -n machdep.cpu.features machdep.cpu.leaf7_features) || return 1 ;;
        *) echo 'Use the native Windows desktop installer or installers/desktop.ps1; this shell entry point supports Linux and macOS.' >&2; return 1 ;;
    esac
    cpu_features_from_flags "$(printf '%s' "$flags" | tr '\n\t' '  ')"
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

    # Match published target names; never guess an x86 variant for ARM.
    case "$os:$arch" in
        darwin:arm64) variant=metal; cpu_features=apple-silicon ;;
        linux:arm64) variant=cpu; cpu_features=neon ;;
        linux:amd64|darwin:amd64|windows:amd64)
            case "$cpu_features" in avx2|avx512) ;; *) error 'This CLI archive requires AVX2, FMA, F16C and BMI2. No basic x64 archive is published; use a compatible host or a custom runtime build.'; return 1 ;; esac ;;
        *) error "No supported CLI archive for $os/$arch"; return 1 ;;
    esac
    
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
        if [ "$variant" = "vulkan" ]; then
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
    
    verify_release_download "${tmp_dir}/bundle${ext}" "${bundle_name}${ext}" "$version" "$tmp_dir" || return 1

    # Extract only after the release archive has been verified.
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

verify_release_download() {
    local file="$1" asset="$2" version="$3" tmp_dir="$4"
    local manifest="$tmp_dir/release-checksums.sha256" expected actual
    case "$version" in ''|*[!0-9A-Za-z.+-]*) error 'Invalid release version'; return 1 ;; esac
    # Always refresh the manifest from the selected release, not the archive.
    if ! download_with_progress "${GITHUB_URL}/releases/download/${version}/checksums-${version}.sha256" "$manifest" 'Downloading release checksums...'; then
        error 'Release checksums unavailable; refusing unverified installation'
        return 1
    fi
    expected=$(awk -v asset="$asset" '
        { sub(/\r$/, ""); digest=substr($0,1,64); marker=substr($0,65,2); name=substr($0,67) }
        name == asset && length(digest) == 64 && digest !~ /[^a-fA-F0-9]/ && (marker == "  " || marker == " *") { count++; hash=digest }
        END { if (count != 1) exit 1; print tolower(hash) }
    ' "$manifest") || { error "Expected exactly one release checksum for $asset"; return 1; }
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$file") || return 1
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$file") || return 1
    else
        error 'SHA-256 verification requires sha256sum or shasum'; return 1
    fi
    [ "${actual%% *}" = "$expected" ] || { error "Release checksum mismatch for $asset"; return 1; }
    ok "Verified $asset"
}

verify_legacy_checksums() {
    local bundle_dir="$1"
    [ "$LEGACY_INSTALL" = "yes" ] || return 0
    if [ ! -f "$bundle_dir/checksums.sha256" ]; then
        error "Bundle checksums missing; refusing legacy installation"
        return 1
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

# Public desktop names changed in 0.4.27; old release assets remain unchanged.
desktop_asset_name() {
    local version="${1#v}" os="$2" arch="$3" kind="$4"
    local core="${version%%-*}" major minor patch
    IFS=. read -r major minor patch <<< "$core"
    if (( major > 0 || minor > 4 || (minor == 4 && patch >= 27) )); then
        printf 'OffGrid-%s-%s-%s%s' "$version" "$os" "$arch" "$kind"
    elif [ "$os" = windows ]; then
        printf 'OffGrid.LLM.Desktop-Setup-%s.exe' "$version"
    else
        printf 'OffGrid.LLM.Desktop-%s-%s%s' "$version" "$arch" "$kind"
    fi
}

download_desktop_app() {
    local os="$1"
    local arch="$2"
    local version="$3"
    local tmp_dir="$4"
    
    local target_os="$os" target_arch="$arch" extension=""
    case "$os:$arch" in
        linux:amd64) target_arch=x86_64; extension=.AppImage ;;
        linux:arm64) dim 'Linux ARM64 provides the CLI/web service only; no desktop package is published.'; return 0 ;;
        darwin:amd64) target_os=macos; target_arch=x64; extension=.zip ;;
        darwin:arm64) target_os=macos; extension=.zip ;;
        windows:amd64) target_arch=x64; extension=-setup.exe ;;
        *) return 0 ;;
    esac
    local app_name
    app_name=$(desktop_asset_name "$version" "$target_os" "$target_arch" "$extension")
    local download_url="${GITHUB_URL}/releases/download/${version}/${app_name}"
    
    if download_with_progress "$download_url" "${tmp_dir}/${app_name}" "Downloading Desktop app..."; then
        verify_release_download "${tmp_dir}/${app_name}" "$app_name" "$version" "$tmp_dir" || return 1
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
    # Historical function name retained for existing sourced fixtures. This
    # installer has no authority to stop any running service or model worker.
    local name status
    for name in offgrid llama-server; do
        status=0
        pgrep -x "$name" >/dev/null 2>&1 || status=$?
        if [ "$status" -eq 0 ]; then
            error "$name is running. Finish active work and close its owning service normally, then retry. No processes were stopped."
            return 1
        fi
        [ "$status" -eq 1 ] || { error 'Unable to inspect running processes; installation stopped'; return 1; }
    done
}

install_cli() {
    local bundle_dir="$1" os="$2" ext="" use_sudo=""
    if [ -d "$bundle_dir/execution-runtime" ]; then case "${VERSION#v}" in ''|*[!0-9A-Za-z.+-]*) spin_fail "Invalid executor version"; return 1 ;; esac; fi
    [ "$os" = "windows" ] && ext=".exe"
    [ "$os" != "windows" ] && [ ! -w "$INSTALL_DIR" ] && use_sudo="sudo"
    stop_running_processes || return 1
    spin "Installing CLI..."
    local binaries=("offgrid${ext}" "llama-server${ext}") files=() installed=() binary parser
    for parser in document ocr; do
        [ ! -f "$bundle_dir/offgrid-${parser}-worker${ext}" ] || { binaries+=("offgrid-${parser}-worker${ext}"); files+=("$bundle_dir/${parser}-NOTICE.txt"); }
    done
    files+=("${binaries[@]/#/$bundle_dir/}")
    for binary in "${binaries[@]}"; do installed+=("$INSTALL_DIR/$binary"); done
    if $use_sudo cp -f "${files[@]}" "$INSTALL_DIR/" && $use_sudo chmod +x "${installed[@]}"; then
        local payload
        for payload in audio web; do
            if [ -d "$bundle_dir/$payload" ]; then
                $use_sudo mkdir -p "$INSTALL_DIR/$payload" &&
                    $use_sudo cp -R "$bundle_dir/$payload/." "$INSTALL_DIR/$payload/" || { spin_fail "Failed to install $payload payload"; return 1; }
            fi
        done
        if [ "$os" = windows ]; then
            local dependency
            for dependency in "$bundle_dir/"*.dll "$bundle_dir/runtime-dependencies.json"; do
                [ ! -f "$dependency" ] || $use_sudo cp -f "$dependency" "$INSTALL_DIR/" || return 1
            done
        fi
        # Retain versioned assets for rollback; older bundles stay compatible.
        if [ -d "$bundle_dir/execution-runtime" ]; then
            $use_sudo mkdir -p "$INSTALL_DIR/execution-runtime/${VERSION#v}" &&
              $use_sudo cp -R "$bundle_dir/execution-runtime/." "$INSTALL_DIR/execution-runtime/${VERSION#v}/" || { spin_fail "Executor assets were not installed"; return 1; }
        fi
        spin_done "CLI installed → $INSTALL_DIR/offgrid"
        return 0
    fi
    spin_fail "Failed to install CLI"
    return 1
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
        $use_sudo mkdir -p "$WEB_DIR" || return 1
        $use_sudo cp -R "$bundle_dir/web/ui/." "$WEB_DIR/" || return 1
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
Name=OffGrid
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
    local bundle_dir="$1" adapter
    local audio_dir="$HOME/.offgrid-llm/audio"
    # Install only payloads covered by the verified release archive. An absent
    # optional runtime must not trigger an unverified upstream download.
    for adapter in whisper piper; do
        if [ -d "$bundle_dir/audio/$adapter" ]; then
            mkdir -p "$audio_dir/$adapter" || return 1
            cp -R "$bundle_dir/audio/$adapter/." "$audio_dir/$adapter/" || return 1
            ok "Installed $adapter runtime files (model download is separate)"
        else
            dim "$adapter is not bundled for this target; see the voice setup guide."
        fi
    done
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
    local os arch cpu_features
    os=$(detect_os) || return 1
    arch=$(detect_arch) || return 1
    cpu_features=$(detect_cpu_features) || return 1
    if [ "$cpu_features" = basic ]; then
        error 'No compatible x64 CLI archive: AVX2, FMA, F16C and BMI2 are required. SSE4.2 alone is insufficient. Use a compatible host or build a custom runtime.'
        return 1
    fi
    
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
    local bundle_name
    bundle_name=$(download_cli_bundle "$os" "$arch" "$VERSION" "$cpu_features" "$tmp_dir" "$variant") || return 1
    
    if [ -z "$bundle_name" ]; then
        error "Failed to download"
        exit 1
    fi
    
    local bundle_dir="$tmp_dir/$bundle_name"
    verify_legacy_checksums "$bundle_dir" || return 1
    
    # Download Desktop app
    local app_path=""
    if [ "$LEGACY_INSTALL" != "yes" ]; then
        app_path=$(download_desktop_app "$os" "$arch" "$VERSION" "$tmp_dir") || return 1
    fi
    
    section "Installing"
    
    install_cli "$bundle_dir" "$os" || return 1
    install_webui "$bundle_dir" || return 1
    install_audio "$bundle_dir" || return 1
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
            darwin) printf "    ${DIM}Desktop:${NC} Open 'OffGrid' from Applications\n" >&2 ;;
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
