# v0.4.27 source verification

This records local verification before exact-source CI and release packaging.
The installed preview still reports 0.4.26; its build identity is recorded in
[startup evidence](desktop-startup-identity-2026-11-06.md). Version labels alone
do not distinguish the local previews.

## Checks

- `go test -p 1 ./...`: passed; `build/release-go-final-20261106.log`.
- `npm.cmd --prefix apps/web run check`: passed;
  `build/release-web-type-final-20261106.log`.
- The full built-renderer/isolated-service suite completed with 1,062 passed,
  31 skipped and nine failures; `build/release-web-final-20261106.log`.
  Eight failures were Help selectors expecting the former ungrouped layout;
  the tests now open the relevant topic and verify its content, typography,
  navigation and draft preservation. All ten Help tests passed in
  `build/release-help-final-20261106.log`.
- The ninth failure assumed an imported speech package always had an unavailable
  runtime. This machine now has a packaged runtime. The test now checks the
  service's hydrated readiness projection while retaining exact import bytes,
  verification/removal and the requirement that a synthetic package is not
  smoke-tested. Both speech-package tests passed in
  `build/release-speech-package-final-20261106.log`.
- These are a full run plus focused corrections, not a claim that one unchanged
  full invocation passed. No product source changed for those selector fixes.
- `npm.cmd --prefix apps/desktop test`: 134 passed, one platform skip;
  `build/startup-desktop-final-20261106.log`.
- Packaged Windows startup checks passed with the minimal recovery UI, pinned
  service version, installation-directory launch, mismatch rejection and owned
  shutdown; `build/startup-packaged-minimal-20261106.log`. First window appeared
  in 903 ms and the recovery fixture in 1,153 ms on this machine.
- Packaged terminal checks passed for CLI/Agent entry, tab/cursor preservation,
  Unicode, zoom and descendant cleanup;
  `build/packaged-terminal-followup-20261106.log`.
- Real-service PDF/Office preparation and PDF preview: two passed;
  `build/real-document-followup-20261106.log`.
- OpenSpec 1.14.0 `validate --all --strict --no-interactive`: 29 passed, zero
  failed; `build/release-openspec-20261106.log`. This checks specification
  structure, not application behavior.

## Installation and limits

The minimal-startup preview installed successfully. Five payloads matched the
candidate, 393 workspace files retained their hashes, and models and native
inference runtimes were unchanged. A rollback backup remains outside the repo.
The normal application was left closed; isolated packaged tests are not a
walkthrough of the user's workspace.

Manual usability, screen-reader and translation review remain unverified, as do
broad document-answer accuracy, unread figure interpretation, enterprise
retention qualification and unavailable native hardware combinations. Skipped
checks and previous failures are retained rather than counted as successful
qualification. Publication still requires CI on the exact release commit and
successful release-artifact verification.
