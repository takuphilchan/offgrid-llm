# Document extraction corpus

These 37 authored files contain public test content, never installed-user data.
`manifest.json` pins each file's bytes, SHA-256, expected ordered text, source
units and error class. Content is MIT licensed; `mapped-latin.pdf` embeds a
Bitstream Vera subset covered by the adjacent font notice. No system fonts are
copied from the author's computer.

Generate with the isolated development requirements in
`scripts/test/document-fixtures-requirements.txt`, then run:

```text
python scripts/test/generate-document-fixtures.py
python scripts/test/generate-parser-corpus.py
go test ./internal/rag -run TestParserCorpusIntegrity -count=1
```

Fixture generation is development tooling, not a runtime dependency. Repeating
the corpus generator produces identical bytes. The Office prerequisites retain
their own manifest in the parent directory.

The corpus includes ASCII85/Flate compressed PDF text, a subset-font Unicode
mapping, escaped strings, multiple pages, columns, rotated text, image-only and
mixed pages, encryption, truncation, an unused large stream and PDF 1.5 object
and cross-reference streams. It also covers multilingual UTF-8 text, HTML, JSON,
CSV, XML, RTF, DOCX, relationship-ordered XLSX/PPTX, invalid UTF-8 and a 65 MiB
expanded ZIP. This is a minimum regression corpus, not a representative sample
of all enterprise documents or a complete OCR-language benchmark.

`TestParserCorpusScore` is an opt-in assessment. Set
`OFFGRID_DOCUMENT_CORPUS_REPORT` to an absolute file in an isolated output
directory. It disables PATH-based PDF extraction to measure the packaged
fallback. `OFFGRID_DOCUMENT_CORPUS_STRICT=1` makes any unresolved text/error
expectation fail. Without strict mode, the assessment writes known failures;
its successful exit does **not** mean the parser passed the corpus.

Text scoring compares ordered exact snippets after whitespace normalization.
Page count is recorded separately and is not source-locator qualification.
For scan/mixed files, the initial expected error is `document_ocr_required`:
an OCR candidate must additionally verify the expected page text before it may
claim success. Mixed native text must ultimately remain available as explicit
partial content. Encrypted and blank PDFs need distinct errors, not the current
generic invalid-document response. Expected outcomes are not relaxed to fit a
candidate engine.
