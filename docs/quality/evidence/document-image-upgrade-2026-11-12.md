# Document evidence and image admission repair — 2026-11-12

## Scope and status

This is the first implementation increment of the approved document/Knowledge
upgrade, plus the user's follow-up image audit. It is **not completion of the
seven-phase plan** and is not a public-sector qualification or release sign-off.
The authoritative work items remain in `document-ingestion-reliability`, with
reader/library coordination in `workspace-outcome-experience`.

No installed workspace was migrated, no application was installed or published,
and no live computer-control demonstration was run. Tests used disposable state.
Existing local model weights were read for an opt-in embedding test; no model or
dependency was downloaded for that test.

## Implemented behavior

- DOCX visible-body footnote/endnote references include their note text beside
  the reference. Deleted/moved-away references are excluded; missing referenced
  notes fail explicitly. Strict/transitional namespaces, alternative namespace
  prefixes, note ID zero and tracked-change disclosure have regressions.
- Shared Unicode lexical segmentation preserves short terms/numbers and adds
  character unigrams/bigrams for unspaced scripts. Literal OR queries and quoted
  phrases replace direct FTS syntax submission. Query processing is capped at
  16 KiB, 64 distinct terms and eight quoted phrases; source text is unchanged.
- Large Chat attachments prioritize matched excerpts before orientation text.
  The reproduced Chinese evidence in the middle of a roughly 528 KB attachment
  now reaches the model request and survives follow-up/history handling.
- A failed lexical/semantic component fails retrieval explicitly. An ordinary
  empty match set is distinct. No semantic-only fallback is labelled hybrid.
- Extraction receipts are separate from ready/index status and survive imports,
  attachment records, previews, saved Knowledge passages, CLI and model context.
  Attachment revisions bind their coverage receipt; changing/removing it is
  rejected. Native text can be readable while the same page needs visual review.
- Complete receipts retain page warnings. Model-facing receipts summarize
  repeated warning prose, disclose the complete warning count and preserve review
  unit identities. They do not turn selected passages into exhaustive review.
- The UI presents generic and format-specific limitations in all 29 locales.
  New translations are authored drafts; no native-review approval is asserted.
- SQLite search uses derived multilingual terms and versioned binary vectors,
  streams vector scoring with a bounded top-20 heap, fuses up to 20 candidates
  per channel, and loads document bodies/metadata only for shortlisted candidates.
  Compatibility JSON is retained; a trigger invalidates stale binary projections
  after compatibility writes. Ranking scores are explicitly not confidence.
- Existing databases receive a checked pre-migration SQLite backup preserving
  FTS row identities. New empty databases do not. Backup retention is a separate
  domain; live source deletion does not erase backups. Import coverage is cleared
  with live source deletion, under the existing hold/access checks.
- The retrieval evaluator now accepts exact evidence spans, source digest,
  extraction revision, content hash and evidence role, with per-language results.
  Returning the right document without its labelled exception fails evaluation.

## Image audit: findings and repairs

| Finding | Evidence and disposition |
| --- | --- |
| Generic Chat accepted incomplete pixels | A valid PNG signature/IHDR without a pixel stream passed `ValidateChatMessages`. The regression failed before repair. Shared validation now bounds dimensions and decodes actual pixels for generic and saved Chat. |
| Browser decoded before checking dimensions | An intercepted decoder was invoked for an oversized PNG header. The regression failed before repair. PNG/JPEG frame headers are now checked before bitmap allocation, followed by full browser decoding. |
| Multiple compressed images decoded concurrently | `Promise.all` was used for up to four images. Admission now decodes sequentially, preserving exact original bytes and all-or-nothing draft publication. |
| Projector presence checks differed by path | Generic Chat checked a nonempty path; saved Chat checked a regular file. Both now require an existing nonempty regular projector before inference admission. This does not certify projector/model visual accuracy. |
| Image Chat is separate from document OCR | Standalone PNG/JPEG/TIFF document ingestion remains unavailable in the format matrix. The existing OCR worker is still a separate printed-PDF path with an English-only pinned language pack. No image-document or multilingual OCR support was newly advertised. |
| Embedded visual content remains unread | Office images/charts and PDF figures are not made understood by successful text extraction. Receipts and localized limitations disclose this. Region extraction and qualified visual interpretation remain open. |
| Storage and practical upload constraints | Chat currently accepts four PNG/JPEG files per turn, 2 MiB each, 8,192 pixels per edge and 32 Mi pixels; saved image bytes share a 12 MiB conversation budget. Browser drafts use local storage with an explicit unsaved warning on quota failure. File-backed assets and larger photo/document workflows remain future work. |
| Orientation/animation/transparency | No comprehensive end-to-end EXIF, alpha-compositing or animated-image interpretation qualification was performed. Browser previews and model decoding must be compared before claiming equivalence. No orientation or animation support claim was added. |
| Visual answer reliability | No actual vision-model answers were qualified in this increment. OCR text, chart values, tiny text, tables, handwriting, signatures and stamps require separate labelled evidence and review. |

