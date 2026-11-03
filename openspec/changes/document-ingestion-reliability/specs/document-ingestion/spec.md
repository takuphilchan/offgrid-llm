## Purpose

Let people reliably import local documents and obtain attributable answers while
preserving source fidelity, privacy, bounded execution and recovery.

## ADDED Requirements

### Requirement: Context-bounded conversation source evidence
Conversation attachments SHALL preserve complete original files and history while
selecting bounded exact source excerpts when full extraction exceeds the current
model allocation. Each answer SHALL retain source revisions and selected ranges,
and clients SHALL disclose incomplete document coverage independently of model output.

#### Scenario: Large readable PDF in an ordinary context
- **WHEN** a prepared PDF exceeds the current context and its question can admit source excerpts
- **THEN** the service selects exact bounded passages, deduplicates repeated references and reserves answer space
- **AND** the model and clients disclose excerpt and visual limitations without changing models or context settings

#### Scenario: Repeated document questions fit the measured context
- **WHEN** completed history exceeds a byte-based estimate but the loaded model's complete prompt and explicit response limit fit its context
- **THEN** follow-up questions reuse the saved source without another upload and use tokenizer-measured excerpt admission under the generation lease
- **AND** all history, source revisions and response settings remain unchanged
- **AND** unavailable counting uses conservative admission while operational counting failures retain the draft without dispatching generation

#### Scenario: Insufficient space for all selected sources
- **WHEN** history, protocol, answer reserve and the selected sources cannot fit
- **THEN** the request retains its draft and files and offers a lower response limit, a supported larger context or fewer selected files
- **AND** it does not silently omit a source, delete history or replay a prior turn

### Requirement: Conversation-private attachments across clients
Web, desktop and CLI SHALL support explicit file attachment through the same
authenticated service contract. Ordinary attachment SHALL NOT automatically
publish content to the shared Knowledge library or grant filesystem authority.

#### Scenario: Attach, send and reopen a document conversation
- **WHEN** an authorized user attaches a supported file, sends a question and reopens the conversation
- **THEN** the retained source identity and extraction state remain inspectable without uploading again or replaying inference
- **AND** another actor cannot access the attachment by guessing its ID

#### Scenario: Failed extraction or send
- **WHEN** a file is unreadable, unsupported, missing OCR support or exceeds the available context
- **THEN** both CLI and UI show an actionable failure and retain the pending input
- **AND** no attachment is silently omitted, published as ready or truncated into a misleading answer

#### Scenario: Attachment changed during a durable retry
- **WHEN** a request ID is reused with changed attachment identities or revisions
- **THEN** admission reports a conflict instead of executing a different request

#### Scenario: Optional Knowledge publication
- **WHEN** a user explicitly chooses to add a conversation file to Knowledge
- **THEN** existing Knowledge authority and import/retention checks apply independently
- **AND** ordinary attachment alone does not trigger this operation

### Requirement: Truthful format readiness
The service SHALL expose supported formats and their required installed local
runtime/languages. Unsupported or unavailable formats SHALL fail with actionable
typed errors without request panics or claims of successful ingestion.

#### Scenario: PDF dependency unavailable
- **WHEN** a valid PDF is uploaded without an optional extraction dependency
- **THEN** a qualified packaged parser processes it or the service reports the missing capability
- **AND** the server stays healthy and no ready document is created from fallback garbage

#### Scenario: Unsupported legacy Office file
- **WHEN** a binary XLS file is uploaded without its dedicated adapter
- **THEN** it is identified as requiring that adapter rather than passed to a ZIP/XLSX reader or decoded as plain text

### Requirement: Faithful structured extraction
Extraction SHALL preserve valid Unicode and ordered source units with original
identity, parser revision and warnings. Unreadable, encrypted and partial content
SHALL remain distinguishable from a complete extraction.

#### Scenario: Multilingual structured document
- **WHEN** a qualified PDF, spreadsheet or slide deck contains Unicode text and multiple source units
- **THEN** extracted text preserves the supported scripts and unit ordering with locators
- **AND** no silently omitted unit or cell is represented as fully processed

