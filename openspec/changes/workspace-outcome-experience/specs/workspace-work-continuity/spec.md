## Purpose

Keep current work, meaningful progress and useful results understandable across navigation without creating another task history or execution authority.

## ADDED Requirements

### Requirement: Conversation navigation during generation
Accepted work SHALL continue independently of conversation selection. Navigation
SHALL NOT cancel, resubmit or attach output to another conversation. Returning
SHALL reconcile the original durable turn without replaying speech. Acceptance
still pending SHALL remain correlated with the original submission identity.

#### Scenario: Switch while generating
- **WHEN** a user opens another conversation or starts a new draft during generation
- **THEN** the selected conversation and its draft are usable while the original work remains discoverable
- **AND** late events and acknowledgments cannot change the selected conversation's output

### Requirement: Integrity-checked artifact preview
Supported bounded text and Markdown artifacts SHALL be previewable inside the
work view through authorized reads. Size and digest SHALL be checked before
strict UTF-8 decoding and safe rendering. Unsupported or oversized content SHALL
retain a labelled download path. Integrity SHALL NOT imply task correctness.

#### Scenario: Inspect a text artifact
- **WHEN** the user opens a supported artifact preview
- **THEN** the authenticated content is checked and displayed without downloading to disk or executing embedded content
- **AND** a revoked read, mismatched digest, invalid encoding or changed scope exposes no unchecked preview

### Requirement: Action-specific recovery and specialist summaries
Recovery SHALL name the affected work, retained output and appropriate safe next
action from typed state. Child summaries SHALL show actual available status and
deliverables while preserving parent ownership and authorized bounded observation.

#### Scenario: A specialist cannot complete
- **WHEN** a child is blocked or fails while independent work remains
- **THEN** the parent view identifies that branch and available partial results
- **AND** it neither claims full completion nor retries uncertain effects

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

### Requirement: Attention and history are distinct
Activity SHALL separate work needing attention, running work and terminal history,
with a shared classification and actor/workspace-scoped filter preference.

#### Scenario: Header opens pending work
- **WHEN** the user opens Activity from Needs attention
- **THEN** the attention filter is selected and completed history does not obscure pending work
- **AND** filtering, navigation and inspection do not approve, delete, restart or cancel work
