# Calendar policies

This package handles calendar data and stable occurrence identity. It is not an
agent executor, model scheduler, shell cron adapter or source of permissions.

Calendars name an explicit IANA zone, hour/minute and distinct weekdays (`0` is
Sunday, `6` Saturday; all seven means daily). `Local`, unknown zones, duplicate
days and invalid times are rejected. Go timezone data is bundled for offline
fallback. Calculation is independent of the service machine's timezone.

Missing DST wall times are skipped. Repeated wall labels select their first real
UTC instant only, including non-hour transitions such as Lord Howe. After that
instant passes, moving through the second fold never yields another occurrence.
Future lookup is bounded; it does not enumerate a downtime backlog. Stable IDs
bind schedule ID, revision and UTC slot regardless of display timezone.

The application must separately persist claims, enforce current grants, avoid
overlap and record missed slots rather than catching up. A calendar result is not
permission to dispatch, proof of completion or an external exactly-once guarantee.

The agent manager owns schedule snapshots and occurrence records in its existing
SQLite database. Up to 128 schedules per actor/workspace keep the management
inventory bounded; revise an existing policy rather than silently losing rows
from the list. Profile/model/budget/resource snapshots are immutable and
digest-checked. Updates use an expected revision; disabling advances the live
revision and re-enable requires complete resource review. Saving or inspecting a
policy never creates a job. Corrupt versions, digests or off-calendar cursors
are not execution candidates. Historical revisions and occurrences are retained;
disabling does not undo work already dispatched or erase its evidence.

Each slot is journaled before root creation and linked before dispatch. Cursor
advance and claim are atomic. Duplicate ticks and clock rollback cannot repeat
a recorded UTC slot, even through a new policy revision. Downtime/forward jumps
produce a missed-range record, not backlog jobs. Unsettled roots block overlap.
Restart reconciliation observes stable request/job identities only: it marks a
pre-admission claim missed or links an interrupted root for inspection, never
creates/resumes a replacement. Terminal rows do not settle while an owned worker,
child, process or unknown call remains active. These records describe mechanical
admission and settlement, not independent model-quality verification.

The application-owned runner invokes the service admission callback alongside
its normal job/child coordinator. Only one preparation is in flight, with a
30-second cancellable bound; it cannot starve queued child work. Shutdown closes
admission, cancels preparation and drains it with the other runner workers. A
callback ending does not cancel an already admitted root. Logical calendar jobs
still share normal inference slots and aggregate budgets.

Public management is capability-gated and account/workspace scoped. Activity and
`agent schedules` use the same APIs. No client has a timer/executor, run-now API,
or authority to restore a native grant. The service rechecks live resources and
refreshes bounded Knowledge only under pinned document IDs before claiming a
slot. Data changes cannot repin profile/model/tools/permissions/budgets. Owned
inspection is separate from dispatch, and unknown effects remain unreplayed.
