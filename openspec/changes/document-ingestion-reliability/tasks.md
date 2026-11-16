# Tasks

## 7. Approved upgrade: evidence loss and retrieval

- [x] 7.1 Freeze the DOCX footnote, punctuation/identifier search and multilingual attachment failures as permanent tests; repair shared literal query segmentation and explicit search failures.
- [x] 7.2 Retain linked DOCX notes and propagate extraction warnings/coverage through imports, attachments, previews, saved evidence, CLI and model context; verify HTTP and UI behavior.
- [x] 7.3 Establish additive API contracts and localized coverage presentation, retaining old-source identity and explicit incomplete-review behavior; pass contract and affected client checks.
- [x] 7.4 Audit Chat and document image handling; reproduce and repair header-only API acceptance and browser allocation before dimension checks. Verify projector admission, byte preservation and all-locale image controls; record unqualified orientation, animation, visual accuracy and image-document OCR separately.

Evidence: [document/image repair increment](../../../docs/quality/evidence/document-image-upgrade-2026-11-12.md).
These checks do not close the broader phases below, native-language review or
model/hardware release qualification. Existing extractions are not rewritten.

Image follow-up: bounded primary Exif orientation now has an inference-only
lossless projection shared by counting/completion/streaming/tool serialization.
Original bytes remain unchanged; ambiguous metadata and animation are rejected
with all-language browser recovery. Eight-orientation browser/pixel fixtures pass.
This does not close image-document OCR, colour/HDR consistency or model-quality
qualification in the adapter and release tasks below.

## 8. Approved upgrade: structured evidence and adapters

- [x] 8.1 Add immutable structured revisions, blocks/relationships, corrections and layout-aware evidence chunks with atomic reindex and old-citation retention tests.
- [ ] 8.2 Extend PDF, DOCX, XLSX and PPTX extraction for the approved printed-record structure, with critical-number/exception/table fixtures.
- [ ] 8.3 Add bounded PNG/JPEG/multipage TIFF OCR and pinned multilingual pack manifests; preserve explicit language qualification and mixed-page provenance.
  - Mixed-pack increment (2026-11-14): up to three ordered verified packs now run in one isolated memory-only OCR engine. Chat/Knowledge controls, immutable selections, client capability gates, retry checkpoints and source-pack readback are implemented. Real Chinese/French image OCR, private HTTP recovery and both checkpoint stores pass; all 29 picker locales and the full 139-test Python suite pass. Page hints, deskew and representative qualification remain open. See `docs/quality/evidence/document-mixed-ocr-2026-11-14.md`; 8.3 remains unchecked.
  - Multilingual pack/selection increment (2026-11-14): one pinned manifest now supplies 25 digest-verified packs to the service and isolated worker, with explicit mappings for 26 interface locales and no substitution for Shona, Ndebele or Zulu. Chat and Knowledge save single-language selection through retries/restarts and explicit attachment publication. All packs initialize; Chinese/French/Swahili real OCR and private HTTP recovery pass. The 29-language picker suite passes. Simultaneous packs, page hints, deskew/checkpoints and representative qualification remain required; this does not close 8.3.
  - TIFF extension implemented (2026-11-14): bounded multipage decoding, page-linked OCR, retained partial-page review and shared reader navigation. Real packaged two-page/rotated OCR, private HTTP context and local Knowledge retrieval/source readback pass; all 29 reader locales pass. See `docs/quality/evidence/document-tiff-2026-11-14.md`. Deskew, multilingual manifests/selection, page checkpoints and representative qualification remain open; 8.3 is not complete.
  - PNG/JPEG increment implemented (2026-11-13): bounded pixel decoding, EXIF/source coordinates, partial receipts, original/text reader, explicit partial-OCR acknowledgement and private/library ingestion regressions pass. See `docs/quality/evidence/document-image-ocr-2026-11-13.md`. TIFF, deskew, mixed-language manifests and qualification remain required; this increment does not close 8.3.
