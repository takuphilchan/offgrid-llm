# Startup and transcript follow-through — 16 November 2026

The reported defects were a focus outline around the first transcript message,
a premature switch from native startup into a disabled Chat surface, and a small
spinner with static introductory text. This work continues
`workspace-outcome-experience`.

## Behavior

- Pointer scrolling and mouse use of Latest no longer outline a message.
  Keyboard entry keeps focus visible on the transcript region itself.
- A local surface appears before the renderer module loads. It remains through
  initial access/model discovery, Chat capabilities, history and selected-turn
  restoration. Failure reveals recovery; a bundle watchdog offers retry after
  30 seconds without pretending the workspace is ready.
- The native and renderer surfaces share a larger animated OffGrid stack and
  three welcome lines. Copy rotates every 5.2 seconds with a 200 ms transition,
  separately from real loading status and elapsed time. It is not fake progress.
- App or system reduced motion disables movement and fades, but retains changing
  text and elapsed feedback. Hidden windows and recovery pause the welcome timer.
  Timers and listeners are disposed on handoff. Background refreshes and running
  generation do not reopen startup.
- All 29 interface languages have the three new messages. Existing translator
  exports include `desktop/startup-welcome`, and imported corrections reach both
  native and renderer startup. Human linguistic approval remains pending.

## Checks run

| Check | Result |
| --- | --- |
| TypeScript check and production renderer build | Passed |
| `npm --prefix apps/desktop test` | 234 passed, 1 existing skip |
| `playwright test tests/startup-readiness.spec.ts` | 12 passed |
| Earlier command/voice renderer regressions in this pass | 72 passed, 2 opt-in capture skips |
| Existing transcript scroll tests | 3 passed |
| `npm --prefix apps/web run i18n:check` | 29 languages, 1,554 messages each, zero missing; review pending |
| Translator pack export | 29 packs in `build/translation-review-startup` |
| Strict OpenSpec validation | Passed |
| Packaged `test-desktop-chrome.mjs` | Passed: strict startup CSP, caption controls, 100/150/200% zoom, new mark and welcome copy |
| Packaged `test-desktop-command.mjs` | Passed: delayed history gate, handoff, draft retention, seven native animation frames, reversal and reduced motion |

The browser checks cover delayed model/history/turn/capability reads, history
failure, a missing application bundle, live elapsed time, changing copy, actual
logo transforms, both themes, both reduced-motion sources, all 29 languages at
360 px width, and pointer versus keyboard transcript focus. A system-motion CSS
specificity conflict found during testing was fixed before the final package.

Visual inspection used `build/ui-craft/startup/light-none.png`, `dark-none.png`,
`scroll-no-outline.png`, and the final hidden native capture at
`%TEMP%/offgrid-caption-bE4ew3/captions-light.png`. These are isolated fixture
captures, not live model results or interaction with the installed workspace.

## Package and installation

Final candidate: `0.4.32-preview.20261116110208432.g9adc304f`.
UI identity: `493093bbb9220c6fb219c9a0a7e27ecdf397ab96095f6594d2415755ce2662a4`.
It is an unsigned local preview built from the current dirty working tree, not a
published or release-qualified artifact. The hidden packaged checks used this
exact executable with disposable profiles and HTTP fixtures. They do not prove
real model quality, OS shortcut delivery or unattended production readiness.

The earlier 09:37 candidate was installed and its payload verified, but the app
reopened during the final profile check. That attempt has no complete unchanged-data
receipt. The subsequent report reproduced a 32 px document overflow from the
caption padding plus a full-height hidden boot screen. The fix uses the remaining
viewport height, confines hidden mounting content, and aligns the startup surface
below the caption. The regression failed before the fix and passed afterward.

The final candidate passed TypeScript/production build, all 12 startup renderer
tests, and both hidden packaged native suites. The handoff suite now checks
caption clearance and absence of document overflow at 100%, 150% and 200% zoom
while history is delayed. Existing recovery, keyboard scroll, actual motion,
both themes and both reduced-motion sources remain covered. Native captures at
`%TEMP%/offgrid-caption-n2XfYo/captions-dark.png` were visually inspected.
Logs: `build/startup-scrollbar-ui.log`, `build/startup-motion-packaged-chrome.log`
and `build/startup-motion-packaged-handoff.log`.

With OffGrid closed, installation completed with exit 0 and 1,201 payload files
verified. Workspace hashes/inventory and the desktop profile were unchanged.
The verified rollback and authoritative `installed.json` receipt are in
`C:/Users/phil/AppData/Local/OffGrid-install-backups/startup-motion-20261116/`.
After this verification, the owner's explicit “Enable animations” choice changed
only `appearance.reduceMotion` from true to false in desktop-presentation.json;
all other bytes were preserved. `animation-preference.json` records both hashes
and that intentional change. System reduced motion is still respected. The app
was left closed; no installed-user session was launched for verification.

Automatic approval review blocked optional removal of two superseded local
preview build directories with only “blocked by policy” as its reason. They were
left in place; no cleanup success is claimed.
