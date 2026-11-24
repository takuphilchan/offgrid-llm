# Task snapshot writer reservation — 2026-11-21

M1.03 persistence follow-up within `agent-conversation-workflows`. This is a
limited implementation correction; the program and aggregate performance gate
remain incomplete.

## Finding and implementation

The full Linux server race suite twice reached its unchanged eight-minute
deadline on the endpoint-owner journal checkpoint. A separately profiled real
server fixture, `TestDesktopObservationContextContinuesIntoControlWorkflow`,
passed in 36.490 s. Its CPU profile attributed 28.54% of cumulative samples to
task save/persist and 16.99% to `putTaskTransaction`. These overlapping cumulative
figures are not additive or proof of the whole suite's timeout cause. The log is
`build/server-fixture-profile-20261121.log`.

Inspection found that the initial writer reservation updated an existing task
row before the actual snapshot upsert. A database UPDATE-trigger regression with
a 256 KiB task prompt observed two updates instead of one before the fix:
`build/task-writer-before-20261121.log`, FAIL, 0.770 s.

`internal/agents/sqlite_tasks.go` now reserves the SQLite writer using an UPDATE
that matches no rows. Snapshot validation, previous-state comparison, control
projections, activity events and transaction commit remain in the same order.
WAL, FULL durability, busy timeout and operation-scoped connection ownership are
unchanged. No task or external effect is replayed by this change.

[SQLite's transaction documentation](https://www.sqlite.org/lang_transaction.html)
states that a first write statement starts a write transaction, while its
[isolation documentation](https://www.sqlite.org/isolation.html) explains why a
read snapshot cannot safely be promoted after another connection commits.
Both primary sources were checked on 2026-11-21. The regression verifies the
actual pinned driver behavior rather than relying only on this interpretation.

## Verification

`internal/agents/task_writer_reservation_test.go` checks one actual task-row
update and uses two distinct SQLite connections to verify that reservation:

- excludes a competing writer before the first snapshot read (SQLITE_BUSY);
- permits a WAL reader;
- works with empty and populated task tables;
- releases on both commit and rollback.

Windows focused tests passed, 1.093 s, in
`build/task-writer-fixed-20261121.log`:

```powershell
go test ./internal/agents -run '^TestTask(PersistenceWritesSnapshotOnlyOnce|WriterReservationExcludesCompetingWriter)$' -count=1
```

Three 20-iteration Windows benchmark samples used the same 256 KiB prompt:

| Measure | Before | After |
|---|---|---|
| ns/op | 19,516,045; 26,629,290; 15,451,170 | 14,478,665; 21,048,945; 18,636,370 |
| allocations/op | 548; 548; 545 | 533; 530; 530 |

Timing ranges overlap; these short samples do not establish a reliable end-to-end
speedup. Logs are `build/task-persist-benchmark-before-20261121.log` and
`build/task-persist-benchmark-after-20261121.log`.

A Go source overlay replaced only the reservation UPDATE with `SELECT 1`,
without editing maintained source. All four competing-writer cases failed as
expected because another connection could write before the snapshot read:
`build/task-writer-negative-control-20261121.log`, 0.873 s. This negative control
shows that simply removing the redundant write would lose the required lock.

The same isolated server workflow was profiled after the change and passed in
32.399 s (`build/task-writer-fixture-profile-20261121.log`), compared with
36.490 s before. Cumulative task persistence CPU samples were 8.33 s rather than
11.27 s; `putTaskTransaction` samples were 4.26 s rather than 6.71 s. One pair of
profiled runs is insufficient to establish a stable performance improvement.
The profile still attributes substantial time to SQLite statement preparation
and task copying; neither should be optimized by weakening durability or
allowing mutable task snapshots to escape their synchronization boundary.

The full Linux race verification was started from 4,148 byte-verified source
files, archive SHA-256
`a95da667269e3a50caf19489e600f904fdba8ccd69b89830d6460cd4736eefc1`.
It uses fresh ext4 test state, Go 1.26.6, CGO enabled, GOPROXY off and
GOMAXPROCS=4. The complete agents race suite passed in 279.127 s. API passed in
1.074 s and configuration passed in 1.035 s. The full server race suite FAILED
at its unchanged eight-minute timeout, 480.050 s. The retained log is
`build/task-writer-full-linux-race-20261121.log`:

```sh
go test -race ./internal/agents -count=1 -timeout=8m
go test -race ./internal/server ./pkg/api ./internal/config -count=1 -timeout=8m
```

At the deadline `TestAutomaticDependentWorkerValidatesSealedEdits` had been
running for 54 s; its `retained_followup` subtest had run for 3 s. The log contains
no earlier failed assertion or race report. This does not establish that the
active subtest caused the aggregate delay. The correction did not resolve the
aggregate performance gate, and the prior two timeout failures remain evidence.

Documentation link validation passed for 489 maintained Markdown files; the
source-size ratchet, scoped whitespace check and strict
`agent-conversation-workflows` OpenSpec validation passed. These are structural
checks, not substitutes for the failed server suite or real workflow qualification.

No larger deadline, removed assertion, installed-service change, model download
or real hardware qualification is part of this correction.
