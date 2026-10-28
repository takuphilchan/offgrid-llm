## Purpose

Bind native-agent work to an explicitly selected execution host and folder instead of implicitly using the inference service's filesystem.

## ADDED Requirements

### Requirement: Explicit native response allocation
Native jobs SHALL support interactive and extended response allocations using
the existing Chat/model limits. Selection MUST be pinned to the saved job and
MUST NOT alter external-agent allocations, grants or resumed job state.

#### Scenario: Foreground context parity
- **WHEN** a new foreground CLI run reaches a service advertising response-profile support
- **THEN** it selects interactive context unless the user explicitly selects extended

#### Scenario: Compatibility and unsupported selection
- **WHEN** a legacy API caller omits the selection or a client reaches an older service
- **THEN** omitted selection preserves legacy behavior and an explicit unsupported selection fails before work is submitted

#### Scenario: Large or resumed work
- **WHEN** the user selects extended context or resumes a saved task
- **THEN** the selected eligible limit or original pinned allocation remains authoritative without silently dropping task history

### Requirement: Explicit execution context
The system SHALL pin actor, service workspace, host, backend, folder, current directory and grant revision to admitted jobs. Relative paths MUST resolve within that context without process-global directory changes.

#### Scenario: Terminal current directory
- **WHEN** an authorized CLI session starts in a selected local folder and requests a listing
- **THEN** the agent lists that folder and identifies the host rather than listing the service container

#### Scenario: Concurrent workspaces
- **WHEN** two sessions use different folders
- **THEN** relative reads and directory changes remain session-local

### Requirement: Explicit host authority
The system SHALL require a locally established execution grant, check actor and job bindings on dispatch, and keep computer, file and terminal authority distinct. Old jobs and profiles MUST gain no authority automatically.

#### Scenario: Computer consent only
- **WHEN** a job has desktop consent but no execution grant
- **THEN** host file and process operations remain unavailable

#### Scenario: Changed grant
- **WHEN** a grant is revoked or its revision differs from the admitted binding
- **THEN** the worker refuses dispatch before effects

### Requirement: Bounded workspace file operations
Workspace tools SHALL expose bounded listing, reading, search, creation and preconditioned edits with explicit location and completeness. They MUST reject path escape and unsupported file types and report independently observed results.

#### Scenario: Read pagination
- **WHEN** a file exceeds one response page
- **THEN** the response identifies its revision and next offset and rejects continuation against changed content

#### Scenario: Outside link
- **WHEN** a path resolves outside the approved file root through a link or traversal
- **THEN** no file content is returned and no write occurs

#### Scenario: Stale edit
- **WHEN** the current file no longer matches an edit's required preimage
- **THEN** the edit is rejected before replacement rather than overwriting the changed file

### Requirement: Task-owned literal input writing
The service SHALL offer a reviewed write capability that selects a literal UTF-8 span from the current root request or an applied user correction using unique boundaries. It MUST use the existing folder/write grant, persist the resolved bytes before dispatch and verify host read-back against those bytes. Ordinary writing and older jobs MUST retain their existing contracts.

#### Scenario: Exact multilingual literal
- **WHEN** an authorized task selects unique boundaries enclosing Unicode and newline content in its own user input
- **THEN** the host writes those exact bytes without model retyping, trimming or normalization
- **AND** the result identifies the source span and digests with independent matching read-back evidence

#### Scenario: Ambiguous or foreign source
- **WHEN** a proposal selects missing, repeated or reversed boundaries, a pending correction or a source outside this root
- **THEN** the service rejects it before host effects without exposing another input or creating authority

#### Scenario: Applied correction and old proposal
- **WHEN** steering reaches a safe boundary before an old proposal dispatches
- **THEN** the existing runner retires that proposal and presents only the root's valid applied inputs for a new decision
- **AND** prior dispatched evidence remains pinned and cannot be replayed through the new source tool

#### Scenario: Read-back or acknowledgment mismatch
- **WHEN** a dispatched source write lacks matching host digest, byte count or independent verification
- **THEN** its outcome remains unconfirmed, cannot be labelled verified and is not retried automatically
- **AND** recovery inspects the original intent/receipt rather than executing a replacement

#### Scenario: Ordinary writing and existing jobs
- **WHEN** an older job resumes or a caller uses ordinary inline-content writing
- **THEN** no new source tool or grant is added to that saved job and its previous argument contract remains usable

### Requirement: Truthful client capability presentation
CLI and apps SHALL use canonical conversations and effective toolsets, preserving execution context and history across turns. Unavailable host, permission and model states MUST remain distinguishable.

#### Scenario: Follow-up read
- **WHEN** a user asks to read a file after listing a folder
- **THEN** the next turn retains the same conversation and authorized folder

#### Scenario: Older service
- **WHEN** a service cannot negotiate execution contexts
- **THEN** the client explains incompatibility without falling back to a client-local agent engine

