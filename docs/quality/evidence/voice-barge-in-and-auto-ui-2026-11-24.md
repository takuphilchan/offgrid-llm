# Voice interruption and automatic Talk feedback — 2026-11-24

Scope: the installed candidate's inability to interrupt spoken replies hands-free,
and the user's report that automatic Talk resembles a timed recording. This
continues `voice-model-foundation` tasks 14.2, 14.5 and 14.6; none is closed by
these bounded corrections. No user microphone or screen was accessed.

## Findings and implementation

- Applied echo cancellation was accepted only when exactly `true`, and noise
  suppression was also required. The new configuration helper accepts applied
  `true` or `all` independently of noise suppression, requests `all` only when
  advertised, and re-reads settings after a rejected upgrade. False, unknown and
  remote-only modes do not qualify local playback interruption. Asynchronous
  configuration is fenced by capture lifetime, including device loss and Stop.
- The previous detector needed nine consecutive 20 ms frames above RMS .018.
  The negative-control unit run rejected quiet speech separated by brief gaps.
  Onset now uses an adaptive floor (minimum .004), accumulates 180 ms voiced
  duration within a rolling 300 ms window across gaps up to 80 ms, and expires
  isolated or periodic short spikes. Review reproduced periodic spikes falsely
  accumulating before the rolling-window correction. Continuation
  uses the same learned noise floor with a lower minimum. Pre-roll, byte/time
  limits, queue bounds and immediate synchronous playback cancellation remain.
- Automatic Talk no longer displays a running recording timer. Its orb uses
  the same visual amplitude scaling as manual dictation, without changing the
  detector input. One status remains visible, explicit completion/discard remain
  available, and the exit control stays in the same position across phases.

## Executed evidence

| Check | Result and retained log |
|---|---|
| Detector and integrated capture/UI | 23 passed, 2.1 minutes; `build/voice-barge-in-ui-unsandboxed-20261124.log` |
| Quiet gapped hands-free conversation | Included above: applied all-mode AEC with noise suppression disabled; onset pauses playback before ASR, natural pause submits the next utterance, one capture lifetime, no background-job Stop |
| Public recorded speech through real browser audio graph | 1 passed, 16.7 seconds; `build/voice-recorded-interruption-20261124.log`. Existing upstream JFK WAV at gain .05, with production AudioWorklet/capture; playback paused before recognition. No hardware microphone, physical echo path or native recognition is exercised by this test |
| Lifetime, canonical steering, capability and playback regression | 93 passed, 2 skipped, 9.0 minutes; `build/voice-barge-in-regression-20261124.log`. Skips are opt-in documentation captures |
| Final quiet-input orb response | 1 passed, 26.7 seconds; `build/voice-orb-response-20261124.log` |
| Final reviewed detector, recorded speech and UI | 25 passed, 4.0 minutes; `build/voice-barge-in-final-20261124.log`, including periodic-spike rejection and the reduced-gain recording |
| UI geometry | Included in 23-case run: light/dark at 630×790, 360×640 and 360×320; no timer, one status, fixed Stop position, no button overlaps and at least 44 px targets |

The sandboxed initial runs completed assertions but hung during browser/server
teardown. Their logs were retained; a complete outside-sandbox rerun exited zero.
They are not substituted for completed test processes.

Final renderer typecheck/build passed; documentation (526 files), source-size
ratchet, translation parity and strict OpenSpec validation passed. Existing
preview-locale translation coverage remains incomplete. Independent review found
no blocking defect after the rolling-window correction; it did not qualify
physical echo rejection. Build log: `build/voice-barge-in-build-final-20261124.log`.

## Local installation

Source checkpoint: `build/release-0.4.33-barge-in-20261124.tar`, 4,331 files,
SHA-256 `f6856f907e04d2add9cc8edb799b7af352e2ddbccbc42ef6b8d1471502acbcb7`.
This installation receipt was added after the immutable source snapshot.

Candidate: `build/release-0.4.33-candidate-20261124-attempt8`, unsigned local
preview. Attempt 7 hit an Electron download timeout; attempt 8 reused the locally
installed matching pinned Electron 43.7.9 runtime. Neither was published.

