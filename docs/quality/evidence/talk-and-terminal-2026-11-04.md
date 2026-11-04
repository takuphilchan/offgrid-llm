# Talk and terminal entry verification, November 4

## Implemented behavior

Ordinary Chat dictation now ends capture after qualified speech followed by a
bounded pause. Its transcript remains an editable draft. The separate,
session-only Talk control checks the selected ASR and TTS packages, captures one
utterance, submits one saved Chat request, reads only the committed answer, then
listens again after natural playback completion. Stop, typing, model changes and
navigation fence or end the loop; failed turns do not replay automatically.
Talk is unavailable for agent execution and does not grant desktop authority.

The Windows terminal now has two explicit entries. **OffGrid CLI** opens a native
PowerShell prompt with the packaged matching `offgrid` executable and configured
service in its environment. **Agent** launches that executable's existing agent
profile chooser. Existing terminal sessions, selected-folder identity, service
authorization, four-tab limit and owned-process shutdown remain in force.

## Checks run in this source tree

- `go test ./...`: passed (earlier in this implementation pass).
- `npm.cmd --prefix apps/web run api:check`: passed.
- `npm.cmd --prefix apps/web run check`: passed after extracting the Chat speech
  and Talk hooks.
- `npm.cmd --prefix apps/web run build`: passed. Vite reported its existing large
  bundle warning; it did not fail the build.
- `npx.cmd playwright test tests/voice-playback.spec.ts --workers=1 --reporter=line`
  from `apps/web`: 30 passed after the hook extraction. The tests cover silence,
  pause, editable dictation, new-conversation Talk, one request, playback/relisten,
  no echo capture, Stop during generation, draft preservation and narrow layout.
  The microphone and speech service are synthetic browser fixtures.
- `node scripts/test/test-web-workspace.mjs
  build/offgrid-voice-release-check.exe --workers=4 --reporter=dot`: 983 passed,
  31 skipped against a disposable service and data directory. The first broad
  run exposed a development-only import in the pause test, an outdated speech
  setup expectation, and two PDF tests using a stale adjacent local worker.
  The test/import expectation was corrected; the pinned PDF worker was rebuilt
  beside the isolated service. The four affected test files then passed
  (96 passed, one skipped) before the full passing rerun. No installed service
  or user workspace was used.
- `npm.cmd --prefix apps/desktop test`: 125 passed, one Windows symlink fixture
  skipped because that fixture cannot create the symlink here.
- `npm.cmd --prefix apps/desktop run build:win -- --dir`: passed. The packaged
  Windows x64 build includes the local terminal and speech assets.
- `node scripts/test/test-desktop-terminal-packaged.mjs
  "apps/desktop/dist/win-unpacked/OffGrid LLM Desktop.exe"`: passed with an
  isolated local profile and HTTP fixture. The real Electron/ConPTY run opened
  OffGrid CLI, executed `offgrid --help`, opened Agent profile selection, and
  checked tab/cursor continuity, Unicode, resize and owned child cleanup.
- `node scripts/maintenance/check-docs.mjs`: 313 maintained Markdown files,
  zero problems. `node scripts/maintenance/check-repository-hygiene.mjs`,
  `node scripts/maintenance/check-application-layout.mjs`,
  `node scripts/maintenance/check-source-size.mjs`, and `git diff --check`:
  passed.
- `openspec.cmd validate --all --strict --no-interactive`: 29 items passed.

## Limits

The browser tests establish state transitions and request discipline; they do
not measure real speech recognition accuracy, acoustic pause detection in noisy
rooms, speaker echo cancellation on physical devices, spoken-answer quality or
latency. This pass did not record a user microphone. The packaged terminal test
qualifies Windows x64 only. macOS, Linux and Windows ARM64 native terminal
packages remain unverified. `go test -race ./internal/audio ./internal/server`
could not run because this Windows Go environment has CGO disabled and no C
compiler available. Full release CI and human accessibility/localization review
have not been claimed by these focused checks.

## Cross-platform CI follow-up

The first pushed CI run exposed a Windows-only path assertion in the terminal
test. Explicit Windows path construction corrected it; the desktop suite and
packaged Windows CLI/Agent launch check passed again. The next CI run passed
that test on both macOS runners, then found two speech-package boundaries:
Windows `tar` treated an absolute drive-letter archive path as a remote host,
and the pinned upstream macOS Piper archives omit `libespeak-ng.1.dylib`.
Archive extraction now uses relative archive and destination names under the
verified cache. Git for Windows `tar` extracted the pinned Whisper source with
those arguments in an isolated smoke check. The
macOS package includes the pinned Whisper runtime but does not bundle or claim
Piper readiness until a complete, locally verified runtime is available.
The Windows x64 desktop package rebuilt successfully with both speech engines.
Cross-platform packaging remains subject to the next CI run; this note is not
macOS speech qualification.
