## Private structured evidence increment (2026-11-13)

Private attachments retain an immutable graph beside their existing text record.
The record's source digest binds the parser, extraction revision and coverage;
ready records cannot be replaced. Graph text is reconstructed from that exact
record, and its fingerprint is verified before retrieval or inspection. Unicode
outer whitespace is preserved by rebasing spans into a newly sealed envelope.
Graph publication precedes the record pointer; interrupted derivatives consume a
separate owner quota and are removed by authorized source deletion. Limits are
32 MiB per graph, 256 MiB of retained graph data per owner, and 64 MiB of graph
data per attachment-source load, separately from original/text limits.

Chat and Knowledge share explicit relationship traversal. Chat selects at most
20 lexical candidates, adds exact separately citable dependency spans within
its measured context, and saves a bounded link receipt. Missing dependencies or
receipt limits never become a complete-review claim. Excerpt size accounts for
metadata; an unrelated opening/tail cannot replace a matching passage merely
because the matching evidence and its disclosure did not fit.

The private structure endpoint pins the attachment revision and returns the same
bounded block/relationship projection as Knowledge. Ownership, original digest
and graph identity are checked on each read. It never reparses or grants write
access. Existing attachments without a retained graph remain readable through
their existing paths; structured inspection reports unavailable. Private
corrections still require a distinct derived-revision workflow. The shared web
reader, CLI and Python expose inspection; Python execution is not yet verified.

## Departmental collection API increment (2026-11-13)

Expose the existing collection policy through authenticated read/manage guards,
never administrator-derived private access. Creation accepts only a name and is
private by default. Owner-only updates require explicit member arrays and the
current revision. Document movement requires the binding revision and both
audience revisions; changing a private source's audience requires its owner.

Use `/v1/collections/imports` for new reviewed imports. Its distinct path is
necessary because older services ignore unknown destination query fields on
legacy upload endpoints. Never fall back to a shared upload. Persist the reviewed
collection revision with the pending import and check it again in the publication
transaction. Audience changes do not prevent authorized cancellation/cleanup.
Keep legacy workspace behavior for old uploads until the coordinated UI and
client migration can require explicit destination review for every attachment.

Policy schema 5 adds a content-free collection audit ledger and the import's
reviewed revision. Policy updates and audit writes are one transaction. Snapshot
existing databases before this additive upgrade, preserving source bytes and
bindings; audit/snapshot retention remains distinct from source retention.

## Context (baseline when this change was opened)

The current parser uses PATH-based `pdftotext`, then a regex containing unsupported
Go lookahead. No packaged PDF dependency was found in the installed Windows app.
`cleanExtractedText` discards valid Unicode scripts. `.xls` is routed into a ZIP
XLSX reader; spreadsheet inline strings/formulas/coordinates are not preserved.
Imports flatten content before indexing; `RawContent` retains extracted text,
not the original file. Page fields exist downstream but are not populated by the
current chunker. Upload errors all become HTTP 500. The outer middleware limits
requests to 64 MiB; ParseMultipartForm's 50 MiB argument is a memory threshold,
not the advertised file-size ceiling. Parser-specific real-document tests are
missing. Existing atomic index replacement and embedding identity checks remain.

## Goals / Non-Goals

Deliver reliable local extraction and evidence-grounded retrieval for qualified
formats, with a simple client experience. Do not promise arbitrary document types,
perfect OCR, accurate interpretations from every model, or compliance certification.
No embedded document scripts/macros, cloud conversion, package auto-installation,
automatic reindexing of existing data, or blanket privilege expansion.

## Decisions

### Approved upgrade defaults (2026-11-12)

Keep the existing Go service, SQLite, isolated native readers and React clients.
Use one additive structured-source contract for private attachments and Knowledge,
without merging their authority. Literal multilingual query terms must not be FTS
operators; failures must not silently remove retrieval channels. Preserve originals,
native characters and existing revisions; corrections create derived revisions.
Expose coverage independently of job/index completion and carry it into prompts.
The optional advanced pack uses the offline Docling standard layout/table pipeline,
not generative full-page replacement. Pack installation and model changes are explicit.
Add `/v1/collections` and `/v1/document-analyses` with revision-checked ownership and
existing durable-job semantics. Departmental private APIs stay gated until all
source, saved-answer, job, export and revocation paths pass. Migration keeps the
legacy workspace audience. Existing clients retain additive compatibility.