Hidden packaged main/preload startup passed against an isolated HTTP fixture;
no screenshot, screen sharing or microphone capture was used. The first startup
fixture incorrectly advertised unsupported desktop bridge protocol 2 and timed
out in recovery. The corrected fixture uses the current protocol 1 and passed;
both logs are retained. Final receipt:
`build/voice-barge-in-packaged-startup-20261124.json`.

The installed desktop shut down through its normal update handshake. The first
launcher inherited `ELECTRON_RUN_AS_NODE=1`; removing that variable for the
launcher allowed normal graceful shutdown. No installed process was force-killed.
A fresh private backup hash-verified 2,700 files / 1,228,948,594 bytes at
`C:/Users/phil/AppData/Local/offgrid-owner-qualification-2d7a211e371141e083ec243875ad226e`.
Previous backups remain intact; model files were retained in place.

The installer matched SHA-256
`aeea2fc44a2075070c98c56797680fa166549c4a6abe5d22284212bbe41d308c`
and exited **0** at 2026-11-24 12:55 Asia/Shanghai. All **73** checked installed
executable, service, app archive and UI files matched the candidate. Receipts:
`build/voice-barge-in-install-result-20261124.json` and
`build/voice-barge-in-installed-byte-proof-20261124.json`.
The installed application was reopened through its executable without reading
the user's screen or using native computer control. Acoustic behavior remains
unverified. No commit, tag, push or public release was made by this increment.

### Follow-up: listening and speaking control layout

User screenshots exposed a visual defect missed by the previous geometry checks:
the fixed-width bordered dock appeared empty during listening and squeezed
“Speak again” onto two lines. The corrective layout removes the dock border and
background, centers the exit button, places recording actions on either side,
and gives “Speak again” an independent content-sized row. Status is grouped closer
to the voice indicator. Initial focus goes to the Talk surface, avoiding a
permanent button focus ring; Tab still focuses the controls and Escape still exits.

All six expanded light/dark layout scenarios passed in 50.1 seconds, including
speaking, one-line action text, centered stable exit position, 44 px targets,
nonoverlap and keyboard focus. Actual isolated renders were visually inspected;
no user screen access was used. Log: `build/voice-layout-polish-20261124.log`;
renders: `build/voice-talk-layout-polish-20261124/`. Renderer build/typecheck
passed in `build/voice-layout-polish-build-20261124.log`. This is a presentation
correction; earlier acoustic limitations remain unchanged.

Six English/Arabic/Chinese light/dark voice-surface checks also passed (14.4
seconds), along with hidden packaged startup. Candidate attempt 9 used source
archive `build/release-0.4.33-talk-layout-20261124.tar`, SHA-256
`29bda2bae507836e50ee602a618eb2a48c337d201f5557e5543d7c993ed4d23a`.
A new verified backup retained 2,710 files / 1,230,451,923 bytes under private
fixture `offgrid-owner-qualification-d62fd5450d794baa8052a7349a456a56`.
Installation exited zero at 2026-11-24 14:06 Asia/Shanghai, all 73 installed
payload checks matched, and OffGrid was reopened. Receipts:
`build/voice-layout-polish-install-result-20261124.json` and
`build/voice-layout-polish-installed-byte-proof-20261124.json`. No screen or
microphone access, public release, commit or push was performed.

To repeat recorded-speech qualification, set `OFFGRID_KNOWN_SPEECH_WAV` to an
existing public known recording and run `talk-continuous-browser.spec.ts` with
`-g "known recorded speech"`. The test records the WAV digest/size and gain as an
attachment. Without the explicit fixture it skips; it never downloads or records.

## Remaining acceptance

Applied browser settings do not prove acoustic cancellation. The detector is
energy-based, not a semantic VAD, and sustained noise or residual speaker echo
can still resemble speech. Speaker/headset/device/noise/language qualification,
native ASR/TTS conversation latency, backchannels, durable interrupted-answer
context and human naturalness testing remain open. The exact user device has
not been diagnosed. No full-duplex, release-wide or M0–M8 completion is claimed.
