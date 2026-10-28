# Spec Delta

## Purpose

Retain useful agent memory only under explicit live consent and ownership scope,
with predictable concurrent updates, revocation and user inspection controls.

## ADDED Requirements

### Requirement: Explicit versioned memory consent
Automatic memory SHALL default off and be scoped to actor, workspace and profile. New tasks MUST pin the live consent revision. Every model-facing retrieval or write MUST recheck that exact enabled revision. Revocation or re-enabling MUST NOT reauthorize an older task.

#### Scenario: Opt in and revoke
- **WHEN** the user enables memory and later revokes it during a running task
- **THEN** subsequent memory reads and writes by that task are denied
- **AND** re-enabling memory does not restore that task's authority

#### Scenario: No consent
- **WHEN** an old profile or ordinary unconfigured task runs
- **THEN** no automatic durable memory retrieval or capture occurs

### Requirement: Bounded scoped memory operations
Memory reads and writes SHALL run within the canonical task's existing tool and aggregate budgets. Entries MUST record provenance, use bounded text and remain untrusted context. Children and imported instructions MUST NOT acquire independent memory authority or cross-profile access.

#### Scenario: Cross-scope reference
- **WHEN** a task requests memory owned by another actor, workspace or profile
- **THEN** the service discloses no entry and performs no write

### Requirement: Concurrent and replay-safe updates
Memory updates SHALL require an expected entry revision and stable operation identity. Repeating the same operation MUST return its original result, while a changed payload or stale revision MUST fail without overwriting newer memory. Persistence failure MUST NOT be reported as successful capture.

#### Scenario: Competing writes
- **WHEN** two tasks update the same entry from the same revision
- **THEN** only one update commits and the other receives a conflict

#### Scenario: Lost response
- **WHEN** an acknowledged write is retried with its original operation identity and payload
- **THEN** it returns the original result without another update

### Requirement: User control and honest deletion
Users SHALL inspect, edit and delete owned memory and inspect live consent through authenticated CLI and shared application controls. Deleted entries MUST be excluded from subsequent retrieval. Deletion MUST NOT claim to erase historical task evidence or previously delivered context.

#### Scenario: Delete after retrieval
- **WHEN** the user deletes an entry previously included in a task response
- **THEN** future retrieval excludes it while historical evidence remains explicitly historical
