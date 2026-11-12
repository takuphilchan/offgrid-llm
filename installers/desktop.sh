#!/bin/bash
# OffGrid LLM Desktop - Easy Installer for Linux/macOS
# Installs both CLI and Desktop application

set -euo pipefail

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_banner() {
    echo -e "${CYAN}"
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
║               DESKTOP INSTALLER                               ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

print_step() { echo -e "${CYAN}[$(date +%H:%M:%S)]${NC} $1"; }
print_success() { echo -e "${GREEN}[OK]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
print_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }

verify_release_checksum() {
    local file="$1" asset="$2" manifest="$3" expected actual
    expected=$(awk -v asset="$asset" '
        { sub(/\r$/, ""); digest=substr($0,1,64); marker=substr($0,65,2); name=substr($0,67) }
        name == asset && length(digest) == 64 && digest !~ /[^a-fA-F0-9]/ && (marker == "  " || marker == " *") { count++; hash=digest }
        END { if (count != 1) exit 1; print tolower(hash) }
    ' "$manifest") || { print_error "Expected exactly one checksum for $asset"; return 1; }
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$file") || return 1
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$file") || return 1
    else
        print_error 'SHA-256 verification requires sha256sum or shasum'; return 1
    fi
    [ "${actual%% *}" = "$expected" ] || { print_error "Checksum mismatch for $asset. Installation stopped."; return 1; }
}

assert_cli_stopped() {
    local status name
    for name in offgrid llama-server; do
        status=0
        pgrep -x "$name" >/dev/null || status=$?
        if [ "$status" -eq 0 ]; then
            print_error "$name is running. Finish active work and close its owning service normally before retrying. No processes were stopped."
            return 1
        fi
        [ "$status" -eq 1 ] || { print_error 'Unable to check running processes'; return 1; }
    done
}

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

print_banner

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        print_error "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

print_step "Detected: $OS-$ARCH"
echo ""

# Installation options
echo "What would you like to install?"
echo "  1) CLI only (command-line tool)"
echo "  2) Desktop app only (GUI application)"
echo "  3) Both CLI and Desktop (recommended)"
echo ""
read -p "Enter your choice [1-3]: " INSTALL_CHOICE

case "$INSTALL_CHOICE" in
    1)
        INSTALL_CLI=true
        INSTALL_DESKTOP=false
        ;;
    2)
        INSTALL_CLI=false
        INSTALL_DESKTOP=true
        ;;
    3|*)
        INSTALL_CLI=true
        INSTALL_DESKTOP=true
        ;;
esac

# Refuse unsupported choices before downloads or changes to an existing CLI.
if [ "$OS:$ARCH" = linux:arm64 ] && [ "$INSTALL_DESKTOP" = true ]; then
    print_error 'Linux ARM64 has CLI/web support only. Select CLI only; no ARM64 desktop package is published.'
    exit 1
fi
if [ "$INSTALL_CLI" = true ]; then
    CPU_FEATURES=$(detect_cpu_features) || exit 1
    if [ "$CPU_FEATURES" = basic ]; then
        print_error 'CLI archives require AVX2, FMA, F16C and BMI2. SSE4.2 alone is insufficient; use a compatible host or custom runtime build.'
        exit 1
    fi
fi

# GitHub release info
GITHUB_REPO="takuphilchan/offgrid-llm"

# Fetch latest release version from GitHub API
print_step "Fetching latest release version..."
VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | \
    grep '"tag_name":' | \
    sed -E 's/.*"tag_name": "v?([^"]+)".*/\1/')

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9.-]+)?$ ]]; then
    print_error "Failed to fetch latest version from GitHub"
    exit 1
fi

print_success "Latest version: v${VERSION}"
RELEASE_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}"

# Temporary directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf -- "$TMP_DIR"' EXIT
MANIFEST="$TMP_DIR/checksums.sha256"
curl -fsSL "${RELEASE_URL}/checksums-v${VERSION}.sha256" -o "$MANIFEST"

