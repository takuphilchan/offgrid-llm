# Browser upload owner integration — 2026-11-22

## Scope and implementation

This is a partial implementation of `browser-file-transfers` task 1.2 and roadmap
M2.07. No OpenSpec task or roadmap milestone is completed by this evidence.

The existing trusted desktop task tuple and validated pairing/action bindings
are now carried through `CompanionSession`, `BrowserWorkspace` and `BrowserDriver`
to the existing `BrowserOwnedUploadStore`. Pairing validation precedes owner
installation, and foreign action binding still rejects before dispatch journaling
or driver preparation. A workspace shares one owner across its tabs; new tabs
receive the same owner. Explicit retained-session renewal rebinds that owner
without adopting old snapshots as new grants or resetting retained capacity.

Immediately before native file assignment, the owner durably records the exact
action and observed frame document. The driver revalidates the observation after
that write and checks Stop/grant revocation before calling `setInputFiles` once.
An ignored durable write prevents assignment. File read-back continues to verify
selection only; it does not claim remote receipt.

Stop now waits for pending post-pairing owner initialization, closes the managed
browser, checks disconnection, records confirmed closure, and then cleans owned
snapshots. Failed browser closure preserves bytes and reservations for a
cleanup-only retry. A legacy owner with a live selected File cannot be replaced
by an upgraded owner. Lost acknowledgments retain the existing dispatch tombstone
and cannot reauthorize selection; a newly opened ledger cannot adopt the previous
owner's retained rows.

The companion runtime include list now includes `browser-owned-upload-store.mjs`
and its `browser-upload-ledger.mjs` dependency. The full suite exposed this
packaging omission after the integration import was added; the production include
list was corrected without weakening its closure check.

## Initial ownership checks

Environment: Windows x64, Node **v24.18.0**, repository Playwright **1.62.1**.
Browser tests used newly launched isolated Chromium, disposable temporary files
and IPv4 loopback receivers. No personal browser, live installation, model weights,
or installed-user state was used.

Focused command:

```text
node --test --test-concurrency=2 apps/computer/test/browser-upload-owner-integration.test.mjs apps/computer/test/session-file-binding.test.mjs apps/computer/test/session.test.mjs apps/computer/test/session-stop-review.test.mjs apps/computer/test/browser-file-transfer.test.mjs apps/computer/test/browser-file-snapshot.test.mjs apps/computer/test/browser-owned-upload-store.test.mjs apps/computer/test/browser-owned-upload-crash.test.mjs
```

Result: **45 passed, 0 failed, 0 skipped**, **11.271 seconds**. This includes the
existing real 51 MiB delayed file-read/independent receiver regression and actual
child-process termination at six snapshot lifecycle phases.

The nine new real-browser integration cases establish:

- Shared tab ownership, foreign/missing binding rejection, durable action/frame
  identity before assignment, source replacement, and exact independent HTTP bytes.
- Replaced target rejection after durable intent; revocation during preparation;
  ignored SQLite `in_use` writes with zero native assignment.
- Preservation of a selected legacy File when upgraded owner installation rejects.
- Manual grant invalidation and explicit renewal without old-grant resurrection,
  while an already selected File remains readable until browser closure.
- Stop between capture and assignment and Stop racing owner initialization.
- Failed browser closure retaining bytes until confirmed cleanup-only retry.
- Lost upload acknowledgment retaining one selection, a no-replay journal
  tombstone, and uncertain owned bytes inaccessible to a reopened ledger owner.

Full companion command, with `OFFGRID_BROWSER_FILE_MAX_TEST`,
`OFFGRID_BROWSER_TRANSPORT_PROBE` and `OFFGRID_BROWSER_NETWORK_PROBE` explicitly
set to `0` in the test shell:

```text
node --test --test-concurrency=2 apps/computer/test/*.test.mjs
```

Result before the packaging correction: **233 tests, 227 passed, 1 failed,
5 skipped**, **79.343 seconds**. The failure was
`current desktop payload includes every first-party imported runtime module`:
`browser.mjs imports unpackaged browser-owned-upload-store.mjs`. All nine new
integration cases passed in this run. The five skipped opt-in cases were the two
attached-browser boundary probes, strict 512 MiB transfer, 512 MiB transport
controls, and native network-policy diagnostic.

An earlier full-suite attempt was deliberately interrupted to avoid overlapping
backend qualification, then restarted after that window cleared. That interrupted
attempt is not counted as completed evidence. The first focused development run
also exposed an assertion expecting no `documentId` key in the canonical action;
the existing validator returns `documentId: ''`. The test was corrected to assert
the complete actual canonical value, then the final focused command above passed.

After the include-list correction:

```text
node --test apps/computer/test/native-pack.test.mjs
```

