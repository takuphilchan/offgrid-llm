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

### Requirement: No parallel acquisition authority
This presentation SHALL consume existing legacy and package APIs without adding a downloader, bypassing provenance/permission checks or treating UI completion as speech qualification. Missing durable acquisition behavior SHALL remain an explicit dependency of the existing voice-model change.

#### Scenario: Pending package capability
- **WHEN** the running service lacks a required package operation
- **THEN** the UI states the actual capability limitation and does not simulate success
- **AND** typed Chat and unrelated installed models remain usable

### Requirement: Usable inventory before inactive transfers
The library SHALL label the selected model and collapse cancelled transfer
history by default while retaining its explicit recovery controls.

#### Scenario: Returning after a cancelled download
- **WHEN** installed models coexist with a cancelled transfer
- **THEN** the installed inventory and selected-model state remain visible
- **AND** expanding Downloads reveals the retained transfer without restarting it
