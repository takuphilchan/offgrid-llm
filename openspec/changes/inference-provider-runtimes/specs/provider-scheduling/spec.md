# Provider scheduling and recovery

## ADDED Requirements

### Requirement: Failed embedding loads retain uncertain resources
The shared embedding owner MUST retire a partially loaded or invalid-readiness
implementation before releasing its resources. An unconfirmed cleanup MUST keep
that implementation tracked without declaring it loaded/ready, record unresolved
residency and block later provider or embedding allocation until explicit recovery.

#### Scenario: Failed load cannot confirm cleanup
- **WHEN** an embedding implementation fails loading or reports invalid dimensions and then fails unloading
- **THEN** the original failure and retirement uncertainty are preserved, the implementation remains owned, and readiness stays false
- **AND** a later embedding factory, native provider or legacy allocation cannot reuse its capacity

#### Scenario: Final owner cleanup after a failed load
- **WHEN** the service shuts down after a failed embedding load with unconfirmed retirement
- **THEN** the same tracked implementation receives final cleanup and the prior uncertainty is not silently erased

### Requirement: Speculative reads respect host memory pressure
Prewarming SHALL have bounded concurrency, bytes, duration and retained history.
It SHALL stop on pause, memory pressure or read failure and SHALL NOT label a
partial read as a completely warmed model. Windows model admission SHALL account
for measured commit headroom separately from physical RAM.

#### Scenario: Full commit despite free physical RAM
- **WHEN** Windows reports insufficient commit headroom for a model load
- **THEN** admission rejects the load without silently reducing its context

#### Scenario: Repeated prewarm requests during pressure
- **WHEN** requests arrive while warming is active or host memory is low
- **THEN** they do not create an unbounded background queue or bypass the budget

#### Scenario: Pause during prewarming
- **WHEN** active model loading pauses speculative reads
- **THEN** the speculative operation stops without a blocked producer or a false complete result

### Requirement: One scheduler governs local inference
Parent, child, perception, capability-check, and ordinary chat generations SHALL
share admission, model-load, memory, cancellation, and budget controls.

#### Scenario: Resident provider
- **WHEN** a compatible provider is already loaded
- **THEN** the scheduler reuses it without an unnecessary model swap

#### Scenario: Unqualified hardware
- **WHEN** hardware concurrency or memory safety is unmeasured
- **THEN** the scheduler permits one active generation and reports queueing
  separately from generation time

### Requirement: Safe provider switching
Provider/model switching SHALL occur only at a persisted safe boundary and SHALL
not retry a mutation whose external effect is unknown.

#### Scenario: Unknown effect
- **WHEN** a tool action has an uncertain receipt
- **THEN** provider fallback cannot replay the action or bypass reconciliation

### Requirement: Stop remains independent
Stop SHALL remain dispatchable while a provider is loading, queued, generating,
or shutting down, and delayed stopping SHALL remain visibly uncertain.

#### Scenario: Stop during generation
- **WHEN** a provider is generating and the user requests Stop
- **THEN** cancellation is sent independently of generation completion and the task does not dispatch another request