Result: **6 passed, 0 failed**, **2.231 seconds**, including first-party import
closure, isolated packaged dependency resolution and manifest integrity checks
for Windows, Linux and both macOS architectures. At that initial checkpoint the
full suite was not repeated for the include-list-only correction; the later
runtime follow-up received the frozen full checks below. Syntax checks for the three
changed runtime modules and `pack.cjs`, scoped `git diff --check`, and the
source-size ratchet passed.

## Follow-up: native protected-storage admission

The managed companion now creates `BrowserStorageCheck` from its fixed runtime
directory and the trusted desktop's private session directory. The host supplies
current execution workspace roots through private IPC, including a fresh lookup
on retained-session resume. Unknown ownership is represented as unavailable, not
an empty allowed-root list; renderer request fields cannot supply these paths.

The wrapper verifies the native pack and its additive `browser_storage:1`
profile, then launches only `offgrid-computer --browser-storage --state <directory>`
with private framed pipes, hidden Windows process creation, no shell, sanitized
Node injection variables, bounded request/reply sizes and timeouts. The request
contains only protocol, fixed operation and trusted workspace roots. A valid
response alone is insufficient: success also requires confirmed clean process
closure. Stop fences pending pack verification and retains any child whose exit
cannot be confirmed. Provider diagnostics and storage paths are not published.

The native mode is implemented separately in the existing native companion. It
does not initialize UI control. It validates private directory ancestry, ACLs,
existing journal files, bounded retained snapshot descendants, and disjointness
from the supplied workspace roots. The JS owner checks before session activation,
again before snapshot capture, and after the snapshot exists before `in_use`.
This latter check covers the actual newly created snapshot rather than assuming
that a private parent alone proves its final permissions.

An available native checker failure stops activation **before** opening or
changing the dispatch journal. Review identified that the first compatibility
fallback would have continued into the journal after such a rejection; that
fallback was removed before the frozen final checks. Regression tests verify
both absence of a newly created journal and preservation of an existing sentinel.
Only explicit missing/old support or unknown workspace ownership permits ordinary
browsing with local upload/download disabled. No protected owner is installed in
that fallback. A retained resume must pass its own new check before rebinding.

The existing provisional `browser_features.transfers:1` wire field currently
negotiates the private task binding: Go uses it for strict pairing and
`file_binding`, not to upgrade transfer evidence or gate available file tools.
The service identity deliberately does not advertise `browser-file-transfers-v1`
yet. This work neither enables that public advertisement nor treats this private
binding field as proof that storage or the complete transfer program is ready.

Lightweight wrapper/session/desktop checks passed **63/63**, **1.392 seconds**
before the final fail-closed regression was added. A focused real-browser,
manual-handoff, session and packaging run passed **33/33**, **9.670 seconds**.
Those fixture-injected storage checks are authority/lifecycle tests, not native
ACL evidence.

The final native composition used the rebuilt production
`build/browser-storage-native-20261122/offgrid-computer.exe`, including the
physical Windows volume-identity exclusion check, inside a disposable verified
fixture pack. Its browser directory is explicitly marked as a fixture, so this
does not claim an installed complete runtime package qualification.

```powershell
$env:OFFGRID_BROWSER_STORAGE_NATIVE_TEST='1'
$env:OFFGRID_BROWSER_STORAGE_NATIVE_BINARY='D:\offgrid-llm\build\browser-storage-native-20261122\offgrid-computer.exe'
node --test apps/computer/test/browser-storage-native.test.mjs apps/computer/test/browser-storage-check.test.mjs apps/computer/test/session-file-binding.test.mjs
```

Result: **16 passed, 0 failed, 0 skipped**, **2.605 seconds**. Log:
`build/browser-storage-composed-20261122.log`. This exercised actual native private
root creation, an actual journal and captured snapshot, native checks before and
after capture, real Chromium selection with an independently verified SHA-256,
denial of an overlapping workspace, and confirmed browser/snapshot cleanup.
The other cases use actual disposable subprocesses to reject malformed,
duplicate, oversized and version-skewed replies; test clean-exit requirements,
timeouts and Stop; and retain an unconfirmed process for explicit cleanup.

## Frozen final suites

The maintained source was frozen before these suites in
`build/browser-storage-checkpoint-20261122.tar` (4,209 files), SHA-256
`7297ffd2e9f468ceb26bb833f9cf6f4f471243966aaf0aca0c6fde30e81de518`.
No maintained source changed while either suite ran; this evidence was updated
afterward.

