# Document ingestion repair — 2026-11-01

Source implementation and fixture evidence only. The installed desktop has not
been replaced by this work. Enterprise ingestion qualification remains open in
`document-ingestion-reliability`.

## Reproduction and changes

The baseline `go test ./internal/rag -run '^TestParser' -count=1` failed on
missing-PDF-tool fallback panics, Unicode loss, legacy XLS classification and
inline XLSX text. The fallback compiled a negative-lookahead expression that Go's
regular-expression engine does not support. The installed Windows environment
had no `pdftotext` on PATH. These are reproducible harness defects, not embedding
quality problems. The baseline is retained in `build/document-parser-baseline.log`.

The repair removes that panic path, bounds optional extractor time/output and
compressed data, retains Unicode, rejects legacy binary XLS explicitly, bounds
actual multipart uploads and removes spilled temporary files. Typed failures
occur before publishing an indexed document. A child-process regression exposed
an additional output-limit bypass through an embedded `bytes.Buffer.ReadFrom`;
the bounded output writer now uses composition and explicitly owns writes.

Office extraction follows workbook/presentation relationships rather than archive
entry order. Generated XLSX and PPTX fixtures include deliberately reordered ZIP
entries, reordered sheets/slides, multilingual text, sparse cells and formulas.
Fixture SHA-256 identities and provenance are under
`internal/rag/testdata/documents/manifest.json`; regeneration is in
`scripts/test/generate-document-fixtures.py` with pinned developer dependencies.
No personal documents or runtime Python requirement were introduced.

## Automated evidence

- Parser, upload and Office regressions passed.
- `go test ./internal/rag ./internal/server ./internal/models ./cmd/offgrid` passed.
- `go test ./...` passed on Windows after the document/vision changes.
- Evidence logs: `build/document-parser-repair.log`,
  `build/document-upload-check.log`, `build/document-office-check.log`,
  `build/document-vision-go-all.log`.

The initial WSL fixture-tool installation timed out; fixture generation succeeded
in an isolated Windows Python environment instead. The original failure log is
retained. Windows race invocation was unavailable because CGO was disabled;
Linux race results are recorded with the companion vision evidence.

## Limits and unfinished work

Basic PDF fallback remains explicitly unqualified for complex layout and font
mapping. No packaged PDF engine, OCR/language-pack runtime, durable original-source
import pipeline, page/cell citation contract or organization retention/ACL expansion
is claimed here. The planned 30-document qualification corpus is unfinished.
XLSX formulas are not executed or refreshed; cached values may be stale. Charts,
images, macros and rendered diagram content are not extracted. Native packaged
Windows/macOS/Linux document qualification remains unverified.

## Conversation attachment extension

The user's subsequent UI/CLI file-upload request is tracked in tasks 1A.1–1A.5.
The initial storage implementation separates original bytes and extracted text
from ordinary session listings. It checks session ownership on every operation,
binds namespaces to the session's creation identity, and records source/extraction
digests. Files remain uploaded/failed until extraction is explicitly completed;
ready extraction cannot be overwritten in place. Initial limits are eight files,
50 MiB per original and 100 MiB of originals per conversation.

`go test ./internal/sessions` passes with tests for source fidelity after restart,
cross-user denial, filename/count limits, failure readiness, immutable extraction,
changed originals and deleted/recreated session isolation. This is foundational
code at that checkpoint; the following continuation records subsequent work.
Installed user conversations and files were not migrated.

### November 2 continuation: shared conversation files

The service now exposes scoped upload/list/prepare/original/delete endpoints,
advertises `session-files-v1`, and binds source revisions into durable turns and
saved user messages. Admission validates history plus current sources against
the 256 KiB direct-text budget before consuming a request identity. Extraction
and original digests are independently checked. Concurrent source removal cannot
win after turn admission. Account storage is additionally bounded at 500 MiB.
Deleting a conversation removes its retained originals and extraction records;
failure keeps the conversation record so cleanup can be retried explicitly.

The shared web/desktop composer includes multi-file upload, draft chips,
extraction failure/retry, source download and failed-send retention. Sources are
private to the conversation. The authenticated CLI has `session attach`,
`attachments`, `remove-attachment`, ordinary `session send --file/--attachment`,
and ordinary `session chat` with draft attachment commands. Truncated streams
are failures; only a persisted terminal response is success. JSON output remains
structured. CLI upload validates the returned source digest and never follows
redirects or repeats an uncertain mutation.

Validation run in this continuation:

- `go test ./...`: passed (`build/conversation-files-go-all.log`), before the
  final owner-quota and history-admission additions.
- Focused final storage/admission/authentication tests passed in
  `build/conversation-files-admission-final.log`; CLI protocol/source-fidelity
  checks passed in `build/conversation-files-cli-contract.log`.
- Web TypeScript/build and generated API check passed; the existing bundle-size
  advisory remains. The subsequent schema description change was regenerated.
- File and existing image Playwright suite: **31 passed**, including narrow
  layouts and keyboard removal in nine locales
  (`build/conversation-files-ui-final.log`). The initial send test intercepted
  the fixture's endless synthetic stream; its failure log is preserved and the
  fixture was corrected before rerunning.
- Repository hygiene and source-size checks passed. No source-size ceiling was
  raised; composer creation/copy logic was extracted.
- WSL/Linux `CGO_ENABLED=1 go test -race ./internal/sessions ./internal/serviceclient
  ./internal/cli ./internal/server` passed in `build/conversation-files-race.log`.
  This run preceded the final owner-quota and history-admission additions; their
  focused follow-up checks are recorded separately rather than implied by it.

Still incomplete: legacy `offgrid run` plain/TUI integration, explicit optional
Knowledge publication from attachments, final client recovery/accessibility
coverage, real-service UI/CLI same-file inference qualification, large-document
retrieval, packaged parsers/OCR and citation/source-unit work. This is not a
release-readiness or enterprise-readiness claim. No installed app, model,
container or user conversation was changed.

### November 2 follow-up: first-use race, recovery and terminal file runs

A new first-conversation renderer test reproduced an actual upload cancellation:
the conversation selection changed before the upload registered its destination.
The destination is now recorded before React selects the new conversation.
The initial failing logs remain in `build/conversation-files-ui-lifecycle.log`;
the corrected lifecycle suite passed all 13 cases. Later recovery additions and
existing image regressions pass **35 tests** in
`build/conversation-files-layout-final.log`.

Chat now offers explicit saved-source inspection/reselection, preparation retry
and a warning when draft metadata cannot persist. Reselection does not upload,
send or publish to Knowledge. Screenshot inspection found the new recovery panel
displaced the editor below the controls. Moving it inside the input group fixed
the order; a geometry assertion now checks the editor precedes the footer.
The inspected synthetic screenshot is
`build/conversation-files-recovery-ui.png`; it contains no user data.

`offgrid run MODEL --file PATH` now starts a service-owned file conversation in
the plain terminal or TUI. `--prompt QUESTION` sends once, and JSON mode requires
that explicit one-shot form. `--save` names a new service conversation; `--load`
selects an existing service conversation without overwriting its history.
Recovery identity is emitted on stderr before uploading. CLI contract tests
exercise all three presentations, exact Unicode source bytes, authentication,
selected model, persisted references and uncontaminated JSON output.

Saved conversation chat supports image commands as well as file commands, source
reselection and retry of extraction without repeated upload. Unavailable saved
sources block resuming instead of being omitted. Cancellation now reaches the
owned durable turn even when cancellation happens before response headers; both
before/after-header cases pass. An initial cancellation test fixture did not
consume its request body and hung on HTTP shutdown; that owned test process was
stopped, the fixture was corrected and bounded, and the corrected test passed.

Additional actual verification:

- `go test ./...`: passed in `build/conversation-files-go-followup.log`, including
  the quota/history admission and cancelled-upload additions.
- WSL race: sessions/server passed in `build/conversation-files-final-race.log`;
  CLI/serviceclient/command entry point passed in
  `build/conversation-files-client-race.log`.
- Web generated API check and production build passed in
  `build/conversation-files-api-final.log` and
  `build/conversation-files-web-final.log`. The bundle-size advisory remains.
- The scoped storage and durable source-context tasks 1A.1/1A.2 are checked;
  renderer/CLI/full qualification tasks remain open for their remaining scope.

Still required: history-preserving attachment handoff midway through an older
local-only `offgrid run` conversation, optional Knowledge publication, real-service
UI/CLI fixture qualification and the packaged parser/OCR/source-unit program.
These tests do not qualify model answer quality, native OCR, enterprise policy,
or release readiness. No installed user workspace or app was replaced.

### November 2: real PDF corpus and isolated engine implementation

Task 2.1 now has **37 frozen authored documents**, with original byte hashes,
ordered text expectations, source positions, error classes and provenance in
`internal/rag/testdata/documents/corpus/manifest.json`. The corpus includes
compressed and font-mapped PDFs, a PDF 1.5 object/xref stream, multilingual text,
scan/mixed/rotated pages, encrypted and corrupt files, Office packages and a
65 MiB expanded ZIP. Repeating the generator produced an identical manifest.
The embedded Vera subset has its redistribution notice beside the fixtures.

Before selecting the candidate engine, the PATH-independent baseline matched
**25/37 text/error expectations**. The immutable assessment is
[document-corpus-baseline-2026-11-02.json](document-corpus-baseline-2026-11-02.json).
This is not 25 successfully readable documents: some matches are expected
rejections. Source units were not qualified. The baseline exposed ordinary
compressed-PDF failures, font-map failures, generic encryption/empty errors and
invalid UTF-8 being accepted as text. The UTF-8 acceptance defect is now fixed
for plain text/Markdown, with a retained-file regression.

The isolated PDFium/WASM evaluation extracted the expected text and page counts
from the text-bearing PDF fixtures, and identified encryption/corruption. Its
first custom runtime configuration omitted the engine's required exception
handling feature; the initialization failure was corrected using the upstream
runtime contract. Dependency inspection initially hit Go proxy timeouts; the
actual worker build later succeeded with pinned production dependencies. The
main module's toolchain/dependencies were not changed.

Implemented a separate `runtimes/pdfium` worker and app-local service adapter:

- One PDF on stdin and one versioned digest-bound JSON receipt; no path/URL/
  command/script or credentials in the document protocol.
- Empty WASI filesystem, no network imports, 512 MiB WASM memory cap, bounded
  input/pages/text/objects and independent process cancellation/deadlines.
- One extraction at a time, with cancellable queue admission and no inference
  lock; inherited credentials, proxy variables and PATH are not forwarded.
- Strict receipt, revision, digest and ordered-page validation; no weaker
  fallback after a worker fails. Windows workers have hidden console windows.
- Actual page source units and retained native text for mixed documents;
  OCR-required pages prevent a complete-only import from reporting success.
- Attachment extraction revision now includes the pinned worker revision.

Actual Windows compiled-worker tests passed **all 18 PDF cases**, plus running
worker cancellation, in `build/pdf-worker-real-corpus.log` (140.9 seconds total).
Cancellation returned in about 90 ms. Receipt corruption, wrong source/revision,
unknown fields/states, oversized output, partial-page preservation, unavailable
worker and queued cancellation tests passed. The final empty-ready guard and
attachment checks passed in `build/document-parser-final-guards.log`.
Focused parser/session/server tests passed in
`build/document-corpus-regressions.log`.

