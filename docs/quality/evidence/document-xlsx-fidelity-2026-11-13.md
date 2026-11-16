# Spreadsheet fidelity increment

Part of `document-ingestion-reliability` 8.2; the complete approved document
upgrade remains open. Existing immutable extractions are not rewritten.

## Implemented

- Preserve native typed values separately from deterministic display projections.
  Decimal rounding uses exact rational arithmetic; financial source digits are
  retained even when a supported display rounds them. Explicit dates preserve
  the 1900/1904 system; Excel's fictitious leap day is flagged. Unsupported,
  locale-dependent and width-sensitive formats retain their IDs/codes and raw
  values with a review warning. This is not an Excel rendering engine.
- Preserve formulas, shared-definition anchors, cached zero/empty text, absent
  cache, error cache and workbook recalculation/manual flags. Never calculate
  formulas, rewrite shared relative references, execute macros or fetch links.
- Preserve hidden sheet/row/column provenance, merged ranges including empty
  sheets, native row structure and explicit table-header relationships. A bounded
  row sweep handles 100,000 cells and 10,000 ranges without a cell-by-range scan;
  overlapping rectangles are rejected even when they contain no populated cells.
- Retain typed metadata through immutable SQLite revisions, private attachment
  graphs and bounded source APIs. Text corrections retain visibility but do not
  mislabel edited prose as a recalculated numeric value. Reader projections
  disclose omitted oversized unit metadata and leave the extraction unchanged.
- Updated format-limit copy across all 29 interface languages. Human translation
  review is still pending; source annotations and formula contents remain source
  evidence, not translated application controls.

## Verification

`build/xlsx-fidelity-full.log` records passing document (3.463 s), rag (26.738 s),
sessions (11.095 s), server (152.958 s), serviceclient (1.164 s), CLI (3.873 s) and
API (1.858 s) suites. Tests include frozen Office files and authored synthetic
workbooks, raw-value preservation, unsupported formats, cache states, hidden
cells, empty/overlapping ranges, restart, correction and exact source readback.

`TestXLSXEvidenceThroughPrivateHTTPGenerationAndSavedCitation` exercises upload,
private typed inspection, exact model input and saved source reopening, including
a header 300 rows before the numeric value. Its response is a transport fixture,
not evidence of model-answer quality.

`build/xlsx-fidelity-real-http.log` records `TestRealKnowledgeEvidenceHTTP`
passing with the already installed local BGE-M3 and llama-server. Five fixtures
exercise ingestion, hybrid retrieval and exact citation delivery; the workbook
case took 2.75 s and the complete test 20.04 s including runtime startup. This is
not a p95 measurement, scale benchmark or generated-answer qualification.

`build/xlsx-fidelity-web.log` records the generated API/type check and production
build (3.65 s). `build/xlsx-fidelity-translations.log` records 1,380 authored
messages per language, zero missing; linguistic review remains pending.

The subsequent combined Office verification adds 58 passing rendered language
cases and final Go/API/web checks; see
[presentation increment evidence](document-pptx-structure-2026-11-13.md).

An initial fraction-display regression exposed incorrect literal treatment of
`?/?` and was fixed. An initial readback fixture omitted retained-source state;
the fixture now represents an accessible retained source and verifies every
chunk. Earlier failed logs are retained rather than presented as passing runs.

## Scope still open

Full original Office visual comparison, 98% cell / 95% header qualification,
200-document/1,000-question corpus, native-language review, 8 GB measurements,
race/Python and clean offline-package checks remain unproven here. Charts,
images, conditional formats and automatic formula calculation are not advertised
as interpreted. Rich spreadsheet UI, other Office/PDF adapters, OCR packs,
analyses and departmental workflows remain in the approved plan.

## Contract references

Implementation decisions use Microsoft's primary documentation for
[cached formula results](https://learn.microsoft.com/en-us/office/open-xml/spreadsheet/working-with-formulas),
[cell types](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.spreadsheet.cell),
[shared formulas](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.spreadsheet.cellformula)
and [number-format international considerations](https://learn.microsoft.com/en-us/openspecs/office_standards/ms-oi29500/17d11129-219b-4e2c-88db-45844d21e528).
