# Shared host protection registry — 2026-11-22

This evidence covers the shared account admission registry, bounded settlement
history, and pending-path identity checks supporting protected browser storage.
It does **not** complete M2.07, establish process filesystem containment, or
qualify every platform. The roadmap remains authoritative.

## Implemented boundary

`internal/execution/host_protection*.go` adds a fixed account registry beside
the existing project-host identity directory:
`os.UserConfigDir()/offgrid/host-protection`. Desktop profile selection does not
select a different registry. Tests use explicitly isolated account state.

Transactions reuse `storage.AcquireOwnership` on the private account parent and
the existing SQLite opener. The lock serializes initial creation and later
cross-process admission. Private path and ancestry checks precede creation;
missing account directories are created within a bounded chain without repairing
existing parents. A persistent identity marker outside the database is checked
against database metadata. Missing, malformed, replaced, or oversized retained
state fails closed instead of becoming an empty registry. Rows have bounded,
strictly decoded payloads with integrity digests; the store is limited to 1024
retained records. Digests detect corruption; they are not authentication against
an unrestricted same-account process.

Browser registration is idempotent for an unchanged existing root. A missing root
reserves its nearest existing ancestor identity plus the missing suffix. Only
the original in-memory capability can finalize that reservation after creation.
Crash/reopen does not adopt the newly created object. Canonical comparisons reject
equal, ancestor, and descendant overlap with active execution workspaces **and
journals**, in either admission order. Live registered identities are rechecked;
a moved or replaced active object fences new admission rather than becoming a
release.

Execution reserves before activation. The returned in-memory capability is opaque;
only its hash is stored. `CancelUnactivated` serializes against worker activation.
Once activated, verified endpoint/process cleanup is required to publish release.
There is no release based on PID absence, elapsed time, EOF, or a renderer boolean.
Unresolved rows survive process exit. Matching explicit restart after verified
release receives a new capability; the existing operation journal owns no-replay.
Recording process authority does not contain that process.

## Bounded settlement history

Previously, 1024 successful execution lifetimes could permanently exhaust the
registry because released rows counted toward its fixed capacity. Admission now
reclaims only verified `released` execution rows, keeping up to 128 by their
**admission order**, or fewer when active/browser reservations need room. Browser,
pending, active, and uncertain rows are never reclaimed. There is no time-based
expiration. Reclamation runs after identity/conflict validation and in the same
transaction as insertion, so insertion or COMMIT failure restores prior history.

An original lease records successful SQL COMMIT before deferred database/lock
close operations. Its later release retry is a no-op even after its diagnostic
tombstone is reclaimed. A missing row alone is not proof of settlement. Knowing
an old capability cannot release a fresh or foreign row. Failed COMMIT never
caches settlement. The callback ordering is tested; a real post-COMMIT close
failure has not been injected.

Retired history is not an unbounded session-binding database. Once a tombstone
is retired, fresh trusted admission and the existing grant/operation journal
provide binding and no-replay checks. While a released row is retained, matching
restart validation remains enforced. At 1024 non-reclaimable reservations,
admission still fails closed; this is an active-resource limit, not lifetime
exhaustion.

## Pending suffix correction

Checking only the originally pinned ancestor missed changes to previously absent
descendants. Every pending path is now re-resolved before admission, including
when another reservation already caused the shared ancestor to be checked.
An invalid component or changed canonical path returns
`host_protection_uncertain`. The newer anchor/identity is never adopted.
Expected intermediate/final directory creation at the unchanged path remains
pending and requires the original finalization capability.

## Actual checks and retained failures

These are focused working-tree checks, not results for the earlier frozen source
snapshot or a substitute for the next applicable CI suite.