The CI workflow now builds and tests the real worker on Windows, Linux and macOS;
that matrix has not run here. Source-size, repository hygiene, OpenSpec validation
and maintained documentation checks passed. Packaged runtime task 2.2 is still
open: complete readiness integration and crash/resource qualification remain.

**Remaining release gates:** the WASM binary's exact upstream build provenance
and full third-party redistribution notice closure; installed packaging on each
platform/container; OCR/language assets; citation/import integration; and import
performance. Current isolated workers recompile the engine for each PDF, costing
about 6–10 seconds on this machine. No persistent code cache or resident worker
has yet been qualified. Scans correctly require OCR; that is not OCR success.
Image/form objects conservatively require OCR even when a page has native text.
This worker was built only under `build/`; it was not installed or released.

#### Follow-up: warm compilation cache and fault boundaries

The initial 6–10 second per-file compilation cost above is preserved as the
baseline. A revision-scoped compiler-code cache now avoids that repeated cost;
each PDF still gets a fresh process and WASM instance. The cache contains the
compiled immutable engine module, not document content. The service supplies
the app-local user cache path, and isolated tests supply their own temporary
directory. No test wrote into the installed user's application cache.

All 18 PDF cases and cancellation passed again in
`build/pdf-worker-cached-corpus.log`: **14.16 seconds** for the real-corpus group,
including **9.07 seconds cold** and **0.27–0.38 seconds** for subsequent engine
imports on these small fixtures. These are same-machine fixture measurements,
not a claim about large/scanned documents or OCR throughput.

Additional owned subprocess fixtures cover a crash, truncated receipt, excessive
output and a nonresponsive worker. Each fails closed, reaps its process and
leaves subsequent receipt admission usable. Private worker stderr is not
returned. These fault/receipt checks passed in `build/pdf-worker-faults.log`.
`go -C runtimes/pdfium mod verify` verified the pinned downloaded modules.
`go test ./...` passed in `build/document-parser-go-all.log`; subsequent cache and
process-helper changes receive their focused follow-up checks separately.

The packaging/provenance, OCR, remaining readiness contract, complete source
citations, large-document/resource and installed cross-platform gates above
remain open. The newly added CI matrix has not been run or published here.

Final follow-up checks passed: complete `internal/rag`, `internal/server` and
`internal/sessions` suites in `build/document-parser-final-regressions.log`, and
the real worker/corpus/fault checks in `build/pdf-worker-final-corpus.log`. The
last run also confirms the exercised compiler cache contains none of the
fixture's unique source-text markers. This complements the code-path boundary;
it is not a general forensic proof that arbitrary user data can never leak.
Filesystem Knowledge ingestion now passes its caller context into the parser,
with a regression proving a cancelled import stops before embedding/storage.
Source-size and whitespace checks remain clean. No installed application,
model, service, container or user document was changed during this work.

#### November 2: readiness, source fidelity and actual CLI/service boundary

Task 2.2 is now implemented and automatically tested. Task 2.3 (distribution and
clean-install qualification) remains open. This does not certify enterprise
ingestion, OCR, citations or model-answer accuracy.

Changes:

- Additive `document_formats` on `GET /v1/rag/status`, independent of embedding
  configuration. Built-in availability, installed-but-unchecked PDF runtime,
  unavailable adapters, limits, OCR languages and qualification are distinct.
  Status reads never run an extractor, inspect PATH or download dependencies.
- PDF parsing now requires the app-local worker. Missing workers fail with a
  typed error; the previous PATH/regex fallback was removed. Earlier baseline
  results above remain historical evidence, not current supported behavior.
- Typed complete/partial extraction state and warnings, digest/revision-bound
  source units, ordered named spreadsheet sheets and presentation slides.
  Text/DOCX use sections, not fabricated rendered page numbers. These units are
  not yet persisted through the full retrieval/citation pipeline.
- DOCX now reads body XML in document order instead of moving all tables after
  paragraphs. Invalid XML, multiple roots, unsupported declarations and excessive
  nesting are rejected. Hyperlink text and in-run tabs/breaks remain ordered;
  deleted text, field instructions and drawing content do not become body text.
- JSON formatting preserves large identifiers, decimal spelling, member order
  and duplicate members without conversion through floating point. XML parses
  entities/CDATA structurally and rejects malformed markup. Invalid UTF-8 in
  code/CSV/HTML/JSON/XML is rejected rather than silently replaced.
- Known unsupported legacy Office, ODF, email and image-document formats cannot
  fall through to plain-text parsing. File/reader ingestion bounds reads before
  indexing; cancelled readers are not touched. The file methods were moved into
  `engine_file.go` to preserve the existing source-size limit.

The first real CLI/service fixture run exposed a missing client error mapping:
encrypted PDF rejection became generic HTTP 422. The original failed run remains
in `build/document-packaged-contract.log`. Safe closed-set recovery messages now
cover encryption, empty content, bounds, timeout, missing worker and protocol
failure without exposing server diagnostics or making unsupported retention claims.

Validation actually run on Windows:

- `go test ./internal/rag ./internal/server ./internal/cli ./internal/sessions ./internal/serviceclient`:
  passed (`build/document-final-regressions.log`; server suite 132.6 seconds).
- Final parser/client follow-up passed (`build/document-parser-latest.log`), and
  `OFFGRID_TEST_DOCUMENT_WORKER=<isolated worker> go test ./internal/rag -run TestParser -count=1`
  passed with the final typed state/warning checks in 19.3 seconds
  (`build/document-parser-qualified-contract.log`). This includes all 18 actual
  PDF cases, worker faults, cancellation, receipt and cache checks.
- An isolated compiled server test executable beside the real worker ran
  `TestPackagedDocumentCLIAndSessionContract`: **10/10 passed**, 13.74 seconds
  (`build/document-packaged-contract-final.log`). It uses real authentication,
  CLI upload/send functions, session HTTP handlers and extraction. It checks
  original-byte recovery, immutable attachment references, failed-source
  retention and cross-user denial. Inference is a deterministic test stub;
  these results are not model-answer quality or an installed desktop test.
- API regeneration/check, TypeScript, source-size, repository hygiene and
  whitespace checks passed. The source-size check initially rejected growth in
  `engine.go`; the focused file-method move resolved it without raising limits.
- A focused Windows race attempt could not start because CGO is disabled and no
  GCC/Clang command is available in this shell (`build/document-readiness-race.log`).
  This is **unrun**, not a pass. The existing CI race job includes these packages.

CI now also exercises the app-local worker discovery and authenticated CLI flow
on its Windows/Linux/macOS matrix. That remote matrix and clean installers have
not run here. The worker provenance audit is recorded in `runtimes/pdfium/README.md`;
its unresolved custom-build/notice closure still gates distribution. No installed
service, desktop, model, user documents or production cache was changed.

### November 2: attachments during an existing CLI conversation

Completed ordinary CLI parity without changing native-agent authority:

- Both terminal presentations can start with `--file` or attach later with
  `/attach PATH`. Mid-conversation attachment atomically copies completed
  user/assistant text and validated image bytes into an authenticated service
  session. The original local history remains intact, and a resume identifier
  is shown. Listing an empty draft does not create remote state.
- The additive `session-history-seed-v1` capability prevents old services from
  silently ignoring history. Seed creation accepts no task, source-reference,
  system-role or tool-result authority; it is bounded and never runs inference.
  Existing session names cannot be overwritten. A lost create response is
  recovered by inspecting the same identity and verifying its history.
- Unconfirmed uploads block sending until resolved or explicitly removed;
  failed extraction retains its source for preparation retry. TUI errors restore
  the text input without overwriting newer typing. Plain chat offers `/retry`.
  An unchanged request retains its durable request ID after an incomplete reply.
  Files and images clear only after confirmed completion.

Verification:

- `go test ./internal/sessions ./internal/cli ./internal/server ./cmd/offgrid
  ./internal/serviceclient ./internal/rag` passed
  (`build/document-cli-full-regression.log`, server 132.10 s).
- After the final draft/retry changes, `go test ./internal/cli ./cmd/offgrid`
  passed again (`build/document-cli-final.log`, 1.81 s / 6.60 s).
- Targeted history, authentication, storage-failure, lost-response, image,
  quoted-path, buffered-input, TUI cancellation and retry-identity regressions
  passed (`build/document-cli-transition-tests-final.log` and
  `build/document-cli-recovery-tests.log`). The initial transition test supplied
  an incomplete upload receipt; strict validation rejected it. That exposed the
  empty-draft send path, now blocked with a regression. Initial failure logs
  remain in `build/document-cli-transition-tests.log`.
- API regeneration/check, TypeScript, source-size, repository hygiene,
  maintained-document links and `git diff --check` passed. A mistaken attempt
  to run a nonexistent Python maintenance script was replaced with the existing
  Node maintenance checks; it is not counted as validation.

These checks use isolated services, authored files and deterministic responses.
They do not establish model-answer quality, packaged PDF distribution, OCR,
native desktop qualification or release readiness. No installation, deployment,
commit, publication or user-data migration was performed.

## November 2: structured sources and simpler Knowledge imports

New parsed uploads preserve page/slide/sheet/section ranges, original digest and
extractor revision through indexing, SQLite reload, explicit reindexing, search
and saved Chat source passages. Chunks stay within units and preserve exact
Unicode substrings; offsets refer to UTF-8 bytes in retained extracted text, not
coordinates in the original PDF. Existing indexes retain legacy behavior. Context
truncation now updates the excerpt locator and hash rather than adding fabricated
ellipsis bytes. Chat shows human-readable source-unit labels in all nine locales.

The shared web/desktop Knowledge page now supports up to 20 files per selection,
sequential bounded transfers, per-file acknowledgments and optional format help.
Unavailable readers and oversize files are rejected before upload. Typed parser
rejections do not discard independent files; unknown responses stop remaining
dispatch and never automatically replay the import. Navigation stops later
dispatch. A failed readiness refresh does not erase the acknowledgment panel.
The setup page shows the proposed search model and download size, with model
selection behind **Change search model**; no automatic activation or download.

This UI batch is not the planned durable import job store. Originals, restartable
imports, partial OCR, cell-level citation objects, organization policy/retention,
full real-service UI acceptance and release qualification remain unfinished.
Tasks 3.3 and 5.1 therefore remain unchecked. Localized fixture checks do not
replace the unavailable human localization/usability reviews.

Validation performed:

- `go test ./internal/rag ./internal/server ./internal/sessions` passed in
  `build/document-source-final.log` (7.24 / 102.46 / 6.95 seconds).
- Frozen source-range fixtures passed parsing through saved/reloaded/reindexed
  citation read-back, including three actual PDF worker cases, Office and Japanese
  text, repeated page contents, invalid ranges, partial extraction rejection and
  Unicode-safe truncation (`build/document-source-real-pdf.log`). That initial
  invocation did not redirect the immutable WASM compilation cache; the test now
  uses temporary home/cache directories. No original document data is in that cache.
- Generated API/TypeScript and targeted saved-source contract tests passed
  (`build/document-locator-contract.log`). The initial source-size check required
  moving existing SQLite schema initialization into `sqlite_schema.go`; the size
  baseline was not raised.
