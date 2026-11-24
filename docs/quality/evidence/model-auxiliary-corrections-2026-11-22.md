# Shared auxiliary model corrections — 2026-11-22

This is implementation evidence for the still-open `model-adaptive-execution`
task 4.2. It does not complete that task, qualify a real model, or establish the
entire transformation program as complete. Tests use synthetic model responses,
disposable task databases, and no native effects, installed-user state or model
weights.

## Reproduced failures

The root correction ledger previously counted only `DecisionRepairs` and
`ProposalRepairs`. Two actual runner regressions both admitted a fourth inference
and completed after three unresolved rejections:

- A typed adapter rejection followed by two reviewed skill answer rejections.
- A typed adapter rejection, an invalid empty work plan, and a typed rejection of
  an edit delegation without an authorized working-copy host.

`build/model-auxiliary-red-valid-20261122.log` retains both failures. Earlier
delegation fixtures stopped at unrelated untyped validation errors; those failed
runs are retained in `model-auxiliary-red-20261122.log` and
`model-auxiliary-red-confirmed-20261122.log`, and are not evidence of that bypass.

Parent review subsequently identified a same-task stale-snapshot defect in the
initial auxiliary implementation. An ordinary older checkpoint could overwrite a
new charge and reduce the shared count from one to zero. The actual failing
regression is retained in `build/model-auxiliary-stale-red-20261122.log`. The final
implementation adds explicit resolution witnesses and canonical transition checks.

## Implemented boundaries

`internal/agents/model_auxiliary_repairs.go` defines versioned, categorized active
debt separate from historical repair counters. Categories cover reviewed skill
answers, delegated answers, workflow completion, workspace completion, typed
workspace-read recovery, typed delegation admission, and known local runtime
argument/metadata rejection. Historical answer-only tool restrictions and the
workspace-read lifetime allowance remain enforced.

The existing root `ModelGraphBudget` retains its aggregate per-run count and adds
an auxiliary contribution projection. The projection lets a matching auxiliary
resolution reduce exactly that contribution without refunding unrelated decision
or proposal debt. New adaptive roots persist budget version 2 at initial admission,
before the first runner checkpoint. Earlier aggregate-only readers reject that
version instead of ignoring auxiliary semantics. Version 1 without auxiliary
fields remains readable and migrates through the existing initialization helper;
version 1 with auxiliary projection, or version 2 without its required auxiliary
version, is rejected. All changes use existing root/child SQLite transactions; they
do not introduce another executor or persistence store. Only the existing typed
adapter-rejection path can request a model switch. Three unresolved shared
corrections prevent another inference reservation, including a stale sibling
reservation.

Resolution is explicit:

- A validated answer-format/briefing correction resolves its corresponding
  answer category. The final accepted completion clears this run's remaining
  active categories, retaining rejection and resolution sequence metadata.
- Newly verified workflow checks resolve workflow debt. Repeating the same
  proof, ordinary metadata, or resetting a historical counter does not.
- Workspace completion requires the service's existing completion validator.
  A successful process observation that still reports a running process does
  not resolve it. Foreign capability identities cannot provide that proof.
- A successful matching trusted workspace-read capability resolves its active
  read category without refunding its historical lifetime allowance.
- Accepted metadata resolves the same runtime operation's category. Admitted or
  reused delegation resolves typed delegation-admission debt.

Matching hooks advance per-category persisted resolution witnesses. Transactions
compare rejection sequences, witnesses and active decreases against canonical
state. Stale own-task publication, an unwitnessed decrease, or a stale root
runtime transaction cannot refund debt, even through generic accepted-progress
persistence. Stop, conversation holds, pending instructions, and parent control
fences remain controlling. Resolving and rejecting the same category in one
checkpoint still counts as a new rejection at those fences.

Pure argument validation in discovery, worker-budget requests, worker commands and
conversation decisions has an explicit typed rejection. Resource lookup,
catalog changes, permission/liveness conflicts, cancellation and persistence
errors are not newly classified as model mistakes. Existing `task_expectations`
proposal handling remains a separate pre-existing path; this increment is not a
claim that every runtime failure classification has been qualified.

New root, child and follow-up constructors initialize the version marker.
Migration is conservative: a legacy incomplete checkpoint with historical
auxiliary rejection evidence has ambiguous unresolved debt and receives the
three-correction pause rather than a fresh inference allowance. A canonically
completed legacy task does not receive that ambiguity debt. Existing explicit
follow-up semantics transfer the prior aggregate once into the new run's
decision debt, removing the old auxiliary projection without copying executable
calls, receipts or grants. Unknown versions/categories, negative counts and
invalid root projections fail existing stored-task validation.

## Actual checks and retained intermediate failures

Environment: Windows x64, Go from the repository environment; all state under
Go test temporary directories. Commands run from `D:\offgrid-llm`.

