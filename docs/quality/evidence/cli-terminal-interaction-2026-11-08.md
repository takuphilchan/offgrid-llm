# CLI and terminal interaction evidence — 2026-11-08

This covers the first implementation milestone of the supplied CLI/terminal
review, not the entire review or release qualification. Tests use isolated state;
no installed app, model, workspace grant or service was changed.

## Implemented

- Bounded, wrapped multiline agent input; Ctrl+J and supported modified Enter
  insert newlines. Grapheme-safe cursor movement/deletion, searchable history,
  shared ordinary/retained-draft history and recoverable bracketed paste.
- Live conversation corrections through the existing authenticated steering API,
  with fresh exact-task eligibility checks and no replay after uncertain ACKs.
  `/queue MESSAGE` retains a follow-up for explicit review and submission.
  Ctrl+C requests Stop and preserves unfinished text.
- One keyboard owner per phase. Pending input becomes draft text, never consent.
  A failure to drain input blocks handoff. Plain/narrow/nonterminal callers retain
  the existing stream path. Control receipts cannot interrupt streamed sentences.
- Local terminal display preferences for font size, spacing, cursor and bounded
  scrollback; active/new tabs, reset and storage-failure feedback. Complete ANSI
  palettes with measured contrast; reduced-motion and forced-color overrides.
- Search/display dialogs retain focus across visibility changes. Their keys do
  not become PTY input. The pinned search addon uses xterm's decoration API,
  without additional IPC authority.
- PowerShell command boundaries, live folder, bounded output snapshots and
  command navigation. Exact native codes are shown only when distinguishable
  from stale `LASTEXITCODE`; ambiguous failures remain failures without a code.
- Editable selected-output sharing with an acknowledged, owned-terminal IPC
  handoff. Chat offers a chosen conversation/new draft, preserves existing text,
  rejects overlapping previews and never sends automatically. Shell output is
  explicitly untrusted context, not a typed artifact or execution receipt.

## Checks

Commands run from the repository root unless noted.

| Command | Result |
| --- | --- |
| `go test ./...` | Passed during implementation; final CLI changes also checked below |
| `go test ./cmd/offgrid ./internal/cli` | Passed after input-handoff corrections |
| `go test -race ./cmd/offgrid ./internal/cli` (WSL Linux) | Passed |
| `go test -c -o build/agent-session-input.test.exe ./cmd/offgrid` then `node scripts/test/test-agent-input-windows.cjs build/agent-session-input.test.exe --isolated-native-test` | Six Windows ConPTY cases passed |
| Linux test executable then `python3 scripts/test/test-agent-session-pty.py /tmp/offgrid-agent-editor.test` | Twelve PTY cases passed |
| `npm test --prefix apps/desktop` | 150 tests: 149 passed, one skipped, no failures; final targeted checks below cover the additional failure-status test |
| `node scripts/test/test-terminal-display.mjs` | Passed bundled renderer/preferences/search checks with synthetic IPC |
| `node scripts/maintenance/check-source-size.mjs` | Passed |
| `node scripts/maintenance/check-repository-hygiene.mjs` | Passed |
| `npm run test:e2e --prefix apps/web -- terminal-context.spec.ts` | 2 passed: edited/selected draft, no send, overlapping preview, malformed context, cancellation and narrow layout |
| `node scripts/test/test-terminal-command-windows.cjs --isolated-native-test` | Passed real PowerShell/ConPTY: Unicode, native exit, stale native code after cmdlet failure, repeated native failure, directory and owned shell exit |
| `npm run build --prefix apps/web` | Passed; existing large-bundle warning remains |
| `npm run check --prefix apps/web` | Passed after extracting the props type |
| `npm run test:e2e --prefix apps/web -- terminal-context.spec.ts terminal-entry.spec.ts terminal-overlay-menu.spec.ts chat-streaming.spec.ts agent-chat.spec.ts` | 104 passed |
| `node --test apps/desktop/test/terminal-context.test.js apps/desktop/test/terminal-protocol.test.js apps/desktop/test/terminal-overlay.test.js` | Final 16 passed, including shutdown delivery cancellation and failure-status handling |

Native fixtures use synthetic streams/control responses, not models or actual
permission prompts. They check text arriving before completion, correction
submission, queued/unfinished drafts, Ctrl+C, Unicode/paste, resizing and fresh
synthetic approval input after ownership is released. Service-client tests cover
stale completion, pending approval, wrong identity, lost ACKs, cancelled parent
contexts and truthful Stop responses.

Browser checks cover active/new tabs, persistence/reset, no input dispatch,
search focus, dark mode, reduced motion, forced colors and narrow geometry.
A 550×350 CSS viewport approximates available layout at 200% zoom; it is not a
packaged Electron zoom qualification. Optional captured-ConPTY replay images are
diagnostic only, not evidence of live geometry or real-model completion.

## Remaining review scope

