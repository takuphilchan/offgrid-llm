# Canonical conversation document handoff

The installed UI exposed `canonical_conversation_required` inside the composer
when a document upload targeted a conversation already imported into the
canonical control store. The picker checked service-wide attachment support,
but did not check conversation ownership. The server correctly rejected a legacy
write; changing that guard would permit conflicting execution paths.

The shared renderer now intercepts a known canonical upload before any request.
It retains the selected files and shows the existing localized capability notice
with New chat and Cancel. An explicit New chat creates a document-compatible
conversation, transfers the unsent draft and uploads the selected files. It
resets conversation-specific configuration, preserves the original history and
does not submit the question. A stale-client ownership rejection offers the same
handoff. Cancel retains and focuses the draft. Send and Enter cannot omit a
pending file by submitting the question before the handoff is resolved.

Verification used isolated HTTP fixtures, without installed-user data or live
desktop automation:

- `npx playwright test tests/canonical-conversation.spec.ts tests/chat-files.spec.ts --workers=1`:
  44 passed, two skipped (opt-in screenshot scenario and disposable real-service
  parser scenario). Existing uploads, saved files, failed sends, reload recovery,
  canonical commands and nine-language file controls passed.
- Final `--grep 'document handoff'` run: three passed after adding the Send/Enter
  guard. Both known ownership and stale-client rejection preserve the file and
  question, leave the original history unchanged, and use the document generation
  path only after an explicit subsequent Send. Cancellation performs no writes.
- Desktop and 390px screenshots were inspected at
  `build/ui-craft/canonical-document-handoff/{desktop,narrow}.png`; the notice,
  filename and both actions fit without horizontal page overflow.
- Translation integrity checks passed across all 29 catalogs; existing translated
  strings are reused. This is not native-language approval.
- Source-size and OpenSpec validation passed. `npm --prefix apps/web run build`
  passed TypeScript and Vite production compilation; Vite reported a non-fatal
  bundle-size warning. Output is in `build/canonical-document-handoff-build.log`.

This is a recovery fix, not canonical attachment execution. Documents still need
a document-compatible conversation; image and additional Knowledge inputs are
also not enabled in canonical conversations. No server ownership guards were
removed and no inference-quality claim was tested.

## Requested installation

After the user confirmed OffGrid was closed, installed local preview
`0.4.31-preview.20261114125524262.gad6d6d3b`. The first packaging attempt timed out
fetching Electron; packaging resumed using the cached Electron 43.7.9 archive
after verifying its SHA-256 against the installed package's checksum manifest.
The packaging hooks and local-preview artifact inspection completed. This build
is not a signed, release-qualified distribution.

The installer exited 0. All 1,191 installed payload files matched the candidate
hashes. Workspace/profile files matched their pre-install hashes and the complete
workspace inventory was unchanged. The application was left closed; no live
desktop automation or post-install model inference was performed.

UI identity: `5986824dd7ee07c9244392a0f111809d7cdf4ed3f1f461a8315f0fecc5dc56c7`.
Verified rollback backup and install receipt:
`build/canonical-handoff-install-backup-20261114/installed.json`.
