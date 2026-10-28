# API reference

OffGrid exposes a local HTTP API at `http://127.0.0.1:11611`. The stable core
contract is [pkg/api/openapi.yaml](../../pkg/api/openapi.yaml) and is also
available from a running instance:

```bash
curl http://127.0.0.1:11611/openapi.yaml
```

The OpenAPI document is the source of truth for request and response fields.
This page explains the main workflows and persistence semantics.

## Native execution workspaces (development)

`GET /api/v2/execution/workspaces` returns ready, already locally approved hosts
scoped to the authenticated actor and service workspace. It does not accept a
folder path or create authority. The catalog is empty without a trusted host
connection. Enrollment and desktop folder approval are explicitly enabled
development-preview capabilities; implementation/fixture evidence does not
qualify an installed platform or model workflow.

With `OFFGRID_ENABLE_EXECUTION_PREVIEW=true`, a matching native CLI/supervisor
can request a two-minute single-use ticket from authenticated
`POST /api/v2/execution/pairing` with `{}`. Local review precedes ticket creation;
the ticket itself cannot approve a folder. The host independently verifies the
worker digest/version and private grant/journal, then establishes protocol-v1
transport. User API credentials are not forwarded onto the host channel.
`POST /api/v2/execution/revoke` accepts `{reference: ...}` for an owned connection.
It revokes service authority, not earlier effects or proof of disconnected process
termination. Tickets, host credentials and grant files must stay private.

Services advertising `agent-response-profiles-v1` accept optional
`response_profile: "interactive" | "extended"` on `POST /api/v2/jobs`.
Interactive uses the configured Chat allocation bounded by the model limit;
extended uses the eligible model limit. Omission retains the legacy allocation.
The selection is persisted per job and participates in request idempotency;
it does not change service settings, external-agent context or existing jobs.
Session generation retains its existing `profile` field for this choice.

Tool-enabled `POST /api/v2/jobs` and agent session generation accept an optional
`execution: {host, session, grant, revision}` reference. The job and conversation
intent pin it, and it participates in submission idempotency. Older callers omit
it and retain their existing behavior. A reference is not a grant: unavailable,
cross-owner or changed grants are rejected before admission. Retrying already
accepted input retrieves the canonical job even after its host disconnects; it
does not dispatch again or renew authority.

Services advertising `execution-access-request-v1` also accept
`execution_request_version: 1` on tool-enabled jobs and agent turns. It makes
local access requestable, not granted. Conversation can complete without a host.
When a tool needs local access, the canonical root enters `waiting_for_input`
with `pending_input.kind: "workspace"` and mode `read`, `edit` or `trusted`.
The trusted client obtains local approval and posts
`{input_id, execution: {host, session, grant, revision}}` to
`POST /api/v2/jobs/{id}/input`. This endpoint accepts exactly one execution
reference or computer-session reference, never both. It independently validates
actor, service workspace, profile/task ceilings and the current pending call.
Duplicate attachment acknowledges the same root without replay; another folder
or host cannot replace an already attached execution scope during an upgrade.
Cancellation, unresolved effects and unsupported versions cannot acquire access.
Older clients/jobs keep their existing behavior. Remembered folder trust belongs
to the trusted local client and does not create service authority by itself.

Selected-workspace jobs use reviewed `workspace_*` tools, not the service's
`read_file`, `write_file` or `list_files`. Relative paths and cwd belong to the
selected host. Reads retain original bytes/revisions and include exact UTF-8 text
when the page is valid UTF-8; no line-number display prefix is added. Writes use
exclusive creation or an exact preimage revision and independent read-back.
Host loss or unknown effects must not trigger a service-file fallback or replay.

`AgentProfileSettings.execution` and per-turn `restrictions.execution` contain
`{read, write, processes}` ceilings. Omission inherits the separately selected
host grant; all-false disables host tools. Turn restrictions may not widen a
pinned profile. Discovery and dispatch independently apply profile/turn and
current host limits. No setting, profile revision or saved conversation creates
an execution grant. The admission-time `execution_workspace` remains an immutable
host snapshot, distinct from this effective policy and current readiness.
The reviewed process tools require a separately approved trusted-process grant.
Routine invocation grammar runs under that session; unknown commands require
the runner's exact per-call approval. The worker repeats policy checks. Tool
arguments cannot carry an approval flag. Process start returns a handle; bounded
output/wait tools retain actual exit codes, and completion requires terminal host
evidence. Stop remains uncertain if owned-tree termination is unconfirmed.
No public arbitrary-command HTTP endpoint or arbitrary-folder registration exists.

For uncertain host mutations, `can_recover_execution` indicates that the original
intent is retained for inspection. `POST /api/v2/jobs/{id}/recover-execution` takes
only `{}`. It may incorporate a conclusive authenticated receipt already saved in
the service or read it from the same ready, pinned host journal. It never claims,
dispatches, substitutes another host, resumes work or accepts a user-supplied
result. Missing/partial/dispatched evidence remains uncertain. A recovered result
moves the canonical job to interrupted for explicit review; it does not prove
overall success or the current file state after later manual edits.

The `/api/v2/execution/host/{enroll,poll,reply,heartbeat,revoke}` routes are private
native-supervisor transport, not ordinary client APIs. Exact routes accept only
their single-use ticket or host-specific credential, strict bounded JSON and
matching protocol/actor/workspace/grant envelopes. They expose no general user API
and never redeliver an operation after a lost response. The local protocol is
documented in [internal/execution](../../internal/execution/README.md).

These are source/API and isolated-fixture results, not installed-app, model or
cross-platform qualification. See the [execution evidence](../quality/evidence/native-agent-execution-2026-10-25.md).

## External integration assessments

The llama.cpp chat path accepts `response_format` with `text`, `json_object`, or
`json_schema` (a named `json_schema.schema` object, at most 64 KiB, local references
only). Constraints are forwarded to the runtime, not dropped. Constrained formats
cannot currently be combined with tools, and native text providers reject them
before generation until qualified. Text-completion endpoints still reject this
chat-only control. A format constraint is not proof of factual correctness; a
truncated response is not a completed structured result.

Unchanged validated schemas retain their submitted property order when forwarded
to grammar-based runtimes, including exact numeric constants. Duplicate object
keys and malformed Unicode are rejected. In-process schema edits supersede any
retained wire representation; no stale schema is forwarded after an edit.

Non-streaming model loads that cannot fit their requested context return HTTP 503 with
`insufficient_runtime_capacity`; they do not silently shrink context or change
the saved external-agent configuration. Battery percentage and transient pressure
do not change the planned catalog context. Load-time memory checks and runtime
confirmation remain separate from service-only readiness inspection.

Accepted streamed chat emits a standard assistant-role chunk before waiting for
the inference runtime. This is acceptance, not generated text or task completion.
SSE comments contain request-local phase/elapsed metadata (`waiting_for_runtime`,
`loading_model`, `waiting_for_output`, `receiving_output` where observed); consumers
may ignore them. No token counts or prefill percentages are inferred. Tool
decisions remain buffered until complete validation. Streaming requests have a
15-minute service budget, including admission/loading; earlier client deadlines
still apply. Keep-alives do not reset that budget.

Authentication/input/selection failures precede stream startup. Once started,
operational failures use the SSE `error` envelope and never `[DONE]` or a fabricated
successful finish. Clients must handle both HTTP errors and SSE errors, including
`insufficient_runtime_capacity` and `inference_queue_full`. Non-streaming chat
cancellation uses `request_cancelled` (499), and deadline expiration uses
`inference_timeout` (504), rather than claiming a model-switch failure. These
outcomes do not authorize automatic replay of external-agent tool effects.

`GET /v1/integrations` and `GET /v1/integrations/{id}/setup` use registry-defined
integration IDs. Existing `ready`, `status`, model and context fields remain.
Additive `contract_revision`, `checks`, `readiness_scope: "service_model"`,
`host_state: "not_inspected"` and `workflow_state: "unverified"` make the boundary
explicit: this service cannot certify an external CLI installation or a tool
workflow. Requests use normal service authentication; generated snippets never
contain the caller's credential. Host `configure/status/test/run` commands use
these APIs, not a separate inference executor. See the
[external agent guide](../guides/external-agents.md).

## Task-wide desktop access

Computer sessions accept one tool call per model turn. A multi-call response
is rejected in full with `computer_multiple_tool_calls`, `effects: "none"` and
paired tool feedback before any call is queued, authorized or dispatched. The
model can correct its next response within the shared three-repair budget;
exhaustion interrupts the saved task rather than permanently failing it. A
successful single call clears the sequencing repair only when it completes the
first rejected dependency; unrelated catalog, plan and history reads do not
reset its budget. Outstanding access, read-back and edit checks remain. Prose
alone cannot resolve the repair. Earlier
actions are not replayed; persisted legacy batches and uncertain actions remain
blocked. Ordinary non-computer tool batching is unchanged.