| Item | State |
| --- | --- |
| Multiline editor, paste/history, live correction/queue/Stop | Implemented and automatically tested |
| Palettes, display preferences, focus/search | Implemented and automatically tested |
| Profile/model/access presentation and service authorization | Preserved; no new authority |
| PowerShell command boundaries, live directory, command navigation | Implemented; native Windows and renderer checks passed |
| Selected-output preview and chosen-conversation draft | Implemented; strict IPC, delivery and browser checks passed |
| Context selection, proposed/applied change review and guarded revert | Not implemented; native-agent task 10.3 remains open |
| Inactive tab restoration, detached supervision and splits | Later work; quit/reload cleanup unchanged |
| Manual screen-reader review, macOS/ARM64, packaged platform qualification | Unverified |
| Real-model mixed workflows and latency | Existing native-agent tasks 6.2/8.8 remain open |

Live input applies to persistent agent conversations. It does not silently
change one-shot/JSON automation or let terminal output authorize agent actions.
No commit, installation, publication or release was performed for this milestone.

During verification, the second preview exposed a missing accessible selector
name, and the native shell fixture initially omitted Windows PATHEXT. Both were
corrected and rerun. The shell check also covers stale exit status rather than
assuming that LASTEXITCODE belongs to the latest command. Source-size validation
required moving ChatPage's props type into its presentation module; no limit was
raised. These corrections are not evidence of general real-model qualification.

## Local desktop installation

The updated 0.4.28 development build was installed on Windows on November 8.
This is a local update, not a new published release or completion of the remaining
review scope above.

The Go service and web UI were rebuilt. Packaging initially encountered an
Electron download HTTP 504; the exact 43.7.9 cached archive was verified against
the installed Electron package's checksum manifest and supplied to the builder.
The service was rebuilt without `-trimpath` so its linker version metadata could
pass the existing package verifier. All package verification hooks then passed.

The candidate is `build/desktop-cli-terminal-20261108`:

- `node scripts/test/test-desktop-startup.mjs "build/desktop-cli-terminal-20261108/win-unpacked/OffGrid LLM Desktop.exe"` passed (972 ms first window, 1,138 ms recovery fixture).
- `node scripts/test/test-desktop-terminal-packaged.mjs "build/desktop-cli-terminal-20261108/win-unpacked/OffGrid LLM Desktop.exe"` passed: interactive CLI, Agent clear/draft/return, clipboard boundaries, search, cursor preservation, tabs, Unicode, 200% zoom, service identity and child cleanup on close/reload/quit.
- Startup evidence: `%LOCALAPPDATA%/Temp/offgrid-desktop-startup-0kdZjK`.
- Terminal evidence: `%LOCALAPPDATA%/Temp/offgrid-desktop-theme-g4iOpQ`.

After confirming the installed app was stopped, the application, desktop profile
and saved workspace were backed up outside the repository. Four files changed
between the initial backup and installation (RAG SQLite sidecars and window
geometry); their latest versions were backed up separately and used as the
installation baseline rather than restoring older state.

The installer exited successfully. All 1,186 candidate payload files matched the
installed files by SHA-256. All 396 workspace/configuration files matched their
pre-install hashes; 93 model/runtime inventory entries retained their paths,
sizes and modification times. The latter is an inventory comparison, not a new
full model-weight hash qualification. The app remains closed for the owner to
open normally. No new real-model quality evaluation is claimed.

Installer SHA-256:
`FF24A6E5149B338F83BC084414F4E788DB0F4DBD4FDEA46DB7E8CC325C6F8B58`.
Machine-local installation results are in
`build/desktop-cli-terminal-20261108/install-result.json`; rollback is retained at
`D:/OffGrid-install-backups/v0.4.28-before-cli-context-20261108-174105`.

## Command preview, search and Settings follow-up

The previous packaged test did not exercise command preview or empty search
decoration cleanup. Its pass did not establish those behaviors. This follow-up
adds explicit regression coverage and a real PowerShell packaged check.

Changes: command completion updates an open preview; private edits survive shell
events; failed handoffs retain the question/output; pending handoffs cannot be
duplicated; late acknowledgements cannot close a newly opened preview. No-output
commands are clearly identified. Commands is offered only for captured history
or selected output, rather than an empty toolbar form. Search reports counts and
clears decorations on empty query/close, with search/modal input isolated from
the PTY.

Settings now has six localized searchable sections using existing preference
owners and the shared search field. No new executor, permission, or service
resource mutation is introduced by browsing settings.

Executed checks:

- `node scripts/test/test-terminal-display.mjs`: passed against real xterm and
  a synthetic bridge; evidence in `%LOCALAPPDATA%/Temp/offgrid-terminal-display-3vlqFG`.
- `node scripts/test/test-terminal-command-windows.cjs --isolated-native-test`:
  passed native boundaries, Unicode, exit status, changed directory and owned
  shell exit; evidence in `%LOCALAPPDATA%/Temp/offgrid-command-check-IHOUoM`.
