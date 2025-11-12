#!/bin/bash
# macOS Application Bundle Creator
# Creates OffGrid.app bundle with proper structure

set -e

OFFGRID_BINARY="$1"
LLAMA_SERVER_BINARY="$2"
VERSION="$3"

if [ -z "$OFFGRID_BINARY" ] || [ -z "$LLAMA_SERVER_BINARY" ] || [ -z "$VERSION" ]; then
    echo "Usage: $0 <offgrid-binary> <llama-server-binary> <version>"
    exit 1
fi

APP_NAME="OffGrid"
BUNDLE_DIR="${APP_NAME}.app"

echo "Creating ${APP_NAME}.app bundle..."

# Clean up existing bundle
rm -rf "$BUNDLE_DIR"

# Create bundle structure
mkdir -p "$BUNDLE_DIR/Contents"
mkdir -p "$BUNDLE_DIR/Contents/MacOS"
mkdir -p "$BUNDLE_DIR/Contents/Resources"
mkdir -p "$BUNDLE_DIR/Contents/Helpers"

# Copy binaries
echo "→ Copying binaries..."
cp "$OFFGRID_BINARY" "$BUNDLE_DIR/Contents/MacOS/offgrid"
cp "$LLAMA_SERVER_BINARY" "$BUNDLE_DIR/Contents/Helpers/llama-server"
chmod +x "$BUNDLE_DIR/Contents/MacOS/offgrid"
chmod +x "$BUNDLE_DIR/Contents/Helpers/llama-server"

# Create launcher script that sets up PATH
cat > "$BUNDLE_DIR/Contents/MacOS/${APP_NAME}" << 'EOF'
#!/bin/bash
# OffGrid Launcher Script

# Get the directory containing this script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HELPERS_DIR="$DIR/../Helpers"

# Add helpers to PATH so offgrid can find llama-server
export PATH="$HELPERS_DIR:$PATH"

# Set config directory
export OFFGRID_CONFIG_DIR="$HOME/Library/Application Support/OffGrid"
mkdir -p "$OFFGRID_CONFIG_DIR"

# Launch offgrid
exec "$DIR/offgrid" "$@"
EOF

chmod +x "$BUNDLE_DIR/Contents/MacOS/${APP_NAME}"

# Create Info.plist
echo "→ Creating Info.plist..."
cat > "$BUNDLE_DIR/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>com.offgrid.llm</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>OffGrid LLM</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSApplicationCategoryType</key>
    <string>public.app-category.developer-tools</string>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2025 OffGrid LLM. All rights reserved.</string>
</dict>
</plist>
EOF

# Create README in Resources
cat > "$BUNDLE_DIR/Contents/Resources/README.txt" << 'EOF'
OffGrid LLM
===========

Edge-optimized AI inference system for offline environments.

Usage
-----

1. Terminal Usage:
   /Applications/OffGrid.app/Contents/MacOS/OffGrid --help

2. Add to PATH (optional):
   echo 'export PATH="/Applications/OffGrid.app/Contents/MacOS:$PATH"' >> ~/.zshrc
   source ~/.zshrc

3. Then use:
   OffGrid server start
   OffGrid chat

Configuration
-------------
Config files: ~/Library/Application Support/OffGrid/

Documentation
-------------
Visit: https://github.com/takuphilchan/offgrid-llm

EOF

# Create installation script
cat > "$BUNDLE_DIR/Contents/Resources/install.sh" << 'EOF'
#!/bin/bash
# Install OffGrid LLM to system

set -e

echo "Installing OffGrid LLM..."

# Copy app to Applications
if [ -d "/Applications/OffGrid.app" ]; then
    echo "Removing existing installation..."
    sudo rm -rf "/Applications/OffGrid.app"
fi

echo "Copying to /Applications..."
sudo cp -R "OffGrid.app" "/Applications/"

# Create symlinks in /usr/local/bin
echo "Creating symlinks..."
sudo ln -sf "/Applications/OffGrid.app/Contents/MacOS/offgrid" "/usr/local/bin/offgrid"
sudo ln -sf "/Applications/OffGrid.app/Contents/Helpers/llama-server" "/usr/local/bin/llama-server"

echo "✓ Installation complete!"
echo ""
echo "Usage:"
echo "  offgrid --help"
echo "  offgrid server start"
echo ""

EOF

chmod +x "$BUNDLE_DIR/Contents/Resources/install.sh"

echo "✓ Application bundle created: $BUNDLE_DIR"
echo ""
echo "Contents:"
echo "  MacOS/OffGrid        - Launcher script"
echo "  MacOS/offgrid        - Main binary"
echo "  Helpers/llama-server - Inference engine"
echo "  Resources/           - Documentation and install script"
echo ""
