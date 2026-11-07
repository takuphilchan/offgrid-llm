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

## Follow-up: full-width 0.4.28 installer

The first refresh retained the stock bitmap-column layout and did not adequately
address the presentation feedback. The follow-up uses a small layer mark and
aligned, full-width heading/body controls on welcome and finish pages. Geometry
uses native dialog units. The existing wizard lifecycle and install/uninstall
boundaries remain unchanged.

The final candidate was built with `npx.cmd electron-builder --config
installer-test.cjs --win nsis --x64 --publish never` from `apps/desktop`, then
tested with `powershell.exe -NoProfile -ExecutionPolicy Bypass -File
scripts/release/test-windows-installer.ps1 -InstallerPath
"D:/offgrid-llm/build/windows-installer-smoke/OffGrid Desktop Install Test-Setup-0.4.28.exe"`.

The test passed clean installation, installed startup, Finish launch choices,
silent running-app refusal, interactive reinstall, uninstall, workspace fixture
preservation and independently managed service survival. Finish closed in 74 ms
for installation, 58 ms for reinstall and 66 ms for removal. The harness also
checked heading/body alignment, usable text width and non-overlapping bounds.
Actual installation and uninstall welcome screenshots were inspected.

Final log: `build/installer-refresh-final.log`. Private captures and result:
`%TEMP%/offgrid-install-d25f300d869a442c9fddccd86b38a9d6/diagnostics`.
Startup evidence: `%TEMP%/offgrid-desktop-startup-y0C3CV`.
Desktop unit tests passed 141 with one skip. This supersedes the earlier candidate
for these installer checks without replacing its historical evidence. Elevated
installation, manual accessibility and other native platforms remain unverified.