Release gates are per format/language/model/hardware: 200 documents/1,000 labelled
questions, >=95% top-20 evidence recall, >=97% supported claims, >=95% unanswerable
handling, >=98% table-cell accuracy and >=95% header association; clean printed OCR
<=2% character error. These are targets, never inferred from unit tests. Require
human language review and pilot evidence separately. No installation/publication
or private-document benchmark is implied by source implementation.

1. **Typed parser contract.** Each adapter returns ordered source units (page,
   slide, sheet/cells, section), stable source digest, extractor revision, warnings
   and completion state. A registry describes actual runtime availability,
   encrypted-file policy, bounds and qualification; filename extensions alone
   never establish compatibility. Keep the existing API through additive adapters.
2. **First repair, then qualification.** Freeze reproductions for PDF fallback,
   Unicode and misleading XLS support. Remove request panics and silent data loss
   before any release. A regex patch alone is not qualified PDF support. Evaluate
   pinned real PDF engines against the corpus, including font maps, compressed
   streams and object streams; prefer an embedded/offline packaged engine over
   a prerequisite on the user's PATH. Supervise native parsing outside inference
   and terminate owned workers on deadline/cancellation. No model credentials.
3. **Local OCR.** Render only pages needing OCR, then use a pinned local OCR
   runtime and explicitly installed language data. Preserve native text on mixed
   pages, track OCR provenance/confidence and avoid duplicate text. Offer a clear
   one-time language pack setup or offline import when missing. Low-confidence or
   partial extraction stays visible and cannot be mislabeled fully indexed.
4. **Bounded untrusted input.** Enforce file bytes before reading, decompressed
   bytes/entries for ZIP, page count, dimensions, worker memory/time, extracted
   text and indexing batch limits. Handle malformed/encrypted/empty documents
   with typed recoverable errors. All temporary state is task-owned and removed
   on completion/cancel; diagnostics exclude private document bodies.
5. **Durable import.** Persist accepted original bytes and metadata under scoped
   content IDs, then queued/extracting/needs-input/indexing/ready/partial/failed/
   cancelled stages. Publish document and index atomically; retry uses the same
   import identity. Keep old usable index until replacement succeeds. Crashes,
   full disk, process death and concurrent retries must not produce ghost-ready
   records or duplicate library entries. Existing legacy text sources remain
   readable; original-file absence is explicit rather than fabricated.
6. **Citations and access.** Chunk within source-unit boundaries and retain exact
   locators. Apply actor/collection/document policy before retrieval scoring and
   again before source/export delivery; selection is not an ACL. Namespace caches,
   deduplication and jobs by authority. Migration keeps the legacy workspace
   library under its existing permissions; no automatic sharing. Retention covers
   original files, extracted text, vectors, jobs and caches; backups follow their
   explicit separate retention policy. Do not call shared role-based access
   tenant isolation before cross-user tests pass.
7. **Format rollout.** Qualify text PDFs, DOCX, XLSX, PPTX, TXT/MD, CSV, HTML/XML/JSON
   and RTF with real fixtures. Scan/mixed PDFs and image documents require OCR.
   Legacy DOC/XLS/PPT, ODF and email need dedicated reviewed adapters; explicit
   unavailable states precede support rather than unsafe generic text fallback.
8. **Low-resource scheduling.** Queue imports independently of UI and inference;
   limit extraction/OCR concurrency, show per-file progress and permit cancellation.
   Existing installed embedding model wins; recommended setup is explicit and
   does not silently change existing indexes or fetch network dependencies.

### OCR page recovery (implementation in progress, 2026-11-14)

Retain validated worker receipts at page boundaries under the existing private
attachment or import identity. A receipt key binds raster bytes, dimensions,
selected pack digest and worker/projection versions. Loading always revalidates
the receipt; incomplete or failed worker responses are never reusable results.
SQLite import checkpoints share the import's current collection/attempt checks
and cascade on removal. Private attachment checkpoints use the existing scoped
root and mutation lock; deletion removes them and stale workers cannot recreate
deleted source authority. Both stores have byte and entry bounds. Explicit retry
reuses saved OCR pages, including uncertain text with its original review state;
it does not promote partial coverage. Native extraction/raster preparation may
repeat, while OCR inference does not. No cross-user cache or runtime download is
introduced. Dedicated pause/progress controls and advanced-worker scheduling remain
part of the broader task.

