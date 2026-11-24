# UI/UX completion implementation — 2026-11-21

## Status and authority

The active user goal authorizes implementation of the existing UI/UX direction and a compatible, verified local installation. The goal is still active. This record separates completed corrections from the broader journey/CI/installed and human gates; it does not replace those requirements with a smaller definition of done.

Implementation remains in `D:/offgrid-ui-20261121`. Source in the concurrent `D:/offgrid-llm` checkout has not been edited by this work. Before-edit bytes and both checkout hashes are retained in `build/ui-completion/before/` and `baseline.json`. No commit, push or publication occurred. The terminal/recovery correction was installed at 12:25 UTC, model-transfer compatibility at 13:05 UTC, the first-use engine correction at 13:49 UTC, and composer reflow at 15:27 UTC. The complete UI/UX program remains active. Later controlled comparisons below resolve the earlier local performance failure; hardware, real-model and wider qualification gates remain open. The separately requested build cleanup is recorded below.

The earlier [review](interface-review-2026-11-21.md) is historical audit evidence. Its prototypes are superseded by the actual source changes and production renderer captures below. The original Models directory/Data directory card is preserved; `SettingsPage.tsx` remains byte-identical to this increment's baseline, and a regression check preserves the original rounded card, two rows and separator.

## Corrections implemented

- Recovery no longer defaults missing status to idle/Ready. Missing desktop support, browser use, denied role, external service, checking and failed reads have distinct explanations. Unsupported contexts show no inert operation form. Retry reads status only.
- A lost operation acknowledgment remains unconfirmed until an explicit successful status check. A delayed idle event cannot silently re-enable a backup or Resume. Stop/cancel remains available for known active work. Uncertain operation state never invites an unavailable Resume.
- Recovery state is keyed to actor/workspace scope; old component callbacks cannot retain receipts across a scope change. Existing native-owned paths/secrets, revisions, result receipts and explicit Resume remain intact.
- Recovery has balanced padding and a short purpose, a dedicated status region, an operation choice, secondary archive-protection disclosure and scope details. Directory styling is untouched.
- Terminal pane, xterm and cursor backgrounds share `surface-1`; the inner pane border/radius is removed. Four named terminal palettes needed 40 ANSI brightness adjustments to maintain the existing 4.5:1 threshold on that surface. Shared UI theme surfaces and text tokens were not changed. Exact changes are in `build/ui-completion/ansi-adjustments.json`.
- Both terminal return controls use “Back to workspace”; the same Settings-category click also hides the terminal without changing its process. The operation remains the existing `terminal-hide` action. No IPC capability was added.
- Eleven recovery messages and the terminal return label are authored in all 29 supported languages. Exact source inventories were extended without marking any human review complete. Translator packs are exported under `build/ui-completion/translator-review/`.

## Requirement-to-evidence ledger at the initial checkpoint

This records the initial checkpoint, not a claim that all rows pass. Subsequent checks and installed evidence below supersede its pending entries. The current journey reconciliation follows the transfer increment.

| Goal requirement | Current evidence / next acceptance |
| --- | --- |
| Repository instructions, existing change, isolation | OpenSpec 1.14.0, existing spec-driven change, before-edit snapshot and isolated worktree confirmed. Continue reading affected contracts and reconciling historical task overlap. |
| Continuous terminal surface, themes, readable ANSI/selection/cursor | Source changes; 41 palette/preference checks pass. Existing real-xterm fixture checks pass. Inspect matched packaged/native display before installed completion. |
| Terminal tabs, search, resize, keyboard and no input from preferences | `test-terminal-display.mjs` passes existing/new tabs, settings, search, reduced motion, forced colors and no PTY-input fixture assertions. Native PTY remains separate. |
| Truthful return/context, route/draft/process preservation | Desktop navigation contract and new same-section Settings test; terminal return issues only hide, preserves tab IDs/current tab and localized accessible names. Matched native route/draft test remains. |
| Original directory card unchanged | Byte-identical Settings page, scoped recovery-only CSS and card/row/separator regression. |
| Recovery unsupported/browser/external/role/loading/failed/ready distinction | Focused recovery suite passes; new negative assertions prohibit false Ready and operation controls before eligibility. |
| Recovery running/stopped/cancelled/failed/uncertain, Stop/Resume/results | Existing lifecycle, retained-path, stale-revision, service-outage and new lost-acknowledgment tests pass. Latest visual correction removes an inapplicable Resume instruction while uncertain; final integrated run must cover it. |
| Recovery read retry/no replay/native secret boundary | Read-count delta and zero mutation assertions; only operation/encryption or operation ID cross the existing bridge. No actual backup/restore was performed. |
| One request surface, compact entry, recent work, advanced configuration | Prior implementation exists. Reconcile first-use/returning-work acceptance against the full current browser run and installed host. |
| First response readiness, installation/runtime/capability distinction, explicit downloads | Existing readiness/model suites included in full run. Guided setup and capability-pack tasks require code/contract reconciliation, not automatic closure. |
| Draft/selection/model/source/work identity across navigation/restart | Existing canonical, continuity, selection and returning-source suites included. Real-service and Quick Access/native qualification remain to finish. |
| Chat streaming/cancel/attachments/IME/no duplicate execution | Existing chat/canonical/voice/attachment suites included. Inspect all relevant failure/reconnect states and correct any failures. |
| Progress/approvals/access/uncertainty/result verification hierarchy | Existing work/Activity/agent suites included; source/journey inspection and final evidence still pending. |
| Document preparation, scope, citations, revisions, extraction limits | Existing Knowledge/reader/review suites included. Structured-source integration and real-service evidence require explicit reconciliation with document owner. |
| Models discovery, transfers, cancellation/readiness/selection | Existing library/acquisition/restart suites included; persistent operation surface and remaining 44.7/44.8 tasks require current-code assessment. |
| Profiles vs running work, Activity attention/results/history/tools | Existing management/task/attention suites included; preserve canonical identity and existing permissions during remaining refinements. |
| Settings hierarchy/search/deep links/persistence/scope | Focused Settings checks pass; remaining palette/preferences/native scope qualification continues. |
| Quick Access/full/text/voice identity, Dismiss/Stop/Quit | Existing command/voice source and tests retained. Native state, zoom, background behavior and matched package checks remain. |
| Browser fallback/no broader IPC authority | Explicit unsupported recovery path and existing terminal entry/overlay suites pass. Inspect broader optional capability boundaries before packaging. |
| Languages/themes/RTL/keyboard/dialogs/IME/motion/forced colors | Translation check/completeness/consistency and focused localized fixtures pass. Complete rendered audit and native 200% zoom; do not substitute font scaling or catalog counts for human review. |
| UI performance separate from inference | Final controlled baseline comparison remains; do not use timing from concurrent CI load. |
| Applicable CI and failure investigation | Full renderer inventory contains 2,730 tests; full browser run with an isolated real service and Go suite are underway. Retain failures and investigate them. |
| Matched package, optional capabilities, rollback, exact payload | Not yet staged. Native presentation must be delivered alongside renderer changes while excluding unrelated unfinished backend work. |
| Safe authorized installation and actual installed verification | Pending final candidate and no-active-work check. Preserve installed data and terminal sessions; do not force shutdown. |
| Honest final reporting and external qualification | Goal remains active. Required missing evidence remains open; human language/accessibility, unavailable platforms and hardware are not inferred from fixtures. |

## Checks and failure history

Environment: Windows x64, Node 24.18, Go 1.26.6, headless Edge/Chromium; isolated fixture data. Logs below are under `build/ui-completion/`.

| Check | Observed result |
| --- | --- |
| `npm --prefix apps/web run api:check` | Passed, `api-check.log`. |
| TypeScript and production build | Passed, `type-initial.log`, `build-final.log`; latest candidate build in `build-candidate.log`. |
| `i18n:check`, `i18n:complete`, `i18n:consistency` | Passed after adding exact new-source inventory entries, `i18n-check-pass.log`, `i18n-complete.log`, `i18n-consistency.log`. Completeness is authored coverage; human review remains incomplete. |
| `npm --prefix apps/desktop test` | 289 passed, 4 opt-in tests skipped, zero failed: `desktop-full.log`. |
| Terminal palette/preference checks | Initial 37 passed / 4 failed because ANSI colors had been calibrated to the darker old surface. After bounded brightness corrections, 41 passed: `terminal-palette-final.log`. No threshold weakened. |
| `node scripts/test/test-terminal-display.mjs` | Passed; actual xterm/source rendering, synthetic bridge, no native shell. Final return-action and 29-language label checks are in `terminal-display-final.log`. Captures retained at the exact temporary evidence path printed in that log. |
| Recovery/Settings first browser run | 51 passed / 3 failed. Two new assertions counted initial application status reads instead of the retry's delta. Corrected to require exactly one additional read and zero mutations. Thai route mount failed while Vite restarted after a translation edit; retained log `recovery-initial.log`. |
| Fixed production bundle focused run | 62 passed, then 63 passed after adding late-idle-event uncertainty coverage. `recovery-final.log`, `recovery-verified.log`. Includes terminal entry/overlay, Settings organization and all-language recovery cases. |
| Production recovery visual fixtures | 10 passed, `captures.log`; unsupported, ready, running, failed and uncertain in light/dark, plus narrow unsupported/ready. Inspection found an inapplicable Resume instruction during uncertainty and overly wide heading focus; source corrected, final captures still to refresh. |
| Quality guard tests | 11 passed: `quality-guards.log`. |
| Source-size ratchet | Initial terminal catalog expansion exceeded its 821-line baseline. Moved the new accessible label into the existing compact accessibility catalog; original terminal-translations file restored byte-for-byte. Ratchet and translation contract checks pass without raising the baseline: `source-size-final.log`, `terminal-copy-final.log`. |
| OpenSpec structure | Passed, `spec-validation.log`; this is not application acceptance. |

