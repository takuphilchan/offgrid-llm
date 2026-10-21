# Model and runtime metadata

## ADDED Requirements

### Requirement: Engine metadata is additive and explicit
Installed model records SHALL expose engine, format, backend, runner revision,
adapter revision, modalities, context/template identity, and readiness states.
Existing GGUF records without the new fields SHALL resolve to llama.cpp and
GGUF. Other legacy formats SHALL retain their actual format rather than be
misidentified as GGUF.

#### Scenario: Existing model
- **WHEN** an existing GGUF model is loaded after migration
- **THEN** it remains selectable through llama.cpp with its prior behavior and
  no new download or permission

#### Scenario: Unavailable provider
- **WHEN** a model's declared provider or adapter is unavailable
- **THEN** the model is shown as installed but not runtime-ready and cannot be
  activated

### Requirement: Qualification is separate from installation
The service SHALL distinguish installed, integrity-checked, runtime-ready,
smoke-tested, and workflow-qualified states.

#### Scenario: Metadata-only family
- **WHEN** no pinned adapter exists for an imported model family
- **THEN** metadata remains inspectable but the model cannot be used for agent
  inference or tool calls

### Requirement: Provider changes preserve identity
Model/provider activation SHALL be bound to immutable model and adapter digests;
changing runtime identity invalidates prior qualification without deleting model
files or changing saved tasks.

#### Scenario: Runtime revision invalidates evidence
- **WHEN** an installed model is activated with a different runner or adapter revision
- **THEN** the model remains installed, but prior qualification becomes stale and execution is blocked until requalified
