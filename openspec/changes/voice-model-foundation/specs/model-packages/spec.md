# Model packages

## Purpose

Let users discover, download, recover, and manage complete local speech models
alongside language and embedding models, without confusing package installation
or integrity with runtime compatibility or qualification.

## ADDED Requirements

### Requirement: Explicit package capabilities
The model service SHALL validate versioned data-only manifests declaring immutable
identity, architecture, runtime requirement, capabilities, languages, licenses,
and every required artifact's relative path, role, byte size, and SHA-256 digest.
Unsupported versions, architectures, or incomplete layouts MUST be rejected.

#### Scenario: Incomplete Piper package
- **WHEN** a Piper manifest omits its configuration artifact
- **THEN** import fails before activation and identifies the missing role

#### Scenario: Multiple speech architectures
- **WHEN** valid Whisper, Piper, streaming Zipformer, Kokoro, Qwen3-ASR, or Qwen3-TTS manifests are inspected
- **THEN** their declared speech capabilities remain distinct from chat and embeddings

### Requirement: Safe atomic local import
Import SHALL copy declared data into managed storage, verify size and digest,
reject traversal, links, special files, and undeclared artifacts, and activate only
a complete package. Failed import MUST preserve existing revisions and source
files. Import MUST NOT execute model code or download dependencies.

#### Scenario: Corrupt artifact
- **WHEN** an imported artifact does not match its declared digest
- **THEN** no installed package is created and the original input remains unchanged

#### Scenario: Unsafe path
- **WHEN** an artifact names an absolute path, parent traversal, or linked file
- **THEN** import fails without reading or writing outside its permitted root

#### Scenario: Immutable revision
- **WHEN** an existing package revision is imported with different metadata or content
- **THEN** the service rejects the conflict without replacing the working package

### Requirement: Truthful inventory and guarded removal
Inventory SHALL distinguish installation and integrity from runtime compatibility,
smoke tests, and qualification. Unavailable runtimes MUST NOT be reported ready.
Verification SHALL recheck artifacts. Removal MUST refuse an actively leased
package, remove only managed package data, and preserve recordings and documents.

#### Scenario: No installed runtime
- **WHEN** a valid speech package is imported without a matching runtime
- **THEN** inventory reports installed and integrity checked but not runtime ready or qualified

#### Scenario: Active work
- **WHEN** removal is requested while a package is leased
- **THEN** removal fails with a conflict and identifies its active usage

#### Scenario: Corrupted installed package
- **WHEN** a required artifact is modified or missing
- **THEN** verification reports failure and the package is not reported usable

### Requirement: Compatibility and authorization
Legacy model IDs and API projections SHALL remain compatible. Speech packages
MUST NOT enter legacy chat scanning, chat selectors, or language-model inference.
Package mutation SHALL require model-management permission; inventory SHALL use
the existing model-read permission. Clients MUST receive actionable errors.

#### Scenario: Speech weights resemble legacy models
- **WHEN** a managed speech package contains a BIN or GGUF artifact
- **THEN** the legacy model registry does not expose it as a chat model

#### Scenario: Unauthorized mutation
- **WHEN** a caller lacking model-management permission requests import or removal
- **THEN** access is denied without changing storage

#### Scenario: Existing language and embedding models
- **WHEN** a legacy installation is scanned
- **THEN** existing IDs remain available and embedding models are excluded from chat selection

### Requirement: Unified model discovery
Models SHALL provide Language, Embeddings, Speech recognition, and Speech generation
filters for installed models, curated choices, and explicit Hugging Face searches.
Speech acquisition SHALL use a normal Download flow; local-folder import SHALL be
an advanced/offline alternative, not a prerequisite.

#### Scenario: Find a speech model without local files
- **WHEN** a user chooses Speech recognition and searches for a supported model
- **THEN** matching repositories and compatible variants are presented without requesting a local folder or user-authored manifest

