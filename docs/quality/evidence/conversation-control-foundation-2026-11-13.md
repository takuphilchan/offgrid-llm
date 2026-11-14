# Conversation control foundation — 2026-11-13

This is an implementation checkpoint for the approved conversation-first
architecture, not completion of OpenSpec sections 9–14 or a product rating.
The earlier [repair evidence](conversation-first-repairs-2026-11-12.md) remains
separate. Existing document/image work in the shared checkout is preserved.

## Implemented foundation

- The existing agent SQLite database now owns scoped project, conversation,
  and work records, command receipts, ordered events, accepted input, work links,
  and an outbox. A new work command commits its run in the same transaction.
  The existing runner/coordinator dispatches it after commit. Work state updates
  commit alongside the task checkpoint and receipts.
- Actor/workspace request identities deduplicate commands before mutable model
  readiness checks. Conflicting payload reuse and concurrent stale revisions
  fail. Read/reconnect requests never dispatch work. Bounded event pages return
  a snapshot and explicit gap when observation history expires; accepted input
  is retained separately.
- Additive v2 resources expose conversation commands/configuration/events,
  projects, and work inspection. Shared Go wire types live in `pkg/api` to avoid
  importing the execution runtime into its HTTP client. TypeScript is generated
  from OpenAPI; Go and Python transports preserve command identity and revision.
- Conversation model preferences take precedence over an optional pinned
  profile's model. Persistent restrictions survive omitted fields and model
  changes; effective execution intersects the profile ceiling. Profile-free
  capability discovery and dispatch still check current actor authority.
  Explicit delegation-off is enforced at discovery and invocation.
- Authorized historical context can be rebuilt while excluding revoked source
  results. Original user input and unrelated authorized exchanges remain usable.
  Selected Knowledge can use the existing admission service without a profile.
- A versioned importer, exposed through an explicit `import_conversation` command,
  retains the exact original session bytes and
  digest, stable old-name mapping, source references, and configuration evidence.
  It refuses active legacy work. The ownership marker commits transactionally;
  legacy mutation and execution paths reject canonical conversations. Legacy
  reads project current canonical runs without writing the original file.
- Schema 8 gates incompatible older database readers and retains verified
  migration backups. Historical runs retain their existing execution contracts.
  The version-6 control-record constraint is rebuilt transactionally while
  preserving its referenced graph, indexes and command receipts. Foreign keys
  are checked before commit and re-enabled on the dedicated connection.
  Version 8 adds durable inbox fences that older runners cannot enforce.
- Project commands retain immutable preference revisions and strict version-1
  TOML digests. Required unavailable resources block admission; optional exclusions
  have inspectable explanations. Scoped skill discovery loads metadata first,
  pins revisions and preserves profile ceilings. No project file can grant
  authority, enable a connection, install dependencies or read a service path.
- Work Stop, Pause, correction and continuation commands share the transactional
  receipt boundary. Graph fences commit before cancellation. Model cancellation
  remains visibly stopping/pausing until teardown; process stopping uses a
  service-owned operation. Continuation retains consumed allowance.
- Canonical child runs now have durable worker sessions. Worker messages retain
  accepted/delivered/applied states and checkpoint deduplication. Worker follow-up
  versions remain separate from work instruction versions. Continuation creates
  another run in the same session and retains context; user-stopped workers
  require explicit user continuation. Accepted messages that cannot be applied
  before Stop or assignment completion retain an explicit `abandoned` outcome,
  timestamp and reason. Pause retains pending delivery. Full worker acceptance
  still requires the remaining lifecycle contracts below.
- Actor/workspace user defaults persist model and delegation preferences through
  the shared transaction boundary. Effective configuration identifies their
  revision and provenance. Accepted runs retain the resolved configuration;
  a concurrent preference edit rejects admission prepared against stale defaults.
- Project selection, work linking and focused-work commands preserve run identity,
  budgets and conversation restrictions. Cross-conversation context uses a durable
  link and rechecks source access; caller-supplied history is insufficient.
- The internal conversation-inbox acceptance path commits the input, history,
  conversation revision, event, receipt and affected graph fences together.
  Acceptance requires no model availability. Held work yields at checkpoints;
  dispatched operations settle through their existing receipt path. New work
  intent does not hold the previously focused graph.
- Internal foreground delivery now pins one existing-runner execution per input,
  serializes delivery within a conversation, and returns the original run on a
  retry without resuming it. Foreground runs have their own allowance and no host
  grants; they cannot create workers or execute provider tools. Reply history,
  result references, run state and ordered events settle in the task transaction.
  A reply alone does not imply that a correction was incorporated.
- Explicit internal incorporation applies the accepted correction text once, or
  records an answered side question without changing work instructions/budget.
  It waits for dispatched operations to settle, respects committed input order,
  and releases only that input's graph hold. Stop, Pause, expired access and
  restart recovery remain effective. Corrected delegated graphs retain completed
  evidence, retire obsolete assignments and invalidate their integration status.
  Public submission and the client composer are not connected to this lifecycle
  yet; responsive chat remains incomplete.
- The existing service coordinator now prepares accepted foreground inputs in
  bounded pages, one head per conversation. Preparation snapshots the current
  actor role without retaining credentials, resolves the shared configuration,
  and pins the run without inference. An unavailable model/context retains the
  input with a durable limitation and retry time. Successful delivery clears it;
  interrupted delivered runs are never replaced or automatically resumed.
- Foreground context includes prior inbox exchanges and focused-work state.
  It excludes later queued messages, keeps user constraints when source access
  is revoked, and withholds affected derived replies. Long replies remain fully
  available to the runner's context accounting instead of triggering a fixed
  history cutoff. Read-only inbox endpoints and Go/TypeScript/Python transports
  expose accepted, delivered and applied state without causing execution.
  The owner can inspect and stop a profile-free foreground run through the
  existing job transport, validated against its durable inbox binding. Foreign
  actors are denied, and legacy deletion cannot orphan the saved input/reply.
- Foreground turns can now record an answer/correction decision through the
  existing runtime. Ordinary replies record an answer automatically in their
  completion transaction, with no classification call. Correction proposals
  and their tool receipts commit together; the service incorporates completed
  decisions at settled checkpoints without consuming inference capacity.
  Waiting/stale/revoked decisions remain inspectable. Interrupted or stopped
  foreground turns cannot apply their proposals. Later context distinguishes
  actual application receipts from earlier replies describing pending work.
  Existing native tool pins do not inherit qualification for the new tool.
  Pending question answers and automatic model-driven revalidation remain
  unfinished; explicit user revalidation is available through input controls.
- A completed foreground `new_work` decision now uses the same transactional
  work reducer as explicit commands. It commits work/run identity, conversation
  membership/focus, admission events and the input's application receipt together,
  releasing only that input's previous work hold. The original user message is
  not inserted again. The service resolves current configuration and authority;
  it never transfers a foreground/focused-work host binding. Required host access
  remains a pending input with no scheduler admission. The new work's objective
  is the exact accepted text and its proposed expected result is retained as an
  acceptance condition. Retries return `created_work_id`, including after restart
  or model-catalog loss, without creating another run. Canonical work rechecks
  actor chat permission before inference as well as during preparation.
- Preview public text submission now persists input without model readiness;
  the service owns delivery. Go/TypeScript/Python transports expose submission,
  Stop, interrupted-attempt continuation and exact-correction revalidation.
  Stop fences decisions before cancelling generation and settles only its own
  work hold. Withdrawing a later input need not wait for earlier input to apply.
  Restart bookkeeping cannot resume work. Stopped input remains historical
  context rather than an active instruction. Controls retain command receipts
  and event cursors; failed receipt writes publish no partial cancellation.
  Explicit pending-question answers are rejected rather than treated as approval.
  Revoked derived decision text is redacted on reads, events and control receipts;
  admitted work retains dependencies from its model-authored acceptance condition.

New conversation/work admission is disabled by default behind
`enable_conversation_preview` / `OFFGRID_ENABLE_CONVERSATION_PREVIEW`.
Disabling admission does not hide accepted records, invalidate identical
request retries, or prevent the owner from stopping accepted work. Legacy run
deletion cannot silently orphan canonical work/history; it returns a specific
compatibility error until canonical deletion is implemented.

## Recorded checks

- Full `go test` suites for `internal/agents`, `internal/server`,
  `internal/sessions`, `internal/serviceclient`, and `internal/execution` passed
  for the initial control-store integration. The first invocation found the
  route-manifest count needed the six intentionally added routes; the corrected
  complete invocation passed. Subsequent compatibility/authority changes have
  passing targeted tests; the final whole-repository gate remains outstanding.
- Isolated tests cover concurrent identical commands, concurrent revision
  conflicts, actor/workspace isolation, transaction rollback after a forced
  receipt failure, restart without implicit dispatch, expiry of event cursors,
  retained input, and atomic work/run outcomes.
- Service tests cover profile-free admission, retained context, no extra model
  classification call, model/restriction persistence, non-admin Stop and
  inspection, foreign-actor denial, current actor revocation, and disabled
  admission without stranding accepted work.
- Import tests cover concurrent retries, exact retained source bytes, unknown
  browser configuration, denied legacy writes/attachment changes, restart,
  untouched active legacy work, and projection failure followed by an identical
  command retry without another model invocation. A legacy non-streaming request
  after migration is rejected before invoking its model.
- `api:generate`, `api:check`, TypeScript checking, source-size checking, and
  application-layout checking passed at the control API checkpoint.
- WSL Ubuntu's installed Python ran `sdk/python/tests/test_client.py` and
  `sdk/python/tests/test_conversations.py`: **39 passed**. No Python installation
  was required. SDK command requests disable automatic transport retries.
- OpenSpec structural validation passed. This does not establish runtime or
  hardware correctness.
- A later complete run of the five affected Go packages passed: agents, server,
  serviceclient, sessions and execution. Subsequent mailbox tests passed for
  delivery/application deduplication, restart and worker-local Stop. Worker
  continuation tests subsequently passed for retained identity/context, explicit
  stopped-worker continuation, model inspect/continue/wait/synthesis, stale
  dependency invalidation, exact result binding and transaction-failure rollback.
  HTTP tests passed for actor isolation and Stop without model availability.
  Concurrent document-parser fixture edits temporarily prevented one invocation
  from compiling; the subsequent selected server checks passed.
- Work-control tests pass for concurrent correction retries, rollback before
  cancellation, Stop acceptance while inference is blocked, late-result rejection,
  completed-outcome preservation, and continuation without resetting allowance.
- Python client/conversation tests now pass **40 tests** after adding project and
  work controls. Generated API checks and TypeScript checking passed at that
  checkpoint. The latest Python client/conversation run passes **41 tests**,
  including user-default command identity and disabled transport retries.
- User-default tests pass for concurrent retries, conflicting request reuse,
  stale revisions, independent actor/workspace state, restart, receipt-failure
  rollback and stale admission. Service/Go-client tests pass for authenticated
  access, strict rejection of authority fields, setting provenance and pinned
  accepted runs. Populated version-6 migration and injected rollback tests pass.
- Mailbox settlement tests pass for both accepted and delivered input under
  worker Stop, root Stop and assignment failure, including restart, closure,
  retained contents and Pause preserving pending input. Public import tests
  pass for receipt/authority rollback, retained original bytes, identical retries
  without the legacy reader, and missing/busy source responses.
- The complete affected five-package Go run after user defaults and mailbox
  settlement passed: agents (39.312s), server (66.516s), sessions (6.819s),
  serviceclient (0.585s), execution (4.431s). API generation/check and TypeScript
  checking passed at that checkpoint. The targeted WSL Linux race run for user
  preferences, worker lifecycle, work controls and import passed (6.088s), after
  a dependency-download TLS timeout was resolved using the existing local Go
  module cache. It is a race check, not native backend qualification.
