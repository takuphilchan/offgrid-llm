# Design

## Context

The owned HTTP engine already normalizes schema-constrained single-step responses.
`checkComputerModelDriverWithBudget` performs a synthetic two-turn smoke check,
with model admission before the probe deadline. It currently repeats this work
for concurrent callers. The lifecycle gate serializes model loading/generation.
Profile revisions and durable task checkpoints already exist; no second runner is needed.

## Goals / Non-Goals

Build measured local adaptation on these components. Preserve existing response
fields, ordinary Chat, effects reconciliation and CPU/GPU admission. No native
driver implementation, deployment, secret capture or performance claims from mocks.

## Decisions

### Shared active checks, not a successful-result cache

Use a per-service, keyed in-flight coordinator around the synthetic tool probe.
The key includes model, normalized driver and probe budget. A check owns its own
cancellable context and waiter count. One waiter leaving cannot cancel others;
the last waiter cancels work and detaches it from the map. Completion deletes only
its own map entry, publishes one immutable result, and releases admission. A later
call always re-probes. A global result cache would incorrectly extend validity
across runtime/context changes and is deliberately rejected.

Expose measured admission and probe durations additively without raw runtime
responses. The probe time budget starts after admission. A cancelled caller gets
an operational cancellation result, not the eventual shared compatibility result.

### Evidence and adaptation

Native agent tool support uses a service-owned qualification policy, not a model
name allowlist or a public request flag. A trusted runtime owner supplies the
current measured fingerprint and suite/platform target; the existing capability
evidence store must report matching workflow qualification. Admission pins the
adapter revision, evidence ID and fingerprint ID in the durable job. No pin means
the existing constrained adapter. Before counting or generating, revalidate under
the held inference lease; missing, failed, changed or superseded evidence stops a
pinned job without silently changing adapters or replaying effects. Both response
paths use the complete-call validator and existing dispatch/approval boundary.
Startup policy installation cannot modify saved jobs. Hardware/workflow evidence
remains distinct from implementation and isolated skill/fixture passes.

Separate installation-target resolution at job admission from live-runtime
measurement under the inference lease. Admission must not load a model merely
to select an adapter. Before counting or generating, both the still-selected
installation target and the active process must match the persisted evidence
pin. Missing live measurement or an operational timeout stops that decision;
it does not turn an installation declaration into qualification or invalidate
otherwise unchanged historical evidence.

Persist versioned records outside model context. A complete fingerprint includes
verified model/projector content identity, runtime/template/adapter/context and
backend configuration; missing identity never qualifies a profile. Preserve past
evidence while projecting changed identity as stale. Probe results and workflow
qualification are separate record kinds. Invalid/unknown versions are non-executable.

Profile execution settings are immutable revision data. Existing revisions remain
unchanged. Approved local fallbacks are resolved/pinned when a task is admitted;
switching requires a durable safe boundary and never resets root counters. Guided
operations still use broker validation, receipts and the existing durable runner.

Keep one inference admission authority. Reserve parent verification/finalization
budget before child allocation. Adapt schema/context projection to measured token
capacity, never by erasing constraints. Queue, loading and generation timings are
distinct from tool/capture latency; no unmeasured parallel GPU default.

### Context is planned per model and pinned per task

Interpret the configured maximum context as an operator ceiling. In adaptive mode,
intersect that ceiling with the model's trained GGUF context, current power/pressure
limits, and a conservative memory estimate based on model size, KV dimensions, cache
quantization and inference slots. Round down to stable tiers so small fluctuations do
not cause reload churn. Missing model or hardware evidence uses a conservative tier;
it never invents long-context readiness. Explicit fixed mode still bypasses the
hardware estimate but cannot advertise beyond a known trained model limit.

Pin the selected tier in admitted task configuration. The existing lifecycle gate
treats a context change like a model switch, drains active leases, and reloads at the
new tier. It therefore remains the sole safe boundary: pressure changes can affect a
later admission but cannot shrink a running task. Model discovery and integration
setup project the selected model's planned tier, while health distinguishes the
configured ceiling from the currently loaded runtime allocation.

