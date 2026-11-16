# Desktop agent implementation evidence — 2026-11-15

The user approved the compact desktop window, quiet full workspace and CLI as
three entries to the same saved work. The later voice references add a focused
voice surface and purposeful motion to that direction. This file records the
replacement work; prior document-led screenshots do not qualify this design.

## Architecture audit and decisions

- `apps/desktop/main.js` owns a single authenticated BrowserWindow, trusted-frame
  IPC, service lifecycle and native computer/execution/terminal sessions. Reuse
  that window in compact mode instead of widening sender trust or creating a
  second conversation executor. The first preview retained operating-system
  caption controls; the screenshot correction below replaces duplicate compact
  chrome with a frameless host and isolated full-workspace captions on Windows.
- `command-window.js` tracks full bounds independently, clamps to monitor work
  areas, registers one explicit bounded shortcut and distinguishes hide/expand
  from Stop/Quit. The renderer requests only one of input/work/voice sizes.
  Preference writes finish before changing the OS binding. Native testing found
  cumulative height growth at Windows display scaling; full-size intent is now
  retained across repeated toggles. Recreated windows start with fresh geometry.
- Existing close-to-tray keeps the host/service alive. A shortcut cannot summon a
  fully quit application. No startup/login registration or microphone permission
  is enabled by this change.
- `ChatPage`, the existing canonical/legacy service adapters and draft store
  remain authoritative. Voice uses `useTalkMode`, existing transcription and
  response speech; the new surface does not synthesize task progress.
  Listening responds to measured microphone input. State transitions use shared
  motion tokens; saved reduced motion applies from startup, before Settings loads.
  Both live voice and saved-message read-aloud stop on dismissal. Recovery links
  lead to Voice settings instead of embedding configuration in the composer.
- Models currently have separate legacy GGUF transfers, typed package operations
  and inference-runtime preparation. The curated speech manifests are pinned
  model assets, **not** complete runtime/qualification packs. A coordinated pack
  transaction has not yet been implemented.
- Default profile-free tool discovery exists behind the conversation-preview
  admission flag. This UI work does not turn the flag on or establish model/native
  action reliability. The remaining agent-conversation-workflows tasks are real
  implementation/qualification work, not merely translator or user testing.

## Completed checks so far

- TypeScript and production renderer build passed.
- Desktop Node suite: 214 passed, 1 existing skip (215 total). The eight compact
  controller regressions cover geometry, DPI rounding, window recreation,
  shortcut collisions, disposal and failed preference writes.
- Model-library interaction tests pass for inventory/discovery/back, retained
  filters, keyboard focus, explicit downloads, identity and failed-operation
  recovery. These are isolated HTTP fixtures, not real model downloads.
- Broad affected journeys: 222 passed initially; three obsolete navigation
  selectors were corrected after moving configuration into Settings. Follow-up
  model/Settings/reader/voice checks: 276 passed, 2 opt-in skips, one fixture
  failure. That failure came from a desktop mock retaining only one event
  subscriber; the real preload supports multiple subscribers. The mock was
  corrected and the final compact/voice run passed 57/57.
- Final dismissal regressions passed 4/4, including cancellation while speech
  is being prepared and while audio plays. Pinned configuration/profile/folder
  checks passed 3/3 and did not issue configuration or permission commands.
- `scripts/test/test-desktop-command.mjs` exercises the actual source Electron
  main/preload/production-renderer boundary with isolated HTTP responses. It
  verifies one window, draft preservation, native caption clearance, bounds,
  persisted shortcut validation and no task submission. Native windows remain
  hidden; show/focus are intercepted. This does not qualify OS shortcut delivery,
  tray interaction, a packaged installer or real model execution.
  An intermittent empty-editor assertion was traced to hidden-window keyboard
  injection: diagnostics reproduced the empty field before any mode switch.
  The native harness now injects a DOM input event and asserts both the field
  and persisted draft before switching; it never rewrites input after a switch.
  Three final repetitions passed with that precondition. Ordinary keyboard/IME
  uses renderer tests; native OS keyboard/shortcut interaction remains unqualified.
