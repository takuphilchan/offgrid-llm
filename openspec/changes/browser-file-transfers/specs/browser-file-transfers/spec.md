# Spec Delta

## Purpose

Preserve user control and exact-byte evidence when browser tasks select local
files or retain downloaded artifacts, including cancellation and recovery.

## ADDED Requirements

### Requirement: Uploads use the exact locally approved file content
The system SHALL bind one user-selected file to its size, digest and task access
grant. Dispatch SHALL consume a private verified snapshot, never reopen the
original mutable selection. The existing 512 MiB file limit SHALL remain
supported. File paths SHALL remain outside model and renderer authority.

#### Scenario: Source is replaced after snapshot preparation
- **WHEN** the original source is replaced after the approved bytes were captured
- **THEN** selection uses only those captured bytes and the resulting file digest
  agrees with the grant; replacement content is never transmitted

#### Scenario: Source changes before or during preparation
- **WHEN** the source is special, replaced, oversized, unreadable or its captured
  bytes no longer match the grant
- **THEN** preparation rejects before browser input and does not refresh consent

#### Scenario: Large approved file
- **WHEN** a valid approved file exceeds 50 MiB and is at most 512 MiB
- **THEN** the supported transfer path preserves its complete content and name
  without truncation or a silent lower product limit

### Requirement: File preparation preserves Stop and access boundaries
The system SHALL bound preparation, retain cleanup ownership and honor Stop,
revocation, manual takeover and target/document replacement. A discarded file
grant SHALL NOT be restored by resume, navigation or command redelivery.

#### Scenario: Stop during preparation
- **WHEN** Stop arrives while a source is being read or staged
- **THEN** no file is assigned to the browser input and owned temporary resources
  are cleaned up or explicitly retained with an uncertain cleanup state

#### Scenario: Manual renewal does not reuse an old upload grant
- **WHEN** a retained browser resumes with fresh task authority
- **THEN** its old local upload grant remains unusable until the user selects a
  file again through trusted local consent

### Requirement: Download staging is bounded and preserves existing files
The system SHALL associate an admitted download with its exact browser action,
tab/frame and task. It SHALL enforce explicit per-file and aggregate staging
bounds, reject unsolicited or ambiguous transfers, cancel excess transfers and
publish only complete validated artifacts. Existing files SHALL NOT be overwritten.

#### Scenario: Repeated filename or occupied destination
- **WHEN** two downloads suggest the same name or a staging destination exists
- **THEN** each admitted artifact receives a distinct opaque identity, without
  replacing the existing object or trusting a page-supplied filesystem path

#### Scenario: Oversized, interrupted or multiple transfer
- **WHEN** a transfer exceeds its budget, fails, is interrupted or produces more
  than the admitted single artifact
- **THEN** it is cancelled or fenced, incomplete output is not claimed verified,
  cleanup ownership remains explicit and the triggering action is not replayed

### Requirement: Transfer evidence states exactly what was verified
The system SHALL record bounded provenance including source URL treatment,
capture time, task/session/action/document identity, filename, size, SHA-256 and
content-type evidence. It SHALL distinguish file-input selection from remote
receipt, staging from user export, and untrusted declared type from verified bytes.

#### Scenario: Selection succeeds without form submission
- **WHEN** approved bytes are assigned to the native file input
- **THEN** the receipt verifies selection only and does not claim a server upload,
  saved remote document or completed downstream workflow

#### Scenario: Source URL contains secrets
- **WHEN** a download source contains credentials, query values or fragments
- **THEN** public/model provenance redacts them explicitly while an opaque local
  artifact reference retains inspectable ownership and integrity evidence

### Requirement: Artifact access is explicit and task scoped
The system SHALL expose retained browser artifacts through the existing task and
trusted host access surfaces. Inspection/export SHALL revalidate artifact identity
and content, require the owning actor/workspace and an explicit destination, and
never automatically open or execute downloaded content.

#### Scenario: Exported bytes differ from retained evidence
- **WHEN** an artifact is replaced or its size or digest changes before export
- **THEN** export rejects and preserves the discrepancy without silent repair,
  redownload or overwriting the user's destination

#### Scenario: Another task or actor requests an artifact
- **WHEN** a caller presents a foreign or stale artifact reference
- **THEN** access rejects without exposing paths or content

### Requirement: Transfer uncertainty survives restart and compatibility fallback
The system SHALL persist dispatched transfer outcomes before acknowledgement and
retain no-replay tombstones across transport loss, restart and renewal. Stronger
file-integrity claims SHALL require matching negotiated components; legacy
metadata SHALL remain explicitly unqualified.

#### Scenario: Reply is lost after a completed transfer
- **WHEN** a completed transfer's acknowledgement is lost
- **THEN** recovery returns the durable receipt or uncertainty without clicking,
  selecting or downloading again

#### Scenario: Older component lacks transfer integrity support
- **WHEN** the service or host lacks the new negotiated transfer contract
- **THEN** the client reports that boundary and does not advertise the stronger
  verification or bypass existing policy to emulate it

### Requirement: Surviving native owners settle only proven cleanup
The original trusted local stdio supervisor MAY retain its worker's reservation
capability through a validated private startup handoff. It SHALL acknowledge the
exact original binding and existing journal identity before accepting dispatch.
Reservation settlement SHALL require that original endpoint's native descendant
cleanup proof and SHALL NOT convert uncertain operation receipts into success.

#### Scenario: Local worker dies after a complete startup handoff
- **WHEN** the original Linux/Windows supervisor survives and its retained native
  owner proves cleanup of the terminated worker and descendants
- **THEN** it can settle only that original reservation; an explicit later restart
  preserves uncertain operations and does not replay them

#### Scenario: Startup handoff or acknowledgment is incomplete
- **WHEN** the handoff is absent, partial, foreign or rejected before acknowledgment
- **THEN** no recovery authority is reconstructed from inspection, PID or age,
  no dispatch occurs before acknowledgment, and unresolved reservations stay fenced

#### Scenario: Cleanup succeeds but publication fails
- **WHEN** the original surviving endpoint has proven cleanup and the registry
  cannot commit settlement
- **THEN** the reservation remains until that same owner successfully retries
  publication, without re-executing a command or releasing another reservation

#### Scenario: Recovery lacks original journal or native cleanup authority
- **WHEN** the journal is missing/replaced, all original owners are lost, or only
  unsupported/isolated cleanup evidence is available
- **THEN** this local recovery path cannot settle the reservation or create an
  empty journal to infer success
