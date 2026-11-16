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

### Requirement: Literal multilingual evidence retrieval
Chat attachment and Knowledge retrieval SHALL share Unicode-aware lexical
segmentation, retaining short identifiers, numbers and unspaced-script n-grams.
The query builder SHALL bound input and term counts, quote literal search data,
and rank term matches without requiring every conversational term. Index
normalization SHALL NOT change original evidence or UTF-8 source offsets.

#### Scenario: Punctuation and Chinese questions
- **WHEN** users ask `harvest?` or a natural Chinese question whose evidence lies in the middle of a large attachment
- **THEN** relevant source passages remain retrievable through the HTTP and Chat paths
- **AND** exact source readback retains original text and revision identity

#### Scenario: Keyword channel is unavailable
- **WHEN** the lexical index fails
- **THEN** retrieval reports failure or explicit degradation instead of returning an ordinary empty result or claiming hybrid retrieval

### Requirement: Coverage survives publication and saved answers
Processing status, extraction coverage and indexing status SHALL remain separate.
Parser coverage and warnings SHALL survive imports, attachment readiness,
previews, saved evidence, CLI presentation and model context. Ready/indexed text
SHALL NOT imply understanding of unprocessed figures, notes, pages or objects.
Partial-use acknowledgement and incomplete-review disclosure SHALL remain in force.

#### Scenario: Policy exception in a referenced DOCX note
- **WHEN** an exception is stored in a footnote or endnote referenced by visible body text
- **THEN** extraction includes the exception beside its reference and retains the original-file digest
- **AND** retrieval, saved evidence and citation opening preserve that exception

#### Scenario: Ready text with unread objects
- **WHEN** a DOCX, spreadsheet or presentation becomes searchable without full object/layout extraction
- **THEN** clients display the relevant limitations in the selected interface language
- **AND** a saved ready attachment cannot lose or change its coverage receipt under the same extraction revision

### Requirement: Immutable structured extraction revisions
Extraction SHALL publish immutable revisions with original identity, parser and
language-pack versions, typed source units/blocks/cells, exact text spans,
available coordinates, reading order, heading/table/note relationships, and
document/block coverage. Native transcription SHALL remain authoritative;
conflicting OCR/model interpretations SHALL remain separate reviewable alternatives.
Corrections SHALL create attributed derived revisions and publish affected indexes
atomically while preserving authorized older citations under retention policy.

#### Scenario: A table or policy clause spans multiple chunks
- **WHEN** layout-aware chunking divides a table or clause
- **THEN** each selected excerpt carries its headers, units and linked exceptions with original cell/block references
- **AND** opening the citation shows those relationships without fabricating page coordinates

### Requirement: Explicit offline extraction capabilities
The core SHALL remain suitable for separately qualified 8 GB CPU operation.
Modern Office, PDF, image and multipage TIFF extraction SHALL preserve the
approved structures and disclose unsupported material. Printed OCR SHALL use
explicitly selected pinned digest-verified language packs, never an implicit
related-language substitution. The optional Docling standard layout/table worker
SHALL use an OffGrid-owned versioned protocol, packaged runtimes/assets, no network
or runtime downloads, bounded queues and page checkpoints. Whole-page generative
transcription SHALL NOT silently replace native text.

#### Scenario: Missing OCR language or optional layout pack
- **WHEN** a document needs an unavailable capability
- **THEN** readiness identifies that missing capability without downloading it or sending content to a remote service
- **AND** available evidence remains inspectable with its limits

#### Scenario: Expensive processing is interrupted
- **WHEN** processing is paused, cancelled or restarted
- **THEN** completed page checkpoints remain reusable and incomplete work cannot publish a complete revision

