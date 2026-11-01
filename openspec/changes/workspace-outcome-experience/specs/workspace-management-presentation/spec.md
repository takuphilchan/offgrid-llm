## Purpose

Make documents, work history and preferences useful to ordinary users while retaining clear, authorized access to maintenance and diagnostics.

## ADDED Requirements

### Requirement: Purpose-led capability configuration
Agent setup SHALL lead with purpose, identity and model, grouping optional
capabilities into discoverable sections. This supersedes the earlier requirement
that every tool and Knowledge choice be expanded at once. Examples, skill use
and explicit test/use actions SHALL preserve drafts, pinned revisions and grants.

#### Scenario: Configure and try an agent
- **WHEN** a user chooses a purpose example and configures selected capabilities
- **THEN** changes remain an editable draft until explicitly saved
- **AND** using or testing the agent requires explicit submission under existing authority

### Requirement: Source-specific questions
Document question handoffs SHALL identify actual selected source scope and enforce
it through the service, preserving explicit retrieval choice and the draft. This
extends the earlier broad-scope handoff without permitting a frontend-only claim.

#### Scenario: Selected source becomes unavailable
- **WHEN** a selected document is revoked, removed or unavailable for retrieval
- **THEN** the request reports that limitation and does not silently search unrelated documents

### Requirement: Readable scheduling and setup
Schedules SHALL lead with task/calendar and readable next-run review, with a
searchable time zone and subordinate budgets. Connections, voice, login and
permissions SHALL expose relevant readiness, scope and recovery without implicit
grants, capture, model downloads or commands on another host.

#### Scenario: Review a recurring task
- **WHEN** the user changes time, days or time zone
- **THEN** the next-run explanation updates before explicit reviewed save
- **AND** no scheduling preview executes or replays a task

#### Scenario: Preview follows the execution calendar
- **WHEN** an authorized user previews a recurring calendar
- **THEN** the service returns three future instants using its execution calendar and clock, including DST gap/fold rules
- **AND** changing the calendar clears obsolete predictions and late responses cannot replace the current preview
- **AND** unavailable previews remain explicit without granting authority, claiming occurrences or creating jobs

### Requirement: Direct profile resource configuration
Agent creation SHALL use available desktop width for identity/instructions and
resource selection. Tools and Knowledge SHALL be visible without opening a
disclosure. Skills and collaborators SHALL not require opening Advanced first.
Narrow screens SHALL retain a single readable order. Presentation changes SHALL
not select resources, grant access or modify drafts automatically.

#### Scenario: Create and revise a configured agent
- **WHEN** a user creates or edits a profile on a wide screen
- **THEN** basic fields and resource choices are visible in adjacent columns
- **AND** the existing validation, revision checks and explicit save apply

#### Scenario: Compact options and history
- **WHEN** the user opens Workspace options or scrolls chat history
- **THEN** aligned compact controls and a stable, visible scrollbar preserve
  keyboard access, native forced colors, RTL placement and dismissal behavior
- **AND** fine-pointer secondary actions use compact sizing while coarse-pointer
  targets remain at least 44px

### Requirement: Learnable terminology without changed authority
The workspace SHALL explain model categories, document retrieval, agent profiles
and task inspection in plain localized language. Guidance SHALL name supported
next actions and distinguish local service reachability from internet access.
Glossary and cognitive-walkthrough evidence SHALL identify protected changes and
unreviewed translations without claiming real-user validation.

#### Scenario: Discover a capability for the first time
- **WHEN** an operator opens Models, Knowledge or Agents
- **THEN** guidance explains its purpose and prerequisites using existing controls
- **AND** reading guidance does not download, execute, grant access or change a draft

### Requirement: Consistent search and profile organization
Search fields SHALL share an icon, accessible name, localized placeholder and conditional clear action. Agent profile editors SHALL group identity/instructions, model, tools and collaboration settings with advanced configuration subordinate, without altering execution semantics.

