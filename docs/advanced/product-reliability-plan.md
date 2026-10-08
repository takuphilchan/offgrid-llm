# Product reliability and usefulness plan

This is the implementation plan following the September 2026 system review.
It is a delivery checklist, not a claim that every capability is production-ready.

The [approved production-readiness delivery contract](production-readiness.md)
defines the next five milestones and release gates. The historical stages below
record earlier work; their checked boxes do **not** imply that the newer milestones
or cross-edition qualification have passed.

## Product outcome

Make OffGrid a dependable private workspace for conversations, document-grounded
answers, and governed tasks. Browser, desktop, CLI, and external agents use the
same Go runtime and behavioral contracts. Keep Go, React/TypeScript, Electron,
and the native OffGrid provider adapters. Do not introduce microservices or a
framework migration to solve local state and lifecycle problems.

## Delivery rules

- Ship small vertical slices with service, API, UI/CLI, documentation, and tests.
- Preserve existing models and data. Never silently claim legacy data for the
  first signed-in user, or replace failed persistent storage with volatile data.
- Authorization belongs at the service boundary as well as the HTTP boundary.
- An unavailable dependency must produce an explicit capability state.
- A successful HTTP response or CI build is not proof of a useful model task.
- Never replay side effects automatically after a crash or approval pause.
- Keep experimental capabilities visibly separate from supported workflows.
- No automatic publishing: validate locally before a separately approved release.

## Workspace outcome experience: baseline (2026-10-08)

Implementation of `workspace-outcome-experience` starts from HEAD
`08df9da72ae49991571faca60cd7c011421134e9` **plus** the retained, uncommitted
shell/recovery changes. The pre-change renderer build ID (SHA-256 of index.html)
is `64feef56af8248b92c5e06e5ec33a3629509d8f9d3e19723cf788596fed78d5d`.
The sorted path/SHA-256 list for tracked and untracked renderer source/tests and
the desktop-theme script hashes to
`bd59318e3ab635f83facbfbabd1f6009bcd96db8d76cfd1f39fe9c7ac925a5e8`.
This fingerprint precedes the new comparison test; it is not a clean Git revision.

Baseline verification on Windows: TypeScript, production UI and fresh Go build
passed. The four requested suites (workspace-shell, workspace-experience,
task-first, control-consistency) passed **39/39**, using temporary isolated service
`offgrid-web-qualification-JFM1Pl`. The additional comparison suite passed **6/6**
in `offgrid-web-qualification-EYqhDe`. Baseline screenshots are preserved locally
under `build/workspace-outcome-experience/baseline/test-results`, rather than the
overwritten Playwright output. No installed data or container was touched.

The comparison suite records all six pages in empty, populated, active-task,
blocked-setup/task-access, failed-read and completed-task workspace fixtures at
1440x900. Background task state does not fabricate a corresponding busy state on
Settings or other idle pages. Empty-page comparisons also use 1280x720, 390x844,
320x480, 1280x400 and a 720x450 effective viewport. That final size simulates
reduced available space, **not** actual browser/OS 200% zoom qualification.
Existing shell/control suites additionally record both themes, German and Arabic,
all-locale menus and keyboard focus. Fixtures mock API responses; passing them is
not an inference, native permission or installed desktop result.

Frozen journey/oracle pairs for before/after comparison:

| Journey | Independent expected observation |
| --- | --- |
| Ready Chat and follow-up | One accepted turn per Send; returned saved messages contain both turns |
| Missing-model setup and return | Existing draft unchanged; no implicit model transfer or Send |
| Knowledge setup and source | Explicit retrieval choice; actual authorized source; truthful whole-base scope |
| Voice preferences, dictation, playback | Synthetic capture edits draft; opt-in speech stops; no automatic submission |
| Ordinary task and result | One stable job; no up-front computer setup; actual saved artifact metadata |
| Local access and approval | Exact job/input/approval identity; no expanded target or implicit grant |
| Leave and return to active work | Same work ID; bounded followers; lifecycle acts only on selected work |
| Lost acknowledgment and recovery | Read reconciliation; no duplicate or uncertain-action replay |
| Model discovery and acquisition | Correct category, source and size; one explicit transfer using synthetic files |
| Interrupted model operation | Authoritative retained state; explicit resume; unrelated model preserved |
| Document source/index/deletion | Protected source readable only when allowed; selected deletion confirmed |
| Activity, history and preferences | Canonical known work; eligible snapshot deletion; draft survives navigation |

Observed starting detours: Chat's model/Knowledge/response settings are above the
conversation, separate from its editor; empty histories show bulk controls;
task entry repeats the Task label and instructions; Models presents public search,
installed inventory and a separate catalog filter in one long page; Knowledge
places indexing maintenance alongside document actions; Activity leads with four
runtime counters; Settings repeats version/service facts. These are source and
screenshot observations, not measured participant click counts. Human timings,
mistakes, assistance, actual zoom/screen-reader and installed OS checks remain
unrun. The five-person formative study and existing production pilot remain open.
Baseline JS is 748.54 kB raw / 229.10 kB gzip; CSS 85.04 / 16.24 kB. The existing
500 kB bundle warning is retained, not suppressed.

The first implementation slice shares headings/action groups, scoped notices and
empty states across Chat, Agents and Activity. TypeScript and production build
passed, followed by 15/15 shell/control tests in isolated service
`offgrid-web-qualification-s97QYw`, including both themes, forced colors, keyboard
focus, long labels, 320px layouts and nine-locale shell menus. No feature authority
or service API changed.

Composer/history slice: Chat's model/context/voice controls are adjacent to its
editor; optional response metrics are disclosed on request. Task-first entry uses
the same grammar without duplicating its visible label or setup prerequisites.
Chat/task-first bulk actions now live in Manage history; individual deletion stays
visible and protected task reasons stay available. New labels cover nine locales.
The existing voice-preference English literals remain a later localization task,
not a claim of complete localization.

After the changes, 124/124 focused renderer regressions passed with one worker
and a fresh native service binary in `offgrid-web-qualification-AIOrN3`: chat
streaming, voice playback/capture cancellation, recovery, task-first, agent layout,
interaction contracts, control consistency, task/history management, workspace
experience and new setup-handoff tests. Smaller iteration suites exposed/fixed a
popover-to-confirmation focus restoration bug and stale cross-tab draft cache
after navigation. Updated task and Chat screenshots were inspected against the
preserved baseline. These remain API-boundary fixtures, not real inference or
native-control qualification.

Final renderer verification for this slice passed **186 tests with 10 opt-in
skips** in `offgrid-web-qualification-Rv8sUt` (four workers, 37.2 seconds).
The preceding full run had two presentation assertions measuring hover rather
than resting button color; moving the test pointer away before those assertions
fixed the test setup without changing product colors. Final Chat/task screenshots
were inspected after the composer typography and desktop-history-toggle fixes.
API drift, TypeScript/build, 49 desktop unit tests, six documentation-check tests,
maintained-document links and strict change validation passed. Full Go/race,
Python, companion, acquisition/restart fixtures, installed-platform and human
qualification were not rerun for this slice; integrated tasks remain unchecked.
Tasks 1.1, 1.2 and 2.1–2.3 are complete (5/30), not the full experience program.

The owner confirmed explicit legacy restoration on 2026-10-08. Task 2.4 now uses
actor/workspace-scoped composer drafts and active-chat selection, separate
copy-only recovery slots and explicit Restore/Keep separate controls. Live draft
values (including cleared tombstones) take precedence during concurrent writes;
originals and execution/approval records remain untouched. Unknown workspace
identity uses disclosed session-memory drafts. A removed conversation on setup
return no longer selects a different conversation. Tests cover copying, conflicts,
concurrent edits, storage failure, reload, missing identity, account/workspace
switches and handoff permissions. Broader work-continuity implementation follows.
The final migration renderer run passed **195/205** tests with **10 opt-in skips**;
the focused return/recovery suites passed **26/26**. TypeScript, UI/native build,
maintained docs and strict OpenSpec validation passed. The source scope is 6/30
completed tasks at that checkpoint. Missing-workspace session memory is disclosed
once rather than also being mislabeled a failed disk write.

The next continuity slice adds a bounded Known work surface, job references from
the existing listing and selected/submitted/recovered chat references. It holds
no parallel execution data, limits retained titles, reuses mounted followers and
stops terminal task polling. Initial continuity/shell/handoff checks passed 26/26;
expanded continuity, task-first, streaming, shell and voice checks passed 50/50.
These fixtures include scope changes, overflow, canonical navigation and logout;
Task 3.2 subsequently passed **51/51** continuity, Chat streaming, task-first and
voice-playback renderer checks against a fresh native build. The new fixtures
cover view detach/return and reload, lost acknowledgments without resubmission,
late events from a different conversation, unavailable turn recovery, older
durable job cursors and removal of a followed task. They assert read-only recovery
and no audio replay; these are renderer/API fixtures, not model or native-control
qualification. Terminal job followers stop; interrupted work remains inspectable.
Tasks 3.3–3.5 now separate exact approval review, result/artifact presentation and
same-task correction. Expired approvals refresh reads instead of dispatching
resume; unknown operations keep canonical arguments visible. Pending and
unconfirmed Stop are explicit. Task artifacts use the existing authorized
download path, a 128 KiB read bound and byte/digest checks, with no executable
preview or factual-correctness claim. Correction acknowledgment recovery uses
the existing instruction ID; unsupported continuation prepares a separate draft.

The combined slice passed 54/54 focused renderer tests, then the rebuilt full
suite passed **224/234, with 10 opt-in skips**, including an additional late-save
draft-preservation case. This run includes the full existing Chat/Agents/model/
voice/shell regression fixtures. API generation, TypeScript/production build,
native Go build, 49/49 desktop unit tests, 6/6 documentation-checker unit tests,
89 maintained-document link checks and strict OpenSpec validation passed.
Result screenshots at 320px in both themes were inspected; these are shared
renderer tests, not new installed macOS/Linux, native-control, real-model or
human-usability qualification. An initial fixture used the wrong approval label;
another run found colliding React sibling keys introduced during extraction,
which were fixed before the passing full run. No unresolved test failure is
counted as passing evidence.

The model-library stage subsequently passed TypeScript/build, 18 library/recovery
browser checks, and seven fixture-tagged real-service acquisition/restart checks.
The latter exercised whisper, Piper, streaming Zipformer and Kokoro synthetic
packages, cancellation, pinned resume after restart, repair and deletion; they
are storage/transport tests, not real speech inference qualification. Evidence:
`C:\Users\phil\AppData\Local\Temp\offgrid-web-qualification-fObcDQ`.
Installed/Discover preserve independent state, legacy and speech reviews stay in
context, model sizes/provenance/readiness remain distinct, and failed operation
reads no longer hide successful inventory reads. No replacement downloader was
introduced. Legacy acquisition still lacks package-style immutable-revision and
disk-space preflight contracts, explicitly disclosed rather than simulated.

Management and quality implementation continued on 2026-10-08: Knowledge now
foregrounds documents/sources, Activity links known canonical tasks independently
of diagnostic failures, Settings separates preferences/workspace/diagnostics, and
tools/connections use the shared list/action grammar. Voice preferences and local
speech feedback use nine typed locale dictionaries. Drafts and replay cursors are
scope-bound; late prior-scope snapshots cannot repopulate cursor state.

Evidence for this continuation:

- The first integrated build passed 261 renderer checks with 10 opt-in skips
  (`offgrid-web-qualification-njRRY2`). The expanded final run passed **275/285
  with 10 opt-in skips**, `offgrid-web-qualification-eW7Shr`; no test failures.
- The new 12 stateful cross-page journey fixtures passed in
  `offgrid-web-qualification-DK8ebp`. Assertions inspect submitted IDs, exact
  approval/access targets, saved-message state, artifact bytes/digests, selected
  transfer and removal targets, and preserved drafts. These are API-boundary
  fixtures with synthetic speech and host contracts, not real inference or input.
  Initial failures were invalid fixture fields/labels and synthetic media state;
  those failures were corrected, not waived.
- Seven tagged real-service acquisition/restart checks passed again in
  `offgrid-web-qualification-ke2mgZ` (four synthetic speech architectures).
- `go test ./...` passed on Windows. The exact CI race package set passed under
  WSL Ubuntu with `TMPDIR`/`GOTMPDIR` on `/var/tmp/offgrid-workspace-race` and a
  project-drive cache. Earlier attempts hit the full WSL `/tmp`, then a mounted
  Windows filesystem restore limitation; Linux-filesystem restore tests pass.
  Desktop unit tests: 49; computer companion tests: 53; Python client tests: 19.
  These do not qualify live native effects on untested operating systems.
- TypeScript/build, API drift, six documentation-checker tests and 89 maintained
  document link checks passed. No model downloads or user recordings were made.
- Matching Windows Electron 43.4.0 test package passed theme switching, all nine
  readable locale options, persisted appearance, and genuine 200% browser zoom.
  Zoom evidence uses Electron capturePage: CDP full-page screenshots at zoom
  produced cropped captures and were not accepted as visual evidence. Current
  evidence: `offgrid-desktop-theme-kP5nSE`. Startup/recovery checks passed in
  `offgrid-desktop-startup-qFb9wy` (first window 552ms, recovery 694ms).

Measured frontend fixtures (not model latency): 1,000-session filtering p95 fell
from 142.1ms before paging to 40.9ms in the one-worker check; initial rendered rows
are bounded to 20 and Show more adds 20, while filtering/deletion use all matches.
Thirty synthetic stream updates measured 59.6ms p95; local Stop feedback 21.9ms.
The current one-worker cold Chat navigation was 257ms; subsequent routes were
33–61ms. Navigation includes local fixture transport; it is not isolated parsing
or inference cost. Final JS: 815.84/252.78 kB raw/gzip; CSS: 92.44/17.69 kB,
under 15% growth from the recorded bundle baseline. The 500kB warning remains.
Group 1 did not preserve timed cold/warm route measurements, so no before/after
route-speed claim is made and task 6.3 remains open for that comparison.

Manual screen-reader checks, the five-participant formative study, and installed
macOS/Linux runs remain unrun (6.2, 7.2, 8.4 stay unchecked). Automated keyboard,
IME, reduced-motion, RTL, all-locale, narrow-layout and sampled contrast checks
are not substitutes. No competitor parity, production pilot, release readiness
or completed voice-runtime program is claimed. No commit, push or release occurred.

Local deployment verification (2026-10-08): stopped container `038495753fc1`
was backed up to the private WSL directory
`/home/phil/offgrid-workspace-backup-jktY3I`. The stopped-workspace ZIP passed
`workspace verify`; the separate 11 GB model archive passed archive inspection
and both archives passed SHA-256 verification. Full configuration stays in that
private directory, not in this document. No work, transfer, or computer session
was active when the service was stopped. The original image/container remains
stopped for matched-backup recovery; neither data/model volume was removed.

The first post-activation check caught a retained `OFFGRID_UI_DIR` override
selecting `/usr/local/share/offgrid/ui` from the old runtime instead of the new
renderer at `/var/lib/offgrid/web/ui`. This was an actual deployment mismatch,
not a cache problem. Both the local-runtime Dockerfile default and the recreated
container override now select the matching build. The service health endpoint,
normalized HTML identity, and served JS/CSS bytes passed validation. Ports,
mounts, network, security/restart configuration, and CPU-only device configuration
were preserved. Exactly one serving container was running. Read-only browser
checks covered all six pages at 1440px and 390px: no mutation requests, failed
HTTP responses, or page exceptions. Visual inspection then found a library
selected-state CSS-layer collision; the final control layer now owns these
states, with light/dark and hover regression checks.

The Windows installation remains version 0.4.12 with an older renderer. Container
replacement does not update it, and its compatibility check was not bypassed.
The matching isolated desktop test package is not an installed-app update.
Dependency audit reports zero production advisories but two high-severity
build/development dependency findings (`brace-expansion`, `source-map-js`).
They remain a separate dependency-review gate before release; no broad automatic
dependency upgrade was performed during this UI testing deployment.

Follow-up proportion/ownership refinement (2026-10-08): removed duplicate
Service details links from navigation and workspace options; Settings remains
the canonical service/diagnostics surface. Saved-answer Copy, Read aloud and
Response details now share that answer's footer, with keyboard/outside dismissal
and no metrics attributed to a subsequent in-flight answer. The desktop header
is 64px, page spacing 24px, reading measure 760px, and composer maximum 800px.
Controls retain 40px minimum targets and the existing monochrome palette. The
task editor no longer has a second enclosing card or an empty-history New task
button. Installed-model metadata shares a compact row; model selection has an
explicit outlined control. Activity work rows align titles and states instead
of presenting centered full-width button text. Settings content is bounded to
960px. These are measured layout changes, not evidence of competitor parity.

Validation of that refinement:

- TypeScript/build, API drift, six documentation checker tests, 89 maintained
  Markdown files and strict change validation passed. Final full renderer run:
  **278 passed / 288, 10 opt-in skips, zero failures**; isolated service state
  `offgrid-web-qualification-juo4Iu`. Screenshots were retained under
  `build/workspace-outcome-experience/refined-ui-results` and visually inspected.
- Final bundle: JS 816.00/252.81 kB raw/gzip; CSS 92.91/17.70 kB. The latest
  1,000-conversation history fixture measured 40.7ms p95, retaining 20 initially
  rendered rows. Cold fixture Chat navigation 243ms; subsequent routes 27–53ms.
  No historical route-latency comparison or model-speed improvement is claimed.
- Rebuilt unpacked Windows Electron 43.4.0 passed the isolated packaged theme
  check, including nine locale options, actual 200% zoom and saved appearance
  after restart. Evidence: `offgrid-desktop-theme-96cb9J`. This does not update
  the user's installed Windows application or qualify macOS/Linux.
- Fresh private stopped-workspace backup:
  `/home/phil/offgrid-refined-backup-LLtDYl/workspace.zip`; both `workspace verify`
  and SHA-256 validation passed. The separately verified model-store backup
  remains in `/home/phil/offgrid-workspace-backup-jktY3I/models.tar`.
- Replacement image `offgrid-llm:workspace-refined-20261008`, immutable ID
  `sha256:440047b6050d205988ab8f15f3b599997ebd87b93fe393458d0e2afb7a080b7b`;
  container `ff01b4917ebe`, version 0.4.14, local revision
  `08df9da-workspace-refined-local-20261008`. UI identity:
  `29881315b28a70b794dcbecfec86682994c403a44195f646e43f62d802372aff`.
  Served assets `index-Cdqgly1C.js` and `index-DZKTgQuH.css` matched installed
  bytes. `/health` was healthy; exactly one serving container, `offgrid`, ran.
  Previous `7158a6b99bed` is stopped as
  `offgrid-before-refined-1791471693623`; no volume was removed.
- Full container host configuration and mounts matched the prior installation.
  Inventory preserved: three chat models, one embedding model, two speech
  packages, three conversations, zero jobs/documents. Read-only post-deployment
  browser inspection covered all six pages at 1440px/390px, with zero writes,
  HTTP failures or page exceptions. Screenshots:
  `build/workspace-outcome-experience/live-ui`.

Tasks 6.2, 6.3, 7.2 and 8.4 retain the evidence limitations above. This is a
local testing deployment, not completion of the usability/installed-platform
qualification gates or authorization to publish a release.

Shared-refresh cleanup (2026-10-08): normal Knowledge, Activity, catalog,
speech inventory, tools and connections headings no longer repeat the header's
Refresh button. Scoped failed-read, approval and operation recovery controls
remain. The shared action refreshes mounted data without remounting editors.

- Final isolated renderer run: **279 passed / 289, 10 opt-in skips, zero
  failures** in `offgrid-web-qualification-YYBZxD`. The new regression checks
  header reads across all four model categories and both library views, tools,
  connections, Knowledge and Activity, plus Chat/task draft preservation and
  zero mutations. The initial new test used the wrong tab label (`Discover`
  instead of `Discover models`); it was corrected before this complete rerun.
- TypeScript, native build, production UI, API drift, documentation checks and
  strict OpenSpec validation passed. Bundle JS: 815.16/252.72 kB raw/gzip;
  CSS unchanged at 92.91/17.70 kB. Fixture history filtering p95 was 35.4ms,
  stream rendering p95 56.7ms and local Stop feedback 10.3ms. Cold fixture Chat
  navigation was 262ms and subsequent routes 41–79ms; these are observations,
  not a recovered historical baseline comparison or inference benchmarks.
- Rebuilt unpacked Windows Electron passed the isolated theme/locale, true
  200% zoom and restart check (`offgrid-desktop-theme-U0C5y6`). The harness now
  tests Activity's Diagnostics disclosure at zoom instead of requiring the
  deliberately removed page-level Refresh button. It also asserts actual focus.
  The installed user application was not replaced; other OS gates remain open.
- Replacement image `offgrid-llm:workspace-refresh-20261008`, immutable ID
  `sha256:adb39e0e9da963b1b67662287cdec2aa3fdc67628e27b817ac3e5c027fbf4b97`;
  container `57607180ef47`, version 0.4.14, local revision
  `08df9da-workspace-refresh-local-20261008`. UI identity:
  `a7336324e4ea09e8a9f5e25aabf2a53c2f9cb39b81eb990a081ba9a84e66f617`.
  Served assets `index-C3XaTqQf.js` and `index-DZKTgQuH.css` matched image bytes.
