# Design

## Context

See proposal.md for the increment and the wider program boundary. Existing
architecture and release authority remain in `docs/advanced/ARCHITECTURE.md`,
`docs/advanced/production-readiness.md` and the reliability evidence log.

Observed code at planning time:

- `internal/server/desktop_computer_tools.go` verifies application readiness,
  while `computer_tools.go` chooses that validator based on the last native tool.
  The runner then accepts a nonempty final answer as completed. This does not
  establish that every part of the user's compound request was covered.
- `internal/agents/runtime_tools.go` persists plans and checks that evidence IDs
  reference action steps, but does not validate the meaning of that evidence.
- `native_model_context.go` projects at most 24 control rows/4000 row bytes.
  Full saved evidence remains available through byte-paged `task_history`.
  The host itself also bounds views and text in `native_adapter.go`; saved
  history cannot recover source data omitted before persistence.
- `NativeElement` has role, name, text and writable/invokable/checkable properties,
  not a portable tree, ranges or geometry. The design must not invent those fields.
- `TaskResult.tsx` already distinguishes answer-only work. It is the current UI;
  the retired agent setup/result page must not return.
- Non-Windows application activation is explicitly unavailable. Existing macOS
  and Linux observation adapters do not imply task-wide launching, general
  pointer input or qualified platform parity.

## Goals / Non-Goals

Goals: add testable evidence semantics and efficient read-only control discovery
to the existing durable runner, without widening host authority or requiring a
new storage system. Keep semantics independent of application/product names,
English prompt matching, model families and the selected desktop platform.

Non-goals: a universal semantic judge, new native mutations, a second agent,
new screen capture, browser profile selection, arbitrary selectors/scripts,
new permission UI, automatic task replay, or production/platform certification.
The remaining program stages are not dropped; this change supplies foundations
they must use rather than claiming they are already implemented.

## Decisions

### 1. Evidence expectations are separate from progress and authority

Add an optional versioned workflow-evidence record to task/checkpoint persistence.
New task-first native workflows use version 1; existing saved tasks retain their
legacy version. The record includes the originating instruction identity,
revision, expectation IDs/descriptions, provenance, typed check parameters,
state, evidence step IDs and service-authored unavailable reasons.

Use one bounded declaration/update runtime operation alongside `task_plan`, with
at most 12 expectations and bounded strings. A simple open request needs one
expectation, not a fabricated multi-step plan. Read-only capability discovery may
precede declaration; the runner rejects a first mutation without the required
record before dispatch. Declaration is model interpretation unless explicitly
supplied by an authenticated caller. Neither source is permission to execute.

Once work starts, model plan edits cannot delete, weaken, or change the expected
value of an unresolved outcome. New expectations can be added. Actual user
steering creates a recorded revision at the existing safe boundary, retaining
previous expectations/evidence and marking superseded items explicitly. A page,
document, MCP result or model-authored sentence cannot act as user steering.

Reject the alternatives of English keyword detection and treating every
`task_plan done` item as proof. Also reject an unconditional second model judge:
it adds latency and is not independent evidence of external state. Model-proposed
coverage stays labeled as such; passing declared checks is not a guarantee that
the model interpreted the user's entire goal correctly.

### 2. Typed verification is evaluated from durable canonical results

Approved implementation refinement: native input element IDs change on each
capture. Add an optional host-issued evidence `identity` to native elements;
retain it only for the same OS control in consecutive observations of the exact
selected target. Windows uses UIA runtime identity, macOS AX object equality and
Linux AT-SPI object identity with retained references. Drop absent controls and
reset on selection, target replacement, close or worker restart. Ambiguous or
unavailable identities must not be inferred from labels, text or row positions.
Input still requires the current observation and its current element ID.

Declarations accept the current element reference; the service pins its evidence
identity and complete target from canonical saved observations, outside model
parameters. A newer read-back can satisfy the unchanged expected value only with
that binding. Older hosts without an evidence identity remain usable but report
cross-observation verification unavailable. This is additive evidence metadata,
not protocol-v3 desktop authority on v2 hosts or native-platform qualification.