### Bounded reader thumbnails (2026-11-14)

Reuse the private/library preview routes with `page` and `thumbnail=true`.
Thumbnails are read-only, at most 192 by 256 pixels, and share extraction
admission. Source access is checked again before delivery; response identity
binds original SHA-256 and derivative kind. The renderer requests them only when
the source advertises support and the user expands Pages. It retains four small
images, requests them sequentially, cancels pending work on close/navigation and
revokes object URLs. Payload bytes and PNG dimensions are checked before display.
The existing full-page reader, coverage, source highlighting and keyboard
navigation remain authoritative. No OCR or cross-source pixel cache is involved.

## Risks / Trade-offs

### Durable analysis traversal and comparison (2026-11-13)

Reuse SQLite, the existing import worker lifecycle pattern and collection policy
checks for owner-private analysis records. Requests bind explicit source revisions
and an idempotency key. A single comparison worker checkpoints each complete unit
pair, atomically storing results and accounted units. Restart marks running work
interrupted; explicit retry resumes after committed checkpoints. No embedding or
generation model is required for deterministic comparison.

Unique exact lines establish monotonic anchors without a quadratic diff matrix.
Unmatched spans remain additions/removals, not guessed semantic equivalence.
Unique sheet names establish cell-address correspondence; other units use explicit
positional alignment. Renamed/ambiguous sheets are unmatched. Native cell metadata
is compared separately so formulas, raw values and cached results remain distinct.
Result citations are validated against retained source bytes before delivery.

Source-unit traversal and original-file coverage remain different guarantees.
Unknown coverage, unread units, figures and unresolved review items keep completed
traversals incomplete. All source access is rechecked, and deleting any source
cascades to the job and checkpoints under existing source hold enforcement.
Current APIs expose comparison only; summary/requirements processors and model
synthesis must use the same traversal accounting before being advertised.

Comparison client increment (2026-11-13): add bounded owner-private history using
the indexed job/source identity projection, not whole-plan deserialization. History
recovery is read-only and never automatically replays an uncertain mutation.
Result previews reuse the exact bytes already checked by citation validation,
cap each side at 512 UTF-8 bytes and include span/digest/truncation receipts.
Web and Go clients validate these receipts; Python parity is authored but unrun.

The app uses a dedicated comparison workspace with separate setup/results/history
views, a compact persistent header, independent before/after columns and contextual
source inspection. Columns retain their source order; visual juxtaposition does
not claim semantic alignment between removed and added passages. Coverage details
reuse the shared structured reader in read-only mode at the job's pinned revision.
Unknown outcomes, unavailable sources and revoked access clear actionable cached
evidence. Presentation controls reuse theme tokens and all 29 language catalogs.

Comparison exports prepare complete JSON/CSV/Markdown artifacts before delivery.
Verify immutable source graphs once, then each changed range/hash; include native
cell metadata, all unit identities and unresolved gaps. No model is invoked.
Keep one HTTP preparation slot, a 25-second request budget and an 8 MiB artifact
limit; fail without partial output. Current source ACLs and a content-free
preparation audit event are checked before delivery. CSV formula-like display
fields are escaped while exact values remain in metadata; Markdown uses inert
fences. Clients validate identity, format, size and SHA-256 before saving. CLI
creates only new files. External copies have independent retention and cannot be
recalled by later source deletion or revocation. The app keeps export choices in
a compact disclosure and gates them on the advertised export capability.

### Summary and requirements synthesis core (2026-11-13)

Connected implementation update: additive candidate-workflow endpoints now use
schema-v2 analysis jobs and a separate model/cursor configuration, sharing the
bounded analysis queue and retention boundary. Checkpoints retain private raw
responses for deterministic revalidation; public responses expose only checked
candidate receipts. The owned local runtime forbids implicit downloads and
releases its inference lease between batches. UI and native clients expose
`awaiting_review` explicitly. Final support assessment and cross-section synthesis
remain open; candidate traversal is not complete-answer qualification. Evidence:
`docs/quality/evidence/document-synthesis-jobs-2026-11-13.md`.