## Broader regression investigation and correction

The full 2,730-case browser run completed in 35.1 minutes: **2,643 passed, 47 skipped, 40 failed**, retained in `full-browser.log`. These original failures have not been erased or described as a clean full run.

- Five failures exposed selected navigation borders/fills conflicting with the existing control hierarchy. Removed the redundant selected-card border and restored Activity's established underline navigation without changing directory-card styling. The shared refinement check now separately verifies Activity's transparent underline navigation and the other management selectors.
- One screenshot failed with `ENOSPC`; the user-authorized cleanup recovered space. Its unchanged assertions passed afterward.
- A recent-work focus test attempted focus while startup was still inert. It now waits for startup completion before checking focus. The product's startup guard was retained.
- The lazy-module recovery fixture previously aborted only the first request, allowing module-preload retry to avoid the intended outage. It now holds the outage until explicit Reload; the actual error and retained draft are checked.
- All 29 strict language audits identified the same sole unclassified text: the separately rendered `Ctrl K` keycap. Added a narrowly scoped physical-key exemption for `button.command-trigger > kbd`; no UI prose or translation requirement was exempted. Added normal and narrow Local storage/recovery to the rendered inventory.
- The offline first-use handoff fixture incorrectly entered through the explicit new-chat deep link, which intentionally bypasses the guide. It now follows ordinary application entry and explicit dismissal before the same import, selection and retained-draft assertions.
- Two real-service PDF cases lacked an adjacent parser binary. The available older worker reported revision `offgrid4`, while current source requires `offgrid5`. Built the current worker in the isolated checkout using existing assets verified against both locked WASM and notice SHA-256 values. No runtime/model download or installed backend replacement occurred.

Follow-up evidence:

| Check | Result |
| --- | --- |
| Five affected journey suites against Vite | 47 passed, `journey-fixes.log`. |
| Strict all-29 rendered audit including Local storage, plus offline first-use suite | 43 passed, `rendered-final.log`; each locale has a retained rendered-text report. |
| Final production bundle: action hierarchy, Activity, refinement, lazy loading, controls, recovery, Settings and offline setup | 117 passed, `production-final.log`. |
| Real-service private PDF/Office attachment preparation, mixed PDF preview/explicit partial acceptance, mixed Knowledge PDF and rejected-Office recovery | 4 passed, `document-service.log`; separate disposable service and freshly built pinned parser. No model inference was performed. |
| Final production build and `i18n:check` | Passed, `build-journeys.log`, `i18n-final.log`. |
| Controlled production responsiveness comparison | Passed, `performance-final.log`: baseline/candidate navigation p95 15.6/15.5 ms; heap 12,924,468/13,188,260 bytes; 180 requests each; candidate immediate acknowledgment 18.3 ms. Existing +10% navigation/heap and <100 ms acknowledgment gates were retained. This is this machine, not 8/16 GB hardware qualification or model latency. |

## Go failure evidence remains open

`go test ./...` failed in executor startup, Windows process cleanup and managed runtime process tests. The four initial failures passed twice under a sequential focused rerun (`go-investigation.log`). A complete `go test -p 1 ./...` then failed `TestProcessProviderCancellationRetiresLiveDescendantBeforeReturning/generate` with an ambiguous worker-event/Unicode error (`go-all-sequential.log`). A three-repeat focused rerun reproduced a different failure in the same test's `blocked-write` case: cleanup returned while its owned descendant was still alive (`go-retirement-investigation.log`, line 5); the next two repeats passed.

These are unresolved backend source/CI findings, not a clean Go result. No Go/executor/inference implementation was edited or installed by this UI increment. The native desktop/service/parser binaries in the installed correction package are byte-identical to the pre-existing installation. Go race, unavailable platforms, opt-in real models, physical hardware and human qualification remain separate open gates.

## Matched package and installed correction increment

The candidate is `C:/Users/phil/AppData/Local/Temp/offgrid-ui-completion-package-7qnTMR/OffGrid`. Its receipt is `build/ui-completion/package/receipt.json`. The native archive changes exactly six files: terminal CSS, palette preference backgrounds, return-label HTML, the compact translated return-label catalog, 40 ANSI values and the existing same-section Settings hide selector in preload. All other **410 native files** and all **166 unpacked-file flags/contents** are preserved. No recovery IPC/backend was imported from the newer source checkout.

The final renderer has 67 files. Its raw index SHA-256 is `0e1e7da370eeac5ca95b7bb76d652ff23acd876bffb2c3b2a3336f4d986dcf5a`; the service's normalized UI build identity is `6dedc44d64845c353f3906ae323a62012c62ae460e135ec4e54f2cfaccfcf6e4`. Native archive SHA-256 is `e51b46789cdcf79073c0131aa42f34a6ce3e46bafa3a7f3dc3998d16e1caad0a`. Desktop/service version remains `0.4.32-preview.20261116110208432.g9adc304f`.