- [ ] 8.4 Package the optional offline Docling standard layout/table worker; add page checkpoints, bounded scheduling, cancellation and clean-install evidence.

OCR page recovery increment (2026-11-14): validated page receipts now survive
cancellation/restart in both private attachment storage and scoped SQLite imports.
Retries validate source/raster/pack/worker identity and reuse saved OCR pages;
uncertain OCR remains partial. Current-access/attempt checks, quotas, migration
snapshots and deletion cascades apply. Real two-page French TIFF tests cancel
after page one, reopen storage, process only page two and then reuse both pages
without launching the worker. Affected full Go suites and the renderer build pass.
This closes the OCR receipt-recovery slice, not the wider Docling/scheduling,
mixed-language or qualification tasks. Evidence:
`docs/quality/evidence/document-ocr-languages-2026-11-14.md`.

Client verification update: local WSL Python is now available. The complete SDK
suite passed 134 tests, including the previously authored collection, source,
analysis, synthesis and composition clients. Historical notes below describing
Python as unrun apply to those earlier increments, not the current suite.

8.1 progress: immutable graph records and typed block/relationship projections now
publish atomically with indexes, retain historical citations under current ACLs,
and cascade on authorized deletion. DOCX carries declared headings, tables and
clause/note relationships; bounded chunking preserves ordinary paragraphs.
Attributed leaf corrections now publish through the engine with revision checks,
unchanged-vector reuse and revocation rechecks. Scoped structure/correction HTTP
contracts, CLI/Python clients and a shared Chat/Knowledge text-review panel are
implemented. The panel pins revisions, rejects incomplete edits and preserves
drafts across view switches and uncertain saves. Knowledge now expands explicit
heading/header/note links into separately citable bounded passages, with saved
receipts that become incomplete on omitted/truncated context. DOCX recognizes
declared repeating-header rows without cross-table associations. Private Chat
attachments now retain digest-bound structured graphs and share link expansion
and bounded reader projections with Knowledge. Saved selections disclose missing
links independently of model prose; HTTP citation readback, private read-only
inspection, CLI and all-language notices are implemented and tested. Python
inspection parity is authored but unrun in this environment. Richer reader views,
private derived-correction workflows, navigation-wide draft retention and the
full end-to-end exit gate remain open.

8.2 progress: XLSX now retains typed native values, styles, formula/cache
provenance, hidden content, merged ranges and explicit table headers through
structured revisions. Supported deterministic displays retain raw numbers;
unsupported formats require review. Bounded range processing rejects ambiguous
overlaps. Parser, persistence/correction, private HTTP/generation/saved-citation
and real local Knowledge retrieval/source tests pass. This is not native Office
display qualification or the full table benchmark. PPTX now retains notes linked
to slide evidence, native table rows/cells/merge attributes, hidden-slide state,
authored visual descriptions and image/chart part identities. It does not infer
semantic headers from first-row styling or claim visual interpretation. A
private HTTP context/citation regression and real local Knowledge retrieval
exercise notes containing a deadline exception. Visual ordering, master/layout
content, geometry and representative table qualification remain open alongside
the wider PDF/DOCX work. See the separate XLSX/PPTX evidence reports.

DOCX margin increment (2026-11-13): live referenced header/footer parts now
retain native text and typed source sections, including header-only documents.
Historical references are excluded, ambiguous/external references fail closed,
and current tracked margin text carries a review warning. First/even-page
visibility, section placement, field recalculation and revision alternatives
remain unresolved and explicitly disclosed. Private HTTP context/saved-citation
and local Knowledge retrieval tests cover a footer exception; all 29 reader
format notices are updated. See `docs/quality/evidence/document-docx-margins-2026-11-13.md`.
This does not close the remaining PDF/DOCX work or task 8.2.

DOCX outline follow-up (2026-11-13): v6 resolves declared paragraph-style
inheritance, defaults and direct body-text overrides, retaining native text and
excluding historical formatting from the current outline. Bounded style parsing
rejects cycles, duplicate definitions and unsafe relationships. This remains
native structure, not rendered Word layout or revision-alternative review.

