# Explicit spoken steering during canonical work — 2026-11-22

This bounded renderer increment advances `voice-model-foundation` task 14.3 and
roadmap M5.08. It builds on [playback ownership and delivery accounting](voice-playback-delivery-2026-11-22.md)
and retains the broader [interactive voice audit](voice-interaction-audit-2026-11-22.md).
Task 14.3 remains open: fixture routing checks do not qualify real models,
reconnect behavior across every lifecycle, or acoustic interaction.

## Behavior and boundaries

The existing canonical inbox already admits finalized input while work is active;
its server-side focus, decision and permission rules own whether an utterance is
a correction, side question, worker answer or another supported input. The voice
renderer now exposes explicit **Speak again** during the thinking phase once
canonical admission is settled. This stops and fences the prior response audio,
then starts a new owned capture cycle. It does not cancel the underlying job.
Legacy streaming request guards remain unchanged. No capture is started merely
because a model is generating or a speaker is playing audio.

A canonical conversation with an assigned agent can explicitly start Talk while
work is active. The existing job Stop remains separately reachable before Talk,
and Stop Talk retains its existing canonical Stop behavior. Choosing Speak again
does not issue Stop, legacy `/generate`, a separate agent-turn request or a new
executor command. Finalized speech uses the same canonical request creation and
retained request keys as other input, with `origin: dictation` and automatic
conversation routing. It cannot accidentally inherit a stale **New work** mode
selected in the typed composer. Current canonical question handling remains in
that existing automatic route; the renderer does not invent a new work target.

Each capture cycle now has an acceptance owner. The first nonempty final
transcript synchronously claims it before invoking send. Repeated final callbacks
before React renders thinking, or a retained callback from an older capture after
Speak again, cannot submit again. Session and utterance settlement fences remain
in place. No semantic deduplication of independently spoken identical sentences
is inferred; separate explicitly re-armed captures retain separate input IDs.

After manual interruption, a late answer for the previous input cannot resume
audio or interrupt the new capture. A matching committed reply may speak and
re-arm listening after device playback completion. Lost input acknowledgements
pause Talk with the transcript and retained canonical request available for
review. Reload restores the draft without automatically resending or acquiring a
microphone. This does not claim exhaustive explicit retry/reconnect qualification.

## Actual checks

Disposable Playwright contexts used isolated Vite on port 5197 with one worker,
synthetic ASR/HTTP fixtures, fake capture and media sinks. No personal microphone,
browser session, installed service, model weights or real audio device was used.

`build/canonical-talk-first-20261122.log`: **12 passed in 22.8 seconds**.

- Two full Chat journeys cover an existing canonical conversation and an assigned
  canonical agent: start Talk during work, finalize a correction, explicitly
  speak again while thinking, deliver the older reply during the new capture,
  finalize a side question, speak only the matching reply and re-arm capture.
- They assert the original focused work/run remains, finalized requests have
  distinct canonical request IDs and dictation origin, no legacy executor path
  is invoked, and no Stop command is sent until the explicit Stop Talk action.
- A lost-acknowledgement case retains the draft and one accepted fixture input,
  then reloads without another submission or microphone request.
- Ending capture before a finalized transcript emits no partial input and still
  uses the normal explicit canonical Stop command.
- Eight hook checks cover session/utterance settlement and synchronous duplicate
  or stale capture callbacks. The older same-session settlement fixture now calls
  `nextCycle()` when simulating playback re-arm, matching the production caller.

`build/canonical-talk-regression-20261122.log`: **16 passed in 34.0 seconds**:
nine playback-delivery ownership cases and seven existing full Chat cases covering
committed Talk playback, new-conversation ownership, Stop during legacy generation,
three replies with silence recovery, repeated empty transcripts, cancellation
during silence re-arm, and a transcription disconnect without retry.

Renderer TypeScript, handwritten source-size, maintained documentation (513 files),
scoped whitespace and translation structural/encoding checks passed. The UI uses
existing localized Talk/Speak again/Stop labels; no new untranslated copy or
acoustic-readiness claim was added. Structural translation checks do not certify
linguistic quality. The full renderer suite was not rerun for this increment.

The initial thinking-button gap and duplicate-callback condition were found
by code inspection; no pre-patch failing test result is claimed for those two.
Existing Vite configuration and color warnings remain visible in test logs.

## Pending-question review correction

Independent review found that automatic voice dispatch could disagree with
earlier checks that still read the typed composer's mode. Both automatic and
stale New work fixtures reproduced a spoken worker answer stranded in thinking
after an applied `worker_answer` receipt: expected a second capture, observed one
(`build/canonical-talk-question-red-20261122.log`, two failures).

Send now derives effective mode and answering-question state once and uses them
for every relevant preflight gate, response-speech creation and canonical
dispatch. An applied worker answer has no foreground reply to play, so its exact
capture owner explicitly re-arms listening after admission cleanup. A reset or
superseding capture invalidates the queued acknowledgement callback.

The first corrected run also exposed a separate phase-notification race: a late
transcribing notification could overwrite thinking after its final transcript
had already been admitted (`build/canonical-talk-question-green-20261122.log`,
one failure and 14 passes). A deterministic isolated reproduction then failed
with expected thinking versus actual transcribing
(`build/talk-phase-red-20261122.log`). Phase updates now require the current,
unclaimed capture owner. These failures were retained and investigated.

Final focused checks passed **16/16 in 30.3 seconds**
(`build/canonical-talk-question-final-20261122.log`): six full canonical Talk
journeys and ten capture/session/utterance hook cases, including both question
modes, late acknowledgement after supersession/End, and the late phase regression.
These are still synthetic capture/API checks, not model or acoustic evidence.

## Remaining acceptance

These fixtures prove renderer ownership and endpoint selection, not real-model
classification or absence of server-side duplicate execution under every fault.
Real ASR/TTS, ambiguity/permission interactions, graph correction and worker
question journeys, process restart and explicit request recovery need integrated
qualification. Acoustic barge-in, echo rejection, simultaneous observation,
adaptive endpointing, partial-utterance continuation and durable played-context
reconciliation remain separate unfinished requirements. No task or milestone is
marked complete by this evidence.
