# Spec Delta

## Purpose

Keep the maintained repository free of generated artifacts and expose one discoverable contributor and security entry point without changing product behavior.

## ADDED Requirements

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
