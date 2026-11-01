# Whole-product experience implementation — October 30, 2026

This continues `workspace-outcome-experience` under the approved whole-product
review scope. Earlier completed presentation tasks do not qualify these changes.
The installed desktop, models, history and native GPU runtime were not replaced
by this work. Existing unrelated desktop/startup and runtime edits were retained.

## Implemented in this batch

- New Chat and conversation selection detach the current view without cancelling
  its durable turn. Returning refreshes saved output even after background
  completion. Late Stop acknowledgments cannot abort a different conversation.
- Artifact Preview explicitly reads authorized bytes, validates SHA-256 and size,
  rejects malformed UTF-8/binary control content, and renders bounded inert text
  or safe Markdown. Closing clears private content and cancels pending reads.
  Reopening checks current access. Downloads remain available separately.
- Model selection searches readable names and exact IDs, shows reported runtime
  readiness/context/loaded state, and retains exact identity. Selection does not
  download, load, substitute or qualify a model.
- Recovery guidance uses typed saved states. Failed work no longer presents
  Refresh as a repair. Interrupted work distinguishes checkpoint, expired access
  and unavailable checkpoint; saved output remains visible. Uncertain effects,
  approval and command ownership are unchanged.
- Specialist summaries use authorized canonical followers for at most four
  children. Child parent identity is checked before rendering returned output.
  Status, dependencies and deliverables stay subordinate to the parent; larger
  legacy plans keep explicit inspect links.
- Static Markdown is memoized and streaming Markdown uses a deferred render.
  Controls and exact final text remain independent from that presentation work.
- A legacy mobile rule that hid all composer button spans was removed. Picker
  rows now retain visible names and exact identifiers on narrow/RTL screens.
  Preview buttons keep a short visual label and a full accessible filename.
- Agent creation now begins with purpose and editable writing, comparison and
  learning examples in all nine locales. Examples only fill an empty unsaved
  instructions field; they do not set resources, permissions or execute work.
  The create/save/use journey exposed a pre-existing handoff bug: shared
  workspace state was written inside a deferred React updater and could be lost
  on navigation. The write now precedes navigation, with a pure React updater.
  The new saved-revision handoff case reproduced the failure before this change
  and passed afterward. Remaining capability grouping is still tracked in 16.6.

## Automated checks and visual inspection

Commands used the isolated service wrapper, never the installed workspace:

```powershell
npm.cmd --prefix apps/web run build
npm.cmd --prefix apps/web run api:check
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-source-size.mjs
openspec.cmd validate workspace-outcome-experience --strict
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe chat-streaming.spec.ts work-continuity.spec.ts work-results.spec.ts --workers=1 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe --workers=4 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe agent-chat.spec.ts ux-recovery.spec.ts model-selection.spec.ts work-results.spec.ts --workers=1 --reporter=line
```

- Type/build, API projection, source-size and strict OpenSpec checks passed.
  Maintained-document checking passed again including this entry: 281 files,
  zero problems (external URLs and heading anchors excluded by that checker).
- Navigation/recovery/result subset: **44 passed**, including background
  completion before returning, late Stop, retained draft, exact approval,
  uncertain effects, lost steering acknowledgment and expired access.
- Broad run: **458 passed, 27 skipped, 4 failed** of 489 cases. Failures were
  old specialist-link selectors, a speech fixture relying on the removed generic
  Refresh, a missing direct-session response in a recovery fixture, and missing
  alert semantics in the new recovery panel. The first three fixtures were
  corrected for the actual interaction/API; alert semantics were restored in
  production code. The speech test now uses the normal polling path.
- Following those corrections and a pending-preview cancellation case:
  **95 passed** in the targeted suite above. This does not retroactively turn the
  earlier broad run into a clean full-suite result.
- The final model/profile subset passed ten cases; its new Enter-in-search test
  initially used an ambiguous option selector that also matched native options
  elsewhere in the form. After scoping the assertion to the model listbox, that
  case passed separately. Enter in model search focuses a result without saving
  the enclosing profile; explicit Save remains required.
- After the purpose/handoff implementation, the full 501-case run recorded
  **466 passed, 27 skipped, 8 failed**. All eight failures were the new non-English
  purpose fixtures: their existing initialization script reset the locale to
  English on reload. The fixture now accepts its locale at initialization;
  all **nine purpose/localization cases passed** on rerun. Production
  account/workspace isolation, navigation, draft, voice, approval and shared-state
  regressions passed in the broad run. Raw output is retained locally in
  `build/workspace-outcome-experience/whole-product-review/web-final.log`.
- Narrow model picker screenshots in English and Arabic and long-filename
  artifact previews in both themes were inspected. Inspection found the hidden
  button-label rule and oversized filename in the preview button; these were
  corrected and covered by visibility/layout assertions. Capture now disables
  transitional animation so entrance opacity is not confused with final contrast.
- JavaScript bundle after purpose examples: approximately **971.87 kB / 307.55 kB gzip**. The existing
  500 kB chunk warning remains; this is not a bundle-performance qualification.

## Streaming measurement

The ordinary synthetic stream fixture measured text p95 **59.8 ms** and local
Stop feedback **9.6 ms** in the 44-case run. These are UI timings, not inference
latency or completion time.

The unchanged stress fixture runs gradient off/on at 4×/6× CPU throttling,
using browser 151.0.7922.34. First updated sample:

| CPU | Gradients | Stop feedback | Frame p95 |
| --- | --- | ---: | ---: |
| 4× | Off | 138.9 ms | 250.0 ms |
| 4× | On | 45.5 ms | 316.7 ms |
| 6× | Off | 121.9 ms | 416.6 ms |
| 6× | On | 100.8 ms | 316.7 ms |

All four Stop samples passed the existing 250 ms gate. Frame stalls remain
visible under extreme streamed-text load. This sample does not establish
smooth rendering or a causal gradient improvement. Raw local evidence is
`build/workspace-outcome-experience/whole-product-review/stream-stress-sample-1.json`.
The second isolated sample also passed all four Stop gates: **88.5 / 58.4 /
89.2 / 90.5 ms**, in the same table order. Frame p95 remained **300.0 / 283.3 /
450.0 / 499.9 ms**. Raw evidence is
`build/workspace-outcome-experience/whole-product-review/stream-stress-sample-2.json`.
Task 16.4 remains open for further heavy-stream smoothness work; passing Stop
latency alone is not a claim of a consistently smooth experience.

## Subsequent main checkpoint and calendar/composer verification

Local `main` received checkpoint `8b83e398` at **2026-10-30 23:34:54 +08:00**,
before the user's 23:50 deadline. It contains the seven desktop reconnect/startup
files only. Fresh checkpoint verification: **92 desktop tests passed, one skipped**;
281 maintained Markdown files checked with no problems. No push or deployment.

The following UI/API work continued after that checkpoint and is **uncommitted**:

- Bounded model-trigger text and measured control non-overlap in English/Arabic
  at 320, 390, 768 and 1440px; exact model identity and selection are preserved.
- Composer context and voice controls have visible labels; submission controls
  occupy a separate row. Broader composer/human acceptance remains open.
- Schedule timezone search supports keyboard selection, manual IANA identifiers,
  empty results without invalid active-descendant references, and coarse-pointer
  targets. Decision limits are subordinate; existing resource review remains.
- Authenticated read-only calendar previews use the actual scheduler calendar
  and service clock. They return three future instants without creating policies,
  jobs or occurrence claims. Calendar changes clear obsolete predictions; aborts
  and late results cannot replace the current preview. Errors remain explicit.

Validation completed across October 30–31:

```text
go test ./internal/server ./internal/automation -run 'Test.*Schedule|Test.*Calendar|Test.*OpenAPI' -count=1
npm.cmd --prefix apps/web run api:check
npm.cmd --prefix apps/web run build
go build -o build/workspace-outcome-experience/offgrid.exe ./cmd/offgrid
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe agent-schedules.spec.ts model-selection.spec.ts chat-streaming.spec.ts voice-playback.spec.ts agent-chat.spec.ts --workers=3 --reporter=line
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-source-size.mjs
openspec validate workspace-outcome-experience --strict
```

### CLI image parity and reviewed-agent qualification follow-up — 2026-10-31

