# Activity archival and management polish

The owner explicitly approved archiving seven historical test runs whose effects
remain unresolved. Archival is a presentation state, separate from task outcome,
deletion and execution. No successful outcome or reconciliation is manufactured.

## Implementation

- Owner-scoped, idempotent archive/restore commands persist `archived_at` in the
  existing durable task snapshot. Active workers, active children and pending
  approvals reject archival. Restoring does not resume execution. An archived
  root cannot resume, accept steering or acquire computer access.
- Normal Activity buckets and attention counts exclude archived roots. The
  paginated Archived view retains their original outcome and inspector link.
  Stale reads are invalidated after mutation. Legacy empty-owner records are
  available only to local admin, consistent with history cleanup.
- Compact menus use ordinary action rows; model inventory no longer moves or
  changes its label when catalog/transfer metadata arrives. Tools can be filtered
  without changing configuration. Knowledge rows are flatter. Settings uses a
  gear; service status is inside its panel instead of an unexplained footer dot.
  Schedules no longer adds a second divider. A stopped Chat offers history
  recovery instead of another Stop action.

## Verification

- `go test ./internal/agents ./internal/server -count=1`: passed both packages.
- Archive tests cover ownership, active workers/children, pending approval,
  storage failure, retained unknown effects, idempotency, restart, restore without
  execution, JSON command validation and archived/normal count separation.
- Seven targeted Playwright files: 93 passed, including archive/restore,
  delayed-count and Stop races, paging, menu dimensions, stable model rows and
  localized narrow layouts. An additional screenshot pass: seven passed.
- TypeScript, API generation/check and production web build passed. The build
  retains its existing large-chunk warning.
- Desktop Node tests: 134 passed, one platform-dependent skip.
- Packaged Windows startup/recovery passed (first window 867 ms, recovery 1107 ms
  in the isolated fixture). Packaged terminal checks passed, including CLI/Agent,
  tabs, Unicode, cursor retention, 200% zoom and close/quit child cleanup.
- Maintained documentation links and source-size checks passed. The manager size
  exception increases by one line for the additive persisted field; archive
  behavior resides in its own file. OpenSpec validation passed.
- The local race invocation could not run because this Windows environment has
  CGO disabled and no C compiler on PATH. Race coverage remains a CI check.
- Screenshots inspected: dark/light Archived view and compact model menu.
  Manual screen-reader, human usability, translation and other native platform
  qualification were not performed.

## Packaging correction

The first candidate was rejected by the bundled-service identity check: this Go
build's `-trimpath` omitted linker flags from build metadata. Rebuilding with an
explicit `main.Version=0.4.27` and retained linker metadata passed the check.
Desktop CI examples now use the same invocation as release packaging. The check
was not weakened; the rejected candidate was not installed.

Subsequent installation evidence is recorded in the
[branding follow-up](branding-speech-chrome-2026-11-06.md). This source-test record
does not itself establish which installed records were archived. The final full
renderer and Linux race reruns are recorded in the
[November 7 verification](terminal-session-followup-2026-11-07.md).
