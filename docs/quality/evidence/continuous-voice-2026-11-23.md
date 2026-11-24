# Continuous voice integration — 2026-11-23

Status: implementation increment under voice-model-foundation 14.2–14.5;
full interactive voice and acoustic qualification remain incomplete.

## Implementation

Talk now has a stable capture-session identity separate from playback revisions
and individual utterances. Its existing input component owns a persistent stream
through thinking and playback. A bundled AudioWorklet supplies bounded 20 ms PCM
frames; a four-frame message limit makes renderer stalls observable. Sequence
gaps during an utterance fail instead of silently omitting recorded words.

The local energy detector retains 300 ms pre-roll and bounds recordings by both
120 seconds and a 12 MiB PCM16 WAV envelope. Start qualification, end hysteresis
and bounded pause adaptation are separate. This is audio-only endpointing, not
semantic completion or a neural VAD. One ASR/admission transaction and one pending
clip preserve order. Done and Discard act on the current utterance.

Owned onset invalidates playback before any transcription/admission await, and
final text uses the existing canonical conversation path. Session, capture and
utterance identities prevent duplicate or foreign callbacks. A newer onset
suppresses speech from an older pending admission. Speech onset itself does not
cancel background work or authorize tools. Explicit job Stop remains separate.

Echo cancellation and noise suppression are requested and actual track settings
are inspected. Playback interruption requires both reported settings. Their
presence is configuration evidence, not proof of room echo rejection. Missing
AudioWorklet support retains manual recording and Speak again. Unavailable
semantic/VAD models do not trigger cloud fallback or automatic downloads.

## Actual checks

- Six pure PCM checks passed (545 ms), including exact pre-roll samples,
  single finalization, spikes, refused onset, hesitation and bounds:
  `build/voice-continuous-leaf-20261122.log`.
- Real browser AudioWorklet with an injected oscillator stream: one case passed
  in 17.0 seconds (`build/talk-worklet-20261123.log`). No physical microphone was
  opened. It verifies one stream across replies, local playback cancellation
  before a second ASR request, canonical correction and exact track release.
- Combined canonical steering, worker answers, Talk lifetime and worklet run:
  **17 passed in 1.2 minutes**, `build/talk-continuous-final-20261123.log`.
- Initial integration runs retained two fallback failures each: automatic
  rearming hid Speak again, then a delayed timer replaced a newer manual
  recording. Separate fallback eligibility and cancellation of old rearm timers
  correct those ownership paths. Logs `talk-continuous-integration-20261123.log`
  and `talk-continuous-integration-green-20261123.log` retain both failures.

- Broad voice regression: **98 passed, 3 failed, 2 skipped in 9.5 minutes**,
  `build/continuous-voice-full-20261123.log`. The two skips are opt-in
  documentation captures. Two compact-layout failures attempted to resolve
  permission before the asynchronous preflight had requested it; the fixture now
  waits for that observable request. The silence/Stop case unexpectedly observed
  a second permission request; its unchanged isolated rerun passed, so its cause
  remains unresolved. The three-case follow-up passed in 10.6 seconds
  (`build/continuous-voice-followup-20261123.log`). This is not a clean final
  full-suite result.
- Production build/type checking passed. The worklet is emitted as a separate
  same-origin asset (`?url&no-inline`); an inlined data URL would violate the
  service CSP. An owned static server with the service's CSP ran the real
  AudioWorklet tests: **2 passed in 5.5 seconds**,
  `build/continuous-voice-production-csp-final-20261123.log`. Synthetic oscillator
  input and fixture ASR/TTS remain explicitly separate from acoustic evidence.
- Earlier production-CSP runs exposed two UI defects: monitoring displayed the
  idle microphone prompt, and Talk CSS hid Discard during recording. Monitoring
  now displays Listening, and Discard is visible while recording. The passing
  production checks exercise empty recognition and discard without sending or
  replaying a turn. Original failures remain in
  `continuous-voice-production-csp-20261123.log` and
  `continuous-voice-production-csp-green-20261123.log`.
- Expanded production-CSP lifecycle checks: **5 passed in 12.6 seconds**,
  `build/continuous-voice-production-csp-lifecycle-final-20261123.log`.
  Added Stop with late transcription, synthetic device-loss event and playback
  with echo cancellation reported unavailable. They verify release/discard and
  suppression of automatic onset when the required echo setting is absent.
  The first expanded run had four passes and one erroneous test expectation:
  it expected explicit Stop Talk not to stop canonical work. Existing approved
  Stop behavior requires that command; the test now verifies exactly one Stop
  targeting the original input, while retaining zero late submissions. Runtime
  Stop behavior was not changed. The earlier failure is retained in
  `continuous-voice-production-csp-lifecycle-20261123.log`.

## Real native speech engines

`scripts/test/test-native-speech-workspace.mjs` starts a fresh built service with
private disposable state and copies existing verified speech runtimes and model
packages. It downloads nothing, opens no physical microphone and changes no
installed state. The first attempt rejected a valid dotted Whisper package ID;
the harness now accepts the package name characters while still prohibiting path
separators and dot-only path components. The failed attempt is retained in
`build/native-speech-current-20261123.log`.

The corrected run passed known-JFK-recording transcription and native Piper
synthesis. Observed wall times were **12,373.5 ms ASR** and **5,224.8 ms TTS**;
the generated WAV contained 170,772 bytes. These single calls are not a latency
distribution and ran alongside Linux recovery checks. They do not qualify
interactive latency. Whisper used the existing tiny.en package and Piper the
existing en_US-lessac-medium package, with exact revisions and artifact hashes
retained in the fixture's `source.json`.

All **3 browser/package checks passed in 18.8 seconds**, including actual
service transcription, synthesized-audio decoding/playback and isolated package
import/verification/removal. Log:
`build/native-speech-current-retry-20261123.log`. Service binary SHA-256:
`5ea7aa20fb899e0942a290879656c605b5fc91730d1b7612f265298d7febe691`.
Retained private fixture:
`C:/Users/phil/AppData/Local/offgrid-owner-qualification-76e1039b960643b9bf67becce11bf819`.
This qualifies existing batch engine paths, not acoustic interruption, local LLM
conversation quality or a human's naturalness assessment.

## Remaining acceptance

Physical AEC/noise/backchannel/hesitation/device tests, language accuracy,
latency, real ASR/TTS interruption, durable played-context reconciliation,
streaming adapters, explicit unsupported-capability presentation and independent
review remain required. The input pipeline retains batch ASR. The synthetic
oscillator has no acoustic echo path and does not qualify conversational quality.
No OpenSpec task, milestone or overall goal is closed by this evidence.