The user approved extending the image scope to the CLI. The existing `--image`
path had separate encoders in plain/TUI modes, guessed MIME from extensions,
advertised unsupported GIF/WebP, cleared attachments before inference succeeded,
and omitted them from saved sessions. Plain mode could continue as text after a
file-read failure. Those paths now share a bounded regular-file loader and the
same complete PNG/JPEG validation as saved Chat.

`offgrid run MODEL --image PATH` accepts repeated flags up to four images.
Both interactive modes support `/image PATH`, `/images` and
`/remove-image [N|all]`. Paths with spaces work; accepted bytes are snapshotted at
attachment time. Users may enter a question or submit an image-only prompt.
Failure, incomplete SSE, malformed events and streamed errors preserve pending
input and do not commit a completed exchange. Successful exchanges save original
bytes and reconstruct typed image history on load. The existing local CLI session
store is retained, rather than presented as a new service-owned conversation.
The aggregate image budget is checked before requests. Native agent `/image`
explains the unsupported execution path and does not attach or transmit a file.

Validation:

- `go test ./internal/cli ./cmd/offgrid ./pkg/api ./internal/sessions` passed
  (`cli-images-final.log`).
- Ubuntu/WSL `CGO_ENABLED=1 go test -race ./internal/cli ./cmd/offgrid` passed
  (`cli-images-race.log`, 3.527/55.328 s).
- Final targeted tests passed after image-only input and repeated-flag assertions
  (`cli-images-focused-final.log`): invalid/oversized/missing/non-regular files,
  attachment removal, byte snapshots, limits, no network/model setup on invalid
  flags, projector rejection, incomplete/error/malformed streams, and independent
  saved-session reload in plain and TUI paths.
- A current native CLI candidate was built into
  `build/workspace-outcome-experience/offgrid-cli-images.exe`; its actual
  `run --help` output includes the new controls (`cli-images-help.txt`). This
  candidate was not installed or deployed. Documentation/source-size/OpenSpec
  checks passed; actual vision inference remains unqualified without a local
  compatible model/projector.

The native desktop harness gained an opt-in `--agent` journey that uses the real
profile editor, saves a resource-free profile revision, invokes Use in Chat,
requests one private downloadable text artifact, and independently checks its
bytes/digest, visible preview, saved conversation and restart without replay.
It does not grant host filesystem/desktop access or attach external connections.
It records bounded state/action metadata on failure without hidden reasoning.

This journey **has not passed**. The first two runs exposed harness assumptions:
false profile settings are omitted from JSON, and job admission is asynchronous.
Those were corrected. The third real Qwen 2.5 3B CPU run admitted one root job but
remained in its first inference processing phase for the 180-second test deadline,
with no completed action/artifact. The profile creation/handoff worked; the final
agent outcome is unverified. Evidence is retained in `desktop-native-agent-r3.log`
and the isolated temporary workspace `offgrid-desktop-native-reply-JM3bNs`.
The test stopped its owned app/service and did not resume the unfinished task.
This observation does not identify the underlying inference cause or establish
that other models/platforms behave the same. Task 16.17 remains open.

#### Agent processing diagnosis and truthful activity — 2026-10-31

A fresh disposable service diagnosis used the same packaged service, copied
Qwen 2.5 3B model and Windows CPU runtime, two threads and an 8192-token context.
No installed workspace, model or service was changed. The diagnostic submitted
the artifact request directly, so it is not a substitute for the desktop journey
or its prior conversation history. Private evidence is retained under
`offgrid-agent-diagnostic-4nf26plk`; the earlier `r48izzef` diagnostic stopped on
a harness null-progress assumption, which was corrected before the new run.

The worker loaded in 4.1 seconds. Initial prompt processing progressed at about
22–24 tokens/second (1920 tokens reached at 85.9 seconds), then generation
continued at about 6.6 tokens/second. At cancellation it had generated at least
628 tokens without returning a complete tool decision or an artifact. This
rules out an idle/deadlocked worker in this diagnostic, but does not identify why
the constrained response failed to finish. It does not qualify the workflow or
justify increasing its deadline. The owned run was cancelled and the service
stopped; process inspection found no remaining diagnostic worker.

The diagnosis exposed a presentation defect: constrained tool envelopes withheld
all progress until final validation, leaving the agent in `processing` even while
generating a tool proposal. A context-local, status-only observer now recognizes
the bounded literal `tool_call` envelope prefix and emits `preparing_tool` once.
It carries no name, arguments or reasoning and does not invoke the normalized
decision callback. Final schema validation and clean stream termination remain
mandatory. Tests cover every prefix split, unrelated/prose/answer prefixes,
callback cancellation, no leaked argument text, truncated streams and the real
server-to-HTTP-engine progress connection. This fixes activity presentation, not
the still-unqualified model decision quality.

Validation: `go test ./internal/inference ./internal/server` passed (21.788 and
117.102 seconds; `agent-tool-activity-regression.log`). Targeted WSL race checks
for `Test(SingleStep|ToolDecisionActivity|AgentStream)` passed both packages
(1.348 and 1.142 seconds; `agent-tool-activity-race-r3.log`). Two preceding WSL
launch attempts failed before tests ran due to shell quoting and a missing Go
PATH; those logs remain retained. Source-size and maintained-document checks
passed (281 Markdown files, zero reported problems). No installed binaries were
replaced and the desktop artifact-completion gate remains open.

Both Go packages passed; API generation check, TypeScript/build, native test
binary build, documentation, source-size and spec validation passed. The final
isolated browser run passed **111/111**. Its initial run passed 110/111: the voice
focus fixture attempted focus during disabled profile loading. It now waits for
loaded options and enabled state before testing focus; no product behavior was
weakened to make it pass. No fixed sleep was added.

Calendar tests cover spring DST gaps, the first repeated fall instant, not
repeating the second fold, weekdays, invalid/duplicate parameters, unauthenticated
access, and unchanged stored work. Browser fixtures cover stale/error previews,
all nine narrow layouts, keyboard, review reset, disabled actions and no writes
from preview. Screenshots of wide light and narrow dark schedule editors were
inspected. Human translation/accessibility and real scheduled model execution
are not established by these fixtures. Task 16.13 records this implementation
and automated coverage, not unattended production qualification.

Final web bundle: JavaScript **979.21 kB / 310.15 kB gzip**; CSS
**111.98 kB / 21.20 kB gzip** (see build log for exact emitted values). The existing
large-chunk warning remains. Logs are under
`build/workspace-outcome-experience/whole-product-review/`:
`composer-schedule.log`, `composer-schedule-final.log`, and
`calendar-build-final.log`. Test artifacts are disposable, not installed-user data.

### Remaining program scope

The whole-product program is **not complete**. Composer hierarchy,
bounded work search, enforced document-scoped questions,
skill and schedule workflows, broader setup/voice refinements and remaining
screen hierarchy work are tracked in task group 16. Model discovery and
specialist conflict journeys need their remaining acceptance coverage.

No fresh-install, sleep/wake, update, real model/native workflow, human
screen-reader or five-participant study is established by these browser fixtures.
macOS/Linux installation and all nine locales' speaker review remain open.
The UI/API changes remain uncommitted. Apart from the explicitly recorded desktop
checkpoint above, no release, push or installation was performed in this batch.

## October 31: agent configuration and model decision flow

Task 16.6 is implemented and automatically tested. Purpose examples remain
editable instructions only. Optional Tools, Knowledge and collaborators have
independent disclosures and selection counts. Existing selected resources open
initially; collapsing a section preserves selections and editing another field
does not force it open. Switching profiles resets presentation without carrying
another profile's selections. Revision-conflict, missing-resource, dirty-draft,
save and explicit Use in Chat behavior remain covered. The actual React handoff
selects the saved revision without submitting a task; these tests use API fixtures,
not a qualified native model execution. Nine locales, wide/narrow and RTL are
covered automatically; human translation and screen-reader review remain open.

Visual inspection found that the old sticky Save footer could cover lower fields.
It now follows the editor fields, and a geometry assertion prevents that overlap.
Fresh editor and comparison screenshots were inspected. This is functional visual
verification, not a claim of design parity or a user-study result.

