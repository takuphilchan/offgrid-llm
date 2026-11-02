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

Windows race tests, manual language review and other-platform qualification were
not run. The preview packaging includes these changes; installation is recorded
separately from this source/test evidence.
