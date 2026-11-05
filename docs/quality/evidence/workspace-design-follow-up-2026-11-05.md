# Workspace design follow-up — 2026-11-05

Scope: current v0.4.25 source, following the [design audit](../../advanced/design-audit.md),
with focused changes to navigation, onboarding, document layout and CLI discovery.
This is incremental implementation evidence, not whole-product qualification.

## Behavior

| Journey | Before | After |
| --- | --- | --- |
| Saved chat model unavailable; another installed model is eligible | Model-management action was primary; the installed suggestion was secondary | Explicit selection of the named installed alternative is primary; other choices remain available |
| First-use guide | Large separate mark and eyebrow, repeated connection explanation and three status rows | Compact heading, visible service state, expandable connection explanation/model count, one primary next action |
| Short desktop navigation | Two columns with collapsed grouping | One vertical sequence; every destination retained |
| First question beside a document | Large centered welcome graphic competed with the source | Compact left-aligned heading; draft and source controls retained |
| CLI first minute | Large command catalog and fixed model examples | Installed-model, conversation and diagnostic commands first; full registry-covered catalog available through `help all` |

The model-selection shortcut remains one deliberate click. This increment improves
action prominence; it does not claim a measured reduction in time to first reply.
No API schema, authority, model selection or user data is migrated.

## Checks

- `go test ./internal/cli ./cmd/offgrid -count=1`: passed.
- `npm.cmd --prefix apps/web run build`: TypeScript and Vite passed. The existing
  large-chunk warning remains (main bundle approximately 1.21 MB before gzip).
- `npm.cmd --prefix apps/web run api:check`: passed.
- `node scripts/maintenance/check-source-size.mjs`: passed.
- `node scripts/maintenance/check-docs.mjs`: passed, 316 maintained files at the
  time of the first run; rerun after this evidence file is added.
- Focused isolated renderer run: 49 passed, one private-PDF check skipped because
  its optional supplied-file environment variable was absent. Covers setup,
  retained selections/drafts, keyboard traversal, nine locales, narrow/RTL,
  reduced motion, 200% layout and document read/recovery. Command:
  `node scripts/test/test-web-workspace.mjs build/ux-audit/offgrid.exe workspace-shell.spec.ts chat-readiness.spec.ts document-reader.spec.ts --workers=2 --reporter=line`.