# Install CLI
if [ "$INSTALL_CLI" = true ]; then
    assert_cli_stopped
    print_step "Installing CLI..."
    
    # Determine the correct bundle file based on OS and architecture
    if [ "$OS" = "darwin" ]; then
        if [ "$ARCH" = "arm64" ]; then
            CLI_BUNDLE="offgrid-v${VERSION}-darwin-arm64-metal-apple-silicon.tar.gz"
        else
            CLI_BUNDLE="offgrid-v${VERSION}-darwin-amd64-cpu-avx2.tar.gz"
        fi
    elif [ "$OS" = "linux" ]; then
        if [ "$ARCH" = "arm64" ]; then
            CLI_BUNDLE="offgrid-v${VERSION}-linux-arm64-cpu-neon.tar.gz"
        else
            # Default to AVX2 for most modern CPUs
            CLI_BUNDLE="offgrid-v${VERSION}-linux-amd64-cpu-avx2.tar.gz"
        fi
    else
        print_error "Unsupported OS: $OS"
        exit 1
    fi
    
    print_step "Downloading CLI bundle: $CLI_BUNDLE"
    if ! curl -fSL "${RELEASE_URL}/${CLI_BUNDLE}" -o "${TMP_DIR}/${CLI_BUNDLE}"; then
        print_error "Failed to download CLI bundle"
        exit 1
    fi
    
    verify_release_checksum "${TMP_DIR}/${CLI_BUNDLE}" "$CLI_BUNDLE" "$MANIFEST"
    print_step "Extracting bundle..."
    cd "${TMP_DIR}"
    tar -xzf "${CLI_BUNDLE}"
    
    # Find the bundle directory
    BUNDLE_DIR=$(find . -maxdepth 1 -type d -name "offgrid-*" | head -1)
    if [ -z "$BUNDLE_DIR" ]; then
        BUNDLE_DIR="."
    fi
    
    # Find the extracted offgrid binary
    OFFGRID_BIN=$(find "$BUNDLE_DIR" -name "offgrid" -o -name "offgrid.exe" | head -1)
    if [ -z "$OFFGRID_BIN" ]; then
        print_error "Could not find offgrid binary in bundle"
        exit 1
    fi
    
    # Find llama-server binary
    LLAMA_BIN=$(find "$BUNDLE_DIR" -name "llama-server" -o -name "llama-server.exe" | head -1)
    
    chmod +x "$OFFGRID_BIN"
    [ -n "$LLAMA_BIN" ] && chmod +x "$LLAMA_BIN"
    
    print_step "Installing to /usr/local/bin/..."
    
    # Recheck before writing: never stop a service owned by another workflow.
    assert_cli_stopped
    
    # Try to install binaries, retry if file is busy
    MAX_RETRIES=3
    RETRY_COUNT=0
    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        if [ "$EUID" -eq 0 ]; then
            if cp "$OFFGRID_BIN" /usr/local/bin/offgrid 2>/dev/null; then
                if [ -n "$LLAMA_BIN" ]; then cp "$LLAMA_BIN" /usr/local/bin/llama-server; fi
                break
            fi
        else
            if sudo cp "$OFFGRID_BIN" /usr/local/bin/offgrid 2>/dev/null; then
                if [ -n "$LLAMA_BIN" ]; then sudo cp "$LLAMA_BIN" /usr/local/bin/llama-server; fi
                break
            fi
        fi
        
        RETRY_COUNT=$((RETRY_COUNT + 1))
        if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
            print_warning "File is busy, retrying in 2 seconds... (attempt $((RETRY_COUNT + 1))/$MAX_RETRIES)"
            sleep 2
        else
            print_error "Failed to install after $MAX_RETRIES attempts. Please close any running instances of offgrid and try again."
            exit 1
        fi
    done
    
    # Make binaries executable
    if [ "$EUID" -eq 0 ]; then
        chmod +x /usr/local/bin/offgrid /usr/local/bin/llama-server 2>/dev/null || true
    else
        sudo chmod +x /usr/local/bin/offgrid /usr/local/bin/llama-server 2>/dev/null || true
    fi
    
    for parser in document ocr; do
    DOCUMENT_RUNTIME="$(dirname "$OFFGRID_BIN")/offgrid-${parser}-worker"
    if [ -f "$DOCUMENT_RUNTIME" ]; then
        if [ "$EUID" -eq 0 ]; then
            cp "$DOCUMENT_RUNTIME" /usr/local/bin/ && cp "$(dirname "$OFFGRID_BIN")/${parser}-NOTICE.txt" /usr/local/bin/ && chmod +x "/usr/local/bin/offgrid-${parser}-worker" || exit 1
        else
            sudo cp "$DOCUMENT_RUNTIME" /usr/local/bin/ && sudo cp "$(dirname "$OFFGRID_BIN")/${parser}-NOTICE.txt" /usr/local/bin/ && sudo chmod +x "/usr/local/bin/offgrid-${parser}-worker" || exit 1
        fi
    fi
    done
    # Keep the packaged executor and application-relative runtime/UI layouts.
    for payload in execution-runtime audio web; do
        if [ -d "$BUNDLE_DIR/$payload" ]; then
            if [ "$EUID" -eq 0 ]; then
                mkdir -p "/usr/local/bin/$payload"
                cp -R "$BUNDLE_DIR/$payload/." "/usr/local/bin/$payload/"
            else
                sudo mkdir -p "/usr/local/bin/$payload"
                sudo cp -R "$BUNDLE_DIR/$payload/." "/usr/local/bin/$payload/"
            fi
        fi
    done
    print_success "CLI binaries installed"
    
    # Install audio components (whisper.cpp and piper)
    AUDIO_DIR="$HOME/.offgrid-llm/audio"
    if [ -d "$BUNDLE_DIR/audio" ]; then
        print_step "Installing audio components (whisper.cpp, piper)..."
        mkdir -p "$AUDIO_DIR/whisper" "$AUDIO_DIR/piper"
        
        if [ -d "$BUNDLE_DIR/audio/whisper" ]; then
            cp -r "$BUNDLE_DIR/audio/whisper/." "$AUDIO_DIR/whisper/"
            print_success "Whisper (speech-to-text) installed"
        fi
        
        if [ -d "$BUNDLE_DIR/audio/piper" ]; then
            cp -r "$BUNDLE_DIR/audio/piper/." "$AUDIO_DIR/piper/"
            print_success "Piper (text-to-speech) installed"
        fi
    fi
    
    # Install web UI
    if [ -d "$BUNDLE_DIR/web/ui" ]; then
        print_step "Installing web UI..."
        WEB_DIR="/var/lib/offgrid/web/ui"
        if [ "$EUID" -eq 0 ]; then
            mkdir -p "$WEB_DIR"
            cp -r "$BUNDLE_DIR/web/ui/"* "$WEB_DIR/"
        else
            sudo mkdir -p "$WEB_DIR"
            sudo cp -r "$BUNDLE_DIR/web/ui/"* "$WEB_DIR/"
        fi
        print_success "Web UI installed"
    fi
    
    # Verify installation
    VERSION_OUTPUT=$(/usr/local/bin/offgrid --version 2>&1)
    print_success "Verification: $VERSION_OUTPUT"
    print_success "CLI installed successfully"
    echo ""
