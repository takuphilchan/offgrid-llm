# Design

## Context

See proposal.md. Release automation publishes `checksums-vVERSION.sha256`. The scripts currently download payloads directly and install them without verification. Windows copies selected executables; Unix stops every process named offgrid.

## Goals / Non-Goals

Preserve published entry points and asset naming compatibility, and isolate tests from installed data. No installer qualification, model download, signing claim, or service lifecycle redesign.

## Decisions

- Embed small verification functions in each script so downloaded standalone scripts keep working. Exact filename matching and one digest are required; no unverified fallback.
- Copy Windows payload children recursively from a separate extraction directory; do not copy the downloaded archive into the installation. Keep the current elevated target and choose LocalAppData/Programs/OffGrid CLI for user scope.
- Refuse Unix replacement on detected running processes rather than guessing service ownership. Failed process inspection also stops installation.
- Keep installation.md authoritative for desktop selection and platform limitations; link the installer README there and remove unsupported setup shortcuts.

## Risks / Trade-offs

- Checksums share the release trust boundary; they do not establish publisher identity. Document this explicitly.
- Copying an existing install is not transactional. Stop on copy errors and advise retry after closing active work; do not delete data or old installs.
- Unix process-name checks conservatively block unrelated OffGrid instances, but never terminate them.
- Native installation remains unqualified by mocked fixtures; report it as unrun.

## Migration Plan

No data migration. Existing installed applications are unchanged until a user deliberately runs an installer. Old releases lacking a manifest fail with guidance to use verified manual packages.
