# Browser upload task binding and ownership — 2026-11-21

This source increment continues [browser-file-transfers](../../../openspec/changes/browser-file-transfers/tasks.md)
and M2.07. All eight transfer tasks remain incomplete. Tests below use disposable
state and synthetic content on Windows, Node 24.18.0. The new resource owner is
not yet connected to browser dispatch or advertised as a completed capability.

## Task authority

The desktop host accepts an optional canonical run ID from its versioned bridge,
reads the saved waiting browser input, checks its initial-session state, requested
origin and policy, and rechecks after consent. Upgraded enrollment additionally
rechecks the authenticated account and task after pack verification and pairing.
The actor comes from authenticated host inspection, never renderer input.

Private IPC carries the expected actor/workspace/run/input tuple. A matching
companion must explicitly support this binding. Pair replies must match the full
tuple and generated session; every polled action must match before journal
mutation or driver preparation. New journal bindings include the task tuple;
legacy peers retain the old binding without stronger transfer claims. Manual
renewal carries the newly approved input and never restores the previous upload.

The foreign-action baseline failed: a mismatched actor still reached driver
execution. The regression now proves zero preparation, zero execution and zero
dispatch rows for that message. Initial upgraded pairing rejects missing,
foreign, extra and version-skewed binding fields before polling.

An older-companion test exposed another issue: Stop changed the generation and
the startup catch returned state instead of preserving the upgrade error. The
host now preserves that specific startup failure while retaining Stop ownership.
Ordinary task and account changes before launch reject enrollment or spawning.

A subsequent source-contract review caught the host reading an invented
`requested_computer_url` field. The actual saved request carries
`pending_input.url`. The canonical-field regression failed before correction
(the fixture reached startup instead of rejecting and expired after 60.143 s).
Both implementation and positive/negative fixtures now use the actual service
field; the regression also fails immediately if it ever reaches launch again.

## Durable allocation owner

`browser-owned-upload-store.mjs` uses the existing `dispatch.sqlite` upload ledger.
It reserves bytes before allocation, records directory and file identities before
copying content, and records ready only after hash verification, flush and handle
closure. Snapshot identities now use bigint device/inode/birth-time fields.

The owner records possible selection before assignment through its `inUse`
boundary. Session rebinding retains old bytes and charges but cannot reuse the
old grant. Stop settles preparation; browser-close eligibility is a separate
caller assertion that must be supplied only after actual browser termination.
Cleanup brackets exact-object deletion with durable checkpoints. Lost final
receipts retain reservations and permit a live owner's explicit retry. Reopened
owners never adopt or delete prior rows.

This module does **not** prove private ancestry, Windows ACLs, application-tool
exclusion or native process termination. Its constructor requires the future
caller to establish that storage boundary. It has not been wired into production
sessions, and its tests do not certify those prerequisites.

## Actual results

| Check | Result | Scope |
| --- | --- | --- |
| Foreign-action baseline | FAIL, 0.334 s | Mismatched actor executed once before correction |
| First combined binding run | 16 pass / 5 fail, 1.504 s | Test workspace omitted `isConnected`; cleanup correctly failed rather than masking the incomplete fixture |
| Corrected binding/ledger suite | 21/21 PASS, 1.480 s | Canonical tuple, pairing, dispatch and ledger faults |
| First host task suite | 5 pass / 1 fail, 0.084 s | Older-companion rejection swallowed during Stop |
| Corrected host task/control/runtime suite | 47/47 PASS, 0.173 s | Consent, startup races, compatibility and renewal |
| Canonical saved-URL correction | 48/48 PASS, 0.191 s | Actual `pending_input.url` contract, including foreign initial origin |
| Initial snapshot/owner suite | 13/13 PASS, 0.309 s | Real filesystem identities/content, SQLite faults, replacement sentinel, Stop |
| Actual killed-owner checkpoints | PASS, 0.594 s | Six subprocesses, real files and SQLite, no browser process |
| Combined ownership/binding suite | 36/36 PASS, 1.981 s | Includes final removal receipt loss and all six crash phases |
| Web TypeScript | PASS | Current bridge/request types; renderer behavior not yet rerun |
| Complete desktop suite, final host source | 288 pass / 4 skip, 2.642 s | Installed/native opt-in checks remain skipped |
| Complete companion suite, concurrency 2 | 219 pass / 5 skip, 48.603 s | Attachment, maximum upload, transport diagnostic and network-policy probes remain opt-in; known probe failures remain open |
| Focused Go browser binding | Computer 0.191 s / server 0.575 s PASS | Actual service/hub contract; not composed browser evidence |

