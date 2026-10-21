# Spec Delta

## Purpose

Keep proposed task access explicit before submission, enforce it during attachment, and distinguish model answers from evidence of executed work.

## ADDED Requirements

### Requirement: Proposed policy precedes execution
New task submissions SHALL accept an optional computer approval policy. The current composer SHALL propose Full task access before Start and explain that selecting it does not grant computer authority. Policy SHALL be included in request identity and preserved on the saved task.

#### Scenario: Lost acknowledgment
- **WHEN** a submission is retried with the same request ID and policy
- **THEN** the existing task is returned without a second execution
- **AND** using that ID with a changed policy is rejected as a payload conflict

#### Scenario: Older clients
- **WHEN** a client omits the proposed policy
- **THEN** the legacy consent flow remains compatible without expanding existing grants

### Requirement: Attachment respects recorded intent
A task with a recorded proposed policy SHALL only attach a locally consented computer session with that policy. The running-task access view SHALL display that policy without another policy selector. Local consent and existing target checks SHALL remain required until a compatible desktop-grant host is implemented.

#### Scenario: Mismatched session
- **WHEN** an Ask-every-time task is attached to a Full-task session
- **THEN** attachment is rejected before reservation or dispatch
- **AND** the task remains available for matching access

### Requirement: Failed proposals cannot become success through prose
An unresolved rejected proposal SHALL prevent successful completion. Prose in place of a repair SHALL receive corrective feedback within the existing three-repair budget, after which the task SHALL pause with a recoverable explanation. This SHALL never repeat dispatched or uncertain actions.

#### Scenario: Chrome request followed by user-facing JSON
- **WHEN** an invalid app access request is rejected and the model asks the user to submit its tool JSON
- **THEN** the task is not marked completed
- **AND** the model receives bounded corrective feedback to call the tool itself

### Requirement: Answers are not execution evidence
Completed task responses with no executed tools SHALL expose an answer-only outcome and display that no external action was performed. Ordinary writing and question-answering tasks SHALL remain usable without mandatory tool calls. Unknown historical evidence SHALL not be promoted to verified execution.

#### Scenario: Instructions instead of opening an application
- **WHEN** the model returns instructions without executing a tool
- **THEN** the result and history identify it as an answer, not a completed application operation
- **AND** text remains visible for review

#### Scenario: Ordinary writing
- **WHEN** the user requests a poem and the model supplies one without tools
- **THEN** the response remains an accepted answer without a forced or fabricated tool invocation