### Image follow-up acceptance work

1. Persist immutable image originals as scoped assets, with content hashes and
   explicit derived-image transforms rather than browser-storage-sized payloads.
2. Define orientation, color/alpha and animation handling. Preserve originals;
   disclose any normalized model input and verify preview/model equivalence.
3. Connect image documents to the structured extraction/OCR pipeline with page
   and region provenance, explicit language packs and uncertain-text review.
4. Qualify effective model resolution, crop/tiling behavior, visual token budget
   and memory on the supported hardware/model pairs. Never silently shrink away
   small labels or treat a projector's presence as visual understanding.
5. Benchmark diagrams/charts and numeric extraction against labelled regions;
   keep model descriptions distinct from native/OCR facts and uncertain values.

## Verification record

Raw local logs are under `build/` and intentionally not committed as product
source. Preserve the before-repair failure logs when reproducing this evidence.

| Check | Result / local record |
| --- | --- |
| Full source Go suite (`internal/...`, `pkg/...`, `cmd/...`) | Initial run failed on a new snapshot-test setup and an executor shutdown timing failure. `document-upgrade-go-source.log`. Other packages passed. |
| Snapshot-test correction | The legacy fixture must remove the new invalidation trigger before dropping its referenced column. Full rag/sessions/textsearch rerun passed: `document-upgrade-final-core.log`. |
| Executor isolated retry | Passed, 6.958 s: `document-upgrade-executor-recheck.log`. Original failure retained; no executor implementation changed. |
| Receipt/context/access regressions | document/rag/sessions/server passed: `document-upgrade-receipts.log`. |
| HTTP Chat, saved Knowledge evidence, locators and identity | Passed: `document-upgrade-final-http.log`. Model response is a transport fixture in the deterministic Chat test. |
| Actual local embedding + HTTP upload/search/source | All three DOCX/punctuation/Chinese cases passed: `document-upgrade-real-embedding.log`. No generated-answer accuracy claim. |
| Packaged PDF worker and parser suite | Passed in 25.350 s: `document-upgrade-packaged-parser.log`. Opt-in OCR/native scheduling checks without their required assets/profile were skipped. This is not a clean-install test. |
| Document reader/Knowledge UI | 86 passed, 3 opt-in checks skipped: `document-upgrade-ui.log`. |
| Final format-warning UI across 29 locales | 29 passed: `document-upgrade-ui-limits.log`. |
| Partial-import recovery UI | 9 passed, 1 real-service opt-in check skipped: `document-upgrade-partial-ui.log`. |
| Image API reproduction before repair | Failed as intended: `image-audit-api-before.log`. |
| Image allocation reproduction before repair | Failed as intended: `image-audit-ui-before.log`. No oversized bitmap was allocated; the decoder was intercepted. |
| Image API/server/CLI/session regressions after repair | Passed: `image-audit-go-after.log`. |
| Image upload/paste/drop/reload/error/preview UI | 22 passed: `image-audit-ui-after.log`. |
| Final expanded image locale/serialization checks | 30 passed (all 29 locale controls plus sequential-decoding/byte-preservation test): `image-audit-locales.log`. |
| Final affected Go packages after image changes | All seven passed: pkg/api, server, cli, sessions, rag, document, textsearch. `document-image-upgrade-final-go.log`. |
| Renderer/TypeScript/generated API | Build and API check passed: `document-image-final-web.log`, `document-image-final-api.log`. Existing bundle-size warnings remain. |
| Desktop unit/package-contract tests | 206 passed, 1 skipped: `document-upgrade-desktop.log`. No native window/install test was run. |
| Translations | 1,360 authored messages per locale, zero missing; consistency check has zero shared-navigation conflicts. Native wording/context review remains pending. |
| Translator packs | `build/document-upgrade-translator-review-20261112/`: 29 JSON packs, glossary, source/runtime-copy audit and review instructions. |
| Race detector | Not run successfully: Go reported CGO required; this environment has CGO disabled and no configured C compiler. `document-upgrade-race.log`. No compiler was installed. |
| OpenSpec/source-size checks | Strict change validation and source-size ratchet passed. |