Reuse the immutable analysis plan and shared structural links for bounded
source-unit batches. Preserve contiguous UTF-8 spans, paragraph boundaries when
available, separately cited headings/headers/exceptions, and native cell metadata.
Do not use byte bounds as token estimates: count the exact final prompt under the
same installed local model lease used for generation, reserving output and
template headroom. Pin model digest, runtime, language, context and protocol in
the generation receipt. Yield the lease between batches. Current-access guards
run before admission, after counting and after generation; durable publication
must recheck again in its own transaction.

Model JSON is an untrusted candidate protocol, with exact unique quotes and
separate requirement conditions/exceptions. Reject ambiguous envelopes, unknown
evidence identities and altered quotations; withhold candidates whose numeric
tokens do not appear exactly in quoted evidence. These deterministic checks prove
source identity and conservative transcription only, not semantic support or
arithmetic. All model candidates retain explicit semantic-review status. A model
reporting no requirements is not verified absence. The future support/synthesis
stage must preserve that distinction; it cannot label a second model independent
verification. Empty or rejected output and omitted dependencies remain gaps.

The internal core is implemented separately from public workflow advertising.
Durable synthesis checkpoint integration, the service local-runtime adapter,
support assessment, section synthesis, API/client/reader controls and actual-model
qualification remain required before enabling summaries or requirements.

### Standalone image OCR increment (2026-11-13)

2026-11-14 language increment: service and worker consume one immutable
`internal/documentocr/languages.json` manifest. The service imports only matching
pack bytes and reports availability independently of unverified quality. A saved
single-language selection belongs to the private attachment or durable import,
not the UI locale. Policy schema 6 snapshots before adding the import selection;
existing records retain English without ownership/audience changes. A new client
requires `document-ocr-language-selection-v1` before sending an explicit choice
and verifies that choice in the receipt. Attachment publication preserves the
saved choice. The Chat readiness endpoint is read-only under Chat permission;
pack installation remains Knowledge-management-only. This is an additive step
toward multiple packs/page hints, not a mixed-script or language-quality gate.

Multipage TIFF extension: enumerate classic TIFF page directories
with bounded counts, metadata and decoded dimensions before processing. Decode
one page at a time using pinned x/image v0.45.0, preserve original bytes and
per-page orientation, and retain ordered page spans and OCR geometry. Unsupported
BigTIFF/subimage structures fail explicitly. Source preview decodes only the
requested page and never reruns OCR. Retained structured page spans, including
partial results, bind review text to the original source rather than guessing
page boundaries from separators. Keep TIFF unqualified until corpus evidence;
deskew, multilingual packs and crash-resumable OCR checkpoints remain part of 8.3/8.4.

PNG/JPEG files use bounded standard-library decoding plus the existing isolated,
digest-checked OCR worker. Decode headers before pixels; reject animation,
multi-picture JPEGs and ambiguous EXIF. Preserve original bytes and hash. OCR
receives a lossless upright PNG derivative, using the same explicit white alpha
composition and 8-bit conversion as the source preview. Store original-pixel
line coordinates, runtime/language identities and printed-text-only limitations.
Successful text extraction does not claim interpretation of figures or tables.
Weak OCR remains partial with retained review text and no automatic generation.
Opening sources never reruns OCR; saved OCR text is loaded under current access.
TIFF, deskew, multilingual manifests/selection and representative qualification
remain required under 8.3, rather than being inferred from PNG/JPEG completion.

### Conversation files and client parity

Use the existing authenticated session owner as the attachment boundary. Upload
original bytes through bounded multipart ingestion, retain a digest-bound source
record outside prompts, and return an opaque attachment reference. Recheck session
ownership on list/read/removal and every inference admission; filenames and file
references confer no authority. Exclude raw bytes from ordinary session listings.

Extraction uses the shared local parser/readiness contract. A file becomes usable
only after extraction succeeds; keep processing, unsupported and failed states
distinct. Display supported type/size limits before upload. Small source text can
be supplied as explicitly untrusted document context; larger sources need bounded
retrieval/source units and must not be silently truncated or overflow the model
context. No mandatory embedding setup for ordinary supported small attachments.

