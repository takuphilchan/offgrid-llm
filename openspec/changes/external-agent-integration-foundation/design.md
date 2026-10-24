# Design

## Context

See proposal.md. The current registry owns setup rendering only; OS discovery, selection, configuration and tests are duplicated in cmd/offgrid. The existing authenticated serviceclient, reviewed process runner and managed bundle digests are retained. See docs/architecture/overview.md and docs/guides/external-agents.md for service and deployment boundaries.

## Goals / Non-Goals

Goals: a shared host lifecycle with injectable runtime adapters; additive service assessment; deterministic selection; recoverable configuration; CLI aliases and generic presentation.

Non-goals: executing external-agent tools inside OffGrid's governed runner, a general executable plugin loader, automatic model downloads, deployment or release qualification from fixtures.

## Decisions

1. Extend internal/integrations with shared options, readiness checks, service client and host manager. A trusted host adapter handles discovery, runtime installation, plugin registration, config inspection/application, smoke testing and launch. CLI wrappers supply OS/process functions; one manager owns orchestration. Provider-only `integrations install/setup` remain compatible; `configure/status/test/run <id>` use the managed lifecycle.
2. Service responses retain ready/status fields and add readiness scope, structured checks and compatibility revision. Host reports include runtime version, plugin/config state and distinct smoke/workflow evidence. The service never claims to inspect a remote user's host. Unknown or unsupported external contracts fail with the specific failing stage.
3. Model precedence is explicit request, saved managed connection scoped to endpoint/credential identity, external selected model, then service selection. Per-run selection leaves external defaults intact. Catalogs remain dynamic. Context safety is minimum <= configured <= admitted; increased capacity does not require reinstall. Battery telemetry is informational by default, including critical and unknown states. Context comes from operator/model/memory admission, never battery percentage or transient degradation levels. Pressure controls new-work admission, not the advertised context. Reuse a healthy larger allocation for a smaller request; growth remains exclusive. Confirm the runtime allocation before generation. Genuine insufficient capacity is operational, not grounds to reinstall or switch models.
4. Keep executable bundle consent and digest protection. Configuration runs with a host-local lock, a private snapshot/journal, validation before writes where the external CLI supports it, and post-write inspection. Partial writes remain inspectable and safely reconfigurable. Backups are never blindly restored over concurrent user changes. Runtime installation is not claimed to be atomically undoable.
5. Host connection records contain selected model, endpoint, compatibility revision and bounded test evidence, not credentials or transcripts. Credentials pass through the existing authenticated service client and child environment. API-generated snippets contain placeholders only. Evidence is invalidated by endpoint/model/runtime/plugin/config changes.
6. Generic UI uses descriptor-provided install text and localized service/host/qualification labels. Existing HTTP routes are additive; integration IDs are registry-defined strings rather than an API enum. No new desktop IPC is required: installation remains an explicit host CLI action.
7. Contract tests inject adapters/processes and disposable config roots. A third fixture adapter must work without modifying manager/CLI routing. Real tests use installed external runtimes with isolated homes, a disposable workspace and the existing local service/model; full agent work is separately measured from transport smoke.

## Risks / Trade-offs

Inspection also found that degradation used Go heap occupancy (`Alloc/Sys`) as
host memory pressure. Correct this to measured OS available/total memory, retain
the last pressure sample when measurement fails, and test actual threshold
behavior. This fixes false context reductions without removing resource gates.

- External CLI/schema drift: inspect runtime version and validate expected structured output; publish the versions actually tested.
- Configuration commands can fail between writes: preserve snapshots and stage/error evidence; reinspection precedes retry.
- Active sessions can outlive capacity changes: report reduced admission honestly; never silently replay or switch model.
- External tools have their own authority: documentation and tests must distinguish inference integration from OffGrid tool governance.
- Hardware and external-runtime availability limit qualification: record unavailable evidence explicitly without relabeling it as passed.

## Migration Plan