| Check | Actual result | Local log |
|---|---|---|
| `node --test --test-concurrency=2 apps/computer/test/*.test.mjs` | **244 total: 238 passed, 0 failed, 6 skipped**, 79.391 s | `build/browser-storage-companion-final-20261122.log` |
| `npm.cmd --prefix apps/desktop test` | **294 total: 290 passed, 0 failed, 4 skipped**, 5.722 s | `build/browser-storage-desktop-final-20261122.log` |
| Frozen Windows `go test -p 1 ./internal/execution ./cmd/offgrid-computer ./cmd/offgrid-executor ./internal/computer -count=1 -timeout=10m` | **PASS**, respectively 36.534 s, 11.156 s, 6.893 s, 2.749 s | `build/browser-storage-full-windows-20261122.log` |
| Same frozen source, Linux `go test -race -p 1 -json -count=1` for those four packages | **PASS**, respectively 99.468 s, 2.212 s, 9.315 s, 5.250 s | `build/browser-storage-linux-race-20261122.jsonl` |

The Linux procedure is `build/test-browser-storage-linux-20261122.sh`; it verifies
all 4,209 archived file hashes before testing and spools JSON locally until exit.
Its three opt-in real-container fixtures were skipped; the crash-helper entry
is also skipped when not invoked as a child. These results predate the subsequent
ancestor-permission and cross-client registry work and cannot qualify it.

Native startup originally failed because the storage flag did not exist
(`build/browser-storage-before-20261122.log`, 0.707 s). Final focused native
permission/framing checks passed 1.521 s in
`build/browser-storage-final-portable-20261122.log`; four Windows symlink cases
were skipped because this account could not create the links. Actual broad-DACL
rejection fixtures for the root, uploads, resource, file and journal passed.

Independent review also reproduced six Windows extended-path alias bypasses
before native physical-volume canonicalization (`build/protected-state-alias-before-20261122.log`,
FAIL 0.583 s). Final focused extended/volume-GUID checks passed 0.528 s in
`build/protected-state-alias-final-20261122.log`, including an independent
`os.SameFile` identity check. One additional ordinary symlink fixture was skipped.
Network/non-Mount-Manager volumes fail closed when the native API cannot provide
the physical local-volume identity; that is not network-volume qualification.
Primary sources read 2026-11-22: [Windows final path identity](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew),
[Windows ACE inheritance](https://learn.microsoft.com/en-us/windows/win32/secauthz/automatic-propagation-of-inheritable-aces)
and [Node filesystem permissions](https://nodejs.org/api/fs.html#fspromisesmkdirpath-options).
These define API limitations; the actual Windows fixtures above establish the
observed behavior of this implementation. Node creation modes alone are not
treated as Windows DACL evidence, and existing descendants are checked rather
than assuming every child inherits a parent's current protection.

For the full companion run, `OFFGRID_BROWSER_FILE_MAX_TEST`,
`OFFGRID_BROWSER_TRANSPORT_PROBE`, `OFFGRID_BROWSER_NETWORK_PROBE`, and
`OFFGRID_BROWSER_STORAGE_NATIVE_TEST` were explicitly `0`. Its sixth opt-in skip
is the actual native composition already run separately above; the other five
remain the previously listed qualification probes. The desktop skips are its
existing opt-in installed/native integration fixtures, not substitutes for those
checks.

## Remaining gates and exact limitations

- Production request routing and the approved **512 MiB** limit are unchanged.
  The strict maximum fixture was not run here. The raw-file interception failure
  remains open in [transport evidence](browser-upload-transport-2026-11-21.md).
- The initial ownership phase lacked native storage admission; the follow-up
  above exercises that boundary on disposable Windows state. It does not prove
  discovery/exclusion of every independently configured profile or concurrent
  external client. Cross-client protected-root coordination and platform coverage
  remain separate gates. No installed directory ACL was changed in this work.
- The direct CLI companion currently has no trusted task-bound native storage
  checker, so its session disables protected file transfer while preserving
  browsing. Completing that CLI integration and its explicit product availability
  messaging remains work; legacy absence is not protected-transfer success.
- Cross-platform native lifetime/cleanup and installed-storage qualification,
  composed service/desktop restart recovery, bounded download publication,
  artifact inspection/export and negotiated evidence completion remain open.
  Primitive process-crash tests plus a composed lost-acknowledgment test do not
  establish the full installed process-crash journey.
- Independent review identified ancestor mutation authority as another gap:
  a private child can still be replaced through a writable ancestor. Native
  ancestry enforcement and isolated negative/positive fixtures are a subsequent
  increment. This snapshot's passing leaf ACL tests do not establish that boundary.
- Rooted workspace exclusion does not contain unrestricted same-user commands.
  Account-wide cross-client reservations and actual process filesystem isolation
  remain required; neither this increment nor randomized paths closes that gate.
- These fixtures establish byte/authority/lifecycle behavior under controlled
  faults. They do not qualify real sites, real models, usability, or hardware.
