# Voice playback delivery and turn ownership — 2026-11-22

This increment supports `voice-model-foundation` tasks 14.1, 14.2 and 14.4 and
roadmap M5.08. It is a renderer prerequisite, not full duplex voice qualification
or durable spoken-context reconciliation. The remaining interaction requirements
are recorded in [the voice audit](voice-interaction-audit-2026-11-22.md).

## Implemented behavior

- `apps/web/src/lib/speech-delivery.ts` retains a bounded, immutable snapshot for
  the current playback owner: local playback/request identity, bound session and
  exact canonical input or legacy turn, generated character count, queued and
  synthesized chunks, requested versus confirmed-started playback, completed
  device chunks, discarded work and uncertain partial playback.
- `ResponseSpeech` records whole-chunk completion only on the matching media
  `ended` event. A resolved synthesis request or `play()` promise is insufficient.
  Stop immediately pauses and detaches local media, aborts synthesis, clears the
  lookahead and prevents late transport completions from starting playback.
  Interruption before `play()` resolves retains `playback_requested` evidence;
  it does not claim playback started. Partial text alignment remains unknown.
- Completed device chunks retain the exact prose sent for synthesis. Canonical
  response text remains unchanged; character counts and timers do not infer what
  a person heard. One active chunk, one lookahead and existing input/lag limits
  bound retained delivery data.
- `useChatSpeech` consumes the snapshot to re-arm Talk only for a completed,
  source-bound, current playback owner and current Talk session. Stop and a new
  owner invalidate scheduled completion callbacks. Chat acceptance binds the
  exact canonical input before waking its answer poll; poll callbacks carry that
  input identity. An older answer or failure cannot finish or stop a new owner.
  Linked job responses use the same factory and bind their session, turn and run.
- A completed chunk's delayed `play()` rejection cannot clear the next chunk's
  media handlers. Each play operation owns its own settlement cleanup.
- Talk send settlement is fenced by both session revision and utterance
  submission identity. A failed earlier utterance cannot pause a later utterance
  in the same uninterrupted Talk session.

## Actual checks

All checks used disposable Playwright browser contexts and isolated fixture Vite
on port 5197. No personal browser, microphone, model, audio device or installed
service was used. The new source-hook tests explicitly fake media and transport;
their purpose is deterministic ownership and accounting evidence.

| Check | Result | Retained log |
| --- | --- | --- |
| Same-session utterance failures before submission fencing | 2 expected failures: newer `thinking` became `needs_attention` | `build/talk-utterance-red-20261122.log` |
| Production speech/Talk hooks after correction | 16 passed, 22.0 seconds | `build/speech-delivery-green-20261122.log` |
| Seven existing full Chat voice scenarios | 7 passed, 17.8 seconds | `build/speech-delivery-chat-20261122.log` |
| Full Chat canonical acceptance and answer-poll wiring | 1 passed, 5.9 seconds | `build/canonical-speech-final-20261122.log` |
| Renderer TypeScript | Passed | `build/speech-delivery-typecheck-20261122.log` |
| Handwritten source-size ratchet | Passed | Command output |

The 16 hook cases cover synthesis versus playback completion, interrupted partial
chunks, completed-prefix retention, abort before synthesis, ignored abort with
late audio, old canonical success/failure, deferred completion versus a newer
owner, delayed play rejection, requested-but-unconfirmed playback, playback
failure, restarted Talk sessions and overlapping utterance settlements. Existing
Chat checks cover streaming tail without replay, bounded prefetch with Stop,
committed Talk response/re-listening, new conversation creation, Stop during
generation, pinned model selection and three consecutive replies with silence
recovery. The dedicated canonical Chat case verifies that acceptance binds the
actual input, another input's answer sharing its run does not speak, the matching
committed answer does speak once, and persisted text and typed origin remain
unchanged.

The initial playback harness run (`build/speech-delivery-first-20261122.log`)
failed because dummy WAV bytes were assigned to a real browser media decoder.
That setup was replaced with an explicit fake media sink; it is not evidence of
a product defect or real audio playback. The first canonical fixture run
(`build/canonical-speech-20261122.log`) used synchronous checkbox `.check()` for
an asynchronous readiness toggle; its assertion was corrected to click and wait
for the actual checked state. The following canonical fixture attempt
(`build/canonical-speech-green-20261122.log`) omitted the normal configuration
command response and failed before admission; the fixture now implements that
existing endpoint. Neither setup failure is reported as a product regression.
Existing Vite configuration and color warnings are retained in logs.

