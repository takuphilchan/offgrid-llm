# Provider scheduling and recovery

## ADDED Requirements

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
