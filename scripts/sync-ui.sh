#!/usr/bin/env bash
# Compatibility entry point: the browser and desktop products now share the
# generated React bundle, so there are no UI source files to copy.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(cd "$script_dir/.." && pwd)"

cd "$project_root/web/app"
npm ci
npm run api:check
npm run check
npm run build

echo "UI bundle built at $project_root/web/dist"
echo "Electron and the Go server both consume this bundle."