Language/embedding discovery now separates curated catalog browsing from explicit
Hugging Face search. Comparison shortlists contain at most three catalog models;
download size, parameter count, quantization, license and catalog minimum RAM are
shown without inventing hardware fit. Missing values stay unknown. Filters and
shortlists are scoped to category/workspace; neither selection nor transfer is
triggered by comparison. Active transfers stay visible when their catalog card
is filtered out or online search is selected. Speech packages retain their existing
reviewed variant flow. Task 16.7 remains open for the remaining model-choice scope.

Checks:

- `npm.cmd --prefix apps/web run build`: passed TypeScript and Vite. Final bundle
  JavaScript **985.22 kB / 312.16 kB gzip**, CSS **113.47 kB / 21.44 kB gzip**.
  The existing large-chunk warning remains.
- Isolated targeted renderer: **63/63**, then **83/83** after adding the distinct
  discovery views. Log: `profile-model-final.log`.
- Full isolated renderer, `node scripts/test/test-web-workspace.mjs
  build/workspace-outcome-experience/offgrid.exe --workers=3 --reporter=line`:
  **498 passed, 27 skipped, 5 failed**. All five failures were journeys assuming
  online search was immediately displayed on entering Discover. They were updated
  to explicitly select Search Hugging Face, preserving no-transfer, authorization,
  variant selection, recovery and retained-query assertions. Log:
  `integrated-oct31.log`.
- Follow-up `model-library-experience.spec.ts model-search.spec.ts
  interaction-contracts.spec.ts workspace-journeys.spec.ts`: **41/41 passed**,
  including all five failed journeys. Log: `discovery-journeys-final.log`.
  This is full-suite evidence plus targeted correction, not a single all-green
  full run. Runtime source did not change after the full run.
- Documentation: **281 maintained Markdown files, zero problems**. Source-size
  ratchet, strict OpenSpec validation and `git diff --check`: passed.

Logs remain under `build/workspace-outcome-experience/whole-product-review/`.
The installed desktop, service and CLI were not replaced; no further commit,
push or release occurred. No new Go implementation changed in this batch.

## Selected-source Knowledge questions — October 31

Implemented document and temporary collection selection (maximum sixteen), an
explicit question handoff preserving the draft, and source scope tied to the
conversation. The service advertises `scoped-knowledge-chat-v1`; clients reject
older services rather than allowing an ignored field to broaden retrieval.
Selected document availability and revision are checked around filtered search.
Profile and subordinate Knowledge scopes can only narrow existing authority.
Unavailable sources and empty retrieval fail without unrelated-source fallback.

Actual retrieved passages, identities and UTF-8 content digests persist with
completed plain/streaming/durable exchanges and linked job results. Replay uses
the same request identity and selected IDs. The renderer shows inert supporting
passages and labels their limits; it does not claim that model prose is verified.

Validation found and fixed a first-visit handoff race: initial history selection
could move from the empty draft to a saved conversation after consuming its source
selection. Consumption now waits for history selection. A separate check ensures
that moving to a different conversation does not turn selected-source retrieval
into a whole-library search.

- `go test ./internal/server ./internal/sessions ./internal/rag`: passed; server
  174.310 s, sessions 1.857 s, retrieval 2.993 s. Focused source/permission/profile/
  replay tests also passed. Logs: `knowledge-scope-packages.log` and
  `knowledge-scope-contracts-final.log`.
- API generation check, TypeScript and Vite build passed. Source-size ratchet,
  documentation (281 maintained files, zero problems), and strict OpenSpec passed.
- Initial dedicated browser run: 10 passed, 2 failed, exposing the handoff race;
  corrected run: 12 passed. Expanded Vite-only run: 89 passed, 2 failed because
  the older streaming fixture relies on a real service identity for work recovery.
  The documented isolated-service harness, using the newly built executable and
  renderer, then passed **91/91** across Knowledge scope, Chat streaming, agent
  Chat and whole-workspace journeys. Log: `knowledge-isolated-ui.log`.
- Supporting-passages screenshot inspected. Raw test markup remained text.
  Follow-up visual adjustment makes source removal a labelled compact action and
  puts digests under Source details. Narrow/RTL scope journeys cover all nine
  locales; native translation review remains unverified.
- `go test -race` could not start: CGO is disabled and no C compiler is available
  on this shell's PATH. Log: `knowledge-scope-race.log`. This is not a race pass.

This evidence covers source-scope mechanics and renderer behavior using isolated
fixtures; it does not establish model answer quality, real native workflows,
screen-reader quality or the five-person study. No deployment or release occurred.

## Rendering, skills and subordinate results — October 31 follow-up

Long saved answers and streaming Markdown now parse in a shared local worker.
Only the newest pending revision for each mounted response is queued. Parsing
preserves whole-document references, code, GFM tables and Unicode. HTML remains
text, images do not fetch, and copying uses canonical source. Worker errors,
message errors and a ten-second stall release the queue to readable plain text.
No inference, authority or dispatch policy changed.

The first worker tests exposed a real browser-export defect: the entity decoder
required `document`, which does not exist in workers. The build now resolves the
same package's data-only decoder, pinned as an explicit dependency. The regression
checks include named entities, reference definitions arriving in later deltas,
exact code/response copying, navigation, startup failure and a stalled parser.
The initial stall fixture incorrectly stopped at an empty initial message, then
advanced its clock before the pending parse existed. It now waits for an actual
stalled request. The corrected worker-only run passed **4/4**.

Three isolated runs of the unchanged 60-delta streaming workload, at both 4x and
6x CPU throttling with gradients on/off, passed the 250 ms Stop gate and the added
250 ms stream-frame p95 assertion. Browser: Chromium 151.0.7922.34.

| Run | Stream frame p95 range | Stop feedback range |
| --- | --- | --- |
| 1 | 33.4–66.7 ms | 36.3–142.5 ms |
| 2 | 33.4–66.8 ms | 36.0–88.8 ms |
| 3 | 50.0–100.1 ms | 12.6–29.6 ms |

Reports: `worker-performance-r1.json`, `worker-performance-r2.json` and
`worker-performance-r3.json` under the existing private whole-product-review
evidence directory. These include individual paint durations/events and download
frames. The separate ordinary streaming fixture measured text p95 **56.3 ms**
and local Stop feedback **13.8 ms**. These are synthetic interface measurements,
not model token speed, installed OS qualification or claims of 60 fps throughout.
Earlier failing stress evidence above remains historical evidence of the defect.

Latest build: main JavaScript **1,031.02 kB / 330.99 kB gzip**, CSS **116.39 kB /
21.92 kB gzip**, worker **153.51 kB** uncompressed. The main bundle still exceeds
Vite's 500 kB advisory; off-main-thread parsing does not solve initial bundle cost.

Skills now have bounded local name/description search, review-state cards,
inert examples, explicit revision attachment and inspect/refresh/removal for
saved references absent from the current list. Absence from the latest-revision
list is not falsely reported as proof that an older revision is unavailable.
Review failures keep drafts inactive. Save and Use in Chat retain their existing
separate, explicit effects. New strings cover all nine locales; native-speaker
review remains unverified. Visual inspection prompted removal of a browser-default
fieldset border and restyling the import control using existing palette tokens.

Specialist fixtures now cover conflicting findings alongside a failed verification
branch and its cancelled dependent. They retain separate attributed drafts,
dependency names and the parent's actual failed state, without lifecycle controls
or writes from the summary. This reflects current runner behavior: a failed branch
fails the parent and cancels remaining work; it does not claim independent branches
continue after failure.

The final targeted group passed **62/63** before the clock-fixture correction;
the remaining worker test then passed in the **4/4** focused run. Knowledge scope,
skills in nine locales, exact revision/review conflicts, profile-to-Chat handoff,
specialist branches, artifact safety, recovery and ordinary streaming passed.
Final screenshots were inspected and retained as `knowledge-passages-final.png`
and `skill-library-final.png`. Desktop unit tests: **92 passed, 1 skipped, 0 failed**.
The subsequent complete isolated renderer run passed **524 tests, skipped 27,
failed zero** (551 collected, 5.8 minutes). Exact command:

```text
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe --workers=3 --reporter=line
```

Log: `full-renderer-oct31.log`. Model-fixture, opt-in visual/performance and
environment-dependent skips remain skips; the three dedicated performance runs
above provide their own evidence. API generation check, TypeScript/build,
source-size ratchet, maintained documentation and strict OpenSpec validation
also passed. Desktop command: `npm --prefix apps/desktop test`; API command:
`npm --prefix apps/web run api:check`. This does not replace outstanding installed
platform, real-model, race, human accessibility or participant evidence and is
not a release-readiness claim.