The first `go test ./...` attempt was stopped during traversal of the large local
build-artifact tree; the explicit source package families above were used instead.

### Real embedding test provenance

- Windows, Intel Core 7 240H, 10 cores / 16 logical processors, 15.71 GiB RAM.
- Existing BGE-M3 GGUF, 437,778,496 bytes; SHA-256
  `40ee48886d763df90be25e1a4d34f1a51982be7a3a3002ed649e5df6f3f48329`.
- Existing portable CPU llama-server executable; SHA-256
  `9032f1a3d30a57268ce99c297024d1190db3c44b5adca4c5a5ae66ebcdb6eeba`.
- `TestRealKnowledgeEvidenceHTTP` uses a fresh index/cache, authenticated runtime
  ownership and three synthetic source fixtures. It completed in 9.10 s including
  startup. This is not retrieval p95, 100,000-chunk qualification or a vision test.

## Still open

Complete structured correction workflows and linked-context expansion;
re-extraction of old documents while preserving old citations; richer PDF/Office
structure; standalone image/TIFF ingestion; multilingual OCR pack selection;
packaged offline Docling; durable summary/requirements/comparison APIs; reader
coverage maps and corrections; departmental collection/member APIs and clients;
broader retention/export/version workflows; optional visual/archive adapters.

The 200-document/1,000-question human-labelled corpus, language-specific review,
OCR/answer accuracy targets, 8/16 GB scale gates, clean offline installations,
cross-platform/race checks and controlled departmental pilot remain open. The
three regressions and authored translations do not satisfy those release gates.

Existing extraction records are intentionally not rewritten. In particular,
reindexing already-extracted DOCX text cannot restore notes the old parser omitted.
That needs the structured-revision migration, not deletion of users' old evidence.

## Structured foundation follow-up

Implemented the shared immutable extraction graph with exact UTF-8 spans,
source/runtime identities, typed blocks, explicit relationships and validated
geometry units. Byte offsets are never advertised as rendered page coordinates.
DOCX extraction now records declared heading levels, paragraphs, tables, rows,
cells and clause/footnote links. This does not yet add inferred table headers,
custom styles, header/footer parts, tracked-change alternatives or rich PDF layout.

SQLite publication retains extraction revisions, normalized blocks and
relationships in the same transaction as the active index. Legacy text is archived
lazily on its first replacement; opening the application does not re-extract old
files. Historical HTTP citations resolve their original exact span, source digest,
parser revision and text hash. Current collection membership is checked before
and after reads. Authorized deletion cascades to history; pre-upgrade snapshots
remain a separate retention domain. A separate 2 GiB extraction-history quota
rolls back publication on overflow; existing original/import quotas are unchanged.

The shared correction model checks expected text hashes, records author/time and
parent revision, adjusts enclosing spans and refuses edits that erase linked
child evidence. Corrected numeric text is not represented as a recalculated typed
cell. Engine publication compares the expected active revision, reuses vectors for
unchanged exact text only under the same embedding identity, and checks current
write permission in the publication transaction. A revocation-during-embedding
test leaves both the active source and history unchanged. At that checkpoint this
was engine-only; the following increment adds supported HTTP/client/reader paths.

Checks completed for this follow-up:

- `build/structured-document-check.log`: rag/document/server suites passed.
- `build/structured-document-rag.log`: initial quota integration broke two
  existing original-quota tests; retained as failure evidence. Extraction history
  now has its own explicitly bounded quota, preserving original-limit semantics.
- `build/structured-document-rag-final.log`: full rag/document suites passed.
- `build/structured-document-final.log`: rag, document, server and sessions passed
  (20.600 s, 1.602 s, 112.849 s and 9.652 s respectively).
- `build/structured-document-parser-final.log`: parser/graph/DOCX/migration
  regressions passed after empty-unit accounting and parent validation changes.
- Additional shared correction tests passed after ancestor-state handling changed.
- Engine correction tests passed for unchanged-vector reuse, historical citation
  readback, stale-edit rejection, reader denial and mid-flight writer revocation.