The focused Go command also compiled execution/agents but selected no tests in
those packages. A separate corrected selection ran the new descendant-proof
wire, leader observation, budget and completion tests: execution 0.306 s,
agents 0.595 s and server 0.489 s, all PASS. These narrow results do not resolve
the previously failing actual native-worker shutdown journey.

The first crash-harness attempt incorrectly paused an async wrapper around a
synchronous ledger method; the caller correctly did not await that test wrapper,
so the supposed directory checkpoint already had a file. A second attempt ran
its microtask before allocation existed. Both failed runs are retained. The final
harness pauses the snapshot's awaited allocation callback after the actual
production checkpoint, then kills only its owned child. These are process-crash
tests, not power-loss, browser-death or installed-filesystem qualification.

Raw local logs are `build/browser-file-binding-baseline-20261121.log`,
`build/browser-file-binding-focused-20261121.log`,
`build/browser-file-binding-focused-final-20261121.log`,
`build/browser-host-task-first-20261121.log`,
`build/browser-host-task-lifetime-20261121.log`,
`build/browser-owned-upload-store-first-20261121.log`,
`build/browser-owned-upload-crash-20261121.log`,
`build/browser-owned-upload-crash-second-20261121.log`,
`build/browser-owned-upload-crash-final-20261121.log` and
`build/browser-upload-ownership-combined-20261121.log`.
Later logs are `build/browser-host-task-url-baseline-20261121.log`,
`build/browser-host-task-url-final-20261121.log`,
`build/browser-binding-desktop-final-20261121.log`,
`build/browser-companion-full-20261121.log`,
`build/transformation-focused-go-20261121.log` and
`build/process-descendant-proof-focused-20261121.log`.

Reproduce with the existing dependencies:

```text
node --test apps/desktop/test/computer-browser-task.test.js apps/desktop/test/computer-runtime.test.js apps/desktop/test/computer-browser-control.test.js
node --test apps/computer/test/browser-owned-upload-store.test.mjs apps/computer/test/browser-owned-upload-crash.test.mjs apps/computer/test/browser-file-snapshot.test.mjs apps/computer/test/browser-file-binding.test.mjs apps/computer/test/session-file-binding.test.mjs apps/computer/test/browser-upload-ledger.test.mjs
```

Remaining gates include independent review, protected host storage, complete
driver/workspace integration, real browser/manual transfer lifecycle, initial
task binding through the composed service, full affected suites, download
completeness/provenance/export and installed/model/hardware/human qualification.
The [512 MiB raw-upload transport failure](browser-upload-transport-2026-11-21.md)
also remains unresolved; successful ownership checks do not qualify it.

The requested independent review did not complete: the review agent hit the
account usage limit. Its absence is not recorded as a clean review. The parallel
provider owner-file integration and Windows worker lifetime investigation also
remain unfinished; inspect current source before continuing them.

Root checked the provider handoff with `go test -p 1 ./internal/server -run
'TestEndpointOwner' -count=1 -timeout 3m`: PASS 1.355 s, recorded in
`build/endpoint-owner-handoff-20261121.log`. This only verifies the existing owner
transaction/host fixtures, not the unfinished UI, generated contracts or complete
provider qualification.