- `npm.cmd --prefix apps/desktop test`: 126 passed, one platform-specific skip.
- `node scripts/test/test-desktop-theme.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"`:
  passed against isolated HTTP fixtures serving the current renderer, including
  saved/native appearance, nine-locale options, all main screens and Help at 200%.
  Local capture directory: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-IixnGA`.
- `node scripts/test/test-desktop-terminal-packaged.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"`:
  passed. Exercises main-area terminal tabs, focus/cursor preservation, search,
  copy, Unicode, interactive CLI, agent draft and `/clear`, service identity,
  blocked OSC clipboard requests, and child cleanup on reload/close/quit.
  Local capture directory: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-yqJBTL`.

The initial focused run failed two new assertions: a case-sensitive readiness
label and a document test that accidentally loaded a populated conversation.
Those fixtures were corrected. Its real PDF preview also failed because the
isolated service binary had no adjacent document worker. Building the worker
from `runtimes/pdfium` restored the real preview check; no runtime policy was
relaxed. The final focused result above includes that real preview test.

## Remaining gates

The first focused run did not establish full renderer coverage; the follow-up
results below supersede that pending check. No release or installed-workspace
replacement is established by these checks. The first-use offline acquisition
path, complete journey comparison, real-model answers, non-developer usability,
manual screen-reader/translation review and other native platforms remain
separate gates. Existing OpenSpec tasks remain open where these checks cover
only part of their acceptance scenarios.

## Installed-app audit implementation follow-up

The baseline is the installed Windows 0.4.25 application and its served UI,
not the older screenshots in the conversation. The installed UI hash was
`a7cdabbdfe70ab88a3496163a906fd24b562ec7ecec0f1e92658d1765ec33e7b`.
This follow-up remains a source candidate; the installed app was not replaced.

- Saved document answers render validated source references as readable document
  chips. A new authenticated, no-store passage endpoint reads the exact retained
  UTF-8 range from the pinned attachment revision. Legacy offset references are
  accepted only when unambiguous and equal to a retained selection. Forged,
  malformed, out-of-selection and unavailable references do not acquire links.
- A unique exact passage opens its preview page and highlights the text. Repeated
  or cross-page passages display the retained excerpt without inventing a page.
  This verifies source identity and bytes, not whether the source proves the claim.
  Closing the reader restores citation focus. Streaming drafts are not treated
  as validated saved citations.
- Models explicitly identifies the selected model and initially collapses
  cancelled download history; active transfers and failures remain visible.
- Activity and the header use the same attention/running/history classification.
  Filters retain their route after inspection, deletion and reload. This changes
  presentation, not task authority or deletion protection.
- The empty Knowledge library explains its distinction from conversation files,
  with copy in all nine locales, and omits an unusable Ask action.

Unlike the initial navigation-only increment above, this increment includes an
additive API schema operation and regenerated TypeScript types. It does not
migrate installed data, grants, model choices or extraction records.

### Final checks

| Command | Observed result |
| --- | --- |
| `go test ./internal/sessions ./internal/server ./internal/cli ./cmd/offgrid ./pkg/api -count=1` | All five packages passed |
| `npm.cmd --prefix apps/web run build` | TypeScript/Vite passed; existing large main-chunk warning remains |
| `npm.cmd --prefix apps/web run api:check` | Passed |
| `node scripts/maintenance/check-docs.mjs` | 317 maintained files checked, zero problems; external URLs/anchors excluded |
| `node scripts/maintenance/check-source-size.mjs` | Clean |
| `git diff --check` and `openspec validate workspace-outcome-experience --strict` | Passed |
| `node scripts/test/test-web-workspace.mjs build/ux-audit/offgrid.exe --workers=3 --reporter=line` | 1,002 passed, 31 skipped; isolated state |
| Focused `history-controls`, `attachment-citations`, `installed-audit-followup`, `document-reader` renderer run | 46 passed, one optional private-PDF test skipped |
| `npm.cmd --prefix apps/desktop test` | 126 passed, one platform-specific skip |
| `node scripts/test/test-desktop-theme.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"` | Passed; current renderer in isolated packaged Windows fixture, appearance and 200% layout |
| `node scripts/test/test-desktop-terminal-packaged.mjs "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"` | Passed; tabs, cursor, Unicode, CLI/agent interaction, clipboard restriction and child cleanup after reload/close/quit |

Final renderer log: `build/ux-audit/full-renderer-verified.log`. Packaged appearance
captures: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-9WLvYc`;
terminal captures: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-i4Y8H0`.
Inspected candidate screenshots include the readable source passage, narrow
Activity, light/dark Models and the RTL/200% reader. The Models screenshot shows
the deliberately expanded download disclosure; the test separately verifies
that it starts collapsed. These fixtures are not installed-user screenshots.

Earlier renderer failures exposed real filter restoration and citation-focus
regressions; both were corrected before the final full run. Existing assertions
were also updated for the deliberate Selected label and explicit All work filter.
The initial packaged terminal check matched both a closing and an active pane;
its locator now targets the visible pane. Process cleanup assertions remain and
passed on the rerun. The initial passage-handler test exposed incorrect error
mapping; the final route preserves the scoped session error contract.

`go test -race` could not execute: CGO is disabled and no compatible C compiler
was available in the checked environment. Race coverage remains unverified.
No real-model citation/follow-up qualification, new installation, human usability,
manual screen-reader/translation review, macOS or Linux native qualification is
claimed. The remaining audit phases and release gates are still open.

