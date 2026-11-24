# Durable adaptive model switching — 2026-11-22

This increment implements root-task switching for typed pre-dispatch model
decision rejection under an admitted adaptive policy. It advances M3.06/M5.06
and `model-adaptive-execution` 4.2–4.3; those tasks remain incomplete for delegated
switching, additional corrective paths, perception and real-model qualification.

## Runtime behavior

The runner now chooses the next unused configured local fallback after the
owned response validator rejects a complete decision before dispatch. Guided
mode and jobs without a policy keep their existing selection. No model names,
tool output, generic error strings or network failures classify retry safety.
Joined operational/persistence errors do not qualify as repairable decisions.

The existing task transaction saves the rejection, cumulative counters, switch
history, selected model/context and exact evidence pin together before another
context count or generation. There is no additional runner or state database.
At most two switches are recorded; the same unresolved sequence stops after
three rejected decisions across models. Explicit Resume does not replenish that
sequence. Root selection never consumes a delegated profile's pool. Child
automatic switching remains unimplemented rather than gaining implicit grants.

Both candidate and current pin must be valid, with identical measured scope and
capability inventory. Counting and generation retain the existing inference
lease and live qualification revalidation. Missing/changed/unavailable fallback
resources stop the attempt; no download, cloud fallback or constrained-adapter
substitution is introduced. Pending input/approval, unsettled calls/receipts,
control/Stop and failed persistence cannot publish a replacement inference.
Saved history and earlier effects remain intact. `model_switch` steps explain
the reason; the API selection reason adds `invalid_model_decision`.

### Context and aggregate input accounting

The first regression found that `pinInputContext` restored the original model
window after a switch. Changing only the model and config would therefore use
the wrong context/evidence allocation. `InputBudget.Window` now remains the
original aggregate-budget unit, while optional `ModelWindow` identifies the
active model allocation. Each subsequent call charges its active window against
the original ceiling; measured settlement refunds only that call's unused
allocation. Model switching never resets charged tokens, calls, time or repairs.
Tokenizer-specific estimate calibration is cleared; exact historical context,
constraints, source records and compaction archives are retained.

New child assignments and explicit worker continuations after a root switch
inherit the original input-budget unit. A larger child context consumes more of
its disjoint reserved capacity. The existing parent reserve and child settlement
logic continue to measure against that original unit. This is input accounting,
not complete delegated model-policy admission or automatic child switching.

## Executed verification

The original focused log was reused during iteration. To retain a reproducible
failure, a source overlay restores only the old active-window reset in input
pinning/reservation without modifying maintained source. The unchanged regression
then fails at call 2: selected `fallback-one`, window 8192 instead of 16384,
one switch, one repair, two charged calls. This expected FAIL is preserved in
`build/model-window-reset-negative-20261122.log`, package elapsed 0.543 s;
the overlay is `build/model-window-reset-overlay-20261122.json`. It is a
negative control, not a passing production test or the original raw log.

| Check | Result | Log under `build/` |
|---|---|---|
| Focused switching, decision recovery, input and worker/child budget regressions | PASS, 11.810 s | `model-switch-focused-20261122.log` |
| Final switching tests after additional unsettled-receipt/control guards | PASS, 1.294 s | `model-switch-final-focused-20261122.log` |
| Actual child-process exit after committed switch, then reopened storage and resume | PASS, test 0.25 s, package 0.583 s | `model-switch-crash-20261122.log` |
| Production runner/service/lease with synthetic engine and isolated qualification records | Valid and changed-evidence cases PASS; package 0.773 s | `model-switch-service-20261122.log` |
| Full Windows agents/server/inference suites | All PASS, 107.704 / 170.294 / 84.294 s | `model-switch-full-windows-20261122.log` |

The Windows aggregate began before the separate process-exit test and final
additional guard cases were added; their focused results are listed separately.
Source-size and API generation consistency checks passed. No assertion, deadline,
durability setting or existing failed qualification was weakened.

