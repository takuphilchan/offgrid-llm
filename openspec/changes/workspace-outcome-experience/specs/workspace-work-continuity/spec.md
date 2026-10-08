## Purpose

Keep current work, meaningful progress and useful results understandable across navigation without creating another task history or execution authority.

## ADDED Requirements

### Requirement: Scoped active-work access
The workspace SHALL provide compact access to known active conversations and jobs, showing their titles and actual status. Entries SHALL be scoped to the authenticated actor and connected workspace. Counts SHALL NOT imply complete discovery when the service cannot enumerate all active work.

#### Scenario: Navigate away and return
- **WHEN** a user leaves an accepted task or conversation for Models or Knowledge
- **THEN** the work remains reachable from the shared workspace and returns to the same identity
- **AND** navigation emits no submit, approval, cancellation or tool-dispatch request

#### Scenario: Authentication or workspace changes
- **WHEN** the user signs out, changes workspace or loses access
- **THEN** prior private titles, drafts, output and active subscriptions are no longer exposed to the new scope
- **AND** no new authority is inferred from cached work

### Requirement: Honest recovery and refresh
Returning, reloading and reconnecting SHALL restore available service snapshots and supported replay. Unknown or expired progress SHALL be labelled, not fabricated. Refresh SHALL be a read operation and SHALL NOT repeat mutations, uncertain actions or old audio.

#### Scenario: Lost connection
- **WHEN** progress disconnects while the service may still be working
- **THEN** the last known result is labelled as reconnecting or unavailable and a status check is offered
- **AND** the task is not labelled failed, completed or cancelled solely because transport ended

#### Scenario: No recoverable turn
- **WHEN** a conversation cannot provide a recoverable current turn
- **THEN** saved messages and retained drafts remain available with an explicit limitation
- **AND** the client does not fabricate a live session or automatically resend

### Requirement: Meaningful progress and controls
Work views SHALL prioritize the current phase, next required user action and applicable lifecycle controls. Stop and consequential pending input SHALL remain immediately reachable. Raw event counts, metrics and tool payloads SHALL be secondary; safety-relevant information SHALL NOT be hidden to reduce clutter.

#### Scenario: Pending approval
- **WHEN** a task requests access or exact-action confirmation
- **THEN** the request appears beside that task with readable operations, destinations and consequences
- **AND** its identity, expiry, local consent and existing approval policy are unchanged

#### Scenario: Stop requested
- **WHEN** a user requests Stop
- **THEN** local feedback appears promptly while dispatch/settlement status is reported separately
- **AND** previously dispatched effects are not described as undone

### Requirement: Result-first work presentation
A completed task SHALL prioritize its saved answer, existing output artifacts and available evidence before the execution log. Generated, saved and independently verified SHALL be distinguishable; downloadable artifacts SHALL NOT be presented as verified merely because they exist.

#### Scenario: Inspect a result
- **WHEN** a task returns output and artifacts
- **THEN** a user can read the result and open or download an authorized artifact without expanding technical details
- **AND** missing, revoked or unsupported previews show an honest fallback rather than unsafe rendering or an invented preview

### Requirement: Supported continuation without duplication
Follow-up and task steering SHALL use existing capability and lifecycle rules, retain request identity and make the destination task clear. Unsupported steering SHALL explain the limitation and offer an explicit new-task draft, never a fake continuation. Child activity SHALL remain subordinate to its parent-facing task.

#### Scenario: Steering available
- **WHEN** the service marks the selected task as steerable and the user submits a correction
- **THEN** the existing task receives one instruction through the established request contract
- **AND** a lost acknowledgment is reconciled without creating another task

#### Scenario: Steering unavailable
- **WHEN** a terminal or incompatible task cannot accept a correction
- **THEN** the interface distinguishes creating a new task from modifying the old one
- **AND** it preserves the original result and requires explicit submission of new work
