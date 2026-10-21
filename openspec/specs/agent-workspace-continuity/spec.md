# agent-workspace-continuity Specification

## Purpose

Keep the current Agents experience stable across service/storage failures without
silently switching to an obsolete execution interface or losing scoped work.

## Requirements

### Requirement: One current Agents workspace

The shared browser/desktop renderer SHALL contain one task-work interface.
Missing capability metadata or an unavailable service MUST NOT expose the retired
mode picker, legacy composer, result panel or polling loop.

#### Scenario: Unsupported service
- **WHEN** the connected service cannot confirm support for the current task workflow
- **THEN** the user sees a localized recovery state rather than legacy task controls
- **AND** tools/connections administration remains separately accessible where permitted

#### Scenario: Previously supported workspace becomes unavailable
- **WHEN** identity or history reads fail after the current task workspace has mounted
- **THEN** its draft, selection and current presentation are preserved for the same actor/workspace
- **AND** no task is submitted, approved or replayed by recovery

### Requirement: Supported contracts are distinct from health

Implemented task contracts SHALL remain advertised when their storage is
unavailable. Task endpoints MUST continue to reject unsafe operations on unhealthy
storage rather than claim success or initialize empty replacement history.

#### Scenario: SQLite initialization failure
- **WHEN** a configured task store fails to open
- **THEN** identity still reports the implemented task workflow contract
- **AND** task requests return a storage-unavailable error

### Requirement: Retirement preserves installed data and current styling

Removal SHALL be limited to obsolete presentation and exclusively used helpers.
Saved work, supported APIs, data migrations and current access enforcement MUST
remain intact. Current monochrome styling and all interface locales SHALL remain.

#### Scenario: Historical tasks and management routes
- **WHEN** the updated UI opens saved tasks, tools or connections
- **THEN** it uses the current components and existing service contracts
- **AND** removing the legacy page deletes no user data