- `native-command.log`: exact packaged Quick Access, draft/geometry, trusted transition, reduced-motion and no-submission checks passed using a disposable profile and HTTP fixtures. OS shortcut delivery and real model use were not claimed.
- `native-final-payload.log`: exact final package passed 24 palette/mode combinations, real disposable PowerShell markers, preserved same shell/route/draft, same Settings-category return, no input from presentation changes, actual native 200% zoom and verified forced-color terminal options. Only two in-memory marker commands were sent; no real workspace or clipboard was used. Captures: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-pdyJln`.
- `native-owned-verified.log`: exact final package with its actual bundled service, empty isolated models/data and no HTTP fixture passed startup, native paths, original directory card, absent-recovery capability, reload and native 100%/200% reflow. Captures: `C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-4trwee`. Inspected dark directory/recovery and light 200% recovery; the explanation and Service details action fit and remain reachable.
- Earlier harness failures are retained: CRLF-dependent harness extraction ran the old terminal script; unsupported-copy expectation was incomplete; external-service paths were correctly withheld; direct host preference changes were not a renderer user interaction; a recovery alias correctly canonicalized when selecting Local storage; one shell startup exceeded a 5-second fixture default; and Windows 150% scaling snapped a card border to 0.666667 CSS pixels. Corrected the harness boundaries/assertions, retained all failure logs and reran. Native appearance is changed through the actual Settings controls, path tests distinguish external/owned service, startup is bounded at 15 seconds and the unchanged border is checked as one physical pixel.

Installation was authorized by the active goal and performed only after confirming no process from the installed application directory was running. **Zero user processes or terminal sessions were terminated.** The executable, native unpacked modules, backend and PDF/OCR workers were preserved, with SHA-256 checks before/after. No model, conversation or application profile was changed.

Rollback: `D:/offgrid-ui-20261121/build/ui-install/rollback-completion-20261121-202505` contains verified original `app.asar`, UI files and their manifest. In-place copies also remain at `resources/ui.rollback-completion-20261121-202505` and `resources/app.asar.rollback-completion-20261121-202505`. The exact installation receipt is `build/ui-completion/installation.json`.

Post-install verification uses the **actual installed executable**, with disposable profiles, not source assets: `installed-owned.log` passed with the bundled service and exact normalized UI identity (`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-mmTf0d`). `installed-terminal.log` passed the native terminal assertions above (`C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-116TJh`). Visually inspected the installed dark directory/recovery card and continuous dark terminal captures. The installation receipt now records successful post-install qualification and both evidence directories. This is verification of the actual installed files using disposable data, not interaction with personal conversations.

## Requested build cleanup

The D: drive exhausted free space during staging. Recursive removal of the incomplete staging directory was rejected by automatic approval review without a more specific reason. After the user explicitly requested build cleanup, bounded per-file removal succeeded. Cleared the stale `build/surface-go-cache` using Go's own `go clean -cache` with that exact cache path, after checking its cache README, six-day-old contents and absence of active Go builds. The default C: Go cache was not changed. Removed 34 obsolete installer/blockmap files (5,223,076,523 bytes) while retaining the two newest installers, all unpacked distributions, installation backups, active test binaries and evidence. The cache held 28,063,993,141 bytes; approximately **31 GiB** was recovered including the incomplete copy. Cleanup manifests are `cleanup-installers.json` and `cleanup-receipt.json` under `build/ui-completion`.

## Models transfer continuity and installed-service compatibility

Task 44.7 is implemented: GGUF and package transfers share one Downloads disclosure across all four model categories, Installed, curated discovery, online search and route return. Package progress has one library-wide read subscription; the category panels consume its saved operation identities for repair/admission. No downloader or service authority was added. Existing source pins, operation IDs, permissions, explicit Resume and confirmed Discard remain authoritative. Read failures retain progress; a missing mutation acknowledgment reconciles by reading and is never replayed. Ordinary members do not request administrative transfer history.

Cancelled history starts collapsed on return. An expanded disclosure now retains its state when active work becomes cancelled, so Resume does not disappear while the user is working. This was caught by the real-service cancellation journey, not just a screenshot. Visually inspected the final single section with both transfer types in desktop light and narrow 320px Arabic dark; long labels and controls reflow without horizontal overflow.

Packaged testing also found the older bundled service returns 404 for endpoint management. The renderer now explicitly states that limitation while preserving installed local models and any service-advertised endpoint selections. A 503 remains a retryable read failure. Supported hosts retain exact endpoint review, probe and disable behavior. This does not activate endpoint backend work or imply an update is available. The new explanation is authored in all 29 registered languages; source inventories were updated without recording human approval.

| Check | Observed result |
| --- | --- |
| Before-change transfer regression | Failed when switching to speech hid the GGUF transfer, `transfer-before.log`. |
| Initial focused follow-up | 16 passed / 2 failed: new tests expected “Check status” instead of the existing “Check saved state” label. Corrected the test expectation; 4 new cases then passed, `transfer-fixed.log`. |
| Production Models, Quick Access, voice and continuity | 173 passed, 2 documentation-capture skips, `transfer-production.log`. This precedes the unified disclosure adjustment; relevant Models checks were rerun below. Audio/capture are fixtures; no real model or microphone qualification is inferred. |
| Unified disclosure regression | 31 passed, then 49 passed with long-name light/dark/RTL layouts. A real-service run exposed automatic collapse after cancellation: 5 passed / 2 failed, `model-service-final.log`; the second failure was the next test observing the retained transfer left by the first failure. Both failures are preserved. |
| Final disclosure and management checks | 50 passed, `transfer-verified.log`, including the added cancellation/open-state regression. |
| Real service with test-tagged local package artifacts | 6 acquisition cases passed, `model-service-verified.log`; separate restart harness passed, `model-restart-verified.log`. Explicit variants, four architectures, install/verify/repair/remove, cancellation, retained partials and pinned restart recovery are covered. These are synthetic local package bytes, not downloaded production models or inference qualification. |
| Endpoint compatibility, all 29 unsupported-language states and affected model flows | 57 passed against the final production renderer, `models-compat.log`. The endpoint 404 is a truthful unsupported state; 503 recovery and supported endpoint controls remain tested. |
| Endpoint selection on an older management host | 1 additional regression passed, `endpoint-selection.log`: service-advertised endpoint models remain selectable when their management collection is unsupported, with no mutation or invented configuration controls. |
| Build, translation and source-size checks | TypeScript/production build, `i18n:check`, `i18n:complete`, `i18n:consistency` and unchanged source-size ratchet passed. Logs: `models-compat-build.log`, `endpoint-i18n-verified.log`, `models-compat-complete.log`, `models-compat-consistency.log`. |
| Responsiveness after transfer changes | Passed, `performance-transfer.log`: baseline/candidate navigation p95 16.3/16.5 ms, heap 12,978,644/13,212,176 bytes, 180 requests each, candidate acknowledgment 16.5 ms. Same thresholds; this preceded the small endpoint-copy/404 correction. |
| Matched native model compatibility | `native-models-compat.log` passed using the actual bundled older service, empty isolated data, all four categories, no model mutations, native 100%/200% reflow and unchanged directory/recovery behavior. Capture/profile: `C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-p2LC0D`. Earlier failed native check identifying the endpoint 404 is retained in `native-transfer-models.log`. |

The final renderer raw index SHA-256 is `930c4395ad33a9fbddedf537c9c1fd0365cf9d7937cbdcb27d19c6915ff7c9fd`; normalized UI build ID is `191080dd2b42457d6a78b5579d7d5da3204fad6b94c81971244cff22d943d887`. The native archive remains `e51b46789cdcf79073c0131aa42f34a6ce3e46bafa3a7f3dc3998d16e1caad0a`, retaining the installed terminal fixes. All native/service/parser files are preserved. `package/receipt-transfers.json` identifies this increment; the earlier package/installation receipts remain historical exact identities.

This increment was installed at **13:05 UTC**, after verifying the prior renderer/native hashes and no running installed application processes. Zero processes were terminated and no user data was changed. Rollback is `D:/offgrid-ui-20261121/build/ui-install/rollback-models-20261121-210511`; the earlier terminal/recovery rollback is also retained. `installation-transfers.json` records the exact installation and successful post-install check. `installed-models-compat.log` passed with the actual installed executable and bundled service in `C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-9XUZe6`, including the unsupported endpoint explanation, all categories, zero model mutations, directory preservation and native 100%/200% reflow. Visually inspected its installed language-model compatibility screen and 200% speech-category capture. The full UI/UX goal remains active.

## First-use engine readiness increment

The first-use guide had an actionable stale-state defect: after a preparation
request lost its response and the follow-up status read failed, the old Prepare
engine button remained available. `runtime-before.log` reproduces this failure.
The corrected component validates the complete readiness shape, reports initial
Checking, clears stale actionable status on failure, distinguishes HTTP 404 from
a retryable read failure, and reconciles cancellation or an unusable acknowledgment
by reading status before another explicit preparation. Reads do not overlap, and
an older poll cannot overwrite the acknowledged result. No preparation is replayed.

Visual inspection of the actual bundled service also found misleading download
copy for a configured NVIDIA engine with `download_required: false`. Its final
explanation now describes an engine check, without promising model/driver
downloads. Engine-ready remains separate from a successful first model response.
The setup namespace was missing from the shared translation inventories. All ten
setup messages now have authored entries in all 29 registered catalogs; this is
not independent linguistic approval.

| Boundary | Actual result |
| --- | --- |
| Final engine browser checks | **50 passed**, `runtime-final-browser.log`, against the production bundle: explicit preparation/cancellation, initial checking, lost/invalid acknowledgment, failed reconciliation, malformed states/modes/progress, serialized slow polling, stale-read suppression, ordinary-user advice, CPU/GPU check copy, workspace/account isolation and all 29 unsupported-host translations. |
| Broader first-use production checks | **113 passed**, `runtime-production.log`: startup/service/model readiness, explicit installed alternatives, offline setup/import, retained draft/selection and narrow/RTL captures. One real-service-only test was incorrectly included against the fixture-only HTTP server and failed before import; its original trace/log are retained. |
| Real service, synthetic offline import | **1 passed**, `runtime-import-service.log`, rerun against a disposable source service in `offgrid-web-qualification-ZabVRg`: exact uploaded-byte digest/inventory/reload, preserved selection, no inference, fixture cleanup. This is not a usable model or real-model qualification. |
| Runtime contract tests | `go test ./internal/server ./internal/inference -run 'TestRuntime(Preparation\|Readiness)' -count=1` passed both packages. Existing tests use isolated directories, synthetic probes/transport and explicit authority checks. This does not clear the separate full-Go-suite descendant-retirement finding. |
| Source/catalog checks | API generation check, TypeScript/build, translation check/completeness/consistency and source-size ratchet passed. Native language review remains pending; bundle-size advisory remains visible. |
| Exact staged native app | `native-runtime-final.log` passed with its actual bundled service in `offgrid-ui-owned-i7fDQK`: HTTP 200 engine read, zero first-use/model mutations, original directory card, unsupported native recovery, all model categories and native 100%/200% reflow. No engine preparation, model inference or backup/restore was invoked. |

Failure history is retained. The first added regression initially used Retry
instead of the existing Try again accessible name; after correction it reproduced
the stale Prepare button. The slow-read fixture initially held the extra initial
development-mode read rather than a preparation poll; tying the hold to an actual
POST corrected the fixture, and the final production case passed. The first test
import also needed Node's JSON import attribute. No application assertion or
acceptance threshold was weakened to resolve these harness errors.

Inspected captures include the native first-use screen at 100% and actual 200%
zoom, English light status-read failure, Arabic dark unsupported setup at 390px,
and the corrected native GPU-check explanation. Browser screenshots are under
`build/ui-completion/runtime-final-results`; native captures are in the stated
disposable profiles.

The first-use increment was installed at **13:49 UTC**, with no running installed
app processes, zero forced terminations and no changes to installed-user data.
The renderer raw index SHA-256 is
`536a68e8408eec4712075b4fea917db507d4dcebcf3ed920f9912fc03d340210`;
normalized UI build ID is
`4fcd074e3525419759ca050de8bd7b9f4696ff4f910b1920b8d31040a11f4231`.
Native archive SHA-256 remains
`e51b46789cdcf79073c0131aa42f34a6ce3e46bafa3a7f3dc3998d16e1caad0a`.
The executable, backend, PDF/OCR workers, native modules and terminal correction
are preserved. Version remains `0.4.32-preview.20261116110208432.g9adc304f`.
`installation-runtime.json` records verification of the installed app and actual
bundled service in `offgrid-ui-owned-xfA5JO`, with zero engine/model mutations,
native 100%/200% first-use/Settings/Models checks and unchanged directory/recovery
behavior. Inspected its final installed 200% first-use capture. Rollback is
`D:/offgrid-ui-20261121/build/ui-install/rollback-runtime-20261121-214851`;
the prior rollback copies remain retained. This local corrective delivery does
not close the unresolved whole-program performance or Go CI findings.

### Responsiveness remains a separate finding

The original pre-redesign baseline at `build/ui-baseline/dist` produced navigation
p95 pairs of **38.7/47.4 ms**, then **33.7/39.5**, **40.8/57.9** and **47.2/35.7**
in three sequential repeats. Three of four comparisons failed the unchanged
+10% relative navigation gate. Acknowledgment stayed below 100 ms and the heap
gate passed. Logs: `runtime-performance.log`, `runtime-performance-repeat.log`.
This finding remains unresolved; the earlier passing comparison is not proof
that these new failures are harmless.

Three identical-build controls passed (35.2/37.0, 38.4/37.4, 36.3/32.8 ms).
Three comparisons against the exact previously installed transfer renderer also
passed (48.2/33.8, 47.4/34.6, 38.4/40.9 ms), with unchanged thresholds and no
request/heap regression. Logs: `runtime-performance-control.log` and
`runtime-performance-increment.log`. These support this bounded increment's
delivery; they do not replace the original baseline or close task 46.6.

### Capability-pack dependency reconciliation

Task 44.8 remains unimplemented. Current contract inspection identifies concrete
missing inputs rather than a visual-design blocker:

- Legacy text-model acquisition exposes no service-bound immutable preflight;
  `LegacyDownloadReview.tsx` correctly discloses unknown free-space/preflight and
  an unpinned source. The package resolver returns a package manifest, not a
  universal text/runtime/voice pack plan.
- `PackagePreflight` sums all manifest artifact bytes and reserves staging and
  recovery space. It does not report a shared-component, verified-cache
  deduplication plan for a bundle. Reporting that sum as additional missing bytes
  would be unsupported.
- Package resolution checks whether the speech adapter is available. Its public
  contract explicitly says a missing runtime is disclosed, not installed. The
  inference-engine preparation endpoint does not install speech runtimes.

The existing per-model reviews/transfers and explicit engine preparation remain
usable. A reviewed multi-capability pack requires the owning acquisition/runtime
contracts to provide these inputs; no fake aggregate readiness, new backend
installer or unfinished runtime payload was introduced here. Groups 22/44 and
the whole-program task 46.4 remain open while independent presentation and
verification work continues.

## Composer reflow correction

The prompt editor previously recalculated its height only after a draft edit.
Narrowing the window could hide wrapped text until the user typed again. The
new English-light and Arabic-dark regression cases both failed against that
behavior (`composer-before.log`). Supported engines now use native content
sizing, bounded by the existing 160px maximum. An isolated sizing hook retains
the older-engine fallback and responds to window resize. An empty conversation
also avoids a needless transcript-follow layout read; streaming and deliberate
scroll-away/Latest behavior remain covered.

| Boundary | Actual result |
| --- | --- |
| Initial regression and broader Chat checks | **53 passed, 16 skipped**, `composer-after.log`; the skips are opt-in visual-craft captures requiring their phase variable. The initial two resize failures remain recorded separately. |
| Scroll, IME and sizing | **6 passed**, `composer-scroll.log`, including intentional scroll-away, growing activity, both-theme IME and resize behavior. |
| Production continuity | **57 passed**, `composer-production.log`, including command-window continuity, interface refinement, retained drafts, lost acknowledgments and read-only navigation. This ran before the final behavior-preserving sizing-hook extraction. |
| Final production sizing | **3 passed**, `composer-delivery-tests.log`, on the extracted-hook build: English/light and Arabic/dark narrowing/widening, bounded long drafts, clearing, old-engine fallback and fallback window resize; zero mutations. |
| Source and build | Final TypeScript/production build passed (`composer-delivery-build.log`); source-size ratchet passed after extracting the hook. The existing large-chunk advisory remains. |
| Exact final staged app | `native-composer-final.log` passed using its actual bundled service in `offgrid-ui-owned-3NATUM`: native content sizing, 100%/200% zoom, retained draft after Settings, zero composer/engine/model mutations, original directory card and unsupported recovery. |

Visually inspected English narrow reflow, Arabic dark narrow reflow and the exact
final staged native composer at 200% zoom. These are rendered-layout checks, not
native-language, screen-reader or real-model acceptance.

The composer increment was installed at **15:27 UTC** after the guarded installer
confirmed the installed app/service were idle. Its 67 renderer files have raw
index SHA-256 `b21e8c936934bc55dcd8d26bd92da77ebee93419c7ba537b086c9650a7e6dc1e`;
normalized build ID is
`97dbcb4eabdae6d3a6f9bbf3225f1fd690365f99d3538e8f3bfc81aec6deb4ed`.
Native archive remains
`e51b46789cdcf79073c0131aa42f34a6ce3e46bafa3a7f3dc3998d16e1caad0a`;
the executable, service and PDF/OCR worker hashes are unchanged. Package version
remains `0.4.32-preview.20261116110208432.g9adc304f`. No installed-user data changed
and no process was forcibly terminated. Verified rollback is
`D:/offgrid-ui-20261121/build/ui-install/rollback-composer-20261121-232653`.

`installed-composer.log` and `installation-composer.json` record a passing check
of the actual installed executable and bundled service in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-9OXvKm`. Its build ID matches
the staged package; native 100%/200% draft reflow, Settings return, first-use and
Models compatibility, the original directory card and unsupported recovery
passed with zero composer/engine/model mutations. Visually inspected the
installed 200% composer capture. OpenSpec strict validation and focused diff
checks passed. These results qualify the corrective increment, not the remaining
whole-program performance, Go, platform or human gates.

