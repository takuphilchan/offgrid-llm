## Purpose

Separate synthetic runtime capability evidence from operational failures and real
workflow qualification while keeping checks bounded and free of external effects.

## ADDED Requirements

### Requirement: Concurrent checks share only active work
The service SHALL deduplicate concurrent identical synthetic model checks without
caching completed success as future execution authority. Each caller SHALL retain
independent cancellation. The last departing caller SHALL cancel remaining work.

#### Scenario: One observer cancels
- **WHEN** two callers await the same check and one cancels
- **THEN** only that caller receives cancellation and the other receives the shared result
- **AND** one synthetic two-step generation sequence runs without host dispatch

#### Scenario: Last observer cancels and a new caller arrives
- **WHEN** all callers cancel an in-flight check
- **THEN** its context is cancelled and a later caller receives a new check
- **AND** late completion of the old check cannot delete or overwrite the new check

#### Scenario: Later task requires validation
- **WHEN** a previously completed check is requested again
- **THEN** a fresh check runs rather than returning cached success

### Requirement: Evidence distinguishes measurement from qualification
Capability evidence SHALL bind model digest, quantization, runtime build, template,
adapter, context and relevant perception identity. Unknown or changed identity
SHALL prevent reusing qualification. Operational unavailability SHALL NOT be a
failed capability measurement.

#### Scenario: Runtime changed
- **WHEN** a recorded runtime fingerprint differs from the active runtime
- **THEN** its evidence is shown as stale and cannot qualify current work

#### Scenario: Runtime queue is busy
- **WHEN** admission times out or is cancelled before a measurement completes
- **THEN** the result reports operational unavailability, not model incompatibility

### Requirement: Checks are isolated and bounded
Synthetic checks SHALL use service-authored fixtures without user context or tool
dispatch, respect inference admission and maintain separate admission/probe budgets.

#### Scenario: Existing inference occupies the runtime
- **WHEN** a check waits for active inference longer than its own probe budget
- **THEN** existing inference is not interrupted and the probe budget starts after admission

#### Scenario: Ordinary conversation
- **WHEN** an actor sends an ordinary text-only conversation turn
- **THEN** a failed desktop check does not disable that conversation path
