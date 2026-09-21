#!/usr/bin/env bash
set -euo pipefail
if [[ "${OFFGRID_NATIVE_ISOLATED_DESKTOP:-}" != 1 || -z "${DISPLAY:-}" || -z "${DBUS_SESSION_BUS_ADDRESS:-}" || "$(id -u)" == 0 ]]; then
  echo 'Use an unprivileged isolated Xvfb + dbus-run-session desktop for this test.' >&2
  exit 2
fi
cd "$(dirname "$0")/../.."
native_test_dir=$(mktemp -d "${TMPDIR:-/tmp}/offgrid-native-linux.XXXXXX")
export XDG_RUNTIME_DIR="$native_test_dir/runtime"
export XDG_CONFIG_HOME="$native_test_dir/config"
mkdir -m 700 "$XDG_RUNTIME_DIR" "$XDG_CONFIG_HOME"
cc computer/native/linux/fixture.c -o "$native_test_dir/fixture" $(pkg-config --cflags --libs gtk+-3.0)
cc computer/native/linux/consent.c -o "$native_test_dir/offgrid-computer-ui" $(pkg-config --cflags --libs gtk+-3.0 json-glib-1.0)
openbox >"$native_test_dir/openbox.log" 2>&1 &
native_wm_pid=$!
trap 'kill -TERM "$native_wm_pid" 2>/dev/null || true' EXIT
export OFFGRID_NATIVE_LINUX_FIXTURE="$native_test_dir/fixture"
export OFFGRID_NATIVE_LINUX_CONSENT="$native_test_dir/offgrid-computer-ui"
go test -p 1 -tags offgrid_native -v -timeout 90s ./internal/computer ./cmd/offgrid-computer -run TestNative -count=1
# This disposable fixture may mount a checkout owned by a different UID.
# Packaged builds record provenance separately; do not change Git trust globally.
go build -buildvcs=false -trimpath -tags offgrid_native -o "$native_test_dir/offgrid-computer" ./cmd/offgrid-computer
printf 'Linux native fixture evidence: %s\n' "$native_test_dir"
