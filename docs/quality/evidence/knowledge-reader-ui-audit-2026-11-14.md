# Knowledge and document workspace presentation audit — 2026-11-14

## Scope and provenance

Continues `workspace-outcome-experience`, coordinated with
`document-ingestion-reliability`. This is renderer implementation and headless
Chromium evidence using isolated synthetic records, not installed-user data,
native desktop interaction, real document extraction or model-answer evaluation.
No installation, publication or database migration was performed for this pass.

The user rejected the initial spacing-only refinement and the next reader's
stacked search/navigation/mode/warning rows. The final design consolidates the
reading toolbar, exposes search on demand and places unread-page links beside
the coverage warning. Source content remains unmodified by presentation.

## Changes

- Chat/document uses one framed reader pane with a distinct page canvas and a
  larger share of the available workspace. Its Chat toggle focuses the document
  without unmounting the conversation. Page, zoom and draft survive the toggle.
- Original, extracted text and side-by-side views use one labelled selector.
  Search opens explicitly, receives focus, and Escape closes search before the
  reader. Reopening a saved query restores the search field.
- The attachment name opens its preview; the duplicate action row is removed.
  Recovery actions and incomplete-coverage warnings remain visible.
- Preparation animation spans the preview canvas. OS reduced motion and the
  application's reduced-motion preference both suppress the moving sweep.
- The structured review presents blocks as a continuous reading surface instead
  of separate numbered grey cards. Revision details follow the content; history
  selection, coverage, corrections and original revision identity remain intact.
- Knowledge keeps the action menu aligned with the document on narrow layouts.
  Its feature layout explicitly overrides the generic shared action-row layout.
- Legacy source previews use reading typography; generic source-dialog styles
  no longer impose a nested 65vh scrolling area on the shared reader.
- Settings spacing is reduced, selected models use a tonal surface/checkmark
  instead of a tall stripe, Activity filters share their selected-state styling,
  and narrow agent lists no longer reserve a large empty instruction panel.
- Comparison dividers use the decorative separator token. Control boundaries,
  keyboard focus and forced-colour indicators remain separate.

All controls reuse existing translated messages. Synthetic English document
content is intentionally preserved in non-English interface captures.

## Verification record

Final verification:

- 54 targeted behavioural cases passed across the final run and one isolated
  rerun. The initial run had 53 passes and one timeout while creating a browser
  page, before application interaction. The isolated citation rerun passed.
  Coverage includes image/TIFF identity, reader modes, search focus, both motion
  preferences, 200% layout, focus/draft continuity, read-only private structure,
  exact saved citations, revocation, legacy sources and Knowledge setup failures.
- Six visual profiles passed across the final matrix and one isolated Arabic
  rerun: light/dark desktop English, narrow German/Arabic, Chinese with the
  Dracula light palette, and English with Warm dark. Each captures six loaded
  routes and Knowledge preview/review (48 final captures). The first Arabic run
  timed out during initial navigation under resource pressure; the isolated run
  passed. Actual screenshots were inspected, including split/focused PDF,
  full-area preparation, narrow Knowledge actions and themed/RTL readers.
- Production TypeScript/Vite build passed with a 1536 MiB Node heap. The earlier
  768 MiB attempt exhausted its heap during concurrent browser verification.
  Existing large-chunk warnings remain; this is not a bundle-size qualification.
- Source-size ratchet and maintained-document link checks passed. Both affected
  OpenSpec changes validate strictly. Translation inventory reports no missing
  catalog copy; preview languages retain native-review-pending status.
- No backend source or API contract was changed in this UI pass, so parser/model
  quality claims rely on their separate evidence, not these renderer fixtures.

Earlier iterations are not counted as final passing evidence:

- The earlier comparison/settings/private-correction regression run passed 250
  tests before the subsequent compact-reader redesign.
- An intermediate reader/Knowledge run passed 240, skipped two service-dependent
  cases and failed two cases while source changes were being made. It is not a
  clean final run. The affected Portuguese image and Chinese structure cases are
  included in the final targeted run.
- The first audit fixture lacked required structure receipt fields; this fixture
  was corrected rather than weakening the client validator.
- A syntax error in the attachment-button edit was corrected before final checks.
- An eight-case focused run passed for focused reading, private structure,
  accessible search and preparation animation. Further assertions cover closing
  search and application-level reduced motion in the final run.

Evidence locations (local, generated and not committed):

- `build/interface-audit/final/`: loaded Chat, Agents, Activity, Knowledge,
  Models and Settings, plus Knowledge preview/review, in desktop/narrow profiles.
- `build/document-workspace-design/`: synthetic PDF page in light/dark split and
  focused views, plus full-area pending preparation screenshots.
- `build/reader-design-final.log`: final focused behavioural regression output.
- `build/reader-design-recheck.log`: isolated citation rerun.
- `build/interface-audit-final.log`: final screenshot matrix output.
- `build/interface-audit-recheck.log`: isolated Arabic matrix rerun.
- `build/interface-audit-build.log`: production TypeScript/Vite build output.

## Limits

This does not close the full document upgrade, native linguistic review,
departmental qualification or an exhaustive audit of every application state.
The fixture checks cannot establish PDF/OCR extraction accuracy, retrieval recall
or factual model-answer quality. User acceptance of visual design remains
distinct from passing automated checks. Installation and packaged/native testing
remain separate from this renderer pass.