### Performance investigation and fixture correction

CDP profiles identified synchronous composer sizing and empty-transcript reads
as avoidable layout work. The reverse-order diagnostic after native sizing used
142 candidate layout passes versus 178 baseline passes; this diagnostic does not
replace the acceptance benchmark. The benchmark still uses the original
`build/ui-baseline/dist`, 40 samples and unchanged navigation/heap/request/ack
thresholds.

The native-sizing-only run passed one of three comparisons. After the empty
transcript guard, all three navigation comparisons passed (36.4/38.6,
34.2/36.8 and 39.6/43.0ms), but one run counted 179/180 requests. Trace comparison
showed that the baseline counter was sampled before the final saved-session
read after Stop. Waiting for settled Send then exposed a shared fixture defect:
its synthetic session POST never stored a session for the subsequent GET,
which crashed both renderers after cancellation. The fixture now stores and
returns that synthetic session, and the benchmark verifies settled controls
and no render failure before counting requests.

The corrected benchmark (`composer-verified-performance.log`) passed **2 of 3**
runs: baseline/candidate p95 was 32.4/31.5, 36.4/38.3 and **31.6/39.2ms**. Requests
were 180/180 in all three, heap stayed within +10%, and acknowledgment stayed
below 100ms. The last relative-navigation failure remains open. No threshold,
baseline or sample count was relaxed; the earlier failures and diagnostic
profiles remain under `build/ui-completion`.

## Current journey reconciliation

| Journey | Current implementation/evidence | Remaining boundary |
| --- | --- | --- |
| First use and readiness | One composer model selector with empty-inventory guidance inside its menu; the rejected separate setup banner is removed. Explicit setup/import, engine uncertainty/polling/localization and native first-use checks remain recorded below. | Reconcile group 22 acceptance/action counts and the identified capability-pack contract dependencies in 44.8. No model reply or human qualification is implied. |
| Returning work and canonical Chat | Recent work, scoped drafts and canonical recovery exist. Fresh continuity checks passed in the 173-case run, including lost acknowledgment, deletion, navigation, late responses and explicit Stop. | Keep real-model/tool execution and cross-client qualification in 44.9 separate. |
| Documents and Knowledge | Shared readers/source scopes, retained failed imports, explicit partial acceptance and detailed cell inspection; four real-service PDF/Office flows passed with the matching parser. The library-return increment retains page/filter/selection and has its own 134 passing browser checks. | D1 is complete for its recorded presentation boundary. Reconcile remaining D2 review/export and group 22 scenarios with the owning document change; model-grounded answer quality remains separate. |
| Models | Task 44.7 now complete; 50 transfer/management, 57 final compatibility/model and 7 real-service acquisition/restart cases pass. | Reviewed capability-pack plans (44.8) remain implementation work. Existing per-model APIs do not prove that feature complete. |
| Agents and Activity | Existing profile/work separation, attention ordering, archive, control hierarchy and scoped tools; full browser inventory plus corrected Activity checks pass for their stated boundaries. | Default-tool admission, real agent execution and platform qualification remain in 44.9. |
| Settings and terminal | Original directory card retained; truthful recovery and continuous terminal installed and verified. Browser/desktop matrices and native 200% storage checks pass. | Supported native recovery operations were contract-tested; the installed older host lacks that capability. No real backup/restore was performed. |
| Quick Access, text and voice | Fresh fixture checks retain draft/work identity, no duplicate sends, capture/playback cancellation, Stop and reduced motion. Packaged native command tests cover trusted geometry transitions and one window. | No OS shortcut delivery, physical microphone/speech-model, assistive-technology or participant qualification is implied. Finish 46.5 reconciliation. |
| CI and delivery | Exact payloads, guarded idle-only installation, verified rollback and disposable native checks are recorded. | The reproduced Windows Go descendant-cleanup failure remains unresolved; full-platform/race/human/hardware gates are not closed by UI checks. |