A missing initial target observation produces a durable
`computer_observation_required` rejection before dispatch, rather than a terminal
task failure. Its activity record includes the proposed tool identity, reason and
operation-local `effects: "none"`, but not rejected arguments or assistant prose.
The runner provides bounded corrective feedback using the existing shared repair
budget and charges each attempt to task iterations. Observation alone does not
complete the rejected operation; unrelated reads and a claimed final answer cannot
erase it. Exhaustion pauses the same saved task. Failed persistence stops recovery;
uncertain dispatched input still requires reconciliation, not another attempt.

The service-owned `task_save_artifact` capability does not require a desktop
observation or native-document expectation merely because desktop access is
attached. Its owner/profile, format, authorization and independent storage
read-back checks still apply. Artifact-only work with verified service receipts
needs no invented native check; an artifact cannot satisfy an outstanding native
expectation, unfinished plan or earlier application action. This does not grant
host filesystem access or certify the factual accuracy of artifact content.

Artifact arguments advertise format-specific filenames (`text` → `.txt`,
`markdown` → `.md`, `json` → `.json`, `csv` → `.csv`) and nonempty content.
The service still enforces UTF-8 byte limits (120 filename bytes, 131072 content
bytes), safe basenames, JSON/CSV parsing and prohibited CSV formulas. A rejected
argument reports a specific field-level reason without echoing private content.
`invalid_task_artifact` is eligible for bounded correction only before dispatch;
it saves no artifact. Unrelated reads and a final prose claim cannot clear this
repair. A storage failure after dispatch is not eligible for automatic retry.

Runtime rejection of the generated tool grammar interrupts the saved job with
`invalid_tool_grammar`, not a model-decision repair loop. Its fixed diagnostic
exposes no raw schema or private runtime response. Earlier work remains saved;
unknown effects still take precedence and require reconciliation. Refresh does
not replay work. Resume is explicit after the service/runtime fault is corrected.

A model-requested computer access operation must inspect its current target
before artifact saving is offered. Persisted proposals pass the same check at
authorization (`task_artifact_work_pending`). Artifact-only completion cannot
erase this pending inspection. This does not apply merely because a desktop
grant is preattached; an explicit subsequent user correction can supersede the
old proposed operation without erasing earlier effects. Target inspection and
storage receipts are necessary mechanical checks, not proof that research is
complete or that cited sources substantiate every claim.

Governed internal tool requests explicitly select a private schema-constrained
single-step envelope. Public `parallel_tool_calls: false` controls cardinality;
it does not select that private protocol. The governed adapter normalizes one complete,
validated response into the existing tool-call contract. Partial JSON, duplicate
keys, unknown tools, truncated generations and incomplete streams cannot
dispatch input. Counting and generation use the same wire messages and schemas.
The changing catalog stays near the current exchange to preserve the cached
instruction/history prefix. Tool-enabled terminal text is delivered after this
envelope finishes; ordinary text-only Chat retains token streaming.
Completion and SSE projections preserve one unique call ID generated only after
clean envelope validation. Native compatibility checks use the same transport
and stage-relevant tools; a synthetic preflight pass is not native workflow
qualification. Failed checks retain their service-authored reason.

External tool requests retain the model's native template, tools and history,
including sequential requests. Answer text streams normally; tool fragments and
successful finish events are withheld until the complete response passes identity,
schema, cardinality and terminal validation. This is not a prose-to-tool fallback. Tool-enabled agent
responses from either transport pass a shared complete-decision gate before
dispatch. It checks terminal metadata, offered tool identity, unambiguous JSON
and the offered argument schema using local references only. It never downloads
schema references or logs rejected argument values. Invalid decisions produce
bounded, persisted pre-effect feedback and consume task iterations; repeated
failures pause the saved task. Operational/provider errors and persistence
failures do not acquire this retry classification. Existing complete computer
batch rejection still uses the runner's paired-feedback recovery path.
Earlier effects, permissions and unresolved outcome checks are unchanged.

Concurrent identical synthetic tool checks share only their in-flight work.
Cancelling one caller does not cancel remaining observers; cancellation of the
last observer cancels the shared check. Completed results are not cached, and
later tasks check the current runtime again. Sharing this synthetic fixture does
not share user prompts, grant access or dispatch computer tools. Separate vision
checks keep their own lifecycle.

Model-check responses optionally include `timings.admission_ms` (queue/model
readiness) and `timings.probe_ms` (runtime metadata plus synthetic generation).
These measure the shared tool check, not one observer's waiting time or the
separate vision check. Completed operational failures retain these timings in
the error envelope; a caller cancelled before completion has no completed timing
measurement. The probe deadline starts after admission. Busy/cancelled checks are
operational failures, not proof that a model is incompatible.

Desktop model discovery follows recorded progress: a catalog precedes launch,
readiness precedes target operations, document inventory precedes creation,
and independent read-back precedes document edits. Check schemas
include only observed references and supported operations. These projections
are planning aids, not grants: service and host validation remain authoritative.
Unchanged plans are not offered repeatedly without intervening action progress.
Model history retrieval lists only unread omitted/archived pages. Document text
paging starts without a guessed page reference; continuations require the exact
returned snapshot, revision and next offset. A complete read is not offered for
repeated paging; fresh inventory permits another independent read. Duplicate
content/edit declarations for the same snapshot are not advertised again.
Unchanged content checks are reused across fresh snapshots. After dispatched
document input, fresh inventory and independent read-back take priority over
further native planning. After a verified document stage, another creation/edit
requires saved plan advancement; MCP tools remain composed throughout.
Canonical APIs, saved history, grants and dispatch checks are unchanged.

Native read failures carry operation-local `effects: "none"`, not a claim that
the whole task made no changes. `computer_target_not_selected` and
`computer_application_not_ready` permit bounded read recovery under the same
valid grant; `computer_document_changed` requires fresh observation. Missing
selection does not revoke authority, while `computer_scope_violation` still
does. Recovery shares the persisted three-proposal-repair budget. Catalog reads
and negative readiness checks cannot reset it; exhaustion interrupts the same
task and requires explicit continuation. An earlier dispatched launch or input
is never repeated by this recovery. Old uncertain records are not rewritten.
If a launch proposal was rejected before dispatch, recovery explicitly permits
the first authorized launch; it does not invent an earlier launch to prohibit it.
Descriptors for reviewed native tools include `revision: "native-effects-v1"`;
changed or absent revisions do not reuse a pending approval.

Task snapshots support an optional `workflow` persistence record (versions 1 and 2).
It holds bounded outcome expectations, their instruction digest and revision,
caller/model provenance, and service-authored check assessments. The checkpoint
retains the completion-repair budget across restart. These records share the
task snapshot/event transaction: failed writes cannot publish verified results.
Older records remain absent, preserving their saved status and result without
retrospective certification. Unknown versions are retained losslessly for
inspection but refused for execution. Downgrading while versioned work is active
is unsupported; stop work first and never replay grants from a backup.
Optional fields omitted by callers do not change existing request identities.

Owner-scoped job GET, snapshot events, export and legacy run projections expose
this record additively as `verification`; existing `status`, run IDs and outputs
retain their meaning. Activity metadata includes only `verification_state`, not
private control text. Version 1 contains `revision`, `instruction_id`,
`instruction_sha256`, `state`, `checks`, and prior revision `history`.
Check states are `pending`, `verified`, or `unavailable`; overall states are
`pending`, `partial`, `unverified`, or `checks_passed`. Unknown versions must be
displayed as unavailable, never as verified. Absence is a legacy record, not a
success claim.

Version 2 adds document-aware persistence and verification. New task-first jobs
select this contract; saved v1 tasks are not upgraded. Document tools additionally
require an actor-owned host session negotiating `native_documents:1`. V2 alone
does not enable native document input or change a legacy control-edit tool.
Legacy records without a workflow retain version-0 behavior, v1 readers/writers
remain supported, and unknown versions remain opaque and non-executable. V1
records reject v2 fields even when null or inside historical revisions.

New jobs additionally pin the service-owned `native_action_contract_version:1`
in their configuration. For this contract, a validated typed application launch,
document creation/edit, or control text/checkbox edit supplies the mechanical
predicate. The runner binds and persists it with the pending invocation **before**
authorization or dispatch; the model does not need a separate `task_expectations`
call to unlock that action. This is not inferred permission, automatic execution,
or proof that the typed action satisfies the entire user request. Existing caller
checks remain immutable. Saved jobs without this flag keep their explicit-check
behavior; unknown action-contract versions cannot execute.