#### Scenario: Existing model discovery
- **WHEN** a user selects Language or Embeddings
- **THEN** existing catalog choices, model IDs, downloads, and selections remain available with their appropriate capabilities

#### Scenario: Advanced offline import
- **WHEN** a user intentionally opens advanced import and supplies a valid local package
- **THEN** the existing validated copy/import workflow remains available without network access

### Requirement: Complete supported package resolution
The service SHALL resolve supported Whisper, Piper, streaming Zipformer, Kokoro,
and reviewed Qwen3-ASR/Qwen3-TTS
variants into all required data artifacts and runtime requirements. Users SHALL NOT
assemble dependency lists. Missing, ambiguous, or unsupported layouts MUST return
a specific explanation instead of an installable partial model.

#### Scenario: Multi-file voice model
- **WHEN** a user selects a supported Piper voice or Kokoro variant
- **THEN** resolution includes its weights and all required configuration, voice, token, and model data before offering Download

#### Scenario: Streaming recognition model
- **WHEN** a user selects a supported streaming Zipformer variant
- **THEN** its encoder, decoder, joiner, tokens, and additional required data are one package rather than independent installable models

#### Scenario: Unsupported repository
- **WHEN** search finds speech tags but no supported complete layout
- **THEN** the result explains why it cannot be installed and does not execute repository code or guess compatibility from a file extension

### Requirement: Immutable source and provenance
Before transfer, installation SHALL bind the complete package to immutable source
revisions, declared paths, sizes, verifiable artifact identities, and licenses.
Curated and discovered provenance SHALL remain distinct from locally supplied
hashes. Unknown identity or incomplete metadata MUST NOT be treated as verification.

#### Scenario: Upstream branch changes
- **WHEN** the repository's default branch changes after an installation is accepted
- **THEN** download and resume continue using the accepted immutable revision, or fail explicitly if that revision is unavailable

#### Scenario: Missing verification metadata
- **WHEN** a required artifact cannot be bound to verifiable source identity
- **THEN** normal download is unavailable with an explanation rather than claiming a locally calculated digest proves trusted provenance

#### Scenario: Incomplete repository listing
- **WHEN** repository metadata exceeds supported limits or cannot be fully resolved
- **THEN** resolution reports the limitation without silently omitting dependencies

### Requirement: Safe acquisition and preflight
Before download, users SHALL see source, variant, license, total transfer size,
storage requirements, and missing runtime components. Transfers SHALL fetch only
declared data from permitted HTTPS sources, with bounded resources and existing
permissions. Model acquisition MUST NOT execute repository scripts or change
network/security settings.

#### Scenario: Insufficient storage
- **WHEN** storage cannot accommodate staging, activation, and retained recovery data
- **THEN** installation fails before transferring weights and reports required and available space without altering the existing model

#### Scenario: Unsafe source or redirect
- **WHEN** a source requests an arbitrary host, insecure redirect, or out-of-scope artifact
- **THEN** acquisition is rejected without sending credentials to that destination or publishing a package

#### Scenario: Unavailable public access
- **WHEN** a selected repository requires unsupported gated/private authentication
- **THEN** the UI explains the access limitation and never requests tokens in task text or URL parameters

### Requirement: Durable idempotent package operations
Installation SHALL be a persisted operation with stable identity, actor ownership,
package/source binding, artifact progress, and aggregate state. Acceptance and
completion SHALL be durable before acknowledgment. Identical submission retries
MUST NOT create duplicate work; conflicting request-ID reuse MUST fail.

#### Scenario: Lost acceptance acknowledgment
- **WHEN** a client repeats an identical request after losing the response
- **THEN** it receives the original operation and no second package writer starts

#### Scenario: Changed retry payload
- **WHEN** the same actor reuses a request ID for a different revision or variant
- **THEN** the service returns a conflict without changing the original operation

#### Scenario: Navigation during download
- **WHEN** a user changes pages, refreshes, or reconnects during an accepted download
- **THEN** the operation continues and its durable progress is restored without resubmission

