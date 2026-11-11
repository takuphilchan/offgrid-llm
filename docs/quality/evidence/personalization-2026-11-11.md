# Personalization implementation — 2026-11-11

Initial tranche below was source implementation only; no installer, installed-user profile, model, service,
commit or release was changed. Existing unrelated agent test changes were preserved.

Implemented:
- Versioned OffGrid/Slate/Warm/Forest/High contrast palettes with independent
  light/dark choices, synchronous initial application, preview/apply/cancel/reset,
  semantic text scaling and reduced motion. Legacy mode preferences remain valid.
- Palette normalization and persistence through desktop presentation; terminal
  surface colors use the bundled token catalog. Missing catalog retains fallback.
- Individual settings navigation links, keyboard focus, palette quick actions,
  cross-tab appearance/language updates and preference-save failure reporting.
- Correct German and Arabic terminal labels; new appearance copy in nine locales.

Verification performed:
- Web TypeScript check and API drift check passed.
- Production build passed; existing large-chunk warning remains.
- Settings and command-palette browser suites: 25 passed. These cover all nine
  locale layouts, narrow/RTL command navigation, four added palette lifecycles,
  corrupt preferences, storage failure, cross-tab updates and deep-link focus.
- Desktop suite: 162 passed, one platform skip. Includes presentation normalization
  and ten palette token contrast cases (essential text on primary surfaces and
  text on accent fills). This is not exhaustive composited UI contrast evidence.
- Documentation check: 338 maintained files, zero problems.
- Inspected the generated Slate settings screenshot. Other full-route/theme,
  forced-color, native-window, screen-reader and native-speaker review remain open.

Failures retained in this account: initial Python command was unavailable and
made no changes; Node was used instead. Initial browser test fixture had corrupted
expected Unicode due to shell encoding (application labels were correct); fixed
the test literals. Catalog import from public produced Vite warnings; moved the
single source into src and emit the desktop asset through Vite. Contrast tests
found faint text below threshold on Warm/Forest light secondary surfaces; darkened
those tokens and reran the suite successfully.

Incomplete approved scope:
- Complete setting registry with individual resets, import/export, regional
  formatting, notification and shortcut preferences, saved workspace layouts,
  remaining context/artifact review refinements and independent response-language
  preference integration.
- Full shared terminal ANSI/code/diff palette qualification and named palette
  support for all native chrome/startup/permission surfaces.
- Native-language review, language quality CI beyond the repaired labels, complete
  native/CLI localization inventory and broader cross-platform qualification.
- Full isolated renderer suite and installed-package acceptance on this revision.

OpenSpec group 31 remains unchecked because these are broader acceptance tasks.
The narrower passing tests above do not establish completion of that group.

## Established themes and chooser follow-up

This follow-up supersedes the initial source-only and unrun-check statements
where explicitly recorded below. No installed-user profile was replaced and no
release was published.

- The canonical desktop catalog now supplies 12 families / 23 variants, including
  Catppuccin, Tokyo Night, Gruvbox, Solarized, Rosé Pine, Nord and Dracula.
  Upstream attribution and licenses are bundled in `theme-NOTICE.txt`. Semantic
  and terminal colors are contrast-adapted; these are not exact upstream ports.
  Palette JSON compresses to 3,217 bytes, with an 8 KiB regression budget and no
  new runtime dependency, downloaded font or image-based preview asset.
- Compact saved light/dark cards open a searchable modal with workspace
  thumbnails, mode tabs, selection indicators and a fixed Apply/Cancel footer.
  Cancel/Escape restores saved colors and opener focus. Text size and motion save
  immediately, with session-only feedback on persistence failure.
- Twenty additional language packs are explicit previews: 29/278 core entries
  each, with complete English fallback for advanced features. The existing nine
  core dictionaries have 278 entries each. This does not certify translation
  quality. Script/region resolution, RTL, selected-language date formatting,
  desktop persistence and separation from voice settings have automated coverage.
- CI now checks keys, placeholders, empty/corrupt text and approximately 7,612
  supplemental string literals. Full translations and contextual native-language
  review remain unfinished; no preview pack has been promoted.

Verification on this follow-up:

- Production web build and API drift check passed. Existing large-chunk warning
  remains. Desktop tests: 193 passed, one platform skip.
