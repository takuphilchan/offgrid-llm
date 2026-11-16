# DOCX header and footer evidence increment

Continues `document-ingestion-reliability` task 8.2 and coordinates the shared
reader with `workspace-outcome-experience` D1/D2. These tasks remain open.

## Implemented behavior

- The DOCX parser follows live section references to internal header/footer
  parts. Repeated references extract a part once. Unused and historical section
  references do not become current text. Missing, external, escaping,
  conflicting-role and wrong-root parts fail explicitly; no links are fetched.
- Native Unicode text, tables and referenced notes in these parts retain exact
  source spans. Typed header/footer containers, child relationships and package
  part identities survive the shared structured projection. A document whose
  body is empty can still retain readable margin evidence.
- Extraction is bounded by existing archive/text limits, XML depth 256 and at
  most 1,000 margin references/source sections. Rendered pages are not inferred.
- First/even-page visibility, inherited section placement and rendered field
  values remain unresolved. Each margin source unit is marked for review while
  readable text stays usable. Current inserted text is retained, deleted text is
  excluded, and tracked margin changes carry explicit warnings. Revision
  alternatives are not yet available.
- Parser identity advances to `offgrid-docx-structure-v5`. Older immutable
  extractions are not rewritten. All 29 format-limit messages and the built-in
  fallback copies reflect the new behavior; linguistic approval remains pending.

## Verification

- `build/docx-margin-focused.log`: parser and private HTTP regressions pass.
  The HTTP fixture verifies header date and footer deadline exception in model
  context, structured source identities, coverage and saved citation reopening.
  Its generated answer is a transport fixture, not a model-quality assessment.
- `build/docx-margin-real-http.log`: seven real local BGE-M3/llama-server Knowledge
  ingestion/retrieval/source cases pass, including the footer exception. The
  suite took 8.57 seconds including startup; the margin fixture took 0.09 seconds.
  These are single-fixture timings, not retrieval p95 or generated-answer quality.
- `build/docx-margin-go.log`: rag, document, serviceclient, CLI and API packages
  pass. Sessions/server initially failed four literal prompt assertions while
  the shared attachment prompt was being changed concurrently. No unrelated
  changes were reverted. `build/docx-margin-shared-recheck.log` records the
  current sessions/server suites passing (5.668 and 56.552 seconds).
- `build/docx-margin-parser-final.log`: the complete rag suite passes after the
  historical-section handling refinement (11.094 seconds).
- `build/docx-margin-ui.log`: 29 headless reader tests pass, one per interface
  language, including narrow-layout overflow checks (40.3 seconds).
- `build/docx-margin-web.log` and `build/docx-margin-build.log`: TypeScript,
  generated-contract check, translation completeness and production build pass.
  All 29 catalogs contain 1,380 messages with zero missing entries; this does not
  establish translation quality. The build retains its existing bundle warning.
- `build/docx-margin-quality.log` and `build/docx-margin-diff.log`: source-size,
  application layout, strict OpenSpec validation and whitespace checks pass.

Translator handoff: `build/document-docx-translator-review-20261113/` contains
29 refreshed packs with pending reviews. No reviewer approvals were fabricated.

## Remaining scope

This increment does not resolve DOCX section-to-rendered-page mapping, field
recalculation, revision alternatives, hyperlinks as structured relationships,
drawings or embedded objects. Nor does it close the approved PDF/layout, TIFF,
multilingual OCR, Docling, durable analysis, richer reader, departmental access,
visual/archive and qualification milestones. No installation or publication was
performed. Race, native language, hardware-profile and representative corpus
qualification are not certified by these checks.
