# Tasks

## 9. ONNX embeddings (approved M5 program)

- [x] 9.1 Preserve shared embedding ownership and poison admission after uncertain cleanup of failed loads or invalid readiness; add failed-load/dimension/retirement regressions before adding another embedding runtime.
- [ ] 9.2 Add a pinned CPU-first offline ONNX embedding package/worker contract with explicit tokenizer, input tensor, pooling, normalization, dimension and dependency identity; reject external graph data/custom execution or unsupported layouts unless independently implemented and verified.
- [ ] 9.3 Route explicitly installed ONNX embedding packages through existing EmbeddingEngine, service embedding API and Knowledge selection without changing defaults or rebuilding existing indexes automatically. Preserve package leases and fail on index/runtime identity mismatch.
- [ ] 9.4 Run protocol/process/scheduler and real ONNX model semantic, cancellation, offline, CPU/platform and frozen retrieval qualification. Record unavailable runtime/model prerequisites rather than substituting synthetic vectors.

ONNX is already required by roadmap M5. These tasks expose that existing scope;
they do not replace the open Ollama/vLLM, MLX or hardware requirements above.
Partial implementation/check evidence:
`docs/quality/evidence/onnx-embeddings-2026-11-20.md`. Package/protocol, explicit
service enrollment and embedding/Knowledge routing are implemented. Review found
that directory read handles did not prevent newly added Python modules; final
Windows execution now requires an administrator-provisioned read-only runtime
namespace and nonprivileged token. Final provisioned execution is an unrun gate;
earlier writable-runtime fixture success is superseded. Tasks 9.2/9.3 stay open
through final validation and ordinary installed/cross-platform packaging. Real
ONNX model and retrieval qualification remains task 9.4, not synthetic vectors.

## 8. Explicit Ollama/vLLM endpoints (approved M5 program, 2026-11-20)

- [x] 8.1 Implement typed Ollama/vLLM HTTP adapters, exact identity checks, bounded NDJSON/SSE, usage/tool validation and request-scoped cancellation. Run focused inference contract tests and race checks.
- [x] 8.2 Connect explicitly authorized remote endpoint enrollment to existing service/model selection and shared provider scheduler; test actor revocation, stale pins, redirects, credentials, reload and no fallback with Go service tests, including colliding legacy local model identities.
- [x] 8.3 Document actual endpoint controls and remaining ownership/qualification limits; run affected Go, docs, source-size and strict OpenSpec checks.
- [ ] 8.4 Complete owned-local Ollama/vLLM supervisor leases, model menu integration and qualified tool-agent routing without pretending HTTP disconnect proves local resources released.
  - [x] 8.4.1 Add bounded endpoint-file reads and a durable subtractive administration policy for already provisioned identities; corruption/restore cannot silently restore revoked authority.
  - [x] 8.4.2 Add authenticated administrator review, explicit scheduler-owned connection checks, revision-bound disable/re-enable and truthful active/queued request semantics without accepting browser-supplied provider destinations or credentials.
  - [x] 8.4.3 Render endpoint-owned model controls and verify actual isolated service/UI management, stale revisions, current actor permission, restart/restore and no-new-request behavior.
  - [ ] 8.4.4 Complete the owner configuration creation/update workflow with explicit validation, durable audit/atomic publication and activation/restart semantics; trusted-file review alone does not complete enrollment administration.
- [ ] 8.5 Run real Ollama/vLLM model/platform text/tool/cancel/offline qualification and frozen workflows; fixture results do not complete this task.

Evidence: `docs/quality/evidence/http-inference-endpoints-2026-11-20.md`. Remote
text enrollment/model-list projection is implemented. The Windows owned Ollama
text supervisor increment is recorded in
`docs/quality/evidence/managed-ollama-windows-2026-11-20.md`; local vLLM/POSIX,
authenticated multi-user runtime isolation, additional Ollama layouts,
qualified tool routing, complete owner enrollment administration and real-runtime gates
remain unfinished. The vLLM deployment owner attests weight identity because
its model-list API does not cryptographically report deployed weight digests.

The rendered review/disable/probe foundation now has actual isolated service/UI,
real loopback HTTP cancellation/no-later-dispatch, restore anti-replay and durable
publication/race evidence in `docs/quality/evidence/endpoint-management-2026-11-20.md`.
This completes only subtasks 8.4.1–8.4.3; the parent task and full owner workflow,
local vLLM, broad platform/runtime/tool/semantic qualification remain incomplete.

