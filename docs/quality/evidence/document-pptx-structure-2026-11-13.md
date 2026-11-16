# Presentation structure increment

Continues `document-ingestion-reliability` 8.2 and coordinates reader contracts
with `workspace-outcome-experience` D1/D2. Full upgrade and qualification remain
open; this increment does not rewrite older immutable extractions.

## Implemented behavior

- Presentation relationship order remains authoritative. Slide XML is bounded
  to 200,000 nodes and depth 128, with at most 1,000 slides and bounded text/block
  output. Missing, external, duplicated or escaping notes relationships fail
  explicitly; no package links are fetched.
- Speaker notes retain their notes-part identity, are labeled separately from
  displayed slide text, and remain linked to the relevant slide evidence.
  Notes-page slide-number/date/header/footer placeholders are excluded from
  speaker prose. A `notes_for` edge intentionally adds notes without recursively
  requiring every unrelated paragraph on the slide.
- Tables preserve native Unicode text, exact numeric characters, rows, cells,
  grid dimensions and merge attributes. First-row/column styling remains
  metadata: it is not a semantic header declaration. Ambiguous associations
  require review rather than invented labels.
- Pictures and charts retain package-part identity; linked external visuals are
  marked without fetching or copying external URLs into projections. Other
  embedded objects and alternate content are explicitly unprocessed. Authored
  visual descriptions are labeled separately from interpreted facts. Unread
  chart payloads never become numerical evidence.
- Hidden slides are retained and marked. Office warnings identify review units
  independently of readable-unit counts and processing/index success. Source
  inspection, saved citations and model context preserve those distinctions.
- Additive OpenAPI/TypeScript contracts carry figure/table metadata. The shared
  reader uses the existing localized coverage presentation. All 29 format-limit
  messages and translator packs were updated; no native review was fabricated.

## Evidence and limits

`build/pptx-structure-regression.log` and `build/pptx-private-http-fixed.log`
record parser, source readback and private HTTP regressions. Fixtures include a
deadline exception in speaker notes, multilingual native text, a table value,
authored alt text, hidden and external visuals, structural bounds and malformed
notes relationships. Saved Chat citations reopen the note's exact source text.
The generated answer in that test is a transport fixture.

The first private HTTP attempt (`build/pptx-private-http.log`) exposed a context
overflow when bidirectional footnote edges expanded all slide paragraphs. The
new notes relationship fixes that actual failure at the same 4,096-token context
and 1,024-token response limit; the fixture was not made smaller to pass.

`build/pptx-real-http.log` records all six `TestRealKnowledgeEvidenceHTTP`
fixtures passing with the existing local BGE-M3 and llama-server. The full test
took 11.18 s including startup; the PPTX case took 1.10 s. This proves the tested
ingestion/retrieval/source path, not generated-answer accuracy or latency p95.

`build/pptx-fidelity-full.log` records passing document, rag, sessions, server,
serviceclient, CLI and API packages before the final hidden-slide/review-unit
refinement. Final verification is recorded separately in
`build/office-fidelity-final.log` and `build/office-fidelity-final-web.log`.

Final package results: document 1.538 s, rag 13.094 s, sessions 6.240 s,
server 60.928 s, serviceclient 0.373 s, CLI 1.155 s and API 0.713 s, all passing.
Generated contracts, type checking and the production renderer build passed.
`build/office-fidelity-localized-ui.log` records 58 passing headless browser
cases (XLSX/PPTX in each of 29 languages), including narrow-width overflow checks,
in 1.3 minutes. These use isolated response fixtures, not installed-user data.
The earlier five shared-reader safety tests also passed in 11.2 s.

`build/pptx-translations.log` records zero missing catalog messages (1,380 per
language). Translator handoff: `build/document-office-translator-review-20261113/`.
Native reviews remain pending. Strict OpenSpec validation, application layout,
source-size and whitespace checks passed; these are not release qualification.

The remaining limitations include master/layout text, animations, visual reading
order/coordinates, chart/image interpretation, merged-header qualification,
original Office rendering and the representative corpus thresholds. Standalone
image/TIFF OCR, multilingual packs, Docling, durable analyses, richer readers,
departmental APIs and release qualification remain on the approved full plan.
Python/race/platform checks and human linguistic review are not certified here.

## Primary references

Microsoft documents [notes slide parts](https://learn.microsoft.com/en-us/office/open-xml/presentation/working-with-notes-slides),
[table cells and merge attributes](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.drawing.tablecell?view=openxml-3.0.1)
and the [first-row formatting flag](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.drawing.tableproperties.firstrow?view=openxml-3.0.1).
These distinctions informed the parser and prevent styling from being presented
as verified table semantics.
