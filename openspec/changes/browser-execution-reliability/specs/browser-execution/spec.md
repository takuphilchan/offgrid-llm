# Spec Delta

## Purpose

Make managed browser work recoverable and inspectable while preserving explicit authority and preventing duplicate uncertain actions.

## ADDED Requirements

### Requirement: Page content cannot grant authority
The companion SHALL NOT classify a click as reversible merely because website-authored text suggests a safe action. Existing task grants and prohibited-action restrictions SHALL continue to apply.

#### Scenario: Misleading save label
- **WHEN** a website labels an arbitrary click action Save or Apply
- **THEN** scoped-changes mode requires exact approval for that click
- **AND** full-task mode retains its existing authorized scope

### Requirement: Effects and observations have separate outcomes
The browser SHALL preserve a successful action receipt when its subsequent observation fails. A receipt SHALL distinguish dispatched input from verified outcome and SHALL NOT invite automatic repetition of a dispatched mutation.

#### Scenario: Click succeeds and observation fails
- **WHEN** a click returns successfully but the follow-up page read fails
- **THEN** the result records dispatched input and unavailable observation
- **AND** verification remains false and further inspection is required

#### Scenario: Mutation call fails after dispatch begins
- **WHEN** an input call fails without proof of its outcome
- **THEN** its outcome remains uncertain and it is not automatically retried

#### Scenario: Acknowledged mutation followed by service restart
- **WHEN** an authenticated staged mutation receipt was persisted before acknowledgement and the service restarts
- **THEN** the original result can be recovered without dispatching input or declaring the whole task complete
- **AND** the expired browser session does not regain authority

### Requirement: Safe failures are durable and recoverable
Trusted browser read failures and stale pre-dispatch observations SHALL produce durable bounded error receipts without terminating the browser session. Agent repair SHALL be bounded to three consecutive attempts and SHALL NOT expand authority.

#### Scenario: Stale control before input
- **WHEN** freshness validation rejects an action before input is invoked
- **THEN** no input is dispatched and the task can inspect again within its repair budget

#### Scenario: Redelivered failed read
- **WHEN** a completed failed-read action is redelivered
- **THEN** the companion returns its recorded receipt without executing it again

#### Scenario: Interrupted read and legacy intent
- **WHEN** the service restarts with a pinned reviewed browser read intent
- **THEN** the task is interrupted without requiring a user to reconcile mutation effects
- **AND** legacy intents without that evidence retain conservative handling

### Requirement: Discovery exposes bounded continuations
The browser SHALL expose explicit continuation for page text and controls beyond one response, bind continuation to its observation, and reject stale or malformed continuation without input effects. Services SHALL advertise extended discovery only for companions that negotiate it.

#### Scenario: Large page
- **WHEN** a page has more controls or text than one bounded response
- **THEN** the task can inspect later controls and text without guessing identifiers
- **AND** response size remains bounded

#### Scenario: Dropdown options beyond the first response
- **WHEN** the desired option occurs beyond the first bounded options page
- **THEN** the task can inspect an issued continuation and select the observed option ID
- **AND** guessed unobserved option IDs are rejected before input

### Requirement: Unrelated changes do not invalidate targets
Browser input SHALL validate the observed document, exact target identity, target semantics and form state. Unrelated decorative DOM changes SHALL NOT alone invalidate a target. Changed target semantics, replacement nodes, navigation and changed form input SHALL invalidate it.

#### Scenario: Updating clock
- **WHEN** a clock changes outside the target and its form
- **THEN** an otherwise unchanged observed target remains actionable

#### Scenario: Silent form edit
- **WHEN** a form property changes without an input event
- **THEN** input based on the earlier observation is rejected before dispatch

#### Scenario: Same-address document replacement
- **WHEN** a page reloads at the same address with identical form state
- **THEN** old capture and continuation references are rejected because the observed document was replaced
