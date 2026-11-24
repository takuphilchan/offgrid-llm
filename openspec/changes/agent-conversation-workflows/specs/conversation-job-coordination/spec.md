# Spec Delta

## Purpose

Keep conversations and durable agent work linked reliably across failures, navigation and clients, with one execution identity per submitted turn.

## ADDED Requirements

### Requirement: Default canonical admission preserves request semantics
Matching ordinary clients SHALL use the canonical conversation inbox without
mandatory profile setup. Normal service configuration SHALL permit this
admission; an explicit operator disable SHALL stop new admission while preserving
saved observations, controls, recovery and identical accepted-command receipts.
Canonical inputs SHALL preserve explicitly selected response context and output
limits in their durable request identity. Source-only ordinary input SHALL retain
its image or attachment meaning without inventing an external work objective.
Legacy requests SHALL retain their explicit semantics, and imported conversation
ownership SHALL prevent a legacy writer from starting a second execution path.

#### Scenario: Default service and ordinary conversation
- **WHEN** a matching client submits ordinary conversation on default service configuration without selecting a profile
- **THEN** one canonical input is accepted through the existing service runner
- **AND** reload or inspection creates no additional input, inference or work

#### Scenario: Response settings and retry
- **WHEN** input selects interactive or extended context and an output token limit
- **THEN** the service pins those options before model qualification and inference
- **AND** identical retry retains the options while changed reuse conflicts

#### Scenario: Source-only ordinary input
- **WHEN** an ordinary input contains authorized supported images or attachment references without text
- **THEN** it can be answered using the same source and model capability checks as text-bearing input
- **AND** empty input without sources, invalid sources and textless corrections or work objectives remain invalid

#### Scenario: Explicitly disabled admission
- **WHEN** the operator disables conversation admission after input was accepted
- **THEN** new input is rejected without discarding its draft
- **AND** observation, Stop, recovery and identical accepted retries remain available without replay

#### Scenario: Explicit legacy request or native protocol
- **WHEN** an existing client explicitly uses a legacy conversation API or a model has a pinned native tool protocol
- **THEN** the legacy request is not silently reinterpreted as canonical work and imported ownership remains enforced
- **AND** default admission does not add unqualified native coordination tools or discard qualification pins

### Requirement: Foreground discovery is metadata only
New canonical foreground turns SHALL pin a versioned discovery ceiling separately
from executable capabilities. The existing task_find_tools control SHALL expose
only bounded, progressively selected metadata after current actor, conversation,
profile and provider identity checks. Discovery SHALL NOT invoke providers, contact
execution hosts, read resource contents, acquire approval, promote executable
schemas, create work or refresh a budget. Provider prose is untrusted metadata.

#### Scenario: Relevant tool found without execution
- **WHEN** an authorized foreground turn searches its pinned tool inventory
- **THEN** at most eight matching metadata entries and a continuation offset are returned
- **AND** each result states that work admission and live authorization remain required
- **AND** subsequent foreground model turns and direct forged calls still cannot execute those tools

#### Scenario: Current scope narrows or provider changes
- **WHEN** actor authority, conversation restrictions, a selected profile or a provider identity no longer permits a pinned discovery entry
- **THEN** ranking and pagination exclude that entry, including after restart
- **AND** metadata already recorded remains historical evidence rather than renewed authority

#### Scenario: Existing protocol and qualification
- **WHEN** a historical turn lacks the versioned discovery ceiling or a runtime is qualified for text only
- **THEN** no new discovery behavior is enabled for that turn
- **AND** pinned native tool scope and native coordination guards remain unchanged

### Requirement: Canonical conversation control
Projects, conversations, work, runs and workers SHALL have stable scoped identities. Accepted commands SHALL atomically persist state, revision, event and outbox before dispatch. Legacy conversation files SHALL become projections only after an explicit versioned migration. Identical retries SHALL return the original outcome and conflicting payloads SHALL be rejected.

#### Scenario: Projection unavailable after acceptance
- **WHEN** a command commits but its legacy conversation projection fails
- **THEN** reconnect observes canonical state and projection repair cannot repeat execution

### Requirement: Persistent usable history
Applied corrections, failures, cancellations and partial outcomes SHALL remain available to later turns. Context selection SHALL preserve essential constraints within the actual model budget and expose authorized retrieval references for omitted content. Source revocation SHALL invalidate dependent projections.

#### Scenario: Long answer and failed follow-up
- **WHEN** a long answer is followed by a failed task and the user asks to fix it
- **THEN** the next turn retains applicable corrections and failure evidence rather than discarding the entire earlier conversation

### Requirement: Ordered controls and responsive input
Work SHALL record instruction versions and control epochs. Corrections SHALL fence obsolete mutations; Stop SHALL prevent dispatch without inference. Late results SHALL retain receipts without reviving stopped work. Foreground input SHALL remain available while background work waits.

#### Scenario: Late child after correction
- **WHEN** a child finishes using an instruction version superseded by a correction
- **THEN** its result is retained as stale evidence and cannot be automatically integrated without revalidation

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