- Subsequent selection/link tests pass for restart, identity/budget/restriction
  retention, actor/workspace isolation, receipt-failure rollback, rejection of
  forged history, retrieval through a stored link and denied revoked-source links.
  The expanded Linux race run then passed for agents (7.432s) and server (5.855s),
  covering user defaults, work selection/linking, worker lifecycle, controls and
  public imports. API generation/check, TypeScript, source-size, application-layout,
  maintained-document checks and OpenSpec validation passed after these additions.
- Inbox tests pass for concurrent accepted retries, receipt-failure rollback,
  focus binding, paging and actor isolation, late model-result rejection,
  measured generation settlement, runner-slot release, worker-graph fencing,
  Stop remaining available and restart requiring recovery. Tests on both sides
  of the effect boundary confirm a proposed operation is not dispatched after
  acceptance and an already-dispatched operation retains its actual receipt.
  A version-7 migration fixture preserves existing work and creates an empty
  inbox under schema 8 with a retained backup manifest.
- After the inbox foundation, complete affected package suites passed again:
  agents (39.649s), server (63.608s), sessions (5.456s), serviceclient (0.580s),
  execution (4.044s). The Linux race subset covering inbox, preferences, worker
  lifecycle and work controls passed (7.465s). No native host effects were run.
- Foreground delivery/incorporation tests pass for FIFO ordering, concurrent
  retries, actor isolation, admission/reply/incorporation rollback, interrupted
  delivery without redispatch, preservation of the work allowance, control and
  recovery precedence, exact-once corrections and retained completed-worker
  evidence. Full agents (39.435s), sessions (4.352s), serviceclient (0.349s) and
  execution (3.719s) passed at this checkpoint. The expanded agents race subset
  passed (9.574s).
- The accompanying full server run exposed a document-context fallback regression:
  selected data was bounded, but the final citation/evidence instructions were
  not fully accounted. The fallback now checks the complete assembled messages
  and reselects within the limit. The document wrapper was shortened while
  retaining uncertainty, missing-context and citation rules. Targeted document,
  spreadsheet-header/native-value and slide-note tests pass without relaxing
  their required-evidence assertions. The final complete server suite passed
  (58.334s), the complete session suite passed (6.967s), and the targeted Linux
  server race run passed (2.182s). An earlier Linux compile attempt encountered
  a document-test helper being changed concurrently; the final rerun compiled
  and passed. API generation/check, TypeScript, source-size, maintained-document
  checks and strict OpenSpec validation also passed. Full browser, desktop and
  backend/model qualification have not been rerun for this internal increment.
- The service-delivery increment passes tests for bounded queue-head paging,
  deferred-preparation rollback, model readiness/backoff, per-conversation
  serialization, execution under the service runner lifetime, preserved run
  identity after restart, and inspection without dispatch. Context tests retain
  a long reply and its preceding constraint, exclude later queued input and
  withhold revoked-source replies while retaining original user input.
  Current-role snapshots carry no credentials; tests deny disabled actors both
  before preparation and after admission but before inference.
- Full agents (40.149s), users (2.763s), sessions (5.878s) and serviceclient
  (0.473s) passed for service delivery. The first full server invocation found
  route-fixture drift while concurrent document-collection routes were changing;
  the updated-tree rerun passed (60.375s). Targeted Linux race tests passed for
  agents (4.246s), users (1.618s) and server (4.353s), after a concurrent document
  test-helper compile failure was resolved in that work. Python client and
  conversation tests passed 41 tests, including escaped inbox paths and GET-only
  observation. Generated API, TypeScript, source-size, maintained-document and
  strict OpenSpec checks passed. Final foreground owner-access tests additionally
  exercise authenticated inspection, Stop, foreign-user denial and deletion
  protection. The full-server follow-up passed (59.588s), and expanded server
  race checks passed (7.053s). A subsequent profile test and targeted preparation,
  delivery and owner-access run passed (1.570s): a computer-enabled optional
  profile retains its revision/instructions without requesting desktop access
  for the foreground turn. No browser/native/model qualification is implied.

- Foreground decision tests verify one-call ordinary answers, exact accepted
  correction text, committed proposals before application, input ordering,
  stale instruction rejection, duplicate application, actor/source checks,
  bounded queue paging, and preservation of the root work allowance. Injected
  proposal and application-receipt failures leave no partially applied control
  state. Interrupted/stopped foreground turns cannot apply saved proposals;
  a late model reply cannot replace the stopped outcome. Restart can finish
  control bookkeeping without resuming interrupted work. Service tests cover
  retry backoff, Pause/Stop precedence, stale/finished-work explanations and
  correction receipts overriding pending narration in later context.
- Final affected suites passed: agents (43.132s), server (68.887s). The expanded
  Linux race subset passed for agents (4.193s) and server (5.337s). Serviceclient
  (0.603s) and API tests passed earlier in this increment. API generation/check,
  TypeScript, source-size, maintained-document checks (366 files, zero problems),
  strict OpenSpec validation and targeted whitespace checks passed. These are
  deterministic fixtures; browser/desktop, real-model tool selection and native
  qualification were not run for this increment. No additional architecture
  task is marked complete on the basis of this partial foreground path.

- Foreground work admission passes concurrent identical retry, complete receipt
  rollback, previous-work hold/budget preservation, exact objective/acceptance,
  nonduplicated user-history, owner/source checks, stale preparation and host-grant
  transfer rejection. Restart returns the original interrupted run; its stale
  scheduler-ready flag is now explicitly cleared during recovery. Service tests
  verify current configuration, admission without inference, unavailable-model
  backoff/recovery, and actor revocation before work inference. A final host-input
  case verifies that required desktop access stays pending and cannot dispatch.
- Full agents (43.125s), server (68.561s), serviceclient (0.491s), and API (0.920s)
  suites passed for foreground work admission. Expanded Linux race checks passed
  for agents (17.289s) and server (12.937s). After adding the shared reducer's
  pending-host-input handling, the targeted work/canonical-conversation suites
  passed (agents 0.939s, server 1.684s), with corresponding race checks passing
  (2.355s and 4.459s). The stricter disposition-conflict assertion passed in a
  final focused run (0.618s). API generation/check, TypeScript, source-size,
  maintained-document and strict OpenSpec checks passed. No browser, native-host,
  real-model or hardware qualification was performed for this increment.

## Latest input and compatibility checks

- Public input acceptance, withdrawal, interrupted-attempt continuation and
  explicit correction revalidation passed the full agents (45.434s), server
  (75.921s), serviceclient (0.515s) and API (0.984s) suites. Expanded Linux race
  checks passed for agents (6.891s) and server (13.165s). Python client and
  conversation coverage passed 42 tests. API generation/check, TypeScript,
  source-size, maintained documents (367 files, zero problems), strict OpenSpec
  and targeted whitespace checks passed. The subsequent real event-cursor
  assertion passed targeted agents tests (1.300s); its earlier race process
  output was unavailable after continuation, so it is not claimed as evidence.
- The follow-up compatibility projection includes inbox exchanges, deduplicates
  input-created work and applied corrections by identity, preserves timestamp
  ordering, withholds revoked replies and retains withdrawal receipts. Reads do
  not alter import bytes or dispatch execution. Legacy turn mutations reject
  canonical conversations with a compatibility explanation. Initial focused
  tests passed (1.095s). The combined input/projection Linux race run passed
  agents (2.916s) and server (6.730s), including the event-cursor change; this
  preceded the final legacy turn-route guard and its dedicated regression.
- Final affected Go suites passed: agents (43.694s), server (71.743s), sessions
  (4.868s), serviceclient and API (cached). Final Linux server race coverage for
  `Test(Canonical|ConversationInput)` passed (12.636s), including the legacy
  mutation guard. Source-size and strict OpenSpec checks passed; the final
  maintained-document check covered 368 files with zero problems. Targeted
  whitespace checks passed. All fixtures used disposable state and scripted
  inference; no installed user data or real native effects were involved.

No browser, desktop, native-host, real-model or hardware qualification was run
for these increments. The broad architecture tasks remain incomplete.

## Integrated conversation, CLI and UI pass

- Model context now assembles imported history, work outcomes and inbox exchanges
  in timestamp order. Input-created work and applied corrections are deduplicated
  by identity. Future input is excluded from earlier turns, revoked derived text
  is withheld, and withdrawal/control receipts remain available. There is no
  fixed character cutoff that discards history after a long answer.
- Behind `EnableConversationPreview`, `session chat` starts without a profile or
  a conversation-creation wizard. Both CLI and UI accept subsequent text while
  inference or focused background work waits. Exiting the CLI detaches observation;
  it does not stop service-owned work. Admission and observation capabilities are
  separate so disabling new admissions does not hide existing work.
- The CLI supports shared model, pinned profile and project settings, effective
  configuration inspection, new work, work controls and input-specific controls.
  The UI retains input across unknown transport outcomes and retries the exact
  envelope before considering any new configuration change. Profile selection
  preserves an explicitly chosen conversation model, including first admission.
  Reload observes state; it does not dispatch or continue work.
- Full affected Go suites passed: agents (62.937s), server (95.492s), sessions
  (0.841s), CLI (1.782s), serviceclient (0.499s), API (0.037s) and cmd/offgrid
  (7.507s). Expanded conversation/canonical Linux race checks passed for agents
  (11.280s), server (31.525s), CLI (1.020s) and serviceclient (1.082s). The later
  CLI profile-preservation regression passed separately (0.270s).
- The canonical-conversation and agent-chat browser suites passed 159 tests.
  After final model-precedence and retry fixes, all four canonical browser
  scenarios passed (9.7s), including a profile with a different model and a
  second client's configuration change between acceptance and retry. TypeScript
  passed after those fixes. Python client/conversation coverage passed 42 tests
  (0.56s). Desktop tests passed 206 tests with one skipped; these are mocked host
  checks, not real native execution qualification.
- API contract checking, web build, source-size checks, translation completeness
  and consistency passed during the integrated pass. All 29 languages have the
  new authored input limitation message; this is not linguistic review. The
  build retains the existing large-chunk warning. Fixtures use isolated state.
  Real models, hardware performance, native integration and representative-user
  qualification remain unverified.
- OpenSpec task 9.2 is complete for the implemented migration/control contract.
  The focused Linux race suite passed agents (2.730s) and server (3.676s):
  concurrent command identity/revision checks, acceptance rollback and restart,
  expired event cursors, import receipt/ownership rollback, retained source
  manifests, rejection of legacy writers, active legacy-run preservation,
  projection failure without replay, public import retry and schema upgrades.
  Upgrade fixtures cover rollback, retained graph/inbox state and backups; the
  storage suite rejects schemas newer than the binary supports. This does not
  complete canonical deletion, unified media admission or the remaining domain,
  client, worker and resource tasks.

## Worker client integration pass

- Parent snapshots now document the already-persisted `worker_id` and
  `invalidated` fields in OpenAPI. The UI groups follow-up runs into a single
  worker activity row. Expanding it reads the worker and mailbox, without
  dispatching or continuing anything. Users can send a follow-up, explicitly
  continue a settled assignment, or Stop that worker independently of its root.
  Precise mailbox states and retained run identities are inspectable in details.
- Worker drafts and unknown-outcome command envelopes persist in the existing
  actor/workspace draft store. Explicit retries retain the original request ID,
  payload and revision, including after reload when a continuation was accepted
  but its response was lost. A stopped worker requires an explicit continuation;
  observation cannot revive it. Root-completed continuation remains unfinished.
- CLI chat adds worker listing, inspection, follow-up, continuation, independent
  Stop, closing and exact retry. Noninteractive `agent worker` inspection and
  command-envelope input use the same authenticated service, with structured
  JSON output and bounded, strictly decoded command input. No client executor or
  profile-selection requirement was added.
- Affected Go suites passed: CLI (1.057s), cmd/offgrid (2.488s), serviceclient and
  API (cached). Linux worker/control race checks passed CLI (1.122s), server
  (2.543s) and agents (5.513s). The subsequent parent-snapshot worker identity
  assertion passed the focused server suite (0.686s); the final malformed CLI
  envelope tests passed the full CLI suite above.
