# Upload snapshot recovery investigation — 2026-11-20

This is an implementation proposal for the remaining ownership/recovery portion
of [browser-file-transfers](../../../openspec/changes/browser-file-transfers/design.md).
The separate [ledger foundation](browser-upload-ledger-2026-11-20.md) now implements
the bounded storage/state portion below. Production snapshot/session integration
and actual recovery actions are still unimplemented by that foundation.
Keep M2.07 and its transfer tasks incomplete until the implementation and actual
crash/recovery checks exist.

## Observed current ownership

`BrowserUploadStore` currently owns one snapshot under a random OS-temp root.
It is shared by the task's tab drivers and retains the snapshot while Chromium's
selected File may read it later. Normal close aborts preparation, waits for
browser closure, then cleans exact identity-checked snapshot objects. Cleanup
failure retains in-memory references; retry does not create a new grant.

There is no durable registry of those temp roots. A companion crash can therefore
leave bytes without a persistent cleanup record. This is a source-traced gap;
this investigation did not kill a production or personal browser to demonstrate
it. Existing tests establish orderly cleanup and delayed reads, not crash recovery.

`CompanionSession` already receives a trusted host-private state directory and
uses `dispatch.sqlite` for durable no-replay state. The journal uses full SQLite
synchronization and permanent dispatch tombstones. Its reply cleanup deliberately
erases live page results. `retainManual()` closes the session journal while the
browser resource stays alive, so a snapshot must not borrow that closing handle
without an explicit lifetime owner.

## Small coherent integration

Extend the private journal schema with a resource table and a shared schema
initializer. Keep resource rows separate from dispatch reply payloads and from
`forgetResults` / `discardPreviousSessionResults`. A resource-ledger reference or
connection is owned by `BrowserUploadStore` and survives manual takeover together
with the actual browser. It is not an executor and cannot restore session/tool
authority. Existing journal migrations and tombstones must remain compatible.

Use a fixed private `uploads` directory below the existing companion directory,
not an agent workspace or OS-temp scan. Construct paths only from validated opaque
resource IDs and validated local filenames; never accept an absolute path from a
ledger row, renderer, model or remote service as cleanup authority. Validate the
private root, native identities and all row fields before filesystem operations.
The old selected source path is unnecessary in the durable registry.

Each row needs bounded metadata: version, opaque resource ID, owner instance,
binding digest, grant ID, expected size/SHA-256, validated basename, allocation
identities, lifecycle state, and a fixed cleanup error code. Native filesystem
identities must use exact integer serialization, including an incarnation/birth
identity where supported; ordinary JavaScript number conversion must not round
64-bit file IDs. Creation timestamps are evidence, not authorization. Unsupported
identity guarantees stay explicitly unqualified.

Reserve the resource ID and retained-byte charge in a SQLite transaction before
allocation. Persist the exclusively created directory's identity before creating
its file, and persist the opened file identity before copying any selected bytes.
After bounded copy, complete digest verification and file synchronization, persist
`ready`. Before assignment, bind the resource to the already journaled action.
Use the existing session/action identity; do not create a second dispatch counter
or infer task authority from an arbitrary resource ID.

The initial generic developer pairing may not yet expose an owning run. Keep its
local cleanup binding honest and private; negotiated task artifact presentation
must obtain the canonical service-validated task association before exposing an
artifact. A locally persisted binding is never a substitute for actor/task
authorization at inspection or export.

Manual handoff retains the old File's storage but invalidates the upload grant.
Fresh pairing must not make the old file assignable again. Renewed session reply
cleanup must not discard the retained resource record or byte reservation.

After confirmed browser closure, durably mark the resource eligible for cleanup,
then delete only identity-matching recorded objects. Persist `removed` only after
confirmed removal; retain an opaque tombstone if needed for recovery. Partial
cleanup and publication errors retain identity, state and budget rather than
releasing capacity optimistically. Native deletion/rename semantics must be
reviewed for their check/use windows; do not generalize identity checks into an
atomic same-user adversary guarantee.

## Restart is reconciliation, not automatic deletion

Read bounded pages of ledger rows only. Never glob-delete `offgrid-browser-upload-*`
or adopt a directory because its prefix, age or path resembles an upload. Existing
unregistered temp roots have no durable ownership proof and require an explicit
separate recovery procedure; preserve them by default.

A stale PID, wall-clock expiry or disconnected transport does not prove an old
Chromium descendant has terminated. Rows interrupted while in use remain
`recovery_uncertain`, retain their byte reservation and cannot renew a grant or
replay an action. The current browser owner does not provide a durable native
process identity sufficient to automatically establish this after a process crash.
Closing that gate requires a trusted native process/boot identity or a separately
specified explicit scoped cleanup path. Do not silently treat absent JavaScript
ownership as proof that a selected File can no longer be read.

A crash after exclusive creation but before identity publication leaves an
unproven allocation. Preserve the row and report uncertainty; do not adopt the
current object at that path. A crash after cleanup began can reconcile only the
objects whose exact identities and eligibility were durably recorded. Permanent
action tombstones remain regardless of artifact cleanup.

Bound retained rows and aggregate charged bytes as well as individual file size.
Uncertain ownership consumes the reservation; a new process must not reset the
budget or accumulate unbounded 512 MiB leftovers through repeated restarts.

## Required focused evidence

Use disposable subprocesses/state and independently hashed synthetic files. Kill
only fixture-owned processes at controlled boundaries: reservation; directory
creation before identity; file creation before identity; partial copy; ready;
assignment before acknowledgement; manual handoff; browser-close checkpoint;
partial unlink; and deletion before final ledger receipt. Reopen the actual
SQLite store after each interruption and assert exact rows, retained bytes,
capacity reservation and no action replay.

Include replaced/recreated directory and file objects, symlink/reparse/special
files, missing paths, corrupted rows, identity mismatch, lost final receipts,
multiple host instances, failed SQLite commit, failed close and cancellation.
An unrelated sentinel directory must survive every case. Retained native File
reads must continue across manual control; a renewed session must not reuse the
discarded upload grant. State lifetime must survive session-journal close.

Run actual supported-platform identity and filesystem checks separately from
fixture database tests. In particular, a PID-only fixture must not be presented
as proof that unknown browser descendants are dead. Keep native process recovery,
installed-user migration and platform qualification open until those procedures
execute successfully.
