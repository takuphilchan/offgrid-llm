# OffGrid LLM v0.4.13

This release adds managed speech-model acquisition and experimental voice controls
to the shared web/desktop workspace. It also improves cold inference startup and
model-download recovery. It is not the completed Voice program.

## Changes

- Browse speech-recognition and speech-generation categories alongside language
  and embedding models. Search public Hugging Face repositories, review complete
  model packages inline, and download verified artifacts without assembling files.
  Single supported variants need no intermediate selection page; multiple variants
  require an explicit choice. Unknown byte sizes are not fabricated from parameter
  counts.
- Track package downloads, verification, cancellation, resume, repair and removal
  through the shared service. Preserve partial transfers and immutable source
  identities. The CLI and Python client expose matching model-operation helpers.
- Keep speech packages out of chat/embedding selectors and llama.cpp requests.
  Capability and architecture determine the runtime, not filename extensions or
  publisher names. Installed, integrity-checked and runtime-available are distinct.
- Add dictation to Chat and Agents, response read-aloud, and optional bounded
  speech chunks while a new Chat answer streams. Voice settings select ASR, TTS
  and declared voices independently near the composer. Preferences are scoped to
  this account/workspace in this browser; microphone consent is never remembered.
- Preserve draft edits during transcription, cancel pending capture after leaving
  the composer, release late microphone permission results, and keep Stop usable
  during generation. Each answer pins its model/voice across chunks. Explicit
  unavailable selections fail visibly instead of falling back.
- Supervise cancellable Whisper, Piper and reviewed Qwen speech adapters. Reject
  unsupported model profiles and mislabeled audio formats. Managed Piper speaker
  selection is validated. Qwen Base and VoiceDesign are not read-aloud profiles;
  no voice cloning or hidden reference recording is introduced.
- Allow up to 60 seconds for a cold llama-server process to start while retaining
  cancellation and visible loading progress.

## Voice availability and limitations

**Downloading a speech model alone does not enable voice.** Standard release
packages do not yet bundle all required speech runtimes. Dictation and playback
require a matching, available runtime and model in the connected service; the UI
reports missing requirements. Existing compatible Whisper/Piper installations
remain supported. Reviewed Qwen adapters require their separate local runtime.
Model inference never installs dependencies or downloads weights automatically.

Kokoro and streaming Zipformer packages can be acquired, but the executable
sherpa-onnx adapter is not implemented in this release. Qwen CustomVoice acquisition
is supported; its real synthesis was not qualified in the recorded local checks.
Voice control copy currently falls back to English. Full Talk mode, streaming ASR,
durable recording transcription, universal offline speech packs and cross-platform
microphone/latency qualification remain incomplete. Typed workflows remain usable
without speech. No hosted ASR/TTS fallback is used.

Computer Tasks remains a preview with the existing scope and approval boundaries.
Signing, independent security review, model-quality evaluation, soak and pilot
gates are not claimed complete. See the [reliability evidence](../advanced/product-reliability-plan.md).

## Upgrade and recovery

Back up both workspace data and models, stop active work, and update the service
and desktop together. A desktop installer does not upgrade an external Docker
service. Existing model files and conversations must not be deleted to upgrade.

Download history migrates to a versioned shared snapshot with a retained legacy
backup. Interrupted transfers require explicit resume; they do not silently
restart. Do not run an older service against migrated state. Restore a matched
application/data backup if rollback is required; see [workspace recovery](../advanced/workspace-recovery.md).

CPU image: `takuphilchan/offgrid-llm:0.4.13` (AMD64/ARM64).
NVIDIA image: `takuphilchan/offgrid-llm:0.4.13-gpu` (AMD64).
Verify release assets with `checksums-v0.4.13.sha256`.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.12...v0.4.13)