## Remaining scope

OpenSpec 46.1–46.3 and 46.7–46.8 are complete for their stated implementation/check/delivery boundaries. Broader task/contract reconciliation (46.4), full cross-surface/native/human qualification (46.5), all applicable CI including the unresolved Go finding (46.6), and external gates remain distinct. This installed correction increment is not a claim that the complete UI/UX program, capability-pack dependencies, real-model journeys, native-language review, screen-reader evaluation or participant study is finished.

## Owner rejection and single model control (2026-11-22)

The owner rejected the installed composer screenshot and its duplicate model
action. Previous functional passes and visual inspection did not establish owner
acceptance. The intervening `3695d09f…` candidate moved the setup notice but still
duplicated model choice; it was never installed. Its 521 passing checks and two
real-service-only skips (`coherent-journey-browser.log`) are retained as bounded
document/library/continuity evidence, not approval of that layout.

Chat now uses its existing model selector for installed choices and setup. Empty
inventory guidance lives inside the picker; its Models link carries the existing
return-to-draft handoff. The external setup card/action and first-reply hint are
removed from ordinary Chat. A lost saved model retains a visible status and the
saved choice; replacement remains explicit. The established bottom composer,
monochrome surfaces and original directory card are retained.

Actual captures exposed an additional defect: an ordinary menu or draft length
change invoked Quick Access presentation fading in full desktop and browser
workspaces. `useCommandWindowLayout` now limits fading to voice swaps and compact
native geometry changes. Regression tests observe transition-attribute mutations
while opening/closing menus and editing long/short drafts in both host contexts.
Compact voice/native geometry and reduced-motion tests still pass.

This candidate also includes the previously staged connected-document changes:
compact failed/preparing attachment summaries, disclosed coverage details,
library selection/metadata spacing, and inert stored-value/formula/cached-result
inspection in the shared reader. Missing formula caches remain Unknown; explicit
empty values remain distinct. This does not complete rich table/cell rendering,
all Office layout fidelity or the broader D2 acceptance scope.

| Check | Result |
| --- | --- |
| `single-model-corrected-browser.log` | 81 passed: readiness, selectors, setup handoffs and complete fixture journeys. |
| `menu-motion-browser.log` | 91 passed, two opt-in visual checks skipped: menu visibility, full/compact windows and voice. |
| `single-model-final-build.log` | TypeScript and production build passed. API, translation completeness/consistency, source-size and strict OpenSpec checks passed in the corresponding `single-model-*` logs. |
| `completion-desktop.log` | 289 passed, four optional native checks skipped. |
| `completion-browser.log` | Full built-renderer run against disposable service `offgrid-web-qualification-Tg23Lz`: 2,804 passed, 47 skipped, three obsolete style expectations failed. |
| `completion-controls.log`, `completion-focus-confirmed.log` | 16 and 11 passed against the same built renderer after updating those expectations. No application code changed during this correction. This is a full run plus focused passing reruns, not a second all-green full run. |
| `native-single-model.log` | Exact staged executable and its own bundled service passed in `offgrid-ui-owned-DyCjOy`: one model control, native 100%/200%, retained draft after Settings, first-use/Models compatibility, original directory card and unsupported recovery. |
| `single-model-native-command.log` | Exact staged native host passed in `offgrid-command-rfNgen`: one window, retained draft, restored bounds, trusted transition acknowledgment, reversal, reduced motion and isolated shortcut preference validation. HTTP fixtures; no OS shortcut delivery or model execution. |
| Native document scope | One synthetic TXT attachment in the disposable profile; exactly two permitted writes (create session, attach file), successful retained-text inspection and draft return. No model request, backup/restore or user-data operation. |

An initial 53-case run started before test navigation had been migrated from the
removed CTA: 37 passed and 16 failed (`single-model-browser.log`). Those failures
reference the removed button/`.setup-inline` locator. The corrected 81-case run
exercises the replacement picker route with the same draft/no-mutation assertions.
Initial blank fixture captures led to the presentation-transition fix; refreshed
captures now retain the full workspace behind the picker.

The full-run failures expected the old 24px composer radius and the former muted
focus color. The radius check now matches the 20px design. The focus check now
measures the rendered boundary's contrast (at least 3:1), retains the single
1px border/no nested editor outline, and checks the 2px forced-colors outline.
It does not weaken focus visibility to accept a color change. The first focus
rerun reported all 11 passing but its PowerShell wrapper returned 1 after stderr
redirection; an explicit native-exit-code run confirmed all 11 and exit 0.

The 47 skips are retained in the full-run log: opt-in documentation/visual
captures, private supplied PDF evidence, explicit endpoint/model-fixture service
cases, installed/live-data tests, native speech, and isolated performance modes.
The original-baseline performance mode ran separately below. Dedicated earlier
model-acquisition/restart evidence is not relabelled as part of this full run.
No installed/live-data opt-in was enabled against the user's workspace.

The same full run passed the 1,000-conversation history test: only 20 rows mounted,
filter p95 34.3ms against its unchanged 250ms budget; fixture cold-route times were
239–395ms and warm routes 12–34ms. Those route timings are measurements, not a
claim about real model latency or physical low-memory machines.

Visually inspected English light short-window Chat and its empty picker, Arabic
dark narrow Chat, and the exact staged native 200% Chat/picker. Fixture captures
are under `build/ui-completion/single-model-captures`; native captures are in the
disposable profile above. This is not linguistic, screen-reader or user-study
approval.

### Current production responsiveness

The final candidate passed all three controlled comparisons against the retained
pre-redesign `build/ui-baseline/dist` using 40 navigation samples per build,
unchanged thresholds and no concurrent browser qualification job. Baseline /
candidate p95 values were **28.9/29.2**, **28.5/29.8** and **29.0/29.7 ms**.
Candidate heap was 1.8–2.4% above baseline (within +10%); request counts were
180/180 each time. Immediate acknowledgment was 17.7–21.6 ms (below 100 ms).
`single-model-performance.log` retains all samples. Earlier failed runs remain
documented; this current fixture comparison passes but does not qualify physical
8 GB/16 GB hardware or model-processing latency.

The separate one-worker 4x/6x CPU paint run also passed both tests
(`completion-paint-corrected.log`, exit 0). Eight menu/stream samples had opening
feedback 13.6–50.5ms, Stop 21.4–79.9ms and frame p95 33.3–66.7ms. Eight additional
stream/download samples had frame p95 16.7–83.2ms; stream Stop stayed below 105ms.
All applicable 250ms budgets are unchanged. The initial paint run failed an
obsolete `blur(12px)` expectation: the approved flat workspace and existing
functional surface tests require opaque, unblurred menus. The test and maintained
surface guide now reflect that contract; no blur was enabled or timing threshold
relaxed. That initial partial run overlapped a short focus check and is not used
as controlled performance evidence. The passing rerun had no concurrent browser
or native qualification job.

Staged UI raw index SHA-256:
`6dd1f0d04e5bffbde1a35c4627926c082008f3b64fdde58fdfe58ad4f2b89561`.
Normalized build ID:
`4d89f8add0023ef54d314f46860b0ffc535a0bef78bae7741b22845d20b89db3`.
Native/runtime payloads remain unchanged. `single-model-delivery-verification.json`
confirms built/staged file equality, the original Settings page byte equality and
a clean focused diff check. Concurrent changes in four main-checkout paths were
observed and preserved; no changes from this pass were written to that checkout.
The guarded installer completed at **17:24:08 UTC on 21 November** (01:24 local
on 22 November). The 67 installed renderer files match the candidate above;
native archive `e51b4678…`, executable, service and PDF/OCR worker hashes are
unchanged. No active process was terminated and no installed-user data changed.
Rollback was verified at
`D:/offgrid-ui-20261121/build/ui-install/rollback-single-model-20261122-012359`.

