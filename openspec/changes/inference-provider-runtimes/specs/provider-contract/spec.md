# Provider contract

## ADDED Requirements

### Requirement: Owner configuration requires explicit reviewed administration
Administrator creation and updates SHALL use one existing owner configuration
and provider registry. Every mutation SHALL validate the exact endpoint identity,
use a bounded actor-bound prepare/commit review, compare the current file digest,
and durably record intent and outcome under the verified owner writer lease.
Uncertain writes SHALL block activation until explicit reconciliation and SHALL
NOT be replayed automatically. Configuration mutation SHALL NOT grant access.

#### Scenario: First endpoint creation and later update
- **WHEN** a current administrator reviews and commits a valid entry in an empty workspace or updates an existing entry
- **THEN** one atomic owner-file publication records the reviewed identity and durable receipt without contacting a provider
- **AND** the entry requires restart installation and a fresh access review before model requests can use it
- **AND** the renderer shows the exact origin, pins, actor scope and activation status without legacy model-file actions

#### Scenario: Credential or managed runtime authority is supplied by a browser
- **WHEN** a draft names a credential outside the installation allowlist, supplies an executable/environment field, or references a changed managed template
- **THEN** preparation or commit rejects it before owner-file publication, endpoint contact or process creation
- **AND** credential values and executable paths are never returned to the renderer

#### Scenario: Concurrent or stale configuration review
- **WHEN** the whole file, installation ceilings, authenticated administrator or review expiry changes before commit
- **THEN** the draft cannot publish and the administrator must prepare a new concrete review
- **AND** a second workspace writer cannot bypass the same external owner-file lease

#### Scenario: Commit outcome is uncertain
- **WHEN** the process stops or durability fails after recording intent and before confirming the receipt
- **THEN** endpoint access remains blocked and the durable record distinguishes the pending outcome from successful activation
- **AND** explicit reconciliation classifies actual bytes as old, committed or conflicting without replaying the configuration write
- **AND** retrying a request identity cannot apply a different draft

#### Scenario: External configuration is not safely writable
- **WHEN** an explicitly selected external owner file lacks installation writer authorization, private ownership or an available writer lease
- **THEN** administrator editing fails without moving the file or broadening filesystem permissions
- **AND** existing readable enrollment keeps its original location and authorization rules

#### Scenario: Historical owner intent is restored
- **WHEN** a backup restores prior owner intent, receipts or pending reviews under a fresh workspace activation epoch
- **THEN** historical records cannot auto-commit or restore endpoint access
- **AND** the current administrator must reconcile actual owner bytes and perform fresh review

### Requirement: Endpoint administration preserves provisioned authority
Endpoint administration SHALL use the existing registry, authorization callback
and scheduler. Its initial rendered review/mutation operations MUST identify a
currently provisioned endpoint and expected configuration/policy revisions;
they MUST NOT accept ad-hoc destinations, credentials or executable authority.
Durable revocation SHALL only subtract from current trusted actor grants.

#### Scenario: Revocation races queued and active requests
- **WHEN** an administrator durably disables an enrolled endpoint
- **THEN** new and queued requests fail their live authorization check before new outbound dispatch
- **AND** admitted HTTP and whole-generation contexts are canceled, including the owned-runtime retirement watcher when a consumer callback is blocked
- **AND** the result does not claim an already dispatched remote prompt or effect was undone, while the existing Stop path remains available

#### Scenario: Restore or missing/corrupt administration state
- **WHEN** a workspace is restored or its endpoint policy cannot establish current reviewed activation
- **THEN** endpoints remain unavailable until explicit current-configuration review; old enabled state cannot silently reverse a later revocation
- **AND** unreadable or corrupt policy fails closed without empty-policy fallback
- **AND** initial explicit review creates a nonempty durable activation marker before grants, while losing an existing marker cannot revert policy to an implicit empty original epoch

#### Scenario: Revocation publication is uncertain
- **WHEN** snapshot writing or durability confirmation fails before or after replacement
- **THEN** admitted leases are canceled and that service blocks further endpoint authority rather than assuming the prior snapshot survived
- **AND** restart inspects the actual persisted snapshot under the existing OS workspace ownership lease; no automatic request replay occurs

#### Scenario: Restored activation metadata differs from historical archive
- **WHEN** the original archive and extracted bytes have been verified
- **THEN** a new random restore epoch is written and synced before publishing the fresh target
- **AND** the original archive remains unchanged, its returned manifest remains historical evidence, and a subsequent backup inventories the new marker bytes