## Composer hierarchy follow-up — October 31

After the full-suite run above, the composer gained a separate voice/action row,
quieter labelled settings, and readable pinned-agent model inspection. Exact
model/profile identity remains available; the disclosure is read-only. Execution
workspace and tool controls remain visible and preserve their existing authority.
Speech presentation was extracted into a controlled component, keeping the
handwritten source-size ratchet without relaxing it.

Six new layout/authority fixtures cover English/Arabic at 320, 768 and 1280px,
including no overlapping controls, retained draft, exact profile revision and
model identity, focus restoration, explicit workspace/tools and zero writes.
Screenshots `composer-en-1280.png` and `composer-ar-320.png` were inspected.

The first regression group passed **116/117**. Its remaining voice test clicked
Agent while the newly positioned open voice popup covered that button. The test
now dismisses the popup with Escape and verifies focus restoration before moving
to Agent; private-child speech exclusion assertions remain unchanged. The entire
agent-chat file then passed **59/59**. Additional short-window, reflow, reduced-
motion, IME, focus, source-scope and interaction checks passed **61/61**. Ordinary
streaming measured text p95 **59.1 ms**, Stop feedback **17.1 ms**.

Commands (all against the disposable service and built renderer):

```text
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe composer-hierarchy.spec.ts model-selection.spec.ts voice-playback.spec.ts chat-streaming.spec.ts agent-chat.spec.ts workflow-presentation.spec.ts workspace-shell.spec.ts --workers=3 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe agent-chat.spec.ts --workers=1 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe workspace-experience-quality.spec.ts compact-workspace.spec.ts interaction-contracts.spec.ts knowledge-scope.spec.ts --workers=3 --reporter=line
```

Logs: `composer-regression.log`, `composer-agent-final.log` and
`composer-accessibility-regression.log`. The 200% case is a reduced-CSS-viewport
reflow fixture, not proof of native Electron zoom or assistive-technology behavior.
No installed service, desktop, model or user workspace was modified.

The final composer build also passed the isolated 4x/6x heavy-stream comparison:
stream frame p95 **50.0–100.1 ms**, Stop feedback **12.9–142.0 ms**, all below
250 ms. Report: `composer-performance.json`; command is the same dedicated
`tonal-gradients.spec.ts --grep "gradient paint comparison" --workers=1` run
with `OFFGRID_PAINT_MEASURE=1`. The unthrottled regression and throttled run are
separate evidence; neither establishes real-model latency or native qualification.

## Saved-work navigation and search — October 31

Quick actions now uses the additive, authenticated `work-search-v1` endpoint for
conversation names and root task requests. The response contains only bounded
navigation metadata and explicit searched scopes; it does not search message
bodies or artifact contents. Queries are debounced/cancellable, result counts
are capped at 20, and scan/storage/time limits produce a visible partial notice.
The service applies the existing session ownership and task actor/profile rules.
Opening a result preserves an origin and draft through the existing navigation
path; it neither submits nor resumes work. Actor/workspace changes dispose both
results and the return path. Older services retain command-only navigation.

The first focused renderer run passed **46/46** across work search, interaction
contracts and work continuity. The expanded final search run passed **14/14**,
including a 500-conversation history, active-task navigation without writes,
draft return, delayed results, failure/partial states, logout/workspace changes,
IME and all nine narrow locale layouts. English and Arabic 320px screenshots
were inspected and retained privately as `search-en.png` and `search-ar.png`.
This is browser-renderer evidence, not native screen-reader qualification.

Two initial checks failed and were corrected: the new workspace-change fixture
used “Refresh workspace” instead of the actual “Refresh” label; the frozen route
manifest still expected 145 routes before the additive search route. The latter
now records the specific new handler and 146 routes without changing middleware
order. The first permission test also incorrectly assumed viewers lack Chat
permission; the corrected test verifies their session results remain excluded.

Commands:

```text
go test ./internal/sessions ./internal/agents ./internal/server -run 'Test(SearchNames|SearchRequests|WorkSearch)' -count=1
go test ./internal/server ./internal/sessions ./internal/agents
npm --prefix apps/web run api:check
npm --prefix apps/web run build
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe work-search.spec.ts interaction-contracts.spec.ts work-continuity.spec.ts --workers=3 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe work-search.spec.ts --workers=3 --reporter=line
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-source-size.mjs
```

All listed commands passed after the corrections. The full affected Go packages
passed (server 118.990s; sessions/agents reused successful package results).
API generation matches its schema; the build retains the existing large-chunk
warning. Documentation checked 281 maintained Markdown files with zero problems;
the source-size ratchet is clean. Logs are under the existing private evidence
directory as `work-search-*`. No installed workspace or service was changed.

The first full renderer run exposed two remaining stale composer style assertions:
they still required filled backgrounds on the deliberately quiet Context/Voice
utility controls. Their replacement asserts the shared utility/secondary roles,
hover boundaries, dimensions and keyboard navigation rather than removing checks.
The focused control suite passed **11/11**. The final complete renderer run then
passed **544 tests, 27 skipped, zero failures** in 3.4 minutes:

```text
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe --workers=3 --reporter=line
```

An additional resource review bounded task snapshots to 4,096 metadata records
and moved case folding/ranking outside the runner lock. A regression covers an
empty result from a truncated 4,097-task scan: it must remain explicitly partial.
Another regression covers exhausting exactly the 32 MiB conversation read budget.
The final sessions package passed in 3.510s and task-search checks in 0.556s.
These bounds avoid treating a resource limit as exhaustive absence; they do not
establish native Stop latency or model performance.
The final lock-scope changes also passed the complete agents (54.700s) and server
(91.134s) packages; log: `work-search-lock-packages.log`.

## Model choices and voice consistency — 2026-10-31

The shared model-choice disclosure replaces dense selects in recognition/speech
preferences, embedding setup, GGUF file selection and package variants. Choices
retain exact IDs/revisions, actual availability and source metadata. Unknown and
removed selections are visible, with no inferred fit or automatic substitution.
Selection does not start installation, microphone capture or draft submission.
The existing Chat/profile model picker and three-item catalog comparison remain
part of this verification scope.

Regression testing exposed a real focus loss when package resolution temporarily
disabled the selected trigger, and a grid sizing defect with longer translated
labels at 320px. Deferred focus restoration now respects users who move to another
control; bounded grid tracks prevent horizontal overflow. Other initial failures
were obsolete select-based test interactions and a case-sensitive expectation
for the existing “Curated catalog” label. Those assertions were updated to test
the new controls and actual copy, retaining exact-selection and no-transfer checks.

The initial final model regression passed **75/75** checks. After visual review
found English-only voice controls, all eight non-English voice copy sets were
implemented, including permission, timeout, transcription and playback states.
The subsequent voice/control/chooser regression passed **50/50**, including all
nine locales at 320×480, keyboard/IME, nested Escape, no implicit capture and
selection preservation. English/Arabic screenshots were inspected; they are
private under `build/workspace-outcome-experience/whole-product-review/`.
These translations are drafts, not independently speaker-qualified.

Real isolated-service fixtures passed **7/7** package acquisition/restart checks,
covering Whisper, Piper, Zipformer and Kokoro data fixtures. These validate service
operations, not actual speech inference or native audio devices. Commands:

```text
npm --prefix apps/web run build
npm --prefix apps/web run api:check
go build -tags modeltestfixtures -o build/workspace-outcome-experience/offgrid-model-fixtures.exe ./cmd/offgrid
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe model-choice.spec.ts voice-playback.spec.ts model-library-experience.spec.ts model-search.spec.ts workspace-journeys.spec.ts model-selection.spec.ts model-comparison.spec.ts --workers=3 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-model-fixtures.exe --model-fixtures model-acquisition.spec.ts model-restart.spec.ts --workers=1 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe model-choice.spec.ts voice-playback.spec.ts composer-hierarchy.spec.ts control-consistency.spec.ts --workers=3 --reporter=line
```

Build/type checks and API generation passed. The existing chunk-size warning
remains. Logs use the `model-choice-*` prefix in the private evidence directory.
No installed service, desktop or model configuration was changed in this batch.

