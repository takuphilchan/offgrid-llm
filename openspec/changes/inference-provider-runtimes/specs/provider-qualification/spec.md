# Provider qualification

## ADDED Requirements

### Requirement: Capability evidence binds runtime identity
Provider evidence SHALL include model digest, format, runner, adapter, template,
context, backend, and relevant perception identities. Changed identity makes
prior evidence stale.

#### Scenario: Runtime update
- **WHEN** a provider binary or adapter changes
- **THEN** existing qualification is stale and the service requires a fresh
  smoke check before activation

### Requirement: Modalities qualify independently
Text qualification SHALL not imply vision or audio qualification. Vision actions
require qualified preprocessing, observation, projector/encoder, and tool-call
behavior for that model/provider pair.

#### Scenario: Text-only provider
- **WHEN** a provider passes text checks but has no qualified vision path
- **THEN** text chat remains available while image actions are unavailable

#### Scenario: Text-only agent profile
- **WHEN** a locally qualified MLX or Transformers text model is selected by a
  profile with no tools, collaborators, or computer access
- **THEN** its canonical job uses the shared provider scheduler and may return
  a verified terminal text answer, while stale qualification or any proposed
  tool/computer/delegation authority is rejected before dispatch

### Requirement: Qualification is honest
Compilation, metadata, fake workers, and mock hardware SHALL not count as real
platform qualification. Results SHALL record completion, unsafe actions, false
claims, latency, queue/load time, and memory for each advertised profile.

#### Scenario: Mock evidence is not qualified
- **WHEN** a provider has only passed metadata validation and fake-worker tests
- **THEN** it is shown as implemented or smoke-tested as applicable, but not as platform-qualified
