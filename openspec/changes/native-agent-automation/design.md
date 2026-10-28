# Design

## Context

See proposal.md. The agent manager owns operation-scoped SQLite connections and
canonical tasks. The runner's existing 200 ms coordinator admits child jobs but
does not replay interrupted work. The application owns that coordinator lifetime;
the HTTP layer borrows it and already resolves owned profiles, current actor
policy and execution resources. See the execution and skills-memory change
designs for grant, context-resource and persistence boundaries.

## Goals / Non-Goals

Add a durable calendar/admission policy, not another model engine or task loop.
Support explicitly reviewed local and isolated host grants without describing
trusted local commands as confined. Do not create OS services, wake computers,
download runtimes, restore desktop sessions or give models a self-scheduling tool.

## Decisions

### Typed local calendars rather than shell cron

Use a bounded typed calendar: IANA zone, hour, minute and selected weekdays (all
seven means daily). Reject host-dependent `Local`, malformed/unknown zones and
invalid days. Bundle Go timezone data for offline operation. Compute the first
real UTC instant matching each candidate wall-clock date; nonexistent dates/times
are skipped and repeated wall labels select their earliest instant. Store next
UTC time, not an ambient-machine cron expression. This keeps interpretation
portable and testable without shell commands or a new dependency. An injectable
clock drives tests for UTC, Harare, New York and non-hour DST transitions.

### Manager-owned immutable revisions and occurrence journal

Add owned schedules, immutable revision snapshots and occurrence tables to the
existing agent database. A revision stores reviewed name/prompt, calendar,
profile snapshot, model, budget and execution reference/resource snapshot.
Default off for existing data; explicit creation enables only the submitted
policy. Updates require CAS and a complete resource review; disabling advances
the live revision and does not rewrite an active root. No credentials are saved.

Use an occurrence key derived from schedule ID/revision and canonical UTC slot.
Claim the slot and advance the next future slot in a transaction before creating
work. Use a stable payload-bound runner request identity, attach its canonical
job ID durably before `Continue`, then dispatch through the same runner. No
separate session transcript, executor or inference budget is created.

### Existing coordinator and application ownership

Add an optional bounded coordination hook, configured through borrowed service
admission hooks before the application's existing coordinator starts. Coalesce
clock polling; do not read full task transcripts every 200 ms. The hook creates
only due roots and uses asynchronous runner admission, not inline generation.
Shutdown cancels the shared coordinator before provider teardown; no timer can
start tasks after runner admission closes. SQLite CAS/unique keys protect against
duplicate callers as well as repeated ticks.

### Recheck authority without repinning it

User-only schedule APIs reuse profile resolution and current actor/Chat policy.
At occurrence admission, validate the original pinned profile revision, model
readiness, skill digests, memory consent pin where present and execution resource
identity. Do not call a resolver that replaces these with current defaults.
Every scheduled tool dispatch also checks its live schedule revision, including
registry, context and computer paths. Schedules cannot request or restore
computer access. A selected execution resource remains subject to independent
host grant checks. Existing routine-command and per-call approval rules remain;
an unattended job awaiting approval is blocked work, not permission to auto-approve.

Pinned document identity does not make cached excerpts fresh. Before claiming a
slot, refresh bounded Knowledge context through the existing retrieval helper
under those exact IDs and current actor permission. Persist the resulting
payload digest in the occurrence before root creation. The manager permits this
data refresh only when all authority/model/budget/profile fields are unchanged;
it cannot become a resolver that repins resources. Recovery still never creates
a root from newly retrieved data after a crash.

### No catch-up, overlap or uncertain restart

Only a slot reached during the current service lifetime and within its clock
minute is eligible. On startup/forward jumps, persist a missed-range record and
advance to a future slot; never drain old slots in a loop. A duplicate/backward
tick cannot reclaim the same key. An earlier root that is active, waiting,
interrupted or uncertain blocks later occurrences, including new revisions.
Terminal status alone is not sufficient if a worker is still settling effects.

Recover claimed/admitted occurrences by observing stable runner request/job
identities. Link known roots for inspection, mark interrupted where appropriate
and never call `Continue` automatically for old work. Missing linkage after a
crash remains explicit and cannot be repaired by creating a new request. Disabled
or expired grants remain disabled. Skipped/blocked slots are never revived when
a host reconnects. Explicit existing root reconciliation/stop remains the path
for unsettled work, not a schedule-level replay button.

### Additive owned clients and Activity delivery

Add authenticated schedule CRUD/history APIs and CLI commands using the same
service client. Shared Activity provides progressive schedule management and
links to canonical job inspection. The editor shows calendar zone/time, profile
revision, host/folder/backend and budgets before an explicit save. Inspection,
refresh and navigation never run a schedule. Keep nine locale controls, keyboard
access and narrow/short layouts; do not resurrect the retired task composer.

## Risks / Trade-offs

- Clock jumps or a busy service can miss a slot → record missed/blocked work,
  never trade honest no-replay behavior for hidden backlog execution.
- Uncertain old effects can block future work → link the actual root and retain
  existing reconciliation instead of bypassing it through a new revision/host.
- Trusted local commands are not a sandbox → disclose this during review and
  support reviewed isolated Linux execution with the same negotiated provider.
- Timezone rule updates can change future slots → preserve recorded UTC
  occurrences and expose the zone; test rule interpretation separately from
  native/model qualification. Do not rewrite historical evidence.
- Resource changes can make an enabled schedule unusable → surface a precise
  blocked reason; never silently switch resources or widen access.

## Migration Plan

Add tables/optional task provenance fields and capability-gated APIs. No old job
or profile becomes scheduled. Unknown stored versions cannot dispatch. Rollback
retains history and does not promise older software can execute new schedules.
No deployment or installed-state replacement is part of this implementation.
