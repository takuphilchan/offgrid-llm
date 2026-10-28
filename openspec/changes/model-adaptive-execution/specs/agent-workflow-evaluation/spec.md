## Purpose

Measure actual useful outcomes on fixed workflows without confusing synthetic
contract tests, capability declarations or package compilation with qualification.

## ADDED Requirements

### Requirement: Qualification uses a frozen representative suite
The evaluation manifest SHALL contain at least thirty uniquely identified cases
across conversation, browser, document, spreadsheet, file and collaboration work.
Cases SHALL specify prompt, setup, checks and prohibited effects, with explicit
native-isolation requirements and no private user fixtures.

#### Scenario: Manifest changes
- **WHEN** a workflow definition changes
- **THEN** its suite digest changes and earlier result records cannot qualify the new suite

### Requirement: Evidence records actual execution environments
Qualification SHALL distinguish implemented, fixture-tested and real-platform
verified work; run each case three times per advertised model/platform profile;
require at least ninety percent verified completion and no unauthorized actions,
repeated uncertain mutations or false completion in the adversarial suite.

#### Scenario: Missing native machine
- **WHEN** native platform testing has not run on an isolated supported desktop
- **THEN** that platform remains unverified regardless of compile/fixture success

#### Scenario: Collaboration comparison
- **WHEN** collaboration is claimed to improve a workflow
- **THEN** comparable single-agent and multi-agent runs record completion, wrong actions,
  tokens, queue/load/model/tool timings and memory on the same hardware

### Requirement: Scoped qualification cannot imply general readiness
A separate versioned frozen conversation/skills/workspace suite MAY qualify only
its reviewed capability subset. Every case SHALL run three times per measured
model/runtime; relevant known failures SHALL pass and unauthorized actions,
repeated uncertain mutations and false completion SHALL be zero. Evidence SHALL
retain per-case results and failures, scope and suite identities. Desktop, visual
and broader release claims SHALL retain the full representative suite requirement.

#### Scenario: Skills pass without desktop tests
- **WHEN** the scoped suite passes but isolated desktop workflows have not passed
- **THEN** the scoped record may select only its enforced tool subset
- **AND** desktop and general workflow qualification remain unverified

#### Scenario: Scope or suite changed
- **WHEN** a tool inventory, scoped workflow or runtime identity changes
- **THEN** earlier results cannot qualify the changed scope without fresh matching evidence
