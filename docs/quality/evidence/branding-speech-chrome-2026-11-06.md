# OffGrid branding, speech recovery and Windows chrome

## Changes

The public application and shortcut name is OffGrid. Release downloads identify
version, OS, architecture and installation format. Existing package IDs,
Windows/Linux executable names, Electron profile paths and workspace paths are
retained. Installer selection supports both the new names from 0.4.27 and old
published asset names.

Windows uses native caption buttons on a sidebar-colored title strip, without
repeating the visible brand. The window title remains OffGrid for task switching.
The preload reserves 36 device-independent pixels, correcting for browser zoom.
The terminal cannot cover this strip, including before geometry arrives.
macOS and Linux title bars are unchanged.

Talk capture owns its listening/transcribing status; a second status is no longer
rendered. Silence is no longer accompanied by a misleading Models link. An empty
draft offers Speak again after a capture failure, while retained drafts are not
automatically submitted. The reported screenshot showed no speech detected;
read-only service status confirmed both installed speech models were available.
This is not evidence that every possible runtime/model availability failure is
resolved.

## Verification

- `node --test apps/desktop/test/*.test.js`: 137 passed, one platform skip.
- `npx playwright test tests/voice-playback.spec.ts --workers=1`: 35 passed,
  including three successive Talk replies followed by silence and fresh capture.
- Management, startup and chat-streaming browser checks: 35 passed.
- Isolated service/browser regression selection: 82 passed, one skip. This
  includes document preparation/reader, navigation and Activity behavior.
- Web TypeScript/build, CLI tests, desktop branding/installer selection tests,
  release finalization/access fixtures and installer entry-point fixtures passed.
- Maintained documentation and source-size checks passed. OpenSpec validation
  passed; this establishes document structure only.
- `test-desktop-terminal-packaged.mjs` passed against the candidate: title strip
  at 100%, 125% and 200% zoom; sidebar color and viewport fit; terminal placement;
  CLI/Agent entry; tab/cursor retention; clipboard boundary; and owned-child
  cleanup on close, reload and quit. The old terminal test assumed y=0 and the
  former New PowerShell terminal label; assertions now check the actual workspace
  origin and the New terminal control. Dark renderer capture was inspected.

Local logs are under `build/chrome-*`, `build/voice-recovery-checks-20261106.log`,
`build/browser-branding-regressions-20261106.log` and
`build/branding-release-checks-20261106.log`. Packaged isolated evidence is in
`C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-yl4Szn`.

## Installation

Installed `OffGrid-0.4.27-windows-x64-setup.exe` from
`build/desktop-chrome-20261106` after confirming the app/service were closed.
SHA-256: `542ce3af91cb9fcdc9c6e885e785276bba00f14edf83c74697e2ea93971eb75b`.
Five installed payloads match the candidate. All 391 current workspace data
files retained their hashes; models and inference runtimes were unchanged.
The Electron profile was hash-checked unchanged. The original uninstall registry
identity remains, now showing OffGrid 0.4.27, and the new Start shortcut targets
the existing installation directory.

Rollback: `D:/OffGrid-install-backups/v0.4.27-before-branding-20261106-200641`.
Receipt: `build/desktop-chrome-20261106/install-result.json`.

The app was left closed. A normal user-workspace launch after installation has
not been observed. macOS/Linux native upgrades, human usability, screen-reader
review and real microphone accuracy remain unverified. This follow-up does not
replace complete exact-commit CI or broader release qualification.

## Canvas and search follow-up

The Chat canvas now meets the sidebar with a rounded logical start corner and
without the previous hard sidebar/header seams. Terminal uses the same corner
treatment. Logical properties retain the corner placement in RTL layouts.

Search controls use one restrained focus boundary: the existing border changes
color, without a second thick outline. This covers shared model, conversation,
profile, Knowledge and Activity searches, standalone skill search, document
search, Quick actions and terminal search. Forced-colors mode keeps an explicit
two-pixel focus outline. Search clearing continues to return focus to the input.