## 9. Approved upgrade: retrieval and analyses

- [ ] 9.1 Share private/library retrieval, bounded lexical/semantic candidates and context expansion; replace JSON/full-body vector scans with bounded binary-vector scoring while preserving ACL checks.
- [ ] 9.2 Add durable summary, requirements and comparison analyses with coverage receipts, source/cell validation and supported-section delivery; expose CLI/Python/API parity.

9.2 connected candidate-workflow increment (2026-11-13): summary/requirements
traversal now has atomic durable checkpoints, restart/cancellation/revocation
guards, pinned local model/runtime execution, HTTP and Go/CLI clients, authored
Python parity, and a 29-language source-review workspace. Traversal completion
remains `awaiting_review`; draft JSON/CSV/Markdown exports now preserve all
candidate quotations, conditions/exceptions and gaps with full checkpoint
validation. Actual owned-runtime English/Chinese HTTP-to-export fixtures pass;
source-value validation is versioned without rewriting old receipts. Support
assessment, final synthesis and representative model
qualification are not closed. See
`docs/quality/evidence/document-synthesis-jobs-2026-11-13.md` for executed checks
and remaining evidence. This does not close task 9.2.

9.2 human-review increment (2026-11-13): completed drafts now support explicit
keep/exclude/unreviewed decisions, bounded notes, section checks, immutable review
history and optimistic concurrency through HTTP, Go/CLI and the shared renderer.
Exports retain both original candidates and decisions, including checked-section
counts, without claiming independent verification or repairing extraction gaps.
Review ownership, current-source access, quotas, snapshots and retention are
enforced. All 29 catalogs include the controls; translator approval remains open.
Python parity is authored, not executed. See
`docs/quality/evidence/document-synthesis-review-2026-11-13.md` for checks and
limitations. Full 9.2, including final synthesis and private attachments, stays open.

9.2 composition increment (2026-11-13): a bounded, checkpointed merge tree now
connects finished summary candidates to a report with original-source lineage.
Input review revisions are pinned; changed reviews stop further work and remain
explicit on historical reports. Authenticated status/history/report/evidence,
cancel/retry and bounded exports are connected to Go/CLI, authored Python and a
29-language report/source workspace. Original candidate gaps and unrepresented
batches remain visible independently of generated prose. The real-model fixture
exposed a summary schema mismatch and translated quotations; generation grammar
was tightened while retaining strict receipt validation. Full task 9.2 remains
open for support assessment, private-attachment integration and representative
qualification. See `docs/quality/evidence/document-composition-2026-11-13.md`.

9.1 progress: binary projections, bounded candidate scoring/fusion, checked
pre-migration snapshots and stale-projection invalidation are implemented.
Bounded structural expansion is implemented for Knowledge, including actual
local-model/HTTP header readback and all-language incomplete-link notices.
Shared private/library indexing, attachment expansion and scale qualification
remain open. Evidence-level/per-language evaluation support is implemented;
the human-labelled corpus and its release targets in 11.2 remain open.

9.2 progress (2026-11-13): shared analysis plans account for every retained
source unit and preserve extraction/review gaps independently of output prose.
Owner-private comparison jobs pin both immutable revisions, checkpoint each unit
pair atomically, resume after restart and enforce current access on publication,
results and source readback. Exact text differences and native spreadsheet-cell
metadata changes are available through `/v1/document-analyses`, CLI and generated
TypeScript contracts; Python parity is authored but unrun. Source deletion cascades
to derived jobs, while existing legal holds protect source/result retention.
This is the deterministic comparison stage, not model-written explanation.
Summary/requirements candidate processing is now connected as described above.
Final supported synthesis and private-attachment analysis integration remain
open; this does not close 9.2.

