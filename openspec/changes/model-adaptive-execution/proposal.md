# Proposal

## Why

The existing durable runner already supports constrained single-step output and
bounded context, but a computer smoke check is not a measured runtime capability
record or workflow qualification. Repeated checks can compete for local inference,
and profiles do not yet provide explicitly approved safe-boundary model escalation.

## What Changes

- Freeze a versioned, thirty-workflow evaluation manifest before behavior changes;
  distinguish fixture execution from real model/platform qualification.
- Deduplicate concurrent synthetic checks without caching permission or carrying
  success across runtime changes. Cancellation of one observer must not cancel
  checks still requested by others; cancellation of the last observer releases work.
- Record runtime-fingerprinted capability evidence with separate unmeasured,
  passed, failed, stale and workflow-qualified states. Operational unavailability
  is not incompatibility. No check dispatches a host tool or uses private prompts.
- Normalize supported native and constrained response adapters behind the existing
  runner contract; never infer executable calls from prose or incomplete streams.
- Add optional profile-approved local fallback/perception models and guided or
  adaptive execution. Existing profiles keep their primary model and gain no grants.
- Enforce root budgets and safe-boundary switching, at most two model switches and
  three consecutive corrective attempts across models, through existing persistence.
- Reuse inference admission for parent/child/perception work, preserve CPU and GPU,
  default to one generation on unqualified hardware and measure queue/load/generation
  separately. Do not install models or silently fall back to cloud inference.
- Replace the fixed normal-operation context clamp with model- and hardware-aware
  context admission. Treat the configured maximum as an operator ceiling, pin the
  admitted window for each task, and report that same allocation to external agents.

### Boundaries and acceptance

This owns model adaptation, not a replacement runner, native driver or UI. Typed
delegation belongs to `agent-conversation-workflows`; document providers remain in
`native-document-workflows`; routing/visual/files belong to the approved dependent
`verified-desktop-workflows` change. No deployment or publication is included.

Acceptance requires cancellation-safe shared probes; runtime changes invalidating
evidence; busy/timeout outcomes remaining operational; no host effects from probes;
safe local escalation without budget reset or uncertain replay; and unchanged
ordinary conversation. Capable machines and models can admit long-context work while
constrained or unknown configurations stay on a smaller tier; active tasks are never
silently resized. Qualification requires the frozen suite on real hardware, not
successful contract tests or capability declarations.

## Capabilities

### New Capabilities

- `measured-model-capabilities`: Runtime-bound evidence and bounded synthetic checks.
- `local-adaptive-execution`: Normalized adapters, approved escalation and shared budgets.
- `agent-workflow-evaluation`: Frozen workflow definitions and honest qualification records.

### Modified Capabilities

None. The published main specification inventory is empty. Existing in-flight
contracts remain controlling for authorization, effects and conversation identity.

## Impact

Existing inference/runtime admission, model registry, agents/profile persistence,
service endpoints, OpenAPI/generated clients and compact profile/Chat projections.
Tests use isolated state and synthetic inference; native effects require separate
explicit test scope. Actual evidence stays in the existing product reliability plan.
