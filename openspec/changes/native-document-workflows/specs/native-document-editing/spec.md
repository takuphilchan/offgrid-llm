# Spec Delta

## Purpose

Create and select identifiable documents or tabs, edit their intended text safely,
and verify the result while preserving existing documents and user input.

## ADDED Requirements

### Requirement: Document operations require negotiated host support

The service SHALL expose document operations only when the connected host and
current surface support their semantics. Availability SHALL distinguish reading,
selection, creation, insertion and replacement. Missing features MUST NOT fall
back to arbitrary shortcuts, shell commands, global clipboard substitution or
unrestricted visual input.

#### Scenario: Older host or unsupported editor

- **WHEN** a host lacks the document contract or an editor lacks required structured capabilities
- **THEN** the requested operation reports the unsupported capability without input
- **AND** legacy tools retain their original scope without being claimed as equivalent document support

#### Scenario: Portable supported controls

- **WHEN** supported UIA, Accessibility or AT-SPI controls provide equivalent document capabilities
- **THEN** the shared identity, precondition, Unicode and evidence semantics apply
- **AND** unavailable task-wide launching or Wayland input is not implied

### Requirement: Document identity is separate from its window and editor control

Discovery SHALL identify the active document/tab and its editor separately from
the application window. Switching, replacing or closing a document SHALL
invalidate affected preparations and ranges, even if the window or editor control
is reused. Discovery MUST NOT activate tabs or infer identity from title text.

#### Scenario: Editor reused between tabs

- **WHEN** another tab becomes active while the same editor control remains alive
- **THEN** an edit prepared for the earlier document is rejected before input
- **AND** fresh discovery identifies the current document

#### Scenario: Duplicate titles or tab reordering

- **WHEN** two documents share a title or their tab order changes
- **THEN** selection uses the exact host-issued document identity rather than title or position
- **AND** a stale or ambiguous identity is rejected

#### Scenario: Incomplete discovery

- **WHEN** the host cannot establish a complete active-document/editor binding
- **THEN** it reports incomplete or unsupported document evidence
- **AND** omitted text or tabs are not assumed empty or absent

### Requirement: New document creation preserves existing work

A new-blank-document request SHALL use an authorized supported creation command
and verify the resulting distinct document and complete empty editor before
writing. It MUST NOT clear an existing document, close a tab, accept a discard/save
prompt, or select an arbitrary restored document to simulate creation.

#### Scenario: Existing unsaved documents

- **WHEN** the application already has disposable unsaved documents and the task requests a new blank document
- **THEN** only the newly verified document receives the requested text
- **AND** existing contents remain unchanged with no save or discard action

#### Scenario: Delayed creation or lost acknowledgment

- **WHEN** creation was dispatched but its result is delayed or acknowledgment is lost
- **THEN** OffGrid can inspect for its result without dispatching creation again
- **AND** unresolved creation effects remain uncertain rather than being replayed

#### Scenario: Creation opens a template or confirmation dialog

- **WHEN** the observed command does not produce a provably blank editable document
- **THEN** no text is entered automatically
- **AND** the task reports the actual intervening state without dismissing it blindly

#### Scenario: Browser tab lifecycle

- **WHEN** a supported native browser surface creates or selects a tab
- **THEN** the resulting tab is verified and bound independently of reused address or page controls
- **AND** blank-tab readiness is not evidence of search completion or permission to select a personal profile

### Requirement: Text operations have explicit noninterchangeable semantics

Insertion, selection replacement and whole-field replacement SHALL be distinct
operations. Each SHALL bind a current document, editor, content revision and
relevant caret/selection. Precondition changes MUST reject input. Selection
replacement MUST preserve surrounding text; insertion MUST NOT silently erase an
existing selection or document.

#### Scenario: Insert at a verified caret

- **WHEN** insertion targets an unchanged collapsed selection in the authorized document
- **THEN** the supplied text is inserted at that caret
- **AND** surrounding text is preserved

#### Scenario: Replace a selected range

- **WHEN** replacement targets an unchanged explicit selection
- **THEN** only that range is replaced
- **AND** a changed, invalid or unsupported range produces no input

#### Scenario: Concurrent typing or focus change

- **WHEN** the user types, changes selection, changes tabs or moves focus between preparation and dispatch
- **THEN** stale input is rejected or stopped at the next safe input boundary
- **AND** any input already dispatched is retained as potentially partial rather than undone or repeated automatically

### Requirement: Unicode and text completeness are preserved

Text input and read-back SHALL preserve the supplied Unicode without lossy
encoding, trimming or normalization. Range handling SHALL avoid splitting encoded
characters and unsupported selection boundaries. Truncation SHALL be explicit;
blankness and exact-match claims require complete relevant text evidence.

#### Scenario: Multilingual exact text

- **WHEN** the task enters `OffGrid test — café 日本語 12345` in a new blank document
- **THEN** independent read-back equals that exact text
- **AND** no save or existing-document mutation is used to obtain the result

#### Scenario: Non-BMP and combining text

- **WHEN** selected text includes emoji, combining characters, RTL text or line breaks
- **THEN** edit boundaries preserve their encoded contents across supported providers
- **AND** unsupported provider range mappings are refused rather than approximated

#### Scenario: Read limits or active composition

- **WHEN** text exceeds the supported complete-read limit or composition/selection stability cannot be established
- **THEN** the operation reports its limitation without claiming blankness or successful exact verification
- **AND** it does not overwrite the document to simplify verification

### Requirement: Edits use durable dispatch and document-bound evidence

Document changes SHALL pass existing task authorization, host preparation,
durable dispatch and independent read-back. Evidence SHALL bind the task,
session, document generation, editor, revision and intended result. A write
acknowledgment or echoed arguments MUST NOT count as verified text.

#### Scenario: Stale document with matching text

- **WHEN** another document happens to contain the expected text
- **THEN** its content cannot satisfy the original document's check
- **AND** reused editor identity alone cannot connect the evidence

#### Scenario: Worker crash during insertion

- **WHEN** input may have been partially dispatched before a worker crash
- **THEN** the existing journal and task retain uncertain effects
- **AND** restart, page navigation and model repair cannot repeat insertion

#### Scenario: Cancellation or failed persistence

- **WHEN** Stop is requested or preparation/result persistence fails
- **THEN** no new input is dispatched after acknowledged stopping
- **AND** unsaved evidence is not announced as verified completion

### Requirement: Document workflows remain bounded and transparent

Document tools SHALL use bounded observations and schemas, deterministic host
waiting and task-local references. The current UI SHALL distinguish application
readiness, document readiness, dispatched input and verified text without new
consent popups or duplicate tasks. Private document text MUST NOT enter general
logs or support bundles by default.

#### Scenario: Small-model continuation

- **WHEN** a workflow continues within an 8192-token model allocation
- **THEN** the original instruction, current document and pending evidence remain available within the enforced budget
- **AND** full histories remain retrievable without silently increasing allocation or dropping MCP tools

#### Scenario: Existing task interface

- **WHEN** a user navigates away and returns during document work
- **THEN** the same task and its evidence are displayed through the current UI
- **AND** navigation does not create, edit, approve or replay work
