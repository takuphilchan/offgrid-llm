# Proposal

## Why

The approved native-agent program has durable interactive jobs but no owned
calendar schedules or persistent occurrence admission. A timer must not turn a
restart, missed slot or unavailable host into duplicated or newly authorized work.

## What Changes

- Add user-created actor/workspace-owned daily/weekday calendar schedules with
  an explicit IANA timezone, immutable profile/resource pins, revisioned enable/
  disable controls and occurrence history.
- Admit each due occurrence once into the existing durable runner and inference
  scheduler. Persist its identity before job creation or dispatch; retain canonical
  results through Activity rather than opening a second conversation history.
- Recheck current actor policy, pinned profile/resources and execution grants.
  A schedule cannot restore desktop consent, mint a folder grant or silently
  select another host, model or profile revision.
- Make missed, blocked-host, overlapping and interrupted occurrences explicit.
  No catch-up replay after downtime, overlap while prior work is unsettled, or
  automatic restart of an uncertain occurrence.
- Expose owned service/CLI/shared Activity controls; no model-facing scheduler
  creation tool, OS wake-up, cloud scheduler, automatic downloads or deployment.

Acceptance: a reviewed isolated task runs once at its calendar slot; duplicate
ticks and restart do not create another root; DST gaps skip and repeated wall
times occur once; disabled/revoked/disconnected schedules do not dispatch; missed
slots remain history rather than executing on reconnect. UI inspection never
submits or resumes work.

## Capabilities

### New Capabilities

- `agent-occurrence-scheduling`: owned calendar policies, pinned authority,
  durable once-only admission, recovery and Activity/client delivery.

### Modified Capabilities

None. Existing execution, outcome evidence, approvals and continuity protections
remain independently enforced by the canonical runner.

## Impact

Depends on `native-agent-workspace-execution` and `native-agent-skills-memory`.
Extend application-owned agent SQLite, its coordination lifecycle and borrowed
service admission hooks; add authenticated API/client contracts and shared
Activity management. Existing jobs, profiles, conversations and native grants are
not migrated into enabled schedules. Local inference and current budgets remain.
Real native/model unattended qualification is separate from clock/storage tests.
