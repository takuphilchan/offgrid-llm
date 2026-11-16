# Reviewed collection API increment

Continues `document-ingestion-reliability` 10.2; coordinates the pending
collection presentation with `workspace-outcome-experience` D2. The full task
remains open.

## Behavior

- `/v1/collections` exposes scoped listing and policy reads, private-by-default
  creation and owner-only membership replacement. Both member lists must be
  explicit; missing lists cannot silently revoke access. Authentication supplies
  the owner, and service read/manage permissions remain additional gates.
- `/v1/documents/collection` returns the document binding and current audience.
  Movement checks the binding and both policy revisions. Private content needs
  its source collection owner; editing content is not authority to change sharing.
- `/v1/collections/imports` is a distinct endpoint for explicit destinations.
  Older services cannot silently ignore query fields and publish these requests
  into their legacy shared workspace. Clients never fall back to that route.
- Imports retain their reviewed audience revision. Acceptance and publication
  check it; membership changes prevent stale publication. Current authorized
  actors can inspect, cancel and remove pending imports after an audience change.
- Policy schema 5 adds the import review revision and collection audit ledger.
  Policy creation/updates, moves and import acceptance commit audit entries in
  the same transaction. Events contain actor/opaque identities/action/revision,
  not names, source bodies or membership lists. Owner-only audit pages contain
  at most 100 records. Audit records have their own retention domain.
- Existing databases receive a validated pre-upgrade SQLite snapshot. Legacy
  workspace ownership/audience and private bindings/source hashes are preserved.
  Older applications require a compatible data snapshot, not a schema downgrade.
- Go and TypeScript clients validate mutation receipts without replay. CLI adds
  collection list/inspect/create/update/move and reviewed file import. Python
  parity and tests are authored, but Python execution is unavailable here.

## Evidence

- `build/collections-foundation.log`: scoped policy, atomic audit, stale audience
  rejection, private import publication rollback and cancellation checks pass.
- `build/collections-client-http.log`: authenticated HTTP and real CLI transport
  test passes. Admin outsiders cannot inspect private policies/documents/pending
  imports; editor and stale sharing updates are rejected; revocation takes effect
  on source and policy readback; reviewed uploads retain the intended destination.
- `build/collections-final-go.log`: rag, sessions, server, serviceclient, CLI and
  API suites pass. The server suite includes existing search, history, source,
  export and ongoing-generation access guards. These are regression fixtures,
  not departmental pilot qualification.
- `build/collections-ts-final.log`: four TypeScript contract tests pass for
  stale/wrong audiences, malformed policies, audit cursors and no fallback on older services.
  These exercise client logic, not a completed collection UI.
- `build/collections-real-http.log`: eight actual local BGE-M3 HTTP cases pass,
  including private acceptance/indexing, owner retrieval/original delivery and
  outsider exclusion. This is retrieval evidence, not generated-answer qualification.
- `build/collections-web-final.log`: generated contracts and production renderer build
  pass; existing bundle-size warning remains.
- `build/collections-i18n-final.log`: all 29 catalogs contain 1,381 messages with
  no missing catalog entries at this increment. The localized receipt error is
  included in `build/document-collections-translator-review-20261113/`.

The initial foundation run exposed a migration fixture retaining the new audit
table while pretending to be a pre-policy database. The fixture now accurately
removes the added schema. A second legacy-import fixture required the same fix;
the failed logs are retained. A separate schema-4-to-5 test verifies the actual
compatible snapshot, source digest and unchanged private audience. The initial
HTTP fixture also required adapting permission middleware to the router's
HandlerFunc signature; its corrected tests pass.

## Remaining requirements

The subsequent [collection-review increment](document-collection-review-2026-11-13.md)
adds Chat audience-review controls and closes unreviewed attachment publication,
with localized controls and all-29-language fixtures. Full collection management
and ordinary library upload destinations remain unfinished. Department/classification/effective-date and
supersession metadata, the complete document/export/hold audit suite, broader
derived retention and the controlled pilot remain open. Native translation,
race/hardware/offline-install and model-quality qualification are not certified.
No installed-user data was changed, and nothing was installed or published.
