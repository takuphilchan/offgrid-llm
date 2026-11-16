# Document comparison workspace evidence — 2026-11-13

Increment under `document-ingestion-reliability` 9.2, coordinated with
`workspace-outcome-experience`. This extends the comparison backend described in
[the preceding report](document-comparison-jobs-2026-11-13.md). It does not complete
the seven-phase document upgrade or qualify model answer accuracy.

## Delivered behavior

- Knowledge opens a dedicated comparison workspace with separate setup, changes
  and history views. Compact persistent controls leave the reading area available
  for before/after columns. Small screens stack the columns; RTL uses logical
  layout. Coverage remains visible independently of the change text.
- Each source column preserves that source's change order. Adjacent excerpts do
  not assert semantic alignment. Native cell modifications retain source metadata.
  Technical identities are available in source details rather than repeated in
  the main reading view. Single-page controls are omitted.
- Inline excerpts derive from source bytes already checked against full citation
  identities, ranges and hashes. Each has its own exact UTF-8 span and SHA-256;
  the 512-byte limit never splits a character. They are delivery projections,
  not another persisted copy of checkpoint evidence. Full source inspection
  retains the existing 256 KiB bound and explicit truncation.
- The contextual source inspector supports keyboard focus entry and restoration.
  Extraction review opens the shared reader at the cited revision in read-only
  mode. Result page position survives history navigation in account/workspace
  scoped memory. It does not store source bodies in browser local storage.
- Owner-private history is paginated, with at most 20 summaries per request and
  offset bounded to 256. SQLite projects job identities and two source IDs without
  loading full plans/bodies. Current access to both sources is checked before a
  summary is delivered. Filtering can produce an empty page with a next cursor.
  Offset pages may shift as jobs are added/deleted; refresh is explicit.
- The UI requires comparison and history capabilities, but not embeddings or an
  enabled semantic retrieval service. Uncertain submission is recovered through
  history; mutations are not automatically replayed. Cancel/retry are explicit.
  Periodic access checks include completed jobs; failures clear displayed source
  names, evidence and history.
- HTTP, generated TypeScript, Go client, CLI and Python SDK expose the history
  contract. Clients validate optional preview receipts while accepting older
  responses without previews. Python implementation/tests remain unexecuted here.

## Verification

All log paths below are relative to the repository root; `build/` is ignored.

| Check | Result / evidence |
| --- | --- |
| Full affected Go packages | RAG, server, serviceclient, CLI and API pass uncached in `build/comparison-preview-go-final.log`. Includes owner isolation, source revocation, bounds, authenticated HTTP, exact preview receipts and UTF-8 truncation. |
| Comparison headless suite | 44 distinct scenarios verified across runs. `comparison-workspace-final.log` passed 43, with one invalid CSS-zoom simulation failing. The test was corrected to emulate a 640×450 CSS viewport at DPR 2 for a 1280×900 screen; `comparison-workspace-zoom-final.log` passed all four selected checks. This is not manual browser/native OS zoom qualification. |
| Final reading-position/recovery changes | `build/comparison-workspace-recovery-final.log`: 3 passed, covering retained pagination after history navigation, zoom-equivalent layout, focus restoration and retrieval-disabled operation. |
| Final desktop renders | `build/comparison-workspace-desktop-final.log`: 2 passed; English light and Arabic dark renders inspected after the final controls-layer fix. |
| Related UI regressions | Earlier combined run `build/comparison-ui-regressions-final.log`: 158 related reader/navigation/collection checks passed, 2 existing skips. Its one comparison failure used an incorrect English retry-label expectation; corrected and covered by subsequent comparison runs. |
| Locales | All 29 exercised in the headless comparison suite, including narrow layouts. `build/comparison-workspace-translations.log`: 1,445/1,445 authored catalog messages, zero missing per locale. Completeness is not native-language approval. |
| API and renderer | `build/comparison-workspace-api-check.log`: generated API check passes. `build/comparison-workspace-build-final.log`: TypeScript and production renderer build pass after the final reading-position change. Existing large-chunk warning remains. |

Tests also cover forged source bodies and preview text, invalid plans/cursors,
revocation while viewing results, unknown submission recovery without duplicate
creation, older-service capability handling, explicit cancellation/retry, native
formula metadata, qualified empty results, enlarged text and keyboard close.
These use isolated fixtures; they do not measure real model answers or extraction
accuracy on a representative corpus.

Final saved headless renders:

- `build/document-comparison-ui-20261113/comparison-light.png`
- `build/document-comparison-ui-20261113/comparison-dark-rtl.png`
- `build/document-comparison-ui-20261113/comparison-setup.png`
- `build/document-comparison-ui-20261113/comparison-zoom-200.png`

The mixed-language policy excerpts and English filenames in these images are
fixture document content, not untranslated interface labels. No live desktop
control or installed-user data was used. Translator review packs were regenerated
at `build/document-comparison-translator-review-20261113` for all 29 locales.

## Remaining gates

Native translation review remains pending. Python tests could not run with the
available Windows Store aliases; no interpreter was installed. Go race, packaged
desktop/offline-install tests, physical hardware profiles and real model-answer
qualification were not run for this increment. No installation or publication
was performed.

Summary and requirements processors, analysis exports, the remaining library and
private-attachment workflows, advanced offline document/OCR packs, complex visual
and archive support, and the representative corpus/pilot gates remain governed by
the open implementation tasks. This comparison UI does not imply those are ready.
