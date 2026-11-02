# Compact composer and terminal progress — 2026-11-02

## Scope and result

Implements the composer portion of the approved defaults-first UX change.
The resting desktop composer is a single message bar. Narrow layouts use two
compact rows. A labelled plus menu contains files, images and saved files;
response settings disclose agent/model choices, response limits and voice setup.
Attachments, selected sources and active agent/access context remain visible.
The microphone keeps cancellable capture/transcription states. Missing runtime
errors open a separate recovery panel and do not enlarge the resting composer.
This does not install the missing Qwen speech runtime or qualify dictation.

Nested utility toggles previously propagated into their parent and incorrectly
updated parent state. Toggle handling now checks the originating element;
Escape closes the deepest open utility and restores its trigger focus.
Removing the final attachment returns focus to the visible plus button.
No submission, model-selection, permission or workspace authority is added by
opening these controls.

## Verification

- `npm --prefix apps/web run check`: passed.
- `npm --prefix apps/web run build`: passed; existing large-chunk warning remains.
- Composer, hierarchy, model selection, voice playback, files, images, readiness
  and agent-chat Playwright suites: final component results **181 passed,
  1 skipped** (179 in the combined recheck, two corrected navigation tests passed
  separately). The skipped test requires an isolated real document service.
- Profile, context, interaction-contract, materials/performance and model-choice
  suites: **72 passed, 1 skipped** across the run and final member-access recheck.
  The old member fixture expected an import button disabled; the current product
  omits administrative import controls for members. Endpoint assertions remain.
- Desktop suite: **107 passed, 1 skipped**. Terminal protocol/output tests:
  **9 passed**. These counts overlap for the existing protocol tests.
- Packaged Windows theme harness passed light/dark/native changes, all page
  checks at 200% zoom, Help, and appearance after restart using isolated state.
- Inspected compact-composer screenshots at 1440px and 390px, English/light and
  Arabic/dark. Keyboard tests cover nested menus, file chooser invocation,
  attachment removal, retained drafts and no implicit writes.

Logs and screenshots are under `build/compact-composer-*`,
`build/composer-regression-recheck*`, `build/composer-last-recheck*`,
`build/composer-cross-surface*`, `build/composer-member-recheck*`, and
`build/terminal-boundaries-final.log`. Earlier failures remain in their original
logs. The complete repository CI suite was not rerun in this pass.

## Terminal: implementation and qualification are separate

Pinned development dependencies: node-pty 1.1.0, xterm 6.0.0, fit 0.11.0 and
search 0.16.0. No automatic shell startup or remote web shell bridge was added.
The protocol rejects untrusted frames, stale renderer identities, malformed
requests and sessions outside its ownership. Output transport has bounded
UTF-8 frames, queued/in-flight byte budgets, acknowledgement validation and
backpressure. The output helper is not yet connected to the desktop dock.

An isolated Windows/Node 24.18.0 smoke check loaded the native PTY module,
launched PowerShell without profiles in a temporary directory, observed its
marker and received exit code zero. It used bundled ConPTY DLL mode, avoiding
the non-DLL console PID fallback. Reproduce with:

```powershell
node scripts/test/test-desktop-terminal-native.cjs --isolated-native-test
```

This is **not** Electron ABI, interactive input, descendant-cleanup or dock/tab
qualification. Those remain open, as do macOS/Linux tests and packaged notices.
Production-dependency audit reported zero known vulnerabilities; the complete
development dependency audit still reported 18 and needs separate triage before
release. Do not use a force audit update as a substitute for compatibility tests.

## Remaining gates

Terminal renderer/dock, native shell ownership and confirmed tree cleanup;
enterprise retention API/lifecycle and full document-answer qualification;
full release CI; unavailable human usability, screen-reader and localization
review. The installed user app, workspace, models and service were not replaced.
