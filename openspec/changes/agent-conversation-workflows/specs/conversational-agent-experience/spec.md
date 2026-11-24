# Spec Delta

## Purpose

Provide one clear conversation surface for agent work and a reusable configuration home, consistently across desktop, browser and terminal clients.

## ADDED Requirements

### Requirement: Conversation-first default and shared settings
CLI and UI SHALL start ordinary assistant conversation without mandatory profile setup and restore the same service-owned context/settings on resume. Specialization SHALL remain optional. Effective configuration and its provenance SHALL be inspectable without exposing credentials.

#### Scenario: Resume across clients
- **WHEN** a CLI user resumes work created in the UI after a reload
- **THEN** context, restrictions and controls refer to the same canonical work without selecting an agent again

### Requirement: Canonical interactive source drafts
Canonical CLI conversation SHALL support the existing explicit file and image draft
commands through the owning session and canonical input service. Draft selection,
inspection, preparation and removal SHALL NOT submit input or delete saved sources.
The client SHALL require source capabilities before uploading or submitting and
retain source-only ordinary input, exact source revisions and shared model checks.
Unknown input receipts SHALL retain an immutable retry payload independently of
later draft edits; receipt recovery SHALL NOT repeat upload or inference.
New text SHALL NOT replace an unresolved source-bearing retry. An explicit local
discard SHALL explain that it does not cancel service work. A confirmed initial
validation rejection before admission MAY release the retry while retaining the
editable draft; a later rejection SHALL NOT erase earlier uncertainty.

#### Scenario: Attach sources during ordinary canonical conversation
- **WHEN** a user selects a file or image and submits ordinary text or an empty source-only caption
- **THEN** one canonical input references the owned file revision or selected image bytes
- **AND** no legacy generation path is used and accepted sources clear from the unchanged draft

#### Scenario: Edit a draft after an unknown input receipt
- **WHEN** an input response is lost, the user changes selected sources and explicitly retries the pending input
- **THEN** retry uses the original identity and complete source snapshot, observing an accepted receipt before any resubmission
- **AND** later draft edits are retained and no upload is repeated

#### Scenario: Unsupported sources or pending clarification
- **WHEN** a service lacks the necessary source capability or an input answering a pending clarification contains selected sources
- **THEN** the client reports the unsupported selection without silently dropping it or submitting another kind of request
- **AND** saved conversation controls and draft removal remain available

#### Scenario: Distinguish rejection from lost acceptance
- **WHEN** the initial submit receives a supported validation rejection before admission
- **THEN** a corrected draft can be submitted with a new input identity
- **AND** transport failures, server failures and receipt-inspection failures retain the original identity until explicit recovery or local discard

### Requirement: Eligible background work survives observation
Closing a client SHALL leave service-owned work running only while its authorized supervision remains available. Client-owned grants MUST NOT silently gain background lifetime. Reconnect SHALL observe existing work without recreating it.

#### Scenario: Closing an invocation-owned executor
- **WHEN** a CLI closes the host lease owned by that invocation
- **THEN** dependent work cannot continue dispatching through the expired grant and unrelated eligible service work remains observable

### Requirement: Profiles in Agents and work in Chat
Agents SHALL provide profile management and Chat SHALL provide profile selection, conversation and work. Job details SHALL remain a secondary inspector for the same canonical work. Existing job links MUST remain inspectable without restoring the retired agent page or resubmitting their prompts.

#### Scenario: Configure and use an agent
- **WHEN** a user creates a profile, attaches an authorized connection and selects it in Chat
- **THEN** subsequent turns show the selected agent and use its pinned configuration with activity displayed inline

### Requirement: Clear conversation hierarchy
User messages, assistant output and concise work activity SHALL form the primary reading flow. Tool payloads, context statistics and detailed verification SHALL be expandable. Manual outcome input MUST appear only for unresolved recovery, with the affected action explained, and MUST NOT be a normal task-completion requirement.

#### Scenario: Completed task
- **WHEN** requested work is verified
- **THEN** Chat presents its result and useful attachments with collapsed technical activity and no required outcome textarea

### Requirement: Accessible control surfaces
Controls SHALL follow existing monochrome design tokens and button conventions. Popovers MUST dismiss on outside click and Escape, restore focus, support keyboard and assistive technology, and fit short or narrow screens. Conversation and configuration SHALL support both themes, all nine locales, RTL and IME input.

#### Scenario: Short screen with keyboard input
- **WHEN** a keyboard user opens Tools on a short screen and presses Escape
- **THEN** all controls were reachable by scrolling, the popover closes and focus returns to its trigger without submitting text

### Requirement: Persistent task controls
Active computer work SHALL retain visible Pause, Take over and Stop controls across navigation. UI acknowledgment MUST distinguish a requested stop from confirmed stopped input. Voice input SHALL submit and steer the same work; speech MUST NOT grant additional access and narration SHALL remain parent-owned.

#### Scenario: Stop from another page
- **WHEN** a user navigates away from an active conversation and requests Stop
- **THEN** the control remains available and the UI shows stopping until the execution authority confirms stopping

### Requirement: Shared client contracts
CLI and Python clients SHALL manage profiles and submit or inspect the same conversations and canonical jobs through authenticated service APIs. Existing commands and endpoints MUST remain compatible adapters. Non-interactive clients SHALL expose structured pending-access, uncertainty and failure states without silently granting access.

#### Scenario: Cross-client inspection
- **WHEN** a CLI user inspects a job initiated in Chat
- **THEN** its identity, status, results and available recovery actions match Chat and inspection executes no work
