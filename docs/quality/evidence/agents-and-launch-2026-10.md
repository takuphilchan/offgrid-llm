## Follow-up: conversational agents and profile persistence (2026-10-13)

Development evidence for `agent-conversation-workflows`, not deployment or a
claim that the new Chat/profile experience is complete:

- Ordinary conversation with available desktop authority can return text with
  zero tools, outcome checks or access requests. Existing declared outcomes and
  uncertain actions still block unverified completion. Versioned desktop,
  selected-window and managed-browser instructions allow answers/clarification
  instead of requiring an unrelated tool call.
- Trusted built-in argument/path rejection is classified before filesystem
  effects and gets three repair attempts. An independent counter survives
  unrelated reads and other repair-state resets. Injected directory creation
  and partial writes retain uncertainty; neither resume nor reconciliation
  replays the write.
- Manual reconciliation retains original call arguments and a timestamp with
  `human_reported` provenance. Runner and API fixtures verify that `done` does
  not verify an unrelated application outcome or complete the task.
- Owned immutable profile revisions support resource validation, optimistic
  conflicts, archive/history, installed models and Knowledge references.
  Tests cover cross-actor denial, upgrade/backup/restore, changed tool identities,
  pinned parent/child revisions, delegation cycles/depth/count and aggregate
  iteration reservations. Profile instructions cannot create authority.
- Held-job and session-link primitives reject execution/steering/access before
  linkage. Concurrent retries create one job; release itself does not execute.
  Restart preserves the explicit-resume boundary. Final exchange attachment is
  atomic/idempotent by turn/hash/job identity, including a disk-failure fixture.
  These primitives are now wired into profile-enabled session generation: a
  private admission outbox pins resolved inputs; the response returns one linked
  canonical job. Plain Chat keeps its existing route behavior.
- Added HTTP fault/restart checks at intent, turn, held-job, outbox binding,
  conversation link and release boundaries. Concurrent identical submissions
  return one job; concurrent conflicting payloads have one winner. Final-response
  failures attach saved output once without inference replay. Event reconnect is
  observation-only; anonymous/cross-owner read, event, recovery, cancel and delete
  requests are denied.
- Conversation deletion requires an explicit retain/stop choice for active
  work. Slow cancellation is not acknowledged as settled. Synthetic lost
  mutation acknowledgments retain uncertainty and canonical recovery after
  conversation deletion, with exactly one recorded synthetic effect. No native
  applications or personal documents are involved in these tests.
- Running-parent corrections are durably queued, applied at a safe boundary and
  distinguished from already dispatched actions. Tests cover inference,
  authorization, final-answer races, recorded/unknown effects and explicit
  restart continuation. The UI distinguishes queued from applied corrections;
  reload does not resubmit. The existing expiry browser fixture now freezes its
  fake clock during page load, removing dependence on loading within 1.5 seconds.
  A further regression check verifies that discarding a proposal for newly queued
  steering preserves its spent inference iteration, including the final allowed
  iteration; corrections cannot replenish the parent/child budget.
- Generated contracts and web/Python clients expose profiles and linked turns.
  New Python mutations do not automatically retry a lost acknowledgment and do
  not select/download a replacement model or use a client-local executor.
- Authorized discovery now ranks bounded schemas, retains native/MCP composition,
  reports oversized schemas, and rechecks actor/profile/child restrictions on
  every search. Regression tests cover 90-tool catalogs, pagination, revoked and
  changed identities, canceled discovery and zero execution during discovery.
  An 8192-token fixture preserves user constraints through a large multilingual
  MCP result, verifies provider/call/digest provenance, and reconstructs the exact
  saved bytes through history pages without a second provider call. Separate
  document fixtures reject service artifacts/files and remote exports as evidence
  for the requested unsaved Notepad text. A verified OBS check permits a normal
  final answer with no calculator or other unrelated dispatch. These deterministic
  cases do not prove an installed model chooses the correct action.
- Current React Agents now hosts the profile library/editor; Chat selects a
  revision and observes the same canonical job inline. Existing task links remain
  secondary inspectors. Tests cover CRUD, conflict preservation, removed models
  and changed tools, unsaved navigation, linked steering/reload, final-result
  attachment, approval/failure/uncertainty states and explicit old-service denial.
  All nine locales now have explicit profile/conversation strings. Browser checks
  exercise RTL, IME, keyboard focus and short-screen scrolling in both themes;
  native-speaker translation review and assistive-device qualification are unverified.
