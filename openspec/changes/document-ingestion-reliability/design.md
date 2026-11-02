## Context

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

## Risks / Trade-offs

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

Ship schema additions with legacy readers; leave existing index identities and
source text intact. Require explicit reimport for unavailable original bytes.
Record parser/index versions and rebuild only through approved durable operations.
Qualify source, packaged binaries and hardware separately. v0.4.24 publication is
held for the PDF blocker; implementing this plan does not itself authorize a new
data migration or certify every platform/model.
