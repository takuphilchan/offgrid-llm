# Shared document reader and Word outline evidence — 2026-11-13

Combined implementation under document-ingestion-reliability 8.2/10.1 and
workspace-outcome-experience D1/D2. This is not completion of the seven-phase
upgrade or a claim of public-sector qualification.

## Delivered behavior

- Chat and retained Knowledge sources use the common reader. Legacy records
  without retained originals or structured revisions keep basic text reading.
  Original/text comparison uses bounded original-page rendering and stacks on
  narrow layouts. It does not render native Office layout.
- Immutable structure projections carry per-unit block, review and figure counts.
  A separately paginated outline supplies exact within-unit block targets for
  headings, tables, figures and review items. Counts come from the complete
  retained graph, not the current excerpt. They do not measure extraction accuracy.
- Opening a target pins its extraction revision, selects the exact block and
  moves focus into view. Unit coverage exposes review and hidden-content states.
  Correction drafts survive reader-tab changes; navigation is disabled during
  active edits. Full route-navigation persistence remains unfinished.
- Shared clients validate preview/navigation identities, bounds and cursors.
  Old services without navigation retain readable text with an explicit notice.
  HTTP, generated TypeScript, CLI and Python contracts carry navigation offsets;
  Python tests are authored but were not executed in this environment.
- DOCX v6 resolves declared paragraph-style outline levels, same-type inheritance,
  defaults and direct body-text overrides. Historical formatting cannot replace
  current headings; tracked formatting changes remain disclosed. Parsing bounds
  style count/depth and rejects cycles, duplicate definitions, unsafe/external
  relationships and missing declared parts. Original text/numbers remain intact.
  Existing extraction revisions and citations are not rewritten.
- Nine reader messages are authored for all 29 interface languages. At this
  verification point every catalog contains 1,415/1,415 messages. Translator
  packs are in `build/document-navigation-translator-review-20261113/`.
  Completeness is not native-language approval.

Style inheritance follows the same-type parent rule in Microsoft's
[BasedOn documentation](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.basedon?view=openxml-3.0.1).
Direct outline values retain the body-text distinction documented for
[OutlineLevel](https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.outlinelevel?view=openxml-3.0.1).
Localized style names are not used as heading heuristics.

## Executed checks

- `build/document-navigation-go-verified.log`: uncached document, RAG, sessions,
  server, serviceclient, CLI and API suites pass. Includes real DOCX parsing,
  private HTTP outline targeting, model-input/saved-source transport, current ACL
  revocation, malformed bounds, Unicode offsets and immutable revision checks.
- `build/document-navigation-ui.log`: 262 renderer/contract tests pass; two
  opt-in real-service tests are skipped. Coverage includes all 29 languages,
  shared reader, structure/corrections and Knowledge original/source behavior.
- `build/document-navigation-ui-final.log`: 37 final focused tests pass after
  marker/range layout fixes. Includes narrow layouts, RTL, enlarged text,
  correction-draft retention, legacy navigation and malformed receipt rejection.
  Final English/light and Arabic/dark screenshots in
  `build/document-navigation-final-results/` were inspected. Source text and
  fixture names remain original content, not interface translations.
- `build/document-navigation-real-packaged-verified.log`: actual local BGE-M3
  embedding and packaged OCR tests pass using disposable state. Nine document
  cases cover DOCX footnotes/margins/style navigation, punctuation, table headers,
  XLSX values, PPTX notes, Chinese retrieval and PNG OCR; the private collection
  case also passes. A separate private-image HTTP test passes OCR extraction,
  saved structure, generation-context transport and source reopening.
- Real runtime invocation: compile the server suite beside the existing
  `build/offgrid-ocr-worker.exe`, then run with explicit local BGE-M3, llama-server,
  `eng.traineddata` and scan fixture paths. No model/dependency downloads occur.
  This validates actual extraction/retrieval paths, not real generated answers
  or representative OCR accuracy.
- API generation check, TypeScript/renderer build, translation completeness,
  literal audit, source layout/size checks, strict OpenSpec validation and
  `git diff --check` pass. The existing renderer bundle-size warning remains.
  Logs use the `build/document-navigation-` prefix.

## Failures retained and corrected

The new HTTP heading assertion first failed because the parser recognized only
direct paragraph outline properties. Declared style inheritance and a real styles
part were added; the same user-facing assertion now passes. A new parser test
initially used the wrong projection signature and was corrected before the
successful full run. Earlier logs remain available.

The first real-runtime run passed the native document cases but could not locate
OCR beside Go's temporary test binary. The packaged test executable fixed that
test layout. An initial direct invocation also needed quoted dotted Go test
flags under PowerShell. Both earlier failures remain recorded; the final packaged
run passes image OCR in Knowledge and private Chat.

## Still open

Full reader state across route changes, source-region highlights, thumbnails,
version/export workflows, richer PDF/forms, tracked-change alternatives,
multilingual OCR/TIFF/deskew and the offline layout pack remain open. Durable
document analyses, complete departmental retention/audit and representative
corpus/hardware/pilot qualification also remain in the controlling plan.

No native-language review, Python execution, Go race run, clean-install packaging,
hardware scale qualification or real-model answer accuracy is claimed here.
No installation, publication or live Computer Use was performed.
