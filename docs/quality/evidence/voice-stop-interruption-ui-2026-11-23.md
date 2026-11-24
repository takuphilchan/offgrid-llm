# Voice Stop, interruption and Talk controls — 2026-11-23

The user reported missed speech interruption, an empty dictation result after
Stop despite a moving meter, and duplicate/overlapping transcription controls.
This corrective increment continues voice-model-foundation sections 13–14.

## Corrections

- Talk capture is keyed to its session, not the draft key changed by saving the
  first utterance. The same microphone owner accepts subsequent utterances.
- Silent TTS preparation no longer activates the playback echo gate. Actual
  playback still requires reported echo cancellation and noise suppression;
  otherwise translated guidance directs the user to Speak again.
- The pinned Whisper source returns no segments for input shorter than one
  second. Captured short clips now receive silent tail padding to 1.1 seconds,
  without delaying Stop or changing recorded samples. Browser MediaRecorder
  audio is decoded to 16 kHz mono PCM16, reducing device-rate/stereo payloads.
- Dictation explicitly requests audio processing, resumes its level graph,
  cancels partial input on device loss, and offers microphone selection after
  permission in recovery. Device choice is session-only and shared with Talk.
  Playback is silenced before manual capture. Existing draft/cancellation
  ownership and explicit model selection remain intact.
- Talk has one central status. Inactive capture controls disappear; recording
  completion, discard and Stop Talk use separate bounded controls. Layout checks
  cover light/dark at 630×790, 360×640 and 360×320.

## Evidence

- Before normalization, real browser MediaRecorder produced 48 kHz stereo. It
  **did recognize** the known JFK recording. Rate alone did not reproduce the
  reported empty transcript. The initial normalization assertion failures are
  retained in `build/voice-device-rate-before-20261123.log` and
  `build/voice-device-rate-recognition-before-20261123.log`.
- Native negative control: a Piper-generated **0.4871655329-second** “Hello”
  returned `text: ""` through the pinned Whisper engine. The corrected real
  MediaRecorder/manual Stop path recognized “Hello”; the longer pause-completed
  recording also passed. **5 checks passed in 21.7 seconds** in
  `build/voice-short-native-final-20261123.log`.
  Private evidence directory:
  `C:/Users/phil/AppData/Local/offgrid-owner-qualification-555803c5094a4ce69a5d4ec8cc3b5e02`.
  Includes exact copied model/runtime/service hashes, short negative control,
  known/generated WAVs and browser traces. No physical microphone or download.
- Manual/playback regression: **61 passed, 2 documentation-only skips**,
  `build/voice-manual-regression-20261123.log`. Includes selecting a microphone
  after empty recognition, preserving edits, and disconnect without submission.
- Initial interruption regression: **23 passed** across real AudioWorklet
  synthetic input and canonical/lifetime tests. UI follow-up: **21 passed**.
  Logs: `voice-interrupt-review-20261123.log`,
  `voice-interrupt-ownership-regression-20261123.log`,
  `voice-talk-layout-20261123.log`, `voice-talk-layout-fallback-20261123.log`
  under `build/`. Eighteen screenshots are in
  `build/voice-talk-layout-20261123/`; light transcription and short-window
  recording screenshots were visually reviewed.
- TypeScript/build, API generation consistency and desktop regression passed.
  Desktop: 292 passed, 4 opt-in skips, zero failures (2.658 seconds).
- Combined production-service/browser run: **87 passed, 12 skipped** in
  2.5 minutes, `build/voice-combined-final-20261123.log`. Ten skips are source-hook
  lifetime tests requiring the separate Vite fixture; these passed in the
  16-case source/canonical run above. Two are optional documentation captures.
  No skipped test is counted as a pass. OpenSpec structure, documentation,
  source-size and translation checks passed; twenty preview languages retain
  four new English fallback messages and pending native review.
- Independent code review caught a disconnected, previously selected microphone
  with no recovery selector after remount. Recovery now always offers System
  default for an explicit selection, without silently switching devices.
  Browser regression verifies remount, unavailable selection and explicit reset.
  Existing WAV blobs bypass conversion; normalization/padding claims here cover
  Chromium encoded capture and continuous PCM, not arbitrary uploaded WAVs.

## Primary sources and limits

[Pinned Whisper short-input behavior](https://github.com/ggml-org/whisper.cpp/blob/51c6961c7b64b406833f4b6a4a20e67142f69225/src/whisper.cpp#L5073)
establishes the subsecond rejection. The real negative control above reproduces
it. [Media track settings](https://developer.mozilla.org/en-US/docs/Web/API/MediaStreamTrack/getSettings)
report applied processing settings, not measured acoustic quality.

The user's exact utterance/device has not been recorded or diagnosed; a moving
meter alone cannot prove recognizable speech. These fixes do not establish
physical echo/noise rejection, all language/model accuracy, conversational
latency, durable played-context completion or full-roadmap readiness. Tasks
14.1–14.6 remain open for their outstanding acceptance requirements.

## Local installation

Source checkpoint: `build/release-0.4.33-voice-fixes-20261123.tar`, 4,328 files,
SHA-256 `05d6ac9a35260652364e9ea7ff5c26415dac9de2a2daa758adf892a415a6e968`.
The post-install evidence in this section was added after that immutable snapshot.
Candidate: `build/release-0.4.33-candidate-20261123-attempt6` (unsigned preview;
not published and not complete-program qualification).

Packaged startup/recovery passed: first window 522 ms, recovery 649 ms,
`build/voice-fixes-packaged-startup-20261123.log`. Latest selected-device recovery
follow-up: 2 passed in 3.9 seconds, `build/voice-device-recovery-final-20261123.log`.

The installed app and its owned service/model runtimes shut down gracefully via
the existing installer-quit handshake. No force termination was needed. A fresh
private backup copied/hash-verified 2,651 files / 1,236,840,114 bytes at
`C:/Users/phil/AppData/Local/offgrid-owner-qualification-fceab873063041fc92b9330a4b4fdb56`.
Earlier rollback backups remain intact; model files stayed in place.

The installer digest matched its artifact report; installation exited **0** at
2026-11-23 23:50 Asia/Shanghai. All **73** checked installed executable, service,
app archive and renderer files matched the candidate byte-for-byte. Receipts:
`build/voice-local-install-result-20261123.json` and
`build/voice-installed-byte-proof-20261123.json`.

The actual installed desktop was reopened and its Home workspace visually
verified through native computer use. Existing conversations and selected model
were present, with microphone and Talk controls. No physical microphone was
opened, no conversation was submitted, and no acoustic success is inferred from
startup. Nothing was committed, tagged, pushed or published by this increment.
