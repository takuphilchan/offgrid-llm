# Shared model correction accounting — in progress, 2026-11-22

This increment continues M3.06 and Adaptive 4.2–4.3. It is not yet qualified or
complete. The full program and these OpenSpec tasks remain open.

The existing task transaction now owns an optional versioned root model budget.
Runner checkpoint publication updates the child's selection and the root's spent
switch count together. Concurrent sibling rejections cannot each spend the last
switch. Unresolved correction entries are per run; success in another worker
cannot erase them. Model decision and proposal correction counts participate.
Follow-up root attempts retain the budget and root switch history. Legacy tasks
seed the optional budget from their existing durable graph instead of receiving
a fresh allowance. Invalid budget shape/ownership fails stored-task validation.

The unchanged safe-boundary and qualification-pin checks still apply. A verified
direct child can now switch within its admitted policy through the transaction.
Inference reservation independently checks exhaustion to cover a sibling's
intervening rejection. Runtime selection, input charges, tool authority and
uncertain-effect handling remain in the existing runner/service paths.

`run_persistence.go` contains the existing persistence method extracted from
`durable.go` and its shared-budget publication integration. There is no new state
database or executor. `model_graph_budget.go` owns aggregate accounting.

## Actual checks so far

- Prior model-switch/delegation/worker regressions: PASS 4.652 s,
  `build/model-graph-existing-1-20261122.log`. The tool observation timed out,
  but the completed log and absence of its process subsequently established exit.
- New tests initially failed to compile because a fixture returns `error` and
  the rejection method requires the typed validation error. Corrected the test
  calls; failed log retained as `build/model-graph-focused-1-20261122.log`.
- Initial three graph tests: PASS 1.865 s,
  `build/model-graph-focused-2-20261122.log`.
- Expanded graph/switch/delegation/worker focused suite: PASS 7.557 s,
  `build/model-graph-focused-3-20261122.log`.
- Full corrected agents suite: PASS 110.742 s,
  `build/model-graph-agents-full-2-20261122.log`.
- Intermediate server suite: PASS 245.853 s in the initial aggregate log;
  it predates final reservation/validation changes and is not final-source evidence.
- Final source-size, 499-document link check, strict OpenSpec and affected-file
  whitespace checks: PASS.
- Added canonical receipt/steering/root-Stop and reservation-race checks:
  PASS with model-switch tests, 4.589 s,
  `build/model-graph-boundaries-2-20261122.log`. The first invocation failed in
  test setup by assigning to an omitted/empty map; corrected the fixture's map
  initialization, preserving `build/model-graph-boundaries-1-20261122.log`.
- Source-size check: PASS before final test additions; no baseline increased.

New tests exercise concurrent sibling rejection, canonical budget lookup from a
stale root, restart, real runner child fallback, a SQLite root-write rejection
rolling back child selection, unrelated sibling success and retained work
attempts. All use isolated state and synthetic model responses.

## Outstanding verification and work

Full agent/server and Linux race verification, more Stop/steering/crash-boundary
coverage, legacy graph and invalid-budget cases, independent review, API/activity
presentation and real model/hardware qualification remain open. The aggregate
Windows invocation started before the test compile correction and retains that
failure in `build/model-graph-full-1-20261122.log`; it cannot count as a full pass.
A corrected full agent run writes `build/model-graph-agents-full-2-20261122.log`.
Final-source Windows server verification is running under
`build/model-graph-server-final-20261122.log`; Linux agents/server race verification
uses the frozen `build/model-graph-checkpoint-20261122.tar` and writes
`build/model-graph-linux-race-20261122.jsonl`.
Inspect live handles and terminal results before rerunning any command or
updating this evidence. No completion is inferred from a launched test.

## Verification follow-up

Code review found that checkpoint publication clears settled receipts before
applying the proposed switch. Shared switch admission now checks the canonical
pre-update checkpoint as well as the proposed one, preserving the safety gate
across that cleanup. New receipt and steering tests cover this path. An earlier
Windows server run passed in 222.472 s but predates this guard; it is not final
source qualification.

The first Linux attempt started reading an unfinished archive and failed with
unexpected EOF (`build/model-graph-linux-race-20261122.log`). The archive creator
then detected this evidence page had changed during capture and rejected that
archive without a manifest. Neither attempt supplies test evidence. The existing
Windows command continued independently and produced the 222.472 s result.

