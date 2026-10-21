# Spec Delta

## ADDED Requirements

### Requirement: Source size has a responsibility-based ratchet
The repository SHALL reject oversized new handwritten production files without review, enforce a 150-line maximum for command entry files, and require owned, dated exceptions for cohesive existing files above the review threshold.

#### Scenario: New oversized source
- **WHEN** a new handwritten production file exceeds 600 lines without a reviewed exception
- **THEN** the repository size check fails with the path and count.

#### Scenario: Existing cohesive exception
- **WHEN** an existing file above 600 lines has an allowlist entry naming its owner, responsibility, and next review date
- **THEN** the checker permits the current size but rejects growth beyond the recorded baseline unless reviewed again.

#### Scenario: Generated or historical content
- **WHEN** a generated schema, lockfile, catalog, or historical record exceeds the review threshold
- **THEN** the production-source size ratchet does not classify it as a handwritten production file.

### Requirement: Layout checks tolerate staged removals
Repository path checks SHALL inspect current readable sources and staged additions without treating an index entry scheduled for removal as a missing working-tree file.

#### Scenario: Staged move
- **WHEN** a contributor stages a source move and the old indexed path no longer exists
- **THEN** the checker evaluates the new path and reports real stale references rather than failing to read the removed path.
