# CLI presentation follow-up — 2026-11-05

Candidate: v0.4.26, revision `f927194b-cli-preview-20261105`.
Installation evidence is recorded separately below; no release publication is implied.

## Findings and changes

The screenshot exposed conflicting presentation owners: local configuration
looked like live service allocation, zero threads was unexplained, essential
values were dimmed and long model IDs collided with fixed columns.

Shared human rendering now uses display-cell widths, normal-contrast values,
stacked metadata when needed, exact copyable IDs and compact headings. Coverage
extends through root/family help, agent profiles/session commands, model lists,
discovery/downloads, diagnostics, Knowledge, speech, integrations, configuration,
users/audit, batch/logs, aliases/favorites/templates and LoRA help. Advanced
structured records and package previews retain their schemas and field names.

Interactive chat bounds its header, viewport and shortcut footer to terminal
width. Downloads animate only in real terminals; pipes receive completion
without cursor controls. Existing agent streaming and cancellation remain intact.
No decorative animation claims model progress. Doctor no longer reports all
checks passed after failure. Benchmarks no longer invent first-token latency
from complete-response timing. Unsupported persona flags and Linux-only generic
recovery advice were removed. Full IDs replace unsafe byte slicing.

Registry quantization remains filename-inferred; Q4_K_L/Q4_K_XL can still be
reported as Q4_K_M there. Concise human inventories omit that misleading second
label; existing machine-readable metadata is not silently migrated.

## Verification

- `go test ./...`: passed; after replacing the redundant maxInt helper with Go's
  identical builtin, `go test ./cmd/offgrid` passed again.
- `go test ./internal/cli ./cmd/offgrid`: passed.
- `go vet ./internal/cli ./cmd/offgrid`: passed.
- New tests cover Unicode/exact IDs, 32/60/100-column inventories, 40/80/120-column
  family help, 20/32/60/100-column interactive chat, JSON suppression, buffered
  profile selection, invalid progress measurements and dependency-free help.
- Web `npm run api:check` and `npm run build`: passed.
- Desktop `npm test`: 129 passed, one platform-dependent skip, zero failures.
- Source-size ratchet, maintained documentation check and strict validation of
  workspace-outcome-experience: passed.
- Executed the candidate's root/agent help and parsed `--json version` on Windows.
- `go test -race ./internal/cli ./cmd/offgrid`: unavailable here because CGO is
  disabled and no GCC/Clang toolchain is on PATH; not counted as passed.

The first packaging command was rejected because PowerShell parsed its unquoted
configuration argument incorrectly; retry uses one quoted output argument.
No installed files were changed by the failed packaging attempt.

This is not qualification of every command on every platform. Human usability,
screen-reader evaluation and new macOS/Linux terminal verification remain unrun.
No models were downloaded, no permissions granted and no saved work replayed.

## Packaged and installed verification

- Packaged startup/recovery check passed on Windows. The first window appeared
  in 1,717 ms in the isolated fixture; this is not a general performance claim.
- Packaged terminal check passed: interactive CLI, profile selection, `/clear`,
  draft/cursor preservation across tabs, Unicode, search/copy, 200% toolbar fit,
  service identity and descendant cleanup on reload, close and application quit.
  The first run stopped on the old `Profile number:` assertion. Updating it to
  the new `Choose a profile number:` prompt made the complete check pass; no
  input or cleanup assertions were removed.
- Installed silently with exit 0 after confirming the user's app had exited.
  Desktop executable, app archive, bundled CLI, UI index and terminal host hashes
  match the staged package. All 381 workspace-data file hashes and the file count
  are unchanged. A verification script initially nested the JSON manifest array;
  correcting the script confirmed zero data differences.
- Installed `--json version` and root help passed. Version remains 0.4.26;
  private build revision is `f927194b-cli-preview-20261105`. This is a local
  follow-up build, not a newly published release.
- Installer SHA-256:
  `5B28FC26058C0C9698089929EBF40A177AE88075F9C0CFD3FD365AFFA02BABE7`.
- Rollback application, workspace-data and desktop-profile backups are retained
  outside the repository under `D:/OffGrid-install-backups/v0.4.26-before-cli-20261105-181132`.
  Private detailed manifests/logs are in ignored `build/cli-review/`.
- No installed OffGrid/model processes remained after verification. Normal app
  launch is left to the user. Use the app's OffGrid CLI/Agent entries for the
  matching bundled CLI; no WSL or global PATH installation was changed.

## Session-menu and progress follow-up, 2026-11-06

- Command selection now uses terminal-native inverse highlighting. Narrow panes
  place the selected command's description below the choices; viewport height
  bounds the list and the selected command remains visible. Help groups session,
  work and discovery controls using the shared measured-width presenter.
- Progress labels use measured display cells and reserve the last terminal cell,
  retaining elapsed time without the previous fixed 18-character truncation.
  Profile purpose text is normalized before rendering to protect cursor layout.
- `go test ./cmd/offgrid ./internal/cli` passed, including 32/48/80/120-column
  layouts, selection visibility, Unicode/control handling and plain-mode input.
- Packaged Windows verification passed with styled output explicitly enabled in
  the isolated fixture. The runner's inherited `NO_COLOR=1` had previously hidden
  normal styling in screenshots. Installed user environment is unchanged.
- The first styled packaged attempt raced closing/reopening Agent tabs. The
  fixture now waits for a new tab identity and verifies typed search before
  submitting; the next complete run passed. Failure logs remain preserved.
- Rendered selection was visually inspected. Draft/cursor continuity, `/clear`,
  tab switching, 200% zoom, copy/search, and cleanup on reload/close/quit passed.
  This does not establish human usability or new macOS/Linux qualification.

The follow-up was installed successfully with all five payloads checked and
workspace/model preservation verified. Installer identity and rollback are in
[the installation evidence](sidebar-and-attachment-polish-2026-11-06.md#follow-up-installation).
The desktop's CLI/Agent entries use this bundled CLI; WSL/global PATH binaries
were not changed by this installation.