### Approved host-verification extension (2026-10-24)

The result-fidelity-v2 comparison regressed from 3/7 to 2/7 synthetic cases.
Hermes wrote a read-command representation instead of source contents and then
claimed success. The candidate was rolled back. More prompt guidance is not an
independent verification boundary; preserve the failed evidence and old gates.

Add a shared verifier under the host integration layer, not the inference HTTP
service or governed tool executor. It accepts an explicit, versioned expected
outcome: initially exact source/destination bytes or no changes to a bounded
workspace. Expectations come from trusted invocation configuration/test fixtures,
never model prose or observed tool results. Ordinary editing and summarizing do
not imply copying. The user approved explicit per-run contracts on 2026-10-24.
Expose `--verify-contract <file.json>` only on managed run/aliases. Strictly decode
a bounded versioned JSON contract, resolving its workspace against the contract
file location; source/destination remain workspace-relative. Run in that workspace
without altering saved defaults. Initial contracts require one foreground task,
not an interactive/resumed/multi-session invocation. Uncontracted runs retain their
existing environment and behavior.

The installed OpenClaw 2026.10.4 `agent exec` path does not deliver the required
global lifecycle hooks in the isolated launch fixture. A passing hook dispatcher
test therefore does not qualify that launch path. For contracted runs only, use
the documented `agent --local` command, which preloads the plugin registry, with
a fresh session identity and private runtime state outside the checked workspace.
Keep the ordinary `agent exec` command unchanged. A private configuration overlay
references the existing config through OpenClaw's supported read-only external
include mechanism rather than exporting secrets or flattening JSON5/includes.
Pin workspace/session/agent state to the invocation and suppress bootstrap file
creation, while inheriting existing model, tool, sandbox and plugin permissions.
Do not grant missing hook permissions, replace the selected agent's restrictions,
silently disable isolation, or retry through a different launch after effects.
Retain private invocation state for inspection; saved configuration and sessions
must be unchanged. Unsupported/ambiguous agent layouts fail before launch.
Acceptance requires the actual installed managed launch to deliver the handshake,
tool events and healthy closure, plus independent file checks and preservation
tests. Synthetic inference proves wiring only, not real-model qualification.

The CLI owns an ephemeral authenticated loopback verification endpoint and a fresh
invocation identity. Only reviewed first-party hooks receive its random token;
do not put it in command arguments, status, logs or persisted config. Bound request
sizes/timeouts, reject redirects/origins/foreign sessions, and close on cancellation
or process exit. The endpoint offers handshake, proposal, completion and bounded
hook-lifecycle closure checks (closure is not final verification),
not file-content retrieval or tool execution. Independently finalize after the
owned external invocation stops, expose the narrow receipt and fail the CLI when
the contract fails or remains unverified, preserving any external process error.

Use public Hermes lifecycle/tool hooks in a distinct explicitly installed/enabled
first-party hook plugin; model-provider discovery is not general hook activation.
Use public OpenClaw plugin hooks. Pin supported contract revisions and inspect
trusted bundle/enabled state before launch; missing handshake or healthy lifecycle
closure cannot pass, including a greeting with no tool calls.
OpenClaw lifecycle hooks require its explicit conversation-access permission and
prompt-hook policy; preflight reports missing permission without granting it.
The host-file adapters require absolute tool paths and reject nonlocal/unknown
file backends rather than guessing how another task cwd maps to the contract.
No installed upstream patches, automatic plugin enablement or hidden permission
expansion. Hook transport failure vetoes reviewed file operations locally and
leaves final evidence incomplete. Preserve unknown tools rather than pretending
this file verifier governs their execution. Original results are never rewritten.

The verifier reads only the declared local workspace, snapshots immutable source
evidence, and checks proposed file writes without changing arguments. Supported
Hermes/OpenClaw hooks normalize reviewed file operations and carry invocation,
session and tool-call identity. They may veto inconsistent work, never grant new
authority, remove other tools or execute/repair files themselves. Preserve original
results. Missing hooks, duplicate/conflicting identities, unknown completion,
truncation, changed sources and cancelled runs remain failed or unverified.