- `npm --prefix apps/desktop test`: 150 passed, one skipped, zero failed.
- `npm --prefix apps/web run test:e2e -- settings-navigation.spec.ts workspace-feedback.spec.ts workspace-experience.spec.ts terminal-context.spec.ts control-consistency.spec.ts --workers=2`: 53 passed.
- `npm --prefix apps/web run build`: passed TypeScript and production build;
  existing large-chunk advisory remains.
- Source-size, repository-hygiene and application-layout checks passed.

These are implementation and fixture/native-shell checks, not real-model quality
or manual screen-reader/localization qualification. Installed-app replacement
and packaged results are recorded separately below when completed.

### Follow-up packaged verification and installation

The `build/desktop-terminal-settings-20261108` Windows x64 candidate passed:

- `node scripts/test/test-desktop-terminal-packaged.mjs "build/desktop-terminal-settings-20261108/win-unpacked/OffGrid LLM Desktop.exe"`: real PowerShell command capture, pending-command completion in an open preview, empty search decoration cleanup, clipboard/input boundaries, tabs and owned-child cleanup. Evidence: `%LOCALAPPDATA%/Temp/offgrid-desktop-theme-IOfqWL`.
- `node scripts/test/test-desktop-startup.mjs "build/desktop-terminal-settings-20261108/win-unpacked/OffGrid LLM Desktop.exe"`: passed; first window 1,363 ms, recovery fixture 1,698 ms. Evidence: `%LOCALAPPDATA%/Temp/offgrid-desktop-startup-U6Wztg`.

After the owner quit OffGrid, no app, service or model process remained. A fresh
application, desktop-profile and workspace backup was created at
`D:/OffGrid-install-backups/v0.4.28-before-terminal-settings-20261108-210905`.
The installer completed successfully at 21:13 local time on November 8.
All 1,189 installed payload files matched the tested candidate by SHA-256;
396 workspace/configuration files retained their pre-install hashes. All 93
model/runtime inventory entries retained their paths, sizes and modification
times (inventory preservation, not a fresh full model-weight hash evaluation).

Installer SHA-256:
`E6AF522EC1006A25D90C108F93E16155CE108AF0DF0F52187C4E5A130D9317D1`.
Machine-local details: `build/desktop-terminal-settings-20261108/install-result.json`.
The installed application is left closed for the owner to open normally.
This follow-up does not claim full CI, macOS/Linux or manual accessibility qualification.

### Live agent, access picker, and Settings follow-up

The Windows agent session now presents actual loading, action and response
phases in the live editor, with streamed answer text and a compact correction
entry. It no longer prints an empty second prompt while work is running. The
folder access choice is a keyboard picker with Cancel selected initially,
Allow once and, when an exact-scope trust store is available, Remember this
folder. Pasted text cannot approve access. Existing exact folder/account/host/
service/permission matching, revocation and noninteractive behavior remain in
force. The packaged Settings footer now returns from a native Terminal view
even when Settings is already the current route. About displays concise
connection information; diagnostics remain available on expansion.

Verification on November 8:

- `go test ./cmd/offgrid ./internal/cli`: passed.
- `node scripts/test/test-agent-input-windows.cjs build/agent-input-test.exe --isolated-native-test`: ten Windows ConPTY checks passed, including live steering, cancellation, input ownership, access choice and paste rejection. Evidence: `%LOCALAPPDATA%/Temp/offgrid-agent-input-siYiHd`.
- `npm --prefix apps/web run test:e2e -- tests/settings-navigation.spec.ts tests/terminal-context.spec.ts --workers=2`: 14 passed. A separate About layout capture passed at wide and narrow widths.
- `npm --prefix apps/desktop test`: 150 passed, one skipped.
- `node scripts/test/test-terminal-display.mjs`: passed with bundled xterm and a synthetic bridge. Evidence: `%LOCALAPPDATA%/Temp/offgrid-terminal-display-nQa7dH`.
- Final candidate `build/desktop-cli-access-final-20261108`: packaged terminal and Settings navigation passed with real PowerShell in an isolated Electron fixture. Evidence: `%LOCALAPPDATA%/Temp/offgrid-desktop-theme-bss5Nm`.
- The final packaged startup check passed: first window 686 ms, recovery fixture 799 ms. Evidence: `%LOCALAPPDATA%/Temp/offgrid-desktop-startup-hSA8tL`.

The final candidate was installed at 23:30 local time after the owner had quit
OffGrid. Backup: `D:/OffGrid-install-backups/v0.4.28-before-cli-access-20261108-232724`.
All 1,189 installed payload files matched the candidate by SHA-256. All 400
saved workspace/configuration files retained their pre-install SHA-256 hashes;
93 model/runtime entries retained their paths, sizes and timestamps. Installer
SHA-256: `BE927CB1EAA61D6EAD5F163C5FAD728C6B31183BEB5D1087FD1B52E6F392657D`.
Machine-local receipt: `build/desktop-cli-access-final-20261108/install-result.json`.
The app remains closed for the owner to open normally. This is a local preview
replacement; it is not a new published release or cross-platform qualification.
