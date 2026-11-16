# Private document correction and publication ? 2026-11-14

This records implementation evidence, not public-sector qualification.

## Delivered behavior

Conversation owners can correct ready extracted text. Immutable revision artifacts
are published before the current record changes atomically. Originals stay unchanged;
old citations and saved messages keep their selected revision. History, previews,
source reads and corrections recheck current access. Orphaned interrupted artifacts
are not readable as published revisions. Separate bounded storage includes history.

The shared reader supports current editing, historical inspection, navigation-safe
drafts and uncertain-save recovery without replay. All 29 languages use the existing
review copy. Knowledge publication requires explicit audience review and a selected
revision for corrected files. The original, selected graph and ancestors are accepted
together, indexed without reparsing away corrections, and published atomically.
Staged history is then released, while its identity remains for idempotency.
The import receipt exposes the accepted extraction revision. Older services cannot
silently ignore revision-preserving publication from current clients.

SQLite document policy schema 8 adds staged revision history. The existing migration
snapshot path runs before upgrade. No legacy owner or collection reassignment occurs.
CLI, Go and Python clients expose history/correction and selected-revision publication.

## Executed checks

- Full sessions, retrieval, server and API suites passed before final receipt additions:
  sessions 40.462 s; rag 83.601 s; server 494.030 s; API 0.972 s.
- Final private correction HTTP/storage tests passed, including Chat admission of
  original and corrected revisions, pinned previews, stale writes and publication.
- Final import/receipt and API checks passed after the receipt addition.
- Real local bge-m3 embedding and SQLite indexing passed in 15.10 s, with exact old
  and new source passage readback. The model and llama-server were read-only installed
  assets; all workspace/cache data was temporary. No download occurred.
- Go serviceclient suite passed. Focused document/session CLI tests passed.
- Full Python SDK: 144 passed. Renderer typecheck and production build passed.
- Headless private-reader suite: 33 passed across all 29 languages, legacy readers
  and uncertain saves. Final English/Arabic visual cases: 2 passed. Arabic 390 px
  dark-theme screenshot was inspected; no horizontal overflow was observed.
- Publication contracts and audience flow after revision gate: 44 passed, 1 skipped
  (optional real service). All 29 localized publication cases passed.
- Generated API contract passed. Translation catalog additions remain previews,
  not native-speaker approval.

## Failures and limits retained

The full CLI suite still fails unrelated conversation scripted-intent cases with EOF
at conversation_discussion_test.go:63. That failure predates this increment and is
not hidden by the focused document checks. Windows race instrumentation, clean OS
installation, full model-answer accuracy, native translation review and departmental
pilot qualification were not completed here. Transport fixtures test context admission,
not whether a language model's resulting answer is factually supported.

Logs: build/private-correction-go-full.log, private-correction-chat-final.log,
private-correction-receipts.log, private-correction-publication-ui.log and the
private-correction UI result directories. No user installation or commit was made.