`installed-single-model.log` passed with exit 0 using the actual installed
executable and bundled service in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-JcACTz`. Its normalized build
matches `4d89f8ad…`; one model control, 100%/200% reflow, draft return, first-use,
Models compatibility, original directory card and unsupported recovery passed.
The synthetic retained-TXT flow again performed only its two bounded writes.
Inspected actual installed Chat at native 100% and 200%; a retained preview is
`build/ui-completion/installed-single-model-preview.png`. The installed app was
closed after qualification; no personal workspace was opened.

`installation-single-model.json` records passing post-install qualification and
the exact rollback. Final TypeScript, documentation (489 files), OpenSpec and
focused diff checks pass. No commit, push or release was made. D1 is reconciled
complete (226/256 total OpenSpec tasks); D2, capability-pack contracts, broader
personalization/journey acceptance, the reproduced unrelated Go failure, and
real-model/hardware/platform/human gates remain open. This delivery does not
claim the complete UI/UX program is finished.

## Retained cell inspection — 22 November continuation

Continued D2 in the same isolated checkout while preserving the installed
single-model correction. `DocumentCellDetails` now exposes retained display
text/provenance, value type, number format/ID, date system, formula type/range/
anchor/shared ID, cache state, merge spans and hidden-source reasons inside the
existing collapsed disclosure. Cells containing only merge metadata are no longer
silently omitted. Shared formulas without local formula text still distinguish
an absent cache from an empty retained string. Nothing evaluates formulas,
reformats source values as dates, follows source links or submits work. Counts
are localized; source identifiers and state codes stay exact. There is no new
Chat panel, model selector or always-visible document metadata block.

Twenty contextual messages have draft coverage in all 29 languages, with exact
English-source inventories. The catalog now has 1,674 messages per language.
Native-language review remains pending; refreshed handoff packs are under
`build/ui-completion/translator-cell-review`.

Verification for this increment:

- `cell-corrected-browser.log`: **470 passed, two skipped** across document
  review, reader, navigation and comparison suites. The skipped cases are the
  opt-in real-service mixed PDF and private supplied PDF. The earlier real-service
  PDF evidence remains historical, not a pass from this fixture run.
- `cell-final-browser.log`: **33 passed** after the final result-state label
  correction, covering rich cell evidence in all 29 languages, merge-only cells,
  unrendered values, dark English/Arabic, forced colors and keyboard disclosure.
  `cell-theme.log` also passed its earlier two dark/forced-color cases.
- Final API drift, TypeScript/production build, catalog check/completeness,
  translation consistency, source-size ratchet and strict OpenSpec checks passed.
  The existing large-bundle warning remains. This increment does not rerun or
  relabel the previous full renderer/desktop/Go results as new passes.
- Initial TypeScript validation caught an incorrect assumption that `useI18n`
  supplies a number formatter; corrected to the repository's existing `Intl`
  pattern. The first browser run was stopped after repeated test-locator
  failures: definition-list terms do not take the accessible names assumed by
  that locator. Matching their exact rendered labels fixed the tests. No product
  behavior or acceptance threshold was weakened.

`cell-final-performance.log` passed all three one-worker comparisons against the
unchanged `build/ui-baseline/dist`, with no concurrent browser/native test job.
Baseline/candidate navigation p95: **28.8/30.2**, **29.9/29.2**, **30.0/29.2 ms**.
Heap increased 2.6–3.0% (within +10%), requests remained 180/180, and acknowledgment
was 15.3–17.6ms (below 100ms). The prior-wording candidate also passed three
comparisons, retained separately in `cell-performance.log`. No physical 8GB/16GB
or model-latency qualification is claimed.

The final staged executable with its own bundled service passed in disposable
profile `C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-Jlec0E`
(`native-cell-final.log`). Actions were bounded to one synthetic TXT attachment
and the repository's `ordered-workbook.xlsx`, in one newly created conversation:
three writes total, no generation, correction, backup, restore or download. The
real workbook exposes formula `B6*2`, date system `1900` and unavailable cache;
source inspection and the retained draft survive actual native 100%/200% zoom.
The final cache row was scrolled into view and checked inside the viewport.
First-use, Models compatibility, the original directory card, unsupported
recovery and the single model control also passed. Visually inspected English
and Arabic narrow metadata, dark Arabic, and staged native cell views at both
zoom factors. These are not screen-reader or linguistic approval.

The guarded installer completed at **18:04:16 UTC on 21 November** (02:04 local
on 22 November). Installed raw index SHA-256:
`0ed3a1916f26131aa9cfe0015ceb329db7b5b87f485139d324cc7ed5b9911bdb`.
Normalized UI build:
`c2c1d68a688f07599d277214ac3c354d0caba9c1f4cc3fcdb2f1cd9741212b3c`.
All 67 renderer files match the candidate; native archive `e51b4678…`, desktop
executable, bundled service and PDF/OCR workers are unchanged. This is an
explicitly verified renderer increment, not a claim that new native recovery
capabilities were installed. Zero active processes were terminated and no
installed-user data changed. Verified rollback:
`D:/offgrid-ui-20261121/build/ui-install/rollback-cell-inspection-20261122-020407`.

`cell-delivery-verification.json` confirms original Settings page byte equality,
built/staged equality and a clean focused diff. Four concurrent main-checkout
changes are observed and preserved. D2's richer-cell implementation gap is
addressed; the full library/review/export acceptance remains open for
reconciliation. Broader personalization, capability-contract and external
qualification gaps listed above remain open. No commit, push or release occurred.

Actual installed verification also passed (`installed-cell-inspection.log`, exit
0) using disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-Q3LsAm`. The service reported the
exact normalized build above. The same bounded TXT/XLSX actions, draft return,
original directory card, unsupported recovery and one model control passed.
Inspected installed Chat and cell review at native 100%/200%; the final cache row
is reachable within the viewport. The installed app closed after qualification;
no personal workspace was opened. `installation-cell-inspection.json` records the
passing post-install result. Retained previews are
`build/ui-completion/installed-cell-chat-preview.png` and
`build/ui-completion/installed-cell-preview.png`. Final documentation validation
checked 489 maintained files with zero problems. OpenSpec remains **226/256**;
this complete increment does not close the wider UI/UX goal.

## Settings persistence — 22 November continuation

Reproduced five appearance/response persistence defects before correcting them:
navigation discarded the unsaved warning, the native save reported Saved before
confirmation, failure before Settings mounted was invisible, and stale native
callbacks could replace the newest outcome. `settings-before.log` retains all
five failures. `voice-before.log` separately records the missing voice-storage
failure notice. These are UI persistence defects, not changes to save ownership
or IPC authority.

Save outcomes now live in a session-local store subscribed by Settings. The
selected value stays active when storage fails; the warning survives navigation.
Appearance waits for the current native acknowledgment, and older callbacks
cannot replace it. Voice also exposes its existing draft-store failure state.
Only another explicit preference change retries the affected save. No preference
values are added to the outcome store, and no background mutation retry is added.
The existing Appearance footer carries its feedback. There is no new Chat banner
or selector, and the original directory card markup and all built CSS remain
identical to the preceding installed candidate.

Current automated evidence:

- `settings-suite.log`: **186 passed, two skipped, one test failed**. The failure
  was in the new voice recovery test: it clicked the already-selected radio,
  which correctly emitted no change. The test now selects Automatic; product
  behavior was not weakened. The skips are opt-in voice documentation captures.
- `settings-final-browser.log`: **36 passed**, including the corrected voice
  case, all persistence races and all 29 language packs. The separate
  `settings-locales.log` also passed all 29 narrow, dark locale cases. English
  and Arabic/RTL failure screenshots were visually inspected; this is not
  linguistic or screen-reader approval.
- `settings-desktop.log`: **289 passed, four skipped, zero failed**. API drift,
  final TypeScript/production build, translation completeness/consistency,
  source-size ratchet and strict OpenSpec validation passed. The existing large
  bundle warning remains. Documentation validation checked 489 maintained files
  with zero problems before the final evidence update.
- `settings-performance.log`: all **three** comparisons passed against the
  original retained baseline, with unchanged thresholds and no competing browser
  or native qualification job. Baseline/candidate navigation p95 was
  **30.2/29.8**, **29.7/29.7**, **30.2/31.8 ms**; heap increased 2.35–2.83%,
  requests stayed 180/180 and acknowledgment was 16.4–30.2ms. These are UI
  measurements, not real-model or physical-memory qualification.