- Source/import renderer tests: 26 passed (`build/knowledge-import-ui.log`).
  Existing Knowledge, setup, model-choice and workspace journeys: 32 passed
  (`build/knowledge-import-regressions.log`). After additional failure/limit cases,
  all 16 import tests passed (`build/knowledge-import-final.log`). These are
  isolated API fixtures, not real embedding or native-desktop qualification.
- Inspected `knowledge-import-narrow.png` at 390 px: filenames wrap, individual
  failures remain visible, controls fit, and format details stay optional. The
  earlier supporting-passages screenshot shows the new page label with inert text.
- Source-size and repository-hygiene checks passed. The first hygiene attempt
  found experimental Go probes under ignored build output; they now carry the
  `ignore` build constraint and no longer enter application package discovery.
- The final web production build passed (481 modules), as did generated API,
  documentation-link and diff-whitespace checks. Vite still warns about the
  1.12 MB minified main JavaScript chunk; this pass does not establish a cold-start
  performance improvement or erase the bundle-size concern.

PDF packaging remains a release blocker. The traceable community PDFium archive
at [chromium/8057](https://github.com/bblanchon/pdfium-binaries/releases/tag/chromium%2F8057)
is not a drop-in replacement for the current custom wrapper module. The first
probe failed on a missing heap import, the next on the legalized `fd_seek`
signature. A direct, isolated binding subsequently opened an authored PDF and
reported three pages (`build/pdfium-provenance/direct-v2.log`). This is only a
compatibility foothold; no engine replacement, production installation or parser
release claim was made from it.

## November 2: pinned engine replacement and package verification

The subsequent implementation replaces the custom `go-pdfium` engine with a
direct binding to the locked community `bblanchon/pdfium-binaries` 8057 archive.
The earlier failed substitutions remain historical evidence above. The new
archive, module, version and complete redistribution notices are checked against
`runtimes/pdfium/engine.lock.json`. This establishes which binary and notices are
packaged; it is not a claim of reproducible upstream compilation or a security
certification. No production installation or user document migration occurred.

The worker preserves protocol 1, using a new pinned extractor revision. Its
closed imports provide bounded heap growth and copying, an empty environment,
and rejected filesystem operations. Unknown imports fail closed. Each document
gets a fresh instance; cancellation remains independent of inference. Scans and
mixed pages still require the unfinished OCR capability.

Packaging now prepares assets explicitly at build time and includes the worker
and notices in desktop, complete CLI and container builds. Desktop staging checks
target architecture and asset hashes; the post-pack hook checks the copied files.
Older CLI bundles remain installable. End users do not acquire Node, Python,
PDF utilities or a build toolchain when importing a document.

Executed checks:

- `go -C runtimes/pdfium test -mod=readonly -count=1` passed in 7.203 seconds
  (`build/document-official-unit-final.log`), including memory boundaries, empty
  environment, embedded notices and immediate cancellation.
- Real Windows PDF corpus, worker failures/cancellation, source locators and
  SQLite/reindex read-back passed in 41.099 seconds
  (`build/document-official-corpus.log`). This includes all 18 PDF fixtures.
- The compiled isolated server test with an adjacent worker passed all ten
  `TestPackagedDocumentCLIAndSessionContract` cases in 12.67 seconds
  (`build/document-official-packaged.log`). It uses authenticated HTTP and CLI
  attachments, saved state and access checks; inference is a fixture, not a
  model-answer quality test. An initial PowerShell invocation split an unquoted
  `-test.v` flag and ran no tests; the corrected quoted invocation passed.
- `go test ./internal/rag ./internal/server ./internal/sessions
  ./internal/serviceclient ./internal/cli` passed
  (`build/document-packaging-go-regressions.log`).
- Linux/amd64 and macOS/arm64 worker cross-builds passed. The Linux executable
  then passed all 18 PDF cases, owned cancellation and cache-content checks under
  WSL in 12.59 seconds (`build/document-linux-corpus.log`). The macOS binary was
  not executed and remains unqualified.
- Four asset/staging Node tests passed with the actual pinned archive
  (`build/document-packaging-node.log`); Windows desktop runtime staging passed.
  Desktop tests reported 101 passed, zero failed, one skipped
  (`build/document-desktop-regressions.log`). This is not a clean installer test.
- `test-executor-install.sh` and `test-installer-entrypoints.sh` passed using
  isolated fixtures in Git Bash, including worker/notice copy, executable mode,
  legacy bundles and retained executor revisions. The first executable-bit test
  used plain text as a fake executable; a shell fixture made that check meaningful
  on Windows. No installed-user files or services were touched.
- Both workflow YAML files parse. Release CI gate, source-size, repository
  hygiene, maintained documentation links and diff-whitespace checks passed.
  No source-size baseline was increased.

The container build verified the pinned download and both worker/service
compilation (`build/document-container-build.log`). A separate scratch target
contains only the reader and notices. Its corpus runner uses an immutable local
image, no network, no host mounts, a read-only filesystem, a non-root user and
bounded memory/processes. CI now includes that same qualification command.
`python3 scripts/test/verify-pdf-container.py --image
offgrid-document-runtime-check:20261102` passed all 18 PDF cases and the embedded
notice digest (`build/document-container-corpus.log`). This demonstrates extraction
without developer tools in that container; it does not qualify an entire installed
desktop, GPU service, OCR pipeline or model-generated answer.

Task 2.3 remains open for complete installer/platform qualification. OCR/language
packs, durable original/import storage and recovery, complete citation coverage,
organization policy/retention, full real-service UI and retrieval-quality gates
also remain open. Existing batch-import UI tests are not substitutes for these
requirements or for human usability review.

## Original retention, binary clients and publication — 2026-11-02

The next storage increment retains original upload bytes in a versioned SQLite
table, with a 50 MiB file limit and a 2 GiB workspace-original quota. New multipart
Knowledge imports bind extraction to the original digest and publish originals,
source text, vectors and index identity in one transaction. Distinct binary
sources with the same extracted text remain distinct. Legacy extracted text stays
readable; no original is fabricated. Reindexing retains originals, and document
deletion cascades to their records. Existing workspace Knowledge permissions
apply; this is not new collection-level or tenant isolation.

Authenticated source downloads are available in the source viewer, with
`offgrid kb original ID --output NEW_FILE`, and through Python
`client.kb.original(ID)`. CLI/Python verify the bounded response digest. The CLI
refuses overwrite and validates before creating its destination. Downloads use
attachment disposition, octet-stream, no-store and nosniff headers. Original
bytes are not included in list/search responses.

Two client defects were found during the review: the Go client did not recognize
flat `error_code` ingestion errors, and Python opened PDFs/Office files as UTF-8
text. Go now accepts both documented error envelopes using safe known messages.
Python sends bounded binary multipart uploads without redirecting or replaying a
mutation. It reports missing paths locally, retains explicit URL import support
through the correct endpoint, and uses the already-configured Knowledge index.
Explicit Python enable supports an installed embedding-model ID; imports do not
silently enable or change the index. Search projections retain source locators.

Verification completed in isolated state:

- `go test ./internal/rag ./internal/cli ./internal/serviceclient -count=1`:
  passed, including legacy schema migration, concurrent quota admission, actual
  SQLite full-disk rollback, digest mismatch, corrupt/future original records,
  exact binary CLI output, existing destination protection and no output after
  invalid/cancelled downloads (`build/document-original-final-go.log`).
- `go test ./internal/server -run
  'TestDocumentOriginalDownloadAndLegacy|TestDocumentUpload' -count=1`: passed
  (`build/document-original-final-http.log`). HTTP tests cover read permission,
  legacy original absence, exact binary delivery and deletion. Their records are
  seeded directly; they do not establish real embedding quality.
- After closing the index-metadata transaction boundary, `go test
  ./internal/rag ./internal/server -count=1` passed (7.895s / 131.613s;
  `build/document-publication-regression.log`). Forced metadata-write failures
  roll back new originals/documents/vectors and preserve the prior source and
  searchable index on replacement. A mismatched embedding-runtime identity is
  rejected transactionally, including when an earlier preflight already passed.
- `node scripts/test/test-web-workspace.mjs build/document-original-ui.exe
  knowledge-original.spec.ts knowledge-import.spec.ts --workers=2`: 28 passed
  (`build/document-original-final-ui.log`). This includes nine locales, narrow
  layout, keyboard focus, download bytes, partial batch rejection and no implicit
  retry. The renderer runs on a disposable service; these feature responses are
  fixtures. The narrow import screenshot was visually inspected.
- `PYTHONPATH=sdk/python python -m pytest -q
  sdk/python/tests/test_knowledge_files.py sdk/python/tests/test_client.py
  runtimes/python/tests`: 92 passed (`build/document-original-python-final.log`).
  Binary transport tests use disposable loopback HTTP servers; PDF/Office byte
  preservation here is transport evidence, not independent parser qualification.
- Generated OpenAPI/TypeScript checks, TypeScript checking, source-size,
  repository hygiene, maintained documentation links, diff whitespace and strict
  OpenSpec validation passed during this increment.

Windows race testing did not run: the default environment has CGO disabled, and
enabling it failed because gcc is unavailable on PATH. The attempted Python test
environment initially lacked pytest and pip; pytest was installed only into the
existing disposable build environment using uv, after which tests passed.

The original is currently committed at successful index publication, not durably
accepted before extraction. Persisted import jobs, recovery/retry stages, OCR,
collection policy, complete retention handling and remaining real-format/model
qualification remain open. Tasks 3.1/3.2/3.4 are therefore not checked complete.
No installed workspace, service, models or desktop files were changed by this
increment, and no release-readiness claim follows from these fixture passes.

## Durable acceptance, recovery and client controls — 2026-11-02

This increment supersedes the preceding storage limitation: multipart Knowledge
uploads now save original bytes before extraction in a versioned, owner-scoped
import record. Stable request keys reject changed content and reuse identical
acceptance. Pending uploads and published originals share the 2 GiB quota;
publication transfers source, document, vectors and index identity in one
transaction. Cancellation/deletion invalidates the persisted attempt before
stopping its owned worker, preventing late publication. Restart marks in-flight
imports interrupted, retains original bytes and requires explicit retry.

Processing remains request-owned, not a background daemon. One import extracts
or indexes at a time; queued imports do not load their original blobs. The shared
index-write gate now observes cancellation while waiting behind other imports or
reindexing. The API does not automatically replay mutations. Existing text/URL
ingestion contracts and workspace-library permissions remain intact.

Web/desktop Knowledge uses the new capability-negotiated accept/process flow and
shows unfinished imports after reload. Users can retry the saved source, cancel,
or remove the pending import. A failed removal remains in its confirmation with
the error. Batch cancellation stops further files; navigation after acceptance
cannot start extraction in an abandoned page. Nine locale fixtures and narrow
English/Arabic screenshots cover presentation; screenshots were visually checked.
This is not human localization or usability qualification.

CLI adds `imports`, `import-status`, `retry-import`, `cancel-import` and
`remove-import`. Python exposes corresponding methods. They use authenticated
service APIs with no independent executor or implicit mutation retry. Import
history removal does not remove a published Knowledge document. The list is
currently limited to the latest 100 records; workspace history is capped at
4,096 records. Collection policy, broader retention and pagination are not
established by this implementation.

Verification:

- Storage/parser/import Go tests passed, including real subprocess exit at
  accepted/queued/extracting/indexing/published boundaries, SQLite full-disk
  rejection, missing original, changed-key conflicts, concurrent acceptance,
  transaction rollback, duplicate publication and stale-worker rejection.
  Logs: `build/document-import-storage.log`, `build/document-import-crashes.log`.
- Authenticated HTTP/CLI tests passed with two independent administrators and a
  reader: another administrator cannot read, retry, cancel or delete an owner's
  pending import by its ID. Real loopback CLI operations use the registered route.
  Log: `build/document-import-http-cli.log`.
- Full Go rag/server/CLI/serviceclient testing initially found the new route
  missing from the frozen registration contract. Reviewed the one-line addition
  with the existing Knowledge-manager guard, updated its count/manifest, and the
  full server suite passed in 139.493s (`build/document-import-server-final.log`).
  The earlier failed log remains `build/document-import-full-go.log`.
- After adding cancellation-aware index-write admission, full
  `go test ./internal/rag ./internal/server ./internal/cli ./internal/serviceclient
  -count=1` passed again: 9.816s / 129.147s / 2.269s / 1.882s
  (`build/document-import-gate-regression.log`). New deadline tests prove that
  imports, ordinary text ingestion and reindexing stop waiting without another
  writer releasing its slot and without publishing a document.
- Python binary/recovery tests: 26 passed, using disposable authenticated HTTP
  servers; `build/document-import-python.log`.
- Browser import/source/scope regression run: 55 passed; the remaining test used
  a negative text assertion on a correctly removed panel. Earlier tests also
  mistakenly looked for “Retry” instead of the existing “Try again” label. After
  fixing these assertions, all 15 recovery tests passed, including real-service
  invalid Office upload, retained digest/attempt after reload, explicit retry and
  deletion (`build/document-recovery-final.log`). The broader run is retained in
  `build/document-import-ui-final.log`. Other feature responses are fixtures;
  this does not prove real embedding or answer quality.
- Final combined browser run of `knowledge-import.spec.ts`,
  `knowledge-recovery.spec.ts`, `knowledge-original.spec.ts` and
  `knowledge-scope.spec.ts`: **56 passed in 25.7s**, wrapper exit 0
  (`build/document-import-browser-verified.log`). The explicit exit check also
  resolved ambiguous PowerShell exit reporting from redirected Node warnings in
  the earlier run. No test was skipped in this combined run.
- Web TypeScript/build, generated API check, source-size ratchet and strict
  OpenSpec validation passed. The reviewed route fixture added only the import
  endpoint. Installed user data, native desktop and serving model were untouched.

Text-PDF corpus evidence remains in the preceding sections. OCR/language packs,
collection access policies, complete retention/retrieval-quality evidence,
packaged platform qualification and the remaining defaults-first experience are
still unfinished. No enterprise-readiness or release-completion claim is made.

## Bounded PDF page rendering for OCR — 2026-11-02

Added an internal page-rendering operation to the pinned PDFium worker. The
receipt binds PNG pixels to the original PDF digest and requested page. Shared
PDF opening retains the same fresh WASM instance, closed host imports and memory
boundary. Rendering rejects excessive dimensions before allocating its bitmap,
preserves intrinsic rotation and converts PDFium BGRA into opaque PNG correctly.
The service independently validates the complete PNG, dimensions, revision and
source identity. Queue cancellation and process cancellation use the existing
owned-worker boundary. A timeout clears dimensions and pixels together rather
than returning a contradictory error receipt.

Verification on Windows, using only authored disposable fixtures:

- `go -C runtimes/pdfium test ./... -count=1`: passed in 23.615s
  (`build/document-raster-tests.log`). Includes 0/90/180/270-degree vector
  render checks, pixel colors, real scan/rotated/mixed pages, invalid pages,
  encrypted input, cancellation and dimension/output bounds.
- Built the worker with `go -C runtimes/pdfium build -mod=readonly -trimpath
  -o ../../build/offgrid-document-worker.exe .`.
- With `OFFGRID_TEST_DOCUMENT_WORKER` set to that absolute build path,
  `go test ./internal/rag -run 'TestParser(Raster|Worker)' -count=1` passed
  in 29.427s (`build/document-raster-parent-verified.log`). This runs the
  existing real PDF corpus after the opening refactor, real raster subprocesses
  with an empty PATH, receipt tampering/CRC/truncation/size failures and running
  and queued cancellation. No installed-user state is used.
- Final full `go test ./internal/rag -count=1`: passed in 10.263s
  (`build/document-raster-rag-full.log`); final worker module suite passed in
  24.963s (`build/document-raster-module-final.log`).
- The compiled server contract beside the real worker passed all ten document
  cases in 11.84s (`build/document-raster-packaged-verified.log`): actual PDF and
  Office extraction, authenticated CLI upload, exact original-byte retrieval,
  source-bound session context/history and cross-owner denial. The answer
  streamer is a fixture, so this does not establish model answer quality.
  The initial direct PowerShell invocation split unquoted dotted test flags and
  exited before running tests; that failure is retained in
  `build/document-raster-packaged-contract.log`. Quoted full flag arguments
  resolved the invocation error without changing test behavior.
- Desktop pack verification and pinned archive/notices tests: 4 passed, none
  skipped (`build/document-raster-packaging.log`). Source-size ratchet, whitespace
  and strict OpenSpec validation passed. No installer/native UI check was run
  for this backend-only increment.

Task 2.4 remains unchecked: rasterization is not text recognition. OCR runtime
selection, language-pack import/readiness, recognition confidence and mixed-page
deduplication are not implemented by this increment. Scanned PDFs still return
the explicit OCR-needed state. No runtime was installed or release published.

## OCR receipts and actual attachment controls — 2026-11-02

The original PDF failure remains the priority. Text extraction and OCR are being
verified separately; a successful attachment upload does not establish retrieval
or answer quality.

Added a bounded hOCR decoder retaining word order, Unicode, page coordinates,
line identity and confidence. It rejects malformed/truncated receipts, invalid
geometry or confidence, multiple pages/roots, unbound trailing text, unknown XML
entities, excessive nesting and input/text limits. It does not render hOCR as
HTML, fetch referenced resources, or label uncertain OCR as ready. Full
`go test ./internal/rag -count=1` passed in **10.366s**
(`build/document-ocr-rag-final.log`).

The initial isolated Tesseract 5.3.0 WASM experiment read two authored phrases
exactly: “Scanned receipt total 42” (3.046s) and “Scanned page total 34” (3.219s).
Its rotated case produced low-confidence fragments. Inspection showed the
original fixture clips its text; it and its failed result remain unchanged.
New, fully visible PDF fixtures cover all four rotations and mixed native/scanned
content on one page. The actual PDFium rendering test for their bounds passed
in **9.303s** (`build/document-ocr-visible-rotations.log`). This is rendering
evidence, not an OCR rotation pass. Captured receipts, digests and engine/language
revisions are in `internal/rag/testdata/documents/ocr`.

The old prebuilt OCR module failed with the current wazero ABI
(`emscripten_stack_get_current not exported`). A separate old-runner experiment
established the recognition baseline only. Neither that runner nor the old
engine was added to the application. Tesseract 5.5.4 contains subsequent security
fixes, so the new memory-only bridge targets pinned 5.5.4/Leptonica 1.87.0 sources.
An initial native build recognized the two scan phrases, but emitted missing
diagnostic-image-codec warnings. It is not a shipping worker. Two attempts to
obtain the Emscripten build image failed with registry TLS handshake timeouts;
no installation or application dependency was silently downgraded. The worker,
language import, isolation and complete OCR quality gates remain unfinished.

Attachment UI verification:

- `node scripts/test/test-web-workspace.mjs build/document-import-ui.exe
  chat-files.spec.ts`: **17 passed in 37.1s**, no skips
  (`build/document-files-real-verified.log`). The real-service test uses the
  actual PDF and DOCX fixture bytes and app-local PDF worker, checks preparation,
  exact original download, reload and draft removal without source deletion.
  It does not run model inference; unrelated UI catalog responses are fixtures.
- File drop preserves the question and does not generate a reply. All nine
  locales pass narrow-screen and keyboard-removal checks. Reviewed the English
  and Arabic 320px screenshots; chips, remove controls and draft text fit,
  including RTL. This is not human localization or screen-reader qualification.
- The first real-service browser attempt failed its default five-second UI
  assertion during cold PDF preparation. The corrected test waits for the actual
  bounded preparation response before asserting readiness; it adds no fixed
  sleep or application bypass. The original failure remains in
  `build/document-files-real-ui.log`. Duplicate locale cases introduced during
  test authoring were removed, preserving nine distinct locale checks.

No current workspace data, installed desktop, serving model, container or
release was changed. OCR and organization-readiness tasks remain open.

The service upload/import/original/conversation-file regression selection passed
in **3.255s** (`build/document-ingestion-service-final.log`); web TypeScript,
source-size, maintained documentation and strict OpenSpec checks also passed.
The PDF fixture directories now explicitly override the blanket PDF ignore rule,
and digest-pinned fixtures/receipts disable Git text conversion. Without this,
new PDFs could be omitted from a commit and Windows checkout could change the
evidence bytes. No personal document directory is exempted. The additional
visible-fixture digest test passed with the OCR tests in **0.752s**
(`build/document-ocr-pins-final.log`).

The final native bridge build passed its invalid-language, dimension and pixel
buffer-length assertions, then produced seven actual receipts
(`build/document-ocr-native-boundaries.log`). Independent text comparison scored
**5/7 exact matches**: both original scan samples, visible 0/90-degree pages and
the mixed native/scanned page. Visible 180/270-degree pages returned incorrect,
low-confidence text. Their output is retained under
`internal/rag/testdata/documents/ocr/native`; receipt digest/word/confidence tests
passed in **0.770s** (`build/document-ocr-native-receipts.log`). This is a failed
orientation qualification, not a seven-case OCR pass. Repository hygiene passed.

The first follow-up ABI compilation found that WSL had removed its `/tmp` build
tree after the prior invocation ended. The isolated build was recreated and now
retains development libraries/test output under ignored `build/ocr-source` for
repeatability. No system libraries were installed. Diagnostic-font codec
warnings persist; their upstream build switch and unverified status are recorded
in `runtimes/ocr/README.md`. No production OCR path is enabled.

## Maintained OCR worker and typed PDF integration — 2026-11-02

The pinned Emscripten 4.0.10 archive was obtained explicitly and verified after
the earlier container registry failure. The source build now uses a persistent
isolated Linux cache, not WSL `/tmp`. No system compiler, package, model or
installed application was replaced. `scripts/build/build-ocr-runtime.py` builds
Tesseract 5.5.4/Leptonica 1.87.0 with image codecs, network dependencies, training,
legacy recognition and debug fonts disabled. Source-path remapping excludes
private build paths. The engine and bundled notices are pinned in
`runtimes/ocr/engine.lock.json`; output digest mismatches block staging.

The source build command was:

```sh
python3 scripts/build/build-ocr-runtime.py --archives /mnt/d/offgrid-llm/build/ocr-source --toolchain /mnt/d/offgrid-llm/build/ocr-toolchain/wasm-binaries.tar.xz --work /home/phil/.cache/offgrid-ocr-wasm-5.5.4 --output /mnt/d/offgrid-llm/build/ocr-repro/ocr.wasm --jobs 4
```

The final build succeeded (`build/document-ocr-wasm-repro.log`): WASM SHA-256
`acfa0590d55b0762fc9d82fbafa6b13fa8700613356bfdf94cf933497dc298cf`,
notices SHA-256
`eb7f09dcdebde4a4dd6a35327ce45e09ea8e1ab4e5abcd4549dee8b2d6d8ef0a`.
A second clean build location has not been tested for reproducibility.

The separate Go worker runs the actual engine with no mounted filesystem and a
512 MiB WASM ceiling. The first execution attempts rejected unreviewed imports
(`emscripten_notify_memory_growth`, then `__syscall_getcwd`); those failures remain
in `build/document-ocr-wasm-corpus.log` and
`build/document-ocr-wasm-corpus-reviewed.log`. Inspection of the full import list
led to an exact allowlist. The memory-growth notification is a no-op because
wazero owns the heap; getcwd/unlinkat/rmdir return permission errors. No host
filesystem fallback was added.

The first successful corpus run exposed a deadline issue: the clipped negative
fixture took 55.54s under the test's 60-second limit. The production worker now
enforces its own 40-second deadline and returns the best validated partial result
when another orientation attempt would exceed the remaining budget. The final
real-engine suite passed in **152.574s**, with **no skips**
(`build/document-ocr-wasm-final-corpus.log`). Seven readable cases matched their
expected text, including visible 0/90/180/270-degree pages. The clipped fixture
returned partial after 35.95s; its incorrect fragments were not labeled complete.
These timings include fixture rendering/cold execution and are not a throughput
claim for arbitrary documents. Confidence thresholds do not prove transcription
accuracy.

The parent validates receipt revision, pixel/language digests, dimensions,
orientation, hOCR bounds and complete/partial claims independently. It maps word
coordinates back to the original raster. The actual subprocess and full typed
PDF extraction test passed in **42.833s**
(`build/document-ocr-parser-integrated-real.log`): scanned text, the previously
failing 180-degree page, and a two-page native-text/scanned PDF all passed. The
combined PDF retained source order, exact scan text once and OCR provenance.
Operational cancellation/errors and output-limit failures return no unfinished
extraction result; validated low-confidence results remain explicitly partial.

Commands and additional checks:

- `go -C runtimes/ocr/worker test -mod=readonly -count=1 -v ./...`, with isolated
  `OFFGRID_TEST_OCR_LANGUAGE` and `OFFGRID_TEST_DOCUMENT_WORKER`: the real-engine
  suite above, plus protocol/orientation/cancellation/import tests.
- `go -C runtimes/ocr/worker build -mod=readonly -trimpath -o ../../../build/offgrid-ocr-worker.exe .`:
  passed on Windows.
- `go test ./internal/rag -run TestParserOCRRealWorker -count=1 -v`, with those
  test variables plus `OFFGRID_TEST_OCR_WORKER`: the 42.833s integration run above.
- `go test ./internal/rag ./internal/documentocr`: passed, RAG **9.844s**
  (`build/document-ocr-rag-final.log`). Real-worker tests require the explicit test
  variables and are skipped by this ordinary suite; the separate real run is the
  evidence for them.
- `build/document-fixtures-win/Scripts/python.exe scripts/test/test-ocr-build.py`:
  **2 passed**, no skips, including staged real engine/notices digest validation.
  The first invocation through bare `python` hit Windows' Store alias; the
  existing isolated fixture environment ran the checks successfully.

Task 2.4 remains unchecked: only pinned English bytes can be explicitly configured
in the parser; user-facing offline language import/readiness is unfinished.
Same-page native/scanned recognition passes at worker level, but source-preserving
spatial merging is not implemented and that document remains partial. Packaging,
clean installs, organization access/retention, citation quality and the defaults-
first Knowledge workflow remain separate acceptance gates. No production OCR flag,
installed service, desktop, model selection, workspace data or release changed.

## Offline language setup and mixed-page fidelity — 2026-11-02

The remaining task 2.4 implementation now includes authenticated offline English
pack import through the service, CLI, Python and optional Knowledge setup. The
service verifies pinned bytes before atomic publication, rejects arbitrary
languages/paths, and loads an immutable snapshot for each extraction. Missing,
invalid, installed and qualified states remain distinct. Installing data neither
downloads an engine nor reindexes documents or establishes release qualification.

PDF raster receipt revision `raster2` includes native-word rectangles derived from
the same page transform as the raster. The parent independently validates these
and merges OCR by spatial coverage, retaining native Unicode. Repeated words at
different positions remain present. Ambiguous overlap or a layout that would
change native reading order stays partial. The parser revision binds the PDF,
OCR, merge and language identities. This does not qualify arbitrary layouts.

Actual configured-worker verification (isolated temporary service data and the
explicit `OFFGRID_TEST_DOCUMENT_WORKER`, `OFFGRID_TEST_OCR_WORKER` and
`OFFGRID_TEST_OCR_LANGUAGE` paths):

- `go test ./internal/rag -run TestParserOCRRealWorker -count=1 -v` passed five
  real scan/rotation/mixed/repeated-text cases in **63.857s**, no skips
  (`build/document-ocr-configured-real.log`). Same-page native plus scanned text
  and distinct repeated phrases both matched the expected content.
- `go test ./internal/rag -run 'TestParserOCRRealWorker/missing_language' -count=1 -v`
  passed in **8.306s**, no skips. Native pages remained available, scanned pages
  remained explicitly unavailable, and no complete extraction was claimed
  (`build/document-ocr-missing-language-real.log`).
- The actual owned-worker cancellation test passed in **0.614s**
  (`build/document-ocr-active-cancel.log`). This proves cancellation of a live
  process waiting for input, not a mid-recognition latency benchmark.
- `go -C runtimes/pdfium test -mod=readonly ./...` passed in **43.555s**
  (`build/document-pdf-spatial-full.log`).
- Full RAG/documentocr/CLI regressions passed (RAG **10.580s**), and the final
  selected service/import/attachment/language regressions passed (server
  **2.873s**, RAG **0.533s**). Real language import through HTTP and CLI was
  exercised with isolated state. Logs: `document-ocr-full-regression-fixed.log`
  and `document-ocr-final-service.log` under ignored `build/`.
- Python Knowledge client tests: **27 passed in 15.07s**
  (`build/document-ocr-python.log`).
- `npm --prefix apps/web run test:e2e -- knowledge-ocr.spec.ts knowledge-import.spec.ts`:
  **28 passed in 49.3s**, covering all nine locales, narrow layouts, keyboard,
  explicit upload, lost acknowledgments and no automatic retries
  (`build/document-ocr-ui.log`). These renderer tests use a fixture service;
  they are not packaged OCR qualification. English/Arabic screenshots were
  visually inspected. Manual language review remains unavailable.
- TypeScript, generated API comparison, source-size ratchet, documentation links
  (302 files) and strict OpenSpec validation passed.

The earlier manifest-count assertion and invalid constant-expression compilation
failures remain in `document-ocr-language-regression.log` and
`document-ocr-full-regression.log`; corrected runs passed. Historical native and
clipped-document OCR failures above remain unchanged.

Task 2.4 is implemented and tested for the scoped fixtures. Clean runtime
packaging, broader formats/languages, partial-source citations, organization
access/retention, resource measurements and integrated defaults-first UX remain
open. Windows race tests were not run with CGO disabled. No installed application,
user models, workspace data or release was changed by these checks.

## Packaging and client boundary follow-up — 2026-11-02

Both parser workers now share desktop staging and copied-package verification.
The desktop resource allowlist excludes staged test executables and language
data. CLI bundles, installer copy paths, CPU/GPU containers and local artifact
reuse include the OCR worker and its notices. A shared CI/release job builds the
pinned WASM from reviewed sources; consumers verify both asset digests before
compilation. Runtime language setup remains explicit. No installed user program
or data was replaced.

The second source build used a fresh owned WSL directory,
`/home/phil/.cache/offgrid-ocr-packaging-repro`, with the same pinned archives and
command as the earlier build. Engine and notice digests matched the reviewed
lock (`build/document-ocr-packaging-repro.log`). This establishes reproducibility
across two directories on this host, not across all build hosts.

Actual app-local worker tests uncovered two additional integration defects:

1. The Go client did not recognize the new language-readiness error codes, so a
   useful missing-language response became generic HTTP 422. Safe static client
   messages now preserve those codes without echoing private diagnostics.
2. The concatenated OCR component revision exceeded the existing attachment
   parser-identity bound and incorrectly produced an attachment-limit error after
   successful OCR. The revision is now a fixed-size full SHA-256 fingerprint of
   all components; provenance remains in parser metadata. No storage quota was
   weakened to accommodate the identifier.

The initial PowerShell test invocation split dotted flags incorrectly; its usage
error remains in `document-packaged-ocr-windows.log`. The subsequent actual
two-defect failure remains in `document-packaged-ocr-windows-fixed.log`. The
corrected compiled test, placed beside both real workers and using temporary
data plus an explicit English fixture, passed **11/11 cases without skips**:

- Windows: **22.35s**, `build/document-packaged-ocr-windows-final.log`.
- Linux binaries running under WSL: **15.91s**,
  `build/document-packaged-ocr-linux.log`.

These exercise authenticated CLI attachment upload, exact retained originals,
source context, saved conversation references and other-actor denial for native
PDF, Unicode, Office, missing language, corruption/encryption and actual scanned
PDF recognition. The inference reply is a fixture: these results do not qualify
model answer quality. Linux-under-WSL is not macOS or clean-installer evidence.

The desktop Node suite passed **101 tests, 1 skipped**. The subsequently added
copied-resource allowlist plus package tamper tests passed **3/3**. The full shared
RAG/documentocr/serviceclient/sessions/CLI suites passed (RAG **11.366s**, client
**5.688s**, CLI **3.007s**). Python build staging passed **3/3**, including no-network
offline staging and preservation after invalid input. The isolated shell installer
fixture passed with both workers, notices, rollback versions and legacy bundles.

Import UX now links missing-language failures directly to the optional setup
panel and focuses it. Language setup does not require enabling Knowledge or
downloading an embedding model. Checking status no longer says a language is
being installed. The import/recovery/OCR renderer suite passed **43 tests with
1 real-service test skipped**; after the last setup dependency correction, all
**12 OCR tests passed in 32.9s**, including the nine locales and no unsolicited
configuration mutation. These are browser fixtures, not human usability review.

TypeScript, API generation comparison, web production build, workflow actionlint,
source-size ratchet and strict OpenSpec checks passed. The web build retains its
large-chunk warning; it is not a measured low-resource performance pass. A
source-size check initially caught one extra installer line; replacing duplicated
parser copy conditions with one common loop restored the reviewed bound.

Container execution remains **unverified**: both build attempts failed before
compilation while Docker Hub resolved `python:3.13-bookworm` with a TLS handshake
timeout (`build/document-ocr-container-build.log` and `...-retry.log`). An offline,
read-only raster/OCR container check is wired into CI but has not passed here.
The explicit x64 compiler build platform is now declared as a build argument.
Task 2.3 remains unchecked, as do clean installers, macOS, enterprise policy and
retention, broader language/format fidelity, citation quality and full UX gates.

### Exact citation readback and partial PDF review — 2026-11-02

Saved Knowledge citations can now open the exact retained passage rather than
the first 200,000 characters of a document. The service checks UTF-8 byte bounds,
original digest, unit boundary, extractor revision and passage content hash;
stale evidence returns 409 without substituting other text. The shared renderer
rejects older service responses that lack explicit citation verification.
Incomplete source units cannot be indexed as complete, and a document cannot
combine locators from different original digests. Native and OCR extractor
revisions may coexist on different pages of the same original.

Partial PDF extraction is now retained separately from indexed documents. The
import owner can explicitly review readable/empty/OCR-needed pages through
`view=extraction`, `offgrid kb review-import ID`, the Python SDK or the shared
web/desktop dialog. Progress listings contain no page text. An additive SQLite
column stores versioned, bounded evidence under the existing storage quota.
Retry clears obsolete evidence; import/document deletion removes it; attempt
checks reject late worker writes. Review never indexes partial content or
implicitly retries work. Explicit use of partial documents in answers is still
unfinished and is not represented as supported by this change.

Validation in this pass:

- `go test ./internal/rag ./internal/serviceclient ./internal/cli ./internal/sessions -count=1`
  passed. The complete server package passed in 52.906s; API/schema/route and
  OCR permission checks also passed. The broader contract run initially found
  the frozen route fixture had not included the OCR-language endpoint. Its
  exact registration and 148-route count were reviewed and corrected.
- Actual Windows PDF-worker source-range checks passed for seven PDF/Office/text
  fixtures in 6.13s, including SQLite reopen, reindex and exact citation offsets.
- The compiled app-local `TestPackagedPartialImportReview` passed in 2.15s:
  real mixed PDF, missing language, native page preservation, CLI review,
  cross-owner denial, zero published documents and deletion denial.
- The development renderer run passed 35 tests with one real-service case
  skipped. Initial new-test failures came from missing fixture capability/session
  projections and React StrictMode read replay; those fixtures were corrected,
  not application authorization or readiness checks.
- The built renderer then passed all 25 partial/recovery tests against a
  disposable service, with `OFFGRID_E2E_DOCUMENT_WORKERS=1`, in 9.9s. This includes
  actual mixed-PDF extraction/review through HTTP, reload without retry, and
  corrupt Office recovery. Log: `build/document-source-renderer-regression.log`.
  Other UI data in these tests remains fixture data; no real model answer quality
  or installed-user workspace was tested. English/Arabic 390px screenshots were
  inspected; manual translation/usability checks remain unavailable.
- Python Knowledge tests: 28 passed. TypeScript, generated API comparison and
  production web build passed; the existing large-chunk warning remains.
- Source-size, repository hygiene, maintained documentation and strict OpenSpec
  checks passed. No commit, installation, deployment or publication occurred.

CI now includes the partial-import packaged test and a Linux built-renderer
worker run. This is configuration, not evidence that remote CI has passed.
The updated workflow parsed with the already-installed `js-yaml`; actionlint
was not available on PATH for this follow-up (the earlier run remains separate).
Task 3.3 remains unchecked: full cell locators, accepted partial-use semantics and
broader retrieval/citation quality still need completion. Enterprise access and
retention, other platform qualification and defaults-first product gates remain.
### Cell citations, measured format coverage and readable recovery — 2026-11-02

XLSX extraction now retains cell identities from worksheet XML with exact UTF-8
spans. Bounded sheet chunks remain intact; this does not create one embedding per
cell. Captured citations include only intersecting cells, including after context
truncation, and survive saved-message serialization and SQLite reindexing. A cell
whose value contains `Z99:` cannot invent a second cell reference. Duplicate,
out-of-range and over-limit cell identities fail explicitly. The XLSX extraction
revision is now `offgrid-xlsx-v3`; existing indexes receive no invented locators.

The new [format matrix](../../guides/document-formats.md) distinguishes available
extraction from missing adapters, scanned-document setup and release qualification.
The [actual Windows score report](document-format-score-2026-11-02.json) contains
37/37 expected outcomes: 25 extracted fixtures and 12 expected partial/rejection
results. No OCR language was installed for that run; the three scan/mixed cases
are correct refusals, not OCR success. Unsupported legacy Office, OpenDocument,
email and standalone image-document formats remain explicitly unavailable. No
positive qualification is claimed for those adapters. Earlier failed baseline
reports remain historical evidence.

The Knowledge recovery list now explains stored failure codes in all nine
locales after reload. It distinguishes unreadable, encrypted, empty, excessive,
unsupported and OCR/language cases without exposing raw internal diagnostics.
Missing OCR language data no longer claims the OCR worker itself is uninstalled.
This changes guidance, not retry, publication, authority or model selection.

Validation recorded for this increment:

- `go test ./internal/rag ./internal/server ./pkg/api -count=1` passed in
  6.830s / 52.056s / 1.428s (`build/document-cell-regression.log`).
- With the actual `build/document-runtime/win-x64/offgrid-document-worker.exe`,
  the frozen corpus score passed 37/37 in 3.06s. The seven-file PDF/Office/text
  source-range, SQLite/reindex and exact-readback test passed in 5.56s. The command
  used `OFFGRID_DOCUMENT_CORPUS_STRICT=1`; all 14 unsupported-format rejection
  checks passed too.
- Four exact citation renderer cases passed, including page/sheet locators,
  English/Arabic, keyboard focus restoration, stale-source refusal and 390px width.
- The wider import/recovery/OCR/citation run initially passed 54 cases, skipped
  one real-service case and failed one old literal-copy assertion (`Invalid
  document.`). The UI correctly showed the new localized recovery guidance. The
  assertion was updated to the intended guidance and its focused rerun passed.
- A fresh built-UI/disposable-service run passed all 32 partial/recovery cases in
  11.5s, including actual mixed PDF extraction/review and Office import recovery.
- API generation/check, TypeScript, production web build, source-size ratchet,
  repository hygiene, 303-file maintained-doc check and strict OpenSpec validation
  passed. The existing large web-chunk warning remains.

Task 2.5 is complete through the explicitly unavailable-adapter branch; it does
not mean legacy formats now extract. Task 3.3 remains open for the rest of its
partial-source/citation acceptance. No clean-install macOS, container, organization
policy, manual localization/screen-reader or model-answer qualification is added
by these results. The release is still held and installed-user state was not changed.

Chat attachment recovery uses the same localized guidance. Visual review at
390px exposed a cramped horizontal card and a duplicated generic error below it.
The corrected card gives the explanation a full-width row and groups Retry/Remove
below; ready-file chips stay compact. Failure remains blocking for send and does
not lose the draft. English and Arabic screenshots were inspected after the fix.
The attachment renderer suite passed 25 cases in 23.9s, with the real-service case
initially skipped for the fixture run. An earlier test edit placed an OCR
assertion in the ready-file scenario; that test-only mistake was corrected before
the passing rerun. No actual extraction behavior was relaxed to obtain a pass.
The subsequent built-UI/disposable-service attachment run passed all 26 cases in
12.3s, including real PDF/DOCX uploads and original-byte preservation. No cases
were skipped in that run. The final web build and maintained-doc/source-size
checks also passed; packaging and installed-app replacement were not performed.

## November 2: explicit attachment reuse and simpler search setup

Saved Chat files now offer an explicit Add to Knowledge operation. The service
copies exact retained original bytes after independently checking conversation
source access and Knowledge management permission. A digest-bound JSON reference
uses the existing durable import endpoint; a stable actor-scoped acceptance key
avoids another import after a repeated acceptance. Acceptance alone does not
extract, index or infer. Source deletion and pending-import deletion remain
independent. CLI `kb import-attachment` and Python `kb.import_attachment` use the
same contract, with no automatic mutation retries.

The shared web/desktop dialog explains the separate library copy before writing.
It checks search readiness, offers setup if needed, and keeps the Chat draft.
Only a freshly accepted import proceeds to explicit indexing; failed, interrupted
or uncertain attempts lead to saved recovery rather than replay. Old services
without `document-attachment-imports-v1` do not display this action. Completion
uses Close, not a misleading Cancel label. English/Arabic narrow dialogs were
visually inspected; automated localization covers all nine locales.

Knowledge setup now says Search your documents and explains the one-time search
model. The installed model is offered first, model choices and model management
remain optional, and initial setup does not repeat library guidance. Working
libraries no longer fetch catalog/download administration during refresh.
Optional catalog failure or slowness does not prevent choosing an installed
model; late metadata cannot overwrite an explicit choice. A missing model plus
unavailable catalog still presents a recoverable error. No automatic download,
enablement, reindex or change of existing index identity was added.

Validation for this increment:

- `go test ./internal/server ./internal/cli ./internal/sessions ./pkg/api -count=1`
  passed (52.262s / 1.217s / 4.928s / 1.573s), including source/digest/permission,
  malformed reference, duplicate acceptance, independent deletion and CLI tests.
- `PYTHONPATH=sdk/python build/document-fixtures-win/Scripts/python.exe -m pytest
  sdk/python/tests/test_knowledge_files.py -q`: 29 passed in 14.34s.
- Initial publication fixtures: 13 passed, one actual-service case deliberately
  skipped. Subsequent production-renderer/disposable-service runs exercised that
  case and actual PDF/DOCX attachment uploads using the app-local PDF worker.
- Combined built-renderer run: 122 passed in 58.0s across attachment reuse,
  document/image inputs, Knowledge setup/imports, model choices and voice.
  After the final setup refinements, the affected suites passed 75/75 in 30.7s,
  including a mixed document/image/text draft reload/removal case. These runs
  overlap; their counts must not be added as unique scenarios.
- Final first-use and library suite: 18/18 passed in 5.9s, including all nine
  locales at 390px, keyboard disclosure, unavailable/slow catalog and late
  metadata preserving an explicit selection. English/Arabic layouts inspected.
- API generated-contract check, TypeScript/production build, maintained docs,
  repository hygiene, source-size ratchet and strict OpenSpec validation passed.
  The existing web bundle-size warning remains.

Failed checks were retained while correcting test assumptions: an older/new
assistant selector race (59 passed, one failed initially); development React
StrictMode's cancelled metadata requests (42 passed, one failed initially); and
a radio-input assertion that incorrectly looked for child text (17 passed, one
failed initially). The tests now target the current answer, live request and
selected input respectively. The voice stale-response test also passed again in
development mode; dismissal still requires all pending requests to be aborted.

Task 1A.3 is complete for the shared renderer and authenticated service contract.
This does not establish real-model answer quality, installed desktop dialog
qualification, organization collection policy, all-platform packages or manual
accessibility/localization review. Those gates remain open. No installed service,
desktop, model, personal document or release state was changed in this increment.

## Shared document scheduling and visible progress — November 2

Task 4.3 remains in progress. Knowledge imports previously serialized their own
work, but conversation attachments could launch additional PDF/OCR readers.
The service now shares one reader admission boundary across normal parser entry
points, with four waiting slots and a 30-second wait limit. Durable import
admission is also bounded before an attempt starts or original bytes are loaded.
Index work remains serialized, and releases the reader before waiting for
embedding/publication. No model or index identity changes are made.

Queue exhaustion is the operational `document_busy` error (HTTP 503), not a
corrupt-file classification. Accepted sources remain available for explicit
retry. Web/desktop imports show waiting, reading and indexing phases in all nine
locales. Status polling is read-only and aborts with its operation; a busy batch
stops before sending subsequent files. API and format documentation describe
these boundaries. These limits do not establish a total-process memory ceiling.

Validation on Windows, using disposable state only:

- `go test ./internal/rag ./internal/server ./internal/sessions -count=1` passed
  (7.196s / 54.149s / 5.309s; `build/document-scheduling-go.log`).
- Following the additional independence and HTTP error-contract assertions,
  `go test ./internal/rag ./internal/server ./pkg/api -count=1` passed
  (7.197s / 51.499s / 1.084s; `build/document-scheduling-final-go.log`).
  The tests cover full admission, timeout, prompt cancellation, idempotent slot
  release, preserved unstarted source identity and reading another attachment
  while index publication is blocked.
- Actual pinned Windows PDF/OCR workers and the verified offline English pack:
  `TestParserSchedulingNativeMeasurements` passed. Compressed PDF extraction
  took 1.844s cold and 0.078s warm; scanned PDF extraction took 4.779s cold and
  3.066s warm. Exact expected text and complete extraction were checked.
  Cold means an empty PDF compilation cache; warm still starts a fresh worker.
  These are single-sample reader measurements, not full import/model throughput
  claims or low-memory hardware qualification (`build/document-scheduling-native.log`).
- Development renderer progress/recovery/publication suite: 45 passed, two
  real-service cases skipped. Rebuilt service and production renderer then
  passed all 74 selected progress, recovery, publication and file tests,
  including those real-service cases (`build/document-progress-service.log`).
- An added browser cancellation check holds both the operation and progress
  request pending, confirms both signals are cancelled, and observes no further
  polling or mutation. The final progress suite passed 12/12 in 17.6s
  (`build/document-progress-cancel.log`). These suites overlap.
- Generated API check, TypeScript, production build, maintained documentation
  (303 files), repository hygiene, source-size ratchet and strict OpenSpec
  validation passed. Existing web bundle-size warning remains.

Still unverified for task 4.3: full cold/warm indexed-import measurements with a
real embedding model and concurrent real-model Chat under constrained memory.
No claim of enterprise readiness, all-platform qualification or improved model
answer quality follows from this increment. No installed app, service, container,
user documents or release state was changed.

## Corpus-wide source and OCR citation persistence — November 2

Task 3.3 verification now covers all 25 successful extraction fixtures in the
frozen 37-file corpus, instead of only seven representative files. Actual pinned
Windows PDF extraction feeds exact page/section/slide/sheet chunks. Every chunk
is persisted, the SQLite store is closed/reopened, the document is rechunked and
reindexed, and every returned citation is independently resolved against retained
UTF-8 bytes, source digest, extraction revision and content hash. Synthetic
vectors exercise persistence/retrieval plumbing, not semantic relevance quality.

The workbook checks all six populated cell identities, including its rich text,
numeric and formula cells. The original manifest lists three anchor cells rather
than an exhaustive inventory; an initial 24-pass/one-fail comparison exposed
that test assumption (`build/document-corpus-citations.log`). The fixture XML was
inspected, and the test now explicitly expects the six real cells. The frozen
document and manifest were not changed. DOCX retains its documented body-section
locator; paragraph anchors in the corpus do not become invented rendered pages.

Actual OCR worker results for a mixed-page document and two same-page native/scan
documents now also pass SQLite restart, exact chunk citation readback, page
identity and stale-OCR-revision rejection. A mixed PDF without the language pack
retains exact native page text and the unreadable page state through restart under
the import owner; another actor cannot read that preview, and no ready document
is published. Existing partial/low-confidence unit checks continue to reject
complete indexing. The OCR provenance assertion was updated to require the
current fixed-size extraction fingerprint plus separate worker/language metadata,
instead of expecting the former concatenated revision.

Checks:

- Actual worker corpus/citation and stale/incomplete-source regression command:
  `go test ./internal/rag -run 'TestSourceRangesOfficeAndTextSurviveSQLiteAndReindex|TestSourcePassage|TestCompleteExtraction|TestWorksheetCitation' -count=1 -v`
  passed in 22.485s (`build/document-corpus-citations-final.log`).
- `TestParserOCRRealWorker` with the pinned Windows PDF/OCR workers and verified
  English pack passed all six scenarios in 15.778s
  (`build/document-ocr-citations.log`).
- Source/citation API and saved-history locator tests passed in 0.602s
  (`build/document-citation-api.log`).
- Browser citation/original/scope suite passed 27/27 in 23.1s, including stale
  sources, unsupported legacy responses, inert content, keyboard focus, narrow
  layouts, English/Arabic citation dialogs and nine-locale scope handoff
  (`build/document-citation-ui.log`). These are renderer fixtures, not proof of
  real-model citation selection.

This completes mechanical source/citation fidelity for task 3.3, not model answer
quality, organization-level policy or all-platform qualification. Those remain
separate open tasks. No installed data or release state was changed.

## Keyword retrieval and pre-ranking source selection — November 2

Tracing the organization boundaries exposed a separate retrieval defect. The FTS
table uses external content from `chunks`, whose identifier column is `id`.
Selecting `chunk_id` from the FTS external-content view produced
`no such column: T.chunk_id`; the store silently fell back to semantic-only
retrieval. Keyword candidate limiting was also global, before the selected
document filter, so unrelated matches could exhaust the candidate list.

The query now joins FTS row identity to `chunks.id`, and applies parameterized
document filters to keyword candidates and semantic rows before ranking/limits.
No database rebuild, model change or reinterpretation of selected IDs as grants
is involved. Selection remains a query constraint, not an organization ACL.

The new regression uses zero-similarity vectors, one matching selected document
and twenty stronger excluded keyword matches. Before the fix it returned no
selected passage. With the fix, the selected passage ranks first; unscoped
keyword retrieval also returns the expected matches rather than silently falling
back. SQL-looking document IDs return no results and cannot broaden selection.
The global top ten are independently confirmed to exclude the selected document,
so the fixture exercises candidate crowding rather than merely result filtering.
The original failing log is retained at `build/document-scope-ranking-before.log`.

`go test ./internal/rag ./internal/server ./pkg/api -count=1` passed
(8.300s / 53.925s / 0.611s; `build/document-retrieval-final.log`). The strengthened
crowding assertion passed separately. Source-size, documentation and strict
OpenSpec checks passed. Organization policy task 4.1 remains open: this corrects
retrieval mechanics but does not introduce private collections or tenant isolation.

The retrieval fix and its regression were isolated from the unfinished changes
and committed on `main` as `e33ffc49` (`fix(rag): restore scoped keyword
retrieval`), then pushed normally to `origin/main`. The exact staged snapshot
also passed the full RAG/server/API suites (0.957s / 50.886s / 1.197s;
`build/retrieval-staged-validation-final.log`). Its initial snapshot test omitted
`cmd/offgrid-executor`, causing two missing-directory test failures; the snapshot
was corrected to include that tracked prerequisite and rerun without modifying
product behavior. Other document/UI changes remain uncommitted. No release was
published by this checkpoint.

## Collection-policy foundation and returning-user recovery — November 2

The original PDF/document reliability and defaults-first UX requirements remain
the release priorities. CI for the isolated retrieval commit `e33ffc49` completed
successfully: run `41394021563`. This does not qualify the unfinished work below.

Added versioned collection records and document bindings in SQLite. The explicit
migration assigns existing documents to the existing workspace policy without
changing their source bytes. Internal methods check current membership, reject
stale updates, restrict private audience changes to the collection owner, and
preserve bindings across atomic reindexing. Legacy replacement cannot reset a
private or damaged binding to shared access. Missing or unknown-version records
fail closed; restart does not recreate a removed grant. Collection creation and
movement are **not exposed to clients**: ingestion namespaces, retrieval and
source/export/delete enforcement still need complete integration. Task 4.1 stays
unchecked, and this is not a tenant-isolation or enterprise-readiness claim.

`go test ./internal/rag -run 'Test(CollectionPolicy|DocumentPolicy)' -count=1 -v`
passed in 0.848s (`build/collection-document-policy.log`), including revocation,
concurrent policy edits, stale movement, invalid bindings, restart and deletion.
The first migration fixture tried to delete a referenced collection; the new
foreign key correctly rejected it. The corruption fixture now removes its
binding explicitly before removing the policy, while verifying retained source
bytes. The initial failure remains in `build/collection-policy-binding-baseline.log`.
Full RAG and server tests passed in 12.497s and 124.901s. That invocation mistakenly
included nonexistent `./internal/api`; the actual `go test ./pkg/api -count=1`
then passed in 1.303s. This command error is not reported as a passing full command.

A returning-user browser regression reproduced a separate UX defect: a saved
import completed in another view/client and disappeared from the processing list,
but its document did not appear until manual refresh. Recovery now refreshes the
library once when an observed active import becomes ready. It performs no retry,
download or configuration mutation. The nine-locale test then selects that source
and hands it to Chat without setup, maintaining the exact selected scope on a
narrow layout. The failing baseline is `build/knowledge-returning-before.log`.

The first broader browser pass caught an error-message regression: clearing an
observation error also cleared a failed retry's explanation. Removed that change
and reran all 71 tests against the freshly built isolated service. All 71 passed
in 1.8m (`build/knowledge-returning-regression-final.log`). This covers first-use
defaults, returning imports, recovery, cancellation/progress, and partial-source
review. It includes real-service malformed-DOCX retention/removal and actual mixed
PDF partial extraction; other browser scenarios use controlled response fixtures.
The initial 70-pass/1-fail result remains in `build/knowledge-returning-regression.log`.
These tests do not establish real-model answer quality or human usability.

With the pinned Windows PDF/OCR workers and the verified English language pack,
`go test ./internal/rag -run 'TestSourceRangesOfficeAndTextSurviveSQLiteAndReindex|TestSourcePassage|TestParserOCRRealWorker' -count=1 -v`
also passed in 135.838s (`build/document-policy-native-sources.log`). This reran
the 25 successful corpus documents' exact citations, OCR scenarios and stale
source rejection with the new storage migration. It ran alongside browser checks;
its wall time is not an isolated performance benchmark.

TypeScript, renderer build, generated API, source-size, repository hygiene,
303-document link check and strict OpenSpec validation passed. The renderer build
still reports its existing large-chunk warning. No installed workspace, desktop,
container, runtime configuration or release was changed by this pass.

## Actor-bound document delivery — November 2

Continued task 4.1 through the engine and service call paths. `KnowledgeView`
binds an authenticated actor to current policy for lists, document metadata,
originals, retrieval, scoped prompt augmentation, counts, reindex and deletion.
Ordinary Chat, profile admission, deferred profile inventory, schedule resource
validation, document handlers, distributed search's local branch and retrieval
evaluation use that view. The older engine getters/search remain compatible with
the shared workspace but cannot become implicit readers of private collections.
This is an additive internal interface, not an exposed collection-management API.

Retrieval resolves permitted IDs before generating the query embedding or ranking
candidates. An empty permitted set returns no matches without inference; an
explicit denied selection fails instead of broadening. Result delivery rechecks
current membership after inference. Reindex checks edit access before work and
again inside the SQLite publication transaction. Deletion checks policy while
holding the same store lock used by membership updates. Source delivery checks
again before writing bytes; document lists, search, status and source responses
are marked `Cache-Control: no-store`. Shared compatibility ingestion cannot reveal
a private duplicate's title or link that private publication as a shared receipt.

New deterministic engine tests passed (0.744s,
`build/knowledge-policy-engine.log`). They exercise shared compatibility callers,
private original downloads, revocation on an existing view, read-versus-edit
authority, candidate exclusion, empty-set handling without an embedding engine,
revocation during query embedding, and rejected publication preserving the old
source. Synthetic vectors establish policy mechanics, not retrieval quality.

Authenticated HTTP tests passed (1.355s, `build/knowledge-policy-http.log`), using
isolated users and real SQLite documents. A global administrator outside a
collection cannot read its original, citation, source, list entry or counts, and
cannot delete it. A collection reader with global management permission still
cannot delete it. A guest listed in the collection remains blocked by the global
Knowledge permission. Revocation denies subsequent source/citation/original
requests; an authorized editor can delete the source, which becomes unavailable.

`go test ./internal/rag ./internal/server ./pkg/api -count=1` passed in
11.634s / 103.822s / 1.565s (`build/knowledge-policy-full.log`). After adding the
evaluation view, explicit no-store headers and private-duplicate regression,
`go test ./internal/rag ./internal/server -run 'TestKnowledge|TestDocumentCollectionPolicy|TestDocumentOriginal|TestDocumentCitation|TestLegacyIngestion|Test.*Evaluat' -count=1`
passed in 0.828s / 2.409s (`build/knowledge-policy-delivery.log`). Source-size,
repository-hygiene, diff whitespace and strict OpenSpec checks passed. Local race
coverage and full platform CI were not run for these uncommitted changes.

**Still incomplete:** collection-scoped import identity/publication and
deduplication, policy treatment of persisted source passages in saved conversations
and jobs, and collection-aware retention. Private collection creation/movement
therefore remains unavailable through HTTP/CLI/UI, and task 4.1 remains unchecked.
No claim of complete cross-tenant isolation, release qualification or installed
user behavior is made from these tests. Existing installed state was untouched.

## Collection-scoped import and deletion — November 2 follow-up

Continued the same document-ingestion task; PDF/Office reliability and the
defaults-first Knowledge journey remain release requirements. This storage slice
does not expose another configuration panel or enable private collections yet.

Policy migration 3 gives each durable import a destination binding and each
published document an immutable original-source namespace. Existing imports and
document IDs retain the shared workspace behavior. Identical bytes in separate
private collections receive different IDs; matching content never silently
changes the requested audience. Publication rechecks current edit authority in
the original/vector/import transaction. Reads, partial-extraction review, retries
and extraction transitions enforce the current collection policy. Import lists
omit entries whose destination is no longer readable.

Deleting a source now invalidates pending copies in its own source namespace,
plus receipts already linked to that document. It does not invalidate identical
originals in another collection. Old workers cannot publish through the deleted
import lease. Original download integrity uses the retained namespace, including
after reopening the database. Unknown import-binding versions cannot authorize
execution or be silently repaired into workspace grants.

The first full RAG run caught a private-title disclosure in a shared ingestion
duplicate check (`build/collection-import-initial.log`). The follow-up requires
destination equality as well as actor access; the regression remains in
`TestLegacyIngestionDoesNotDisclosePrivateDuplicate`. The subsequent RAG suite
passed (`build/collection-import-regression.log`, 11.350s).

Validation from this working tree:

- `CGO_ENABLED=0 go test ./internal/rag ./internal/server ./pkg/api -count=1`
  passed in 14.257s / 139.772s / 0.819s
  (`build/collection-import-full.log`). New tests cover scoped duplicates,
  cross-destination replay, revocation during indexing, no ghost publication,
  original/preview denial, deletion isolation, stale-worker rejection, restart
  and legacy/future-schema behavior.
- Real app-local PDF/OCR workers passed `TestParserWorkerRealCorpus`,
  `TestParserOCRRealWorker`, and
  `TestSourceRangesOfficeAndTextSurviveSQLiteAndReindex` in 131.583s total
  (`build/collection-import-native-corpus.log`). These exercise compressed,
  mapped-font, object-stream, rotated/scanned/mixed PDFs and exact persisted
  source ranges for qualified Office/text fixtures. This duration includes test
  setup and concurrent fixture work; it is not an end-user latency claim.
- The report-only corpus scorer initially skipped because no report destination
  was set. A separate strict run with an explicit isolated report path passed
  all 37 expected text/error outcomes (8.063s,
  `build/collection-import-corpus-score.log` and
  `build/collection-import-corpus-score.json`). Expected refusals count as correct
  classifications, not successful extraction; OCR and citation checks are the
  separate tests above.
- A freshly compiled isolated server test executable beside the pinned workers
  passed `TestPackagedDocumentCLIAndSessionContract` (21.82s) and
  `TestPackagedPartialImportReview` (6.30s), including real CLI attachment uploads,
  original readback, other-actor denial, OCR-language readiness and partial import
  recovery (`build/collection-packaged-contract-retry.log`). The initial command
  did not run tests because PowerShell split an unquoted dotted test flag; its
  log is retained as `build/collection-packaged-contract.log`. Corrected quoted
  arguments ran the tests. Their model completion is controlled fixture output,
  not a claim about real model-answer accuracy.
- API schema generation check, source-size, repository-hygiene, maintained-link,
  diff-whitespace and strict OpenSpec checks passed. Local race coverage,
  full platform CI and clean installed-platform qualification were not run for
  this slice.

Task 4.1 remains unchecked: saved conversation/job passages and their revocation
semantics still need implementation and acceptance. Full retention/backup/hold
behavior and actual model-answer/UI/CLI acceptance also remain outstanding.
Private collection creation/movement remains unavailable in public clients.
No installed workspace, desktop, container or runtime configuration was changed.


## Saved Knowledge access — November 2 follow-up

Implemented live source-policy checks for saved conversation read/search,
continuation and response delivery. Saved messages now retain document dependency
IDs even when an answer used the attached-source inventory without quoting a
passage. These references are not grants. Owner/admin session access cannot
bypass current Knowledge access; owner stop/delete controls use a redacted
projection so revocation does not trap work in an undeletable state.

Canonical and legacy job reads, event streams, artifacts and error responses
check the same policy. The durable runner checks before inference, before tool
dispatch, after authorization and before publishing progress/results. Revocation
stops further work rather than accepting a completed answer from cached passages.
Previously delivered text cannot be recalled. Stored history and backups are not
erased by revocation; full retention/hold handling remains unfinished.

Validation (isolated state; no installed workspace changes):

- `CGO_ENABLED=0 go test ./internal/sessions ./internal/cli ./internal/agents ./internal/server ./internal/rag ./pkg/api -count=1`
  passed: 9.809s / 2.706s / 62.493s / 113.625s / 13.054s / 1.890s
  (`build/knowledge-history-complete-regression.log`).
- Additional final response/access regression checks passed in 17.630s
  (`build/knowledge-history-response-checks.log`). Tests cover revocation after
  retrieval, during authorization and streamed delivery, inventory-only
  dependencies, owner deletion, other-actor denial, legacy endpoint filtering
  and rejection of client-seeded source authority.
- Generated API check, TypeScript, source-size, repository-hygiene, maintained
  links and strict OpenSpec validation passed. Local race checks and full platform
  CI were not run for this slice.

Task 4.1 remains open for the remaining policy surface and acceptance audit.
Private collection management is not exposed in public clients. These tests are
not a claim of complete enterprise readiness or actual model-answer quality.

## Defaults-first Knowledge presentation — November 2 follow-up

The visible Knowledge flow now puts adding documents, readiness and asking ahead
of configuration. First use has one enable action using the installed default;
the model selector follows it inside optional settings. Empty-library guidance
no longer competes with setup. A working library groups format and OCR details
under Document options, shortens the main instructions and keeps maintenance
in a labeled header menu. Source-storage details remain in document management.
A saved missing-language failure opens the specific OCR controls and their parent
disclosure directly, without retrying the import or installing anything.

Successful library refresh removes vanished source selections; failed refresh
clears stale document names and selections instead of leaving a misleading
usable library. This is a UI safeguard, not a substitute for server policy.
The underlying upload input now shares the button's disabled state.

Validation:

- TypeScript and production build passed (`build/knowledge-simple-build.log`).
  The existing build chunk-size warning remains; this is not a performance claim.
- The initial browser run passed 58/59. A navigation fixture submitted directly
  through the hidden file input before the Add document action was enabled.
  The fixture now waits for real UI readiness; the follow-up passed 59/59
  (`build/knowledge-simple-flow.log`, `build/knowledge-simple-flow-verified.log`).
- After the final wording/placement changes, 81 browser checks passed (2.6m,
  `build/knowledge-simple-final.log`): nine locales, narrow layouts, first use,
  returning imports, explicit OCR setup, source handoff, selection, lost replies,
  navigation cancellation, no implicit writes and stale-library recovery.
- Ten action-hierarchy checks passed (21.6s,
  `build/knowledge-action-hierarchy.log`), including keyboard menu restoration,
  Arabic/English 320/1440px layouts, forced colors and 44px touch controls.
- Inspected first-use and returning-library screenshots. The tests use isolated
  controlled responses, not actual model-answer or native desktop qualification.
- Source-size, repository hygiene, generated API, strict OpenSpec and maintained
  documentation checks passed. The broader defaults-first program, real-service
  acceptance, manual localization/accessibility and platform checks remain open.

No deployment or installed-workspace modification was performed for this slice.

## Source-retention acceptance audit — November 4

Task 4.2's source-storage scope is implemented and checked. The persisted,
revision-checked policy defaults to no expiry, supports a legal hold, and is
enforced for manual deletion and bounded service-owned expiry. One transaction
removes a source's original bytes, extracted text, vectors and related import
payloads. Stale import publication cannot resurrect it. The maintenance pass
also reconciles offline rebuild caches against the live library; it does not
alter independent backup archives. Restoring a backup into a new workspace
preserves its snapshot and holds, then reapplies expired policies. Saved answer
text and independent backup archives have separate retention, as documented in
the API reference; source deletion makes no secure-erasure claim.

Current acceptance evidence: Windows `go test ./internal/rag ./internal/server
./internal/cli -run 'TestRetention|TestDocumentRetention' -count=1` passed.
WSL `CGO_ENABLED=1 go test -race ./internal/rag ./internal/server -run Retention
-count=1` passed. The targeted CLI retention contract passed, and the Python
retention client suite passed 12 tests. These cases cover restart, unknown
policy, hold/release, offline backup and restore, concurrent expiry/deletion,
stale-worker publication, cache batch/restart/error paths, redirected staging
paths, and source/original/preview denial after expiry. This focused audit does
not close collection policy (4.1), scheduling (4.3), complete document-answer
quality (5.2), or unavailable platform and human checks.