Regression coverage includes two switches/three rejected decisions, persistence
before inference, resumed limit enforcement, exact pinned contexts/adapters,
lost-process recovery, cancellation and Stop, a real SQLite abort trigger,
operational errors, unknown policy versions, changed qualification/scope,
unsettled effects, immutable original input ceilings, per-call refunds and an
actual post-switch delegation using the original root budget unit.

The service test uses the production model-policy resolver, runner, context
counter, request adapter and shared lifecycle lease. It observes 8192 then 16384
context allocations. Changing fallback runtime evidence after primary rejection
prevents both its token counter and generator from running. Its model files,
runtime and qualification records are synthetic fixtures, not real capability
qualification or evidence that an installed model completes useful work.

## Final Linux race verification

All 22 packages in the CI race step passed with `-race -json -count=1` and the
same package list as `.github/workflows/ci.yml`. The normal CI ten-minute Go
timeout was unchanged. Server elapsed was 287.920 s; agents 153.381 s; inference
24.460 s. Process wall time was 313.31 s, user CPU 629.99 s, system CPU 66.39 s,
maximum RSS 884,572 KiB, exit 0. No failed test/package or race report occurred.
All new switching cases, including the actual process exit and service lease
cases, passed. The 85 skip-event identities exactly match the prior
[race inventory](server-race-recheck-2026-11-22.md); no new skip was introduced.
This remains a package-suite result, not a full hosted CI/installed/model gate.

The run restored and byte-verified 4,176 maintained files from
`build/model-switch-checkpoint-20261122.tar`, SHA-256
`512d9e03061e6c59e0791ad414e826f4e985eee278df9f76b0daed1d82d4f009`.
Environment: Ubuntu/WSL, Go 1.26.6, CGO enabled, GOPROXY off,
GOMAXPROCS=4 and isolated ext4 test state. Final production Go/test source was
unchanged after that checkpoint; later edits are documentation/evidence only.

Logs: `build/model-switch-ci-race-20261122.jsonl`,
`build/model-switch-ci-race-20261122-resources.txt` and
`build/model-switch-ci-race-20261122.log`. The exact package command is retained
in `build/test-model-switch-linux-20261122.sh` and the resource log.
API consistency, TypeScript, source-size, scoped whitespace, strict OpenSpec and
497-file maintained documentation link checks passed. Earlier timeout/capture
failures elsewhere in the program remain recorded; this does not prove their
root cause or establish performance qualification.

## Remaining acceptance

### Current primary-source checks

Checked on 2026-11-22: [Ollama's concurrency documentation](https://docs.ollama.com/faq#how-does-ollama-handle-concurrent-requests)
ties simultaneous models/requests to available memory and queues requests when
another model cannot fit. OffGrid therefore keeps switching inside its existing
admission owner; this increment does not introduce parallel fallback generation.
The service regression observes both context loads through that shared lease.

The [llama.cpp server documentation](https://raw.githubusercontent.com/ggml-org/llama.cpp/master/tools/server/README.md)
distinguishes configured context, processing slots and unavailable-slot responses;
it also describes prompt-cache reuse as a runtime optimization. OffGrid retains
exact task history independently, revalidates model/context qualification and
does not classify capacity errors as invalid model decisions. The operational
error and changed-evidence regressions exercise these distinctions. These are
design comparisons with current primary documentation, not proof that upstream
master is the bundled runtime or that either product meets OffGrid's no-replay
requirements.

Tasks 4.2–4.4 remain unchecked. Required remaining work includes delegated policy
pinning and graph-wide automatic switching/correction coordination; other safe
repair triggers; perception/provider integration; compact localized profile and
switch controls; independent review; installed/runtime/hardware qualification
and repeated frozen real-model workflows. The existing full program and earlier
failures remain authoritative. No commit, push, installation or release is part
of this increment.
