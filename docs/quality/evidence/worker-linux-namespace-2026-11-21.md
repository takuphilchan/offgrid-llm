# Linux worker namespace ownership — 2026-11-21

Continues M1.05/M3.07 and the existing Conversation execution and inference
runtime changes. Overall requirements and Conversation 13.1/13.6 remain open.
This record supersedes the process-group limitation at the checkpoints in
[Windows discovery](worker-job-watch-2026-11-21.md) and
[worker environment](worker-environment-2026-11-21.md); their failures remain
historical evidence, not erased checks.

## Implementation and acceptance

`internal/processowner` now owns the shared Linux lifetime primitive. Execution
and managed inference use this primitive; it supplies no task authorization.
Go creates an exact namespace init with `CLONE_NEWUSER|CLONE_NEWPID` and a
kernel-returned pidfd. Only the approving account's UID/GID is mapped. A pinned
util-linux launcher mounts private procfs. Live identity checks verify the PID
namespace, init role and mappings. Immediate exit is observed through the retained
pidfd, without trying to discover a process after it has gone. Stop targets that
exact init. Close requires exit/reap evidence and retains uncertainty on failure;
neither an empty process group nor a PID-number scan proves completion.

The creating OS thread stays alive through Wait. A fixed shell gate executes
literal argv only after the supervisor confirms ownership and writes an activation
token. Before activation, owner loss closes the pipe and prevents command effects,
including the interval before the parent-death signal is armed. The shell and
launcher are separately hashed and opened through verified descriptors. The
system startup path checks root ownership, ancestry and write permissions before
capturing pins. Missing kernel, launcher or identity support fails closed.

Nested workers exposed a real integration defect: their user namespace sees
host-root-owned binaries as an unmapped UID. Repeating the host ownership check
there rejected the launcher. The trusted native supervisor now transfers its
versioned launcher/shell pins through private stdio startup environment, separate
from grant JSON and HTTP/model input. The pinned executor verifies those bytes
and supplies an immutable session process starter; prepared workers inherit it.
Pins are excluded from ordinary task environments. Unknown fields, versions,
oversized input and changed hashes are rejected. This does not add permission to
select a runtime from a model message or restore a grant after restart.

The real working-copy escape fixture starts a `setsid` descendant, obtains its
host PID from kernel Unix-socket credentials, and retains a pidfd before allowing
the parent to exit. It verifies actual descendant exit, settled receipt proof and
release of the copy/reservation. Lost ownership still fails closed in separate
tests. The authenticated native bridge test builds a matching executor, starts
an approved disposable workspace, compiles/runs its Go test, observes exit, and
closes the host. These are actual native fixtures, not model or installed-product
qualification. The new package is included in CI's explicit race-test list.

## Observed failures and corrections

- The original full Linux execution race run failed with unproven descendant
  cleanup (recorded in the linked discovery evidence). A group-only design was
  insufficient for the required working-copy settlement.
- The first shared-owner compile used the wrong `MutationError` field; it was
  corrected before functional results. A later gate-edit script and nested-test
  variable caused compile failures. Their logs are retained.
- An ungated prototype passed tests but source review exposed the parent-death
  startup window. Those passes do not qualify the final gate. The gate now has
  EOF, literal-argument and actual pre-activation owner-kill coverage.
- The initial owner-kill fixture failed three times (15.059 s). Its connected
  Unix datagram socket could not send ancillary descriptors through Go's
  `WriteMsgUnix`; it never delivered the expected observation. A diagnostic run
  identified that exact API error. Using an unconnected socket fixed the fixture;
  the same three owner-death cases then passed (1.118 s).
- The first native journey command had invalid shell quoting around inherited
  PATH and did not run. A corrected three-run command failed (180.055 s): one
  executor build exceeded its existing 90-second context, one dispatch was
  rejected, and the overall test deadline expired during another build. Deadlines
  were not increased in the fixture. A single diagnostic run reproduced rejected
  dispatch (72.460 s). A separate real nested process reproduced the precise
  root-ownership rejection (0.087 s). Explicit supervisor pin transfer fixed it.

## Results at this checkpoint

| Check | Actual result | Local log |
| --- | --- | --- |
| Pre-activation owner death, three runs after fixture correction | PASS 1.118 s | `build/worker-namespace-parent-death-fixed-20261121.log` |
| Nested launch before pin transfer | FAIL 0.087 s, unmapped owner rejection | `build/worker-namespace-nested-before-20261121.log` |
| Shared Linux owner race suite, including nested launch | PASS 4.810 s | `build/worker-namespace-private-runtime-consumers-20261121.log` |
| Full Linux execution race suite | PASS 112.143 s | same consumer log |
| Full Linux inference race suite | PASS 44.664 s | same consumer log |
| Authenticated Linux native compiler/host journey | PASS 51.287 s | `build/worker-namespace-native-runtime-fixed-20261121.log` |
| Full Windows execution suite | PASS 51.549 s | `build/worker-namespace-windows-consumers-20261121.log` |
| Windows executor suite | PASS 10.419 s | same Windows log |
| Activation cases with live-init preconditions, three runs | PASS 4.311 s | `build/worker-namespace-activation-live-init-20261121.log` |
| Prepared child dispatch uses inherited owner, including refusal | PASS 1.202 s under Linux race | `build/worker-namespace-inherited-owner-20261121.log` |
| macOS ARM64 executor cross-build | PASS; not executed | `build/worker-namespace-darwin-build-20261121.log` |
| Complete Linux root-module Go suite on verified source snapshot | PASS; executor 1.722 s, server 87.989 s | `build/worker-namespace-go-checkpoint-20261121.log` |

