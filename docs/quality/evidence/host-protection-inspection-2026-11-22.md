# Read-only host protection inspection — 2026-11-22

This increment makes retained account reservations inspectable when a missing,
changed, or unfinished path prevents new admission. It adds observations and a
fixed native diagnostic, not recovery authority. It does not release reservations,
adopt replacement paths, replay effects, delete resources, or complete the browser
storage/process-containment gates.

## Implemented contract

`HostProtectionRegistry.Inspect(ctx, after, limit)` accepts an opaque cursor and
a page limit from 1 to 64. It validates every stored row within the existing
1024-record bound before returning an ID-sorted page. Corruption outside the
requested page still fails the inspection. Pages are deterministic within each
snapshot; concurrent changes between separate page requests can change later
results.

Output distinguishes a never-created registry (`status: absent`) from an existing
valid registry (`status: ready`). Each record reports its opaque ID, kind, stored
state, backend, binding summary, root/journal observations, admission scope and
blocking reason. It does not export release capabilities, capability hashes,
native identity hashes, grant tokens or registry credentials. Paths are intended
for trusted local diagnostics; this is not a model tool or remote account catalog.

Observations include matching, missing, replaced, redirected, unavailable,
still-missing pending paths and created-but-unfinalized pending paths. A changed
record is reported rather than invoking the global admission fence and losing
the diagnostic. Expected pending creation remains pending. A missing path from a
verified released record does not falsely become an admission blocker.

The inspector validates private ancestry, existing paths, marker identity,
database integrity and stored payloads without creating or repairing them.
`storage.AcquireExistingOwnership` opens the existing regular ownership file
read-only, checks its identity around lock acquisition, and uses the same OS lock
and release semantics as ordinary account ownership. It never creates a parent
or lock file. A concurrent owner produces `host_protection_busy`.

Under that exclusive account lock, SQLite is opened with read-only, immutable
and query-only options. This deliberately avoids the normal `OpenSQLite` setup
and its WAL initialization. Immutable inspection is used only after confirming
there is no nonempty WAL or rollback journal. Retained recovery state returns
`HostProtectionSnapshotUnavailableError` with `retained_wal` or
`retained_journal`; it is never silently omitted to show stale rows. A nonempty
journal is conservatively refused without trying to decide whether it is hot.
No WAL checkpoint, journal recovery, sidecar creation or permission repair is
performed.

## Fixed native CLI

The matching newly built executor supports:

```text
offgrid-executor --host-protection-status
offgrid-executor --host-protection-status --host-protection-limit 1
offgrid-executor --host-protection-status --host-protection-limit 1 --host-protection-after <opaque-record-id>
```

The command selects only the current account's fixed registry. It accepts no
caller-selected registry path and rejects combinations with stdio, connection,
grant, state or positional arguments. It requires no execution credential and
starts no worker or native input owner. Its inspection context is bounded to ten
seconds. Failures produce an unavailable diagnostic reason, including owner busy,
corruption, retained journal, authority or record-limit failures. These commands
describe source behavior, not an installed-product update or release.

## Actual verification

All fixtures use isolated state. Positive private-directory fixtures use a checked
temporary ancestor or a newly created private test directory under a checked home
ancestor. No installed registry or existing user permissions were changed.

| Environment and selection | Actual result | Log under `build/` |
| --- | --- | --- |
| Windows x64, existing-lock and inspector tests | storage PASS 2.799 s; execution PASS 1.501 s | `host-protection-inspect-windows-bounded-20261122.log` |
| Linux/WSL, same bounded selection | storage PASS 0.008 s; execution PASS 0.250 s | `host-protection-inspect-linux-final-20261122.log` |
| Windows native CLI subprocess and flag tests | PASS 0.667 s | `host-protection-cli-inspection-20261122.log` |

No cases skipped in these selections. Commands:

```text
go test ./internal/storage ./internal/execution -v -run '^Test(ExistingOwnership|HostProtectionInspect)' -count=1 -timeout=60s
```

```sh
CGO_ENABLED=1 GOPROXY=off GOMAXPROCS=2 go test -p 1 -v ./internal/storage ./internal/execution -run '^Test(ExistingOwnership|HostProtectionInspect)' -count=1 -timeout=60s
```

The tests compare bytes, permission modes and directory/file inventories before
and after clean inspection, absent state, invalid requests, corrupted/partial
stores, retained recovery sidecars, actual SQLite WAL state and busy ownership.
They cover deterministic pagination, omission of private authority fields,
missing/replaced roots and journals, pending creation without adoption, a corrupt
record beyond the first page, the 1025-record rejection boundary, released history,
and mutual exclusion between existing-only and ordinary ownership acquisition.

The CLI tests run an actual subprocess with an empty execution credential, check
absent-account inspection creates no OffGrid directory, and inspect two records
through separate pages. They also reject mixed authority flags.

Development failures are retained rather than relabeled as product regressions:

- `host-protection-inspect-first-20261122.log`: the fixture tried to read bytes
  from a Windows lock file while its byte range was locked. The fixture now
  captures before acquisition and after release, and always defers cleanup.
- `host-protection-inspect-windows-20261122.log`: a no-op SQL update did not
  generate WAL content. A real metadata update now creates retained WAL and
  proves inspection refuses it without changing the source.
- `host-protection-inspect-linux-bounded-20261122.log`: shell argument quoting
  failed before Go ran. The subsequent command preserved the regex correctly.

## Limits and follow-up

- The later [frozen integration run](host-protection-integration-2026-11-22.md)
  covers this inspector and namespace correction with six full Windows and Linux
  race package passes. The earlier `b3b0cacd` snapshot did not cover inspection;
  its failures remain preserved rather than superseded silently.
- Native macOS execution, private-ACL and filesystem alias qualification remain
  open. Windows/Linux checks are not substitutes.
- Inspection reports a point-in-time observation, not a proof that a process has
  stopped or that a path will remain unchanged after return.
- Retained WAL requires the existing owner/recovery path; this inspector reports
  that prerequisite instead of performing recovery. Corrupt state remains intact.
- Surviving-owner settlement, explicit isolated cleanup retry, browser-profile
  retirement and lost-owner recovery are separate unfinished work. The inspector
  cannot turn user intent, PID absence, EOF or elapsed time into cleanup evidence.
- Same-account unrestricted coding, PTY, MCP and plugin processes remain outside
  the filesystem containment boundary. The registry and inspector do not deny
  their arbitrary filesystem access. See the
  [registry evidence](host-protection-registry-2026-11-22.md).