The subsequent complete renderer suite passed **557 tests, 27 skipped, zero
failures** in 3.5 minutes (`model-choice-full-renderer.log`). Documentation checked
281 maintained Markdown files with zero problems; the source-size ratchet passed.
The full suite uses isolated service state; skipped hardware/native checks remain
unqualified. Task 16.7 is complete; the broader accessibility, installed-platform,
participant and setup/readiness tasks remain open.

## Sign-in context and scoped setup recovery — 2026-10-31

Sign-in now identifies the service origin, explains the service-account boundary,
supports explicit password inspection and retains form details after a failed
attempt. Concurrent submissions are guarded synchronously; fields are held stable
while a request is pending. Desktop URL metadata is restricted to HTTP(S) origins,
excluding credentials, query strings and paths. Invalid metadata falls back to
the current renderer origin and never changes the authenticated transport.
Account-recovery copy is explicit in all nine locales. The screen remains
scrollable in short windows, with visible input focus and bounded field widths.

The first login regression passed 46 of 49 checks. Three desktop-metadata fixtures
omitted the existing required theme bridge, causing their renderer to fail before
login. Adding the actual theme bridge contract corrected those fixtures; the
focused login/stateful suite then passed **25/25**, including successful sign-in,
rejected credentials, duplicate-submit prevention, safe metadata, all nine locales
and both themes at 320px. English and Arabic light/dark screenshots were inspected.
The unchanged screen-size/zoom checks passed in the initial combined run.

Native Tools now reads its inventory without depending on MCP/external-provider
availability. Connections presents external readiness failures beside their own
section, retains readable MCP state and offers a read-only retry. Pending setup
holds input fields stable; editing a tested endpoint clears its obsolete result.
Restricted accounts still cannot mount connection administration or issue its
requests. These presentation changes do not replace server authorization.

The connection/task-management/interaction/handoff suite passed **46/46** after
correcting two new fixture assumptions: tools inventory includes `?all=true`, and
the existing localized retry label is “Try again”. No permission, result or
no-mutation assertions were removed. Commands:

```text
npm --prefix apps/web run build
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe login-experience.spec.ts stateful-ui.spec.ts workspace-experience-quality.spec.ts --workers=3 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe login-experience.spec.ts stateful-ui.spec.ts --workers=3 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe connection-readiness.spec.ts task-management.spec.ts interaction-contracts.spec.ts workspace-handoffs.spec.ts --workers=3 --reporter=line
```

Private evidence uses `login-*`, `connection-*` and `setup-*` under the same
whole-product-review directory. Task 16.15 remains open for the rest of the
whole-product readiness/empty-state review. No installation or publication was
performed; real-device, screen-reader and participant evidence remains separate.

Final combined renderer verification passed **583 tests, 27 skipped, zero
failures** in 6.3 minutes (`setup-full-renderer.log`):

```text
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe --workers=3 --reporter=line
npm --prefix apps/desktop test
node --test scripts/maintenance/check-docs.test.mjs
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-source-size.mjs
openspec validate workspace-outcome-experience --strict
```

Desktop unit/contract tests passed **92**, with **1 skipped**; documentation checker
tests passed **7/7**. Maintained documentation, source-size and strict OpenSpec
checks passed. This does not establish installed native rendering, assistive
technology compatibility, model quality or production readiness. The full suite's
wall time is recorded, not used as a same-hardware performance comparison.

## Whole-screen hierarchy, boundaries and placement — 2026-10-31

This pass covers task 16.14 and the remaining source/UI acceptance for 16.15.
It incorporates the user's explicit follow-up about excessive outlines and poor
placement throughout the app, including the Knowledge actions and composer.
The existing design now records that direction instead of treating the earlier
outlined-action rule as permanent. Palette values, service APIs and authority
remain unchanged.

The source review and screenshots found mismatched Activity content widths,
unbounded Settings controls, repeated Service labels, an installed-model heading
that implied readiness, misleading create-agent guidance when profiles already
existed, and empty filters without recovery. These are corrected at their owners:
aligned Activity sections, bounded responsive Settings rows, accurate inventory
counts, explicit profile selection, and local Clear search/focus restoration.
Empty model inventory offers discovery without starting a download; restricted
areas provide an administrator explanation and return to the retained Chat draft.
All new permission and profile-selection guidance has nine explicit locale copies.

Shared controls now distinguish solid primary, tonal secondary and quiet utility
actions. Profile editing and tool/skill lists no longer add redundant surrounding
frames. Input fields, popovers, dialogs, errors and focus retain their boundaries.
Forced colors restores secondary and selected-tab boundaries. A new touch fixture
found that Send was still 40px; the shared coarse-pointer rule now includes it,
fields, theme choices and response/document controls at a 44px minimum.

Organization changed as well as styling. The composer groups agent/model/workspace
choices separately from the lower context/voice utility group and microphone/
Send/Stop actions. Knowledge keeps Add primary, Ask secondary, and management
quiet beside its affected source. Model comparison is a compact hint until models
are selected. The profile list is wider on desktop, with instructions/resources
sharing the editor width and no enclosing form card.

Validation history is retained, including failures:

- Initial hierarchy run: 41 passed, 19 failed. The responsive Settings rule lacked
  its own container, a fixture treated a label-only unavailable row as a control,
  RTL alignment compared left edges, and the legacy navigation selector was wrong.
  Fixes preserved the alignment/no-write requirements; the next run passed 60/60.
- Permission/continuity regression: 72/72 passed, covering all nine locales,
  forbidden administration calls, draft return and independent connection errors.
- First shared-boundary regression: 62 passed, 5 failed. It exposed the actual
  Send touch target and a wrapper-specific microphone selector. Two keyboard
  checks needed keyboard modality, and profile editing needed a valid effective-
  profile fixture. After correction the same suite passed 67/67.
- Initial full renderer after the visual change: 625 passed, 27 skipped, 2 failed.
  The two old category tests required border differences. They now verify the
  intended visible surface difference before/after hover, selected weight and
  unchanged selection semantics; forced-colors boundaries are tested separately.
- Composer placement/model-comparison/voice regression: 52/52 passed, including
  responsive/RTL grouping, model pinning, no overlap and no unintended mutation.
- Final opt-in visual matrix: 8/8 passed (19.0 seconds), light/dark, English/Arabic,
  390px/1440px. It captures six routes plus agent editor, tools, connections,
  discovery, empty/loading/error, saved/streaming Chat, palette, workspace popover
  and onboarding. Files are private build artifacts under
  `build/ui-craft/placement-final-20261031/`.

Rendered images directly inspected include the wide English light composer and
Activity, narrow Arabic dark composer/popover/discovery, English dark Knowledge,
wide agent editor and Connections, and English Settings. The matrix proves fixture
containment, not that every image was manually assessed or that native desktop,
physical touch, screen-reader or translation-speaker qualification passed.

Commands (all renderer traffic uses isolated service state and API fixtures):

```text
npm --prefix apps/web run build
npm --prefix apps/web run api:check
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe action-hierarchy.spec.ts screen-hierarchy.spec.ts control-consistency.spec.ts compact-workspace.spec.ts composer-hierarchy.spec.ts --workers=3 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe action-hierarchy.spec.ts composer-hierarchy.spec.ts model-library-experience.spec.ts model-comparison.spec.ts voice-playback.spec.ts --workers=3 --reporter=line
# With OFFGRID_VISUAL_PHASE=placement-final-20261031:
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe visual-craft.spec.ts --grep "visual craft (light|dark) (en|ar) (390|1440)" --workers=2 --reporter=line
```

API contract, TypeScript/build and source-size checks passed. Desktop unit/contract
checks passed 92 with one skipped; documentation checker tests passed 7/7. The
pre-existing Vite chunk-size warning remains. No installed workspace, microphone,
model, service process or desktop installation was changed by this pass.

Final integrated renderer rerun on the completed placement/boundary source passed
**627 tests, 27 skipped, zero failures** (4.1 minutes), recorded in
`build/workspace-outcome-experience/hierarchy-full-renderer-r2.log`:

```text
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe --workers=3 --reporter=line
```

The 281 maintained-document check reported zero issues; strict OpenSpec validation
passed. Tasks 16.14 and 16.15 are complete for their source/UI acceptance. Remaining
manual, installed-platform, broad integration and participant tasks stay unchecked.
No whole-program production-readiness or competitor-parity claim is made.

