# Terminal dialogs and scrolling — 22 November 2026

The owner requested a complete command-context redesign after rejecting the
original form, an intermediate spacing adjustment, and a large two-column
editor. The delivered design uses a content-sized, directly editable output
preview, a compact command-details toolbar, and the existing Chat composer
pattern for the question. Short output does not reserve a large blank box.
Long output uses the available space above the composer and scrolls internally.
Small or zoomed windows can scroll the dialog to reach every control.

The separate host-owned close confirmation now fits its content and uses
neutral rounded rectangular controls. Cancel remains focused initially. Long
details scroll within a display-bounded window. Only the local dialog main frame
can report geometry or confirm; other frames, navigation, closing and cancellation
retain their prior denial behavior. No recovery-only host behavior was shipped.

While a native terminal covers Settings, the underlying document no longer
exposes a redundant outer scrollbar or responds to wheel scrolling. Returning
preserves its scroll position at unchanged geometry. The terminal retains its
own scrollback. Commands, folder fields and output retain left-to-right source
order in RTL interfaces; explanations and questions use application typography.

## Verification

- `npm.cmd --prefix apps/desktop test`: **290 passed, 4 skipped** out of 294.
  Final log: `build/ui-completion/terminal-compact-desktop-fit.log`.
- `node scripts/test/test-terminal-display.mjs`: passed. Final log:
  `build/ui-completion/terminal-compact-browser-fit.log`; browser evidence:
  `C:\Users\phil\AppData\Local\Temp\offgrid-terminal-display-zCqDmS`.
  The existing suite covers named palettes, forced colors, reduced motion,
  all 29 locale transitions, narrow layouts, source redaction, running/empty
  output, failed handoffs, duplicate suppression, retained edits, focus and
  no settings/context keystrokes reaching the terminal. Added assertions check
  that one-line output stays under 100 CSS pixels and remains readable when narrow.
- `node scripts/test/test-desktop-dialog.mjs <packaged executable>`: passed
  on the staged package and again on the actual installed executable.
  Staged profile: `C:\Users\phil\AppData\Local\Temp\offgrid-dialog-check-bzVUc1`.
  Installed profile: `C:\Users\phil\AppData\Local\Temp\offgrid-dialog-check-J6bRDm`.
  Logs: `terminal-compact-native-fit.log`, `terminal-dialog-installed.log`.
  Both are under `build/ui-completion`.
- Native checks create one disposable PowerShell session and print 100 synthetic
  lines. They verify scrollback, close cancellation, explicit final stop, covered
  Settings scroll containment/return, English/light command context at 100%,
  Arabic/dark command context at native 200%, and host confirmations in English,
  Arabic and German with long paths, keyboard traversal and Escape. Service
  writes are empty. Show/focus are intercepted; this is not physical OS-input
  or screen-reader evidence. No personal workspace, real model or audio is used.
- Source-size checks, documentation checks, focused whitespace checks and strict
  OpenSpec validation pass. The renderer/service binaries did not change, so
  renderer build/performance and Go suites were not rerun for this native-only
  presentation increment. Their earlier evidence and open gates remain separate.

Visual inspection covered the short-output browser rendering, narrow output and
question layout, actual native light command context and close confirmation, and
native Arabic/dark 200% long content. Actual installed light command context was
inspected after delivery. Screenshots are rendering evidence, not linguistic,
participant-study or accessibility approval.

Intermediate failures remain in the logs: incomplete startup fixture metadata,
an unavailable `require` in the Electron test evaluation context, an Escape
response destroying its window before key-up, a five-second shell-start timeout,
and obsolete fixed-height/palette assertions after the requested redesign.
The large two-column layout and fixed-height editor were rejected and were never
installed. The final preview sizing also removes an unnecessary dialog scrollbar
at normal native size, verified by an explicit layout assertion.

## Delivered identity and preservation

Installed at **2026-11-22 08:57:45 UTC** after the owner explicitly quit OffGrid.
The installer rechecked that no installed app/service process was running. No
user process was killed and no installed-user data was changed.

- Installation: `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop`.
- Native archive SHA-256:
  `055da05a5d447a691601fd82bd057ebcf985b6b6382b3b72234c801eac3684f3`.
- Unchanged renderer index SHA-256:
  `a08cc705421a9f2b560eb089025046b3939e5049a15469a33ae9268cedd79a99`.
- Exactly six native files changed: `app-dialog.js`, `app-dialog-preload.js`,
  `app-dialog.css`, `preload.js`, `terminal-command-ui.js`, and `terminal.css`.
  All other 410 native archive files and unpacked flags were verified unchanged.
  Executable, service, PDF/OCR workers and 67 renderer files retain their hashes.
- Rollback:
  `D:\offgrid-ui-20261121\build\ui-install\rollback-terminal-dialog-20261122-165733`.
- Finalized installation receipt:
  `build/ui-completion/installation-terminal-dialog.json` (`PostInstallQualification: passed`).
- Candidate receipt: `build/ui-completion/package/receipt-dialog.json`.
- Previews: `build/ui-completion/installed-terminal-context-preview.png`,
  `build/ui-completion/installed-terminal-close-preview.png`, and
  `build/ui-completion/terminal-context-short-preview.png` (the last is a browser fixture).

All edits remained in the isolated `D:\offgrid-ui-20261121` checkout. The main
checkout retains its four previously observed concurrent model/catalog changes.
No commit, push or public release was performed. The broader UI/UX program and
its real-model, other-platform, human-review and full-CI gates remain open.