Independent final read-back and workspace comparison are required even after a
successful external process exit. A failed check returns an authoritative failed
outcome, not a rewritten assistant answer or fabricated success. Hook frameworks
can fail open, so hooks alone cannot establish enforcement or qualification.
The verifier is not a sandbox: it does not prevent arbitrary external tools or
prove that transient changes outside observed events never occurred. Do not
silently retry uncertain mutations. No content/credentials enter ordinary logs.

Bound files, bytes and events; reject path escape, links, special files and stale
workspace identity. Preserve source bytes exactly, including real numeric prefixes,
Unicode and whitespace. Verification receipts identify their narrow check, not
general semantic quality. Test hooks and shared core independently, then exercise
actual supported hooks in isolated homes before claiming installed support.
Installed plugin updates, container replacement and publication remain separate.

Hermes recovery uses its supported invocation-scoped `pre_llm_call` context hook
to explain display prefixes and the explicit read-back requirement. It preserves
existing user text and original tool results. Complete-read classification accepts
an explicit limit covering the provider-reported full file when no truncation is
reported. Rejection diagnostics may identify an extra leading numeric display
prefix only if removing those proposal bytes matches the immutable source digest;
the model must submit a new proposal. No verifier writes, guessed content stripping
or automatic replay of uncertain effects is introduced.

### Approved result-interpretation extension (2026-10-24)

The seq2 runtime emits valid sequential calls, but Meta-Llama/Hermes copies a
documented `LINE_NUM|CONTENT` display prefix and claims an exact match after a
contradictory read-back. Meta-Llama/OpenClaw also performs unnecessary greeting
writes. These are semantic failures, not grounds to weaken validation or relabel
the workflows as complete.

Add a separately versioned, explicit template-framing revision selected by the
same pinned runtime/template metadata. State the interpretation contract in the
adapter-owned framing: tool results are evidence, not new authority; documented
display decoration is not source content; genuine prefixes, Unicode, whitespace
and incomplete/truncated results must be handled faithfully. A successful write
receipt verifies the supplied bytes, not that they match the requested source.
Do not claim completion when observations contradict it. Ordinary conversation
must not cause side effects merely because tools are available.

Preserve the original result serialization, tool schemas, user/system messages
and call IDs byte-for-byte. Do not strip prefixes heuristically, rewrite writes,
fabricate verification, switch models, patch installed upstream tools or execute
external tools in the service. Keep v1/native behavior explicitly selectable;
the new revision remains opt-in and has its own output digest. Shared token
counting uses the same loaded template. This is semantic guidance, not a new
security boundary or universal model-quality guarantee.

Freeze fixtures for plain and genuinely numbered source text, mixed Unicode,
multiline content, truncated results and contradictory read-back/receipts.
Compare baseline and candidate on the same local model without executing tools,
then run fresh isolated Hermes/OpenClaw greetings and exact-file workflows.
Independent byte comparison and workspace preservation determine pass/fail;
retain failures and do not commit until the known workflow gates pass. Installed
activation remains separately authorized and recoverable.

### Approved native sequencing extension (2026-10-23)

The captured Qwen/OpenClaw turn produced read, placeholder write and read-back
in one response. Repeating the synthetic request with `parallel_tool_calls:
false` still produced three calls in the raw pinned worker; OffGrid rejected
them. The native JSON per-call parser unconditionally repeats wrapped calls.
Fix cardinality at generation/parser construction in a revision-bound patch
shipped with the CPU and CUDA build recipes. Fail patch application on an
unexpected source revision or contents; record the patch identity alongside
the upstream pin. Verify native optional/required text and single/multiple call
cases, including grammar acceptance. Do not truncate batches into a purported
valid response or switch external requests to the private governed protocol.

