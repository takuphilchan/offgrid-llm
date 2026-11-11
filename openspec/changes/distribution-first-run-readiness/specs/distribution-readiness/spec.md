## Purpose

Ensure users can identify a supported distribution, install complete runtimes and verify a first local response without confusing packaging success with workflow readiness.

## ADDED Requirements

### Requirement: Installed payloads are discoverable
Supported installations SHALL retain and resolve their packaged UI, speech, inference and execution runtimes independently of the launch directory, while respecting explicit configuration and preserving user data.

#### Scenario: Clean CLI launch
- **WHEN** a complete CLI package is installed and launched from another directory
- **THEN** packaged runtime/UI discovery succeeds without manual copying or global environment changes

### Requirement: Setup reports actual inference readiness
Setup SHALL expose inference runtime availability, CPU/GPU mode, download requirements and progress/failure before declaring readiness. A model download alone SHALL NOT complete first-answer setup.

#### Scenario: Missing runtime
- **WHEN** a user selects a model without a suitable inference runtime
- **THEN** setup offers explicit bounded runtime preparation with progress and recoverable failures

#### Scenario: First response
- **WHEN** an actual model response succeeds
- **THEN** first-answer setup can complete for that workspace

### Requirement: Artifact trust and identity are explicit
Release checks SHALL verify actual artifact signing/notarization when claiming trusted publication. Local previews SHALL have distinguishable build identity and truthful trust status.

#### Scenario: Unsigned preview
- **WHEN** no signing identity is available
- **THEN** a local build is labelled as a preview and is not reported as publisher-signed or release-qualified

### Requirement: Supported downloads have accurate requirements
Download guidance SHALL lead with desktop choices and distinguish hardware instructions, platform limitations, offline prerequisites and manual upgrades.

#### Scenario: Unsupported hardware
- **WHEN** hardware cannot run a selected archive
- **THEN** installation refuses or guides to a supported distribution rather than claiming universal CPU compatibility

### Requirement: Installed upgrade is verified
The requested Windows upgrade SHALL preserve existing workspace data, identify the installed build, and verify startup and an actual model response. Native tests SHALL use isolated data before production upgrade.

#### Scenario: Updated application
- **WHEN** the new app is installed
- **THEN** its build identity and working response are observed and existing workspace state remains available
