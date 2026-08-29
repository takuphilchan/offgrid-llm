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

# npm updates package.json and its lockfile together. --allow-same-version keeps
# this command idempotent for release retries.
if [ -f "$ROOT_DIR/desktop/package.json" ]; then
    echo "Updating desktop package metadata..."
    npm version "$VERSION" --prefix "$ROOT_DIR/desktop" --no-git-tag-version --allow-same-version
fi

if [ -f "$ROOT_DIR/web/app/package.json" ]; then
    echo "Updating web package metadata..."
    npm version "$VERSION" --prefix "$ROOT_DIR/web/app" --no-git-tag-version --allow-same-version
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

# Update python/pyproject.toml
# Note: Python client version is managed separately (currently 0.1.x vs system 0.2.x)
# if [ -f "$ROOT_DIR/python/pyproject.toml" ]; then
#     echo "Updating python/pyproject.toml..."
#     sed -i "s/version = \"[^\"]*\"/version = \"$VERSION\"/" "$ROOT_DIR/python/pyproject.toml"
# fi

# Update internal/agents/mcp_client.go
if [ -f "$ROOT_DIR/internal/agents/mcp_client.go" ]; then
    echo "Updating internal/agents/mcp_client.go..."
    sed -i "s/\"version\": \"[0-9]*\.[0-9]*\.[0-9]*\"/\"version\": \"$VERSION\"/" "$ROOT_DIR/internal/agents/mcp_client.go"
fi

# Server and desktop runtime versions are injected by the build. Do not rewrite
# Go source or generated UI files to publish a release.
echo ""
echo "Version updated to $VERSION in all files."
echo ""
echo "For development builds, cmd/offgrid/main.go reads VERSION file automatically."
echo "For release builds, use: go build -ldflags=\"-X main.Version=$VERSION\" ..."
