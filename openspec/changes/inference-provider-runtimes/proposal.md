# Proposal: Provider-neutral local inference runners

## Why

OffGrid's agent, authorization, and recovery layers are provider-neutral, but
the inference lifecycle still assumes a llama.cpp server and GGUF metadata.
That prevents MLX/safetensors and Transformers/PyTorch runtimes from becoming
first-class local models and makes CPU, CUDA, Metal, and MPS readiness harder
to report truthfully.

## What changes

- Add a supervised provider contract for model loading, streaming, cancellation,
  health, memory/device reporting, and normalized tool decisions.
- Keep the current llama.cpp path as the default compatibility provider.
- Add pinned local MLX and Transformers worker providers behind explicit
  readiness and qualification gates.
- Extend model/package metadata additively with engine, format, backend,
  runner/adapter revisions, modalities, and readiness evidence.
- Move admission and lifecycle decisions behind one provider-aware scheduler.
- Preserve existing profiles, tasks, APIs, GGUF models, CPU/GPU behavior, and
  stored state without automatic downloads or cloud fallback.
- Qualify text inference first; keep VLM/audio execution unavailable until
  model-specific preprocessing and tool-call evidence exists.

## Boundaries

This change owns inference provider lifecycle and model/runtime contracts. Agent
delegation, native computer workflows, document editing, and visual safety stay
in their existing changes. No public API removal, storage reset, automatic
runtime installation, arbitrary remote code, deployment, or release publication
is included.

## Acceptance

- Existing llama.cpp models and clients behave unchanged.
- MLX and Transformers workers use the same typed local protocol and normalized
  response gate.
- Provider selection, cancellation, crashes, memory limits, and model switches
  are bounded by the shared scheduler.
- Engine metadata and readiness are visible without claiming qualification.
- Text qualification is reported per real provider/platform profile; VLM remains
  gated until independently qualified.