- Focused voice captures: English, Arabic and Simplified Chinese in light/dark,
  320×400 viewport, keyboard Escape and OS reduced motion. Microphone permission,
  capture and speech responses are deterministic fixtures. They do not establish
  real ASR/TTS quality or native microphone permission behaviour.
- Voice tests cover one submitted utterance, no capture during playback, Stop,
  retained transcripts on failure, silence recovery and late permission cleanup.
- All 29 catalogs have 1,535/1,535 authored messages and zero missing keys.
  Consistency checks report zero shared-navigation conflicts. Linguistic approval
  remains pending; authored coverage is not native review.
- Strict rendered-text inventory passed for all 29 languages (29/29). Light/dark,
  RTL, narrow/short screens, 200% reader zoom, keyboard navigation and reduced
  motion are covered by affected UI suites. These are automated accessibility
  checks, not a human screen-reader or linguistic sign-off.
- Translator packs exported to `build/desktop-agent-translator-review`.
- Strict validation of `workspace-outcome-experience` passed.

## Still open

The compact controller, request-first navigation, shared-reader simplification,
configuration movement and automated locale checks have source/test evidence.
Still open: unified transfer coordination, capability-pack acquisition,
default-agent cross-client/native action qualification
and actual OS shortcut/tray delivery checks. These include real implementation
work; they are not all external reviews. Packaged window/startup checks and the
explicitly requested local installation are recorded below; no publication or
full redesign completion is claimed.

Evidence artifacts in `build/ui-craft/desktop-agent` are isolated renderer
captures. Logs under `build/desktop-agent-*.log` distinguish each test run;
counts from different runs must not be added as though they were unique tests.

## Requested preview installation (2026-11-15)

- Installed `0.4.31-preview.20261115070734125.gda2a178e` after the owner's
  explicit request to see the current preview. The NSIS installer exited 0;
  1,195 installed payload files matched the unpacked candidate by SHA-256.
  Authenticode inspection reports unsigned; this is a local preview, not a
  release-qualified or published artifact.
- The packaged command-window test passed using the actual main/preload and
  bundled renderer, isolated HTTP fixtures and a disposable profile. It covers
  retained drafts, caption clearance, compact layout, repeated bounds restore,
  shortcut preference validation and no task submission. A first layout check
  sampled before Chromium delivered the native resize; the harness now waits
  up to two seconds for the settled viewport. No production code change or
  post-switch draft rewrite was needed. OS shortcut delivery remains untested.
- Packaged startup/recovery passed against disposable state and the bundled Go
  service, including incompatible contracts, separate-workspace recovery,
  reconnect, mixed assets and unavailable external services. Native test
  windows stayed hidden. Logs: `build/desktop-agent-packaged-command-final.log`
  and `build/desktop-agent-packaged-startup.log`.
- Rollback copy: `C:\Users\phil\og-backup\1115-150716`. Application, profile
  and mutable workspace copies were hash-verified. Models/runtime payloads
  were not duplicated; their original inventory was preserved. Pre-launch
  installation checks confirmed the workspace and desktop profile unchanged.
  The first backup destination exceeded Windows copy path limits; it was moved
  to this shorter path and verified before installation.
- Opened the installed application for the owner. The visible OffGrid process
  is responsive; `/api/v2/system` reports the installed preview version, ready
  desktop bridge and UI identity
  `8a6bada2bc6b5dc0167743490d10873d52126dd68b64252cc13a73a20872324e`.
  This is startup verification, not a real-model task or native voice benchmark.

## Reference-led Quick Access, navigation and Settings correction (2026-11-15)

The owner's subsequent screenshots supersede the earlier centered-input and nested
Settings presentation. Current source uses a slim destination rail and adjacent
context column for recent work or Settings categories, a centered welcome above
a bottom composer, direct Models navigation, a stable Installed/Discover/Search
model path, dedicated Appearance controls and shared themed native selects.
Quick window shortcuts and local storage have separate destinations. General
contains account actions. The redundant footer refresh menu is removed; returning
to the app refreshes read data without resetting drafts, profile editors or model
selection. Compact model labels preserve family/size while picker rows retain
full names and exact identifiers. No backend model or execution authority changed.

