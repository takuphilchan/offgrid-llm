# Provider contract

## ADDED Requirements

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
Non-llama workers SHALL use a versioned local protocol with request IDs,
cancellation IDs, model digest, typed stream events, terminal status, and
capability metrics. Workers SHALL NOT receive credentials, desktop authority,
or unrestricted command/file access.

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