| Check | Actual result | Retained log |
| --- | --- | --- |
| `go test ./internal/agents -run '^TestModelAuxiliaryMixedRejectionsStopBeforeFourthInference$' -count=1` before implementation | FAIL: both corrected fixtures made four calls and completed | `build/model-auxiliary-red-valid-20261122.log` |
| Same regression after initial implementation | PASS, 1.515 s | `build/model-auxiliary-green-initial-20261122.log` |
| Initial selected existing model/skill/workflow/read tests | FAIL, 83.022 s: old invalid-metadata assertion expected total one, while the new rejected-plan charge correctly makes total two | `build/model-auxiliary-focused-initial-20261122.log` |
| Auxiliary/storage tests plus `TestModelGraphOnlyAcceptedResultsResolveDecisionDebt` after strengthening its separate decision/total assertions | PASS, 4.945 s | `build/model-auxiliary-storage-corrected-20261122.log` |
| Same-task stale checkpoint regression before witness fix | FAIL: count one became zero | `build/model-auxiliary-stale-red-20261122.log` |
| `go test ./internal/agents -run '^TestModelAuxiliary' -count=1` after witness fix and expanded regressions | PASS, 4.629 s | `build/model-auxiliary-witness-expanded-20261122.log` |
| Direct adaptive workflow limit plus expanded malformed stored sequence/witness/projection cases | PASS, 1.826 s | `build/model-auxiliary-workflow-state-20261122.log` |
| Version admission/migration/ambiguous-combination regressions before the version fix | FAIL: admission had no ledger; migration emitted version 1; version 1 accepted auxiliary semantics | `build/model-auxiliary-version-red-20261122.log` |
| Complete auxiliary selection after version-2 emission and migration | PASS, 6.494 s | `build/model-auxiliary-version-focused-20261122.log` |
| Final bounded model/progress/continuation, answer, completion, read, worker-budget/runtime, conversation-decision and discovery selection | PASS, 20.205 s; 146 test/subtest pass events, zero failures | `build/model-auxiliary-focused-final-20261122.json` |
| `node scripts/maintenance/check-source-size.mjs` | PASS; no limit increases | Tool output |
| `git diff --check -- internal/agents` | PASS | Tool output |
| `node scripts/maintenance/check-docs.mjs` | PASS; 503 maintained Markdown files, zero problems | Tool output |

Additional retained intermediate fixture failures are not hidden: the initial
corrupt-projection test encountered the existing write-side rejection before its
intended reload check (`model-auxiliary-storage-initial-20261122.log`); the test
now independently verifies storage rejection and injects isolated database
corruption to exercise reload rejection. Expanded tests initially failed to
compile because of an unused fixture variable (`model-auxiliary-expanded-20261122.log`).
The delegated-answer fixture then needed the normal synthesis cursor closure,
avoidance of automatic background scheduling, and the existing supplied-figures
contract syntax; failed results remain in the `model-auxiliary-expanded-corrected`,
`model-auxiliary-expanded-final`, and `model-auxiliary-delegated-answer-corrected`
logs. Its corrected targeted run passed in 1.036 s
(`model-auxiliary-delegated-answer-validated-20261122.log`).

Coverage includes direct mixed-category runner execution, delegated answers,
pending workspace completion without replay, concurrent sibling charging and
stale reservation, matching versus unrelated resolution, retained still-running
process debt, foreign evidence, typed read allowance, cancellation versus pure
argument rejection, charge and resolution rollback, restart, explicit follow-up
transfer, malformed stored state, legacy ambiguity/completion, Stop/inbox/steering,
and stale own/root-runtime snapshots.

The final bounded command was:

```powershell
go test ./internal/agents -run 'TestModel(Auxiliary|Graph|Progress|Continuation|Switch)|TestSkillAnswerCorrection|TestSkillCorrectionRestart|TestPendingProcess|TestWorkflow.*Completion|TestWorkspaceRead|TestWorkerBudgetRequest|TestWorkerRuntime|TestConversationDecision|TestDiscovery' -count=1 -json
```

Maintained source was unchanged during that run. The result is a selected suite,
not the complete agents package, server package, race suite or CI program.

## Remaining qualification

Independent source review found no new concrete defect in monotonic resolution
witnesses, cross-category accounting, root/child transactions, version admission,
migration and continuation transfer. The combined 4,251-file source snapshot
`build/host-model-checkpoint-20261122.tar` has SHA-256
`b3b0cacd030c4cb150645a33d4ea97b4d0315e4639b5e17ba2ee53e3be51d8ec`.
Every extracted file was checked against its manifest. Full Windows packages
passed: agents **131.851 s**, server **175.793 s**, execution **57.080 s**,
native computer **8.377 s**, executor **7.135 s**, computer **2.084 s**.
Command: `go test -p 1 -count=1 ./internal/execution ./cmd/offgrid-computer ./cmd/offgrid-executor ./internal/computer ./internal/agents ./internal/server`.
Log: `build/host-model-full-windows-20261122.log`. Matching Linux race results:
agents **PASS 222.858 s**, execution **PASS 114.084 s**, native computer **PASS
2.160 s**, computer **PASS 4.191 s**; executor **FAIL 7.587 s** and server **FAIL
324.787 s**. `TestPinnedStdioEndpointRunsRealExecutor` and two native-host server
journeys rejected startup. The server failure path also exposed an unsynchronized
diagnostic-buffer read. Raw results are retained in
`build/host-model-linux-race-20261122.jsonl`. Startup diagnostics identified host-root
ancestors appearing as unmapped UID 65534 inside the worker namespace; repair and
full affected reruns were required. Later registry-inspection and
isolated-activation changes are not part of this frozen snapshot.

The corrected [host integration snapshot](host-protection-integration-2026-11-22.md)
subsequently passed all six affected Windows and Linux race packages, including
executor and server. The agents source was unchanged from the passing runs
above. The original failures remain retained; whole-tree CI and real-model,
hardware, browser maximum-file transport and broader task 4.2/product requirements
remain separate, incomplete gates; these synthetic tests do not substitute for
them. No OpenSpec task or roadmap requirement was marked complete by this increment.

The initial additive version-1 design was not downgrade-safe: an older binary
could ignore category fields and refund sub-threshold debt on unrelated accepted
progress. It was replaced by the tested version-2 admission and migration boundary
above. Installed-binary downgrade/restore qualification remains separate; no
installed state or binary has been changed.