### Requirement: Bounded retrieval and deterministic evidence identity
Each retrieval channel SHALL nominate at most twenty candidates before fusion,
deduplication and diversity selection. SQLite SHALL remain authoritative; vector
scoring SHALL use versioned binary projections and bounded working sets, loading
bodies only for shortlisted candidates. Collection access SHALL filter candidates
before ranking and be rechecked before generation/source delivery. Scores SHALL
remain ranking signals, never accuracy percentages.

#### Scenario: Existing index is upgraded
- **WHEN** derived multilingual or binary-vector projections are introduced
- **THEN** original text, hashes, ownership and configured embedding identity remain unchanged
- **AND** the pre-upgrade database is backed up and checked before schema changes

### Requirement: Accounted-for document analyses
The `/v1/document-analyses` resource family SHALL support durable summary,
requirements and comparison workflows, with status/cancel/results, resumable unit
checkpoints and independent coverage receipts. Question answering SHALL identify
selected evidence rather than imply exhaustive traversal. Analysis output SHALL
validate citation identity/ranges/access and deterministic numeric/cell arithmetic;
unsupported sections SHALL become gaps before delivery. Model support checking
SHALL NOT be presented as independent verification.

#### Scenario: Required source units remain unread
- **WHEN** a summary or requirements workflow reaches its end with unread required units
- **THEN** its result is explicitly incomplete and cannot claim a full review

#### Scenario: Model supplies a fabricated quotation or number
- **WHEN** a candidate section contains an unknown evidence identity, changed or ambiguous quote, or an unsupported numeric transcription
- **THEN** the candidate is withheld and its gap remains in the processing receipt
- **AND** passing source-identity checks does not label the remaining prose independently verified

#### Scenario: Model or source access changes during a batch
- **WHEN** the pinned model/runtime identity changes, the exact prompt cannot fit, or source access is revoked before publication
- **THEN** the batch produces no publishable result and does not automatically retry generation
- **AND** counting and generation use the same local runtime lease, released between source batches

### Requirement: Inspectable document workspace
Chat and Knowledge SHALL share a reader supporting source highlighting,
original/extracted/side-by-side inspection, outline, coverage map and authorized
corrections. Library metadata search SHALL be distinct from content search.
Collections, pagination, batch recovery, filters, duplicate detection within an
access boundary, version history and CSV/JSON/Markdown evidence exports SHALL
retain source revisions and unresolved gaps. New controls SHALL use all 29
locales and pass keyboard, RTL, narrow-layout, zoom and theme checks.

#### Scenario: Navigation after a partial import
- **WHEN** a user leaves and returns to a partial document
- **THEN** draft text, reading position, selected sources and recovery state remain available
- **AND** the user can identify and inspect missing source material without developer assistance

### Requirement: Departmental collection enforcement
The `/v1/collections` resource family SHALL expose revision-checked membership
and owner/editor/reader roles, private by default. Global administration SHALL
NOT imply private-content access. Legacy Knowledge SHALL retain its existing
workspace audience without reassignment to the first authenticated user. Chat
publication SHALL require an explicit destination/audience review. Permissions,
revocation, holds and retention SHALL cover originals, revisions, page images,
jobs, indexes, saved evidence and exports; audit events SHALL omit document bodies.
Backup/export retention domains and already-downloaded-copy limits SHALL be explicit.

#### Scenario: Membership is revoked during work
- **WHEN** an actor loses collection access while generation or an export is active
- **THEN** subsequent source delivery and saved-evidence reads deny access consistently
- **AND** no stale cache or administrative role restores private access

#### Scenario: Reviewed audience changes before a move or import publishes
- **WHEN** either reviewed movement policy or the reviewed import destination changes
- **THEN** the mutation rejects the stale revision without publishing to the new audience
- **AND** current authorized users can still inspect or cancel a pending import
- **AND** successful sharing changes and movement commit with content-free audit events

#### Scenario: A private import reaches an older service
- **WHEN** a client sends a reviewed collection import to a service without the new resource
- **THEN** the client uses the distinct collection-import endpoint and reports unavailable
- **AND** it does not fall back to a legacy shared upload

