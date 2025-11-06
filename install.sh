#!/bin/bash
set -e

# OffGrid LLM Installation Script
# Usage: ./install.sh [--user|--system]

BINARY="offgrid"
INSTALL_MODE="${1:---system}"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  OffGrid LLM - Installation Script"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check for Go installation
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed"
    echo "   Please install Go 1.21 or higher: https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | grep -oP 'go\d+\.\d+' | grep -oP '\d+\.\d+')
echo "✓ Go version: $GO_VERSION"

case "$INSTALL_MODE" in
    --system)
        echo "📦 Installing system-wide to /usr/local/bin..."
        echo "   (requires sudo privileges)"
        echo ""
        
        # Build binary
        echo "🔨 Building OffGrid LLM..."
        make build
        
        # Install to system
        echo "📥 Installing to /usr/local/bin..."
        sudo install -m 755 "$BINARY" "/usr/local/bin/$BINARY"
        
        # Verify installation
        if command -v offgrid &> /dev/null; then
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "✅ Installation successful!"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            echo "Location: /usr/local/bin/offgrid"
            echo "Version:  $(offgrid --version 2>/dev/null || echo 'dev')"
            echo ""
            echo "Quick Start:"
            echo "  offgrid catalog        # Browse available models"
            echo "  offgrid quantization   # Learn about quantization"
            echo "  offgrid serve          # Start server"
            echo ""
            echo "Documentation:"
            echo "  offgrid help           # Show all commands"
            echo "  cat README.md          # Full documentation"
            echo ""
        else
            echo "❌ Installation failed: 'offgrid' command not found in PATH"
            exit 1
        fi
        ;;
        
    --user)
        echo "📦 Installing to user bin (no sudo required)..."
        echo ""
        
        # Install using go install
        echo "🔨 Installing via 'go install'..."
        make install
        
        GOPATH=$(go env GOPATH)
        GOBIN="$GOPATH/bin"
        
        # Check if Go bin is in PATH
        if [[ ":$PATH:" == *":$GOBIN:"* ]]; then
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "✅ Installation successful!"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            echo "Location: $GOBIN/offgrid"
            echo ""
            echo "Quick Start:"
            echo "  offgrid catalog        # Browse available models"
            echo "  offgrid serve          # Start server"
            echo ""
        else
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "⚠️  Installation complete - PATH configuration needed"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            echo "Location: $GOBIN/offgrid"
            echo ""
            echo "To use 'offgrid' from anywhere, add Go bin to your PATH:"
            echo ""
            echo "  1. Add to ~/.bashrc or ~/.zshrc:"
            echo "     export PATH=\"\$PATH:$GOBIN\""
            echo ""
            echo "  2. Reload shell:"
            echo "     source ~/.bashrc"
            echo ""
            echo "  3. Verify installation:"
            echo "     which offgrid"
            echo ""
            echo "Or use the full path:"
            echo "  $GOBIN/offgrid catalog"
            echo ""
        fi
        ;;
        
    *)
        echo "Usage: $0 [--user|--system]"
        echo ""
        echo "Options:"
        echo "  --system   Install to /usr/local/bin (requires sudo) [default]"
        echo "  --user     Install to \$GOPATH/bin (no sudo required)"
        echo ""
        echo "Examples:"
        echo "  $0              # System-wide installation"
        echo "  $0 --system     # System-wide installation (explicit)"
        echo "  $0 --user       # User installation"
        exit 1
        ;;
esac

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
