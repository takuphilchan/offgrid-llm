# Design

## Context

See proposal.md. Current jobs have no execution reference; profile/task filtering intentionally blocks shell. `list_files` uses service cwd. The CLI default loop submits independent ten-step jobs, while `session chat` already supports linked canonical turns. Preserve the boundaries described in docs/advanced/client-contracts.md and docs/guides/agents.md.

## Goals / Non-Goals

Deliver a real selected-folder and process runtime through the existing durable runner. A worker is an operation provider, not a new planner/task store. This change does not claim universal model qualification or implement memory/schedules inside the execution layer. The dependent skills-memory and automation changes remain required for the approved program.

## Decisions

1. Put reusable host contracts and operations in `internal/execution`; ship `cmd/offgrid-executor`. Contexts pin version, actor, service workspace, host, backend, approved root, cwd and grant/revision. Provider operations require a binding and never trust model-supplied identity alone. Paths belong to one provider; do not infer mappings between WSL, Windows and containers.
2. Implement file access with Go rooted filesystem handles, bounded pages, revision checks and regular-file checks rather than prefix-only validation. Creation uses exclusive creation. Edits check a preimage, stage and read back; document that atomic replacement is not an OS-level compare-and-swap against noncooperating external writers.
3. Use supervised stdio framing for an owned child and enrolled authenticated transport for a long-lived host. Protocol fields include version, request/job/operation IDs and grant binding; bounded frames and strict decoding reject unknown/truncated requests. No renderer receives a generic spawn/exec IPC method. Reuse local enrollment and pack verification patterns without changing computer grant semantics.
4. Journal command intent before dispatch and result before acknowledgment. Operation IDs are bound to canonical payload digests. Replay of an executing entry returns uncertainty, never starts a replacement. Worker lifecycle and process handle state do not replace service job checkpoints.
5. Trusted local processes run with disclosed user-account authority, explicit cwd and minimal environment. Keyword filtering is not confinement. Default timeout is 180 seconds within aggregate root budget; local cancellation closes new dispatch and terminates only owned trees. Isolated Linux execution uses explicit mounts, no Docker socket exposure to the service and honest network configuration.
6. Add execution-context references to jobs/turns and effective profile capabilities. Existing records have no execution grant. Runtime discovery and dispatch share eligibility checks; the old `shell` and renamed scripts remain blocked. Profiles may narrow, not mint, grants. Research/MCP/computer providers remain independently authorized.
7. Consolidate default interactive agent chat with existing conversation service functions; no separate CLI planner. Native clients use consistent profile/model/workspace/resume/toolset controls. Keep legacy one-shot and lifecycle adapters. Show effective host/folder/tool readiness in the existing composer; preserve calm layout and nine locales.
8. Broader dependencies: scoped automatic opt-in memory, revisioned Agent Skills and occurrence scheduling use the same jobs, grants and inference scheduler. They are tracked separately rather than embedding unreviewed code or another executor. Schedule grants never restore desktop consent; missed occurrences are recorded, not replayed.

## Risks / Trade-offs

### Conversation-first workspace admission

New interactive submissions advertise only that a trusted client can offer a
local workspace, not a folder grant. The reviewed request tool may pause the
canonical job for read/edit/trusted access; model arguments cannot choose an
arbitrary host path. The client resolves its captured launch folder or an explicit
picker, establishes the existing authenticated executor grant and attaches its
reference through an idempotent input-resolution boundary. Revalidate actor,
service workspace, pinned profile ceiling and requested permissions. Never replay
the original model call or create a replacement root. Cancellation, declined
access and lost acknowledgments retain the same saved job and bounded recovery.
JSON/detached callers receive explicit pending state, never an interactive prompt.

A CLI conversation retains its executor under the conversation lifetime, not the
individual turn's observation context. Completing one turn cancels only that
turn's watcher. Conversation exit/cancellation and explicit permission upgrades
still close the owned host; subsequent turns reuse the same valid grant until
then. Test actual context cancellation, not only retention of a handle pointer.

