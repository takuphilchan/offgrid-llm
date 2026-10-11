# Design

## Context

See proposal.md. BrowserHub, NativeSession and the Go worker bind protocol-v2 to a single TargetIdentity. NativeSupervisor already journals before input and independently revokes dispatch. Agent CreateRequest currently starts inference immediately; TaskAccess discovers windows only after a model-requested interruption. The manual Electron launcher is not an agent tool and must not become an unjournaled bypass.

## Goals / Non-Goals

**Goals:** Task consent separate from execution targets; real typed application launching and switching; retain exact observations, prepared actions, immutable policies, duplicate tombstones and local Stop.

**Non-Goals:** Removing OS permission prompts or existing prohibited-action rules; treating any application as perfectly sandboxed; implementing other independent stages of the approved program in this change.

## Decisions

- Negotiate protocol 3 explicitly. Represent desktop authority with a host-issued desktop/session identity and a task/actor/companion binding. Each prepared execution step still binds the actual process generation/surface. V2 compares exact targets; v3 compares the task authority and signed-in OS session, then validates each actual target locally. Never infer desktop scope from Full task policy.
- Extend the existing private worker and journal rather than exposing a new listener or letting Electron execute model commands. Use typed discovery, launch and focus operations with host catalog identities. Restrict launch to validated registrations supported by reviewed activation providers, verify executable identity before dispatch, and discover a matching usable surface with a deadline. Never dispatch a path or arguments supplied by a model. Report unsupported activation forms honestly.
- NativeSupervisor remains the final authority for both launch and window input. A task grant lasts at most 60 minutes/500 mutations; v2 retains its existing limits. Local Stop revokes independently of network/provider progress. Already dispatched launches cannot be undone or retried on an uncertain acknowledgment.
- Add an explicit desktop scope to task submission. Persist initial waiting-for-access state atomically with task creation, without synthesizing a model tool call. Consent attaches to that exact input/task identity and releases the same job. Repeated attachment is idempotent; absent/older hosts cannot silently fall back to window grants. Existing clients omitting scope retain their flow.
- The desktop composer requests task access at Start when desktop scope is selected. Navigation only observes the existing request. Existing pending app requests can explicitly obtain a new desktop grant, but a previously granted v2 window is never widened. Keep optional limited window mode clearly named.
- Service discovery composes desktop tools with existing permitted MCP tools. App-open verification is a separate typed read-back of a host-discovered surface and executable identity, not a successful spawn or title substring. Other mutations retain their existing content/read-back completion rules.
- Tests inject catalogs/launchers and native fixtures, not production consent bypasses. Hardware/platform and model qualification are recorded separately in the existing reliability plan.

## Risks / Trade-offs

- Desktop consent exposes more ordinary surfaces → explicit local task consent, immutable actor/task/session scope, single input lease, protected process/control checks and emergency stop remain mandatory.
- Launch catalogs include interpreters and installers → reviewed activation providers validate registrations and reject unsupported arguments, executable types and known unsafe helpers rather than executing arbitrary shortcuts/commands. Missing coverage is an actionable limitation, not silent shell fallback.
- Existing process reuse → verify a fresh matching surface rather than requiring a new PID; process generation remains checked for subsequent operations.
- Stale observations after switching → invalidate prepared actions/views, require new observation, and never reuse an approval across targets.
- Protocol/package skew → explicit v3 capability and matching pack checks, fail with upgrade guidance rather than misleading picker fallback.

## Migration Plan

### Approved session-access refinement (2026-10-10)

An explicit Enable action obtains native Full access consent before task start.
A long-lived owned private worker retains only an in-memory service/workspace/
actor authorization. Each task still receives a distinct bound grant with its
existing time/action budget and journal. Completing or failing one task clears
its execution state before another can attach; it does not repeat consent.
The worker never accepts a renderer-provided `approved` flag. Its local Stop,
OS session events, pipe closure and host quit revoke the authorization. Main
also revalidates the authenticated actor and workspace and revokes on changes.
No grant is serialized to preferences or restored after restart. Older clients
retain explicit per-task consent. Switching to narrower access revokes the
remembered full-access authorization. Desktop-owned Windows workers have no
additional Stop popup: the existing app banner, File menu and tray stop through
local IPC even offline. A hidden native monitor retains the independent emergency
shortcut and OS-session revocation; renderer loss/unresponsiveness revokes control.
Standalone workers retain their visible fallback. Internal popover scrolling must
not trigger placement recalculation; content/viewport changes still do.

Additive task/session fields preserve old records and idempotency hashes when omitted. Keep v1 managed-browser and v2 selected-window handlers. Build matching service and desktop packages together; test in isolated state before any separately authorized replacement. Existing installed data and sessions are not migrated into wider grants. Rollback requires ending new grants, never restoring the host dispatch journal.

### Runtime and consent regression repairs (2026-10-10)

Cold/queued model admission uses the existing lifecycle deadline before a separate
three-minute two-call smoke check begins. Transient admission/probe failures keep
the saved access request pending and return retryable 503 errors with phase-specific
guidance, not a fabricated tool incompatibility. Clients budget for default load,
probe and optional vision checking. Cancellation releases the lease; no host tool
is dispatched during these synthetic checks.

