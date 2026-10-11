# Spec Delta

## Purpose

Let users distinguish executed actions, independently verified results and
unfinished outcomes in the same saved task without repeating external work.

## ADDED Requirements

### Requirement: Outcome expectations are durable and versioned

New task-first native workflows SHALL persist a bounded versioned expectation
record, tied to task identity and the originating user instruction, before their
first mutation. Expectations SHALL retain their caller-authored or model-proposed
provenance and SHALL NOT grant tool, host or connection permissions.

#### Scenario: Compound request

- **WHEN** a task declares opening a browser and performing research as separate outcomes
- **THEN** both expectations survive checkpoint recovery and context compaction
- **AND** the original user instruction remains available without an English keyword classifier replacing it

#### Scenario: Missing expectations

- **WHEN** a new evidence-versioned workflow proposes a mutation before recording its expectations
- **THEN** the mutation is rejected before dispatch with bounded corrective feedback
- **AND** read-only discovery remains available to prepare the expectations

#### Scenario: Model attempts to remove unfinished work

- **WHEN** the model replaces its progress plan or omits an unfinished expectation
- **THEN** the expectation remains unresolved unless an explicit user correction supersedes it
- **AND** the correction and prior evidence remain recorded with their revisions

### Requirement: Verification is limited to the proven result

Verification SHALL evaluate supported typed predicates against canonical
successful results bound to the correct task, target and action. Application
readiness, control text and checkbox state SHALL remain distinct check kinds.
Unsupported checks, model assertions and arbitrary tool-step references MUST NOT
be promoted to independently verified outcomes.

#### Scenario: Application opened but research not performed

- **WHEN** only application readiness is verified for an open-and-research task
- **THEN** the research outcome remains unresolved or explicitly unverified
- **AND** the result cannot claim that all declared checks passed

#### Scenario: Mismatched or invalidated evidence

- **WHEN** evidence refers to another target, task, unsuccessful dispatch, unrelated check or a later-invalidated postcondition
- **THEN** it does not satisfy the expectation
- **AND** reading or searching the evidence does not make it fresh

#### Scenario: Verified text is not a saved document

- **WHEN** text read-back succeeds but no supported saved-file verifier exists
- **THEN** the text-state check can pass while the saved-document outcome remains unverified
- **AND** a save shortcut or model statement cannot substitute for file evidence

#### Scenario: A fresh observation replaces input references

- **WHEN** the host observes the same OS control after an edit with a new input element ID
- **THEN** only its host-issued evidence identity and exact target can connect the new read-back to the immutable expectation
- **AND** a missing, ambiguous, replaced or different control cannot match by label, text or row
- **AND** the old observation still cannot authorize input

#### Scenario: Older host has no evidence identity

- **WHEN** a control expectation requires cross-observation evidence from a host without that metadata
- **THEN** its verification is explicitly unavailable, not guessed or certified
- **AND** its existing selected-window permissions remain unchanged

#### Scenario: Interpretation is not independent proof

- **WHEN** every model-proposed expectation has a passing check
- **THEN** the UI reports those checks and their scope without asserting that the model covered every aspect of the user's intent
- **AND** provenance remains inspectable

### Requirement: Incomplete completion proposals recover without replay

A completion proposal with unresolved declared work SHALL receive at most three
consecutive corrective opportunities through the same durable task. Exhaustion
SHALL interrupt the task with actionable remaining-work information. Unsupported
verification SHALL permit an explicitly unverified result. Recovery MUST NOT
resubmit the task, replay a mutation or bypass uncertain-outcome reconciliation.

#### Scenario: Model stops after opening the browser

- **WHEN** the model proposes completion while a declared research outcome is still pending
- **THEN** it receives the remaining expectation and existing launch evidence
- **AND** another browser launch is not dispatched merely to repair completion

#### Scenario: Repair budget exhausted

- **WHEN** three consecutive completion repairs fail without new valid progress
- **THEN** the task is interrupted with its results, evidence and remaining work preserved
- **AND** reconnecting or viewing the task does not resume it

#### Scenario: External action outcome is uncertain

- **WHEN** a previously dispatched submission has no trustworthy acknowledgment
- **THEN** completion repair cannot repeat the submission
- **AND** the existing reconciliation boundary takes precedence

### Requirement: Clients distinguish execution from verification

Existing job snapshots, events and exports SHALL expose additive verification
information without replacing task identity or legacy execution status. Current
clients SHALL display partial, unavailable and verified-check states accurately.
Absent or unsupported verification versions SHALL not imply successful checking.

#### Scenario: Legacy task

- **WHEN** an older task without the evidence contract is read or exported
- **THEN** its saved status and result are preserved without retrospective certification or new dispatch

#### Scenario: Persistence failure

- **WHEN** evidence or completion bookkeeping cannot be durably saved
- **THEN** no verified-completion event is acknowledged
- **AND** previously dispatched actions remain recorded for recovery instead of being rerun

#### Scenario: Current interface and privacy

- **WHEN** the task is inspected through the current Agents UI or authenticated job clients
- **THEN** concise check results and remaining work use existing task components and owner-scoped APIs
- **AND** no legacy page, private-text logging, new consent prompt or duplicate task history is introduced
