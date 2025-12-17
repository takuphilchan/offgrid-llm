#!/bin/bash
# Sync web UI to desktop app
# Run this after making changes to web/ui to update the desktop app

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WEB_UI="$PROJECT_ROOT/web/ui"
DESKTOP="$PROJECT_ROOT/desktop"

echo "Syncing web UI to desktop app..."

# Sync HTML
if [ -f "$WEB_UI/index.html" ]; then
    cp "$WEB_UI/index.html" "$DESKTOP/index.html"
    echo "✓ Synced index.html"
fi

# Sync CSS
if [ -d "$WEB_UI/css" ]; then
    mkdir -p "$DESKTOP/css"
    cp -r "$WEB_UI/css/"* "$DESKTOP/css/"
    echo "✓ Synced css/"
fi

# Sync JS
if [ -d "$WEB_UI/js" ]; then
    mkdir -p "$DESKTOP/js"
    cp -r "$WEB_UI/js/"* "$DESKTOP/js/"
    echo "✓ Synced js/"
fi

echo ""
echo "Desktop app synced successfully!"
echo ""
echo "Files synced:"
find "$DESKTOP" -maxdepth 2 \( -name "*.html" -o -name "*.css" -o -name "*.js" \) -type f | while read f; do
    lines=$(wc -l < "$f")
    echo "  $(basename "$f"): $lines lines"
done