Synthesis-core progress (2026-11-13): immutable source batches now preserve full
unit traversal, Unicode byte identity, linked exceptions and native cell metadata.
A pinned local-provider protocol counts the exact prompt and generates under one
lease, with current-access guards and no automatic replay. Strict model candidate
validation binds unique source quotations and conservatively rejects unsupported
numeric tokens. Model inspection/no-findings reports remain unverified; candidate
sections are not published answers. The shared link index now prunes unrelated
blocks and has deterministic equal-span ordering. Durable synthesis persistence,
actual service runtime integration and public candidate controls are now
implemented in the connected increment above. Support assessment and final
synthesis remain required. Evidence:
`docs/quality/evidence/document-synthesis-core-2026-11-13.md`.
Evidence: `docs/quality/evidence/document-comparison-jobs-2026-11-13.md`.

Comparison export increment (2026-11-13): finished comparisons now export JSON,
CSV and Markdown through the API, app and CLI, with authored Python parity.
Artifacts retain pinned references, full changed text/cells, source units and
coverage gaps. Immutable graphs and evidence hashes are verified before delivery;
limits fail without a partial file. Access checks and content-free preparation
audit events cover exports. CSV formula protection retains exact metadata and
Markdown source text is fenced. Downloaded copies have separate retention.
See `docs/quality/evidence/document-comparison-export-2026-11-13.md`; native
review and Python execution remain open, alongside the rest of 9.2.

Comparison client progress (2026-11-13): bounded owner-private history, API/CLI/Go
and authored Python parity, inline verified source previews and an app comparison
workspace are implemented. Setup, results and history have separate views; desktop
differences use independent before/after columns, with compact coverage and exact
source inspection. The shared structured reader opens pinned revisions read-only.
All 29 locales include the new controls; native approval remains pending. Validation
and presentation evidence: `docs/quality/evidence/document-comparison-ui-2026-11-13.md`.

## 10. Approved upgrade: reader and departmental workflow

Reader thumbnail increment (2026-11-14): the shared source reader now offers
four bounded original-page thumbnails at a time, with review indicators and
localized keyboard navigation. Source hashes, current access, byte/pixel limits
and sequential loading protect the private/library paths. Actual packaged native
and scanned PDF raster tests pass; all 29 language fixtures plus denial/legacy
cases pass. Screenshot review corrected an overlapping disclosure marker. This
closes the thumbnail slice, not the wider version/library task. Evidence:
`docs/quality/evidence/document-thumbnails-2026-11-14.md`.

- [ ] 10.1 Integrate the common source reader, outline/coverage map, corrections, compact library filters/pagination, versions and evidence exports; verify all 29 locales and accessibility fixtures.
- [ ] 10.2 Expose private collection/member APIs and clients, explicit publication destination, metadata, source lineage, derived retention and audit events; verify all access/revocation paths.

10.1 follow-up (2026-11-14): reader page, zoom, search, mode and vertical reading
position now survive closing and app navigation in account/workspace-scoped
memory. Opening still revalidates source access and digest; no text/pixel cache
is reused. Image OCR regions use exact retained spans, original hash and all
eight EXIF coordinate transforms. Missing/ambiguous/corrected geometry is not
drawn. The broad reader run passed 128 cases with one timeout that passed in the
final isolated run; two real-service checks were skipped. Eleven final geometry/
navigation cases and affected Go preview suites pass. Correction drafts now also
survive navigation, wait for fresh source authority and retain an uncertain save
state before dispatch; changed extraction bases require explicit review/rebase.
All 39 final correction cases pass, including all 29 interface languages; the
final production build, generated API contract and source/documentation checks pass.
Thumbnails and the wider version/library scope remain open. See
`docs/quality/evidence/document-reader-position-2026-11-14.md`.

10.1 progress (2026-11-13): shared retained navigation now supplies bounded
heading/table/figure/review targets and per-unit block/review/figure counts.
Chat and Knowledge consume the same revision-pinned reader, with coverage and
outline navigation, correction-draft protection and original/text comparison.
Legacy text without a retained original remains available. Reader-wide persistent
state, page thumbnails, source-region highlights and complete version/export
workflows remain open; counts do not establish extraction accuracy.
Combined evidence: `docs/quality/evidence/document-reader-navigation-2026-11-13.md`.
The full affected Go suites, 262 renderer tests and 37 final layout/navigation
checks pass. Actual local embedding/OCR paths pass nine document cases, a private
collection case and private-image HTTP handling. Native review, model-answer
qualification and the remaining task scope are still open.

