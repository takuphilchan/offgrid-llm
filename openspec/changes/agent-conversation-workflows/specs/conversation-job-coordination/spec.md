# Spec Delta

## Purpose

Keep conversations and durable agent work linked reliably across failures, navigation and clients, with one execution identity per submitted turn.

## ADDED Requirements

### Requirement: Canonical work per turn
A tool-enabled conversation turn SHALL use the existing durable execution service, including turns answered without tools. Each submission MUST persist an actor-scoped request identity and payload digest, link exactly one canonical job and hold that job from execution until its conversation association is durable.

#### Scenario: Duplicate admission
- **WHEN** two submissions have the same actor, request identity and payload
- **THEN** both return the same turn and job, while reuse with a different payload returns a conflict

#### Scenario: Failure during linkage
- **WHEN** persistence fails before or after job creation or association
- **THEN** recovery completes accepted bookkeeping without a second job or unassociated execution

### Requirement: Idempotent final response
Final conversation output SHALL be committed once by turn and job identity. If final persistence fails, the canonical job MUST retain its result and expose conversation recovery without repeating the task.

#### Scenario: Result saved only in job
- **WHEN** a completed job's conversation write fails and the user reconnects
- **THEN** its recorded result can be attached once and no tool is rerun

### Requirement: Observation and steering have distinct effects
Navigation, reconnecting and opening work details SHALL observe existing work without submission, approval or replay. Steering SHALL persist for the next safe execution boundary and MUST NOT claim to cancel an already dispatched action. Restart recovery MUST preserve explicit resume and uncertain-action boundaries.

#### Scenario: Navigate while a tool runs
- **WHEN** a user leaves Chat, opens Agents and returns while a call is executing
- **THEN** every view refers to the same job and no duplicate execution occurs

#### Scenario: Correction after dispatch
- **WHEN** a correction arrives after an external action was dispatched
- **THEN** the correction is acknowledged as pending and applied after that action's recorded result or recovery boundary

### Requirement: Conversation lifecycle and context
Tool-enabled turns SHALL preserve authorized conversation history, selected Knowledge context, model choice and voice input. Profile changes SHALL affect future turns. Deleting a conversation with active work MUST require stopping it or retaining its canonical work for inspection. Minimal deduplication records MUST survive visible-history deletion without retaining unnecessary conversation content.

#### Scenario: Delete while running
- **WHEN** a user chooses to retain active work while deleting its conversation
- **THEN** the job remains accessible in the work inspector and the request identity cannot launch a replacement job

#### Scenario: Existing text-only conversation
- **WHEN** an existing conversation uses no agent profile or tools
- **THEN** its current streaming, persistence, Knowledge and model behavior remain compatible
