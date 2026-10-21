# Spec Delta

## Purpose

Keep current user and contributor guidance discoverable, distinguish it from historical records, and preserve traceable qualification evidence without implying unverified readiness.

## ADDED Requirements

### Requirement: Maintained documentation has clear owners
The repository SHALL provide discoverable architecture, operations, contributing, and quality entry points, with current guidance linked from the documentation index and old maintained paths resolving to the current owner or an explicit compatibility stub.

#### Scenario: Contributor follows an established link
- **WHEN** a contributor follows a maintained link from the root README or documentation index
- **THEN** the destination resolves and identifies the current owner of that guidance.

#### Scenario: Historical release link
- **WHEN** an old release note contains a former path or command
- **THEN** the release note remains historical and is not silently rewritten as current guidance.

### Requirement: Qualification evidence remains distinguishable
The quality index SHALL link capability-specific qualification and release-specific evidence while preserving whether each claim is implemented, automatically tested, or qualified on real hardware.

#### Scenario: Evidence is unverified
- **WHEN** an installed-platform or model check has not run
- **THEN** the maintained quality entry point labels it unverified rather than claiming readiness from a build or fixture test.

#### Scenario: Existing evidence is reorganized
- **WHEN** a long reliability record is split into smaller documents
- **THEN** its substantive results, dates, limitations, and provenance remain findable without changing their status.

### Requirement: Specification and documentation roles are explicit
Contributor guidance SHALL distinguish current specifications, active change artifacts, user documentation, the product roadmap, and release evidence.

#### Scenario: Contributor seeks current behavior
- **WHEN** a contributor needs an authoritative current requirement
- **THEN** the guide directs them to main OpenSpec specs, not an unarchived proposal or a historical release note.

#### Scenario: Change is unfinished
- **WHEN** an active change has incomplete implementation or qualification tasks
- **THEN** its presence does not make the feature a current product guarantee or eligible completed archive.
