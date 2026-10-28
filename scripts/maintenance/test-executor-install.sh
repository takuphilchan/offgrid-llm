#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture_root="$(mktemp -d /tmp/offgrid-executor-install.XXXXXXXX)"
cleanup(){
  case "$fixture_root" in /tmp/offgrid-executor-install.*) [ ! -L "$fixture_root" ] && rm -rf -- "$fixture_root" ;; *) return 1 ;; esac
}
trap cleanup EXIT
# Load only the reviewed installer function, never the live installer workflow.
eval "$(sed -n '/^install_cli() {/,/^}/p' "$repo_root/install.sh")"
stop_running_processes(){ :; }
spin(){ :; };spin_done(){ :; };spin_fail(){ :; }
LEGACY_INSTALL=no
INSTALL_DIR="$fixture_root/bin"
mkdir -p "$INSTALL_DIR" "$fixture_root/bundle/execution-runtime" "$fixture_root/legacy"
for bundle in bundle legacy; do
  printf 'fixture cli\n' > "$fixture_root/$bundle/offgrid"
  printf 'fixture inference\n' > "$fixture_root/$bundle/llama-server"
done
printf 'fixture worker\n' > "$fixture_root/bundle/execution-runtime/offgrid-executor"
printf '{"version":"0.4.20"}\n' > "$fixture_root/bundle/execution-runtime/execution-pack.json"
VERSION=v0.4.20
install_cli "$fixture_root/bundle" linux
cmp "$fixture_root/bundle/execution-runtime/offgrid-executor" "$INSTALL_DIR/execution-runtime/0.4.20/offgrid-executor"
VERSION=v0.4.21
install_cli "$fixture_root/bundle" linux
test -f "$INSTALL_DIR/execution-runtime/0.4.20/execution-pack.json"
test -f "$INSTALL_DIR/execution-runtime/0.4.21/execution-pack.json"
VERSION=v0.4.19
install_cli "$fixture_root/legacy" linux
test ! -e "$INSTALL_DIR/execution-runtime/0.4.19"
VERSION=../../forbidden
printf 'preserved cli\n' > "$INSTALL_DIR/offgrid"
if install_cli "$fixture_root/bundle" linux; then printf 'invalid version accepted\n' >&2; exit 1; fi
test ! -e "$fixture_root/forbidden"
test "$(< "$INSTALL_DIR/offgrid")" = 'preserved cli'
printf 'executor install fixtures passed: versioned rollback, old bundles and pre-effect path rejection\n'