- Six focused browser scenarios passed (17.0s), including mailbox retry,
  independent Stop, one row per retained worker, and a lost continuation receipt
  recovered explicitly after reload. TypeScript, API contract checking, web
  build, source-size, maintained docs (369 files) and strict OpenSpec passed.
  The build retains the existing large-chunk warning. Translation checking
  reported three uncatalogued strings in the concurrent document-navigation
  work (`api/document-structure.ts`); it is not claimed as passing this checkpoint.
- The final combined worker-controls, canonical-conversation and agent-chat
  browser suites passed all 163 tests (5.3m), including narrow worker details,
  existing locale/IME layouts, parent-only speech and reload recovery. These are
  isolated HTTP fixtures, not model-quality or real-host qualification.

## Retained work-attempt pass

- Explicit continuation of a settled worker after parent completion now creates
  a distinct parent run under the existing work identity. Work records retain
  ordered `run_ids`; canonical conversation history includes earlier outcomes.
  The completed parent snapshot is preserved, and its original result appears
  once in retained model context. Historical archive metadata cannot republish
  an old attempt as the current work.
- Root model-call counters, token/time charges and previous worker allocations
  carry into the next attempt. Repeated follow-ups execute until the original
  allowance is exhausted. Worker-session limits include sessions from earlier
  attempts. The assistant can reuse another historical worker within the active
  attempt, but cannot automatically reopen completed work or revive a user-
  stopped worker. Unresolved operations, stopped work and stale instruction
  versions remain admission conflicts.
- The work pointer/history, new parent/child runs, worker binding, receipt and
  event/outbox transition share one transaction. A failed receipt rolls all of
  them back. Desktop session and execution bindings are not copied into the new
  parent. Schema 9 fences old runners; v8 records without `run_ids` retain their
  original run through the compatibility fallback and verified migration backup.
- CLI inspection gathers retained workers across attempts. The UI exposes an
  explicit follow-up from completed parent activity and follows the same worker
  identity. Seven isolated browser scenarios passed (52.2s). API contract and
  TypeScript checks passed. No real inference or host control was involved.
- Full affected Go suites passed: agents (106.288s), server (255.808s), CLI
  (2.238s), serviceclient (0.547s), API (0.124s), cmd/offgrid (10.209s). These ran
  before the final historical-archive and context-deduplication refinements;
  subsequent focused worker/attempt tests passed (14.345s), with the final
  grant-clearing assertion passing separately (0.987s). Expanded Linux race
  checks passed agents (17.684s), server (9.649s), CLI (1.148s) before those final
  refinements. The later two-worker reuse test passed (1.494s).
- Final Linux worker/attempt race coverage passed (18.101s), including historical
  archive behavior, outcome deduplication, grant clearing, repeated execution
  until exhaustion, two-worker reuse and the v8 single-run record fixture. Web
  build passed with the existing chunk-size warning. Maintained docs (370 files),
  source-size, strict OpenSpec and targeted whitespace checks passed. Hardware,
  native execution and real-model qualification remain unverified.
- Task 11.1 is complete for durable sessions/mailboxes, retained follow-up runs,
  explicit user-stopped continuation and delivery/application states. Worker
  questions, remaining dependency/budget behavior and the broader adversarial
  qualification remain in 11.2 and 11.3; those tasks are not marked complete.

## Worker allocation settlement pass

Canonical worker allocations now settle back into the root allowance after a
terminal, quiescent run. A persisted receipt retains the maximum of attempted
calls and conservative output/time charges and releases unused slots once.
Unknown or interrupted runs retain their allocations. Receipt failure cannot
publish a refund, and exact command retries do not charge or refund twice.
Restart honors existing receipts without creating new refunds. Work attempts
retain inherited child charges and settlement provenance. Schema 10 fences older
runners through the existing transactional upgrade/backup path.

Regression coverage includes completed/failed/cancelled and uncertain outcomes,
unmeasured usage, active generation, invalid charges, missing checkpoints,
transaction rollback, repeated command delivery, restart, and cumulative
attempt accounting. This is a conservative allocation settlement, not complete
input/context/tool/process/storage accounting; task 12.1 remains open.

Validation for this pass:

- Full agents suite passed (83.171s) after replacing the old assertion that
  permanently retained every child allocation. CLI (2.428s), service client
  (0.553s), API (0.003s) and CLI entrypoint (11.935s) also passed.
- Linux worker/control/budget race suite passed (24.530s). Focused server
  worker/migration/projection checks passed (2.842s), and the schema-9 upgrade
  fixture passed (0.904s).
- Final conservative handling for completed/started runs without attempt
  evidence passed focused continuation/budget tests (6.331s) and the Linux
  budget race suite (4.254s).
- Full server suite failed its route-registration fixture: this checkout has
  173 routes while the fixture guard expects 168. The additional document-analysis
  routes and frozen manifest are outside this budget patch and were preserved.
- Maintained docs (370 files), source-size, strict OpenSpec and targeted
  whitespace checks passed. Browser, desktop, Python and hardware/model
  qualification were not rerun for this backend-only increment.

## Shared inference scheduling pass

The provider scheduler now has bounded foreground/background queues with a
three-to-one contention policy and actor/work rotation within each class.
Canonical conversation turns are foreground; work roots and workers share a
background work identity across attempts. Direct chat/completion/embedding
requests derive foreground identity from the authenticated actor. Unlabelled
internal operations default to background. Scheduling metadata grants no access.

Queue admission precedes family/runtime acquisition on legacy, native,
maintenance and managed runtime-preparation paths. Family exclusion, resident
retirement and conservative memory admission remain in place. Shared model
preparation keeps the initiating scheduling scope without inheriting its client
lifetime. Cancellation, shutdown and repeated release are covered, including
cancellation racing with admission. Active generations are not preempted.

Health exposes aggregate scheduling counts and policy version without actor or
work identities. Its OpenAPI contract and generated TypeScript representation
were updated. The original lifecycle fields retain runtime-specific meaning.

Validation:

- Full Go suites passed: inference (29.543s), agents (117.448s), server
  (162.575s), CLI (2.650s), service client (0.920s), API (0.106s) and CLI
  entrypoint (13.712s). The earlier route-fixture mismatch was absent in this
  current checkout; this pass did not alter its registrations or fixture.
- Full Linux inference race suite passed (15.464s). Focused worker budget/task
  scheduling race checks passed (6.501s); deterministic fairness tests cover
  all three actors, multiple work items, background progress, bounded capacity,
  cancellation, shutdown and provider-family ordering.
- Final focused scheduling/provider checks passed (3.162s), and public health
  and route-contract checks passed (2.290s). API generation/check and TypeScript
  checks passed. Maintained docs (370 files), source-size, strict OpenSpec and
  targeted whitespace checks passed.
- Final Linux scheduling/provider/health race checks passed: inference (2.224s)
  and server (1.407s). Web build passed with the existing large-chunk warning.
  Browser journeys, desktop execution, Python and real-model/hardware workloads
  were not rerun for this scheduling increment.

Measured alternative-model selection, coordinated indexing identity, additional
resource accounting and matching-hardware overhead qualification remain open.
Tasks 12.1–12.3 are not complete. These deterministic results do not establish
real-model throughput, native backend qualification or a 9/10 product rating.

## Input-token and context-preparation accounting pass

Model calls now reserve a pinned input-context ceiling before context/model
dispatch. Measured prompt usage releases only the observed unused portion;
missing usage, probes, failed generations and historical unknown attempts retain
conservative charges. Duplicate usage settlement is rejected before changing
the ledger. Oversized or inconsistent usage cannot reach action admission.
Child settlement accounts for input charges, and work attempts retain prior
input charges instead of widening their allowance.

Context preparation has separate measured count/time counters within the existing
inference allowance. Failed calls preserve those measurements, and a late result
fenced by conversation input retains valid resource settlement without becoming
an accepted answer. Schema 11 fences old runners through the existing backup and
transactional upgrade path. A schema-10 fixture verifies that observation does
not create measurements or dispatch; explicit continuation charges old attempts
conservatively.

Job responses expose an accounting-only `resource_usage` projection and a shared
Go/OpenAPI/generated TypeScript contract. Existing CLI/Python dictionary responses
preserve it. The web context inspector shows charged and measured counters on
demand, with keyboard focus and bounded wrapping for narrow layouts. Executable
checkpoints and grants are not part of this projection.

Validation recorded so far:

- Full Go suites passed: agents (129.933s), server (192.594s), CLI (2.360s),
  service client (0.665s), API (0.125s) and CLI entrypoint (15.392s).
- Focused Linux accounting/worker/inbox race suite passed (15.973s); final
  accounting/upgrade/correction/public-projection race checks passed for agents
  (3.134s) and server (1.048s). Focused server conversation/worker/projection
  checks passed (21.848s).
- Python client/conversation checks passed (42 tests). API generation/check,
  TypeScript and web build passed with the existing large-chunk warning.
  Maintained docs (371 files), source-size and strict OpenSpec checks passed.
- Isolated Edge worker-control and resource-inspection journeys passed (4 tests,
  31.6s), including a 390px viewport and inspection without mutation. The test
  opens the nested inspection controls explicitly; entering a narrow layout
  starts a fresh inspection journey. Final API suite passed (1.401s) and targeted
  whitespace checks passed. Desktop and real-model/hardware checks were not run.

Tool/process/storage accounting, measured model selection, scheduling overhead
qualification and native/model hardware evidence remain open. Task 12.1 is not
complete; this pass does not claim full resource-accounting qualification.

## Tool invocation accounting and recovery

The runner now commits a full pinned tool-runtime reservation with its executing
marker before provider dispatch. Successful uncancelled calls release only
measured unused capacity. Failed/cancelled calls retain conservative charges;
measured deadline overruns increase charges. Reservation sequences distinguish
repeated call IDs, and a matching late settlement can record usage after Stop
without accepting an answer or reviving execution. A failed result-receipt write
leaves the durable pending reservation intact across restart.

Worker allocation settlement considers invocation count and charged runtime.
Explicit work continuation carries these charges into the next root attempt.
Schema 12 uses the existing verified backup/transactional migration path to fence
older runners. A schema-11 fixture retains absent historical measurements during
observation and conservatively reserves historical capacity on explicit resume.
Historical reservations are labelled separately from observed calls. Public
Go/OpenAPI/generated TypeScript usage includes these counters without exposing
private call identities or executable checkpoint state.

New regression cases cover failed reservation and receipt writes, unknown
outcomes, late Stop results, repeated call IDs, runtime overruns, child allocation
retention, work continuation, schema-11 upgrades and private projection fields.
The injected receipt-write test uses an already-open database handle to remove
its fault because the manager correctly refuses new handles after storage failure.
API contract, TypeScript, maintained-document, source-size, strict OpenSpec and
whitespace checks passed. Full Go suites passed for agents (168.333s), server
(269.256s), CLI (6.443s), service client (1.464s), API (8.617s), CLI entrypoint
(26.908s) and computer entrypoint (47.594s). Focused Linux accounting, worker,
inbox and public-projection race checks passed for agents (11.879s) and server
(1.059s). The executor package failed its disposable command-start fixture's
10-second deadline during the concurrent run. Its isolated
`TestPinnedStdioEndpointRunsRealExecutor` rerun passed (5.962s), with no code
change; the original timeout remains recorded. No UI implementation changed in this slice;
browser, full web build, Python and real-model/hardware qualification were not
rerun.

This measures tool invocation elapsed time. Host processes that outlive a start
call, workspace storage, measured model selection and hardware qualification
remain open; task 12.1 remains unchecked.

## Owned-process runtime accounting

