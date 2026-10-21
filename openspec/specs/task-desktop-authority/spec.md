# task-desktop-authority Specification

## Purpose

Allow one explicitly consented task to discover, open and switch ordinary applications while retaining independent host enforcement, durable action evidence and reliable stopping.

## Requirements

### Requirement: Consent precedes desktop-enabled task execution

A desktop-enabled task SHALL be saved and held before execution and SHALL not dispatch inference or tools until its actor, policy and task-bound local desktop grant are validated. Explicit desktop-session consent may be obtained before task submission. Starting or recovering the same request MUST NOT create another task.

#### Scenario: Start with desktop access
- **WHEN** the user starts a task with desktop scope and Full task access
- **THEN** a valid explicit desktop-session authorization is reused, or local consent is required before execution, without an application picker
- **AND** acceptance continues that same saved task; decline leaves it undispatched

#### Scenario: Explicit session Full access
- **WHEN** the user explicitly enables Full access for this desktop session and accepts local host consent
- **THEN** subsequent full-access tasks in the same actor/workspace use separate task grants without another consent popup
- **AND** default selections, renderer flags, reloads and stored preferences cannot create or restore this authorization
- **AND** local revocation, quit, lock/suspend or actor/workspace change invalidates it before more work starts

#### Scenario: Usable access controls
- **WHEN** the access options exceed the available viewport
- **THEN** wheel, touch and keyboard can reach every option without moving the page or resetting panel scroll
- **AND** desktop-owned Windows control uses the persistent in-app Stop instead of another floating window, including between tasks and while the service is disconnected
- **AND** the independent emergency shortcut and OS-session revocation remain active; loss of the app controls revokes authority

#### Scenario: Renderer reconnect
- **WHEN** the renderer reloads while consent or attachment is pending
- **THEN** it observes the saved request without silently approving, submitting or replaying work

### Requirement: Task authority is separate from a window target

A protocol-v3 desktop grant SHALL cover only its actor, workspace, task, host and signed-in desktop, for at most 60 minutes or 500 mutations. Each input SHALL independently validate its current target and observation. V2 window grants MUST remain window-limited.

#### Scenario: Switch applications
- **WHEN** a consented task focuses another ordinary application in the same desktop
- **THEN** it requires a fresh target observation but not another window permission

#### Scenario: Cross-task or stale target
- **WHEN** a request changes task, actor, OS session, process generation or uses a stale prepared action
- **THEN** input is denied before dispatch

### Requirement: Application lifecycle is a durable capability

Application discovery, opening and focusing SHALL use host-issued identities and the durable intent/result path. Models MUST NOT supply executable paths, command strings or arbitrary launch arguments. Unsupported or protected launch profiles SHALL be refused before dispatch.

#### Scenario: Open Chrome without a URL
- **WHEN** Chrome is installed in a supported launch profile and an authorized task requests opening it
- **THEN** the agent discovers its catalog identity, opens it, and verifies a usable matching surface without asking the user to launch or select it
- **AND** an existing Chrome process can satisfy verification through its matching surface

#### Scenario: Ambiguous application
- **WHEN** multiple distinct installed applications genuinely match the user's request
- **THEN** the task asks which application rather than selecting by a title substring

#### Scenario: Uncertain launch
- **WHEN** acknowledgment is lost after launch dispatch
- **THEN** the same action is not launched again, and uncertainty remains visible

### Requirement: Grant does not bypass operation restrictions

Task-wide scope SHALL preserve credential/payment, financial, privilege/security, installation, script, permanent-deletion and uncertain-retry exclusions. Local stopping, screen lock, logout, permission loss and host restart SHALL revoke input authority independently of the service.

#### Scenario: Stop while disconnected
- **WHEN** local Stop is activated while the service is unreachable
- **THEN** new dispatch becomes impossible before a stopped acknowledgment; an unsettled worker remains visibly uncertain

### Requirement: Current UI and compatibility stay coherent

The current task interface SHALL expose access policy before Start and use a compact desktop-consent path without mandatory app selection. Old hosts MUST report an upgrade requirement for desktop scope, not silently substitute a window session. Existing text, MCP, voice and legacy scoped tasks SHALL remain compatible.

#### Scenario: Missing matching host
- **WHEN** desktop scope is requested with a v2-only host
- **THEN** no desktop action executes and the user receives explicit matching-desktop upgrade guidance