## Secondary-page loading and shared-control ownership

Chat now defers the Activity, Agents, Knowledge, Models and Settings page modules.
The loader caches code only; it does not retain actor data or page state. Pending
navigation remains dismissible, loading has an accessible status and reduced-motion
fallback, and a failed module offers an explicit app reload that retains the saved
Chat draft. It does not retry mutations or automatically reload the workspace.

The first full run found 48 failures (957 passed, 31 skipped): shared Chat profile
controls depended on CSS imported by the deferred Agents page. Names, descriptions
and revisions lost their block separation, including in accessible labels. Those
controls now import their own shared stylesheet. One touch-target test was also
corrected to wait for the actual screen control before measuring it, retaining its
44px assertion. No agent selection or authority assertions were weakened.

- `npm.cmd --prefix apps/web run build`: passed. Main JavaScript is 836.50 kB;
  its eager shared chunk is 160.70 kB. The large-chunk warning remains.
- `deferred-pages.spec.ts` records 997,212 decoded JavaScript bytes across two
  initial Chat assets, with no secondary page modules requested until navigation.
  This is a transfer/code measurement, not a model-generation latency claim.
- Corrected regression run across eight affected suites: **183 passed**.
- Full `test-web-workspace.mjs` run: **1,005 passed, 31 skipped**
  (`build/ux-audit/deferred-final-renderer.log`).
- After moving the final shared hint style, the deferred-page, action-hierarchy
  and agent-chat suites passed **88 tests** (`deferred-final-focused.log`).
- API generation check, source-size ratchet and diff whitespace check passed.

The full run overlapped isolated installer packaging: history feedback p95 was
55.1 ms, stream feedback p95 64.5 ms and Stop feedback 30.4 ms. These pass the
existing fixture feedback checks. Cold-route timings were materially slower under
that competing build load, so this run does not establish the unchanged-hardware
15% performance comparison or close task 6.3. Human, native-platform and real-model
qualification limits above remain open.

CI at `eabd70c3` passed the Linux suite but failed the Windows installer Finish
close check. Its diagnostics are being investigated separately; the renderer
results do not make that release gate pass.

## Runtime readiness and subsequent verification

Models now uses the same chat-eligibility predicate as Chat. Explicit
`runtime_ready: false` disables selection while retaining the installed row and
saved preference. An explicit failure overrides a stale loaded flag; missing
legacy readiness remains unchecked rather than being treated as failure.
First-use guidance remains visible when all installed chat runtimes are unavailable.

- Web TypeScript/production build passed.
- Installed-audit follow-up, model-library and model-search suites: **19 passed**.
- Chat and connection readiness suites: **21 passed**, including localized
  unavailable-model recovery and draft preservation.
- Source-size ratchet and maintained documentation checks: passed (317 files,
  zero link problems).

Before this readiness increment, CI **41952580287** at **2377369b** completed
successfully in all jobs, including Windows/macOS native, Go/API/UI/desktop and
document-processing checks. This identifies the tested revision; it does not
claim that CI has tested later unpushed work.

## Isolated Windows packaging and installer diagnostics

The packaged renderer theme check passed with isolated state
(`build/ux-audit/deferred-final-packaged-theme.log`): six screens, 200% zoom,
Help, nine language options, saved appearance and restart. This was the local
candidate package, not a replacement of the user's installed application.

An initial isolated installer failed startup because its generated service binary
was still **0.4.24**, while desktop was **0.4.25**. The refusal was correct. The
isolated candidate was repackaged with the matching service and passed the full
installer test (`build/ux-audit/installer-verified-test.log`), covering install,
startup, Finish launch, silent active-app refusal, interactive reinstall and
uninstall. Finish closed in **247 ms**; reinstall Finish closed in **157 ms**.
No personal workspace or installed OffGrid application was replaced by these tests.