#### Scenario: Transfer complete but verification pending
- **WHEN** all bytes have arrived but validation or activation is unfinished
- **THEN** progress shows verification or preparation, not an installed or voice-ready model

### Requirement: Cancellation and explicit recovery
Cancellation SHALL settle owned writes before resume or discard. Interrupted work
SHALL retain identity-bound partial data and provide explicit recovery after
restart. Resume MUST recheck source identity and artifacts; it MUST NOT silently
switch revisions, append mismatched bytes, or restart work automatically.

#### Scenario: Cancel and resume
- **WHEN** a package download is cancelled and later resumed
- **THEN** retained valid bytes are reused only for the same source and cancellation completes before another writer begins

#### Scenario: Restart midway through a package
- **WHEN** the service restarts after some artifacts are complete
- **THEN** the operation is shown as interrupted with Resume, complete artifacts are revalidated, and partial installation is not reported as installed

#### Scenario: Server ignores a range request
- **WHEN** a resumed artifact receives a full response instead of the requested range
- **THEN** the transfer safely replaces the partial artifact rather than appending duplicate bytes

#### Scenario: Discard retained staging
- **WHEN** a user confirms discarding an inactive operation's partial download
- **THEN** only its staged bytes are removed; installed revisions, source imports, recordings, and other downloads remain intact

### Requirement: Verified activation and recoverable repair
Network installations SHALL activate only complete verified packages. Repair SHALL
stage and verify the exact revision before replacement, preserve the prior data
until publication is committed, and obey leases. Failure or restart MUST leave a
recoverable state, never a partially published or silently overwritten model.

#### Scenario: One bad artifact
- **WHEN** an artifact fails its size or identity check after the others download
- **THEN** installation fails with recoverable state and no complete installed package is published

#### Scenario: Repair a missing configuration
- **WHEN** Repair is requested for a network-installed package with a missing configuration
- **THEN** the original pinned source is used and the complete replacement is verified before publication

#### Scenario: Failed repair or active lease
- **WHEN** repair fails or the target is leased by active work
- **THEN** no in-use package is replaced and its original data remains recoverable

#### Scenario: Lost completion acknowledgment
- **WHEN** the process stops after package activation but before the client receives completion
- **THEN** recovery reconciles the immutable installation and operation record without installing a second copy or falsely reporting failure as success

#### Scenario: Local package without a repair source
- **WHEN** a locally imported package has no independently verifiable download source
- **THEN** recovery offers re-import and does not invent a network Repair action

#### Scenario: Removal races an installation
- **WHEN** deletion targets a package with an active installation or repair
- **THEN** deletion waits for explicit cancellation/settlement or refuses with an actionable conflict rather than allowing the package to reappear

### Requirement: Readiness remains explicit
Acquisition SHALL distinguish supported package layout, installed bytes, checked
integrity, runtime compatibility, smoke testing, and qualification. Missing runtime
support SHALL be disclosed before download and after installation. Setup guidance
MUST reflect real available actions; downloaded weights MUST NOT enable speech UI
or be labelled ready for voice without runtime evidence.

#### Scenario: Download weights for later use
- **WHEN** a user explicitly downloads a supported package despite its missing runtime
- **THEN** installation can complete but the entry retains an actionable unavailable-runtime explanation and is not offered as a working Talk model

#### Scenario: No runtime installer exists yet
- **WHEN** the build cannot install a required speech runtime
- **THEN** it states that limitation instead of showing a nonfunctional setup button or requiring terminal configuration as normal first use

### Requirement: Compatible operation migration and clients
Existing model IDs, files, downloads, and partial bytes SHALL survive migration to
typed operation records. Corrupt or incompatible records MUST block their migration
without disappearing. Web, matching desktop, CLI, and Python SHALL use the same
authorized operation lifecycle while valid legacy clients remain functional.