Task 8.4.4 now has typed owner prepare/commit/reconcile APIs, creation/update UI,
all 29 authored locale catalogs and an actual Windows service-to-renderer journey
covering lost publication response, explicit reconciliation, restart and fresh
access review. Full Windows checks pass; the earlier owner UI source passed Linux
race, but the later journal integrity follow-up twice hit the full server's local
eight-minute package deadline. That aggregate gate remains unresolved. Evidence:
`docs/quality/evidence/endpoint-owner-configuration-2026-11-21.md`. Keep the task
open for journal/decision capacity maintenance, damaged-state recovery, final
managed/installed-platform and independent/human qualification; do not replace
these with the successful remote metadata fixture.

The [2026-11-22 archival increment](../../../docs/quality/evidence/endpoint-owner-archive-2026-11-22.md)
adds explicit revision-bound retirement of settled owner receipts, immutable
historical duplicate lookup, process-exit/failure checks and actual service
archival/restart evidence. Task 8.4.4 remains open for rendered maintenance and
history, obsolete policy-decision retirement, archive recovery/backup qualification
and the broader gates above. Active-journal capacity relief does not remove the
separate policy-decision limit or establish full server race readiness.

The [policy-maintenance increment](../../../docs/quality/evidence/endpoint-policy-maintenance-2026-11-22.md)
now implements explicit obsolete-decision retirement with full prior-policy
retention, current/installed/active protections, uncertain-publication fencing,
66 reviewed publication cycles and actual service restart evidence. Plain and
encrypted workspace backup/restore preserve both archives without restoring
grants. Task 8.4.4 stays open for rendered maintenance/history, damaged-state
recovery and the broader qualification gates; the full Linux server race
deadline remains unresolved.

The [maintenance UI follow-up](../../../docs/quality/evidence/endpoint-maintenance-ui-2026-11-22.md)
adds exact rendered archive/retirement review, consumed submissions without
automatic retry, 29 authored locale catalogs, actual service/UI restart and
narrow multilingual layout evidence. Historical browsing/export, damaged-state
recovery and broader qualification still keep task 8.4.4 incomplete.

The [history follow-up](../../../docs/quality/evidence/endpoint-history-2026-11-22.md)
adds bounded receipt pages, hash-linked policy browsing and explicit JSON page
export without writes or grant activation. Actual service/UI exports match the
retained records; full Windows and focused Linux race checks pass. Damaged-state
recovery and the broader qualification gates keep task 8.4.4 unchecked.

[Damaged-state recovery evidence](../../../docs/quality/evidence/endpoint-damaged-recovery-2026-11-22.md)
now covers six damaged metadata types in plain/encrypted workspace restore on
Windows and Linux race, preserving originals and requiring fresh access review.
The existing recovery screen remains reachable from an unreadable owner file.
Installed/native, external-owner and independent/human qualification remain open;
no speculative reset or task completion is claimed.

The [full race recheck](../../../docs/quality/evidence/server-race-recheck-2026-11-22.md)
passes the complete server suite under the unchanged eight-minute limit and all
22 CI race packages. Earlier timeouts remain retained without a claimed cause;
85 opt-in/helper skips do not establish qualification. Task 8.4.4 stays open for
the remaining installed/native/external-owner and independent/human gates.

Task 8.4 prerequisite progress: existing Windows `ProcessProvider` now uses
per-runtime suspended-start Job Objects and bounded descendant/reap proof.
Evidence: `docs/quality/evidence/provider-process-ownership-2026-11-20.md`.
Windows owned Ollama listener/launch integration now has fixture evidence above;
the restricted Linux root-provisioned factory, artifact/namespace/listener checks
and service routing are recorded in
`docs/quality/evidence/provider-linux-namespace-2026-11-20.md`. Full provisioned
Linux factory success is an unrun explicit fixture gate; broader Linux
installations, macOS and the remaining task 8.4 requirements are not complete.

## 7. Memory-pressure follow-up (2026-11-15)

- [x] 7.1 Measure Windows commit headroom separately from physical RAM and apply
  it to model load admission without changing requested allocations. Test CPU,
  GPU, exhausted commit, unknown measurement and ordinary capacity cases.
- [x] 7.2 Bound prewarming to one active operation, a finite byte/time budget and
  bounded history; stop on pressure or pause, propagate read failures, and test
  repeated async requests and cancellation without leaked producer goroutines.
- [x] 7.3 Document memory versus disk limits and the verified local archive
  relocation; run resource/inference/server checks and preserve user data.

Evidence: `docs/quality/evidence/reader-library-flow-2026-11-15.md` (memory/archive
section); safeguards and limits: `docs/advanced/low-memory.md`. Verified disk
recovery is not a claim about freed runtime RAM or full hardware qualification.

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
