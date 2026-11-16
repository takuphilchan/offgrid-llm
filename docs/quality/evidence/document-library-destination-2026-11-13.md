# Reviewed library batch destinations

Continues `document-ingestion-reliability` 10.2 and
`workspace-outcome-experience` D2. Full collection management, metadata,
retention/export and departmental qualification remain unfinished.

## Implemented behavior

Knowledge's ordinary file picker now opens a review dialog before transferring
files. The selected filenames remain local until the user chooses a writable
collection and acknowledges its owner/editors/readers. Private creation and
audience display reuse Chat's `CollectionDestination`, with batch-specific copy.
Closing the dialog sends no files; the next selection starts without consent.
Account/workspace changes remount the import component and abort its owned work.

All batch files use the same reviewed collection revision through the distinct
collection-import endpoint. The current UI has no legacy shared-upload fallback.
Only a fresh accepted import starts processing. Completion must match the accepted
import, source digest, collection and revision before the file is marked added.
An audience conflict or uncertain response stops the rest of the batch. Input
rejections retain individual results and permit other valid files to proceed.
Cancelling or navigating away aborts the current request and leaves undispatched
files unsent; already accepted originals remain in durable recovery.

Recovery translates collection conflicts using shared copy. CLI private-file
uploads now reject absent, mistyped or wrong audience revisions in receipts;
Python binary upload parity and equivalent tests are authored but unrun.
Legacy CLI/API multipart uploads keep their documented workspace destination.

## Verification evidence

- `build/library-destination-ui-final.log`: 145 checks passed across import,
  progress, recovery, collection, Chat-publication and client suites, with two
  opt-in real-service cases skipped. One focus test failed because it populated
  the hidden input directly rather than opening the real file picker; it had no
  button opener to restore. It now uses the Add document action and file-chooser
  event. This initial log is retained, not described as a fully green run.
- `build/library-destination-focus-journey.log`: the corrected focus/consent-reset
  test and the import → source inspection → disabled indexing → explicit deletion
  journey both pass.
- `build/library-destination-collections-verified.log`: the final collection
  suite verifies all 29 interface languages, alternating light/dark themes at
  390px width, plus dismissal, stale audience, old-service refusal and invalid
  completion receipts. Dialog overflow is checked before confirmation; filenames
  and member identities remain user data, not translated UI copy.
- The earlier `build/library-destination-ui.log` had 70 passing cases and one
  interrupted by Vite hot reload while copy was changing. The stable-source run
  above covers those cases without that reload failure.
- `build/library-destination-clients.log`: scoped authenticated server/CLI
  transport and receipt regressions pass. `build/library-destination-go.log`:
  complete CLI and serviceclient suites pass.
- `build/library-destination-build.log`: API generation parity, TypeScript and
  production renderer build pass; existing bundle-size warning remains. Source
  layout, source-size ratchet, UI literals, translations and diff checks are
  recorded in the matching `library-destination-*` logs.
- `build/library-destination-i18n.log`: all 29 catalogs contain 1,394/1,394
  messages with no missing catalog entries. Translator packs are at
  `build/library-destination-translator-review-20261113/`; native review remains
  pending and catalog coverage does not prove linguistic quality.

These UI tests use isolated API fixtures, not generated model answers. Prior
real local embedding/private collection evidence remains in the collection API
report; this increment does not qualify a new model, corpus or hardware profile.
No installed-user workspace, application installation or publication was changed.
