# Mixed-language document OCR — 2026-11-14

This increment continues `document-ingestion-reliability` task 8.3 and the shared
reader work in `workspace-outcome-experience`. It does not close the full document
upgrade, printed-document qualification, native-language review or installation.

## Implemented behavior

- Chat and Knowledge accept up to three explicitly selected, ordered OCR packs.
  The first is primary. A new row starts empty, never with an implicitly selected
  English pack. Duplicate selections are disabled and rejected server-side.
- Every pack is pinned and individually verified. Missing packs stop OCR rather
  than silently dropping languages. Packs are never downloaded automatically.
- A memory-only bundle stays within the existing 32 MiB language-data limit.
  The isolated Tesseract module verifies that every requested language loaded;
  its reader has no host filesystem, network, arbitrary paths or runtime options.
- Selection identity binds ordered IDs and hashes. Saved private attachments,
  imports, retries, page checkpoints and explicit publication retain it. Native
  source text and original bytes are unchanged.
- Mixed requests require `document-ocr-mixed-languages-v1`. Web, Go, CLI and Python
  reject older services before sending the file, and check the returned selection
  without replaying an ambiguous upload.
- Shared source projections expose parser and individual language-pack identities.
  Oversized provenance projections are bounded and explicitly marked omitted.
- All 29 interface languages include the additional controls. Translator review
  packs are regenerated; translations remain drafts with native review pending.

## Runtime provenance

The compiler, Tesseract, Leptonica and language source pins are unchanged. The
memory-only bridge changes the engine revision to
`tesseract-5.5.4-21dd0440aa3cc3cc-offgrid2`, with SHA-256
`21dd0440aa3cc3ccd5b117f2016fc9dad9ca17c99a0c63d00b647d3e05703b38`.
The two existing isolated build directories produced the same bytes without
network access or tool installation. The redistribution-notice digest is unchanged.
The engine lock remains `qualified: false`.

## Executed evidence

- Actual Chinese/French combined-image transcription retains both authored
  sentences and the amount 42. Both individual pack identities survive the sealed
  extraction and citation-persistence checks (`build/ocr-mixed-real.log`).
- Actual two-page mixed-pack TIFF processing cancels after page one, reopens both
  private and SQLite storage, processes only the remaining page and then reuses
  both validated receipts without launching a worker
  (`build/ocr-mixed-checkpoint.log`, 28.041 seconds).
- Actual private HTTP upload first reports missing packs; explicit import and
  retry retain the selection, extracted text, page progress and both source pack
  identities (`build/ocr-mixed-http-provenance.log`). Single-language recovery
  passes alongside mixed selection.
- The complete isolated OCR worker suite passes, including initialization of all
  25 pinned packs (`build/ocr-mixed-worker.log`, 90.374 seconds). This is runtime
  initialization evidence, not language-specific accuracy qualification.
- The affected full Go run passes document OCR, rag, sessions, server,
  serviceclient and API packages. Its initial CLI run caught a concurrent
  conversation-fixture mismatch. The subsequent full CLI and document package run
  passes (`build/ocr-mixed-full-go.log`,
  `build/ocr-mixed-cli-document-final.log`); no unrelated CLI source was changed.
- The full Python SDK suite passes 139 tests
  (`build/ocr-mixed-python.log`, 43.77 seconds).
- The mixed-picker suite has 31 passing cases across its initial and corrected
  runs: 29 locales, reviewed Knowledge destination, and old-service refusal.
  Initial English/French/Spanish/Arabic attempts hit a JSX syntax error; the four
  corrected cases pass in `build/ocr-mixed-ui-final.log`. The initial failure is
  retained in `build/ocr-mixed-ui.log`.
- API generation checks, localization completeness, source-size and application
  layout checks pass. Preview catalogs contain 1,515/1,515 messages with no missing
  catalog text; this is not linguistic approval.
- Production renderer build passes (`build/ocr-mixed-renderer.log`), with the
  existing large-bundle advisory. Final keyboard/icon checks and planning
  validation are recorded in the completion update below.

### Completion checks

- Three-pack engine initialization passes for Chinese/French/English and
  Arabic/English/French. Direct bridge tests corrupt the final pack and confirm
  refusal of a partially loaded selection (`build/ocr-mixed-three-pack.log`).
- Eight final compatibility, keyboard and layout cases pass, followed by four
  status-copy cases in English, Arabic, Swahili and Simplified Chinese. Headless
  screenshots were inspected at desktop light and 390-pixel dark RTL with 200%
  text. Remove controls use shared icons, and removing a language returns keyboard
  focus to the primary selector (`build/ocr-mixed-ui-polish.log`,
  `build/ocr-mixed-status-ui.log`). No live computer control was used.
- Final renderer build passes (`build/ocr-mixed-renderer-final.log`). Both active
  OpenSpec changes validate strictly; the documentation check reports 382 files
  and no problems. Final whitespace check passes after removing two extra EOF
  blank lines introduced in the documentation update.
- The runtime-preparation unit check passes; its optional PDF archive case is
  skipped because no archive path was supplied. That is not a packaged-install
  qualification result (`build/ocr-mixed-packaging.log`).
- The 29 translator packs in `build/document-upgrade-review-20261114` include all
  new draft messages. Native-language sign-off is still pending.
- Race checks remain unrun: this Windows Go environment has CGO disabled and no
  available GCC/Clang compiler; WSL has no Go toolchain. No toolchain was installed.

The initial private HTTP test was launched from the wrong working directory and
could not locate its authored fixture. The next run exposed missing source
provenance fields; adding the shared projection fixed that failure. Final actual
HTTP evidence is `build/ocr-mixed-http-provenance.log`; earlier failures remain in
their original logs rather than being presented as passing runs.

## Remaining gates

Per-page language hints, deskew, the optional offline Docling pack, representative
OCR accuracy and 8/16 GB hardware qualification remain open. Chinese/French
fixtures do not establish accuracy for every combination or script. Long jobs,
advanced layout, private correction history, departmental lifecycle coverage,
advanced formats and the broader corpus/pilot gates remain tracked in the existing
change. No native-language approval, clean-install qualification, publication or
installation was performed by this increment.