#### Scenario: Re-enable under stale authority
- **WHEN** configuration, endpoint revision, expected policy revision or administrator/actor permissions changed after review
- **THEN** re-enable fails without broadening the trusted file's actor grants

#### Scenario: Explicit connection check
- **WHEN** an authorized operator requests a check for a provisioned identity
- **THEN** only that identity reaches the shared provider scheduler under bounded cancellation, and credential values remain service-side
- **AND** successful connection/model identity checks do not mark workflows, tools, offline operation or hardware as qualified

#### Scenario: Endpoint card maintenance
- **WHEN** Models renders a provisioned endpoint
- **THEN** endpoint administration owns its controls and legacy file verification/deletion never receives the endpoint selector

### Requirement: Offline ONNX embedding identity and bounds
ONNX embedding execution SHALL use explicitly installed graph, tokenizer and
preprocessing artifacts through the existing supervised worker and embedding
owner. Runtime dependencies, tokenizer behavior, tensor names, pooling,
normalization, dimensions and model digest MUST bind the embedding identity.
Unsupported execution providers, external graph data or custom execution MUST
fail closed without network fetch, installation, fallback or silent truncation.

#### Scenario: Token or tensor contract mismatch
- **WHEN** input exceeds the declared token budget or runtime tensors differ from the pinned input/output shape contract
- **THEN** embedding fails before publishing vectors or changing any index
- **AND** source text is never silently truncated to make the request succeed

#### Scenario: Invalid vector or interrupted response
- **WHEN** a worker returns wrong counts/dimensions, non-finite or zero vectors, malformed events, cancellation or an incomplete terminal sequence
- **THEN** no successful embedding batch is returned and uncertain runtime retirement blocks capacity reuse

#### Scenario: Existing Knowledge identity differs
- **WHEN** an ONNX package or its tokenizer/preprocessing/runtime changes after an index was created
- **THEN** existing Knowledge identity checks reject incompatible retrieval until an explicit verified rebuild
- **AND** selecting another provider never silently reindexes installed user data

#### Scenario: New module appears after runtime enumeration
- **WHEN** an ordinary service or worker account can create a new importable module in a runtime directory, change its owner/ACL, or retarget its ancestors
- **THEN** runtime admission is rejected even when every previously enumerated file has a matching digest and a retained read handle
- **AND** a second inventory scan is not treated as proof of immutable import authority

#### Scenario: Restricted Windows installation
- **WHEN** explicitly configured ONNX execution uses the initial Windows provisioned mode
- **THEN** the service verifies administrator-owned read-only runtime namespaces, exact file/dependency pins, constrained Python search paths and a nonprivileged process token before launch
- **AND** unavailable installation/ownership prerequisites remain explicit failures without automatic provisioning or alternate runtimes

#### Scenario: Model bytes change after a package lease is verified
- **WHEN** graph, tokenizer or settings paths change while a trusted worker loads the selected package
- **THEN** the worker compares each actual parser-input byte buffer with the manifest-bound digest supplied by the service
- **AND** changed bytes fail before graph/tokenizer execution, while later path mutations cannot alter an already verified immutable buffer

#### Scenario: Restricted Linux ONNX installation
- **WHEN** explicitly configured ONNX execution uses a root-provisioned Linux runtime
- **THEN** the non-root service verifies immutable runtime/import paths and a pinned namespace launcher before starting the existing supervised process protocol under its owned PID/user namespace
- **AND** failed startup closes pipe handles and retains uncertainty without claiming resource release, while missing provisioning or kernel support fails without a fallback

#### Scenario: Unselected installed package
- **WHEN** a package is imported or runtime configuration is enrolled without selecting that embedding model
- **THEN** no worker starts, default model changes or Knowledge rebuild occurs
- **AND** the reserved package selector cannot route to a colliding legacy chat model

#### Scenario: Package management from Models
- **WHEN** an operator imports, verifies or removes an ONNX package in the Embeddings category
- **THEN** the existing typed package transaction owns its exact ID and revision, and no legacy file mutation receives the compatibility selector
- **AND** first-package import remains available, category mismatches are rejected, global model selection refreshes after successful mutations, and unqualified metadata never becomes runtime readiness