Conversation source selection validates retained revisions and deduplicates sources
across turns before allocating context. Small sources remain complete. Larger
sources use deterministic local lexical retrieval with opening/distributed passages.
Production llama streams select excerpts under the same loaded-model lease as
generation, after Knowledge enrichment. The runtime tokenizer measures the full
template, history and candidate sources. Serialized UTF-8 bytes only bound candidate
selection; they are not counted as history tokens. At most six read-only candidate
counts may shrink excerpts without dropping any selected source. Providers lacking
token counting retain conservative byte admission. Cancellation and operational
count failures do not trigger generation, model switches or replay. Explicit answer
limits remain unchanged; genuine exhaustion offers a lower answer limit or a
supported larger context. Runtime generation admission remains authoritative. Exact extracted-text byte
ranges and immutable source revisions are persisted with the answer; they are not
invented page coordinates. Every selected source receives space or the request
fails with actionable recovery. History and original files are never truncated.
Web and CLI disclose excerpt coverage independently of model prose. This is a
limited view, not whole-document summarization or visual interpretation; source
selection evidence cannot be imported through client-seeded history.

Web and desktop share an Add files control alongside text, image and voice inputs.
CLI one-shot file flags and interactive `/attach`, `/attachments` and
`/remove-attachment` commands call the same service. Existing image commands remain
compatible. Both retain pending references after errors and restore saved sources
after reload without replaying generation. Knowledge publication is explicit and
uses its separate access/retention contract. Closing a draft or removing a chip
must distinguish draft removal from deleting a source already used by history.

Durable turn identity includes attachment IDs and source revisions. Concurrent
deletion, account changes, revoked access and missing files fail before inference;
changing attachments under an existing request ID conflicts. Agent attachments
require validated context references, not a rewritten user instruction containing
raw file text. Until that path passes tests, its unavailable state is explicit.

## Attachment risks

- Large/private files: per-file, per-session and per-actor quotas; no automatic
  sharing, outbound transfer or attachment body logging.
- Prompt injection: source content stays untrusted data, never system instructions.
- Original/extracted drift: digest-bound extraction versions and independent
  source readback; no fabricated citations or missing-page summaries.

- Native parsers and OCR add package size and attack surface -> pinned builds,
  license notices, restricted workers, real packaged tests and vulnerability review.
- OCR and table reading are lossy -> previews, source coordinates, quality gates
  and explicit uncertainty; do not synthesize missing text with a language model.
- More persistent originals increase storage/privacy exposure -> scoped access,
  retention, quota and deletion tests; do not log source bytes.
- New organization policy can hide old documents -> versioned migration with
  preflight inventory and rollback; keep established workspace semantics explicit.

## Migration Plan

### Human review of generated candidates

### Cross-section composition

Compose long summaries through a deterministic balanced merge tree rather than
concatenating every section into one prompt. Pin the parent analysis, each
candidate digest, its review revision/decision, the original extraction plan and
the model/runtime identity. Excluded candidates remain accounted for in the plan
but are not model inputs. A merge returns bounded prose with references to its
immediate input sections; the service resolves those references through the
validated tree to original quotations. It never accepts model-written source
identifiers or inherits human approval for newly written prose.

Each merge is an independently validated checkpoint, with an exact token count
under the same local generation lease. Resume from the first uncommitted merge;
do not replay completed model work. Retain omitted/invalid input gaps through all
levels. Recheck numeric values against original quotations at each level, not
only against intermediate model text. Retain conditions and exceptions in the
provenance even when composing concise prose. Requirements tables retain their
individual rows; summary reduction must not replace an exhaustive requirement
register. Composition remains a draft pending support review and qualification.

Expose compact status and gap counts separately from paginated original-source
evidence. Reports show candidate-stage gaps and unrepresented source batches as
well as extraction and merge omissions. Keep original draft review distinct from
newly composed prose. Changing a review stops future merges against the old
snapshot; historical reports remain available with an explicit stale-review
state. Summary generation constrains empty condition/exception arrays to match
the existing validator. Short batches offer bounded, exact native quotation
choices so translated prose cannot silently translate its source quotations;
larger batches still undergo the same exact-quote validation. This is a generation
constraint, not a replacement for support or language qualification.

### Human review of generated candidates

### Human review of generated candidates

