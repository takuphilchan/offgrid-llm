# PNG/JPEG document OCR implementation evidence — 2026-11-13

This is an implementation increment under document-ingestion-reliability 8.3,
not completion of the document upgrade or OCR qualification.

## Behavior

- Chat file attachments and Knowledge accept single PNG/JPEG documents through
  the existing isolated OCR worker, independently of vision inference.
- Decode dimensions before pixels; enforce 24 MiB, 8192 pixels/side and 16 million
  pixels. Reject malformed pixels, mismatched extensions, animation, multipicture
  JPEGs and ambiguous EXIF. Cancellation propagates through decoding/rotation.
- Preserve originals. OCR and preview share an upright lossless PNG derivative,
  explicit white alpha compositing and 8-bit channels. Line geometry maps back to
  original encoded pixels, including all eight EXIF transforms.
- Bind extraction to source, runtime and verified language digest. Retain line
  text spans and geometry. No inferred headings, table associations or visual
  interpretations. OCR success still has text-only visual coverage.
- Missing packs/workers and uncertain OCR retain originals and partial review
  state. Readers reopen saved text without running OCR. Source access and digest
  checks apply to raster and text; private review cannot grant generation.
- Explicit “Use available text only” reuses retained uncertain OCR instead of
  erasing it through a native-text preview. New `partial_text_only` coverage stays
  partial in saved sources/model context, with explicit uncertainty instructions.
  Ordinary OCR failure never automatically performs this acknowledgement.
- Shared localized reader controls work for images. Existing all-language
  coverage notices are reused; no native-language approval is inferred.

## Executed checks

Local ignored logs retain exact output:

| Check | Evidence/result |
| --- | --- |
| Go document, RAG, image input, sessions, server, CLI, service client, API suites | `build/image-document-go.log`: all eight packages passed |
| Bounded pixels, EXIF geometry, alpha composition, source identity, partial persistence, missing pack/worker, cancellation | `build/image-document-final-focused.log`: passed |
| Actual packaged Tesseract, verified English pack, existing authored scan fixture in PNG and JPEG | `build/image-document-real-ocr.log` and final focused run: both passed |
| Actual private HTTP OCR/structure/context/saved receipt and Knowledge ingest/search/citation with existing BGE-M3 and llama runtime | `build/image-document-real-http-final.log`: passed; PNG Knowledge case 3.10 s, private case 3.19 s in that run |
| Image reader across all 29 locales plus private-reader/focus regressions | `build/image-document-final-ui.log`: 33 passed |
| Explicit partial-OCR acknowledgement and retained warning in English, Arabic, Chinese | `build/image-partial-ui.log`: 3 passed |
| Partial recovery server/session/CLI regressions | `build/image-partial-recovery.log` sessions passed; concurrent agent edits temporarily prevented server/CLI compilation. Definitions appeared in current tree; `build/image-partial-recovery-retry.log` server/CLI passed without modifying that work |
| API generation/check, TypeScript, production renderer build | Passed; existing bundle-size warning remains |
| Translation catalog checks | `build/image-document-i18n.log`: 1380/1380 messages, zero missing in all 29; native review pending |
| Source-size/layout, strict OpenSpec validation | Passed |

The first compiled HTTP-test invocation used PowerShell-misparsed dotted flags;
`build/image-document-real-http.log` retains that failure. Quoted flags passed in
the retry/final logs. The private generation answer is a transport fixture, not
an actual-model answer-quality evaluation. Timing above is one test run, not p95.

## Still required

Multipage TIFF, deskew, multilingual pinned manifests and explicit per-document
language selection; page checkpoints; optional Docling pack; full reader overlays
and side-by-side presentation; advanced visual interpretation. The wider approved
analysis/collection/retention/pilot work remains open. These checks do not qualify
handwriting, charts, signatures, difficult scans, colour/HDR, all-language OCR,
8/16 GB performance or clean installations. No dependencies/models were fetched;
tests used existing local assets and disposable state. No install or publication
was performed. Python, race/native compiler and external reviewer gates remain
unverified as documented in the broader change.
