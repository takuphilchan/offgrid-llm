# Multipage TIFF document implementation — 2026-11-14

Implementation under document-ingestion-reliability 8.3, coordinated with the
shared workspace reader. This does not close the document upgrade or qualify
TIFF/OCR for departmental production.

## Behavior

- Chat file attachments and Knowledge accept classic multipage TIFF through the
  existing local OCR worker. Main page directories, page identity and orientation
  are retained. Originals are never rewritten.
- Inspect directories before pixel allocation. Bound source files to 24 MiB,
  pages to 100, dimensions to 8192 per side, pixels to 16 million per page and
  200 million per document. Bound metadata entries, decoder reads and tile size;
  reject cycles, duplicate tags, invalid offsets, unsupported subimages/BigTIFF,
  planar samples and JPEG compression. Decode and OCR one page at a time.
- Store exact per-page text spans, printed-text coverage and original-pixel line
  coordinates. A failed/uncertain page cannot become a complete extraction merely
  because another page succeeded. Partial Knowledge imports remain review-only.
- Failed private extractions can retain a validated structured graph for review;
  ordinary generation still requires explicit partial-text acceptance. The graph
  remains bound to its original and text. Ready attachments stay immutable.
- Source previews use saved page spans, not separator guesses or fresh OCR.
  Page requests decode only the requested raster and recheck source access/hash.
  Readers share existing page navigation, source/text views and partial notices
  in all 29 interface languages. OCR language qualification remains separate.
- `document-tiff-ocr-v1` advertises the additive capability. Format readiness
  reports the current local worker/English-pack state and stays unqualified.
- Pin `golang.org/x/image` v0.45.0 with module checksums, preserving the project's
  Go 1.25 minimum. Its module requires x/sys v0.47.0 and x/text v0.41.0. The TIFF
  dependency includes the fixes for the published strip-offset and oversized-tile
  issues; see [Go's strip-offset advisory](https://pkg.go.dev/vuln/GO-2026-5066)
  and [tile-allocation advisory](https://pkg.go.dev/vuln/GO-2026-5062). Dependency
  notices are retained and wired into desktop, CLI bundle and container packaging.
  This maintainer dependency fetch does not add runtime downloads.

## Executed evidence

- `build/tiff-parser-tests.log`: directory/byte-order/page identity, malformed
  metadata, allocation bounds, cancellation, selected-page pixels, ordered OCR
  spans and partial coverage pass. Synthetic fixtures are authored in the repo.
- `build/tiff-connected-tests.log`: TIFF, image, attachment review and format
  readiness tests pass through parser/session/server packages. HTTP tests retain
  partial page identity across explicit acceptance and reject stale source hashes.
- `build/tiff-real-ocr.log`: actual packaged OCR reads a two-page, 7,480,284-byte
  TIFF made from the existing authored printed receipt. The second page carries
  orientation 6. Both retain “Scanned receipt total 42”, verified geometry and
  citation readback after SQLite restart. Run time was 21.66 seconds; this is a
  single fixture measurement, not p95 or memory-profile qualification.
- `build/tiff-real-http.log`: actual local BGE-M3/llama Knowledge upload, hybrid
  retrieval, citation readback, retained two-page preview and page-2 rendering
  pass. The TIFF case took 22.50 seconds after embedding startup. Actual private
  HTTP OCR, saved structure, source reopen and generation-context transport pass
  in 22.80 seconds. The generated answer is mocked; answer quality is not measured.
- `build/tiff-ui-tests.log`: all 29 locale variants pass page navigation,
  partial coverage, original/text pairing, source digest requests and narrow-layout
  overflow checks. No native linguistic approval is inferred.

- `build/tiff-full-go.log`: complete documentimage, RAG, sessions, server,
  service-client, CLI, API and image-input suites pass, run sequentially by package.
- `build/tiff-web-build.log`: production TypeScript/renderer build passes; the
  existing bundle-size advisory remains. Layout, source-size, documentation,
  literal checks and both strict OpenSpec validations pass.
- `build/tiff-document-pack-tests.log` and
  `build/tiff-packaging-source-tests.log`: three existing document-package checks
  and six container-source checks pass. These are source/fixture checks, not a
  newly built installer or container. Notice-copy wiring still needs actual
  clean-install verification alongside the wider packaging gate.
- Bounded fuzz smoke: the first 20-second run completed only nine executions;
  a count-bounded rerun completed 100 without a failure in
  `build/tiff-fuzz-count.log`. This is not a sustained security campaign.
- A visual rerun initially refused occupied fixture port 5197. It did not reuse
  or stop that service. The isolated retry passed on another port; the failed
  invocation remains in `build/tiff-ui-visual.log`.
- Final visual fixtures pass in English/light and Arabic/dark in
  `build/tiff-ui-visual-final.log`; both screenshots were inspected. Page images
  in this UI test are authored canvas fixtures, not real TIFF processing. Actual
  packaged OCR is covered separately above.
- A shared-worktree catalog check found 25 new task-changes messages without
  preview-language translations. All 20 preview packs now contain those messages;
  `build/reader-translations-check.log` and
  `build/reader-translations-complete.log` report 1,501/1,501 messages and zero
  missing entries. Refreshed review packs for all 29 languages are in
  `build/translator-review-20261114`. Native approval remains pending.

Real OCR assets used in these checks:

| Asset | SHA-256 |
| --- | --- |
| Authored printed-receipt PNG | `7352c87d8c9a18f364366ea22094b3534909adc19e4832e515e4709f791da843` |
| Verified English pack | `7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2` |
| Packaged OCR worker executable | `b6160606f59a60d3fc3ac35aaeede198c0f057f4ac8181ca25f1f032ce8753ec` |

## Remaining work

Deskew, additional digest-pinned OCR languages and explicit per-document selection,
crash-resumable page checkpoints, optional Docling layout/table processing and the
representative OCR corpus remain open. TIFF compression variants beyond the
uncompressed fixture need separate corpus qualification. No handwriting, figures,
charts, tables, signatures or colour-profile interpretation is claimed. Human
translation review, 8/16 GB performance, clean offline installation and departmental
pilot gates remain unverified. No application installation or publication occurred.
