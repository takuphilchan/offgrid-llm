# Tasks

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
- [ ] 4.2 Implement documented retention/deletion across originals, text, vectors, jobs and caches with explicit backup/hold behavior; test restart, stale-worker resurrection, concurrent delete and export denial.
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
