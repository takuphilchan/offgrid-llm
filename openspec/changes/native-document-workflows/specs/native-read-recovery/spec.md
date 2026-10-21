# Spec Delta

## Purpose

Report and recover trusted native observation failures without inventing mutation
outcomes or weakening safeguards around dispatched changes.

## ADDED Requirements

### Requirement: Trusted read failures are not uncertain mutations

The durable runner SHALL distinguish reviewed native read-only operations from
mutations using trusted provider identity and operation semantics. Failed reads
SHALL retain actionable error evidence without requiring a human mutation result.
Names, model claims, low-risk labels and remote annotations alone MUST NOT confer
read-only status.

#### Scenario: Observation after unmatched launch

- **WHEN** a trusted native observation fails because no verified target was selected after launch
- **THEN** the task records a no-input observation failure with a recovery explanation
- **AND** the earlier launch remains recorded while no text-edit outcome is requested

#### Scenario: Untrusted tool copies a read name

- **WHEN** an MCP or other untrusted tool uses a native observation name or claims to be read-only
- **THEN** those claims do not bypass ordinary uncertainty handling
- **AND** a potentially mutating failure still requires reconciliation

### Requirement: Missing target is distinct from violated authority

The host SHALL distinguish a missing or unready selection from a forged,
foreign, revoked or replaced authority where it can prove that distinction.
Recoverable target failures MUST NOT broaden authority. Actual authority
violations SHALL continue to revoke or reject access.

#### Scenario: No document selected yet

- **WHEN** the task grant is valid but readiness has not produced a usable target
- **THEN** the host returns a recoverable target/readiness error without input
- **AND** supported read-only discovery can continue under the same grant

#### Scenario: Foreign session or revoked grant

- **WHEN** an operation targets another actor, session or revoked grant
- **THEN** it is denied and no target content is disclosed
- **AND** it is not relabeled as harmless missing selection to preserve access

### Requirement: Recovery is bounded and cannot repeat mutations

Recoverable reads SHALL use bounded corrective feedback in the existing task.
Repeated failures without progress SHALL pause with an actionable reason.
Recovery MUST NOT relaunch an already dispatched application, recreate a document,
repeat text input or claim an earlier mutation was rolled back.

#### Scenario: Repeated readiness failure

- **WHEN** three corrective attempts fail to establish progress
- **THEN** the task pauses with its launch and diagnostic evidence preserved
- **AND** further catalog reads do not reset the repair budget indefinitely

#### Scenario: Launch rejected before dispatch

- **WHEN** a multi-call launch proposal is rejected before any host mutation
- **THEN** recovery distinguishes it from a dispatched launch and permits the first authorized launch from observed catalog identity
- **AND** unrelated plan/catalog/history reads cannot resolve the rejected sequence or reset its repair budget

### Requirement: Sequential planning follows observed stages

The model projection SHALL offer native actions and outcome-check references only
after their prerequisite observations. Sequential HTTP inference SHALL admit no
more than one complete schema-constrained tool call per response. Projections
MUST NOT broaden authority or replace canonical dispatch validation.

#### Scenario: Future document declarations

- **WHEN** no document inventory or independent read-back has been recorded
- **THEN** model schemas do not offer checks requiring those future references
- **AND** canonical validators still reject fabricated references

#### Scenario: Truncated sequential stream

- **WHEN** a sequential inference stream ends without a clean terminal response
- **THEN** no partial envelope or proposed input is dispatched
- **AND** ordinary text-only streaming retains its compatible contract

#### Scenario: Read failure after earlier input

- **WHEN** a read fails after a recorded input action
- **THEN** the failure describes only the read's lack of effects
- **AND** it does not assert that the whole task made no changes or replay earlier input

### Requirement: Successful reads cannot hide lack of progress

New-contract tasks SHALL persist bounded semantic no-progress accounting for
trusted native reads. Volatile observation IDs/timestamps and metadata calls
SHALL NOT reset it. Meaningful changed state SHALL permit fresh observation.

#### Scenario: Repeated unchanged observation cycle
- **WHEN** successful observation/inventory/read calls repeat unchanged semantic state
- **THEN** bounded corrective feedback is issued and continued nonprogress pauses with a specific reason
- **AND** no mutation is forced, no permission is expanded and root budgets are not reset

#### Scenario: Document capability is unavailable
- **WHEN** a trusted document inventory reports the current surface unsupported
- **THEN** the task retains typed no-effect negative capability evidence without mutation reconciliation
- **AND** only previously supported structured controls become eligible after fresh authorized observation

### Requirement: Restart and legacy recovery remain conservative

Read-only intent evidence SHALL be persisted before dispatch and survive restart,
pause and cancellation. Old uncertain records without trustworthy classification
MUST NOT be silently rewritten or resumed. Viewing or upgrading SHALL remain
observation-only and preserve current history and reconciliation records.

#### Scenario: Restart during a classified read

- **WHEN** restart finds a durable intent proven to be a trusted read
- **THEN** the task is interrupted and can be explicitly resumed without inventing a mutation result
- **AND** no automatic tool dispatch occurs on restart

#### Scenario: Old uncertain task

- **WHEN** an earlier saved task has an uncertain call without trustworthy read classification
- **THEN** its record and conservative recovery boundary remain intact
- **AND** an upgrade does not automatically certify the outcome or retry it

### Requirement: Recovery presentation names the failed stage

Current clients SHALL distinguish readiness, observation and edit failures using
stable codes and accessible localized explanations. Technical details SHALL be
available without exposing private control text in general logs. Successful
specification or fixture tests MUST NOT be presented as completed native work.

#### Scenario: Observation-only failure displayed

- **WHEN** a task has a proven read-only failure
- **THEN** the current task UI presents safe recovery instead of a verified-outcome text box
- **AND** CLI and Python projections retain equivalent error and effect semantics
