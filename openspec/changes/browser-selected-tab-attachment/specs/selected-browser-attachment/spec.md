# Spec Delta

## Purpose

Let users grant an OffGrid task access to explicitly selected, already signed-in browser tabs while preserving their browser, existing data and independent task authority.

## ADDED Requirements

### Requirement: Explicit selection precedes attachment
Attachment SHALL require a browser-owned user selection and trusted local confirmation of the same saved task, actor, workspace, policy and exact origin scope. Page content, renderer-supplied tab identifiers and extension installation SHALL NOT grant authority. Unselected tab contents and profile authentication data SHALL NOT be inventoried or exported.

#### Scenario: Selected authenticated tab
- **WHEN** the user selects a supported signed-in tab and accepts matching local task consent
- **THEN** that task can use the selected tab through the canonical browser tools after all required gates pass
- **AND** the same site's unrelated open tabs remain inaccessible

#### Scenario: Page or renderer forges selection
- **WHEN** a page message, title, URL parameter or renderer request supplies another tab or task identity
- **THEN** no attachment is created and no page content or credentials are returned

#### Scenario: Consent changes before dispatch
- **WHEN** the actor, workspace, selected tab generation, pending request, origin set or policy changes while consent is pending
- **THEN** attachment fails before any observation or effect is admitted
- **AND** an older confirmation cannot silently authorize the changed scope

### Requirement: Attachment is task-bound before publication
An attached browser grant SHALL be bound atomically to its actor, service workspace, task, pending request, local owner and selected resource generation before it becomes usable. Initial attachment and manual renewal SHALL preserve recorded policy. A grant SHALL NOT authorize another same-actor task, native driver or different browser connection.

#### Scenario: Same actor with another task
- **WHEN** another task tries to reserve a newly paired attached session before the intended task consumes it
- **THEN** reservation and direct dispatch are denied without changing either task

#### Scenario: Unsupported service or extension
- **WHEN** any component lacks the required attachment contract or reports an invalid version
- **THEN** the user receives a specific compatibility failure before attachment
- **AND** no managed-browser, whole-profile or native-control fallback occurs

### Requirement: Borrowed resources retain user ownership
The system SHALL distinguish user-owned borrowed tabs from task-created tabs and limit a task to eight attached tabs. Additional existing tabs SHALL require explicit user selection. Stop, revocation and cleanup SHALL detach borrowed tabs without closing them or the browser, changing profile settings or clearing authentication state. Task-created tab cleanup SHALL follow the displayed ownership policy.

#### Scenario: Stop with unrelated work open
- **WHEN** Stop is requested while the task uses a borrowed tab and unrelated tabs are open
- **THEN** no new automation is admitted and detachment is reported only when confirmed
- **AND** borrowed and unrelated tabs, cookies, profile data and browser settings remain intact

#### Scenario: Popup or replacement tab
- **WHEN** a page opens a popup, the selected tab is replaced, or another tab enters a visible group
- **THEN** it acquires no authority from its title, opener or grouping alone
- **AND** over-cap or unsupported targets cannot become silent replacements for prepared input

### Requirement: Shared browser evidence remains exact
Attached tools SHALL preserve canonical session, tab, frame, document, observation and action identities, serialized mutation and privacy filtering. Features SHALL be advertised only when the attached provider enforces their existing contract. Stale, opaque, foreign or incompletely covered targets SHALL fail before input or private output; no raw model script or CDP channel SHALL bypass these checks.

#### Scenario: Frame navigation between observation and action
- **WHEN** an attached same-process or out-of-process frame navigates or changes authority after observation
- **THEN** the prepared action is rejected and cannot target a similar element elsewhere

#### Scenario: Unsupported visual or file capability
- **WHEN** the attachment provider cannot establish the existing capture-redaction or file-provenance guarantees
- **THEN** the unavailable feature is explicitly withheld or rejected
- **AND** the broader approved requirement remains incomplete rather than being represented as supported

### Requirement: Attached network enforcement is evidenced
Attachment automation SHALL enforce the approved public-HTTPS origin and network scope without modifying unrelated browser traffic or profile settings. Every advertised protection SHALL have prevention evidence for its supported request paths. Unproved coverage SHALL block dependent automation and remain visible as incomplete; post-effect observation or a URL list SHALL NOT be presented as a proven containment boundary.

#### Scenario: Redirect or private destination
- **WHEN** an admitted page action attempts a disallowed redirect, subresource or private destination
- **THEN** the applicable required boundary prevents the request before its effect
- **AND** it does not grant the destination from page content or silently expand consent

#### Scenario: Existing socket or shared worker
- **WHEN** an already active tab can use a socket, service worker or shared execution path outside proven enforcement
- **THEN** dependent automation is refused with the coverage limitation recorded
- **AND** unrelated user tabs are not disrupted to manufacture isolation

### Requirement: Disconnect and uncertainty never replay
Worker restart, transport loss, debugger detachment, extension disable, browser restart, actor change and Stop SHALL revoke automated dispatch. An admitted effect with a lost outcome SHALL remain uncertain and SHALL NOT replay on reconnect. Duplicate requests SHALL observe an existing receipt where available; changed payloads or bindings SHALL be rejected.

#### Scenario: Effect occurs before the native reply is lost
- **WHEN** the fixture confirms an input effect and its reply is lost before receipt completion
- **THEN** the same action cannot execute again after reconnect, restart or renewal
- **AND** the user sees the actual confirmed or uncertain outcome independently of model prose

#### Scenario: Local detach fails
- **WHEN** detaching an owned debugger session cannot be confirmed
- **THEN** the interface reports unconfirmed cleanup and retains the exact cleanup handle for retry
- **AND** it does not close the user's browser or report successful Stop prematurely

### Requirement: Manual control requires renewed authority
Manual takeover SHALL fence automation immediately and confirm only after admitted work settles and debugger control is released. Continuation SHALL require explicit selection or confirmation of the retained exact tab plus a fresh task-bound grant, invalidate prior observations, and preserve no-replay history. Manual activity SHALL NOT be observed by the agent.

#### Scenario: User signs in during manual control
- **WHEN** the user takes over, authenticates manually and explicitly continues the same task
- **THEN** automation resumes only under renewed authority on the retained tab with fresh observations
- **AND** neither credentials nor old prepared actions are carried into model context

#### Scenario: Old selected tab disappeared
- **WHEN** the original borrowed tab was closed or replaced during manual control
- **THEN** continuation requires a new explicit selection
- **AND** another open tab is never substituted automatically

### Requirement: Installed compatibility and qualification stay truthful
The product SHALL expose accessible localized attach, detach, manual, compatibility and uncertain states. Registration and removal SHALL affect only verified OffGrid-owned artifacts. Synthetic-profile evidence SHALL be distinguished from installed Chrome/Edge, real-account and model/workflow qualification, with each unavailable gate retained as open.

#### Scenario: Another product owns a host registration
- **WHEN** installation or removal encounters a conflicting or changed native-host registration
- **THEN** it preserves that registration and offers explicit recovery without overwriting or deleting it

#### Scenario: Synthetic success without real-profile authorization
- **WHEN** disposable-browser tests pass but installed or real-profile checks are unavailable
- **THEN** evidence identifies exactly which configurations passed and which remain unqualified
- **AND** no real browser profile is opened or installation performed merely to fill the evidence gap