#### Scenario: Interactive one-shot in a local folder
- **WHEN** a terminal user runs a task without an existing execution session
- **THEN** the CLI offers one local current-folder approval and streams the canonical result in place
- **AND** declined or unavailable local authority never silently redirects file work to the service directory

#### Scenario: Explicit background or unattended use
- **WHEN** a caller uses JSON, nonterminal submission or explicit detach
- **THEN** existing asynchronous submission remains available without creating local authority implicitly
- **AND** detach cannot abandon an invocation-owned folder lease; callers select an independently approved session

#### Scenario: Cancel a foreground task
- **WHEN** a user interrupts a foreground run or its stream fails
- **THEN** the CLI stops the same canonical root and closes owned workspace authority, preserving uncertain effects and a stable inspection identity without resubmission

### Requirement: Conversation-first local access
New capable clients SHALL allow ordinary conversation before creating an executor
or requesting filesystem approval. Local file/process work SHALL request and
resolve access through the same canonical root; advertising a client capability
MUST NOT grant access or expose an unapproved folder.

#### Scenario: Creative request without a workspace
- **WHEN** an interactive user requests a story without selecting an execution session
- **THEN** the agent may answer directly without a folder prompt, executor startup or fabricated file read

#### Scenario: Local work needs access
- **WHEN** the agent requests local file work and no workspace is attached
- **THEN** the trusted client offers the intended folder and requested permission level
- **AND** a validated attachment resumes the same job without repeating earlier effects

#### Scenario: Capability question
- **WHEN** a user asks what the agent can do
- **THEN** the answer describes effective and requestable capabilities and their limits rather than substituting a directory report

### Requirement: Explicit local remembered trust
Clients MAY remember an explicit folder authorization locally, bound to canonical
folder identity, host, service workspace, actor, backend and permission ceiling.
They SHALL expose inspection and revocation. Existing session grants MUST NOT
be silently converted, and remembered trust MUST NOT authorize desktop input.

#### Scenario: Subsequent approved folder work
- **WHEN** a later invocation requests covered access to the identical trusted resource
- **THEN** the client can establish a fresh scoped host grant without asking again

#### Scenario: Changed identity or broader request
- **WHEN** folder identity, host, service workspace, account or requested permission exceeds the remembered decision
- **THEN** automatic approval is denied and explicit local authorization is required

#### Scenario: Unattended or declined request
- **WHEN** local authorization is unavailable or declined
- **THEN** the same job reports that state without falling back to service files, silently granting access or duplicating submission

### Requirement: Concise truthful activity
Clients SHALL distinguish internal preparation, observed actions, generated
answers and actionable failures. Default output SHALL avoid duplicate answer/run
blocks while retaining inspection identities and machine-readable compatibility.

#### Scenario: Failed unnecessary read
- **WHEN** a model unnecessarily reads a nonexistent file for a creative request
- **THEN** recovery preserves the original request and the failed read evidence rather than presenting internal recovery instructions as task fulfillment

### Requirement: Discoverable persistent agent sessions
Interactive CLI sessions SHALL present one welcome, the selected profile/model,
and an input prompt. They SHALL provide a shared slash-command registry for help,
keyboard selection and pinned resource inspection, without submitting those
commands as model requests. Recovery identities SHALL remain available on demand.

#### Scenario: Explore the current session
- **WHEN** the user types `/help`, `/skills`, `/agents`, `/tools` or `/session`
- **THEN** the client shows the requested view without creating a job, changing the selected revision or granting access

#### Scenario: Streaming and local consent
- **WHEN** a turn streams or requests local workspace access
- **THEN** the input editor has restored terminal mode, the existing stream/consent path retains input ownership, and no background reader consumes consent or future messages

#### Scenario: Terminal compatibility
- **WHEN** the terminal is narrow, plain output is requested, or input is redirected
- **THEN** bounded line input and explicit slash commands remain usable without cursor-control escape sequences

#### Scenario: Unknown command
- **WHEN** an unrecognized slash command is entered
- **THEN** the client explains how to discover valid commands without submitting the text to the agent

### Requirement: Consistent governed decision transport
Unchanged validated response schemas SHALL retain their property order through
public API forwarding, and historical governed calls SHALL present the tool name
before arguments consistently with fresh decisions. Retained wire representation
MUST NOT conceal changes to the decoded schema or bypass schema validation.

#### Scenario: Forward a discriminator-first decision schema
- **WHEN** a valid schema places the tool name before its arguments
- **THEN** forwarding preserves that order and exact numeric constants without changing the selected adapter or tool authority

#### Scenario: Caller modifies the schema
- **WHEN** in-process code changes a decoded schema, including a nested field
- **THEN** forwarding uses that current schema rather than an earlier retained wire value

#### Scenario: Ambiguous schema JSON
- **WHEN** retained schema input contains repeated object keys or malformed Unicode
- **THEN** decoding rejects it rather than validating one interpretation and forwarding another