The staged matched executable and bundled service passed in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-yXkTIJ`
(`native-settings-persistence.log`). The check obstructed only that profile's
atomic preference temporary path with an empty directory. A failed save kept
the prior on-disk value, retained the new session choice and warning through
navigation, then recovered after the obstruction was removed and another choice
was explicitly made. Restart restored the native preference over an older
browser cache and preserved the Chat draft. Native 100%/200% failure captures
were inspected. The same run also checked the one model selector, original
directory card, unsupported recovery, Models compatibility, and bounded synthetic
TXT/XLSX inspection with three service writes. No inference, model download,
physical control, microphone, backup or restore was performed.

`settings-delivery-verification.json` records exact directory markup equality,
unchanged CSS, built/staged equality, focused diff validation and the same four
preserved concurrent main-checkout changes. The guarded installation completed
at **18:36:07 UTC on 21 November** (02:36 local on 22 November), with all 67
renderer files verified. Raw index SHA-256:
`26fb8a19b3b6d7ac346ba8322dbc336876f247a47b882442447da323736a9225`.
Normalized UI build:
`ffcdbde5edbc54b298727a26984261d5c2d9ca8267e044fbfb4d270735b281b0`.
Native archive `e51b4678…`, desktop executable, service and PDF/OCR workers are
unchanged. Zero active processes were terminated and no installed-user data was
changed. Verified rollback:
`D:/offgrid-ui-20261121/build/ui-install/rollback-settings-persistence-20261122-023559`.
Actual installed qualification passed (`installed-settings-persistence.log`,
exit 0) in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-Dsb4o2`. Its own bundled service
reported the exact normalized UI build above. Native save failure, navigation,
explicit recovery and restart all passed, with the same bounded synthetic
document actions and no personal workspace access. Visually inspected installed
Chat and the unsaved Settings notice at native 100%/200%, plus Settings after
restart. The app closed after qualification. `installation-settings-persistence.json`
records passing post-install qualification and verified rollback hashes.
The current Chat preview is `build/ui-completion/installed-settings-chat-preview.png`.

Task 31.1 is reconciled complete: the current Settings navigation tests cover
named palettes, preview/apply/cancel/reset, corrupt storage and cross-tab text
size/motion; the existing theme key remains supported by the shared theme owner
and light/dark fixtures. The unchanged installed native presentation previously
passed all 24 terminal palette/mode combinations with an actual disposable PTY
(`offgrid-desktop-theme-pdyJln`, recorded above). Current installed checks add
real preference-write failure/recovery and restart/draft retention. This closes
the bounded palette/persistence task, not broader Settings registry, regional
preferences, full-platform or human qualification. Tasks 31.2–31.5 and the wider
journey requirements remain open. No full Go or whole-renderer rerun is claimed
by this increment; previously recorded backend and external gates still apply.
No commit, push or release was made.

## Individual Settings discovery — 22 November continuation

Continued task 31.2 against the existing controls. `settings-discovery-before.log`
reproduces three defects: response search had no link to the individual field,
recognition model labels were absent from search, and browser search offered a
desktop-only Quick window destination. The registry now uses the displayed
localized control labels and links to the existing owning section. Response
display/profile/limit, recognition/speech model, speech voice, per-mode themes,
shortcut, setup guide and diagnostics are individually reachable. Legacy section
and response links remain supported. Search/focus does not open a picker or
disclosure, change a value, start audio, or mutate service state.

A delayed voice-metadata test found that the initial enabled trigger could take
focus just before loading disabled it. Inline voice settings now begin in their
actual loading state. A direct link focuses the stable explanatory group while
the control is unavailable; later metadata never steals focus. No CSS or visual
layout was changed, and the original directory card markup remains exact.

Current checks:

- `settings-discovery-final.log`: **242 passed, two skipped**, covering the
  existing Settings navigation/organization/entry/persistence, desktop-command
  and voice-playback suites plus 35 discovery cases. All 29 language packs run
  the search → focused control → retained-draft flow at 390px, with reduced
  motion and English forced colors. Arabic/RTL and English captures were
  visually inspected. The skips are opt-in voice documentation captures.
- Earlier `settings-discovery.log`: **137 passed, one failed**. The new
  recognition-control test incorrectly used an empty audio inventory while
  expecting an available selector. Its fixture now declares availability;
  unavailable handling remains covered separately. `settings-discovery-focus.log`
  retained the real cold-link focus failure alongside two passing cases, before
  the loading-state fix. Both corrections are included in the final passing run.
- `settings-discovery-desktop.log`: **289 passed, four skipped, zero failed**.
  TypeScript/production build, API drift, catalog completeness/consistency,
  source-size ratchet and strict OpenSpec validation passed. The existing
  large-bundle warning remains; no native-language approval is claimed.
- `settings-discovery-performance.log`: all **three** one-worker comparisons
  passed against the unchanged original baseline. Baseline/candidate p95 was
  **29.6/30.2**, **29.5/29.9**, **29.7/30.2 ms**; heap increased 1.92–2.51%,
  requests stayed 180/180, and acknowledgment was 18.3–20.1ms. No competing
  browser/native test job ran during these measurements.

The staged matched app passed with its bundled service in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-XFT0U1`
(`native-settings-discovery.log`, exit 0). It verified response/voice/shortcut
discovery, closed pickers/disclosures, retained draft across navigation/reload,
one model selector, the original directory card and unsupported native recovery.
Native 100%/200% light/dark response views were captured; dark normal and light
200% were visually inspected. There were **zero service writes**, no audio,
model, terminal, backup, restore or physical-control operations, and no personal
workspace access.

`settings-discovery-delivery-verification.json` records identical directory-card
markup and CSS, exact built/staged renderer equality, focused diff validation,
and the same four preserved changes in the other thread's checkout. Raw renderer
index SHA-256: `45b00e9db770df834584cbb534b8bd5526e7bbf2a872ae3555cfab7d3eeb033d`.
Normalized build: `50c57d2e5e8904f6bc16a350d02b5b2785623d2286d369aaafd100f0ad84a9b5`.
The guarded installer completed at **06:08:53 UTC on 22 November** (14:08 local).
All 67 renderer files matched the candidate. Native archive `e51b4678…`, desktop
executable, bundled service and document workers are unchanged. Zero active
processes were terminated and no installed-user data changed. Verified rollback:
`D:/offgrid-ui-20261121/build/ui-install/rollback-settings-discovery-20261122-140832`.

Actual installed qualification passed (`installed-settings-discovery.log`, exit
0) in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-AGCe1W`. Its own service reported
the exact normalized build above. Individual-setting focus, unavailable voice
explanation, closed pickers, shortcut state, reload/draft retention and zero
service writes all passed. Visually inspected installed Chat, response settings
in both themes at native 100%/200%, and recognition settings. The app closed
after qualification. `installation-settings-discovery.json` records the verified
installed result and rollback hashes. Current previews are
`build/ui-completion/installed-discovery-chat-preview.png` and
`build/ui-completion/installed-discovery-preview.png`.

Task 31.2 is reconciled complete. This does not close the full document-library,
regional/workspace preferences, cross-client, full-CI, real-model, hardware,
platform or human qualification requirements. Earlier backend failures remain
recorded and no full Go rerun is claimed here. No commit, push or release occurred.

## Library return and single-model recheck — 22 November continuation

The library retained its filter and source selection but reset pagination on
every mount. Returning from Chat to page three therefore showed page one while
the chosen source remained elsewhere. Two English/light and Arabic/dark
regressions reproduced this failure in `library-return-before.log`. Pagination
now uses the existing account/workspace session state. Only a changed filter
resets the page; a shorter result list displays its last valid page. No CSS,
labels, directory-card markup, service contract or native code changed.

Current increment checks, under `build/ui-completion`:

- `library-return.log`: **72 passed**, collection-management and
  knowledge-collections. An additional nonexistent `library-flow.spec.ts` path
  on that command matched no tests and is not counted as coverage.
- `library-return-integration.log`: **62 passed**, knowledge-returning,
  knowledge-scope, knowledge-simple-flow and knowledge-experience. These cover
  page/filter/selection return, shorter lists, scope and draft retention.
- TypeScript/production build, API drift, source-size ratchet and strict
  OpenSpec validation passed. The existing large-bundle warning remains.
- `library-return-performance.log`: **three passed** against the unchanged
  original baseline. Baseline/candidate p95: **41.6/29.6**, **37.0/34.8**,
  **33.3/33.7 ms**. Candidate heap increased **2.12–3.71%**, requests were
  **181/180**, **180/180**, **180/180**, and acknowledgment **16.9–35.6 ms**.
  Existing thresholds were unchanged. These are UI fixtures, not inference.
- The preceding Settings desktop result (**289 passed, four skipped**) is
  retained evidence for unchanged desktop code, not a new desktop-suite run.

