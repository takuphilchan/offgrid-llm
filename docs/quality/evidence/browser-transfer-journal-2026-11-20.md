# Transfer journal verification — 2026-11-20

Status: a verified source increment for M1/M2.07; complete file transfers and
power-loss/platform qualification remain open. The existing `DispatchJournal`
remains the sole companion action no-replay owner. The separate upload resource
table records cleanup obligations; it cannot authorize or repeat an action.

## Reproduced missing-intent acknowledgement

A real SQLite `BEFORE INSERT ... RAISE(IGNORE)` fault made `prepare` return
`execute:true` without inserting its action intent. The focused regression failed
with a missing expected exception: **one failed, 0.069 seconds**,
`build/browser-journal-ignored-write-baseline-20261120.log`. This is an injected
database-write fault, not a claim that an ordinary filesystem error installs a
trigger or that the fixture was a power failure.

`prepare` now checks both the affected-row count and the complete expected intent
before committing. `complete` checks the exact receipt and unchanged identity
inside one transaction; an ignored update or trigger-altered receipt cannot be
acknowledged as durable success. Failure preserves uncertainty and does not
authorize dispatch. Existing tombstones, schema version and result-retention
rules remain unchanged.

## Actual process termination and durable settings

Three new fixtures keep an owned child and its actual SQLite connection alive,
then terminate that exact process with SIGKILL after intent, after an independent
synthetic file effect, or after receipt persistence. Reopening returns uncertainty
for the first two and the exact saved reply for the third. The independent effect
file is absent before dispatch and contains exactly one effect afterward; no
action becomes executable on recovery. Only disposable directories/processes
participate.

The journal and upload-ledger connections enforce and read back DELETE journaling
with `synchronous=EXTRA`. SQLite's [official synchronous documentation](https://www.sqlite.org/pragma.html#pragma_synchronous)
explains that FULL in rollback mode can lose the latest committed transaction
after power failure on some filesystems; EXTRA adds the journal-directory sync.
These settings strengthen the software contract. Process termination tests do
not qualify physical power loss, storage firmware, filesystem/VFS behavior or an
installed Mac/Linux/Windows product.

The first actual crash plus existing journal run passed **8/8, 0.393 seconds**.
After the injected-write correction, the combined journal, actual crash and
latest upload-ledger suite passed **21/21, 1.920 seconds**:

```powershell
node --test --test-concurrency=1 apps/computer/test/journal.test.mjs apps/computer/test/journal-crash.test.mjs apps/computer/test/browser-upload-ledger.test.mjs
```

Logs: `build/browser-journal-crash-20261120.log` and
`build/browser-journal-ledger-final-20261120.log`. The latter includes actual
ledger child termination at its persisted phases and a two-process aggregate
reservation race. The final complete companion/desktop suites and negotiated
transfer-binding integration must run after their remaining code changes.

Implementation: `apps/computer/journal.mjs`, its existing journal tests,
`journal-crash.test.mjs` and `fixtures/dispatch-crash-child.mjs`.
An independent source review found no concrete ownership/regression blocker in
the sampled journal delta: both intent and receipt confirmation stay inside the
existing transaction/owner boundary. This is a source review, not additional
hardware or full-program qualification.

See [upload recovery design](browser-upload-recovery-design-2026-11-20.md) for
remaining private-root, snapshot/ledger wiring and browser-settlement obligations.
