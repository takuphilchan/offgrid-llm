#!/bin/bash
# Build script for OffGrid LLM Desktop
# Builds desktop applications for all platforms

set -e

cd "$(dirname "$0")"

echo "Building OffGrid LLM Desktop Applications..."
echo ""

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    npm install
fi

# Clean previous builds
echo "Cleaning previous builds..."
rm -rf dist/

# Build for current platform
if [ "$1" = "linux" ]; then
    echo "Building for Linux..."
    npm run build:linux
elif [ "$1" = "mac" ]; then
    echo "Building for macOS..."
    npm run build:mac
elif [ "$1" = "win" ]; then
    echo "Building for Windows..."
    npm run build:win
elif [ "$1" = "all" ]; then
    echo "Building for all platforms..."
    npm run build:all
else
    echo "Building for current platform..."
    npm run build
fi

echo ""
echo "Build complete! Installers are in desktop/dist/"
ls -lh dist/
