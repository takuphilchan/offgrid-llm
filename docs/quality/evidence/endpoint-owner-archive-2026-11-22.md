# Endpoint receipt archival — 2026-11-22

This is a partial task 8.4.4 implementation within `inference-provider-runtimes`,
supporting M1 durable receipts and M5 endpoint administration. It does not
complete endpoint maintenance, the server performance gate or the program.

## Implemented boundary

`POST /api/v2/inference-endpoints/configuration/archive` requires the current
administrator and exact configuration/journal revisions. The existing owner
writer lease and mutex serialize it with publication. It accepts only a journal
whose records are all `committed` or `not_applied`; pending, published and
conflicting effects must be reconciled first.

Each settled receipt is copied using the existing private durable JSON writer,
then read back and compared before the active journal is cleared. A workspace
archive identity binds the version-2 journal to historical files. Each receipt
has a fixed hash-derived filename and a 4 KiB read bound, so duplicate lookup
does not scan the growing history. The original actor, review, epoch, digests
and outcome remain unchanged; archival actor/time are separate metadata.

Interrupted partial copies leave the active journal intact. A lost final write
acknowledgement returns uncertainty and requires a fresh read. No automatic
retry, endpoint-file publication, access-policy mutation, grant, runtime launch
or provider contact is performed. Missing archive identity or missing active
journal after archival fails closed. Version 1 remains readable; version 2 is
written only by explicit successful archival and is rejected by older services.
Workspace backups retain the archive alongside the journal. No history is
silently deleted to release capacity.

The archive trusts the existing private workspace/same-account boundary; it is
not an independent tamper-proof ledger. Disk use grows with retained history,
and the existing workspace backup file/byte limits still apply. There is no
automatic expiry or claim of unlimited disk capacity.

The same request identity must return its original result or reject different
intent after archival. This acceptance matches the token/parameter distinction
in [Amazon EC2's primary idempotency documentation](https://docs.aws.amazon.com/ec2/latest/devguide/ec2-api-idempotency.html),
checked on 2026-11-22. OffGrid deliberately retains its own no-automatic-replay
rule rather than inheriting a remote service's retry recommendations. The
existing durable writer uses file synchronization plus platform namespace
publication; [Go's File.Sync contract](https://pkg.go.dev/os#File.Sync) alone does
not establish crash or power-loss qualification.

## Actual results

- Windows focused archival tests: PASS, 8.292 s,
  `build/owner-archive-final-focused-20261121.log`.
- The suite exercises before/after failures at archive identity, receipt copy
  and journal replacement; process exit after all three actual write boundaries;
  restart, immutable duplicate outcomes, conflicting review identity, stale CAS,
  all unsettled states, cancellation before clearing, corrupt receipt retention,
  missing identity/journal and a full 64-record active journal. The 64-record
  boundary test supplies validated historical records; it is not a 64-operation
  real-provider qualification.
- Typed client contracts: 7 PASS, 4.7 s,
  `build/owner-archive-client-20261121.log`. They include missing/stale-shape
  journal tokens, unsettled history, mismatched count/digest and no retry after
  a lost acknowledgement. Generated API check and TypeScript check passed.
- Production renderer build passed in
  `build/owner-archive-web-build-20261121.log`; existing Vite/Rolldown build
  warnings remain recorded.
- Actual freshly built service plus renderer: 1 PASS, 11.2 s (body 9.4 s),
  `build/owner-archive-service-20261121.log`. The existing create/update/lost
  response/reconcile/restart journey now explicitly archives two settled real
  receipts, restarts again, retrieves both through duplicate requests and checks
  byte-identical configuration and access policy. Access remains review-required.
  Archival is invoked through the actual API, not a rendered maintenance button.
- Linux focused endpoint race checks passed in 12.264 s. Full Windows server
  passed in 232.873 s, API in 2.351 s and configuration in 2.453 s. Logs:
  `build/owner-archive-full-windows-20261122.log` and
  `build/owner-archive-linux-race-20261122.log`.

Linux source checkpoint: 4,152 byte-verified maintained files; archive SHA-256
`7d0810ba5f6779a7d7fa59ffab7e5baf5b5490e27dab5d38b2d7116cec057ff6`.
Go 1.26.6, CGO enabled, GOPROXY off, GOMAXPROCS=4 and fresh ext4 test state.

```sh
go test ./internal/server ./pkg/api ./internal/config -count=1 -timeout=8m
go test -race ./internal/server -run 'Test(Endpoint|InferenceEndpoint)' -count=1 -timeout=8m
```

## Receipt-size review correction

Review found that a timestamp with valid RFC3339 fractional precision can fit
the active journal while making the archived receipt exceed its 4 KiB read
bound. The initial archiver copied that file, then rejected it on read-back.
The regression reproduced an unreadable published receipt, FAIL 0.912 s in
`build/owner-archive-bound-before-20261122.log`.

The archiver now validates encoded size before copying and retains active
history without publishing the oversized file. All Windows owner tests passed
afterward in 11.758 s (`build/owner-archive-bound-fixed-20261122.log`). The full
Windows and actual service results above predate this additional guard.
Corrected Linux endpoint race verification passed in 10.446 s from 4,153 byte-verified
files, SHA-256
`78ffb8ba5e54af90a9d7a52f106850b71c33e52419a2fa8c6f06f68d0e467ab5`,
with output in `build/owner-archive-bound-linux-race-20261122.log`.

Documentation links (490 maintained files), source-size ratchet and strict
OpenSpec validation passed. No task checkbox was completed and no additional
commit or push was made for this increment.

## Remaining acceptance

Task 8.4.4 remains unchecked. Rendered explicit archival review, browsable
historical receipts/export, retirement of obsolete access decisions without
reviving grants, damaged-state recovery and archival backup/restore qualification
remain unfinished. The policy still has its separate 64-decision limit. Do not
claim that this journal-only increment permits unlimited endpoint revisions.

The full Linux server race suite previously timed out at its unchanged
eight-minute deadline after the task-writer correction; see
[persistence evidence](task-writer-reservation-2026-11-21.md). Focused endpoint
checks cannot replace that full gate. Independent security review, real
providers/models, installed platforms, power-loss/storage qualification and
human accessibility/language/workflow qualification remain open.