fi

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

# Install Desktop
if [ "$INSTALL_DESKTOP" = true ]; then
    print_step "Installing Desktop application..."
    
    if [ "$OS" = "linux" ]; then
        # Check for package manager
        if command -v apt-get &> /dev/null; then
            # Debian/Ubuntu - use .deb
            # Map architecture for file naming
            if [ "$ARCH" = "amd64" ]; then
                DESKTOP_ARCH="amd64"
                APPIMAGE_ARCH="x86_64"
            else
                DESKTOP_ARCH="arm64"
                APPIMAGE_ARCH="arm64"
            fi
            
            DESKTOP_FILE=$(desktop_asset_name "$VERSION" linux "$DESKTOP_ARCH" .deb)
            print_step "Downloading Debian package..."
            
            if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/offgrid-desktop.deb"; then
                print_warning "Debian package not available, trying AppImage..."
                DESKTOP_FILE=$(desktop_asset_name "$VERSION" linux "$APPIMAGE_ARCH" .AppImage)
                
                if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage"; then
                    print_error "Failed to download desktop application"
                    exit 1
                fi
                
                verify_release_checksum "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage" "$DESKTOP_FILE" "$MANIFEST"
                # Install AppImage
                print_step "Installing AppImage..."
                mkdir -p "$HOME/.local/bin"
                cp "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage" "$HOME/.local/bin/"
                chmod +x "$HOME/.local/bin/OffGrid-LLM-Desktop.AppImage"
                
                # Create desktop entry
                mkdir -p "$HOME/.local/share/applications"
                cat > "$HOME/.local/share/applications/offgrid-llm-desktop.desktop" << EOF
