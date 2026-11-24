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

#### Scenario: Recovery capability or status is unavailable
- **WHEN** the desktop recovery bridge is missing, the actor is ineligible, the service is external, or the initial status read is pending or fails
- **THEN** recovery names the observed blocker or checking state and does not report Ready or present an actionable operation without confirmed eligibility
- **AND** a retry reads status only, retains uncertainty and never repeats an operation; supported active work retains Stop and explicit Resume
- **AND** the existing local-directory card remains unchanged while recovery uses balanced padding, readable status and a useful existing navigation or retry action

#### Scenario: Return from the desktop terminal over Settings
- **WHEN** the terminal is opened from Settings and the user returns to the workspace
- **THEN** the action is labelled for that return, preserves the previous route/draft and does not close the process or send terminal input
- **AND** the terminal canvas and surrounding workspace form one continuous themed surface without an inset bordered panel, including supported palette and forced-color modes

#### Scenario: Inspect an external service
- **WHEN** desktop uses a container or explicitly configured service
- **THEN** Settings distinguishes that service from desktop-local storage and identifies relevant build information
- **AND** a UI repair or refresh action cannot replace or stop the service

### Requirement: Contextual management and deletion
History, document and model maintenance SHALL use consistent labelled controls, selection and confirmation patterns. Consequential deletion SHALL describe exactly what is removed or retained, honor service eligibility and avoid claiming undo when none exists. Bulk operations SHALL retain snapshot-based execution and partial-result reporting.

#### Scenario: Ordinary versus protected conversation deletion
- **WHEN** an ordinary conversation is selected for deletion
- **THEN** confirmation identifies the conversation without an unconditional active-work policy form
- **AND** a typed active-work conflict reveals explicit stop/retain choices without sending either until confirmation, while a newly opened dialog forgets previous choices

#### Scenario: Compact management actions
- **WHEN** a history row's action menu opens
- **THEN** actions use a compact anchored list without nested button frames, preserve keyboard dismissal/focus restoration and remain usable on narrow, RTL and coarse-pointer surfaces

#### Scenario: Partial deletion
- **WHEN** some selected items cannot be removed or deletion is stopped
- **THEN** processed and remaining items are distinguished and the selection is updated accurately
- **AND** already-deleted items are not resubmitted and unrelated documents/artifacts are preserved

#### Scenario: Return to a filtered document library
- **WHEN** a user leaves a later page of the library and returns in the same account/workspace session
- **THEN** its filter, valid page and still-accessible selection remain available alongside the retained Chat draft
- **AND** changing the filter starts at the first page, a shorter refreshed list clamps to a valid page, and navigation performs no mutation

### Requirement: Discoverable integrations without composer clutter
Tools and connections SHALL remain reachable through clearly labelled administration surfaces with persisted routes and role-aware access. Their configuration SHALL NOT be a permanent prerequisite panel on an ordinary task composer.

#### Scenario: Configure a connection and return
- **WHEN** an authorized user leaves a draft to inspect or configure a connection
- **THEN** the draft remains available on return and the connection shows its actual status
- **AND** connection availability alone does not grant task execution permission

#### Scenario: Edit a reusable profile without accidental grants
- **WHEN** a user selects a profile or creates a draft
- **THEN** save/use actions are grouped with the editor heading and model selection is labelled without a stretched control
- **AND** instruction examples edit only the draft, while optional resources remain explicit selections
- **AND** archived profiles are identified and cannot be newly selected as delegates; existing unavailable selections remain removable

#### Scenario: Recover a failed tool or connection change
- **WHEN** a tool or MCP mutation is pending
- **THEN** conflicting mutations are disabled until it settles
- **AND** failure preserves the prior tool state or connection draft and exposes retry without automatic replay
- **AND** responsive connection and tool cards retain complete boundaries and accessible controls in RTL and narrow layouts

#### Scenario: Inspect external-agent setup
- **WHEN** a user opens external-agent setup instructions
- **THEN** service/model compatibility, host setup and workflow qualification remain distinct
- **AND** copying never executes a command, clipboard failures appear inside the open dialog, and closing restores focus