### Requirement: Local scanned-document processing
Scanned and mixed documents SHALL use installed local OCR only where needed,
retain OCR provenance and report missing language data or low-confidence results.

#### Scenario: Mixed PDF with unavailable OCR language
- **WHEN** some pages have native text and other pages need an unavailable language pack
- **THEN** native text is preserved, missing pages are identified, and setup or partial-use choices are explicit
- **AND** no network download or cloud processing begins automatically

### Requirement: Bounded cancellable ingestion
The service SHALL bound input, decompression, page/raster size, output, execution
time and indexing concurrency. Cancelling an import SHALL stop its owned work
without affecting unrelated tasks or exposing partial results as ready.

#### Scenario: Malformed or expanding input
- **WHEN** a parser encounters malformed data, a decompression bomb, timeout or cancellation
- **THEN** the operation ends with a bounded error, cleans task-owned temporary state and preserves service availability

### Requirement: Durable import and atomic publication
Accepted imports SHALL have stable identities and persisted stages. Retry and
restart SHALL preserve originals and avoid duplicate publication. A replacement
index SHALL become current only after successful validation.

#### Scenario: Crash during indexing
- **WHEN** the service restarts after extraction but before index publication
- **THEN** it resumes or reports the durable import state without duplicate documents or a false ready state
- **AND** an existing valid index remains available until replacement succeeds

### Requirement: Traceable sources and scoped access
Retrieval, citations, originals, exports and caches SHALL obey current document
access policy. Source locators SHALL identify actual pages/sections/cells and
content revision; passing a document ID SHALL NOT grant access.

#### Scenario: Permission revoked after retrieval
- **WHEN** an actor loses access before opening or exporting a cited source
- **THEN** delivery is denied and cached content from that source is not exposed

### Requirement: Explicit retention and deletion
Authorized deletion SHALL apply to retained originals, extracted text, derived
indexes and active caches under the selected retention policy. Backup retention
and legal holds SHALL be explicit rather than implied by history removal.

#### Scenario: Document removed and service restarted
- **WHEN** a permitted document deletion completes and the service restarts
- **THEN** the document is absent from search and source access, and derived content does not reappear from a stale job or cache

### Requirement: Real-document qualification
Advertised format/platform support SHALL be backed by packaged real-document
extraction and retrieval tests, including corruption, scans, multilingual content
and citations. Mock providers SHALL NOT establish native parser qualification.

#### Scenario: Extension accepted but content extraction fails
- **WHEN** a format passes upload validation but fails its real fixture corpus
- **THEN** it is not advertised as qualified and the failed evidence remains recorded
### Requirement: Inspectable document previews and honest partial use
Authorized clients SHALL inspect retained document units and PDF pages without
publishing, indexing or submitting them. Rendering SHALL be bounded, local and
subject to current source access. Partial native text SHALL remain inspectable;
using it for answers SHALL preserve and disclose unread content. Substantial
native text on every nonempty page may be admitted with native-text-only coverage;
other incomplete extraction requires explicit partial-use acknowledgment.

#### Scenario: Native research PDF with figures
- **WHEN** every nonempty page has usable native text and the bounded classifier excludes large scanned surfaces and sparse text
- **THEN** Chat accepts that text without a repeating OCR/retry gate
- **AND** the attachment, source pages and answer context disclose uninterpreted visual content without claiming complete extraction

#### Scenario: Scanned page with a native header
- **WHEN** a page contains only sparse native text or a large scanned surface
- **THEN** it still requires OCR or explicit partial-use recovery
- **AND** successful classification of other pages cannot make the missing page complete

#### Scenario: Native text inside a PDF form
- **WHEN** a page contains native text or vector-only nested forms
- **THEN** form presence alone does not require OCR
- **AND** bounded inspection still flags actual raster content or uncertain structure

#### Scenario: Preview a partially extracted PDF
- **WHEN** an authorized user opens a source whose OCR is unavailable
- **THEN** original pages and available text remain inspectable with missing coverage identified
- **AND** previewing alone does not make it ready for complete-document answers

#### Scenario: Source access or revision changes
- **WHEN** a preview request references an inaccessible or changed source
- **THEN** it fails without exposing another source or a stale citation as current
