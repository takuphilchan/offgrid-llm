# Trusted desktop encrypted recovery — 2026-11-20

This records the next `encrypted-workspace-backups` increment after the
[storage](encrypted-workspace-backups-2026-11-20.md) and
[CLI](encrypted-backup-cli-2026-11-20.md) gates. It does not complete M1/M7 or the
whole change. All executed fixtures used disposable roots, private pipes and
loopback addresses; no installed service, personal workspace, model weights or
personal browser session was used.

## Implemented boundary

- `cmd/offgrid/desktop_stdio.go` and `workspace_desktop*.go` add explicit private
  owned-service EOF drain and bounded version-1 start/cancel/result frames. The
  existing application runtime remains the cleanup owner. Ordinary serve and
  workspace commands retain their dispatch. No HTTP shutdown/backup route exists.
- `internal/storage/backup_retained.go` gives cleanup diagnostics a typed path and
  directory flag, preserving the underlying error and existing retention behavior.
  The private protocol projects compact counts/authentication/publication/database
  status and necessary retained paths, never complete manifests or raw errors.
- `apps/desktop/workspace-maintenance*.js`, existing runtime/main/preload/backend
  wiring, and the host-owned app dialog implement operation-scoped authority,
  native path/key choice, ephemeral passphrase input, pinned files and executable
  digest, companion/terminal quiescence, strict drain, cancellable helper ownership
  and explicit original-workspace resume. Host admission closes before quiescence
  and stays closed across asynchronous rechecks and maintenance. Terminal session
  creation and menu/native-view opening also check that hold after awaited work.

Private frames are limited to 2 MiB for the start, 4 KiB for cancel and 32 KiB for
results. Keys remain limited to 1 MiB/32 native keys and passphrases to 4096 UTF-8
bytes with bounded scrypt work. Existing archive/manifest bounds remain authoritative.
Executable pinning rejects nonregular replacements before reading, uses a 512 MiB
size bound and a 30-second cancellable hash stream. These are host admission checks,
not a guarantee against all same-user filesystem substitution races or secure memory
erasure. Host dialog content protection is a platform request, not proof against
every possible screen capture.

## Actual checks and discovered defects

- Windows new Go `TestDesktop*` protocol/owned-service tests initially passed,
  including real isolated encrypted subprocess backup/verify/restore, invalid and
  duplicate frames, EOF/cancel, service health and ownership release after drain.
- Full desktop suite checkpoint: **262 passed, 1 skipped**, no failures, 2.704 s.
  Additional authority/terminal/FIFO tests were added after that checkpoint and
  require a final aggregate run. Log: `build/desktop-recovery-host-tests-20261120.log`.
- A pinned source-built production binary with `main.Version=desktop-recovery-fixture`
  passed the actual host-to-Go integration in **5.742 s**: starts the real owned
  service, backs up, rejects a wrong passphrase, restores and validates content and
  SQLite, rejects corrupted ciphertext, and explicitly resumes only original data.
  Native picker/dialog answers are injected; this is not a real renderer/native
  consent or installed-edition result. An initial assertion incorrectly expected a
  23-byte header from a 22-byte slice; correcting that expectation exposed the
  remaining journey, which passed. No encryption assertion was removed.
- Independent review found a regular-executable-to-FIFO opener race. It is fixed
  with nonblocking/no-follow Unix open, opened regular identity checks, bounded
  hashing and final identity checks. Actual WSL Node 24.22 FIFO replacement plus
  four file-boundary tests passed (**5/5**, 116 ms).
- Independent review found a gap between companion quiescence and service drain
  where new host work could start during awaited checks. The persistent admission
  hold now gates IPC and actual computer/execution/terminal owners. A delayed
  post-quiescence check regression attempts real `TerminalSessions.create` and
  observes rejection with zero children; fresh and pending picker cases pass.
- The first Linux race command had a cross-shell regex quoting error and executed
  no tests. The corrected invocation found a **real data race**: OS pipe `Close`
  ran concurrently with Unix poll-reader `File.Fd()`. The failure remains in
  `build/desktop-recovery-private-protocol-race-retry-20261120.log`.
  Cancellation now joins context-aware OS readers before closing their handles;
  generic blocking pipe readers still receive close-to-unblock. Windows affected
  `go test ./cmd/offgrid -run TestDesktop -count=1` passed **1.392 s** after the fix.
  The exact Linux race recheck passed **9.683 s**; log:
  `build/desktop-recovery-private-protocol-race-fixed-20261120.log`.
- Final desktop aggregate after these fixes: **267 passed, 3 skipped, 0 failed**
  in **2.350 s**. Skips are opt-in real-binary integration, the Unix FIFO fixture
  on Windows, and the existing optional fixture. The independently enabled real
  production-binary journey passed again in **5.765 s**. Binary SHA-256:
  `4eb9fcd47aca26328d845d869decbeb9815e072f8824487319ce0ed8ac034a1b`.
  Logs: `build/desktop-recovery-host-tests-final-20261120.log` and
  `build/desktop-recovery-host-integration-final-20261120.log`.

