#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture_dir="$(mktemp -d)"
cleanup() {
    rm -f "$fixture_dir/local.txt" "$fixture_dir/stream.txt" \
        "$fixture_dir/wrapper-env-local.txt" "$fixture_dir/wrapper-env-stream.txt" \
        "$fixture_dir/install.sh" "$fixture_dir/scripts/install.sh" \
        "$fixture_dir/attempts.txt" "$fixture_dir/commands.txt" \
        "$fixture_dir/offgrid-unit.txt" \
        "$fixture_dir/checksums.sha256" "$fixture_dir/offgrid"
    rmdir "$fixture_dir/scripts" 2>/dev/null || true
    rmdir "$fixture_dir"
}
trap cleanup EXIT

# Help must exit before detection, downloading, stopping processes, or writing
# installed files. Test both public modes, including the published pipe form.
OFFGRID_TEST_CANONICAL="$repo_root/install.sh"
export OFFGRID_TEST_CANONICAL
curl() {
    case " $* " in
        *'https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh'*) ;;
        *) printf '%s\n' 'unexpected canonical URL' >&2; return 1 ;;
    esac
    local output=""
    while [ "$#" -gt 0 ]; do
        if [ "$1" = '-o' ]; then
            output="$2"
            break
        fi
        shift
    done
    [ -n "$output" ] || return 1
    cp "$OFFGRID_TEST_CANONICAL" "$output"
}
export -f curl
HOME="$fixture_dir" bash "$repo_root/scripts/install.sh" --help > "$fixture_dir/local.txt"
HOME="$fixture_dir" bash -s -- --help < "$repo_root/scripts/install.sh" > "$fixture_dir/stream.txt"
cmp "$fixture_dir/local.txt" "$fixture_dir/stream.txt"
grep -q 'OffGrid' "$fixture_dir/local.txt"
grep -q 'AUTOSTART' "$fixture_dir/local.txt"
printf '%s\n' 'installer local and streamed entry-point fixtures passed'

# A disposable canonical script proves the wrapper forwards supported env and
# arguments in both forms, rather than merely printing its own help.
mkdir "$fixture_dir/scripts"
cp "$repo_root/scripts/install.sh" "$fixture_dir/scripts/install.sh"
printf '%s\n' '#!/usr/bin/env bash' 'printf "%s:%s:%s:%s\n" "$OFFGRID_INSTALL_LEGACY" "$VERSION" "$AUTOSTART" "$1"' > "$fixture_dir/install.sh"
VERSION=v0.test AUTOSTART=no bash "$fixture_dir/scripts/install.sh" --probe > "$fixture_dir/wrapper-env-local.txt"
OFFGRID_TEST_CANONICAL="$fixture_dir/install.sh"
export OFFGRID_TEST_CANONICAL
VERSION=v0.test AUTOSTART=no bash -s -- --probe < "$repo_root/scripts/install.sh" > "$fixture_dir/wrapper-env-stream.txt"
cmp "$fixture_dir/wrapper-env-local.txt" "$fixture_dir/wrapper-env-stream.txt"
grep -Fxq 'yes:v0.test:no:--probe' "$fixture_dir/wrapper-env-local.txt"
OFFGRID_TEST_CANONICAL="$repo_root/install.sh"
export OFFGRID_TEST_CANONICAL
unset -f curl

# Source definitions only; the production main entry point must never run in
# this fixture. Function stubs below cannot copy binaries or touch systemd.
source <(sed '/^# Help is safe for local and streamed compatibility checks\./,$d' "$repo_root/install.sh")
uname() {
    if [ "${1:-}" = '-m' ]; then printf '%s\n' riscv64; else command uname "$@"; fi
}
if (detect_arch >/dev/null 2>&1); then
    printf '%s\n' 'expected unsupported architecture to fail' >&2
    exit 1
fi
unset -f uname
download_with_progress() {
    printf '%s\n' "$1" >> "$fixture_dir/attempts.txt"
    case "$1" in
        *-vulkan-avx512.tar.gz) [ "$download_case" = "gpu" ] ;;
        *-cpu-avx512.tar.gz) [ "$download_case" = "cpu" ] ;;
        *-cpu-avx2.tar.gz) [ "$download_case" = "avx2" ] ;;
        *) return 1 ;;
    esac
}
tar() { [ "${fixture_fail_tar:-no}" != 'yes' ]; }

