# Agent proposal recovery

## Purpose

Let a saved agent task correct invalid pre-dispatch proposals without inventing consent, repeating uncertain actions, or requiring a replacement task.

## ADDED Requirements

### Requirement: Access validation is actionable and does not grant authority
The service SHALL distinguish an invalid access proposal from denied authority, return bounded service-authored corrective guidance, and create no consent or execution from invalid input.

#### Scenario: Open Chrome without a browser URL
- **WHEN** a model requests browser mode for Chrome with an empty URL
- **THEN** the service returns an invalid-access error explaining app mode for installed applications and HTTPS URLs for managed browsing
- **AND** a corrected app request pauses the same task for local consent without dispatching input

#### Scenario: Malformed and unauthorized additions
- **WHEN** an access proposal contains unsupported modes, extra fields, trailing JSON, or a non-HTTPS managed-browser URL
- **THEN** the service rejects it without inferring permission or starting an application

### Requirement: Pre-dispatch repair is bounded and durable
The runner SHALL permit at most three consecutive automatic repairs of eligible authorization-stage validation failures, persist the repair budget, and pause with an actionable error on exhaustion. Rejected calls SHALL be recorded as unexecuted and removed from the dispatch queue.

#### Scenario: Correction before a computer session exists
- **WHEN** an eligible invalid-access proposal is returned before local access
- **THEN** the model receives corrective tool feedback and can issue a valid proposal in the same run

#### Scenario: Repair limit and restart
- **WHEN** the model exceeds three consecutive repair attempts, including after restart and explicit continuation of an interrupted attempt
- **THEN** the task pauses without dispatching rejected proposals or leaving pending approvals
- **AND** explicit correction or resumption of the repair-limit pause permits a fresh repair budget

#### Scenario: Repair does not hide policy denials or uncertainty
- **WHEN** authorization denies scope, ownership, a prohibited action, or reports an uncertain effect
- **THEN** that error does not enter automatic proposal repair
- **AND** an execution failure with uncertain effects is never automatically replayed

### Requirement: Existing durable history stays conservative
The service SHALL preserve legacy snapshots and SHALL NOT reinterpret an old failed or uncertain task as repairable based only on display text.

#### Scenario: Historical permission failure
- **WHEN** an old task is marked failed without explicit new recovery evidence
- **THEN** ordinary resume remains unavailable and its history is preserved