A new archive completed before any reader was launched:
`build/model-graph-final-checkpoint-20261122.tar`, 4,186 files, SHA-256
`c0066f18a1e9fe95fa784728a717004126423e05f4b760d3773acd24bec05975`.
Final Windows agents/server output is
`build/model-graph-final-windows-20261122.log`. Linux agents/server race output
is `build/model-graph-linux-race-20261122.jsonl`, with orchestration output in
`build/model-graph-linux-race-2-20261122.log`. These are pending until their
terminal results are inspected. Subsequent evidence-only edits do not alter
the archived code or tests.

Review also identified a remaining continuation case: unresolved correction
entries currently use run IDs. Verify and implement how a successful explicit
follow-up resolves the prior assignment's sequence without refunding spent
switches or resetting an exhausted sequence before inference. Current tests
establish retention across attempts, not resolution across that lineage. This
must be addressed before Adaptive 4.2 is complete.

### Continuation implementation follow-up

Windows verification of the prior shared-budget snapshot finished: agents PASS
129.232 s, server PASS 350.257 s in `build/model-graph-final-windows-20261122.log`.
The corresponding Linux race process finished: agents PASS 296.623 s, server
PASS 546.716 s, no failure events in its JSONL. This is the shared-budget snapshot
before continuation changes; the continuation source has its own checks below.

`model_budget_continuation.go` now transfers unresolved correction debt from the
old run ID to the new run ID in the existing command transaction. It carries the
charge into the new checkpoint before inference, preserving the shared switch
count. A valid decision clears only that run's correction entry. Root work-attempt
continuation performs the same transfer and seeds legacy graph accounting before
discarding the old attempt's child list. No executable proposal or effect receipt
is copied into a new attempt by this helper.

Focused tests cover a typed rejection and actual model switch, runtime interruption,
explicit worker Stop, user-requested continuation, retained debt before generation,
restart, successful correction without erasing sibling debt, exhaustion preventing
any generation, and a receipt-write failure rolling back the transfer.
PASS 1.729 s: `build/model-continuation-focused-3-20261122.log`.
Existing graph/follow-up regressions passed 13.610 s in
`build/model-continuation-initial-20261122.log`.
The first two new-test runs assumed runtime failure was terminal; the runner
correctly returned interrupted. Tests now use the real explicit Stop/continue
path. Those failed logs remain as `build/model-continuation-focused-20261122.log`
and `build/model-continuation-focused-2-20261122.log`.
This addresses the identified lineage implementation gap, but broad final-source
verification, crash qualification and the remaining Adaptive requirements stay open.

The continuation source checkpoint completed before execution:
`build/model-continuation-checkpoint-20261122.tar`, 4,188 files, SHA-256
`4c96cf961c2b6c7ddec5cfaa6234e86eb7272648c0a746f79fb9a21d8bc6ded0`.
Full Windows agents/server output is
`build/model-continuation-full-windows-20261122.log`; full Linux agents race output
is `build/model-continuation-linux-race-20261122.jsonl`. Both require terminal
result review. The earlier Linux agents/server race run uses the previous
shared-budget snapshot; it does not include these continuation changes.

Continuation snapshot verification finished: Windows agents PASS 275.804 s and
server PASS 339.450 s; Linux agents race PASS 315.213 s. These results precede
the recovery guard below and do not establish final-source verification for it.

### Crash recovery and Resume accounting

`model_graph_crash_test.go` exits a child process with code 74 immediately after
the switched child's transaction commits. Reopening isolated storage finds both
the selected fallback and the root switch/correction charge. Explicit parent
resume restores the delegation without generating; starting the selected child
uses only the fallback, retains prior generation charges, resolves its correction
and leaves the sibling unexecuted. Invalid budget versions/counts/ownership are
also checked. PASS 1.536 s in `build/model-graph-crash-2-20261122.log`. The first
test tried resuming a child before its interrupted parent and was correctly
denied; that failure remains in `build/model-graph-crash-20261122.log`.

Review found a real accounting bug: Resume's existing local proposal cleanup
could clear the aggregate correction entry on the next checkpoint write, allowing
another Resume to regain inference. Aggregate entries now decrease only with
recorded decision progress or completion of an invocation, never typed rejection
or control cleanup alone. The prior source overlay fails the new regression:
`build/model-resume-budget-negative-20261122.log`, expected FAIL 0.680 s,
"local proposal cleanup refunded shared debt". The maintained source is not
modified by the overlay. Existing root/worker/continuation/switch tests passed
5.621 s before the final explicit rejection exclusion; the final focused result
is in `build/model-graph-resume-final-20261122.log` and must be inspected.