The staged matched package passed in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-7qOCzq`. The library portion
intercepts only document and RAG GET responses with 65 synthetic document
records. It verifies page-three selection return, a list shortened to 35,
filter reset and Chat draft retention without importing files or writing to
the service. Its 100% and native 200% library captures were visually inspected.
Other checks use the actual owned bundled service and verify one composer
model selector, Settings discovery, directory-card preservation and truthful
unsupported recovery. The run recorded **zero service writes**.

`library-return-delivery-verification.json` verifies exact built/staged equality,
unchanged CSS and directory markup, all three performance samples and a focused
diff check across 85 snapshotted paths. Four concurrent main-checkout changes
remain preserved. The main checkout was not written.

The guarded installer completed at **06:31:22 UTC on 22 November**. Raw renderer
index SHA-256 is
`30e9b8046559e2317ec5d816381e53b7a680db1c749f3c2226cb415c56ede5ed`;
normalized build is
`d1517ddbfbc743a89d840dcf42ac33bd06c44b9775adc38915102ca88caca415`.
All 67 renderer files matched. Native archive `e51b4678…`, desktop executable,
bundled service and document workers are unchanged. No active processes were
terminated or installed-user data changed. Verified rollback:
`D:/offgrid-ui-20261121/build/ui-install/rollback-library-return-20261122-143105`.

Actual installed qualification passed (`installed-library-return.log`, exit 0)
in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-ui-owned-EZcp6k`. The bundled service
reported the normalized build above. The same library-return and Settings
checks passed, and an additional **820×560 CSS-pixel** window reproduced the
owner's long-draft example in both light and dark appearances. Both showed one
composer model selector, no separate setup banner and a fully visible composer.
The empty-inventory explanation appeared only inside the opened selector;
Escape restored focus and retained the draft. These small-window Chat views,
the light model menu and library views at native 100%/200% were visually
inspected. This window-size test does not claim Quick Access mode coverage.

The run recorded zero service writes and closed its temporary app. The finalized
`installation-library-return.json` retains verified installed/rollback hashes
and evidence. The current light Chat preview is
`build/ui-completion/installed-library-chat-preview.png`. Documentation checks
passed for 489 Markdown files using `scripts/maintenance/check-docs.mjs`; an
earlier command used a nonexistent `scripts/quality/check-docs.mjs` path and
failed before running any checks (`library-return-docs.log`).

The broader D2 library/review/export and full-program qualification tasks remain
open; this focused navigation correction does not close them. No real-model,
human accessibility, language or other-OS qualification is claimed.

## Quick Access sizing at native zoom — 22 November continuation

The actual installed app reproduced a separate Quick Access defect at native
200% zoom: the input viewport became 340×82 CSS pixels and the model selector
ended at y=123, below the window. `command-zoom-before.log` and disposable
profile `offgrid-command-U1gVrh` retain the native failure and cropped capture.
The new browser regression also failed before correction
(`command-zoom-browser-before.log`).

The shared command-layout hook now checks whether the compact composer's
contents fit and requests the existing work layout when necessary. It retains
that layout at the clipped width so the resulting resize does not immediately
shrink the window again. Widening permits the compact input size. No CSS,
labels, native host, service or IPC contract changed, and the composer retains
one model selector.

Native transition checks found two measurement hazards: intermediate animated
bounds, and `innerHeight` reporting a new size before CSS viewport units caught
up. Both must settle before detecting clipping. Their failures remain in
`command-zoom-native-final.log`, `command-zoom-native-settled.log` and
`command-zoom-native-trace.log`. The browser regression also simulates the stale
workspace geometry, without claiming that viewport resizing is native zoom.

Current source checks:

- Initial command suite: **13 passed, one timed out** in Arabic's narrow model
  menu (`command-zoom-browser.log`). The unchanged Arabic case and the new zoom
  case then passed together in `command-zoom-focused.log`.
- Command/voice suite: **74 passed, two skipped** in
  `command-zoom-final-browser.log`, before the final CSS-viewport timing guard.
  The two skips are opt-in voice documentation captures.
- Final affected layout/voice selection: **22 passed** in
  `command-zoom-settled-browser.log`. The expanded stale-viewport regression
  passed separately in `command-zoom-viewport-browser.log` (**one passed**).
- Production TypeScript/build and API checks passed. The final build's actual
  native exit status is retained as **0** in `command-zoom-build-exit.txt`;
  earlier PowerShell stderr handling reported a shell failure on Vite warnings
  despite completing the build. Existing large-chunk/config-loader warnings
  remain. Source-size, documentation and strict OpenSpec checks passed.

The packaged command check uses a disposable profile and an HTTP fixture, with
show/focus suppressed. Native 100%/200%, light/dark and English/Arabic geometry,
menu access, focus return, draft retention, dismissal, expansion, reversal and
reduced motion are checked without model execution or physical microphone use.
Initial native screenshots returned stale hidden-compositor frames, so those
images are not visual acceptance evidence. A screenshot assertion also wrongly
expected the closed popover to be removed; the corrected check verifies its
actual hidden state. These test/capture failures remain recorded.

An initial hidden-native stability run passed in disposable profile
`C:/Users/phil/AppData/Local/Temp/offgrid-command-k95sB3`
(`command-zoom-native-stability.log`). It samples geometry and opacity for a
second at each locale/theme/zoom combination, rejecting recurring resizing or
fading, then captures the settled native frame. English/light and Arabic/dark
native 200% screenshots were visually inspected: the single model selector and
input actions are reachable. The native geometry is 680×164 at normal zoom and
680×480 when enlarged, using the existing bounded work layout. At 200%, that
larger window provides 340×240 CSS pixels. Resetting zoom returns to the compact
size. No physical shortcut delivery, audio or model result is claimed.

`command-zoom-performance.log` passed all **three** comparisons against the
unchanged original baseline, with no concurrent browser/native qualification
job in this thread. The final measurements are retained in the delivery
verification receipt. This measures UI cost only. No threshold or baseline
was relaxed.

That candidate was installed at **07:38:22 UTC**, but its installed qualification
then exposed an additional menu-close/zoom race in Arabic/dark: an older input
resize could arrive after zoom changed. It was **not accepted as qualified**.
`installed-command-zoom.log` retains the failure, and
`installation-command-zoom-failed.json` records the guarded restoration at
07:47 UTC of the last qualified library renderer (`30e9b804…`). All previous
files and the failed payload were retained, with zero terminated processes or
installed-user data changes.

`command-zoom-race-before.log` independently reproduces the handoff in a browser
fixture: closing the open model menu at the enlarged viewport wrongly requests
input height. The corrected hook also considers the ratio of native outer
width to the zoomed viewport, including tolerance for frame borders. It can
retain work height before the older close callback requests a smaller window.
The ratio was measured in the installed host (`command-zoom-ratio.log`): native
outer width 694, content viewport width 340 at native 200%. No bridge or native
binary change is required. The fixture explicitly models that native geometry;
native zoom remains separately qualified in the matched app.

The corrected candidate passed **75 command-window/voice tests, two optional
documentation-capture skips** (`command-zoom-race-browser.log`), **two IME/focus
tests** (`command-zoom-ime.log`), and **three** final performance comparisons
(`command-zoom-race-performance.log`). Baseline/candidate navigation p95 was
**32.9/29.1**, **29.0/28.2**, **31.4/32.2 ms**; heap increased **2.80–3.78%**,
requests remained **180/180** in all samples, and acknowledgment was
**24.8–26.9 ms**. The original baseline and thresholds were retained.

The corrected native matrix passed in `offgrid-command-ssUybn`; the matched
owned-service check passed in `offgrid-ui-owned-aMJoyX`. The delivery verifier
checks exact built/staged bytes, all unchanged CSS, unchanged directory markup,
86 snapshotted paths, preserved native/runtime hashes and zero main-checkout
writes. The same four concurrent model/translation source changes remain
preserved.

The corrected package was installed at **07:53:51 UTC on 22 November** (15:53
local). Raw renderer index SHA-256:
`a08cc705421a9f2b560eb089025046b3939e5049a15469a33ae9268cedd79a99`.
Normalized UI build:
`574e632c1568ec6ab77774ca351402ff269049bf396946df664b2f0b20cb2b45`.
All 67 renderer files match the candidate. The native archive, desktop
executable, service and document workers are unchanged. Verified rollback to
the previously qualified library renderer:
`D:/offgrid-ui-20261121/build/ui-install/rollback-command-zoom-20261122-155343`.
No active processes were terminated or installed-user data changed.

Actual installed qualification **passed** in both disposable profiles:

- `offgrid-command-AhAUOa`: final native zoom/menu/stability matrix, using HTTP
  fixtures, with one window, retained draft, restored bounds and zero writes.
  English/light at native 100%/200% and Arabic/dark at native 200% were visually
  inspected. All English/Arabic and light/dark combinations ran automatically.
- `offgrid-ui-owned-DUxzqN`: actual bundled service identity, normal small Chat
  in light/dark, a single model selector, Settings discovery, retained draft,
  original directory card and truthful unsupported recovery all passed. Library
  responses alone are synthetic GET fixtures; no documents were imported.
  Both small Chat views and the light empty-model menu were visually inspected.
  Zero service writes were recorded.

Both temporary apps closed. `installation-command-zoom.json` contains the
finalized installed/rollback identities and both native evidence paths. The
current Chat preview is `build/ui-completion/installed-zoom-chat-preview.png`.
Task 46.5 is complete for its stated isolated automation boundary. Browser IME
events are not physical OS IME qualification; native captures are not a human
accessibility review. Physical shortcuts, real audio/models, other platforms,
native-language/participant studies and the unresolved backend/full-CI gates
remain separate and open. The full UI/UX program is not declared complete.
