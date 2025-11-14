#!/bin/bash
# Quick script to check GitHub Actions build status

REPO="takuphilchan/offgrid-llm"
TAG="v0.1.0"

echo "Checking build status for $TAG..."
echo ""

# Check if release exists
echo "=== Release Status ==="
if curl -sf "https://api.github.com/repos/$REPO/releases/tags/$TAG" > /dev/null; then
    echo "✅ Release exists: https://github.com/$REPO/releases/tag/$TAG"
    
    # Count assets
    ASSET_COUNT=$(curl -s "https://api.github.com/repos/$REPO/releases/tags/$TAG" | grep -o '"name":' | wc -l)
    echo "📦 Assets found: $ASSET_COUNT"
    
    # List assets
    echo ""
    echo "=== Available Downloads ==="
    curl -s "https://api.github.com/repos/$REPO/releases/tags/$TAG" | \
        grep '"name":' | \
        sed 's/.*"name": "\(.*\)".*/  - \1/'
else
    echo "⏳ Release not created yet"
fi

echo ""
echo "=== GitHub Actions ==="
echo "🔗 https://github.com/$REPO/actions"
echo ""
echo "View in browser? (y/n)"
read -n 1 answer
if [ "$answer" = "y" ]; then
    xdg-open "https://github.com/$REPO/actions" 2>/dev/null || \
    open "https://github.com/$REPO/actions" 2>/dev/null || \
    echo "Please open: https://github.com/$REPO/actions"
fi