- Verified stopped-workspace backup:
  `/home/phil/offgrid-refresh-backup-WNTMO3/workspace.zip`; archive validation
  and SHA-256 check passed. Previous container `ff01b4917ebe` is stopped as
  `offgrid-before-refresh-1791473097913`. Existing model backup and live volumes
  are retained. Before/after inventory matched the user's state at replacement:
  four legacy models, two speech packages, two conversations, one job and zero
  documents. Full host configuration and mounts matched; `/health` was healthy;
  exactly one serving container (`offgrid`) ran.
- Read-only live inspection passed all six routes at 1440px and 390px, with no
  writes, HTTP failures or page exceptions. Screenshots were inspected under
  `build/workspace-outcome-experience/live-ui`; renderer evidence is retained in
  `build/workspace-outcome-experience/refresh-ui-results`.

The OpenSpec change remains **26/30**: manual screen-reader evidence, the exact
pre-change latency comparison, the five-person formative study and remaining
installed-platform checks are not supplied by this deployment.

### Workspace release preparation (2026-10-09)

- Updated the locked Electron runtime from 43.4.0 to 43.7.9 and compatible
  transitive build dependencies. `npm ci` succeeds for web and desktop. The web
  audit reports zero vulnerabilities; the desktop audit reports zero high or
  critical findings and eight remaining moderate build-chain findings. The
  browser companion audit reports zero. No forced builder downgrade was used.
- API drift, TypeScript, production UI build and all 49 desktop unit tests pass.
  The complete renderer suite passes **279/289, ten opt-in skips, zero failures**
  in `offgrid-web-qualification-Ob47Mh`. These are automated fixture checks, not
  real-model qualification or the pending human usability study.
- The rebuilt, isolated Windows package passes theme/locale synchronization,
  all six routes at true 200% zoom, keyboard focus and appearance after restart
  in `offgrid-desktop-theme-DnMb1V`. The first runs exposed a test race: a generic
  last-button locator focused Knowledge's disabled loading control before the
  intended action appeared. The harness now waits for the committed route and
  the named, enabled Knowledge action, retaining the actual-focus assertion.
- All five release-automation suites pass against a clean LF export of the
  committed scripts in `/home/phil/offgrid-release-checks-niCO1k`. The initial
  WSL run against a Windows CRLF checkout failed the workflow-step text parser.
  A temporary export then encountered an already nearly full `/tmp`; its own
  partial files were removed and the bounded script export used `/home/phil`.
  No workspace data, models or unrelated temporary files were removed.
- Publication still requires passing CI at the exact release revision and the
  existing complete-asset/container gate. These local checks neither close the
  remaining OpenSpec qualification tasks nor update the installed user app.

## Workspace redesign stage 2: shared shell and controls (2026-10-08)

- Implemented OpenSpec change `workspace-shell-foundation`: compact header,
  remembered desktop navigation collapse, and workspace options for language,
  theme, service details and account actions. Existing routes, authorization,
  drafts and page lifecycle remain intact. New shell labels cover all nine
  locales; mobile navigation remains independent of desktop collapse.
- Navigation/header presentation and geometry now have dedicated modules.
  Shared native utility popovers dismiss on outside interaction, Escape and
  navigation, keep keyboard focus behavior, and remain bounded on narrow or
  short viewports. Voice settings retain request cancellation and late-response
  protection; these utility panels do not replace consequential confirmations.
- The requested control-consistency correction gives secondary actions visible
  outlined button boundaries, including voice, microphone, read-aloud, setup and
  recovery controls. Route actions keep anchor semantics with button styling;
  actual content references remain links. Existing monochrome colors, fonts and
  focus indicators are preserved; no color redesign is implied.
- Windows verification passed: API drift, TypeScript, production UI build,
  native service build, 49 desktop unit tests, 6 documentation-check tests,
  maintained-document links and strict OpenSpec validation. The existing
  >500 kB bundle warning remains. The desktop presentation smoke script was
  syntax-checked and its selectors updated; installed-package execution was
  not rerun.
- The final full production-renderer suite passed 172 tests with 10 opt-in
  skips through `dev/scripts/test-web-workspace.mjs`, using a fresh service
  binary and disposable data in Windows temporary directory
  `offgrid-web-qualification-brp5SO`. Coverage includes keyboard and pointer
  dismissal, aborted voice metadata reads, draft retention, layout preferences,
  all nine locales, short/narrow viewports, RTL, both themes and mocked Electron
  presentation synchronization. Representative shell and light/dark control
  screenshots were inspected. Browser fixtures do not qualify speech inference,
  operating-system permissions or installed desktop behavior.
- Full Go/race, Python, acquisition/restart model fixtures, installed-package
  and other-OS suites were not rerun for this renderer-only slice. Earlier
  stage-1 acquisition evidence below remains separate. No container/application
  replacement, model download, real microphone capture, native target operation,
  commit, push or publication occurred. Work-continuity and remaining feature
  redesigns are subsequent stages, not completed by this shell change.

## Workspace redesign stage 1: state and recovery (2026-10-08)

- Implemented OpenSpec change `workspace-state-recovery`: application discovery,
  launch and consent now have distinct labels; an empty launcher cannot dispatch
  input. Model search/inspection, inventory, catalog and mutation failures have
  separate recovery. Read retries retain their inputs, superseded reads are
  cancelled, and checking uncertain installation state never repeats a download.
- Microphone feedback distinguishes readiness, pending permission, recording and
  transcription, with cancellation and late-track cleanup. Knowledge distinguishes
  checking, failed checks and setup without silently clearing requested retrieval.
  New feedback is supplied in all nine locales. Existing broader voice-copy and
  real speech/runtime qualification work is not marked complete by this change.
- Replaced unconditional client-local processing/storage claims with connected
  service wording and a sidebar link to endpoint details. Monochrome styling,
  typography, authorization and installed-user data are unchanged.
- Windows verification passed: API drift, TypeScript, production UI build,
  49 desktop unit tests, 6 documentation-check tests, maintained-document links,
  and strict OpenSpec validation. The existing >500 kB bundle warning remains.
  Full Go/race, Python, installed-package and other-OS suites were not rerun for
  these renderer/client changes; this is not a claim that all release CI passed.
- The full Vite fixture suite passed 162 tests (11 opt-in skips). A fresh native
  service and production renderer then passed 163 tests (10 opt-in skips) through
  `dev/scripts/test-web-workspace.mjs`, using disposable data in Windows temporary
  directory `offgrid-web-qualification-jd7geF`. English/dark and Arabic/light
  service-detail screenshots were inspected; no palette or layout redesign is
  claimed. Development-fixture failures in new tests were corrected for React
  StrictMode and keyboard activation of the styled checkbox before final runs.
- A fresh `modeltestfixtures` binary passed all 9 acquisition, restart and speech
  package tests with one worker in `offgrid-web-qualification-VJjr49`. These use
  synthetic package data and do not qualify inference. Acquisition/restart cases
  skipped by the normal suite were covered here; explicit installed-UI checks and
  the separate pre-seeded real-agent-task scenario remain unrun.
- No installed application/container was replaced, no models were downloaded,
  and no real microphone or native target was used. No commit, push, release,
  signing, hardware qualification, soak or pilot is implied. The approved broader
  shell, work-continuity and feature redesigns remain subsequent stages.

## Voice review corrections, source-only validation (2026-10-05)

- Fixed microphone ownership across delayed permission, composer changes and
  unmount; transcription retains concurrent draft edits, and Stop stays usable
  during text generation. Tests use synthetic media, not user recordings.
- Added independent ASR/TTS/declared-voice selection in shared browser-scoped
  preferences. Selection is pinned per recording/answer; removed explicit models
  cannot silently fall back. Managed Piper validates and forwards speaker IDs.
  Speak responses and its settings now sit beside the composer. Existing
  monochrome styling is preserved; the rendered desktop-width panel was inspected.
- Removed duplicate hidden acquisition previews. Review expands conditions in
  the model card; a real multi-variant choice is retained, while single variants
  need no intermediate selection screen. Hub keyword searches no longer use a
  Qwen-only shortcut. Safetensors parameter counts are not shown as byte sizes.
- Passed the full Windows Go suite, all CI-listed Linux race packages, 49 desktop
  unit tests, 19 Python client tests, TypeScript/build, API drift, maintained-doc
  checks and strict OpenSpec validation. Linux tests include the executable
  Piper speaker fixture; fixture success is not real-model quality evidence.
- The full isolated browser run passed 152 tests, with three opt-in cases skipped,
  in `offgrid-web-qualification-WxL4Hd` under Windows temporary storage. Earlier
  failures exposed four stale composer-button selectors, now fixed. A parallel
  acquisition/restart run interrupted another test's metadata request; the
  final run serialized these shared-service tests, as the CI fixture steps do.
- A final targeted voice run passed 14 tests, including removed-model refusal
  before device access and keyboard/narrow-screen checks in both themes.
  Evidence is in `offgrid-web-qualification-0QHexH`; Playwright screenshots are
  under `web/app/test-results` until the next run. These mock recognition and
  synthesis and do not establish real hardware latency or echo handling.
- The first Linux race attempt exhausted the existing `/tmp` tmpfs. Rerunning
  with an isolated `/var/tmp/offgrid-voice-race.KdorA7` build directory passed;
  no user files, installed models or running containers were deleted or changed.
- **Release gates remain:** standard container/desktop speech-runtime packs,
  installed Windows/macOS/Linux microphone and playback qualification, complete
  translated speech-control copy (currently English fallback), and real-model
  quality/latency, soak and pilot evidence. The sherpa-onnx execution adapter,
  Talk sessions and durable recording workflows remain later program work.
  Browser-local preferences are not the planned server preference API.
- This correction did not download models, record a microphone, deploy, commit,
  push or publish. Prior live-image measurements below are not requalification
  of this source build. The broader acquisition tasks remain unchecked where
  their specified recovery/platform evidence has not been established.

## Response speech correction and local deployment (2026-10-05)

- Managed speech now routes by declared architecture/capability, with leases,
  private cancellable workers, truthful unsupported-profile errors, and no hidden
  reference-voice cloning. Status polling no longer replaces loaded engines.
  Whisper/Piper execution fixtures and noisy Python import/cancellation fixtures
  test protocol behavior, not model quality. Kokoro/Zipformer still lack executable
  sherpa adapters; their installed files are not advertised as usable voices.
- Read-aloud now checks readiness, shows preparation/errors, stops immediately,
  and permits owned blob audio in both CSP policies. Chat's session-only **Speak
  responses** opt-in consumes new text incrementally, with one look-ahead request,
  a 2,400-character unsynthesized backlog limit and five-minute playback budget.
  Code/reasoning are excluded; the final fragment waits for persisted completion.
  Failure/stop/navigation discards audio without replaying restored turns. This is
  bounded sentence/chunk synthesis, not native audio-token streaming or full Talk.
- Passed: `go test ./...`; audio/server Linux race checks; UI typecheck/build and
  API drift; 49 desktop unit tests; documentation check (71 files, zero problems).
  Thirteen focused browser tests cover playback, text streaming, package lifecycle,
  early audio, final-tail delivery, errors, interruption, bounded prefetch and
  navigation. Latest isolated evidence: Windows temp
  `offgrid-web-qualification-L5qvvE`. Browser tests use synthetic audio/transport;
  real inference checks are recorded separately below.
- Authorized deployment: `offgrid-llm:voice-streaming-20261005`, image manifest
  `ea0e9dbea3606a01b8922b76b0cc1b485955760b8ae2dd36ed81de6cb7e25cf7`.
  Exactly one serving container, `offgrid`, on loopback port 11611. Previous
  container `offgrid-backup-voice-routing-20261005-215421` is stopped. Consistent
  workspace copy: `build/container-backups/voice-routing-20261005-215421/data`.
  Existing model/data volumes and Base/ASR packages were preserved. The Qwen
  transfer was explicitly settled before replacement and resumed afterward.
