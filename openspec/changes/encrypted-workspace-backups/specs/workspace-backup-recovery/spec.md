# Spec Delta

## Purpose

Protect workspace backups containing private history and credentials while
preserving independently verifiable, bounded recovery into a new directory.

## ADDED Requirements

### Requirement: Explicit interoperable encryption
Workspace maintenance SHALL support age-encrypted format-1 ZIP archives using
native recipient keys or a passphrase. It MUST identify encryption status
truthfully, retain legacy ZIP compatibility and never fall back from requested
encryption to plaintext.

#### Scenario: Recipient backup round trip
- **WHEN** an operator backs up a stopped workspace to native age recipients
- **THEN** each selected recipient can decrypt the published archive with a compatible independent age implementation
- **AND** OffGrid verifies and restores the same inventory without exposing a plaintext archive temporary file

#### Scenario: Legacy archive compatibility
- **WHEN** an operator explicitly uses the existing plaintext backup syntax or verifies/restores a legacy format-1 ZIP
- **THEN** existing semantics remain supported and the result identifies the archive as unencrypted
- **AND** providing encryption-only options never silently selects this legacy path

#### Scenario: Filename is not an encryption claim
- **WHEN** archive content and filename extension disagree
- **THEN** verification uses validated content and reports its actual format
- **AND** a requested encrypted output is not published as plaintext under any filename

### Requirement: Bounded secrets and key derivation
Secret inputs MUST be bounded, excluded from command arguments, environment
configuration, logs, manifests and persisted progress. Passphrase mode MUST be
exclusive of recipient mode, confirm interactive backup input and enforce a
documented maximum derivation cost before expensive work.

#### Scenario: Passphrase automation
- **WHEN** an operator explicitly supplies a passphrase through the documented standard-input mechanism
- **THEN** input is limited to the documented size, invalid or excess input is rejected, and the secret is not echoed or recorded
- **AND** an absent secret never triggers an invisible noninteractive prompt or plaintext fallback

#### Scenario: Hostile key derivation header
- **WHEN** an archive requests a passphrase derivation cost above the supported ceiling
- **THEN** verification and restoration reject it before allocating that derivation's memory
- **AND** cancellation remains observable without starting further archive work

#### Scenario: Malformed or excessive keys
- **WHEN** identity or recipient files exceed documented byte/count limits or contain unsupported key types
- **THEN** the operation fails before publication without including private key contents in diagnostics

### Requirement: Exclusive and complete source snapshot
Backup MUST acquire existing exclusive workspace ownership and include all
supported regular files under the explicit data root except its ownership lock.
It MUST refuse active ownership, source changes, unsupported files and unsafe
paths rather than silently omit data. Destinations MUST be outside that root.

#### Scenario: Service still owns the workspace
- **WHEN** backup or a pre-rebuild backup cannot acquire workspace ownership
- **THEN** it refuses the operation without stopping the service or changing source data

#### Scenario: Source changes or is unsupported
- **WHEN** a file changes during copying, or the source contains a symlink, special file or nonportable/colliding path
- **THEN** no usable archive is published and the failure identifies the affected category

### Requirement: No plaintext intermediate archive
Encrypted backup and restore MUST avoid plaintext archive files on disk.
Ciphertext staging MUST be private. Restored plaintext data MUST be created only
inside a fresh private restore stage after complete archive authentication, then
published only after all content and database checks pass.

#### Scenario: Inspect temporary storage during encrypted operations
- **WHEN** encrypted backup, verification or restoration pauses at any testable processing boundary
- **THEN** temporary archive files contain ciphertext only and no plaintext archive exists
- **AND** any extracted plaintext is confined to the operation's private restore stage

### Requirement: Whole archive authentication and bounded validation
Before accepting an encrypted archive, maintenance MUST authenticate every
encrypted chunk and its final length, not merely ZIP entries visited by extraction.
It MUST enforce bounded archive/metadata/expansion processing and preserve strict
manifest inventory, ZIP checksum and per-file SHA-256 validation.

#### Scenario: Corrupt data outside visited ZIP entries
- **WHEN** a header, middle chunk, unvisited ZIP region or final chunk is tampered with, removed, appended to or truncated
- **THEN** verification and restoration fail without publishing a restored workspace

#### Scenario: Valid encryption contains an invalid archive
- **WHEN** authenticated plaintext contains traversal, aliases, duplicates, unexpected entries, oversized metadata, decompression bombs or inconsistent manifest sizes/digests
- **THEN** validation rejects it within documented resource bounds before publishing output

### Requirement: Verify before safe publication
Backup and restore MUST publish only fully verified output at a previously
nonexistent destination. Write, close, flush, synchronization, cancellation and
publication races MUST preserve existing destination data. A post-publication
durability failure MUST report the observed outcome without destructive rollback
or automatic retry.

#### Scenario: Another process creates the destination
- **WHEN** a destination appears after initial checks and before publication
- **THEN** publication fails without overwriting, merging or removing the other process's data

