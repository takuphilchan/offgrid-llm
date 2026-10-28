## Purpose

Provide observable, bounded and interruptible native-agent command execution through authorized supervised workers rather than an implicit service shell.

## ADDED Requirements

### Requirement: Completion observes already-owned processes

When a proposed answer precedes terminal process evidence, the harness MAY
schedule a bounded read-only wait for a handle established by a recorded start
in the current execution session. Such a call SHALL be persisted and pass the
ordinary capability, profile, host, cancellation and dispatch checks. Completion
repair limits SHALL remain bounded; no start or mutation may be synthesized.

#### Scenario: Model omits required process observation
- **WHEN** the model proposes a final answer while the current root has a recorded running process
- **THEN** the proposed completion is rejected and a bounded observation may be scheduled for that same handle without restarting its command
- **AND** the next answer uses the recorded observation and cannot bypass terminal evidence requirements

#### Scenario: Foreign or revoked process authority
- **WHEN** a pending process belongs to an older session or its wait capability is no longer allowed
- **THEN** no automatic observation is authorized through a different session or expanded grant

#### Scenario: Restart before observation dispatch
- **WHEN** the service restarts after persisting the completion observation
- **THEN** its canonical call identity is retained and ordinary safe recovery applies without creating another command

### Requirement: Supervised user-authorized processes
The system SHALL bind processes to approved execution sessions and durable operation IDs, use an explicit directory and minimal environment, stream bounded output and return exit, signal, timeout and effect state. Trusted local execution MUST NOT be represented as a sandbox.

#### Scenario: Failed command
- **WHEN** an authorized test command exits unsuccessfully
- **THEN** the result retains its actual exit status and output rather than reporting a successful test run

#### Scenario: Duplicate dispatch
- **WHEN** a previously claimed operation is dispatched again after losing acknowledgment
- **THEN** recorded evidence is returned or uncertainty is reported without starting the command again

### Requirement: Owned process lifecycle
The system SHALL provide foreground/background inspection, bounded output retrieval and cancellation using owned handles. Stop MUST close new dispatch before acknowledging it and MUST NOT kill an unrelated or reused PID.

#### Scenario: Process tree stop
- **WHEN** a user stops a command that started children
- **THEN** the provider stops its owned process tree and distinguishes requested stopping from confirmed termination

#### Scenario: Worker restart
- **WHEN** the worker restarts without conclusive completion evidence
- **THEN** earlier mutating work remains uncertain and is not automatically replayed

### Requirement: Authority-aware execution
Ordinary commands covered by a trusted execution session SHALL run without repetitive approval. Sensitive or destructive operations MUST use their explicit approval path. Unknown inspection results MUST NOT establish safety or widen authority.

#### Scenario: Routine project test
- **WHEN** a covered project test is requested under a valid session grant
- **THEN** no additional desktop/window approval is required

#### Scenario: Isolated backend
- **WHEN** isolated execution is selected
- **THEN** only approved mounts and networking are exposed and the agent reports that backend instead of claiming execution on the personal host
