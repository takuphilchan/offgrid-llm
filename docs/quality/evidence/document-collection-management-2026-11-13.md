# Collection management and library integration

This combined increment continues `document-ingestion-reliability` tasks 10.1 and
10.2 and coordinates with `workspace-outcome-experience` D2. Those broad tasks
remain open; this report does not claim the seven-phase upgrade is complete.

## Implemented behavior

- Knowledge exposes Collections independently of embedding setup, gated by the
  service's collection capability and existing management permission hint. The
  authenticated service remains the authority for every operation.
- Private creation, collection names and owner-only reader/editor changes are
  available in one dialog. Proposed members are shown before explicit audience
  acknowledgement. Updates pin the reviewed revision. Unknown outcomes and
  conflicts are not replayed. Reloading discards the review.
- The existing user directory supplies account names and immutable identifiers.
  Unlisted existing members remain visible and are preserved on rename. Directory
  failure disables role edits instead of silently dropping memberships. Member
  search renders 40 entries at a time and supports loading more.
- Document moves display the source audience and reuse the reviewed destination
  selector. They pin the document binding and both audience revisions. Private
  sources require their owner; administrative privileges do not bypass this rule.
  A different destination must return an incremented binding before success.
- Document projections include collection identity and binding revision from
  SQLite. These are informational fields, not assignment authority. Collection
  filtering never guesses the audience of an older record without these fields.
  Metadata search and collection filters compose; library pages render 30 cards.
  Filtering does not silently change the scope of a Knowledge question.
- Workspace refresh reloads collection policy listings as well as documents.
  Account/workspace changes discard mutation dialogs and audience consent.
- Eleven collection messages are authored for all 29 languages. The concurrent
  conversation-input notice also received its 20 missing preview-language
  translations, without changing its behavior. All 1,406 current catalog messages
  are authored in every supported language. Native review remains pending.

## Verification

- `build/collection-management-go.log`: full RAG, server, serviceclient, CLI and
  API Go suites pass. The store regression checks moved list/read bindings and
  proves caller-supplied projection fields cannot assign private authority.
- `build/collection-management-http-projection.log`: authenticated HTTP test
  passes for authorized listing, outsider exclusion and removal after revocation.
- `build/collection-management-ui.log`: 182 renderer/contract tests pass; two
  opt-in real-service tests are skipped. Includes all 29 languages for management,
  audience review, moves and filtering, plus import/publication/recovery coverage.
- `build/collection-management-ui-final.log`: 43 focused cases pass after adding
  the unchanged-binding receipt rejection and workspace refresh integration.
  Includes stale policy conflicts, unknown outcomes without replay, non-owner
  restrictions, private creation, unavailable directories, unseen member retention,
  pagination, narrow layouts, alternating themes and 200% root text size.
- The initial smoke run exposed duplicate React keys between the manager and
  importer. Unique component keys fixed the unwanted remount. A subsequent test
  selected the directory-warning status instead of the save status; its assertion
  now selects the intended announcement. Earlier logs remain available.
- Narrow English/light and Arabic/dark move screenshots were inspected from
  `build/collection-management-results/`. Controls use the scrollable dialog;
  strings wrap without horizontal overflow. Fixture account/document names are
  user data, not untranslated interface strings.
- `collection-management-api-final.log`, `collection-management-build-final.log`,
  `collection-management-translations-check.log`, `collection-management-layout.log`,
  `collection-management-source-size.log`, `collection-management-literals.log`
  and `collection-management-spec.log` record passing contract, TypeScript/build,
  catalog, layout, source-size, literal and OpenSpec checks. The existing renderer
  bundle-size warning remains. `git diff --check` passes.
- Translator packs: `build/collection-management-translator-review-20261113/`.
  Completeness is not linguistic approval or representative usability evidence.

## Remaining qualification and scope

UI tests use isolated API fixtures, not model answers. This increment does not
claim Python execution, Go race coverage, clean-install packaging, OCR/table/model
quality, hardware performance or a departmental pilot. Existing local embedding
evidence is recorded separately in the collection API report. Directory access
uses the existing administrative endpoint; this does not introduce a tenant-wide
directory or new identity provider. Pagination bounds mounted cards, not the
service's document-list payload, and is not a large-library performance claim.

Departmental metadata, complete derived retention/audit/export coverage, richer
reader/version workflows, advanced extraction and the approved corpus/hardware
gates remain open in the controlling plan. No installation, publication, model
download, live Computer Use or installed-user state changes were performed.