Validation (isolated fixture services, hidden browsers; no installed-user writes):
- TypeScript and production renderer build passed. Vite still reports its existing
  large-chunk warning; this is not a startup/performance qualification.
- settings-model-navigation-tests.log: 41 passed, including model picker geometry
  at 320/390/768/1440 widths, Arabic, exact-ID search, variant distinction, focus,
  theme appearance, direct navigation, all 29 locale shell layouts and Help.
- settings-organization-tests-final.log: 6 passed, covering desktop-only settings,
  shortcut changes, storage separation, English/Arabic/Chinese/German, narrow
  layouts and specific deep-link focus. The first fixture omitted mandatory
  desktop theme methods; completing the mock fixed its blank startup.
- settings-refresh-regressions.log: 6 passed; the remaining matrix had obsolete
  model-navigation selectors. After updating those selectors,
  settings-refresh-matrix-final.log passed its cross-page/category refresh check.
  Tests cover retaining profile drafts, unavailable selections and no task writes.
- Earlier reference-settings-final.log had 61 passes and a real focus regression:
  Appearance deep links tried focusing before mounting the control. Focus now
  follows the committed destination; both deep-link regression checks pass.
- Earlier quick-access-desktop-locales.log: 216 passed and one existing skip;
  source native command-window and terminal display checks passed. These do not
  establish physical shortcut delivery, real microphone or model-task quality.
- Translation inventory: 1538/1538 in each of 29 locales, zero missing catalog
  messages. New desktop caption strings are in the translator review packs;
  native linguistic approval remains pending.

Inspected captures are under build/ui-craft/quick-access-repair (Appearance light
and dark, desktop storage, narrow Quick window settings, voice and model views).
Current Windows installer rebuild uses a distinct local preview identity. The
first package attempt ran out of disk space. Only its failed installer/archive
files were removed; previous builds and rollback copies were preserved. Final
packaging/native results and installation status are recorded separately below.

A final 150% text/320px check exposed an actual overlap between response settings
and the model trigger. The footer now wraps its two action groups when space is
insufficient, retaining readable labels and touch targets. The complete affected
composer/model suite then passed all 21 checks (model-label-wrap-tests.log),
including English/Arabic enlargement, keyboard menus and exact variant IDs.
The initial extra test measured a display:contents wrapper with no box; after
correcting measurement to the composer it isolated the real overlap above.
Inspected enlarged captures are model-picker-enlarged-en/ar.png.

The owner confirmed quitting OffGrid and free space became sufficient. A fresh
rollback copy at C:\Users\phil\og-backup\1115-170112 was hash-verified before
installation. The earlier rollback was preserved. Packaged command-window and
startup checks passed against the first completed candidate; the final candidate
includes the enlarged-text correction and is checked again before installation.

### Final local installation

Installed and running: 0.4.31-preview.20261115081827104.gda2a178e.
UI identity: 8a845f118576ea000d6309fd523d0576ae45501cbcf979ee2f8eefa2e8318a4e.
The final packaged command-window check passed (settings-packaged-command-final.log),
as did real bundled-service startup/recovery (settings-packaged-startup-final.log).
The startup/recovery windows were hidden and used disposable state. The ordinary
installed app started after installation; its read-only system identity reports
this version/UI and a ready desktop bridge.

All 1,195 payload files matched the unpacked candidate by SHA-256. Installer SHA-256:
48984f580801b8b9497dfb3c1acfa232e4c023e48c932bb37e3ff3b0ebca0ca8.
Trust inspection records an unsigned local preview, not a published or release-
qualified artifact. Final build log: settings-desktop-preview-final.log.

