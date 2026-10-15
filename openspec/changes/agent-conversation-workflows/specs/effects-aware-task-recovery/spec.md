# Spec Delta

## Purpose

Recover agent work according to evidence about execution effects, keeping avoidable validation failures separate from genuinely uncertain actions.

## ADDED Requirements

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