10.2 progress (2026-11-13): private-by-default collection creation, owner-only
membership updates, scoped list/policy reads and reviewed document moves are
exposed through API, Go/TypeScript clients and CLI; Python parity is authored but
unrun here. Moves pin both audience revisions and the document binding. A distinct
collection-import route prevents fallback to an older service's shared default;
accepted imports pin audience revisions through atomic publication. Collection
and movement audit events commit with their mutations. Policy migration snapshots
preserve prior bindings, owners and hashes. HTTP admin-isolation/revocation,
stale-audience publication, cancellation and client receipt tests pass. Chat now
offers collection selection, private creation and explicit audience acknowledgement;
changes of account, workspace or destination discard the review. All 29 language
fixtures pass, including RTL keyboard/zoom and ambiguous-outcome recovery cases.
Attachment publication without destination review returns HTTP 426; CLI and Python
require the reviewed collection/revision (Python tests remain unrun). The real
isolated service verifies exact copying and idempotency. See
`docs/quality/evidence/document-collection-review-2026-11-13.md`.
Ordinary Knowledge uploads now use the same audience review before transfer,
with batch-wide revision pinning and receipt checks. Current UI never falls back
to legacy shared uploads; cancellation/navigation stops undispatched files. CLI
and Python private file imports also reject wrong-revision acknowledgements.
The combined collection UI now includes owner-only membership editing with a
proposed-audience review, private creation, revision-pinned document moves,
collection filters and 30-document pagination. Failed mutations require inspection
and fresh review; unavailable directory entries never silently remove members.
The 182-test renderer run covers all 29 languages, narrow RTL/light/dark and
200% text layouts. Store and authenticated HTTP projections preserve binding
identity and exclude revoked collections. See
`docs/quality/evidence/document-collection-management-2026-11-13.md`.
Departmental metadata,
complete retention/audit/export coverage and pilot qualification remain required.

## 11. Approved upgrade: optional formats and qualification

- [ ] 11.1 Add source-linked visual interpretation and isolated legacy/ODF/email adapters after printed-document gates; keep handwriting experimental and remote fetches disabled.
- [ ] 11.2 Expand to 200 provenance-recorded documents and 1,000 human-labelled questions; evaluate per-language passage recall, supported claims, table/OCR accuracy and full-review coverage.
- [ ] 11.3 Qualify 8/16 GB profiles, offline packaged installations, recovery and a controlled departmental pilot; leave native review, unavailable hardware and unmet quality gates open.

## 1. Reproduce and contain current failures

- [x] 1.1 Add isolated parser regressions for missing PDF dependency, malformed PDF, unsupported regex paths, Unicode preservation, legacy XLS and XLSX inline strings; demonstrate the current failures with `go test ./internal/rag -run Parser -count=1` and record the baseline without personal documents.
- [x] 1.2 Remove PDF request panics, bound optional extractor execution/output and decompression, preserve valid Unicode and classify unsupported legacy formats accurately; pass the parser regressions and document remaining unqualified PDF/OCR behavior.
- [x] 1.3 Enforce the actual upload byte limit, multipart cleanup and typed client errors; pass server upload tests including oversize, corruption, cancellation and no ready record after failure.
- [x] 1.4 Repair spreadsheet inline strings, named-sheet/cell fidelity and slide ordering with real Office fixtures; pass parser tests and document unsupported formulas/content explicitly.

## 1A. Conversation files in UI and CLI (user-requested extension)

