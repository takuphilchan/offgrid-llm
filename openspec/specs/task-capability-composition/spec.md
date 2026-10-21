# task-capability-composition Specification

## Purpose

Allow task-first work to use connected services and computer controls together
without transferring authority between providers or losing verification.

## Requirements

### Requirement: Independently authorized provider composition

Task-first runs SHALL retain permitted registry tools when computer access is
attached. Discovery SHALL omit disabled and actor-denied registry tools.
Execution MUST recheck the current provider policy and task owner.

#### Scenario: Documentation and application work
- **WHEN** a task with an enabled MCP documentation tool gains native or browser access
- **THEN** its model can discover both capabilities in the same durable run
- **AND** the MCP call uses its own approval policy, not the computer grant

#### Scenario: Revocation after discovery
- **WHEN** a registry tool is disabled or denied for the actor after discovery
- **THEN** a later attempted dispatch is rejected without invoking its executor

#### Scenario: Cross-user request
- **WHEN** another actor attempts a tool call using the task identity
- **THEN** no registry or computer action is dispatched

### Requirement: Authority and execution location remain separate

Computer access MUST NOT grant service-file or arbitrary local execution
authority. Task-first discovery and dispatch SHALL exclude unreviewed local
execute tools. Host paths MUST NOT be silently translated into service paths.
Legacy computer-only tasks SHALL retain their narrower tool surface.

#### Scenario: Script hidden under another name
- **WHEN** a task-first model requests a local execute tool not named shell
- **THEN** the tool is absent from discovery and denied before dispatch

#### Scenario: Host path after computer access
- **WHEN** a service-file tool receives a recognizable foreign host path while a computer session exists
- **THEN** the service file operation does not run and the model receives guidance to use authorized computer access

#### Scenario: Legacy session
- **WHEN** an existing computer-only run without task-first semantics resumes
- **THEN** registry tools are not added to its original scope

### Requirement: Provider-specific observation and verification

Registry work SHALL be allowed before a computer observation, but computer input
MUST still require that observation. MCP text MUST NOT count as computer
verification, and computer automatic-approval labels MUST NOT describe MCP work.

#### Scenario: Research before editing
- **WHEN** a model calls a permitted documentation tool before observing the application
- **THEN** normal registry authorization applies without an observation-order failure
- **AND** subsequent computer input still requires a fresh observation

#### Scenario: External work after a verification
- **WHEN** registry work occurs after the last computer verification
- **THEN** it does not fabricate or replace that verification
- **AND** the task obtains a new computer verification before claiming completion
