# Native Windows installer refresh

## Scope and implementation

The existing NSIS assisted wizard now has a compact OffGrid welcome, neutral
artwork, shorter completion copy and uninstall guidance distinguishing application
removal from retained models and saved work. Native scope/folder choices, keyboard
controls, high-contrast guard, running-app protection and deferred unelevated
launch remain. No installer identity or workspace location changed.

## Executed verification

- `go test ./...` passed across the repository.
- Desktop tests passed 139 tests with one skip; installer tests passed 3/3.
- `powershell.exe -NoProfile -File scripts/release/test-windows-installer-probe.ps1`
  passed, including frozen-window detection and bounded diagnostic capture.
- From `apps/desktop`, `npx.cmd electron-builder --config installer-test.cjs --win
  nsis --x64 --publish never` built the isolated 0.4.27 candidate successfully.
- `powershell.exe -NoProfile -File scripts/release/test-windows-installer.ps1
  -InstallerPath 'D:\offgrid-llm\build\windows-installer-smoke\OffGrid Desktop Install Test-Setup-0.4.27.exe'`
  passed clean installation, real installed startup, silent repair, checked Finish
  launch, refusal to silently replace a running app, interactive repair with
  normal shutdown, unchecked Finish, interactive uninstall and fixture retention.
  The independently managed fixture service remained available and unchanged.
- Finish closed in 160 ms for install, 74 ms for repair and 64 ms for uninstall.
  The isolated application and its uninstall registration were removed; retained
  workspace data was checked by SHA-256. Production user data was not used.
- Actual install welcome/progress/finish and uninstall welcome/finish captures
  were inspected. Text and controls were readable without clipping at the tested
  Windows scale. The test-only window title identifies the isolated package.
- Documentation, source-size, diff whitespace and strict OpenSpec checks passed.

Private result and captures are retained under
`%TEMP%\offgrid-install-02511f4edb8f400796edf65d1fe2f0b7\diagnostics`;
`result.json` records the final checks. Startup evidence is in
`%TEMP%\offgrid-desktop-startup-fTQ9RN`.

## Failures resolved during verification

The first interactive run launched the wizard hidden, which kept its new welcome
page invisible. Only the explicitly interactive isolated wizard now launches
normally; background helpers and silent installs remain hidden. A second run
exposed ineffective uninstall completion definitions. The supported
`customUninstallPage` hook now sets the page-local finish definitions immediately
before the native uninstall finish page. The corrected run and final persisted
result passed. Earlier diagnostic directories were retained, not erased.

## Limits

This is per-user Windows x64 testing, not elevated all-user/UAC qualification,
screen-reader testing, a high-contrast manual review or a full mixed-DPI matrix.
macOS/Linux installer appearance and hardware/model behavior were not qualified
by this test. The candidate uses 0.4.27 package metadata; final release CI must
test the exact new release revision. Installation into the owner's production
workspace is a separate operation.
