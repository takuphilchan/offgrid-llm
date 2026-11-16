# Reader position and image evidence — 2026-11-14

Implementation under document-ingestion-reliability 10.1, sharing the existing
workspace reader. This is implementation evidence, not document-release approval.

## User behavior

- Closing/reopening a source or navigating to another app page retains page,
  zoom, search, original/text mode and vertical reading position. Explicit
  citation navigation selects its passage. Changing pages starts at the top.
- Position memory is bounded to 40 sources and owned by the account/workspace/
  permission boundary. It does not persist document text, pixels or queries to
  disk. Every opening fetches and checks the source again; stale hashes and
  revoked sources do not display remembered content.
- Image preview units expose the original encoded dimensions and orientation.
  The reader can mark retained OCR line regions in PNG/JPEG/TIFF originals using
  all eight EXIF transforms. Bounds require the same original hash, matching
  UTF-8 text spans, page and OCR provenance. Duplicate text, corrected/model text,
  unknown frames and truncated blocks do not produce a guessed highlight.
- Raster responses must echo the expected original SHA-256 and decode to the
  declared upright dimensions before an overlay is drawn. A mismatched source
  header rejects the page. Legacy responses without a source header remain
  readable without asserting a geometric correspondence.
- Geometry reads remain revision-pinned, cancellable and bounded to 2,000 blocks
  for the selected page. Missing optional geometry leaves original/text reading
  usable. This does not add word-level geometry, PDF layout or chart accuracy.
- Source rasters keep left-to-right pixel coordinates inside RTL interfaces.
  Overlay contrast is defined against the original page, independently of dark
  application chrome. Extracted-text marks use the active theme.
- Correction drafts survive reader closure and app navigation in session-local,
  account/workspace-scoped memory. Fresh structure authority is required before
  displaying them. Changed extraction bases require explicit inspection/rebase;
  read-only access cannot submit a restored draft. A dispatched save is recorded
  as uncertain before the request leaves, so leaving the view never makes it
  replayable. Cancellation clears the saved draft. No draft is written to disk.

## Verification

- `build/reader-geometry-go.log`: affected TIFF/image/preview tests in RAG,
  server and service-client packages pass. The API package had no matching tests
  in this targeted invocation; it is not counted as an API suite result.
- `build/reader-navigation-typecheck-final.log`: TypeScript check passes.
- `build/reader-regions-ui.log`: eight checks pass: all eight orientation
  mappings, exact UTF-8/source/provenance rejection, bounded position memory,
  English/Arabic image regions, app-navigation restoration and TIFF reopen state.
- Visual inspection of the first English/light and Arabic/dark screenshots found
  insufficient dark-mode overlay contrast and RTL raster scrolling. Both were
  corrected before the expanded reader run. UI images are authored fixtures,
  not a real OCR demonstration or a model-accuracy assessment.
- `build/reader-navigation-full-ui.log`: 128 passed, two real-service cases
  skipped, one Arabic private-source fixture timed out at 30 seconds. Its trace
  is retained; the isolated rerun passes. This was not an all-green broad run.
- `build/reader-regions-final-ui.log`: all 11 final tests pass, including the
  timed-out case, source-header/dimension mismatches, legacy no-overlay behavior,
  restored scroll and hiding old pixels while another page loads. Final English
  and Arabic screenshots were visually inspected.
- `build/reader-geometry-final-go.log`: affected RAG, server and service-client
  TIFF/image/preview/source/structure tests pass. `build/reader-api-check.log`
  confirms generated TypeScript contracts match OpenAPI.
  `build/reader-final-production-build.log` passes after the correction-draft
  follow-up; the existing large-bundle advisory remains.
- `build/reader-draft-ui.log`: 13 correction tests pass, covering navigation,
  revoked/read-only access, changed-base recovery, an in-flight save, stale receipts
  and revision-pinned paging. `build/reader-draft-all-locales-ui.log` subsequently
  passes all 39 cases, including navigation, fresh authority and no local-storage
  draft writes in all 29 interface languages.
- Documentation/link structure and handwritten source-size checks pass. Both
  coordinated OpenSpec changes validate strictly. The final diff has no whitespace
  errors. No race, Python-runtime, native linguistic or hardware qualification
  is inferred from these checks.

## Translation handoff

The shared worktree added 25 task-changes messages and then three recovery
messages while this increment was in progress. Their missing entries were added to all 20 preview-language packs
without replacing existing translations. Final catalog checks report 1,504/1,504
messages with no missing entries. Updated JSON review packs, glossary, source
contexts and coverage report for all 29 languages are in
`build/translator-review-20261114`. Native review remains pending; catalog
completeness does not certify linguistic quality.

## Still open

Thumbnails, horizontal pane-position memory, full library/version UX,
multilingual OCR packs, layout/table processing,
performance and representative document/model qualification remain open. No
installer, deployment or publication is included in this increment.