Keep reviewer decisions separate from immutable extraction and model receipts.
Each section review pins the analysis, batch identity and response digest, records
one keep/exclude/unreviewed decision per candidate, optional bounded notes, and an
explicit section-check acknowledgement (including sections with no candidates).
Append revisions with authenticated authors and timestamps; compare the expected
section revision on save so simultaneous edits cannot overwrite each other.
Every read, save and export requires current access to all pinned sources and
the private analysis owner. Review history follows source retention and holds.
Exports preserve original candidates, gaps and review decisions; a checked
section never establishes complete extraction, independent verification or
linguistic approval. Clients recover uncertain saves by reading the saved review,
never by silently replaying a write.

Ship schema additions with legacy readers; leave existing index identities and
source text intact. Require explicit reimport for unavailable original bytes.
Record parser/index versions and rebuild only through approved durable operations.
Qualify source, packaged binaries and hardware separately. v0.4.24 publication is
held for the PDF blocker; implementing this plan does not itself authorize a new
data migration or certify every platform/model.
## Document reader and PDF classification correction

Boundedly recurse PDF form objects rather than classifying every form as a scan.
Native text and vector content do not require OCR by themselves. Raster images
remain explicitly unexamined unless qualified OCR succeeds. Preserve all partial
source units, and distinguish text-only use from complete extraction. Existing
failed records require explicit retry; existing citations retain their revisions.

Serve previews through authenticated source access: original digest, ordered
source units, extraction warning and page count. Render PDF pages with the pinned
isolated worker, bounded queue/time/output and cancellation; no document scripts,
external resources or arbitrary paths. Recheck access on every request, use
no-store responses, and discard client caches on scope changes. Preview is a read,
not permission to publish, index or submit. Office previews show extracted
sections/sheets/slides and are labeled as structured text, not original layout.

### Readable-text admission correction

For conversation attachments, first inspect native PDF units. A bounded worker
classification distinguishes substantial native text with bounded visual regions
from sparse/header-only or predominantly scanned pages. Only when every nonempty
unit is ready or native-text-with-visuals can the attachment be admitted without
OCR. Persist `native_text_only` coverage, retain exact native text, show visual
coverage notices and page navigation, and propagate the restriction into model
context. The overall extraction remains partial. Unknown geometry and large
scanned surfaces stay on the OCR path. Explicit text-only recovery remains for
other partial documents. This replaces mandatory extra approval for the qualified
native-text case only; it does not claim figures have been read.

## Mixed OCR selection increment (2026-11-14)

Continue task 8.3 with up to three explicitly selected, ordered OCR packs per
document. The existing `ocr_language` field retains a canonical `eng+fra` selection
so persistence, retries and attachment publication remain identity-bound. Single
pack requests retain their wire compatibility. Mixed requests require a new
service capability. A bounded memory-only bundle carries digest-verified packs
to the isolated worker; every requested pack must load, with no inferred fallback
or dependency download. Structured runtime identities list each pinned pack.
This increment does not claim page hints, deskew or language accuracy qualification.
## Private correction revisions (implemented, 2026-11-14)

Continue task 8.1 with owner-authorized corrections to ready private attachment
graphs. Keep original bytes unchanged. Publish immutable revision artifacts before
an atomic current-record update; bind the ordered history into the source revision.
Readers may access only revisions reachable from the current record, never orphaned
pre-publication files. Saved citations and conversation history retain their exact
revision, while new selections can explicitly use the corrected revision. Apply
separate bounded derived-storage quotas and include revision artifacts in deletion.
The shared reader must distinguish current editable and historical read-only views.

## Native PDF form evidence (implementation, 2026-11-14)

Extend the pinned PDFium worker with bounded read-only AcroForm getters. Retain
field identity, exact stored value, checkbox/radio state, visibility and original
PDF coordinates separately from page appearance text. Emit typed form blocks
with byte spans in the shared extraction graph. Native fields survive OCR and
are never overwritten by visual recognition. Field calculations, ambiguous
labels, hidden values and signature authenticity remain explicit review limits.
No form actions or scripts execute. Revisions change with the worker contract;
existing retained evidence is not reparsed implicitly.

## Tracked Word alternatives (implementation, 2026-11-14)

Preserve current visible text as the retrieval transcription. Retain bounded
tracked insertion/deletion/move text with author, date and source identity as
separate block metadata. Excluded text never joins current-text chunks or model
context automatically. The source reader exposes these alternatives for review;
coverage remains limited while tracked changes exist. Property changes retain
metadata without inventing a rendered historical appearance. Corrected revisions
keep prior extraction history rather than overwriting native alternatives.