- Wiring Chat exposed the legacy admin-only job-route mismatch. Owned profile
  jobs now have Chat-authorized lifecycle/read access; legacy jobs remain admin
  only. Current actor checks reject non-admin service-filesystem/MCP/host tools
  and recheck disablement/demotion before dispatch, including approved calls.
  Pure calculator/time tools do not confer external resource access.

Executed checks in this working tree:

- Windows Go 1.26.6: `go test ./...` passed. Focused agent/server/session/
  inference/computer/storage/capability suites also passed.
- WSL Ubuntu Go 1.26.6: `go test -race ./internal/agents ./internal/sessions
  ./internal/server ./internal/capabilities` passed. This is selected race
  coverage, not the complete release matrix.
- `npm.cmd --prefix web/app run api:generate`, `api:check` and `check` passed.
- The isolated `task-access-intent.spec.ts` browser suite passed 12/12, including
  greeting without consent and deferred task access without replay on reload.
- The isolated `task-first.spec.ts` suite passed 23/23 after the clock-fixture fix,
  including the new pending-correction assertion. An initial invocation from the
  repository root missed the Playwright configuration; the recorded passing run
  used `web/app` and its disposable Vite fixture, not the installed service.
- New isolated `agent-profiles.spec.ts` + `agent-chat.spec.ts`: 19/19 passed.
  Existing Chat streaming, task-first, control consistency, voice playback and
  history suites: 63/63 passed. Light/dark and narrow profile screenshots were
  captured; primary buttons were corrected after visual inspection. This is not
  the complete UI acceptance matrix. Profile-job ownership/role-change Go tests
  passed; the original event fixture was corrected to use a paused saved job
  rather than subscribing indefinitely to never-started pending work.
- Chat/profile + task-access fixtures passed 35/35: remembered access is reused once,
  greeting/reload do not request control, and late consent after Stop cannot
  start input. A transient capability loss no longer switches the profile home
  back to task composition. Screenshots of inline attachments and short-screen
  Arabic/English light/dark Tools were inspected; partial verification is shown
  instead of an unqualified Completed label.
- A further parent-only speech fixture passed: five minutes of child/tool activity
  does not consume the final answer's TTS preparation timeout; no nested payload is
  spoken, the parent answer is synthesized once, and reload does not replay it.
- Built renderer + disposable service: the latest 351-test Playwright invocation
  finished with **341 passed, 10 skipped** in 3.1 minutes, no failures. Skipped
  scenarios require separately enabled model/restart fixtures, packaged-task
  fixtures or explicit installed-UI checks; they are not counted as passing.
  Evidence directory: `offgrid-web-qualification-CVCnpK` under the local temporary
  directory. No installed service or port 11611 was used. Narrow partial-result
  and short-screen Arabic/English Tools screenshots were inspected.
- The complete CI race package list passed in WSL on rerun, as did the companion
  command race suite. The first broad run reported a server-suite failure whose
  detail was lost in truncated output; a fresh server-only run and the subsequent
  full command passed. This is not a claim that the initial failure was diagnosed.
- Desktop contract tests passed 65/65; companion tests passed 73/73. These use
  isolated fixtures/controlled browser surfaces, not personal native applications.
- CLI/service-client suites passed with reusable-profile CRUD, explicit revision
  conflicts, pinned run submission, linked conversation send/inspection/recovery,
  interactive safe-boundary steering and Stop, pending-access/uncertainty blocking,
  explicit deletion choices, terminal-control sanitization and old-service
  rejection. No local executor, native launch or model download is used by these
  fixtures. Existing SSE rendering/cancellation tests run in the same suite.
- Python 3.14.4 / pytest 9.0.2 in WSL: `python3 -m pytest tests/test_client.py`
  from `python/` passed 36/36, including old-service guards before agent turn/job
  submission and no mutation retry after a lost reply. Native Windows `python` was unavailable; it is not
  counted as a Windows SDK qualification pass.
