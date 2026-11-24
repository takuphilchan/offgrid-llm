# Proposal

## Why

Workspace archives currently contain conversations, execution history, Knowledge,
WAL files and credentials as plaintext ZIP files. The approved roadmap requires
encrypted backup and restore under M1/M7 without weakening recovery, exposing a
plaintext intermediate archive or treating source tests as installed qualification.

## What Changes

- Add interoperable authenticated age encryption around the existing workspace
  ZIP format, including recipient-key and bounded passphrase operation. Preserve
  explicit legacy ZIP backup/verify/restore compatibility and identify the actual
  format in results; encryption failures never fall back to plaintext.
- Stream encrypted backup creation directly to a private ciphertext temporary
  file and verify the entire authenticated archive before no-replace publication.
  Restore reads encrypted ZIP content without a plaintext archive temporary file.
- Preserve stopped-workspace ownership, strict paths/inventory/digests, bounded
  expansion, application-version checks, SQLite/WAL integrity and restoration to
  a fresh directory. Corruption, cancellation and uncertain publication never
  overwrite installed state or replay work.
- Extend maintenance CLI commands, including the recovery backup made before
  `rebuild-knowledge`, with explicit encryption/identity inputs and safe secret
  handling. Existing command syntax remains compatible.
- Add administrator recovery controls after shared storage/CLI correctness passes,
  with narrowly scoped trusted-host operations, explicit local destinations,
  cancellable progress and accessible/localized outcomes. This UI is required
  follow-through, not completed by the CLI increment.
- Record executable corruption, fault, interoperability and cross-platform
  qualification procedures and evidence. Unavailable platforms and installed
  recovery checks remain open rather than being represented by fixtures.

## Capabilities

### New Capabilities

- `workspace-backup-recovery`: confidential, authenticated, bounded workspace
  backup and staged restoration across maintenance CLI and administrator controls.

### Modified Capabilities

None. Existing execution recovery, desktop trust, distribution and migration
requirements retain their owners; this change implements their shared encrypted
workspace archive boundary rather than introducing another storage system.

## Impact

Primary code is `internal/storage/backup.go` and
`cmd/offgrid/workspace_commands.go`, existing storage recovery tests and new
encryption/CLI tests. Pin and review the age Go dependency and transitive changes.
Administrator controls later touch the trusted desktop maintenance bridge and
shared renderer; they must reuse the storage implementation rather than add a
live HTTP archive endpoint that bypasses workspace ownership.

Update `docs/advanced/workspace-recovery.md`, relevant CLI/desktop guidance and
dated evidence. The roadmap remains authoritative. Signed offline installation
packs, automatic updates and general migration orchestration retain their existing
program owners and are not made complete by this change. No live workspace access,
installation, model download, commit, publication or release is authorized here.