Process starts now reserve their requested timeout in the same transaction as
their authenticated execution intent. Dispatch cannot proceed if this write
fails. Each pinned model-call slot supplies 16 process starts and
`16 * max(step timeout, 180 seconds)` of supervised runtime; requested commands
retain the existing one-hour maximum. Allocation and settlement use that pinned
policy across children and later work attempts.

Local and isolated supervisors report monotonic elapsed lifetime, including
launch/termination handling, on process observations and in terminal journal
receipts. This is not CPU time; it overlaps tool invocation duration. Only typed
authenticated receipts for the recorded run, host, grant and owned handle can
settle capacity. Running observations keep conservative charges, measured
overruns remain charged, and confirmed terminal failures/cancellation can settle
unused time without marking the work successful. Exact duplicate receipts do
not refund twice. Known pre-start rejection releases time but retains its start
attempt. Missing older-host timing, lost acknowledgments and unavailable hosts
retain conservative charges; Stop does not rely on inference or accounting
availability to revoke dispatch. Unobserved terminal timing is not invented.

The adapter's accounting survives stale runner snapshots. Schema 13 fences old
runners and uses the verified migration backup path. Observation of schema-12
state does not dispatch or invent old measurements; explicit continuation pins
conservative historical charges. Public usage projects only aggregate counters,
without process handles, operation identities or grant references.

Validation:

- Process accounting, continuation, upgrade, worker settlement and API projection
  tests passed (agents 3.492s, server 1.296s). Fault injection covers failed
  reservation/receipt writes and conservative restart recovery. Ownership and
  measurement rejection cases cover foreign handles/grants/runs, invalid timing,
  premature terminal claims and duplicate settlement.
- A real disposable test subprocess traversed exact runner approval, the service
  adapter, authorized host, process wait and durable root settlement. Final
  focused Windows checks passed (server 1.615s, execution 0.590s), including the
  isolated-backend fixture's journalled terminal timing. All native effects were
  confined to test-owned subprocesses and temporary directories.
- Full Go suites passed: agents 112.798s, server 159.174s, execution 8.688s,
  CLI 3.356s, service client 0.717s, API 2.263s, CLI entrypoint 11.684s and executor
  entrypoint 7.183s; the computer entrypoint package was cached (0.002s).
- Focused Linux process/accounting/worker/receipt/projection race checks passed:
  execution 2.204s, agents 11.323s, server 2.727s.
  Final isolated-supervisor timing and service-to-host subprocess race checks
  passed (execution 1.446s, server 2.764s).
- API generation/check, TypeScript and web build passed, with the existing chunk
  size warning. Maintained docs (371 files), source-size, both coordinating and
  native-workspace OpenSpec validations, and targeted whitespace checks passed.

Workspace storage accounting, measured model selection, complete independent
worker environments/integration and hardware/performance/usability qualification
remain open. This does not complete task 12.1 or qualify real container hardware.
Python/browser/desktop suites were not rerun in this pass.

## Host working copies and storage reservation foundation

The existing host library can now prepare an independent worker from a live,
already admitted source binding and an explicitly configured private store.
The supervisor narrows permissions, creates fresh session/grant identities,
binds the session to one worker run, and separates cwd, journal and process
ownership. Parent cancellation/revocation closes children; parent cancellation
waits for their shutdown evidence. Identical live preparation retries return the
same worker, and changed retries conflict. Persistent artifacts never restore
live grants after restart. Unsupported preparation has no shared-folder fallback.

Ordinary copies include the actual working tree regardless of Git tracking or
ignore rules. Git administration is explicitly excluded so linked-worktree paths
cannot redirect a worker at the user's original index. Required excluded/missing
inputs fail preparation. Manifests pin source binding, assignment versions,
paths, kinds, byte sizes, modes, digests and exclusions. Copying validates source
bytes again, and completion compares a fresh source inventory. External editors
can still change files afterwards; this is not an atomic filesystem snapshot.
Unsupported links/special files fail with a preparation limitation. Journals and
private state remain outside worker-visible roots; copies do not use hard links.

Host-private SQLite reserves logical-byte capacity before copying. Policy defaults
are 512 MiB per copy, 2 GiB total, 10,000 entries per copy and 32 active retained
preparations. Failures and interrupted preparation/removal retain reservations.
Before new admission, existing ready copies are remeasured, including growth
whose earlier measurement receipt was lost. Unknown measurement conservatively
blocks further admission. Confirmed cleanup releases capacity; closing a worker
alone retains reviewable files. These are admission/accounting limits, not an OS
disk quota for arbitrary trusted native processes or physical allocation metrics.

Validation recorded:

- Working-copy tests passed initially (2.050s) and after growth refresh (1.619s).
  Tests exercise independent files/cwd/grants/journals/processes, sibling Stop,
  parent shutdown, read-only narrowing, retry identity, dirty/untracked input,
  Git-index preservation, missing inputs, unsupported links, changed source
  bytes, admission limits, release, injected reservation/completion/removal
  failures and restart without grant restoration.
- Broader Go suites passed: execution 9.999s, agents 91.666s, server 136.730s,
  CLI 2.388s, executor entrypoint 7.424s and CLI entrypoint 10.390s. API, service
  client and computer entrypoint checks used cached passing results.
- Linux working-copy/process/isolated-fixture race checks passed (3.055s).
- Full Windows execution suite passed (6.269s), including added lost-growth
  measurement/restart coverage. Windows symlink fixtures were skipped because
  this host lacks symlink-creation permission. Final Linux working-copy race
  checks passed (1.822s), including unsupported links and cleanup preserving an
  external symlink target.
  After tightening directory-type checks and separating unmeasured preparation
  from measured stored bytes, final focused Windows checks passed (2.269s) and
  Linux race checks passed (1.796s). Final docs covered 372 maintained files with
  no reported issues; source-size and whitespace checks remained clean.
- Source-size, coordinating/native-workspace OpenSpec and targeted whitespace
  checks passed. No installed state or user folders were changed; filesystem and
  process effects were confined to disposable test fixtures.

This is a host-side foundation. Native bridge preparation/observation, service
worker admission, root storage-budget projection/settlement, environment templates
and change-set integration remain unimplemented. Automatic execution-worker
admission is not enabled, and tasks 12.1 and 13.1 remain unchecked. Renderer,
Python, desktop and real container/hardware qualification were not rerun.

## Asynchronous native worker preparation transport

The private stdio executor and enrolled-host bridge now expose typed begin,
inspect and cancel preparation operations. A native-approved working-copy policy
is required; existing host setup leaves it disabled. Model operation dispatch
still rejects supervisor operations. The parent journal commits acceptance before
background copying, and inspection never starts work. Identical requests return
the existing preparation; conflicting request reuse fails. Control requests can
run while preparation waits, and parent Stop cannot acknowledge quiescence until
pending preparation settles. Process cancellation precedes that wait.

Native endpoints route exact prepared child bindings to their independent
sessions and journals. The bridge validates the parent, assigned run, narrowed
permissions and host-attested child context before recording a child. It rejects
foreign contexts and arbitrary child-job admission. Parent cancellation and
independent child Stop retain their existing ownership boundaries.

Preparation completion persists artifacts without the live child context. A
failed completion write closes the child and reports uncertainty. Restart keeps
the files and reservations but returns `interrupted` for a formerly ready
preparation; neither retry nor inspection reconstructs its grant. Preparation
status is bounded to 128 commands per supervisor lifetime, independently of the
active-copy storage bound. Cancellation of a completed preparation does not
implicitly stop its worker; that uses the child's separate Stop operation.

Validation recorded:

- Focused Windows asynchronous lifecycle, retry, cancellation, foreign-binding,
  claim/completion-failure and child receipt checks passed (0.735s).
- The real pinned Windows stdio executor passed independent preparation,
  exact retry, original-folder preservation, child dispatch/Stop/receipts and
  restart without grant restoration (6.254s, including existing shutdown checks).
- Enrolled-host preparation, child routing, foreign-context rejection and
  model-route exclusion tests passed (0.662s).
- Linux working-copy/preparation/enrolled-bridge race checks passed (2.147s).
- The Linux real-executor fixture could not complete its nested `go build`
  within its existing 60-second deadline, including one retry. It never reached
  the transport assertions; Linux real-executor qualification remains unverified.
- Final focused Windows preparation/bridge regressions passed (0.808s) after
  ordering root process cancellation before waiting for preparation quiescence.
- Broader Go suites passed for execution (12.853s), agents (84.961s), CLI (3.353s),
  service client (0.700s), API (0.145s), CLI entrypoint (10.811s), executor
  entrypoint (7.724s) and computer entrypoint (cached). The server suite failed
  its route-registration fixture: concurrent document work added
  `/v1/document-analyses/export`, absent from that frozen fixture. This pass did
  not rewrite document-route expectations to conceal the failure.
- Source-size, maintained docs (372 files), both OpenSpec validations and targeted
  whitespace checks passed. All native effects used disposable test directories
  and owned fixture processes; installed data and real desktop controls were not
  touched.

This completes private preparation transport, not canonical execution-worker
admission. Root storage-budget projection, service-owned child registration,
transport-managed artifact release, environment templates and change-set
integration remain open. Tasks 12.1 and 13.1 stay unchecked. Browser, Python,
desktop, real-container and hardware/performance qualification were not rerun.

## Canonical worker binding and root storage accounting

The service controller now connects canonical assignments to the native
preparation transport. It reserves root-work capacity before calling the host,
records returned storage evidence, and separately registers/binds a ready child.
Inspection alone creates no registration. The final canonical bind checks the
current root/child state, instruction version, control epoch and existing storage
receipt. A failed bind cancels the new native child. Independent Stop uses that
child's binding even before its first process starts; parent Stop also covers
pending preparations. Follow-up runs retain context and charges, but clear the
previous run's native binding.

The host registry now fences cancelled jobs locally, including cancellation
before first admission. Prepared children are private, single-run resources, not
additional entries in the general host picker. Child revocation does not close
the borrowed parent transport. Parent revocation removes descendant entries, and
parent cancellation blocks further child dispatch.

Schema 14 adds durable root-work storage reservations. Identical retries share
one reservation; changed payloads and stale/foreign assignments fail. Branches
share the pinned work ceiling (initial service default 2 GiB). Unknown attempts,
failed preparation and restart retain their charge. Live shrinkage does not
refund headroom; growth can exceed the allowance and then prevents further
admission. Only the exact copy's confirmed release returns capacity. Subsequent
work attempts retain prior reservations, and stale model checkpoints cannot
overwrite controller accounting. Public usage exposes aggregate logical bytes
and copy counts without host grants or private copy identities.

Focused checks passed for canonical preparation/binding/independent Stop through
a real temporary native host (1.182s), host registry isolation/lifetime (0.944s),
and storage retry/restart/fault/continuation/shared-capacity boundaries (1.978s).
The fault fixture confirms a failed reservation latches the manager against
dispatch; recovery uses a newly loaded manager rather than clearing that latch.
API type generation/check, TypeScript and source-size checks passed.

Automatic editing/test-worker assignments and model tool admission, dependent
candidate construction, workspace environment templates, transport-managed
artifact release, and preservation-safe change-set integration remain unfinished.
Tasks 12.1 and 13.1 remain unchecked; this is not a backend/model performance or
usability qualification.

Broader verification passed:

- Go execution 10.585s, agents 94.656s, server 140.542s, CLI 2.895s,
  service client 1.259s, API 2.377s, CLI entrypoint 11.261s and executor
  entrypoint 8.910s; the computer entrypoint used a cached passing result.
  The route fixture failure from the preceding pass is absent in this run.
- Focused Linux race checks passed for execution 1.674s, agents 4.384s and
  server 2.104s, including the native-host canonical binding/Stop fixture.
- API generation/check, TypeScript and production web build passed. The build
  retains its existing large-chunk warning.
- Maintained docs (373 files), source-size, both OpenSpec validations and targeted
  whitespace checks passed. Python, browser/desktop journeys, hardware/model
  qualification and the Linux nested real-executor build were not rerun.

