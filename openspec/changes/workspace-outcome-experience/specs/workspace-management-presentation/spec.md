## Purpose

Make documents, work history and preferences useful to ordinary users while retaining clear, authorized access to maintenance and diagnostics.

## ADDED Requirements

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
