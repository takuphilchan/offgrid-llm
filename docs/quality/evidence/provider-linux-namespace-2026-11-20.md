# Linux provider namespace lifecycle evidence

Date: 2026-11-20. Change: `inference-provider-runtimes`, prerequisite within open
task 8.4. Restricted Linux enrollment is implemented; full provisioned-factory
success and real-provider qualification remain unverified.

## Implemented and exercised boundary

`internal/inference/provider_namespace_linux.go` and its identity helper provide
an owned Linux user/PID/mount namespace for a trusted runtime command. A reviewed,
SHA-256-pinned util-linux `unshare` executable launches fixed arguments with a
private proc mount and parent-death termination. The existing service must supply
the complete runtime environment; this helper does not authorize arbitrary
renderer commands. The creating Go OS thread remains alive until its monitor is
reaped, preserving the actual Linux parent-death relationship.

The service retains monitor/init pidfds and a PID namespace descriptor, checks
the unique child, namespace-init PID, exact monitor/child PID and user namespace
relationships, and one-entry UID/GID mappings to the current account. Close kills
the retained namespace init, waits for its exit and reaps the monitor within ten
seconds. Linux namespace-init teardown includes descendants that created another
session. Unproven termination returns `errProviderStopUncertain` and retains the
ownership handles. No process-group fallback, cgroup delegation, host mount or
sysctl change is attempted.

The launcher is opened nonblocking and without following a final symlink, then
checked as the same bounded ordinary executable before hashing. An opened file
descriptor prevents a path replacement from selecting another launcher at exec;
it does **not** prevent an authorized writer from changing that file's bytes.
The restricted factory therefore adds administrator-owned, non-service-writable
artifact/ancestor checks before invoking this lifecycle primitive.

## Actual checks and failures

Host: isolated WSL Ubuntu process fixtures, Linux
`6.18.58.1-microsoft-standard-WSL2`, util-linux `unshare` 2.41.3. The service account
was unprivileged. An isolated `unshare --user --map-root-user --pid --fork
--mount-proc true` probe succeeded. The host cgroup root was not writable and was
not changed. No model weights or installed inference runtime was used.

- `CGO_ENABLED=1 go test -race ./internal/inference -run ProviderNamespace
  -count=1` initially passed in 2.313 s.
- Expanded user-namespace and monitor-death tests exposed `poll` returning EINTR
  during the owner-death test (failed in 1.325 s). The test's wait now retries
  interruption against its original five-second deadline, rather than extending
  or removing that deadline. Production nonblocking pidfd checks also retry
  interruptions with a finite bound. The unchanged exit expectations then passed
  in 2.352 s.
- Independent review identified a regular-file-to-FIFO substitution between
  launcher Lstat and open. The opener was changed to nonblocking/no-follow and a
  regression now performs that substitution with an actual FIFO. The latest
  complete focused namespace race run passed in 2.297 s.
- Actual subprocess checks include child/grandchild `setsid`, private proc PID
  identity, two independent live namespaces, concurrent Close, clean init exit,
  monitor SIGKILL, service-owner SIGKILL, missing runtime, mismatched launcher
  digest, and the FIFO substitution. The observer retains host pidfds received
  via Unix socket peer credentials; it checks process exit rather than merely
  accepting a helper's textual claim.
- After Linux factory/listener integration, namespace race checks passed again
  in 2.339 s and the expanded namespace/live-listener run passed in 2.972 s. The
  live HTTP test accepts the namespace's server and rejects an unrelated server
  before any HTTP request reaches it; retired ownership also rejects new dials.
- Linux private-state/artifact rejection race checks passed in 1.023 s, exact
  TCP tuple/parser checks in 1.026 s, and existing service enrollment/selector
  tests in 0.010 s. Service enrollment remains metadata-only; its test reaches
  the actual factory and proves missing files fail without launching a process.
- Review found missing opened state-directory identity checks. The corrected
  opener compares pre-open/opened identities and private owner/mode, with
  root-owned non-writable ancestors rechecked around open. The replacement
  regression and state/DAC checks passed under race in 1.027 s. That run explicitly
  skipped the unprovisioned full-factory test below; the skip is not qualification.
- The initial compile of that new optional test failed because it referred to
  `ProviderRequest.Payload` instead of the existing `Prompt` field. It was fixed
  without changing the intended generation/cancellation assertions.
- Windows managed/HTTP/process/uncertainty regression checks passed in 53.252 s
  after the shared inventory extraction and fixture update; Windows managed
  service enrollment passed in 0.601 s. Maintained docs passed (449 files, zero
  problems), source-size ratchet and strict change validation passed, and diff
  whitespace checks reported only line-ending conversion warnings. The fixture
  provisioning Python AST parsed successfully without executing provisioning.