## Administrator renderer and supported locales

`WorkspaceRecoverySettings.tsx` adds administrator backup, verify and fresh-directory
restore to Settings → Storage and backup. The optional preload bridge accepts only
operation/encryption enums and issued operation IDs. The service renderer never
receives a key, passphrase or selectable filesystem path. Native host dialogs own
selection and credentials. Progress is an accessible phase, not an invented percent;
Cancel waits for a terminal result, uncertainty prevents resume, and Resume explicitly
starts only original data. Results distinguish age authentication from legacy ZIP,
publication from verification, and retained plaintext from encrypted files.

Monotonic host revisions reject late stale responses. Navigation/account revocation
redacts prior results/retained paths. `App.tsx` preserves already admitted controls
during an intentional owned-service network outage, while a real HTTP 401 still
revokes the current identity; the renderer regression tests both outcomes. Existing
browser/external/nonadministrator contexts display unavailable host-local guidance.

The 49 shared renderer/host strings are authored for all 29 supported locales using
the existing translation packs. `npm --prefix apps/web run i18n:check` passed.
Translations remain source-level drafts pending native linguistic review.
The actual Playwright suite passed **36/36 in 4.2 minutes**, including keyboard
order, accessible names, phase/status, duplicate requests, escaped long retained
paths, unsupported authority, stale responses, intentional outage versus 401, all
locales and RTL at 760×560. This is DOM/keyboard/layout evidence, not an actual
screen-reader participant result. Log: `build/workspace-recovery-ui-locales-20261120.log`.

Initial test setup required a JSON import attribute under the actual ESM runner;
that invocation ran no tests. A selector initially compared the combined native
select label/options text; it was corrected to the actual exact accessible combobox
name while retaining name/focus assertions. No application check was removed.

`npm --prefix apps/web run api:check`, `check` and `build` all passed after the
renderer/translation changes. Vite completed in **2.20 s** with the pre-existing
large-chunk warning retained. Log: `build/workspace-recovery-combined-web-build-20261120.log`.
The next full desktop checkpoint passed **268 tests, 4 skipped, 0 failed in 7.615 s**;
the additional skip is the explicitly enabled hidden Electron fixture below.
Log: `build/workspace-recovery-desktop-final-20261120.log`.

## Real hidden Electron composition

