#!/usr/bin/env bash
# Compatibility entry point for the published scripts/install.sh raw URL.
set -e
export OFFGRID_INSTALL_LEGACY=yes

script_path="${BASH_SOURCE[0]-}"
if [ -n "$script_path" ] && [ -f "$script_path" ]; then
    canonical_local="$(cd "$(dirname "$script_path")/.." && pwd)/install.sh"
    if [ -f "$canonical_local" ]; then
        exec bash "$canonical_local" "$@"
    fi
fi

# When streamed from a raw URL, there is no neighboring checkout. Fetch the
# canonical entry point separately and keep stdin free for its existing prompts.
canonical_url='https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh'
tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT
curl -fsSL "$canonical_url" -o "$tmp_file"
bash "$tmp_file" "$@"