- [x] 1A.1 Add scoped original/attachment storage and bounded multipart/list/read/remove APIs with digest-bound extraction states; test cross-user references, traversal, corrupt uploads, cancellation, quotas and source deletion without private-content logging.
- [x] 1A.2 Bind attachment IDs/revisions into durable session turns and source context with context admission, source fidelity and no tool authority; test replay conflicts, unavailable extraction, reload/history and concurrent removal.
- [x] 1A.3 Add shared web/desktop Add files/drop controls, readable chips and progress/recovery/removal, preserving images/voice/drafts and explicit optional Knowledge publication; test all locales, keyboard, narrow layouts and real service uploads.
- [x] 1A.4 Add ordinary CLI file-path flags and interactive attach/list/remove commands through authenticated APIs; preserve JSON/pipes, local path identity, failed-send drafts and legacy image behavior. Gate agent attachment execution until scoped context tests pass.
- [ ] 1A.5 Verify same-file questions through UI and CLI against frozen text/PDF/Office fixtures; run contract/client/privacy/regression tests and record actual packaged/runtime limitations.

## 2. Packaged extraction and source fidelity

- [x] 2.1 Freeze at least 30 real document fixtures with expected text, source units, failure classes and license/provenance; include compressed/font-mapped PDFs, Unicode scripts, scans/mixed PDFs, corrupt/encrypted documents, Office/text and decompression bombs. Record corpus scoring before choosing the pinned packaged PDF engine.
- [x] 2.2 Implement typed format/readiness and source-unit contracts plus a pinned local PDF adapter/worker with timeout, cancellation, resource limits and no PATH dependency for advertised support; pass real-PDF corpus and worker crash/isolation tests.
- [ ] 2.3 Bundle and verify parser runtime assets/redistribution notices for Windows, macOS, Linux and containers; prove extraction on clean packaged installs without developer tools. Mark unavailable hardware evidence separately.
- [x] 2.4 Add local raster/OCR adapter and explicit language-pack readiness/offline import; pass scan/mixed/rotation/low-confidence fixtures, missing-language and cancellation tests without network or duplicate text.
- [x] 2.5 Add dedicated legacy Office/ODF/email adapters where qualified and explicit unavailable states otherwise; publish a precise format matrix backed by fixture scores, not filename acceptance.

## 3. Durable import and citations

- [x] 3.1 Add versioned original-source/import storage with stable scoped identity, quotas and migrations preserving legacy extracted text; pass rollback, collision, full-disk and missing-original tests.
- [x] 3.2 Persist import stages, retries, cancellation and atomic index replacement; fault-test crashes at acceptance/extraction/index/publication and prove no ghost-ready records or duplicate imports.
- [x] 3.3 Chunk structured source units and return stable page/slide/sheet/cell locators with content revision; verify exact source readback and citations against the frozen corpus, including partial OCR.
- [x] 3.4 Expose additive import/readiness APIs and authenticated CLI/Python projections; pass generated contracts and compatibility tests, and update API/user documentation.

## 4. Organization and resource boundaries

- [ ] 4.1 Implement collection/document policy applied to ingestion, retrieval, source/export and scoped caches; preserve legacy workspace policy through an explicit migration and pass cross-user/collection/revocation/deduplication tests.
- [x] 4.2 Implement documented retention/deletion across originals, text, vectors, jobs and caches with explicit backup/hold behavior; test restart, stale-worker resurrection, concurrent delete and export denial.
- [ ] 4.3 Bound extraction/index scheduling and expose useful progress on low-memory/offline systems without changing existing model/index identity; measure warm/cold import, cancellation and concurrent chat behavior.

## 5. Integrated acceptance

- [ ] 5.1 Integrate the defaults-first Knowledge workflow with batch import, partial/failed states, source preview and Ask; pass real-service renderer tests for first use, returning work, failed/retried import, scope and nine locales.
- [ ] 5.2 Run Go/race, API/web, desktop, CLI/Python and packaged parser checks from current CI; publish per-format/platform corpus results and retrieval/citation quality evidence. Retain all failures and unavailable manual/hardware checks.
## 6. Approved document workspace correction

