## Purpose

Adapt local inference to measured capabilities without creating another executor
or expanding permissions, replaying uncertain work, or silently changing models.

## ADDED Requirements

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
