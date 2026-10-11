# Proposal

## Why

The observed `open chrome` task failed before dispatch because a managed-browser access proposal omitted its URL. The access validator reported permission denial, while the durable runner repaired malformed proposals only after a computer session existed. A recoverable model error therefore became a terminal task failure.

## What Changes

- Return actionable, service-authored validation errors for malformed access proposals without creating consent or executing input.
- Repair eligible pre-dispatch proposals through the existing durable model loop, with at most three consecutive repairs and a persisted budget.
- On exhaustion, pause the same task with no pending rejected calls or approvals; permit explicit correction/resume without silently repeating an action.
- Preserve hard denials, ownership checks, local consent, and uncertain-effect reconciliation. Do not reinterpret old failed tasks as safe based only on their display message.
- Clarify that opening a named installed application uses app access, not managed-browser access requiring a URL.
- Preserve the current checked-out shared renderer, layout, monochrome styling, and voice controls. Historical screenshots are diagnostic context, not a UI baseline.

This is stage 1 of the approved task-wide computer/shared-tools program. Subsequent independently reviewable changes still must deliver capability composition; durable Chat/job linkage; authenticated shared connections; protocol-v3 desktop authority and enrollment; cross-platform controls, vision, files and verification; and installed-package/model/platform qualification. This slice does not enable or advertise those later capabilities.

## Capabilities

### New Capabilities

- `agent-proposal-recovery`: Durable, bounded repair of explicitly classified pre-dispatch proposal errors without bypassing authorization or replaying uncertain effects.

### Modified Capabilities

None; the repository has no published main specifications yet.

## Impact

The Go access validator, durable runner/checkpoints, focused server/runner regressions, and existing agent/computer-task guides. No new dependency or database replacement. Additive private checkpoint fields preserve old snapshots. No deployment, model downloads, native application effects, commits, or publication.
