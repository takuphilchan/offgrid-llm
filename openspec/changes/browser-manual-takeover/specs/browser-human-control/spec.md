# Spec Delta

## Purpose

Let a user safely take control of an owned task browser, finish manual steps and
explicitly return its preserved context to the same authorized work.

## ADDED Requirements

### Requirement: Confirmed manual handoff
Takeover SHALL block new automated reads and effects immediately and confirm
manual control only after previously admitted work has settled. It SHALL preserve
the owned browser tabs and distinguish requested, confirmed and unconfirmed
handoff. Dispatched outcomes SHALL remain truthful and SHALL NOT replay.

#### Scenario: In-flight input at takeover
- **WHEN** a user takes over while a browser input is already running
- **THEN** no subsequent automation starts and handoff waits for its outcome
- **AND** an unconfirmed outcome remains uncertain without a replacement input

#### Scenario: Manual authentication
- **WHEN** handoff is confirmed and the user signs in on an approved fixture site
- **THEN** the task tabs and session state remain available to the user
- **AND** agent reads, captures and input stay disabled until explicit continuation

### Requirement: Renewed continuation authority
Continuing a retained browser SHALL require explicit local consent and new
service pairing for the same actor, workspace and task access request. It SHALL
retain the approved origin/network scope, clear old observations and preserve
manual tab/session changes. Missing or changed authority SHALL reject continuation.

#### Scenario: Same browser after sign-in
- **WHEN** the user explicitly continues the paused task after manual sign-in
- **THEN** new authority can observe the preserved tabs and authenticated state
- **AND** old element, cursor, capture and session references cannot authorize work

#### Scenario: Another actor or stale request
- **WHEN** a different actor, workspace or task tries to reuse the retained browser
- **THEN** continuation is rejected before pairing or dispatch
- **AND** no website content or authentication material is returned

#### Scenario: Lost resume response
- **WHEN** the new session is created but its response is lost
- **THEN** recovery observes that same session and does not pair or navigate again

#### Scenario: Renewed grant publication
- **WHEN** a new task-bound browser pairing is consumed
- **THEN** its original actor, run, pending input, workspace, origins and policy are enforced before the grant is visible
- **AND** another task or native executor cannot consume or reserve that authority

### Requirement: Retained browser remains owned and stoppable
Manual retention SHALL NOT survive host/renderer loss or automatically restore
automation after restart. Stop SHALL close the owned resource and remain available
during handoff, manual control and service outage. Close or failed continuation
SHALL NOT select or modify unrelated tabs or personal profiles.

#### Scenario: Stop while the service is offline
- **WHEN** the service is unavailable and the user stops a retained manual browser
- **THEN** local owned cleanup remains available and reports actual confirmation
- **AND** no new service authority is created

#### Scenario: A manually closed selected tab
- **WHEN** the user closes the formerly selected task tab during manual control
- **THEN** continuation requires fresh selection where necessary
- **AND** no old prepared action is retargeted to another tab

#### Scenario: Failed local close
- **WHEN** closing an owned browser fails and the process remains alive
- **THEN** Stop remains unconfirmed and the owner retains the exact cleanup resource
- **AND** a later explicit Stop can retry without affecting another task

### Requirement: Accessible truthful handoff controls
The existing task surface SHALL show localized keyboard-accessible handoff,
manual-state, continuation and Stop feedback. Unsupported hosts SHALL preserve
their existing safe lifecycle and SHALL NOT claim retained-browser continuation.

#### Scenario: Old host or unconfirmed handoff
- **WHEN** the client lacks retained-browser support or local handoff cannot be confirmed
- **THEN** the visible state accurately describes the available recovery
- **AND** it never says manual control is ready merely because a request was sent