#### Scenario: Valid inputs exceed a combined worker frame
- **WHEN** individually valid embedding inputs exceed the encoded byte limit when grouped together
- **THEN** they are split into whole-input batches bounded by both vector count and exact JSON size
- **AND** oversized individual inputs still fail without truncation or partial successful results

### Requirement: Restricted administrator-provisioned Linux runtimes
Managed Linux HTTP runtimes MUST require a non-root service without process
capabilities, a pinned trusted namespace launcher, immutable-to-service
administrator-provisioned root-owned runtime/model trees and ancestors, and
private writable service state. They MUST NOT provision or change host ownership,
permissions, cgroups or sysctls automatically. Privileged administrator mutation
and trusted base OS components MUST remain explicit limits.

#### Scenario: Writable or substituted runtime artifact
- **WHEN** any runtime/model artifact or ancestor permits non-administrator replacement or modification, is a link, or mismatches its hash
- **THEN** local runtime admission fails before launch without weakening the integrity policy or falling back to another provider

#### Scenario: Listener outside the owned namespace
- **WHEN** the selected loopback tuple cannot be tied through a retained process identity and socket inode to the owned namespace
- **THEN** the connection closes before HTTP bytes are sent and startup retires its owned namespace
- **AND** unavailable procfs or namespace support fails closed without process-group fallback

#### Scenario: Missing administrator provisioning
- **WHEN** a normal user-managed installation cannot meet the restricted mode
- **THEN** the provider reports unavailable and leaves its files and host policy unchanged
- **AND** broader Linux installation support and real-runtime qualification remain incomplete

### Requirement: Providers expose one normalized lifecycle
Every executable provider SHALL support load/start, capability handshake,
streaming completion, cancellation, health, resource metrics, and close through
the internal provider contract.

#### Scenario: Provider startup
- **WHEN** a model is admitted with a compatible provider
- **THEN** the provider returns its protocol, runner, model, adapter, backend,
  and capability identities before generation is allowed

#### Scenario: Provider crash
- **WHEN** a worker exits during load or generation
- **THEN** the task receives an operational runtime failure and no tool action
  is dispatched from incomplete output

### Requirement: Worker boundaries are typed and supervised
Supervised native subprocess workers SHALL use a versioned local protocol with request IDs,
cancellation IDs, model digest, typed stream events, terminal status, and
capability metrics. Workers SHALL NOT receive credentials, desktop authority,
or unrestricted command/file access.
Enrolled HTTP adapters SHALL translate their provider's native wire protocol to
the same internal identity/event boundary; upstream credentials remain in the
service transport and are never exposed to model input or model output.

#### Scenario: Cancellation
- **WHEN** a caller cancels an active generation
- **THEN** the matching request stops without terminating unrelated providers or
  canceling another caller's request

#### Scenario: Unknown protocol
- **WHEN** a worker reports an unsupported protocol or runner revision
- **THEN** activation is rejected as unavailable and no generation starts

#### Scenario: Ambiguous protocol JSON
- **WHEN** a request, handshake, generation event, or health reply contains
  repeated object keys or malformed Unicode
- **THEN** it is rejected before interpretation, and an affected worker with
  uncertain stream alignment is retired before another request

### Requirement: Responses normalize before dispatch
Provider output SHALL normalize into plain text or one complete registered tool
decision with validated arguments. Prose, malformed JSON, unknown tools, and
truncated streams SHALL never execute.

#### Scenario: Truncated tool stream
- **WHEN** a worker disconnects before the terminal event
- **THEN** no tool dispatch occurs and the run records an operational failure

### Requirement: HTTP providers retain endpoint authority and outcome identity
Ollama and vLLM SHALL use the shared scheduler and normalized event gate. Every
request SHALL recheck explicit endpoint authorization, exact model/runtime
identity and credential scope. Redirects, implicit downloads and automatic
fallback SHALL NOT occur. Externally managed resources SHALL NOT be reported as
released local memory when only the HTTP client closed.

#### Scenario: Revoked or changed enrollment
- **WHEN** actor access, endpoint identity or model/runtime pin changes after selection
- **THEN** generation fails before sending prompt data under the stale selection

#### Scenario: Legacy local model collides with an explicit endpoint identity
- **WHEN** a local model record has the same literal ID as an `endpoint:` selection
- **THEN** endpoint authorization and provider routing remain authoritative for chat and canonical jobs
- **AND** missing or revoked enrollment cannot fall back to that local model, tokenizer, projector or cached local lease
- **AND** local-only preparation, completion, embeddings, computer qualification and deletion reject that identity without modifying the local record or files
- **AND** model lists cannot expose a revoked endpoint through the colliding local record

