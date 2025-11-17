#!/bin/bash
# OffGrid LLM Desktop - Easy Installer for Linux/macOS
# Installs both CLI and Desktop application

set -e

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
║               DESKTOP INSTALLER v0.1.4                        ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

print_step() { echo -e "${CYAN}[$(date +%H:%M:%S)]${NC} $1"; }
print_success() { echo -e "${GREEN}[OK]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
print_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }

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

# GitHub release info
GITHUB_REPO="takuphilchan/offgrid-llm"
VERSION="0.1.4"
RELEASE_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}"

# Temporary directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Install CLI
if [ "$INSTALL_CLI" = true ]; then
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
    
    print_step "Extracting bundle..."
    cd "${TMP_DIR}"
    tar -xzf "${CLI_BUNDLE}"
    
    # Find the extracted offgrid binary
    OFFGRID_BIN=$(find . -name "offgrid" -o -name "offgrid.exe" | head -1)
    if [ -z "$OFFGRID_BIN" ]; then
        print_error "Could not find offgrid binary in bundle"
        exit 1
    fi
    
    chmod +x "$OFFGRID_BIN"
    
    print_step "Installing to /usr/local/bin/offgrid..."
    
    # Check if offgrid is currently running
    if pgrep -x offgrid > /dev/null; then
        print_warning "OffGrid is currently running. Stopping it first..."
        pkill -x offgrid 2>/dev/null || true
        sleep 1
    fi
    
    # Try to install, retry if file is busy
    MAX_RETRIES=3
    RETRY_COUNT=0
    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        if [ "$EUID" -eq 0 ]; then
            if cp "$OFFGRID_BIN" /usr/local/bin/offgrid 2>/dev/null; then
                break
            fi
        else
            if sudo cp "$OFFGRID_BIN" /usr/local/bin/offgrid 2>/dev/null; then
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
    
    print_success "CLI installed successfully"
    
    # Verify installation
    if command -v offgrid &> /dev/null; then
        VERSION_OUTPUT=$(offgrid --version 2>&1 || echo "version check failed")
        print_success "Verification: $VERSION_OUTPUT"
    fi
    echo ""
fi

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
            
            DESKTOP_FILE="OffGrid.LLM.Desktop-${VERSION}-${DESKTOP_ARCH}.deb"
            print_step "Downloading Debian package..."
            
            if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/offgrid-desktop.deb"; then
                print_warning "Debian package not available, trying AppImage..."
                DESKTOP_FILE="OffGrid.LLM.Desktop-${VERSION}-${APPIMAGE_ARCH}.AppImage"
                
                if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage"; then
                    print_error "Failed to download desktop application"
                    exit 1
                fi
                
                # Install AppImage
                print_step "Installing AppImage..."
                mkdir -p "$HOME/.local/bin"
                cp "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage" "$HOME/.local/bin/"
                chmod +x "$HOME/.local/bin/OffGrid-LLM-Desktop.AppImage"
                
                # Create desktop entry
                mkdir -p "$HOME/.local/share/applications"
                cat > "$HOME/.local/share/applications/offgrid-llm-desktop.desktop" << EOF
[Desktop Entry]
Name=OffGrid LLM Desktop
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
            DESKTOP_FILE="OffGrid-LLM-Desktop-${VERSION}-${ARCH}.AppImage"
            print_step "Downloading AppImage..."
            
            if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage"; then
                print_error "Failed to download desktop application"
                exit 1
            fi
            
            # Install AppImage
            print_step "Installing AppImage..."
            mkdir -p "$HOME/.local/bin"
            cp "${TMP_DIR}/OffGrid-LLM-Desktop.AppImage" "$HOME/.local/bin/"
            chmod +x "$HOME/.local/bin/OffGrid-LLM-Desktop.AppImage"
            
            # Create desktop entry
            mkdir -p "$HOME/.local/share/applications"
            cat > "$HOME/.local/share/applications/offgrid-llm-desktop.desktop" << EOF
[Desktop Entry]
Name=OffGrid LLM Desktop
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
        # macOS - use .dmg
        # Map architecture for DMG naming
        if [ "$ARCH" = "amd64" ]; then
            DMG_ARCH="x64"
        else
            DMG_ARCH="arm64"
        fi
        DESKTOP_FILE="OffGrid LLM Desktop-${VERSION}-${DMG_ARCH}.dmg"
        print_step "Downloading macOS DMG..."
        
        if ! curl -fSL "${RELEASE_URL}/${DESKTOP_FILE}" -o "${TMP_DIR}/offgrid-desktop.dmg"; then
            print_error "Failed to download desktop application"
            exit 1
        fi
        
        print_step "Mounting DMG..."
        MOUNT_POINT=$(hdiutil attach "${TMP_DIR}/offgrid-desktop.dmg" | grep Volumes | awk '{print $3}')
        
        print_step "Installing to /Applications..."
        cp -R "${MOUNT_POINT}/OffGrid LLM Desktop.app" /Applications/
        
        print_step "Unmounting DMG..."
        hdiutil detach "$MOUNT_POINT"
        
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
echo -e "${GREEN}║  OffGrid LLM has been installed successfully!                 ║${NC}"
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
        echo "  - Or run: open -a 'OffGrid LLM Desktop'"
    fi
    echo ""
fi

echo "Next steps:"
echo "  1. Download a model:"
echo "     offgrid download llama-2-7b-chat"
echo ""
echo "  2. Start using:"
if [ "$INSTALL_DESKTOP" = true ]; then
    echo "     - Launch the desktop app"
else
    echo "     - Run: offgrid chat"
fi
echo ""
echo "Documentation: https://github.com/${GITHUB_REPO}"
echo ""