Implement a small registry of read-only evidence evaluators. The initial kinds
are `application_open`, `control_text_equals`, and `control_checked_equals`, mapped
to the current supported host results. Check canonical arguments/results, target,
session boundary, ordered step IDs and successful dispatch/read-back; reject
forged/foreign evidence and relevant later mutations that invalidate a current
postcondition. Observation expiry for input is distinct from historical proof
that an earlier check passed; presentation includes check time and scope.

Research, saved documents, file transfers and external submission outcomes can
be declared but have explicit unsupported verification in this increment. The
model cannot downgrade a supported predicate to unavailable merely to finish.
The service decides support and generates the reason. A control-text check never
satisfies a saved-file check, and arbitrary MCP text never becomes native evidence.

Keep existing authorization, uncertainty and independent host verification
guards. Replace fragile reliance on the last raw tool name for new versioned
tasks with evidence evaluation over ordered canonical steps. Classify read-only
search/inspection and plan metadata explicitly: they neither invalidate a check
nor establish one. Preserve the existing legacy validator path for old records.

### 3. Completion repair is bounded and cannot replay work

Before accepting final output, evaluate declared expectations. Pending supported
work yields service-authored corrective feedback containing outstanding checks
and preserved progress, not a new task submission. Count consecutive completion
repairs durably, with three opportunities; valid progress resets the count, while
rewording a plan or repeatedly searching the same evidence does not. Exhaustion
uses the existing interrupted state with an actionable error code.

If unsupported predicates prevent independent verification, allow a clearly
partial/unverified result rather than an infinite loop. Retain legacy execution
status semantics and add `verification` to snapshots/events/exports with version,
check states, provenance, remaining work and overall `pending`, `partial`,
`unverified` or `checks_passed`. Never label this field `task_success`.

Unknown mutation outcomes take the existing reconciliation path before any
completion repair. No new repair condition authorizes retries, broader tools,
fresh consent, dispatch after Stop, or silent resumption after restart. Persist
the assessment and final result transactionally before publishing their event.

### 4. Read-only discovery uses saved evidence, not new host authority

Add `computer_find_controls` and `computer_inspect_control` as service-side
helpers over an explicitly identified canonical native observation. The first
supports literal case-insensitive Unicode name fragments, role and the existing
writable/invokable/checkable properties; the second returns one exact control
and Unicode-safe text pages. No regex, fuzzy identity resolution, scripts, paths,
DOM selectors, synthetic hierarchy or mutation fields are accepted.

Limit query strings to 256 UTF-8 bytes; return at most 16 matching rows and a
4096-byte complete serialized reply. Bound names/text with explicit indicators,
including an actionable size error if even the envelope cannot fit. A continuation
is a validated observation/query-bound cursor, never an arbitrary byte offset
into another task. Ordering follows the canonical observation. Exact short refs
resolve through the existing task-local mapping before canonical recording.

Read-only discovery is available only to the authorized owner of the active
native task/session, using its latest saved observation for the current target.
Reject revoked, replaced, switched, forged or foreign references. A time-expired
but otherwise permitted snapshot can be searched as historical evidence, with
age and `input_fresh: false`; it cannot pass as a new observation. After the task
ends, existing owner-authorized history APIs remain the way to inspect old data.

Keep `model_omitted` separate from `source_limited`. If the host dropped a subtree
or text, report incomplete coverage rather than a definitive no-match result.
Unknown evidence versions/security fields fail explicitly rather than being
silently removed by permissive JSON decoding. Search does not invoke host input,
focus, refresh, prepare, capture or a subprocess. Later fresh targeted observation
and tree navigation will require a separately implemented host capability.

This is deliberately more useful than raw 1024-byte JSON paging for finding a
known control while preserving `task_history` as the exact historical fallback.

### 5. One portable contract, honest platform reporting

Put matching, bounds and pagination semantics in shared Go code and run identical
fixtures for Windows UIA, macOS Accessibility and Linux AT-SPI. Compose these
helpers with existing authorized MCP and computer tools, and include their
schemas in actual prompt-budget tests. No per-model ratios or context increases.