- A later combined WSL command was rejected by shell quoting before its final
  namespace/HTTP checks began. It is not a successful rerun; the actual executed
  race checks are recorded above. The separate Python AST check subsequently
  succeeded using a direct argument invocation.

The tests explicitly skip unavailable unprivileged namespaces or absent
util-linux; such a skip is missing platform qualification, not a pass. The stated
host completed the live scenarios. These checks establish only the exercised
process-lifecycle behavior, not real inference, memory reclamation or production
platform qualification.

## Remaining integration and qualification

The existing HTTP factory, service selector and shared scheduler now reach
`managed_ollama_linux.go`. Admission requires a non-root service without process
capabilities, a separately pinned root-owned launcher, root-owned runtime/model
files and ancestors that reject group/other writes, no symlinks or regular-file
hardlinks, the existing complete runtime/legacy model inventory, and separate
private service-owned state under root-owned non-writable ancestors. No host
provisioning occurs. Hashing, actor revalidation, restricted environment,
namespace readiness, exact TCP tuple/socket-inode/retained-pidfd membership and
tree retirement share the existing HTTP provider/scheduler path.

The integrity boundary excludes privileged administrator mutation and trusts the
base OS loader/libraries. The state flock coordinates cooperating instances,
not hostile same-UID code deleting its lock; the lock inode remains in place
after ordinary retirement. User-owned runtime/model trees, root-run services,
missing namespace/procfs support and ambiguous listeners fail closed. Broader
Linux installation modes, private authenticated daemon IPC and measured resource
admission remain open.

### Explicit full-factory fixture procedure: not executed

No privileged provisioning was run in this session. The following procedure
creates only a new isolated fixture tree and contains no model weights. It is
intended for an administrator-approved test VM/account. Review the script first;
the script is never invoked automatically by Go tests or the application.

```sh
go build -o /tmp/offgrid-linux-provider-fixture ./internal/inference/testdata/managedollama
sudo python3 internal/inference/testdata/managedollama/provision-linux-fixture.py "$(id -u)" /tmp/offgrid-linux-provider-fixture
```

The explicit administrator command prints the newly created
`/opt/offgrid-linux-provider-test.<unique>/fixture.json`. Run as the original
non-root service account, substituting that exact printed path:

```sh
OFFGRID_MANAGED_LINUX_FIXTURE=/opt/offgrid-linux-provider-test.<unique>/fixture.json CGO_ENABLED=1 go test -race ./internal/inference -run '^TestManagedOllamaLinuxProvisionedFactorySchedulerAndCancellation$' -count=1 -v
```

The test drives the actual registered factory/scheduler through streaming,
legacy-provider switching and cancellation. Unix socket credentials and retained
host pidfds observe the real fixture parent/child/grandchild exits. It creates
only an observer socket and runtime state within the explicitly provisioned test
tree. Without the record it explicitly skips. Preserve output and the exact
fixture/runtime/launcher/source identities; do not mark the gate passed from
compilation or the ordinary skipped run. After inspection, an administrator can
remove only that exact newly provisioned fixture tree and the isolated binary.

This is process lifecycle containment, not a filesystem/network sandbox. Local
daemon authentication, hostile same-account code, measured memory/admission,
real Ollama/vLLM models, network-denied operation, tool workflows and hardware/soak
qualification remain separate open requirements. Nothing here changes the default
provider or existing Linux ProcessProvider behavior.

For qualification, provision an approved isolated supported Linux environment
and run the focused command above without skips; preserve kernel/util-linux/Go
versions, source identity and full output. After the full-factory fixture gate,
repeat actual provider load/generation/Stop/owner-death/switching while recording
all process identities and measured memory, then run the frozen workflow suite
three times per declared model/hardware profile. Do not substitute these fixtures
for those results.

## Primary sources reviewed

- [Linux PID namespace semantics](https://man7.org/linux/man-pages/man7/pid_namespaces.7.html)
- [Kernel namespace teardown implementation](https://github.com/torvalds/linux/blob/master/kernel/pid_namespace.c)
- [util-linux parent pidfd and namespace launcher](https://github.com/util-linux/util-linux/blob/master/sys-utils/unshare.c)
- [pidfd identity and lifetime](https://man7.org/linux/man-pages/man2/pidfd_open.2.html)
- [cgroup delegation requirements](https://docs.kernel.org/admin-guide/cgroup-v2.html)

The installed Go Linux fork implementation was also inspected: its parent-death
check compares the original host parent PID, whereas a newly created PID namespace
observes that external parent as PID zero. The implementation therefore uses the
reviewed namespace monitor rather than combining Go's parent-death check directly
with CLONE_NEWPID.