## Automatic assignment and sealed-output pass

Schema 15 extends canonical delegation with temporary edit/test assignments.
The service prepares independent host working copies before admitting child
generation. Discovery requires the host's explicit prepared-worker support;
dispatch requires the separately registered child binding and exact capability
identity. Test processes retain the existing requirement for write authority
because trusted native tests can produce artifacts. Test mode does not expose
the model-facing file-write tool. Custom executables still require exact approval.

Native preparation has a separate bounded controller slot, so slow preparation
does not occupy foreground conversation delivery or a generation slot. Canonical
graphs admit at most two active children. Stop can cancel preparation before its
Begin command, leaving a durable cancellation record, or stop an already prepared
child before service registration without cancelling its parent.

The host working-copy store now captures sealed file operations and immutable
content with chunk-verified bounded retrieval. It preserves source permission
metadata, rejects unrepresentable changes, reuses closed-copy storage headroom,
and conservatively recovers interrupted capture/validation/application states.
These host artifacts are not yet the complete service/public change-set lifecycle.

Focused Windows checks passed after the process-authority correction:
execution 1.404s, agents 0.265s, server 3.063s. These include real private-file
editing and an explicitly approved owned test process that verifies fixture input
and writes a proof file only in its temporary working copy. They also cover
Stop-before-Begin, independent child Stop, sealed-output restart, storage headroom,
and unsupported-change rejection. Unix permission-change coverage skips on Windows.
Source-size and targeted whitespace checks passed. API generation/check,
TypeScript, both OpenSpec validations and maintained-doc checks passed earlier
in this pass. The broad Go run did not produce a final result before interruption;
the Linux race run also lacks a final server result. Neither is counted as passing.

Still incomplete: dependent edit/test candidate composition, follow-up reuse of
prior file changes, service/public sealed-output transport, combined validation,
preservation-safe integration and crash recovery, host-access setup/background
lifetime, remaining conversation/project/multimodal client workflows, and full
model/backend/usability qualification. Tasks 13.1–13.4 remain open. The earlier
29% acceptance-heading count is not a product-completion estimate.

## Private sealed-output transport

The local supervisor, pinned stdio executor, and authenticated host bridge now
share typed sealed-output commands. Sealing first records its request in the
native journal and then quiesces/captures the worker asynchronously. Status reads
cannot initiate capture. Exact retries observe the recorded outcome, including
after executor restart, without restoring a child grant or repeating capture.
Stale assignment versions are rejected before closing the live worker.

Change-set pages and chunk-verified blobs are retrievable through the private
transport. Responses respect the frame limit; a busy capture returns a conflict
for artifact reads rather than blocking the transport's cancellation lane.
Model-operation routes cannot call these supervisor commands. Parent/actor
binding checks remain independent of artifact identifiers.

Verification passed: full execution tests 7.096s and real executor tests 4.084s
on Windows, including capture, restart and exact blob retrieval through stdio.
Focused Linux race checks passed in 1.636s for asynchronous capture, bridge
retrieval and change-set recovery. Source-size checks passed. This does not
qualify original-folder integration or complete the public change-set API.

## Automatic capture and root-budget settlement

Schema 16 records capture reservations and result receipts in the canonical
root ledger. One controller owns the first capture dispatch; concurrent/retried
admission observes that intent. Capture borrows a bounded amount from the root's
unallocated storage allowance, and the native store enforces that ceiling before
writing output blobs. A matching sealed result and measured copy receipt settle
unused extra capacity. Missing receipts and restart retain the charge; observed
growth beyond the allowance is charged and prevents further admission.

Completed editing/test workers now enter automatic capture before parent
synthesis. Dependent admission waits for captured results and rejects an
unavailable capture. Parent handoffs contain the immutable change-set identity,
operation digest, assignment versions and state without native grants. A sealed
proposal is not validation, integration or semantic completion.

The updated end-to-end fixture passed on Windows (3.439s), including real private
edits, an owned test process, automatic sealing and refund of unused capture
capacity. Focused accounting/concurrency/restart/receipt checks passed (agents
1.189s, execution 0.394s). The broader run passed agents 95.526s, execution 8.957s
and executor 4.186s; server 117.825s failed its route-registration baseline-shape
assertion (179 routes, 5 outer middleware). This pass adds no HTTP routes.
An initial Linux race attempt passed agents/execution but server compilation
encountered a concurrently changing document-synthesis handler; it is not counted
as a passing server race run. After that handler became available, the focused
Linux race rerun passed agents 2.930s, execution 1.167s and server 5.211s,
including concurrent capture ownership and the real automatic worker fixture.
Public change-set inspection, dependency candidate
composition, preservation-safe integration and associated fault qualification
remain incomplete.

## Dependent worker candidates

Schema 17 in the control store and schema 3 in the native copy store retain
exact prerequisite change-set references in preparation requests and input
manifests. The service derives these from recorded worker dependencies, checks
them at storage reservation and final environment binding, and rejects missing,
forged, stale or invalidated references. Caller-owned request slices cannot
modify accepted reservations after persistence.

The native host composes the complete candidate inventory before copying. It
checks source ownership, assignment versions, exact operation digests and file
preimages, resolves bounded transitive dependencies once, and detects overlapping
changes or orphaned paths. Every candidate file is copied into a new private
directory from either the approved original or a hash-verified sealed blob.
Capacity is reserved before writing; failed preparation never admits a worker or
changes the original. Required inputs can be supplied by prerequisite changes.
A dependent worker's own sealed diff excludes inherited edits and retains the
candidate manifest as its baseline.

An end-to-end service fixture now runs an editing worker followed by a separate
test worker. The owned helper process succeeds only after reading the editor's
sealed content. Both workers receive independent copies and receipts, the root
settles their shared storage allowance, and the original remains unchanged.
Native tests cover independent and transitive composition, retained deletion,
exact capture despite later private-file edits, user-edit conflicts, overlapping
changes, corrupted blobs, foreign ownership, version/epoch mismatch, abandoned
inputs, storage limits and new user files underneath a proposed directory deletion.

Validation in this pass:

- Windows full package checks passed: agents 82.937s, execution 10.567s,
  executor 4.922s. The full server run failed
  `TestDocumentEvidenceThroughConversationHTTP/records.txt` with HTTP 409 on
  a conversation follow-up; the isolated document test subsequently passed
  (1.404s). This does not establish a green full server suite or resolve that
  intermittent conflict.
- Focused Windows candidate/admission/capture/automatic-worker checks passed:
  agents 3.869s, server 3.061s.
- Focused Linux race checks passed: execution 1.864s, agents 5.898s,
  server 10.537s. After the final canonical admission hardening, the affected
  agents/server race checks passed again (5.135s / 5.023s).
- Source-size, both relevant OpenSpec changes and targeted whitespace checks
  passed. Maintained-doc links initially passed; the final check reported a
  missing `document-synthesis-review-2026-11-13.md` link from the concurrently
  edited document-synthesis evidence page. No document files were removed by
  this pass.

This implements dependent private candidates, not automatic integration or
semantic validation. Public change-set inspection, validation/integration receipts, displaced-content
preservation and backend qualification remain open. Task 13.2 remains unchecked.

## Retained editing-worker follow-ups and partial files

Control schema 18 pins a retained worker's exact prior change-set identity,
operation digest, source run and assignment versions when continuation is
accepted. It rejects continuation before the private output has been captured.
The next run retains the worker session and model context, but receives a fresh
copy and narrowed execution binding. Earlier prerequisite files are retained
through the sealed input manifest instead of rerunning the old graph.

A new root attempt retains its selected source as metadata. The service must
resolve the same actor/workspace source against live host authority before
binding it; expiry or revocation prevents preparation and cannot recreate a
grant. Native schema 4 supports exact earlier source-run references under that
same live folder grant, including transitive candidates from several attempts.
This does not silently relink historical artifacts to a different or renewed
grant. Such relinking, and its source-access review, remain unimplemented.

Settled failed/cancelled workers also receive bounded output capture. Parent
partial synthesis waits for capture or its recorded uncertainty. Capture does
not change the worker outcome or make a failed prerequisite successful. User
Stop continues to prohibit model-driven revival; an explicit continuation can
use its captured partial files. Unacknowledged process shutdown and uncertain
capture cannot be treated as completed file preparation.

The end-to-end service test now covers live and revoked source access, plus Stop
after a private edit while a model turn is active. In the Stop case, it confirms
the cancelled outcome and sealed partial files before explicitly continuing the
same worker in another work attempt. The continued worker reads those exact
bytes, uses a new private environment, and retains all previous storage charges.
The original user folder remains unchanged throughout.

Validation:

- Full Windows agents, execution, server and executor packages passed (59.868s,
  9.627s, 108.669s and 4.521s). The earlier document-follow-up failure did not
  reproduce in this run; its cause is not claimed fixed by these changes.
- Focused Linux race tests passed: agents 7.302s, execution 1.788s,
  server 10.642s. The additional real Stop/continue fixture passed on Windows
  (5.479s); the final Linux race run of all three end-to-end cases passed
  (17.027s).
- Maintained-doc links (376 files), source-size, targeted whitespace and both
  relevant OpenSpec checks passed.

The complete validation/integration lifecycle, public result review, grant
relinking, failure/restart qualification and rollout gates remain open. No broad
execution or qualification acceptance task was checked off by this increment.

## Native validation and combined integration candidates

Test-purpose workers now bind process requests to their admitted input manifest.
The host checks those inputs before dispatch, retains actual settled process
receipts, and persists validation evidence after owned processes settle. Passed,
failed, not-run, changed-input and uncertain validation remain distinct. Evidence
includes exact commands and request/receipt digests, with bounded owner-scoped
retrieval. A zero exit status is process evidence, not semantic task completion.

After successful validation, native capture builds a separate ready candidate
containing the combined prerequisite edits, excluding test-generated logs and
caches. It reverses the exact prerequisite lineage to retain the original
baseline, captures immutable blobs and charges the same root storage allocation.
The main test-worker output remains independently retrievable. Candidate failure
does not discard it. Repeated requests observe the recorded candidate; changed
requests conflict; interrupted candidate creation remains uncertain after restart.
The service validates candidate identity, source, worker, instruction/control
versions, digests and validation linkage before storing a grant-free projection.
Late evidence cannot revive stopped work or authorize integration.

Verification in this increment:

- Full Windows agents, execution, server and executor packages passed (93.115s,
  21.057s, 154.173s and 7.839s). These ran before the subsequent unused replacement
  primitive was added; its focused tests are recorded separately below.
- Focused Linux race tests covering validation, combined candidates, capture
  accounting, service handoff and replacement passed: execution 15.011s,
  agents 11.800s, server 21.576s.
- Real helper-process fixtures cover failure, changed inputs before/during/after
  validation, missing/unmeasured receipts and evidence-persistence failure.
- Candidate fixtures cover combined update/create/delete operations, reuse in a
  later worker, exact restart/retry behavior, capacity rejection, changed inputs,
  abandoned prerequisites and failed blob/final receipt writes. Agent tests reject
  forged evidence and retain late candidates without reviving cancelled work.

This increment raised the native copy-store schema to version 6 (validation evidence and separate
combined baselines); the process journal is version 2; agent storage is version
19. Unsupported older versions reject these stores instead of silently discarding
the new control evidence.

## Displaced-file preservation primitive

An internal replacement primitive now uses Windows replacement-with-backup and
Linux atomic exchange. It retains the displaced contents in supervisor-private
storage, verifies both observed versions, and never retries, removes the backup
or blindly rolls back. Directory identities are pinned; Windows additionally
holds non-delete-sharing handles for local path ancestors and rejects reparse
directories. Cross-volume stores and unsupported platforms do not fall back to
ordinary overwrite. The caller must journal intent and retain all possible
names after an uncertain operation.