for download_case in gpu cpu avx2 none; do
    : > "$fixture_dir/attempts.txt"
    if [ "$download_case" = "none" ]; then
        if download_cli_bundle linux amd64 v0.test avx512 "$fixture_dir" vulkan >/dev/null; then
            printf '%s\n' 'expected exhausted downloads to fail' >&2
            exit 1
        fi
    else
        result="$(download_cli_bundle linux amd64 v0.test avx512 "$fixture_dir" vulkan)"
        case "$download_case:$result" in
            gpu:offgrid-v0.test-linux-amd64-vulkan-avx512|\
            cpu:offgrid-v0.test-linux-amd64-cpu-avx512|\
            avx2:offgrid-v0.test-linux-amd64-cpu-avx2) ;;
            *) printf 'unexpected candidate: %s:%s\n' "$download_case" "$result" >&2; exit 1 ;;
        esac
    fi
    grep -q 'offgrid-v0.test-linux-amd64-vulkan-avx512.tar.gz' "$fixture_dir/attempts.txt"
    if [ "$download_case" = "avx2" ]; then
        grep -q 'offgrid-v0.test-linux-amd64-cpu-avx512.tar.gz' "$fixture_dir/attempts.txt"
        grep -q 'offgrid-v0.test-linux-amd64-cpu-avx2.tar.gz' "$fixture_dir/attempts.txt"
    fi
done

: > "$fixture_dir/attempts.txt"
download_case=cpu
result="$(download_cli_bundle linux amd64 v0.test avx512 "$fixture_dir")"
[ "$result" = 'offgrid-v0.test-linux-amd64-cpu-avx512' ]
[ "$(wc -l < "$fixture_dir/attempts.txt")" -eq 1 ]
grep -q 'offgrid-v0.test-linux-amd64-cpu-avx512.tar.gz' "$fixture_dir/attempts.txt"
fixture_fail_tar=yes
if download_cli_bundle linux amd64 v0.test avx512 "$fixture_dir" cpu >/dev/null 2>&1; then
    printf '%s\n' 'expected archive extraction failure to fail' >&2
    exit 1
fi
fixture_fail_tar=no

LEGACY_INSTALL=yes
printf 'fixture binary\n' > "$fixture_dir/offgrid"
(cd "$fixture_dir" && sha256sum offgrid > checksums.sha256)
verify_legacy_checksums "$fixture_dir"
printf 'changed\n' > "$fixture_dir/offgrid"
if verify_legacy_checksums "$fixture_dir" 2>/dev/null; then
    printf '%s\n' 'expected checksum mismatch to fail' >&2
    exit 1
fi

USER=fixture
AUTOSTART=yes
id() { printf '%s\n' 0; }
tee() { printf '%s\n' "tee $*" >> "$fixture_dir/commands.txt"; }
mkdir() { printf '%s\n' "mkdir $*" >> "$fixture_dir/commands.txt"; }
systemctl() {
    printf '%s\n' "systemctl $*" >> "$fixture_dir/commands.txt"
    if [ "${fixture_fail_start:-no}" = 'yes' ] && [ "${1:-}" = 'start' ]; then return 1; fi
}
setup_legacy_systemd linux
grep -Fq 'tee /etc/systemd/system/offgrid@.service' "$fixture_dir/commands.txt"
grep -Fq 'systemctl enable offgrid@fixture.service' "$fixture_dir/commands.txt"
grep -Fq 'systemctl start offgrid@fixture.service' "$fixture_dir/commands.txt"
awk '/^\[Unit\]$/{capture=1} capture && /^UNIT$/{exit} capture {print}' "$repo_root/install.sh" > "$fixture_dir/offgrid-unit.txt"
cmp "$repo_root/installers/systemd/offgrid@.service" "$fixture_dir/offgrid-unit.txt"
: > "$fixture_dir/commands.txt"
AUTOSTART=no
setup_legacy_systemd linux
if grep -Eq 'systemctl (enable|start) offgrid@fixture.service' "$fixture_dir/commands.txt"; then
    printf '%s\n' 'AUTOSTART=no attempted to enable or start the service' >&2
    exit 1
fi
fixture_fail_start=yes
AUTOSTART=yes
if setup_legacy_systemd linux >/dev/null 2>&1; then
    printf '%s\n' 'expected systemd start failure to fail installation' >&2
    exit 1
fi
printf '%s\n' 'installer candidate, checksum, and isolated service fixtures passed'
