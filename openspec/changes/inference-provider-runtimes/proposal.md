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
- Add the roadmap's ONNX embeddings through the existing embedding owner,
  with pinned local model/tokenizer/runtime identities, explicit CPU execution
  first, bounded vectors and separate retrieval/hardware qualification.
- Extend model/package metadata additively with engine, format, backend,
  runner/adapter revisions, modalities, and readiness evidence.
- Move admission and lifecycle decisions behind one provider-aware scheduler.
- Preserve existing profiles, tasks, APIs, GGUF models, CPU/GPU behavior, and
  stored state without automatic downloads or cloud fallback.
- Qualify text inference first; keep VLM/audio execution unavailable until
  model-specific preprocessing and tool-call evidence exists.
- Add explicitly enrolled Ollama and vLLM HTTP adapters through the same provider
  scheduler, with pinned endpoint/model/runtime identities, per-actor authority,
  bounded streams and no automatic discovery, downloads or failover. Remote
  enrollment and owned-local lifecycle integration are distinct acceptance work.
- Complete explicit administrator creation/update of that owner configuration
  through reviewed, revision-bound durable publication and reconciliation, with
  installation-owned credential/runtime ceilings and separate restart/access
  activation. Keep ordinary model requests identity-only.

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
