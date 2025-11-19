#!/bin/bash
# Version management script
# Reads VERSION file and returns the version string

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION_FILE="$SCRIPT_DIR/../VERSION"

if [ ! -f "$VERSION_FILE" ]; then
    echo "dev"
    exit 0
fi

cat "$VERSION_FILE" | tr -d '\n\r '