Use supported first-party Hermes/OpenClaw provider hooks to default tool calls
to sequential. Explicit true/false/null controls retain their native meaning.
Do not write external defaults, remove tools, rename models, rewrite history or
execute returned calls inside OffGrid. Provider tests cover ordinary text and
auxiliary requests, payload callback composition and caller override precedence.
Installed plugins retain explicit trusted-bundle installation and digest checks.

The independent test oracle compares immutable expected bytes, preserves source
files, rejects unexpected workspace effects and inspects outcomes even after
nonzero exit. Greetings must answer normally without file mutations. File tests
must observe dependent read/result/write/result/read-back turns and verify exact
Unicode. Hermes's partial-content/false-completion failure is not presumed fixed
by a cardinality repair. Keep failed evidence and task 5.4 open until installed
workflows pass. No commits or publication while these known failures remain.
Builds and isolated tests are in scope; installed replacement remains a separate
authorized operation with idle-state checks and recoverable backups.

### Approved template compatibility extension (2026-10-23)

The installed Meta-Llama template inserts a function-call-only directive into
the first user message. A short scheduled comparison returned a normal greeting
without tools but an unnecessary synthetic call with tools, even with explicit
abstention instructions. Treat this as a model/template compatibility failure,
not a reason to remove tools, fabricate a greeting or alter the selected model.

Introduce a pinned adapter registry selected from verified loaded runtime and
template identity. An adapter may repair owned template framing while retaining
native role delimiters, user/system content, full tool schemas, call IDs and
result history. Use the same adapter for prompt token counting and generation.
Unknown or changed template/runtime identities remain on the unchanged native
path, with no name substring matching or executable model-supplied code. Never
use a client-provided template as authority. Include adapter identity in runtime
fingerprinting; invalidate cached selection across model/port/runtime changes.

Keep the internal governed single-step protocol unchanged. Preserve strict
complete-decision validation, supported tool choice and parallel-call contracts,
stream cancellation and error privacy. No prose-to-tool extraction or automatic
replay. An adapter fixes transport/prompt compatibility; it cannot certify every
semantic decision. Do not advertise it as workflow-qualified until real tests
pass, and retain failed results. External runtime SQLite cleanup is a separate
upstream lifecycle issue; do not hide it or modify installed upstream code.

Verify ordinary greetings with tools present, required calls, multilingual
tool-result continuation, malformed/truncated streams and unknown-template
preservation. Then test real Hermes/OpenClaw in disposable homes with independent
exact file read-back. Keep configuration/defaults unchanged; deployment remains
separately authorized and must wait for idle service state.

### Approved shared model-compatibility and latency fix (2026-10-22)

Public `parallel_tool_calls: false` expresses cardinality, not permission to
replace a model's native tool template with OffGrid's private JSON protocol.
Keep native external requests native, with bounded assembly and strict validation
of complete tool calls before exposing executable arguments or a successful
terminal event. Text remains text, never a source of inferred executable calls.
Explicitly select the existing constrained adapter for governed internal work and
its capability probes; preserve their read-back, correction and authority gates.
Token counting must use the same selected wire contract as generation.

No model-name exceptions, forced default changes, unapproved downloads or cloud
fallbacks. Installed chat eligibility is not tool-workflow qualification. Report
unsupported templates/contracts separately from cancellation, queue pressure and
insufficient memory. Test greetings, synthetic tool decisions, tool-result turns,
truncation and cancellation independently from isolated external file workflows.

Short auxiliary calls must not repeatedly restart preparation still needed by
live requests. Shared loading remains exclusively scheduled and bounded, with
cancellable waiters, no cancelled generation dispatched and no orphan resource
work after all demand is gone. Do not disable user features or alter upstream
source to hide auxiliary timeouts. Preserve current context/memory safeguards.