The runnable [desktop fixture](../../../apps/desktop/README.md#trusted-workspace-recovery-bridge)
uses production `DesktopRuntime`, preload/IPC, host secret/confirmation dialog DOM,
the built web renderer and the SHA-256-pinned production Go binary above. Within
recovery dialogs, only native file-picker answers are supplied by the fixture main
process. Unrelated platform IPC uses minimal fixture responses; computer/execution/
terminal owners have no active children. The Go service, drain and filesystem
operations are real. Windows remain hidden;
no installed profile, personal application, global protocol/shortcut, model download,
clipboard capture, secret screenshot or trace is used. All service/model/config and
Chromium directories are newly created below one verified temporary root.

The initial dialog click failed because the real dialog correctly destroyed its own
window before Playwright's click acknowledgement. The test now accepts that specific
close only while the parent stays alive and the next actual operation state arrives.
The initial Cancel click also waited for animation-frame stability in a hidden,
throttled renderer until maintenance had already finished. The fixture now includes
32 MiB of incompressible data, observes the actual running status, verifies Cancel
is enabled and sends a real mouse click without the two-frame stability wait.
Failure logs remain under `build/workspace-recovery-electron*-20261120.log`.

The complete journey passed **78.789 s** on Windows/Node **24.18.0**, with real
encrypted backup, wrong-passphrase rejection, checked restore, ciphertext corruption,
active cancellation without a published destination, a competing destination created
during host confirmation, and explicit original-workspace resumes. A hidden,
disposable PowerShell helper opens an actual extracted payload without delete sharing:
Windows then rejects publication/cleanup, and the exact retained plaintext-stage path
travels through Go, private frames, host IPC and the rendered warning. The helper
releases its own handle before explicit fixture cleanup; production code has no new
fault hook or automatic deletion/retry. The retained case is Windows-specific.

After all children acknowledged drain, a bounded UTF-8 streaming scan checked **85 isolated
files / 152,364,628 bytes**, including service data and Chromium persistence, and
found no test passphrase. The renderer body/localStorage and compact result checks
also exclude it. Raw maintenance stderr/input is not copied to host diagnostics or
support output. Log: `build/workspace-recovery-electron-final-20261120.log`.
This proves these source-composed journeys; it does not qualify real native picker
interaction, installed editions, physical power loss or perfect memory erasure.

An additional same-composition pass, **95.438 s**, seeded three historical agent
snapshots through the real supported legacy import: running work, running with a
dispatched write, and already uncertain work. Production startup converted/retained
their interrupted/uncertain statuses and removed scheduler admission. Every explicit
original-workspace resume retained zero executed steps, the exact unresolved call
and an unchanged effect sentinel. Read-only `node:sqlite` inspection of the disconnected
restored database confirmed those durable records and intents; no restored service
was activated. The post-drain UTF-8 secret scan covered **95 files / 152,407,692 bytes**.
Log: `build/workspace-recovery-electron-task-recovery-20261120.log`.

The private terminal-result projection is now separately callable inside the CLI
package, with no behavior or public-protocol change. A **test-executable-only**
subprocess invokes real `BackupWorkspaceThen`; its existing callback returns
cancellation after verified publication. The same production projector writes the
compact cancelled result and exit 130, retaining published/authenticated counts.
The parent independently decrypts and verifies the retained archive against that
result. No production binary environment switch or fault hook was introduced.
Windows focused `TestDesktop` passed **2.723 s**; Linux CGO race passed **26.829 s**.
An initial direct WSL invocation lacked Go on PATH and ran no tests; the retry used
`/usr/local/go/bin/go`. Logs: `build/workspace-recovery-publication-go-20261120.log`
and `build/workspace-recovery-publication-race-path-fixed-20261120.log`.

## Late workspace identity regression

The private protocol and host implementation checks satisfy tasks 5.0–5.1. The
rebuilt production fixture SHA-256 is
`43a1c07d86155b51f4948eb9bb908cac7b99d033fd16a605d9c32657c878c186`.
One latest-source run hid Settings before maintenance; an unchanged behavior retry
passed the full journey in **108.585 s**. We did not treat the retry as resolving
the failure. Five isolated startup probes reproduced it twice: document visibility
was normal, but `.page-content` was hidden while the route remained recovery.
The asynchronous per-workspace onboarding identity effect could restore automatic
setup after the explicit route had dismissed it. `App.tsx` now honors the current
route when that identity arrives, preserving the new workspace's pending setup.
The deterministic regression and the full recovery renderer suite now pass:
**37/37 in 7.0 minutes**, including all 29 locales, after an isolated run on port
5187. `api:check`, `check` and `build` passed; Vite took **2.29 s**, retaining its
existing large-chunk warning. The shared built `index.html` SHA-256 is
`f94be35163e0ca388a63787d542238c0342ef5c771a17332827f744be223698f`.
Task 5.2 was reopened while the defect was investigated and is now satisfied by
these checks. Failure logs:
`build/workspace-recovery-electron-final-source-20261120.log`,
`build/workspace-recovery-electron-visibility-diagnostic-20261120.log` and
`build/review-recovery-startup-20261120.log`.
Passing logs: `build/workspace-recovery-renderer-final-20261120.log` and
`build/workspace-recovery-shared-final-web-build-20261120.log`.

The unchanged five-startup diagnostic then passed **5/5 in 32.801 s**, with each
fresh hidden Electron launch taking **5.78–7.62 s**. The full actual Electron
journey against the rebuilt renderer and pinned production Go binary passed
**91.077 s**, including the retained-stage OS fault and restored task inspection
described above. After acknowledged child drain, the strengthened scan found no
test passphrase in **95 files / 152,394,045 bytes** as UTF-8, UTF-16LE or UTF-16BE.
This extends the earlier UTF-8-only checks; it makes no claim about arbitrary
encodings, process memory or perfect erasure. These composition checks satisfy
task 5.3. Logs: `build/review-recovery-startup-fixed-20261120.log` and
`build/workspace-recovery-electron-route-fixed-20261120.log`.

The final full desktop aggregate passed **279 tests, 4 skipped, 0 failed in
9.374 s**, including a regression that places each of the three secret encodings
across a scanner read boundary. The skipped cases retain their opt-in/platform
requirements; the full Electron journey was explicitly enabled and run separately
above. Log: `build/workspace-recovery-desktop-route-fixed-20261120.log`.
Documentation checks passed across **459** maintained Markdown files, the
source-size ratchet was clean, and the translation check covered **171 catalogs /
1620 authored messages** without missing copy. Strict validation of
`encrypted-workspace-backups` and `git diff --check` passed. These structural checks
do not qualify native translation quality or installed behavior.

## Remaining acceptance

Final full Go/affected race, web/API, desktop and integrated suites must qualify
the eventual combined source under task 6.1.

Installed Windows/Linux/macOS filesystem permissions, publication/crash recovery,
matching-version recovery and M7 human participant checks still need their explicit
isolated scope/resources. Follow the [installed qualification procedure](../workspace-recovery-qualification.md),
the runnable source fixture in the desktop README and the representative-workspace
procedures in the recovery guide; retain actual
inventories/database records and do not replay uncertain task effects.