Keep optional remembered trust in client-owned local configuration outside the
repository, bound to canonical folder identity, host, service identity, actor,
backend and permission ceiling. A new host session still receives a fresh grant;
remembered trust is local authorization to establish it, not a reusable transport
credential. Do not inherit approval into nested folders, broader permissions,
changed roots/services/accounts or desktop input. Provide inspection/revocation;
corrupt/unknown records fail closed. Existing session approvals are not imported.

Capabilities are described from the effective catalog with attached versus
requestable resources distinguished. Keep the current user objective visible
through tool-result turns; recovery instructions are internal guidance, not an
answer or a new task. No English greeting classifier, prompt-specific canned
answers, stripped user instructions or unrestricted prose-to-action fallback.
Ordinary creative and explanatory responses remain available with tools attached.

Repair a file precondition only after the matching persisted host receipt proves
rejection before effects. Preserve the exact-input tool's source selectors while
the model obtains a current revision; a handwritten replacement is not proof of
that selected span. Repairs consume the existing root limit. A running process
may request bounded further observation before completion, never implicit restart.
Unknown effects and unrelated failures retain existing interruption behavior.
When permissions increase on the same approved root, persist the old/new reference
transition so earlier receipts remain verifiable. Historical references are
evidence only and cannot authorize new operations.

CLI progress defaults to concise state and observed actions, without duplicate
run headers, prompt-preparation chatter or replaying the same preview as a final
answer. Preserve machine-readable output, actionable approvals, failure states,
Stop and inspection identities. Apps use the same pending workspace request,
effective capability context and existing activity disclosure in all nine locales.

### Interactive one-shot command refinement

The observed installed CLI still submits an asynchronous service-filesystem job
for `agent run "list files in this directory"`. A terminal run should instead
offer the current local folder once through the existing execution supervisor,
then stream the same canonical job to its result. Add matching explicit folder/
access options. Detect terminal input/output through an injected process-owned
predicate, never by reading ambient OS state in reusable CLI code. JSON, piped
and unattended invocations preserve their existing asynchronous semantics unless
they explicitly select waiting. `--detach` makes background intent explicit and
cannot outlive an invocation-owned folder lease; select an independently approved
session for detached work. Declined/unavailable local approval must never become
an implicit service-directory listing. Preserve exact submission/request identity,
stream failure recovery and unknown-effect protections; cancellation must stop
the canonical job before closing owned execution authority, without replay.

Help prioritizes interactive chat/run examples and explains local workspace,
streaming and detach behavior before secondary saved-run control commands.

New foreground CLI jobs negotiate `agent-response-profiles-v1` and explicitly
select interactive context, matching Chat's existing operator-configured
allocation. `--response-profile extended` selects the eligible model limit.
Omitted API fields and older services preserve their previous behavior; JSON and
detached submissions are not automatically changed. Persist the selected profile
and allocation at admission, include explicit selection in idempotency identity,
and reject unsupported/invalid selections before submission. Do not change
external-agent context, shrink a resumed job, silently discard history or switch
models. Existing context accounting/compaction and bounds remain authoritative.
Running-process model guidance prefers a bounded wait over repeated immediate
polls; original operation schemas, receipts and cancellation remain unchanged.

- Local commands can access what the OS account permits: label trusted mode honestly; use actual isolation for untrusted code.
- Host disconnect after dispatch: reconcile journal evidence, preserve unknown effects and never retry via another backend/model/child.
- Foreign concurrent file writers: use preconditions/read-back and do not promise exclusive access or universal exactly-once effects.
- Tool/frame/output growth: strict envelopes, bounded pages and retained output references; do not re-execute to recover output.
- Worker/schema changes: negotiated version/pack identity and explicit unavailable state, not permissive fallback.

## Migration Plan

### Typed writing from user input

Add `workspace_write_from_input` as a separately reviewed tool for new jobs with
the existing workspace write ceiling. Keep `workspace_write` and its descriptor
unchanged. The new tool selects `request` or an already applied correction ID
from this canonical root, plus unique nonempty `after`/`before` strings. Pending
corrections, earlier conversation turns, child/tool/document text and foreign
roots are not input sources. This is literal selection, not English-keyword
authorization or an intent parser. The model still selects the intended span;
mechanical equality alone does not certify the whole task's meaning.