`OutcomeCheck.action_derived_call_id` and `AgentStep.call_id` are optional,
service-authored identities. A derived check is assessed from the matching
successful invocation and a later independent read, never from a saved success
flag, input echo or earlier read. Once verified, it records a historical action
milestone: later navigation does not require a search field to retain its old
query or a new document to remain blank. Explicit checks retain their existing
current-state semantics. Pending or uncertain actions are not historical proof,
and further input must wait for the required action read-back.

The new contract also persists bounded semantic read-progress accounting. Fresh
observation IDs/timestamps and metadata changes are not progress. Three unchanged
reads produce corrective feedback; five pause with `computer_no_progress`.
Changed target/content/evidence or an applied user correction establish progress.
This consumes the original iteration budget and never forces or repeats input.
An explicitly configured budget may still be too small for a multi-stage workflow;
the runner does not silently expand it.

A trusted `computer_documents` unsupported-capability response is a nonterminal,
no-effects result (`status:unavailable`, `capability:native_documents`,
`supported:false`, `code:computer_document_unavailable`, `effects:none`). It
permits fresh inspection of controls on that same target, not blind input or a
permission bypass. Incomplete, stale, revoked or uncertain results do not qualify.
Changed application/process/surface identity invalidates this fallback evidence.

The v2 predicates are `document_blank`, `document_text_equals` and
`document_edit_matches`. Blankness declarations name an observed creation
`command`, never a guessed future document. Text predicates name observed
`document` and `snapshot` references; edit matching also names an explicit
`edit_mode` (`insert_text`, `replace_selection`, or `replace_value`) and the
requested text, up to 16,000 UTF-8 bytes. These are declarations, not authority.

The service-authored `binding.document` pins document ID, generation, container,
editor, revision and snapshot beneath the existing session/full-target digest.
Creation and edit checks additionally bind one action ID and operation. Edit
evidence includes the host-prepared preimage and expected-result SHA-256 digests;
these are computed from exact bytes, not accepted from model arguments.

Canonical successful action results and a later independent native read-back
are both required for creation/edit verification. Read-back carries its own
snapshot, full document binding, completeness, composition state, byte length
and exact text digest. An input echo, truncated text, another tab with identical
content, a replaced generation, stale revision or uncertain mutation cannot
verify that action. For legacy and explicit checks, contradictory observations clear the check. Once independently
verified, a creation/edit milestone may survive a later verified edit to the same
document: the later action must match the earlier read's document generation,
editor, revision, snapshot and preimage digest, and have its own independent
read-back. The complete evidence chain is retained. An unknown input, changed
tab, unmatched preimage or missing initial blankness read cannot extend it.
An independently verified exact-text checkpoint can survive valid reads or
edits of a different document on the same target. Affected-document input,
target replacement and unknown or uncertain dispatch remain conservative.
`document_text_equals` proves only the observed text, not who changed it or a
particular action's completion. Evidence contains no full private preimage;
this does not make saved task instructions or declared text non-sensitive.

The v2 pre-dispatch guard requires a `document_blank` check for the exact
observed creation command, or a `document_edit_matches` check for the current
document/snapshot, explicit edit mode and requested text. An application-only
or `document_text_equals` check cannot authorize an edit. Canonical successful
host results bind the actual created document and one action identity; independent
read-back is still required afterward. Old, already-dispatched or foreign-session
bindings cannot cover another input. Declarations are staged atomically: a
malformed item cannot partially bind other checks. The guard reuses the existing
three-repair budget; metadata cannot turn partial/uncertain input into a retry.

OpenAPI describes `OutcomeDocumentBinding`, `OutcomeBinding`, `NativeReadFailure`
and `NativeApplicationReadiness`. `AgentStep.tool_result` stays a string for
compatibility; trusted read failures contain the documented JSON inside it.
CLI/Python projections retain v2 and unknown-version results without interpreting
them as success. Public submission still requires a supported negotiated task
contract; exposing response types does not enable an absent native provider.
The public Python client in `sdk/python` uses this same authenticated HTTP
contract; the separate `runtimes/python` service workers are not an SDK API.

A locally qualified MLX or Transformers text model can also be pinned to a
text-only agent profile and run through the canonical job path. Such a job has
no tools, collaborators, or computer access; selecting a tool-enabled profile
requires a separately qualified tool-calling model. Native models are not
enabled by merely importing their files, and a stale or absent runtime never
falls back silently to llama.cpp.

`POST /api/v2/jobs` accepts optional `expectations` (at most 12) with stable ID,
title, kind and predicate-specific arguments. Their provenance is
`caller_explicit`; `task_expectations` declarations are `model_proposed`.
The model cannot submit bindings, provenance or successful assessments. These
records activate only on new task-first native workflows and grant no access.

For legacy jobs and explicit additional checks, `task_expectations` accepts staged additive declarations: `application_open`
requires an `application_id` from discovery; control checks require an observed
`target_id`, `element`, and `text` or `checked`. Invalid declarations return
`invalid_task_expectations` with field-specific feedback and no dispatch. These
failures consume the existing proposal-repair budget, including across read-only
discovery and persistence. A corrected, validated declaration resolves that
repair; unrelated authorization failures are not cleared. Arguments remain in
owner-scoped task history, not general logs. Older saved system prompts receive
the same staged guidance through the versioned workflow context.

For version-1 native workflows, `computer_replace_text` and
`computer_set_checked` also require a matching declared control check before
host preparation or dispatch, including automatically or exactly approved edits.
The check must match the proposed value, current session, full target identity
and host-issued control identity. An application-only check is insufficient.
Missing checks return `workflow_control_expectation_required` with corrective
guidance. Reads and unrelated declarations do not reset its persisted repair
budget. A validated matching declaration or successfully guarded edit resolves
the repair; it does not establish successful read-back. Older hosts can bind an
edit to its exact current observation/control but still cannot claim
cross-observation verification. Legacy version-0 tasks keep their original path.