The implementation follows the documented
[Windows replacement behavior](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew)
and [Linux rename semantics](https://man7.org/linux/man-pages/man2/rename.2.html).
Windows has no supported ReplaceFile write-through flag, so process-crash tests
cannot stand in for power-loss qualification.

Windows temporary-filesystem tests pass for normal replacement, edits after the
preimage check, newer edits after replacement, lost acknowledgments, cancellation,
invalid preimages/staging, pre-existing backups and invalid paths. Windows symbolic
link tests are explicitly skipped because the process lacks symlink privilege;
Windows link-substitution qualification remains unverified. Linux race tests also
exercise linked parents, a late file-link substitution and parent-directory rename.
The final full Windows execution package passed (21.762s); the final Linux
replacement race suite passed (1.044s). Maintained-doc links (376 files), source-size,
targeted whitespace and both relevant OpenSpec validations passed. No web, desktop
UI, Python, model, hardware or usability qualification was performed in this
increment.

At this checkpoint the primitive was not connected to original-folder integration
or Session.Write. The next increment below connects the native integration path;
Session.Write, service controls, public review and rollout qualification remain
open. Neither a ready candidate nor primitive tests complete task 13.2 or 13.3.

## Journaled native integration and real process-exit recovery

The private supervisor protocol now accepts an integration command asynchronously
and exposes paged, observation-only receipts. Native integration verifies the
exact ready candidate, instruction/control versions, live source authority and
original input manifest. It reserves staging/quarantine capacity before mutation.
Original files are updated through displaced-content preservation; new files and
directories use exclusive moves; deletions move into private recoverable
quarantine. Directory/file conversions are separate ordered steps and can have
partial outcomes. No path touches the user's Git index or creates commits.

Each step commits a dispatch marker before its native operation, then records
actual effects and observations. File receipts and aggregate status distinguish
integrated, pending, conflict, partial and uncertain outcomes. Baseline conflicts
retain an inspectable source path. An accepted request retry only observes its
original attempt, even after process restart or a lost outer acknowledgment.
Recovery retains known completed steps, marks an interrupted dispatch uncertain,
and leaves later steps pending. It never rolls back or replays files to discover
what happened. Quarantine cannot be abandoned or released as an unused proposal.

The loop pins its source directory without holding the source-session mutex for
the whole batch. It rechecks authority at each step and connects live source
revocation to cancellation. Native status requests do not wait for copying or
integration. No inference slot or model call is used by this protocol.

Native copy storage is now schema version 8, including integration commands,
per-file receipts and persisted failure details. Original artifacts and previous
copy-store records remain intact; unsupported older binaries reject the version.

Checks performed in isolated temporary state:

- Full Windows agents, execution, server and executor packages passed (173.036s,
  80.507s, 266.014s and 17.678s). After the failure-detail addition, the full native
  execution package passed again (33.175s).
- The Linux integration/replacement race suite passed (48.101s). The final focused
  conflict, private-protocol and process-exit recovery race run passed (24.274s).
- Real subprocesses exited immediately after each of five native file dispatches,
  bypassing defers and database/session cleanup. Reopened stores retained earlier
  receipts, marked the interrupted step uncertain, and did not dispatch remaining
  steps when the same request was retried with explicit test-only folder authority.
- Native tests cover combined edits and deletions, recoverable original contents,
  file/directory conversions, source-manifest ordering, foreign access, changed
  request reuse, stale versions, unvalidated sets, bounded storage, concurrent
  creates/edits, revocation, failed/ignored dispatch writes, failed file receipts,
  completion persistence, lost acknowledgments and immutable response projections.
- The final failed/ignored receipt-persistence regression run passed (10.614s).
  Maintained-doc links, source-size, targeted whitespace and both relevant
  OpenSpec validations passed. Web/browser, desktop UI, Python, model, hardware
  and representative-user qualification were not run in this increment.

This is native execution capability, not completed automatic product behavior.
The service still needs integration admission/accounting and work-control fencing,
post-integration worker continuity, and CLI/UI inspection/review. Session.Write
still uses its existing replacement path. Windows late-link substitution,
power-loss/disk-full behavior and complete backend qualification remain unverified.
Tasks 13.2 and 13.3 remain unchecked; no automatic rollout or 9/10 implementation
rating follows from these checks.

## Service-owned worker integration and receipt recovery

The conversation preview now exposes `workspace_integrate_changes` to the primary
assistant when a ready worker candidate exists. Workers cannot acquire that tool.
The service admits an exact completed test worker through the existing durable
tool checkpoint and capability policy. Its captured prerequisite closure must
include all current completed editing branches; references retain exact digests,
instruction versions and source identities. Changed duplicate references and
cycles are rejected. Ready candidates alone do not satisfy this contract.

The root storage ledger reserves integration capacity transactionally before host
dispatch. Concurrent admission has one dispatch owner; retries only inspect the
saved attempt. Unknown outcomes retain reservations and measured overruns remain
charged. Settled native measurements release unused capacity. Integration state
is retained with the captured worker output; late receipts update evidence and
accounting without changing a stopped work item's outcome. Agent storage is now
schema version 20.

Specific integration cancellation has a durable cancellation record when it
arrives before dispatch. Cancelling that operation does not revoke the parent's
execution session. Global Stop waits for integration settlement before native
acknowledgment. The service checks current instructions/control state while
observing an accepted operation and requests cancellation when fenced.

A bounded controller observes unresolved integration receipts outside inference
slots. Observation does not restore expired authority or dispatch missing work.
The existing execution-recovery control can reconcile an integration result from
saved or authenticated host evidence; it leaves the work interrupted for explicit
continuation. Recovered file effects do not establish semantic task completion.
The completion guard accepts current, matching integrated receipts, so an edit
request does not demand a second direct write after successful integration.

Checks in isolated test folders and owned helper processes:

- The full Windows agents, execution, server and executor suites passed
  (107.129s, 40.962s, 164.254s and 8.167s). The JSON transcript is
  `offgrid-service-integration-e69478b5e4f34939a87eb9d9ebdc5c33.jsonl` in this
  machine's temporary directory; it is local evidence, not a committed artifact.
- The service journey runs editing and test workers, verifies the exact edited
  input through a real helper subprocess, integrates the combined candidate, and
  checks that generated test output is absent from the original folder. Dropping
  the final service result recovers the receipt without repeating integration or
  claiming work completion. Disabling preview admission preserves receipt reads;
  revocation prevents further host retrieval. The final focused Windows service
  run passed (15.072s).
- Ledger tests cover concurrent admission, restart, conservative unknown charges,
  actual overruns, late settlement after Stop, corrections, stale epochs,
  missing validation, omitted editing branches, changed duplicate references,
  foreign sources and dependency cycles. The final focused Windows agent run
  passed (10.341s).
- Native cancellation tests verify cancellation before delayed dispatch, waiting
  for settlement, and the distinction between operation and parent cancellation.
- Linux race tests passed for the agent integration ledger/recovery, service
  integration/lost-response journey and native protocol/cancellation
  (9.554s, 11.531s and 6.419s). An earlier Linux build overlapped a source-file
  move and failed compilation; it was rerun against the settled file layout.
  The final agent/server race run, including the overrun, admission-disable and
  revoked-source checks, passed (10.245s and 11.301s).
- Source-size, targeted whitespace, maintained-document checks and both relevant
  OpenSpec validations passed. No web/browser, desktop UI, Python, model,
  hardware or representative-user qualification was performed in this increment.

This completes a tested service path, not the complete execution-worker product.
Integration of retained follow-ups across attempts, post-integration worker
continuity, complete public change inspection/review,
environment templates, retention and cleanup, the existing Session.Write race,
background-grant setup and remaining backend qualification are still open.
Automatic integration remains within the default-off conversation preview and
requires an explicitly configured private-copy host. Tasks 13.2 and 13.3 remain
unchecked; these checks do not establish a 9/10 implementation rating.

## Retained worker follow-ups before and after integration

Follow-up preparation now carries references to completed native integration
receipts from the work's existing ledger. The supervisor verifies each reference
against the same live folder scope, its completed per-file journal, and the
immutable validated candidate manifest. Only fully recorded integration marks
earlier prerequisite effects as already applied. Partial/unknown receipts, wrong
owners, missing receipts and changed result digests cannot do so.

A worker continuing after integration receives the current source inventory,
including newer user edits and files recreated after an earlier deletion. Native
manifests distinguish already-applied prerequisites from pending proposals. The
next combined candidate reverses only pending prerequisites when deriving its
baseline; it does not propose the old edits again. Source and integration evidence
remain separately recorded. Preparation still makes a new private copy and fresh
run binding; no prior worker grant or writable directory is reused.

The service also supports a new assignment depending on a unique current,
completed assignment key in the same attempt. It retains that exact link as
settled evidence instead of starting another worker or allocating another model
budget for it. This enables a new test worker to validate a retained editor's
follow-up. Ambiguous, invalidated, unfinished or stale dependencies are rejected;
cycles remain invalid. Cross-attempt prerequisite closure requires the same work,
actor and source scope. Existing session-count and root-budget limits still apply.

The native store is schema version 9 and the agent store is version 21. Existing
records remain intact; earlier binaries reject unsupported versions instead of
interpreting the new applied-prerequisite semantics incorrectly.

Checks in isolated temporary state:

- Native tests pass for continued editing after integration, preservation of a
  newer user edit and recreated file, fresh validation, and integration of only
  the new operation. Missing/foreign/partial/unknown/digest-mismatched proofs are
  rejected without modifying the source (focused Windows run: 3.435s).
- Service fixtures pass for continuing both unintegrated proposals and already
  integrated work. The same editor session obtains a new private environment;
  a new test assignment consumes its current result; the parent integrates the
  validated follow-up. They check inherited charges, advancing model usage,
  duplicate-command deduplication, and exclusion of generated test output
  (final focused Windows run: 31.728s).
- Retained-dependency tests cover current completed inputs, stale versions,
  unfinished results, ambiguous keys, invalidation, cycles and legacy behavior.
- Linux race coverage passed for the integration ledger/retained dependencies,
  native follow-up and rejection paths, and the service worker journeys
  (14.096s, 10.504s and 47.136s respectively).
- The final Linux race run for both integrated and unintegrated follow-up
  journeys passed (21.519s).
- Full Windows agents, execution, server and executor suites passed. The
  `go test -json` package completion events reported 318.691s, 158.434s,
  405.037s and 9.167s respectively. Local transcript:
  `offgrid-integrated-continuity-22c9626e922642f3a088728e3b3fb72d.jsonl` in the
  machine's temporary directory. These concurrent test runs are correctness
  checks, not controlled performance measurements.
- Source-size, targeted whitespace, maintained-document checks and both
  coordinating/native OpenSpec validations passed. Web/browser, desktop UI,
  Python, installed-model, hardware and representative-user qualification were
  not run in this increment.

Complete CLI/UI change inspection and review, environment templates, cleanup,
renewed-grant relinking, background setup and qualification remain outstanding.
This increment does not complete the broad execution-worker acceptance tasks or
establish model/backend/usability qualification.

## Still required by the approved plan

The CLI/UI text journey now uses this API behind the default-off admission flag.
Canonical deletion, project-folder identity/automatic configuration loading and
the complete unified modality/context catalog remain. Both legacy transcript
projection and model context now interleave inbox and work results and suppress
duplicate input-created work prompts and applied corrections by identity.
Further UI project/resource inspection and pending-question interactions remain.
Work attempts retain spent charges and release settled unused worker slots;
additional resource accounting remains required. The result/change-set record kinds still reserve domain
identities; executable change-set lifecycles are not yet implemented by them.

Independent worker environments, preserved-file integration and retained-worker
follow-ups now have the implementation and focused evidence recorded above.
Their broader acceptance tasks remain open for environment templates, cleanup,
background authority and qualification. Complete focused-work/question routing,
resource accounting and scheduling qualification, and backend/model/usability
qualification also remain required.
No automatic behavior or 9/10 implementation rating is justified by this record.

## Public change inspection (2026-11-14)

The service projects saved proposals and candidates through owned work resources.
File, validation, integration-receipt and blob views are bounded and use the
original live source binding. Reads do not dispatch or restore authority.
Metadata remains visible after revocation; content does not. Public receipts
exclude grants, private working-copy/displaced-file paths and process commands.
File inspection preserves the recorded integration state instead of reporting
the immutable manifest's earlier sealed state.

The CLI supports `/changes` and `session changes`; the Python and generated web
clients expose the same GET contracts. CLI content pages preserve full bytes,
check identity/cursor continuity and verify the final digest. Partial streamed
output with a nonzero exit is not a verified file. The web Changes panel shows
files, validation and per-file partial/conflict receipts. Downloads verify all
chunks before publication. Source failures clear cached detail; work/account
changes remount its inspection state. Refresh retains previously loaded pages
and expanded reviews.

Checks completed in isolated state:

- Full Windows CLI and service-client suites passed (2.514s and 0.897s).
- All seven native service worker scenarios, including both follow-up journeys
  and public change inspection, passed (59.884s).
- Linux race checks for those service journeys and CLI paging/integrity passed
  (54.772s and 1.313s).
- The initial six Edge browser scenarios passed: pages and partial receipts,
  full verified downloads, corrupt bytes, mismatched work identity, revocation,
  and narrow RTL keyboard use. A subsequent refresh-persistence expansion
  reproduced Chat's automatic-scroll interference. Pointer/focus interaction
  with work evidence now pauses following until explicit scrolling or Latest
  restores it. The failing multi-panel review, existing output-follow/manual
  scrolling test, and narrow RTL test passed together (3 tests, 28.2s).
  The complete affected Chat/worker/change browser run is still in progress.
- API generation drift, TypeScript/production build, source-size, translation
  structure, documentation and both OpenSpec validations passed. The build
  reported existing large-chunk warnings. The nine original languages have
  authored change-inspection labels; preview languages use explicit fallback
  for these 25 new messages. No linguistic review is claimed.
- Focused Python conversation/client checks passed, 43 tests, using the locally
  cached uv Python 3.12/pytest environment without downloads. The broader SDK
  and runtime run initially reported 165 passes and two setup errors in
  `test_document_synthesis.py`: its `service` fixture was not imported. Reusing
  the existing isolated HTTP fixture, as the adjacent SDK tests do, repaired the
  harness; the complete SDK/runtime rerun passed all 167 tests (45.54s).
- Desktop contract tests passed 206 tests with one skipped (44.120s). These are
  contract tests, not a packaged native desktop qualification.

This increment does not complete interactive conflict resolution, original-file
recovery UI, canonical domain-record completion, multimodal continuity or the
remaining resource/native/model/usability qualification gates. Admission remains
default-off. No whole-plan completion or percentage is inferred from these tests.

## Direct file-write preservation (2026-11-14)

The older direct host write path also had a preimage-check/rename race. It now
uses the displaced-content-preserving primitive with durable private journal
records. The proposal is retained separately from the exchange operand, so a
later external write cannot destroy the only retained proposed version. Normal
receipts record a replacement identity and observed original retention. Lost
acknowledgments remain uncertain; final receipt failure revokes dispatch. Receipt
inspection exposes bounded recovery metadata without restoring grants or
promoting unknown operations to completion.

Recovery retention is admitted before staging. Prepared workers consume their
existing allocated copy storage, and host measurements include recovery files.
Unknown reservations survive restart. Standalone journals cap retained
replacements at 128 and logical reservations at 2 GiB. The execution journal is
now schema 3 and the copy store schema 10; old receipts and data survive the
transactional upgrade, and unsupported newer schema versions are rejected.
The unjournaled host-side `Session.Write` helper retains exclusive creation but
cannot bypass the replacement contract. Supported supervised endpoints continue
to replace through their journal.

Focused Windows tests passed (4.851s), covering competing edits before/after
replacement, lost acknowledgment, failed receipt persistence, ordinary endpoint
retries, a real subprocess exit after replacement, restart without replay,
retained quotas, version upgrade, recovery observation, and prepared-worker
storage accounting. Linux race checks for the same paths passed (1.842s). Source
size and targeted whitespace checks passed. Full execution/executor checks are
still running.

Other checks in this pass remain explicitly incomplete: the full server run hit
the default 10-minute timeout (the CLI command package passed); the companion
run passed 73 tests and failed two local-browser opens with `network_unavailable`.
The large Chat/worker/change browser run is still running and has reproduced a
React provider failure in one locale case. These are not counted as passes and
require isolated follow-up. The latest production web build passed with its
large-chunk warnings.

This repair does not qualify every filesystem/backend, Windows late-link races,
power loss or disk-full recovery. Direct-write artifact export/review and
retention cleanup remain unfinished. Unsupported same-volume preservation
retains a proposal without changing the target; it is not yet the complete
reviewable-change fallback required by the plan.

## Retained integration files and qualification follow-up (2026-11-14)

The saved-change inspector now exposes recovery files through the existing
authorized supervisor channel. `recovery` pages enumerate retained files by
integration step; `preserved` pages bind downloads to both an artifact identity
and an observed SHA-256. This works through the service API, structured CLI,
Python client and the web **Preserved files** view. Private storage paths and
grants are excluded. The web download uses a separate `retained-` filename.

Recorded displaced-file hashes and current observations remain separate.
An interrupted operation can expose an unconfirmed staged or displaced copy
without changing its outcome. Corrupt content, unsafe selectors, foreign work,
revoked access and unsupported file types cannot produce a verified download.
Inspection rechecks access after preparing recovery output. A bounded derived
chunk index avoids rehashing an entire retained file for every byte page; each
returned chunk is verified and clients verify the complete digest. Transport
trimming preserves integration-step cursors, including both retained names for
a step. Downloads never restore over current files or resume integration.

Previously sealed proposal bytes also remain available after an uncertain
integration. This exception requires an existing integration request matching
the set, owner and operations digest; a header flag or an interrupted seal does
not establish sealed provenance. Original-retention labels require observed
digest evidence or a completed deletion receipt, rather than an intended backup
filename alone.

Checks recorded in this increment:

- Full Windows `internal/execution`, `cmd/offgrid-executor`, `internal/server`
  and `cmd/offgrid` suites passed (62.656s, 30.978s, 269.446s and 5.767s).
  The final transport-bounding and recovery chunk tests also passed in the
  focused Windows suite (8.032s) and Linux race suite (9.277s).
- Native cases cover complete preserved downloads, lost receipts, restart
  without replay, unconfirmed provenance, newer user edits, foreign identities,
  revocation, unsafe selectors, tampered chunks, cross-chunk pages and bounded
  transport frames. Previously interrupted seals still fail sealed-byte reads.
- Service worker integration/revocation/lost-result journeys passed (12.738s),
  including HTTP retrieval of the original dirty input. CLI/service-client suites
  passed (2.327s/0.507s); Python conversation contracts passed 6 tests (0.20s).
- The fixed production build passed all 170 affected Chat/worker/change browser
  tests (6.4 minutes), including new preserved-file download, corrupt-content and
  revoked-access cases. The earlier development-server run completed with
  164 passes and 3 failures in 35.1 minutes. All three failures first passed in
  an isolated 10-test fixed-build run (1.4 minutes), then in the full 170-test run.
  This is evidence for using a fixed fixture build; it does not establish the
  exact cause of the earlier provider error/timeouts.
- Both earlier companion-browser failures passed in isolation (2 tests,
  4.769s). The previous complete companion run remains recorded as 73 passed
  and 2 failed; it is not rewritten as a full passing run.
- TypeScript, production fixture build, generated OpenAPI check, translation
  structure, source-size, documentation and both relevant strict OpenSpec
  validations passed. The nine existing locale catalogs include the new labels;
  preview locales use explicit English fallback for the three new messages.
  Linguistic and native/model qualification are not implied.

The earlier full execution/executor run also finished successfully
(150.334s/50.068s). The follow-up server run passed (303.733s), but its subsequent
CLI build overlapped evolving source files and failed; the later complete run
above passed all four packages. The original 10-minute server timeout remains
part of the recorded test history, not a passing result.

Interactive conflict resolution/restoration, direct-write journal export,
renewed-grant recovery, retained-store cleanup, canonical domain completion,
project/context and multimodal continuity, worker questions, and remaining
resource/model/hardware/usability qualification are still incomplete. Existing
admission flags and whole-plan completion gates remain unchanged.

## Approved-folder project continuity — 2026-11-14

Native CLI and desktop supervisors can now share a private per-OS-account project
identity, initialized only after local approval. A supervisor-only operation
identifies the approved physical folder and reads the fixed
`.offgrid/project.toml` path with a 64 KiB bound. It does not admit a job or expose
the host identity in model-facing receipts. Child contexts, changed host identity,
revoked/read-denied grants, oversized files and unsafe links are rejected.

The shared `use_project_folder` command transaction creates or reuses an owned
project, imports safe configuration, increments conversation state, records the
event/receipt, and stores a private folder reference. Schema 22 adds that binding
with an upgrade backup and version fence. Identical retries return accepted
state; file changes refresh file-origin preferences, while explicit service
preferences survive later refreshes. `clear_project_folder` retains the project
and focus. Neither command rewrites already-pinned work or creates authority.

New work resolves the saved folder against live access and existing profile and
conversation restrictions. Unavailable optional access remains inspectable and
does not block unrelated text. Foreground coordination strips execution grants
and capabilities before admission. The UI restores folder selection from service
state, supports it without a profile, and keeps exact input retries separate from
later folder choices. CLI `/folder SESSION|none` and `session folder NAME
SESSION|none` use the same commands; Python uses the shared command envelope.

Verification recorded in this increment:

- Full Windows execution (20.638s), agents (45.670s), CLI (1.211s), service-client
  (0.455s), executor (28.007s) and command (2.844s) suites passed. The first full
  server run had one failure: the concurrent OCR work added a GET-only readiness
  route while the frozen route count remained 185. After reviewing the route's
  method/permission boundary and updating its fixture to 186, the targeted
  contract passed (0.485s) and the full server rerun passed (98.231s).
- Focused Linux race suites passed for execution (1.116s), agents (1.484s), and
  server (3.698s). Cases cover private bridge retrieval, identity drift, rollback,
  concurrent commands, migration/backup, restrictions, revocation, receipt retry,
  project reuse and foreground authority isolation. The first native bridge test
  exposed delayed identity detection; the immediate native check repaired it.
- Six canonical browser journeys passed against a fixed production fixture
  (6.5s); 169 existing agent/workspace browser journeys also passed (2.4 minutes).
  The six canonical cases were rerun after refining pending-choice handling and
  passed (7.4s), including preserving a new clear-folder choice while observing
  an older accepted input. The owned fixture server was stopped afterward.
- Python conversation contracts passed seven tests (0.09s). Go API tests passed
  (1.159s). The final structured CLI folder test passed (0.333s). TypeScript,
  generated OpenAPI, production build, source-size, documentation and both
  relevant OpenSpec validations passed. An initial Go setup failure from the
  temporarily missing OCR language manifest was superseded by the real-worktree
  runs above, with no stub or test-only replacement manifest.

This does not finish project-context acceptance. On-demand access approval still
needs a service-owned automatic project-binding path; project instruction-file
retrieval, complete context/source revocation, CLI retry recovery for folder
selection, and the remaining multimodal flows are open. Full qualification,
worker questions, remaining recovery/domain contracts and the other whole-plan
gates listed above remain incomplete. No whole-plan completion, release rating,
or final commit is claimed by these results.

## Automatic project binding and retained CLI choices — 2026-11-14

Canonical work now saves an approved folder's project binding as part of the
same SQLite transaction that accepts its pending workspace input. Host metadata
is read outside runner locks; the reducer rechecks the current work attempt and
conversation selection before committing. A separate context revision tracks
explicit project/folder choices without treating ordinary side questions as a
new selection. Access acceptance does not rewrite the work's pinned project,
model, instruction version or execution allowance. Permission upgrades refresh
the conversation reference only while the original selection remains current.

The focused regressions cover duplicate acceptance, restart without revival,
late Stop and correction fences, newer project/clear choices, permission
upgrades, and faults at folder/task/event persistence. Service tests exercise
real temporary-folder metadata through canonical work admission, access input,
continued execution and next-work configuration. Invalid project files and
revoked access leave both input and project binding unaccepted. Metadata
discovery checks both current conversation restrictions and pinned run ceilings;
tool attachment respects exact persisted tool restrictions and schema revisions.

Interactive CLI folder choices now retain their exact envelope after unknown
responses, including incomplete successful responses. `/retry-folder` and
repeating the same choice reuse its original request ID, expected revision and
host reference without refreshing host discovery. A different choice cannot
discard the unresolved command. An explicit revision conflict permits a fresh
choice. This retention currently lasts for the interactive process; recovery
after that process exits is still open.

Verification for automatic binding:

- Full Windows agents (61.583s), server (131.578s), API (1.697s), service-client
  (0.570s) and CLI (1.167s) suites passed. The retained CLI choice code was added
  afterward and is checked separately below.
- Focused Linux race suites passed for agents (4.899s) and server (4.774s).
  The subsequent pinned-discovery restriction regression passed on Windows
  with the affected project suite (3.735s) and Linux race suite (9.878s).
- TypeScript, generated OpenAPI, source-size, maintained documentation (379
  files), and both relevant strict OpenSpec checks passed. The contract includes
  `context_revision`; no database schema version change was needed for that
  additive snapshot field. Existing schema 22 ownership remains unchanged.
- Retained CLI choices passed targeted tests (0.695s), the final full CLI suite
  (2.807s), and focused Linux race tests (1.277s). Tests cover an unknown server
  response, incomplete receipt, expired authentication, unchanged retry payload,
  unavailable subsequent host discovery, and a fresh choice after conflict.
  The first full/race CLI attempts encountered an undefined `tx` in concurrent
  OCR edits. That source was corrected by the concurrent work before this
  increment's proposed patch applied; the final runs above used the corrected
  real worktree. No OCR stub or replacement was introduced.

The previous test compilation errors were incorrect test field/method names;
they were repaired and superseded by the passing runs above. No live user host,
installed service or model was used. This completes the automatic binding path,
not the broader project-context, multimodal, worker-question, integration-recovery
or hardware/model/usability gates. Whole-plan tasks remain unchecked where their
full contracts are still incomplete.

## Durable worker questions and recovery — 2026-11-14

Worker questions now persist their assignment/instruction versions, pending
checkpoint and stable identity. Answer commands use the canonical command and
mailbox transaction: duplicate requests retain their receipt, failed writes
roll back, and accepted/delivered/applied remain distinct. Answering retains the
same run and allowance. API, structured CLI, interactive CLI and expandable
worker controls expose recovery answers; browser retries retain the original
question identity and revision across reload.

New question admission remains disabled by default. Automatic primary-assistant
mediation and ordinary conversation answers are still unfinished. These manual
recovery controls do not satisfy the planned automatic coordination experience,
and task 11.2 remains unchecked.

A subsequent regression reproduced an explicit graph continuation rejecting a
child waiting for a question after service restart. Continuation now retains
that checkpoint without scheduling it. An answer remains necessary. Active
execution, Stop, unsettled controls, expired authority and stale assignment or
instruction versions still reject continuation.

Verification:

- Full Windows agents (146.794s), server (263.229s), CLI (3.770s), service-client
  (1.137s) and API (2.038s) suites passed before the continuation repair.
- After the repair, focused Windows question/control regressions passed
  (5.108s), as did the Linux race run (7.240s). The CLI answer-command regression
  passed (0.479s). The subsequent full Windows agents suite passed (149.840s).
- Earlier focused Linux question/HTTP race suites passed for agents (5.940s)
  and server (4.971s). Five worker-control browser journeys passed (29.3s),
  including a lost acknowledgment followed by reload and identical answer retry.
- Eight Python conversation-client tests passed (0.70s). TypeScript, generated
  OpenAPI, source-size, maintained documentation (380 files), and strict
  conversation-workflow OpenSpec checks passed before this evidence addition.

All fixtures used isolated state; no installed service, model, or user host was
used. These results do not qualify automatic worker questions, native backends,
model quality, hardware efficiency or the full architecture.

## Primary worker-question review — 2026-11-14

An admitted worker question can now wake the primary assistant through the same
durable runner. The parent consumes its existing model/input/output/time ledger;
the review cannot use the reserved verification calls, dispatch host tools or
create workers. Its restricted `worker_answer` decision either answers from the
existing context or persists a user clarification. Question source dependencies
follow the review and are checked again before its answer can be applied.

Answers preserve the existing worker/run and mailbox. The parent releases its
inference slot while waiting. The original delegation call receives one interim
receipt; the later synthesis receives the actual worker outcomes without a
duplicate reply to the original call. The answer command is pinned before
dispatch so recovery of a committed answer does not repeat it or call the model.
User clarification answers release the parent and child waits transactionally.

The regressions also exposed two control defects. Correction could retain an
obsolete question-waiting child; it now cancels that assignment while retaining
its evidence. Stopping the worker during review now cancels the parent's review
generation and returns coordination to the surviving graph. Late model output
cannot ask the stopped question again or replace a stopped outcome. Pause and
explicit continuation retain a pending user clarification without dispatch.

Schema 23 adds an older-binary rejection boundary for these review checkpoints.
The version-22 upgrade fixture preserves the pending worker question and records
the migration backup and manifest. No installed-user database was migrated by
these tests.

Verification:

- Full Windows suites passed for agents (146.697s), server (244.466s), CLI
  (3.750s), service-client (1.084s) and API (2.894s). This run began before the
  final worker-Stop and clarification-pause refinements; their focused checks
  and subsequent agents results are recorded separately.
- The expanded question/control/delegation suite passed (14.935s), including
  primary answer/synthesis, atomic escalation answers, lost parent receipt,
  Stop, correction, worker-only Stop, revocation, reserved verification capacity
  and schema migration. Linux race coverage of those cases passed (19.745s).
  A subsequent full Windows agents run passed (147.206s). After the final
  clarification pause/continue repair, all question regressions passed (9.363s).
  Focused Linux race checks of clarification continuation and restart passed
  (3.687s).
- Six browser worker-control journeys passed against the isolated production
  fixture build (26.4s), including the new primary clarification journey and
  reload after an unknown answer response. The fixture HTTP process was stopped.
- TypeScript, generated OpenAPI, source-size, maintained documentation (381
  files) and strict conversation-workflow OpenSpec validation passed. The web
  production build passed with its bundle-size warning.

Ordinary question creation remains disabled: normal-composer answer routing and
model qualification are still required. The worker panel and CLI recovery
commands are not the finished conversation-first experience. Task 11.2 and the
broader continuity, execution-integration and qualification gates remain open.

## Conversation answers and retained worker results — 2026-11-14

The ordinary composer and interactive CLI now submit a displayed primary
clarification through the canonical conversation inbox. The exact question ID,
request ID and focused work are retained through unknown-response retries and
reloads; a later question cannot silently receive an earlier answer. The service
commits conversation history, worker mailbox and continuation state together.
Answers require neither a new model turn nor an available model, and remain
usable for existing work when new conversation admission is disabled. The
noninteractive CLI supports `--answer QUESTION_ID` with structured output.
Question-derived prose is withheld after source revocation while user-authored
answer text remains in conversation history.

Review found that the primary assistant could lose the actual user decision
after forwarding it to the worker. Accepted answers now persist with the parent
and enter its context after the pending tool receipt. Success, partial failure,
transaction rollback, retry and restart tests exercise both the conversation
command and direct worker-answer route. This does not confer host authority.

The Go work client also decoded the wrong response shape: it expected a bare
work item while the service returns a `{work, run}` envelope. It now validates
the real envelope and matching identities before issuing a work control command.
The HTTP regression exercises that client against the actual handler.

Retained-result retrieval had a separate reproduced failure: after an explicit
worker follow-up created a new root attempt, `task_history` rejected the prior
result as outside the new graph. Retrieval now checks persisted worker/run
membership within the same actor, workspace and work, or an exact pinned
dependency. The primary and continued worker can retrieve prior full results;
older assignment results retain stale-revision metadata. Cross-work,
cross-workspace and revoked-source requests are rejected. Bounded paging retains
the final citation of a large result.

## Partial-result synthesis and inherited source access — 2026-11-14

Previously every failed branch immediately ended coordination with a
deterministic status dump. Canonical work with useful independent completed
branches can now synthesize those results within its existing allowance. The
deterministic outcome is persisted first. The synthesis stage can retrieve saved
evidence but cannot dispatch new work or host operations. Recorded failures and
result references remain visible alongside model prose, and the overall outcome
remains partial (`delegation_partial`, failed status), not completed.

Unavailable inference, provider failure and exhausted budget retain the saved
summary. Stale dependencies do not enter automatic synthesis. Stop rejects late
output; restart retains the partial summary and requires explicit continuation.
Ending or correcting this stage closes its temporary instructions so subsequent
authorized work is not trapped in a synthesis-only context.

Both successful and partial handoffs now retain child source dependencies on
the parent. Without this propagation, parent context and output access could
miss a source used only by a worker. Revocation now blocks synthesis before
inference and remains enforced after restart. This is conservative rejection;
it does not yet establish selective reconstruction of the remaining authorized
context required by the complete architecture.

Verification recorded during this pass:

- The conversation-answer pass completed 14 browser journeys (41.0s), nine
  Python conversation-client tests (0.52s), TypeScript, generated API, web
  production build, documentation and strict OpenSpec checks. The browser build
  used an isolated fixture server, which was stopped afterward.
- Windows answer/question and independent-result regressions passed for agents
  (12.042s) and server (1.675s). The first expanded Linux race run passed for
  agents (22.607s) and server (2.714s).
- A full Windows run passed agents (102.066s), server (191.750s), CLI (4.881s),
  service-client (2.194s) and API (1.944s). This run preceded the final retained
  result retrieval and synthesis-ending refinements.
- The retained-result regression first failed with "result is outside this
  task's settled dependencies". After the repair, retained results, worker
  runtime, dependency invalidation, partial synthesis and source revocation
  regressions passed together (6.179s).
- Final focused Windows coverage of answers, partial synthesis, retained full
  results and inherited source revocation passed (8.370s). It includes an actual
  rejected `delegate_tasks` proposal during synthesis and correction arriving
  during generation. The correction fixture releases generation at the next
  checkpoint; correction acceptance does not promise to cancel that generation.
- The final Linux race run passed agents (22.868s) and server (2.858s), including
  the added dispatch, correction, restart and retrieval cases. TypeScript,
  generated API and strict OpenSpec validation passed; source-size and maintained
  documentation checks passed (382 Markdown files).
- After all implementation refinements, the full Windows agents suite passed
  (152.714s). The final test log is `offgrid-worker-continuity-final.jsonl` in the
  task user's temporary directory; it is not an installed-user state store.

One verification attempt caught the concurrent document-ingestion edit between
its new `validAttachmentHistory` call and helper definition. After the helper
appeared, compilation succeeded. No document-ingestion files were changed to
work around that transient failure.

All checks use isolated state and fake model responses. No installed service,
user host, model download or real model/hardware qualification was used. Normal
worker-question creation remains gated pending qualification. Unified modality
admission, selective context rebuilding after revocation, remaining execution
and background-lifetime contracts, resource qualification and representative
usability evidence remain outside these completed checks. The full architecture
and tasks 11.2/11.3 are not marked complete by this evidence.
