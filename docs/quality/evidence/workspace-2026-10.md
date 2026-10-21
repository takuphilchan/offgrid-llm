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
