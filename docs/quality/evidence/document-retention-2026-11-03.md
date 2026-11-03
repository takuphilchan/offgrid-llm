# Document retention controls and lifecycle

This source change connects the existing versioned retention store to service
composition and authenticated clients. Existing documents keep expiry zero.
No installed user documents were deleted or assigned a new policy during tests.

- GET/PUT policies are actor scoped, explicitly revision checked, and fail closed
  on malformed/unknown records. Private collection changes require the owner;
  shared workspace changes require the existing management permission.
- Legal-hold deletion now returns a typed conflict rather than “not found.”
  Removing an import cannot bypass its published source's hold. Source deletion
  invalidates related import attempts transactionally to prevent resurrection.
- One service-owned worker runs bounded startup/minute sweeps and is joined on
  shutdown. Failure leaves the policy stored for the next sweep. Status omits
  private document details and other collections' deletion counts.
- Web/desktop document options, CLI and Python expose the same policy. Missing
  hold/expiry fields and mismatched update receipts are rejected. Failed writes
  are not blindly retried. The UI requires a fresh read after a failed save.
- Backup and saved-answer retention remain separate from source deletion. This
  is not secure erasure or a compliance certification.

## Executed checks

- `CGO_ENABLED=0 go test ./internal/rag ./internal/server -count=1`: RAG passed;
  server initially failed its frozen route-count fixture after the new endpoint.
  The added route and its read/manage guards were reviewed, the fixture was
  updated from 148 to 149, then `go test ./internal/server -count=1` passed.
- `CGO_ENABLED=0 go test ./internal/cli ./cmd/offgrid -count=1`: passed.
- Targeted retention HTTP/store tests passed, including ownership, missing fields,
  stale revisions, held deletion, expiry, shutdown, original-source removal and
  stale import protection. They use disposable stores and authenticated handlers.
- Python `test_knowledge_retention.py`: 12 passed against loopback HTTP fixtures.
  Initial collection failed because a test fixture import was not package-relative;
  correcting the import produced the passing run.
- Web `knowledge-retention.spec.ts`: nine locales passed in Edge at 390px width.
  Explicit saves, conflict recovery and no automatic replay were checked. The
  initial test assumed one React effect GET; it was corrected to measure the
  initial StrictMode reads and assert exactly one explicit reload afterward.
- Web type/build and generated API checks passed. Existing bundle-size warning
  remains. This does not establish real document-answer quality or enterprise
  qualification. Broader OpenSpec collection/resource/acceptance tasks remain open.

## Offline backup and concurrent deletion checks

`go test ./internal/rag -run='TestRetentionAcrossOfflineBackupAndRestore|TestConcurrentRetentionAndManualDeletionFencePublication' -count=1`
passed against disposable SQLite stores and real workspace ZIP archives.

- Live expiry leaves an earlier archive byte-for-byte unchanged. Restore refuses
  an existing workspace, preserves the snapshot's hold, and reapplies the saved
  expiry when maintenance runs in the separate restored workspace.
- Concurrent manual deletion and expiry remove source bytes, text chunks,
  vectors and policy bindings. A pending duplicate import cannot republish the
  deleted source through its stale publication lease.

These checks do not establish retention of external archives, saved answers or
rebuild staging, and do not justify closing the whole enterprise retention gate.

The follow-up deletion audit exercised text, original-file and preview requests
after expiry. It initially failed because retention-policy errors lacked
`Cache-Control: no-store`. The handler now sets it before every response,
including missing/revoked sources and stale-policy errors. The regression passes.
Additional staged-index tests verify that publication cannot resurrect a deleted
source or replace current private access and legal holds with staging defaults.
They do not erase historical staging bytes; automatic staging retention remains
unfinished.

Windows race tests, manual language review and other-platform qualification were
not run. The preview packaging includes these changes; installation is recorded
separately from this source/test evidence.

## Rebuild-cache reconciliation follow-up

The service maintenance worker now reconciles offline-rebuild staging with the
live library. Deleted sources and obsolete text versions are removed using the
same transactional deletion path, including originals, chunks, vectors and
related import payloads. Current source versions and legal holds are retained.
Only OffGrid's pipeline-digest cache directories are examined; redirected paths,
unknown policies and unknown schemas fail closed. No filesystem tree is deleted
and no independent backup is edited. Cleanup is cooperative and bounded to eight
cache databases and 100 sources per database per sweep; restart safely rescans.

Executed on disposable Windows workspaces:

- `go test ./internal/rag -run 'TestRetentionRebuildStaging' -count=1 -v`
  passed: current/held/replaced/deleted sources, vector removal, 101-source
  batching, nine-cache rotation, restart, cancellation, unknown policy/schema,
  and rollback/retry after injected write failure. Initial fixture construction
  failed because it assigned names as digest-bound source IDs; using the actual
  original-source identity corrected the fixture without relaxing validation.
- Three symlink cases were **skipped**: this Windows account lacks symlink
  creation privilege. They remain in the suite for capable CI hosts.
- `go test ./internal/rag ./internal/server ./internal/sessions ./internal/storage -count=1`
  passed in 10.468s / 57.100s / 4.979s / 3.666s respectively.

This follow-up closes the automatic rebuild-cache cleanup gap recorded above,
not the entire enterprise or document-answer qualification gate. It provides
logical deletion, not forensic erasure. No installed user source, policy,
backup or model was changed during these checks.