The installer test now checks the bundled service version before testing startup,
so stale build artifacts fail with their actual mismatch instead of a generic
startup timeout. Process inspection is bounded and retains its handle until exit
to obtain a reliable exit code on Windows PowerShell 5.1.

The earlier CI Finish failure was not reproduced by this test or the subsequent
green CI run. Per-poll HWND existence, visibility, owner, title and elapsed time
are now captured after Finish for future diagnosis. The original **two-second
window-destruction requirement is unchanged**; a hidden window is not counted as
closed. Compilation, version matching and these local results are not a diagnosis
of that earlier intermittent CI failure.

## Offline GGUF setup increment (task 22.2)

The model library now exposes an explicit file-import disclosure on services
advertising `gguf-offline-import-v1`, for model administrators only. Choosing a
file does not upload it. Review shows the selected name/size and an explicit import
action. Existing model files and current selection are preserved; the UI supports
cancellation, real in-flight status, reduced motion and all nine locales.

The shared authenticated API streams into a private staging file and records a
SHA256. Bounded metadata checks reject unsafe filenames, invalid headers,
projectors, split filenames and renamed split metadata. Exact byte lengths are
required. Publication is atomic and cannot replace an existing file; staged files
are removed on ordinary failures/cancellation. Concurrent imports are bounded.
No model is loaded, selected or downloaded. These checks are **storage/header
validation, not tensor validation, runtime qualification or model-quality evidence**.

Lost acknowledgment or cancellation does not establish whether publication
completed. The UI retains uncertainty, requires inspection/refresh and does not
automatically replay the upload. A confirmed conflict is shown separately.

The first real-service test caught selection of the newly imported file when the
previous selection was empty: the shared inventory refresh chose the first model.
Import refresh now explicitly preserves empty selections too. That run recorded
48 passes and one failure; the corrected run passed 49, followed by **51 passing
checks** after adding cancellation/motion and read-only-user scenarios. The real
service test uses synthetic metadata/storage bytes, verifies the returned digest
against the input, and verifies inventory and unchanged selection. It performs no
inference and is not a real-model qualification test.

`go test ./internal/models ./internal/server ./pkg/api -count=1` passed after
reviewing the new guarded route and updating its frozen registration fixture from
149 to 150 routes. Focused tests cover exact-byte storage, conflicting identities,
concurrent publication, nested existing models, malformed input, interrupted
reads, cancellation and authorization before reading the body. Web build, generated
API check, documentation links and source-size checks passed. Wider task 22.2
qualification remains open; this entry does not close the whole first-use program.

The full renderer run recorded **1,023 passed, 31 skipped, one failed**. The new
mock import test read its baseline selection before initial page setup settled;
it now starts with an explicit saved selection. The independent real-service test
waits for the imported inventory row to remain unselected and removes only its own
synthetic model afterward. The corrected 17-case import suite passed three
repetitions (**51 passed**, `offline-import-repeated-ui.log`). This focused rerun
does not rewrite the earlier full-run result.

The overlapping `go test ./...` run failed the isolated default-server startup
and local-endpoint background-job deadlines while resource pressure was reported.
With browser testing finished, **`go test -p 1 ./...` passed**
(`offline-import-go-all-serial.log`); test deadlines and assertions were not changed.
The request-limit regression also confirms GGUF uploads can exceed the ordinary
64 MiB JSON limit while ordinary inference requests retain their existing bound.

## Model-picker scroll-edge correction

CI **41957045585** at **7c7068b9** passed 1,006 renderer tests and skipped 31,
but failed the English dark model-picker check: the focused final option had an
intersection ratio of **0.994140625**, rather than full visibility. The list now
has a three-pixel inset and matching scroll padding, keeping the last row and its
focus outline inside the scrolling surface instead of flush against its edge.
The full-visibility assertion is unchanged. `model-selection.spec.ts` passed
three repetitions (**45 passed**, `model-picker-inset-tests.log`), and its current
candidate capture was inspected. This is a targeted correction, not a claim that
the failed historical CI run passed.

