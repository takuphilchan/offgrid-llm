#!/bin/bash
# Update version across all files
# Usage: ./scripts/update-version.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
VERSION_FILE="$ROOT_DIR/VERSION"

# Read version from VERSION file
if [ ! -f "$VERSION_FILE" ]; then
    echo "ERROR: VERSION file not found at $VERSION_FILE"
    exit 1
fi

VERSION=$(cat "$VERSION_FILE" | tr -d '\n\r ')
echo "Updating version to: $VERSION"

# Update package.json
if [ -f "$ROOT_DIR/desktop/package.json" ]; then
    echo "Updating desktop/package.json..."
    sed -i "s/\"version\": \"[^\"]*\"/\"version\": \"$VERSION\"/" "$ROOT_DIR/desktop/package.json"
fi

# Update desktop/index.html (version displays)
if [ -f "$ROOT_DIR/desktop/index.html" ]; then
    echo "Updating desktop/index.html..."
    sed -i "s/v[0-9]\+\.[0-9]\+\.[0-9]\+/v$VERSION/g" "$ROOT_DIR/desktop/index.html"
fi

# Update scripts/build-all.sh
if [ -f "$ROOT_DIR/scripts/build-all.sh" ]; then
    echo "Updating scripts/build-all.sh..."
    sed -i "s/^VERSION=\"[^\"]*\"/VERSION=\"$VERSION\"/" "$ROOT_DIR/scripts/build-all.sh"
fi

# Update internal/p2p/discovery.go
if [ -f "$ROOT_DIR/internal/p2p/discovery.go" ]; then
    echo "Updating internal/p2p/discovery.go..."
    sed -i "s/Version: \"[^\"]*\"/Version: \"$VERSION\"/" "$ROOT_DIR/internal/p2p/discovery.go"
fi

# Note: cmd/offgrid/main.go uses ldflags, so it's set at build time
echo ""
echo "Version updated to $VERSION in all files."
echo ""
echo "Note: cmd/offgrid/main.go version is set via ldflags at build time."
echo "Build with: go build -ldflags=\"-X main.Version=$VERSION\" ..."