#### Scenario: MCP and desktop together
- **WHEN** a task acquires desktop access after using an authorized MCP connection
- **THEN** its permitted MCP tools remain available without acquiring any extra connection authority

#### Scenario: Cold model readiness
- **WHEN** an access request waits for active inference or a cold model load
- **THEN** admission does not consume the subsequent bounded tool-call probe budget
- **AND** transient failure preserves the pending task with retryable, phase-specific guidance and no host action dispatch

#### Scenario: Readable native consent
- **WHEN** a Windows user explicitly enables session access
- **THEN** the owned native consent uses OffGrid's monochrome styling with scrollable details and visible Allow/Cancel actions on short/high-DPI windows
- **AND** initial session consent is at most 440 by 330 logical pixels, subject to work-area clamping, with full exclusions and technical details expandable, and permission scope, limits, revocation and a restrictions summary visible
- **AND** expanding or collapsing details never grants permission or changes the safe default

#### Scenario: Application catalog freshness
- **WHEN** explicit application discovery follows a transient missing file or an installed application update
- **THEN** reviewed paths are inspected again and unchanged executables retain their IDs
- **AND** replacement executables cannot reuse older IDs or prepared input

#### Scenario: Unverified launch through compaction
- **WHEN** a task's completed history is compacted after dispatching an application launch
- **THEN** the stable system prefix is preserved, the pending verification is retained and another launch of that app is rejected before dispatch until its outcome is verified
- **AND** repeated invalid proposals use bounded corrective feedback, not unbounded new launches
- **AND** initial Enter, Escape, Cancel or closing cannot grant permission, while intentional keyboard activation of Allow remains possible

#### Scenario: Native observation fits the allocated context
- **WHEN** a supported application returns a normal multi-control view after launch, including a 46-control observation of at least 8 KB at the tested 8192-token allocation
- **THEN** context preparation uses a bounded model view with explicit omissions and scoped detail retrieval, and the next control step remains possible without an application-specific workaround or increased allocation
- **AND** canonical results, observation/element identities, text, state and truncation indicators remain intact; host validation and independent outcome checks still apply
- **AND** genuinely oversized context stops before another dispatch rather than dropping user instructions or pretending earlier actions were undone

#### Scenario: Slow model with unchanged Windows structured controls
- **WHEN** inference outlasts observation freshness before a new protocol-v3 Windows control preparation or verification
- **THEN** the host re-observes only exactly retained host-issued state and proceeds only if the complete view, runtime/state digest, control properties and execution target are unchanged
- **AND** it uses a new capture identity/time, retains model-visible element identities only after exact matching and independently enforces dispatch preconditions, policy and replay protection
- **AND** changed, incomplete, forged or foreign views require new observation; v2 behavior is unchanged and neither expired approvals nor uncertain mutations are retried

### Requirement: Progressive observations preserve evidence and authority

Desktop model context SHALL bound control text and rows, expose omission metadata
and task-scoped detail retrieval, and retain original canonical evidence. Retrieval
MUST NOT refresh host observations, authorize input or access another actor's task.
Short model references MUST resolve to exact canonical identities within that task.

#### Scenario: Long URL content
- **WHEN** an observation contains multiple high-entropy URLs and hundreds of controls
- **THEN** the initial model view is bounded, editable controls remain prioritized and omitted content can be retrieved in bounded pages
- **AND** exact host results and user instructions remain unchanged

#### Scenario: Verified launch survives archival
- **WHEN** launch and readiness records leave the current context
- **THEN** application progress remains available and a repeated launch requires a read-only state check before another dispatch
- **AND** an app readiness result counts as an observation but not proof of unrelated requested outcomes

### Requirement: Application discovery is not a product-name allowlist

OS-backed providers SHALL discover ordinary registered applications without adding
product names to source. Launch descriptors SHALL be validated independently of
display names, retain executable identity/replacement checks and exclude arbitrary
commands and unsupported activation mechanisms with actionable limitations.

#### Scenario: Registered application absent from source
- **WHEN** an ordinary installed application has a supported OS registration
- **THEN** it is discoverable and launchable using a host-issued identity without a product-name code change
- **AND** discovery alone does not claim that its workflows are qualified

#### Scenario: Malicious registration
- **WHEN** a registration contains a script, interpreter command, arbitrary arguments or a replaced executable
- **THEN** it cannot become an authorized launch by using a familiar display name