The full isolated renderer suite passed 1,081 tests with 31 skips, and
`go test ./...` passed. After the canvas/search changes, the management and
document-reader suites passed 40 tests with two real-service/private-file skips;
web type checking and production build passed. Light/dark search and canvas
captures were generated. The earlier installer above predates this follow-up;
its installation receipt must not be used to claim these newest styles installed.

## Cross-page actions and disclosures

The final control layer now owns both Agents/Models section navigation and
native disclosure presentation. The filled Agents pill and native triangles
were replaced with a shared underline and small chevron respectively. Enter,
Space, RTL direction and reduced-motion/forced-colors behavior remain covered.
Search focus, rounded Chat/terminal canvas boundaries, paired button heights
and destructive-action styling use the same control rules.

The Connections setup form is bounded at 352px on wide screens and stacks on
narrow screens. A competing generic grid declaration was removed; its wider
second column had overridden the feature layout. Connection remove actions are
aligned with the owning row. Provider command examples now use theme-aware
surfaces. No tool activation, connection authority or confirmation behavior was
changed.

Targeted command: `npx playwright test tests/action-presentation.spec.ts
tests/management-polish.spec.ts tests/mcp-connections.spec.ts
tests/connection-readiness.spec.ts tests/agent-availability.spec.ts
tests/screen-hierarchy.spec.ts tests/model-library.spec.ts --workers=3`.
Initially 71 passed and one assertion failed because the global reduced-motion
reset reports .01ms rather than exactly zero. The assertion now checks that
negligible duration; all five action-presentation tests passed on rerun. The
production TypeScript/Vite build and source-size ratchet passed.

Packaging initially timed out retrieving Electron after 600 seconds. Retrying
with the existing matching Electron 43.7.9 cache produced the candidate, without
changing runtime versions. Packaged-test follow-ups wait for terminal layout
after navigation and allow native device-pixel rounding of the one-pixel search
border; initial assertions had raced layout and assumed integral CSS pixels.
These corrections do not relax terminal positioning or focus requirements.

Final verification of this follow-up:

- Full isolated suite: 1,085 passed, 31 skipped, six initial failures. Four
  palette assertions expected the intentionally removed inset shadow; their
  replacement checks assert the one-pixel border changes on focus and returns
  when focus leaves. All four passed. The first correction focused the close
  button inside the search container, so the focus-within border correctly
  remained; the test now moves focus to a palette option instead.
- The other two failures were document-runtime setup: the standalone service
  executable had no sibling document runtime. Rerunning document/attachment
  suites with the actual candidate's packaged service passed 59 tests with one
  private-file skip, including both failed real PDF/Office cases. No document
  implementation change or missing-runtime bypass was introduced.
- Packaged terminal checks passed for CLI/Agent entry, Unicode, tab/cursor
  preservation, search/copy, blocked OSC clipboard writes, 200% zoom and child
  cleanup on close/reload/quit. Captures are in the isolated temporary directory
  `offgrid-desktop-theme-OklHer`; light terminal and dark native chrome were
  visually inspected.
- Source-size and repository-hygiene checks passed. Documentation check covered
  327 maintained Markdown files with zero problems. `git diff --check` passed.

Logs: `build/browser-actions-full-20261106.log`,
`build/actions-recovery-tests-20261106.log`,
`build/palette-focus-final-20261106.log`,
`build/desktop-actions-packaged-20261106.log` and
`build/docs-actions-20261106.log`. These are local checks and do not claim a
single green full-suite run or exact-release-commit CI for the final candidate.

### Follow-up installation

Installed the latest controls candidate from `build/desktop-actions-20261106`.
Installer SHA-256:
`d1c01524a3952b1201ae6e3aec7764a2f3432c51da68e391fee4aaa5d5deac5b`.
All five installed payloads matched the candidate. All 391 workspace files,
the Electron profile, model inventory and inference-runtime hashes were
preserved. The Start shortcut and stable uninstall identity passed verification.
Rollback: `D:/OffGrid-install-backups/v0.4.27-before-controls-20261106-203715`.
Receipt: `build/desktop-actions-20261106/install-result.json`.
This receipt supersedes the earlier installation for the canvas, search, button,
disclosure and section-navigation changes. The normal app remains closed;
no walkthrough of the user's live workspace is claimed.
