## ADDED Requirements

### Requirement: Complete source-bound downloads
An explicit vision-model download SHALL resolve and verify the selected weights and required projector at immutable revisions before reporting complete vision installation. Alternative quantizations, draft models and importance matrices SHALL NOT all be downloaded as dependencies.

#### Scenario: Generic projector filename
- **WHEN** the chosen repository supplies one compatible projector family with generic filenames
- **THEN** OffGrid selects a supported alternative and binds it to the exact model without a model-name-specific runtime rule
- **AND** another model's generic projector cannot overwrite or satisfy that binding

#### Scenario: Missing or ambiguous companion
- **WHEN** the required companion is missing or cannot be uniquely matched
- **THEN** download completion does not claim vision readiness and the user receives an actionable explanation
- **AND** OffGrid does not substitute a foreign projector using a name heuristic

### Requirement: Recoverable acquisition and repair
The service SHALL persist artifact identity before transfer and verify all required bytes before publishing a companion receipt. Resume and repair SHALL reuse that identity, preserve existing model bytes, and obey cancellation and authorization.

#### Scenario: Interrupted companion download
- **WHEN** transfer is cancelled or fails after the model weights arrive
- **THEN** the operation stays incomplete, retained bytes are inspectable, and explicit retry fetches the missing artifact without silently changing revision

#### Scenario: Image submission while unavailable
- **WHEN** installed companion data or runtime support is unavailable
- **THEN** images remain attached and the interface offers model setup or repair without downloading, switching models or claiming image interpretation succeeded