| Check | Actual result | Log under `build/` |
| --- | --- | --- |
| Moved active root regression before correction | FAIL, 0.671 s | `host-protection-moved-before-20261122.log` |
| Confirmed-close restart regression before correction | FAIL, 0.215 s | `host-protection-restart-before-20261122.log` |
| Original primitive `TestHostProtection*` selection, Windows | PASS, 2.593 s | `host-protection-final-sync-20261122.log` |
| Released-capacity regression before reclamation | FAIL, 0.844 s | `host-protection-reclamation-before-20261122.log` |
| Missing component replaced by file, before correction | FAIL, 0.690 s | `host-protection-pending-before-20261122.log` |
| Windows pending/reclamation/failed-write/bounded selection | PASS, 6.086 s | `host-protection-pending-reclamation-final-20261122.log` |
| Linux pending-path selection | PASS, 0.053 s; no skips | `host-protection-pending-linux-20261122.log` |
| Linux failed-isolated-recovery reservation regression | PASS, 0.038 s; no skips | `host-protection-isolated-recovery-linux-20261122.log` |
| Windows independent native browser/executor admission | PASS, 4.018 s | `host-protection-cross-process-pass-20261122.log` |
| Rebuilt native checker with actual Chromium upload | PASS, 1/1, 3.421 s; no skips | `browser-storage-registry-composed-20261122.log` |

The primitive selection includes actual competing subprocess admission (one
winner), crash-row retention, original capability ownership, root/ancestor
replacement, corrupt/missing state, busy locks, failed INSERT/UPDATE/COMMIT,
pending finalization, safe account initialization, and matching explicit restart.
Those subprocess checks predate the later reclamation/pending changes; they have
not been rerun here against the final combined source.

The capacity fixture seeds reachable reserved-to-released transitions in one
transaction before exercising real admission/release at capacity; it is not a
claim that 1024 native workloads were run. Later checks cover all-active capacity,
mixed active/released capacity, unchanged browser/pending reservations, bounded
history, rejected admission without reclamation, DELETE/INSERT/deferred-COMMIT
rollback, stale capabilities, canceled retries, and commit observation.

The Windows symlink fixture explicitly skips because the test process lacks
symlink privilege. Linux executes both the actual symlink and regular-file
replacement fixtures, plus expected creation and original finalization.

Reproduction commands:

```text
go test ./internal/execution -v -run '^TestHostProtection(Pending|ReleasedCapacity|ReservedCapacity|Reclamation|ReleaseCaches|FailedCommit|FailedWrites|BoundedRecords)' -count=1 -timeout=60s
```

```sh
CGO_ENABLED=1 GOPROXY=off GOMAXPROCS=2 go test -p 1 -v ./internal/execution -run '^TestHostProtectionPending' -count=1 -timeout=45s
```

## Recovery-startup reservation regression

`cmd/offgrid-executor/recovery_protection_linux_test.go` exercises the actual
`run` entry point with a private grant, a disposable absent Docker socket, and a
pre-existing journal fixture recording an uncertain prior container operation.
It never contacts a live Docker daemon or starts a container. Failed isolated
recovery must retain the new reservation instead of canceling it as an unactivated
attempt. Workspace and journal browser overlaps must remain denied; another
attempt must remain uncertain; unrelated browser admission must still work.
The original journal ownership evidence must remain and its dispatched operation
must become uncertain, without replay.

Actual Linux result: **PASS, 0.038 seconds**, with no skips. An initial test
fixture used positional parameters across multiple SQL statements incorrectly;
the resulting ownership assertion failed. Separate parameterized INSERTs corrected
the fixture. That development failure is retained in
`build/host-protection-isolated-recovery-fixture-failure-20261122.log`; it is not
reported as a production pre-fix regression.

```sh
CGO_ENABLED=1 GOPROXY=off GOMAXPROCS=2 go test -p 1 -v ./cmd/offgrid-executor -run '^TestExecutorFailedIsolatedRecoveryRetainsProtection$' -count=1 -timeout=30s
```

## Cross-client and real browser composition

`TestIndependentHostsProtectCustomBrowserProfile` starts actual native browser
storage and executor processes with an isolated account configuration, a custom
browser profile, and no desktop-home environment dependency. It exercises both
launch orders: browser registration excludes an independent overlapping CLI;
an existing executor excludes browser storage until verified executor EOF cleanup.
Actual Windows result: **PASS, 4.018 seconds** in
`build/host-protection-cross-process-pass-20261122.log`.

Three earlier fixture failures remain in
`host-protection-cross-process-20261122.log`,
`host-protection-cross-process-final-20261122.log`, and
`host-protection-cross-process-verified-20261122.log` under `build/`. They exposed
an incomplete strict readiness schema, a deleted subtest profile retained in a
shared account registry, and an assertion expecting the wrong denial wire format.
The fixture now validates the complete schema, uses an independent account per
case, and parses the actual framed denial. No production assertion or uncertain
reservation was relaxed to hide these failures.