Resolve boundaries without trimming or Unicode/newline normalization. Reject
missing, repeated, reversed or invalid boundaries and excess bytes before host
dispatch. Existing safe-boundary steering invalidates undispatched proposals;
applied correction inputs remain immutable and scoped to their root. Tool
presentation offers only those source IDs. No caller-provided job/actor/source
path can select another task or filesystem.

Translate the selected bytes into the existing `file_write` request under the
same host/folder grant and preimage rules. Persist that resolved payload before
dispatch; the intent validator checks it against the selected immutable input.
Return bounded source ID/digests/byte-span provenance only after the original
host receipt proves matching digest, byte count and independent verification.
Post-dispatch mismatch is uncertain, never a no-effect retry or successful
completion. Receipt recovery uses the original pinned payload and checks, never
resolves an uncertain mutation into a replacement invocation. Preserve canonical
evidence and ordinary writing; new tool identity does not expand old job grants.

CLI and apps share this service contract. Reuse existing localized edit activity
and its single technical disclosure, with no new consent surface or required
composer syntax. Check literal Unicode/newlines, ambiguous sources, correction
scope, tampered receipts and crash/recovery automatically. Repeat real
Meta-Llama/Qwen disposable-folder workflows separately; preserve failed evidence
and keep broader/hardware qualification open until actually established.

Additive fields/tables only; no new tools, roots or grants on old profiles. Capability-gate new clients. Package matching workers without changing installed state in implementation. Deployment requires separate authorization, backups, one active service and old/new worker compatibility checks. Keep real Windows/macOS/Linux/model qualification unchecked until actually run.

### Follow-up context fidelity

Present the pinned workspace's current read/write/process permissions separately
from requestable access. Explain upgrades only when the effective catalog offers
the access-request tool; presentation cannot widen grants. Attachment is setup,
not evidence of the requested file or command outcome. A conversation-owned
executor survives individual turn watchers and is closed with the conversation.

The single-step inference projection uses the same decision envelope for prior
assistant answers and prior tool calls. Encode ordinary assistant text literally
inside `message`, including JSON-looking strings; never parse prose into actions.
Keep canonical history, user messages, dispatch validation and uncertain-effect
protections unchanged. Real mixed conversation/read/edit/command tests remain
required: schema consistency and passing unit tests do not qualify task success.

If completion validation finds a recorded running process in the current
execution session, the harness may schedule a bounded `workspace_process_wait`
observation of that exact handle. Persist it as an ordinary call before dispatch;
recheck current capability identity, profile restrictions, host authority and
cancellation through the existing path. Retain the bounded completion-repair
budget. Never synthesize a start, write or stop, infer a handle from model prose,
observe an older session under a new grant, or accept a proposed final answer
before terminal evidence exists. Native versus constrained response formats can
be compared in explicitly enabled, decision-only/disposable fixtures; production
adapter selection remains unchanged until its own qualification passes. The
approved qualification-gated native selection is owned by
`model-adaptive-execution`; it must preserve the same runner and tool authority.

### Interactive client presentation

The CLI remains an adapter to canonical conversations. A per-session presentation
context carries the pinned profile and existing style tokens. Typed output markers
select concise session receipts while preserving the real terminal for streaming
and animation; never filter output text to hide errors or approvals.

One command registry drives help and the terminal picker. Local commands inspect
the pinned profile; `/status` observes service state and existing explicit work
commands retain their authority/recovery semantics. Unknown commands are rejected
locally. A synchronous input editor shares the consent reader, restores terminal
mode before dispatch, bounds input, supports Unicode and bracketed paste, and
falls back to line input for pipes, plain output and small terminals. No parallel
stdin consumer, alternate executor, automatic model/profile switch or grant is
introduced. Profile resource views describe configuration, not verified readiness.

Response-format transport must preserve the submitted property order for an
unchanged validated JSON schema: grammar-based runtimes may generate in that
order. Preserve exact numeric constants and reject ambiguous duplicate keys or
malformed Unicode before retaining wire bytes. If an in-process caller mutates
the decoded schema, forward the current value rather than stale cached bytes.
Governed historical calls should use the same discriminator-first ordering as
fresh decisions. Neither projection grants authority or turns prose into calls.
The public test bridge must exercise this boundary rather than claiming that
preserving order in its own proxy alone reproduces production inference.
