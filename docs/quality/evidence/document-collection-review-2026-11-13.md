# Reviewed Chat attachment publication

Continues `document-ingestion-reliability` 10.2 and coordinates the shared
destination component with `workspace-outcome-experience` D2. Both full tasks
remain open; this is one implementation increment.

## Behavior

- Chat's Add to Knowledge dialog now requires an explicit destination. It reads
  the current policy on selection and displays owner/editor/reader identities.
  The legacy workspace collection explicitly states its wider Knowledge audience.
  Read-only collections are omitted from the destination choices. Server access
  checks remain authoritative.
- A separate checkbox acknowledges that audience. Selecting or reloading a
  destination discards consent; account/workspace changes close the saved-source
  view and discard pending publication. New private collection creation does not
  acknowledge or publish the file. A lost creation response requires collection
  inspection before another creation attempt.
- Publication uses `/v1/collections/imports`, with the exact source digest and
  reviewed policy revision. Acceptance and completion receipts must match the
  source, destination, revision and import identity before showing success.
  Changed membership prevents processing; no request falls back to shared storage.
- Legacy JSON attachment publication without destination/revision returns HTTP
  426 (`collection_review_required`) before source access or copying. CLI now
  requires `COLLECTION_ID REVIEWED_REVISION`; Python requires corresponding keyword
  arguments. Ordinary legacy multipart library uploads remain compatible.
- Draft conversation text and the private original remain intact. After an
  ambiguous import outcome, the interface directs users to durable Knowledge
  recovery rather than replaying publication.

## Verification

- `build/collection-destination-http-2.log`: real authenticated HTTP and CLI
  checks pass for exact private copying, owner isolation, explicit workspace
  selection, destination-scoped idempotency, stale-audience rejection, cancellation
  and HTTP 426 for legacy unreviewed publication.
- `build/collection-destination-go-all.log`: rag, sessions, server, serviceclient,
  CLI and API suites pass. This includes existing citation/access/retrieval
  regressions; it is not a departmental pilot.
- `build/collection-destination-ui-verified.log`: 42 headless browser/client checks
  pass, including one publication fixture per supported interface language,
  destination/consent reset, unknown outcomes, older-service refusal, malformed
  receipts, narrow layouts and RTL keyboard operation at CSS 200% zoom. The real
  service case is skipped in this mocked run and executed separately below.
- `build/collection-destination-real-service.log`: the freshly built executable
  with temporary data and an ephemeral loopback port passes the actual HTTP test
  for private copy, source digest, repeat acceptance and unchanged original. No
  embedding/model inference is needed or claimed by that test.
- `build/collection-destination-final-checks.log`: generated API contract check,
  TypeScript and production renderer build pass. Existing bundle-size warning
  remains. Source-size/layout, translation and OpenSpec checks are recorded in
  the corresponding `collection-destination-*` logs.
- `build/collection-destination-i18n.log`: all 29 catalogs have 1,393/1,393
  messages, zero missing catalog entries. Review packs are exported to
  `build/collection-destination-translator-review-20261113/`. These are authored
  translations awaiting human review, not native-language approval.

The initial browser run exposed an ambiguous test locator and then a real
StrictMode remount bug that left the selector loading. The asynchronous operation
lock now belongs to the current request; the corrected runs pass. The initial
zoom fixture resized a still-open attachment popover, closing it before the
dialog was opened; zoom is now applied to the opened review dialog. Its retained
RTL screenshot was inspected for text wrapping and action visibility. CSS zoom
is not an exhaustive browser/OS accessibility qualification.

## Still open

Full collection management/member editing, ordinary library upload destinations,
departmental metadata, broader retention/export/audit and pilot requirements
remain unfinished. Python parity tests are authored but unrun because no usable
Python runtime is installed. Native linguistic review, full theme/screen-reader
qualification, race builds, hardware profiles, offline-install qualification
and the broader document-model benchmark remain separate open gates. No user
workspace was changed, installed, or published by these checks.
