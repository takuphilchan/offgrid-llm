#!/bin/bash
# Master build script for OffGrid LLM
# Builds backend binaries and desktop applications for all platforms

set -e

# Build script for all platforms
VERSION="0.1.5"
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

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
║                    BUILD SYSTEM v${VERSION}                   ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

print_step() { echo -e "${CYAN}[BUILD]${NC} $1"; }
print_success() { echo -e "${GREEN}[OK]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
print_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Parse arguments
BUILD_CLI=false
BUILD_DESKTOP=false
BUILD_ALL=false
TARGET_PLATFORM="current"

while [[ $# -gt 0 ]]; do
  case $1 in
    --cli)
      BUILD_CLI=true
      shift
      ;;
    --desktop)
      BUILD_DESKTOP=true
      shift
      ;;
    --all)
      BUILD_ALL=true
      shift
      ;;
    --platform)
      TARGET_PLATFORM="$2"
      shift 2
      ;;
    *)
      echo "Usage: $0 [--cli] [--desktop] [--all] [--platform linux|macos|windows]"
      echo ""
      echo "Options:"
      echo "  --cli       Build CLI binaries only"
      echo "  --desktop   Build desktop applications only"
      echo "  --all       Build everything for all platforms"
      echo "  --platform  Target platform (linux, macos, windows, or current)"
      exit 1
      ;;
  esac
done

# Default to building everything if no options specified
if [ "$BUILD_CLI" = false ] && [ "$BUILD_DESKTOP" = false ] && [ "$BUILD_ALL" = false ]; then
  BUILD_ALL=true
fi

if [ "$BUILD_ALL" = true ]; then
  BUILD_CLI=true
  BUILD_DESKTOP=true
  TARGET_PLATFORM="all"
fi

# Detect current platform
CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

case "$CURRENT_ARCH" in
  x86_64|amd64)
    CURRENT_ARCH="amd64"
    ;;
  aarch64|arm64)
    CURRENT_ARCH="arm64"
    ;;
esac

print_step "Build Configuration:"
echo "  Version: $VERSION"
echo "  Current Platform: $CURRENT_OS-$CURRENT_ARCH"
echo "  Target Platform: $TARGET_PLATFORM"
echo "  Build CLI: $BUILD_CLI"
echo "  Build Desktop: $BUILD_DESKTOP"
echo ""

# Build CLI binaries
if [ "$BUILD_CLI" = true ]; then
  print_step "Building CLI binaries..."
  
  cd "$PROJECT_ROOT"
  
  if [ "$TARGET_PLATFORM" = "current" ] || [ "$TARGET_PLATFORM" = "linux" ]; then
    print_step "Building Linux binary..."
    GOOS=linux GOARCH=$CURRENT_ARCH go build -ldflags "-X main.Version=$VERSION" -o "build/linux/offgrid" ./cmd/offgrid
    print_success "Linux binary built: build/linux/offgrid"
  fi
  
  if [ "$TARGET_PLATFORM" = "macos" ] || [ "$TARGET_PLATFORM" = "all" ]; then
    print_step "Building macOS binary..."
    GOOS=darwin GOARCH=$CURRENT_ARCH go build -ldflags "-X main.Version=$VERSION" -o "build/macos/offgrid" ./cmd/offgrid
    print_success "macOS binary built: build/macos/offgrid"
  fi
  
  if [ "$TARGET_PLATFORM" = "windows" ] || [ "$TARGET_PLATFORM" = "all" ]; then
    print_step "Building Windows binary..."
    GOOS=windows GOARCH=$CURRENT_ARCH go build -ldflags "-X main.Version=$VERSION" -o "build/windows/offgrid.exe" ./cmd/offgrid
    print_success "Windows binary built: build/windows/offgrid.exe"
  fi
  
  print_success "CLI binaries built successfully"
  echo ""
fi

# Build desktop applications
if [ "$BUILD_DESKTOP" = true ]; then
  print_step "Building desktop applications..."
  
  cd "$PROJECT_ROOT/desktop"
  
  # Check if node_modules exists
  if [ ! -d "node_modules" ]; then
    print_step "Installing Node.js dependencies..."
    npm install
  fi
  
  # Clean previous builds
  print_step "Cleaning previous desktop builds..."
  rm -rf dist/
  
  # Build desktop apps
  if [ "$TARGET_PLATFORM" = "current" ]; then
    print_step "Building desktop app for current platform..."
    npm run build
  elif [ "$TARGET_PLATFORM" = "linux" ]; then
    print_step "Building desktop app for Linux..."
    npm run build:linux
  elif [ "$TARGET_PLATFORM" = "macos" ]; then
    print_step "Building desktop app for macOS..."
    npm run build:mac
  elif [ "$TARGET_PLATFORM" = "windows" ]; then
    print_step "Building desktop app for Windows..."
    npm run build:win
  elif [ "$TARGET_PLATFORM" = "all" ]; then
    print_step "Building desktop apps for all platforms..."
    npm run build:all
  fi
  
  print_success "Desktop applications built successfully"
  echo ""
  
  # List built files
  if [ -d "dist" ]; then
    print_step "Built packages:"
    ls -lh dist/ | grep -E '\.(AppImage|dmg|exe|deb|rpm)$' || true
  fi
fi

echo ""
print_success "Build completed successfully!"
echo ""
print_step "Next steps:"
if [ "$BUILD_CLI" = true ]; then
  echo "  CLI binaries are in: build/{platform}/"
  echo "  Install CLI: sudo cp build/linux/offgrid /usr/local/bin/"
fi
if [ "$BUILD_DESKTOP" = true ]; then
  echo "  Desktop installers are in: desktop/dist/"
  echo "  Install desktop app by running the installer for your platform"
fi
echo ""