- `OFFGRID_CROSS_CLIENT_TESTS=1 go test ./internal/server -run
  TestAgentCrossClientIntegration -count=1 -v` passed in WSL with an actual
  newly built CLI, Python SDK and headless Chromium. Python creates one linked
  job, browser network requests inspect/steer it (same-ID correction retry is
  idempotent), CLI observes/cancels it, and Python observes the same cancellation.
  Separate browser identities verify cross-actor read/steer/cancel denial.
  There is one model-fixture invocation and one canonical job. All stores and
  credentials are disposable; UI rendering remains covered by the separate
  current-renderer suite. This is not real inference or native-platform evidence.
  An initial browser test used context-wide Alice headers while attempting Bob
  fetch headers; it was corrected to use a separate Bob browser context.
- OpenSpec strict validation, documentation-checker tests (6/6), maintained-doc
  link checks and `git diff --check` passed; line-ending warnings are not failures.

Application matching follow-up (2026-10-14): full catalog lookup now gates both
model launch choices and dispatch. Exact display names and registered aliases
outrank fragments without selecting the first duplicate name. Tests cover Settings
at entry 61 of 95, localized names, ambiguity, missing apps, stale/revoked sessions,
changed instructions, failed newer lookups and refusal to substitute Access.
The service recomputes the result from saved catalog/lookup arguments instead of
trusting an application ID in model output. Windows aliases come from additional
OS registrations of the same executable; paths are never used as aliases.

The first integrated fixture exposed an additional defect: the runner required
a selected-window observation before permitting application lookup. The read-only
lookup now runs before selection, and the durable MCP-plus-launch and complete
multilingual document fixtures pass through that order. `go test ./internal/server
./internal/computer ./internal/agents` passed after these fixes. These fixtures
dispatch only to controlled in-memory hosts. No personal application was launched.
An initial dispatch test changed a detached task snapshot rather than persisted
state; it was corrected to use the manager's durable step API.
New application outcome declarations are checked before persistence as well as
before launch. A mismatched declaration receives bounded correction rather than
freezing the wrong immutable outcome. `TestApplicationDeclarationRejectionDoesNotFreezeWrongOutcome`
covers the three-repair limit with no saved check or native dispatch. The combined
MCP fixture was updated to declare its research outcome before MCP approval and
its application outcome after lookup. Its initial unbounded pre-approval wait
was stopped after exposing that fixture-ordering error; startup now shares the
fixture's 30-second deadline. The corrected focused suite passed.

Frozen sanitized regression baseline for this change (2026-10-14):

| Reported regression | Executable fixture and current result |
| --- | --- |
| Hello with desktop authority invents work | `TestConversationWithDesktopAuthorityNeedsNoOutcomeDeclaration`: one model answer, zero executed tools, access requests, approvals or outcome checks |
| Verified OBS needs an unrelated calculator call to finish | `TestVerifiedOBSCanFinishWithoutUnrelatedTool`: a controlled trusted assessment of the OBS check permits one final answer and zero tool calls |
| Settings is missing from the first 40 applications; Access is chosen instead | `TestApplicationSearchFindsOmittedNamesAndInvalidatesOldCatalog` and `TestApplicationMismatchRejectedByDispatchBeforeHostContact`: entry 61 resolves; unrelated identity rejected before host contact |
| Unsaved multilingual Notepad is replaced by a service file/artifact | `TestServiceArtifactCannotFulfillUnsavedMultilingualNotepad`: artifact, built-in file and MCP export variants cannot fulfill native document checks |

The frozen focused command using these five test names passed with `-count=1 -v`
in the agents/server packages, as did the complete agents/server/computer suites.
These are retrospective sanitized regression representations, not a replay of the
user's personal applications or a claim that all five tests were run against an
untouched historical binary. The before/after runner lookup failure above was
observed directly during this implementation. The OBS assessment is synthetic;
real-model and real-application completion remain separate acceptance gates.
WSL race coverage for `internal/server`, `internal/agents` and `internal/computer`
also passed after the application-matching repair.
The subsequent complete `go test ./...` run passed. OpenSpec strict validation,
the six documentation-checker tests, 133-file link check and whitespace check
also passed. These checks do not replace the outstanding installed-model matrix.

### Authenticated action receipt recovery — 2026-10-14

