# Delegated inference admission — 2026-11-22

This increment advances M3.06 and Adaptive 4.2–4.3. Delegated switching,
graph-wide corrective coordination and real-model qualification remain open.
All fixtures use isolated state and synthetic qualification records; they
establish harness behavior, not model or hardware quality.

## Implementation

Child readiness now receives the requested response-context planner. Previously
recursion restored the default agent context, rejecting qualified interactive
children or accepting evidence for a different window. The service now persists
each child's context, adapter and optional model-pool selection in the admitted
config (`agent_profiles.go`, `profile_model_policy.go`). Admission loads no runtime.

`internal/agents/child_model_policy.go` applies these pins during existing
delegation. Conversation workers retain the parent's current qualified model
unless a selected immutable child profile explicitly enables a local pool.
Legacy direct profile delegation retains its selected child model. Native pins
also require the native response instruction and task-first path. Missing or
mismatched policy evidence rejects selection. Tool, computer and delegation
permissions remain bounded by existing parent/profile checks. Inherited
selection records `inherited_qualified_model`; API types expose that reason.

`input_budget.go` and `worker_continue.go` retain the original root token unit
when a child has a different context, including before any root switch. Larger
contexts consume more of the child's disjoint allocation. Explicit follow-ups
retain inference pins while accounting for earlier work. The existing service
lease revalidates evidence before token counting and generation.

## Verification

- `profile_model_policy_delegate_test.go`: real service config resolution for
  interactive/extended contexts, wrong-window rejection, native child without
  a pool, exact candidate validation and detached durable configuration.
- `child_model_policy_test.go`: real persisted delegation and worker execution
  for inherited conversation models, explicit child pools and legacy profiles;
  native protocol, permission ceiling, original token units, restart loading,
  explicit follow-up execution, missing and mismatched policy rejection.
- `delegate_inference_lease_test.go`: production admission, runner, counter,
  generation path and lease with synthetic runtime fixtures. Changing the child
  fingerprint after delegation blocks its count and generation.

| Check | Result / raw evidence |
| --- | --- |
| Initial targeted agent/service regression | PASS, 0.706 / 0.925 s; `build/delegate-policy-focused-1-20261122.log` |
| Worker execution and follow-up regression | PASS, 0.792 s; `build/delegate-policy-execution-1-20261122.log` |
| Production delegated lease regression | PASS, 0.875 s; `build/delegate-policy-lease-1-20261122.log` |
| Prior server source negative control | Expected FAIL, 1.220 s; `build/delegate-policy-context-negative-2-20261122.log` |
| Prior delegation source negative control | Expected FAIL, 0.824 s; `build/delegate-policy-worker-negative-2-20261122.log` |
| API generate/check, TypeScript, source-size ratchet | PASS |
| Full Windows agents/server | PASS, 127.523 / 199.437 s; `build/delegate-policy-full-windows-20261122.log` |
| Initial full Windows inference | FAIL, 89.008 s, blocked-write descendant assertion; same full-suite log, retained below |
| All 22 Linux CI race packages | PASS; `build/delegate-policy-ci-race-20261122.jsonl`; server 298.791 s, agents 166.839 s, inference 24.618 s; no new skips |
| Final full Windows inference | PASS, 84.224 s; `build/delegate-policy-inference-recheck-20261122.log` |
| Documentation and strict OpenSpec | PASS, 498 maintained Markdown files, no link problems |

Negative controls use the previous source checkpoint through Go overlays without
altering maintained source. The old server path fails all four admission cases,
including accepting the wrong context. Old delegation fails all three worker
cases. Its overlay only renames the retained input-budget helper to build against
the current helper name. The first negative-control invocation failed PowerShell
argument parsing, not application behavior; its logs remain without the `-2`
suffix. Corrected runs quote the complete `-overlay=...json` argument.

The frozen code checkpoint contains 4,181 files:
`build/delegate-policy-checkpoint-20261122.tar`, SHA-256
`d03f974017f0739b5ddf5ed9b8aabc835cd900ffca5c69e6d79a2f649650f308`.
Its manifest verifies every restored file before Linux execution. Later edits
to this evidence page, documentation links and the Windows-only cancellation
test described below do not change production code in that checkpoint. Linux
Go 1.26.6, CGO enabled, `GOMAXPROCS=4`, isolated WSL ext4 restore, offline module
resolution. Wall time 359.71 s, user/system CPU 811.86 / 96.61 s, maximum RSS
897,328 KiB, exit 0. The 85 skip identities match the preceding run; no new skips
or race failures. Resource log: `build/delegate-policy-ci-race-20261122-resources.txt`.

### Windows cancellation-test boundary investigation

The initial aggregate Windows run failed
`TestProcessProviderCancellationRetiresLiveDescendantBeforeReturning/blocked-write`:
the descendant handle was still nonsignaled when the call returned. Fifteen
isolated repetitions of the unchanged test passed (40.875 s,
`build/provider-descendant-repro-20261122.log`); that did not establish a fix.

Inspection found the 100 ms deadline started before construction/validation of a
4 MiB prompt and before stream admission. A cancelled request can return from
queued admission without owning or retiring the provider. The old test did not
observe its claimed blocked-write phase. An overlay probe makes that deadline
already expired and records admission count: five of eight repetitions reproduce
the same live-descendant assertion with **zero admitted requests** (expected FAIL,
1.570 s, `build/provider-cancellation-preexpired-20261122.log`). This proves the
test's timing ambiguity; the original failure's exact branch was not captured.

The maintained blocked-write test now constructs the prompt first, witnesses one
byte successfully written into the real owned pipe, and then cancels. Its 5 s
watchdog is not the cancellation trigger. The immediate process-handle exit
assertion and maximum-return-time assertion remain unchanged. Other cases retain
their 100 ms deadline. Production cancellation/retirement code is unchanged.
The queued-stream cancellation regression is run alongside the descendant test
so cancelling unadmitted work remains covered separately.

During witness development, the first test attempt exposed a repeat-close panic
in the test witness; `sync.Once` fixed it. Applying first-byte cancellation to the
ordinary generate case then exposed a partial-protocol fixture shutdown race.
The witness is now scoped to the blocked writer, whose fixture never reads stdin.
Both failed logs remain (`build/provider-descendant-witness-20261122.log` and
`build/provider-descendant-witness-2-20261122.log`). Final combined regressions
pass 15 repetitions in 40.679 s (`build/provider-descendant-witness-3-20261122.log`),
followed by the full Windows inference pass above.

The process-exit assertion follows Microsoft's documented distinction between
requesting termination and observing a signaled process handle. Sources reviewed
2026-11-22: [TerminateProcess](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-terminateprocess)
and [WaitForSingleObject](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-waitforsingleobject).
This test repair is not evidence of a production process-retirement fix or
complete Windows provider qualification.

## Remaining requirements

Children carry admitted inference pins but still cannot automatically switch.
The next dependency is atomic root/child corrective accounting, including
concurrent siblings, Stop, restart, failed writes and explicit continuations.
Additional repair paths, perception dispatch, generalized provider evidence,
localized controls, independent review and real model/hardware/workflow
qualification remain incomplete. No task checkbox or release gate is closed.
