# Spec Delta

## Purpose

Recognize and activate installed applications using verified operating-system
identities, including packaged applications, without guessing execution targets.

## ADDED Requirements

### Requirement: Application catalog identities reflect installed registrations

The host SHALL issue opaque identities for supported executable and packaged
application registrations. Registration replacement SHALL invalidate prepared
activation. Models MUST NOT supply executable paths, package identities, launch
arguments or scripts to activate an application.

#### Scenario: Packaged application discovered

- **WHEN** an installed Windows application has a supported package registration
- **THEN** discovery returns a host-issued identity and display name
- **AND** activation resolves through the validated registration rather than a guessed path

#### Scenario: Registration changes after preparation

- **WHEN** the registered executable, package revision or application entry changes before dispatch
- **THEN** the old preparation is rejected without launching the replacement
- **AND** fresh discovery is required

#### Scenario: Unsupported launcher or ambiguous registration

- **WHEN** a launcher requires unsupported arguments or cannot be associated with a verified application identity
- **THEN** it is not advertised as ready for reliable activation
- **AND** distinct matching applications are not silently substituted for each other

### Requirement: Readiness binds the resulting application rather than a launcher filename

Readiness SHALL verify the application identity, signed-in OS session, process
generation and usable surface. It SHALL support validated packaged activation
and existing-process reuse. Matching a title, display name, basename, arbitrary
descendant process or package family alone MUST NOT establish readiness.

#### Scenario: Modern Notepad activation

- **WHEN** the OS-registered Notepad entry activates its verified packaged application
- **THEN** its usable window is recognized despite a different executable path from a legacy redirector
- **AND** readiness does not launch another Notepad instance

#### Scenario: Similar or forged window

- **WHEN** an unrelated process presents the same application title or executable basename
- **THEN** it cannot satisfy the requested application's readiness check
- **AND** it cannot receive document or text input through that check

#### Scenario: Existing process and multiple surfaces

- **WHEN** activation reuses a verified running application with multiple windows
- **THEN** returned surfaces retain distinct identities and deterministic discovery
- **AND** readiness does not claim that any particular existing document is new or blank

### Requirement: Readiness reports bounded actionable states

Readiness SHALL distinguish ready, still starting, unavailable surface,
unsupported identity, and unavailable observation where determinable. Polling
SHALL be bounded and cancellable. A failed check MUST NOT report successful
readiness, repeatedly activate the app or fabricate an empty usable target.

#### Scenario: Activation acknowledged without a usable surface

- **WHEN** an activation was acknowledged but no verified observable window becomes ready within the deadline
- **THEN** the task retains activation evidence and receives a structured readiness reason
- **AND** subsequent recovery inspects state without repeating activation

#### Scenario: Revocation during readiness

- **WHEN** the desktop grant is revoked or the signed-in session changes during polling
- **THEN** polling stops and the grant remains unusable
- **AND** the failure is not converted into an implicit new grant

### Requirement: Identity support does not imply platform authority

Application identity additions SHALL preserve existing consent scope and legacy
protocol behavior. A host SHALL advertise only implemented identity and activation
functions. Unsupported platforms MUST NOT acquire task-wide launching through
shared contract fields or capability flags.

#### Scenario: Older or selected-window host

- **WHEN** the client connects to a host without packaged activation support or with only selected-window consent
- **THEN** unsupported activation is unavailable with an actionable reason
- **AND** existing supported selected-window operations remain available without broader authority