- After the user authorized necessary downloads, the shared model service installed
  Piper Lessac medium (63,207,027 bytes), revision
  `c10ece1aade47bb51c153c893d14e5bf8e5b7117`. Local test image includes Piper 1.8.0
  (Linux x64 wheel SHA-256
  `25b4d3f31ff70c8fa7151908e00aaa5650cbdf16bca8fcf21299f3941b89a7d3`),
  pathvalidate 3.3.1 (SHA-256
  `5263baab691f8e1af96092fa5137ee17df5bdfbd6cff1fcac4d6ef4bc2e1735f`),
  existing ONNX Runtime 1.23.2, Qwen TTS 0.1.1, ASR 0.0.6, and CPU torch 2.9.1.
  Wheels were hash-checked and installed offline while building, not during
  inference. [Piper license/runtime metadata](https://pypi.org/project/piper-tts/1.8.0/)
  and the separate voice model card remain distribution obligations; this local
  image is not a qualified/published speech pack.
- Real Piper synthesis of “Hello. This is a local speech test.” produced nonempty
  PCM WAV at 22,050 Hz in 1,738 ms and 1,293 ms (2.29/2.35 seconds of audio).
  Qwen ASR transcribed that synthetic audio back to the exact sentence in 39,061 ms
  including cold loading. No microphone or private recording was used.
- Real Phi chat and concurrent Piper synthesis succeeded: first text/audio/text
  completion at 9,590/15,715/22,724 ms on the first run and
  520/7,430/9,795 ms on a warm follow-up. Audio was available before text completion
  in both. These individual CPU observations are **not** p95 qualification and do
  not meet the full Talk five-second target. UI buffering cannot eliminate model
  loading, first-sentence generation or inference cost.
- A headless Chromium check used the deployed UI and real `/v1/audio/speech`,
  decoded/played/completed the generated Piper WAV without CSP/autoplay errors
  (5,102 ms to playback). Conversation history was an unsaved isolated fixture;
  no user conversation was read or modified. Screenshot:
  `bin/voice-repair/playback.png` (local ignored test artifact).
- Qwen CustomVoice acquisition resolved revision
  `85e237c12c027371202489a0ec509ded67b5e4b5` and retained 2,054,584,519 of
  2,498,386,873 bytes in operation `model-d1972257c19843159a4d5899636468da`.
  Subsequent resume could not reach the source; a bounded container HTTPS probe
  reported OS error 101, “Network is unreachable.” No VPN/proxy setting was changed.
  The partial package remains explicitly resumable, not installed/usable. Actual
  Qwen CustomVoice synthesis remains unverified; default Piper playback works
  without it. Existing Qwen Base and ASR packages are preserved.
- Windows desktop installation was not replaced. No commits, pushes, publication,
  microphone recording or voice cloning. Full cross-platform installed tests,
  speech-language/quality/latency qualification, complete translations, soak and
  pilot remain unrun gates. The Voice program is not complete.

## Voice program: model-package foundation evidence (2026-10-04)

The approved local Voice program is **not complete**. The first independently
reviewable OpenSpec change is `voice-model-foundation`. It provides:

- Strict schema-1, data-only manifests for Whisper, Piper, streaming Zipformer,
  and Kokoro layouts; explicit capabilities, dependencies, digests and licenses.
- Registry-owned managed package storage, bounded atomic local imports, explicit
  verification, corrupt-record visibility, and lease-protected removal.
- Permission-gated package HTTP APIs, generated TypeScript contracts, and a
  collapsed development section in Models for folder import, verification and
  confirmed deletion, with text in all nine interface locales.
- Positive chat-model admission in shared selectors and service/CLI paths. Both
  historical `chat`/`llm` types and `vlm` remain supported; speech and unknown
  explicit types are not accepted for language inference.

Validation performed on the working tree:

- Windows `go test ./...`: passed; targeted models/server/CLI/API checks rerun
  after the compatibility changes.
- WSL Go 1.26.6 race suite: sessions, RAG, server, users, cache, agents, runs,
  storage, serviceclient, models and audio passed. Windows race execution was
  unavailable without a configured C compiler; it was not represented as a pass.
- API generation/drift check, TypeScript check and production UI build passed.
  The existing bundle-size warning remains; no production performance claim.
- Isolated real-service browser suite: **133 passed, 3 opt-in tests skipped**.
  Includes actual temporary-directory package import/verify/delete and ordinary
  model download/search regressions. The initial run caught a rejected legacy
  `chat` type; the allowlist was corrected before the passing rerun.
- Desktop unit tests: **49 passed**. Documentation checker: **71 maintained files,
  zero problems**, plus its six tests. These are not installed-package tests.
- Python client unit suite: **16 passed** after repairing pre-existing test
  isolation and obsolete URL-constructor expectations. The initial test mocks
  missed the pooled opener and inadvertently contacted the running localhost
  service. The harness now routes pooled requests through its mocks and blocks
  actual socket connections. No Python production behavior was changed.

Fixtures contain synthetic model bytes, not real inference weights. Symlink
checks run on Linux; Windows skips link fixtures if local privileges prohibit
creating them. This evidence does **not** establish actual ASR/TTS inference,
speech-language accuracy, macOS installation, live microphone permissions,
Talk latency, accessibility qualification, or speech-model quality.

Remaining approved work, in delivery order (not waived):

Acquisition implementation update (2026-10-04): typed shared catalog/inventory,
four data-only resolution recipes and space/source preflight have passed targeted
Windows `go test ./internal/models ./internal/server ./pkg/api`. Resolver fixtures
cover missing dependencies, source changes, digest identity, unsafe pagination,
redirects, credentials, gated access and capability filters. This is not yet the
completed download/API/UI integration or the final integration-suite rerun.

Real metadata was resolved through the already configured Windows proxy without
changing the VPN. No model weights or executable runtimes were downloaded.
Small configuration, model-card, token and phonemizer data were fetched to check
Git object identity and derive SHA-256; manifests are in
`internal/models/catalog-packages`. These are source-identity checks, not inference.

| Recipe/source | Immutable source revision | Complete transfer bytes |
| --- | --- | ---: |
| [Whisper tiny.en](https://huggingface.co/ggerganov/whisper.cpp/tree/5359861c739e955e79d9a303bcbc70fb988958b1) | `5359861c739e955e79d9a303bcbc70fb988958b1` | 77,707,911 |
| [Piper lessac medium](https://huggingface.co/rhasspy/piper-voices/tree/c10ece1aade47bb51c153c893d14e5bf8e5b7117/en/en_US/lessac/medium) | `c10ece1aade47bb51c153c893d14e5bf8e5b7117` | 63,207,027 |
| [Streaming Zipformer English int8](https://huggingface.co/csukuangfj/sherpa-onnx-streaming-zipformer-en-2023-06-26/tree/672fbf1b30579d6585301139bb363f42a0ad4a24) | `672fbf1b30579d6585301139bb363f42a0ad4a24` | 72,654,998 |
| [Kokoro v1.0 English profile](https://huggingface.co/csukuangfj/kokoro-multi-lang-v1_0/tree/f7b96bb6bef5c5da4d3aa4f4e0498fbbf62dc78b) | `f7b96bb6bef5c5da4d3aa4f4e0498fbbf62dc78b` | 384,089,462 |

Kokoro includes 362 declared artifacts (including eSpeak data), not just weights.
Only English voice 0 is exposed by this initial recipe. Piper's voice license is
explicitly a model-card reference; the repository's MIT label is **not** projected
onto the voice/dataset. Its separate conditions must be reviewed before use.
Kokoro model assets and bundled eSpeak data have separate Apache/GPL declarations.
Redistribution review remains a release gate, as does all runtime qualification.

Adapter requirements pin upstream commits: whisper.cpp `v1.8.2`
(`4979e04f5dcaccb36057e059bbaed8a2f5288315`), Piper `v1.3.0`
(`fee9b9cefae4ebf9e196cfe994dea418f051506c`), sherpa-onnx `v1.12.14`
(`26aa2fa93210376a89de3a65a1a4dd320c37f5e9`). Public GitHub tag metadata was
checked; no executable was installed, tested, or marked compatible.

The following program stages remain required:

1. Complete shared durable model operations: curated multi-artifact downloads,
   resumable operations, runtime/offline packs, verified repair/rollback, legacy
   speech migration and dependency-aware removal. The current synchronous import
   endpoint is a foundation, not the final resumable operation contract.
2. Replace legacy global audio initialization with per-service supervised,
   cancellable adapters; accurate format encoding/decoding, resource admission,
   whisper.cpp/Piper/sherpa-onnx execution, runtime validation and packaging.
3. Shared explicit-gesture dictation/read-aloud in Chat and Agents, trusted desktop
   microphone permissions, accessible controls, preferences and CLI/SDK parity.
4. Dedicated authenticated audio WebSocket protocol, independent capture/turn/
   playback lifecycle, Talk finalization, echo handling, interruption, bounded
   queues and idempotent submissions. Never replay old audio after reconnect.
5. Existing durable agent submission/steering/approvals with concise narration;
   voice never grants access or treats recognized approval words as authorization.
6. Bounded recording uploads, restricted decoding, durable transcription jobs,
   transcript review/versioning/export, explicit Knowledge indexing, provenance,
   retention and deletion.
7. Real profile qualification on Windows, both macOS architectures, Linux desktop
   and container-backed clients; 200 English utterances, 30 workflows repeated
   three times per advertised profile, human speech-quality review, hardware and
   latency measurements, installed/offline/recovery tests, soak and pilot gates.

No speech runtime/model was downloaded, no microphone was activated, and no
commit, live replacement or publication was performed for this change. Existing
Whisper/Piper installations remain untouched. Importing a package does not yet
enable Voice, and no profile is marked runtime-compatible or qualified here.

## Stage 1: privacy and data safety

First delivery slice:

- [x] Enforce session ownership on list, read, create, delete, append, and generate.
- [x] Prevent session creation from overwriting an existing conversation.
- [x] Preserve legacy unowned sessions for local mode and authorized administrators.
- [x] Enforce RAG access for both streaming and non-streaming chat and sessions.
- [x] Disable knowledge ingestion when durable storage cannot initialize.
- [x] Make requested-but-unavailable/failed retrieval an explicit error, not
      a silent ungrounded model answer.
- [x] Add regression tests and document upgrade behavior.
- [x] Fix the queue/cache races found during validation; add focused race checks
      to CI and replace the timing-dependent priority test with an order assertion.

Durable execution and draft slice:

- [x] Replace task resubmission with run-ID-based approve/deny/cancel/resume.
- [x] Bind approvals to run ID, call ID, actor, tool, canonical arguments, and
      expiry; consume a grant atomically once.
- [x] Persist pending tool calls and execution checkpoints before side effects.
- [x] Record an uncertain outcome after an interrupted side effect; require
      reconciliation or an explicitly approved retry rather than assume success.
- [x] Reconcile orphaned running jobs to interrupted on startup, and surface
      persistence failures. Use atomic task snapshots or transactional storage.
- [x] Protect drafts from navigation, reload, failed sends, and user switching.

Exit tests: two authenticated users cannot access or mutate each other's
conversations; guest/viewer chat cannot retrieve protected knowledge; a damaged
database cannot acknowledge a successful import; approving once cannot execute
twice; denial reaches the server; restart does not leave phantom running jobs;
failed requests and navigation do not discard drafts. Ordinary local chat must
remain usable when optional knowledge storage is unavailable.

## Stage 2: complete everyday workflows

- Add stable opaque conversation IDs, separate editable titles, owner-scoped
  pagination, and an explicit migration for existing name-addressed sessions.
- Stream durable chat with one request ID, replayable progress, cancellation,
  and an atomic completed turn. Define partial-response retention explicitly.
- Move job state out of individual React pages. A reconnecting client should
  reconstruct current work from the server instead of restarting it.
- Add document collections, folder import, extraction preview, incremental
  reindexing, and clear scanned-PDF/OCR capability messaging.
- Define collection ownership and sharing. Enforce access when listing sources,
  retrieving passages, ingesting, and deleting; a client-side filter is not an
  authorization boundary. Today the knowledge index is shared by authorized RAG
  users, not isolated per user or collection.
- Persist retrieval status and structured citation locators with each answer;
  let users open the exact source passage and export a sourced answer.
- Distinguish retrieval failure, no matching evidence, and successful retrieval.
  Add source-only answering and document scope; do not present rank scores as
  probabilities of factual correctness.
- Maintain the monochrome design system, keyboard navigation, focus management,
  accessible status messages, responsive layout, and shared desktop/web behavior.

Exit tests: a new user imports documents, gets a cited answer, verifies a passage,
restarts the app, and continues without terminal troubleshooting; long-running
chat and agent jobs remain understandable while navigating between pages.

## Stage 3: measured quality and performance

- Establish representative CPU-only, low-memory, NVIDIA GPU, and Apple Silicon
  profiles. Report actual GPU offload and allocated context, not only settings.
- Benchmark cold load, first token, prompt processing, generation, peak memory,
  queue wait, cancellation, and retrieval at increasing collection sizes.
- Consolidate scheduling around one bounded, cancellable inference admission
  service with fairness between interactive chat, agents, and background work.
- Paginate session and run history. Avoid replaying the entire event log for
  every activity request; use indexed summaries and bounded event queries.
- Optimize measured retrieval bottlenecks: compact vectors, filtering before
  ranking, bounded candidate selection, and an index only where benchmarks justify it.
- Version retrieval evaluation fixtures and test citation support and abstention,
  not just successful HTTP responses. Review multilingual fixtures with speakers
  of the target languages; interface translation is not evidence of model quality.
- Extract conversation, knowledge, model, and run application services from
  large transport/CLI files. Keep generated API types aligned with behavior.

Exit criteria: publish reproducible baselines, document tested hardware limits,
and gate regressions against those baselines. Set numeric budgets from measurements
rather than promising hardware-independent latency or quality.

## Stage 4: dependable distribution and focused expansion

- Manage supported Hermes/OpenClaw versions, compatibility probes, staged installs,
  repair/uninstall, and rollback. Separate installed/configured/connected/task-tested.
- Test actual tool calls and an artifact-producing task with each supported agent.
  External runtimes retain their own tool permissions; an inference adapter does
  not place their filesystem/network actions under OffGrid's approval broker.
- Add packaged-app launch, inference, graceful shutdown, update, and data-migration
  smoke tests on Windows, Linux, and macOS; add race and vulnerability checks to CI.
- Design backups and restore verification before automated upgrades. Add desktop
  signing/notarization and a tested rollback path before advertising seamless updates.
- Extend the existing doctor command with actionable, redacted diagnostics for
  runtime version, storage, model integrity, GPU placement, context, and integrations.
- Generate a tested capability matrix and stable installation guides from release
  metadata; clearly separate stable images from development/edge instructions.
- Pilot offline packs and trusted LAN model distribution, with signatures,
  checksums, explicit trust, license metadata, and resumable transfer.
- Keep unrestricted computer use, distributed inference, and fine-tuning out of
  the supported core until they have owners, threat models, and platform tests.

Exit criteria: install and upgrade from a clean supported machine, restore a
backup, complete a useful task, and diagnose a missing optional dependency without
special knowledge of the repository.

## Tracking and release evidence

Record completed slices below with commands run and limitations. Do not mark a
stage complete because its unit tests pass while its user workflow remains absent.

### 2026-09-16: Stage 1 first slice implemented locally

Implemented owner-scoped session operations, non-overwriting creation, shared
chat authorization/retrieval policy, and explicit unavailable knowledge storage.
Legacy data is preserved; no ownership migration runs automatically. Also fixed
the queue-size error-path race, synchronized cache enable/disable and cleanup
start/stop, and made cleanup shutdown wait for its worker to exit.

Validation passed:

- Windows: `go test ./...`.
- WSL/Linux: `go test -race ./internal/sessions ./internal/rag ./internal/server
  ./internal/users ./internal/cache`.
- Web: `npm run api:generate`, `npm run api:check`, `npm run check`, and
  `npm run build`.
- CI syntax: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
  -shellcheck= .github/workflows/ci.yml`.
- `git diff --check`.

The tests exercise cross-user requests, forged ownership, legacy access,
concurrent creates/appends, broken database/directory initialization, retrieval
failure/no-evidence behavior, and concurrent cache lifecycle. They do not prove
model answer quality, OS packaging, or real Hermes/OpenClaw tool execution.

At this checkpoint, durable approval/resume and drafts were still pending.
The next entry records their implementation; collection-level authorization and
end-to-end citations remain later-stage work.

### 2026-09-16: durable execution and draft recovery implemented

Built-in agent runs now use persisted native-tool checkpoints with actor-bound,
single-invocation expiring approvals. Every model-requested tool in a batch is
processed, not only the first. A checkpoint write must succeed before execution.
Startup never replays a side effect; interrupted calls become uncertain and
require a human-verified reconciliation. Concurrent approval attempts are
serialized; models release inference admission while waiting for human input.

Web and CLI controls inspect, approve, deny, cancel, resume, and reconcile the
same run. Returning to the Agents page restores the selected run from the server.
Legacy history is preserved but has no invented resumable checkpoint. Run
snapshots remain authoritative if the event-log projection fails. Storage
failures are explicit. CLI requests use the configured API key and render the
new committed-step/final-result stream.

Chat/task drafts are scoped to account and conversation in browser storage,
saved during editing, retained on failed sends, and protected from late response
clears. Storage failure keeps an in-memory draft with a warning. Identity must
resolve before rendering account-scoped state. Browser drafts are not encrypted
or synchronized across devices.

Validation passed before the container smoke test:

- Full Windows `go test ./...`, then affected CLI/agent/server packages again.
- WSL `go test -race ./internal/agents ./internal/runs ./internal/sessions
  ./internal/rag ./internal/server ./internal/users ./internal/cache`.
- OpenAPI generation/check, TypeScript check, production UI build.
- 16 Edge browser tests for workspace, functional pages, and recovery, including
  failed storage, failed sends, late responses, user switching, and approval/deny
  after navigation/reload. Model/tool interactions use controlled fixtures.
- CI actionlint, Electron main/preload JavaScript syntax, and whitespace checks.

Stage 1's listed implementation is complete; this is not a declaration of
production readiness for the whole roadmap. Stage 2 remains: opaque conversation
IDs/titles and pagination, durable streaming chat, collection isolation, and
stored source citations. Stage 3 needs measured hardware/quality baselines;
Stage 4 needs supported-platform release and real external-agent task evidence.
No commit, push, registry publication, or release is part of this local slice.

Local deployment verification:

- Built `offgrid-llm:reliability-20260916` from the working tree with runtime
  version `workspace-20260916-reliability` (not a published release tag).
- All 20 browser tests passed against an isolated image instance; real session
  create/reload/delete and navigation exercised the actual service, while model
  and approval scenarios used API fixtures. The isolated instance was removed.
- Replaced the local `offgrid` container on loopback port 11611, preserving
  `offgrid-models`, `offgrid-data`, and `OFFGRID_MAX_CONTEXT=65536`.
- Preserved a stopped rollback container `offgrid-pre-reliability-20260916` and
  a data snapshot at `D:\offgrid-backups\pre-reliability-20260916` before startup
  recovery changed old task statuses. Never run both containers against the data
  volume concurrently. The rollback container alone is not a data snapshot.
- Replacement is healthy and serves the new UI bundle. Seven navigation and
  recovery browser tests also passed against the replacement's built UI.
- This remains the existing CPU-only container configuration; no GPU backend,
  platform packaging, or new release publication is implied by this rebuild.
- A real `/v1/chat/completions` request to installed Phi 3.5 returned
  `OFFGRID_READY` (HTTP 200). Observed cold model load was 98.3 seconds and total
  request time 108.1 seconds with the preserved 65K context; a single sample is
  not a benchmark or a quality evaluation. This confirms that startup/memory
  profiling and GPU validation remain important Stage 3 work.

### 2026-09-16: responsive saved chat and context profiles implemented

Saved chat streams by default in the shared web/desktop renderer. Queue,
retrieval, loading, prompt processing, generation, and persistence are visible.
Stop cancels queue waits and inference. A completed turn is saved atomically
before the final event; transport failure or partial output is never reported
as a saved answer. Drafts and visibly provisional text survive failures.
The client batches rendering and follows output only while near the bottom.

Interactive chat defaults to an 8K context (bounded by the service limit),
while Extended and external-agent requests retain the configured service
context. Model/context switches wait for active leases. The runtime does not
silently truncate history, increase model residency using GGUF size alone,
or restart a cached process independently of inference admission. Backend
processes retain their environment, including CUDA loader/device settings.

The stream reports queue/load/first-text/generation timings and actual backend
token usage when available. Output budgets and context preferences persist per
account. The API contract and performance guide describe these semantics without
hardware-independent speed promises.

Validation passed:

- Full Windows `CGO_ENABLED=0 go test ./...`, then affected packages again.
- WSL `go test -race ./internal/server ./internal/inference ./internal/sessions`.
- OpenAPI generation, TypeScript check, and production UI build.
- All 23 Edge browser tests, both against the development build and the rebuilt
  image's packaged UI. Streaming tests cover incremental text, split UTF-8,
  premature EOF, cancellation, draft recovery, and saved completion. Model
  responses use controlled fixtures, not a hardware benchmark.
- Built `offgrid-llm:streaming-20260916` and confirmed its runtime version,
  health endpoint, and new UI asset hashes in an isolated temporary container.
  The temporary instance had no access to the user's model/data volumes and
  was stopped and automatically removed after testing.
- NVIDIA passthrough probe detected the RTX 5060 Laptop GPU with 8151 MiB VRAM.
  A device probe is not proof of GPU model inference.

Remaining limits: no replayable chat request ID or reconnect/resume protocol
yet. A disconnect during final persistence has an uncertain client outcome;
reload before retrying. Full hardware memory admission, fairness, and repeated
representative hardware benchmarks remain unverified. The source CUDA build
was interrupted by a large image-layer download error; local GPU validation
used the alternative runtime assembly described below.

Deployment checkpoint: the new CPU image is built, but replacing the running
`offgrid` container was blocked by the execution environment's policy. The
existing `offgrid-llm:reliability-20260916` service and its named volumes remain
unchanged. No registry publication, commit, push, or release was performed.

GPU validation follow-up:

- The released CUDA runtime failed to start its inference binary because
  `libgomp.so.1` was absent. Added `libgomp1` to `docker/Dockerfile.gpu` and a
  final-image linked-library check. Only `libcuda.so.1`, supplied by the NVIDIA
  container runtime, is permitted to be absent at build time.
- Built local `offgrid-llm:streaming-gpu-20260916` using the fresh CPU image's
  pure-Go binary and UI, plus the released CUDA runtime at immutable digest
  `sha256:b6290a2d4059859d3801cc4fbdb08e7030c0997971380fa648ab29f133f298b5`
  and the missing dependency. Verified its llama.cpp revision `e9fa078` matches
  the current source Dockerfiles. This local validation assembly is not a new
  published release or a successful from-source CUDA rebuild.
- In an isolated, auto-removed container with read-only models, separate
  temporary data, and a 1536 MiB RAM limit, actual Phi 3.5 saved-chat requests
  streamed text, reported a token-limit finish, committed the turn, and returned
  the same saved text on reload. NVIDIA VRAM usage reached roughly 3334 MiB.
- At 8192 context and a 32-token output limit, one cold request showed 8.41s
  model loading, 8.56s client-observed first text, and 9.12s through saved
  completion. A warm repeat showed 0.24s first text and 0.71s through completion.
  Backend-usage-derived decoding rates were about 63 and 69 tokens/s. These
  short identical-prompt samples are not representative benchmarks or a
  like-for-like comparison with the earlier CPU/65K test. The model returned
  the requested marker but continued with unwanted text; instruction-following
  quality is not established. Actual 65K GPU/agent inference remains untested.
- The temporary GPU container was stopped and automatically removed, including
  its disposable test conversations. The normal container on port 11611 remains
  unchanged because the replacement operation was blocked.

### 2026-09-16: production-readiness correctness foundation

The approved five-milestone [delivery contract](production-readiness.md) now
tracks the next stage without treating historical unit-test passes as production
qualification. This checkpoint does not complete Milestone 1 or the full plan.

Implemented:

- Shared authenticated CLI service transport with bounded responses, deadlines,
  cancellation, redirect refusal, and redacted service errors. Knowledge commands
  and agent lifecycle controls use it and return explicit process exit codes and
  JSON errors. Directory imports stream files and stop on failure; clear requires
  explicit confirmation and retains partial-operation counts on failure.
- Exact public `/api/v2/system` identity route, generated client contract, and
  settings display. Electron checks product/API/version/capabilities/UI identity
  before attachment; CRLF/LF checkout differences do not create false mismatches.
  Navigation and IPC require the exact origin and trusted main frame. External
  services are never killed or presented as desktop-owned storage.
- Completion-reason checks in durable and structured agents. Truncated/filtered/
  missing terminal reasons cannot execute tools or produce completed tasks.
  Partial text stays explicitly incomplete, outside completed model context.
- Pinned `modernc.org/sqlite v1.48.2` (SQLite 3.51.3) and matching libc dependency.
  The shared opener checks the WAL fix and applies foreign keys, FULL durability
  and a bounded busy timeout to every connection. Tests force four separate
  connections, check orphan rejection/cascades and reopen integrity, and cover
  Unicode/escaped filesystem paths. Existing RAG uses this opener.
- Composition-safe Enter handling and resilient browser preferences. Storage
  denial leaves the workspace usable without pretending drafts were persisted.
- Native Windows/macOS Intel/Apple Silicon CI contract jobs and expanded Linux
  race checks. Newly added CI jobs have not been observed running remotely yet.

Validation passed locally:

- Windows: `go test ./...`.
- WSL/Linux: `go test -race ./internal/storage ./internal/serviceclient
  ./internal/agents ./internal/rag ./internal/server ./cmd/offgrid`.
- `go vet ./internal/serviceclient ./internal/storage ./internal/agents
  ./internal/server ./cmd/offgrid`.
- Web: `npm run api:check`, TypeScript checking, production UI build; 21 Edge
  browser tests across workspace experience, reliability, streaming, and functional
  pages with controlled API/model fixtures. These are browser behavior tests, not
  real model task-quality results or full service integration tests.
- Desktop: syntax checks and six Node handshake/IPC/URL/build-identity/loading
  recovery tests. Retry opens the compatible workspace instead of reloading the
  loading page indefinitely, and startup text no longer invents model-load progress.
- Actionlint v1.7.12 for the changed CI workflow; `git diff --check`.
- WSL `govulncheck ./...`: no reachable vulnerable symbols reported. It also
  reported one vulnerability in imported packages and 21 in required modules
  without detected calls; these still require dependency triage. This result
  is not a clean bill of health or independent security review.

Limits and outstanding checks:

- The temporary-service launch for the full browser integration suite was blocked
  by execution policy. The existing service/container and user data were untouched.
  No container rebuild, installation replacement, commit, push or release occurred.
- `go mod tidy` was attempted but could not fetch the pre-existing optional
  `go-skynet/go-llama.cpp` module (proxy EOF). The SQLite dependency update succeeded;
  ordinary builds/tests passed. A successful tidy is not claimed.
- No workspace database cutover, generated conversation IDs, durable v2 chat/jobs,
  SSE replay, or whole-workspace backup/restore exists yet. Existing prompt-derived
  session filenames still need migration; punctuation/Unicode/duplicate-title
  acceptance is not passed. Do not substitute another sanitization patch.
- Not all CLI commands use the new transport/error contract. Shared collections,
  citation persistence, artifact verification, fairness and full localization
  remain pending. Structured-agent completion safety is not proof of task quality.
- No installed-Electron, Mac hardware, offline-pack/update/signing, quantitative
  agent/RAG, soak or pilot qualification is claimed.

Next delivery slice: exclusive workspace ownership, staged legacy migration with
generated IDs and recovery manifest, then authoritative conversation/job services
and a coordinated CLI/web/desktop cutover. Preserve old data and test malformed and
unowned fixtures before any activation. Do not dual-write.

### 2026-09-16: ownership, recovery, admission, and approved local deployment

This later checkpoint supersedes the earlier notes that ownership, offline
backup/restore, and container rebuilding were not yet implemented. The full
production-readiness plan is still incomplete; no milestone is certified.

Implemented and tested:

- Process-scoped Windows/Linux/macOS workspace ownership, including rejection of
  simultaneous service/maintenance access and lock release after a process crash.
  Startup validation fails closed. Shutdown drains HTTP/agent workers, persists
  interrupted work and closes storage before releasing ownership; a failed drain
  retains the lock until process exit.
- Offline `workspace backup`, `verify`, and `restore` commands. Whole stopped data
  directories are inventoried and hashed; unsafe paths, symlinks, collisions,
  tampering, existing targets, mismatched application versions and damaged SQLite
  state are rejected. Restore stages privately and recovers/checks SQLite WAL and
  foreign keys before non-overwriting activation. These primitives do not yet
  implement application/data revision manifests, migration or the recovery UI.
- Nonblocking persistent agent-event publication; slow subscribers disconnect for
  replay instead of stalling execution. Invalid sequences, duplicate event IDs and
  uncertain writes fail closed. Transactional v2 events/compaction remain pending.
- Bounded FIFO inference admission with cancellation and explicit queue-full
  responses. Model switches cannot be starved by new active-model requests.
  The default is one active generation and 32 queued requests. This is not yet
  per-actor/workload scheduling or coordinated indexing admission.
- Saved-conversation `session` and `export-session` commands use the shared
  authenticated API with machine-readable errors and no local fallback. Exports
  do not overwrite files. Interactive chat saving and other CLI commands remain
  outside this completed slice; conversation identity still needs migration.
- Container builds inject source revision in `/api/v2/system` even when `.git`
  is excluded from the build context.

Additional validation:

- Full Windows `go test ./...`; WSL race tests for storage, inference, runs,
  agents, server and CLI; `go vet` for those components and the shared client.
- Darwin Intel/ARM64 storage-test cross-compilation (not execution on Macs).
- UI contract/TypeScript/build checks, six desktop Node tests, and 25 Edge browser
  tests against an isolated container. Some browser scenarios use controlled API
  fixtures; these do not establish model quality or installed-Electron behavior.
- Isolated final-image startup, backend/renderer identity, JSON session listing,
  GPU visibility, exclusive service/backup rejection, and offline backup/verify/
  restore smoke checks. Temporary verification containers were removed afterward;
  no user volumes were removed.

Deployment explicitly approved by the user:

- Built CPU image `offgrid-llm:readiness-20260916` and local GPU refresh
  `offgrid-llm:readiness-gpu-20260916`, version `0.4.3-readiness-dev`, source revision
  `1b3334ff39cb204313d21dca75bc2792ea7f4989-dirty`.
- Final GPU image ID:
  `sha256:47fcc8590d0898992fe26a9c7de8c9b5524261c4250d44d970752b55dbbc47d0`.
  It contains the newly built Go application/UI and reuses the existing local
  CUDA/llama runtime. It is **not** a clean from-source GPU release build. A separate
  full GPU build first encountered Docker Hub credential rejection, then was
  intentionally cancelled during the large anonymous CUDA toolchain download.
  Saved Docker credentials were not modified, and publishing was not attempted.
- Stopped `offgrid`, archived the complete `offgrid-data` volume, compared the
  archive against the stopped source and verified SHA-256, then replaced the
  container on `127.0.0.1:11611`. The existing model volume and runtime settings
  were retained, including GPU access and 65,536 configured context.
- The original container remains stopped as `offgrid-rollback-20260916-readiness`,
  with automatic restart disabled so it cannot become a second writer. Its image
  and a private configuration snapshot are retained. The backup location is in
  the private local deployment record, not a portable public recovery package.
  Do not downgrade against newer data without checking compatibility/restoring
  the matched snapshot.
- New `/health`, `/api/v2/system`, UI content hash and authenticated-client session
  listing passed on the active instance. NVIDIA RTX 5060 Laptop GPU was visible.
- Two real, unsaved streaming Phi 3.5 requests at 65,536 context and a 16-token
  output limit returned the requested `OFFGRID_READY` marker. Cold first text:
  29.77 s, total 30.63 s. Warm first text: 1.19 s, total 2.19 s. Both ended with
  `length` and extra newlines; they prove working streaming, not correct natural
  completion or agent-task readiness. GPU memory afterward was about 5902 MiB.
  These two samples are not a qualified performance benchmark.

Still required: transactional workspace migration and generated IDs, v2 durable
chat/jobs/collections, shared client recovery state, permission-scoped knowledge
and citations, full CLI conversion/localization, matched signed installation and
update/offline packs, independent security review, platform/hardware qualification,
quantitative RAG/agent evaluation, soak and pilot evidence. No commit, push, tag or
release was made in this checkpoint.

### 2026-09-16: live agent response previews and runtime progress

Implemented [live agent progress](agent-live-progress.md) across the shared
web/Electron renderer and interactive CLI streams:

- Durable provisional response previews, explicit queue/loading/prompt/generation/
  tool/approval phases, model-turn numbers, elapsed time and last-progress age.
  The preview is bounded at 64 KiB; per-token writes are coalesced. It is never
  treated as completed output or complete conversation context.
- Structured model streaming preserves native tool-call fragments and terminal
  reasons privately. Reasoning fields and provisional arguments are excluded
  from previews. Truncation cannot execute tools or mark a run completed.
- Owner-scoped GET snapshot streaming, heartbeats, bounded viewer writes and
  reconnect-to-latest-snapshot behavior. Disconnect/navigation never cancels or
  resubmits the job. The UI detects actions on paused runs from another client.
  This is a v1 snapshot contract, not the planned v2 durable event-replay API.
- A default-on live-preview checkbox; disabling display does not stop execution.
  Terminal partial output is explicitly incomplete. Monochrome presentation and
  labels for all nine interface languages; new translations need speaker review.
- CLI phases/preview text go to stderr, final results to stdout. Snapshot updates
  do not duplicate printed fragments. CLI automatic reconnect and full localization
  are not implemented by this slice.

Validation:

- Windows `go test ./...`; Linux race tests and `go vet` for agents/server/CLI.
- Added tests for provisional persistence/restart, Unicode preview bounds,
  cancellation winning over later writes, missing/truncated terminal responses,
  structured tool assembly, reasoning-field exclusion, response limits, SSE owner
  isolation and disconnect/reconnect without cancelling execution.
- Contract generation/drift checks and production UI build passed. All 27 Edge
  browser tests passed against an isolated rebuilt container, including preview
  before completion, reconnection without resubmission, navigation recovery,
  hiding the preview without cancellation, and explicit incomplete cancellation.
  Controlled API fixtures remain distinct from real-model qualification.
- Six desktop compatibility/security Node tests passed. No new installed-Electron
  packaging or native macOS qualification is claimed.
- User explicitly approved another backup/replacement. The complete stopped data
  volume was archived, compared and SHA-256 verified before activation. Models,
  environment, GPU access and security settings were retained. The previous
  container is stopped as `offgrid-rollback-20260916-agent-live` with restart
  disabled. Temporary verification containers were removed without deleting any
  user volumes.
- Active development version: `0.4.3-agent-live-dev`; GPU image ID
  `sha256:6b7d915caa339b8f617f58a2f42a5b12a83dcb86e09453c18f11a730ac8ea819`;
  renderer identity
  `9bf2d3255fef761f5f4739260004415131dc7307c515c6ab3899d359c2762125`.
  The Go application/UI are newly built; the existing CUDA/llama runtime is reused
  locally, not represented as a fresh release-runtime qualification.
- A clearly labelled, harmless live-agent smoke task on the active Phi 3.5 model
  produced successive 1-, 11-, and 32-character previews while still running.
  Observed loading, processing and generating phases. Disconnecting the viewer
  left the same run active; explicit cancellation returned `cancelled`. Its
  cancelled test entry is retained in history for inspection. This proves the
  actual progress path, not general task quality or external-agent integration.

The broader production-readiness milestones remain incomplete. No commit, push,
tag or release was performed.

### 2026-09-16 — Stable agent task/output layout

- Separated task-form sizing from the growing results column. Run task stays
  directly below its input; content-width container queries stack narrow panes.
- Added one bounded, labelled, keyboard-scrollable output region. Status, Cancel
  and preview controls stay outside it. Long model names, paths, previews and
  step histories cannot widen the workspace.
- Live output follows within its pane only while the reader is at the bottom.
  Reading earlier text preserves scroll position and focus. Size observation
  covers new steps, wrapping and connection-notice changes; approval/reconciliation
  transitions return to the decision rather than prior output.
- Production renderer build, TypeScript and API drift checks passed. All 35 Edge
  tests passed against the final isolated image (two workers), including eight
  new layout cases covering 390/768/1024/1440px, 100-step histories, long Unicode
  output, live read-back, approval transitions, German and Arabic/RTL. Dark and
  light captures were inspected. An earlier run exposed a scroll sizing defect
  that was fixed; overloaded parallel runs also timed out and were not counted
  as passes. Six desktop Node compatibility/security tests passed; this is shared
  renderer coverage, not a new installed-Electron qualification.
- With explicit user approval, backed up the complete stopped data volume,
  compared the archive with its source and verified SHA-256 before replacement.
  Backup: `/home/phil/offgrid-backups/agent-layout-20260916/deploy.YzCTrWYC`.
  Previous container: `offgrid-rollback-20260916-agent-layout` (stopped, restart
  disabled). Models and original environment/security/GPU settings retained.
- Active local version: `0.4.3-agent-layout-dev`; GPU image
  `sha256:afecbfe4bef77deee60c61a32bcd29f1e8e4faca56e9c25ae3c4c29cb6dc1b16`;
  UI identity `35ffc20c49e28ae947d9376c21ceafd12c1c4c671496d659bcc9ebab6cc8b0f2`.
  Health, CLI session JSON, UI identity and NVIDIA visibility checks passed.
  This local rebuild reuses the existing CUDA/llama runtime; no new runtime,
  model-quality, native-platform, or production-release qualification is claimed.

No commits, pushes, tags or releases were made for this layout change.

### 2026-09-16 — Safe chat and agent history controls

- Made conversation deletion visible rather than hover-only. Added confirmation,
  title search, refresh, filtered bulk deletion and honest partial-failure retry.
  The confirmation captures a fixed set of records; successful deletions are not
  retried, and other conversations' drafts are preserved.
- Agent history now supports search, Show more beyond twenty entries, prompt reuse
  without replacing a draft, and copying results. Finished owned runs can be
  deleted individually or together. Pending/active work, unsettled workers,
  approvals and unresolved tool outcomes cannot be deleted. Busy conversations
  reject deletion immediately instead of deleting a newly completed answer.
- Persisted scrubbed tombstones prevent retained agent events from recreating
  deleted history after restart. History deletion is explicitly not secure
  erasure: tool-created files, separate event/audit logs, artifacts and backups
  remain. See [history management](../guides/history-management.md).
- Added regression tests for cross-owner denial, protected states, storage
  failure, restart, Activity projections, Unicode, selected-item removal,
  filtering, partial retry, draft preservation and mobile confirmation. Windows
  `go test ./...`, Linux race tests and vet for agents/server/sessions, TypeScript,
  API generation/drift checks and the production UI build passed. All 39 Edge
  browser tests passed against isolated data. Six desktop compatibility/security
  Node tests passed; installed Electron/native macOS qualification is not claimed.
  The nine language catalogs contain the new controls; speaker review is pending.
- With explicit user approval, backed up the complete stopped `offgrid-data`
  volume, compared its archive against the source and verified SHA-256 before
  replacement. Backup:
  `/home/phil/offgrid-backups/history-20260916/deploy.3zVFfxP6`.
  Prior container: `offgrid-rollback-20260916-history` (stopped, restart disabled).
  Models and original environment/security/GPU settings were retained. No user
  conversations or tasks were deleted during testing or deployment.
- Active local version: `0.4.3-history-dev`; GPU image
  `sha256:a7ee8f52f76ce59a700bf298f15523e345d1ae0e14c14ae4b75c3ebaa33503a0`;
  UI identity `e34308498f21bb3b0b981e9a0e8c382ae1f5fa7db936a9e8c567bbde35f776cb`.
  Health, UI identity, CLI session JSON and NVIDIA visibility checks passed. A
  read-only browser check verified chat/agent controls on port 11611 with zero
  page errors or write requests. The Go application/UI are rebuilt; the unchanged
  CUDA/llama runtime is reused locally, not newly release-qualified.

No commits, pushes, tags or releases were made. Broader production-readiness
milestones remain open; these checks do not certify the whole product.

### 2026-09-16 — Requested local commit checkpoint

After the verified history deployment, the user requested local commits. The
pending changes were grouped by storage/recovery, agent runtime, server contracts,
CLI behavior, desktop compatibility, shared UI, and build/evidence updates. The
earlier no-commit notes describe those earlier deployment checkpoints. No push,
tag, release publication or additional live-data mutation accompanies this
checkpoint. The running development image contains the tested source from before
these commits; its recorded dirty revision is retained honestly.

### 2026-09-17 — Desktop startup and installer recovery

- Confirmed the reported Windows attachment failure: installed desktop `0.4.4`
  was connecting to the still-running `0.4.3-history-dev` service. Retained the
  correct product/API/version/UI identity checks; removed the blocking startup
  dialog and duplicate readiness loops instead of bypassing compatibility.
- All desktop editions now share one bounded, cancellable lifecycle controller.
  The monochrome startup window renders before service discovery. Recovery offers
  retry, opening an identified external web workspace, or an explicitly separate
  local workspace with distinct data/models and another loopback port. Native
  menus let users change the remembered next-launch choice without interrupting
  work. The desktop never stops/replaces an externally managed service.
- Window geometry uses debounced asynchronous writes. Windows child launches do
  not flash a console. Missing binaries, hung ports and child exits are visible
  recovery states. IPC controls remain restricted to the trusted startup main
  frame; renderer navigation cannot start services. Keyboard focus, dark mode,
  reduced motion and minimum-window overflow were checked.
- Windows Setup/portable packages build successfully using monochrome NSIS
  branding and Segoe UI; native scope/location controls remain. The redundant
  MIT acceptance page is removed, with the license retained in resources. This
  is not a measured decompression or installation-speed improvement.
- Fifteen Node compatibility/lifecycle/security tests pass on native Windows and
  WSL Linux. UI type checks/build and workflow lint pass. A real, unpacked Windows
  Electron package passed mismatch recovery, separate bundled-runtime startup,
  persisted relaunch, native next-launch selection, matching external attachment,
  hung-port timeout, keyboard retry and startup-only IPC denial. Local warm-fixture
  recovery was visible after 236 ms; this is not a cross-hardware latency promise.
  Evidence: `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-startup-0j5pzG`.
  An initial hidden-window screenshot attempt hit a transient compositor error;
  bounded capture retries fixed the test harness, not the application behavior.
- Added the real packaged-app smoke to CI for Linux, Windows, macOS Intel and
  Apple Silicon. The new remote jobs have not run in this change. Native Mac and
  packaged Linux results remain pending; Linux Node tests are not Mac validation.
- The actual generated Windows installer reports `NotSigned`. SmartScreen cannot
  be removed by changing the UI, and signing does not guarantee immediate
  reputation. Windows verified signing/Store distribution and macOS Developer ID
  signing/notarization remain external release requirements. Do not describe
  these preview artifacts as trusted, notarized or production-qualified.

The existing installed desktop and live container/data were preserved. Local
preview artifacts are under `build/desktop-startup-preview`; no installer was
applied to the user's machine, and no push, tag, release or container replacement
was performed. Installed upgrade/uninstall flows, speaker review of native shell
translations (currently English), signing and full performance qualification
remain outstanding. See [desktop startup](../setup/desktop-startup.md).
Final read-only environment checks also found Ubuntu/WSL stopped between commands:
the old service answered inside WSL after startup, while Windows localhost probes
failed. This external environment lifecycle issue is separate from the isolated
native-app tests; Windows access to the user's container is not reported as passed.

### 2026-09-17 — Installation qualification and patch preparation

- After the user restored WSL, Windows reached the existing service at port 11611
  again. It still identifies as `0.4.3-history-dev`; no container was replaced.
- Found an additional release-only mismatch: Go binaries embedded `v0.4.4`, while
  Electron metadata used `0.4.4`. Canonicalized numeric release-tag prefixes in
  the CLI/service identity, retaining labels such as `validation` unchanged.
  Added Go regression cases and made native CI use the actual tag-form ldflag.
  Genuine version, contract and UI mismatches are still rejected.
- Full Windows Go tests, Windows/Linux CLI version tests, 15 desktop Node tests
  on both hosts, type/contract checks and workflow lint passed. Real packaged
  Windows startup was rechecked; timings varied with load and are not an SLA.
- Built a separate `OffGrid Desktop Install Test` NSIS identity, with its own
  registry GUID and temporary directory. Clean install, installed Electron/Go
  startup, same-version reinstall, uninstall, and SHA-256 preservation of a
  workspace fixture passed. The test uninstaller removed only its test application
  and registry entries. Evidence: `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-startup-6nNDdw`.
  The production app, shortcuts, container, models and conversations were untouched.
- The installer smoke is now also in Windows CI. Linux CI configures its isolated
  unpacked sandbox helper without disabling Chromium sandboxing. Mac and remote
  packaged-platform results remain pending until those jobs run. Interactive
  installer accessibility, elevated installs, historical-version upgrades and
  signing/notarization still require qualification.

The user requested commits and a patch release after these checks. Prepare a new
0.4.5 version rather than overwrite published 0.4.4 artifacts. A prepared version
or pushed commit is not proof that CI or publication completed.

### 2026-09-17 — Installer handoff, Windows downloads, and model discovery

- Fixed the Hugging Face download path promoting its `.tmp` while its own file
  handle remained open. Flush/close now precede promotion; Windows sharing locks
  receive bounded, cancellable retries and preserve partial bytes on failure.
  Native Windows tests reproduce an actual sharing violation, release it, and
  exercise persistent locks, cancellation, resumed transfers and HTTP 416 recovery.
- Web/desktop distinguish transfer from finalization; failed/cancelled partial
  downloads offer Resume. Cancellation uses exact filenames, not substring
  matching. Model discovery refresh happens before completion is published.
- Restored model search to the shared Models page. It makes one explicit public
  Hugging Face search, then loads files only for a selected repository. Choices
  include large quantizations without a catalog/RAM cap. Repository/file-derived
  IDs isolate identically named files. Split weights/projectors are labelled as
  requiring companion files, not advertised as standalone models.
- CLI search remains available, adds `--files` and file metadata in JSON, validates
  arguments with exit code 2, propagates cancellation, and bounds file-list
  concurrency/deadlines. Live upstream TinyLlama search returned twelve GGUF
  variants without downloading model weights. Unknown upstream failures now
  surface as errors instead of an apparently successful empty search.
- Windows setup uses system DPI awareness, 4× monochrome artwork, Segoe UI 9,
  a monochrome progress bar and concise completion text. Show details previously
  opened an empty log because the template disabled interactive detail output;
  it now displays real extraction/copy/registration messages and failure guidance.
- Finish records launch intent, closes the wizard, then starts the app. Running
  app detection no longer repeatedly spawns PowerShell or force-kills processes.
  Silent reinstall refuses active applications; interactive reinstall requires
  consent and requests a normal quit. Legacy apps offer a manual-quit retry.
- The isolated native Windows installer test passed clean install, real installed
  Electron/Go startup, both Finish checkbox states, populated details, silent
  running-app refusal, consent-driven same-version reinstall, uninstall, and
  SHA-256 fixture preservation. Final wizard-close measurements were 73 and 119 ms;
  these are warm local observations, not a cross-hardware SLA. Evidence is under
  `%TEMP%/offgrid-install-74663d8e08c9455390cf642d5aad768d` and
  `%TEMP%/offgrid-desktop-startup-mnI3xj`. The final installed-app smoke also
  passed with profile/port environment overrides removed and only test-package
  launch arguments supplied. A test-driver race reading a destroyed page's empty
  button text was fixed by ignoring that transition while retaining hard deadlines.
- 41 browser tests passed against disposable real services on native Windows/Edge
  and WSL/Linux/Chromium. They cover navigation/history, streaming, recovery,
  keyboard/IME, locales, responsive layouts and model search/download fixtures.
  Added a cross-platform fixture wrapper so these tests do not touch a developer's
  normal service. Fixed Vite's missing `/api` proxy. Go model/server/CLI tests passed
  on Windows and with the race detector on WSL; 18 desktop Node checks passed on
  both hosts. Contract generation, UI build and workflow lint pass. Native CLI
  subprocess validation confirmed JSON `invalid_usage` with exit code 2.
  These checks do not certify model inference quality or throughput.

Test packages, data and registration are isolated from the installed product.
Test-app arguments preserve that isolation if an elevated installer launches via
Explorer without inheriting its environment; release packages ignore those test
arguments. Native Windows test capture uses Windows PowerShell, not a new runtime
dependency for end users. The test launcher clears `ELECTRON_RUN_AS_NODE`, which
otherwise makes an IDE-launched Electron executable behave as Node and exit.

No production desktop/container was replaced, and no commit, push, tag or release
was made in this slice. Native macOS packaging, elevated/historical upgrades,
mixed-DPI monitors, independent accessibility review, signing/notarization, and
speaker review of translations remain separately required. Installer shell text
is still English. See [model discovery](../guides/model-discovery.md) and
[desktop recovery](../setup/desktop-startup.md).

## 2026-09-19 — Real embedding and inference correctness slice

- Replaced the default hash-derived embedding backend with an owned loopback
  llama-server worker. Missing files and failed readiness checks fail closed;
  vectors are checked for count, indices, dimensions, finite values and nonzero
  norm. Requests and queued embedding calls honor cancellation; shutdown reaps
  only the owned child. Windows workers do not open console windows.
- Knowledge schema 3 records model/runtime digest-based embedding identity.
  Unverifiable prior indexes cannot be activated. Sources remain accessible.
  Offline `workspace rebuild-knowledge` makes a backup, takes exclusive ownership,
  stages documents resumably and publishes vectors plus metadata in one SQLite
  transaction. It does not silently skip missing sources or delete old data.
- Removed filename-derived chat stop tokens and blanket tool-capability claims.
  Seed zero is preserved; unsupported research controls fail explicitly instead
  of being silently discarded. Batch milliseconds and completion-token throughput
  now have explicit units/basis. Unicode truncation no longer splits UTF-8 bytes.
- Removed port-based process killing and unfiltered retrieval fallback. Shared
  embedding API calls bind model loading and execution together; knowledge checks
  embedding identity so another model cannot silently contaminate its vectors.
- Validation: full native Go suite passed; Linux race checks passed for inference,
  RAG, batch and API packages; generated API contracts and renderer type checks
  passed. Real local BGE-M3 returned 1024 dimensions, 36 measured prompt tokens,
  and related/unrelated cosine scores 0.7291/0.3305 on a three-sentence smoke test.
  Real-runtime interrupted/retried index rebuilding and retrieval passed on
  disposable data. Transaction tests cover cancellation, changed sources, invalid
  dimensions and injected write failure without replacing original vectors.
- Local container checkpoint: application and renderer rebuilt as
  `0.4.8-repair-20260919`; unchanged pinned CUDA runtime reused after a registry
  connection failure prevented the full GPU rebuild. Runtime binary hashes match
  before/after. Packaged BGE-M3 produced 1024 dimensions and 36 prompt tokens;
  paraphrase/unrelated cosine scores were 0.7252/0.3371. An isolated ingest/query
  returned its correct source. Nine browser navigation/recovery checks passed
  (recovery cases use fixtures; navigation uses the real packaged service).
  After a stopped-workspace backup and checksum verification, the authorized
  local container replacement preserved its existing model/data volumes and
  settings. Live health, build/UI identity, knowledge activation and real-service
  browser navigation passed. No release images or stable aliases were published.

This smoke evidence is **not** retrieval-quality qualification. Online durable
rebuild controls, shared inference admission for indexing, full sampling
provenance/diagnostic bundles, transactional workspace migration, project/team
permissions and research workflows remain unfinished. The native `llama` build
tag is not qualified by the default HTTP-runtime tests. No new release is certified.

## September 2026 recovery and workflow slice

Commit checkpoint (2026-09-19): the complete Go suite and targeted race suites
for inference, RAG, batch, agents, server and API packages passed. API contract
drift, TypeScript checks, renderer build, 28 desktop host tests and all 79 Edge
browser tests against an isolated packaged service passed. The renderer build
still reports its existing large-chunk warning. Opt-in real-runtime tests were
not rerun at this checkpoint; their earlier smoke evidence is recorded above.
Changes are grouped into runtime, history, shared-client, local-build and evidence
commits. The application version remains 0.4.8; this is not a new release or an
installed-platform qualification.

UI control consistency checkpoint (2026-09-19): shared web/Electron renderer
controls now use one sizing/focus/disabled contract. Knowledge actions are grouped
with document status rather than presenting Disable as a full-width first action.
Search and reconciliation fields have explicit form styling/labels. Action rows,
connector forms and long localized headings wrap at narrow widths. Type checking,
35 browser checks (including long-label/RTL/light/dark control regressions) and
18 desktop host tests passed. These are renderer/host checks, not evidence of
freshly installed Windows/macOS/Linux packages.
The local container was rebuilt as `0.4.8-ui-controls-20260919`, reusing the
unchanged verified GPU runtime. Six packaged-browser checks passed before the
authorized replacement. A stopped-workspace backup was verified; model/data
volumes and configuration were retained. Live health, new stylesheet/build
identity and real-service navigation passed after replacement. The previous
container remains stopped for recovery. No installers or release aliases were
published.

UI workflow presentation checkpoint (2026-09-19): removed badge backgrounds from
download status text and terminal progress bars. Installed models now have status
indicators instead of disabled primary actions. Knowledge setup tracks only its
selected embedding activation, distinguishes installation from retrieval readiness,
and unlocks recovery if its progress record disappears. Model search precedes the
catalog; catalog filtering, in-place page refresh, explicit unknown feature status,
named tool switches, keyboard tabs, fully visible narrow-screen navigation, safe
Markdown results and expandable Activity diagnostics are covered by regressions.
The full 60-test browser suite passed, followed by all eight focused presentation
tests (including two additional cases), type/API checks and 18 desktop host tests.
Screenshots were inspected for light-mode knowledge/catalog and mobile navigation.
Fourteen Edge checks passed against the isolated rebuilt image (real-service
navigation plus fixture-driven interaction/layout tests). The application/UI image
is `0.4.8-ui-workflows-20260919`; the unchanged native GPU runtime was reused and
its binary hash checked. After a stopped-workspace backup and checksum verification
(59 files, 213857 bytes), the authorized local replacement retained model/data
volumes and configuration. Live health, build identity and all-route navigation
passed. The stopped previous container is retained for recovery. No new native
installer, release qualification, push or publication is implied by these checks.

UI interaction recovery checkpoint (2026-09-19): history toolbars/search have
explicit spacing; command palette/mobile history contain keyboard focus and
respect IME. Initial history loading cannot change the conversation under an
editable draft. Account-scoped navigation state retains filters, connector drafts,
model discovery selection and Activity selection without writing connector URLs
to disk. Permission-aware controls avoid administrator-only requests for members;
`/v1/users/me` now reports authentication enforcement explicitly. Chat knowledge
readiness and unknown document index states are truthful. History/event/statistics
failures are distinct, bulk deletion exposes progress and a stop-after-current
boundary, and stale integration setup responses are ignored after model changes.
Desktop startup/custom menus share nine-locale resources and validated saved
appearance preferences. Native OS menu roles retain platform localization.

Evidence: API drift/type checks and renderer build passed; all 73 browser tests
passed in Edge against the isolated rebuilt application, including real-service
navigation/session persistence and fixture-driven interaction/fault cases. Desktop
host tests passed (28); the Go authentication-enforcement contract test passed.
Desktop/mobile history and narrow-screen Settings screenshots were inspected.
The local image `offgrid-llm:ui-recovery-gpu-20260919` reuses the unchanged native
GPU runtime (hash verified). After a stopped-workspace backup and archive checksum
verification (59 files, 213857 bytes), the authorized replacement retained volumes
and environment configuration. Live health/build identity and all-route navigation
passed. UI build: `977f3464f042bb7db1b4c4ea68beb3d1c1ccb9a9fa6c8f87ba112ce9e0a3f05c`.
Backup: `/home/phil/.local/state/offgrid/backups/20260919-ui-recovery/workspace-before-ui-recovery.zip`.
Recovery container: `offgrid-rollback-20260919-ui-recovery` (stopped). The disposable
test container was removed. No installed desktop upgrade, full native-runtime
requalification, translation speaker review, commit, push, or release is implied.

Legacy agent-history repair checkpoint (2026-09-19): interrupted records without
any checkpoint or pending approval can now be removed by their authorized owner
(local administrator for unowned legacy records). Active workers, resumable
checkpoints and uncertain outcomes remain protected. Deletion persists a minimal
tombstone so Activity cannot resurrect the record. The bulk action is now
"Clear removable tasks", with matching explanations across all nine locales.

Evidence: agent-package race tests, server history deletion/restart race tests,
API contract drift checks and renderer build passed. All 17 targeted packaged
Edge history/interaction/navigation tests passed; live all-route navigation passed
after deployment. This does not qualify installed desktop packages or translations.
The approved deployment uses `offgrid-llm:history-repair-gpu-20260919`, retaining
the unchanged hash-verified GPU runtime, environment and data/model mounts.
UI build: `e9aab400c59ccf7c5c83f4c2cc0fb0652500f939ea92e25be84307794b5b4a05`.
The stopped-workspace backup verified 59 files / 152104 uncompressed bytes at
`/home/phil/.local/state/offgrid/backups/20260919-history-repair/workspace-before-history-repair.zip`
(archive mode 0600). The old container is retained stopped as
`offgrid-rollback-20260919-history-repair`; the isolated test container was removed.
Only the three explicitly approved legacy D-drive task records were deleted via
the authorized API; their absence from task history and Activity was verified.
Tool-created files, audit records and the recovery backup were not deleted.
No commit, push or publication was performed.

Palette and agent-layout checkpoint (2026-09-19): replaced the square search focus
frame with an inset focus line; palette height now accounts for its header and
viewport offset. Keyboard selection scrolls only the list; search has combobox
semantics and Esc is also a clickable dismissal control. Agent status cards became
a compact strip, task entry precedes model/style settings, desktop results receive
more width, and selected history is marked. Finished execution steps are expandable.
Large/live output and approvals preserve Run/cancel positioning.

Evidence: renderer build, API/type checks, 78 Edge tests against the isolated
packaged container, and 28 desktop host tests passed. Screenshots were inspected
for agent desktop layout and palette compact-height/RTL presentation. This does
not qualify installed desktop packages or replace screen-reader/user review.
Authorized local deployment uses `offgrid-llm:agent-layout-gpu-20260919`, preserving
the unchanged native GPU runtime (hash checked), model/data mounts and service
environment. Live health, build identity and all-route navigation passed.
UI build: `9b5ee03849af2dacd14174e3d8245c6976f8262fc78a727641bf04c62b6f62de`.
Stopped-workspace backup verified: 59 files, 197302 bytes, stored with mode 0600 at
`/home/phil/.local/state/offgrid/backups/20260919-agent-layout/workspace-before-agent-layout.zip`.
The old container `offgrid-rollback-20260919-agent-layout` is retained stopped;
the disposable test container was removed. No commit, push or publication.

- Durable chat turns are admitted before inference, survive navigation and stream
  disconnects, and replay their persisted snapshot without resubmitting model
  work. Explicit Stop is tied to the exact turn ID; partial, cancelled, failed,
  and interrupted output never enters completed conversation context.
- Model downloads persist repository, source file, local identity, progress, and
  knowledge-setup intent. Restart exposes retained work for explicit Resume;
  partial bytes cannot be reassigned to another source. Completion is persisted
  after model discovery and optional knowledge activation.
- The CLI model list/download paths now use the running service, have stable
  success/usage/operational/cancellation exit codes, and keep JSON stdout clean
  while human progress goes to stderr.
- Knowledge documents remain visible while retrieval is disabled. Retained
  extracted text can be inspected with authorization rechecked by the service;
  deletion and model removal require explicit confirmation, and active runtime or
  knowledge use blocks unsafe model removal.
- Activity selection ignores late responses, onboarding wraps keyboard focus,
  API reads have bounded deadlines, and recovery controls use localized labels.

Evidence for this slice includes Go unit tests, Windows and Linux race tests,
real-service browser tests, contract generation, renderer builds, and desktop
host tests. It does not replace the remaining multi-user SQLite migration,
installed-package qualification, signing/notarization, representative-hardware
benchmarks, security review, soak test, or user pilot gates above.

Release preflight (2026-09-19, v0.4.9): the packaged startup check still expected
raw timeout text in the primary status message after recovery details moved to
the expandable technical section. Reproduced that assertion failure with the
Windows package. The check now verifies the unavailable state, localized recovery
guidance, keyboard expansion and underlying timeout details; no recovery check
was removed. The real Windows package passed mismatch recovery, isolated service
startup/restart, external-service attachment, timeout/retry and IPC checks.
Evidence: `%TEMP%/offgrid-desktop-startup-3TLa8o`. This is local Windows startup
evidence, not macOS/Linux or installed-upgrade qualification.

Windows inference ownership repair (2026-09-19): the installed v0.4.9 service
returned HTTP 503 for both chat models because orphaned llama-server processes
occupied the fixed inference port. Removing only the verified orphan PIDs restored
streaming without deleting models or history. The source fix reserves OS-selected
loopback ports, avoids leaking port tracking on failed starts, uses Windows-native
process liveness checks and attaches chat/embedding children to a non-inheritable
kill-on-close job. Normal unload waits for each process only once.

Windows inference/server/CLI tests and Linux inference/server race tests passed.
An isolated patched Windows service loaded the installed TinyLlama and Morena
files while the desktop's existing inference listener remained running. Both
returned HTTP 200, streamed text, normal stop and `[DONE]` for a short arithmetic
prompt (about 1.8 and 2.4 seconds respectively). Repeated calls reused live runtime
PIDs; forcibly stopping the test service terminated all its inference children
without stopping the installed service. Evidence is under
`%TEMP%/offgrid-runtime-repair-6f0542319cab43e4a79c942f26643e06`.
These are runtime/transport checks, not model answer-quality qualification or a
visual desktop UI pass. No release publication or installed-binary replacement
is implied by this source checkpoint.

### Computer Tasks foundation — 2026-09-20 (in progress, not operational)

The approved program is supervised, local-only browser and native automation on
Windows, macOS and Linux. Structured browser/accessibility observations come
first; separately qualified local vision is a fallback. No platform driver,
companion, automation pack or model/hardware profile is qualified at this checkpoint.

Implemented foundation:

- Agent snapshots (including checkpoints, approvals and completed output) now
  use `agent-state.sqlite` through the existing durable SQLite connection policy.
  Snapshot updates and status-transition records commit together. This is the
  agent persistence adapter, not completion of the broader workspace migration.
- Initial legacy import is staged in one transaction. Every JSON record is
  checked for identity, status and approval ownership/arguments before activation;
  counts and database integrity are verified. The `agent_schema.recovery_manifest`
  records source hashes, IDs and owners. Original `agent_tasks/*.json` files remain
  unchanged and are never dual-written or reimported after activation. Unowned
  records remain local-admin controlled. Errors block agent admission with an
  actionable storage error; no partial task list is published.
- Running work recovers as interrupted, or uncertain if a tool call was dispatched.
  Existing exact-call approval, cancellation and explicit reconciliation tests
  continue to run against the new adapter. No restart automatically executes work.
- Legacy `/v1/computer/{session,action,reset,stop}` execution requests now return
  HTTP 426 with `computer_api_upgrade_required`; caller-supplied approval booleans
  cannot activate the controller through HTTP. Read-only legacy status remains.
  The first-party UI uses `/api/v2/computer/status` and `/api/v2/computer/stop`.
- Administrator-only `/api/v2/computer/capabilities` explicitly reports preview,
  local-only, supervised and `companion_unavailable`. Every driver is unavailable
  and unqualified. Generated TypeScript contracts include these endpoints.

Recovery/operator notes:

- Back up the stopped workspace with the existing exclusive backup command before
  upgrading. It includes the database, WALs, originals and blobs together.
- On a first-import error, stop the service and repair the named original using a
  trusted backup, then retry. Do not delete records merely to bypass validation.
- After activation SQLite is authoritative; editing old JSON cannot repair or
  modify live history. Retained migration originals and backups may contain history
  deleted from the active store. Deletion is not a secure erase of backups/WALs.
- This adapter refuses unknown schema versions. Older released binaries do not
  understand this migration and must not be used on the upgraded workspace: restore
  a matched application/workspace backup for rollback. Enforcement in old binaries
  cannot be added retroactively.

Remaining implementation gates, in order:

1. Companion enrollment, OS credential storage, protocol negotiation, local consent
   and an independently usable emergency stop.
2. Immutable action IDs and companion dispatch/result journal; exact-call grants
   bound to the observed target and expiry, with uncertain-outcome reconciliation.
3. Managed Chromium pack, bounded domains/download destinations and local fixtures.
4. Windows UIA, macOS Accessibility/ScreenCaptureKit, Linux AT-SPI/portal drivers
   with real OS-session and window-scope enforcement.
5. End-to-end typed image observations, sensitive-data redaction and verified
   model/projector/runtime profiles; no cloud fallback.
6. Integrated Computer Tasks UI/CLI, job-event replay, pairing/permissions recovery,
   nine locales, offline packs and installed-application qualification.

The Computer Tasks mode may be exposed for supervised preview, but it must not
advertise vision readiness or native-platform qualification. No running user
instance, release artifact, tag or container is replaced by this checkpoint.

Foundation validation: Windows Go tests passed for agents, computer, server,
storage, CLI, RAG and sessions. Linux/WSL agent and computer race tests passed.
OpenAPI TypeScript generation/check and renderer type-check passed. All 31
selected Playwright functional-page, interaction-contract, workflow-presentation
and workspace-experience tests passed. Those browser tests use API fixtures;
they are regression evidence, not real-companion or installed-driver qualification.

### Browser preview checkpoint — 2026-09-20

This supersedes the foundation-only availability description above, not the
remaining platform qualification gates. A source-installed managed Chromium
companion is now available for supervised testing; see
[companion setup and limitations](../../computer/README.md). It is not a signed
automation pack or generally qualified computer-use capability.

- Administrator pairing uses short-lived, single-use codes and visible terminal
  consent. Session credentials remain in memory, expire after ten minutes, and
  cannot survive service restart. Persistent OS-keystore enrollment is pending.
- Browser tasks reuse the durable agent runner, exact-call approvals and task
  history. Typing, clicks and navigation require approval. Stale observations
  fail closed. Dispatch IDs are journaled before execution; lost acknowledgements
  do not cause automatic action replay. Completion requires an explicit final
  page-text verification, not merely a model assertion.
- The dedicated browser permits one selected public HTTPS origin. Reserved and
  private destinations, cross-origin resources and downloads are blocked. No
  personal browser profile, screenshot capture or native application driver is
  exposed. This narrow policy intentionally makes some sites unsupported.
- Agents has a preview setup/target panel; authenticated CLI commands provide
  pairing, status, targets, run and stop. These are not the completed pack setup,
  pause/takeover, vision or cross-platform native workflows.

Validation: focused Go suites and Linux race checks for computer, agents and
server passed; renderer type-check and 14 browser regression checks passed.
Two companion tests passed, including real Chromium interaction with an isolated
local fixture. Runner approval tests use a deterministic model callback. No
installed local model has passed a complete real-browser task qualification.

Deployment: the application and GPU-runtime-reuse images were built locally from
the working tree. The existing stopped workspace was backed up and its archive
checksum verified. The replacement first passed health, agent-history and UI
checks against a separate data clone, then replaced the authorized `offgrid`
container using the existing model/data volumes. The prior container remains
stopped; rollback requires restoring its matched data backup, not starting it
against the migrated workspace. No release or Git publication was performed.

The replacement reports `0.4.10-computer-preview`, healthy with zero automatic
restarts at verification. WSL loopback HTTP checks passed after replacement.
Windows loopback returned HTTP 200 once but a subsequent check failed to connect;
forwarding remains intermittent and is not qualified as working. WSL was not
restarted, as requested. Public HTTPS companion testing remains
blocked by VPN fake-DNS reserved addresses; the VPN was left enabled and the
network guard was not weakened. Native drivers, vision, offline packs, OS
credential storage and model/platform acceptance evidence remain outstanding.

### Browser setup and workspace recovery repair — 2026-09-20

Confirmed incident: Agents fetched a saved task ID that returned 404 while tools,
history, integrations and computer status returned 200. The UI incorrectly turned
the missing selection into a persistent generic error. It now clears that
selection, preserves the task draft, and explains the missing task. Authorization
errors remain distinct and are not silently treated as deletion.

The service creates an opaque persistent `workspace-id` while holding exclusive
workspace ownership and includes it in `/api/v2/system`. Agent selections are
namespaced by workspace and actor, not just browser origin/user. Unscoped legacy
selections are not silently adopted by a different workspace. Identity corruption
blocks startup instead of silently changing the namespace. Backups retain the ID.

Companion preflight checks local service health and opens the permitted browser
before asking for a short-lived pairing code. Reserved-address failures explain
fake DNS and the available local-demo alternative. Public browsing through VPN
fake DNS remains unsupported; there is no blanket reserved-address exception or
implemented explicit proxy transport.

The `demo` target owns an ephemeral loopback HTTP server serving only a fixed
research-notes page. It permits no arbitrary loopback target, file access, upload,
proxy or external submission. Pairing uses the exact logical target
`offgrid-demo://research`; other custom/local origins remain rejected. The browser
uses the same tool/approval path, and closing the companion closes the fixture.
Saving a demo draft is deliberately a page-only effect, not a verified artifact.

UI setup instructions cover the new target; unpaired status no longer claims a
missing installed driver. Pairing errors are no longer erased by successful
background polling, and expired selected sessions are cleared.

Evidence: Windows Go storage/computer/server suites, Linux race checks for those
packages, generated TypeScript and renderer type-check passed. All 16 focused UI
tests passed, including stale-selection reload/draft preservation and different
workspace selection isolation. Three real Chromium/network-policy checks passed,
including owned-demo edit/verification and rejection of other loopback services.
The authorized local replacement reports `0.4.10-computer-recovery`; a live
headless-browser check confirmed the new setup text and absence of the generic
banner. The stopped-workspace archive checksum passed before replacement; the
prior container and backup remain available. No model-driven full task outcome,
native driver, vision profile or public VPN browsing is qualified by these checks.

### Computer model compatibility gate — 2026-09-20

The first real demo task on `phi-3.5-mini-instruct.Q4_K_M` returned prose describing
simulated actions, no tool calls and no approval request. The browser companion
remained connected. Runtime `/props` reported a content-only template with
`supports_tools: false` and `supports_tool_calls: false`. A separate harmless
inference request asking for a named tool call also returned prose. This is
evidence for this installed model/template/runtime combination, not a universal
claim about the Phi model family.

Added administrator-only `POST /api/v2/computer/model-check` and
`offgrid computer check <model>`. The bounded 90-second check uses the shared
inference admission gate and the agent's structured streaming parser where
available. It requests an observation call followed by a verification call whose
argument must match a randomly generated fixture heading. Neither call is
executed. Plain text, wrong tools/arguments, null arguments, truncated output and
missing terminal tool calls fail the check. Transport failures/cancellation do
not claim model incompatibility. Runtime build, template hash and allocated
context are reported when obtainable; template contents and model paths are not.

The UI requires an explicit model check and clears its result on model changes.
Submission independently repeats the probe before creating a task, returns 422
with `computer_tool_calling_unavailable` for a negative result, and rechecks the
browser session after inference. Runtime failures return retryable 503 instead.
No cached pass, caller-supplied approval or UI flag bypasses admission. Ordinary
chat/agent workflows without a computer session are unchanged. Passed probes
remain smoke evidence, not a guarantee of complete tasks or safe model behavior.

The no-browser-actions failure now explains that the model returned text without
using tools; final result verification and exact-action approval are still
mandatory. No prose is parsed into executable actions. Empty-argument tool
schemas no longer emit invalid `required: null` declarations.

Validation: focused Windows Go server/inference/CLI suites, Linux server/inference
race tests, TypeScript/contract drift checks and all 17 focused UI tests passed.
Tests cover rejected admission without task creation or browser dispatch, wrong
tool/arguments, cancellation, runtime failure distinction, and changing models
after a successful UI check. Local application and GPU-runtime-reuse images were
built as `0.4.10-computer-preflight`. With separate operator approval, the stopped
workspace was backed up and checksum-verified, the image was checked against an
isolated data clone, and the running container was replaced. Windows localhost
health, product identity and the new model-check UI asset were verified. The
previous container remains stopped; no models were removed. The live probe
correctly rejects the installed Phi template with no browser dispatch. See
[upstream tool-calling/template guidance](https://github.com/ggml-org/llama.cpp/blob/master/docs/function-calling.md)
before qualifying a replacement model/runtime/template combination.

The operator also approved downloading Qwen2.5 3B Instruct Q4_K_M from the official
Qwen repository for testing, not as an automatically qualified replacement.
Upstream file size is 2,104,932,768 bytes; expected SHA-256 is
`626b4a6678b86442240e33df819e00132d3ba7dddfe1cdc4fbb18e0a9615c62d`.
The completed download matched that SHA-256. Initial live checks with the
existing `q4_0` K/V cache setting failed, including direct llama.cpp requests
which returned malformed/prose responses. An isolated same-model/runtime test
with `q8_0` K/V cache and 32,768 context returned the expected observation and
verification calls. The local service was backed up/recreated with `q8_0`, and
its two-step streaming checks then passed repeatedly. The actual runtime reports
32,768 allocated context; the configured 65,536 request is not an effective
64K capability for this model. This is profile-specific evidence, not a blanket
quality claim for either cache type.

The opt-in `computer/test/live-demo.mjs` harness tests the real service/model
against its own temporary Chromium page. It permits exactly the known field edit
and save-button approval, independently checks the resulting DOM, refuses other
mutations and retains the diagnostic run in history. It is not a replacement for
interactive companion journal/recovery tests or the workflow qualification suite.
Browser observations now include associated HTML labels; real-browser regression
coverage confirms `Report title` identifies the correct input without collecting
its value. All three browser-driver tests passed.

Full model-driven task qualification **failed** for this 3B profile. The first
attempt invented an initial observation ID before inspecting the page. Computer
tasks now use temperature zero (matching the probe) and explicit sequential
observation instructions; ordinary agent sampling is unchanged. With that build,
the model observed the real page but proposed saving before filling. A further
explicitly sequenced prompt observed the page but supplied a placeholder rather
than the actual observation ID. The fixture allowlist refused each change before
execution and cancelled the diagnostic runs; no draft was saved. These attempts
remain in history as cancelled tests, not successful tasks:

- `run-4f35eab89aaffd90035ea41d92ae39ec`: invented initial observation ID.
- `run-7a259e3eb503e75ad75bd7a7fd3cbabd`: save requested before filling.
- `run-959057adb4278da61e3f7a0a60403bb7`: placeholder observation ID.

Do not describe Qwen2.5 3B as a qualified browser-task model based on the synthetic
probe. The operator subsequently approved a larger-model trial. The official
7B Q4_K_M artifact is split, which the current standalone installer rejects;
the trial uses the single-file
[Bartowski Qwen2.5 7B Instruct Q4_K_M build](https://huggingface.co/bartowski/Qwen2.5-7B-Instruct-GGUF)
(4,683,074,240 bytes; publisher SHA-256
`65b8fcd92af6b4fefa935c625d1ac27ea29dcb6ee14589c55a8f115ceaaa1423`).
This is a community quantization, not an official Qwen artifact. The transfer
reached 1,833,587,687 bytes (39.15%) before repeated network EOF and HTTP/2 stream
CANCEL failures. A resume preserved the existing bytes but failed again; Windows
curl also failed its TLS handshake. A small HTTP/1.1 range diagnostic succeeded
slowly. The partial file was preserved at that checkpoint; the operator later
completed the download. Its full installed file now matches the publisher SHA-256
above. The required VPN and TLS/network safety checks were not disabled.
Focused Windows Go suites and Linux
server/agent race tests passed after the sequential-protocol change. The final
local image is `offgrid-llm:computer-verified-gpu-20260920` (the tag is not a
qualification claim), service version `0.4.10-computer-preflight`. Health and
Windows localhost access passed; diagnostic browser sessions were closed. No
commit, push or release was performed.

### 7B sequential browser protocol repair — 2026-09-20

The installed 7B model's template digest is
`55b2f4a26ac9ee719330a61a0c39d9e538b0e9322212f048326318cff4f8674a`,
runtime `b1-e9fa078`, actual context 32,768 and K/V cache `q8_0`. Its old preflight
response bundled `browser_observe` and `browser_verify` with invented text before
receiving an observation. Direct runtime testing reproduced this; template
support declarations were not proof of correct sequential behavior.

The preflight and task planner now share explicit one-call-and-wait instructions.
Computer requests also send `parallel_tool_calls: false` through streaming and
non-streaming transports. Ordinary requests preserve the runtime default unless
the caller explicitly selects a value. The flag alone did **not** prevent the
observed response on this pinned runtime; strict call validation, observation
identity and exact approvals remain necessary. See the pinned
[runtime parameter documentation](https://github.com/ggml-org/llama.cpp/blob/e9fa0781f1c25fc4fe8c86be1edc6970661ad6f0/tools/server/README.md).
Failure messages now identify missing/invalid calls, multiple calls or incorrect
verification text without exposing raw model output.

After rebuilding, clone validation and a checksum-verified workspace backup,
the local container runs `0.4.10-computer-sequential` from
`offgrid-llm:computer-sequential-gpu-20260920`. Windows Go suites, Linux
server/inference race tests, TypeScript and contract checks passed. The real 7B
model passed the streaming probe and completed an explicitly sequenced fixture
task (`run-d1909df84729762ba481f23b927c4e1a`): observe, approved fill, observe,
approved click, observe, verify. Independent DOM inspection confirmed the exact
temporary draft text. No file or external submission was created.

A shorter task prompt then reused a stale observation after filling
(`run-6cca11c18725e1063cc6f02c084a3944`); the test refused the click and cancelled
the run. Browser mutation results now include a freshly observed, scope-checked
page and element IDs. Old IDs still fail; a click still reports `verified: false`
until the separate final check. Real-browser tests cover those invariants.
Further short-prompt validation is pending operator consent to stop an already
paired browser session. One successful fixture is not general browser-task,
native desktop, vision or model/platform qualification. No release was made.

### Browser preview safety and recovery repairs — 2026-09-20

The defect review reproduced a stale-form click after manual input and automated
typing into an associated-label-only password field. Browser observations now
track input/change events and compare a local-only form-state digest before
mutations, including silent JavaScript value changes. One shared accessible-name
classifier checks associated labels, aria-labelledby, aria-label and placeholders
at observation and input execution. The digest/field values are not sent to the
service. These checks are defense in depth, not a claim that hostile websites or
all credential-field naming conventions can be perfectly classified.

Computer runs now require an immutable user-authored `computer_expected_text`.
The API, CLI (`--expect`) and UI carry it through persisted agent configuration;
tool authorization rejects a model-substituted criterion, and completion requires
a matching positive `page_contains_text` result as the last step. This is explicitly
page-text verification only, not proof of file creation, a transaction or all
semantic aspects of an arbitrary task. Old runs with no criterion cannot be
silently promoted to verified completion. Ordinary agents are unchanged.

The durable runner enforces one computer tool call per response and observation
before the first tool action. Old batched checkpoints are refused too. Browser
sessions are reserved before launching a task, and list responses expose ready,
assigned, finished and exhausted states so consumed sessions are not selectable
for another run. The user must still stop/re-pair for a fresh task consent.

The workspace shell provides a browser-stop strip across page navigation. Active
computer mode and the recorded success criterion are reconstructed from run
snapshots after reload; the Agents page waits for workspace identity before
mounting its interactive form to avoid focus loss. Optional model diagnostics
move under Advanced settings; admission automatically performs the compatibility
check instead of making users run it manually first. Pairing has a copy control,
consumed-state guidance and a session countdown. Nine locales have draft copy;
speaker review remains pending.

The source companion is still terminal-launched. Packaged installation/repair,
OS-keystore enrollment, native desktop drivers, vision, complete pause/takeover,
proxy qualification and broad real-model workflow qualification are not delivered
by these repairs. No live container replacement, release, push or commit is
implied. The previous successful 7B fixture is not evidence for this changed
build; a fresh real-model end-to-end qualification remains required.

Validation for this repair batch: Windows Go tests passed for agents, computer,
server, inference and CLI; Linux race tests passed for agents, computer and
server. Five real Chromium driver tests passed (including manual/silent field
changes and credential-label exclusions). All 36 selected renderer interaction,
workflow, functional-page and workspace tests passed, including navigation/reload
stop controls and used-session rejection. TypeScript, generated API contract
drift checks and the production renderer build passed. The renderer still emits
the existing >500 kB chunk-size warning. These are scoped regression results,
not installed-package, real-model task or cross-platform qualification.

### Optional browser verification and intermediate checks — 2026-09-20

Follow-up to the failed two-stage title task: verification authorization had
incorrectly required every read-only check to equal the final criterion. The
recorded run also contained `done` as its final criterion, which was not the
requested page result. Intermediate checks now remain read-only and allowed;
strict final comparison is enforced only at completion.

Normal UI/API/CLI submissions no longer require expected page text. The UI moves
it into optional verification settings, and CLI `--expect` is optional. Newly
accepted runs record the `page-evidence-v1` policy. Completion still requires an
affirmative companion check matching its recorded arguments after any mutation;
an unchanged initial heading does not establish a mutation's success. Legacy
criterion-less runs are not silently reclassified. The model interprets which
evidence is relevant: these checks are not independent semantic proof of the
whole task or external effects. Idempotent changes may remain unverified.

Validation: Windows Go server/agent/CLI tests, Linux server/agent/computer race
tests, 36 renderer regression tests, five real Chromium driver tests, TypeScript,
API drift and renderer build passed. The existing bundle-size warning remains.
Two real Qwen2.5 7B runs passed without `computer_expected_text`: the plain request
"Set Report title to OffGrid test and save the draft" (two approvals,
`run-fb0603e2be287bb24bdb6e0e3f38acd5`) and preliminary-to-final Unicode revisions
(four approvals, including an intermediate verification,
`run-9981929ec3400362f85f77b9c1a0c874`). The harness independently inspected the
final input and status. These are two owned-fixture runs, not general browser
qualification; runtime `b1-e9fa078`, effective context 32768, template prefix
`55b2f4a26ac9`, Qwen2.5-7B-Instruct-Q4_K_M with q8_0 cache.

The user authorized backup and live replacement. Clone checks passed, then
`offgrid-llm:computer-evidence-gpu-20260920` was activated with the existing GPU
runtime. Windows localhost health and new renderer assets were verified. Backup:
`/home/phil/offgrid-computer-backup-AU4HfpZP`; stopped prior container:
`offgrid-before-computer-1789905577`. Models and original data volumes were
preserved. No release, push, or commit was performed.

### Remove developer verification controls from the task UI — 2026-09-20

The expected-page-text field and its explanatory result block are removed from
the shared web/Electron renderer, not merely collapsed. The first-party run
client no longer accepts or sends a criterion. The scoped obsolete draft is
cleared on workspace mount; another old tab rewriting it cannot influence new
requests. Task drafts, other accounts/workspaces, and historical runs are
preserved. Existing in-flight strict checks retain their saved semantics; this
change does not rewrite history or weaken backend approval/completion checks.
Strict criteria remain available to developer API/CLI tests only. Unused UI
translations and styles were removed along with the field.

TypeScript, API drift, production Docker renderer build and all 36 selected UI
regressions passed. Regression coverage includes a saved `done`, an oversized
value recreated by an older tab, reload, preservation of another workspace and
task drafts, and restoration of an existing approval without restoring the old
input. A real Chromium smoke against the deployed service confirmed absence of
the field and cleanup of an isolated browser's stale setting; it submitted no
task. This renderer cleanup does not qualify the broader Computer Tasks program
as production-ready or constitute installed Electron qualification.

After explicit user approval, clone validation and backup preceded deployment of
`offgrid-llm:computer-clean-ui-gpu-20260920`, version
`0.4.10-computer-clean-ui`. Live health and renderer assets were verified from
Windows. Backup: `/home/phil/offgrid-computer-backup-vngEEkdC`; previous stopped
container: `offgrid-before-computer-1789906663`. Models remain in their existing
volume. No commit, push or release was made.

### Desktop-owned browser setup and task-first workspace — 2026-09-20

The updated desktop owns the browser companion as an Electron utility process.
Users describe a task, enable **Use a browser**, choose an HTTPS origin or the
practice page, and approve native local consent. Node/npm installation and manual
pairing are not part of the installed desktop flow. Enrollment occurs **after**
consent and integrity verification using the desktop's authenticated session;
the renderer supplies only origin/workspace, not a pairing code or credential.
The service still enforces administrator authorization. The worker receives its
short-lived enrollment over private IPC and keeps its session token in memory.

The shared CLI/desktop session core retains journaled immutable action IDs,
duplicate refusal and uncertain-outcome behavior. Stop during startup cleans up
late resources; local Stop, lock, suspend and quit close only the owned worker.
The UI shows readable action/approval summaries with expandable exact arguments,
a browser activity timeline, visible verification outcomes, and no model reasoning
preview for browser tasks. Work leads with the task instead of runtime metrics;
Permissions and Connections remain supporting views. Copy is shared in all nine
locales, without claiming independent translation review.

Desktop packages assemble pinned Playwright 1.62.1 and its Chromium 1234 build for
each advertised desktop architecture. An after-pack integrity check validates
the actual copied payload. Tests caught and fixed electron-builder's exclusion
of root `node_modules`; the dependency directory now has an explicit resource
mapping. Vendor browser binaries are preserved rather than re-signed as OffGrid.
Hashes detect corruption, not authenticity independently of a trusted package.
This bundled approach adds browser download/disk size; independently signed
optional automation-pack install/repair remains pending.

Web provides an `offgrid://computer` handoff without a service URL, token or action
payload. The desktop must attach to the same compatible local workspace. A web
page alone cannot control the host. Developer pairing remains collapsed. Alternate
desktop/test profiles do not register a protocol handler. The local preview's
handler points to `build/desktop-browser-preview/win-unpacked/OffGrid LLM Desktop.exe`;
the installed production application has not been overwritten.

Recorded local evidence:

- 86 web tests passed; 18 interaction tests re-passed after enrollment moved out
  of the renderer. Type checking, UI build and generated API drift check passed.
- 34 desktop tests and 9 companion tests passed, including consent refusal,
  workspace mismatch, stop during startup, duplicate/lost acknowledgement,
  manifest tampering and all-nine-locale key parity.
- Packaged Windows x64 startup/recovery passed with an isolated real Go backend:
  `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-startup-FdTUyg`.
- Final packaged Windows renderer/preload/main/utility/Chromium test passed:
  `C:\Users\phil\AppData\Local\Temp\offgrid-packaged-browser-dWFLFs`.
  The test checks authenticated cookie forwarding for enrollment, observe/fill/
  save/verify, local Stop and a second session after runtime use. Native consent
  is intercepted **only in the test process** and checked against its owned demo;
  this does not constitute manual OS-dialog or model-planning qualification.
- Windows Go tests passed for computer, agents, server and CLI. Workflow actionlint
  passed. CI now exercises the packaged browser path on its desktop matrix;
  macOS/Linux results for this change are not yet recorded locally or remotely.

After user authorization, backup and clone verification preceded activation of
`offgrid-llm:desktop-browser-gpu-20260920` (local version 0.4.10, existing GPU runtime
reused). Final backup: `/home/phil/offgrid-computer-backup-ZCrXgdwQ`; prior stopped
container: `offgrid-before-computer-1789912480`. The earlier checkpoint backup
`/home/phil/offgrid-computer-backup-DOvvVYBP` is also retained. Models/data volumes
were preserved; no WSL restart, commit, push, release or live desktop install was
performed. Desktop and service UI identities are checked for equality.

Remaining qualification is explicit: general native desktop control, vision,
arbitrary public-site workflows, VPN fake-DNS support, OS-store persistent pairing,
signed optional packs, macOS signing/notarization and the full task/model/platform
matrix are not delivered or qualified by this browser-setup work. The feature
remains preview; these checks are not a 9/10 or production-readiness certification.

### Browser form and deployment checkpoint (2026-09-21)

The supervised browser now exposes typed single-select and checkbox actions in
addition to observation, navigation, clicking, filling and verification. Fresh
observations supply option identifiers and state; arbitrary fields, invalid types,
oversized arguments, disabled controls and stale observations are rejected.
Mutations still require exact-call approval. The practice page includes a report
format and source-inclusion checkbox so repeated multi-control tasks can be tested.
Custom widgets, multi-select and native/vision automation remain unsupported.

Local Stop and service revocation are attempted independently. An unacknowledged
worker shutdown retains ownership, reports `stop_unconfirmed` and prevents a new
session. The packaged application owns the integrity verifier rather than loading
verification code from the unchecked runtime payload. Chat history responses also
carry selection/request revisions so stale replies cannot replace a newer draft.

Recorded local validation:

- Full Windows `go test ./...` passed; WSL race tests passed for computer, agents
  and server. API drift, TypeScript, UI build and workflow actionlint passed.
- 90 browser UI tests, 35 desktop unit tests and 11 companion tests passed.
  The six history tests additionally passed three repeated runs.
- Final packaged Windows browser form workflow, local Stop and second session
  passed: `C:\Users\phil\AppData\Local\Temp\offgrid-packaged-browser-ZNdlWy`.
- Final packaged startup/recovery passed, first window 1062 ms, recovery 1264 ms:
  `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-startup-wgDZFP`.
  These isolated fixtures are not real-model or all-platform qualification.

With explicit user approval, the idle service was stopped and its workspace
archived with a verified checksum before clone validation and live replacement.
Backup: `/home/phil/offgrid-computer-backup-6KENbl7p`; retained previous container:
`offgrid-before-computer-1789919962`. Live local image:
`offgrid-llm:browser-forms-gpu-20260920`, using the existing GPU runtime.
All four models and 13 task records remain available. Windows localhost health
passed and the service UI matches both `web/dist` and the matching unpacked desktop
at `build/desktop-browser-forms-preview/win-unpacked/OffGrid LLM Desktop.exe`.
The installed desktop and its existing protocol-handler registration were not
replaced; use that matching unpacked build for this preview. No WSL restart or
release publication was performed. macOS/Linux packaged results, native control,
vision, signed optional packs and the full model/task qualification remain pending.

### Release-gate repair checkpoint (2026-09-21)

The release now requires successful CI for its exact immutable source revision.
An explicitly authorized replacement of an unpublished draft discards artifacts
from a different source and forces container rebuilds; published releases cannot
be replaced through that path. Release-gate and draft-retry fixtures passed.

Packaged browser tests use hydrated navigation and the runtime's bounded Stop
deadline, while still requiring confirmed shutdown and independently verified
form results. Three consecutive packaged Windows browser runs passed locally.
The installer driver now excludes hidden maintenance windows from visible-UI
responsiveness checks; its process and Finish deadlines remain enforced. A real
Windows fixture verifies that a visible frozen window is still rejected.

The final isolated installer run passed clean installation, installed startup,
repair, Finish with/without launch, silent running-app refusal, interactive
running-app consent, uninstall and workspace-fixture preservation. Finish closed
in 70 ms and 69 ms respectively. Evidence:
`C:\Users\phil\AppData\Local\Temp\offgrid-install-6b21ca173f1a494792c0e4b550824b07`.
Earlier local attempts exposed a startup timeout and a missed Finish transition;
these are not counted as passes. Fresh hosted CI is still required before
publication. This evidence does not qualify signing, native computer control,
vision or general model-driven browser task reliability.

### Native Computer Tasks foundation checkpoint (2026-09-21)

This is a partial implementation of the all-platform program, not a native-control
release or a completed milestone. The existing browser preview remains the only
executable computer driver. No native/vision availability flag was enabled.

Implemented in the working tree:

- Removed the legacy session-wide-approved controller from production server
  wiring. Browser actions still pass through durable agent authorization.
- Added protocol-v2 typed operations, opaque target/process-generation identity,
  framed private-worker messages, exact bounded-step digests, consent identity,
  five-minute/session-capped approval validation, fresh-observation validation,
  and separate confirmation requirements for consequential/unknown actions.
  These are executable contract validators, **not an integrated native dispatch
  supervisor**. Existing browser transport remains explicitly protocol 1.
- Added schema-2 agent activity metadata in the existing SQLite database. Snapshot
  updates and ordered events commit atomically; metadata retention is bounded to
  256 entries per task. Schema-1 activation requires a verified consistent backup,
  preserves task ownership/results/import metadata, and records backup provenance.
  Corruption or an injected migration failure blocks activation without publishing
  partial work. Older status-only history remains in the recovery copy.
- Added owner-scoped `/api/v2/jobs/{id}` and `/api/v2/jobs/{id}/events` reads with
  Last-Event-ID replay, explicit snapshot recovery, and bounded subscriber write
  deadlines. Web/Electron renderer negotiates `durable-agent-events-v2`; it commits
  a cursor only after applying a snapshot. Existing v1 clients remain supported.
- Computer approvals and resumability expire on service restart; historical
  results/checkpoints and uncertain-action reconciliation remain available.
- The host journal binds immutable actions to workspace/session and normalized
  arguments, saves results before acknowledgment, and never repeats input on a
  duplicate. Changed bindings fail; missing results remain uncertain. Session
  closure clears result payloads while preserving duplicate-prevention records;
  a new session clears payloads left by an unclean prior session. Packaging now
  includes the journal module. The host journal is not part of service rollback.
- Known duplicate/uncertain host outcomes now give review guidance rather than
  suggesting a reinstall. Added draft copy for all nine locales; speaker review
  has not been performed.

Threat boundaries retained: model/page content is not authority; local consent
is independent of service authorization; process/window identity is not a title
or PID; an unknown click is not read-only; protocol 1 cannot grant native input;
private framed IPC is not a host network-control endpoint. Driver implementations
must still prove target-addressed scope enforcement and independent local Stop.
These contract tests do not prove an OS application sandbox or perfect redaction.

Validation recorded for this checkpoint:

- Windows `go test ./...` passed. Focused tests also cover migration rollback,
  corrupt activity refusal, slow-subscriber isolation, expired control consent,
  exact step bindings, malformed worker frames and duplicate dispatch recovery.
- WSL/Linux race tests passed for computer, agents and server.
- All 17 companion tests passed, including real managed-Chromium fixtures.
- All 35 desktop unit tests passed; these are not installed-native-driver tests.
- All 91 browser UI tests passed after fixing an identity negotiation regression
  exposed by the first full run (85 passed / 6 failed). Missing capability lists
  no longer hide Agents; replay is used only when explicitly advertised. Most UI
  tests use HTTP fixtures and do not constitute real-service/model qualification.
- TypeScript, generated OpenAPI drift and UI production build passed. Vite still
  reports the existing large-chunk warning; bundle optimization is not claimed.

No macOS/Linux desktop worker, native model workflow, installed companion update,
local vision profile, trusted-proxy routing or new package qualification was tested
because those implementations are not part of this checkpoint.

Program work still outstanding (do not mark the program complete):

1. Integrate protocol 2 with a Go companion/supervisor, durable bounded-step
   approvals/action lifecycle, actor/request-ID job submission, secure persistent
   enrollment/revocation, and locally enforced Pause/Take over/Stop.
2. Implement and test real Windows UIA/WGC, macOS AX/ScreenCaptureKit and Linux
   AT-SPI/portal/EIS/X11 workers. No native worker has been delivered here.
3. Add explicit trusted-proxy mode, consented multi-origin browsing, optional
   remembered profiles, staged transfers, and race-safe folder-scoped trash/files.
4. Implement typed images end-to-end, protected-region capture controls, qualified
   local vision profiles, coordinate approval and independent artifact oracles.
5. Complete no-terminal native setup and CLI lifecycle, generated Python contracts,
   full v2 job cutover, standalone companion packaging and verified offline repair.
6. Qualify all advertised platforms/models together, including installed packages,
   signing/notarization, independent security review, the 30-case repeated workflow
   matrix, the 72-hour soak and the 30-day everyday-user pilot.

No commit, push, publication, live installation update, container replacement,
VPN change, or WSL restart was authorized or performed for this checkpoint.

### Authorized local container replacement — 2026-09-21

Following the separate request to replace the container, built the current working
tree (including new untracked implementation files) through `application-artifacts`
and reused the unchanged local CUDA/llama runtime. This is a local development
deployment, not a new release or native-computer qualification.

- Active container: `offgrid`; image
  `offgrid-llm:computer-foundation-gpu-20260921`, image digest
  `sha256:d331c882c993d4ae2ea1ad4de737ef6aec78c1acf2f8f2d59a6d023d51a2a095`.
- Service reports version `0.4.11`, revision
  `af273c6-worktree-computer-foundation`, and `durable-agent-events-v2`.
- Stopped-workspace backup:
  `/home/phil/offgrid-computer-backup-1TfUvFIK/workspace.tar.gz`;
  SHA-256 `eb307f83ab5fffbcea77a962a9de1fa77885e5269dce68b91e7d94ef7a812dbe`.
  Container configuration and isolated migration copy are retained beside it.
- Previous container retained stopped as `offgrid-before-computer-1789963680`.
  **Rollback requires restoring the matching schema-1 workspace backup before
  starting that old binary. Do not start it against the migrated live volume.**
- Isolated schema-1 to schema-2 migration passed integrity and foreign-key checks.
  All 34 stored task records (13 visible tasks and 21 deletion tombstones) retained
  their identities, ownership and saved outcomes. Live task API matches the 13
  visible records; deleted history was not resurrected. All four models remain
  available and the workspace identity is unchanged.
- Live job snapshots, SSE reconnect with a current cursor, and explicit snapshot
  recovery for an expired/ahead cursor passed read-only checks. No browser session
  was active at cutover. No model generation or host input was performed for these
  deployment checks.
- `/health` and `/ui/` are reachable from Windows and WSL at localhost port 11611;
  Docker reports healthy. Served UI build identity matches its index hash:
  `06307e339bcf8146bc990a08a8a6510586fdcc3135761da1355b1db1b8fa40c7`.
  JavaScript bundle is `index-DYK2rckN.js`.

Models, runtime settings, local port binding and GPU device access were preserved.
No desktop/host-companion installation was changed: container replacement cannot
deliver host journal updates or native/vision drivers. No commit, push, release,
VPN change or WSL restart was performed.

### Real-site browser repair checkpoint — 2026-09-21

This checkpoint addresses real browsing blockers, not completion of native
Computer Tasks. The user explicitly approved a public HTTPS test through the
existing VPN. No VPN configuration, live container, installed desktop, tags or
release artifacts were changed.

Implemented:

- Desktop and developer companion accept full public HTTPS page URLs (including
  paths, query strings and fragments), while service pairing remains origin-scoped.
  Embedded credentials and literal/local destination names are rejected.
- Direct networking is still the default. An advanced desktop network selector
  offers trusted-VPN routing, with an additional native consent disclosure.
  That mode permits only the selected host's `198.18.0.0/15` fake-DNS mapping;
  it does not grant arbitrary private IPs or other origins. TLS verification is
  retained. The operator trusts the VPN's hidden routing, not independently
  verified public-IP isolation. There is no automatic fallback into this mode.
- A loopback HTTPS CONNECT relay pins the selected destination and rejects other
  hostname/port pairs, plaintext HTTP and loopback proxy bypass. This closes the
  redirect boundary missed by route-callback-only enforcement (see the upstream
  [Playwright routing behavior](https://playwright.dev/docs/api/class-page#page-route)).
  Stopping the companion closes its owned relay sockets.
- Observed links expose exact in-scope destinations. Bounded read-only retries
  handle hydration during observation; no click, edit or submission is retried.
  Blocked subresource origins and truncated-control indicators remain visible
  to the agent so it can report incomplete pages rather than invent missing data.
- Known network errors give specific recovery guidance instead of reinstall
  advice. Shared desktop/web strings cover all nine locales; speaker review
  remains outstanding.

Evidence:

- 21 companion tests passed, including Chromium redirect/private-subresource
  rejection, DNS policy, stale input, journal recovery and bounded observation
  retries. The self-signed key/certificate under `computer/test/fixtures` are
  public test material only; TLS bypass exists only in the owned fixture test.
- 36 desktop unit tests passed, including local consent refusal for trusted VPN,
  invalid-mode rejection, no arbitrary proxy arguments and private IPC forwarding.
- All 92 UI tests passed. The initial 90/92 run exposed two incorrect selectors
  in the added test cases; those were corrected and the entire suite rerun.
- Focused Go tests for computer, agents and server passed; OpenAPI drift,
  TypeScript and production renderer build passed. The existing bundle-size
  warning remains.
- Built and verified the Windows bundled companion runtime. Real Electron
  utility-process integration passed the owned demo and, separately with explicit
  operator consent, opened `https://playwright.dev/docs/intro`, discovered the
  Writing tests link, navigated to `/docs/writing-tests`, verified page text and
  shut down the owned browser. VPN remained enabled and HTTPS checks were not
  disabled. Evidence: `build/computer-public-check.log`; isolated test profile
  `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-browser-f9Q7FO`.
  Repeat with Electron and `dev/scripts/test-computer-desktop.cjs
  --public-docs-trusted-vpn` only after consciously approving that network trust.

These tests used a deterministic tool dispatcher, not a model planner; they do
not qualify autonomous task quality. No new installed-app installer was tested.
Native OS drivers, local vision, bounded-step approvals, multi-origin resource
consent, remembered login, downloads/uploads and typed filesystem operations
remain unfinished. Explicit HTTP/SOCKS proxy configuration and IPv6-only public
sites remain unsupported. Do not claim general computer use or a 9/10 product
from this checkpoint. The matching host runtime and application must be deployed
together under separate approval; a container-only update cannot deliver these
host changes.

### Native Windows implementation checkpoint — 2026-09-21

Implemented actual native control rather than extending the browser demo:

- An unprivileged `cmd/offgrid-computer` Windows x64 worker calls Windows UI
  Automation through Go COM bindings. This is an implementation adjustment from
  the proposed C++ worker: it retains the existing Go toolchain and process
  isolation without introducing a compiler/SDK prerequisite for end users.
- Discovery binds opaque target/control identities to OS session, process
  creation time and selected window. Dispatch checks process ownership,
  non-elevation, unlocked desktop, focus, UIA ancestry and fresh control state.
  Supported mutations are ValuePattern text replacement and InvokePattern
  activation. Invocation is reported as dispatched, not proof of task completion.
- Exact local consent and bounded-step grants cannot be supplied as a model or
  renderer boolean. Approvals display the locally prepared control and exact
  proposed value, rather than trusting a model-written action description.
- The host-local SQLite journal durably claims action IDs before dispatch and
  records results before reply. Changed arguments, incomplete claims and corrupt
  or newer stores fail closed. Identical completed IDs replay results only;
  failed actions cannot become success on replay. The journal uses the existing
  storage policy and exclusive ownership; it is not a second service task store.
- A separate Windows message thread provides a visible Stop window and registers
  Ctrl+Alt+Shift+F12. Revocation is independent of the service/model. A watchdog
  exits a hung owned worker after two seconds; an in-flight journal claim remains
  uncertain. No target application is killed. Native IPC retains ownership until
  process exit and refuses new work after an unconfirmed stop.
- Windows desktop runtime packaging builds the worker, includes the private
  `native-worker.cjs` bridge and covers these files with the existing pack digest.
  Public capability flags remain unchanged. CI now includes isolated Windows
  UIA/worker tests; tests are serialized to avoid competing desktop controllers.

Recorded evidence:

- A real owned Win32 application accepted an approved Unicode text replacement
  via UIA. Independent `WM_GETTEXT` checks matched the requested content. Marked
  password controls were excluded; manual changes invalidated prepared actions.
  A repeated test encountered a focus change and correctly received a scope
  rejection rather than a stale-state code. The fixture now explicitly restores
  its own focus before checking stale-state rejection; the serialized suite passed.
- A separate real worker process passed framed IPC, both actual local consent
  dialogs, approved native editing, independent result checks and stop/exit.
  Test-only automation confirmed dialogs belonging to that owned worker and the
  exact disposable document. Production contains no automatic-consent switch.
- The local Stop button, close control and registered hotkey message path passed
  without service/model connectivity. A physical keyboard/layout qualification
  and installed-app accessibility review remain separate gates.
- All Go packages passed on Windows; Linux/WSL race tests passed for computer,
  agents and server. Companion tests passed (26) and desktop unit tests passed
  (36). Focused native journal tests cover reopening, uncertain dispatch, failed
  persistence, duplicate actions, concurrent ownership and stop admission.
- Built and verified `build/computer-runtime/win-x64`. The real bundled executable
  launched through the private bridge, discovered targets and exited on Stop.
  That package smoke test selected and modified no user application.

Not complete: the Agents service/session/tool adapter and end-user native setup,
cross-platform native drivers, vision, file/download/upload workflows and broader
origin consent. HWND replacement/scope-edge cases, application-specific effects,
provider hangs, accessibility and privacy require more adversarial qualification
before exposing native mutations. A local unit/fixture pass does not satisfy the
30-workflow, model-planning, signing, soak or pilot gates. No native capability is
advertised as production-ready, and no existing container/installation, release,
tag or remote commit was changed at this checkpoint.

### Cross-platform native review checkpoint — 2026-09-21

The private worker core is now shared by Windows x64, macOS Intel/Apple Silicon,
and Linux x64. macOS uses a small Objective-C Accessibility shim and an AppKit
consent/Stop process; Linux uses libatspi with a GTK consent/Stop process. This
adjusts the planned Swift/C++ implementation languages without replacing Go
or introducing script execution. Native control is still not wired into public
Agents sessions, and the runtime manifest explicitly reports `qualified: false`.

Local validation:

- Real Windows UIA edit, actual private worker/consent IPC, independent Win32
  result verification and Stop tests passed from an interactive terminal.
  Running from noninteractive pipes could not obtain foreground permission and
  correctly refused mutation. Focus activation now waits briefly for Windows'
  asynchronous transition; it never bypasses foreground restrictions.
- In an isolated unprivileged Ubuntu 24.04 container with Xvfb/Openbox and a
  private D-Bus session, the real AT-SPI fixture passed discovery, an approved
  Unicode edit, an independent GTK read and stale-state rejection. The Linux
  worker and GTK consent helper compiled. This is not GNOME/KDE/Wayland or
  installed-package qualification. The fixture required explicit GObject linkage;
  the build now declares it rather than relying on transitive linker behavior.
- All ordinary Go packages passed on Windows. Companion contracts passed (30),
  desktop contracts passed (36), and four additional platform-specific pack tests
  rejected tampering and invented native/vision qualification. API drift,
  TypeScript, renderer build and workflow syntax/release-CI checks passed.
- The first full UI run passed 91/92: an Arabic locale test measured before its
  font was ready. Its geometry baseline now awaits `document.fonts.ready` while
  retaining exact containment/position assertions. The complete 92-test rerun
  passed with four workers and no retries.

Review-branch CI is explicitly authorized, including macOS Intel/Apple Silicon
compilation and installed-bundle startup checks. No release, tag movement or
live installation/container replacement is authorized by this checkpoint.
macOS permission-boundary tests do not modify TCC or pretend a hosted runner has
user consent. Native app mutation qualification on macOS remains a separate gate.

Remaining gaps include the shared service/client native-session cutover,
end-user enrollment/permission setup, Linux global emergency shortcut and portal
capture/input, all local vision, typed files/downloads/uploads, and multi-origin
browser workflows. Source and build coverage alone cannot establish model task
quality, OS permission usability, privacy guarantees, signing, soak or pilot gates.

### Native task transport and runner integration — 2026-09-21

This checkpoint implements an internal protocol-2 path through the existing
computer queue and durable agent runner. It does not complete application control
and does not enable a native picker in the released UI.

- Native enrollment binds a host-issued OS/session/process/window identity.
  Browser-v1 sessions cannot receive native tools; native sessions cannot receive
  browser tools. Both share the same active-session queue. Actor-scoped listing
  and execution retain the selected driver without exposing another user's target.
- `native-session.mjs` translates typed agent calls into private native-worker
  requests. It verifies workspace identity, uses the actual worker target list,
  binds one task, obtains local consent, prepares exact actions locally, requests
  local approval, and journals results before acknowledging them. Credentials
  never pass through renderer APIs or command-line arguments.
- The durable runner persists native tool-call approvals before dispatch and
  requires a real first observation. Native-model preflight uses the actual native
  tool schemas and unpredictable observed IDs, not the browser-only probe.
- Native operations remain limited to structured observation, text replacement,
  and control activation. Control values and screenshots are not returned by this
  profile. Native task completion deliberately remains unavailable until a real
  outcome verifier can distinguish editing from saving/submission. Recorded
  action evidence remains inspectable; an invocation cannot imply task success.
- Native capabilities are labelled `native_development`, never qualified. The
  existing browser picker filters native sessions instead of mislabelling or
  automatically selecting them. The native transport ships in build-time packs,
  but no new terminal-dependent end-user workflow or incomplete native button is
  introduced.

Validation at this checkpoint:

- Native transport tests cover all three driver identities, local consent and
  approval refusal, actor/task/target replacement, stale observations, unexpected
  fields/tools, reply replay without duplicate input, cancellation during startup,
  and unconfirmed Stop without false success. These are contract tests with a
  simulated worker, not macOS/Linux desktop qualification.
- Go tests cover native HTTP enrollment and renderer-origin rejection, immutable
  driver bindings, native model-call probes, and a durable runner that observes,
  reaches persisted approval, and does not dispatch a mutation before approval.
- The real Windows UIA and private-worker tests passed again against owned Win32
  fixtures: Unicode editing, independent Win32 verification, marked-password
  exclusion, consent, and local Stop. This does not yet test the complete new
  service-to-JavaScript-to-native pipeline with a real planning model.
- Existing computer/agent/server Go tests and the companion suite passed.
  Generated TypeScript API schemas and type checking passed.

Remaining implementation: value/document observation with bounded privacy
controls, independent application outcome verification, broader structured
actions and validated keyboard/pointer input, existing-browser attachment,
vision, file operations, persistent enrollment, packaged end-to-end tests on
all platforms, and the remaining qualification gates. Desktop target-picker,
shared host controller, and catalog-based app launching are now implemented in
the native preview but are not yet platform-qualified for release.
The prior Windows hosted-runner privilege mismatch is not resolved by these
transport changes. No live container, installed application, tag, or release was
changed at this checkpoint.

### Native application-selection checkpoint — 2026-09-21

The desktop bridge now exposes a real application-selection path for the
native preview. In the trusted desktop main process it catalogs user-launchable
Windows Start Menu shortcuts, macOS applications, and Linux desktop entries,
returns opaque IDs, and launches only a selected catalog entry. Renderer code
cannot provide an executable path or command. After launch, the companion
re-discovers the actual application windows and pairs the selected window using
the existing protocol-v2 target identity and approval journal. Existing browser
selection remains available as a separate mode.

Desktop tests cover catalog parsing, opaque-ID enforcement, and shell-free Linux
launch; the web type-check/build and native picker interaction checks pass.
This is not evidence that every installed application is controllable: native
drivers still expose only their currently qualified structured operations
(observe, bounded text replacement, activation and read-back verification).
Vision capture/actions, file upload/download/trash workflows, broader control
patterns, persistent OS-keystore enrollment, and real macOS/Linux desktop
qualification remain release gates. Users must still grant OS accessibility
permissions and select a target window; unsupported or elevated applications
fail closed.

### Native checkbox, shortcut and browser-transfer checkpoint — 2026-09-22

- Managed Chromium now stages downloads privately with size and SHA-256 evidence,
  and accepts uploads only through a trusted desktop file picker and an opaque,
  digest-bound grant. The renderer and model never receive the host path. Neither
  transfer operation submits a form or executes downloaded content.
- Native selected-window drivers now expose a fixed shortcut vocabulary and
  explicit checkbox state. Both bind to a freshly observed, non-protected control,
  require exact service and local approval, revalidate target/focus/state before
  dispatch, and require a fresh independent state check for task completion.
  There is no arbitrary chord, key string, pointer coordinate, or shell surface.
- The Windows UIA fixture independently observed Ctrl+S through its window
  message queue and checkbox state through `BM_GETCHECK`. The isolated Ubuntu
  X11 fixture independently observed Ctrl+S and GTK checkbox state. An AT-SPI
  modifier-mask defect found by that fixture was corrected; the control mask is
  now passed as a bitmask rather than an enum ordinal.
- Hosted Windows runners that execute elevated now record the native mutation
  fixture as a policy skip. OffGrid continues to reject elevated targets in
  production; CI does not weaken that boundary merely to obtain a green check.
  The same fixture passes on the normal-user Windows development desktop.
- The same closed action now includes bounded navigation keys (Enter, Escape,
  Tab/reverse-Tab, arrows, Page Up/Down and Home/End) on Windows, macOS and X11.
  These remain selected-control, foreground-validated, separately approved
  dispatches; arbitrary key strings, global typing and Wayland input are absent.
- Browser/companion tests pass 49 cases; desktop contracts pass 44; focused Go
  computer/server/agent suites, real Windows UIA/Stop tests, real Linux AT-SPI,
  TypeScript checks, and the production renderer build pass locally.

This evidence does not qualify macOS input, GNOME/KDE Wayland portals, vision,
whole-desktop input, native uploads/downloads/trash, arbitrary applications, or
model-planned end-to-end workflows. Native manifests remain `qualified: false`.

### Bounded managed-browser vision transport checkpoint — 2026-09-22

- Chat content now has a typed, bounded text/image contract. OffGrid accepts only
  local base64 PNG/JPEG parts, validates encoding, format, dimensions and byte
  limits before inference, and rejects remote/file URLs. Image messages require
  an installed projector; the service returns stable
  `invalid_message_content` or `vision_projector_unavailable` errors before
  sending unsafe input to llama.cpp.
- Managed Chromium can capture only the current, freshly observed viewport.
  Password, payment-autocomplete, one-time-code and explicitly private controls
  are masked before bytes leave the companion. The companion uploads the capture
  through its authenticated action-bound transport, not through renderer APIs.
- The service keeps image bytes only in a bounded in-memory record tied to actor,
  run, browser session and pending action. Durable tool steps contain an opaque
  reference. The reference is consumed only by the immediately following model
  turn and cannot be replayed across actors, sessions or runs.
- An installed projector is no longer treated as enough evidence to expose the
  capture tool. OffGrid generates a bounded synthetic PNG containing a random
  hexadecimal code, sends it through the same typed multimodal path, and requires
  exactly one governed tool call containing the code read from the image. A pass
  is held only in memory and is bound to the current model/projector files; file
  replacement or service restart invalidates it. Failed vision checks disable
  capture without blocking structured browser control.
- Model listings still report `vision: unknown`, distinct from tested support.
  The smoke check proves local image transport plus one exact tool call, not
  grounding accuracy or end-to-end task quality.

Local evidence: API content-validation, deterministic synthetic-image, exact
vision-tool-call and server policy tests pass; the browser suite passes 50 cases
including real Chromium capture, masking stability, stale observation rejection
and transfer workflows. This checkpoint does **not**
qualify grounding accuracy, a specific VLM/projector/runtime tuple, native-window
capture, coordinate actions, macOS/Linux capture APIs or general computer vision.

### Session-scoped approval policies — 2026-09-22

- Computer sessions now bind one immutable policy selected before local consent:
  `ask_every_time`, `scoped_changes`, or `full_task`. Existing sessions and
  checkpoints without a recorded policy retain exact-action approval; they are
  never upgraded silently.
- Action risk is classified by the host driver from the freshly observed local
  control and target context. Model arguments cannot supply an approval class.
  The durable service applies the policy before dispatch, and the independent
  companion recomputes and enforces it immediately before browser or native input.
- Scoped mode permits only read-only and reversible actions automatically. Full
  task mode additionally permits consequential actions exposed by the typed
  driver. Both modes still block credential/payment controls, financial actions,
  privilege or security changes, software installation, scripts, permanent
  deletion, stale actions, and repeat dispatch after an uncertain outcome.
- Exact approvals remain actor/run/call/arguments/capability bound and expire as
  before. Automatically authorized actions are marked with their session policy
  in durable task steps so history and support evidence distinguish them from
  explicit approvals.
- Desktop and browser setup show the policy before pairing and include it in the
  local consent summary. The current policy is visible in the connected-session
  selector and activity strip. Changing the selector requires a new session;
  it cannot mutate authority for work already underway.

Evidence at this checkpoint includes policy-matrix and classifier unit tests,
browser and native companion enforcement tests, service integration tests for
automatic reversible work, exact consequential approval, full-task submission,
and non-overridable prohibited actions, plus desktop IPC and browser UI contract
tests. These controls reduce approval fatigue; they do not qualify general native
computer use or remove the remaining platform/model release gates above.

### Task-first agent execution and architecture review — 2026-09-24

The working tree adds task-first submission through `/api/v2/jobs` to the existing
durable runner. An actor-scoped request ID deduplicates submissions, including
lost acknowledgments and deleted-task tombstones. A model's request for computer
access persists a `waiting_for_input` checkpoint before returning control to the
user. Locally approved access resumes that same task; it never submits a second
task or grants authority from a model-proposed application name. Its first
computer operation must observe the selected target. Schema 2-to-3 migration
creates and verifies a SQLite backup and recovery manifest before activation.

The shared renderer now leads with the task, defers access controls until needed,
and separates new drafts from previous results. Desktop handoff carries only the
saved task ID, not credentials, task text or executable actions. Older services
retain the compatibility view. Native runtime failures do not prescribe browser
repair. The existing monochrome tokens and typography are retained: a discovered
CSS layer-order regression was corrected and guarded by light/dark computed-style
tests, including navigation links. This is not a new branding palette.

Local validation:

- Windows: `go test ./...` and a subsequent focused run of agents, computer,
  server and CLI packages passed.
- WSL/Linux: `go test -race ./internal/agents ./internal/computer ./internal/server`
  passed.
- Web: TypeScript, generated API drift check and production build passed. The
  build still reports the existing large-JavaScript-chunk warning.
- Browser contracts: 35 task-first, streaming and interaction tests passed,
  including nine-locale layout, RTL, reconnect, request deduplication and shared
  palette checks. These use API/desktop fixtures, not installed-OS or model-task
  qualification.
- Desktop: 15 computer-runtime tests passed, including strict ID-only deep links.

Architecture gaps found in this review remain explicit:

- The legacy workflow and multi-agent libraries use in-memory execution state
  and `RunImmediate`, not durable child jobs. The old workflow registration route
  also acknowledged work without registering it. Their production HTTP adapters
  now report unavailable rather than executing a second, non-durable authority
  or falsely acknowledging registration.
- The new UI uses task-first submission; CLI computer submission still uses
  the legacy route and requires a session ID. Existing commands can inspect,
  cancel and recover shared tasks, but first-use/client lifecycle parity is not
  complete. Approval/recovery mutations also still use compatibility routes.
- The durable tool loop has iteration/time limits but no integrated token-aware
  compaction, durable hierarchical work plan or parent/child budget accounting.
  Other memory/workflow modules are not evidence of integration into this runner.
- A connected task receives one scoped computer toolset. Multi-application
  handoff, general follow-up steering, and a complete shared pause/takeover flow
  require further work; removing setup fields does not implement those behaviors.
- Browser/native verification checks bounded application evidence. They do not
  constitute general independent document/spreadsheet/task outcome verification.

The target architecture and required coordinator invariants are recorded in
[ARCHITECTURE.md](ARCHITECTURE.md#bounded-durable-coordination).
Keep one authoritative runner, SQLite persistence, governed tools and shared
inference admission. Add coordination only through durable scoped child jobs;
do not equate extra agents, reasoning-style labels or another framework with
measured task quality. This checkpoint does not qualify general native/vision
control, multi-agent workflows, signing, installed upgrades or the pilot gates.
No live container/desktop replacement, commit, push or publication is included.

### Durable task lifecycle and bounded coordination follow-up — 2026-09-24

This follow-up supersedes the implementation gaps listed at the preceding
checkpoint where specifically described below; it does not qualify the entire
native-computer or research program.

- Web, desktop renderer and CLI use the same `/api/v2/jobs` submission and
  lifecycle commands. Pause, follow-up instructions, resume, takeover, stop,
  reconnect and evidence export operate on the saved job. Actor-scoped request
  IDs prevent duplicate submissions and duplicate follow-up instructions.
- Computer handoffs remain in the same job, require verification of the current
  target and fresh local consent for the next target, revoke the previous
  session, and require a fresh observation. Uncertain input is never replayed.
- Model-reported work plans reference recorded steps. Context management archives
  complete older call/result groups with digests, retains user instructions and
  exposes bounded history retrieval. Token estimates are labelled and use the
  allocated window, not a model's advertised maximum.
- Bounded read-only child jobs and dependency edges share the existing runner,
  admission control and SQLite transaction. Children have pinned tool grants,
  fixed iteration/depth/count limits, no computer authority and no shell/write
  capabilities. Failed branches cannot silently become successful parent jobs;
  restart recovery requires explicit resume. The old workflow engine remains
  unavailable rather than becoming a second execution authority.
- Typed workspace artifacts support UTF-8 text, Markdown, JSON and CSV. Stored
  bytes are reread, hashed and parsed before evidence is returned. Downloads
  require a committed owner-task reference; knowing a digest is insufficient.
  The CLI verifies downloaded bytes before creating a new local file and refuses
  overwrites. Format/integrity checks do not establish factual correctness or
  prove that an external document application saved a file.
- Schema 4 migration preserves a verified prior-schema database and recovery
  manifest. Older binaries must not use the migrated workspace. The task-first
  route cannot fall back to arbitrary shell execution.

Validation for this follow-up:

- Windows full Go suite and subsequent focused agents/server/CLI tests passed.
- WSL race checks passed for agents, server, computer and artifact packages.
- TypeScript, generated API drift check, production UI build and 15 desktop
  runtime tests passed. The existing large-JavaScript-chunk warning remains.
- 39 browser-contract tests passed (10 task-first, 3 streaming, 26 interaction),
  including light/dark monochrome tokens, responsive layouts and locale cases.
  These fixtures are not installed-platform qualification.
- A packaged isolated service using the installed Qwen2.5 7B Q4_K_M model,
  upstream runtime from the existing GPU image, explicit 8192-token context and
  q8_0 KV cache passed `dev/scripts/test-task-jobs.mjs`: exact CSV bytes/digest
  and parsed dimensions, deduplicated submission, two calculator children with
  dependency ordering and independently checked result 90, and durable native
  access interruption without dispatch. Synthetic history remains isolated.
  An earlier adaptive 2048-token fixture correctly refused an oversized context
  before executing tools; this is not silently retried with a larger allocation.

Still outside this evidence: arbitrary reusable workflow registration, recursive
agent teams, multi-model scheduling, adjustable graph-wide token/wall-time
budgets, concurrent computer controllers, general semantic outcome verification,
native/vision platform qualification, signing, installed desktop upgrades and
pilot/soak gates. The real-model smoke check establishes these bounded execution
paths, not general model-planning reliability. No commit, push or release is
included in this checkpoint.

Authorized local deployment at this checkpoint:

- Replaced `offgrid` with `offgrid-llm:task-runtime-gpu-20260924`, revision
  `2c75c51-worktree-task-runtime-20260924`; the existing release version remains
  0.4.11 (this is a local working-tree build, not a published release).
- Saved and checksum-verified the stopped workspace at
  `/home/phil/offgrid-computer-backup-78d0tazy/workspace.tar.gz`, with private
  container configuration and a separately validated migration clone alongside.
- Kept `offgrid-before-computer-1790225789` stopped for rollback. The isolated
  real-model probe and migration-check containers are also stopped; exactly one
  OffGrid container is running on port 11611.
- Verified health, matching UI build identity, preservation of the pre-upgrade
  19 task IDs/statuses and four model IDs/sizes. Later user-created tasks are not
  part of that migration baseline. Models remain in their existing volume.
- The opt-in read-only installed-UI Playwright check passed against the replaced
  service: task-first composer, existing history/results, mobile width, no
  upfront browser-pairing control and no JavaScript runtime errors. No synthetic
  task was submitted into the live workspace. Desktop host binaries were not
  replaced by this container deployment.

Post-deployment user testing exposed an additional correctness issue: a built-in
`list_files` call against `d:\\` ran in the Linux service environment and returned
`no such file or directory`. The durable runner categorizes all executor errors
after intent persistence as uncertain, including this known read-only failure,
and presents manual outcome reconciliation. This is not evidence of a lost
mutation acknowledgment. Repair needs effect-aware typed tool errors and clear
service/host filesystem routing; ordinary read failures must not require invented
human verification. Genuine uncertain mutations must retain no-replay protection.
This issue is diagnosed, not repaired or qualified by the tests above.

### Read failure recovery and history management repair — 2026-09-24

This repair supersedes the immediately preceding read-error diagnosis:

- Only pinned, service-owned read-only built-ins carry a durable no-side-effect
  intent marker. Missing files, denied reads and invalid calculations fail with
  stable safe errors, not an uncertain outcome or a request for invented human
  verification. Pause/cancel/restart recovery preserves that distinction. A
  legacy checkpoint, user/MCP replacement or merely low-risk descriptor is not
  sufficient evidence to classify a tool as read-only.
- Recognizable foreign desktop paths in task-first filesystem calls pause for
  local file-manager access before dispatch, after checking tool authorization.
  The service does not translate paths, expose host mounts or infer consent.
  Read-only children cannot obtain computer authority. Access-granted results
  explicitly contain no directory contents; a fresh observation is required.
- Agents and Activity expose individual confirmed deletion and bulk removable
  task cleanup using the same `/api/v2/jobs/{id}` deletion authority. Active work,
  uncertain effects and referenced child records remain protected. Local-owner
  cleanup of unowned legacy history uses the same rules. Deleted projections do
  not reappear after restart; audit records and backups are intentionally retained.
- Companion system audit notifications no longer appear as a perpetually
  running task. Tools and Connections share top-level Agents navigation and
  responsive management panels; the existing monochrome tokens and locale
  resources are preserved. Raw uncertain-call arguments are disclosure details,
  not the primary content of the exceptional recovery screen.

Evidence: full Windows Go tests, focused agents/server regressions, WSL race
checks, TypeScript/API drift checks, UI build and 42 browser-contract tests
(10 task-first, 3 management, 3 streaming, 26 interaction) passed. Tests include
real built-in filesystem failures, restart markers, protected history, legacy
deletion, and confirmed cleanup from both UI views. Browser fixtures verify
management placement and mobile width; they do not qualify native control or
general model-planning reliability. The existing large UI bundle warning remains.

Historical uncertain calls without trusted effect metadata are not rewritten as
successful reads. This repair does not add unrestricted desktop filesystem access
or independently verify previously user-reconciled results.

Authorized local deployment of this repair:

- `offgrid-llm:recovery-ui-gpu-20260924`, revision
  `2c75c51-worktree-recovery-ui-20260924`, is healthy on loopback port 11611.
  The local build retains version 0.4.11 and reuses the existing native runtime;
  it is not a published release or an installed desktop-host update.
- The stopped workspace backup is
  `/home/phil/offgrid-computer-backup-4MN11JBa/workspace.tar.gz`; its checksum
  passed. Private configuration, clone-validation results and activation
  snapshots are in the same directory. Rollback container
  `offgrid-before-computer-1790229860` is stopped. Only one OffGrid container
  runs; the validation clone was stopped before live activation.
- Clone and live activation snapshots contained all 24 pre-update task IDs with
  unchanged statuses. The four model IDs and sizes were preserved. Subsequent
  authenticated-local deletion requests removed finished history; those later
  changes were not undone by validation. The backup retains the earlier records.
- Two read-only installed-UI checks passed against the actual replacement:
  desktop/mobile composer and saved-task inspection; Tools/Connections panels;
  Activity deletion metadata parity and absence of the phantom system task.
  Confirmation dialogs were inspected without confirming deletion, and mutating
  browser requests were blocked in the management smoke test. Screenshots were
  visually reviewed. No synthetic task or model request was submitted by these
  live checks. No commit, push or publication is included.

### Pre-release review repairs — 2026-09-24

The five findings from the task-first review are addressed in the working tree:

- Access-panel and task-toolbar cancellation use scoped desktop IPC. Pending
  discovery/startup is bound to the saved input request; connected control is
  matched by session ID in the desktop main process before revocation. A stale
  setup panel cannot stop a replacement task. Late host replies cannot attach
  access after explicit cancellation. Service revocation uses the actor-scoped,
  idempotent `/api/v2/computer/sessions/stop` endpoint, not global emergency stop.
  Global emergency stop remains a separate safety action. Older desktop bridges
  receive an update-required explanation before task-first setup starts.
- Delegated grants require known built-in source, namespace, kind and risk,
  both at creation and dispatch. Pre-existing user replacements with permitted
  names cannot acquire read-only child authority. Existing unsafe child grants
  fail closed. Governed HTTP GET remains network access, not evidence that an
  interrupted request was side-effect-free.
- Task-first browser input now performs the existing installed-profile vision
  check. Valid cached passes are reused; missing/failed checks leave only
  structured controls enabled. A service restart requires a fresh check.
- Coordinator ticks collect eligible IDs/actors/statuses under the manager lock
  without copying completed histories, checkpoints or context archives. A
  regression fixture with large historical payloads checks zero idle allocations.
- Snapshots expose `can_steer` using the same saved-state predicate as instruction
  submission. Completed subtasks no longer hide follow-up editing; active
  delegation and uncertain actions still prevent it. Dispatch rechecks worker
  settlement and actor ownership.

Validation includes the Windows Go suite and focused uncached tests, WSL race
checks for agents/server, TypeScript and generated-contract drift checks,
production UI build, desktop runtime tests and shared UI regressions. The vision
test uses a test-only reader of the synthetic PNG challenge, not a production
fake engine or evidence of real-model grounding quality. Browser tests use
isolated API/IPC fixtures. The pre-existing large UI bundle warning remains.

These repairs do not qualify native/vision workflows on additional platforms,
installed upgrades, signing, or the soak/pilot gates. Matching service/renderer
and desktop-host builds are needed to deploy scoped cancellation. No commit,
push, publication, live container replacement or installed desktop update is
included in this repair.

Authorized matching local deployment of these repairs:

- Activated `offgrid-llm:review-repairs-gpu-20260924`, revision
  `2c75c51-worktree-review-repairs-20260924`, retaining local version 0.4.11
  and the existing native inference runtime. The stopped workspace archive at
  `/home/phil/offgrid-computer-backup-nqHoQDV3/workspace.tar.gz` passed SHA-256
  and archive-to-source comparison. Private configuration and clone/live checks
  are alongside it. Rollback container `offgrid-before-computer-1790234470`
  remains stopped; exactly one OffGrid container runs on loopback port 11611.
- Activation preserved the two pre-deployment task IDs/statuses, workspace
  identity and all four model IDs/sizes. Three additional completed records
  appeared after activation and were left intact; deployment checks do not
  create synthetic tasks or roll back subsequent client activity.
- Updated the installed Windows desktop at
  `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop`. The previous
  application remains in the adjacent
  `OffGrid LLM Desktop.rollback-review-repairs-20260924` directory. Verified
  data/settings, Electron profile and companion-journal backups are in
  `build/desktop-backups/before-review-repairs-20260924`; models were untouched.
  Backup restoration must not roll back the companion duplicate-action journal.
- Staged and installed Electron executables passed read-only startup checks
  against the new container with isolated test profiles: matching build identity,
  no duplicate service, task-first UI and the scoped Stop IPC. Two live browser
  UI checks and 18 desktop runtime tests passed. Installed browser/native pack
  digests passed; the service reports healthy with zero restarts. The installed
  smoke evidence is in
  `C:\Users\phil\AppData\Local\Temp\offgrid-review-deployment-j6xS0O`.
- This is an unsigned local Windows build, not a signed-publisher distribution
  or a new native/vision qualification. WSL and the VPN were not restarted or
  reconfigured. No commit, push or publication is part of this deployment record.

### MCP connection removal repair — 2026-09-24

- Added an administrator-only, idempotent removal API and a confirmed per-row
  **Remove connection** action in the shared web/desktop Connections view.
  Disconnected or disabled saved entries stay discoverable and removable.
- Removal persists the configuration change before revoking the exact source's
  tool executors and capability descriptors, then closes the MCP transport.
  Configuration writes use a synced temporary file and replacement, not in-place
  truncation. Corrupt/unwritable settings block removal without discarding the
  connection or other configuration. Connect-and-save is serialized with removal;
  duplicate live names and conflicting normalized tool namespaces are rejected.
- Removed connections do not return after restart. Other connections, built-in
  tools, custom tools and task history are retained. The confirmation explicitly
  warns that tasks may fail and already dispatched actions are not undone.
- Evidence: real local Streamable HTTP handshake/tool-call/removal/restart
  regression tests; closed-session cached-executor rejection; corrupt-storage
  rollback; unavailable/disabled entries; admin/ordinary-user/unauthenticated API
  checks; focused Linux race tests. Fourteen browser tests passed, including
  removal/cancel/error/reload, existing task-management tests, all nine locales
  and narrow-screen RTL layout. TypeScript, contract drift and production UI
  build pass. The pre-existing large-bundle warning remains.
- This is source/build validation, not a live deployment or qualification of
  any third-party MCP service. No user's saved connection was removed for testing.

Authorized matching local deployment of the MCP removal repair:

- Activated `offgrid-llm:mcp-removal-gpu-20260924`, revision
  `2c75c51-worktree-mcp-removal-20260924` (local version 0.4.11), reusing the
  unchanged GPU/llama.cpp runtime. Exactly one OffGrid container runs on loopback
  port 11611, healthy with zero restarts. The previous container
  `offgrid-before-computer-1790240737` remains stopped.
- Workspace archive `/home/phil/offgrid-computer-backup-dS8WbUIB/workspace.tar.gz`
  passed its SHA-256 check and archive-to-source comparison. Clone validation
  ran with separate data while the original was stopped. Activation preserved
  all nine pre-cutover task IDs/statuses, four model IDs/sizes, workspace identity
  and the saved MCP connection, which reconnected and discovered its two tools.
- Updated the installed Windows application at
  `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop`; the previous
  application remains in sibling `OffGrid LLM Desktop.rollback-mcp-removal-20260924`.
  Data, settings, Electron profile and companion-journal copies were verified in
  `build/desktop-backups/before-mcp-removal-20260924`. Model folders were untouched.
  Restoring the workspace must not roll back the companion duplicate-action journal.
- Installed and container UI build identities match
  `b3611a157887f30657fe195f1d3cadbc1ee0900228be64424214b422a2626740`.
  Two live browser tests and an installed Electron smoke test confirmed the
  task-first workspace, Connections removal dialog/cancel and unchanged history
  without submitting tasks or deleting a user's connection. Installed smoke
  evidence: `C:\Users\phil\AppData\Local\Temp\offgrid-review-deployment-HE6iER`.
- All 49 desktop unit tests passed. Both staged and installed Electron theme
  tests passed, including main-process theme synchronization and nine readable
  option styles; installed evidence is in
  `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-theme-8usiNv`.
  This is still an unsigned local Windows build, not a published release or new
  native/vision qualification. No WSL restart, VPN change, commit, push or tag
  movement was performed for this deployment.

### Task-history spacing and release preparation — 2026-09-24

- The task list now separates each card from its delete control by 8 px and
  adjacent rows by 12 px, with scrollbar clearance. Geometry checks cover light,
  dark, mobile and Arabic RTL layouts without changing the monochrome palette.
- Authorized local deployment activated
  `offgrid-llm:history-spacing-gpu-20260924`, revision
  `2c75c51-worktree-history-spacing-20260924`. The stopped workspace archive
  `/home/phil/offgrid-computer-backup-AWijRN89/workspace.tar.gz` passed digest
  and archive-to-source verification. Rollback container
  `offgrid-before-computer-1790242118` remains stopped; only one OffGrid
  container runs on port 11611, healthy with zero restarts.
- The matching Windows application was replaced with a verified staged package.
  Its previous files remain in `OffGrid LLM Desktop.rollback-history-spacing-20260924`
  beside the installation; data/settings/profile/journal backups are in
  `build/desktop-backups/before-history-spacing-20260924`. Models were untouched.
  Workspace restoration must not roll back the companion execution journal.
- Browser and installed Electron read-only checks confirmed the current
  task-first UI and exact spacing. Both UI identities match
  `acc10841eeede7d3a630c56413def3d4946c15b0a84089ccff006aaffcf57a52`.
  Installed smoke evidence: `offgrid-review-deployment-fotdnG`; packaged theme
  evidence: `offgrid-desktop-theme-hz6aM3`, under the Windows temporary directory.
- Nine existing task records, four models and workspace identity were preserved.
  A successful `DELETE /v1/agents/mcp` at 09:30:11 after activation removed the
  saved MCP connection. Verification confirmed that subsequent change and left
  it removed; deployment tests did not send that deletion or restore it.
- Release preparation passed the full Windows Go suite, Linux race tests for
  agents/server/capabilities, 49 desktop unit tests, TypeScript, API generation
  and production UI build. The complete local browser suite passed 131 tests;
  three installed/real-service opt-in cases were skipped in that fixture run.
  Older tests now explicitly mock task-first identity/jobs and distinguish a
  task selection button from its new delete button. This changes test coverage,
  not the current application layout. Cross-platform CI remains a separate gate.
- This local package is unsigned. These checks do not establish broader native
  or vision qualification, signing, independent security review, or soak/pilot
  completion. WSL and the VPN were not restarted or reconfigured.
