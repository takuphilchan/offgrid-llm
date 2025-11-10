#!/bin/bash

# OffGrid LLM Desktop Build Script
# Builds desktop apps for all platforms

set -e

echo "╔═══════════════════════════════════════════════╗"
echo "║   OffGrid LLM Desktop App Builder            ║"
echo "╚═══════════════════════════════════════════════╝"
echo ""

# Check if we're in the right directory
if [ ! -f "package.json" ]; then
    echo "❌ Error: Must run from desktop/ directory"
    exit 1
fi

# Check if Go binary exists
if [ ! -f "../offgrid" ] && [ ! -f "../offgrid.exe" ]; then
    echo "⚠️  Go binary not found. Building server..."
    cd ..
    make build
    cd desktop
    echo "✓ Server built"
fi

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    echo "📦 Installing dependencies..."
    npm install
    echo "✓ Dependencies installed"
fi

# Determine build target
BUILD_TARGET="${1:-current}"

echo ""
echo "Building for: $BUILD_TARGET"
echo ""

case $BUILD_TARGET in
  "current")
    echo "🔨 Building for current platform..."
    npm run build
    ;;
  "windows"|"win")
    echo "🪟 Building for Windows..."
    npm run build:win
    ;;
  "mac"|"macos")
    echo "🍎 Building for macOS..."
    npm run build:mac
    ;;
  "linux")
    echo "🐧 Building for Linux..."
    npm run build:linux
    ;;
  "all")
    echo "🌍 Building for all platforms..."
    npm run build:all
    ;;
  *)
    echo "❌ Unknown target: $BUILD_TARGET"
    echo ""
    echo "Usage: ./build.sh [current|windows|mac|linux|all]"
    exit 1
    ;;
esac

echo ""
echo "✅ Build complete!"
echo ""
echo "📦 Installers are in: desktop/dist/"
echo ""
ls -lh dist/ 2>/dev/null || echo "No dist directory found"
echo ""
