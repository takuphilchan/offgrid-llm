# Speech, composer and terminal follow-up — 2026-11-06

## Reported failures and corrections

- Two terminal-open requests could each navigate the same unloaded renderer,
  aborting the first navigation and showing an error despite successful opening.
  Opening now shares a pending load. Returning to Chat while loading prevents
  the eventual load from reopening the terminal. Genuine load failures remain
  retryable; shutdown does not display a spurious opening-error dialog.
- Whisper silence annotations were copied into both transcript text and segments.
  Provider-specific normalization removes edge `[BLANK_AUDIO]` annotations and
  ignores silence-only segments. Interior literal mentions remain intact. Saved
  messages and typed input are not rewritten.
- Empty speech now reports that no speech was detected, retains the draft, and
  does not send an empty message. Built-in microphone errors use the current UI
  language even when permission completes after a language change.
- The empty-chat layout explicitly centered the composer. It now uses the same
  bottom slot as populated Chat. The input remains available while listening,
  including typing to exit Talk.
- Attachment actions now use a compact panel with consistent row spacing,
  distinct document/image/folder icons, and a separator before saved files.
- Help uses expandable topics, including documents and speech, with desktop
  terminal guidance. Locale initialization accepts supported regional preferences,
  checks the browser's ordered language list, and rejects inherited object keys.
- CLI transfer progress accounts for the complete rendered line and falls back
  to compact progress on narrow terminals instead of wrapping animation output.

## Verification

- `go test ./internal/audio`: passed (Windows; POSIX worker fixtures remain skipped).
- `go test ./cmd/offgrid -run TestTransferProgress -count=1`: passed.
- `npm run build` in `apps/web` (`tsc --noEmit` and Vite): passed.
- `npm test` in `apps/desktop`: 132 passed, one skipped.
- Browser shell, document attachment and voice suites: 70 passed, one skipped;
  `build/ui-followup-verified-20261106.log`.
- Cross-page browser suites: 83 passed, one skipped, one stale Send-button
  selector failed. The selector now checks the empty composer's Talk action.
  The follow-up nine-locale test and four attachment/speech tests passed:
  `build/ui-followup-final-targets-20261106.log`.
- Real-service PDF/Office preparation and PDF preview: both passed using the
  packaged service and colocated document worker. The earlier standalone test
  binary lacked that worker, causing two preparation failures. Evidence:
  `build/real-document-followup-20261106.log`.
- Packaged Windows appearance/Help/zoom check passed:
  `build/packaged-speech-composer-followup-20261106.log`.
- Packaged terminal check passed: CLI interaction, Agent draft and clear,
  tabs, Unicode, cursor preservation, hidden-session retention, clipboard
  restrictions, child cleanup after close/quit/reload, and service identity.
  Evidence: `build/packaged-terminal-followup-20261106.log`.
- Attachment menu screenshots inspected in light desktop and dark narrow Arabic
  RTL layouts. Outputs are private generated evidence under
  `build/ui-followup-visual-20261106`.

An intermediate attempt to hide the empty editor while recording failed the
typing-to-exit-Talk test. That rule was removed and the test passed again. The
initial menu test reset its locale through fixture setup; it was corrected to
switch language/theme through the real controls and assert both before capture.

These checks do not establish native macOS/Linux qualification, subjective
speech accuracy, or independent human review of translations/usability.

## Installation candidate

Prepared preview, version 0.4.26; no new public release:
`build/desktop-speech-composer-followup-20261106/OffGrid LLM Desktop-Setup-0.4.26.exe`.

- Installer SHA256: `A2C255BF281B8C2FE4A135632D1835114136C4432355E65E970F55EE32D30789`.
- Packaged UI index SHA256: `0320C0FE1E470ADB39E3B47F54A95087A839AB3A73F353216CF8988BAB3E00FB`.
- Prepared guarded installation script: `build/install-speech-composer-20261106.ps1`.
- Installed after the user confirmed quitting. The guarded installer exited zero;
  all five checked application payloads matched the verified candidate. All 383
  root-workspace and 10 desktop-workspace data-file hashes were unchanged.
  Model inventory and native inference executable hashes were unchanged.
- The previous application, workspace data, configuration and desktop profile
  remain in a verified rollback backup outside the repository. The private
  `install-result.json` beside the candidate records its location and checks.
- The normal application has not been launched after replacement. Packaged
  behavior checks above apply to the identical installed payload; they do not
  constitute a new observation of the user's running workspace.

Local focused commits include `c6d490d` (Whisper markers), `f484944` (terminal
opening), `fc68d280` (transfer progress), `147cc4bc` (terminal presentation),
`674c477c` (CLI command layouts) and `a048765` (agent session continuity).
The remaining web UI changes still await grouped release review and commit.

## Subsequent installed-startup failure

The user subsequently opened this preview and encountered a real bundle-identity
failure. The service had no embedded release version and read Electron's `version`
file from the installation directory on Windows. Payload equality above did not
establish startup correctness. See [the startup investigation and fix](desktop-startup-identity-2026-11-06.md)
for the preserved failure, packaging prevention and installed-directory regression.
