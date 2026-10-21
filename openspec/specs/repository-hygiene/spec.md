# repository-hygiene Specification

## Purpose

Keep the maintained repository free of generated artifacts and expose one discoverable contributor and security entry point without changing product behavior.

## Requirements

### Requirement: Generated output is not tracked
The repository SHALL reject tracked build binaries, caches, test results, and generated Go source in output directories while retaining those locations for ignored local output.

#### Scenario: Tracked output is detected
- **WHEN** a tracked file appears under `bin/`, `build/`, `dist/`, a cache directory, or a test-result directory
- **THEN** the repository check fails and reports the offending path.

#### Scenario: Local output stays usable
- **WHEN** a contributor builds to `bin/` or `build/` or runs tests that create ignored cache files
- **THEN** the files remain usable locally and do not appear as new tracked source.

### Requirement: Go package discovery excludes output
The repository SHALL prevent temporary generated Go source from becoming a package under a build or work-output directory.

#### Scenario: Package discovery
- **WHEN** `go list ./...` runs on the maintained source tree
- **THEN** no discovered package path is rooted below `/build/` or `/_work/`.

### Requirement: Project guidance is discoverable
The repository SHALL expose maintained contribution, security, conduct, support, issue, and pull-request guidance at standard root or GitHub community locations.

#### Scenario: Contributor seeks guidance
- **WHEN** a contributor opens the repository root or a new GitHub issue or pull request
- **THEN** the maintained entry point and relevant template are available without following obsolete `dev/` paths.

#### Scenario: Security report
- **WHEN** a reporter finds a vulnerability
- **THEN** the guidance directs them to private reporting rather than a public issue or general discussion.

### Requirement: Historical records remain historical
Repository cleanup SHALL not rewrite historical release notes, Git history, or installed-user state.

#### Scenario: Cleanup completes
- **WHEN** obsolete development files and tracked output are removed from the current tree
- **THEN** historical release text, prior commits, and user data remain unchanged.

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