## Guided offline setup and retained selection

CI **41964736383** completed successfully at **9d394e9e**. That result covers
the preceding import/readiness and picker changes, not this later increment.

When setup has no usable chat model, a capable service now offers an explicit
offline import entry. It opens the existing review, selects the appropriate
library category and restores keyboard focus; it never opens a file dialog,
uploads, downloads or selects a model automatically. Older services, non-admin
accounts and offline connections do not advertise this unavailable operation.
All nine locales use the existing translated import label.

Review found a reload gap after preserving an empty selection during import:
normal startup could subsequently choose the new first model. An import now
remembers that automatic first-model selection is disabled. Existing clients
retain their first-launch defaults; explicit selection still works. The real
isolated-service storage test now reloads and verifies unchanged selection too.

Once the user selects a usable model, first-use Models offers **Start your first
chat**. The handoff retains the existing question without submitting it. The
initial test fixture inadvertently loaded a saved answer and reset setup state
on every reload; those fixture assumptions were corrected without weakening
the selection, reload or draft assertions. Failure logs remain in the private
`build/ux-audit` directory.

Validation: the TypeScript/production build and source-size ratchet passed.
`node scripts/test/test-web-workspace.mjs build/ux-audit/offgrid-import.exe
offline-model-setup.spec.ts offline-model-import.spec.ts chat-readiness.spec.ts
--workers=2 --reporter=line` passed **48 tests**
(`offline-setup-handoff-ui.log`). The service fixture contains synthetic GGUF
storage metadata, not a qualified inference model. Wider journey, real-model,
manual accessibility and human evaluation gates remain open.

The complete renderer suite at **fe45f734** passed **1,038 tests, 31 skipped**
(`setup-final-full-renderer.log`). Its new first-use and handoff screenshots were
inspected. The dialog entrance capture also revealed underlying text bleeding
through its fade; that separate animation correction is recorded below.

## Windows close-versus-quit feedback

The existing Windows hide-to-tray lifecycle now requests one localized, quiet
notification on the first close per launch. It explains that work continues and
identifies the tray's open/Quit actions. No task is stopped, shell started or
authority changed. The notification respects Windows quiet time; refusal or
suppression does not block closing or produce repeated errors. macOS/Linux do
not invoke this Windows-only API.

The helper is explicitly included in the desktop package. Focused lifecycle,
presentation and notification tests passed **19/19**; the full desktop unit suite
passed **129, with one platform skip** (`tray-desktop-suite.log`). These tests
verify behavior and locale keys, not actual OS notification delivery or native
translation quality. A live Windows notification observation remains unverified.

## Opaque overlay entrance

The inspected first-use screenshot showed background headings through the setup
dialog during entrance. The shared animation faded the whole overlay, including
text, even though its settled surface was opaque. New frame-sampled checks
reproduced this in all six setup/palette/menu light/dark cases before the fix.
Overlays now retain opacity while moving by the existing four-pixel distance;
the existing reduced-motion fallback remains unchanged.

The first combined rerun exposed eight material-geometry test failures: those
tests used opacity reaching one as a proxy for completed motion. They now await
the actual finite entrance animation before comparing exact bounding boxes.
Geometry and contrast assertions were not relaxed. Corrected overlay, material
and offline-setup tests passed **32/32** (`opaque-overlays-final-ui.log`), and
the corrected setup capture was inspected.

The opt-in one-worker paint test also passed with `OFFGRID_PAINT_MEASURE=1`
(`opaque-overlays-performance.log`). Across light/dark, materials on/off, and
4x/6x CPU throttling, observed open feedback was **20.7–42.4 ms**, Stop feedback
**3.1–38.7 ms**, and frame p95 **16.7–33.4 ms**. This is one isolated fixture run
on this machine, not a cross-device performance or inference-speed claim. It
does not close the broader historical 15% comparison requirement.
