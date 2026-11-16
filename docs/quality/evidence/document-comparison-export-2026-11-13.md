# Document comparison export evidence — 2026-11-13

Continues `document-ingestion-reliability` 9.2 and the comparison workspace.
This increment implements exports, not summary/requirements generation or the
remaining seven-phase qualification gates.

## Behavior and boundaries

- Finished comparisons export JSON, CSV or Markdown via authenticated HTTP, app
  and CLI. Python parity is authored but unrun. Capability discovery gates the
  app control; older services keep existing comparison behavior.
- Artifacts carry both pinned source references and original hashes, display
  names, every source unit, all unit-pair receipts, changed passages/cells and
  unresolved gaps. Incomplete coverage remains incomplete. No model runs.
- Immutable graphs are verified once, including native cell metadata, and the
  stored plan/pairs are checked against them. Each exported range and text hash
  is checked. Exports contain full changed passages, not the 512-byte UI previews.
- One HTTP export preparation runs at a time, with a 25-second request budget.
  The 8 MiB artifact limit fails without a partial result. Output is prepared
  before delivery, then current source access is rechecked. This is bounded
  implementation behavior, not an 8 GB hardware performance qualification.
- Source policy governs both documents. A content-free preparation event commits
  with an access recheck; it does not prove that a client saved the artifact.
  Downloaded copies have independent retention and cannot be recalled later.
- CSV prefixes formula-like display cells, including whitespace-prefixed formula
  starters. Exact text and cell values remain in `metadata_json`. Markdown fences
  are longer than any source backtick run; source text cannot become active links,
  images or HTML. Stable export field identifiers remain English; source content
  is not translated.
- Clients check artifact identity, format, size and digest. CLI verifies before
  creating a new file, refuses overwrite and removes its own failed partial file.
  The app aborts pending transfer on unmount and never automatically retries an
  export. Its compact disclosure and retention copy use all 29 interface locales.

## Executed checks

Paths are relative to the repository root; `build/` contains ignored local evidence.

| Evidence | Result |
| --- | --- |
| `build/analysis-export-go-final.log` | Full uncached RAG, server, serviceclient, CLI and API packages pass. Includes HTTP identity/revocation, forged evidence, output limits, cancellation, content-free audit, formatter safety, client receipts and no-overwrite CLI behavior. |
| `build/analysis-export-native-office.log` | Real DOCX/XLSX fixtures pass through parsing, persisted jobs and all three export formats. Changed footnote exceptions, both formulas, cached-value metadata and incomplete coverage survive. |
| `build/analysis-export-ui-final.log` | 75 headless comparison checks pass. All 29 locales export; forged downloads are rejected; unfinished jobs hide export; existing recovery, source inspection, RTL, narrow layouts, keyboard and zoom-equivalent tests pass. |
| `build/analysis-export-translations-final.log` | 1,447/1,447 authored messages per locale, zero missing. This is catalog completeness, not native review. |
| `build/analysis-export-api-check.log` | Generated TypeScript matches OpenAPI. |
| `build/analysis-export-build-final.log` | TypeScript and production renderer build pass; existing large-chunk warning remains. |

Initial checks caught the new route missing from the frozen registration fixture
and incorrect Portuguese locale keys. Both were corrected; the final route suite
checks 174 registrations and their permission wrappers. The UI literal audit
explicitly allows the standard format name “Markdown”; surrounding actions and
retention guidance are translated. Initial failures remain in the earlier logs.

Final light and dark RTL desktop renders were inspected and retained under
`build/document-comparison-export-20261113/`. These are headless fixture screenshots,
not installed-app or model-performance evidence. Translator packs were refreshed
under `build/document-export-translator-review-20261113/`; native review remains
pending. Source-size/layout checks, strict OpenSpec validation and diff checks pass.

## Open work

Python tests were not executed because a usable interpreter is unavailable here.
No race suite, packaged installation, native spreadsheet-application execution,
hardware benchmark or model-answer qualification was performed in this increment.
CSV checks parse exported records and verify neutralization; they do not certify
every spreadsheet application's import behavior. Summary/requirements processors,
advanced extraction packs, broader library/retention work and corpus/pilot gates
remain open. No installation, publication or deployment was performed.