Final focused suite: PASS 5.899 s. Frozen recovery source: 4,189 files,
`build/model-graph-recovery-checkpoint-20261122.tar`, SHA-256
`9f29c52ffde5affe417c1da8c4f0a03735efbb7b4a9e9fe955a697c3137af526`.
Windows agents/server final-source verification is running in
`build/model-graph-recovery-full-windows-20261122.log`; the full 22-package CI race
set is running in `build/model-graph-recovery-ci-race-20261122.jsonl`. Neither
launched command counts as a pass until its terminal results are inspected.

### Accepted conversation model integration

A regression using canonical conversation admission reproduced a storage error
when a configured fallback was selected: `validateStoredTask` required the
effective model to equal the accepted conversation model. Both primary and
explicit-fallback starting choices failed after one generation (3.557 s,
`build/conversation-model-switch-before-20261122.log`). This blocked ordinary
Chat integration despite the lower-level switching checks passing.

`conversation_configuration.go` now validates the immutable accepted choice
against the first source model in the persisted adaptive switch chain. A mismatch
without a chain, a different initial choice, a guided policy, or an invalid
selection/pin remains rejected. The accepted configuration is never rewritten
to pretend the fallback was originally selected. Regression coverage includes
durable canonical execution and reload for both starting choices and negative
stored-state cases. `agent_model_switch_test.go` also covers canonical conversation
execution through the production service lease, both valid and revoked evidence.

Focused agents PASS 148.736 s in
`build/conversation-model-switch-after-20261122.log`; service PASS 1.952 s in
`build/conversation-model-switch-service-20261122.log`. The recovery Windows/full
CI race suites use the earlier frozen source and do not verify this subsequent
change. Do not relabel those runs as conversation-integration evidence.

### Recovery snapshot broad-run failures retained

The Windows recovery run failed: agents 511.648 s, with Stop-controller readiness
and five delegated completion waits exceeding their existing bounds; server
600.647 s, with native executable cleanup denied, an unfinished workspace-access
continuation, and the ten-minute suite timeout. The original output remains in
`build/model-graph-recovery-full-windows-20261122.log`.
Without changing assertions or deadlines, all six agent cases passed three
isolated repetitions (18.157 s, `build/model-recovery-failure-reprobe-20261122.log`)
and both service cases passed two repetitions (80.487 s,
`build/model-recovery-service-reprobe-20261122.log`). These reruns do not establish
the cause of the original failures or replace complete-suite evidence.

The frozen Linux recovery race run passed 21 of 22 packages, including agents
324.511 s and server 563.147 s. Inference reported PASS internally, then failed
because test output remained open after process exit (`WaitDelay expired before
I/O complete`, 107.259 s). It counts as a failed package. Its cause remains under
investigation; there were no individual test failure events. Full output is in
`build/model-graph-recovery-ci-race-20261122.jsonl`. No qualification gate is closed
by these partial results. Documentation checks passed for 499 maintained files,
as did the source-size ratchet and focused whitespace check.