- Final full document/rag suites passed after correction publication changes:
  `build/structured-document-corrections-final.log` (1.544 s / 20.419 s).
- Generated API freshness and TypeScript checks passed after adding the optional
  document extraction revision field. No renderer layout changed in this follow-up.
- Historical citation/revocation HTTP regressions passed; generated API check,
  strict OpenSpec validation and source-size checks passed.

No live Computer Use, installation, publication, dependency download or installed
user-data migration was performed. The broader gates above remain open.

## Supported review APIs and reader controls

Added scoped `GET /v1/documents/structure` and revision-checked
`POST /v1/documents/corrections`, advertised as `document-structure-review-v1`.
Read responses bound text and optional metadata, with explicit block and
relationship cursors. Permission checks include global Knowledge read/manage
permissions and collection membership; global management does not grant private
collection access. Attribution comes from the authenticated request, not its body.
Malformed envelopes, forged authors and stale revisions fail without publication.

CLI and Go clients now expose `kb structure` and `kb correct REVIEW.json`.
Python has matching `kb.structure` and `kb.correct` methods and fixture tests.
Generated TypeScript contracts and web client receipt validation cover the same
endpoints. No client automatically retries a correction. Python tests remain
unrun because this environment has no usable Python interpreter; Store aliases
are not evidence of an installed interpreter.

The Knowledge source dialog and Chat citation reader share a text-review panel.
Historical citations pass their retained extraction identity, rather than silently
opening the active text. Pagination and unit filtering stay revision-pinned.
Truncated blocks, incomplete metadata, historical revisions and readers without
write access do not offer editing. Unknown write outcomes preserve the draft,
disable another save, and require latest-text inspection and an explicit new-base
choice. If the latest text already equals the draft, the panel says so without
claiming that the uncertain request was confirmed. View switches retain the draft.

This is not the complete Phase 5 reader: full original/structure/side-by-side
unification, relationship navigation, outlines/coverage maps and draft retention
across closing/navigation remain open. The panel displays bounded text blocks;
it does not claim to expose the entire structured graph or all review annotations.
Legacy documents without retained structured revisions remain explicitly
unavailable. Original-file re-extraction remains a separate unfinished workflow.

Checks for this increment:

- `build/document-structure-http-final.log`: document/rag/sessions/API packages
  passed. Server failed only on the frozen route inventory's old 153-route count.
  The reviewed fixture adds precisely the read and manager-guarded write routes;
  the full server rerun passed (59.316 s) in
  `build/document-structure-server-final.log`.
- `build/document-structure-real-http.log`: real packaged llama-server/BGE-M3
  embedding test passed (8.164 s package). A DOCX footnote correction publishes a
  new revision, old HTTP citations still return July, the new revision contains
  August, original bytes match, and replaying the old revision returns 409.
  The punctuation and large-Chinese-attachment cases also pass. This tests actual
  local embeddings and HTTP integration, not model-answer quality certification.
- `build/document-structure-clients-final.log`: CLI/serviceclient suites passed.
- `build/document-review-ui.log`: 37 headless tests passed, including all 29
  locales at 390 px, draft retention across reader-view changes, confirmed saves,
  unknown outcomes, stale receipts, changed permissions and pinned pagination.
- `build/document-review-visual.log`: three additional tests passed for historical
  Chat citation handoff and enlarged English/Arabic keyboard layouts. Screenshots
  were inspected; text wrapping and RTL controls remained readable. These are
  mocked UI tests with CSS zoom, not native screen-reader or installed-app evidence.
- `build/document-review-final-focused.log`: six affected recovery/enlarged-layout
  tests passed again after clarifying the unavailable-state copy and isolating RTL
  count order. `build/document-review-build-final.log` records the final production
  build after those copy/layout changes.
- `build/document-review-reader-regression.log`: 76 existing reader/original/
  citation tests passed, two existing fixture-dependent skips.
- Generated API freshness, TypeScript and renderer production build passed.
  The existing bundle-size advisory remains; it is not a memory qualification.
- `build/document-review-i18n-final-pass.log`: catalog, encoding and contextual
  UI-copy checks passed. Every language has 1,378 authored catalog messages,
  zero missing. Added exact source tracking and narrow allowances for internal
  image/receipt diagnostics whose callers render localized errors. No native
  reviews were recorded or fabricated. Updated 29 translator packs are in
  `build/document-upgrade-translator-review-20261112/`.
