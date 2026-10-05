# Proposal

## Why

The approved OffGrid Voice program needs interchangeable local speech models
that people can find and download as easily as language and embedding models.
The implemented import-only foundation puts package assembly on users; it is not
the intended model-management experience and does not complete this change.

## What Changes

- Preserve the implemented versioned manifests, atomic local import, verification,
  leases, guarded removal, and positive chat-model selection boundaries.
- Make Hugging Face discovery and curated downloads the primary speech acquisition
  path inside the existing Models page. Add Language, Embeddings, Speech recognition,
  and Speech generation filters; move folder import into advanced/offline controls.
- Resolve supported Whisper, Piper, streaming Zipformer, Kokoro, and reviewed
  Qwen3-ASR/Qwen3-TTS layouts into
  complete packages automatically. Users select a model/variant, not individual
  dependencies or a handwritten manifest. Unsupported layouts explain the reason.
- Extend the shared model/download services for pinned multi-artifact transfers,
  aggregate progress, cancellation, explicit resume, restart recovery, atomic
  activation, repair, and safe removal. Do not introduce a second speech downloader.
- Add typed v2 catalog, resolution, and durable operation contracts used by the
  shared renderer and service-backed clients. Retain existing model IDs and v1
  language/embedding behavior, including partial-download recovery.
- Explain total download/staging space, source, license, and runtime requirements
  before starting. Distinguish installed, integrity checked, runtime compatible,
  smoke-tested, and workflow-qualified states; downloading does not enable voice.
- Record actual tests in the existing reliability plan. Reopen this change's
  implementation status rather than treating import-only evidence as completion.

## User-visible acceptance

- A user can find a supported speech model through Models, review its variant,
  complete download size, license, and readiness, then choose Download without
  leaving OffGrid, collecting local files, or using a terminal.
- Every required artifact is obtained from pinned sources and checked. Partial,
  corrupt, or incompatible packages never appear as usable models. Navigation or
  refresh does not lose a download; interruption has a working recovery action.
- Installed speech models appear under their correct categories with Verify,
  Repair, and Delete, not in chat selectors. Existing language/embedding search,
  installs, selections, and recovery continue working.
- A missing speech runtime is explained before download and after installation.
  The UI never labels a package ready for dictation/Talk merely because it exists.

## Boundaries and subsequent stages

This change completes model acquisition and package management, not runtime pack
installation, real ASR/TTS execution, dictation/read-aloud, Talk, agent voice,
recording/Knowledge workflows, or language/hardware qualification. Those remain
required, ordered stages of the approved Voice program, not waived requirements.
Offline pack distribution and legacy speech-runtime migration also remain in that
program; compatible local-folder import is retained here as an advanced fallback.

Development tests use isolated state and synthetic HTTP fixtures. Real model
downloads require explicit approval and isolated storage. This revision authorizes
no microphone capture, installed-data mutation, deployment, commit, or publication.

## Capabilities

### Authorized correction to the original acquisition-only boundary

On 2026-10-05 the user additionally requested repair of response speech, correct
routing of non-Qwen speech models, and reduced playback delay. The bounded
corrective implementation in design/tasks section 9 addresses those existing
paths: architecture selection, leases, worker protocol/cancellation, readable
runtime errors, page media policy, and bounded read-aloud playback. It does not
waive the broader runtime-pack, sherpa, Talk, or real-model qualification gates.
The user subsequently authorized necessary compatible-model downloads and speech
during response streaming. Section 10 covers explicit playback opt-in, bounded
incremental speech and interruption, without replaying restored turns. Microphone
capture and voice cloning remain excluded. Deployment remains separately
authorized, never implied by specs.

### New Capabilities

- `model-packages`: Discovery, pinned acquisition, integrity-checked installation,
  recovery, and lifecycle of typed multi-artifact models through shared model
  management, with compatibility and lease-aware deletion.

### Modified Capabilities

None; this repository has no pre-existing OpenSpec capability artifacts.

## Impact

Model registry and package service, Hugging Face discovery, shared download state
and recovery, API/client contracts, React Models and selectors, CLI/Python clients,
regression tests, and maintained discovery/model/API/reliability documentation.
Existing LLM and embedding files remain in place; download-state migration must
preserve originals and compatible API projections. No parallel model catalog,
conversation store, or voice-specific task history is introduced.
