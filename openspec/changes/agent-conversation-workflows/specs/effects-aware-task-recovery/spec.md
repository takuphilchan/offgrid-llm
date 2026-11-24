# Spec Delta

## Purpose

Recover agent work according to evidence about execution effects, keeping avoidable validation failures separate from genuinely uncertain actions.

## ADDED Requirements

### Requirement: Recoverable change-set integration
Worker changes SHALL retain baseline digests, instruction versions, validation evidence and per-file receipts. Integration SHALL preserve displaced contents and expose conflict, partial and uncertain outcomes. Unsupported replacement guarantees MUST yield reviewable changes rather than unsafe automatic replacement.

#### Scenario: External editor races replacement
- **WHEN** the actual displaced file differs from the expected baseline
- **THEN** integration stops, both versions remain recoverable and no blind rollback overwrites newer work

#### Scenario: Special-file substitution during worker preparation or verification
- **WHEN** an inspected regular file becomes a link or FIFO before it is opened for a worker copy or replacement observation
- **THEN** the open rejects the substituted object without following the link or waiting for a FIFO writer
- **AND** no shared-root fallback or mutation replay occurs, while any already displaced content remains retained

#### Scenario: Oversized working-copy directory
- **WHEN** a source or retained worker directory exceeds its admitted entry budget
- **THEN** enumeration rejects it with bounded read batches before collecting the full directory
- **AND** incomplete growth measurement cannot release reserved storage capacity

#### Scenario: Cleanup receipt is lost and the pathname is recreated
- **WHEN** cleanup removes its owned directory but cannot publish its final receipt, and another directory appears at the old pathname
- **THEN** an explicit retry rejects the changed native identity, preserves the replacement directory and retains the unresolved reservation
- **AND** restart does not automatically repeat deletion or restore worker authority

#### Scenario: Cleanup is incomplete or ownership is unavailable
- **WHEN** a private copy has oversized or changing cleanup contents, cancellation, or legacy metadata without native ownership proof
- **THEN** cleanup does not recursively delete an unverified pathname or release unobserved capacity
- **AND** original input/change manifests and retained artifacts remain available for review without adopting a new directory as the old copy

#### Scenario: Worker termination does not prove all descendants exited
- **WHEN** a worker has a failed launch, an unobserved native descendant, an escaped process-group child or lost supervisor state
- **THEN** its durable worker-use lease and storage reservation remain unresolved even if the command leader exited or a Job reports no active members
- **AND** restart or bounded process-history eviction cannot invent the missing proof or automatically delete its working copy

#### Scenario: Original pathname changes during cleanup detachment
- **WHEN** another directory replaces the worker path between verification and the no-replace quarantine move
- **THEN** post-move identity verification rejects deletion, retains the displaced object and original ownership evidence, and never rolls it back over a newer path

#### Scenario: Exact Windows cleanup handle outlives pathname replacement
- **WHEN** a cleanup object is renamed and another file appears at its old pathname after the deletion handle was opened
- **THEN** native disposition removes only the verified opened object and preserves the replacement
- **AND** existing or newly arriving write handles cannot enter its identity/disposition interval

### Requirement: Trusted effects classification
The system SHALL distinguish rejection before effects, completed effects, partial effects and unknown effects using trusted execution evidence. Provider annotations and model claims MUST NOT establish absence of effects. A known pre-effect rejection SHALL receive bounded corrective feedback rather than an unknown-outcome form.

#### Scenario: Protected service path
- **WHEN** a service write is rejected by path validation before directory creation or writing
- **THEN** the action is recorded as not executed and the model receives actionable correction within the repair limit

#### Scenario: Failure after partial write
- **WHEN** an operation changes storage and subsequently fails without a conclusive result
- **THEN** it is not classified as having no effects and automatic mutation retry is prevented

### Requirement: Bounded repair
Malformed or unsupported proposals proven to precede dispatch SHALL permit at most three consecutive corrections. Unrelated reads MUST NOT reset that limit. Exhaustion SHALL explain the blocked capability or argument and preserve the task's prior results.

#### Scenario: Repeated wrong location
- **WHEN** a model repeatedly attempts to use service-file tools for a host document despite corrective feedback
- **THEN** the turn stops with the location mismatch explained and without fabricating a successful document edit

### Requirement: Evidence before manual recovery
For uncertain work the system SHALL first use available trusted journals or safe observations that cannot repeat the mutation. If these cannot establish the relevant outcome, it MUST pause and expose the affected action and recovery choices. State that merely resembles success MUST NOT prove an external submission occurred exactly once.

#### Scenario: Lost acknowledgment after input
- **WHEN** acknowledgment is lost after native text insertion
- **THEN** the insertion is not repeated; permitted journal or read-back checks run and unresolved uncertainty remains visible

### Requirement: Truthful reconciliation and completion
Manual recovery SHALL record who reported what happened and SHALL remain distinguishable from independent verification. Entering arbitrary text such as done MUST NOT automatically verify all requested outcomes. Completed, partially completed, unavailable and uncertain outcomes SHALL remain distinguishable in every client.

#### Scenario: Human reports completion
- **WHEN** a user records done for an uncertain file operation
- **THEN** the report is saved as human evidence without fabricating file contents or marking unrelated checks verified
