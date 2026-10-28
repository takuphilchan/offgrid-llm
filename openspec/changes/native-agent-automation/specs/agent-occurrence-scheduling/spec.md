# Spec Delta

## Purpose

Run explicitly reviewed recurring agent work through canonical jobs while keeping
calendar occurrences, permissions and recovery durable and independently visible.

## ADDED Requirements

### Requirement: Owned reviewed calendar schedules
Users SHALL create, inspect, revise and disable owned daily/weekday schedules with an explicit IANA timezone and wall-clock time. Each revision MUST pin its profile, model, budgets, resources and execution grant. Creation or inspection MUST NOT execute work. Existing records MUST NOT become enabled schedules automatically.

#### Scenario: Reviewed schedule
- **WHEN** a user saves an owned profile, selected host and calendar policy
- **THEN** the service records the exact approved revision and next UTC occurrence without creating a job immediately
- **AND** changing a profile does not silently upgrade the schedule

### Requirement: Calendar discontinuities are explicit
Nonexistent DST wall times SHALL be skipped; repeated wall times SHALL select only their first instant. Missed slots during downtime or forward clock jumps MUST be recorded without catch-up execution. Backward clock changes MUST NOT repeat an occurrence.

#### Scenario: DST transition
- **WHEN** a daily wall time falls in a spring-forward gap or an autumn repeated interval
- **THEN** the gap has no dispatch and the repeated interval has at most one occurrence

#### Scenario: Offline service
- **WHEN** the service resumes after one or more scheduled slots
- **THEN** it records missed work and computes a future slot rather than executing the backlog

### Requirement: Durable non-overlapping canonical admission
The service SHALL persist a unique occurrence identity before job creation or dispatch and link at most one canonical root. Prior unsettled work MUST block overlap, including across schedule revisions. The same runner, inference admission and aggregate budgets SHALL govern scheduled and interactive work.

#### Scenario: Duplicate tick or crash
- **WHEN** a tick repeats or the service crashes between claim, job creation and linkage
- **THEN** inspection reconciles saved identities without creating another root or replaying uncertain work

#### Scenario: Previous work awaiting intervention
- **WHEN** a prior occurrence is running, awaiting approval or interrupted with unsettled effects
- **THEN** a later occurrence is recorded as skipped/blocked rather than overlapping it

### Requirement: Schedule permission is a restriction not a grant
Each admission and action MUST recheck live actor policy, schedule revision, pinned resources and provider authority. Revocation, host loss or changed grant identity MUST block new dispatch. Re-enabling MUST NOT restore an old job's schedule authority. Schedules MUST NOT obtain desktop consent, new folders, replacement models or unrestricted execution.

#### Scenario: Host disconnected
- **WHEN** the pinned execution host is unavailable at its scheduled slot
- **THEN** the occurrence records a blocked-host reason without using service files or another host
- **AND** reconnect does not replay that occurrence

#### Scenario: Disable during work
- **WHEN** a user disables a schedule while a job is active
- **THEN** no subsequent action may use its revoked schedule revision
- **AND** already-dispatched effects remain subject to existing settlement and reconciliation

### Requirement: Activity and client controls preserve truth
Authenticated CLI and shared application controls SHALL expose schedule policy, next time, occurrence history, blocked/missed reasons and canonical result links. Inspection and refresh MUST NOT submit, approve or resume work. Completed job status MUST NOT imply broader independent verification than its evidence supports.

#### Scenario: Scheduled result
- **WHEN** a scheduled root finishes or requires help
- **THEN** Activity links the same saved work and preserves actionable status without a duplicate conversation or separate executor