### Packaged layout and observed feedback follow-up — 2026-10-31

Built a fresh unpacked Windows x64 desktop into
`build/workspace-outcome-experience/desktop-quality-20261031/` using:

```text
# From apps/desktop:
npx.cmd --no-install electron-builder --dir --win --x64 --publish never --config.directories.output=../../build/workspace-outcome-experience/desktop-quality-20261031
# From the repository root (also run with --narrow):
node scripts/test/test-desktop-theme.mjs "build/workspace-outcome-experience/desktop-quality-20261031/win-unpacked/OffGrid LLM Desktop.exe"
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-search.exe workspace-experience-quality.spec.ts --workers=1 --reporter=line
```

The packaged Windows checks passed in normal and narrow-window modes: live and
saved native theme, nine language options, keyboard language selection, actual
Chromium 200% zoom across seven routes, visible focused controls, no horizontal
overflow, and saved dark appearance after restart. These use a temporary profile
and a local HTTP fixture serving the built renderer, not the installed workspace
or real inference. Captures remain private under temporary directories
`offgrid-desktop-theme-0WKUyz` and `offgrid-desktop-theme-qdvNaD`. A reduced contact
sheet of Chat, Agents, Models and Settings was directly inspected; keyboard focus
outlines remain deliberately visible. Other captures are not claimed manually
reviewed.

Initial harness failures are retained: a Windows native language selection could
dismiss its parent popover, so the harness now reopens through its visible trigger
before resetting the language. The first narrow restart resized before the first
BrowserWindow existed; it now waits for `firstWindow()` before sizing. Neither
change forces hidden controls or alters application behavior. Authoritative logs:
`quality-packaged-theme-r2.log` and `quality-packaged-theme-narrow-r2.log` under
`build/workspace-outcome-experience/`.

History measurements now require the actual matching DOM rows before two paint
frames, rather than timing frames alone. Three repeated 1,000-conversation fixture
runs measured p95 **36.7 / 38.5 / 40.7 ms**, below the unchanged 250 ms gate.
Document-reload and warm hash-navigation costs are reported separately and wait
for route content. Reload uses normal browser caching; it is not a cold-machine
or empty-cache measurement. The first stricter Agents check exposed a fixture
missing `agent-profiles-v1`; the fixture now supplies valid empty profile/resources
responses and measures the loaded profile view instead of an unsupported-feature
message. The final complete quality suite passed **24/24**, with p95 **34.3 ms**
and all six routes measured (`quality-final-r3.log`). Prior failure logs remain.

This strengthens measurement; it is not a before/after speedup claim because the
previous benchmark had different completion criteria. No model latency improvement,
manual screen-reader pass, macOS/Linux qualification, installed update test or
participant study is inferred. Tasks 6.2, 6.3, 8.4 and the broader qualification
gates remain open. No installed app, service, model or user configuration changed.

### Windows worker distribution repair — 2026-10-31

Task 16.5's missing-DLL defect is reproduced and repaired at the release owner.
The pinned source is `b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9`.
Its `LLAMA_OPENSSL` option defaults on; the old `LLAMA_CURL=OFF` argument did not
disable cpp-httplib's OpenSSL linkage. The release workflow now sets
`LLAMA_OPENSSL=OFF` and `LLAMA_BUILD_UI=OFF`: the managed worker uses loopback
HTTP and OffGrid's own renderer, downloads and service TLS are unchanged.

The Windows bundle previously also omitted required MSVC/OpenMP DLLs. The new
`scripts/release/prepare-windows-runtime.ps1` resolves only the CMake-selected
MSVC toolchain's redistributables, walks the worker's dependency closure, records
hashes and rejects unknown dependencies instead of searching developer PATH.
OpenMP remains enabled. Only Windows system libraries/API sets are exempt from
app-local packaging. A bounded `--version` check uses a system-only PATH. DLLs
and the dependency manifest enter release checksums; manual-copy instructions
include the DLLs. The create-bundle step now fails on assembly/check errors.
The Windows ZIP is expanded into a unique temporary directory and rechecked
before artifact/release upload; verification cannot fill missing dependencies.

Verification performed locally:

- Fresh native CPU worker build: 322 build steps, exit 0, MSVC 14.44, static
  ggml/llama with OpenMP enabled. Output is isolated under
  `build/workspace-outcome-experience/portable-cpu/`; no installed runtime changed.
- Actual worker dependency closure and restricted-environment `--version` passed:
  worker plus `msvcp140.dll`, `vcomp140.dll`, `vcruntime140.dll` and
  `vcruntime140_1.dll`. Private manifest is in `portable-cpu-bundle/`.
- The prior published 0.4.21 worker was copied to an isolated test directory and
  rejected for `libcrypto-3-x64.dll`, reproducing the missing dependency rather
  than accepting this developer machine's globally available libraries.
- Compiled MSVC/OpenMP fixture tests passed for valid closure/version, ZIP
  roundtrip, missing
  dependency, altered DLL, verify-only non-mutation and an unapproved imported
  DLL. The regression now runs as a Windows CI job. The first local attempt had
  no CMake on PATH; rerunning with the installed Build Tools CMake passed. The
  fixture uses the Visual Studio generator also selected by the release.
- An isolated actual Qwen 2.5 3B CPU request returned `Hello!`, finish `stop`,
  36 prompt/3 completion tokens. Load/readiness was 3.066 seconds and request
  0.888 seconds on this machine. The worker used 2 threads, 1,024 context,
  an ephemeral loopback port/key and system-only PATH, then exited. This is one
  smoke reply, not an agent workflow, clean-install test or performance promise.
  Existing model bytes were read, not imported or changed; installed GPU settings
  were not changed.

Commands and retained logs:

```text
powershell.exe -NoProfile -File scripts/release/test-windows-runtime-bundle.ps1
powershell.exe -NoProfile -File scripts/release/prepare-windows-runtime.ps1 -BundleDirectory build/workspace-outcome-experience/portable-cpu-bundle -CMakeCache build/workspace-outcome-experience/portable-cpu/CMakeCache.txt -VerifyOnly
node scripts/release/test-release-ci.mjs
node scripts/release/test-release-context.mjs
```

`portable-cpu-build.log`, `windows-runtime-regression-r3.log`,
`published-runtime-rejection.log`, `portable-cpu-reply.json` and
`portable-cpu-worker.log` remain private build evidence. The release-context test
initially selected WSL Bash for Windows paths and failed; it passed under both
Git Bash and WSL's native Node. The complete existing release-script suite passed
**5/5 under WSL**, its supported POSIX environment; three scripts failed under
Windows/Git Bash because their POSIX command fixtures did not execute correctly.
The actual CPU worker's ZIP roundtrip also passed. Actionlint 1.7.12 passed on
both changed workflows using its cached WSL module source: direct Windows module
resolution timed out, and the first offline versioned invocation attempted an
unavailable deprecation lookup. The final offline source invocation succeeded
(`runtime-actionlint-final.log`). Documentation (281 files), source-size and
strict OpenSpec checks passed. No release was uploaded,
tag changed or installation replaced. Task 16.5 remains unchecked: complete
fresh-install/first-reply/restart/sleep/update qualification across Windows,
macOS and Linux has not been established by this repair.

### Real Windows desktop first reply and retained conversation — 2026-10-31

Added `scripts/test/test-desktop-native-reply.mjs` to close a gap left by HTTP
fixtures. It launches the packaged Electron application and its actual owned Go
service in a new home and profile, with an ephemeral loopback port. Installed
OffGrid environment settings, credentials and configuration are not inherited.
The only model and runtime inputs are explicit local artifacts copied into the
test workspace and SHA-256 checked. This is a prepared offline-artifact test,
not an online download or clean installer qualification.

The service in the prior isolated desktop build was an older staged executable;
the theme/layout tests had not exercised its inference. Before this test, current
Go source was built into `offgrid-native-qualification.exe` with version 0.4.21
and copied only into the private unpacked test application's `resources/bin`.
Its previous test executable was retained. Neither installed desktop nor service
was changed. No publication or source-version bump occurred.

The real journey verifies:

- Desktop starts and owns the native service; model/data paths belong to the
  temporary profile, not an attached external service.
