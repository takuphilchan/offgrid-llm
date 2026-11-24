# Client correctness contracts (development)

These changes are in the working development source, not a claim about the
published v0.4.3 artifacts. See [production readiness](production-readiness.md)
for remaining work.

## CLI service requests

Knowledge commands (`offgrid kb ...`), saved-conversation commands (`session`,
`export-session`), and agent lifecycle controls (`status`, `approve`, `deny`,
`cancel`, `resume`, `reconcile`) now use the shared authenticated service client.
They do not fall back to editing service files. Other CLI commands, including
interactive chat saving, are still being migrated and do not yet all implement
this contract.

- `OFFGRID_SERVER_URL` is an HTTP(S) origin, e.g. `http://127.0.0.1:11611`.
  Do not include `/v1`, user credentials, query parameters or fragments.
- `OFFGRID_API_KEY` supplies the service Bearer credential. Never pass a real key
  in a URL or paste it into an issue. Remote use requires trusted TLS.
- Redirects are rejected; model registries/download hosts do not inherit the key.
- Requests have deadlines and support Ctrl+C, including while reading a response.
- Exit codes: `0` command succeeded, `1` operational failure, `2` invalid usage,
  `130` user cancellation. Reading a failed task's status is a successful read;
  the snapshot's `status` tells you whether the task itself succeeded.
- `--json` writes one JSON result or `{ "error": { "code", "message", "retryable",
  "request_id"? } }` to stdout. Progress/interactive prompts go to stderr. Do not
  infer success from empty output or ignore the exit code.
- `kb clear` requires explicit `--yes` when noninteractive or using JSON. Partial
  deletion/import errors report how much work completed. Cancellation/timeouts
  are not a guarantee that a server-side mutation was undone: inspect state first.

Examples (use your installed binary path if `offgrid` is not on PATH):

```bash
offgrid kb status --json
offgrid kb list --json
offgrid kb search "Where is the emergency procedure?" --json
offgrid kb add "./documents/meeting notes.md"
offgrid agent status RUN_ID --json
offgrid session list --json
offgrid session show "Meeting notes" --json
offgrid export-session "Meeting notes" --format markdown
```

Conversation exports default to stdout; an explicit output file must not already
exist. Protected or unavailable services never fall back to local conversation
files. These commands still use the existing v1 session names; generated IDs and
duplicate-title support depend on the pending staged migration. `offgrid export`
is the separate model/USB export command, not a conversation export alias.

Local file upload is bounded to 50 MiB and streamed rather than buffered wholesale.
Directory imports skip symlinks and unsupported extensions and stop on failure.
This does not expand supported extraction formats or add OCR.

### Profile and conversation SDK contracts (development)

Python exposes `client.agent.profiles` for owned profile list/get/effective/
resources/create/update/archive. Updates and archival require the expected
revision. Profiles reference installed models and authorized tools; these calls
never download models or change connection credentials.

`client.sessions.submit_agent_turn(name, content, request_id=..., profile_id=...,
profile_revision=...)` returns the service's `turn` and canonical `job`. Use
`client.sessions.turn(name)` or `client.agent.job(id)` to observe work.
`recover_agent_turn(name, turn_id)` repairs bookkeeping or attaches a saved
answer without executing again. An interrupted job still needs explicit resume;
uncertain effects still need inspection. These methods do not use the legacy
SDK's client-side `chat_with_session` history-save sequence.

`client.agent.steer(id, instruction, request_id=...)` distinguishes accepted
`pending_instructions` from applied `instructions`. It does not cancel an action
already sent. `stop(id)` requests cancellation; the returned status is the
authority, not an assumption of rollback. Conversation deletion accepts
`work="retain"` or `work="stop"` and propagates a pending-stop conflict.

New mutation methods disable transport retries. After a lost acknowledgment,
inspect saved state, then explicitly retry the identical ID/payload if needed.
Never change the request ID to recover a missing result. Errors from unsupported
servers propagate; there is no fallback executor or silently selected model.
Generated web contracts and client methods use these same service endpoints.
The current React Agents page manages profiles; Chat submits profile-enabled
turns and observes the canonical job. Old job links open a secondary inspector,
not a second execution path. Navigation, reload and final-result attachment do
not approve, resume or replay work. Shared-shell computer controls remain present
across navigation, and an unacknowledged Stop remains visibly unconfirmed.

