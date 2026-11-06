# Management and terminal follow-up — 2026-11-06

## Implemented and installed

Installed the authorized Windows x64 OffGrid 0.4.27 candidate after the owner quit the application. Changes cover Activity cleanup placement, a compact Knowledge menu, MCP before the tool inventory, focused external-provider setup with focus restoration, stopped GGUF partial-download discard, and terminal welcome/navigation.

Sidebar Chat and New chat return from the native terminal even when the route does not change. Existing terminal sessions are retained. Discard requires explicit confirmation and rejects active downloads; published models are preserved.

## Checks

- `npm --prefix apps/desktop test`: 139 passed, one skipped.
- `npm --prefix apps/web run check`, `build`, and `api:check`: passed; existing build chunk-size warning remains.
- `go test ./internal/models ./internal/server`: models passed; server found the stale route-count contract. After updating the count and route fixture, `go test ./internal/server -run 'TestServerRouteRegistrationContract|TestDiscard' -count=1` passed.
- Focused initial browser management/integration/action tests: 15 passed.
- Full browser run with the complete packaged service: 1,105 passed, 31 skipped, one failed (Knowledge import-limit alert). The complete Knowledge import and management refinement files rerun with one worker passed all 23 tests. The intermittent failure is not treated as a clean full-suite pass.
- `node scripts/test/test-desktop-terminal-packaged.mjs build/desktop-management-refinement-20261106/win-unpacked/OffGrid\ LLM\ Desktop.exe` (path supplied as one quoted argument): passed. Verified CLI/Agent entry, same-route sidebar navigation, Unicode, cursor and draft preservation, hidden tabs, search, clipboard boundary, and child cleanup on close/reload/quit.
- `openspec validate workspace-outcome-experience --strict`: passed.
- `node scripts/maintenance/check-docs.mjs`: 328 maintained files, zero problems before this evidence addition.

Visual inspection covered the Knowledge menu, MCP placement with a large inventory, Activity toolbar at narrow/wide widths, and the packaged terminal welcome. Prior test attempts exposed obsolete selectors and an onboarding/navigation race in the packaged test fixture; these were corrected rather than suppressing assertions.

## Installation preservation

Installer: `OffGrid-0.4.27-windows-x64-setup.exe`.

SHA-256: `BFB4AEAD2E5032AFA897FAA2A379214013E91D632F5D3AE2313B47501520E615`.

Rollback directory: `D:\OffGrid-install-backups\v0.4.27-before-management-refinement-20261106-231805`.

Receipt: `build/desktop-management-refinement-20261106/install-result.json`.

Verified five principal payloads against the candidate, all workspace data hashes (383 root files and 10 desktop-workspace files), unchanged model inventory, inference executable hashes, Electron profile, stable uninstall identity and Start shortcut. The installer did not start the user's application or run user tasks.

## Remaining qualification

No macOS/Linux/ARM64 native qualification or human usability/accessibility evaluation is claimed. The intermittent full-browser failure remains a release verification limitation despite the successful focused rerun. This installation is not a Git commit, publication, or release qualification.

The later [November 7 full-suite rerun](terminal-session-followup-2026-11-07.md)
passed in the built-service environment and supersedes that renderer limitation.
The earlier failure and native/human qualification limits remain recorded here.
