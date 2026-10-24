## Purpose

Provide predictable, extensible setup and use of external agents backed by OffGrid inference while preserving user configuration and reporting actual readiness.

## ADDED Requirements

### Requirement: Independent scoped host outcome verification
Explicit outcome contracts SHALL be verified by shared host code independently of assistant claims and external process exit. The verifier SHALL NOT execute tools, rewrite results or infer exact-copy intent from ordinary prose. Normal runs without an outcome contract SHALL retain their behavior.

#### Scenario: Explicit per-run contract
- **WHEN** a managed foreground run supplies `--verify-contract <file.json>`
- **THEN** strict bounded contract validation and supported enabled-hook inspection precede launch, execution uses the declared workspace without changing saved defaults, and failed/unverified checks return a nonzero outcome even after external zero exit

#### Scenario: Hook authentication and cancellation
- **WHEN** a hook uses a wrong token/session, the endpoint is unavailable, or the owned invocation is cancelled
- **THEN** no passing evidence is accepted, reviewed file operations are vetoed on hook transport failure, credentials/content remain out of reports, and endpoint cleanup never replays work

#### Scenario: Contracted OpenClaw launch
- **WHEN** a supported OpenClaw run explicitly supplies a verification contract
- **THEN** it uses the supported hook-initialized local launch with private session state, the declared workspace and selected model, preserving configured tools and permissions through a read-only configuration overlay; saved config/sessions and uncontracted launch behavior remain unchanged

#### Scenario: Launch-path evidence
- **WHEN** dispatcher tests pass but an actual managed launch omits lifecycle hooks
- **THEN** the contract remains unverified, no alternate execution is replayed automatically, and support is not claimed until the actual installed launch passes independent lifecycle and file checks

#### Scenario: Exact copy with decorated source display
- **WHEN** a contract requires exact source/destination bytes and a proposed write includes a display prefix or a serialized read command
- **THEN** the verifier rejects the mismatching proposal without changing its arguments, retains immutable expected source evidence, and cannot report completion until independent destination read-back matches

#### Scenario: Missing or conflicting hook evidence
- **WHEN** a hook is absent, a call identity is reused inconsistently, a write completion is unknown, or the invocation is cancelled
- **THEN** the outcome remains unverified or failed even if the external process exits zero or claims success; no automatic mutation retry is performed

#### Scenario: Scope and preservation
- **WHEN** a declared workspace contains escaped paths, links, special files, excessive content, a changed source or unexpected final mutations
- **THEN** verification fails closed without reading unrelated files, rewriting user files or granting external tool authority

#### Scenario: Bounded evidence, not a sandbox
- **WHEN** explicit file checks pass with correlated supported hooks and independent read-back
- **THEN** the receipt reports only the contracted checks; it does not certify arbitrary agent prose, unobserved transient effects, other workspaces or general workflow qualification

### Requirement: Shared managed lifecycle
Supported integrations SHALL use the same lifecycle and generic command surface with runtime-specific adapters. Existing aliases and provider-only setup SHALL remain compatible.

#### Scenario: Another trusted adapter
- **WHEN** a third fixture adapter is registered
- **THEN** generic inspect, configure, test and run work without integration-specific manager or UI branches

### Requirement: Model selection and safe context
Integration selection SHALL use installed model metadata and explicit user choices. Per-run model overrides SHALL preserve saved defaults. Configured context SHALL not exceed admitted capacity; greater available capacity SHALL not force reinstallation.

#### Scenario: Capacity grows
- **WHEN** an external agent has a compatible 32768-token context and OffGrid admits 65536
- **THEN** preflight accepts the conservative configured context

#### Scenario: Capacity shrinks
- **WHEN** admitted capacity is below the external agent's configured context
- **THEN** preflight reports insufficient capacity without inflating the allocation, reinstalling or switching models

#### Scenario: Battery transition
- **WHEN** default power policy changes from AC to ordinary, low, critical or unknown battery
- **THEN** power is informational, context remains subject to the same model and memory limits, and no automatic battery-based restriction is introduced

#### Scenario: Heap occupancy is not host pressure
- **WHEN** the Go heap has high occupancy but the OS reports sufficient available memory
- **THEN** degradation does not reduce context based on the heap ratio; actual pressure limits new-work admission without rewriting context, and failed measurements do not imply recovery

#### Scenario: Compatible resident allocation
- **WHEN** a smaller-context request follows a larger allocation for the same healthy runtime
- **THEN** it reuses the allocation without unloading or reducing the advertised capacity; growth remains exclusive and runtime capacity is independently confirmed

### Requirement: Usable interactive launch
Interactive external runtimes SHALL receive foreground terminal input and signals. OffGrid SHALL restore terminal ownership/settings after exit or failed start and report the terminal state truthfully. Noninteractive cancellation SHALL remain bounded to the owned invocation.

