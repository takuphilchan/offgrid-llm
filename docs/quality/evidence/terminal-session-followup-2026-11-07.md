# Terminal and session follow-up — 2026-11-07

## Scope and findings

- Agent was the terminal's root process. `/exit` therefore ended the tab's
  process. It now runs as a fixed, escaped command inside the supervised
  PowerShell session, with `-NoExit`. The same folder and bundled CLI stay usable.
- The terminal output had an unnecessary centered width limit. Output now uses
  the pane width, tab labels truncate without losing their start, and the plus
  menu groups CLI, Agent and PowerShell. “Session open” does not imply a busy command.
- Generated Chat names included Windows-invalid filename characters. The exact
  example `So, how are you?` could fail before saving. Generated titles now have
  portable punctuation and a UTF-8 byte bound; the original message is retained.
  Windows session storage rejects invalid names before file I/O.
- Settings treated zero active computer sessions as proof that no platform
  component was installed. Native component presence and connected sessions are
  now separate. Component presence does not grant control or prove qualification.

## Verification

- `go test ./internal/sessions ./internal/cli`: passed.
- `go test ./internal/sessions`: passed after adding the exact question
  persistence regression.
- `go test ./internal/server -run 'Session'`: passed.
- `npm --prefix apps/desktop test`: 139 passed, one skipped.
- Web `npm run check` and `npm run build`: passed. Existing chunk-size warning remains.
- `npx playwright test tests/session-storage-presentation.spec.ts tests/knowledge-import.spec.ts --workers=1`: 23 passed.
- `OFFGRID_DOC_CAPTURES=1 npx playwright test tests/documentation-captures.spec.ts --workers=1`: one passed.
- `node scripts/test/test-desktop-terminal-packaged.mjs "build/desktop-terminal-followup-20261107/win-unpacked/OffGrid LLM Desktop.exe"` with documentation captures enabled: passed on Windows x64. Verified Agent exit, same-folder command execution, `offgrid --help`, literal-path handling in unit tests, tab switching, input/cursor preservation, search/copy, Unicode, 200% zoom, and descendant termination on close/reload/quit.
- `openspec validate desktop-integrated-terminal --strict`: passed.

Final packaged evidence: `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-theme-PnBIOE`.
Logs: `build/terminal-followup-*.log`.

## Test and packaging failures retained

The first build used `-trimpath`, which omitted the linker metadata required by
the existing packaging identity check. Rebuilt with the version linker flag and
without trimpath; the check was not relaxed. An initial packaged harness attempt
hit its deadline and left isolated Electron test processes; these were removed
before repackaging. The harness now has explicit locator timeouts and Windows
tree termination on its deadline. A later build was blocked by those test file
locks and was rerun after cleanup. The documentation capture initially expected
the prompt without its trailing space; corrected the expectation. Final
packaged assertions passed, including cleanup.

The previous full browser result (1,105 passed, 31 skipped, one Knowledge test
failure) remains recorded in the November 6 evidence. The test now waits for its
file input to become enabled before a second selection; the full Knowledge file
passes. This is not a claim that the full browser suite was rerun in this pass.

## Documentation captures and limits

README and the workspace tour use real UI captures with isolated sample data.
The terminal capture runs the packaged CLI with a shortened demo prompt. These
images do not establish inference accuracy or document-ingestion qualification.
No GIF was produced; the available Playwright FFmpeg build lacks a GIF encoder.
No macOS/Linux/ARM64 terminal qualification, human usability review or release
publication is claimed.

## Installer

Candidate: `build/desktop-terminal-followup-20261107/OffGrid-0.4.27-windows-x64-setup.exe`.
SHA-256: `30C36A98EFEAA47B43F11954B1CDEFE28BB12E24A1E8D7AC4DD3C9D9DD9C1D1F`.
Installation succeeded. Receipt:
`build/desktop-terminal-followup-20261107/install-result.json`.
Verified the five primary payloads, 384 root data files and 10 desktop-workspace
files, unchanged model inventory and inference runtimes, Electron profile,
stable uninstall identity and Start shortcut.
Rollback: `D:\OffGrid-install-backups\v0.4.27-before-terminal-followup-20261107-004928`.
The installed personal workspace was not launched or used to run test tasks.

## Final source verification, November 7

The later complete rerun supersedes the open full-suite limitation above without
removing its failure history:

- `go test ./...`: all packages passed (`build/release-0427-go-final.log`).
- `CGO_ENABLED=1 go test -race ./internal/agents ./internal/models ./internal/sessions ./internal/server`
  in WSL with Go 1.26.6 and GCC: all four passed. The initial Windows attempt could
  not run because CGO was unavailable; it is not counted as a race pass.
- `go build -ldflags '-X main.Version=v0.4.27' -o build/offgrid-release-review.exe ./cmd/offgrid`
  and web build/API checks: passed.
- `node scripts/test/test-web-workspace.mjs build/offgrid-release-review.exe --workers=2`:
  **1,115 passed, 32 skipped** in the isolated built-service environment
  (`build/release-0427-service-full.log`). This includes the new two-case
  duplicate Chat destination regression across all management pages.
- An earlier development-server run had 1,101 passes, 38 skips and six failures
  in tests requiring built assets or a real service. Those cases passed in the
  required built-service environment. Their failed log is retained as
  `build/release-0427-browser-final.log`.
- Companion suite: **75 passed**, including both browser-download failure orders.
  See [browser recovery](browser-recovery-2026-11-07.md).
- Linux release fixtures for finalization, asset access, desktop branding,
  source context, exact-revision CI and containers: passed. Installer entrypoint
  and executor-install shell tests passed. Windows execution of the Bash fixtures
  failed due to the launch/temp environment; the actual Linux runs provide the
  passing evidence, not that failed invocation.
- Documentation, repository hygiene, application/developer/Python layouts,
  source-size and both affected OpenSpec validations: passed.
- Refreshed the opt-in documentation captures after the sidebar correction:
  one test passed. The Knowledge image now has one Chat destination.

The final browser and sidebar source corrections postdate the installed candidate
identified above. They do not claim a newer installed payload. Exact-revision
remote CI and release-asset production remain separate gates. The build's existing
chunk-size warning and unavailable human/native-platform checks remain disclosed.

## Remote CI fixture correction

CI run `42269334837` exposed an environment dependency in the simulated terminal
supervisor failure test on both macOS runners. After Agent was moved inside
PowerShell, that fixture needed a Windows system root and executable-presence
stub before it could reach its intended rejected-spawn path. It now supplies
and restores those prerequisites explicitly. The production launcher and shutdown
assertions are unchanged. The six terminal-session tests passed on Windows and
Linux; the Windows desktop suite passed 139 tests with one platform skip.
Remote CI must be rerun on the corrected revision before release.
