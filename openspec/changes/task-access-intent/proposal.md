# Proposal

## Why

Recent real-model tests returned instructions instead of executing the requested application work, and a rejected access proposal was subsequently reported as completed. Approval policy is also chosen midway through execution rather than with the task.

## What Changes

- Persist the proposed computer approval policy with new task submissions, including request deduplication; present it in the current composer before Start.
- Carry that policy into local consent and reject a mismatched attached session. Choosing a policy is not host consent and does not widen an existing grant.
- Prevent unresolved pre-dispatch failures from becoming successful completion merely because the model returns prose; retain bounded recovery.
- Expose text-only results as answers without execution evidence, rather than presenting them as completed external work.
- This is the first independently reviewable access-intent slice of the approved task-wide program. Protocol-v3 desktop grants, durable application launch, cross-application input, and matching installed host qualification remain required follow-on work; this slice must not claim to remove window-bound host authorization.

## Capabilities

### New Capabilities

- `task-access-intent`: Persisted proposed access policy and honest task outcome presentation across submission, recovery, and local consent.

### Modified Capabilities

None. The repository has no archived main specs. Existing access-recovery and capability-composition changes remain intact.

## Impact

Go durable runner and job API, generated TypeScript contracts, current React task composer/access/result surfaces, nine locales, tests and agent/computer guides. No deployment, host input, model downloads, commits, or publication are part of validation.