The service-backed CLI exposes `agent profiles`, pinned `agent run --profile`,
and `session send/chat/turn/recover`. Interactive profile conversations queue
corrections on their active job; they do not start a second job while awaiting
access, approval or reconciliation. `/exit` detaches without stopping work.
Both CLI and Python check the advertised capability before profile-enabled
submission so older services cannot silently ignore agent fields and run plain
Chat. Cross-client installed-package qualification remains separate from the
HTTP, terminal and isolated browser contract tests.

The opt-in cross-client regression uses actual CLI, Python SDK and headless
Chromium against disposable authenticated handlers and fixture inference:
`OFFGRID_CROSS_CLIENT_TESTS=1 go test ./internal/server -run
TestAgentCrossClientIntegration -count=1 -v` (Go, Python and Playwright Chromium
must already be installed). It checks one canonical identity, idempotent steering,
cross-actor denial and cancellation across clients; it does not use port 11611 or
qualify an installed model or native desktop.

## Backend identity and desktop attachment

`GET /api/v2/system` is a public, uncached identity document. It contains product,
version, source revision (or `unknown`), API version, UI build ID and capability
names. It exposes no local paths, credentials, account data or model inventory.
Only this exact GET is public; it does not bypass authentication on the v2 prefix.

The initial capabilities describe existing v1 session/stream/agent contracts.
An API version of 2 here does **not** mean every v2 operation is available.
`durable-agent-events-v2` specifically advertises owner-scoped job snapshots and
ordered activity replay; it does not advertise v2 submissions, native control or
vision. See [live agent progress](agent-live-progress.md).

Desktop attachment distinguishes **external service compatibility** from
**bundled package consistency**. External services can have different release
versions and renderer builds when their API capabilities and renderer/desktop
bridge protocol are supported. An owned bundled child must additionally match
the exact desktop package version and renderer fingerprint.

The additive `desktop_bridge` identity field has `status: missing | invalid |
ready`. Ready metadata includes `schema_version: 1`, `protocol`, and `ui_build_id`.
The service validates a bounded `desktop-compatibility.json` from its actual UI
root against that root's index. Invalid/unsupported metadata cannot fall back to
legacy handling. The UI build ID is SHA-256 of `index.html` with CRLF normalized
to LF (it references content-hashed assets): a compatibility identifier, **not** a
signature or complete artifact-integrity proof. Protocol compatibility never
grants authentication, computer access, or local consent.

Protocol 1 describes the existing bounded `window.electron` main/preload API.
Breaking argument, response, or permission semantics require a new protocol;
supporting an older protocol requires retained tests. The renderer build emits
its declaration, and packaging verifies the copied UI. Keep declarations and
tests aligned when changing desktop IPC; do not derive protocols from semver.

Legacy metadata absence is accepted only by `apps/desktop/legacy-compatibility.json`:
service 0.4.14 or 0.4.15 with renderer fingerprint
`a7336324e4ea09e8a9f5e25aabf2a53c2f9cb39b81eb990a081ba9a84e66f617`.
This is the reviewed workspace-refresh renderer, not every 0.4.14 build. Its
main/preload/computer-runtime contracts are unchanged between those release tags.
The mapping survives a newer desktop renderer build; unknown legacy tuples
remain blocked. Old installed desktops retain their old strict policy until
updated. Explicit invalid metadata is never treated as legacy absence.

An incompatible or nonresponsive
occupied port is not replaced. An externally managed backend is never stopped
by Electron, and its paths are not misrepresented as desktop-local paths.

Settings shows the active service address, versions, API, UI build ID, attachment
basis, and ownership. The service supplies its own UI; updating desktop alone
does not update an external service's features. Build
the UI and runtime from the same checkout before testing desktop development;
restart/redeploy an old service explicitly rather than expecting new source to
change a running process or container.

## Native document feature negotiation (development)

Document support is an explicit `native_documents:1` contract, not inferred from
desktop protocol 3, a release number or whole-value replacement support. Pack
metadata declares the understood contract revision. Worker startup reports only
contracts implemented by its provider; the parent rejects a revision absent from
the verified pack, unknown feature keys, and malformed startup fields. A legacy
pack/startup has no document feature and remains limited to its existing tools.