- [x] 6.1 Correct PDF form traversal with bounded nested-form, raster, empty, malformed and cancellation regressions; run the PDF worker suite and inspect the reported 37-page PDF privately. Preserve scan protections and record actual extracted/partial states.
- [x] 6.2 Expose scoped digest-bound preview units and bounded PDF page rendering for attachments and Knowledge; retain partial text, explicit text-only recovery and CLI state/source parity. Run sessions/server/rag tests covering access, cancellation, stale revisions and no implicit publication.
- [x] 6.3 Update API/client/document guidance and validate contracts plus same-source UI/CLI fixtures. Preserve earlier failed evidence and unverified enterprise/hardware gates.

Evidence for this increment: [document workspace verification](../../../docs/quality/evidence/document-workspace-2026-11-03.md). Source and automated checks only; installed/native, model-answer and human qualification remain separate.

- [x] 6.4 Admit substantial native PDF text with explicit persisted visual limits, without automatic OCR or a repeating retry gate; retain scan/header/geometry protections, page-level visual links, CLI warnings and context restrictions. Verify real supplied PDF upload/retry plus scan and mixed-document regressions.

- [x] 6.5 Implement bounded exact source selection for oversized ordinary-Chat attachments, deduplicate historical references, persist immutable excerpt ranges and disclose limited coverage in web/CLI. Verify late evidence/Unicode, history preservation, source budget failures, forged seed rejection and the supplied PDF with a real local model at the unchanged 8,192-token allocation. This is a focused context regression, not full document-answer qualification.

Follow-up regression for 6.5: tokenizer-measured selection under the generation
lease now passes six real-model PDF questions at 8,192 context / 4,096 maximum
response tokens, plus eight deterministic turns without source re-upload or
history changes. See the evidence above; this does not close 1A.5 or 5.2.

8.1 completion (2026-11-14): private corrections now join the existing Knowledge
revision path. Old citations, current and historical reader modes, exact Chat
admission, revision-preserving publication and atomic real embedding/indexing
pass. The final reader and publication suites cover all 29 languages. Python
parity is now executed (144 passed). See
`docs/quality/evidence/document-private-corrections-2026-11-14.md`. This closes
the structured revision implementation, not parser fidelity, advanced packs,
complete workflows or release qualification. An unrelated full CLI EOF failure
is retained in that report.

Native-evidence and audit increment (2026-11-14): native PDF AcroForm field/state
metadata and tracked Word alternatives now reach retained structure, the shared
reader and real HTTP/Chat/Knowledge evidence paths. Correction, hold, retention
and deletion events commit atomically and survive source deletion. Injected audit
failures roll back source mutations. All-language reader fixtures and the affected
backend suites pass. See `docs/quality/evidence/document-native-evidence-2026-11-14.md`.
Tasks 8.2 and 10.2 remain open for their remaining implementation and qualification;
these additions do not close the seven-phase program.

Reader presentation coordination (2026-11-14): workspace-outcome-experience 40
refines the existing Chat/Knowledge reader, revision review, source actions and
preparation state. It preserves the extraction/access contracts and does not
replace this change's remaining backend or qualification tasks. Evidence:
`docs/quality/evidence/knowledge-reader-ui-audit-2026-11-14.md`.

Canonical source coordination (2026-11-14): workspace-outcome-experience 43.8 uses the existing immutable attachment store and structured extraction for explicit page selection. Page selection must preserve original offsets and linked evidence, reject unavailable pages, and never broaden silently to the whole file. Backend admission remains owned by agent-conversation-workflows 15.

Library scope completion (2026-11-15): the shared reader now submits immutable extraction revision plus page or block IDs through the same canonical inbox. `KnowledgeView.SelectEvidence` uses retained native spans, expands linked headers/footnotes, bounds context, and enforces current collection access before and after retrieval. HTTP admission/preparation/restart, correction-to-new-revision, old citation opening and revocation tests pass. Precise selection works without embedding search. Reader scope/draft reload tests and generated API/CLI/Python parity pass. The digest-pinned synthetic native PDF form fixture is explicitly included in source control. Evidence remains in the shared work-surface report; broader parser/model qualification gates above are unchanged.