#### Scenario: Filter and clear a collection
- **WHEN** a user enters and clears a search by keyboard
- **THEN** the same collection updates with truthful empty/loading treatment and focus remains on the search field
- **AND** filtering or profile inspection does not execute work or change access

### Requirement: Document-first Knowledge
Knowledge SHALL foreground document identity, meaningful readiness and source inspection, with Add document as its primary entry action. Index internals, embedding configuration, reindexing and disabling Knowledge SHALL move to named management surfaces. Missing setup SHALL retain an actionable contextual path.

#### Scenario: Review a document
- **WHEN** a user opens a listed document
- **THEN** its available source, readiness and permitted actions are reachable without reading chunk counts or embedding settings first
- **AND** missing source retention or unsupported format is explained without an unsafe preview

### Requirement: Truthful Knowledge handoff
Moving from Knowledge to Chat SHALL preserve drafts and disclose the actual retrieval scope. The UI SHALL NOT imply selected-document-only answers when the service searches the broader knowledge base. No navigation action SHALL ingest a document or enable retrieval without explicit user intent.

#### Scenario: Ask using Knowledge
- **WHEN** a user chooses to ask a question using ready Knowledge
- **THEN** the composer shows the supported retrieval scope and requires explicit submission
- **AND** any existing draft is preserved or the user chooses how to replace it

### Requirement: Work-first Activity
Activity SHALL foreground current and recent permitted work with meaningful status and links to the canonical work view when identity is known. Runtime counters and raw payloads SHALL be secondary. It SHALL NOT create a second editable task or broaden existing administrator-only access.

#### Scenario: Existing agent task link
- **WHEN** a user follows an existing agent task or history route
- **THEN** Activity presents the same authorized canonical inspection or work list without a second permanent history sidebar
- **AND** non-administrator access remains limited to existing authorized task inspection, not administrative history or profile management

#### Scenario: Return from inspection
- **WHEN** a user returns from a selected task to Activity
- **THEN** scoped filters and list position remain available
- **AND** legacy run events remain readable without pretending they are editable canonical tasks

#### Scenario: Open a history item
- **WHEN** Activity identifies an existing task
- **THEN** opening it targets the same durable task and evidence rather than a duplicate history object
- **AND** independent statistics failure does not erase available work or its status

### Requirement: Coherent Settings groups
Settings SHALL group personal preferences, workspace/capability setup and diagnostics. Endpoint/version information SHALL have one authoritative detail section rather than repeated cards with conflicting values. Existing compatibility guidance, service ownership and emergency stopping SHALL remain available.

#### Scenario: Inspect an external service
- **WHEN** desktop uses a container or explicitly configured service
- **THEN** Settings distinguishes that service from desktop-local storage and identifies relevant build information
- **AND** a UI repair or refresh action cannot replace or stop the service

### Requirement: Contextual management and deletion
History, document and model maintenance SHALL use consistent labelled controls, selection and confirmation patterns. Consequential deletion SHALL describe exactly what is removed or retained, honor service eligibility and avoid claiming undo when none exists. Bulk operations SHALL retain snapshot-based execution and partial-result reporting.

#### Scenario: Partial deletion
- **WHEN** some selected items cannot be removed or deletion is stopped
- **THEN** processed and remaining items are distinguished and the selection is updated accurately
- **AND** already-deleted items are not resubmitted and unrelated documents/artifacts are preserved

### Requirement: Discoverable integrations without composer clutter
Tools and connections SHALL remain reachable through clearly labelled administration surfaces with persisted routes and role-aware access. Their configuration SHALL NOT be a permanent prerequisite panel on an ordinary task composer.

#### Scenario: Configure a connection and return
- **WHEN** an authorized user leaves a draft to inspect or configure a connection
- **THEN** the draft remains available on return and the connection shows its actual status
- **AND** connection availability alone does not grant task execution permission