The authenticated companion reply now reaches the existing canonical task store
before acknowledgment. The receipt is pinned to actor/task/session/call,
canonical arguments and the reviewed native capability revision. Only conclusive
typed native results qualify; failed/uncertain replies remain unresolved. Recovery
consumes the saved result once, records `companion_receipt` provenance and leaves
the task interrupted for explicit resume. It does not grant access, replay input,
mark all checks verified or infer exactly-once submission from a visible screen.
Service restart can recover the receipt while still expiring desktop consent.

Evidence: `TestComputerReceiptRecoversLostAcknowledgmentAfterRestart`,
`TestComputerReceiptRejectsMismatchesAndDoesNotInferSuccess`,
`TestComputerReceiptSurvivesWorkerSnapshotAndPause`,
`TestComputerReceiptRequiresConclusiveTypedHostResult`, and
`TestNativeReplyPersistsBeforeAcknowledgment`. The complete controlled-host
document fixture includes `lost-service-ack`: actual synthetic input and host
journal completion happen once, receipt persistence succeeds, then the transport
handoff fails. The runner recovers the exact result without another dispatch or
automatic continuation. The separate `lost-ack` and `partial-input` cases retain
uncertainty because no conclusive receipt reached the service. This is not a
claim that an offline/revoked host can be queried after consent ends.

`go test ./internal/server ./internal/inference ./internal/agents
./internal/computer -timeout=5m` passed after these changes; focused receipt,
document and transport regressions also passed. Desktop unit tests: 65/65;
companion unit tests: 73/73. Generated API drift, TypeScript and production UI
build passed (the existing large-JavaScript-bundle warning remains).

The first installed-model matrix exposed gaps hidden by deterministic callers:
a direct greeting fell into legacy native completion validation; completed name
lookup kept inviting lookup; verified app readiness prompted unnecessary tool
discovery or premature completion of a larger task. The fixes separate the actual
answer boundary from handoff validation, advance guidance from durable progress,
keep original intent beside stage guidance, and omit redundant tool discovery
only when all authorized optional schemas are already presented. No English app
name routing or model-specific execution bypass was added. A read-only token
count transport EOF retries once within the original deadline; generation and
mutations do not inherit this retry. Frozen real-model results follow separately.

Remaining: human locale/accessibility qualification, installed
cross-client package qualification, and integrated
model/desktop acceptance. The passing profile/Chat fixtures are not a completed
program or installed-package qualification.
No installed-model evaluation
or real Windows/macOS/Linux native-app workflow is qualified by these fixtures.
No container, desktop installation, commit or release was changed by this work.

## Follow-up: rejected-launch loops and staged model execution (2026-10-12)

The 2026-10-13 correlation repair fixes a service transport defect: complete,
validated single-step calls lacked IDs in both completion and SSE responses,
causing the native preflight to reject otherwise valid Qwen calls. Correlation
IDs are now created only after clean envelope validation and retained through
stream parsing. The installed Qwen2.5-7B-Instruct-Q4_K_M passed the synthetic
two-step native preflight three times; this did not dispatch desktop input.

The same installed model then completed three consecutive full runner/executor
workflows against an in-memory document provider: one creation, one exact
`OffGrid test — café 日本語 12345` insertion, independent read-back, unchanged
pre-existing document, and completed task. Runs used 18/16/16 model calls and
took approximately 64.0/52.8/50.4 seconds at the unchanged 8192-token allocation.
These are synthetic workflow measurements, not a native Notepad or speed claim.
Earlier unsuccessful repetitions exposed repeated baseline declarations and
post-verification input proposals; they remain failures, not discarded passes.
Stage schemas now reuse identical checks, prioritize fresh post-input reads,
and require saved plan advancement before proposing more input after a verified
document stage. Stage guidance follows the result it describes. MCP tools stay
composed; canonical evidence and dispatch guards are not relaxed.

Owned loopback probes returned responses with both the installed CPU-only
worker and CUDA worker (about 2952 ms and 184 ms respectively for a three-token
probe). CUDA device discovery reports RTX 5060. The captured runtime log did not
provide explicit layer-offload evidence; these probes do not qualify hardware
profiles or prove native application behavior. No personal desktop input,
permission grant, model download or retry of a user's saved task was performed.
Installed Windows Notepad, other application profiles, macOS and Linux native
acceptance remain unverified and their OpenSpec gates remain unchecked.