- Strict OpenSpec validation and whitespace checks passed. The repository-wide
  source-size check currently flags `internal/agents/coordination.go` and
  `internal/agents/manager.go`, outside this document-review increment. Their
  concurrent changes were preserved; their size baselines were not relaxed.

No installation, publication, live Computer Use or automatic dependency download
was performed. Full adapter, image-document OCR, analysis-job, departmental,
corpus, hardware and linguistic qualification gates remain open.

## Linked structural evidence increment

Knowledge retrieval now expands explicit retained heading, repeating-table-header,
caption and footnote relationships into separate exact, citable source spans.
DOCX revision `offgrid-docx-structure-v4` records explicitly marked repeating
headers within their own table; it does not infer that every first row is a header.
Old extractions and configured embedding identities remain unchanged.

Expansion is bounded to 16 passages and 64 KiB added text per anchor, with 256 KiB
added text per search and two concurrent graph readers. Busy or corrupt graph
reads fail explicitly. Receipts retain revision/block identities, required chunks
and known omitted blocks. Removing or truncating a required chunk makes the saved
receipt incomplete and adds a model-context restriction and localized UI notice.
Model context identifies linked citation numbers instead of duplicating the full
receipt. This completeness flag covers declared links only, not full-document
coverage, undeclared relationships, answer correctness or native text accuracy.

Checks and retained failures:

- `build/linked-evidence-core.log`: full rag, document, sessions and API packages
  passed. Earlier regression fixtures omitted index publication identity and
  incorrectly assumed a 16-byte budget could not fit a 14-byte heading. Those
  failures remain in `linked-evidence-regressions*.log`; corrected fixtures check
  actual byte bounds, omission receipts and exact source spans.
- `build/linked-evidence-real-http.log`: real existing BGE-M3 and packaged
  llama-server passed the DOCX, punctuation, Chinese and distant-header HTTP cases
  (4.355 s package). A numeric row retrieves its remote marked header, and source
  GET returns that exact header. Historical correction citations still read the
  old text, originals remain unchanged and stale correction writes conflict.
  This is retrieval/integration evidence, not generated-answer qualification.
- `build/linked-evidence-server-final.log`: full server (70.806 s) and rag
  (13.517 s) suites passed.
- `build/linked-evidence-context-final.log`: focused linked-evidence, source and
  context tests passed (4.859 s), including truncation changing the receipt.
- `build/linked-evidence-receipts-final.log`: rag and sessions receipt regressions
  passed, including durable saved-answer replay and omitted-block identities.
- `build/linked-evidence-ui-final.log`: 29 locale tests passed (1.4 min), showing
  the incomplete-link notice at narrow width and hiding it for complete receipts.
  The initial UI run clicked a conversation selector hidden by the narrow layout
  after the conversation had already loaded; its interrupted failure is retained
  in `linked-evidence-ui.log`. The corrected test inspects the loaded receipt.
- API generation/freshness, TypeScript, production renderer build, all-language
  catalog checks and strict OpenSpec validation passed. The catalog now contains
  1,379 authored messages per language, zero missing. Translator review packs were
  refreshed; native review is still pending. The earlier unrelated source-size
  failures were subsequently resolved without weakening their baseline.
- `build/linked-evidence-capability-check.log`: capability/server regression
  passed (1.568 s). Capability identity includes `document-linked-evidence-v1`.

Private attachment structured expansion, richer format structure, scale tests,
complete reader workflows and the other approved phases remain open. No live
Computer Use, installation, publication or dependency acquisition was performed.

## Still-image orientation and preview fidelity (2026-11-12/13)

The image audit found that preview orientation and model-input orientation were
not governed by one contract. Added bounded primary Exif parsing and all eight
rotation/mirror transforms. The llama HTTP serializer uses an inference-only PNG
projection for non-default orientation, shared by token counting, completions,
streaming and the single-step tool envelope. It does not resize images or perform
another lossy JPEG encoding. Original uploads, saved history and durable request
identities retain their exact bytes. Projection output is capped at 16 MiB per
image and 32 MiB aggregate image bytes; oversize fails before runtime dispatch.

Animated PNGs, multi-picture JPEGs and duplicate/invalid Exif directories are
rejected rather than silently taking one frame or guessing orientation. Browser
preflight does this before bitmap allocation and retains previous files and text.
The recovery message is authored in all 29 languages. Services advertise
`chat-image-orientation-v1`; the composer blocks oriented drafts and saved-image
follow-ups on an older service, while ordinary unrotated images stay compatible.