- The first-use dialog reports a usable model and opens Chat normally. Setup
  remains incomplete until a real answer is saved.
- A user submission from the composer produces a nonempty completed answer.
  Independent service reads verify exactly one user/assistant pair with the same
  prompt and answer shown by the renderer.
- Quit stops the owned service. Relaunch retains workspace identity, conversation
  bytes and first-reply completion, with no second generation request.
- The original model and copied model retain their hashes. No browser request
  leaves the test origin, and runtime discovery uses the prepared worker files.
  This does not assert a system-wide offline firewall or all-service egress audit.

The first two runs exposed harness mistakes: the exact first-use button text was
`Start your first chat`, and generation POSTs use `/generate`, while `/turn` is a
read endpoint. Both were corrected; prior failures remain in build logs. The
next run passed with Qwen 2.5 3B on CPU, one submission, and reply `hello there`
in 3.436 seconds including load. Its actual native first-reply screenshot was
directly inspected. Model response semantics beyond this greeting and all agent
workflows are not inferred from that result.

The final harness rerun, including recorded desktop/service/worker/renderer
identities, also passed: 3.454 seconds, the same greeting, one submission and
retained conversation after restart (`desktop-native-reply-final.log`). Private
result JSON is in `offgrid-desktop-native-reply-EpPSnD` under the OS temporary
directory. Both are fresh worker processes with prepared model files and warm
OS file caches, not cold-machine measurements. Syntax, 281-file documentation,
source-size and strict OpenSpec checks passed after the harness changes.

The existing packaged startup/recovery suite also passed with the current
service, including incompatible-service recovery, explicitly isolated service,
retained isolated data, compatible cross-version attachment, revalidation after
reload, remembered external ownership, delayed-service reconnect and dark/
reduced-motion keyboard recovery. First window was 248 ms, recovery 314 ms in
this one run (`native-startup-current.log`); these are not latency guarantees.

```text
go build -trimpath -ldflags "-X main.Version=0.4.21" -o build/workspace-outcome-experience/offgrid-native-qualification.exe ./cmd/offgrid
node scripts/test/test-desktop-native-reply.mjs DESKTOP_EXE LOCAL_MODEL_GGUF VERIFIED_CPU_RUNTIME_DIRECTORY
node scripts/test/test-desktop-startup.mjs DESKTOP_EXE
```

The explicit runtime input must contain the dependency/hash manifest generated
by `prepare-windows-runtime.ps1`. The new harness currently qualifies Windows CPU
only, has bounded waits, and records binary/runtime/renderer identities. It does
not acquire models or approve native access. Private test profiles and evidence
remain in OS temporary directories. Full fresh installer/update/sleep/wake,
macOS/Linux, document-grounded and reviewed-agent journeys, assistive technology
and participant gates remain open.

### Placement follow-up and real Knowledge journey — 2026-10-31

Knowledge management now sits beside Add document in the library heading. Asking
about sources is a separate action group beside its scope explanation, rather
than a row of equally weighted outlined buttons. Models verification uses the
shared secondary treatment. Fresh English/Arabic screenshots at 320 and 1440 px
were inspected for placement, wrapping and visible keyboard focus. The applicable
action/composer/screen/Knowledge/model-library batch passed 77 tests in 41.7 s
(`placement-followup-final.log`). Four earlier fixture failures came from treating
a localized resource function as an object; those logs remain retained.

The packaged Windows harness now optionally accepts an explicit local embedding
GGUF. With copied Qwen 2.5 3B and BGE-M3 artifacts, it ingested two disposable
documents through the actual Knowledge page, selected only Mariner, handed off
to Chat and produced the correct code/date with its source name. Independent
service reads checked the selected document identity, exact supporting passage
and passage digest, excluding the unrelated Atlas facts. Restart preserved the
conversation and evidence with exactly two total generation submissions.

`desktop-native-knowledge-r2.log` passed: greeting 5.005 s, grounded answer
8.425 s. Private result/screenshots are under OS temporary directory
`offgrid-desktop-native-reply-0U6kUO`. The original attempt incorrectly assumed
that an Enable button would be present; the service already enabled the installed
embedding model. That harness failure is preserved. This is one actual Windows
CPU document-grounded journey, not general retrieval or cross-platform qualification.

### Approved image input: implementation and boundaries — 2026-10-31

Ordinary saved Chat now supports PNG/JPEG attachment, clipboard paste and drop,
previews, removal, image-only prompts and retained image history. Text remains a
string for compatible clients. Images travel as validated typed content only
when a local model has an installed projector; the shared inference path checks
again before model loading. Agent image turns and unsupported legacy submissions
are rejected. There is no download, silent model switch or URL fetching.

Limits are four images per turn, 2 MiB decoded per image, 8192 px per edge,
32 megapixels, and 12 MiB total encoded conversation image content. The service
validates both format headers and complete decoded pixels. Request identity
includes image bytes/name. Scoped session persistence stores pending input and
completed user-message attachments, with atomic history-budget admission and
no duplicate completed copy. Interrupted turns retain their input without replay.
Unsent drafts use existing actor/workspace/conversation scoping; browser quota
failure is visible and retains the in-memory draft, without promising reload
recovery for a failed browser-storage write.

Automated evidence:

- `go test ./pkg/api ./internal/sessions ./internal/server` passed, including
  existing session/API compatibility tests (`image-backend-full.log`).
- Focused boundary tests passed after the atomic budget/recovery changes
  (`image-boundary-tests.log`); final PNG/JPEG, persistence and rejection checks
  passed again after the JPEG-positive case was added.
- `CGO_ENABLED=1 go test -race ./pkg/api ./internal/sessions ./internal/server`
  passed in Ubuntu/WSL (`image-race-wsl.log`, server 131.769 s). The initial Windows
  attempt could not run with CGO disabled; it was not counted as passing.
- API generated-contract check, TypeScript/build, source-size ratchet, 281-file
  maintained documentation check, seven documentation-checker tests and strict
  OpenSpec validation passed. The existing >500 kB build-chunk warning remains.
- The full isolated renderer suite passed **650 tests with 27 skips**
  (`image-renderer-full.log`). After the final history-draft cleanup and added
  JPEG/text-drop regression, the targeted image/streaming batch passed **27 tests**
  (`image-final-targeted.log`). A prior raw Playwright invocation lacked the real
  service needed by streaming tests; the isolated service runner resolved that
  test-environment failure. Earlier failed logs remain available.
- The separate final history/continuity run passed **22 tests**
  (`image-history-final.log`), including removal of an unsent image draft when
  its conversation is deleted. The earlier targeted command named the nonexistent
  `chat-continuity.spec.ts`; it selected image/streaming tests only. The actual
  files are `history-controls.spec.ts` and `work-continuity.spec.ts`, exercised
  explicitly in this final run.
- Browser fixtures cover exact-byte upload/paste/drop, image-only send,
  success/error retention, history reload, quota errors, invalid/excess files,
  model/service rejection and keyboard/narrow layouts in all nine locales.
  English/Arabic captures and the real Windows attachment screenshot were
  inspected. These fixture image answers are not model interpretation evidence.

The actual Windows packaged renderer/service also passed the new attachment
journey (`desktop-native-images-r3.log`): a red PNG remained byte-identical after
quit/restart, text-only Qwen caused a visible readiness block, a direct image
submission returned 422 without creating a turn, and removal restored focus to
Add images. A real greeting completed in 6.644 s, and restart retained exactly
one generation submission and the saved conversation. This was an isolated
CPU/offline-artifact test; neither installed app nor user workspace was changed.

Private result/screenshots: `offgrid-desktop-native-reply-xn6Olo` in the OS
temporary directory. Tested service SHA-256:
`b3df93d59f783080e6e5db16a3c0d1d98033845ca9ff4d639fcaffa70d321691`;
renderer build identity:
`77d43c901bde12cc65d917d881d7c0868310524dd28f4b72c5c9e5d4015f59f5`.
The first native attempt used an unversioned test-service build and did not
reach the UI. The second reached Chat but was refused by actual memory
admission while race checks were running. After rebuilding with the matching
0.4.21 identity and finishing competing checks, the isolated run passed. Neither
failure was hidden by disabling compatibility or memory checks.

