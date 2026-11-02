# Tasks

## 1. Boundary and native runtime

- [x] 1.1 Add a versioned desktop-only terminal capability and bounded typed IPC contracts; verify wrong origins, stale renderers, malformed resize/input and absent capability are rejected without spawning.
- [ ] 1.2 Pin terminal renderer/PTY dependencies and build native Windows/macOS/Linux packages with notices; verify Electron ABI/load and report unavailable native platforms honestly.
- [x] 1.3 Implement a desktop-owned PTY supervisor with explicit folder/shell identity, bounded output/backpressure and no elevation; run isolated Unicode, streaming, resize, Ctrl+C and crash tests.

## 2. Optional terminal experience

- [x] 2.1 Add trusted bundled terminal surface and optional resizable dock/tabs with View action and keyboard shortcut; test initial closed state, focus, copy/search, themes, 200% zoom and unavailable-runtime recovery.
- [x] 2.2 Add explicit matching-OffGrid-CLI launch using existing service/auth contracts; verify executable/service identity, no token leakage and native interactive prompt behavior without WSL.
- [x] 2.3 Guard output links/clipboard and model-context transfer; test hostile escape sequences, remote renderer denial, bounded scrollback and no automatic transcript export.

- [x] 2.4 Replace the bottom dock with interchangeable main-workspace Chat/Terminal views; retain sidebar navigation and test tab switching with unfinished input, unchanged resize suppression, focus restoration, narrow/zoom layouts and existing process cleanup. Add agent CLI `/clear` with display-only semantics and plain-output regression checks. User-requested refinement, 2026-11-03; earlier dock evidence remains historical.

## 3. Lifecycle and qualification

- [x] 3.1 Integrate tab close, hide, reload and app quit with owned process-tree termination and truthful stop states; test long-running descendants, shell crash, rapid reopen and no command replay.
- [ ] 3.2 Document local shell authority, keyboard actions, folder selection and recovery; run API/renderer/desktop shutdown checks and real packaged native PTY tests on each advertised platform.

Windows x64 evidence: `docs/quality/evidence/desktop-terminal-2026-11-02.md`.
The unchecked packaging/platform tasks retain macOS/Linux/ARM64 qualification;
those terminal platforms remain unavailable. Human accessibility/localization
review and full release CI are not claimed by the Windows checks.
