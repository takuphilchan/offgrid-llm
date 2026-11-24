#!/usr/bin/env bash
# Compatibility entry point. Prepared assets only; no network or installation.
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec node "$SCRIPT_DIR/create-usb-package.mjs" "$@"