Authorized local testing deployment replaced the service with
`offgrid-llm:preflight-correlation-cpu-gpu-20261013` (container `2c3a658f5307`)
and installed the matching unsigned local 0.4.16 desktop package. The service
reports revision `cd87033-worktree-preflight-correlation`; the current UI build
and workspace identity are unchanged. Before replacement, an idle-state check
and verified workspace backup completed; the private backup is
`/home/phil/offgrid-agent-sequencing-backup-v9oEHqnt`. The stopped rollback
container is `offgrid-before-agent-sequencing-1791870482`. Desktop application
and state backup is `D:/offgrid-llm-backups/desktop-preflight-correlation-20261013`.
One OffGrid service is running; models, history and failed tasks were preserved.

The live service's authenticated API admission path returned
`computer_tool_check_passed` for the installed Qwen model and Windows driver at
8192 context tokens. Its worker is the installed CUDA binary. Windows Go tests,
WSL server/agent/inference race checks, computer/desktop unit suites, strict
OpenSpec validation, documentation and whitespace checks passed. Packaged
Windows startup/recovery and current-access presentation passed in isolated
profiles, including 2x zoom. Read-only installed-package verification matched
all nine deployment files and the native pack; it submitted no tasks or grants.
This is a local testing deployment, not a signed release or platform qualification.

The 2026-10-13 follow-up retains both CPU and GPU operation. The deployed
service was explicitly forced to CPU (`OFFGRID_ENABLE_GPU=false`, zero layers,
and an explicit CPU worker path). Merely detecting the RTX 5060 does not undo
that configuration. Runtime selection now accepts the effective configuration,
including file-loaded settings, and cannot prefer a cached CPU worker over a
validated installed GPU worker. Embedding workers retain independent placement.
The CUDA image also includes a separate CPU-only worker at the same pinned
llama.cpp revision. GPU compilation, device listing and successful accelerated
inference are separate checks; none qualifies native application workflows.

The later CPU synthetic model run reached launch verification and document
read-back, but repeatedly declared text checks on the pre-existing document
instead of creating the requested new one. Read schemas now expose only actual
inventory identities and returned continuation pages; duplicate declared checks
are removed from subsequent model schemas. The focused Windows inference,
agent and server tests pass after the configuration fix. A complete real-model
create/edit/read-back repetition and installed native-app acceptance are still
unverified; the failed model attempt is not recorded as a pass.

The later reported Notepad task recorded twenty steps without launching an
application or editing a document. Its catalog included Notepad; this was a
different regression from the earlier catalog omission. Rejected dependent tool
batches were followed by successful catalog reads that incorrectly cleared the
repair budget. Recovery also referred to a prior launch even though those launch
proposals had never reached dispatch. The saved failed task remains untouched.

- Dependent computer batches remain rejected before dispatch. Their first
  dependency now stays unresolved across unrelated reads and metadata, with the
  persisted three-repair ceiling. Fresh evidence, not a rejected proposal,
  determines whether an initial launch or post-launch observation is needed.
- Sequential local inference uses an owned, schema-constrained single-call
  envelope. The pinned runtime's native template parser did not reliably enforce
  `parallel_tool_calls=false`. Streaming must finish cleanly before any call is
  emitted. Token counting uses the same wire contract. Ordinary typed Chat and
  callers allowing parallel tools retain their existing transport.
- Model schemas follow recorded catalog, launch, target, inventory and read-back
  stages. Document mutations are offered only with a currently valid matching
  declared check. These projections do not widen grants, infer requested text,
  weaken host preconditions or replace canonical outcome evidence. MCP tools
  remain composed with computer capabilities.
- The follow-up model runs also exposed duplicate exact-history retrieval and
  confusion between whole-control verification and document verification. Model
  retrieval now lists omitted/archived evidence and unread page offsets only;
  full canonical history stays available through existing APIs. Document-aware
  instructions distinguish the two verification paths, and legacy verification
  is offered only for a declared, bound control check. Tool-local JSON-pointer
  references are independently rebased inside the private inference envelope,
  without changing the original MCP schema identities.
- Full Windows `go test ./...` passes after these changes. Scripted complete
  runner/host/journal fixtures pass, including partial input and lost
  acknowledgments. Installed-model repetitions are still being investigated:
  initial runs exposed repeated metadata/readiness choices and a missing
  observation operation in the synthetic host. Those failed runs are not
  qualification passes. CPU-only generation measured roughly 1–3 minutes per
  step; prompt counting took milliseconds. This is not an interactive performance
  claim.
