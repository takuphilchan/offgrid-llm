# Dictation and spoken responses

Use voice to draft a message or agent task, or listen to a Chat answer. These
controls are experimental and require a compatible speech runtime and model in
the connected OffGrid service. Typed Chat and Agents work without speech.

The development renderer groups **Voice settings** beside the selected language
model below the Chat/Agents prompt. **Speak responses** stays beside Chat's
microphone and Send/Stop controls; changing voice settings does not start capture
or playback. This placement requires a matching rebuilt renderer, not merely a
service restart. Released builds may retain the earlier composer layout.
This guide does not describe the planned hands-free Talk mode.

## Before you start

Three model roles are independent:

| Role | Used for | Where to select it |
| --- | --- | --- |
| Chat/language model | Producing answers and planning agent work | Model selector beside the Chat or Agents prompt |
| Speech recognition (ASR) | Converting a recording into text | Voice settings → Recognition model |
| Speech generation (TTS) | Reading an answer aloud | Voice settings → Speech model |

Dictation needs ASR; playback needs TTS. A speech model does not replace the
language model and should not appear in its selector. You can use dictation
without playback, or playback without a microphone.

Check which service owns your workspace before installing anything. A model in
your Windows profile is not automatically installed in a Docker volume. See
[workspace selection](getting-started.md#check-which-workspace-you-opened).
The browser captures audio on your device; inference runs in the connected
service. If that service is on another computer, audio and synthesis text travel
to that computer. Do not describe that configuration as on-device inference.

## Download and select speech models

1. Open **Models → Speech recognition** for dictation, or **Speech generation**
   for playback.
2. Find a catalog entry or explicitly search Hugging Face by model/publisher.
3. Choose **Review download** on the result. Review size, required disk space,
   license and runtime availability there. Choose a variant only when the
   repository has multiple supported packages.
4. Choose **Download** in the same card. Wait for installation and verification;
   receiving 100% of the bytes is not the final step.
5. Open **Voice settings** inside the Chat composer footer or beside the
   microphone in the Agents task composer. Select recognition and speech models
   independently. If the chosen TTS profile offers voices, select a voice too.

There is no need to collect individual files for a supported online package.
Folder import is an advanced offline option. See [download recovery](model-discovery.md#speech-models-from-hugging-face)
for cancellation, resume, repair and removal.

**A model download does not install its executable runtime.** Standard release
packages do not yet bundle every speech dependency. If the runtime is missing,
re-downloading the weights will not fix it. Check the installed package/profile
and the connected service's runtime before retrying.

| Model family | Execution in this implementation |
| --- | --- |
| Whisper | Requires compatible whisper.cpp runtime and weights |
| Piper | Requires compatible Piper runtime, ONNX weights and configuration |
| Qwen3-ASR | Requires the reviewed local Qwen ASR runtime and complete package |
| Qwen3-TTS CustomVoice | Supported read-aloud adapter; requires its local runtime; real synthesis is not qualified by the recorded local checks |
| Qwen3-TTS Base / VoiceDesign | Not supported for response read-aloud; Base requires reference audio, and OffGrid does not introduce voice cloning |
| Kokoro / streaming Zipformer | Package acquisition is supported; the executable sherpa-onnx adapter is not implemented |

Search results and downloadable packages are not promises of execution support
or model quality. The [reliability plan](../advanced/product-reliability-plan.md)
owns measured qualification evidence.

**Automatic** uses an available profile. An explicit selection that is removed
or becomes unavailable produces an error instead of silently changing models.
Selections are saved for the current account/workspace in this browser, not
synchronized across devices. Each recording or answer keeps its selected model
revision and voice until it finishes; preference changes affect the next one.

Click outside the settings panel or press **Escape** to close it without losing
selections. Escape restores keyboard focus to **Voice settings**.

## Dictate into Chat or Agents

1. Open a Chat conversation or a new Agent task.
2. Choose **Use microphone**. OffGrid checks ASR availability before requesting
   microphone permission. Allow the browser/OS permission when prompted.
3. Speak a short message, then choose **Stop recording**. Capture automatically
   stops after two minutes; use shorter recordings while testing.
4. Wait for transcription. The text is appended to your current draft, preserving
   typed edits made while recognition was running.
5. Review names, numbers and intent. Edit the draft, then choose **Send** in Chat
   or **Start task** in Agents yourself.

Dictation does **not** submit a task automatically, grant computer access, or
approve an action. Agent permissions and approvals still apply. Saying “yes”
into a recording is not an authorization mechanism.

While a recording or transcription is active, the microphone control can stop or
cancel that work. Before capture it distinguishes **Checking voice availability**
from **Waiting for microphone permission**, with **Cancel microphone setup**
available in both phases. After capture, **Transcribing** has a separate **Cancel
transcription** action; it is not labelled as recording. These setup/cancellation
labels are localized in all nine interface languages.
Changing composer context or leaving the page cancels pending
capture/transcription and releases microphone tracks, including late permission
results. This does not cancel an already-submitted agent task.

## Listen to Chat answers

For an existing answer, choose its **Read aloud** control. It shows preparation
while the runtime loads/generates audio; use **Stop reading** to cancel.

For new answers, enable **Speak responses** in the composer before sending.
This checks TTS readiness and speaks bounded chunks as answer text arrives. The
setting lasts only for the open Chat page; it does not replay history or remain
enabled after leaving and returning.

Complete sentences can play before the answer is saved, so that speech is
provisional. The final incomplete fragment waits for committed completion.
Text and citations remain authoritative. Playback uses cleaned visible prose,
not raw Markdown or executable code.

| Control | Effect |
| --- | --- |
| Stop recording / cancel transcription | Ends microphone work; does not stop an accepted task |
| Stop reading / stop speaking | Stops playback and pending synthesis; does not undo a tool action |
| Chat Stop | Stops audio and requests cancellation of text generation |
| Leave Chat | Stops playback and pending capture; does not silently cancel accepted background tasks |
| Agent task controls | Pause, resume or stop the actual durable task according to its current state |

This is chunked speech during text streaming, not native audio-token streaming.
At most one speech chunk is prefetched. If playback falls too far behind, audio
stops with an explanation while text generation continues. Cold model loading
and CPU synthesis can still be slow; interactive latency is not guaranteed.

## Troubleshoot without deleting your models

| Symptom | Next step |
| --- | --- |
| Installed model is unavailable in Voice settings | Inspect its runtime/profile issue; installed bytes and runnable inference are different states |
| Base model cannot read answers | Select a compatible direct-speech model, not Qwen Base or VoiceDesign |
| No microphone permission prompt | Check ASR readiness first; then browser/OS microphone permission and whether capture is available in this context |
| Microphone works on localhost but not a remote HTTP page | Use a correctly secured deployment; do not disable browser security to enable capture |
| Transcription works but no answer appears | Select a working chat model, inspect the generation error, and use request-status recovery before resending |
| Text arrives but there is no sound | Enable Speak responses before sending, or use Read aloud; check TTS availability, playback errors, output device and system volume |
| Model selection disappeared | Confirm the service/workspace and browser account, then explicitly reselect an available model |
| Speech preparation is slow | Try a short answer; check memory contention and CPU runtime suitability, not only download size |
| Model removal says it is in use | Stop playback/transcription and wait for the runtime lease to release; do not manually delete managed files |
| Qwen API request fails with `language=en` | Automatic language detection worked in the recorded smoke test; omit the optional language override. This adapter limitation is not fixed by this UI release |

For a read-only service diagnosis, inspect **GET `/v1/audio/status`** using your
usual authenticated client. `profiles[].available` describes runtime availability;
`smoke_tested` records a successful request in the current service lifetime, not
accuracy or hardware qualification. See the [speech API contract](../reference/api.md#experimental-speech-compatibility-endpoints).

Keep credentials, private audio and transcripts out of support reports. Include
the OffGrid version, client/OS, service location, selected package revision,
runtime and exact safe error text instead.

## Current boundaries

This preview does not provide always-on listening, wake words, full-duplex Talk,
voice-only consequential approvals, agent progress narration, or the planned
recording-review-to-Knowledge workflow. File transcription APIs do not establish
that those UI workflows exist. Voice preferences, capture and playback feedback
use all nine interface locales; technical model identities and service-provided
diagnostics retain their original text. Interface locale alone does not establish
recognition-language support or native-speaker translation review.

Speech inference does not silently fall back to a hosted provider or download
code/models during inference. Model acquisition can use the network explicitly.
The preview's short-lived microphone processing is not an archival recording
feature; the resulting draft and any submitted text follow normal conversation
and task persistence. Keep sensitive speech out of a workspace you do not trust.
