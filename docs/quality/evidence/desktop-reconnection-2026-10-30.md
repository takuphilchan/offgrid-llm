# Desktop startup reconnection — 2026-10-30

## Finding and implemented scope

A remembered external workspace returned `service_unavailable` immediately on
ECONNREFUSED. A timeout similarly ended the initial attempt. Only desktop-owned
children received startup polling. Neither case automatically retried a delayed
external service. The screenshot's immediate attention state matched this path.

DesktopRuntime now gives remembered external endpoints the 30-second startup
grace period, followed by one bounded handshake every five seconds. All nine
desktop locales display waiting/reconnection guidance. Recovery actions become
available after grace expires. Explicit Retry shares in-flight work; isolated
recovery and Quit cancel scheduled checks. Compatibility failures stop retries.
Identity revocation, external ownership and prohibition on replacement workspaces
remain enforced. Fresh standalone installs still start their native child.

No Docker/WSL launcher, task replay, data migration or permission grant was added.
Windows/macOS desktop native service startup does not require Linux or Docker.

## Actual checks

- `npm.cmd --prefix apps/desktop test`: 92 passed, 1 skipped. Includes delayed
  external readiness, timeout, automatic reconnection after grace, manual retry,
  cancellation, isolation, compatibility, native child startup and nine-locale UI.
- `npm.cmd --prefix apps/web run build`: passed; existing chunk-size warning.
- `node scripts/test/test-desktop-startup.mjs "build/ui-craft/startup-package/OffGrid LLM Desktop.exe"`:
  passed with an isolated packaged Windows app and disposable HTTP/workspace
  fixtures. The external service stayed offline beyond grace, then attachment
  completed without clicking Retry. Native bundled child and IPC rejection cases
  also passed. First window 2520 ms; initial recovery fixture 2812 ms.
- `node scripts/test/test-desktop-theme.mjs "C:/Users/phil/AppData/Local/Programs/OffGrid LLM Desktop/OffGrid LLM Desktop.exe"`:
  passed after installation with an isolated profile: saved/live native themes,
  nine readable options and 200% zoom across the affected routes.
- `node --test scripts/maintenance/check-docs.test.mjs`: 7 passed.
- `node scripts/maintenance/check-docs.mjs`: 278 maintained files, zero problems.
- `node scripts/maintenance/check-source-size.mjs`: passed.
- `openspec validate workspace-outcome-experience --strict`: passed.
- `git diff --check`: passed (line-ending notices only).

Private fixture evidence: `%TEMP%/offgrid-desktop-startup-ZY4Eud` and
`%TEMP%/offgrid-desktop-theme-fuaDMW`; logs under ignored `build/ui-craft`.
Waiting-state screenshot was visually inspected.

## Local installation and separate environment issue

Installed the tested desktop archive and local UI resources, preserving backend,
executor and computer binaries; verified archive/UI identity and companion pack
digests. Rollback remains at
`%LOCALAPPDATA%/OffGrid-install-backups/startup-reconnect-20261030-183628`.
The UI index fingerprint remains
`e3b59b7d617bce87cfb3cde0d2fd704b8e837843d52a7b4be37ce9e72133dbfe`:
only the desktop presentation dictionary changed in web output.

The existing Ubuntu/Docker environment repeatedly stopped between diagnostics.
Keeping a temporary foreground WSL process alive allowed Docker to complete
initialization. Windows localhost health then reported healthy, five models,
zero active inference and exactly one healthy `offgrid` container. No container,
model or workspace data was replaced. The temporary diagnostic process is not a
permanent startup repair. The user subsequently selected native Windows workspace
migration with backups and preservation of GPU acceleration. Preparation and
cutover evidence are tracked separately in
[native Windows migration](native-windows-migration-2026-10-30.md); the startup
fix alone does not migrate or replace an external workspace.

No actual laptop reboot, real macOS/Linux package qualification, native-speaker
translation review or full unrelated Go/renderer suite was performed in this
focused desktop fix. Earlier unresolved usability/performance gates remain open.