- The synthetic model fixture cannot launch or edit real applications. It does
  not qualify installed Notepad, other Windows applications, macOS or real
  X11/Wayland. The native acceptance and hardware/pilot gates remain open.

## Packaged application discovery and sequential-tool repair (2026-10-12)

The reported Notepad attempt failed before application launch or document input.
Its catalog omitted Notepad, then a multi-tool model response was permanently
failed by the sequential-control guard. The measured prompt was 5,536 of 8,192
tokens; the recorded failure was not context overflow.

- Reproduced the catalog omission read-only against the current user's installed
  package. AppModel returned its path, but `filepath.EvalSymlinks` failed with
  access denied at the protected WindowsApps ancestor. The replacement validates
  the opened object's normalized final DOS path, without changing filesystem
  permissions. Package registration, revision, manifest and file-identity checks
  remain enforced. Remote/device paths, alternate streams, missing files and
  junction redirection are rejected. The same Notepad registration now appears
  in the full **95-entry** installed catalog (44 packaged entries). This did not
  launch, focus, select or edit an application.
- Computer multi-call responses are rejected in full before queueing or dispatch,
  with uniquely paired model feedback and the existing persisted three-repair
  budget. A corrected single call can continue; exhaustion interrupts the task.
  Earlier actions, pending read/expectation/access checks and uncertain-outcome
  boundaries are preserved. Non-computer batching remains compatible. Tests
  cover rejected-batch side effects, repeated proposals, false completion,
  restart state, storage failure and stronger outstanding repair guards.
- Windows `go test ./...`, the complete CI Linux race set plus host-command and
  native-tagged packages, companion **73/73**, desktop **65/65**, API drift,
  TypeScript and production UI build pass. The existing large-bundle warning
  remains. Windows junction rejection passed; symbolic-link fixtures were
  skipped because this account lacks symlink-creation privilege. No privilege
  elevation or directory ACL changes were made.
- This is not installed-app input qualification, model-accuracy evidence or a
  completion claim for the wider document program. The failed saved task is
  unchanged and was not rerun automatically.

### Matching local replacement receipt

- With explicit approval, replaced the existing service with
  `offgrid-llm:notepad-repair-20261012`, container `7d7cc5257c96`, image digest
  `sha256:7a519159c9904dd54872c635048c88d365da5f9932a5e81d170e703edaca521c`.
  Exactly one container service is running, `/health` is healthy and restart
  count is zero. The running binary matches the new staged build. The previous
  service is stopped with restart disabled as
  `offgrid-before-notepad-repair-1791800038`.
- Private backup `/home/phil/offgrid-notepad-repair-backup-Rr5eijBH` contains the
  stopped workspace archive, successful verification and SHA-256 receipt.
  Workspace identity, task IDs/statuses, conversation names and installed model
  IDs were compared before/after. Model volumes were retained, not copied over
  or repaired. The reported failed task retains its original two steps.
- Updated `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop` using the
  rebuilt installer. All nine selected installed resources, including service
  and native worker binaries, match the staged package; full computer-pack
  integrity verification passes. Protected application/state/profile backup:
  `D:\offgrid-llm-backups\desktop-notepad-repair-20261012`. Five desktop
  connection/journal files were hash-checked unchanged after replacement.
- Packaged startup/recovery passes (652/735 ms; `offgrid-desktop-startup-Olh8o8`),
  as do access/zoom checks (`offgrid-desktop-access-GPHymF`) and the owned packaged
  browser workflow (`offgrid-packaged-browser-umYMxN`). Installed read-only smoke
  evidence is `offgrid-installed-notepad-repair-powBSr`; all evidence directories
  are under the Windows temporary directory. The installed test submitted no
  task, granted no desktop authority, and confirmed the same external workspace.
- Service and desktop retain the current UI identity
  `d586cd9823a7b87537c2ebea1a5cd944bbe067f430b5f3fbe90fd00da33d8795`.
  This remains an unsigned local **0.4.16** testing build, not a new release.
  No commit, push or publication was performed. Full installed Notepad
  creation/input/read-back and other-platform qualification remain unverified.