#### Scenario: Authenticated background request outlives HTTP admission
- **WHEN** an endpoint text job continues after its admitting request context ends
- **THEN** provider selection and each request revalidate the persisted task actor against current user-store permissions and enrollment
- **AND** disabled or deleted actors cannot retain a grant through an old request object or become anonymous owners when authentication is disabled

#### Scenario: Truncated external stream
- **WHEN** an HTTP provider disconnects without terminal status and token usage
- **THEN** the partial response is not a completed answer and no tool dispatch occurs

#### Scenario: Excess trailing data after a valid terminal
- **WHEN** NDJSON or SSE exceeds its total byte limit after an otherwise valid terminal response
- **THEN** overflow is an explicit protocol error rather than a synthetic clean EOF
- **AND** no tool decision or successful done event is released

#### Scenario: Redirect or unexpected address
- **WHEN** an endpoint redirects or attempts to dial outside explicitly pinned addresses
- **THEN** the request fails without forwarding credentials or prompt data

#### Scenario: Local external runtime has no ownership lease
- **WHEN** a loopback provider is selected without a service-owned resource lifecycle
- **THEN** admission fails without claiming local memory is safe to reuse

### Requirement: Owned local HTTP runtimes prove startup and release
Local Ollama/vLLM startup SHALL use trusted pinned launch configuration and the
existing scheduler's resource lease. The lease SHALL cover all owned descendants
and the exact local listener. Local runtime launch SHALL neither inherit user
credentials/cache state nor acquire missing models, execute remote model code,
or enable provider cloud routing. Unproven release SHALL block reuse of capacity.

#### Scenario: Actor revoked before local startup
- **WHEN** the actor is removed or loses chat permission before the launch lease is acquired
- **THEN** no local server or descendant is started, even if an earlier request selected it

#### Scenario: Descendant survives the server parent
- **WHEN** an Ollama runner or vLLM worker outlives the server's direct child process
- **THEN** shutdown terminates and verifies the complete owned runtime tree within bounded time
- **AND** a failed proof retains the resource lease and prevents another runtime from claiming the released capacity

#### Scenario: Another process occupies or replaces the endpoint
- **WHEN** a local listener does not belong to the exact owned runtime instance
- **THEN** health success cannot authorize prompt dispatch, credentials or model-memory claims

#### Scenario: Offline model or runtime dependency is missing
- **WHEN** a configured local model, tokenizer, executable or pinned dependency is unavailable or changed
- **THEN** launch fails without pulling weights, modifying a user installation or selecting another provider

#### Scenario: Cancellation races startup or shutdown
- **WHEN** cancellation, failed readiness and concurrent close overlap
- **THEN** at most one owned runtime lease is released after confirmed tree exit
- **AND** unrelated user processes and another runtime's descendants are untouched

#### Scenario: Explicit Windows Ollama single-user enrollment
- **WHEN** the operator enrolls a Windows Ollama runtime with an existing private runtime, legacy GGUF model store and state directory
- **THEN** startup verifies the complete pinned runtime inventory and manifest/config/layer bytes, holds their read leases, and uses a separate owned process tree with a restricted environment
- **AND** missing pins, modified bytes, unsupported manifests, links or occupied unowned listeners reject launch without downloads or imports
- **AND** the exact established TCP connection must belong to that tree before HTTP sends prompt data

#### Scenario: Unauthenticated local daemon boundary
- **WHEN** a local Ollama enrollment lacks the explicit trusted-single-user and unauthenticated-loopback acknowledgments
- **THEN** service startup rejects that enrollment
- **AND** an acknowledged enrollment still cannot establish authenticated multi-user IPC, offline qualification or tool-agent qualification

#### Scenario: Linux owned descendants create separate sessions
- **WHEN** an explicitly pinned namespace launcher starts an owned Linux runtime and its descendants create new sessions
- **THEN** process ownership remains bound to a separate user/PID namespace and its retained namespace-init identity
- **AND** retirement verifies namespace-init exit and launcher reap within a deadline rather than treating a process-group signal as complete release
- **AND** service-owner or launcher death terminates the owned namespace without affecting unrelated runtimes
- **AND** missing pidfd, namespace, mapping or launcher proof keeps local admission unavailable without host configuration changes