Commands use Go 1.26.6. Linux checks run under WSL Ubuntu with CGO enabled,
`GOPROXY=off` and `GOMAXPROCS=4`; Windows checks use CGO disabled.

```text
go test -race -p 1 ./internal/processowner ./internal/execution ./internal/inference -count=1 -timeout=5m
go test -race -p 1 ./internal/server -run ^TestNativeApprovalStartsPinnedWorkerOverAuthenticatedBridge$ -count=1 -timeout=2m
go test ./internal/execution ./cmd/offgrid-executor -count=1
```

The native compiler journey uses an explicit PATH containing the installed Go
toolchain and system binaries. Fixtures own their temporary workspaces, sockets,
processes and loopback services. No model downloads, installed-user state, personal
browser sessions, live runtime provisioning or host policy changes were used.

The initial complete `go test -p 1 ./... -count=1 -timeout=6m` attempt produced
no package results after more than seven minutes. The exact test-driver pidfd
received SIGQUIT for diagnosis; its preserved stack shows package-discovery
`MatchDirs`/`EvalSymlinks` traversal, before test execution. This is an interrupted
check, not a passing suite or an application test failure:
`build/worker-namespace-go-all-linux-20261121.log`. Source inventory from tracked
and nonignored untracked Go files identifies all seven maintained roots in
`build/worker-namespace-go-inventory-20261121.json`. The subsequent sweep names
all seven roots explicitly; no maintained Go source root is excluded. Generated
and dependency trees remain untouched. Separate modules/testdata in `runtimes`
and `scripts` produce Go's no-matching-package warning, just as the root module's
recursive test selection omits nested modules and testdata.

The explicit-root sweep completed with one failure: the executor fixture's
unchanged 60-second context expired while building its child executable. All other
tested packages passed; this failing run remains in
`build/worker-namespace-go-source-roots-20261121.log`. To distinguish application
behavior from mounted-filesystem traversal/build overhead, 4,134 maintained source
files (tracked and nonignored untracked, including current edits) were exported
to `/tmp/offgrid-namespace-qualification-20261121`. Every extracted file's SHA-256
was compared with its source. The source archive SHA-256 is
`83e21d78d28f9254504d056838edd64ef90cd80bb48c5e57c9fcf6f46eb18f85`;
`build/worker-namespace-checkpoint-manifest-20261121.json` records the individual
file hashes. It excludes repository metadata and ignored dependencies/build
outputs, not maintained requirements or tests. From that snapshot, the exact
`go test -p 1 ./... -count=1 -timeout=6m` command passed. The unchanged native
executor test completed within its 60-second context (package 1.722 s).
This supports the mounted-checkout overhead diagnosis; the earlier failing result
is not relabeled a pass. Later evidence/architecture documentation edits are
outside that application-source checkpoint and checked separately.

The full snapshot run passed 41 tested packages; 22 additional packages reported
no tests. The broader CI race selection is separately recorded in
`build/worker-namespace-ci-race-rehydrated-20261121.log`. Its first launch could
not start because the temporary source directory was no longer present
(`build/worker-namespace-ci-race-checkpoint-20261121.log`). The archive was then
extracted again and all 4,134 hashes verified before starting the race selection.
The full selection finished successfully: all 22 packages passed, including server
(341.389 s), agents (273.293 s), execution (101.700 s), the shared process owner
(4.941 s), inference (32.129 s) and executor (5.545 s). This qualifies that frozen
Go checkpoint, not later endpoint-owner API/UI changes or the complete product.
Documentation checks pass 487 maintained
Markdown files; source-size, scoped whitespace and strict validation of both
affected OpenSpec changes also pass.

## Primary-source basis and remaining limits

[Go's Linux child setup](https://go.dev/src/syscall/exec_linux.go) compares the
child's parent PID with the original host PID. In a new PID namespace that
numeric comparison is insufficient; the activation pipe is the explicit startup
requirement derived from this review. Installed Go 1.26.6 source was also checked.
[Linux PID namespace semantics](https://man7.org/linux/man-pages/man7/pid_namespaces.7.html)
provide the init-exit descendant teardown contract; the native tests observe
actual child exits rather than assuming it from group counts.
[Parent-death semantics](https://man7.org/linux/man-pages/man2/PR_SET_PDEATHSIG.2const.html)
require retaining the creating thread and covering the pre-arm interval.

This is lifetime containment for approved native code, not a hostile-code
filesystem/network sandbox. Code changing credentials or deliberately clearing
its own parent-death signal requires separate qualification; do not generalize
ordinary fixture owner-death results to it. System libraries/loader remain part
of the trusted host. macOS descendant ownership, other kernels/distributions,
Windows physical persistence, extended churn/soak, authorized background work,
custom environment/toolchain/cache policy, independent review and installed
model/hardware/workflow qualification remain open. Prior inference integrity
and endpoint-enrollment requirements are unchanged. A full CI/release or complete
transformation claim cannot be inferred from this checkpoint.