The rebuilt native storage checker also passed the actual managed Chromium upload
composition in `apps/computer/test/browser-storage-native.test.mjs`: **1/1 PASS,
3.421 seconds**, no skips, recorded in
`build/browser-storage-registry-composed-20261122.log`. This is evidence for that
native-storage/browser integration path, not proof of filesystem containment
against arbitrary coding, terminal, MCP, or plugin processes.

## Remaining qualification and containment

The combined 4,251-file frozen source snapshot
`build/host-model-checkpoint-20261122.tar` (SHA-256
`b3b0cacd030c4cb150645a33d4ea97b4d0315e4639b5e17ba2ee53e3be51d8ec`)
passed full Windows execution **57.080 s**, native computer **8.377 s**, executor
**7.135 s**, computer **2.084 s**, agents **131.851 s** and server **175.793 s**.
The extracted sources were verified against every manifest digest. Results are
in `build/host-model-full-windows-20261122.log`. Matching Linux race results are
execution **PASS 114.084 s**, native computer **PASS 2.160 s**, computer **PASS
4.191 s**, agents **PASS 222.858 s**, executor **FAIL 7.587 s** and server **FAIL
324.787 s**, retained in `build/host-model-linux-race-20261122.jsonl`.
The three native-host startup failures exposed namespace UID translation:
`/tmp` appears as UID 65534 while worker effective UID is 0, so the new ancestry
check rejects startup. The server's error-reporting branch also raced its stderr
copier. These are genuine integration failures; the source correction must retain
strict ancestry and namespace lifecycle protection and receive affected reruns.
Subsequent read-only inspection and isolated-activation changes are separate
increments and are not covered by this snapshot.

Complete companion checks on the same 243 computer/desktop source files passed
**238/244 tests, 6 opt-in skips, 43.225 s**; desktop passed **290/294 tests,
4 opt-in skips, 3.096 s**. Logs are
`build/host-registry-companion-full-20261122.log` and
`build/host-registry-desktop-full-20261122.log`. The relevant maintained files were
hash-compared with the frozen manifest by
`build/verify-companion-desktop-source-20261122.mjs`.

- Shared registration fences cooperating OffGrid supervisors. A same-account
  unrestricted shell, PTY, MCP subprocess, or plugin can still access private
  browser files outside its declared workspace. Job Objects and PID namespaces
  establish lifecycle ownership, not filesystem confinement. The registry must
  not be presented as solving this boundary.
- The existing Linux isolated executor is a concrete containment integration
  candidate. Its real daemon/image, mount, host IPC/network, descendant, Stop,
  crash/recovery, and browser/coding coexistence qualification remains necessary.
  Other launch surfaces must use the boundary too; approvals and path declarations
  alone cannot substitute for it.
- Native Windows AppContainer/restricted-process policy and native macOS policy
  still require implementation/integration and actual platform qualification.
  Native Darwin alias/ACL evidence is tracked separately; cross-compilation is
  not native execution evidence. See
  [private ancestor evidence](private-storage-ancestry-2026-11-22.md).
- Directory-entry durability across Windows power loss remains unqualified.
  Linux/macOS directory-sync code does not establish Windows equivalence.
- Reclamation does not provide administrative recovery of unresolved browser or
  execution reservations. No uncertain row was deleted to make a test pass.

The final corrected [host integration snapshot](host-protection-integration-2026-11-22.md)
passed all six affected Windows and Linux race packages. It includes registry
inspection, isolated cancellation and the namespace ancestry correction; the
earlier startup failures remain in the evidence. Actual containment and owner
recovery requirements listed above remain open.

Current primary comparisons support separating enforcement from approval and
mounting only intended writable resources: [OpenHands Docker sandbox](https://github.com/OpenHands/docs/blob/main/openhands/usage/sandboxes/docker.mdx)
and [Open Interpreter sandbox contract](https://github.com/openinterpreter/openinterpreter/blob/main/docs/sandbox.md).
Goose maintainers [confirmed withdrawal of the earlier macOS sandbox experiment](https://github.com/aaif-goose/goose/issues/10895);
its old feature description is not evidence of current tool containment.
The [Linux Landlock documentation](https://docs.kernel.org/userspace-api/landlock.html)
explicitly lists filesystem metadata operations it cannot fully restrict, so
Landlock alone over a visible host filesystem does not prove this storage gate.
