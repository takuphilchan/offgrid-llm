## Purpose

Give all model categories a coherent library experience while preserving the existing installation, provenance and runtime-readiness boundaries.

## ADDED Requirements

### Requirement: Searchable model choice
Every ordinary model choice SHALL support readable identity and search including
exact identifiers. Actual capability and readiness SHALL be available at the
decision point; unknown runtime fitness SHALL remain unknown. Selection SHALL
preserve the exact model ID and SHALL NOT download, load or substitute models.

#### Scenario: Find a long model identifier
- **WHEN** a user searches by a readable name or exact identifier
- **THEN** matching eligible models remain identifiable without relying on a clipped filename
- **AND** unavailable selection and readiness details do not falsely claim runtime qualification

#### Scenario: Compare catalog choices without downloading
- **WHEN** a user filters curated language or embedding choices and compares up to three models
- **THEN** comparison shows supplied download size, parameter count, quantization, license and labelled catalog RAM estimates, with unknown values explicit
- **AND** comparison and filter state remain category/workspace scoped without changing the active model or starting transfers
- **AND** changing between curated browsing and online search retains their independent filters and leaves active downloads inspectable

### Requirement: Distinct library intents
Models SHALL distinguish Installed, Discover and ongoing operations without simultaneous competing search boxes. Language, embedding, recognition and synthesis categories SHALL retain their capability boundaries and meaningful filter state across navigation.

#### Scenario: Browse installed and discoverable models
- **WHEN** a user changes between installed inventory and discovery
- **THEN** each view names its scope and retains its own filters
- **AND** empty discovery results cannot claim that installed models are absent

### Requirement: Decision-ready model identity
Model rows SHALL present a readable name, publisher/source where known, relevant capability, download or installed size with its meaning, and readiness. Exact IDs, quantization, revisions, licenses and runtime requirements SHALL remain inspectable without making filenames the only useful identity.

#### Scenario: Unknown size or runtime
- **WHEN** reliable package size or runtime readiness is unavailable
- **THEN** the interface labels it unknown or unchecked rather than zero or ready
- **AND** installed files are not equated with usable speech or chat capability

### Requirement: One acquisition decision surface
Discovery SHALL expand model decisions at the selected result or an anchored accessible panel. A single compatible variant SHALL NOT require an intermediate selection screen; multiple meaningful variants SHALL be chosen in the same decision surface. Source, license, size, free-space and runtime limitations SHALL remain available before explicit download.

#### Scenario: Review a single variant
- **WHEN** a user selects Review download on a single-variant result
- **THEN** its conditions and Download action appear in context without a second Review or Select package step or a required page-bottom search
- **AND** navigation, review and variant selection do not start a download

#### Scenario: Multiple variants
- **WHEN** a repository exposes different compatible model variants
- **THEN** selection updates size and requirements in the same surface before one explicit Download action

### Requirement: Operations remain attached to their models
Download, verification, preparation, cancellation, resume and repair presentation SHALL use existing operation state, with aggregate progress and inspectable artifact detail when available. Operations SHALL remain findable across view changes; acknowledged failure or uncertainty SHALL show a specific cause and safe next action.

#### Scenario: Return after interrupted transfer
- **WHEN** a user returns to a model with a failed or interrupted operation
- **THEN** retained progress and supported Resume, Check status or Discard controls are shown beside that model
- **AND** the client neither invents durability nor automatically restarts the transfer

#### Scenario: Transfer continuity across model categories
- **WHEN** an administrator changes between Language, Embeddings, Speech recognition and Speech generation, or returns to Models from another route
- **THEN** ongoing and recoverable GGUF and package transfers remain inspectable independently of the selected category and discovery filter
- **AND** package operations use one library-wide read subscription and one set of controls for each operation identity
- **AND** Cancel, Resume and confirmed Discard preserve the original source/operation identity; navigation and a status refresh never replay a transfer
- **AND** a failed status read retains the last observed transfer with an error and a read-only Check status action
- **AND** ordinary members do not gain access to administrative transfer history

### Requirement: No parallel acquisition authority
This presentation SHALL consume existing legacy and package APIs without adding a downloader, bypassing provenance/permission checks or treating UI completion as speech qualification. Missing durable acquisition behavior SHALL remain an explicit dependency of the existing voice-model change.

#### Scenario: Pending package capability
- **WHEN** the running service lacks a required package operation
- **THEN** the UI states the actual capability limitation and does not simulate success
- **AND** typed Chat and unrelated installed models remain usable

#### Scenario: Older service without endpoint management
- **WHEN** the endpoint-management collection returns HTTP 404
- **THEN** the library explains that this service does not support that management feature without claiming an empty provisioned inventory or offering ineffective review/probe controls
- **AND** local model selection and any endpoint models actually advertised by the service remain available under their existing readiness rules
- **AND** other failures such as HTTP 503 remain visible with read-only refresh, rather than being classified as unsupported

### Requirement: Usable inventory before inactive transfers
The library SHALL label the selected model and collapse cancelled transfer
history by default while retaining its explicit recovery controls.

#### Scenario: Returning after a cancelled download
- **WHEN** installed models coexist with a cancelled transfer
- **THEN** the installed inventory and selected-model state remain visible
- **AND** expanding Downloads reveals the retained transfer without restarting it

### Requirement: Explicit stopped-transfer discard
Model managers SHALL be able to confirm discard of stopped GGUF download partials without deleting installed model data or another transfer.

#### Scenario: Active or incomplete removal
- **WHEN** a worker remains active or storage removal cannot complete
- **THEN** the service rejects the active removal or reports the storage failure and retains a retryable progress record
- **AND** the UI does not claim the model was deleted or restart acquisition

#### Scenario: Successful discard
- **WHEN** a stopped transfer is explicitly discarded
- **THEN** only its exact temporary artifacts and saved transfer record are removed
- **AND** installed weights, projectors and unrelated transfers remain unchanged after reload