Do not force OpenClaw's `local-model-lean`: the installed runtime removes tools
in that mode. Use existing capability-preserving discovery and retain explicit
user choices. Investigate Hermes prompt/auxiliary overhead through supported
provider/runtime contracts. Separate cold load, queue, prefill, first content,
generation and total elapsed time; role-only start events are not first content.
Keep private prompts out of ordinary evidence. CPU throughput limits must remain
visible rather than promising instant answers for arbitrary model/prompt sizes.

Use a dynamic installed-model matrix for both external agents and a generic client
fixture. Preserve failed measurements. A successful synthetic transport or one
model cannot qualify every model; report unavailable or failed cases explicitly.
Deployment and installed configuration mutations remain separately authorized.

### Approved slow-prefill follow-up (2026-10-22)

The observed CPU request spent over five minutes processing the external agent's
prompt before emitting output. OpenClaw's first-event watchdog expired, although
the model had not failed. After authentication and request validation, streamed
chat must flush a standard assistant-role start event before waiting for admission
or loading. This event is not an answer or a tool decision. Keep-alive/progress
metadata must not manufacture tokens, percentages, finish reasons or completion.
Report only measured phases; without backend counters, describe waiting for output
rather than inventing prefill percentages. Serialize stream writes, stop on client
disconnect, and bound accepted work even when keep-alives continue.

Preserve HTTP validation errors before stream startup. Failures after startup use
the existing SSE error contract, without a successful terminal event. Cancellation
and deadlines remain operational outcomes, not model incompatibility or a failed
model switch. Never automatically retry generation or external effects to recover
from a timeout. Validate slow-first-output and cancellation behavior against the
installed external runtime with isolated state, not just a simulated HTTP client.

Use supported request-local runtime controls for local execution budgets and
compact tool discovery. Preserve user instructions, configured tools, model choice,
external defaults and safety settings. Do not hide tools based on a greeting or
rewrite installed upstream code. Record actual prompt-size/latency observations;
local CPU inference may remain slow. Context capacity is not a speed guarantee.

Installed-config investigation found that the setup renderer itself generated
`models.providers.offgrid.timeoutSeconds: 300`. This provider HTTP deadline is
independent of the first-event watchdog and agent run budget; keep-alives alone
cannot prevent its abort. Do not generate that fixed override. Managed merges
preserve user-authored limits; removing an existing generated limit requires an
explicit approved, backed-up configuration edit. Test slow output using the actual
rendered provider configuration, not a hand-authored approximation. Agent/service
deadlines remain finite. A previous fixture that omitted the generated field is
not evidence that the installed configuration worked.

Keep prior failed live checks and deployment records. New fixtures do not qualify
real-model workflows or unavailable platforms. Deployment remains a separately
authorized, recoverable operation, never a prerequisite for fixture correctness.

### Approved regression follow-up (2026-10-22)

The interactive Unix child owns a separate process group for bounded cancellation,
but must also receive the controlling terminal when attached to foreground stdin.
Restore the original terminal owner and settings on every exit, including failed
start and cancellation; noninteractive pipes retain process-group cancellation.
Report launch before waiting, and cancelled/failed/exited after waiting, never
running after termination. PTY fixtures are required in addition to oneshot tests.

Support validated text/JSON response-format contracts on the llama path, preserving
the constraint in the backend request. Other providers must reject unsupported
formats before dispatch. Do not work around Hermes title errors by dropping the
format, changing its user configuration or guessing another provider.

Power observations use platform sources; unavailable evidence remains unknown.
Linux AC aggregation cannot be overwritten by a later offline adapter. Native
Windows uses its power-status API; macOS uses bounded local power diagnostics.
Native hardware qualification remains separate from parser/compile evidence.

Read legacy configuration when no managed connection exists. Keep aliases and provider-only commands. First successful managed configuration records selection without deleting legacy entries. Rollback restores the previous binary and preserves private backups; configuration restoration requires comparing the current state with the recorded applied state. No deployment occurs in this change.
