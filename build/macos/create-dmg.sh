#!/bin/bash
# macOS DMG Creator
# Creates distributable DMG file with OffGrid.app

set -e

VERSION="$1"
ARCH="$2"

if [ -z "$VERSION" ] || [ -z "$ARCH" ]; then
    echo "Usage: $0 <version> <arch>"
    echo "Example: $0 v0.1.0 arm64"
    exit 1
fi

APP_NAME="OffGrid"
BUNDLE_DIR="${APP_NAME}.app"
DMG_NAME="offgrid-${VERSION}-darwin-${ARCH}.dmg"
VOLUME_NAME="OffGrid LLM ${VERSION}"

if [ ! -d "$BUNDLE_DIR" ]; then
    echo "Error: $BUNDLE_DIR not found"
    echo "Run create-app-bundle.sh first"
    exit 1
fi

echo "Creating DMG: $DMG_NAME"
echo "→ Checking for create-dmg..."

# Check if create-dmg is installed
if ! command -v create-dmg &> /dev/null; then
    echo "Installing create-dmg..."
    brew install create-dmg
fi

# Clean up any existing DMG
rm -f "$DMG_NAME"
rm -rf dmg_temp

# Create temporary directory for DMG contents
mkdir -p dmg_temp

# Copy app bundle
echo "→ Copying application bundle..."
cp -R "$BUNDLE_DIR" dmg_temp/

# Create Applications symlink
echo "→ Creating Applications symlink..."
ln -s /Applications dmg_temp/Applications

# Create README
cat > dmg_temp/README.txt << EOF
OffGrid LLM ${VERSION}
======================

Installation
------------

1. Drag OffGrid.app to Applications folder
2. Open Terminal and run:
   /Applications/OffGrid.app/Contents/Resources/install.sh

This will create symlinks in /usr/local/bin so you can use:
  - offgrid
  - llama-server

Quick Start
-----------

  offgrid --help
  offgrid server start
  offgrid chat

Documentation
-------------

Visit: https://github.com/takuphilchan/offgrid-llm

System Requirements
-------------------

- macOS 11.0 or later
- 8GB RAM minimum (16GB recommended)
- 10GB free disk space

EOF

# Create the DMG
echo "→ Building DMG..."
create-dmg \
  --volname "$VOLUME_NAME" \
  --volicon "$BUNDLE_DIR/Contents/Resources/AppIcon.icns" 2>/dev/null || true \
  --window-pos 200 120 \
  --window-size 800 400 \
  --icon-size 100 \
  --icon "${APP_NAME}.app" 200 190 \
  --hide-extension "${APP_NAME}.app" \
  --app-drop-link 600 185 \
  --hdiutil-quiet \
  "$DMG_NAME" \
  dmg_temp/

# Clean up
echo "→ Cleaning up..."
rm -rf dmg_temp

echo ""
echo "✓ DMG created: $DMG_NAME"
echo ""

# Show DMG info
if command -v hdiutil &> /dev/null; then
    echo "DMG Information:"
    hdiutil imageinfo "$DMG_NAME" | grep -E "Format|Size|Compressed" || true
fi

echo ""
echo "Distribution Package Ready!"
echo ""
echo "Next steps:"
echo "  1. Test the DMG on a clean macOS system"
echo "  2. (Optional) Code sign: codesign --deep --force --verify --verbose --sign 'Developer ID' OffGrid.app"
echo "  3. (Optional) Notarize with Apple"
echo "  4. Upload to GitHub Releases"
echo ""
