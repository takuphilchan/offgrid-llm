# Upload ownership ledger foundation — 2026-11-20

This implements only the bounded persistent-state portion of the
[upload recovery investigation](browser-upload-recovery-design-2026-11-20.md).
It is not yet wired into the snapshot, browser driver, session, installed host or
service protocol. Browser transfer tasks 1.1/1.2/3.1 and the overall program remain
incomplete. The existing generic browser pairing lacks the actor/run/input binding
needed by this ledger; production integration must add the negotiated canonical
contract rather than infer identity from an action string.

## Implemented boundary

`apps/computer/browser-upload-ledger.mjs` and its narrow values module open a
separate owner connection to the existing private `dispatch.sqlite`. Additive
STRICT tables have their own version-one metadata and persist aggregate limits.
They never change dispatch `user_version`, tombstones or result cleanup. Each
instance has a fresh private owner identity. Retaining that instance with a manual
browser survives closure of the old session's DispatchJournal; reopening a new
instance cannot adopt the old owner's resources.

The synchronous transactional API reserves capacity before allocation, records
directory/file identity, marks verified preparation, and writes selected-possible
state before assignment. The binding has exactly actor/workspace/run/input/session
IDs; no URL, token, original path or arbitrary JSON is accepted. Run/input IDs use
their canonical syntax, action IDs must belong to that run, and optional document
identity is a bounded UUID. Native filesystem identity uses positive bounded
decimal strings from bigint stat fields; lossy number conversion, zero and
unsupported identity are rejected.

Preparation abort can authorize cleanup of an unassigned snapshot. It cannot
clear selected-possible state. Only a caller assertion on the current live owner
can record browser closure for selected resources. Cleanup is bracketed by
`beginCleanup` and `removed`; this module performs no filesystem deletion and does
not itself prove native browser closure. Closing the ledger does not release
capacity. Reopened rows—including interrupted deletion and a persisted
browser-closed assertion—remain recovery-uncertain and cannot be adopted.

`BEGIN IMMEDIATE` serializes aggregate admission across connections and processes.
Each authority transaction verifies active DELETE journal mode and
`synchronous=EXTRA` (numeric value 3) after obtaining the write transaction.
Mismatched configured limits reject rather than let another connection raise the
cap. Retained row count and bytes remain charged until confirmed removal; listing
uses at most 64 deeply frozen records plus a bounded continuation check. Listing
does not mutate recovery state. Removed history is not presented as executable
authority or a replacement for the permanent dispatch journal.

## Review correction and executed checks

The parent identified a concrete ignored-write gap during source review: an
SQLite trigger using `RAISE(IGNORE)` could make an UPDATE return without advancing
the durable phase. No failing pre-fix command was run for that review finding.
The correction now requires exact affected count and complete expected post-row
state for single transitions and reservations. Bulk browser closure verifies all
owned rows and their unchanged metadata as well as the closed flag and row count
before committing. Ignored or altered writes roll the transaction back.

Executed on Windows with Node v24.18.0:

```text
node --test --test-concurrency=1 apps/computer/test/browser-upload-ledger.test.mjs apps/computer/test/journal.test.mjs
Initial storage/journal checkpoint: PASS 13/13, 1.331 s
Ignored-update and postcondition regressions: PASS 15/15, 1.572 s
Reservation postcondition regressions: PASS 16/16, 1.498 s
Actual connection DELETE/EXTRA check: PASS 17/17, 1.642 s
```

That 17-case suite contains twelve ledger cases and five journal cases.
Actual owned Node children are killed with asserted SIGKILL at eleven phases:
reservation; directory creation before identity; recorded directory; file
creation before identity; partial copy; ready; selected-possible; caller-reported
browser closure; deletion intent; deletion before final receipt; and removed.
Reopened SQLite state retains uncertain phases, exact resource identity and budget;
foreign owners cannot clear them. An unrelated sentinel survives. Two real child
processes contend for one reservation and only one succeeds.

Fault tests install actual SQLite triggers that ignore seven transition types,
ignore one row of a bulk close, alter a digest after UPDATE, ignore INSERT, or
change a reservation's charged size after INSERT. Each reports failure and
preserves/rolls back the durable state; simulated assignment is never reached
after a failed `inUse`. Other tests cover future/missing version metadata,
constraint corruption, active-record limits, bounded read-only pagination,
detached inputs, scope rejection, manual-surviving ownership and unchanged
dispatch no-replay behavior. A fixture audit trigger observes the actual ledger
connection's DELETE/EXTRA settings; a later external change to WAL prevents a new
authority write. No production fault-injection switch was added.

The parent subsequently strengthened DispatchJournal itself and ran the latest
combined ledger/journal/process-crash gate: PASS 21/21, 1.920 s, recorded in
`build/browser-journal-ledger-final-20261120.log`. That parent-run gate includes
additional journal checks and is separate from the reviewer-run 17-case result.

The subsequent reviewer command `node --test
apps/computer/test/browser-upload-ledger.test.mjs
apps/computer/test/journal.test.mjs` passed 19/19 in 1.405 s. It adds a real
ledger-first legacy-database opening: the dispatch migration preserves both
permanent tombstones and the independently versioned resource record. This
19-case command includes thirteen ledger and six journal cases; it does not
include the parent's separate journal subprocess-crash file.

Independent sampled review by the scheduling agent found no additional concrete
ledger transition/ownership defect. It preserved three integration obligations:
real browser closure and all pending preparation writers must settle before the
closure assertion; the host must supply an already-enforced private state root;
and snapshot filename validation must precede writes (the ledger's bounded
metadata validation is not Windows ADS/device-name file-opening policy).

The source-size ratchet passed. Full companion/desktop/Go/renderer gates and Linux
or macOS native identity qualification have not been run for this foundation.
Storage subprocesses contain no Chromium process, so the persisted closure phase
is explicitly not evidence that real browser descendants are dead. No personal
browser, selected file, installed state or model weights were accessed.

## Primary implementation references

Read 2026-11-20: SQLite's [STRICT table documentation](https://www.sqlite.org/stricttables.html)
supports type/constraint checks, including integrity checks; its
[transaction documentation](https://www.sqlite.org/lang_transaction.html) explains
single-writer transaction behavior. These properties are exercised by the actual
process contention and trigger-failure tests above; neither STRICT tables nor a
successful SQL call alone proves a state transition or native resource closure.
SQLite's [synchronous pragma documentation](https://www.sqlite.org/pragma.html#pragma_synchronous)
specifies additional directory synchronization for DELETE/EXTRA. Process SIGKILL
does not test OS power loss, filesystem write ordering or storage hardware; those
qualification gates remain unrun.
