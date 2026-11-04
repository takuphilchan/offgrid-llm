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