The installer returned successfully and payload verification passed, then the
strict profile check stopped on a Chromium preload cache changed by app launch.
The follow-up check records Chromium caches, local/session-storage logs and RAG
SQLite WAL/shared-memory files changed, with runtime locks/cookies unavailable
for hashing. The other pre-existing workspace files matched their backup hashes.
No post-launch whole-profile immutability claim is made and no live files were
restored or overwritten. The original pre-install rollback remains at
C:\Users\phil\og-backup\1115-170112. Detailed results:
build/settings-postlaunch-data-check.json and build/settings-installed-receipt.json.

Tasks 44.12?44.18 are implemented and their named checks recorded. Broader model
capability-pack, persistent-transfer, real agent qualification and release gates
remain open in 44.7?44.9/44.11; this installation does not close those tasks.

## Control, voice and host-dialog refinement (2026-11-15)

Tasks 44.19, 44.20 and 44.22 implement the owner's next screenshots: shared
destination cards; grouped response/voice preferences; compact speech model
labels with exact identities retained in the picker; direct speech-model
management; segmented model navigation; and restrained terminal launch cards.
Rows respond to their available container width so enlarged text does not
collapse into narrow columns. The voice visual uses theme-derived colors,
continuous listening motion and measured input energy, with both reduced-motion
preferences respected. No synthetic audio activity or extra execution authority
is introduced.

Routine confirmations and errors now use a packaged, sandboxed, host-owned
dialog. Terminal close, folder/computer access and remembered-folder management
retain explicit actions and cancellation defaults. Only the exact local child
main frame may answer; close, navigation, hide and process failure cancel. Queued
prompts are invalidated on navigation too. File and directory pickers remain OS
dialogs. Untrusted detail text is rendered literally, never as HTML.

Validation and inspected evidence:

- Renderer build/typecheck passed (`control-refinement-build-final.log`). The
  existing large-chunk warning remains; no new performance qualification claim.
- `control-refinement-tests-final.log`: 84 passes, five screenshot-selector
  failures. The selector matched hidden headings; the focused corrected checks
  passed. After the final container-width correction,
  `control-refinement-settings-final.log` passed all 20 Settings/navigation cases,
  including the five corrected cases and English/Arabic 200% CSS zoom in Dracula.
  The distinct affected matrix totals 91 passing cases: 56 voice, 13 model-choice,
  two model-display, eight navigation and 12 settings cases. Earlier obsolete
  composer-voice selectors and a fixture string encoding error were corrected;
  their initial failed logs are retained.
- `npm --prefix apps/desktop test`: 223 passed, one existing skip, zero failed
  (`control-refinement-desktop-final.log`). This includes local-dialog sender,
  checkbox, cancel and queued-lifecycle tests.
- Hidden real Electron rendered the local dialog/preload and terminal welcome
  in light/dark themes; verified Cancel focus, literal hostile-looking detail,
  long scrolling text, RTL and explicit checkbox response. No real permission
  was granted or terminal command stopped (`control-refinement-native-dialog.log`).
- The first native test launcher incorrectly required Electron's npm wrapper,
  producing the owner's `setPath` exception screenshot. It was an isolated
  harness failure, not the installed application. The launcher now uses
  `require('electron')`, catches harness exceptions without an OS popup and
  suppresses show/focus. A subsequent terminal fixture had an arrow-object syntax
  error; theme assertions caught it. Final native checks pass. Hidden captures
  finish presentation transitions before capture to avoid stale compositor frames.
- Inspected images are in `build/ui-craft/control-refinement`: response and Voice
  settings (English, German, Arabic, Simplified Chinese), light/dark/violet voice
  frames, terminal welcome, themed confirmation, RTL overflow and enlarged Dracula
  settings. Visual inspection caught and corrected the cramped zoom layout even
  though functional controls had passed.
- Existing translation checks report 1,538/1,538 authored messages per locale
  across 29 locales, zero missing. This pass reuses their strings; native human
  review remains pending. These are UI fixtures, not linguistic approval or real
  microphone/model-quality qualification.
- Focused `git diff --check` and strict OpenSpec validation passed. Backend/API
  contracts were not changed in this refinement; the full Go/race/model corpus
  matrix was not rerun. Packaged and installation evidence follows separately.

