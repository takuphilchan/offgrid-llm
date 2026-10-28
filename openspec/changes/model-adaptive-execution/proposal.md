# Proposal

## Why

The existing durable runner already supports constrained single-step output and
bounded context, but a computer smoke check is not a measured runtime capability
record or workflow qualification. Repeated checks can compete for local inference,
and profiles do not yet provide explicitly approved safe-boundary model escalation.

## What Changes

- Add explicitly installed, qualification-gated native agent tool selection for
  new jobs. Pin the adapter and evidence before inference, revalidate the measured
  runtime under the existing lease, and retain the constrained path for old or
  unqualified jobs. The reported skill loop motivates this integration; isolated
  skill passes alone do not establish general workflow qualification.

- Add capability-scoped native-tool qualification for conversation, skills and
  workspace operations. Freeze a separate suite, run every case three times per
  model/runtime, retain failures, and enforce its reviewed tool subset at discovery
  and dispatch. Scoped evidence does not qualify desktop, vision, delegation or
  broader release claims; those retain the thirty-workflow gate.

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

Add optional reviewed skill response contracts to address measured output-format
failures without silently rewriting answers. New skill revisions may declare a
versioned, bounded text contract; persisted reviewed metadata pins it. Loaded
contracts are checked by code before completion. At most two answer-only
corrections consume the existing root budget, with no tool dispatch or replay.
Old skills and revisions remain unchanged. A format pass does not establish
factual accuracy, tool success or general model qualification.

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