[Desktop Entry]
Name=OffGrid
Comment=Local AI Platform
Exec=$HOME/.local/bin/OffGrid-LLM-Desktop.AppImage
Icon=offgrid-llm
Terminal=false
Type=Application
Categories=Development;Science;
EOF
                
                print_success "Desktop app installed as AppImage"
                print_step "Location: $HOME/.local/bin/OffGrid-LLM-Desktop.AppImage"
            else
                verify_release_checksum "${TMP_DIR}/offgrid-desktop.deb" "$DESKTOP_FILE" "$MANIFEST"
                # Install .deb package
                print_step "Installing Debian package..."
                if [ "$EUID" -eq 0 ]; then
                    dpkg -i "${TMP_DIR}/offgrid-desktop.deb"
                    apt-get install -f -y
                else
                    sudo dpkg -i "${TMP_DIR}/offgrid-desktop.deb"
                    sudo apt-get install -f -y
                fi
                print_success "Desktop app installed from .deb package"
            fi
            
        else
            # Use AppImage for other distros
            APPIMAGE_ARCH="$ARCH"
            [ "$ARCH" = amd64 ] && APPIMAGE_ARCH=x86_64
            DESKTOP_FILE=$(desktop_asset_name "$VERSION" linux "$APPIMAGE_ARCH" .AppImage)
            print_step "Downloading AppImage..."
            
            if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage"; then
                print_error "Failed to download desktop application"
                exit 1
            fi
            
            verify_release_checksum "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage" "$DESKTOP_FILE" "$MANIFEST"
            # Install AppImage
            print_step "Installing AppImage..."
            mkdir -p "$HOME/.local/bin"
            cp "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage" "$HOME/.local/bin/"
            chmod +x "$HOME/.local/bin/OffGrid-LLM-Desktop.AppImage"
            
            # Create desktop entry
            mkdir -p "$HOME/.local/share/applications"
            cat > "$HOME/.local/share/applications/offgrid-llm-desktop.desktop" << EOF
[Desktop Entry]
Name=OffGrid
Comment=Local AI Platform
Exec=$HOME/.local/bin/OffGrid-LLM-Desktop.AppImage
Icon=offgrid-llm
Terminal=false
Type=Application
Categories=Development;Science;
EOF
            
            print_success "Desktop app installed as AppImage"
            print_step "Location: $HOME/.local/bin/OffGrid-LLM-Desktop.AppImage"
        fi
        
    elif [ "$OS" = "darwin" ]; then
        ZIP_ARCH="$ARCH"
        [ "$ARCH" = amd64 ] && ZIP_ARCH=x64
        DESKTOP_FILE=$(desktop_asset_name "$VERSION" macos "$ZIP_ARCH" .zip)
        print_step "Downloading macOS application..."
        if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/offgrid-desktop.zip"; then
            print_error "Failed to download desktop application"
            exit 1
        fi
        verify_release_checksum "${TMP_DIR}/offgrid-desktop.zip" "$DESKTOP_FILE" "$MANIFEST"
        mkdir -p "${TMP_DIR}/desktop"
        ditto -x -k "${TMP_DIR}/offgrid-desktop.zip" "${TMP_DIR}/desktop"
        APP_PATH="${TMP_DIR}/desktop/OffGrid.app"
        [ -d "$APP_PATH" ] || APP_PATH="${TMP_DIR}/desktop/OffGrid LLM Desktop.app"
        if [ ! -d "$APP_PATH" ]; then
            print_error "The download does not contain the expected application"
            exit 1
        fi
        print_step "Installing to /Applications..."
        ditto "$APP_PATH" "/Applications/$(basename "$APP_PATH")"
        print_success "Desktop app installed to /Applications"
    fi
    
    echo ""
fi

# Create config directory
print_step "Creating configuration directory..."
mkdir -p "$HOME/.offgrid-llm/models"
mkdir -p "$HOME/.offgrid-llm/data"
print_success "Config directory: $HOME/.offgrid-llm"
echo ""

# Print success message
print_success "Installation complete!"
echo ""
echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                                                               ║${NC}"
echo -e "${GREEN}║  OffGrid has been installed successfully!                 ║${NC}"
echo -e "${GREEN}║                                                               ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ "$INSTALL_CLI" = true ]; then
    echo "CLI installed:"
    echo "  - Run: offgrid --help"
    echo "  - Location: /usr/local/bin/offgrid"
    echo ""
fi

if [ "$INSTALL_DESKTOP" = true ]; then
    echo "Desktop app installed:"
    if [ "$OS" = "linux" ]; then
        echo "  - Launch from your applications menu or:"
        if command -v apt-get &> /dev/null && [ -f "/usr/bin/offgrid-llm-desktop" ]; then
            echo "  - Run: offgrid-llm-desktop"
        else
            echo "  - Run: $HOME/.local/bin/OffGrid-LLM-Desktop.AppImage"
        fi
    elif [ "$OS" = "darwin" ]; then
        echo "  - Launch from Applications folder or Launchpad"
        echo "  - Open the installed app in /Applications"
    fi
    echo ""
fi

echo "Next steps:"
echo "  1. Open the desktop app, or run offgrid serve and open http://127.0.0.1:11611/ui/"
echo "     Download a chat model from Models."
echo ""
echo "  2. Start using:"
if [ "$INSTALL_DESKTOP" = true ]; then
    echo "     - Launch the desktop app"
else
    echo "     - Open Chat in the web UI and send a prompt."
fi
echo ""
echo "Documentation: https://github.com/${GITHUB_REPO}"
echo ""
