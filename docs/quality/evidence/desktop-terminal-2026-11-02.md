# Windows desktop terminal — 2026-11-02

This adds to the earlier composer/terminal foundation evidence. It does not
replace its failures or qualify enterprise ingestion, speech, or model accuracy.

## Implemented

Separate bundled xterm renderer and narrow workspace open-panel bridge; native
PowerShell and matching CLI tabs; streamed Unicode input/output; resize, search,
selection/copy, hiding/reopening; bounded queues; four-tab limit; Windows Job
Object supervisor created before the worker can execute. Tab close, renderer
reload, pipe loss and desktop quit stop the owned process tree. A successful
stop receipt requires the job's active process count to reach zero.

Pinned node-pty 1.1.0 and xterm 6.0.0 use the shipped Windows N-API prebuild with
the bundled ConPTY DLL. Packaging disables generic npm native rebuild, unpacks
node-pty native assets, builds/digests the supervisor and ships dependency
notices. The packaged Electron test, rather than N-API metadata alone, verifies
native loading. Other platforms remain unavailable.

## Checks

- `node --test apps/desktop/test/*.test.js`: 117 passed, one existing skip.
- `npm run build --prefix apps/web`: passed type checking and production build;
  existing large-chunk warning remains.
- `CGO_ENABLED=0 go test ./cmd/offgrid-terminal-host`: compiles; no Go unit tests.
- `node scripts/test/test-terminal-process-tree.cjs --isolated-native-test`:
  passed Unicode, resize, Ctrl+C and independently observed descendant cleanup.
- `node scripts/test/test-terminal-supervisor-crash.cjs --isolated-native-test`:
  passed forced supervisor loss, descendant termination and uncertain-stop state.
- `node scripts/test/test-desktop-terminal-packaged.mjs <packaged exe>`: actual
  Electron/ConPTY run in a disposable folder and local HTTP fixture. Passed two
  tabs, Unicode, hidden-session retention, search, explicit copy, blocked OSC52
  clipboard writes, the matching CLI's interactive profile prompt, selected
  service identity, light/dark screenshots, narrow 200% zoom, tab-close and
  app-quit descendant cleanup. Renderer reload also kills the old tree and starts
  with empty tabs, without replay. This fixture does not test model-generated answers.
- `openspec validate desktop-integrated-terminal --strict`: passed structure.
- Application-layout and maintained-document-link checks passed. The wider
  source-size check still fails at `ChatPage.tsx` (605 lines, 600 limit); it is
  a release gate, not evidence of terminal test failure or merge readiness.

The initial package attempt exposed an unnecessary native npm rebuild requiring
Python; the pinned Windows prebuilt path resolved it and passed real Electron
loading. The native input test exposed PowerShell 5's legacy output encoding;
the owned shell now explicitly initializes UTF-8. The first renderer test used
textarea fill, which does not exercise xterm input; the passing test uses actual
key events. The CLI fixture initially lacked its required capability; adding the
real capability contract allowed interactive profile selection to be checked.
The first installed-payload comparison found unused ARM64 PE files that NSIS
excluded from the x64 installation. The package now excludes those files and
unused upstream build assets before archiving, so the candidate and installed
payload inventories can be compared without weakening verification.
At 200% zoom, Electron's own capture is used: Playwright's page screenshot
cropped the embedded view. Toolbar bounds are also checked directly.

Build/test logs are under ignored `build/terminal-*`; screenshots and shell
fixtures are in private temporary `offgrid-desktop-theme-*` directories. No
personal shell output is included in this evidence.

## Limits

No real macOS/Linux/Windows ARM64 qualification, human screen-reader/localization
review, or full release CI is claimed. The terminal chrome currently uses English.
Existing dependency-audit and wider product qualification work remains separate.
The local installer is a 0.4.24 preview, not a published release.

## Local installation verified, 2026-11-03

The Windows preview was installed into the existing per-user installation.
All 786 candidate payload files matched the installed files. All 481 workspace
entries matched both the final-install inventory and the original pre-install
inventory; model files retained their recorded sizes/timestamps, and other
workspace files retained their SHA-256 digests. No model download or data
migration occurred. The original and final rollback backups remain under the
user's `OffGrid-install-backups` directory.

Installer SHA-256:
`ba88d9c1a241eff8da67c35d65cc2e7e62a9737468e577258ff37a214236cb60`.
The installed user app was left closed. Its bytes match the packaged application
tested in isolated state; a live user-workspace launch is not claimed here.
