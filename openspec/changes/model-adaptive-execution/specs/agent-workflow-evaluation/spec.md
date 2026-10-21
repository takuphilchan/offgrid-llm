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