#### Scenario: Cancellation or storage failure
- **WHEN** cancellation or a storage error occurs before publication
- **THEN** no final output is advertised as usable and only operation-owned temporary state is cleaned up
- **AND** cleanup failure reports retained temporary paths without exposing contents or claiming secure erasure

#### Scenario: Crash during publication
- **WHEN** the process crashes around final publication
- **THEN** inspection distinguishes an absent, verified published or unresolved output
- **AND** recovery never replaces existing output, deletes unrelated state or automatically repeats an uncertain operation

### Requirement: Usable staged database recovery
Restore MUST preserve the exact application-version gate and verify copied
SQLite databases, committed WAL state and foreign keys before publishing a new
workspace. It MUST leave the original workspace unchanged and MUST NOT start
services or resume active/uncertain tasks automatically.

#### Scenario: Recover committed WAL state
- **WHEN** a valid encrypted backup includes committed state still resident in WAL files
- **THEN** restored database checks observe those committed records before publication
- **AND** restored interrupted or uncertain jobs remain inspection/recovery work without automatic dispatch

#### Scenario: Wrong version or corrupt database
- **WHEN** the application version mismatches, is an unsupported development build, or staged database integrity fails
- **THEN** restoration fails without publishing the stage or modifying the original workspace

### Requirement: Encrypted recovery before Knowledge rebuild
Knowledge rebuild MUST use the same selected backup format, recipient/passphrase
policy and verification path as ordinary backup. Requested encrypted recovery
must be successfully published before any rebuild mutation starts.

#### Scenario: Encrypted pre-rebuild backup fails
- **WHEN** a requested encrypted recovery backup fails or is cancelled
- **THEN** Knowledge rebuild does not start and existing sources/index state remain unchanged

#### Scenario: Rebuild fails after verified backup
- **WHEN** rebuild fails after its encrypted backup has been verified and published
- **THEN** the backup remains available, failure identifies its location without secrets, and no automatic restore overwrites newer data

### Requirement: Trusted administrator recovery controls
Administrator UI SHALL expose backup, verify and staged restore with explicit
workspace/destination selection, encryption status, progress, cancellation and
verified outcomes. Host operations MUST preserve local IPC, actor and ownership
boundaries; connected-service metadata alone MUST NOT grant host filesystem
access. Controls MUST meet existing accessibility and localization requirements.

#### Scenario: Desktop-managed workspace maintenance
- **WHEN** an authorized administrator selects a supported desktop-owned workspace and explicit destination
- **THEN** trusted host controls execute the same verified maintenance operations after safe ownership handoff
- **AND** untrusted frames/renderers cannot substitute paths, capture persisted secrets or invoke generic filesystem/command execution

#### Scenario: External service or unavailable host permission
- **WHEN** the selected backend is externally managed or its trusted maintenance capability is unavailable
- **THEN** the UI reports the required host-local recovery procedure without silently stopping that backend or guessing local paths
- **AND** unavailable installed-edition UI execution remains an open qualification requirement

#### Scenario: Owned service cannot confirm drain
- **WHEN** its private stdin shutdown channel closes but the existing application drain returns an error, a signal terminates the process, or the deadline expires
- **THEN** the trusted host reports an unconfirmed handoff and does not start backup or restore
- **AND** forced termination is never treated as a successful drain

#### Scenario: Private maintenance framing and cancellation
- **WHEN** the trusted host invokes its explicit versioned maintenance child
- **THEN** bounded start/cancel frames select exactly one storage operation with ephemeral credentials, and EOF cancels outstanding work
- **AND** malformed, duplicate, oversized or mismatched frames cannot start additional operations or expose secrets
- **AND** compact results preserve publication/retained-state truth without forwarding full manifests

#### Scenario: Stale local authority or restore activation
- **WHEN** a picker or confirmation completes after actor, service, child, workspace, selected path or window authority changes
- **THEN** no maintenance process starts under the stale selection
- **AND** successful restore never activates the restored directory or automatically restarts/replays work

#### Scenario: Restored endpoint approvals require fresh review
- **WHEN** historical archive bytes and staged databases have been verified
- **THEN** restore replaces historical activation metadata with a fresh 32-byte random epoch encoded as 64 lowercase hexadecimal characters through the retained private stage root before synced publication
- **AND** archived enabled endpoint decisions cannot silently undo later revocation, even when the workspace identifier is retained
- **AND** the returned manifest describes the verified original archive rather than claiming the new epoch matches historical digests; the next backup inventories the actual new epoch and policy
- **AND** failure before publication leaves the target unactivated and retains the existing owned-stage cleanup/no-overwrite rules

### Requirement: Evidence and qualification remain distinct
Completion MUST include actual storage/CLI tests, independent age interoperability,
failure/cancellation evidence, administrator UI checks and supported-platform
recovery qualification. Fixtures MUST NOT substitute for installed-user recovery,
human usability, unavailable hardware or the wider M1/M7 program gates.

#### Scenario: A required platform is unavailable
- **WHEN** source tests pass but a required installed-platform restore or UI check cannot be run
- **THEN** its runnable procedure and missing prerequisite are recorded, and that requirement and overall change remain incomplete