The first completed candidate passed packaged command-window integration and
real bundled-service startup/recovery. It is being repackaged under the same
unreleased preview version to include the final container-width layout fix;
the final UI identity and installation checks must identify that rebuilt payload.
The verified pre-install rollback is at
`D:\offgrid-install-backups\control-refinement-20261115-1800`; earlier backups remain.

Final candidate: `0.4.31-preview.20261115094913829.gda2a178e`, UI identity
`f77a37ec5c93f55237b3d594133f4a104db6061b15b0f8066977cc14e4749657`.
Installer SHA-256:
`d58b69475652dfe0e3d5ad2aa9275f2fac1c30dd3258b3d7d1d1b9dc19dd0046`.
The installer is an unsigned local preview. Packaging completed, while the
trust-inspection helper returned nonzero for the expected NotSigned status;
this is not release qualification. Final package source comparison verifies the
four dialog assets, main process, terminal dock/renderer/styles and select styles
match the tested source (`control-refinement-source-payload.json`).

Final packaged command-window integration passed
(`control-refinement-packaged-command-final.log`), as did actual bundled-service
startup/recovery (`control-refinement-packaged-startup-final.log`: first hidden
window 410 ms, initial recovery 512 ms on this host). These timings are specific
checks, not broad UI or model-performance qualification. All windows remained
hidden and used disposable state. No physical global-shortcut delivery is claimed.

Read-only inspection of the installed presentation preference found
`appearance.reduceMotion=true`, explaining why the owner's orb appears static.
The new motion intentionally respects this preference; the user has been told
where to disable it. Installation does not silently change accessibility settings.

### Installed refinement

The final candidate above is installed and running. Read-only service identity
returns the exact preview version/UI and `desktop_bridge.status=ready`.
All 1,195 installed payload files match the candidate by SHA-256.
`build/control-refinement-installed-receipt.json` and
`build/control-refinement-installed-check.log` record verification.

The installer returned zero and application-file comparison passed, but the app
started before strict profile comparison finished. That check stopped on a locked
Chromium cache file (`control-refinement-install.log`). The read-only follow-up
matched 656 profile files and 475 saved-work files. It recorded 20 profile changes,
four unreadable profile files, and changes to `agent-state.sqlite`, RAG WAL/shared
memory, run events and window state. The owner lock was busy and the temporary
window-state file had been renamed. No byte-for-byte live-database/profile
immutability claim is made; no live records were restored or overwritten. The
verified pre-install backup remains at the path above. The app is left running.

## Installed UI follow-up — surfaces and Library controls (2026-11-15)

Continues the approved refinement without changing model, collection or document
execution contracts. The terminal caption reserve now matches the host's 32 DIP
inset, and layout values retain fractional precision until native zoom conversion.
Both launch cards share hover feedback and honor reduced motion. Startup uses the
selected palette, a centered brand/status and a distinct recovery presentation.
Main workspace corners are rounded and the rail's full-height border is removed.

ModelChoice presents readable rows and selected markers. Exact IDs, repository
names and detailed runtime information remain searchable and inspectable under
Technical details. Unavailable reasons remain visible; revision badges distinguish
otherwise identical speech model names. Selection still submits exact IDs.

Library uses shared action cards for Collections, Compare documents and Analyse
documents. Collection creation and Summary/Requirements choices follow the same
rounded hierarchy. Save, cancel and submission remain explicit compact actions.
Quick window and setup guide destinations also use shared cards. Existing locale
strings are reused; no linguistic approval or new runtime capability is claimed.

Verification recorded so far:

- Full desktop suite: 223 passed, one existing skip (`build/surface-polish-desktop-final.log`).
- After adding startup palette change subscription: loading, presentation and
  terminal geometry suites, 18 passed (`build/surface-polish-desktop-focused.log`).
- TypeScript check passed. Translation check reports all 29 locales at 1,538/1,538
  messages, zero missing (`build/surface-polish-i18n.log`). Human review remains separate.