### Requirement: Qualified optional visual and archive processing
Visual interpretations SHALL remain distinct from extracted facts and linked to
their source regions. Chart estimates, handwriting, signatures and stamps SHALL
NOT acquire unsupported accuracy/authenticity claims. Optional legacy/ODF and
email workers SHALL retain originals/provenance and disable macros, external
links, network access and remote resource fetching.

#### Scenario: Visual or archive support has not passed its release gate
- **WHEN** a user opens an unqualified format or requests an uncertain interpretation
- **THEN** capability discovery and the result disclose the limitation instead of advertising full support

### Requirement: Evidence-based release qualification
Qualification SHALL use versioned provenance-recorded documents and labelled
passage-level evidence, including exceptions, numeric fields, unanswerable and
adversarial questions. The approved 200-document/1,000-question corpus and
per-language/OCR/hardware targets SHALL remain proposed gates until measured.
Human linguistic review and departmental pilot evaluation SHALL remain explicit
external gates. Contract/build/spec checks SHALL NOT count as model-quality or
native-review evidence.

#### Scenario: Correct document but missing exception
- **WHEN** retrieval returns the correct document but omits a labelled exception span
- **THEN** the evidence-level evaluator records a failure despite the document-level hit
- **AND** per-language failures are reported separately from the aggregate

### Requirement: Private attachment structural identity
Private attachments SHALL retain structured revisions without publishing them to
Knowledge. Retrieval and inspection SHALL verify the original and extraction
identity. Missing or corrupted retained graphs SHALL NOT silently downgrade to
unstructured evidence. Explicit linked headers and exceptions SHALL be included
as exact source spans when they fit, with omitted links disclosed in persisted
answer receipts and model context. Read-only inspection SHALL NOT confer correction
rights. Legacy sources SHALL retain their existing text-reading behavior.

#### Scenario: Private exception survives saving and citation opening
- **WHEN** a large private document's relevant clause links to a distant exception
- **THEN** selection includes both exact spans within the available budget or explicitly reports the omitted dependency
- **AND** saved answer citations open the same retained source revision after reload
- **AND** another user's request cannot read its blocks or relationships

### Requirement: Bounded and truthful image admission
Chat image admission SHALL check encoded size and pixel dimensions before image
allocation, then validate actual pixel data. Browser batches SHALL decode
sequentially and preserve original bytes. Generic and saved Chat SHALL require
an existing nonempty projector before admitting local vision inference.
Image transport SHALL NOT imply OCR, chart accuracy or document understanding.

#### Scenario: Valid image header without usable pixels
- **WHEN** a request contains a PNG/JPEG header with incomplete pixel data
- **THEN** both generic Chat and saved Chat reject it before inference

#### Scenario: Small compressed file with oversized dimensions
- **WHEN** a browser upload declares dimensions outside the supported pixel budget
- **THEN** admission rejects it before invoking the bitmap decoder
- **AND** previously accepted images and the text draft remain intact

#### Scenario: Photo orientation and ambiguous animation
- **WHEN** a still PNG/JPEG has a valid primary Exif rotation or mirror transform
- **THEN** runtime token counting and generation use the same bounded upright pixel projection while saved original bytes and durable request identity remain unchanged
- **AND** animated/multi-picture images or ambiguous orientation metadata require an explicitly exported still image rather than silent first-frame selection

### Requirement: Standalone printed-image evidence
PNG/JPEG document ingestion SHALL use bounded actual pixel decoding and the
isolated local OCR worker, independently of Chat vision models. Originals SHALL
remain immutable. Upright OCR/viewer derivatives SHALL use the same explicit
alpha composition. Source coordinates SHALL identify their original pixel frame.
OCR-only success SHALL disclose unprocessed visual content. Missing components
and low-confidence results SHALL NOT be silently promoted to ready evidence.