The native tools `computer_find_controls` and `computer_inspect_control` query
only the current task's latest canonical observation. They check live ownership,
session and target on authorization and execution, resolve exact short aliases,
and make no companion calls. Stale capture age is explicit; querying never
refreshes input authority. See [bounded discovery](../../apps/computer/README.md#workflow-evidence-and-bounded-discovery)
for filters, byte limits and pagination. A revoked session cannot use discovery;
separately authorized task history retains its existing read-only semantics.

`POST /api/v2/jobs` accepts `computer_scope: "desktop"` with a saved
`computer_approval_mode`. Admission atomically returns `waiting_for_input` and
`pending_input.mode: "desktop"` before model/tool dispatch. Omitting scope keeps
legacy behavior and request hashes; `selected_window` is explicitly narrower.

The trusted desktop requests `/api/v2/computer/pairing` with
`protocol_version: 3`, `run_id`, and `input_id`. These must identify the actor's
current saved access request. Pairing returns one short-lived enrollment code;
it does not authorize execution. The private companion pairs, obtains local
consent, activates its exact token, then the client resolves the existing
`/api/v2/jobs/{id}/input`. Retrying resolution does not duplicate work.
Session snapshots distinguish `awaiting_consent` from `in_use` and include the
immutable desktop authority and pending-input identity. Never log enrollment
codes, bearer tokens, or raw observations. Ending the task revokes its grant.

In the matching Windows desktop, explicit session Full access may already be
enabled before job submission. This is a host-memory permission, not a server
preference or an API bearer grant. Each task still requires the held job,
actor/workspace-bound enrollment, host activation and idempotent input resolution
above. Task completion releases its grant while session permission remains until
local revocation or desktop-session/identity loss. REST callers cannot set a
`consent: true` flag to bypass the host. There is no persisted session-consent API.
The current desktop menu combines policy selection and explicit host consent
in one Full task access choice; its checked state reflects host permission,
not the saved proposed policy. This does not change the REST authorization boundary.

Desktop model calls cap generation at 1,024 tokens to retain observation/tool
headroom. Completed call/result exchanges may be durably archived, but the latest
exchange and user instructions are retained; `task_history` retrieves exact older
evidence. A context-limit error does not roll back earlier dispatches or authorize
replaying them. Inspect recorded actions before starting another task.

Desktop native observations use a bounded model view (up to 24 controls and
4,000 encoded row bytes), prioritizing writable controls and marking shortened
names/text and omitted rows. Saved steps and checkpoints retain the original JSON.
`task_history` accepts `step:N` references from observation details and returns
UTF-8-safe pages of at most 1,024 bytes with `next_offset`. Reading saved evidence
does not refresh host authority. Task-local short application/control references
resolve to canonical IDs before proposal persistence and authorization; MCP
arguments and entered text are never rewritten. Legacy selected-window projections
remain lossless. The selected runtime's
optional input-token-count operation measures the complete model request, including
tool schemas and template options, under an inference lifecycle lease without
generation. Older runtimes returning unsupported keep conservative content-byte
accounting (not double-escaped HTTP JSON); other counting failures stop safely.
Transient browser images do not use this text-only measurement. Output headroom
and the allocated runtime window remain enforced. Archive references `archive:1`
through `archive:N` are stable task-local aliases; existing digest references
remain valid. Listing every digest in each prompt is unnecessary.

Protocol-v3 Windows structured-control preparation/readback can freshly re-observe
an expired retained view. It continues only when the complete view, runtime/state
digest and exact target are unchanged, with a new capture identity/time. Changed,
incomplete or foreign observations require new model observation; neither an
expired approval nor a dispatched action is retried. V2 and other drivers retain
their existing freshness checks.

The archive index follows stable system instructions rather than rewriting their
prefix. Application launch evidence is reconstructed from durable steps, including
after archival/restart. Reopening an unverified launched application returns
`computer_application_verification_required` before dispatch and enters the bounded
proposal-repair loop. `computer_verify_application` must establish matching
readiness; catalog discovery alone does not clear the repair. Discovery refreshes
the host's OS-registered installed-app catalog, preserving IDs only for unchanged
executables. A newly installed or replaced executable gets a fresh ID; old prepared
launches cannot authorize its replacement.
Verified application state also survives archival; `computer_application_already_ready`
rejects redundant launch proposals before dispatch. Continue the unfinished outcome
or inspect/focus the application's surface. Application readiness is not proof of
a search, saved file or external submission.

The companion-only `activate` and `revoke` routes under
`/api/v2/computer/companion/` require the enrolled bearer token, reject browser
origins, and are not general user APIs. Existing protocol-1/2 transports remain
limited; no automatic desktop downgrade is allowed. See the generated OpenAPI
contract and [host protocol reference](../../apps/computer/README.md).

## Model packages (development contract)

The shared catalog/inventory types distinguish `legacy_file` and `package`
installation targets and four capability categories (`language`, `embeddings`,
`speech_recognition`, `speech_generation`). Each variant carries provenance and
independent readiness fields. This is not a change to the existing v1 model IDs
or chat/embedding projections.

| Resource | Behavior |
| --- | --- |
| `GET /api/v2/models?category=...` | Typed installed inventory with independent readiness |
| `GET /api/v2/models/catalog?category=...` | Shared curated variants |
| `GET /api/v2/models/catalog?category=...&source=huggingface&q=...` | Explicit public repository metadata search |
| `GET /api/v2/models/discover?repository=owner/repo` | Complete variants bound to an immutable commit |
| `POST /api/v2/models/resolve` | `{catalog_id}` or `{repository,revision,variant,architecture}`; actor-bound preview, licenses and space preflight |
| `POST /api/v2/models/operations` | Install with `{action:"install",request_id,resolution_id}`, or repair with `{action:"repair",request_id,source_operation_id}` |
| `GET /api/v2/models/operations[/{id}]` | Actor-scoped durable progress; administrators can inspect all |
| `POST /api/v2/models/operations/{id}/{cancel,resume,discard}` | Explicit lifecycle controls; discard requires settled work |

GET requires model-read permission; POST requires model-management permission.
Previews expire after ten minutes and do not start transfers. Acceptance is `202`
only after source/actor/request/checkpoints are persisted. Retry the same request
ID and payload after a lost acknowledgment; changed source returns `409
request_conflict`. A new unexpired actor-owned preview of the identical immutable
manifest/source also deduplicates. This never replays a completed download.
An operation is a package target; legacy v1 file transfers retain their original
projection in the same versioned download store. Do not fabricate package filenames.

States: `queued`, `downloading`, `verifying`, `activating`, `complete`, `cancelling`,
`cancelled`, `failed`, `interrupted`. `bytes_done` is not installation readiness.
`retained_bytes` includes staged artifact and recovery-copy bytes, excludes small
control metadata and may exceed transfer size during repair. Requests and snapshots
carry safe errors (`resolution_expired`, `package_in_use`, `source_conflict`,
`source_identity_mismatch`, `insufficient_space`, etc.). Restart reconciles durable
local publication receipts but never resumes network transfers. See
[storage recovery](../advanced/workspace-recovery.md#model-download-recovery).

`GET /api/v2/models/packages` lists managed speech packages without changing the
legacy `/v1/models` chat/embedding projection. It requires model-read permission.
The `installed`, `integrity`, `runtime_compatible`, `smoke_tested`, and `qualified`
fields are independent. Currently package storage does not certify speech runtime
compatibility or qualification. `verified_at` records a past byte check, not a
guarantee against later local modification; runtime leases always reverify.

Model-management permission is required for these mutations:

- `POST /api/v2/models/packages`: streamed multipart import. First send `manifest`,
  then artifact parts in manifest order with each exact relative artifact path as
  its **form name**. Extra parts are rejected before activation. The response is
  `201` after activation, not `202` acceptance of an asynchronous job.
- `POST /api/v2/models/packages/{id}/{revision}/verify`: recheck all declared data.
- `POST /api/v2/models/packages/{id}/{revision}/remove`: remove only that managed
  revision; an active lease returns `409 package_in_use`.

Imports are bounded to 32 GiB of model artifacts plus limited multipart metadata;
ordinary endpoints retain their existing 64 MiB request limit. Failed imports do
not activate partial models. Local inputs remain untouched. The manifest/layout
rules and outstanding program work are in the [model guide](../guides/models.md).
Folder import is synchronous; repository acquisition uses the durable operations
above. Verified portable speech/runtime offline-export packs remain a later stage.

### Native application readiness

`computer_verify_application` performs a bounded read (maximum ten seconds), not
another activation. Its response keeps `verified`, `check: "application_open"`
and the opaque `application_id`, and adds `readiness`: `ready`, `starting`,
`surface_unavailable`, `unsupported_identity`, or `observation_unavailable`.
Only a verified result includes `target` and `view`; a failed check omits them
rather than serializing zero-value identities. Cancellation and revoked authority
remain errors. Older responses without `readiness` are still accepted.

The durable runner records a negative check as an operation-local `effects:none`
failure with its readiness reason. Recoverable checks share the existing bounded
repair limit; unsupported identity requires explicit intervention. Neither case
replays earlier launches or satisfies a document/text outcome. Package paths,
AUMIDs and activation arguments are host-only data, not model-controlled inputs.

### Native document contract (implementation in progress)

The shared `native_documents:1` types define document/tab identity separately
from a window and its editor. A binding pins `target`, `document`, `generation`,
`container`, `editor`, `observation` and `revision`. Display titles and tab order
are not identities. Replacing a document invalidates its generation even when an
application reuses the editor. Inventory carries `complete`; missing evidence
cannot establish an empty document or permit creation. Supported read, selection,
insertion, selection-replacement and whole-value operations are separate flags.

Snapshots distinguish complete text, selection, and `stable | active | unknown`
composition. Editing requires complete text and stable composition. Model-facing
ranges are half-open Unicode scalar offsets over the exact snapshot; native
UTF-16/UTF-8 conversions reject split graphemes, surrogate pairs and CRLF. Text is
not trimmed or normalized. Limits are 16,000 UTF-8 input bytes, 64 KiB per complete
host snapshot and 4 KiB per transport page; these are rejection limits, not silent
truncation. The shared host pager returns `snapshot`, `binding`, scalar `start`
and `end`, `total_scalars`, `text`, `source_complete`, and an optional `next`
offset. Each page rechecks a fresh provider snapshot against the pinned document
and exact content revision. A changed document/revision fails; it cannot mix old
and new text. A grapheme larger than one page is unsupported, never split.

Shared host preparation distinguishes `select_text`, `insert_text`,
`replace_selection`, and `replace_value`. Selection accepts only a range;
the other modes accept only text. Insertion requires a collapsed caret;
selection replacement requires a nonempty selection. The host computes the
expected result from its complete preimage, preserving the prefix and suffix.
It rejects an oversized result, changed selection/content/identity/capabilities,
or unstable composition. Preparations own copies of their inputs; fresh
observations may revalidate them but cannot extend their authority. These are
host-internal contracts, not additional public dispatch endpoints.

The private version-2/version-3 worker transport now carries document inventory,
revision-bound read pages, and typed preparations through the existing supervisor
and dispatch journal. A page request contains only `snapshot`, `revision`, and
scalar `start`; the host performs a fresh read before returning it. A new
preparation can revalidate an unchanged document after model latency, but cannot
renew an expired approval or reuse an old dispatched action. Changed text,
selection, composition, document sets or capabilities invalidate it. Input is
attempted once; partial input, a lost result, or invalid result evidence cannot
trigger fallback typing. Journal recovery validates the stored document identity
and action evidence before acknowledging a completed result.

The Windows backend uses numeric UI Automation roles, SelectionItem's
selection container, and child-pane/ControllerFor relationships. It does not
associate a tab with an editor by title, order or matching text. Complete
inventory and text are rechecked before input. Unsupported relationships,
indeterminate composition, or missing editable attributes remain unavailable.
It now negotiates `native_documents:1`. A package-revision-bound Notepad profile
adds observed WinUI creation semantics; its exact version and unverified native
status are listed in the [computer guide](../guides/computer-tasks.md).

AX and AT-SPI adapters also implement the negotiated document interface, retaining
native object identities rather than matching titles. They expose operations
only when the current surface provides their required evidence. AX writes require
settable attributes and explicit marked-text state. AT-SPI currently reports
unknown composition, so document input remains unavailable even where a complete
read succeeds. Their reviewed creation profiles and browser blank-tab profiles
are not implemented. macOS compilation and native operation checks remain
unverified; Linux Xvfb read/fixture tests are not Wayland qualification.

The service routes negotiated document tools through the same capability broker,
host preparation, durable journal and result assessment as other computer tools.
Schemas follow the current observed stage and preserve authorized MCP tools.
Both discovery and dispatch check the negotiated feature. Unnegotiated/foreign
sessions cannot enqueue document commands. Complete document requests allow the
16,000-byte input even when JSON escaping expands its wire representation; old
control requests retain their existing bound.

| Tool | Arguments beyond the current `observation_id` | Availability |
|---|---|---|
| `computer_documents` | No arguments, including no observation ID | Negotiated host; read only |
| `computer_create_document` | `command` | An observed reviewed creation command and declared blankness check |
| `computer_select_document` | `document` | Observed selectable document |
| `computer_read_text` | `document`; optional `page` with `snapshot`, `revision`, `start` | Active readable document |
| `computer_select_text` | `document`, `snapshot`, `start`, `end` | Complete read and supported selection |
| `computer_insert_text` | `document`, `snapshot`, `text` | Supported operation and collapsed caret |
| `computer_replace_selection` | `document`, `snapshot`, `text` | Supported operation and nonempty selection |
| `computer_replace_text` (document form) | `document`, `snapshot`, `text` | Explicit whole-value support |

Unknown/duplicate fields, nulls, invalid Unicode and invalid ranges are rejected.
`computer_document_unavailable`, `computer_document_incomplete`,
`computer_document_range_invalid` and `computer_document_composition_unstable`
have actionable pre-dispatch/read errors; a failure after input remains uncertain.
Only actual mutating document operations consume the action budget.

The legacy `computer_replace_text` control form keeps its meaning outside a
document stage. A fresh generic observation returns to that control catalog;
the model is never offered both argument shapes together. These service paths
do not qualify UIA/AX/AT-SPI providers, and an application launch cannot verify a
new document. Installed document workflows remain unavailable until their host
provider and reviewed application profile are implemented and tested.

## Health and readiness

```bash
curl http://127.0.0.1:11611/health
curl http://127.0.0.1:11611/ready
```

`/health` confirms the process is alive. Readiness additionally reflects
whether required runtime components can accept work. Kubernetes-style
`/livez` and `/readyz` aliases are available.

## Authentication

Loopback quick start is unauthenticated. Set `OFFGRID_REQUIRE_AUTH=true` for a
network deployment. Browser login uses an HTTP-only session cookie; API clients
can use a bearer API key:

```bash
curl -H "Authorization: Bearer og_YOUR_API_KEY" \
  http://127.0.0.1:11611/v1/models
```

Authentication does not replace network controls. Bind to a trusted interface,
use TLS at a reverse proxy, and grant only the permissions a client needs.

## Models

List installed models and discover the curated catalog:

```bash
curl http://127.0.0.1:11611/v1/models
curl http://127.0.0.1:11611/v1/catalog
```

Model lifecycle endpoints include:

- `POST /v1/models/download`
- `GET /v1/models/download/progress`
- `POST /v1/models/download/cancel`
- `POST /v1/models/verify`
- `POST /v1/models/delete`

Downloads retain a partial file when cancelled and resume through an HTTP range
request. A successful download is rescanned into the registry before it is
reported as installed. Verification hashes the model contents and can be
cancelled with the request context.

## Stateless chat

`POST /v1/chat/completions` accepts the OpenAI chat-completions shape:

```bash
curl http://127.0.0.1:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "phi-3.5-mini-instruct.Q4_K_M",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

Set `stream` to `true` for server-sent events. Structured streaming preserves
tool-call deltas and optional usage information for agent runtimes.

## Durable conversations

The UI uses server-owned sessions rather than browser-only message state:

- `GET /v1/sessions` lists conversations.
- `POST /v1/sessions` creates one.
- `GET /v1/sessions/{name}` loads its complete history.
- `DELETE /v1/sessions/{name}` removes it.
- `POST /v1/sessions/{name}/generate` generates and atomically persists a full
  user/assistant exchange.

If generation fails or the caller cancels before persistence, no half-turn is
written. Cancellation during the final save may still commit a complete turn;
reload the session before retrying. This route is the preferred application
workflow when conversation continuity matters.

The web/desktop chat requests `stream: true` and renders text as it arrives:

```json
{"content":"Explain this briefly","model_id":"installed-model-id","stream":true,"profile":"interactive","max_tokens":1024}
```

The response is `text/event-stream`, with JSON `data:` events:

- `phase`: `queued`, `retrieving`, `loading`, `processing`, `generating`, `saving`.
  These are stages, not estimated percentage progress. Heartbeat comments keep
  the connection active while a model loads or a request waits for a slot.
- `delta`: text to append to the provisional assistant response.
- `done`: `session`, `message`, `finish_reason`, and timing `metrics`. This event
  is emitted **after** atomic persistence, not simply after model generation.
- `error`: generation or storage failed; no completed exchange is acknowledged.

HTTP authorization/validation errors happen before streaming. Retrieval,
inference and persistence failures after headers use an SSE `error` event.
Treat EOF without `done` as interrupted/unknown, keep the draft, and reload the
session before retrying. There is no automatic replay or reconnect/resume of a
partially generated chat turn. Stop aborts the request, including queue waits.

`interactive` allocates the smaller of `OFFGRID_CHAT_CONTEXT` (default 8192) and
the selected model's admitted service context. `extended` uses that admitted
model context. In adaptive mode, `OFFGRID_MAX_CONTEXT` (default 65536) is a
ceiling intersected with known GGUF trained context, a conservative local memory
estimate, cache/slot settings, and power/pressure policy. Unknown evidence stays
on a conservative tier. The allocation is pinned when task work is admitted;
resource changes do not resize active KV state. `/v1/models` and integration
setup advertise the same per-model planned allocation. Health distinguishes the
configured ceiling from `inference.context_window`, the loaded allocation.
Profile changes are exclusive runtime switches and may reload the model. They
do not lower `/v1/chat/completions` context or the Hermes/OpenClaw configuration.
`max_tokens` accepts 1–4096, defaults to 1024, and bounds the response, not history.
The UI identifies `finish_reason: length` as a response-token limit.

Metrics include `total_ms` (through saving), `ready_ms` (including queue/load),
`queue_ms`, `load_ms`, `prompt_ms` (ready to first text, including transport),
`first_text_ms`, `generation_ms`, `context_window`, and, when the backend reports
usage, `completion_tokens` and an estimated `tokens_per_second`. Chunk counts
are not passed off as token counts. Streaming improves time to visible output;
it does not itself make model computation faster.

With authentication required, sessions are owned by their creator and all
operations check ownership. `owner_id` is server-assigned; clients cannot set
it. The `sessions:all` permission grants administrative access, including to
legacy conversations without an owner. Ordinary users cannot read or claim
those legacy conversations. Unauthenticated local mode retains access to them.
Enabling authentication does not delete or reassign existing data.

Names are currently unique across the installation. Creating a name that
already exists returns `409`, never overwrites history, and does not disclose
its owner or contents. Reading or modifying another user's conversation
returns `404`. Missing authentication returns `401`; missing session permission
returns `403`. Per-session HTTP mutations are serialized with generation.

### Profile-enabled conversation turns (development API)

`POST /v1/sessions/{name}/generate` also accepts an immutable agent selection:

```json
{"content":"Explain this briefly","request_id":"chat-turn-001","agent":{"id":"profile-id","revision":1},"restrictions":{"tools":[],"knowledge":[],"computer":false}}
```

With `agent`, the response is **202 JSON** containing `turn` and `job`, even
when `stream` is true. `Location` identifies the canonical `/api/v2/jobs/{id}`.
Without `agent`, existing JSON/SSE behavior above is unchanged. Agent-enabled
admission requires conversation ownership, not merely `sessions:all`.
Restrictions may only narrow the profile: omitted fields inherit; empty lists
disable that resource category. Selecting a profile never grants desktop access.
The legacy `profile` string is an inference preset, not an agent identifier.
Computer-enabled profiles default to proposed desktop scope and Full task access
unless explicitly restricted. This never acquires consent for a conversational
answer and does not fall back to the legacy selected-window flow.

Use one stable 8–128-character `request_id` for the submitted payload. Identical
retries return the same job; a changed payload or deleted-history identity
returns 409. The service pins the resolved model, instructions, resources and
bounded conversation history before creating a held job. It links the session
before releasing admission. Interrupted work is never resumed by retrying.

`GET /v1/sessions/{name}/turn` observes saved state and includes `job` for linked
work. Its `/events?id={turn-id}` endpoint uses the canonical job snapshot/activity
SSE protocol for linked turns, not the plain Chat delta protocol. Disconnecting
does not cancel the job. Job controls continue to use `/api/v2/jobs/{id}`.
Completed exchanges carry `job_id`/`turn_id`; the session's `jobs` list retains
historical links, including work that did not produce a completed answer.
The job's `steer` command accepts a stable request ID and instruction while the
parent runs. `pending_instructions` acknowledges durable receipt, not application
or cancellation of dispatched input. At the next safe boundary, undispatched
proposals are discarded and the correction enters `instructions`. Lost mutation
acknowledgments keep the job uncertain and the correction pending. Restart does
not apply queued instructions until explicit continuation.

If `recovery_required` is true, explicitly call
`POST /v1/sessions/{name}/turn/recover` with `{"id":"chat-turn-001"}`. This
finishes accepted bookkeeping or attaches an already saved result exactly once;
it does not run inference, approve, resume or repeat tools. The job remains the
result authority if conversation persistence is unavailable.

Deletion of linked active work requires `DELETE /v1/sessions/{name}?work=retain`
or `?work=stop`. Retain leaves work in the canonical inspector; stop returns 409
until the worker settles and refuses to erase uncertain recovery. Minimal
request/digest/job records survive visible-history deletion to prevent replay;
the redundant admission prompt/configuration is cleared after job binding.
This API implementation does not by itself indicate completion of the profile
library, Chat integration or native-platform qualification.

## Knowledge

Knowledge endpoints live under `/v1/documents` and `/v1/rag`. RAG is optional:
clients must read `/v1/rag/status` and present an unavailable state when a local
embedding model has not been configured. Setting `use_knowledge_base` on a chat
request explicitly requires retrieval. It is never silently ignored:

- Authenticated callers need both `chat` and `rag` permission (`403` otherwise).
- Disabled knowledge or a retrieval failure returns `503` before generation.
- Retrieval with no matching evidence returns `422` before generation.
- The caller can configure knowledge, add relevant documents, or explicitly
  retry with `use_knowledge_base: false` for an ordinary model answer.

These rules apply to streaming, non-streaming, and saved-conversation chat.
Successful retrieval supplies untrusted source context to the model; it does
not guarantee that every generated statement is supported. Structured, persisted
citations and source-only evaluation remain planned work.

The knowledge index is currently shared by callers granted RAG access. Session
ownership does not make uploaded documents private to their uploader; separate
collection-level access controls are planned. Do not use one instance as an
isolated multi-tenant document service.

The supported setup workflow is:

- `GET /v1/catalog` to choose an entry with `type: embedding`.
- `POST /v1/models/download` with its stable `model_id`, repository, and file.
- `GET /v1/models/download/progress` until that model is complete.
- `POST /v1/rag/enable` with the installed embedding model ID.
- `POST /v1/documents/ingest` to retain and index a source.
- `POST /v1/documents/reindex` when a retained source needs rebuilding.

Activation metadata is persisted even before the first document is indexed.

If knowledge storage cannot initialize, the server keeps ordinary chat available
but rejects knowledge operations with `503`. It does not substitute a temporary
in-memory index or acknowledge imports it cannot persist. `/v1/rag/status` stays
available and reports `enabled: false`, `stats.storage_available: false`, and an
actionable error. Repair disk/access/database problems and restart; retain a
backup before any database recovery. Do not delete the database as a routine fix.

## Agent profiles (development API)

Profile endpoints use the authenticated actor and current workspace; callers
cannot set ownership in the request. They configure the existing durable runner,
not a separate execution engine. The profile library and linked Chat UI are still
under implementation; these endpoints do not imply those interfaces are shipped.

| Resource | Purpose |
| --- | --- |
| `GET/POST /api/v2/agent-profiles` | List active owned profiles or create revision 1 |
| `GET/PUT /api/v2/agent-profiles/{id}` | Read or save an immutable revision |
| `GET /api/v2/agent-profiles/{id}?revision=N` | Inspect an owned historical revision |
| `POST /api/v2/agent-profiles/{id}/archive` | Archive using `expected_revision`; preserve historical work |
| `GET /api/v2/agent-profiles/resources` | Authorized tool identities, locations and computer eligibility; no credentials |
| `GET /api/v2/agent-profiles/{id}/effective` | Resource readiness, pinned identities and actionable issues |

Create with `{"settings":{"name":"Research","model":"INSTALLED_MODEL_ID",
"instructions":"Keep answers concise.","max_iterations":12}}`. An edit sends
the complete `settings` and `expected_revision`. Conflicts return `409`; missing
resources return `422` with `ready:false` and `issues` without inventing a saved
revision. Foreign profiles return `404`. Include `archived=true` in listing to
inspect archived profiles. See OpenAPI for limits and optional resource fields.

Tools reference `provider`, `tool`, `schema_revision` and `location` from the
authorized resources response. Knowledge references are document IDs. Legacy
workspace-admin connections remain administrator-owned, not personal credentials.
Instructions cannot confer access, install a model or change execution policy.
Ordinary Chat actors can inspect and control their own profile jobs, but not
legacy administrator tasks or another actor's work. They cannot attach computer
sessions or reconnect host access. Calculator/time tools need no external grant;
service filesystem, network and administrator-owned MCP tools remain restricted
until an applicable actor-use grant exists. Discovery and dispatch recheck
current actor authority, including disablement or role changes, independently
of the pinned profile and an older action approval.

The system identity advertises `agent-profiles-v1` and, when session coordination
is wired, `agent-conversations-v1`. Clients must check the latter before sending
additive profile fields to an older generation endpoint; never silently fall
back to ordinary Chat if the selected contract is unsupported.

Submit a profile job with `profile_id`, `profile_revision`, `prompt` and
`request_id` at `/api/v2/jobs`. Its installed model is selected from the revision;
an explicit conflicting model is rejected. Admission pins instructions, Knowledge,
tool identities and child-profile revisions. Later edits do not alter accepted
work. An identical retry observes that same job even if the profile was archived
after admission. Current tool authority is checked again before dispatch.

Profile turns also pin a document-name inventory for the effective Knowledge
selection and the attached agent names, roles and revisions. Model context keeps
this inventory distinct from relevance-ranked excerpts and persona/history.
An empty search result leaves the authorized inventory available but supplies no
content evidence; missing documents, unavailable Knowledge and retrieval errors
still reject admission. Plain Chat's existing retrieval contract is unchanged.

`delegates` selects up to four direct child profiles owned by the same actor.
Children cannot delegate or receive computer control. Their Knowledge and reviewed
low-risk read tools must be subsets of the parent's selections. Child iteration
reservations count against the parent's total budget; they are not extra budgets.
The delegation schema exposes only the effective child tool intersection. A
Knowledge-only child must submit an empty `tools` array and receives its retrieved
excerpts as bounded context. When a prompt explicitly names two attached
specialists, admission requires one complete delegation graph with the requested
dependency edges before the parent can answer. A completed dependent artifact is
forwarded with its headings, citations and stated length preserved; a parent
summary that drops those requirements is not a verified completion.

## Durable jobs and computer access

Desktop application discovery uses `computer_applications` followed by
`computer_find_applications` with a non-empty `query` and the catalog's saved
`catalog_step`. Lookup searches the full saved catalog, including omitted model
context entries and OS-registered aliases. Results include `resolution` (`unique`,
`ambiguous`, or `not_found`), `total_matches` and optional `next_offset`, with at
most 16 entries per page. Exact display-name/alias matches outrank fragments;
duplicate exact names remain ambiguous. IDs are host-issued, not executable paths.

Application launch must match the latest unique lookup. An unrelated ID receives
`computer_application_lookup_required` before dispatch, with bounded corrective
feedback. Catalog refresh, host-session replacement/revocation, a failed newer
lookup or a user correction invalidates the selection. Old saved tasks also need
a fresh lookup before a new launch. Already-dispatched launches retain their
verification/reconciliation boundary; this change never retries them. Name lookup
checks selection consistency, not whether the model perfectly understood prose.

First-party tasks use `/api/v2/jobs`, not a separate history or runner:

| Resource | Purpose |
| --- | --- |
| `GET/POST /api/v2/jobs` | List owner-scoped history or submit a saved task |
| `GET /api/v2/jobs/{id}` | Authoritative snapshot and event cursor |
| `GET /api/v2/jobs/{id}/events` | Ordered persisted activity and snapshot SSE |
| `POST /api/v2/jobs/{id}/input` | Attach independently consented computer access to a pending request |
| `POST /api/v2/jobs/{id}/{action}` | Approve, deny, pause, resume, cancel, take over, steer, reconnect, or reconcile as supported by the contract |
| `GET /api/v2/jobs/{id}/export` | Evidence export |
| `GET /api/v2/jobs/{id}/artifact` | Owner-authorized artifact download; parameters are defined in OpenAPI |
| `DELETE /api/v2/jobs/{id}` | Delete eligible history; not cancellation or undo |

Submit with an installed model ID and a caller-generated request ID:

```bash
curl http://127.0.0.1:11611/api/v2/jobs \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Calculate 25 * 47","model":"YOUR_MODEL_ID","request_id":"docs-example-0001"}'
```

This Bash example assumes loopback unauthenticated mode; protected services also
need authentication. Use a new request ID for new work. Reuse it only when
retrying the identical submission. Actor-scoped identical retries return the
original task; conflicting reuse returns `409`. A `202` acknowledges persistence,
not completion. Follow the returned `run_id` and `Location`.

Trusted built-in argument or path rejection before filesystem entry is recorded
as a rejected proposal with `effects: "none"`. The model has at most three
corrective attempts; unrelated reads cannot reset this budget. Exhaustion pauses
with the specific rejection, not a manual unknown-outcome requirement. This does
not cover failures after directory creation, partial writes, or remote/native
dispatch: those retain conservative uncertainty and are never automatically
repeated. Provider error text cannot declare its own call safe to retry.

Successful, typed native mutation results are journaled in the canonical task
before the authenticated companion reply is acknowledged. Recovery binds the
receipt to the actor, task, session, call, arguments and pinned capability.
If acknowledgment or checkpoint handoff is lost, a matching saved receipt becomes
an action step with `provenance: "companion_receipt"`; the job stays interrupted
with `computer_result_recovered` until explicit resume. This does not certify
the whole task, renew desktop consent or dispatch the action again. Missing,
conflicting, failed or uncertain receipts leave uncertainty unresolved. A screen
that merely resembles success is not proof that an external submission occurred.

Manual reconciliation preserves the original call arguments and records
`steps[].provenance: "human_reported"`. It does not independently verify declared
outcomes. The owning actor and step timestamp identify the report; entering
`done` cannot verify all task checks. Resume remains explicit.

Submissions can include `computer_approval_mode`: `ask_every_time`,
`scoped_changes`, or `full_task`. It is part of the request payload identity and
is returned as `requested_computer_approval_mode`. This records the user's
proposed policy, not a host grant. Attaching a locally consented session with a
different policy returns `409 computer_policy_mismatch` before reservation or
dispatch. Omit the field to preserve legacy local policy selection.

Task snapshots and history can report `outcome: "answer_only"` when a task-first
model returned text without executed external tool actions. Legacy status stays
`completed` for compatibility; that status alone is not proof of external task
completion. Absence of `outcome` makes no new verification claim. The current UI
labels answer-only work **Answered** and retains the text for review.

Computer setup is a durable interruption: `waiting_for_input` and
`pending_input` direct the user to local consent. Resolving that input attaches
an existing owner-scoped session; it does not grant OS permissions or invent a
target. The desktop bridge controls local selection/consent. See
[Computer Tasks](../guides/computer-tasks.md).

### Activity replay

Connect to `/api/v2/jobs/{id}/events` using `Last-Event-ID` to resume after a
persisted sequence. Keep IDs as decimal strings, not floating-point numbers.
The stream delivers `activity`, `snapshot`, and, when the cursor is expired or
ahead, an explicit `snapshot_recovery` with the current `event_cursor`.

Replace local state from that recovery snapshot; never resubmit the task.
The current retention window is 256 activity metadata entries per task; completed
snapshots remain. Persisted events precede delivery and slow/disconnected readers
do not authorize duplicate execution. Disconnect does not cancel a job. Terminal
or input-waiting snapshots may close the stream; reconnect after a lifecycle action.

Saved chat streaming has different semantics: the conversation section above
does **not** promise this job replay protocol.

### Approval and recovery

Approve/deny requests identify the exact pending `approval_id`. The durable
approval binds actor, run, invocation, canonical arguments, capability, and
expiry. Display `canonical_arguments` without lossy numeric conversion.
Stale, duplicate, or conflicting actions fail rather than resubmitting a prompt.
Computer session policies independently govern which typed actions may be
automatically approved; a client-provided blanket grant is not accepted.

A mutating call with no durable result may be `uncertain`. Inspect the target
and reconcile the exact `call_id` with an independently verified result.
Reconciliation records an outcome; it does not execute the action again.
Known read-only failures do not automatically imply uncertain side effects.
See [agent recovery](../guides/agents.md#restart-and-uncertain-outcomes).

Agent snapshots and replay events use transactional
`OFFGRID_DATA_DIR/agent-state.sqlite`. Legacy `agent_tasks` JSON files are
validated/imported with originals retained; they are not the current write
store. Corruption or failed persistence blocks task operations. Preserve the
workspace and follow [backup/migration recovery](../advanced/workspace-recovery.md).

### Compatibility routes

`/v1/agents/run`, `/v1/agents/tasks`, and their lifecycle endpoints remain for
older explicit-run clients. They reuse the durable runner. The old
`stream: true` run response is not a substitute for reconnectable v2 job events.
New clients should use the versioned job contract.

The implemented computer resources include `/api/v2/computer/capabilities`,
`/status`, `/sessions`, `/sessions/stop`, `/model-check`, `/stop`, and the
advanced `/pairing` endpoint. Check methods and fields in OpenAPI rather than
assuming the entire roadmap resource set exists. Public raw `/v1/computer`
mutation routes are retired; they do not bypass the job/consent flow.

Computer-model admission waits through the configured model-load/queue deadline
(five minutes by default), followed by a separate three-minute, two-call smoke
check. Cold loading does not consume the smoke-check budget. Optional managed
browser vision checking has its own 90-second bound. Current web clients allow
ten minutes for model checking and saved-job access attachment; deployments with
longer custom model-load limits need a corresponding client/proxy deadline.
These checks use synthetic observations and never dispatch computer actions.
For a saved task awaiting computer access, the harness reserves the check's
maximum 384 output tokens and three minutes against that task's durable local
inference budget before the check starts. An installed optional vision check
reserves another 128 tokens and 90 seconds when it runs. Stop cancels a check
in flight; interrupted or repeated checks do not regain their reservation.
If the remaining root budget cannot cover a required check, attachment stops
before model inference or computer input. An optional vision check can be
skipped while structured controls remain available. Standalone legacy checks
made before a job exists retain their own independent time/output limits.

Transient saved-job attachment failures return `503` with `retryable: true` and
retain the same pending input; incompatible tool output returns `422`.
Actionable codes distinguish `computer_model_admission_unavailable`,
`computer_model_check_timeout`, `computer_model_check_cancelled`, and other
`computer_model_check_unavailable` runtime failures. A timeout is not evidence
that a model lacks tool support. Retry the saved input, not a duplicate task.

## Tools and MCP

These administrator-managed surfaces configure the running service:

- `GET/PATCH /v1/agents/tools`: inspect/update enabled tools.
- `GET/POST /v1/agents/mcp`: list/save connections.
- `POST /v1/agents/mcp/test`: test a supported HTTP endpoint.
- `DELETE /v1/agents/mcp?name=URL_ENCODED_NAME`: remove a saved connection,
  including one currently offline.
- `GET /v1/capabilities`: inspect broader service capabilities.

A successful connection persists configuration; deletion persists removal
before detaching the runtime and tools. Failure is reported without claiming
removal. Neither operation erases remote data or undoes a request already sent.
See [MCP setup and troubleshooting](../guides/mcp.md).

Treat external tools and their responses as untrusted. Local model inference
does not keep tool arguments local when an external endpoint is selected.

## Experimental speech compatibility endpoints

For the user-facing workflow and model prerequisites, see [Voice](../guides/voice.md).

`GET /v1/audio/status` and `GET /v1/audio/models` include `profiles` for managed
speech packages alongside the legacy fields. Each profile provides `id`,
`revision`, `name`, `architecture`, `adapter`, `capabilities`, `languages`,
`voices`, `available`, `smoke_tested`, and an actionable `issue` when unavailable.
These are service runtime capabilities, not client microphone permission or
qualification. Streaming capabilities from a model manifest are not advertised
as implemented by these file/chunk endpoints.

`POST /v1/audio/transcriptions` accepts multipart `file`, optional `model`,
`language`, and `response_format`. The request body is limited to 25 MiB.
`POST /v1/audio/speech` accepts `input`, optional `model`, `voice`, `speed`, and
`response_format`. WAV is the only currently encoded output; another requested
format returns `422 unsupported_audio_format`, never WAV labelled as MP3.
Qwen currently accepts speed 1; unsupported values fail explicitly.
The Qwen ASR adapter currently fails when the API language code `en` is explicitly
passed. Omit the optional language field to use automatic detection; the UI
dictation path does not set that override. This is a known adapter limitation,
not evidence that all language overrides or profiles are qualified.

Managed model selection accepts the installed ID, `id@revision`, exact display
name, or source repository ID. Omitted models select an available profile for
the requested capability; `whisper-1` and `tts-1` are default-capability aliases.
An explicit incompatible package fails instead of falling back to another model.
Legacy local model names continue through their matching legacy engine.

Web voice controls send `id@revision` for the selected managed model on each
transcription and synthesis request. A response pins one model/voice across its
chunks. Managed Piper `voice` values are validated numeric speaker IDs from the
selected package, not alternate model paths; the legacy voice-name path remains
compatible. Client preferences are browser-local, not a new server preference API.

Requests are bounded to five minutes including the speech queue. Disconnecting
cancels an owned managed inference process; no microphone data is replayed.
Request completion is not proof of transcription accuracy or acceptable voice
quality. Runtime packs, Talk WebSockets, and language/hardware qualification
remain separate delivery gates.

## Error handling

### Owned agent context resources

`agent-context-resources-v1` adds authenticated Chat-permission APIs. Ownership is
always derived from the authenticated actor and current workspace; requests
cannot nominate another owner or a service filesystem path.

- `GET/POST /api/v2/agent-skills` lists metadata or imports explicit files. Each
  file has a portable relative `path` and base64 `content` bytes. Bounds: 64 files,
  64 KiB each, 512 KiB total; UTF-8 `SKILL.md` is limited to 32 KiB.
- `GET /api/v2/agent-skills/{id}?revision=N&digest=SHA256` inspects the exact owned
  revision. `PUT` with `expected_revision` and `files` adds a revision without
  changing prior bytes. No automatic attachment or code execution occurs.
- `POST /api/v2/agent-skills/{id}/review` with `expected_revision` and `digest`
  copies an inspected draft into a new reviewed ID. Explicit profile attachment
  is still required. `ProfileSettings.skills` pins `{id, revision, digest}`.
- `GET /api/v2/agent-memory/{profile}` returns live consent and owned entries for
  user inspection, even with automatic use disabled. `PUT .../consent` requires
  `expected_revision` and explicit `enabled`; re-enable never authorizes old jobs.
- `POST /api/v2/agent-memory/{profile}` performs an explicit user edit using a
  stable `operation`, `expected_revision`, `label`, `content` and optional `id`.
  It does not enable automatic use. Entry CAS and payload-bound operation IDs
  reject stale updates or changed replays. Model tools instead derive provenance
  and operation identity from the canonical root job/call and recheck live consent.
- `DELETE /api/v2/agent-memory/{profile}/{id}?revision=N` clears live text and
  cached write responses. Historical task evidence/backups are not erased.

Context operations do not grant host, desktop, MCP or outbound-data authority.
Unknown fields and duplicate JSON fields are rejected. Foreign resources return
404, stale revisions/operation identities 409, stale model consent 403 and
unavailable storage 503. Old profiles/tasks acquire neither skills nor memory
consent automatically. See OpenAPI for additive schemas and exact response types.

New reviewed skill revisions may declare optional `response_contract` frontmatter
with `version: 1`, a `pattern` (RE2, at most 2,048 UTF-8 bytes) and `max_bytes`
(1–32,768). The pattern matches the complete answer, not a substring. Import
rejects unknown contract fields/versions, invalid patterns and excessive limits.
The parsed contract is exposed on skill/bundle metadata and pinned with the skill
revision. Old stored revisions with no reviewed contract metadata stay inert,
even if their original frontmatter used the same previously unknown key.

After a matching skill is successfully loaded, the runner checks its contract
after existing effect verification. A mismatch allows at most two persisted,
answer-only corrections within the original job budget. Tools are unavailable
and the dispatch boundary also rejects calls during correction. Exhaustion returns
`skill_response_contract_limit`; malformed pinned metadata returns
`skill_response_contract_invalid`. Earlier effects remain saved and are never
replayed to repair an answer. Draft streaming is provisional; only a validated
answer can become the completed result. A format pass does not verify factual
accuracy or grant permission to perform the checklist or workflow it describes.
The contract applies to the whole final answer once loaded, including clarification
or limitation answers. Reviewers must make the skill's instructions and pattern
consistent with those possibilities. It is not a conditional instruction inferred
from model prose. Canonical skill receipts retain the full file; the model-facing
view of a contracted `SKILL.md` presents its instruction body without validator
frontmatter. Supporting files and legacy skill views retain their complete content.

For v2 job errors, use the structured `error.code`, `message`, `retryable`,
and `request_id` envelope with its HTTP status. Older routes can retain their
legacy error shape; consult the contract and support both only where required.
Do not parse human-readable messages as machine codes.

A retryable transport/storage error is not permission to rerun an uncertain
side effect. Read the saved job or retry its identical submission with the
original request ID. Keep server logs private; omit tokens and private tool
data from issue reports.

## Owned calendar schedules (preview)

Services advertising `agent-schedules-v1` expose actor/workspace-owned Chat APIs:

- `GET /api/v2/agent-schedules` lists reviewed policy projections.
- `POST /api/v2/agent-schedules` creates a policy without creating a job.
- `GET /api/v2/agent-schedules/{id}?revision=N` inspects a current/historical revision.
- `PUT /api/v2/agent-schedules/{id}` performs a full live-resource review with
  `expected_revision`; stale writers return 409.
- `POST /api/v2/agent-schedules/{id}/disable` takes `expected_revision`, revokes
  dispatch first and stops only owned roots without concealing uncertain effects.
- `GET /api/v2/agent-schedules/{id}/occurrences` returns up to 128 most recent
  occurrence records, reasons and canonical `job_id` links; it cannot replay work.

`AgentScheduleWrite` contains `settings` (name, prompt, enabled and typed IANA
calendar), exact `profile_id`/`profile_revision`, optional approved `execution`
reference and bounded `max_iterations`. Creation uses absent/zero
`expected_revision`. Unknown/duplicate fields and caller-supplied actor, task
config, grant creation, due-time overrides or run-now flags are rejected.
Responses omit private runner context and runtime review epochs. Policy revisions
and occurrence identities remain durable; no credentials are stored in schedules.
Jobs expose additive `schedule` provenance. Existing APIs and ordinary jobs keep
their current behavior. See OpenAPI and the agent guide for concrete schemas and
missed/DST/revocation/overlap semantics. This is not an OS wake-up service.

## Changing the contract

After editing `pkg/api/openapi.yaml`:

```bash
cd apps/web
npm run api:generate
npm run api:check
npm run check
```

Commit the contract, generated types, handler implementation, and tests
together. CI rejects generated-client drift.
