# Human review of document drafts — 2026-11-13

This increment connects human review to the existing durable summary/requirements
candidate workflow. It does not qualify the entire seven-phase document upgrade.

## Implemented behavior

- Keep, exclude or leave each claim unreviewed; add notes and explicitly check a
  section, including a section where the model reported no candidates. Checking
  a section requires a decision for every candidate, not just opening its sources.
- Pin decisions to the immutable analysis, batch and model-response digest.
  Keep original claims, quotes and extraction gaps unchanged. A kept claim is a
  recorded human decision, not independent verification of the model.
- Append authenticated author/time and optimistic review revisions. Stale saves
  return conflict. Previous decisions remain readable through the API, CLI,
  authored Python client and the renderer's review-history control.
- Enforce private analysis ownership and current all-source access on reads,
  historical reads, writes and exports. Administration does not grant private
  review access. Source holds and deletion extend to review records.
- Bound notes to 2,048 UTF-8 bytes, history to 256 revisions per section, storage
  to 8 MiB per job and 256 MiB overall. Pre-upgrade SQLite snapshots preserve
  existing source and analysis state before adding the review table.
- Include original candidates, human decisions/notes, reviewer attribution and
  checked-section counts in JSON, CSV and Markdown exports. CSV retains its
  original nine columns and adds `human_decision` and `review_note`. No review
  automatically changes extraction coverage or the generation state.
- Keep unsaved review drafts in memory across pagination and analysis tabs in
  the open dialog. Do not put document review notes in browser local storage.
  Closing the dialog ends that unsaved editing session. Failed/uncertain saves
  offer an explicit reload; there is no automatic write replay.
- Reuse the existing source reader, result workspace, theme tokens and export
  control. Correct the inherited “Export comparison” label for synthesis drafts.
  Review-note direction follows its text in RTL interfaces; checkboxes follow the
  theme. Accessible labels remain stable when a populated textarea is remounted.

## Executed verification

- Storage tests cover immutable history, source identity, author attribution,
  concurrent/stale saves, invalid decisions, notes, quotas, snapshots, source
  holds/deletion, private access and corrupt-review export rejection.
- Authenticated HTTP tests cover save/read, stale conflicts, administrator
  isolation, current revocation and review-aware exports. Go client tests reject
  forged response identities, revisions, decisions and attribution, without
  replaying writes. CLI tests exercise reading and saving a request file and
  rejecting a caller-supplied reviewer field.
- `build/synthesis-review-focused-final.log` records the affected server, storage
  and Go-client checks. `TestKnowledgeSynthesisReviewCommandRoundTrip` passed
  separately after adding the new CLI commands.
- `build/synthesis-review-go-full.log`: full uncached RAG, service-client, CLI,
  server and API suites passed (server 79.5 seconds). The later review-quota
  regression also passed in the focused review suite.
- `build/synthesis-review-ui-final.log`: 33 synthesis tests passed. The combined
  comparison/synthesis run in `build/synthesis-review-comparison-final.log` passed
  **108 tests**, including all 29 interface languages, source focus restoration,
  narrow layout, history, conflict recovery, uncertain writes and export checks.
- The additional enlarged-display/keyboard regression passed in
  `build/synthesis-review-zoom.log`: a 200% browser-zoom-equivalent viewport,
  keyboard decision/note/save interaction, and Escape focus restoration. This
  brings the executed distinct comparison/synthesis UI checks to **109**.
- The initial draft-preservation test exposed an implicit-label problem when
  a populated textarea remounted. Explicit stable label/input identities fixed
  it; the regression passes. A transient full-build failure from concurrently
  edited execution files is retained in `build/synthesis-review-focused.log`;
  the later focused suite compiled and passed without editing that other work.
- Renderer type check and production build passed. Vite retains the existing
  large-chunk advisory. Generated TypeScript contracts are updated.
- Structural translation checks report **1,470 authored messages per language,
  zero missing**. Shared-navigation consistency, source-size and source-layout
  checks pass. Reviewer packs are refreshed in
  `build/document-synthesis-review-translators-20261113`.
- English/light desktop and Arabic/dark narrow screenshots were inspected.
  Synthetic English model output and reviewer notes in an Arabic fixture are
  user/model data, not untranslated interface labels.
  Final images, including the enlarged display, are retained in
  `build/document-synthesis-review-visuals-20261113`; the English/Arabic final
  screenshot fixtures passed again in `build/synthesis-review-visuals.log`.
- Both OpenSpec changes validate strictly, generated-contract checking passes,
  and 376 maintained Markdown files pass the documentation checker. No broad
  OpenSpec task was checked merely because this increment passed.

## Remaining qualification and scope

Python tests are authored but cannot execute in this environment, which has no
working Python interpreter. Race/CGO, native linguistic approval, installed
package/offline validation, representative model quality and hardware-scale gates
remain open. This increment does not rerun inference: its inputs are previously
validated immutable candidates. Actual-model evidence from the preceding
[candidate-job increment](document-synthesis-jobs-2026-11-13.md) remains separate.

Final cross-section synthesis, private-attachment analysis integration and broader
extraction/OCR/layout, reader/library and departmental qualification remain open.
OpenSpec task 9.2 and the broader upgrade are not marked complete. No installation,
publication or deployment occurred in this increment.
