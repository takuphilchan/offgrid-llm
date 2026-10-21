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
[companion setup and limitations](../../../apps/computer/README.md). It is not a signed
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
[ARCHITECTURE.md](../../architecture/overview.md#bounded-durable-coordination).
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

### Conversation-work UI and specialist handoff check — 2026-10-14

- Chat now follows growing streamed/agent activity inside its own scroll pane,
  without a second page scrollbar or stealing the user's deliberate scroll-up.
  Chat and Agents history use the same compact, accessible search and overflow
  control. The Work inspector no longer describes profile configuration as its
  purpose. The pre-access agent guidance now describes task-wide desktop access,
  not the retired per-window sequence.
- A durable runner regression verifies a dependent researcher-to-document-draft
  handoff with pinned child profiles; child work remains read-only, and the
  parent owns any actual document editing and verification. This is a synthetic
  runner fixture, not evidence of native OS or MCP-service completion.
- Current checks: `go test ./...` passed; web type check/build and API contract
  check passed; 65 desktop unit tests passed; focused current Chat/history
  browser suite passed 30/30 and responsive/locale UI suite passed 33/33.
  At this checkpoint the full isolated browser suite was **not green**: 313 passed, 43 failed,
  10 skipped. Failures still exercise the removed Agents task composer and old
  access picker; they must be migrated to the current Chat/profile workflow and
  rerun. No broad-release or full-web qualification follows from the focused
  passes. Native Windows/macOS/Linux workflow and real-model repetitions remain
  unverified.

### Search-surface inventory — 2026-10-15

- Chat and task history, Agent Profiles, Knowledge documents, installed models
  (legacy and package inventory), and both model-discovery forms now use the
  same compact search field with a visible placeholder, persistent accessible
  label, shared icon/focus surface, and no separate stacked caption. The quick
  action palette retains its own combobox control so active-option keyboard
  semantics and Escape dismissal remain intact.
- `npm.cmd --prefix web/app run build` passed. Focused browser checks passed:
  26/27 initially (one stale size assertion), then 10/10 for the corrected
  control suite; model/control rerun passed 21 with six opt-in real-service
  acquisition cases skipped. After consolidating duplicate search CSS, the
  profile/history/control suite passed 24/24, including narrow and RTL layouts.
  At this checkpoint, 43 full-suite failures from retired Agents-composer tests
  remained unresolved. The later integrated rerun below supersedes that browser
  status; these targeted checks alone did not close the release gate.
- The existing local `offgrid` service was replaced with
  `offgrid-llm:unified-search-preview-20261015`; one service remains running.
  Workspace identity, task states, conversations, model inventory and speech
  readiness matched before/after replacement. RTX 5060 enumeration and the CPU
  runtime executable check passed. The verified backup is
  `/home/phil/offgrid-unified-search-backup-NL4buAc6`; the previous container is
  retained stopped as `offgrid-before-unified-search-1792020409`.
- The local desktop 0.4.16 preview was reinstalled and nine installed package
  files matched the staged package. Service and desktop UI identities both equal
  `6789a4ad09f102719e076434d230eb11db80b2aabb211b930687b548b1c755ed`.
  Application/state backups are retained in
  `D:\offgrid-llm-backups\desktop-unified-search-20261015`. This local update is
  not a published release or native workflow qualification.

### Remaining-work audit and integrated browser rerun — 2026-10-15

- Completed the current-renderer search inventory: Chat/task history, profiles,
  Knowledge, installed model inventories and model discovery use `SearchField`.
  Quick actions intentionally retains its accessible combobox and active-option
  behavior. No second legacy search-field implementation was found.
- Migrated retired task-composer tests to profile-enabled Chat and its canonical
  work inspector using a shared test-only conversation/job fixture. Access,
  uncertain-submission, navigation, legacy-link, voice, setup-return and history
  assertions still check their behavior rather than restoring the removed page.
  Profile/policy selections are session-memory choices: tests explicitly select
  them again after reload instead of claiming they were persisted.
- Narrow-screen testing found and fixed an unnamed Chat action button: its text
  is visually hidden on small screens, so Send/Stop/Send instruction now retain
  an explicit localized accessible name.
- The built renderer and freshly built Go service passed the full isolated
  browser suite twice: **348 passed, 10 skipped**, with four workers and with the
  specified single worker. Disposable service logs are respectively under
  `offgrid-web-qualification-JedT6A` and `offgrid-web-qualification-LHCKwA` in the
  Windows temporary directory. The former 43 browser failures are resolved.
- Seven skipped cases passed separately using the `modeltestfixtures` binary:
  six acquisition scenarios (Whisper, Piper, Zipformer, Kokoro, explicit variant
  selection and cancellation/resume/discard) and one real service restart case.
  These use synthetic package bytes and disposable storage, not downloaded
  production models or speech-quality qualification. The other three opt-ins
  require explicit installed-task UI or seeded task-service fixtures and were
  not enabled in this run.
- Desktop unit tests passed **65/65**; companion tests passed **73/73**. Web type
  checking and API-contract drift checks passed. Targeted Go packages
  `internal/agents`, `internal/server`, `internal/computer`, `internal/sessions`
  passed (cached). Documentation tests passed **6/6** and the maintained-doc
  check found zero problems. `git diff --check` passed with line-ending warnings.

Acceptance mapping for `agent-conversation-workflows` (contract evidence is not
real-model or native-platform qualification):

| Proposal acceptance | Recorded check coverage | Remaining evidence |
| --- | --- | --- |
| Greeting without unnecessary tools | `conversation_workflow_test.go`; `agent-chat.spec.ts` | Three installed-model repetitions |
| Requested application and full catalog | `application_discovery_test.go`; verified-OBS runner test | Installed-model repetitions and real launch evidence |
| Exact unsaved native document | `native_document_workflow_test.go` runner/host fixtures | Installed Notepad and other OS/application checks |
| Pre-effect repair versus uncertain effects | `conversation_workflow_test.go`, `computer_receipt_test.go`, `agent_reconciliation_test.go` | Real-model recovery matrix |
| Crash-safe turn/job linkage | `conversation_jobs_test.go`; Chat/lost-acceptance browser journeys | Broader installed/soak qualification |
| Profiles, tools and accessible Chat | `agent-profiles.spec.ts`, `agent-chat.spec.ts`, workspace/voice/locale browser suites; desktop/companion contracts | Manual assistive-technology, usability and cross-platform installed evidence |

The current change has no remaining failed browser gate. Its installed-model
matrix (task 7.3) is still unverified; it is not satisfied by synthetic tools.
The 11 unchecked native-document tasks include provider/harness acceptance as
well as platform execution evidence and must not be summarized as only manual
testing. The separate workspace-experience change still requires the manual
screen-reader pass, baseline performance comparison, participant study and
cross-platform installed checks. The voice-model foundation's remaining task
checkboxes require a dedicated reconciliation against acquisition implementation
and evidence; this UI audit does not silently certify them.

The current built UI identity is
`cf857bf6d98b57a8bba55e8759cf18a32661d4ced5ea52107d444ce2b4c6693e`.
The earlier deployed search preview does not yet include the final mobile action
label fix. No new deployment, commit, push or publication is part of this audit.

### Packaged presentation follow-up — 2026-10-15

- The isolated Windows packaged theme check exposed a stale harness assumption:
  its fixture omitted `agent-profiles-v1` and still expected the retired Task
  composer. Updated the fixture to advertise profiles and exercise the profile
  search plus the secondary activity inspector, with an explicit assertion that
  neither route restores the old composer.
- `node dev/scripts/test-desktop-theme.mjs
  "build/release-0.4.17-local/desktop/win-unpacked/OffGrid LLM Desktop.exe"`
  passed: light/dark/system native and renderer appearance, nine readable locale
  options, 200% zoom on seven routes, and saved appearance after restart.
  Screenshots and isolated profile evidence are in
  `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-theme-Vsv4QK`.
- This is packaged Electron presentation evidence against a local HTTP fixture,
  not a real-model, native-control, or other-platform qualification. No live
  container replacement, desktop installation, push, or release was performed
  for this follow-up; prepared local build artifacts remain unpublished.

### Chat response and attachment regression follow-up — 2026-10-15

- Fixed the extra top margin on **Inspect work**, aligned response actions,
  and applied Chat Markdown typography to live agent results. Notices, nested
  approval actions, access cards, activity disclosures, technical payloads and
  composer resource popovers now have explicit spacing and wrapping rules.
  Browser assertions measure action alignment and gaps at 1280px and 390px in
  light/dark themes, including expanded steps and error/retry controls.
- Read-only inspection of saved turns found missing attachment inventories, not
  context overflow: one affected Knowledge turn used 1,495 of 8,192 estimated
  tokens with zero compactions. Similarity excerpts represented only one of two
  selected documents, while stale persona/history influenced resource answers.
  Another saved turn had no attached child agents. Later profile edits do not
  retroactively change those pinned turns.
- Admission now records effective document names independently of retrieval
  matches. Each inference receives current pinned document and child-agent
  inventories after old conversation history; compaction and computer-context
  replacement retain them. Empty matches permit inventory answers without
  claiming document contents; missing documents and retrieval failures still
  block admission. Child resources remain restricted, and all nine UI locales
  use the generic **Attached agents** label instead of **Research agents**.
- `go test ./...` and the local Go service build passed. Web build/type checking,
  API-contract drift checks and desktop unit tests (**65/65**) passed. The full
  isolated browser run passed **350 tests, 10 skipped**; final focused resource,
  response and nested-approval spacing checks passed **3/3**. The additional
  attachment-compaction regression passed after correcting its queued fixture
  to the running state required for durable compaction writes.
- Opt-in `TestProfileResourceAnswersInstalledModel` passed **15/15** checks using
  installed `Qwen2.5-7B-Instruct-Q4_K_M`: three repetitions each of document
  inventory, cited excerpt content, inventory without matching excerpts,
  attached-agent names/roles, and empty attachments. Synthetic isolated state
  was used; proposed tools were rejected before dispatch. Observed inference
  calls took approximately 0.42–1.91 seconds, including warm-cache effects;
  this is not a general latency or native-control qualification.
- The WSL race run passed `internal/agents` but failed the existing server
  concurrent-admission test with `SQLITE_BUSY`; a focused three-repeat run
  reproduced it. The same focused check also failed on an isolated clean
  archive of the existing HEAD, demonstrating that it predates this patch.
  No race-detector data race was reported. The failure was subsequently fixed
  by reserving the SQLite writer before reading the previous task snapshot;
  independently committed conversation intents can no longer invalidate a
  deferred read-to-write promotion. No inference or tool replay was added.
  A controlled writer-contention regression fails on the old implementation
  for both new and existing tasks and passes with the fix, including unchanged
  snapshot event deduplication. The original concurrent-admission test passed
  **10 consecutive race-enabled repetitions**. The final WSL race run passed
  `internal/agents`, `internal/server`, `internal/storage`, and
  `internal/sessions`; `go test ./...` also passed after this repair.
- Documentation tests passed **6/6**, maintained-document checks found zero
  problems, and strict validation of `agent-conversation-workflows` passed.
  Its broader installed-model/native qualification task 7.3 remains open.
- The authorized deployment follow-up found intermittent missing citations
  (two of three content answers) despite correct facts. Removed competing
  filename/numbered citation guidance and explicitly retained the excerpt's
  supplied citation convention. Two subsequent complete installed-Qwen runs
  passed **30/30** assertions, without dispatching tools. These are bounded
  regression results, not a guarantee of model compliance on arbitrary inputs.
- The updated service/UI was deployed by backing up and replacing the existing
  container; the stopped predecessor and verified private workspace archive
  were retained. Workspace identity, conversation names, job IDs/statuses,
  models and speech configuration were compared across replacement. The
  desktop bridge is unchanged; this update needs no native-host reinstall.
  No Git push, release tag or publication is included in this follow-up.

### Calmer workspace implementation evidence — 2026-10-15

- Continued `workspace-outcome-experience` against source HEAD `29d6860`, with
  the approved profile/Chat architecture retained. One contextual 280px sidebar
  replaces the second Chat sidebar; below 1024px the same content is a drawer.
  Search clearing, compact composer preferences, follow-latest recovery, aligned
  response actions and quieter activity/settings/profile surfaces are implemented.
  Shared palette/type/spacing tokens now live in `web/app/src/tokens.css`;
  retired history-drawer CSS was removed. No public API or storage migration.
- The renderer checks use disposable service state and API/media fixtures, not
  the user's container or native applications. Full run: **354 passed, 10
  skipped**. Coverage includes history, draft isolation, canonical work links,
  approval/recovery, voice, IME, RTL/nine locales, short windows, zoom and nested
  popover/dialog focus. Added explicit short-window visibility checks for voice
  and Send/Stop, and drawer focus/Latest regressions. Navigation cannot submit work.
- New six-page light/dark captures cover 1440×900, 1280×720, 390×844 and 320×480.
  Existing comparison fixtures additionally capture empty/populated/running/
  blocked/error/completed presentations; targeted suites exercise streaming and
  loading states. Representative screenshots were inspected and clipped composer
  actions, a redundant Activity minimum height and focus theft were corrected.
  Repeated checks also exposed native keyboard scrolling continuing after Latest.
  Following now corrects non-user scroll movement while fresh scroll intent still
  suspends it; the rapid keyboard-scroll/Latest regression passed **20/20** runs.
  Local artifacts: `build/workspace-outcome-experience/refinement-evidence/` and
  `refinement-final.log` in its parent directory. These are generated local
  evidence, not checked-in product assets or installed-platform certification.
- Measured 1,000-conversation filtering feedback p95 was 34–58.8 ms in successive
  runs (250 ms fixture gate). Route measurements include local transport and
  varied with concurrent tests; no cold/warm regression comparison or inference
  speed claim follows from these numbers. Final production assets: CSS 94.40 kB
  (18.10 kB gzip), JS 869.20 kB (270.91 kB gzip). Vite still reports the existing
  large-chunk warning; this pass does not claim a bundle-size optimization.
- TypeScript, production build, API-contract check, `go test ./...`, companion
  checks and desktop unit tests (**65/65**) passed. Documentation tests (**6/6**),
  maintained-document link checks and strict change validation passed. Native
  consent presentation changes preserve authorization and IPC boundaries. No
  opt-in native input tests were enabled. Race/Python/model-qualification suites
  were not rerun for this presentation-only change; earlier evidence is not a
  fresh full-CI claim.
- Remaining qualification is explicit: no new controlled pre-change image set,
  exhaustive human review of every state/viewport, manual screen-reader study,
  participant usability study, or matching installed Windows/macOS/Linux consent
  and desktop verification. macOS/Linux native presentation edits were not built
  on those platforms. Task 10.5 and the existing 6.2/6.3/7.2/8.4 gates remain open;
  implementation checkmarks do not certify those gates. No deployment, desktop
  replacement, commit, push or publication occurred in this pass.

### Calmer workspace local deployment evidence — 2026-10-15

- On the user's subsequent deployment approval, built the service, current UI,
  Windows runtime and packaged Windows desktop from the same working tree.
  Local image `offgrid-llm:calm-workspace-20261015` retains the existing CPU,
  CUDA and speech runtimes; this is a local 0.4.17 build, not a new publication.
  Installed desktop and service report UI fingerprint
  `156211ec5f6e860169377ca292d5482d434eba1dbe8d8787a5e2e7fd8a57bb74`.
- Backed up and verified the stopped service workspace before replacing the
  container. Preserved both named volumes, workspace identity, conversation
  names, job IDs/statuses, model inventory and speech selections. Retained the
  stopped predecessor and a private WSL backup. No saved task was resumed.
- Gracefully quit the old desktop, verified private data/settings backups and
  staged package hashes, then replaced the installation while retaining its
  uninstall integration and complete rollback directory. Installed-package
  connection checks confirm an external service, matching 0.4.17 versions and
  declared bridge compatibility. Closing the isolated desktop check leaves the
  container running. Evidence is in `build/calm-workspace-deploy-20261015/`;
  desktop backups are under `build/desktop-backups/before-calm-workspace-20261015/`.
- Packaged Windows startup/recovery, access-dialog layout at 100%/200%, native
  light/dark/system theme synchronization, nine locale options, persistence and
  all page routes at 200% zoom passed with isolated profiles and HTTP fixtures.
  The startup fixture temporarily tampers with package metadata, so access
  verification was rerun after it restored the manifest. Replaced the theme
  harness's timed-out hidden-window Playwright screenshot with Electron capture,
  matching its existing zoom capture method; the complete rerun passed.
- Initial post-start GPU device enumeration failed, causing the replacement
  script to stop the new service. After review, restarted that same new container:
  repeat CUDA enumeration passed, and a bounded Qwen completion returned `ready`
  with automatic NVIDIA offload and approximately 4.6 GiB GPU memory in use.
  The CPU worker version check also passed; a fresh CPU inference benchmark was
  not performed. Exactly one OffGrid service is running on loopback port 11611.
- These are local Windows deployment and presentation checks, not native-input
  workflow qualification or macOS/Linux verification. The previously listed
  usability/platform gates remain open. No commit, push, tag or release occurred.

### Composition-first workspace implementation evidence — 2026-10-16

- Continued the approved `workspace-outcome-experience` refinement from HEAD
  `29d6860`, preserving the prior uncommitted presentation changes. Captured
  populated Chat, running/reconnecting work and blocked access before editing
  at 1440×820 and 1120×820 in
  `build/workspace-outcome-experience/composition-before/`. These are controlled
  API fixtures, not evidence of native task execution.
- Implemented the 248px sidebar/64px collapsed rail, early New chat, compact
  contextual history and overflow actions, consolidated workspace footer, and
  concise running/attention status. Activity now owns the canonical full-width
  inspector, sharing `ConversationWork` with Chat. Old agent task links resolve
  to that same work; Agents retains profiles, tools and connections. Removed
  duplicate inspector/presentation branches without reviving the task composer.
  Administrative history/steering remain permission-gated; child status is not
  fabricated as waiting when it is unavailable. No API or storage migration.
- Final isolated renderer run: **358 passed, 10 skipped** in
  `composition-release-check.log`. The ten skips require separately opted-in
  model acquisition/restart, installed UI or real-task fixture environments;
  they are not qualification passes. Tests cover route aliases, drawer/collapse,
  draft retention, history confirmation, single canonical work, approvals,
  uncertain recovery, streaming/Latest, speech, all nine locales, RTL, keyboard,
  IME, short windows and 200% zoom. Navigation-only fixtures assert zero writes.
- Captured all six pages in light/dark at 1440×900, 1280×720, 390×844 and 320×480
  in `composition-release-check/`, plus the representative populated/running/
  blocked comparisons. Reviewed all 48 six-page responsive/theme captures;
  corrected narrow header wrapping and noninteractive heading focus styling.
  Automated overflow/action-visibility checks pass. This completes the automated
  integration and fixture-review scope of 11.7 and 10.5, not exhaustive human
  review of every operational state or the separate usability/platform gates.
- Filtering 1,000 fixture conversations measured **34.3ms p95**, below the 250ms
  local-feedback gate. This is not a cold/warm regression study or inference
  benchmark. Production assets: CSS **96.16kB / 18.40kB gzip**, JavaScript
  **868.13kB / 271.28kB gzip**. The existing large-chunk build warning remains.
- TypeScript/production build, API-contract check, fresh Go service build,
  desktop unit tests (**65/65**), documentation unit checks (**6/6**), maintained
  links and strict OpenSpec validation passed. Logs use the `composition-`
  prefix in `build/workspace-outcome-experience/`. This presentation pass did
  not rerun Go all/race, Python, companion or real-model qualification suites;
  prior evidence is not a fresh full-CI claim.
- Windows packaged-theme fixture passed with an isolated profile: native theme
  synchronization, nine language options, preference persistence and all page
  routes at 200% zoom. Evidence:
  `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-theme-nPdZR7`.
  It used the existing `desktop-calm-workspace-20261015/win-unpacked` host and
  current renderer fixtures, not a newly installed desktop package. No native
  input, live service changes, personal-document actions or consent grants ran.
- Manual screen-reader and participant studies, full timing comparison, matching
  installed desktop/access-dialog qualification and macOS/Linux checks remain
  unverified (6.2, 6.3, 7.2, 8.4). No deployment, installation replacement,
  commit, push or release was performed for this implementation request.

### Post-launch interruption investigation — 2026-10-17

#### Full-flow repair evidence — 2026-10-17

- The new action contract and progress guard were exercised through the
  production local inference client. The exact multilingual document workflow
  completed **3/3** times with Qwen2.5-7B-Instruct-Q4_K_M: one application
  launch, one new-document creation, one Unicode insertion, independent read
  back, no save, and 12 model decisions per run. Prompt usage stayed below the
  8,192-token context allocation (about 44.7k cumulative prompt tokens per
  run). This is an in-memory owned provider check, not Windows Notepad evidence.
- The browser action sequence completed **3/3** times with the same model and
  an owned synthetic Edge host: one launch, one query insertion, independent
  field verification, one Enter, and a fresh results observation. The host did
  not contact the public web, so this verifies action sequencing and recovery,
  not three-source Zimbabwe tourism research or a real Edge window.
- The collaboration harness now constrains delegated tools to each pinned
  profile, requires a complete researcher-to-writer graph when both attached
  specialists are explicitly named, preserves dependency results, and runs
  Knowledge-only children as content-producing calls. The deterministic fixture
  passes with durable reload checks. Three installed-Qwen attempts created the
  correct two-child graph, but the parent returned a roughly 100-word summary
  instead of the requested 300-word briefing; collaboration model qualification
  therefore remains failed and must not be reported as complete.
- New bumblebee tasks now receive a 30-decision profile budget (revision 6).
  Existing tasks were not resumed or modified. The live service still runs the
  prior image; no replacement or desktop installation was performed from this
  evidence alone.

- The saved Edge research task stopped after verified application launch, before
  search input. A read-only replay of its inference checkpoint reproduced the
  local runtime's `peg-native` structured-output parsing error twice. Its
  measured prompt fit the 8192-token runtime context; this was not evidence of
  context overflow. No replayed diagnostic dispatched any computer action.
- The constrained schema now serializes the tool name before its arguments.
  The exact checkpoint then returned a valid observation proposal in three
  repetitions with the installed Qwen2.5-7B-Instruct-Q4_K_M runtime, retaining
  the original response budget. This proves this decision checkpoint only,
  not completion of the public-source research request.
- Complete invalid decisions and the specifically identified runtime parser
  rejection enter the existing bounded, pre-dispatch repair path. Network,
  unknown runtime and incomplete transport failures do not become retryable;
  existing action receipts and uncertain-effect protections are unchanged.
- Broader installed-model document qualification failed: after launch, the
  model repeatedly selected observation/document reads and exhausted 24 turns
  without creating or editing the new document. This used the real local model
  with an in-memory native provider, not personal applications or documents.
  Experimental history/result normalization and tool ordering did not provide
  repeatable success (one repetition passed, the next failed) and were removed.
  These runs are failed qualification evidence, not successful workflow tests.
- Final focused Go suites and affected race suites passed for inference, agents,
  server and computer. Earlier 71 desktop tests passed; these do not close
  real native, model or other-platform qualification gates. Temporary diagnostic
  source files were removed; private checkpoint material remains outside tracked
  sources in ignored local diagnostic state, not ordinary exports.
- Replacement is withheld: the existing `full-access-repair-20261017` container
  remains running and the user-quit desktop remains closed. No saved task was
  resumed, no new native consent was granted, and the approved isolated Edge
  end-to-end test has not yet run with a replacement build. A full-flow fix must
  address planning progress as well as response parsing before being called
  complete. No commit, push or release occurred in this investigation.

### Composition workspace local replacement — 2026-10-16

- On subsequent explicit deployment approval, rebuilt the Linux service,
  Windows runtime/native companion and Windows desktop from the current source.
  Local image `offgrid-llm:composition-workspace-20261016` preserves the prior
  CPU/CUDA/speech runtime layers; version remains 0.4.17, not a new publication.
  Service and installed desktop share UI build
  `342480b5bf5019c7ebb9b1345755d661964bc969dcb635dceb5de0c5216b68c1`.
- Confirmed idle inference, no computer sessions or active download operations,
  and no executing tasks before replacement. Quit the desktop normally. Verified
  a stopped-workspace archive and retained the stopped predecessor container
  `offgrid-before-composition-workspace-1792127335`. Private service backup:
  `/home/phil/offgrid-composition-workspace-backup-zdbvUTis` in Ubuntu WSL.
  Both persistent volumes, loopback binding, GPU device, environment and restart
  policy were preserved; no saved task was resumed or reconciled.
- Hash-verified desktop data/settings backups and staged package files before
  replacing the application. Backup:
  `build/desktop-backups/before-composition-workspace-20261016/`. The intact old
  installation remains at
  `C:/Users/phil/AppData/Local/Programs/OffGrid LLM Desktop.rollback-composition-workspace-20261016`.
  Uninstall integration and existing model directories were preserved.
- Newly packaged Windows startup/recovery, light/dark/system appearance,
  nine locale options, preference persistence, page routes at 200% zoom, and
  access-panel checks at 100%/200% passed with isolated profiles. No consent
  was granted and no native input task ran. The replaced installation then
  passed a read-only connection check against the deployed service, including
  matching UI identity, external-service ownership and the 248px shared sidebar.
- Verified unchanged workspace identity, saved job IDs/statuses, conversation
  names, model inventories and speech selections. Exactly one service container
  is running: `93828310f1df`, on `127.0.0.1:11611`. Closing the isolated desktop
  check left it running. A bounded installed-Qwen completion returned `ready`;
  the RTX 5060 reported 4666 MiB in use. CPU worker version check passed; this
  does not constitute a fresh CPU inference benchmark.
- Build/replacement/test logs are in `build/composition-deploy-20261016/`.
  The initial Docker base-ID syntax and Electron output-argument errors occurred
  before activation and were corrected. No live data was affected by them.
  The updated desktop was reopened. This local Windows testing deployment does
  not close manual usability, screen-reader or macOS/Linux qualification gates.
  No commit, push, tag or release publication was performed.

### Full-flow repair replacement — 2026-10-17

- Rebuilt the service and current web bundle as
  `offgrid-llm:full-access-repair-20261017-r2` from the verified source. The
  replacement reports version `0.4.18`, revision
  `full-access-repair-20261017-r2`, and UI build
  `6cfa9d3f39c3cac0c807b1791b1d38dea0bd88bd59163e6e7747e13cf089c7f7`.
- Confirmed no active inference, task, model operation, chat turn or computer
  session before replacement. The workspace archive and verification completed
  at `/home/phil/offgrid-full-access-backup-SGn1Xu`; the predecessor remains
  retained as `offgrid-before-full-access-1792231835398`. Workspace identity and
  saved inventory were unchanged, with exactly one `offgrid` container on
  `127.0.0.1:11611`.
- Replaced the installed Windows desktop from the matching unpacked build and
  verified 606 staged files. The installed connection check passed for the
  external-service ownership contract, matching service/UI version and bridge
  protocol. The install ACL was normalized to remove the sandbox-runner-only
  inherited ACE while retaining SYSTEM, Administrators, the local user and
  `ALL APPLICATION PACKAGES` read/execute access.
- Post-replacement health is `healthy`; CPU and CUDA capability are available,
  including the RTX 5060. Bumblebee remains at profile revision 6 with a
  30-decision budget for new tasks. The desktop was left closed after the
  read-only check.
- This is a local replacement, not a release publication. Real native Notepad
  and Edge runs, other-platform qualification, and the installed-model
  collaboration quality gate remain unverified/failed as recorded above. No
  Git commit, push, tag or release was performed.

### Multi-engine inference development evidence — 2026-10-18

- **Implemented in source:** provider identity/event interfaces, bounded
  supervised-process protocol checks, a standalone admission scheduler, offline
  text-package manifest validation, additive API metadata, and a Models-page
  projection of imported language packages as unqualified. Existing GGUF Chat
  remains on the llama.cpp service path; MLX/Transformers cannot be selected.
- **Automatically tested here:** `go test ./internal/... ./cmd/... ./pkg/...`
  passed, as did 71 desktop tests, the web TypeScript/build and API-contract
  checks, and strict OpenSpec validation. A targeted Models-page Playwright
  assertion initially passed while its managed dev-server runner hung on exit;
  rerunning against an explicitly managed local Vite server produced a clean
  exit with 10 Models/profile/Chat fixture tests passed. This is not the full
  end-to-end suite. `go test ./...` could not traverse
  an unrelated access-denied backup under `build/`; the source package trees
  were tested directly. Fake workers establish protocol behavior only; they do
  not establish real generation or model compatibility.
- The Python SDK suite subsequently passed 37 tests in WSL, including
  authenticated provider-metadata projection. The Go CLI inventory test passed
  using an authenticated local HTTP fixture. The Python worker compiled for
  syntax, but neither test exercised MLX/PyTorch generation.
- The follow-up source Go suite (`./internal/... ./cmd/... ./pkg/...`) passed.
  Provider-scheduler fixtures now cover serialized loads across different
  models, cancellation before worker startup, and a live waiter retrying after
  its load leader is cancelled; the new concurrency fixtures passed five
  consecutive runs. The combined Python SDK/worker protocol suite passed
  43 tests in WSL. Request admission now rejects invalid/bounded tool menus
  and context requests beyond a loaded runner's declared limit. These are
  source and fake-worker checks, not live provider qualification.
- A service inventory regression test imports a synthetic, integrity-checked
  MLX text package into isolated test storage and confirms `/api/v2/models`
  remains available without starting the unrelated speech runtime. The package
  remains explicitly not runtime-ready or qualified. This catches a real
  category-routing error but does not execute the model.
- API schema drift check, web TypeScript check/build, and 71 desktop unit tests
  passed. Targeted `go vet` initially found an unreachable return in the
  GGUF context reader; removing that dead statement made `go vet` pass for
  inference, models, server, and CLI, and the models suite passed again.
- A further source pass added a trusted-adapter and freshly verified package
  lease before native text workers may start, with rejection tests for unknown
  adapters, altered artifacts, and unqualified modalities. A persisted legacy
  profile/task plus GGUF/API compatibility fixture passed after restart.
  On this checkout `go test ./...` passed, as did Linux/WSL
  `go test -race ./internal/inference ./internal/models -count=1`, six Python
  worker protocol fixtures, the web API/TypeScript/build checks, and all 71
  desktop unit tests. Windows Go race testing was unavailable because the
  Windows Go environment has CGO disabled and no C compiler; the WSL race
  result does not qualify Windows-specific code. The live service still uses
  its established llama.cpp lifecycle; the standalone provider scheduler and
  native worker entry point are not integrated into user Chat or agent runs.
- The next source pass connected llama-backed Chat, agent and model-probe
  acquisition to the provider scheduler's shared generation admission while
  preserving the existing model-cache and HTTP transport. A fake-native
  contention fixture proved a queued native request cannot load past a held
  legacy generation; cancellation leaves no ghost lease. The native worker
  pipe-write path now terminates only its owned process on a bounded Stop
  timeout, covered by a worker that deliberately stops reading. `go test ./...`
  and WSL `go test -race ./internal/inference ./internal/server -count=1`
  passed after this change. This does **not** yet route actual MLX/Transformers
  requests from Chat, provide unified memory accounting, or prove native-model
  qualification; the corresponding OpenSpec tasks remain unchecked.
- Provider descriptors now require separate, complete projector and
  preprocessing digests before advertising vision, and a preprocessing digest
  before advertising audio. Capability-evidence fingerprints include the
  preprocessing digest, so changing it invalidates an earlier check. The
  existing llama.cpp vision path and native text-only worker gate are unchanged;
  this metadata validation is not a VLM/audio qualification.
- Native package routes now bind a freshly verified package lease to a
  SHA-256-pinned interpreter and worker script, and recheck both worker identity
  and qualification before opening or reusing a resident process. The new
  evidence policy requires an exact package revision, adapter/template,
  runtime build, backend, measured hardware and frozen suite result; missing or
  stale evidence fails closed. A strict text adapter rejects unsupported Chat
  controls, tool/multimodal messages and incomplete streams instead of
  interpreting provider-specific prose as actions. This initial pass was a
  library boundary; the later gated Chat route below still does not establish
  real native-provider qualification.
  Isolated synthetic tests, `go test ./...`, WSL race tests for inference,
  models and server, and strict OpenSpec validation passed on this pass.
- The Python worker now hashes its loaded tokenizer chat template and requires
  it to match the reviewed adapter identity before its ready handshake. Seven
  isolated worker-protocol tests pass; this does not constitute a real MLX or
  PyTorch model load.
- A dead resident provider is evicted and replaced once at a subsequent
  pre-dispatch health check. The scheduler does not replay a failed generation,
  and it will not close a runtime while another admitted generation is active.
  Fake-runtime recovery and active-use tests pass; process crash recovery with
  real MLX/PyTorch workers remains unverified.
- Provider workers now return an observed backend/device and an explicit
  measured-versus-unknown memory report in startup and health responses. The
  scheduler checks it against the selected descriptor and carries it with
  generation timing; this is not yet a complete memory-admission policy. A
  service-owned qualification selector gates native text requests in
  non-streaming Chat, durable session streaming and OpenAI-compatible SSE.
  Isolated fake-provider tests cover denial before generation, normal replies,
  and truncated streams without a completion marker. No trusted native runtime
  is installed or automatically enabled; agent tool dispatch remains on the
  established llama path.
- The service now retains the provider registry and exposes a startup-only
  installation seam for reviewed adapters, SHA-256-pinned worker files and a
  complete hardware/evaluation target. It seals that seam when startup begins;
  duplicate or late registration cannot silently change a running service.
  No installed runtime currently invokes the seam. After these edits,
  `go test ./...`, WSL race tests for inference/models/server, 45 Python
  client/worker tests, web API/TypeScript/build checks, 71 desktop tests,
  documentation lint and strict OpenSpec validation passed. These are
  compatibility and isolated-provider checks, not native hardware evidence.
- OpenSpec task 3.4 is checked for the implemented modality gate and distinct
  projector/preprocessing identities. MLX/Transformers text qualification,
  provider-wide memory admission, agent tool routing and real platform testing
  remain unchecked.
- The native text worker now tokenizes the fully formatted prompt and rejects
  requests whose prompt plus requested response exceed the qualified context,
  before starting generation. Resource-report validation also rejects measured
  memory use above a reported device limit. These fail-closed checks passed
  isolated fixtures; they are not real MLX or PyTorch model evidence. The
  affected llama.cpp compatibility and source regression gate (task 6.2)
  passed: `go test ./...`, WSL `go test -race ./internal/inference
  ./internal/models ./internal/server -count=1`, `go vet` for those packages
  and the CLI, web API/TypeScript/build checks, 71 desktop tests, 47 Python
  client/worker tests, and strict OpenSpec validation. No installed GGUF or
  native model was loaded by those fixture checks.
- The provider scheduler now keeps only one resident provider on the
  unqualified path: it unloads idle workers before another model opens, and
  waits for a safe boundary while the prior worker is actively generating.
  Cancellation during that wait does not open a replacement. Focused
  scheduler tests and WSL inference race tests passed. This reduces unmeasured
  multi-model memory pressure but is not a complete device-memory admission
  policy or a real provider-switch qualification.
- Text workers now include an observed resource report in their terminal
  event; the process boundary validates it and updates the scheduler's timing
  record after generation. Malformed completion reports are rejected in a
  subprocess fixture. CPU memory and platform-specific device limits remain
  unknown where the worker cannot measure them, and no real-model resource
  claim follows from these fixtures.
- Scheduler instrumentation now attributes waiting behind an active runner or
  serialized load to queue time, closing the previous idle runner to unload
  time, and opening the next runner to load time. Five repeated scheduler
  fixture runs passed; cancellation and real-device timings remain unqualified.
- With multiple inference slots, a generation now reserves its resident runtime
  before leaving the load/selection boundary. A concurrent model switch must
  wait even while that generation is obtaining its final pre-dispatch resource
  report; it cannot retire the worker in that gap. A deterministic two-model
  regression passed three repeated inference-suite runs. Real concurrent-device
  qualification remains outstanding.
- OpenSpec task 4.2 is checked for the scheduler's resident reuse, pre-dispatch
  bounded replacement, safe switching, independent cancellation and no
  automatic replay after generation failure. This is source/fixture completion
  only; model-family and installed-hardware qualification are separate gates.
- The supervised process boundary now treats intentional worker retirement as
  successful cleanup, so an otherwise safe provider switch is not failed by
  the expected killed-process exit status. A complete typed worker error keeps
  the resident process for a later explicit request, whereas malformed or
  truncated output still invalidates it. Cancelled terminal events propagate
  cancellation, and startup/health replies reject mismatched request identity
  or sequence. The Transformers worker also disables duplicate special-token
  insertion during generation so its tokenization matches context admission,
  following the upstream chat-template guidance. Subprocess and Python
  regressions, `go test ./...`, WSL race tests for inference/models/server, and
  48 Python tests passed. These are fixture and source checks, not native-model
  qualification.
- The scheduler now tracks a request before queueing or loading, so a
  request-scoped Stop cancels those waits without opening a worker; active Stop
  also forwards a bounded provider cancellation and records cancellation
  latency. The process provider's stdout turn-taking is cancellation-aware.
  Native text workers generate from exactly the token IDs admitted after the
  pinned chat template: MLX receives those IDs directly, and Transformers no
  longer performs a second tokenizer pass. They emit prompt/completion usage,
  and the service rejects inconsistent or over-context usage before accepting
  completion. Device allocator memory reports now use the respective MLX,
  CUDA, or MPS APIs where available; these are not proof of free device memory
  or complete scheduler memory admission. Focused cancellation, usage, and
  tokenizer fixtures passed, as did `go test ./...`, WSL `go test -race
  ./internal/inference ./internal/models ./internal/server -count=1`, 51 Python
  tests, `go vet` for inference/models/server, documentation lint, and strict
  OpenSpec validation. The native
  hardware qualification and frozen workflow suite remain unrun.
- Native package activation now requires the imported tokenizer-template
  digest to match the trusted pinned adapter before a worker can launch;
  metadata-only import without that digest is still allowed. Missing,
  mismatched, and matching-template fixtures passed; `go test ./...` and WSL
  `go test -race ./internal/inference ./internal/models -count=1` also passed
  after this change. This closes a late startup rejection, not the missing
  real-model or process-isolation qualification.
- Native generation now has an exclusive unqualified admission lane relative
  to llama inference, even when an installation configures multiple llama
  slots. Configured llama requests may still run with one another. Native and
  llama waiters hold no numeric slot while waiting for the other family; Stop
  cancels a waiting native request and reports its queue time. Five repeated
  scheduler fixture runs, including native-waiter fairness, passed. A later
  full-suite run failed in inference without a retained failure line; ten
  repeated inference runs, a clean `go test ./... -count=1`, a clean two-run
  `go test ./... -count=2`, and WSL inference/server race tests then passed.
  The earlier isolated failure is not attributed to a cause on this evidence.
  This does not establish concurrent native inference, device-memory
  qualification, or the still-unfinished unified process/memory ownership task.
- The shared scheduler now retires an idle MLX/Transformers resident before
  admitting the existing llama loader. It keeps compatible llama residents and
  latches a failed native unload as unresolved; later legacy admissions and
  provider switches cannot assume memory was released. Cancellation during
  retirement releases admission without starting a legacy load. Repeated
  transition, cancellation, failure, fairness and unload-timing fixtures passed, as did
  `go test ./... -count=1`, WSL `go test -race ./internal/inference
  ./internal/server -count=1`, `go vet` for those packages, documentation lint,
  and strict OpenSpec validation. The legacy binary manager still owns
  its processes, so this is not completion of unified provider ownership or
  a hardware memory qualification.
- The reverse boundary is now connected to the service's existing llama cache:
  native admission retires loaded llama processes, disconnects the engine,
  invalidates its lifecycle identity, and refreshes registry load state before
  opening a native worker. A failed retirement blocks subsequent admission.
  Isolated scheduler ordering/failure checks and a service-wiring fixture pass;
  no actual native model was loaded. WSL inference/server race tests and
  `go test -p 1 ./... -count=1 -timeout=180s` and a separate
  `go test -p 4 ./... -count=1 -timeout=180s` passed. An overlapping default-
  parallel `go test ./... -count=1` run timed out after 11 minutes in cmd,
  agents, inference, and server; the affected packages and the complete suite
  passed afterward with bounded package parallelism, but the failure has no confirmed
  cause and remains a reliability finding. The cache remains the llama process
  owner, so task 4.1 and device-memory qualification remain open.
- The existing llama lifecycle lease and model/context load now enter through
  one provider-scheduler admission call. This preserves the model cache and
  HTTP engine while attributing queue versus load time for ordinary Chat.
  Isolated tests cover reuse, dead-process invalidation, load failure cleanup,
  and cancellation while a native family lease is held. Full ownership of
  llama processes, embeddings and memory accounting remains unfinished; tasks
  2.1 and 4.1 stay open.
- Native admission's wait for a concurrent service model operation is now
  cancellation-aware. A blocked model mutex can no longer prevent Stop from
  abandoning admission before llama retirement; a repeated isolated regression
  passed. A pre-effect cancellation does not latch the scheduler's uncertain-
  retirement state, so a later *new* request can be admitted; a partial or
  failed unload still latches and blocks reuse. Both paths have scheduler
  fixtures. This does not cancel an already dispatched model-cache unload.
- The service-owned scheduler now applies the same family and llama-retirement
  gate to direct runtime preloading, not only generation. A regression holds a
  llama lease, confirms that native preload times out without opening a
  worker, then releases the lease and confirms ordered retirement/opening.
  Standalone fake schedulers retain their existing direct runtime-probe
  semantics; no production caller uses that probe to bypass admission. The
  final `go test -p 4 ./... -count=1 -timeout=180s` rerun, WSL inference/server
  race tests, affected-package vet, documentation lint, and strict OpenSpec
  validation passed after this gate was added.
- The pinned Transformers worker now requires local safetensors weights and
  disables remote model code explicitly during load. If a Transformers
  producer survives its bounded join after a stream failure, the worker marks
  itself unhealthy before another request can begin; the supervisor's health
  check rejects that process for bounded replacement. Isolated Python worker
  tests passed **54/54** in WSL; Go process-protocol regression tests cover
  terminal error reuse versus an unhealthy worker. A supervised-process fixture
  also confirms that a poisoned worker is replaced on a distinct, explicit
  request and that the failed generation is not replayed. Task 3.2 records this
  source implementation, not a real PyTorch CPU/CUDA/MPS qualification.
- Native worker admission now rejects MLX requests without the Metal backend
  and Transformers requests outside CPU/CUDA/MPS before spawning a process.
  On the MLX path the Python worker verifies actual Metal availability before
  loading and at resource reporting; an unavailable device no longer produces
  a synthetic ready response. The WSL Python suite passed **55/55** and focused
  Go provider tests passed. The full bounded-parallel Go suite, inference race
  suite, affected-package vet, documentation lint, and strict OpenSpec
  validation also passed after this change, but
  no Apple Silicon model or real Metal measurements were available, so task 3.1
  remains unchecked.
- The native text completion boundary now keeps a token-limit terminal distinct
  from a normal stop and from cancellation across the Python worker, supervised
  protocol, Chat completion, session stream, and OpenAI SSE. An omitted output
  limit is selected only after the pinned tokenizer counts the formatted prompt;
  an explicit user limit remains strict. A terminal answer without measured
  usage is rejected rather than reported with fabricated zero counts. Isolated
  Go/Python tests, the bounded-parallel full Go suite, and WSL inference/server
  race tests passed. This is
  protocol correctness, not actual MLX/PyTorch model qualification.
- The llama provider adapter now preserves structured HTTP streaming finish
  reasons and measured token usage without changing the ordinary Chat engine
  path. It rejects missing terminal reasons, interrupted streams, ambiguous
  chunks, undeclared tool catalogs and tool deltas in text-only requests.
  Isolated raw-engine and HTTP SSE fixtures pass. The actual llama process
  cache remains outside provider ownership, so task 2.1 stays open.
- GGUF cache activation, engine port connection, and native-switch retirement
  now pass through an inference-owned lifecycle adapter with isolated order and
  partial-unload and post-load cancellation tests. A failed model connection
  clears stale fast-path state before another request. This does not yet put
  ordinary llama Chat generation
  or binary process ownership fully behind ProviderRuntime, so task 2.1 stays
  open. The bounded-parallel full Go suite, WSL inference/server race tests,
  affected-package vet, documentation lint, and strict OpenSpec validation
  passed after this adapter change. No live GGUF CPU/CUDA/projector run was
  performed in this pass.
- Embedding model loads and generation now use shared provider admission.
  A native switch retires the idle embedding process, and a subsequent
  Knowledge request reloads only against its previously verified runtime
  identity. HTTP hot-swap model loads are exclusively admitted and cancellable
  even when parallel generation slots are configured.
  Focused inference/server/RAG tests, the bounded-parallel full Go suite,
  WSL race tests for those three packages, affected-package vet, documentation
  lint and strict OpenSpec validation passed, including held-generation,
  native-switch and changed-identity fixtures. The llama and embedding process
  owners and real device-memory measurement are not yet unified, so task 4.1
  remains open.
- The native text worker now rejects empty message lists and unreviewed prompt
  fields, avoids starting MLX generation after an already-delivered Stop, and
  explicitly disables remote model code in both the MLX model loader and its
  tokenizer loader. WSL Python worker protocol fixtures passed 24/24 and the
  full Python SDK/worker fixture suite passed 61/61;
  documentation lint and strict OpenSpec validation passed. These are
  source/fixture checks, not an MLX hardware qualification or provider-specific
  tool-call qualification, so tasks 3.1 and 3.3 remain open.
- Trusted native model-family adapter records now declare the exact tokenization
  and text-streaming modes the worker implements and explicitly disallow a
  tool envelope. Unsupported or implicit modes fail registration before a
  package can become executable. The full bounded-parallel Go suite and WSL
  race runs for models, inference, and server passed after this change. This
  does not qualify a positive native tool-call envelope or real MLX/PyTorch
  model behavior, so task 3.3 remains open.
- The provider scheduler now holds its serialized model-load gate through the
  worker identity/health/resource handshake, residency publication, and any
  rejected-worker cleanup. Previously a second model could begin loading after
  the factory returned but before the first process was recorded or closed.
  Repeated concurrent-load, rejected-worker and close-during-health fixtures
  passed; the full bounded-parallel Go suite and WSL models/inference/server
  race run passed. This closes that load-overlap gap but does not yet unify
  llama process ownership or establish real-device memory admission; task 4.1
  remains open.
- The scheduler's load timing now includes the complete provider identity,
  health, and resource handshake instead of stopping at process creation; a
  delayed-handshake fixture verifies that accounting. The shared generation
  boundary now requires a matching terminal event and successful provider
  return before exposing a tool decision or completion. It validates request
  identity, sequence, the offered tool catalog and arguments, mixed output,
  resource reports, admitted context usage, and cancellation. A provider that
  ignores a rejected callback cannot erase the failure. Fake-provider
  regressions cover truncated text/tool streams, crash after a terminal event,
  forged or duplicate tool payloads, and unqualified native tool envelopes.
  The full Go suite, Go vet for models/inference/server, WSL race run for those
  packages, documentation lint, and strict OpenSpec validation passed on
  2026-10-19. One full inference test run encountered an intermittent Windows
  worker-retirement error
  (`TerminateProcess: Access is denied`); the affected isolated rerun and
  subsequent full Go suite passed. This is automated boundary evidence, not
  a claim that external tool effects are exactly once or that native provider
  tools are qualified.
- Generation events now reject fields belonging to another event type; for
  example, a text delta cannot smuggle a tool payload or a provider identity
  change. Focused process-provider and stream-gate tests, the full bounded-
  parallel Go suite, and WSL inference/server race tests passed after this
  envelope validation.
- The shared scheduler now validates the provider's resource identity again
  immediately before generation and after the terminal event but before any
  buffered tool decision is delivered. A changed backend or invalid memory
  report fails the operation without dispatch. Focused scheduler fixtures cover
  both boundaries. The full bounded-parallel Go suite, WSL inference/server
  race tests, and Go vet passed on 2026-10-19. This uses provider-reported
  evidence; it is not a measured host-wide RAM/VRAM admission claim, so
  scheduler task 4.1 remains open.
- Stop now detaches only the worker-cancel signal from a cancelled caller
  transport context, with its own one-second deadline. The generation context
  is still cancelled immediately, and a request-scoped signal reaches the
  provider even when the UI/HTTP Stop caller has already disconnected. A
  context-aware worker fixture verifies the signal and that the generation
  ends cancelled. The scheduler also reserves its cancellation identity while
  that signal is in flight, preventing a delayed Stop from targeting a new
  request that reuses the identity; the race fixture passed 20 repeated runs.
  These are local scheduler checks, not yet the complete parent/child/
  perception budget integration in task 4.3.
- Windows native-provider retirement now distinguishes an already-exited owned
  worker from an uninspectable or still-running one after `Kill` fails. A
  synchronize-only OS observation must confirm exit before `Close` treats the
  error as harmless; otherwise unresolved residency stays blocked. The
  exited-worker and poisoned-worker replacement fixtures passed 20 repeated
  Windows runs. This addresses the intermittent retirement diagnostic above
  without treating Access denied itself as proof that memory was released. The
  full bounded-parallel Go suite, WSL inference/server race tests, and Go vet
  passed after this change.
- Native provider requests and worker control messages now reject repeated
  JSON keys, malformed Unicode, non-finite Python control values and
  unreviewed operation fields before
  interpretation. The supervised Go process also checks handshake, generation
  and health event bytes before unmarshalling them, and retires a worker after
  an ambiguous health reply so that a later request cannot consume a stale
  event. Focused protocol fixtures, the full bounded-parallel Go suite,
  WSL inference/server race tests, the WSL Python worker suite (27/27), and
  the full WSL Python suite (64/64)
  passed on 2026-10-19. This verifies the local protocol boundary, not a
  real-model run or OS process sandbox.
- **Not tested or qualified:** an actual MLX model on Apple Silicon, a pinned
  Python/PyTorch worker on CPU/CUDA/MPS, process sandboxing, shared service
  memory/process ownership, tool decisions across engines, the frozen workflow suite, and
  platform-specific memory/performance claims. No Apple Silicon or suitable
  Python/PyTorch environment was available for these checks. Do not advertise
  MLX/Transformers as runnable or release-qualified on this evidence.
- No deployment, desktop replacement, model download, commit, push, tag, or
  release publication was performed for this provider work.
