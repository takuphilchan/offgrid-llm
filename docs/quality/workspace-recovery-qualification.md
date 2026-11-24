# Installed workspace recovery qualification

This is the runnable procedure for the remaining platform and participant gates
in [encrypted workspace backups](../../openspec/changes/encrypted-workspace-backups/tasks.md).
It records no successful installed qualification by itself. The automated
[source-composed evidence](evidence/desktop-encrypted-recovery-2026-11-20.md) remains
separate. Run one native edition at a time on the actual Windows, Linux or macOS
filesystem; WSL does not establish native Linux desktop or physical crash behavior.

## Required scope and materials

Use a separately authorized disposable OS account or VM, an installed test edition
and its matching Go binary/UI build, a local interactive desktop, and a new private
profile. Installation, native dialogs, assistive technology and failure injection
must be included in that qualification's explicit scope. Do not attach to an
existing service or use a personal profile. Retain package/binary SHA-256, version,
OS/architecture, filesystem/mount options, free disk/RAM, language and edition.

Prepare synthetic conversations, documents, credentials and committed SQLite/WAL
records. Include historical interrupted and uncertain task records plus a harmless
effect sentinel whose original bytes are known. Retain source inventory/digests and
database row counts. Keep native recipient identities and the passphrase outside
the workspace; use newly generated test secrets. Select no model download.

The installed desktop accepts an explicit absolute `OFFGRID_DESKTOP_HOME`, which
also isolates Electron cookies/preferences, and `OFFGRID_PORT`. Choose an unused
loopback port and verify it is still unused before launch. These examples assume
the installed executable path has already been selected and inspected:

```powershell
$recoveryProfile = Join-Path ([IO.Path]::GetTempPath()) ('offgrid-recovery-qualified-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $recoveryProfile | Out-Null
$env:OFFGRID_DESKTOP_HOME = $recoveryProfile
$env:OFFGRID_PORT = '23617'
$env:OFFGRID_CONFIG = Join-Path $recoveryProfile 'qualification-config.json'
[IO.File]::WriteAllText($env:OFFGRID_CONFIG, '{"enable_p2p":false,"prewarm_models":false}', [Text.UTF8Encoding]::new($false))
$env:OFFGRID_INFERENCE_ENDPOINTS_FILE = ''
if (Get-NetTCPConnection -LocalPort 23617 -ErrorAction SilentlyContinue) { throw 'Select an unused port' }
# Set this to the separately installed, inspected test executable.
$recoveryExecutable = 'C:\Isolated Test Installation\OffGrid LLM Desktop.exe'
Get-FileHash -LiteralPath $recoveryExecutable -Algorithm SHA256
& $recoveryExecutable
```

```sh
recovery_profile=$(mktemp -d "${TMPDIR:-/tmp}/offgrid-recovery-qualified-XXXXXX")
chmod 700 "$recovery_profile"
export OFFGRID_DESKTOP_HOME="$recovery_profile" OFFGRID_PORT=23617
export OFFGRID_CONFIG="$recovery_profile/qualification-config.json"
export OFFGRID_INFERENCE_ENDPOINTS_FILE=''
printf '%s\n' '{"enable_p2p":false,"prewarm_models":false}' > "$OFFGRID_CONFIG"
# Verify the selected port is unused with the platform's socket inventory first.
# Select one installed edition, never an unrelated running application.
recovery_executable='/absolute/isolated/install/OffGrid'
# macOS example: /absolute/isolated/install/OffGrid.app/Contents/MacOS/OffGrid
"$recovery_executable"
```

If the desktop reports another service or a compatibility mismatch, stop the
qualification and record the condition; do not attach or repair a live workspace.
Inspect Settings to confirm the exact new profile/data paths and matching build.
After the session, unset these qualification environment variables. Retain test artifacts for
review; delete only the explicitly identified disposable profile after inspection.

## Required journeys and evidence

1. With native file pickers and the host password dialog, create both recipient-key
   and passphrase backups. Confirm the service drains first, the ciphertext is
   private at creation, no plaintext archive spool exists, and success requires
   verification/publication. Record file count/bytes, elapsed time and peak RSS.
2. Verify with the correct identity/passphrase, a different identity, a wrong
   passphrase, truncated data and a flipped ciphertext byte. Require truthful
   authentication versus legacy ZIP status, no fallback, no leaked secrets and no
   extracted destination for rejected input. Independently decrypt a copy with
   the pinned age CLI and compare the ZIP inventory; keep the decrypted test copy
   private and distinguish this deliberate inspection from application behavior.
3. Restore to a fresh sibling directory, inspect all manifest hashes and committed
   database records, and confirm the original directory remains unchanged.
   Inspect the restored agent database while disconnected: interrupted/uncertain
   statuses and unresolved call evidence must remain; the effect sentinel must
   not change. Restore itself must not start that workspace.
4. Cancel during authentication and extraction; record time to acknowledged exit,
   publication state and any retained path. Create a competing destination during
   confirmation and again before publication. Require no overwrite/merge. Exercise
   actual permission, disk-full, sharing-lock and filesystem errors using only a
   dedicated scratch volume or fixture-owned handles; record the exact OS error
   and preserved state, not a simulated permission result.
5. Close/reload the renderer, change fixture account, lock/suspend, and interrupt
   the maintenance helper at agreed stages. Confirm no stale consent survives,
   no active work is replayed and no unconfirmed helper becomes success. Inspect
   retained ciphertext/private plaintext stages before a separately explicit
   cleanup. Physical power-loss claims require a dedicated machine/volume and a
   separate crash protocol; a killed process is not equivalent evidence.
6. Choose explicit Resume original workspace. Confirm its identity/path, inspect
   its task states and sentinel again, and verify a competing service on its port
   prevents unsafe attachment. Only after inspection may a separate explicit test
   start the restored directory with the matching binary; that action belongs in
   the qualification record and is never an automatic restore consequence.

Do not include passphrases, private keys, secret-dialog screenshots, clipboard data
or full private manifests in support evidence. Record counts, public fixture
digests, bounded safe error codes and necessary retained paths. Inspect service,
host and Chromium persistence for the unique synthetic secret sentinel.

## Accessibility, language and participant gate

Within the existing [production-readiness](production-readiness.md) and M7 protocol,
run five formative participants before the 30-day pilot of at least 15 representative
people. Include administrator and nonadministrator contexts. Ask participants to
make a backup, verify an unknown archive, recover to a new directory, understand a
wrong-key/retained-plaintext warning, cancel, and resume original data without help.
Record completion, time, critical errors and interventions per task; preserve the
program's at-least-90-percent unassisted core-workflow target.

Use actual NVDA/Windows, Orca/Linux and VoiceOver/macOS where supported, keyboard-only
operation, 200-percent zoom, narrow/short windows, reduced motion and both themes.
Check native picker/password focus, secure input behavior, status announcements,
Cancel/Resume names and focus return. Native-language reviewers must review all 29
supported catalogs, RTL and regional number formatting. Automated DOM labels and
catalog completeness do not replace those reviewers.

Required resources still unavailable in this session include separately scoped
installed native Windows/Linux/macOS editions and accounts, native file-picker/
screen-reader participants, all-language reviewers, dedicated physical crash media,
and the formative/pilot time and people. Keep their tasks and overall change open
until actual reports are attached; do not populate this procedure with fixture
measurements as if they were installed results.