#### Scenario: Interactive input and cancellation
- **WHEN** the user launches Hermes from a terminal, enters text, then interrupts or exits
- **THEN** the child receives input without job-control suspension, the original terminal owner is restored, and OffGrid does not report a stopped child as running

### Requirement: Explicit structured response support
Supported response-format constraints SHALL be validated and forwarded, not silently ignored. Unsupported providers or incompatible tool modes SHALL reject them before generation.

#### Scenario: Auxiliary JSON response
- **WHEN** Hermes requests a supported JSON schema response through llama.cpp
- **THEN** OffGrid forwards that schema to the backend without substituting unconstrained text or another provider

### Requirement: Scoped truthful readiness
Readiness SHALL distinguish service compatibility, host runtime, plugin, configuration, smoke evidence and workflow qualification. Unmeasured stages SHALL be explicit. Status SHALL perform no installation or inference.

#### Scenario: Service-only inspection
- **WHEN** the browser queries integrations from a container service
- **THEN** it identifies service/model compatibility and leaves host and workflow readiness unverified

#### Scenario: Operational failure
- **WHEN** service admission is busy or unavailable
- **THEN** diagnostics identify the operational condition without declaring model incompatibility or prescribing reinstallation

### Requirement: Preserving configuration
Managed configuration SHALL preserve unrelated entries and locally modified bundles, record recovery evidence before changing configuration and verify the applied result. Partial failures SHALL not be reported as configured.

#### Scenario: Repeated configuration
- **WHEN** an installed integration is configured twice with the same model
- **THEN** user-added model entries and settings survive and the second configuration is idempotent

#### Scenario: Partial or concurrent write
- **WHEN** configuration fails midway or changes concurrently
- **THEN** the operation reports its stage and recoverable snapshot, does not blindly overwrite newer state and reinspects before retry

### Requirement: Authenticated and bounded integration operations
All integration service requests SHALL use the shared authenticated transport. Executable plugin/runtime installation SHALL require the explicit install workflow. External tool authority SHALL remain with the external runtime.

#### Scenario: Authenticated service
- **WHEN** setup, status or model discovery uses a service API key
- **THEN** the key is sent only to the selected service and is absent from status, journals and generated setup responses

### Requirement: Evidence for external workflows
Transport smoke and full agent workflow verification SHALL be separate and tied to the tested model/runtime/configuration. Qualification SHALL not be inferred from installation or configuration success.

#### Scenario: Model reply only
- **WHEN** an external runtime returns a valid model response
- **THEN** smoke verification can pass while tool workflow qualification remains unverified

#### Scenario: Changed configuration
- **WHEN** the selected model, endpoint, runtime, plugin or relevant configuration changes
- **THEN** prior smoke evidence is not reported as current

### Requirement: Bounded local streaming and operational outcomes
Accepted streamed inference SHALL emit a protocol-compatible start event before
slow admission/loading. Progress SHALL be truthful and distinct from generated
content. Work SHALL remain bounded and cancellable; malformed or partial tool
decisions SHALL never become executable through progress reporting.

#### Scenario: Slow first output
- **WHEN** a valid local request waits for loading or prompt processing longer than the external runtime's first-event watchdog
- **THEN** stream startup arrives promptly, progress contains no invented answer or tool arguments, and an explicit finite execution budget still bounds the request

#### Scenario: Cancelled or expired request
- **WHEN** the client disconnects or its execution deadline expires during queueing, loading or generation
- **THEN** cancellation reaches the owned inference request, reporting distinguishes the operational outcome from incompatibility, and no success or automatic replay is emitted

#### Scenario: Invalid request
- **WHEN** authentication, model selection or input validation fails
- **THEN** the existing HTTP error is returned before accepting the stream

### Requirement: Request-local context preservation
Integration latency optimizations SHALL preserve configured instructions, tool
capabilities, authority, model selection and saved defaults. Runtime-specific
controls SHALL be scoped to the invocation and supported by the installed runtime.

#### Scenario: Compact local execution
- **WHEN** an integration uses compact tool discovery or a local execution timeout
- **THEN** available tools remain discoverable, user configuration is unchanged, and neither context capacity nor qualification is inflated

### Requirement: Model-neutral external decision transport
External requests SHALL retain the selected model's native tool transport unless
an explicitly selected supported adapter requires otherwise. Sequential tool
cardinality SHALL NOT implicitly select the private governed-agent protocol.
Complete tool identity, argument schema, finish and cardinality validation SHALL
precede release of executable tool arguments. Ordinary prose SHALL never execute.

#### Scenario: Sequential external tool request
- **WHEN** Hermes, OpenClaw or another client requests one tool at a time
- **THEN** native tools, history and template semantics are preserved, a valid native call is returned, and multiple or malformed calls are rejected without replay

#### Scenario: Truncated or invalid tool stream
- **WHEN** a tool stream is cancelled, truncated, malformed or references an unavailable tool
- **THEN** no executable tool arguments or successful terminal event escape validation