The service advertises `native-document-negotiation-v1` only to indicate that it
understands this handshake. Companions send optional `native_features` during
pairing only when both service and worker understand it, and validate the echoed
selection. Missing support at either end yields no negotiated document tools.
Unknown revisions are refused, not silently downgraded. Features live in the
actor-owned session and do not extend its selected-window or desktop grant.

Negotiation is not execution readiness: the provider must additionally expose
the operation for the current, fully bound document. New task-first jobs use
workflow-evidence v2; document schemas appear only for a negotiated actor-owned
host and the current observation/read stage. Execution checks this again before
queuing input. Legacy adapters still advertise no document feature. Service and
synthetic-worker integration tests do not qualify UIA, AX or AT-SPI editing.

## Incomplete agent output

The durable and structured agent runners validate the model's completion reason
before using its answer or tool calls. Token-limit, filtered, missing and unknown
reasons fail the task; even syntactically valid tool arguments cannot bypass this
check. Durable history keeps any partial text labelled `incomplete`, outside
completed model context. Prior committed tool actions remain recorded; failing
a later model turn does not undo them. Models must return a supported terminal
reason, not silently omit it.

This prevents false success; it does not verify that an otherwise complete answer
is correct. Artifact checks and representative agent evaluations remain pending.

## SQLite connection policy

