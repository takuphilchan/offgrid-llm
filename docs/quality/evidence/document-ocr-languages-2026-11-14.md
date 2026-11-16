# OCR languages and page recovery — 2026-11-14

Implementation evidence for document-ingestion-reliability 8.3/8.4 and the shared
workspace controls. This is not OCR accuracy qualification or release approval.

## Implemented behavior

- One pinned manifest supplies 25 traineddata packs, digest/size/source identities
  and explicit mappings to 26 interface locales. Shona, Ndebele and Zulu are
  explicitly unavailable; no related language is substituted. English compatibility
  is retained. All advertised accuracy qualification fields remain unverified.
- Pack installation is explicit, atomic and verified. Worker processes receive
  the selected bytes through the existing isolated protocol. No runtime downloads,
  cloud service, user-installed Python or PATH OCR command is introduced.
- Chat and reviewed collection imports save one selected OCR language. Interface
  locale changes, retries and explicit Chat-to-Knowledge publication preserve that
  choice. Unsupported services and mismatched receipts fail before accepting a
  language-sensitive operation. The Chat catalog is authenticated and read-only;
  it does not confer pack-install permission.
- Validated OCR worker receipts are saved per page under the private source or
  import. Keys bind the raster bytes/dimensions, pack digest and worker revision.
  Loading revalidates evidence. Restart or cancellation does not discard completed
  page writes. Explicit retry performs only the remaining OCR work; native parsing
  and raster preparation may repeat. Uncertain OCR keeps its review state.
- Import page writes enforce current collection access, attempt and processing
  state. Successful publication removes temporary receipts atomically. Source
  removal cascades to receipts; stale attempts cannot resurrect them. Private
  records share source authority, deletion and owner storage quotas. Per-page and
  per-source bounds are 3 MiB and 32 MiB, with no cross-user receipt cache.
- Chat and Knowledge show saved-page progress and retry guidance in all 29
  interface languages. These are translation drafts, not native-language approval.

## Provenance

Language files come from the [pinned tessdata_fast source](https://github.com/tesseract-ocr/tessdata_fast/tree/87416418657359cb625c412a48b6e1d6d41c29bd).
Manifest digests, byte sizes and exact URLs live in
`internal/documentocr/languages.json`; its redistribution license is retained.
Maintainer fixture downloads went into ignored build storage, not installed-user
data. All 25 downloaded files matched their recorded digests and initialized the
actual WASM engine. This checks compatibility, not recognition accuracy.

Chinese, French and Swahili PNG fixtures were authored locally, with CC0 text and
rendering/hash provenance in `internal/rag/testdata/documents/ocr/languages/`.
They contain printed text and the critical number 42. They are not the planned
representative multilingual benchmark or native-reviewed expectations.

## Executed checks

- Actual isolated worker initialized all 25 packs: `build/ocr-language-worker-tests.log`.
- Actual Chinese/French/Swahili parser-to-source/citation checks passed:
  `build/ocr-language-real-parser.log`.
- Actual two-page French TIFF recovery passed for both SQLite imports and private
  attachments: cancel after page one, reopen the store, process only page two,
  then use both saved pages with a deliberately unlaunchable worker path. Graph
  identity remains unchanged: `build/ocr-checkpoint-runtime.log`.
- Complete affected Go suites passed: documentocr, rag, sessions, server,
  serviceclient, CLI and API (`build/ocr-checkpoint-full-go.log`). Regressions cover
  corruption, quotas, stale writes, deletion, restart and owner separation.
- Full Python SDK suite passed through local WSL Python: 134 tests
  (`build/ocr-language-python-full.log`). This supersedes earlier evidence notes
  that Python parity was authored but unexecuted; it does not rewrite those runs.
- Renderer TypeScript/production build, generated API consistency, translation
  checks, source-size ratchet and whitespace checks passed. The production build
  retains the existing large-chunk advisory.
- Initial combined OCR/upload UI run: 97 passed, one skipped real-service case,
  one stale menu assertion (expected three buttons after a fourth was added).
  Updated assertion retained all icon/width/keyboard checks; its isolated rerun
  passed in light English and narrow dark Arabic layouts.

- Final OCR picker/progress suite passed all 61 cases, including all 29 languages
  in both Chat and Knowledge (`build/ocr-checkpoint-ui.log`). English/light and
  Arabic/dark/narrow screenshots were inspected: translated recovery copy fits
  the available width and actions remain visible.
- Translator review packs were exported for all 29 languages to
  `build/ocr-page-recovery-review-20261114`. Catalog completeness does not mark
  these additions linguistically approved.

## Open boundaries

Simultaneous OCR packs, page-language hints, mixed-script quality, deskew,
Docling packaging/advanced-worker scheduling and representative OCR benchmarks
remain open. Existing cancel/retry resumes OCR; dedicated pause controls remain
part of the wider durable-job scope. Clean packaged/offline installation,
8/16 GB scale and latency gates, native translation review and the controlled
departmental pilot are not established by these fixtures. No installer was
published or installed in this increment.
