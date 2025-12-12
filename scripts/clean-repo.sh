#!/bin/bash
# Clean up repository artifacts

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Cleaning repository..."

# Remove binaries
if [ -f "$ROOT_DIR/offgrid" ]; then
    echo "Removing offgrid binary..."
    rm -f "$ROOT_DIR/offgrid"
fi

# Remove build directory
if [ -d "$ROOT_DIR/build" ]; then
    echo "Removing build directory..."
    rm -rf "$ROOT_DIR/build"
fi

# Remove logs
echo "Removing log files..."
rm -f "$ROOT_DIR"/*.log

# Remove Python artifacts
echo "Removing Python artifacts..."
rm -rf "$ROOT_DIR/python/dist"
rm -rf "$ROOT_DIR/python/build"
rm -rf "$ROOT_DIR/python/"*.egg-info
find "$ROOT_DIR" -type d -name "__pycache__" -exec rm -rf {} +

# Remove Node modules (optional, but good for deep clean)
if [ -d "$ROOT_DIR/node_modules" ]; then
    echo "Removing root node_modules..."
    rm -rf "$ROOT_DIR/node_modules"
fi

if [ -d "$ROOT_DIR/desktop/node_modules" ]; then
    echo "Removing desktop node_modules..."
    rm -rf "$ROOT_DIR/desktop/node_modules"
fi

if [ -d "$ROOT_DIR/desktop/dist" ]; then
    echo "Removing desktop dist..."
    rm -rf "$ROOT_DIR/desktop/dist"
fi

# Remove coverage files
rm -f "$ROOT_DIR/coverage.txt" "$ROOT_DIR/coverage.html"

echo "Cleanup complete."