#### Scenario: Image source is reopened without its OCR pack
- **WHEN** a user opens an authorized image document whose OCR text was already saved
- **THEN** the reader displays that retained text and a source-validated raster without rerunning OCR
- **AND** missing OCR components do not substitute or erase saved text
- **AND** stale-source and access checks apply to both raster and text delivery

#### Scenario: Uncertain image transcription
- **WHEN** the worker returns incomplete or low-confidence printed text
- **THEN** the original and available review text remain inspectable with partial coverage
- **AND** the result does not enter generation without explicit limited-text acknowledgement, and never enters a complete Knowledge index
- **AND** acknowledgement retains partial coverage and recognition uncertainty in saved sources, readers and model context; it does not certify the OCR text

### Requirement: Referenced Word header and footer evidence
DOCX extraction SHALL retain live referenced header/footer text as identifiable
source sections, preserving original bytes and package-part provenance. Unused
and historical section references SHALL NOT enter the current reading. External,
missing, escaping or conflicting part references SHALL fail explicitly without
network access. Unrendered page placement and field values SHALL remain subject
to review; extracted sections SHALL NOT be presented as rendered page numbers.

#### Scenario: A policy exception appears only in a footer
- **WHEN** a referenced footer contains an exception absent from body paragraphs
- **THEN** ingestion, retrieval, model context and saved citation reopening retain the exception and its source identity
- **AND** the reader retains a coverage warning for unresolved page placement
- **AND** older immutable extraction revisions are not silently rewritten
### Requirement: Conversation publication requires a reviewed audience
Clients SHALL select a collection and acknowledge its current audience before
publishing a conversation-private attachment. The selected policy revision SHALL
bind both acceptance and publication. Changing the account, workspace or selected
destination SHALL discard the interface acknowledgement. Creating a private
collection SHALL NOT implicitly acknowledge or publish an attachment.

#### Scenario: A library batch changes audience during processing
- **WHEN** a user selects files for the library and reviews a destination collection
- **THEN** no file bytes are transferred before explicit audience acknowledgement
- **AND** every acceptance and publication in that batch uses the same reviewed revision
- **AND** an audience conflict or unconfirmed receipt stops later files without shared-storage fallback
- **AND** cancelling the review discards the local selection without creating imports

#### Scenario: An older client omits collection review
- **WHEN** a client submits a saved attachment for Knowledge publication without a destination or reviewed policy revision
- **THEN** the service returns HTTP 426 with `collection_review_required` before copying source bytes
- **AND** the client does not fall back to a shared multipart publication

#### Scenario: A collection creation response is lost
- **WHEN** private collection creation is not acknowledged
- **THEN** the interface requires inspecting collections before retrying creation
- **AND** no attachment is published automatically

### Requirement: Human review remains separate from generated evidence
The service SHALL retain append-only human review revisions pinned to an immutable
analysis section and response digest. Reviews SHALL require private job ownership
and current access to every source. A review SHALL NOT overwrite the model output,
clear extraction gaps, imply independent verification or certify exhaustive review
of unread original material. Clients SHALL retain drafts across review pagination
and analysis tabs in the same dialog and expose saved attribution and history.

#### Scenario: Two windows review the same section
- **WHEN** a second window saves against a revision that has already changed
- **THEN** the service rejects the stale save without changing either original candidates or the newer review
- **AND** the client offers reloading the saved review before another write

#### Scenario: A review save response is lost
- **WHEN** the service commits a review but its response is not confirmed
- **THEN** the client does not automatically replay the write
- **AND** the user can reload the saved revision and inspect its decisions and author

#### Scenario: Reviewed material is exported or revoked
- **WHEN** an authorized owner exports a finished candidate traversal
- **THEN** original candidate evidence and extraction gaps remain alongside keep/exclude/unreviewed decisions, notes and section-check counts
- **AND** source revocation blocks current and historical review reads, writes and exports
- **AND** enforced source deletion removes derived review history, while legal holds prevent that deletion
