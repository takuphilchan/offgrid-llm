# Linux supervised storage ancestry — 2026-11-22

This corrects Linux startup failures introduced by private ancestry enforcement
under `native-agent-workspace-execution`. It preserves the ordinary ancestry,
private-leaf, journal, registry, process ownership and no-replay boundaries.

## Actual failure and diagnosis

The frozen `build/host-model-linux-race-20261122.jsonl` reported
`execution_protocol_invalid` for the real pinned stdio executor and both native
server integration paths. Windows checks passed. The original frozen Linux
stage was no longer present, so its archive was extracted into isolated Linux
storage for a focused reproduction.

`TestPinnedStdioEndpointRunsRealExecutor` actually failed in **0.657 s** without
instrumentation (`build/linux-executor-startup-red-20261122.log`). Capturing
worker stderr only in the disposable diagnostic source copy revealed
`execution_authority_denied` (**0.772 s**,
`build/linux-executor-startup-stderr-20261122.log`). A further diagnostic-only
ancestry trace identified `/tmp`, observed UID **65534**, worker euid **0**
(**0.531 s**, `build/linux-executor-startup-ancestry-20261122.log`). No maintained
source acquired this diagnostic output or a general stderr forwarding path.

The process owner maps the calling account into namespace UID zero. Host-root
ancestors become unmapped there; the ordinary checker correctly refuses to
equate an unknown owner with host root. `XDG_CONFIG_HOME` was already forwarded
through the captured process environment and was not the root cause.

## Scoped correction

The existing private supervised runtime payload now has version 2 and can carry
bounded original-host ancestry witnesses. Actual stdio startup captures them
before entering the namespace, after the normal ancestry checks, for the exact
journal and fixed account-registry paths. Only original host-root directories
are witnessed. Capture requires the original full UID mapping; a nested
namespace cannot manufacture new host-root proof from its apparent UID zero.

Each witness binds its exact path, existing folder identity (device, inode and
birth time), and full directory mode. The receiving pinned worker checks the
single expected UID mapping, exact journal/registry scope, strict payload,
duplicate/extra witness rejection, identity and mode. Its checker belongs only
to that registry instance and is initialized before journal directory creation.
There is no process-global checker override, generic overflow-UID exception or
new model/renderer authority. Missing or unsupported private proof fails closed.
Ordinary account registries and inspection continue using ordinary ancestry
validation. Project subprocess environments do not inherit the private payload.

Bounds: 128 witnessed ancestors, existing 8192-byte path limits, and a 65536-byte
private runtime envelope. Original host administrators remain trusted, as under
the existing ancestry policy. Identity/mode witnesses do not claim resistance to
a privileged host administrator changing ownership after validation; directory
ctime is not pinned because ordinary entry churn changes it.

## Focused verification

After applying the correction to the isolated archive copy, this command passed:

```sh
go test ./cmd/offgrid-executor -run '^TestPinnedStdioEndpointRunsRealExecutor$' -count=1 -timeout=90s -v
```

Result: **PASS, 0.989 s**, including actual owned-command shutdown and independent
prepared-worker restart. Log: `build/linux-executor-startup-green-20261122.log`.
This used the new code with its constructor dependencies, not the original
frozen source bytes. A temporary staging-copy build failed because the new
read-only ownership dependency had not yet been copied; copying that dependency
resolved the build without changing production behavior or weakening a test.

Adversarial checks ran through actual owned Linux user/PID namespaces:

```sh
OFFGRID_TEST_FOREIGN_ANCESTOR=/var/cache/man go test ./internal/execution -run '^Test(SupervisedStorage|SupervisedRuntimePins|WorkingCopyInheritsSupervisedProcessOwner)' -count=1 -timeout=90s -v
```

Result: **PASS, 0.099 s**, with no skips, recorded in
`build/linux-supervised-ancestry-focused-20261122.log`. All twelve namespace
proof cases passed: valid scope, old version, missing storage/roots, unwitnessed
ancestor, altered identity/mode, duplicate/extra ancestors, changed journal or
registry, and wrong UID mapping. A valid remapped worker also refused to mint new
host-root witnesses or inspect an additional scope. The existing private runtime
pin/environment and prepared-worker owner inheritance checks passed.

The real foreign-owner metadata negative passed: parent capture rejected host
UID **6**, mode **0755**, while the actual namespace observed UID **65534**, euid
**0**, and rejected it without a matching witness. No probe directory was created.
This distinguishes legitimate witnessed host-root ancestors from other unmapped
owners without synthetic UID evidence or privilege changes.

`node scripts/maintenance/check-source-size.mjs` passed after this correction.
The subsequent [frozen integration pass](host-protection-integration-2026-11-22.md)
passes all six affected full Windows and Linux race packages, including the
previously failing native-host startup/server journeys. Original failures remain
recorded; actual macOS and broader containment qualification remain open.

## Qualification boundaries

No privileges, account ownership or system-directory permissions were changed.
The selected foreign-owner fixture is existing `/var/cache/man`, metadata-only:
host UID 6 and mode 0755, distinct from both root and the calling UID 1000. No
contents are listed/read and no file is created there. Normal positive and
negative registry/journal state remains in disposable account paths.

The correction enables the established lifecycle owner and private registry to
coexist. It does not establish filesystem containment for trusted local commands,
MCP/plugins, desktop terminals or browser upload snapshots, and does not complete
the overall program or hardware/model qualification.
