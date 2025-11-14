#!/bin/bash
# Delete old GitHub releases using the GitHub API

REPO="takuphilchan/offgrid-llm"

echo "Deleting old releases from GitHub..."
echo ""

# Get release IDs
for tag in "v0.0.1" "v0.9.0-rc1"; do
    echo "Checking for release: $tag"
    RELEASE_ID=$(curl -s "https://api.github.com/repos/$REPO/releases/tags/$tag" | grep '"id":' | head -1 | sed 's/[^0-9]*//g')
    
    if [ -n "$RELEASE_ID" ]; then
        echo "Found release ID: $RELEASE_ID"
        echo "Delete with: gh release delete $tag --yes"
        echo "Or manually at: https://github.com/$REPO/releases/tag/$tag"
        echo ""
    else
        echo "No release found for $tag"
        echo ""
    fi
done

echo "---"
echo "To delete all releases, run:"
echo "  gh release delete v0.0.1 --yes"
echo "  gh release delete v0.9.0-rc1 --yes"
echo ""
echo "Or delete manually at:"
echo "  https://github.com/$REPO/releases"