#### Scenario: Governed compatibility
- **WHEN** an internal governed workflow or its probe selects the constrained adapter
- **THEN** it retains the existing single-step protocol, matching token accounting and independent dispatch protections

### Requirement: Cancellation-safe shared preparation
Preparation needed by live requests SHALL NOT be repeatedly restarted merely
because another request cancels. Scheduling SHALL retain exclusive model changes,
memory admission, bounded preparation and prompt cancellation of each waiter.

#### Scenario: Auxiliary cancellation during loading
- **WHEN** a short auxiliary request cancels while a live request needs the same compatible model preparation
- **THEN** the auxiliary caller exits, the live request can reuse preparation without duplicate loading, and the cancelled caller never generates

#### Scenario: No remaining demand
- **WHEN** every requester for in-progress preparation cancels
- **THEN** preparation is cancelled and cleaned up before another exclusive switch proceeds

### Requirement: Pinned tool-template compatibility
An explicitly supported compatibility adapter MAY repair a known template's tool-only framing. Selection SHALL use verified runtime/template identity, not model display names or client-provided executable templates. User instructions, model selection, available tools, native call/result semantics and validation SHALL remain intact. Token counting and generation SHALL use the same adapter.

#### Scenario: Conversation with tools available
- **WHEN** a matching adapter handles an ordinary greeting with optional tools present
- **THEN** its template allows a normal answer without a call, does not replace the user message with a function-call task, and does not synthesize the answer or hide tools

#### Scenario: Required call and result
- **WHEN** an adapted request requires a registered tool and later supplies its multilingual result
- **THEN** complete validated calls and correlated result history survive both stream modes; independent outcome checks, not assistant prose, determine workflow success

#### Scenario: Unknown or changed identity
- **WHEN** the template or runtime is not positively matched, or the active model changes
- **THEN** no compatibility override is guessed or reused from the previous model; the existing native path remains unchanged

#### Scenario: Rejected decision
- **WHEN** an adapted response is malformed, truncated, unregistered or incompatible with requested cardinality
- **THEN** the existing decision gate rejects it without releasing executable fragments or replaying work

### Requirement: Comparable per-model workflow evidence
Compatibility and latency evidence SHALL identify model/runtime and cold versus
warm state, distinguish first transport event from first generated content, and
cover both supported external runtimes without hardcoded model identities.

#### Scenario: Another installed model
- **WHEN** an eligible installed chat model is selected
- **THEN** it uses the same integration contract, with measured reply/tool results and explicit unsupported, failed or unverified states instead of inheriting another model's qualification

### Requirement: Faithful tool-result interpretation evidence
A supported semantic-framing revision SHALL preserve original tool results and user instructions, use explicit pinned selection and distinct evidence identity, and remain unqualified until independent outcome checks pass. Guidance SHALL distinguish display formatting, actual content and operation receipts without silently modifying external actions.

#### Scenario: Decorated and genuinely numbered text
- **WHEN** a documented read result adds a line-number display prefix to Unicode or source text that itself begins with a numeric prefix
- **THEN** qualification requires preserving the actual source bytes, including genuine prefixes, while no transport layer strips or rewrites tool results or generated write arguments

#### Scenario: Truncation or contradictory verification
- **WHEN** observed content is truncated or destination read-back differs despite a successful write receipt
- **THEN** the qualification oracle rejects a full-completion claim; the adapter guidance requires further authorized evidence or an explicit incomplete/mismatch report instead of fabricated success

#### Scenario: Revision preservation
- **WHEN** the semantic revision is not explicitly selected or template/runtime identity is unknown
- **THEN** existing native/v1 behavior remains unchanged, and prior qualification is not attributed to the new revision

### Requirement: Native sequential generation and preserved provider controls
The bundled native parser SHALL respect sequential cardinality during generation as well as final validation. Reviewed build patches SHALL be pinned and reject unexpected source identity. First-party provider hooks SHALL default tool requests to sequential without overriding explicit caller controls or removing capabilities.

#### Scenario: Wrapped native calls with parallel disabled
- **WHEN** a native per-call JSON format receives `parallel_tool_calls: false`
- **THEN** its grammar/parser admits no more than one call, including optional and required tool modes, while independent validation still rejects malformed or truncated decisions

#### Scenario: Explicit parallel policy
- **WHEN** a caller explicitly enables, disables or suppresses a parallel-call parameter
- **THEN** supported provider hooks preserve that control, native tool schemas and correlated history; no private governed protocol is selected implicitly

#### Scenario: Runtime patch mismatch
- **WHEN** a runtime build uses an unexpected source revision or patch preimage
- **THEN** the patch step fails without accepting a partially patched or falsely identified runtime

#### Scenario: Dependent file operations and greeting
- **WHEN** an isolated external runtime is tested with a greeting and a source-to-destination Unicode copy
- **THEN** qualification requires a normal greeting without file changes and dependent calls using actual results, exact immutable expected bytes, preserved source and no unexpected workspace mutations; zero exit or assistant success prose alone is insufficient
