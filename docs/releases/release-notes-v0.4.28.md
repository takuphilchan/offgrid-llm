# OffGrid 0.4.28

This maintenance release improves Talk continuity, external-agent diagnostics,
the terminal's copy/interrupt behavior and the Windows installation experience.

## Changes

- Talk keeps listening after silence or a successful empty transcription. It
  starts a fresh capture after a short delay without creating an empty message
  or replaying audio. Stop, typing and navigation cancel the pending capture.
  Real recording, transcription and request failures still pause for review.
- Hermes and OpenClaw status identify missing local runtimes explicitly, show
  dependent checks as not checked, and distinguish Windows from WSL. CLI output
  separates setup steps from blockers without repeating the entire report.
  Model selections and context requirements are unchanged.
- Windows Setup uses a quieter welcome, clearer progress and shorter completion
  pages with full-width text and a small OffGrid mark. Uninstall explains that
  application removal keeps models and saved work.
  Running-app protection, install scope/folder choices and optional launch remain.
- In the desktop terminal, Ctrl+C copies selected text; without a selection it
  interrupts the command. Ctrl+Shift+C remains available. Keyboard focus on the
  welcome cards is subtler, and native title-bar geometry follows zoom changes.

## Verification and limits

Local Go tests passed across the repository. Speech browser regression tests
passed 43/43 with isolated capture and ASR/TTS fixtures. Desktop tests passed
141 with one skip. Real Windows tests passed clean installation, installed
startup, repair, running-app refusal, Finish launch choices, uninstall and saved
data/independent-service preservation. Installer watchdog checks also passed.
Packaged terminal checks passed selection copy during a running command,
interruption without selection, Unicode paste-event handling, tab/cursor
preservation and owned child-process cleanup.

See the [speech and integration evidence](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.28/docs/quality/evidence/talk-silence-and-integration-status-2026-11-07.md)
and [native installer evidence](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.28/docs/quality/evidence/native-installer-refresh-2026-11-07.md).
The [terminal evidence](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.28/docs/quality/evidence/terminal-session-followup-2026-11-07.md)
records the clipboard fixture and platform limits.
Exact-source CI and release artifact checks must pass before publication.

These checks do not establish physical-microphone accuracy, every external-agent
model workflow, elevated Windows installation, manual accessibility or broader
enterprise/hardware qualification. Those limits remain recorded separately.

## Downloads and upgrades

Choose the desktop package matching your operating system and architecture;
Windows desktop users should choose `OffGrid-0.4.28-windows-x64-setup.exe`.
Verify downloads using the published checksum file. Finish active tasks and quit
OffGrid before installing. Existing application identity, models, conversations
and workspace locations remain compatible. Docker/WSL deployments are separate
and are not replaced by the desktop installer.
