# OffGrid v0.4.32

This is an **unsigned preview**. Windows installers do not have a verified
publisher signature, and the macOS apps are not Developer ID signed or
notarized. Windows may show an unknown-publisher warning; macOS may block the
app. Checksums verify downloaded bytes, not publisher trust. The release build
will publish its artifact trust reports and checksums when available.

This release brings the current workspace, document and agent updates together
in one version. It is a preview, not a claim that all planned capabilities or
qualification gates are complete.

## Changes

- Refines the desktop work surface, startup presentation, quick window, chat
  scrolling, attachment handling, document reader, collection flows, settings,
  model selection, voice controls and agent management.
- Adds a shared structured-document and evidence workflow across Chat and
  Library, with clearer extraction coverage, review states, source navigation,
  comparison and synthesis flows. Extraction limits remain visible to users.
- Improves image and document attachment admission, metadata handling, source
  selection and preview behavior while preserving local processing boundaries.
- Extends agent task coordination, profile lifecycle, tool presentation and
  provider/runtime handling, with corresponding CLI, Python SDK and API updates.
- Updates product documentation and screenshots to show the current light and
  dark interfaces, Library, agents, terminal and voice experiences.

## Verification and limits

The tagged source must pass the repository's complete CI workflow before the
release workflow builds artifacts. Local implementation evidence is recorded
in `docs/quality/evidence/` and the relevant OpenSpec changes. Those checks do
not certify native-speaker translation review, physical 8 GB qualification,
the full document benchmark, handwriting or visual-interpretation accuracy, or
departmental pilot readiness. Publisher signing and notarization are also not
provided by this unsigned preview.

Use the platform installation guide and verify downloads against the published
SHA-256 manifest. Complete active work and quit OffGrid before upgrading.
