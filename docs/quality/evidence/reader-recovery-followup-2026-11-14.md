# Reader and recovery screenshot follow-up

Implemented against the user's two installed-app screenshots:

- Moved the canonical attachment handoff above the composer, preserving explicit
  decisions and drafts. Empty-question voice mode no longer supersedes a pending
  file handoff. Ready-file cards use smaller icons, metadata and spacing.
- Removed the full-height inset focus stripe from the conversation scroll area;
  keyboard focus instead marks its first content item.
- Removed the reader's nested rounded-card frame, shortened its header and
  thumbnails, and grouped affected-page links inside a coverage disclosure.
  The incomplete-coverage warning remains visible when collapsed.
- Added `model_load_failed` for otherwise unclassified model-start errors, with
  model-selection recovery rather than history refresh. Specific capacity,
  queue-full, cancellation and deadline causes remain distinct. Generic failure
  wording no longer claims available memory is the established cause.

## Verification

Nineteen focused Playwright scenarios passed across the final runs: three
canonical file handoffs, three 37-page layouts, model-failure recovery, page-key
navigation, nine-language keyboard controls, narrow view switching and 200% zoom.
The 12-case final rerun is recorded in
`build/reader-recovery-renderer-test.log`. Earlier failures exposed incorrect
thumbnail fixture metadata and stale keyboard expectations: Escape first closes
focused search, then closes the reader. Those fixtures/expectations were corrected
and the affected cases rerun successfully.

Inspected generated desktop dark/light and narrow Arabic screenshots in
`build/ui-craft/reader-recovery-followup/` plus the updated handoff screenshots in
`build/ui-craft/canonical-document-handoff/`. Document imagery is synthetic UI
test material, not a rendering or quality assessment of the user's paper.

Targeted Go checks passed for model-load error classification and existing
stream-start/cancellation/validation behavior, recorded in
`build/reader-recovery-go-test.log`. Translation integrity passes across 29
catalogs; this pass reuses existing translated messages, without claiming native
review. Production renderer output is in `build/reader-recovery-build.log`.

## Remaining functional limits and environment

Canonical conversations still do not execute document inputs; the handoff is an
explicit route to a document-compatible chat. Reader navigation does not yet
bind questions to the visible page. No page-scoped evidence contract, automatic
figure interpretation, or model-loading qualification is claimed here.

The actual cause of the screenshot's earlier model-load failure was not established.
A later machine snapshot showed about 2.1 GiB of free physical memory on a 16 GiB
machine. During backend verification C: reached zero free space. Moved the exact
previously verified Electron 43.7.9 cache archive to
`build/packaging-cache/electron-v43.7.9-win32-x64.zip`, verified its SHA-256 again,
and updated the local packaging resume script. This freed about 134 MiB at that
moment; system drive capacity remains low. No installed application, model,
conversation, desktop profile or source record was modified for this cleanup.

No live computer-use automation or actual-model inference was performed for this
follow-up. Installed-user data was not used as a test fixture.

## Requested installation

Installed local preview `0.4.31-preview.20261114134918462.gda2a178e` after verifying
OffGrid was closed. Packaging used the verified local Electron archive, passed
the packaging hooks and completed local-preview artifact inspection. The preview
is unsigned and is not a release-qualified distribution.

Installer exit code was 0. All 1,191 installed payload files matched the candidate
hashes; workspace/profile hashes and the full workspace inventory were unchanged.
The application was left closed. UI identity:
`a0a23e9eedee0343ee3c800d0a1da8f25aa4361e62ca109243d344b6c4243007`.
Rollback backup and receipt:
`build/reader-recovery-install-backup-20261114/installed.json`.
During packaging C: had approximately 6 GiB free, so the earlier disk-space
snapshot should not be treated as its current state.