The runtime pins `modernc.org/sqlite v1.48.2` / SQLite 3.51.3, which includes the
[upstream WAL-reset fix](https://www.sqlite.org/wal.html#walresetbug).
The shared opener applies `foreign_keys=ON`, `synchronous=FULL` and a bounded busy
timeout on every pooled connection, verifies WAL mode and rejects known unpatched
versions. Paths are URI-escaped independently from settings; UNC paths are rejected.
Use a local filesystem—mapped network drives are not automatically detected.

This hardens the existing knowledge database; it is **not** the new transactional
workspace migration. Exclusive workspace ownership and offline backup/restore
are implemented separately; see [workspace recovery](workspace-recovery.md).

## Inference admission and event delivery

### Conversation control preview

The additive conversation resources require `enable_conversation_preview` for
new conversations, imports and work. Existing records, controls and identical
command retries remain available when admission is disabled. Clients submit
`request_id`, `target_id`, `expected_revision`, `kind` and a typed `payload`.
Keep an identical envelope when retrying an uncertain response; a changed
payload under the same identity conflicts. GET and reconnect never dispatch.

`GET /api/v2/conversations/{id}/inputs` pages accepted input in committed order
using `after` and `limit` (1–200). `GET /api/v2/conversations/{id}/inputs/{input_id}`
returns its current delivery and incorporation state. An unavailable model or
context retains the input with a `delivery_issue` and service retry time.
Observation does not reset that retry time, release work holds, or resume runs.
The service delivers through the existing durable runner; interrupted delivered
turns retain their original identity after restart. A `decision` is a saved
proposal; only `applied_at` and `applied_instruction_version` confirm its
incorporation. Ordinary replies need no classification call. Foreground
corrections use the exact accepted user text and wait for a settled checkpoint;
`decision_issue` explains waiting, stale instructions or changed access.
Settlement uses no inference and cannot override Pause, Stop or restart recovery.
Existing native tool qualifications do not automatically enable the new
coordination tool. A `new_work` decision admits the exact accepted request using
fresh service configuration, with its proposed expected result as an acceptance
condition. `created_work_id` identifies the resulting work; `work_id` retains
the previous focus. Work, run, link, event and input receipt commit together.
Retries return that identity without replenishing the previous work's budget,
copying its host grants, or duplicating the user message. A required host-access
request remains pending and prevents dispatch. CLI/UI default-flow integration
remains under implementation; preview APIs do not yet replace ordinary chat.

`POST /api/v2/conversations/{id}/inputs` accepts the shared command envelope with
`kind: submit_input`, the conversation as `target_id`, and text in
`payload.content`. Acceptance does not require model availability. Omitted focus
uses the current conversation focus; an explicit empty `focused_work_id` is
unbound, and `intent: new_work` is unbound automatically. Typed and dictation
input use the same semantics. New admissions require the preview flag; identical
retries and existing controls remain available when the flag is disabled.
Explicit pending-question answers are rejected with a compatibility explanation;
this route never approves a host request from answer text.

`POST /api/v2/conversations/{id}/inputs/{input_id}/commands` uses the input as
`target_id` and the containing conversation's `expected_revision`. `stop`
withdraws unapplied input and cancels active foreground generation. It remains
`stopping` until operations settle and its own work hold is released, then becomes
`stopped`; it does not stop unrelated work. An already-applied decision retains
its outcome. `continue` resumes only the pinned interrupted foreground attempt
with its remaining allowance. `revalidate` requires the current work
`instruction_version` and explicitly revalidates the saved exact correction;
the original model decision is retained alongside revalidation provenance.
Reads and retries never implicitly continue execution. Derived decision text is
withheld from reads, event replay and receipts when source access changes, with
`context_unavailable: true`; the original user input remains available.

`POST /api/v2/conversations` accepts `import_conversation` with an empty target,
revision zero and `payload.legacy_name`. The owned legacy session must be
inactive and its sources accessible. Import commits canonical history,
ownership and the receipt together, retaining the original source bytes and
digest. Legacy writes then return a compatibility error; legacy reads project
canonical results and inbox exchanges in timestamp order. An input that creates
work or becomes an applied correction appears once; later work results retain
their completion time. Queued and withdrawn input remains visible without
starting a run. Source-dependent inbox replies are withheld after revocation,
while original user text and control receipts remain inspectable. A concurrent
input change can return a refresh conflict instead of a mixed projection.
Legacy turn recovery and cancellation return the same compatibility error;
use the canonical input or work controls. Missing sources return 404, active turns return 409 and
revoked Knowledge access returns 403. An identical accepted retry returns its
receipt even if the legacy reader is subsequently unavailable.

Existing conversations accept `select_project` with `payload.project_id`,
`link_work` with `payload.work_id`, and `focus_work` with an already linked work
identity. Omit the work identity to clear focus. Linking rechecks ownership and
source access and retains the original work/run identity, allocation and receipts.
Project selection clears focus and retains links and conversation restrictions;
accepted work continues under its pinned configuration. These commands never
dispatch, copy grants or reset budgets. Historical context accepts cross-conversation
work only through a stored link and checks current source access on retrieval.

`GET /api/v2/user-preferences` returns the current actor's model/delegation
defaults, with revision zero when none exist. It creates no preference record.
`POST /api/v2/user-preferences/commands` accepts `configure_user_preferences`,
target `user-defaults`, and a payload containing optional `model` and
`delegation`. The payload replaces those preferences; an empty payload restores
inheritance. It cannot carry grants, credentials, restrictions or memory consent.
Request, conversation, profile and project preferences take precedence. Effective
configuration exposes provenance; accepted runs retain the resolved values and
revision. A later preference update applies to future admissions.

Canonical worker resources expose session identity, ordered mailbox inspection
and `message`, `continue`, `stop` and `close` commands. A follow-up creates a new
run under the existing worker and consumes the root work allowance. The model
cannot revive a user-stopped worker. Messages distinguish accepted, delivered,
applied and abandoned states. Unapplied messages abandoned by Stop or assignment
completion retain their original run, contents, timestamp and reason; they are
not silently replayed in a follow-up. Pause keeps the inbox pending. An explicit
user continuation after completed work opens a new parent attempt under the same
work identity. `run_ids` retains earlier attempts while `run_id` identifies the
current one. Prior results remain in conversation history; spent and reserved
model allowances carry forward. Automatic worker calls cannot reopen completed
work. A stopped work item, unresolved effects, stale instructions or exhausted
budget prevent this admission. Existing retained workers can then be reused
within the current attempt.

The attempt transition, worker run, receipt, events and outbox commit together.
Schema 13 fences older runners and preserves a verified upgrade backup. A new
attempt cannot inherit a previous run's desktop session or execution binding;
new execution uses the ordinary authorized access path. Restart interrupts
pending work and requires explicit continuation; observation never restarts it.

Canonical workers return unused model-call allocations after their run is
terminal and generation/process ownership is quiescent. The parent persists a
per-run settlement receipt with the reduced reservation; retries cannot refund
twice. Retained slots cover attempted calls and conservatively charged input,
output and time. Interrupted or uncertain assignments retain their full allocation.
Recovery honors committed receipts and never invents a refund. Follow-up work
attempts inherit prior charges; legacy graphs keep their existing accounting.

Input tokens reserve the pinned context ceiling before model/context dispatch.
Measured prompt usage refunds only that call's observed unused input capacity;
missing usage, probes and interrupted attempts keep conservative charges. Older
runs acquire conservative historical charges on explicit continuation. Oversized
or inconsistent usage cannot reach action dispatch. Context-preparation count and
elapsed time are recorded separately within the existing inference allowance.
They are not a second time charge.

Tool invocations reserve their pinned timeout before dispatch. Successful,
uncancelled calls settle observed elapsed time once; failed or cancelled calls
retain their conservative reservation, and measured overruns remain charged.
Settlement matches the durable invocation sequence, so late Stop receipts can
record usage without reviving work. Failed receipt writes retain the original
reservation across restart. Legacy runs conservatively reserve historical call
capacity on explicit continuation, labelled separately from observed calls.
Child allocations and subsequent work attempts retain these charges. This is
invocation elapsed time. Workspace storage requires separate accounting.

Owned processes reserve their requested timeout in the same transaction as the
authenticated execution intent, before host dispatch. The process allowance is
pinned separately: each model-call slot supplies up to 16 starts and
`16 * max(step timeout, 180 seconds)` of supervised runtime. The existing
one-hour maximum for a single command still applies; a longer command can consume
several slots' runtime capacity. Child allocations and later work attempts share
the same remaining root capacity.

Hosts report monotonic supervised elapsed time, including startup and termination
handling, separately from tool invocation duration. This is not CPU time, and the
two elapsed measurements overlap. Running observations retain the reservation;
authenticated terminal timing settles unused capacity once, including failed or
cancelled commands. Measured overruns remain charged. Known pre-start rejection
releases reserved time while retaining the attempted start count. Lost replies,
missing legacy timing and unavailable hosts retain conservative charges. Stop
does not wait for inference to update accounting or authorize new dispatch.
Process usage projection excludes operation IDs, handles and grant references.

The host library and private stdio/enrolled-host transports support worker-copy
preparation and explicit canonical binding. Conversation-preview editing/test
assignments use this path with an explicitly configured private store and a live
admitted source binding. Each prepared worker
gets a fresh narrowed session/grant, its own cwd, process ownership and journal.
Parent cancellation/revocation closes dependent sessions; workers cannot enroll
unrelated runs or recursively prepare more workers. Identical live retries return
the same prepared worker. Restarted artifacts never recreate a live grant.

Direct supervised `file_write` replacements now use the same displaced-content
preservation primitive as integration. The private execution journal reserves
retention space and persists dispatch intent before replacement. It retains a
separate proposed version and the actual displaced file; conflict or uncertain
receipts never cause rollback or replay. A failed final receipt revokes further
session dispatch. Successful receipts include an opaque `replacement_id` and
`original_retained`; unknown receipt inspection can return recovery metadata
without promoting the operation to success. The journal is version 3 and worker
copy storage is version 10; incompatible older readers reject these versions.

Prepared workers charge recovery bytes against their existing copy allocation,
and later measurements include their private recovery artifacts. Standalone
journals retain a conservative maximum of 128 replacements and a 2 GiB logical
reservation allowance. Unsettled records survive restart; automatic cleanup is
not enabled. Existing-file replacement requires a supported same-volume backend
and private journal. Unsupported replacements retain the proposal but do not
mutate the target. Export/review of these direct-write recovery artifacts remains
unfinished; worker change-set downloads use their separate public API.

Preparation is an asynchronous supervisor command. Acceptance is journaled before
copying begins; begin, inspect and cancel do not wait for a directory copy. Exact
retries observe the same preparation, changed payloads conflict, and inspection
cannot dispatch. Cancellation reports `cancelling` until preparation settles;
parent Stop cannot acknowledge quiescence while preparation is unsettled. Failed
completion receipts close any new child grant and retain an uncertain outcome.
Durable results contain artifacts rather than live child contexts. After restart,
a formerly ready preparation reports `interrupted`, without recreating authority.

The native grant must explicitly include a nonzero working-copy policy. Existing
host setup leaves it disabled; ordinary model calls cannot invoke supervisor
preparation. The enrolled bridge accepts only children attested by that native
endpoint for the exact parent and worker run. Child calls, receipts and Stop use
the child's narrowed binding. Retained status is bounded to 128 preparations per
supervisor lifetime, in addition to the storage policy's active-copy bound.

Preparation inventories all ordinary working-tree files, including dirty,
untracked and ignored content, without running Git or modifying its index. It
records explicit Git-administration exclusions and rejects unsupported links,
special files or missing required inputs. A Git command that requires excluded
metadata is not supported by this ordinary-copy backend. Source digests are
rechecked during copying and against a fresh inventory; this does not claim an
atomic snapshot against external editors. Copies use independent files and
private permissions, with journals outside the worker-visible root.

Version-one host policy defaults to 512 MiB per copy, 2 GiB of aggregate reserved
logical file bytes, 10,000 entries per copy and 32 retained active preparations.
Capacity commits before copied bytes are written. Failed preparation or release
retains its charge and artifacts. New admissions refresh observed growth of prior
copies, including after restart or a failed measurement write. Incomplete growth
measurement blocks additional admission conservatively. These are managed-copy
admission limits, not physical-disk allocation measurements or an OS quota for
arbitrary trusted native programs. Closing a worker retains its files for review;
explicit release removes only its generated directory after process shutdown.
The service now reserves capacity in the root work before native preparation.
Schema 14 preserves these reservations across restart and subsequent work attempts;
older binaries reject the unsupported schema. A failed or unknown preparation
keeps its charge. Observed growth increases it, and only the matching host's
confirmed artifact-release record refunds it. Root policy initially pins 2 GiB,
independently of the host's aggregate policy. A model checkpoint cannot overwrite
the controller's storage accounting. `resource_usage.storage` exposes charged and
last-measured logical bytes and retained/unmeasured copy counts, without bindings.

Explicit service admission records the narrowed environment on its canonical
worker only after persisting the host storage receipt. The final binding checks
the current assignment, correction epoch and Stop state. Failed binding cancels
the newly registered child. Observing preparation alone cannot register it.
Child resources are excluded from general host selection, cannot admit other runs,
and share transport without sharing ownership: child Stop/revoke leaves siblings
and the parent usable. Parent Stop fences new child dispatch even before the first
native operation. Continuing a worker never copies its previous run's grant.

Automatic editing/testing assignment, environment templates, transport-managed
artifact release and change-set integration remain pending; this controller path
does not enable them or widen the model's currently qualified tool set.

Owner-scoped job responses include `resource_usage`; the CLI/Python job methods
retain it, and the web context inspection exposes it on demand. Measured usage
and charged capacity are distinct. Historical input/tool accounting may be absent;
absence does not mean zero. Root attempts inherit cumulative root charges, so
clients must not sum those counters across attempt history. Child allocations
include unsettled reservations and retained spent slots; exact child measurements
remain on the corresponding child runs.

Expandable worker activity groups runs by stable worker identity and exposes
mailbox follow-ups, explicit continuation and independent Stop. Opening it or
reloading only observes state. Drafts and unknown-outcome command envelopes are
actor/workspace scoped; explicit retry uses the original request and revision,
including when a continuation already moved the worker into its next run.
Activity details retain run history and precise mailbox delivery states. The
ordinary CLI chat has corresponding worker controls; `agent worker command`
accepts the same typed JSON envelope for noninteractive callers.

The CLI and Chat UI use canonical input for eligible conversations when the
service advertises `conversation-input-admission-v1`. New service configuration
enables this path by default; an explicit `enable_conversation_preview: false`
or `OFFGRID_ENABLE_CONVERSATION_PREVIEW=false` disables new admission. Observation,
controls and exact retries of already accepted inputs remain available. Updated
clients do not fall back to legacy generation when the service advertises input
format support but disables admission. Explicit legacy API routes retain their
existing contract. Source-capable Chat uses the shared canonical source path;
named CLI conversations with legacy source history retain their existing flow
until their interactive source migration is supported.

`conversation-input-options-v1` advertises `response_profile` (`interactive` or
`extended`) and `max_tokens` (0–4096). The accepted input pins these settings before
model qualification: interactive uses the configured chat context, extended uses
the model's available context, and output is capped by one quarter of that window.
Zero or omission preserves the existing canonical output limit; omitted profile
preserves the existing canonical context allocation. Explicit retry must retain
these fields with the original envelope. This format capability remains advertised
after admission is disabled. Older services keep the previous text contract; the
CLI rejects explicitly requested options if support is absent, while older-service
Chat requests retain their previous canonical defaults.

With both input-options and source-input capabilities, ordinary image/file input
may have an empty caption. Ownership, revisions, image capability and source access
are still checked. Its saved caption remains empty and it cannot create an inferred
work objective. Explicit new work, corrections and clarification answers still
require text. Unsupported source combinations retain the draft.

The UI returns the composer after durable acceptance and observes inbox activity
independently of focused work. Reload does not submit, approve or continue work.

New foreground turns can use `task_find_tools` to search a separately pinned,
currently authorized metadata inventory. Search returns at most eight capability
descriptions per page, marked as requiring work admission. It does not expose
callable provider schemas, contact hosts, grant approval or execute tools. Current
actor, profile and conversation restrictions and provider identities are checked
before ranking. Previously saved metadata remains historical evidence. Native tool
qualification and foreground execution restrictions still apply; historical runs
without the new metadata pin keep their original behavior.

Stop targets the chosen input; work controls remain on the work indicator. The
full request envelope is retained for explicit retry after unknown transport
outcomes. Optional specialization pins its revision, and submitting from a stale
view cannot silently widen restrictions saved by another client. The CLI offers
the same input path through `session chat` / `session send`, plus explicit work,
input and preference controls. Both clients retain service-owned execution.

Imported history, work results, foreground replies and correction receipts now
share a chronological model-context builder. Input-created work prompts and
applied corrections are included once by identity. Later queued input is excluded
from earlier turns, revoked derived content is withheld, and remaining authorized
context reaches the existing model-context accountant without a fixed cutoff.

Saved worker changes are available through owner-scoped
`GET /api/v2/work-items/{id}/changes` and `GET .../changes/{change_id}`. Clients
page the metadata list and select `files`, `validation`, `integration`, or
`content` details. File reads require the original live source authority and do
not renew it. Public projections omit host grants, private working-copy paths,
process commands and displaced-file locations. Disabling preview admission does
not disable these reads for existing work. List metadata retains unavailable
status after revocation; detail reads fail. The web panel clears cached details
on access failure and remounts on actor/work changes. Downloads and CLI content
streams verify the complete sealed SHA-256 across pages.

Settled canonical outputs now have immutable control-store records. Retrieve
them with `GET /api/v2/results/{reference}?offset=0&limit=4096`; follow
`next_offset` until `complete`. Each page retains the run, instruction version,
control epoch, recorded status and separate verification flag. Current actor and
source access are checked on every page, including dependencies captured when
the result was sealed. A later run or context edit cannot overwrite old output.
The worker activity panel retrieves pages on demand. CLI `session result` emits
complete text or JSON Lines; Go, TypeScript and Python expose the same read.

Change-set identity, operation digest and lifecycle revisions commit with the
root run in the control database. Exact files, validation and integration
receipts remain in the host journal. Control events never contain host grants.
Schema 25 transactionally backfills recorded results and changes during upgrade;
it does not restore authority or dispatch execution. Older binaries reject the
new schema rather than independently editing canonical state.

Conversation preview admissions now pin durable worker clarification support
for generic tool runtimes. Native adapters retain their separate qualification
gate. Worker policies pin session/concurrency limits and initial allocations;
`worker_request_budget` moves only unallocated root capacity and retains parent
verification capacity. Duplicate requests settle once, including after restart.
Workers see their remaining allowance and must preserve partial findings when
additional capacity is denied.

The read-only conversation Options panel shows the pinned project, preference
provenance, restrictions and resource explanations. An unrelated foreground
question can proceed with revoked work context withheld; correction and answer
commands still require access to the work they affect. Typed input and dictation
use the same command ordering, and explicit message/new-work modes do not
silently answer a pending clarification.

Unified multimodal admission, interactive conflict resolution, environment
retention/templates, measured model selection and full backend/model/usability
qualification remain incomplete.

### Existing inference gate

For live agent phases, response previews, reconnect behavior and CLI presentation,
see [live agent progress](agent-live-progress.md).

The shared scheduler defaults to one admission slot and at most 32 queued
requests. Under contention its `conversation-fairness-v1` policy admits three
foreground requests followed by one background request, rotating actors and work
items within each class. Foreground conversation turns and direct chat requests
receive priority; work runs and their workers share their work identity in the
background class. Model residency cannot bypass this order. Queue admission
for unlabelled internal maintenance/indexing defaults to the background class.
Requests carry no user-controlled priority or authority grant. Admission
precedes provider-family and runtime resources; existing memory protections and
exclusive native/llama transitions still apply. Active generation is not killed
to improve priority. Shared model preparation retains its initiating scheduling
scope while its lifetime remains service-owned.

A full queue returns HTTP 429 with `inference_queue_full` and `Retry-After: 2`.
Health's `scheduling` field reports admitted slots and both queue classes without
exposing actor/work identities. The separate `inference` lifecycle fields retain
their runtime meaning; an admitted request may still be awaiting family/runtime
resources. Cancellation and shutdown remove queued demand and release admission.
Measured automatic model selection, coordinated indexing priority and qualified
hardware performance evidence remain pending.

Agent events are persisted before publication. A slow subscriber is disconnected
instead of blocking execution and must replay from its last persisted event ID.
Invalid persisted sequences and uncertain write failures fail closed. This is
hardening of the existing run log. Agent job replay now separately uses the same
SQLite transaction as the task snapshot, with bounded metadata compaction and
explicit snapshot recovery. The old run log is not its authoritative replay store.


## Canonical source input (local preview, 2026-11-15)

A service advertising `conversation-source-inputs-v1` accepts attachments,
images and Knowledge selection on the existing canonical inbox. Submit one
`ConversationInputCommand`; keep its request identity when an acknowledgment
is lost. `source_session` must map to the same canonical import. Every attachment
contains its immutable `id` and `revision`; optional `page` is the original
1-based page index, with zero meaning normal document retrieval. An unavailable
page or revision is rejected rather than broadening the source scope.

`knowledge_documents` narrows current authorized sources. `use_knowledge`
requests current permitted retrieval. Neither field expands profile permissions.
Images use the existing bounded SessionImage contract and installed vision
capability; attaching a PDF does not interpret its figures automatically.
Answers to pending questions and corrections cannot change sources.

Immutable attachment upload remains available after canonical migration, while
legacy transcript writes and generation remain blocked. Preparation and delivery
recheck source access. Evidence selections persist with the run and project into
compatibility reads. Request admission is not proof that inference succeeded.

CLI: `offgrid session send NAME --attachment ID --page 8 --request-id ID QUESTION`.
Use `--file PATH` to upload, `--image PATH` for an image, `--rag` for permitted
Knowledge, or repeated `--knowledge-document ID` for explicit selection. Page
scope requires exactly one file and a canonical conversation. Older services
retain text-only admission and reject source operations with a capability error.
Python's `conversations.submit_input` sends the same payload without retrying
writes; generated TypeScript uses the additive OpenAPI schema.

Services advertising `conversation-source-scopes-v1` additionally accept
`knowledge_selections`: up to 16 entries with `document_id`, immutable extraction
`revision`, and either a positive original `page` or 1–32 unique `block_ids`.
These exact selections cannot be combined with `knowledge_documents` or
`use_knowledge`. They include linked headers and exceptions, use retained text
without requiring embeddings, and fail with `knowledge_scope_unavailable` when
the selected revision, linked evidence or bounded context cannot be supplied.
They never fall back to a broader search. Collection/profile access is checked
at admission, preparation and delivery; historical revisions retain their source
identities and remain subject to current access. CLI accepts repeatable
`--knowledge-selection` JSON; Python uses the identical `submit_input` payload.
