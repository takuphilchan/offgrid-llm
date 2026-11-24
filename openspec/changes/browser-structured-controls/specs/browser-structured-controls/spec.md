# Spec Delta

## Purpose

Find and operate controls in authorized browser documents while retaining exact
frame identity, bounded evidence and independently checked effect outcomes.

## ADDED Requirements

### Requirement: Search is literal bounded and observation bound
Browser discovery SHALL support literal name/text queries and semantic role
filters, with bounded pages and explicit continuation/coverage. Search MUST NOT
execute supplied scripts/selectors, choose an ambiguous control, refresh an
expired observation or authorize input.

#### Scenario: Desired control is beyond the first page
- **WHEN** a matching Unicode-named control occurs after hidden or unrelated controls
- **THEN** bounded search can reach its exact observation-bound identity
- **AND** an empty intermediate page retains continuation and incomplete coverage

#### Scenario: Query identity or document changes
- **WHEN** a continuation is reused with a different query, tab or replaced document
- **THEN** discovery rejects it without input or disclosure from another target

### Requirement: Modern control changes are verified
Open shadow-root and contenteditable controls SHALL be discoverable and operable
through exact observed identities. Supported custom control state changes MUST
use typed preconditions and independent read-back. Unsupported/closed structures
MUST be reported honestly. Existing sensitive-field and consequential-action
policies SHALL remain enforced.

#### Scenario: Shadow-root form changes without events
- **WHEN** a relevant field property or host/target semantics change silently
- **THEN** earlier input is rejected before dispatch
- **AND** an unrelated decorative change alone does not invalidate the target

#### Scenario: Editable multiline content
- **WHEN** authorized fill updates a contenteditable field with multiline Unicode text
- **THEN** read-back checks the exact text and reports any mismatch or uncertainty
- **AND** a failed post-action observation does not erase the effect receipt

#### Scenario: Custom checkbox and expanding combobox
- **WHEN** a custom checkbox exposes supported checked semantics or a combobox exposes newly rendered options
- **THEN** the task can observe, target and verify the supported interaction
- **AND** site labels do not make arbitrary clicks reversible or authorize payment/credential entry

### Requirement: Frames retain exact document authority
Frame discovery and selection SHALL be bound to the owned task tab and approved
origins. Nested frames MUST have stable frame identities and replaceable document
identities. Navigation, detachment or parent replacement MUST invalidate prior
references without silently selecting another frame or expanding origin scope.

#### Scenario: Two matching controls in nested frames
- **WHEN** two approved frames contain identically named controls
- **THEN** discovery identifies their distinct frame/document ownership
- **AND** input affects only the explicitly selected observed control

#### Scenario: Same-address frame reload and detach
- **WHEN** the selected frame reloads at the same address or is detached
- **THEN** old capture, search and input references are rejected before input

#### Scenario: Unapproved or opaque frame
- **WHEN** a frame is outside the approved origin set or its origin cannot be established
- **THEN** the task cannot read or act within it and coverage reports its unavailability
- **AND** an iframe cannot override the existing network boundary

### Requirement: Structured extensions preserve compatibility and privacy
New tools SHALL require explicit version negotiation through the existing
service and companion. Results MUST preserve transport bounds, protected-field
filtering and exact provenance. Captures MUST account for protected nested-frame
content; unsupported masking MUST fail closed or visibly mask the entire frame.

#### Scenario: Older companion
- **WHEN** a companion has only the existing discovery/workspace features
- **THEN** legacy tools remain usable and new structured tools are not offered

#### Scenario: Secret field inside a child frame
- **WHEN** a capture includes a child frame containing a protected field
- **THEN** the captured output cannot expose that field through missing frame masking
