# Choose and manage models

Start with [model discovery and download recovery](model-discovery.md) for the
web/desktop installer. This guide explains model roles, storage, and verification;
it is not a ranked list of current model recommendations.

## Choose for the workflow

| Workflow | What the model/runtime must support |
| --- | --- |
| Conversation or drafting | Instruction/chat generation and the correct template |
| Document retrieval | A separate compatible embedding model, pooling, and dimensions |
| Agent tools | Reliable structured tool calls, not just fluent chat |
| Managed-browser images | Compatible model/projector pair, image transport, and preflight |
| Native desktop control | The implemented driver and typed tools; a vision model does not add native capture |

Check the publisher's model card, license, language coverage, and architecture.
A filename, parameter count, or successful download is not compatibility evidence.
Evaluate tool calling and useful outcomes separately from text generation.

## Fit memory and storage

Model file size is only part of memory use. Context, KV cache, image processing,
runtime buffers, and the operating system also need room. Increasing context or
loading embeddings alongside chat can change what fits.

Quantization trades weight size against accuracy; a smaller quantization is not
automatically suitable for tool calling. Start with a modest context and check
actual runtime allocation/offload. See [performance](../advanced/PERFORMANCE.md)
and [CPU support](../advanced/cpu-support.md). Hardware recommendations must stay
within the [recorded qualification profiles](../advanced/product-reliability-plan.md).

## Download into the correct workspace

With the intended service running:

```sh
offgrid list --catalog
offgrid download phi-3.5-mini-instruct
offgrid list
```

The catalog example is not a promise of agent or vision quality. For other models,
use **Models → Find more models** or the [Hugging Face guide](huggingface.md).
Choose an exact supported GGUF file. Split weights/projectors need their companion
files; the installer does not treat a single shard as a complete model.

Use the installed ID returned by the service. Native default storage is
`~/.offgrid-llm/models`; containers normally use `/var/lib/offgrid/models`.
`OFFGRID_MODELS_DIR` selects the service's actual root. A second empty model list
may mean a different workspace, not a failed download.

## Verify what you installed

1. Wait for **Ready**, not merely 100% transferred.
2. Inspect the installed model entry and file identity.
3. Run a short chat request and check model/runtime errors.
4. Test the intended embedding, tool, or image workflow separately.
5. Retain source, license, checksums, runtime revision, template, context, and
   hardware settings when recording reproducible results.

The registry can recognize some legacy file extensions, but recognition is not
a guarantee that the current llama.cpp runtime can load an old GGML/BIN format.
Use a compatible GGUF for current workflows; do not rename an incompatible file.

## Offline transfer and backups

### Speech package foundation (development)

The shared model contract distinguishes four capability categories: `language`,
`embeddings`, `speech_recognition`, and `speech_generation`. An installation
target is explicitly either `legacy_file` or `package`; its filename is not a
runtime decision. Legacy chat/embedding IDs and the v1 catalog projection stay
unchanged. Speech packages are never offered as chat or embedding choices.
Catalog provenance, downloaded-byte integrity, runtime compatibility, smoke
testing and qualification are separate evidence. Catalog presence alone proves
none of the latter states. The shared Models workspace exposes all four categories.

Acquisition preflight reserves complete staging, publication/recovery space and
any preserved prior package, with 16 MiB additional metadata headroom. Partial
bytes are not counted as trusted space savings. `insufficient_space` reports
required and available bytes; `space_unavailable` requires repairing storage
access. `unsafe_source`, `source_identity_mismatch`, `incomplete_listing`, and
`missing_artifact_identity` stop resolution before installation. Private/gated
sources return `public_access_required` rather than prompting for URL tokens.
An absent speech runtime is reported separately: package download may be useful
for preparation/offline transfer, but does not enable speech in this build.

Speech models use versioned multi-file manifests, not renamed GGUF downloads.
Normally choose a speech category, discover a Hugging Face package and select
**Review download → Download**. See [discovery and recovery](model-discovery.md).
For compatible offline/local packages, expand **Advanced · offline import** and
select a folder containing `manifest.json`
and exactly its declared artifacts, review the name/revision/size, then choose
**Import package**. Missing files and size mismatches are reported before upload.
The service checks digests before activation. **Verify** checks managed files;
**Delete** asks for confirmation. Cancelling an in-flight upload aborts transfer;
refresh the inventory if an acknowledgment was lost near completion.

The model package storage code accepts Whisper, Piper, streaming Zipformer,
Kokoro, and reviewed Qwen3-ASR/Qwen3-TTS declarative layouts. This is
**storage validation**, not a claim that their speech runtimes, model quality,
or voice interfaces are implemented or qualified.

Schema `1` requires `id`, immutable `revision`, `name`, `architecture`,
`runtime: {adapter, revision}`, explicit `capabilities`, `languages`,
`sample_rates`, `license`, and `artifacts`. Each artifact declares a relative
`path`, `role`, positive byte `size`, lowercase `sha256`, and its own `license`.
Optional source metadata must pin a full repository commit, not `main`.
Synthesis packages also declare `voices: [{id, language}]`. Optional hardware
evidence references do not automatically establish qualification.

| Architecture | Adapter | Required artifact roles | Declared capabilities |
| --- | --- | --- | --- |
| `whisper` | `whisper.cpp` | `weights` | `transcription` |
| `piper` | `piper` | `weights`, `config` | `speech_synthesis` |
| `zipformer-streaming` | `sherpa-onnx` | `encoder`, `decoder`, `joiner`, `tokens` | `transcription`, optional `streaming_recognition` |
| `kokoro` | `sherpa-onnx` | `weights`, `voices`, `tokens` | `speech_synthesis` |
| `qwen3-asr` | pinned OffGrid Qwen3-ASR adapter | one or more `weights`, one or more `config` | `transcription` |
| `qwen3-tts` | pinned OffGrid Qwen3-TTS adapter | one or more `weights`, one or more `config` | `speech_synthesis`, `incremental_synthesis` |

