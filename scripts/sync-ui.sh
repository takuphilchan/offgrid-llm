#!/bin/bash
# Sync web UI between all locations:
# - web/ui (source)
# - desktop/ (Electron app)
# - /var/lib/offgrid/web/ui (installed system)
#
# Run this after making changes to keep all UIs in sync

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WEB_UI="$PROJECT_ROOT/web/ui"
DESKTOP="$PROJECT_ROOT/desktop"
SYSTEM_UI="/var/lib/offgrid/web/ui"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "=========================================="
echo "  OffGrid UI Sync Tool"
echo "=========================================="
echo ""

# Sync to Desktop App
echo -e "${YELLOW}=== Syncing to Desktop App ===${NC}"

# Sync HTML
if [ -f "$WEB_UI/index.html" ]; then
    cp "$WEB_UI/index.html" "$DESKTOP/index.html"
    echo -e "${GREEN}✓${NC} Synced index.html"
fi

# Sync CSS
if [ -d "$WEB_UI/css" ]; then
    mkdir -p "$DESKTOP/css"
    cp -r "$WEB_UI/css/"* "$DESKTOP/css/"
    echo -e "${GREEN}✓${NC} Synced css/"
fi

# Sync JS
if [ -d "$WEB_UI/js" ]; then
    mkdir -p "$DESKTOP/js"
    cp -r "$WEB_UI/js/"* "$DESKTOP/js/"
    echo -e "${GREEN}✓${NC} Synced js/"
fi

echo -e "${GREEN}Desktop app synced!${NC}"
echo ""

# Sync to System UI (if installed and writable)
echo -e "${YELLOW}=== Syncing to System UI ($SYSTEM_UI) ===${NC}"

if [ -d "$SYSTEM_UI" ]; then
    # Check if we can write to the directory
    if [ -w "$SYSTEM_UI" ] || [ "$EUID" -eq 0 ]; then
        # Sync HTML
        if [ -f "$WEB_UI/index.html" ]; then
            cp "$WEB_UI/index.html" "$SYSTEM_UI/index.html"
            echo -e "${GREEN}✓${NC} Synced index.html"
        fi
        
        # Sync CSS
        if [ -d "$WEB_UI/css" ]; then
            mkdir -p "$SYSTEM_UI/css"
            cp -r "$WEB_UI/css/"* "$SYSTEM_UI/css/"
            echo -e "${GREEN}✓${NC} Synced css/"
        fi
        
        # Sync JS
        if [ -d "$WEB_UI/js" ]; then
            mkdir -p "$SYSTEM_UI/js"
            cp -r "$WEB_UI/js/"* "$SYSTEM_UI/js/"
            echo -e "${GREEN}✓${NC} Synced js/"
        fi
        
        echo -e "${GREEN}System UI synced!${NC}"
    else
        echo -e "${YELLOW}⚠ Cannot write to $SYSTEM_UI (need sudo)${NC}"
        echo "  Run with sudo to update system UI:"
        echo "  sudo $0"
    fi
else
    echo -e "${YELLOW}⚠ System UI not found at $SYSTEM_UI${NC}"
    echo "  (OffGrid may not be installed system-wide)"
fi

echo ""
echo "=========================================="
echo "  Sync Complete"
echo "=========================================="
echo ""
echo "Files in web/ui:"
find "$WEB_UI" -maxdepth 2 \( -name "*.html" -o -name "*.css" -o -name "*.js" \) -type f 2>/dev/null | while read f; do
    lines=$(wc -l < "$f")
    relpath="${f#$WEB_UI/}"
    echo "  $relpath: $lines lines"
done