- Initial 31-case renderer run: 26 passed, five failed. Two exposed a reduced-motion
  hover specificity defect, now fixed. Two exposed a test locator selecting the
  deliberately hidden General setup card, corrected to visible cards. One timed
  out during browser navigation. Final rendered checks are recorded below.
- Focused diff whitespace check passed. Light/dark startup, readable speech menu,
  rounded terminal and English/Arabic comparison captures were visually inspected.

Tests use isolated fixture data and headless browsers. The installed application
and its live model processes are left untouched. C: free space fell below 250 MB;
subsequent temporary test/build state is directed to a dedicated directory on D:.

### Final surface, Help and deletion checks

- Shared dropdowns passed English/light, English/dark/enlarged, Arabic/dark/narrow
  and Japanese/light checks; all Settings destinations have distinct icon paths.
  Forced-colors, non-Latin text and searchable desktop Help passed. Final evidence:
  `surface-controls-final.log`, `surface-icons-final.log` (an initial desktop stub
  omitted shortcut support; corrected before the final icon pass).
- Help passed language-specific searches, empty recovery, draft preservation and
  keyboard navigation across the supported locales; Japanese passed its targeted
  retry after a resource-pressure navigation timeout. Both integrated 29-language
  Help checks passed (`surface-help-integrated.log`). Light/dark/Arabic captures
  were inspected. The Quick window help article no longer displays a shortcut
  failure message when there is no failure.
- Twelve history-controls tests passed, including migrated-conversation retries,
  active-work recovery and partial batch failure. Go agent/session/server deletion
  tests passed, including concurrent rename/delete, transaction rollback,
  tombstone persistence, attachment cleanup retry and access isolation
  (`conversation-delete-final.log`). No installed-user conversation was deleted.
- TypeScript, generated API and translation checks passed. All 29 catalogs contain
  1,542/1,542 messages with zero missing keys and no shared-navigation conflicts.
  New translator packs are at `build/translation-review-surface-polish-20261115`;
  native-language approval is still pending. Contextual wording variants remain
  review work, not an automated linguistic sign-off.
- Full resource/inference tests and focused server deletion/capacity suites passed
  after memory safeguards (`memory-resource-inference-full.log`,
  `memory-server-final.log`). Race tests require CGO/compiler unavailable here;
  WSL startup also failed. No race qualification is claimed.
- Candidate: `build/desktop-0.4.31-preview.20261115113858216.gda2a178e`.
  TypeScript, renderer, Go service and NSIS packaging passed. It is an unsigned
  local preview, not a signed/public release. Installation is tracked separately.

### Packaged integration

The candidate above passed both hidden Electron checks:
`surface-memory-packaged-command.log` (draft retention, bounds, shortcut validation,
trusted window and no submitted work) and `surface-memory-packaged-startup.log`
(packaged startup/recovery against disposable fixtures and the bundled service).
The startup fixture measured first-window creation at 214 ms and recovery at
268 ms; this is a fixture measurement, not general boot/model latency. No OS
shortcut delivery, visible computer control or real-model quality was claimed.
Both affected OpenSpec changes validate strictly. Focused whitespace check passed
following removal of an extra task-file trailing blank line.

### Installed final candidate

Installed `0.4.31-preview.20261115114714884.gda2a178e` after the owner confirmed
OffGrid was closed. The final candidate additionally projects `memory_measured`
through health diagnostics; focused health tests passed. Its UI build identity
matches the command-window-tested candidate. Final hidden packaged startup passed
(`surface-memory-final-startup.log`, 715 ms first-window fixture, 847 ms recovery).

The NSIS installer returned zero. All 1,198 payload files matched the candidate
by SHA-256. Workspace files/inventory and the desktop profile matched their
pre-install backup; the app was not launched by this installation. Receipt:
`D:/offgrid-install-backups/surface-memory-20261115-final/installed.json`.
Rollback snapshot: the same directory, with application, desktop-profile and
workspace copies. Immutable models/runtime payloads were inventoried, not copied.
No live source data was reset and no model or accessibility preference changed.
