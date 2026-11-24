# Isolated container activation cancellation — 2026-11-22

Scope: a correction to the existing `native-agent-workspace-execution` process
lifecycle and changed-grant requirements. This is not filesystem-containment or
real-Docker qualification, and does not complete the remaining change tasks.

## Reproduced defect

`LinuxIsolation.start` accepted the session revocation signal but did not use it
until the post-start supervisor. The attach handshake used a background context.
Revoking while that handshake was held could therefore still dispatch a fresh
container start after revocation. Caller cancellation also failed to interrupt
the pending handshake promptly.

The new test uses the existing isolated worker, SQLite journal and fake Docker
transport in disposable state. It holds attach before any start, cancels or
revokes, then checks handshake cancellation, start count, exact owned-container
cleanup, retained uncertainty and duplicate operation behavior. No Docker daemon,
container, native process, installed-user state or model is accessed.

Before the runtime fix, this command actually failed:

```powershell
go test ./internal/execution -run '^TestIsolationActivationCancellation$' -count=1 -timeout=45s -v
```

Result: **FAIL, 4.537 s**. All four original subcases reported that held attach
did not observe cancellation. Both revocation cases dispatched one container
start after revocation. The retained log is
`build/isolation-activation-red-20261122.log`.

## Implementation

- Activation now derives a cancellable context from the caller and observes the
  session revocation signal. The successful supervisor owns its remaining
  lifetime; failure cancels it immediately.
- Attach uses that context. Explicit checks before attach and before start also
  cover an acknowledgment already received when revocation occurs, without
  depending solely on the watcher goroutine's scheduling.
- Before command dispatch, failed activation reconciles only the durably recorded
  container through existing daemon and ownership checks, using a bounded
  cancellation-independent cleanup context.
- Cleanup inspection failure, foreign ownership or failed journal removal keeps
  the outcome uncertain and its recovery record retained. A proven pre-start
  cleanup may reject the operation, but its original journal claim still prevents
  replay. An already-issued start with lost acknowledgment retains the existing
  uncertain-effect semantics even when cleanup succeeds.

Owned implementation files: `internal/execution/isolation_runtime.go` and the new
`internal/execution/isolation_activation_test.go`. The existing unrelated
`cleanupVerified` change in the runtime file was preserved.

## Focused verification

The original four cancellation cases passed after the fix: **PASS, 0.559 s**,
recorded in `build/isolation-activation-green-20261122.log`.

Final focused command:

```powershell
go test ./internal/execution -run '^TestIsolation(Activation|DurableLifecycleAndUnknownStartNeverReplay)' -count=1 -timeout=45s -v
```

Result: **PASS, 0.793 s**, recorded in
`build/isolation-activation-focused-20261122.log`. All six held-attach subcases
passed: revoke, caller cancellation, each with cleanup inspection failure,
foreign owner, and cleanup journal failure. The separate attach-acknowledgment
boundary test passed. Existing successful lifecycle, lost-start-acknowledgment,
identify-persistence failure and foreign-owner regressions also passed.

`gofmt` was applied to both owned Go files. Full affected package/race checks and
the next frozen-source verification belong to the parent integration pass; they
were not run by this increment while the frozen server suite was active.

Maintenance checks: `git diff --check` passed for the owned files;
`node scripts/maintenance/check-docs.mjs` passed with 505 maintained documents and
zero problems. `node scripts/maintenance/check-source-size.mjs` reported the
concurrent, separately owned `cmd/offgrid-executor/main.go` at 155 lines against
its 150-line entrypoint limit. That failure was reported to the parent rather
than altering another agent's file or changing the ratchet.

## Remaining qualification

The corrected frozen [host integration snapshot](host-protection-integration-2026-11-22.md)
also passed the full execution, storage, native computer, executor, computer and
server packages on Windows and under Linux race instrumentation. These results
include this cancellation implementation; they do not replace the failed real
daemon attempt recorded below.

These fixtures establish cancellation ordering, durable ownership and no replay
at the simulated daemon transport boundary. They do not establish real Docker
mount/network isolation, real container termination, browser snapshot integrity
against unrestricted same-account processes, Windows/macOS containment, native
hardware performance or real-model task success. Those requirements remain open.

## Real runtime qualification attempt

The subsequent attempt ran the existing real tests from the live working tree
at `D:\offgrid-llm` (`/mnt/d/offgrid-llm` in the default WSL distribution), not
from an immutable archive or recorded source manifest. The selected source was
`internal/execution/isolation_policy_test.go` and
`internal/execution/isolation_runtime_test.go`, with the current execution package.
This result must not be attributed to an earlier frozen source snapshot.
The environment used `CGO_ENABLED=1`, `GOPROXY=off`, `GOMAXPROCS=2`, `TMPDIR=/tmp`,
and `go test -p 1 -v ./internal/execution -count=1` with each exact test selected
separately. Disposable mounts/journals were on Linux-native temporary storage.

The already installed image was `alpine:3.24`, pinned for the tests to
`sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b`.
The local socket was `/var/run/docker.sock`; Docker Engine was 29.1.3, daemon ID
`83a7d501-9575-41f8-b520-e7cd11dedabe`, Linux amd64 on Ubuntu 26.04 LTS.
The daemon reported memory, CPU quota and PID limits; the image had no declared
volumes. Host UID/GID were 1000. Details are retained in
`build/real-isolation-environment-20261122.json`. No image was pulled or changed.

| Selection or diagnostic | Actual result | Log under `build/` |
| --- | --- | --- |
| `TestRealLocalIsolationPreflightWithoutRunningContainer`, 45 s test deadline | PASS, 0.055 s | `real-isolation-preflight-20261122.log` |
| `TestRealIsolationWorkerCrashCleanup`, 60 s test deadline | FAIL, 5.008 s, `execution_host_unavailable` at preflight (`isolation_runtime_test.go:204`) | `real-isolation-crash-cleanup-20261122.log` |
| Read-only `/v1.47/info` request | Zero bytes; timeout after 10.002944 s | `real-isolation-readonly-diagnostic-20261122.log` |
| Read-only exact-image inspection | HTTP 200 after 3.397159 s | `real-isolation-readonly-diagnostic-20261122.log` |

The failed test returned before container creation. OffGrid ownership-label
inventory was empty before and after the attempt: **zero containers were created**.
The daemon identity remained unchanged during diagnosis. The cause of its
inconsistent responsiveness is unproven; this evidence does not attribute it to
load or Docker Desktop path visibility. Production preflight timeouts and
assertions were not weakened. The diagnostic log's final shell status follows
the final identity query; the recorded curl timeout remains a failure.

After that preflight failure, container progression stopped. Real crash cleanup,
the real command lifecycle/Stop/timeout cases, activation cancellation against a
live daemon, mount/network boundaries and browser/coding coexistence remain
unqualified. No existing containers, volumes, images or installations were
modified, and no host-root mount workaround was attempted.
