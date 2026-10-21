# Design

## Context

See [the proposal](proposal.md) and [the repository map](../../../docs/repository-structure.md). The current tree has substantial staged moves, unstaged implementation edits and untracked source; Git status cannot be used as a deletion list. Existing layout and repository checkers pass but do not inspect ignored nested outputs. Public installer wrappers and old documentation URLs have compatibility value. The previous root-output backup is outside this repository.

## Goals / Non-Goals

**Goals:** Preserve a verifiable source snapshot; correct active path drift; remove only confirmed structural redundancy; quarantine enumerated generated output; leave a reproducible evidence and restoration trail.

**Non-Goals:** Behavioral refactoring, changing the public CLI/API or installed application, rewriting archived history, cleaning the Git index, deleting backups, or qualifying native/model workflows.

## Decisions

1. **Use an external, generated inventory.** A small maintenance command records current nonignored source paths, size and SHA-256, Git porcelain status, and the specific ignored-output targets. Full per-file hashes live outside the repository; a compact digest and summary live in change evidence. This avoids a huge tracked manifest or treating staged deletions as files to remove.
2. **Preserve semantic owners.** Update obsolete source metadata and maintained prose after checking active consumers. Keep documented public wrappers, `docs/advanced` compatibility stubs, release history, generated API schema, and installed `web/ui` destination paths. Normalize duplicate ignore rules only when before/after `git check-ignore` fixtures agree. Any overlapping behavior that cannot be proved redundant remains and is recorded, not silently merged.
3. **Verify before quarantining dependencies.** Run relevant checks against the final source bytes while local npm/Python dependencies are present. After validation, freeze source edits, record a final source inventory, then move only validated ignored directories/files to a unique same-volume backup. Post-move checks require no quarantined dependencies; compare final source hashes to the pre-move snapshot.
4. **Use explicit targets and recoverable moves.** Resolve every target below the repository and every destination below `D:\offgrid-llm-backups`, inspect reparse points and tracked/nonignored occupants, then move each target with literal paths. Never recursively delete or operate on broad roots. Preserve private test captures with the backups; no backup is purged in this change.
5. **Do not alter the existing index.** Changes made here remain attributable through a baseline/final Git comparison. No wholesale staging, reset, stash, commit or publication occurs.

## Risks / Trade-offs

- **Dirty moves resemble missing files** → Compare source path/hash inventories, Git rename mappings, and archived change locations before modifying a path.
- **An ignored directory contains user-owned material** → Classify and inspect targets first; quarantine rather than delete, and preserve a restoration manifest.
- **Tests recreate output** → Run dependency-requiring checks before quarantine; freeze source afterwards and use dependency-free post-move checks.
- **Historical paths resemble stale instructions** → Treat active configuration and maintained guidance separately from archives, compatibility stubs and installed destinations.
- **Git remains visibly dirty** → Report the exact cleanup delta; a clean Git status requires a separate reviewed commit request.

## Migration Plan

Record the baseline; reconcile source/config/docs; run source and compatibility checks; freeze and hash source; quarantine validated outputs; rehash source and run dependency-free checks; publish a restoration manifest and verification report. Rollback consists of restoring only listed quarantined targets whose destinations are absent and reverting only this change's source hunks. No installed-user migration is involved.
