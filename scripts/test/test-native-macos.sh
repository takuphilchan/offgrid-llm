#!/usr/bin/env bash
set -euo pipefail
if [[ "$(uname -s)" != Darwin || "${OFFGRID_NATIVE_ISOLATED_DESKTOP:-}" != 1 || "$(id -u)" == 0 ]]; then
  echo 'Use an unprivileged isolated macOS desktop with explicit native-test opt-in.' >&2
  exit 2
fi
case "$(id -un | tr '[:upper:]' '[:lower:]')" in
  offgridtest-*) ;;
  *) echo 'A disposable OffGridTest-* account is required; personal desktops are refused.' >&2; exit 2 ;;
esac
cd "$(dirname "$0")/../.."
native_test_dir=$(mktemp -d "${TMPDIR:-/tmp}/offgrid-native-macos.XXXXXX")
export OFFGRID_NATIVE_TEST_DIRECTORY="$native_test_dir"
export OFFGRID_NATIVE_DOCUMENT_FIXTURE="$native_test_dir/owned-documents"
xcrun clang -fobjc-arc -framework AppKit apps/computer/native/macos/document-fixture.m -o "$OFFGRID_NATIVE_DOCUMENT_FIXTURE"
echo 'Accessibility must already be granted to this test host. This runner never changes TCC or grants permissions.' >&2
go test -p 1 -v -timeout 90s ./internal/computer ./cmd/offgrid-computer -run TestNative -count=1
go build -trimpath -o "$native_test_dir/offgrid-computer" ./cmd/offgrid-computer
printf 'macOS native fixture evidence: %s\n' "$native_test_dir"