A further pixel-level browser regression reproduced Chromium ignoring PNG Exif
after IDAT. The corrected thumbnail/full-size display path relocates the unchanged
Exif chunk before image data in a display-only copy. Compressed pixel data is not
re-encoded, and this copy never replaces the stored upload or model-input source.
The service already reads the late metadata. The first failure remains recorded
in `build/image-orientation-late-metadata.log`.

Format references used for the implementation are the
[W3C PNG specification](https://www.w3.org/TR/png-3/) and the
[CIPA Exif specification](https://cipa.jp/std/documents/e/DC-X008-Translation-2019-E.pdf).
Source-format semantics are not evidence of browser/runtime or model quality;
the results below are the actual checks performed.

- `build/image-orientation-backend.log`: API, image metadata/transform, inference
  and sessions suites passed. `build/image-orientation-codec-final.log` confirms
  final codec/API tests after sharing the decode and adding late-PNG fixtures
  (2.893 s / 2.686 s). Both TIFF byte orders, all transforms, exact source
  preservation, 16-bit values, invalid offsets/counts/types, animation and
  duplicate metadata are covered.
- `build/image-orientation-runtime-focused.log`: HTTP token-count, completion,
  streaming and single-step projection regressions passed (0.771 s). These use
  an isolated HTTP fixture inspecting actual encoded pixel requests; they do not
  constitute a real vision-model answer benchmark.
- `build/image-orientation-service-focused.log`: affected server image, vision
  and identity regressions passed (6.455 s), including the advertised capability.
- `build/image-metadata-fuzz.log`: five-second bounded fuzz run passed, 215,351
  executions. This is additional parser robustness evidence, not certification.
- `build/image-orientation-ui.log`: initial 46 image tests passed (3.1 min).
  `build/image-orientation-ui-verified.log`: 47 passed after the older-service
  guard (3.9 min), including all 29 locales, narrow layouts, original-byte
  persistence, failed-send recovery and keyboard image inspection.
- `build/image-orientation-history-final.log`: six targeted tests passed
  (37.9 s) after moving image-history derivation into its owning hook, including
  older-service saved-history blocking, PNG/JPEG orientations and send/reload.
- `build/image-orientation-late-metadata-fixed.log`: four final tests passed
  (41.4 s), checking every orientation for JPEG, early-Exif PNG and late-Exif PNG,
  unchanged draft bytes, thumbnail/full-size agreement and inspection controls.
- Earlier failures are retained: `image-orientation-initial.log` rejected an
  old fake `base64,fixture` image; its test now uses actual PNG pixels.
  `image-orientation-ui-final.log` caught a missing React import during the
  older-service integration. That run was stopped, corrected and rerun; no
  failed run is counted as passing evidence.
- TypeScript, production build and UI-copy checks passed. Catalog checks report
  1,380 authored messages per language, zero missing. Translator packs were
  refreshed; native review remains pending. No linguistic approval was invented.
- Full server runs in `image-orientation-final.log` and
  `image-orientation-server-verified.log` failed on the concurrently expanded
  route inventory (161 actual versus its frozen 155-route expectation).
  Those unrelated routes/fixtures were not overwritten or relaxed. This is not
  a clean full-suite result despite the affected image tests passing.
- Race tests could not run: `image-orientation-race.log` records CGO required;
  this Windows environment has CGO disabled and no configured C compiler.
- The source-size check no longer flags ChatPage after image-history derivation
  moved into the image hook. It still flags concurrent unrelated
  `sdk/python/offgrid/client.py` at 767 lines against its 765-line baseline.
  That work and its baseline were preserved.

No installation, publication, live Computer Use or dependency download occurred.
This increment does not qualify colour profiles/HDR, alpha compositing across
vision runtimes, OCR accuracy, chart/diagram understanding or public-sector use.
Standalone PNG/JPEG/TIFF document OCR, multilingual packs, advanced structure,
document analyses, departmental workflows and the approved qualification gates
remain active work in the full plan.

## Private structured evidence and reader parity — 2026-11-13

Private Chat attachments now retain the parser's immutable structured graph in a
revision-bound sidecar. The original digest, parser, coverage and graph identity
are bound by the attachment source revision. Graph text is stored once in the
existing record and reconstructed/verified on read. Outer Unicode whitespace is
preserved with rebased exact spans. Ready sources cannot be replaced; missing,
modified or stripped graph references fail generation instead of silently
falling back to unstructured text. Interrupted derivatives count against a
separate owner quota and authorized deletion removes them. Cross-user graph
access, restart, immutable-ready records, corruption and cleanup are covered.

Chat and Knowledge now use the same explicit heading/header/note/caption link
traversal. Private selection ranks up to 20 lexical candidates, accounts for
metadata when choosing excerpt sizes and includes bounded exact dependency
spans. Unfittable links produce a persisted incomplete receipt and model
restriction, plus the existing localized notice in all 29 languages. A matching
passage that cannot fit is not silently replaced by an unrelated tail. The first
integration run exposed exactly that Chinese-evidence regression after adding
metadata; adaptive excerpt sizing fixed it. Initial failure evidence is retained
in `build/attachment-structure-initial.log`.

`GET /v1/sessions/{name}/attachments/{id}/structure` pins the attachment source
revision and returns a bounded, no-store, read-only `DocumentStructurePage`.
Knowledge and private attachment readers share the projection implementation.
Draft files, saved files and saved answer citations use the same web inspection
panel. Server and client checks reject stale sources, invalid pagination and
unexpected private correction rights. CLI inspection and authenticated Python
SDK parity are authored; Python tests remain unrun because no usable Python
runtime is configured here. The capability is `session-attachment-structure-v1`.

Verification evidence:

- `build/attachment-structure-linked.log`: complete document and session package
  suites passed (1.523 s / 9.936 s), including exact linked spans, omitted long
  exceptions, persisted receipts and Unicode-envelope immutability.
- `build/attachment-structure-verified.log`: focused document, sessions, rag,
  server, CLI and client checks passed. Real DOCX HTTP upload, generation-context
  assembly, saved selections and citation readback retain the distant exception.
  Model output is a transport fixture, not answer-quality evidence.
- `build/shared-structure-reader.log`: existing Knowledge structure regressions
  passed after extracting the common projection (1.444 s).
- `build/document-attachment-full.log`: complete package suites passed for
  document (4.985 s), sessions (12.687 s), rag (19.643 s), server (114.556 s), CLI
  (3.079 s), serviceclient (1.261 s), imageinput (3.195 s), inference (22.144 s)
  and API (3.093 s). The earlier unrelated route-inventory failure is no longer
  present in this run; concurrent work was preserved.
- `build/attachment-structure-clients.log`: subsequent private CLI/client and
  server checks passed, including authenticated revision/pagination requests and
  rejection of editable private receipts.
- `build/attachment-linked-ui.log`: 58 tests passed (6.5 min), checking the saved
  missing-link notice for both Knowledge and private attachments in every locale.
- `build/attachment-private-reader-ui.log`: three private-reader tests passed
  (18.5 s), covering English, Arabic and Simplified Chinese, narrow layouts,
  pinned pagination and read-only response enforcement.
- `build/attachment-shared-review-ui.log`: retained initial failure caught two
  overlapping live status announcements during correction reload. The panel now
  uses one loading/saved announcement. `build/attachment-shared-review-verified.log`
  passed all eight private/shared-reader tests (49.7 s), including uncertain-save
  recovery, stale receipts, historical citations and pinned pagination.
- TypeScript/production build passed in `build/attachment-structure-web-build.log`
  with the existing bundle-size advisory. API generation checks, OpenSpec strict
  validation, source-size/layout and diff whitespace checks passed. Final copy
  checks in `build/attachment-localization-check.log` passed with 1,380 messages
  per locale and zero missing; an internal diagnostic first flagged by the
  literal checker was moved into the shared response validator. No new catalog
  messages or linguistic approval were introduced.
- The post-fix build passed in `build/attachment-final-web-build.log`, but its
  API check detected types drifting behind concurrent schema edits. That failed
  check is retained and prompted regeneration. Focused server/CLI/client checks
  passed in `build/attachment-final-contracts.log`.
  Regeneration, API consistency, TypeScript and production build then passed in
  `build/attachment-final-api-refresh.log` (production build 3.39 s).
  Translation consistency reports zero shared-navigation conflicts in all 29
  locales; its contextual wording variants remain translator-review items.

Private derived corrections, side-by-side original/structure views, rich outlines,
coverage maps and navigation-wide drafts remain open with the full adapter,
analysis, collection and qualification phases. Race execution remains unavailable
under the current CGO/compiler setup. No installation, publication, live Computer
Use, model download or dependency installation occurred in this increment.
