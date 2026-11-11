## ADDED Requirements

### Requirement: All maintained installer entry points preserve running work
Root, compatibility, desktop and archive installers SHALL refuse unsafe replacement while an existing OffGrid service or its inference worker is active rather than terminate processes by name. They SHALL report copy and preparation failures without claiming installation success.

#### Scenario: Existing work
- **WHEN** installation detects a running OffGrid or llama-server process or cannot inspect processes
- **THEN** replacement stops with actionable guidance and no process termination

### Requirement: Release payload verification precedes use
All network installers SHALL require exactly one matching digest from the same release manifest before extracting or executing a payload. Missing manifests and invalid digests SHALL stop installation.

#### Scenario: Invalid payload
- **WHEN** a payload or manifest is absent, corrupted or ambiguous
- **THEN** it is not extracted, installed or executed

#### Scenario: Supported fallback
- **WHEN** an optional hardware variant is absent and a supported CPU variant is selected
- **THEN** the final selected archive is verified before use and the selection is reported