- Targeted renderer run: 155 passed; final summary/header follow-up: two passed.
  Added 320×480 English/Arabic chooser cases: two passed. Inspected the Arabic
  screenshot and native 200% chooser screenshot, in addition to the wide and
  150% text-size chooser captures. Theme content scrolls while footer actions
  remain visible; no horizontal chooser overflow.
- Packaged Windows theme check passed for all 13 newly introduced mode variants,
  native window background, all 29 language options, restart persistence and
  actual Chromium 200% zoom (including the chooser). Evidence:
  `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-theme-1GVWJw`.
- Packaged startup/recovery check passed: first window 3,179 ms, recovery 3,691 ms.
  Evidence: `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-startup-6gvFMR`.
- xterm renderer harness passed new ANSI palette variants, new/existing tab
  synchronization, reduced motion and forced colors. This harness uses a bridge
  fixture, not native PTY execution. Evidence:
  `C:\Users\phil\AppData\Local\Temp\offgrid-terminal-display-ffTzJU`.
- Full browser result and final artifact verification are recorded below when
  complete. Targeted passes alone are not a full-suite result.

## Reported package crash

The reported `agent-completion-install-20261111` archive required palette and
language JSON that its packaging manifest omitted. Its original archive is
preserved under that build's `archive-backups` directory (SHA-256
`5bb86850acffd4e8e6d2c09237f5cdb2827a239173007cff218b4d707cb8ca53`).
The repaired archive adds the required assets and preserves all 166 native
unpacked entries and their bytes. Repaired SHA-256:
`12f2a9af7b5889ef2f0731aa474e14f8ff64dfa0ac5a85c9df936fd61ca58e17`.

Source packaging explicitly includes the assets, verifies their actual ASAR
contents after packing, and has missing/mismatched asset regression tests.
Runtime loading also retains safe defaults for missing/corrupt optional assets.
A coherent current-source candidate was built at
`build/personalization-complete-20261111/win-unpacked/OffGrid LLM Desktop.exe`;
the native checks above exercised that candidate with isolated profiles.

Retained failures: early ASAR repair used an incorrect unpack glob; corrected and
verified all native entries before replacing the candidate. Some initial browser
assertions still assumed nine language options and the old Settings geometry;
updated to assert the registered option count and actual visible layout. A fixture
missing `electron.getPaths` was repaired. The native background assertion was
normalized for Electron's uppercase hex representation. An earlier full browser
run ended without a summary and is not counted as a pass.

Human screen-reader, native-speaker, macOS/Linux and broader installation matrices
remain unverified. Group 31 and translation completion remain open; do not infer
the full original personalization plan is complete from theme acceptance.

## Final browser and artifact results

The isolated full browser run completed: 1,185 passed, 32 skipped, one failed
(`build/personalization-full-browser.log`). The sole failure was the desktop
bridge test's obsolete exact payload expectation, which omitted the new
`appearance` object. Updated it to assert the complete expected defaults, then
reran all nine workspace-shell cases against the same built renderer and disposable
service: nine passed (`build/personalization-shell-rerun.log`). Application code
did not change during that correction. This is a full run plus a focused passing
rerun, not a second all-green full run. The two new tiny-viewport chooser cases ran
separately. Latin, CJK, Indic and RTL captures were inspected for layout; this does
not provide native-language review.

Both final local artifacts were built successfully:

- `build/personalization-complete-20261111/installers/OffGrid-0.4.28-windows-x64-setup.exe`
- `build/personalization-complete-20261111/installers/OffGrid-0.4.28-windows-x64-portable.exe`

Extracted `resources/app.asar` from each without installing. Both match the
native-tested unpacked candidate exactly, SHA-256
`cbf0ccb80e0ad78835f0c723466abb7f4d590a1faed228fbdc8d53e34133e1b2`.
The packaging verifier passed for all three copies, including palette JSON,
language JSON and license notice byte equality with source. No installation,
commit, push or publication was performed in this follow-up.

## Subsequent app-wide interface audit

The later locale-routing, terminal/menu copy, theme-aware mark, divider placement,
dropdown and direct Settings changes are documented in
[the interface audit](interface-audit-2026-11-11.md). Its final verified unpacked
candidate supersedes the UI in the installers listed above; it does not replace
the installed-user application. Global preview translation and native-language
review remain incomplete.
