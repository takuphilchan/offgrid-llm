#!/usr/bin/env bash

set -euo pipefail

desktop_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(cd "$desktop_dir/.." && pwd)"
target="${1:-current}"

cd "$project_root/web/app"
npm ci
npm run api:check
npm run check
npm run build

cd "$desktop_dir"
npm ci

case "$target" in
  linux) npm run build:linux ;;
  mac) npm run build:mac ;;
  win) npm run build:win ;;
  current) npm run build ;;
  *)
    echo "Usage: ./build.sh [current|linux|mac|win]" >&2
    exit 2
    ;;
esac

echo "Desktop artifacts are in $desktop_dir/dist"
