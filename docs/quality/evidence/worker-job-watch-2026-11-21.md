# Windows worker member discovery — 2026-11-21

Continues [Conversation task 13.6](../../../openspec/changes/agent-conversation-workflows/tasks.md)
and M1/M3 reliability. The full task remains unchecked: Linux escaped-descendant
ownership, other platform/persistence qualification, environment templates and
background lifetime still require work.

## Failure and implementation

The previous owner discovered descendants only while stopping. A short-lived
child could exit before discovery, so retained handles could not account for the
Job's lifetime membership. The actual native compiler journey could observe a
successful leader but could not verify whole-worker shutdown. Dropping that
count check would erase uncertainty.

The added actual-child regression reproduced the missing handle before the
change: FAIL 0.665 s. Production now creates a private completion port and
associates it with the existing Job before assigning the suspended root. A single
owned monitor captures new-process notifications, opens a synchronization/query
handle and verifies membership in that exact Job. Retained handles prevent PID
reuse from becoming cleanup authority. The maximum retained member count remains
4096. Invalid/missing/late evidence retains uncertainty.

Release terminates owned members, drains the monitor through a sequenced bounded
barrier, waits for exact handles and compares final lifetime accounting. Only
then may the Job/monitor/handles close. The monitor joins before its completion
port closes. The change does not add another executor, authorize process replay,
clear copy leases on restart or use notifications as proof of exit.

Microsoft documents that [Job completion notifications](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_associate_completion_port)
are not guaranteed and that a reported PID can be recycled unless a handle is
retained. [Job association](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-setinformationjobobject)
supports attaching the port before launch. Those primary contracts were read on
2026-11-21 and inform both discovery and the retained no-proof behavior.

## Executed evidence

All processes are fixture-owned Windows children. The native journey creates a
disposable authenticated service, builds the existing host pack, runs a stdlib-only
Go test in its approved temporary folder and checks host shutdown. It does not
touch an installed service, user project, personal browser or downloaded model.

| Check | Result | Evidence |
| --- | --- | --- |
| New short-lived child before wiring watcher | FAIL 0.665 s | `build/worker-job-watch-baseline-20261121.log` |
| First focused real Windows cleanup cases | PASS 0.948 s | `build/worker-job-watch-first-20261121.log` |
| Previously failing authenticated native compiler journey | PASS 4.859 s | `build/worker-job-watch-native-consumer-20261121.log` |
| First complete execution package | PASS 20.533 s | `build/worker-job-watch-execution-full-20261121.log` |
| Final complete execution package | PASS 21.967 s | `build/worker-job-watch-execution-final-20261121.log` |
| Actual native compiler journey, three repetitions | PASS 12.300 s | `build/worker-job-watch-native-repeat-20261121.log` |
| Complete Linux execution race suite, following Windows change | **FAIL 68.866 s** | `build/worker-job-watch-linux-race-20261121.log` |

The Linux failure is not covered by the Windows passes. Validation/integration
fixtures cannot seal positive results because the Linux process-group owner does
not establish descendant settlement. The process-receipt fixture's repeat read
is rejected after that uncertainty revokes its session. A separate real escaped
child exposed a bookkeeping bug: close returned before recording uncertainty,
leaving the copy state `ready` even though its lease correctly prevented removal.

Close now records `uncertain` on endpoint/descendant or settlement-write failure,
preserving the ownership lease and byte reservation. A failed uncertainty write
is returned and retried by the retained closed owner; it cannot authorize cleanup.
The real Linux escaped-child race test passed 2.133 s after this fix, with the
child still alive and data retained (`build/worker-linux-close-20261121.log`).
Windows working-copy tests passed 5.255 s, including an ignored SQLite uncertainty
write and retry (`build/worker-close-windows-20261121.log`). This fixes the status
defect; it does not resolve the broader Linux ownership/validation failures.

The negative case deliberately suppresses new-process discovery while allowing
the real short-lived child to run and exit. Both first and subsequent release
attempts must remain uncertain. Its earlier assumption that all production
short-lived children remain undiscovered was replaced with this deterministic
missing-notification case; the assertion denying cleanup is unchanged. Separate
positive cases require actual captured-child settlement. Additional tests reject
the current unrelated process as a member and verify monitor draining and exit.

Commands:

```text
go test ./internal/execution -run '^TestProcessCleanupWindows' -count=1 -timeout 1m
go test -p 1 ./internal/execution -count=1 -timeout 3m
go test -p 1 ./internal/server -run '^TestNativeApprovalStartsPinnedWorkerOverAuthenticatedBridge$' -count=3 -timeout 3m
```

Windows Go is 1.26.6 with CGO disabled; no gcc/clang was found. Windows race
instrumentation has therefore not run. The separate Linux race suite cannot
qualify this Windows-only watcher. Full affected CI, independent review, extended
member-churn/limit/resource measurements, other platforms and physical power-loss
qualification remain open. Three actual compiler passes establish this regression
checkpoint, not universal subprocess or production reliability.