Packages are copied into the reserved `.packages` model subtree. Local imports
do not run scripts or download runtimes. They reject undeclared files, links,
unsafe paths, incomplete layouts, incorrect sizes, and digest mismatches.
Limits are 1 MiB of manifest metadata, 2,048 artifacts, and 32 GiB total artifacts.
An import activates only after all files verify; existing revisions cannot be
overwritten. A new revision preserves the old one. Source files are untouched.

Integrity is distinct from runtime compatibility, smoke testing, and qualification.
A locally supplied hash proves identity, not trusted origin. Verification checks
the managed bytes again; restarting clears in-memory verification evidence.
Runtime consumers must hold a package lease, which prevents removal until the
work releases it. Removal is confined to that package revision, not recordings or
user documents. Interrupted staging is not an installed model. Source-bound
downloads support durable cancel/resume and exact-revision repair. Runtime/offline
distribution packs, legacy speech migration, and user-facing voice workflows
remain later stages of the approved Voice program.

### Speech runtime selection and read-aloud

Speech downloads and speech execution are separate. The service routes a managed
package using its declared architecture, not its suffix, publisher, or ASR/TTS
category. Whisper packages use whisper.cpp, Piper packages use Piper, and Qwen
packages use the matching Qwen adapter. Each requires its installed runtime.
An explicit model selection never silently falls back to a different package.
Kokoro and Zipformer packages can be stored, but this build still lacks their
executable sherpa-onnx adapter; it reports that limitation rather than sending
their files to Qwen. Installing a package never installs executable dependencies.

For Qwen response playback, select **CustomVoice**, not **Base** or **VoiceDesign**.
Base needs reference audio; read-aloud does not clone a voice or select a hidden
reference recording. Installed incompatible packages are retained and explained.

The response's **Read aloud** control prepares short WAV chunks, displays progress,
and permits cancellation before or during playback. It prefetches at most one
chunk. Navigating away stops playback and pending synthesis. Text remains the
authoritative response; this is not automatic Talk mode or native streaming TTS.
Cold model loading and CPU synthesis can still be slow; interactive latency and
voice quality have not been qualified. No response speech is generated until the
user requests it.

In Chat, turn on **Speak responses** before sending to hear new answers while
their text arrives. This opt-in lasts only for the open Chat page, never replays
history, and checks for a compatible installed voice first. Complete sentences
are synthesized with at most one look-ahead chunk. The last incomplete fragment
waits until the response is saved. Speech heard before saving is provisional.
**Stop reading** stops only audio; Chat's **Stop** stops audio immediately and
requests cancellation of generation. Leaving Chat stops audio. If speech falls
too far behind, it stops with an explanation while text generation continues.
This is chunked speech during text streaming, not native audio-token streaming
or the full Talk/turn-taking feature.

**Voice settings**, beside the Chat composer and in the Agents task composer,
selects recognition and speech models independently. Only runtime-available
profiles can be selected. Automatic selection shows the available default;
an explicitly selected model that becomes unavailable produces an error rather
than silently switching. Where the selected TTS profile declares voices, choose
one there. Settings are saved for the current account/workspace in this browser,
not synchronized between devices. They do not remember microphone permission
or enable automatic playback. Each recording or answer pins its model revision
and voice; changes take effect on the next recording/playback, not mid-answer.

Speech controls now sit immediately above the Chat composer. Dictation preserves
edits made while recognition is running. Stop remains usable during generation;
changing composer context or leaving the page cancels pending capture and
transcription, including a microphone permission request that resolves late.

In speech-model search, **Review download** expands transfer size, required space,
licenses and runtime warnings inside the result card; **Download** starts the
transfer there. A single supported variant is resolved directly. Repositories
with multiple supported variants require an inline choice, never an arbitrary
first variant. Keyword search queries Hugging Face rather than a Qwen-only
shortlist. Unknown transfer sizes stay unknown: parameter counts are not bytes.

One speech package is kept loaded for short follow-ups and released after 30 idle
seconds. Switching packages releases its process and lease before loading the
next one. Removal/repair of a leased package is refused; wait for idle unloading.
Runtime availability is not a passed inference test. A successful request records
an in-memory smoke result for that revision, never hardware/language qualification.

### Existing model transfer

Prepare the matching application, UI, runtime, models, and supporting files on a
connected machine, verify digests after transfer, and test with networking off.
See [offline deployment](../advanced/DEPLOYMENT.md#prepare-an-offline-installation).

Workspace backup covers the data root, not automatically the separate model
directory. Preserve models and licenses separately. Local-file import/export
commands must use the intended host paths; they do not magically copy into a
different container's volumes. Do not overwrite a model currently in use or
delete partial download files while the worker owns them.

## Troubleshoot

| Symptom | Next check |
| --- | --- |
| Model absent | Confirm service address, workspace, installed ID, and completed download |
| Loading fails | Check architecture, runtime revision, file integrity, and available memory |
| Chat works but tools fail | Check template and structured tool calling; reasoning style is not a repair |
| Knowledge will not enable | Select an embedding model, not a chat model; inspect index/runtime compatibility |
| Interrupted download | Resume the same source/file through Models; do not replace partial bytes with another model |
| GPU detected but slow | Verify actual offloaded layers and context allocation, not just device detection |
| File copied manually but not listed | Inspect the service's model root; a deliberate rescan/restart may be needed |

Keep installed files and sources when investigating; deleting the whole models
directory is not routine repair. For CLI/local-path differences, see
[CLI reference](../reference/cli.md#terminal-chat-and-local-model-files).