## Limits and remaining acceptance

Delivery snapshots are renderer-local and retained for the current/last owner;
they are not durable conversation records and do not rewrite model context.
Audio interrupted before completion can have an unknown audible prefix, even
when the browser has not yet confirmed `play()`. Device completion does not prove
human hearing. No acoustic barge-in detector, echo qualification, simultaneous
capture, active-job spoken steering, semantic endpointing or reconnect protocol
is claimed here. Real ASR/TTS quality, device timing, speaker/headset behavior,
offline packaging, hardware and human comparisons remain unqualified. The full
renderer CI suite and packaged-service acceptance were not run for this bounded
increment. No broad roadmap/task completion follows from these fixture checks.

## Follow-up: deferred job speech ownership

Independent review identified a separate legacy job path: submitting an agent job
retained only its run ID as future speech intent. A user could then select manual
read-aloud, but job completion would start another player over that manual owner.
The full Chat regression reproduced this before the correction: expected only
`Historical answer.`, observed both it and `Deferred job answer.` in synthesis
requests (`build/deferred-speech-red-20261122.log`).

The hook now captures a revocable audio-owner capability before asynchronous
session/admission work. An accepted legacy job retains that capability alongside
its exact session, turn and run only if it remains current. Completion consumes
the intent once and checks both ownership and identity. A new audio owner, the
existing audio Stop event, disabling speech, desktop dismissal, document hiding,
navigation or unmount invalidates the capability. A late admission response
cannot recreate a revoked intent, and turning speech back on does not revive it.
Starting response speech also claims the existing global audio stop event so
manual and automatic playback share the same exclusion boundary. No job control
or cancellation command is issued merely to interrupt audio.

`apps/web/tests/deferred-job-speech.spec.ts` exercises real Chat, manual
ReadAloudButton, HTTP admission, job following and completion with isolated API
fixtures and a fake media sink. The initial corrected run passed all nine cases
in 39.5 seconds (`build/deferred-speech-green-20261122.log`): manual playback,
uninterrupted positive delivery, audio Stop, dismissal, hiding, disable/re-enable,
manual/dismissal during a delayed admission, and navigation. Cases assert that
the job continues without stop/cancel writes. The manual case also verifies its
existing player is not paused when the background job completes. Renderer
TypeScript passed (`build/deferred-speech-typecheck-20261122.log`) and the
source-size ratchet passed. The final combined run passed all 26 tests in 1.0
minute (`build/deferred-speech-final-20261122.log`): nine deferred-job cases, nine
playback-delivery hook cases, seven Talk lifetime cases and one full canonical
Chat case. Maintained documentation (509 files) and scoped whitespace checks
passed. This follow-up does not add acoustic barge-in or change the qualification
limits above.

## Frozen integrated renderer verification

The final playback and deferred-job ownership source was frozen in
`build/voice-ownership-checkpoint-20261122.tar`: 4,270 files, SHA-256
`09628ed474cada25649efb083b45c2440dcec19aa8285d4f068d292676614128`.
Every extracted file was hash-checked before verification. The isolated stage
was `build/voice-ownership-windows-20261122`; it reused installed dependencies
through a stage-local junction and used synthetic service/media fixtures.

- `npm run api:check`: passed (`build/voice-ownership-api-check-20261122.log`).
- `npm run build`: TypeScript and production Vite build passed
  (`build/voice-ownership-renderer-build-20261122.log`). Existing large-chunk
  warnings remain; no size thresholds were changed.
- Playwright, one worker, the complete `voice-playback`, `talk-lifetime`,
  `speech-delivery`, `canonical-speech`, `deferred-job-speech` and
  `speech-packages` files: **86 passed, 3 skipped, 2.8 minutes**
  (`build/voice-ownership-renderer-full-20261122.log`). The skipped cases require
  an explicitly supplied isolated real-service URL. `voice-native` was not run;
  its packaged real ASR/TTS and known-recording prerequisites are separate.
- Independent read-only review of the deferred ownership correction found no
  new defect. This is review evidence, not real-device qualification.

The source-stage dependency junction caused Vite font allow-list warnings in
the test log. Assertions passed, but this is not a visual/font qualification.
The run covers the complete selected voice files, not every renderer test or
the complete CI matrix. It does not close the acoustic, durable-context,
active-steering, model or human gates above. Subsequent interactive-voice work
must obtain its own verification against its changed source.