A single global adaptive window and hot in-place resizing were rejected. The former
misrepresents heterogeneous models; the latter can invalidate active KV state and
make external-agent configuration disagree with the runtime.

### Frozen evaluation manifest

Native-tool selection may alternatively use a versioned conversation/skills/
workspace qualification scope. Its immutable suite and reviewed tool inventory
define the boundary, not a provider's annotations or the model's claims. Every
case runs three times for the same measured model/runtime; relevant known failures
must pass, with zero unauthorized actions, repeated uncertain mutations or false
completion. Record per-case results, source artifact identity and failures; a
synthetic aggregate or isolated skill pass cannot qualify the scope.

Persist the scope with the adapter/evidence pin. Filter both initial menus and
discovery, and independently check the scope before every dispatch, including
runtime tools and resumed checkpoints. Check registered capability identity as
well as tool name. Existing permission, profile, folder and approval boundaries
remain additional restrictions. Unknown scopes fail closed. Out-of-scope work
cannot switch adapters mid-task, delegate to escape the boundary or use a desktop
grant to expand qualification. Explain the limitation without claiming completion.

Evidence also binds runtime bundle bytes and actual launch configuration. Changes
to either invalidate reuse even when build labels are unchanged. Historical
build-only records remain inspectable. Full-suite qualification remains required
for desktop, visual and broader release claims; scoped evidence must not project
as general workflow qualification.

Store public synthetic prompts and deterministic acceptance checks in a versioned
test fixture. Six families each contain five cases. Every case declares setup,
required evidence, forbidden effects and whether an isolated native desktop is
required. A fixture-validation test proves only manifest structure. Baseline and
qualification results are separate evidence; missing machines do not count as passes.

## Risks / Trade-offs

### Reviewed skill response contracts

Newly imported reviewed skill revisions may declare `response_contract` in
SKILL.md frontmatter: version 1, a bounded RE2 text pattern and maximum UTF-8 byte
length. Match the complete answer, not a substring. Reject unknown versions,
unknown contract fields, invalid patterns and excessive limits at import. No
scripts, model-generated validators, external references or network access.
Persist the parsed contract in immutable bundle metadata and profile/task skill
metadata. Old stored bundles with no reviewed contract field do not acquire one
merely because a previously inert frontmatter key is now understood. A draft
cannot activate a contract; the normal explicit import/revision review applies.

Only a successfully loaded, exactly pinned skill can constrain completion. Check
all active contracts after existing effect and workflow verification. On mismatch,
persist a correction counter and concise failure feedback; retain the rejected
answer in history but do not publish it as completed. Correction has no tools,
and the dispatch boundary independently refuses calls, including restored calls.
Allow at most two corrective answers within existing iteration/token/time limits;
failure remains explicit without replaying any previous effect. Cancellation and
restart preserve the counter. Conflicting contracts fail within the same bound.
Ordinary skills and earlier jobs keep their behavior. Formatting verification is
not semantic quality verification or permission to change user instructions.

Test the reviewed checklist revision separately from the unchanged legacy
fixture. Record both failures and new-contract results; changing the frozen
qualification fixture requires a new suite identity, never retroactive passes.

- Shared cancellation races -> deterministic concurrent tests and race coverage.
- Stale model evidence -> content/runtime fingerprints, never names alone.
- Model switching costs -> measured local scheduling and bounded configured chains.
- Conservative memory estimates can underuse capable hardware -> expose the operator
  ceiling/fixed-mode escape hatch and keep estimates deterministic and testable.
- Resource changes after admission -> pin active work and apply new limits only at a
  later lifecycle boundary rather than invalidating live KV state.
- Broad program hidden by a small task count -> separate implementation and real
  qualification tasks; keep dependencies on the other approved changes explicit.

## Migration Plan

Use additive API fields and versioned storage. Do not modify installed models,
permissions or persisted old profile revisions. No automatic task resume. Existing
clients retain the old model-check result fields. Deployment and rollback remain
separately authorized operations; this change does not replace the live service.