Do not change `desktopComputerProtocol` or package qualification flags merely
because shared tests pass. V2 remains selected-window. Missing launch/capture/
Wayland backends stay missing, with accurate reasons. Read-only helper support
is not evidence that macOS and Linux task-wide control is implemented.

### 6. Current clients and verification evidence

Extend OpenAPI and generate TypeScript. Reuse current task progress, result and
history components to show concise checks and remaining work with expandable
details. Keep model output visibly separate from independently checked facts.
Cover all nine UI locales, keyboard navigation, RTL and narrow screens without
new access popups. Keep existing CLI JSON and Python agent snapshot methods
compatible; where they expose job snapshots, preserve the additive assessment.

Regression tests use canonical fixtures, controlled model responses and isolated
service state. An 8192-token continuation test must include the added schemas and
finding a relevant control beyond the initial compact rows. Synthetic success is
not model accuracy or an application workflow qualification. Measure added model
round trips and query size/time separately; declaring expectations has a latency
cost and must not be hidden in reported results.

## Risks / Trade-offs

- Model omits part of a goal -> retain the original instruction and provenance;
  never equate all declared checks with universal semantic completion. Frozen
  compound-request tests must include omitted or misclassified expectations.
- Stale or unrelated evidence looks persuasive -> exact typed validation,
  session/target checks, invalidation rules and unchanged dispatch enforcement.
- New schemas increase context pressure -> bounded schema exposure, complete
  prompt accounting and 8192-token regression; no silent truncation of instructions.
- Legacy clients only understand execution status -> preserve the old meaning,
  document additive assessments and never rewrite old results as newly certified.
- Source truncation prevents discovery -> show incomplete coverage; do not fetch
  protected or unobserved content through a purported read-only helper.
- A shared test looks like platform parity -> keep current platform capability
  gates and record contract tests separately from real OS evidence.

## Migration Plan

Add optional fields through existing task serialization and SQLite adapters;
test old snapshots, new snapshots, interrupted writes and reload. Do not replace
the workspace store or backfill old tasks with model-generated expectations.
Omitted caller fields must retain existing request hashes and idempotency;
explicit expectation changes conflict with reuse of the same request ID.

New readers refuse dispatch for unknown evidence versions and show them as
unsupported. Previously released binaries cannot be retroactively assumed to
enforce new fields: downgrade while versioned work is active is unsupported.
Before any later authorized rollback, stop active work and retain its records;
never restore old grant or host-journal state to resume it.
Implementation checks use disposable state. Container and desktop deployment,
signing, release and native personal-app tests require their own explicit scope.

## Required Subsequent Program Stages

This sequence records dependencies, not implementation completion or new native
test authorization. The full requested program remains open after this increment.

1. Cross-platform structured workflows: implement macOS and Linux task-wide
   activation/target lifecycle, text ranges, selection, menus, scrolling and
   bounded state waits; qualify Windows, macOS Intel/Apple Silicon, X11 and
   GNOME/KDE Wayland separately. Add real providers, not capability flags.
2. Useful browser/document/file work: browser search with cited sources, document
   saves/exports, spreadsheet ranges/formulas, approved downloads/uploads and
   scoped host file operations; each adds typed outcome verifiers here. Expand
   structured policy beyond English keyword matching before adding broader input.
3. Local visual fallback: platform capture/input, screenshot-bound grounding,
   multi-monitor/DPI transforms, protected-surface handling and independently
   tested local vision profiles; no automatic model downloads.
4. Shared presentation and qualification: canonical Chat/Agents jobs, safe-boundary
   steering and recovery, measured 8 GB/16 GB CPU and acceleration profiles,
   installed/upgraded packages, offline Stop, frozen workflow repetitions and the
   existing soak/pilot/release gates. Reuse the established reliability log.

Each stage needs its own reviewable OpenSpec change and tests; no changes are
added to the voice-model foundation or presented as a generic completed roadmap.
