# Worker environment and dispatch evidence — 2026-11-21

Continues [Conversation task 13.1](../../../openspec/changes/agent-conversation-workflows/tasks.md)
and M3.07. The task and overall program remain incomplete. This source increment
builds on the uncommitted worker-ownership changes; it is not a repository-wide
qualification checkpoint.

## Implemented contract

`internal/execution/process_environment.go` captures the existing allowed host
environment at `OpenSession`, identifies it as `minimal-host-v1`, and resolves
commands using its PATH/PATHEXT. Prepared workers clone the parent's snapshot.
Later ambient changes cannot change those values or executable search. Explicit
relative commands use the approved cwd; empty/relative PATH entries do not
implicitly choose project executables. The existing credential exclusions remain.

`process_manifest.go` publishes a private per-command manifest before journaled
local dispatch. It binds operation, request digest, actor/workspace/session/grant,
selected program, cwd and environment descriptor. The descriptor stores key names
and a digest, without raw values. The session's saved descriptor must still match.
The transaction verifies both the manifest and unchanged claim/environment rows;
ignored or altered writes cannot launch a command. Schema 4 upgrades old journals
without inventing prior manifests or replaying pending work. Working-copy records
retain their inherited environment descriptor independently of content manifests.

## Executed checks

Windows 11, Go 1.26.6 (CGO disabled); WSL Ubuntu, Go 1.26.6 with CGO/race enabled.
All state is disposable fixture data. Commands only run fixture executables and
the existing approved native compiler journey; no models, installed state or
personal sessions are used.

| Check | Result | Log |
| --- | --- | --- |
| First full Windows execution suite | FAIL 21.188 s: old future-schema test still used version 4 | `build/worker-environment-first-20261121.log` |
| New test compilation | FAIL: output byte slice compared to string | `build/worker-environment-windows-20261121.log` |
| Full Windows execution after compile fix | FAIL 20.411 s: new fixture lacked process permission | `build/worker-environment-windows-final-20261121.log` |
| Linux environment race before permission correction | FAIL 0.132 s, same fixture defect | `build/worker-environment-linux-20261121.log` |
| Full Windows execution after corrections | PASS 21.617 s | `build/worker-environment-windows-pass-20261121.log` |
| Linux environment race cases | PASS 2.139 s | `build/worker-environment-linux-pass-20261121.log` |
| Linux process-manifest race cases | PASS 2.186 s | `build/worker-manifest-linux-20261121.log` |
| Actual authenticated native compiler/shutdown journey | PASS 4.717 s | `build/worker-environment-native-20261121.log` |

The subsequent consumer check passed execution (20.161 s), agents (62.913 s) and
executor (4.202 s), but initially failed server. Its fixture reconciliation and
newly reproduced completion-ownership defect are recorded in
[completion evidence](worker-completion-2026-11-21.md). After correction, the full
Windows server suite passed 112.915 s and focused Linux completion race passed
1.031 s. Documentation (485 maintained Markdown files), source-size ratchet,
strict Conversation OpenSpec validation and scoped whitespace checks passed at
the environment checkpoint; later documentation additions require their own check.

The future-version rejection now uses version 5; a new explicit schema-3 migration
test verifies that old unfinished work stays uncertain and receives no fabricated
manifest. Negative publication tests inject ignored inserts and changes to the
manifest, claim and environment rows, asserting no registered native process and
no partial committed manifest. Actual child execution verifies the original LANG
and absent supervisor token after the parent's ambient environment changes.

Commands:

```text
go test -p 1 ./internal/execution -count=1 -timeout 3m
CGO_ENABLED=1 GOPROXY=off GOMAXPROCS=4 go test -race -p 1 ./internal/execution -run Environment -count=1 -timeout 1m
CGO_ENABLED=1 GOPROXY=off GOMAXPROCS=4 go test -race -p 1 ./internal/execution -run ProcessManifest -count=1 -timeout 1m
```

## Remaining work

This initial template does not provide custom environment selection, executable
byte/package pinning, private home/dependency caches, environment portability or
installed-toolchain qualification. Trusted native project code still has the
approving account's authority. The isolated-container backend retains its own
existing environment policy. The low-level trusted `ProcessManager.Start`
primitive has no journal; authenticated `Worker` dispatch supplies the manifest
journal. These tests do not complete Linux descendant ownership, background
supervision, all affected CI, independent review, model or hardware qualification.

The full Linux execution race failure remains recorded in
[worker ownership evidence](worker-job-watch-2026-11-21.md); focused passes here
do not supersede it. Its separate uncertain-close bookkeeping fix passed the
actual escaped-child race test (2.133 s), Windows working-copy tests (5.255 s), and
Linux ignored-state-write/retry race test (1.083 s,
`build/worker-close-receipt-linux-20261121.log`).

Primary [Linux PID namespace documentation](https://man7.org/linux/man-pages/man7/pid_namespaces.7.html)
and [util-linux namespace launcher documentation](https://kernel.googlesource.com/pub/scm/utils/util-linux/util-linux/+/refs/tags/v2.41.1/sys-utils/unshare.1.adoc)
were checked for the next ownership increment. Namespace-init exit supplies a
kernel descendant-lifetime boundary; the current inference owner's live identity
capture still needs adaptation for immediately exiting worker commands. No new
Linux ownership implementation or namespace qualification is claimed here.
