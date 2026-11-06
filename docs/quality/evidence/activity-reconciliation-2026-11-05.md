# Activity reconciliation follow-up — 2026-11-05

This is a local follow-up to released v0.4.26, based on `f927194b` plus the
current Activity changes. It is not part of the already published tag.
The installed service identifies itself as
`f927194b-activity-preview-20261105`. No normal-user work was stopped or deleted
by the implementation or its isolated tests.

## Changes and limits

- Added actor-authorized `/api/v2/work` metadata and complete root-job counts,
  with 20-item default pages and a 100-item maximum. Full task payloads remain
  available through existing compatible endpoints.
- Prioritized attention before the header preview limit, excluded completed
  chats from that preview, and deduplicated known linked chat/job references.
- Added paginated Activity, protection reasons and explicit bulk review.
  The review collects up to 5,000 matching records, then confirms fixed IDs.
  A larger review asks for a narrower search. Partial failures remain selected.
- Acknowledged Stop/delete invalidates pending reads. Mutations reconcile
  immediately and coalesce metadata refreshes to avoid a request per deletion.
- Retained children do not become new root entries after parent-history removal.
  Existing deletion authority, receipts and unknown-effect protection remain.

The server still scans root metadata to count/filter/sort it; this is not a
persistent indexed query engine or an unlimited-scale performance claim.
Pages are live views. Expanded history stops automatic first-page replacement
until refresh, a filter change or a mutation. Ordinary conversations opened in
this browser remain bounded local references; job counts are not organization-
wide conversation totals. Older services retain their compatible list path.

## Verification

- Three initial regressions failed before fixes: attention starvation behind
  running tasks, completed chat previews and duplicate filtered-Activity reads.
  Logs: `build/ux-audit/activity-regressions-before.log`.
- Browser history/task suites: **67 passed**. After coalescing mutation reads,
  all **26** work-continuity tests passed again. Coverage includes exact counts,
  paging, partial deletion, arrivals after confirmation, linked identities,
  stale Stop responses, scoped logout/navigation and keyboard/narrow/RTL layout.
- An additional delayed-history/deletion test passed: a held pre-delete read
  cannot restore the removed row or its old history count.
- Narrow English and Arabic captures were inspected. Translation quality and
  assistive-technology usability still require human review.
- `go test ./internal/agents ./internal/server ./pkg/api`: passed. The first
  full run caught the newly added route missing from its frozen registration
  fixture; the fixture was updated for that route only and the full run passed.
- `TestWorkSummary*`: passed a **1,005-job** test, stable pagination, authorized
  counts, foreign-user exclusion, revoked sources, restart and protected children.
- Web API generation/check, TypeScript/build, source-size ratchet and strict
  `workspace-outcome-experience` validation: passed. Vite's existing large-chunk
  advisory remains. Desktop unit tests: **129 passed, 1 skipped**.
- Packaged Windows startup: passed (first window 1,423 ms; recovery 1,765 ms).
  Packaged appearance: passed saved/live theme, nine locale choices and zoom.
  These timings are fixture observations, not general performance guarantees.

Full repository CI, Windows race instrumentation, physical macOS/Linux testing,
participant testing and manual screen-reader/localization checks were not run
for this preview. This record does not close the wider UX release gates.

## Installation

The user requested installation and confirmed quitting the desktop. A process
check confirmed the installed application and inference worker had exited.
The application, desktop profile, configuration and workspace data were copied
to a recoverable backup outside the repository before the installer ran.
Existing model weights and native GPU runtime were retained in place.

The installer exited zero. Installed `app.asar`, bundled `offgrid.exe` and
`ui/index.html` matched the tested package hashes. All **391** recorded workspace
data-file hashes were unchanged. Private preservation manifests and rollback
paths are stored under ignored `build/ux-audit/activity-install-*`.

The installed executable then passed the isolated startup check (first window
1,954 ms; recovery 2,212 ms). No test desktop/service/model processes remained
after cleanup. The normal user application was not automatically launched.

## Subsequent source verification

After that installation, shared count ownership was tightened so an older
Activity page cannot replace a newer header count. Known ordinary conversations
are represented consistently in both surfaces, and a new ordinary turn cannot
inherit an obsolete job link. All **29** work-continuity tests then passed,
including delayed Stop/deletion reads, linked identity deduplication and
English/Arabic narrow layouts. TypeScript and API contract checks passed.
`go test ./...` also passed for the source tree. These subsequent count fixes
have not yet been included in the installed preview described above.
