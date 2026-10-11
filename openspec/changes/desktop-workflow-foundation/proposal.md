# Proposal

## Why

OffGrid can open and inspect applications, but application readiness is not proof
that a compound request was completed, and bounded model views can omit the
control needed for the next step. The next independently reviewable increment
must make progress evidence precise and control discovery useful across existing
native adapters before adding more input authority.

## What Changes

- Persist a versioned workflow-evidence contract for new task-first native work,
  separate from execution status, human-readable plans and host permission. Track
  expected checks, their provenance, supporting evidence and unverified outcomes.
- Validate supported checks against canonical successful host results. An
  application-open check cannot satisfy research, document-save, file-transfer or
  form-submission outcomes. Model prose and a plan item marked done are not proof.
- Add host-issued control evidence identities across consecutive observations,
  separate from expiring input references, so fresh read-back can verify the
  same control without guessing by label or retaining stale action authority.
- Give unfinished declared work bounded corrective feedback in the existing
  runner. Preserve completed actions and results; never replay an uncertain action
  or start another job to recover a completion check.
- Add read-only, bounded control search and inspection over authorized canonical
  observations, with stable references, Unicode support, explicit omissions and
  freshness information. Finding a control never refreshes or grants input rights.
- Describe these helpers through one shared contract and test Windows UIA, macOS
  Accessibility and Linux AT-SPI observations. Do not advertise task-wide launching,
  visual input or Wayland input where no implementation exists.
- Expose evidence and remaining work through existing jobs/events/export and the
  current Agents result view. Keep the current renderer, monochrome controls,
  nine locales, voice and MCP composition; do not resurrect legacy pages.

### User-visible acceptance

1. A declared open-and-search request that only opened the browser cannot be
   presented as fully verified research; the saved task identifies the remaining
   outcome without relaunching the browser.
2. A requested control omitted from the initial model view can be found and
   inspected when it exists in the saved canonical observation. If the host itself
   omitted it, the result explicitly says the search is incomplete.
3. A genuine launch-only check and supported text/checkbox read-back checks retain
   their narrow verification meaning, without requiring an artificial long plan.
4. Restart, context compaction, steering and reconnect preserve evidence and do
   not submit, approve, grant access or dispatch input automatically.
5. The same read-only observation contracts work for existing native driver
   shapes; unsupported host functions remain explicitly unavailable.

### Program boundary

This is the first stage of the computer-use improvements requested on 2026-10-10,
not a replacement for the full request. Subsequent independently reviewable
stages remain required: cross-platform task-wide launching and richer structured
controls; browser/document/spreadsheet workflows and scoped host files; local
visual fallback; and installed-platform/model/latency qualification with shared
Chat/Agents presentation. Each needs its own concrete acceptance requirements
before implementation. Platform qualification runs alongside each feature.

This increment does not implement new native mutations, broaden consent, change
English-keyword policy into a claimed general security sandbox, implement OAuth
or canonical Chat/job linkage, or claim semantic proof for arbitrary user goals.
It does not add models, capture personal screens, control personal apps, deploy,
replace installations, commit, push or release. Existing broad-program scope is
not marked complete by this change.

## Capabilities

### New Capabilities

- `task-outcome-evidence`: Durable, provenance-aware checks and truthful completion
  assessment through the existing task runner and clients.
- `native-control-discovery`: Bounded read-only search and inspection of scoped
  native observation evidence, independent of platform and model family.

### Modified Capabilities

None: the maintained spec inventory is currently empty. This change builds on
`task-wide-desktop-access` and `agent-capability-composition` without reopening
their completed tasks or expanding the voice-model change. Their consent,
composition, replay and platform limitations remain controlling.

## Impact

Affected areas: `internal/agents` task/checkpoint/persistence/progress handling;
`internal/server` native tool composition, projection, completion and job views;
`internal/computer` shared observation contracts; `pkg/api/openapi.yaml` and
generated TypeScript; current task-result/progress UI and translations; applicable
CLI/Python job projections; focused Go, browser and companion contract tests.

Update the existing API, computer-task and companion documentation. Record actual
verification in `docs/advanced/product-reliability-plan.md`; release claims remain
governed by `docs/advanced/production-readiness.md`. No new runtime dependency or
workspace storage replacement is required. Preserve legacy records and APIs with
additive versioned fields and conservative unknown/unverified fallbacks.