No installed compatible vision model/projector was available, so **actual image
interpretation remains unverified**. No model was downloaded to bypass that
constraint. Agent image execution, real macOS/Linux attachment journeys,
assistive-technology and participant studies remain unqualified. Image task 17.3
stays open for that evidence; implementation/fixture coverage does not close it.

```text
npm.cmd --prefix apps/web run api:check
npm.cmd --prefix apps/web run build
go test ./pkg/api ./internal/sessions ./internal/server
CGO_ENABLED=1 go test -race ./pkg/api ./internal/sessions ./internal/server
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-image-qualification.exe --workers=2 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-image-qualification.exe chat-images.spec.ts chat-continuity.spec.ts chat-streaming.spec.ts --workers=2 --reporter=line
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-image-qualification.exe history-controls.spec.ts work-continuity.spec.ts --workers=2 --reporter=line
node scripts/test/test-desktop-native-reply.mjs DESKTOP_EXE LOCAL_MODEL_GGUF VERIFIED_CPU_RUNTIME_DIRECTORY [EMBEDDING_GGUF]
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-source-size.mjs
openspec validate workspace-outcome-experience --strict
```

### Approved surface materials and nested navigation — 2026-10-31

Implemented the explicitly approved material prototype through the existing
generator and shared renderer: stronger neutral sidebar/composer ramps, bounded
96%-opaque floating panels with 12px blur, solid primary actions and opaque text.
The decorative budget remains three families. Reduced transparency, increased
contrast, forced colors, unsupported CSS and the diagnostic off switch restore
opaque fills. No model, authority, installed workspace or deployment changed.

A new regression reproduced Tab escaping the mobile navigation drawer while
Workspace options was open (English and Arabic: two failures). The focus scope
now distinguishes a contained nonmodal popover from a separate modal dialog.
The corrected focus/interaction/compact batch passed **27 tests**. Escape closes
the menu first, then the drawer, with focus restored at each boundary.

Initial current-renderer captures covered three layouts; the approved prototype
passed four light/dark desktop/narrow-RTL capture cases. The material/tonal batch
then passed **21 tests**, with its opt-in performance test skipped. Composited
floating-surface minimum contrast over black/white backgrounds was 4.9143:1
(light) and 5.5671:1 (dark); text/muted labels were higher. The prior conservative
sidebar/composer faint-text bounds remained 4.77835:1 and 5.50056:1.

The first performance attempt used Vite development serving and **failed both
tests**, including material-disabled samples, before timing out at the original
180-second limits. Retained evidence: `build/ui-craft/materials-performance-r1.log`
and its failure traces. This result is not discarded or presented as a pass.
The same tests against the built production renderer in a disposable service
passed **2/2 in 3.9 minutes**, Chromium 151.0.7922.34, one worker, unchanged
250ms Stop/frame gates. No performance implementation or thresholds were changed
between those runs; serving mode is a material distinction in this comparison.

| CPU | Materials | Theme | Menu feedback ms | Stop feedback ms | Frame p95 ms | Paint total ms |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| 4x | off | light | 89.0 | 45.9 | 50.0 | 374.113 |
| 4x | off | dark | 91.6 | 40.0 | 33.4 | 365.648 |
| 4x | on | light | 107.6 | 61.9 | 50.0 | 406.332 |
| 4x | on | dark | 88.2 | 52.2 | 50.0 | 451.553 |
| 6x | off | light | 132.9 | 66.0 | 83.2 | 418.042 |
| 6x | off | dark | 82.4 | 81.9 | 83.3 | 407.711 |
| 6x | on | light | 138.6 | 64.5 | 100.0 | 424.687 |
| 6x | on | dark | 142.3 | 72.4 | 83.4 | 362.471 |

These are single synthetic samples per combination, not hardware qualification
or a claim of zero overhead. For example, 4x dark paint increased 23.5%; 6x dark
paint decreased 11.1%. All response/frame samples stayed below the absolute gate.
The unchanged stream/download comparison also passed: Stop 31.1–104.5ms, stream
frame p95 50–83.5ms. Blur is restricted to floating surfaces and is absent from
the transcript/canvas. No inference speed claim follows from these measurements.

API generation, TypeScript/build, source-size and generator checks passed.
Production bundle: CSS 124.49kB (gzip 23.30), JS 1069.84kB (gzip 344.81), worker
153.51kB; the existing chunk-size warning remains. Desktop unit suite: **92 passed,
1 skipped**. This does not establish a fresh installed Electron visual result.

The historical whole-declaration color inventory against `7357aca` still reports
drift from earlier approved border/action restructuring. Its failure is retained
in `build/ui-craft/material-palette-audit.json`; the baseline was not broadened
to bless those changes. A separate exact comparison confirmed that all authored
palette/type tokens before the generated material block are unchanged from HEAD.
That narrower check does not claim the historical inventory passes.

```text
npm.cmd --prefix apps/web run test:e2e -- navigation-overlays.spec.ts interaction-contracts.spec.ts compact-workspace.spec.ts --workers=1 --reporter=line --output=../../build/ui-craft/overlay-focus-regression
npm.cmd --prefix apps/web run api:check
npm.cmd --prefix apps/web run check
npm.cmd --prefix apps/web run build
node scripts/maintenance/generate-ui-gradients.mjs --check
# OFFGRID_PAINT_MEASURE=1; development run failed, production run passed:
npm.cmd --prefix apps/web run test:e2e -- surface-materials-performance.spec.ts tonal-gradients.spec.ts --grep CPU --workers=1 --reporter=line --output=../../build/ui-craft/materials-performance-r1
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-image-qualification.exe surface-materials-performance.spec.ts tonal-gradients.spec.ts --grep CPU --workers=1 --reporter=line --output=../../build/ui-craft/materials-production-performance-r1
npm.cmd --prefix apps/desktop test
```

Final built-renderer regression run passed **682 tests, 12 environment-gated
skips, 7.5 minutes**, with `OFFGRID_VISUAL_PHASE=materials-final` and performance
opt-in disabled (the two stress tests had already passed separately). This
includes all 16 visual cases: both themes, English/Arabic, widths 390/768/1024/1440,
all six routes, agent editor, tools/connections, empty/error/loading states,
saved/streamed Chat, command palette and Workspace options. Final captures live
under `build/ui-craft/materials-final`; representative route, menu, desktop and
narrow-RTL captures were visually inspected. Unsupported-enhancement simulation
also passed; it is not evidence from an actual old browser.

The full-run large-history feedback fixture measured p95 **40.4ms**. Warm route
cost was 21–54ms; cold route cost was 229–392ms, reported separately rather than
described as a sub-250ms cold-start result. Documentation checker: 281 maintained
Markdown files, zero problems; its 7 tests passed. Source-size, generated token
consistency, strict OpenSpec and `git diff --check` passed (Git emitted only line
ending notices).

```text
# OFFGRID_VISUAL_PHASE=materials-final; OFFGRID_PAINT_MEASURE unset
node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-image-qualification.exe --workers=2 --reporter=line --output=../../build/ui-craft/materials-full-results
node --test scripts/maintenance/check-docs.test.mjs
node scripts/maintenance/check-docs.mjs
node scripts/maintenance/check-source-size.mjs
openspec validate workspace-outcome-experience --strict
git diff --check
```

Real vision interpretation, reviewed-agent native completion, manual assistive
technology, representative participant studies and remaining installed-platform
gates remain open. Material fixture results do not qualify the whole release.

The subsequent [native-agent diagnosis](native-agent-termination-2026-10-31.md)
confirmed a generated filename-grammar defect and added a candidate repair
without changing tool authority. The first correctly assembled candidate reached
one completed artifact action, but failed exact-content read-back because a
requested trailing newline was missing. The original timeout and this newer
failure remain recorded; task 16.17 is still open. A missing Desktop folder in
the disposable test home was also corrected without changing installed data.

The final candidate then passed the complete packaged journey **three times**
(82.033s, 92.502s, 82.062s) after preserving the prompt-cache prefix across tool
continuations and clarifying exact whitespace handling. Each pass independently
checked exact artifact bytes, preview and restart without replay. See the linked
diagnosis for model/runtime hashes, earlier failures and scope. This resolves
the observed Windows CPU artifact blocker, not the remaining cross-platform,
vision, assistive-technology or participant gates in 16.17 and related tasks.
