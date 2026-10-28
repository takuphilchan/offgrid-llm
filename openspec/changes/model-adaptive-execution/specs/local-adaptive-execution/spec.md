## Purpose

Adapt local inference to measured capabilities without creating another executor
or expanding permissions, replaying uncertain work, or silently changing models.

## ADDED Requirements

### Requirement: Reviewed skill contracts validate answers without replay
New skill revisions MAY declare a versioned bounded text response contract.
Only persisted reviewed metadata and a successful matching pinned load SHALL
activate it. The runner SHALL validate complete final answers after existing
effect verification, preserve failed evidence, and allow at most two answer-only
corrections within the root budget. Correction SHALL expose no tools and SHALL
independently reject tool dispatch, including restored calls.

#### Scenario: Missing output requirement
- **WHEN** an answer fails a loaded reviewed skill's contract
- **THEN** completion is withheld and bounded answer-only correction is persisted
- **AND** earlier tools are not replayed and failure is reported if correction exhausts its limit

#### Scenario: Old skill or unreviewed metadata
- **WHEN** an existing stored skill has no reviewed contract field, a skill is only a draft, or a returned contract differs from the pinned revision
- **THEN** no new contract is activated from that content
- **AND** unavailable or invalid explicitly pinned contracts cannot be treated as passed

#### Scenario: Restart or malicious correction call
- **WHEN** correction resumes after restart or proposes a tool call
- **THEN** its persisted limit remains and no tool call dispatches

#### Scenario: Contract definition rejected
- **WHEN** a new contract has an unknown version/field, invalid pattern or excessive bound
- **THEN** import fails without changing the previous revision

### Requirement: Local escalation is explicitly configured and bounded
Profiles SHALL retain their primary model and optionally approve at most two
installed fallback models and a perception model. Root tasks SHALL allow at most
two switches and three consecutive corrective attempts across models, sharing
the root budget and persisting switches before resumed inference.

#### Scenario: Existing profile
- **WHEN** an existing profile is loaded after upgrade
- **THEN** no fallback model, download or permission is added

#### Scenario: Unsafe switch boundary
- **WHEN** a task has dispatched input with an unknown outcome
- **THEN** model escalation cannot retry the action or bypass reconciliation

#### Scenario: Missing configured fallback
- **WHEN** an approved fallback is not installed or cannot become ready
- **THEN** the task reports its unavailable reason without downloading or using cloud inference

### Requirement: Adapter normalization precedes execution
All executable model responses SHALL normalize to complete registered tool calls
with validated arguments. Prose, partial streams and malformed envelopes SHALL
never become executable calls. Guided and adaptive modes SHALL share enforcement.

#### Scenario: Explicit qualified native selection
- **WHEN** a service-owned policy selects native tool calling for a newly admitted model and context with matching workflow evidence
- **THEN** the job persists its adapter, evidence and fingerprint before inference
- **AND** counting and generation use that same adapter without changing the selected model or tool authority

#### Scenario: Evidence changes after admission
- **WHEN** a pinned native job no longer matches the current runtime or qualification evidence
- **THEN** inference stops before another decision and does not silently fall back or replay earlier effects

#### Scenario: Existing or unqualified job
- **WHEN** a job has no native adapter pin or an installation has no qualified native policy
- **THEN** the existing constrained path remains in use

#### Scenario: Scoped native tool selection
- **WHEN** a new job selects capability-scoped qualification
- **THEN** its scope is persisted with the adapter and evidence pin
- **AND** initial menus, discovery and every dispatch enforce the reviewed subset in addition to existing authority

#### Scenario: Out-of-scope or forged tool
- **WHEN** a model or resumed checkpoint proposes an unqualified tool, an unknown scope or a same-named capability from a different provider
- **THEN** no effect is dispatched, including runtime delegation or desktop input
- **AND** the job cannot silently change adapters or claim the requested action completed

#### Scenario: Truncated response
- **WHEN** a runtime emits a partial call or unclean stream termination
- **THEN** no tool dispatch occurs and bounded pre-effect correction is available

### Requirement: Inference admission and context preserve root constraints
Parent, child and perception work SHALL share admission and aggregate budgets.
Compaction SHALL preserve user constraints and pending effects. Unqualified
hardware SHALL default to one active generation; scheduling SHALL not block Stop.

#### Scenario: Child admission exhausts remaining budget
- **WHEN** requested child work would consume the reserved parent verification/answer budget
- **THEN** admission rejects the child before creating runnable work

#### Scenario: Model changes after repair
- **WHEN** a corrective attempt escalates to another approved model
- **THEN** existing repair counters, authority and unresolved effects remain in force

### Requirement: Context allocation is model-aware and stable
Adaptive context admission SHALL bound each model by the operator ceiling, known
trained context, safe local resource capacity and current power or pressure policy.
The admitted window SHALL remain pinned for active work and SHALL be the value
advertised to external-agent setup and model discovery.

#### Scenario: Capable long-context model
- **WHEN** a model declares at least 65,536 trained tokens and local capacity safely admits that tier under the operator ceiling
- **THEN** new extended and external-agent work receives a 65,536-token allocation

#### Scenario: Constrained or unmeasured capacity
- **WHEN** the model or machine cannot establish a safe requested long-context tier
- **THEN** the system advertises and allocates the same smaller tier without claiming long-context readiness

#### Scenario: Pressure changes during active work
- **WHEN** power or resource pressure changes after a task has been admitted
- **THEN** its context is not silently shrunk and any different tier waits for a later safe lifecycle boundary

#### Scenario: Model trained limit is lower than configuration
- **WHEN** the configured operator ceiling exceeds the model's known trained context
- **THEN** the model's advertised and allocated context does not exceed its trained limit
