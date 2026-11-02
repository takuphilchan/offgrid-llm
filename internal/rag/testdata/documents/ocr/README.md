# OCR evaluation receipts

These are actual local Tesseract hOCR outputs from the repository's authored
PDF fixtures, not user documents. `evaluation.json` binds their source and output
digests, candidate engine, language data and measured times. OffGrid's pinned PDF
worker rendered each selected page at 200 DPI. The mixed fixture uses page 2.

The candidate recognizes the scan and mixed-page phrases. The rotated fixture
fails: visual inspection shows that its original PDF places part of the text
outside the page, and the resulting recognition contains low-confidence fragments.
That failure is retained. It must not be scored as a successful rotation test or
fixed by inventing the missing characters. The `inputs` directory adds fully
visible 0/90/180/270-degree fixtures and one mixed-content page, authored by
`scripts/test/generate-ocr-fixtures.py`. Their source digests are checked and
PDFium rendering is verified, but recognition must pass separately.

The outputs exercise receipt parsing, word order, Unicode, bounds and confidence
preservation. They do not qualify the engine for shipping. The candidate's old
Emscripten ABI fails against wazero v1.12.0 (`emscripten_stack_get_current` missing);
the isolated comparison used its compatible v1.5.0 runner. Neither was added to
OffGrid's runtime dependencies. Current engine build provenance, redistribution
notices, maintained dependencies, language import, cancellation and full OCR
integration remain required. No OCR availability flag is enabled by these tests.

`native/` records the maintained Tesseract 5.5.4 native ABI comparison. Five of
seven cases match their expected text exactly. Fully visible 180/270-degree
pages still fail with low confidence; those failures must remain visible until
orientation handling passes. Both parts of the same-page mixed fixture are
recognized, but that does not test merging OCR with the existing native-text
extractor without duplication. The ABI rejects invalid dimensions, short pixel
buffers and unsupported language identifiers. Native compilation and these
receipts do not qualify a packaged, isolated worker.
