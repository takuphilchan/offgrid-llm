# Design: Provider-neutral local inference runners

## Memory-pressure follow-up (2026-11-15)

Use current OS physical availability and, on Windows, separately measured commit
headroom. CPU budgets cannot exceed either pool; GPU availability cannot bypass
the host reserve needed by the process/driver. Failed measurement is not spare
capacity. Prewarming is speculative: at most one operation, 512 MiB per batch,
30 seconds, a 1 MiB read buffer, 32 history entries, and pressure checks between
reads. Pause cancels the current operation even if resumed before its next read.
Partial reads never claim the complete model is warm; page-cache residency is
an OS-controlled hint, not verified retention. No active inference is killed and
no model, context, download or user document is changed by these controls.

## Architecture

The service keeps one agent-facing inference contract and delegates native work
to a provider selected from immutable model metadata:

```text
Agent / chat / capability probe
             |
      provider-aware scheduler
             |
  llama.cpp | MLX worker | Transformers worker
             |
 CPU/CUDA/Metal/ROCm/Vulkan/MPS local resources
```

The scheduler owns admission, safe model switching, budgets, process ownership,
and cancellation. Providers own native loading and transport details. Workers
never receive desktop authority, credentials, or unrestricted filesystem access.

## Provider contract

Add an internal provider interface with load/start, capability handshake,
streaming completion, normalized tool decision, token accounting, memory/device
metrics, cancellation, health, and close operations. Non-llama providers use a
versioned authenticated loopback protocol containing protocol/provider/runner
revisions, request and cancellation IDs, model digest, typed stream events,
terminal status, and capability metrics.

The service supervises worker processes with bounded startup, request, shutdown,
and restart limits. A crash is operational unavailability, not model
incompatibility. No worker can execute arbitrary commands or install packages.

## Providers

- **llama.cpp:** adapt the existing binary manager and HTTP engine with no
  behavioral change; GGUF models without engine metadata default here.
- **MLX:** pinned worker for MLX/safetensors text models, Metal-first, with
  other backends admitted only after measured capability evidence.
- **Transformers:** pinned local Python/PyTorch worker with remote code disabled,
  explicit device selection, and pinned model-family adapters.

VLM/audio metadata is additive, but dispatch requires qualified preprocessing,
projector/encoder identity, prompt assembly, and tool-call evidence.

## Model metadata and trust

Extend manifests and installed projections with engine, format, backend,
runner/adapter revisions, modalities, tool-call status, context/template
identity, preprocessing dependencies, and separate installed/runtime-ready/
smoke-tested/qualified states. Runtime binaries are trusted bundled assets or
explicitly imported offline providers. Model artifacts never carry executable
provider code. Unknown families remain metadata-only until a pinned adapter is
qualified.

## Scheduling and migration

One admission scheduler serves parent/child/perception/probe/chat inference.
It prefers a resident compatible runner, serializes unqualified loads, measures
load/queue/generation/cancel/unload separately, and preserves Stop. Provider
changes occur only at persisted safe boundaries and never retry unknown effects.

Existing GGUF records receive the llama.cpp default during an additive
migration. Existing profiles, tasks, revisions, permissions, API fields, and
model files remain valid. No download or automatic re-import occurs.

## Compatibility and rollout

The public API adds provider metadata and readiness fields while preserving old
responses. Feature flags expose MLX/Transformers only after worker contract and
real hardware checks pass. Text is qualified independently from VLM/audio.

## Memory-pressure follow-up (2026-11-15)

- Replaced Go-heap-as-free-RAM prewarming estimates with current OS memory.
  Windows commit headroom is measured separately and used in llama load admission;
  free VRAM cannot bypass the host commit reserve. macOS no longer assumes 80%
  of installed RAM is free. Failed monitor samples are marked unmeasured.
- Prewarming now has one operation, one 1 MiB buffer, a shared maximum 512 MiB
  byte budget, 30-second deadline, bounded 32-entry history and no producer queue.
  Pause/resume invalidates old work; partial/truncated reads cannot claim completion.
- Full `go test ./internal/resource ./internal/inference -count=1` passed
  (`build/memory-resource-inference-full.log`). Focused server context/capacity
  tests passed (`build/memory-pressure-tests.log`; its initial warmer history test
  exposed equal-timestamp eviction, fixed with ordered bounded history before the
  full successful run). Race instrumentation is unavailable without CGO/compiler;
  the installed WSL distribution also failed startup. No race pass claimed.
- Real model quality and sustained memory behaviour across supported hardware
  remain separate qualification; these checks do not establish a hard total-PC
  memory limit. No installed model/context preference was silently changed.
