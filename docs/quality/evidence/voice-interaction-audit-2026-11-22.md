# Interactive voice audit and Talk ownership correction — 2026-11-22

Status: one reproduced renderer lifecycle defect corrected; the interactive voice
program remains incomplete. This evidence supports roadmap M5/M7 and the existing
`voice-model-foundation` change. It does not qualify microphone acoustics, real
speech models, comparative latency, or natural-conversation quality.

## Sources and expected behavior

These are current primary sources, reviewed on 2026-11-22. They provide concrete
interaction patterns, not a certification standard or evidence of OffGrid quality.

- [OpenAI Realtime interruption handling](https://developers.openai.com/api/docs/guides/realtime-conversations):
  stop client playback and discard queued output immediately when speech starts;
  track played duration separately and reconcile conversation context. Do not
  wait for server cancellation acknowledgment before silencing local playback.
- [OpenAI turn detection](https://developers.openai.com/api/docs/guides/realtime-vad):
  semantic endpointing can wait longer for an unfinished thought; interruption
  and response creation are separately controllable. Silence alone is not intent.
- [Pipecat speech input and turn detection](https://docs.pipecat.ai/pipecat/learn/speech-input):
  local Silero VAD supplies activity signals; separate start/stop strategies and
  local Smart Turn handle utterance completion. Speech activity, turn completion,
  and transcription are distinct events. Its local components are candidates for
  qualification, not approved additions or permission to download weights.
- [LiveKit turn handling](https://docs.livekit.io/agents/logic/turns/):
  supports explicit and detected interruptions, separate user-turn commit/discard,
  and false-interruption recovery. Tool cancellation needs its own semantics.
  Some advanced detectors use cloud services; an SDK default is not an acceptable
  implicit cloud fallback for OffGrid.
- [Meta's Muse voice announcement](https://about.fb.com/news/2026/11/introducing-muse-ai-voice-expressive-avatars/)
  describes real-time conversation and interruption. This confirms a current
  Muse product reference; [Meta AI/Muse Spark](https://about.fb.com/news/2026/04/introducing-muse-spark-meta-superintelligence-labs/)
  also describes topic and language switching. Neither announcement publishes
  enough implementation or benchmark detail to infer equal performance locally.

## Observed OffGrid behavior and gaps

| Area | Actual code and behavior | Remaining requirement |
| --- | --- | --- |
| Voice ownership | `apps/web/src/features/chat/useTalkMode.ts` owns session revision, preflight, silence re-arm and transcript submission. Delayed send settlement previously checked only whether *some* Talk session was active. | Corrected below. Retain revision fencing across every future capture, transcription, playback and reconnection callback. |
| Barge-in | `ChatPage.tsx` mounts `VoiceInputButton` only in listening/transcribing states; `TalkSurface.tsx` offers Speak again only while speaking. The old design explicitly disables capture during playback. | Speech cannot currently interrupt speech. Add a continuous, explicitly owned input observer with echo safeguards, an immediate local output cutoff, and bounded captured pre-roll. Do not merely enable the current recorder beside the speaker. |
| Thinking and active work | `startTalk` excludes selected agents, unsettled linked work and busy requests. `speakAgain` rejects busy state. | Accept spoken corrections/side questions during thinking and existing work through the canonical conversation inbox. Preserve background work and the distinction between interrupting presentation, steering work, and stopping effects. |
| Canonical input | Chat submits final transcripts through existing request paths. Canonical admission uses `origin: dictation`, retained command envelopes and the existing input mode. `useCanonicalConversation.ts` waits for the matching completed reply. | Preserve those request IDs and deduplication. Introduce stable utterance/session provenance and explicit partial/final ownership; do not add a parallel voice agent or replay ambiguous accepted inputs after reconnect. |
| Heard versus generated | `response-speech.ts` advances `consumed` when scheduling synthesis, not when audio is heard. Stop clears the player/queue but publishes no played-duration or interrupted-delivery receipt. The conversation retains the full generated answer. | Keep durable answer/effect evidence intact, while adding separate playback-delivery metadata so the assistant knows which speech was actually delivered. Do not pretend queued text was heard or silently erase canonical work history. |
| Pauses and noise | `speech-pause.ts` uses RMS levels, 180 ms activity and 1,050 ms silence, sampled every 90 ms. It is a level gate, not a speech/turn-completion classifier. `VoiceInputButton` requests `audio: true`, with no explicit echo/noise configuration or verified settings. | Distinguish brief acknowledgments, coughs, far-end echo, background voices and incomplete thoughts. Qualify requested/actual device processing, local VAD and adaptive endpointing. Existing browser defaults are not acoustic evidence. |
| Partial utterances | `VoiceInputButton` collects MediaRecorder chunks, then decodes a full WAV and submits ASR only after stop; recording is bounded at 120 seconds/12 MiB compressed input. | Support bounded pre-roll/continuation and explicit commit/discard. A natural thinking pause must not force a partial instruction into work. Streaming ASR is a separate runtime capability, not established by a catalog entry. |
| Response latency | Talk intentionally waits for committed text; ordinary Speak responses can synthesize provisional chunks. Canonical reply polling adds up to its one-second interval. `PackageRuntime.withEngine` serializes speech requests and changes its single engine when ASR/TTS package identity changes. | Measure endpoint-to-transcript, transcript-to-first-text, first-playback and interruption latency, including cold/warm resource contention. Retain conservative resource limits while enabling bounded committed speech progress; no unsupported latency claims. |
| Stop/reconnection | Capture owns its recorder/tracks/controller; late permission is released. Playback Stop pauses and clears the player synchronously. Managed worker cancellation stops its owned worker and settles the protocol reader. Errors pause Talk; silence re-arms. | Test lost acknowledgment, repeated disconnect/reconnect, device removal, sleep/wake, tab backgrounding and late ASR/TTS replies. Reconnection must reconcile canonical receipts and must not automatically resubmit or replay speech. |
| Privacy/offline | Ready-package selection rejects unavailable explicit choices. ASR/TTS use the configured OffGrid service, with bounded request bodies and transient files; no cloud fallback is introduced by this fix. Transcripts enter the ordinary draft/conversation stores. | Explain service location: a remotely configured OffGrid service receives audio. Qualify network-off operation, temp-file cleanup/crash retention, transcript retention, device consent and packaged runtime dependencies separately. |

The existing tests explicitly assert that capture does not run during speaker
playback. Those tests establish the former half-duplex behavior, not natural
barge-in. The new user direction requires updating the old scenario in both
`voice-model-foundation` and the relevant workspace-composition specification;
do not retain contradictory acceptance owners or silently count the old test as
proof of the new behavior.

## Reproduced defect and bounded correction

The actual React hook regression starts Talk, submits an unresolved utterance,
resets and restarts Talk, submits a new utterance, then settles the old send as
false. Before the fix, the new session changes from `thinking` to
`needs_attention` and loses active ownership. This is a session-lifetime defect,
independent of audio/model quality.

`acceptTranscript` now captures its originating revision. A false result or
rejected send promise can pause only that still-active revision. Old success has
no side effect. Current failures still pause for review; no callback resubmits
the transcript. Canonical request construction, IDs and admission are unchanged.

The isolated Playwright harness mounts the production hook with actual React DOM.
Only readiness and send completion are fixtures; it opens no microphone, plays no
audio, accesses no installed service and downloads no model. Its generated HTML
uses the existing Vite React preamble and dependency modules.
It explicitly skips under `OFFGRID_E2E_URL`, where the packaged service does not
serve development source modules; packaged-service voice qualification remains a
separate gate. No case was skipped in the Vite runs recorded below.

| Check | Actual result | Evidence |
| --- | --- | --- |
| Old-code, correctly mounted late-false regression | FAIL: expected `thinking`, received `needs_attention` | `build/talk-lifetime-red-confirmed-20261122.log` |
| New-code hook browser suite | PASS: 5 cases, 11.2 seconds | `build/talk-lifetime-green-20261122.log` |
| Cases | Old false/rejection/success cannot affect restarted session; current false/rejection still pause; transcript submission count remains unchanged; no unhandled rejection | `apps/web/tests/talk-lifetime.spec.ts` |
| Final bounded renderer browser selection | PASS: 8 cases, 20.4 seconds; includes the five hook cases plus existing generation Stop, silence-delay Stop and transcription-disconnect behavior | `build/talk-lifetime-focused-final-20261122.log` |
| `npm run check` in `apps/web` | PASS | `build/talk-lifetime-renderer-check-20261122.log` |
| Documentation/source-size/scoped whitespace | PASS: 507 maintained Markdown files, no documentation problems; source-size ratchet clean | Tool output |

The final browser selection was:

```powershell
$env:OFFGRID_FIXTURE_PORT='5197'
npx playwright test tests/talk-lifetime.spec.ts tests/voice-playback.spec.ts --workers=1 --grep 'old Talk|current Talk|Stop Talk during generation|Stop during the silence|a real transcription disconnect' --output=test-results-talk-lifetime-final
```

The complete browser suite, production renderer build, packaged-service suite,
installed-platform checks and physical acoustic tests were not run for this
bounded correction. Existing Vite startup emitted its configuration-loader and
color-environment warnings; no test failure was suppressed.

Two earlier harness attempts failed before mounting because of a CommonJS
dependency export and then a missing Vite React refresh preamble. Logs
`talk-lifetime-red-20261122.log` and `talk-lifetime-red-valid-20261122.log` are
retained; despite the second filename, neither proves the product defect.

## Dependency-ordered remaining implementation

1. Finish exact voice-session ownership and capture/output generation identifiers.
   Cover reset, Stop, unmount, old replies, queued audio and late permission.
   Persist no automatic microphone consent. This correction is one part only.
2. Add playback delivery accounting and a common interruption operation: silence
   local audio immediately, abort queued synthesis, record the played boundary,
   then reconcile the owned response. Separate audio interruption from job Stop.
   Keep an explicit push-to-talk/manual interruption fallback available.
3. Extend the existing microphone owner for simultaneous bounded observation and
   playback, with requested/observed echo cancellation and local speech detection.
   Preserve pre-roll so the interrupting word is not lost. Qualify speaker and
   headset modes before claiming full duplex; noise-only input commits no turn.
4. Admit finalized utterances through the existing canonical inbox while a reply
   or job is active. Route corrections, side questions and explicit Stop through
   existing control semantics and revision fences. Never treat speech activity or
   an ASR hypothesis as approval for an external effect. Preserve context and
   pending question/answer identities across interruptions.
5. Add adaptive endpointing and partial-utterance continuation, qualifying local
   VAD/turn models against fixed bounded fallback behavior. Integrate streaming
   ASR/TTS only where actual adapters support it; coordinate warm pools and resource
   scheduling instead of removing existing limits. Honor language switching.
6. Run waveform, integration, acoustic and human conversation qualification.
   Cover cough/backchannel versus intentional interruption, near/far speakers,
   room echo, quiet/short speech, long pauses, overlapping speech, long utterances,
   correction during tool work, unknown effects, disconnect after acceptance,
   sleep/wake, language switching, accessibility and offline operation. Report
   false-interruption rate, missed barge-ins, lost words, duplicate inputs,
   heard-context mismatch and p50/p95/p99 latency. Match hardware/model conditions
   for product comparisons and retain failed cases. Synthetic hook tests cannot
   substitute for these acoustic and participant gates.

No roadmap or OpenSpec task is marked complete by this report. No user microphone,
personal session, live installation, model acquisition, commit or publication was
performed for this increment.