### Requirement: Consistent public product and download identity
The application SHALL present itself as OffGrid. Desktop release filenames SHALL
identify the version, operating system, architecture and installation format.

#### Scenario: Existing desktop upgrade
- **WHEN** an existing installation upgrades to the renamed public product
- **THEN** workspace files, model/runtime locations, application ID and uninstall identity are preserved
- **AND** visible shortcuts and application labels say OffGrid without creating a new workspace

#### Scenario: Choosing a platform download
- **WHEN** a user chooses a release asset or uses a maintained installer
- **THEN** the selected filename matches the published platform/architecture/format
- **AND** older published release names remain supported without rewriting historical assets

### Requirement: Discoverable application settings
Settings SHALL offer searchable General, Models & responses, Voice, Terminal, Access & data and Advanced/About sections with truthful existing controls and explicit scope.

#### Scenario: Change a scoped preference
- **WHEN** a user changes response or voice preferences and returns to Chat or reloads
- **THEN** the same scoped preference is used and unrelated work, grants and pinned agent profiles remain unchanged

#### Scenario: Preference persistence fails
- **WHEN** browser or native preference persistence fails after a selection is applied
- **THEN** the current session retains that selection and discloses that saving failed, including after leaving and reopening Settings
- **AND** appearance does not report Saved while its native write is pending or failed
- **AND** only the latest presentation save owns its feedback; an older reply cannot erase or introduce a newer failure
- **AND** recovery follows an explicit preference change without replaying a request, enabling audio or changing grants

#### Scenario: Browse without side effects
- **WHEN** a user searches Settings or opens a section
- **THEN** no model load, microphone capture, playback, grant or resource change occurs
- **AND** unavailable desktop controls are explained in the browser

#### Scenario: Localized navigation
- **WHEN** Settings is used by keyboard in any supported locale, a narrow window or RTL
- **THEN** matching settings remain reachable without clipped controls or losing search focus

#### Scenario: Find an individual preference
- **WHEN** a user searches for a displayed response, voice, appearance or shortcut setting and follows its result or saved deep link
- **THEN** the existing owning section opens and focus reaches that control, or its stable explanatory region while the control is unavailable
- **AND** search indexes localized control labels, preserves previous section routes and does not offer desktop-only shortcut settings in the browser
- **AND** focusing a setting never opens its picker, changes its value, starts audio, submits work, changes access or steals focus after a delayed metadata read

### Requirement: Purposeful action and preference hierarchy
Settings SHALL distinguish destination cards from inline preferences and expose full model identities in selection menus while using readable names in compact triggers.

#### Scenario: Inspect response and voice settings
- **WHEN** a user opens Models & responses or Voice
- **THEN** destination actions and preference rows are visually distinct, all descriptions and controls remain accessible in narrow and RTL layouts, and readiness limitations stay visible
- **AND** inspecting or changing presentation does not load a model, start capture, download or alter an agent profile

### Requirement: Editable comparison source selection
Comparison SHALL keep its two source selectors editable alongside the result,
resolve current extraction revisions before a new submission and retain immutable
references when opening history.

#### Scenario: Change the comparison pair
- **WHEN** a user changes either source after a comparison finishes
- **THEN** the old result is hidden until its matching pair is restored or a new comparison succeeds
- **AND** one primary action starts the explicitly selected pair; source updates are revalidated before submission
- **AND** incomplete coverage and source/export access checks remain visible and enforced

### Requirement: Reviewed permanent profile deletion
The owner SHALL be able to review and permanently delete an eligible profile and
its associated standalone saved work and private memory in one atomic operation.

#### Scenario: State changes after review
- **WHEN** exact private-memory identities, task revisions or dependencies change after the owner reviews deletion
- **THEN** the deletion token is rejected without partial removal and a fresh review is required

#### Scenario: Active or independently retained work
- **WHEN** a profile has active work, canonical retained work or dependent profiles, jobs or schedules
- **THEN** deletion is blocked with a useful explanation
- **AND** no administrative role bypasses ownership and shared files, exports and backups remain separate retention domains
