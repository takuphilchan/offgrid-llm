#!/bin/bash
# Test the installer in a safe way (without sudo)

export INSTALL_DIR="/tmp/offgrid-test-install"
export VERSION="v0.9.0-rc1"

echo "Testing installer with:"
echo "  VERSION=$VERSION"
echo "  INSTALL_DIR=$INSTALL_DIR"
echo ""

mkdir -p "$INSTALL_DIR"

# Run installer (will fail at sudo step, which is expected)
./install.sh 2>&1 | head -50
