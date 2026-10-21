#!/bin/bash
# Build whisper.cpp binaries for distribution
# Run this script on each platform (Linux, macOS, Windows) to create release artifacts
#
# Usage:
#   ./build-whisper-binaries.sh           # Build for current platform
#   ./build-whisper-binaries.sh all       # Show instructions for all platforms

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_step() { echo -e "${CYAN}▶${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1" >&2; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$REPO_ROOT/build/whisper"
WHISPER_VERSION="v1.7.4"  # Use a stable release tag

echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  Building whisper.cpp Binaries for OffGrid LLM             ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ "$1" = "all" ]; then
    echo "To build for all platforms, run this script on each platform:"
    echo ""
    echo "  Linux (x64):   ./build-whisper-binaries.sh"
    echo "  Linux (ARM64): ./build-whisper-binaries.sh"
    echo "  macOS (x64):   ./build-whisper-binaries.sh"
    echo "  macOS (ARM64): ./build-whisper-binaries.sh"
    echo "  Windows:       ./build-whisper-binaries.sh (in Git Bash or WSL)"
    echo ""
    echo "Then upload these files to GitHub releases:"
    echo "  - whisper-linux-amd64.tar.gz"
    echo "  - whisper-linux-arm64.tar.gz"
    echo "  - whisper-darwin-amd64.tar.gz"
    echo "  - whisper-darwin-arm64.tar.gz"
    echo "  - whisper-windows-amd64.zip"
    exit 0
fi

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) print_error "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux) PLATFORM="linux-$ARCH" ;;
    darwin) PLATFORM="darwin-$ARCH" ;;
    mingw*|msys*|cygwin*) PLATFORM="windows-amd64"; OS="windows" ;;
    *) print_error "Unsupported OS: $OS"; exit 1 ;;
esac

print_step "Building for platform: $PLATFORM"

# Check dependencies
print_step "Checking build dependencies..."
for cmd in git cmake make; do
    if ! command -v $cmd &> /dev/null; then
        print_error "Missing required tool: $cmd"
        exit 1
    fi
done
print_success "All dependencies found"

# Create build directory
mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

# Clone whisper.cpp
if [ -d "whisper.cpp" ]; then
    print_step "Updating whisper.cpp repository..."
    cd whisper.cpp
    git fetch --tags
    git checkout "$WHISPER_VERSION"
    cd ..
else
    print_step "Cloning whisper.cpp ($WHISPER_VERSION)..."
    git clone --depth 1 --branch "$WHISPER_VERSION" https://github.com/ggml-org/whisper.cpp.git
fi

cd whisper.cpp

# Build
print_step "Configuring build..."
cmake -B build -DCMAKE_BUILD_TYPE=Release

print_step "Building (this may take a few minutes)..."
cmake --build build --config Release -j$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

# Create output directory
OUTPUT_DIR="$BUILD_DIR/output"
mkdir -p "$OUTPUT_DIR"

# Copy binaries
print_step "Packaging binaries..."
BINARY_NAME="whisper-cli"
if [ "$OS" = "windows" ]; then
    BINARY_NAME="whisper-cli.exe"
fi

# Create package directory
PKG_DIR="$OUTPUT_DIR/whisper-$PLATFORM"
rm -rf "$PKG_DIR"
mkdir -p "$PKG_DIR"

# Copy the main binary
cp "build/bin/$BINARY_NAME" "$PKG_DIR/"

# Copy shared libraries on Linux
if [ "$OS" = "linux" ]; then
    cp build/src/*.so* "$PKG_DIR/" 2>/dev/null || true
    cp build/ggml/src/*.so* "$PKG_DIR/" 2>/dev/null || true
    
    # Create wrapper script
    cat > "$PKG_DIR/whisper" << 'EOF'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LD_LIBRARY_PATH="$SCRIPT_DIR:$LD_LIBRARY_PATH"
exec "$SCRIPT_DIR/whisper-cli" "$@"
EOF
    chmod +x "$PKG_DIR/whisper"
fi

# Copy dylibs on macOS
if [ "$OS" = "darwin" ]; then
    cp build/src/*.dylib "$PKG_DIR/" 2>/dev/null || true
    cp build/ggml/src/*.dylib "$PKG_DIR/" 2>/dev/null || true
    
    # Create wrapper script
    cat > "$PKG_DIR/whisper" << 'EOF'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export DYLD_LIBRARY_PATH="$SCRIPT_DIR:$DYLD_LIBRARY_PATH"
exec "$SCRIPT_DIR/whisper-cli" "$@"
EOF
    chmod +x "$PKG_DIR/whisper"
fi

# Create archive
cd "$OUTPUT_DIR"
if [ "$OS" = "windows" ]; then
    ARCHIVE_NAME="whisper-$PLATFORM.zip"
    zip -r "$ARCHIVE_NAME" "whisper-$PLATFORM"
else
    ARCHIVE_NAME="whisper-$PLATFORM.tar.gz"
    tar -czvf "$ARCHIVE_NAME" "whisper-$PLATFORM"
fi

print_success "Created: $OUTPUT_DIR/$ARCHIVE_NAME"
echo ""
echo "Upload this file to GitHub releases:"
echo "  $OUTPUT_DIR/$ARCHIVE_NAME"
echo ""

# Show file size
ls -lh "$OUTPUT_DIR/$ARCHIVE_NAME"