Windows host consent is an owned native OffGrid dialog with real accessible
controls, monochrome typography/actions, bounded high-DPI layout and a scrollable
read-only summary. The initial confirmation is compact (440 by 330 logical pixels,
clamped to the work area), rather than a page-sized permission document. Session
scope, time/action limits, a sensitive-action restrictions summary and revocation
remain visible; complete exclusions and technical workspace identifiers expand
within the same dialog. Cancel is the
initial/default action. It replaces presentation,
not authority: no renderer approval flag, saved consent or additional listener.
The local Windows app theme/high-contrast palette is used; renderer-specific theme
overrides and localization of the existing English native consent are not added
by this repair. Current UI pages and host protocol remain unchanged.

### Edge discovery and continuation repair

The reported Edge task had no Edge in its worker's catalog despite the installed
executable being present; a fresh read-only controller discovers it. Discovery
must refresh the reviewed paths, keeping opaque identities only while the file
identity/content metadata still match. Replacements receive new identities and
invalidate prepared actions. No model path or shell lookup is added.

Context compaction must not rewrite the stable first system message on every
iteration: it invalidates prompt-prefix reuse. Archive references belong after
the stable messages. Pending application launches remain explicit durable facts
through compaction; another open of the same app must be rejected before dispatch
until verification, with bounded corrective feedback rather than an endless loop.

### Native observation context representation (historical repair, superseded below)

The subsequent Edge report reached verified application readiness, then a 46-control
observation exhausted the allocated 8192-token prompt budget. Keep exact host JSON
in durable checkpoints and steps, but project known native observations into a
lossless column-oriented model view before estimating and sending context. Do not
summarize away controls, text, state, IDs or limits. Unknown protocol fields retain
the original representation. Projection is not new authority and never changes
host observations, prepared actions or completion evidence. Count model-facing
content and call schemas, not the additional escaping of the HTTP envelope, while
retaining template/output headroom and reported-usage calibration. The selected
runtime's optional input-token-count endpoint measures the full request (including
tool schemas and chat-template options) under a model lifecycle lease, without
generation or tool dispatch. Use that count rather than rejecting on a byte
estimate. Older runtimes explicitly lacking the endpoint keep the conservative
fallback; other count failures stop with runtime guidance. Never count a transient
browser image as text-only. Task-local ordinal archive aliases keep the prompt
index bounded while preserving append-only evidence and legacy digest lookup.
No Qwen-specific ratio, template or context increase. Oversized instructions or
genuinely unfit observations still stop without dispatch; do not
silently raise runtime allocation, truncate current evidence or replay prior work.

### Slow inference and retained Windows controls

The native 15-second observation lifetime is shorter than measured CPU inference.
Do not simply raise it. Before a new protocol-v3 Windows preparation/readback,
an optional host adapter re-observes exactly retained, host-issued evidence. Only
an unchanged complete view, ordered runtime identities/state digest, control
properties and exact process/session/surface permit mapping retained element IDs
to the fresh capture. New observation identity/time is used internally; existing
preparations are cleared. Changed/limited/foreign views fail closed. Dispatch
preconditions, policy, consent and replay protection remain independent. V2 and
other adapters retain strict expiry until they implement equivalent revalidation.
This applies to structured controls, not pointer/vision input or expired approvals.

### Approved progressive context and general application follow-up

The real Edge trace still overflowed after lossless projection: large URL values
dominated the newest observation, which archival cannot remove. Canonical steps
remain exact; the model view must instead bound text and controls, explicitly mark
omissions, and offer actor/task-scoped paged retrieval of saved observations.
Retrieval neither refreshes a capture nor grants dispatch authority. Host expiry,
revocation and target checks remain mandatory. Small observations and legacy
selected-window clients retain their representation. Short references resolve
only through the current task's canonical evidence, before durable authorization.
Unknown protocol fields must not be silently discarded.

Derive verified application progress from durable steps across compaction and
restart. Repeated launch proposals must inspect current application state rather
than dispatch another launch; a subsequent read-only failed readiness check can
establish that reopening is needed. An application verification observation is a
real observation, not a reason to require a redundant full tree. Launch evidence
does not establish a compound task's completion.

Replace name-specific Windows paths with OS-backed discovery in independently
tested providers. Resolve registrations into host-owned executable/package
identities; reject shell commands, interpreter/installer shortcuts, arbitrary
arguments, links/replacements and protected targets. Preserve unchanged IDs and
deduplicate by file identity. Discovery, launchability and workflow qualification
are separate claims. Other platform activation providers and broad control remain
explicitly pending until implemented and verified; no silent v2 scope widening.

Deployment follows the user's explicit request: back up installed state, build
matching service/UI/host, replace rather than run a second OffGrid service, verify
health and packaged identities, and preserve rollback. Never replay personal
tasks, restore grants, or claim real workflow/latency qualification from mocks.
