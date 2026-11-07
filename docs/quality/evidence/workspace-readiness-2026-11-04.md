# First-use and terminal readiness follow-up — 2026-11-04

The first-use guide now offers a direct, explicit selection when the saved chat
model is unavailable but another installed model is eligible. It preserves the
saved selection until clicked. With no eligible installed chat model, the guide
opens the curated Discover view. Neither path starts a download or sends a
message; the first successful model reply still completes onboarding.

The desktop header asks the trusted native bridge whether its terminal runtime
is available. Browser mode and a desktop build without that runtime omit the
terminal entry. The status request does not create a terminal view or shell;
opening remains an explicit click. This does not change terminal authority or
the two already tested OffGrid CLI and Agent entry modes.

## Checks

- `npm run check` and `npm run build` in `apps/web`: passed.
- `npm test` in `apps/desktop`: 126 passed, one existing skip.
- `npm run build:win -- --dir` in `apps/desktop`: packaged Windows x64 build
  passed. `node scripts/test/test-desktop-terminal-packaged.mjs
  "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"`: passed with a
  disposable profile and service fixture. It exercised both CLI entry modes,
  retained cursor/draft, overlay focus, 200% zoom and descendant cleanup on the
  candidate containing the native status bridge.
- `node scripts/test/test-web-workspace.mjs build/offgrid-voice-release-check.exe
  tests/workspace-experience.spec.ts tests/navigation.spec.ts
  tests/navigation-overlays.spec.ts tests/terminal-entry.spec.ts
  tests/chat-readiness.spec.ts --workers=4 --reporter=dot`: 34 passed against
  disposable service data. Tests cover explicit model replacement, no download,
  absent terminal capability, no shell startup on mount, navigation, narrow
  first use and nine localized draft-preservation cases.
- `node scripts/maintenance/check-source-size.mjs`,
  `node scripts/maintenance/check-application-layout.mjs`,
  `openspec.cmd validate --all --strict --no-interactive` and
  `git diff --check`: passed.
- The earlier [packaged Windows terminal record](desktop-terminal-polish-2026-11-03.md)
  details the focus and ownership boundaries. The current candidate check does
  not qualify macOS, Linux or Windows ARM64.

The first browser-only navigation attempt failed because it expected a live
service and received HTTP 404; the same test passed with the disposable service
fixture. An older first-use test expected the previous Installed route; it was
updated to assert Discover. A synthetic desktop bridge fixture initially omitted
the required theme methods; it was corrected before the passing rerun.

This follow-up addresses the terminal integration task. It does not close the
larger guided setup, document-answer, human usability or platform gates. No
installed application or personal workspace was changed in this pass.

## First-use recovery follow-up (2026-11-08)

Three new browser tests reproduced two defects before the patch: dismissing
the guide with Close or Escape displayed it again after reload, and a failed
model inventory read was presented as an empty library with installation
guidance. Explicit dismissal now persists the existing working stage without
marking onboarding complete. A failed inventory read shows its error and a
retry action inside the guide; model selection and import actions require a
successful inventory read. A retry preserves the saved model selection.

Validation on the follow-up source:

- `npx.cmd playwright test tests/chat-readiness.spec.ts tests/workspace-experience.spec.ts --workers=1`:
  32 passed. The invocation also included the unmatched name
  `tests/setup-handoff.spec.ts`; that name selected no additional tests.
- `npx.cmd playwright test tests/offline-model-setup.spec.ts --workers=1`:
  14 passed, including all nine locales, explicit import, permission/capability
  restrictions and retained drafts.
- `npm.cmd run check`, `npm.cmd run api:check`, and `npm.cmd run build` in
  `apps/web`: passed. The build retains its existing large-chunk warning.
- Documentation links, source-size ratchet and `git diff --check`: passed.

One initial post-patch test used the label `Retry` while the existing localized
English action is `Try again`; correcting that selector produced the passing
run above. The original three pre-patch behavior failures remain distinct from
that test-label error.

This is partial progress on task 22.2, not completion of guided setup or human
usability qualification. These changes follow the published 0.4.28 source and
are not included in its installed or published binaries. The installed app and
personal workspace were not changed during these isolated browser tests.
