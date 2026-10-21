# Tasks

## 1. Contracts and metadata

- [x] 1.1 Add the provider lifecycle interface, typed worker protocol, event
  envelopes, cancellation semantics, and fake-provider test harness.
- [x] 1.2 Extend model/package metadata and OpenAPI projections with engine,
  format, backend, runner/adapter revisions, modalities, and readiness states.
- [x] 1.3 Add pinned model-family adapter registry and reject unknown executable
  adapters or remote-code execution.

## 2. llama.cpp compatibility provider

- [x] 2.1 Wrap existing llama HTTP/binary lifecycle behind the provider contract
  without changing GGUF, CPU, CUDA, projector, embedding, or API behavior.
- [x] 2.2 Add migration defaults for existing records and compatibility tests for
  saved profiles, tasks, model revisions, and clients.

## 3. MLX and Transformers providers

- [ ] 3.1 Implement the supervised pinned MLX worker and Metal/device handshake;
  qualify text streaming, cancellation, restart, and offline operation.
- [x] 3.2 Implement the supervised pinned Transformers/PyTorch worker with
  remote code disabled and explicit CPU/CUDA/MPS readiness.
- [x] 3.3 Add provider-specific adapters for templates, tokenization, tool-call
  envelopes, and model-family validation.
- [x] 3.4 Keep VLM/audio dispatch behind separate modality qualification and
  record projector/preprocessing identity when eventually enabled.

## 4. Shared scheduler and recovery

- [x] 4.1 Move inference admission, process ownership, memory accounting, and
  load/queue/generation/cancel/unload timing behind one provider-aware scheduler.
- [x] 4.2 Add resident-runner reuse, safe provider switching, crash handling,
  bounded restart, cancellation, and uncertain-effect protections.
- [x] 4.3 Integrate parent/child/perception/probe budgets and preserve Stop,
  CPU/GPU behavior, and local-only execution.

## 5. API, UI, and clients

- [x] 5.1 Add additive provider/runtime/readiness fields to API and generated
  clients while preserving existing response shapes.
- [x] 5.2 Update Models and profile surfaces to show engine, format, backend,
  context, readiness, qualification, and provider diagnostics truthfully.
- [x] 5.3 Update CLI/Python projections through authenticated service APIs only.

## 6. Verification and qualification

- [x] 6.1 Run provider contract, protocol, cancellation, crash, malformed-output,
  and scheduler race tests with fake workers.
- [x] 6.2 Run llama.cpp regression suites and affected Go/API/TypeScript/desktop
  checks.
- [ ] 6.3 Run MLX text qualification on Apple Silicon and Transformers text
  qualification on each supported CPU/CUDA/MPS profile; record unavailable
  hardware explicitly.
- [ ] 6.4 Run the frozen workflow suite three times per advertised provider /
  platform profile and record latency, memory, unsafe actions, and false claims.
- [x] 6.5 Update the reliability plan and deployment documentation with separate
  implemented, automatically tested, and real-hardware-qualified states.