#### Scenario: Legacy interrupted download
- **WHEN** the service upgrades with an interrupted single-file model download
- **THEN** its original recovery data is preserved and its existing client can resume it without treating it as a speech package

#### Scenario: Failed migration
- **WHEN** download records are corrupt or migration persistence fails
- **THEN** activation is rejected with recovery guidance, originals are retained, and no empty replacement history is published

#### Scenario: Unauthorized operation access
- **WHEN** a caller lacks the required model permission or requests another actor's operation without administrator authority
- **THEN** inspection or mutation is denied without leaking private operation details or changing data

#### Scenario: Non-browser clients
- **WHEN** CLI or Python clients install, inspect, cancel, resume, repair, or remove a speech package
- **THEN** they use the same service-managed state and policy rather than a separate downloader or direct filesystem mutation

### Requirement: Accessible shared model interface
The web and matching desktop Models interfaces SHALL preserve monochrome styling,
nine interface locales, keyboard navigation, focus restoration, screen-reader
status, RTL, and responsive layout. Speech progress and recovery SHALL use the
same interaction conventions as other model types.

#### Scenario: Keyboard-only recovery
- **WHEN** a keyboard or screen-reader user cancels, resumes, repairs, or deletes a package
- **THEN** controls remain operable, focus returns predictably, and meaningful status changes are announced without per-byte announcement noise

#### Scenario: Narrow or RTL layout
- **WHEN** Models is displayed on a narrow screen or with an RTL locale
- **THEN** category selection, progress, model metadata, and actions remain readable and reachable without page-level horizontal overflow

### Requirement: Explicit bounded response speech
When response speech is opted into, new Chat turns SHALL synthesize bounded prose
chunks as text arrives without blocking text generation. Restored turns MUST NOT
replay automatically. The UI SHALL inspect speech readiness before synthesis and
explain incompatible installed models. Provisional speech SHALL be labelled.

#### Scenario: Text is still arriving
- **WHEN** Speak responses is enabled and a complete prose sentence arrives before the turn commits
- **THEN** speech can start with at most one look-ahead synthesis request, without waiting for the entire response

#### Scenario: Turn fails or is interrupted
- **WHEN** generation fails, the user stops it, or the user leaves Chat
- **THEN** pending speech and playback stop without speaking the unfinished final fragment or replaying audio on return

#### Scenario: Speech cannot keep up
- **WHEN** unsynthesized prose exceeds the bounded backlog
- **THEN** speech stops with a visible explanation while text generation continues

### Requirement: Independent speech selection and safe dictation
Chat and Agents SHALL expose shared, account/workspace-scoped browser preferences
for available recognition and synthesis profiles. Explicit unavailable selections
MUST NOT silently fall back. A recording or answer SHALL pin its model revision
and voice until completion or cancellation. Preferences MUST NOT grant microphone
permission or opt into playback.

#### Scenario: Preferences change during an answer
- **WHEN** a user selects another speech model while chunks are playing
- **THEN** all remaining chunks use the original selection and the next playback uses the new selection

#### Scenario: Delayed microphone permission
- **WHEN** device permission resolves after the composer is unmounted or its context changes
- **THEN** returned tracks are released and no recorder starts

#### Scenario: Concurrent typing and generation
- **WHEN** a user edits the draft while transcription runs or sends a typed request while recording
- **THEN** dictation preserves current draft edits and recording can still be stopped

### Requirement: Inline acquisition without arbitrary variant selection
Speech repository review SHALL show download conditions in the selected result
card. Single supported variants SHALL resolve without an intermediate selection
screen; multiple supported variants SHALL require a choice before download.
Keyword search MUST NOT replace upstream results with a publisher-specific
shortlist, and parameter counts MUST NOT be presented as transfer bytes.

#### Scenario: Multiple supported profiles
- **WHEN** discovery returns more than one supported variant
- **THEN** no variant is resolved or installed until the user selects it and installation requires the inline Download action