The conversation snapshot is frozen separately: 4,190 files, SHA-256
`5cfcb28a6c382919af4b762aa0b1ac0bd1aefca04850f210bad944cf33b81014`,
`build/conversation-switch-checkpoint-20261122.tar`. Running the full Windows
packages sequentially with unchanged assertions/timeouts passed: agents
120.360 s, server 220.260 s (`build/conversation-switch-full-windows-20261122.log`).
The same frozen snapshot's sequential Linux race verification is in
`build/conversation-switch-linux-race-20261122.jsonl`: inference PASS 31.781 s,
agents PASS 280.851 s and server PASS 519.358 s. The prior inference output-pipe
failure did not recur in this run; its original cause is not established.
[Go's test runner](https://go.dev/src/cmd/go/internal/test/test.go) treats expired
output draining as package failure even if the test binary prints PASS, so the
original failed package remains a failure.

### Independent review: mixed rejection accounting

A read-only independent agent review found that a syntactically valid tool
envelope cleared `DecisionRepairs` before authorization. The subsequent queued
iteration was incorrectly accepted as progress by aggregate accounting. Two
typed response rejections followed by one owned authorization rejection could
therefore admit a fourth inference, with only one correction retained.

The concrete regression reproduced this in
`build/model-mixed-repair-before-2-20261122.log`: expected FAIL 0.814 s, four model
calls, zero tool effects, and only one retained correction. The initial fixture
had insufficient input budget for the fourth call but still exposed the refund
(`build/model-mixed-repair-before-20261122.log`, FAIL 0.783 s); its ordinary
iteration allowance was raised to isolate correction accounting.

Persistence now receives an explicit accepted-progress signal from successful
operations, never inferring it from iteration advancement or a cleared execution
field. Conversation hold/steering preemption returns no accepted progress.
Ordinary successful tools, successful runtime metadata, verified final answers,
accepted clarification and delegation/worker-continuation transactions resolve
the decision portion while retaining outstanding proposal debt and spent switches.
Root-only runtime transactions clone the canonical budget under the manager lock;
explicit user continuation still transfers unresolved debt.

The focused mixed/graph/conversation/proposal/decision suite passed 10.256 s in
`build/model-mixed-repair-after-3-20261122.log`. It covers mixed rejection limits,
accepted tool/metadata/delegation progress, pending approval/input, failed tools
and invalid metadata. A previous graph unit fixture treated a queued iteration
as success; it now first asserts no refund from ordinary persistence, then tests
explicit accepted progress preserving the sibling charge. The original failed
run remains in `build/model-mixed-repair-after-20261122.log`; a test status-name
compile typo remains in `build/model-mixed-repair-after-2-20261122.log`.
Additional transaction/fence review and complete final-source checks remain open.
None of the earlier frozen full-suite runs includes this subsequent correction.

Independent review added `model_progress_boundaries_test.go`: four regressions
passed 1.716 s (`build/model-progress-boundaries-review-2-20261122.log`). They
cover hold/steering preemption, child/root transaction rollback and restart,
failed delegation insertion, and model-triggered continuation of a historical
worker under a newer active root. That last path revealed a second integration
issue: the acceptance helper initially used the worker's historical root instead
of the canonical current work attempt. `worker_continue.go` now resolves
`manager.tasks[root.ID]` under its existing lock. The first review-test log retains
test-setup failures (nil repair map and retrying a deliberately latched storage
manager); the corrected test reopens isolated state without clearing the fence.
Existing worker-flow checks also passed 7.311 s in
`build/model-mixed-workflow-20261122.log`; combined progress/continuation checks
passed 3.761 s in `build/model-progress-combined-20261122.log`.

Matching computer/workspace access resolution now retires decision debt in the
same validated binding/task transaction, while preserving unrelated proposal,
read, workflow and pre-effect obligations. Initial attachment, duplicate
acknowledgment and non-access calls do not refund debt. Before-fix regressions
failed 1.685 s in `build/model-input-progress-before-20261122.log`; final focused
input/execution-input/progress checks passed 6.396 s in
`build/model-input-progress-final-20261122.log`, including project/task rollback,
restart, idempotence and coexisting stronger repairs. No authorization rule was
broadened. The existing computer resolver's attachment behavior during an inbox
hold remains distinct from the workspace resolver; this increment does not
certify that broader control contract. Final combined verification remains open.

Combined progress/access source is frozen as 4,194 files in
`build/model-progress-checkpoint-20261122.tar`, SHA-256
`2c45bf6554f690fa45975a3287f10e5cfb02424c524c571dbf4fc645d97da972`.
The full Windows agents/server run executes packages sequentially with unchanged
ten-minute package timeouts: `build/model-progress-full-windows-20261122.log`.
Agents PASS 115.175 s; server PASS 216.246 s. The matching Linux race
procedure `build/test-model-progress-linux-20261122.sh` completed with exit 0:
agents PASS 283.585 s; server PASS 506.709 s, zero failed test events in
`build/model-progress-linux-race-20261122.jsonl`. Output was spooled in the
isolated WSL stage and copied after test completion.
Browser integration work after this freeze is a separate increment; do not infer
its evidence from this backend snapshot.

Independent failure analysis found measurable output-conversion delay in the
earlier recovery race log: a native cache unload logged at 15:05:26 appeared as
JSON at 15:06:01.523, about 35.5 seconds later. The passing inference rerun's
corresponding log and JSON event both occurred at 15:25:08. The earlier final
PASS event preceded the incomplete-I/O diagnostic by only 9 ms; those JSON
timestamps cannot establish actual test-process exit time.
[Go 1.26.6's output-copy and wait implementation](https://github.com/golang/go/blob/go1.26.6/src/os/exec/exec.go)
also permits unfinished output copying/conversion to trigger this failure; it
does not by itself prove an orphaned process. No owned-child leak was established.
The exact source of the observed output backlog remains unknown. The next race
script spools output to WSL-local storage and copies the exact result afterward,
preserving the test exit status and unchanged assertions/timeouts. If the issue
recurs, capture actual child exit, pipe-inode holders and output-write latency
before changing runtime cleanup or attributing the backlog to mounted storage.
Manifest comparison found identical paths and SHA-256 values for all 186
`internal/inference/` files in the failed recovery and successful conversation
snapshots. That establishes identical recorded inference source, not identical
execution conditions or every dependency. No production cleanup change was made
to explain away the failed qualification run.
