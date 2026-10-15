# Spec Delta

## Purpose

Let users configure reusable agents whose instructions and authorized resources follow their conversations without creating a separate execution system.

## ADDED Requirements

### Requirement: Actor-owned versioned profiles
The system SHALL let an authorized actor create, inspect, edit and archive their profiles. A profile SHALL contain instructions, an installed model preference, authorized Knowledge and tool references, and bounded execution settings. Each accepted edit MUST create an immutable revision; concurrent edits MUST detect conflicts.

#### Scenario: Editing a profile during work
- **WHEN** a user changes the instructions of a profile used by an active turn
- **THEN** the active turn retains its recorded revision and subsequent turns can select the new revision

#### Scenario: Archive and ownership
- **WHEN** an actor archives their profile or attempts to access another actor's profile
- **THEN** archived revisions remain inspectable through authorized historical work, new selections exclude them, and cross-actor access is denied

### Requirement: Configuration cannot confer authority
Profiles MUST reference credentials indirectly and MUST NOT store secrets. Effective tools SHALL be the intersection of workspace availability, actor authority, profile and conversation restrictions, current grants and runtime readiness. Changing instructions MUST NOT override execution policy. Connection administration and connection use SHALL remain distinct permissions.

#### Scenario: Revoked or changed connection
- **WHEN** a referenced connection is revoked or its tool schema changes during a turn
- **THEN** execution rechecks the current identity and authority and does not reuse an approval for a different schema

#### Scenario: Instructions request broader permissions
- **WHEN** profile instructions or a tool result request unrestricted host access
- **THEN** existing grants and policy still bound execution and the content does not create authority

### Requirement: Explain effective configuration
The system SHALL expose the selected profile revision, model, enabled resources and actionable unavailability reasons. Missing models MUST NOT be silently downloaded or replaced. Administrator-configured workspace connections SHALL be identified as such rather than presented as the actor's personal accounts.

#### Scenario: Model removed after configuration
- **WHEN** a selected profile references an uninstalled model
- **THEN** submission explains the missing model and lets the user explicitly choose an installed alternative

### Requirement: Bounded delegation
Configured delegation SHALL use the existing parent-child task mechanism. Children MUST inherit only a strict subset of parent capabilities and remain within aggregate budgets. Parent tasks SHALL own approvals, narration and results; children MUST NOT acquire desktop input or broader permissions.

#### Scenario: Research children
- **WHEN** a parent delegates two read-only research subtasks
- **THEN** their authorized findings return to the same parent work and child requests for desktop input or broader tools are rejected

#### Scenario: Research-to-document specialist handoff
- **WHEN** a parent delegates source research to one pinned profile and a dependent document draft to another
- **THEN** the writer receives the completed research as untrusted evidence, the parent receives the draft, and only the parent may perform and verify a document edit under its own authority
