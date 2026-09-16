# Product reliability and usefulness plan

This is the implementation plan following the September 2026 system review.
It is a delivery checklist, not a claim that every capability is production-ready.

## Product outcome

Make OffGrid a dependable private workspace for conversations, document-grounded
answers, and governed tasks. Browser, desktop, CLI, and external agents use the
same Go runtime and behavioral contracts. Keep Go, React/TypeScript, Electron,
and the native OffGrid provider adapters. Do not introduce microservices or a
framework migration to solve local state and lifecycle problems.

## Delivery rules

- Ship small vertical slices with service, API, UI/CLI, documentation, and tests.
- Preserve existing models and data. Never silently claim legacy data for the
  first signed-in user, or replace failed persistent storage with volatile data.
- Authorization belongs at the service boundary as well as the HTTP boundary.
- An unavailable dependency must produce an explicit capability state.
- A successful HTTP response or CI build is not proof of a useful model task.
- Never replay side effects automatically after a crash or approval pause.
- Keep experimental capabilities visibly separate from supported workflows.
- No automatic publishing: validate locally before a separately approved release.

## Stage 1: privacy and data safety

First delivery slice:

- [x] Enforce session ownership on list, read, create, delete, append, and generate.
- [x] Prevent session creation from overwriting an existing conversation.
- [x] Preserve legacy unowned sessions for local mode and authorized administrators.
- [x] Enforce RAG access for both streaming and non-streaming chat and sessions.
- [x] Disable knowledge ingestion when durable storage cannot initialize.
- [x] Make requested-but-unavailable/failed retrieval an explicit error, not
      a silent ungrounded model answer.
- [x] Add regression tests and document upgrade behavior.
- [x] Fix the queue/cache races found during validation; add focused race checks
      to CI and replace the timing-dependent priority test with an order assertion.

Durable execution and draft slice:

- [x] Replace task resubmission with run-ID-based approve/deny/cancel/resume.
- [x] Bind approvals to run ID, call ID, actor, tool, canonical arguments, and
      expiry; consume a grant atomically once.
- [x] Persist pending tool calls and execution checkpoints before side effects.
- [x] Record an uncertain outcome after an interrupted side effect; require
      reconciliation or an explicitly approved retry rather than assume success.
- [x] Reconcile orphaned running jobs to interrupted on startup, and surface
      persistence failures. Use atomic task snapshots or transactional storage.
- [x] Protect drafts from navigation, reload, failed sends, and user switching.

Exit tests: two authenticated users cannot access or mutate each other's
conversations; guest/viewer chat cannot retrieve protected knowledge; a damaged
database cannot acknowledge a successful import; approving once cannot execute
twice; denial reaches the server; restart does not leave phantom running jobs;
failed requests and navigation do not discard drafts. Ordinary local chat must
remain usable when optional knowledge storage is unavailable.

## Stage 2: complete everyday workflows

- Add stable opaque conversation IDs, separate editable titles, owner-scoped
  pagination, and an explicit migration for existing name-addressed sessions.
- Stream durable chat with one request ID, replayable progress, cancellation,
  and an atomic completed turn. Define partial-response retention explicitly.
- Move job state out of individual React pages. A reconnecting client should
  reconstruct current work from the server instead of restarting it.
- Add document collections, folder import, extraction preview, incremental
  reindexing, and clear scanned-PDF/OCR capability messaging.
- Define collection ownership and sharing. Enforce access when listing sources,
  retrieving passages, ingesting, and deleting; a client-side filter is not an
  authorization boundary. Today the knowledge index is shared by authorized RAG
  users, not isolated per user or collection.
- Persist retrieval status and structured citation locators with each answer;
  let users open the exact source passage and export a sourced answer.
- Distinguish retrieval failure, no matching evidence, and successful retrieval.
  Add source-only answering and document scope; do not present rank scores as
  probabilities of factual correctness.
- Maintain the monochrome design system, keyboard navigation, focus management,
  accessible status messages, responsive layout, and shared desktop/web behavior.

Exit tests: a new user imports documents, gets a cited answer, verifies a passage,
restarts the app, and continues without terminal troubleshooting; long-running
chat and agent jobs remain understandable while navigating between pages.

## Stage 3: measured quality and performance

- Establish representative CPU-only, low-memory, NVIDIA GPU, and Apple Silicon
  profiles. Report actual GPU offload and allocated context, not only settings.
- Benchmark cold load, first token, prompt processing, generation, peak memory,
  queue wait, cancellation, and retrieval at increasing collection sizes.
- Consolidate scheduling around one bounded, cancellable inference admission
  service with fairness between interactive chat, agents, and background work.
- Paginate session and run history. Avoid replaying the entire event log for
  every activity request; use indexed summaries and bounded event queries.
- Optimize measured retrieval bottlenecks: compact vectors, filtering before
  ranking, bounded candidate selection, and an index only where benchmarks justify it.
- Version retrieval evaluation fixtures and test citation support and abstention,
  not just successful HTTP responses. Review multilingual fixtures with speakers
  of the target languages; interface translation is not evidence of model quality.
- Extract conversation, knowledge, model, and run application services from
  large transport/CLI files. Keep generated API types aligned with behavior.

Exit criteria: publish reproducible baselines, document tested hardware limits,
and gate regressions against those baselines. Set numeric budgets from measurements
rather than promising hardware-independent latency or quality.

## Stage 4: dependable distribution and focused expansion

- Manage supported Hermes/OpenClaw versions, compatibility probes, staged installs,
  repair/uninstall, and rollback. Separate installed/configured/connected/task-tested.
- Test actual tool calls and an artifact-producing task with each supported agent.
  External runtimes retain their own tool permissions; an inference adapter does
  not place their filesystem/network actions under OffGrid's approval broker.
- Add packaged-app launch, inference, graceful shutdown, update, and data-migration
  smoke tests on Windows, Linux, and macOS; add race and vulnerability checks to CI.
- Design backups and restore verification before automated upgrades. Add desktop
  signing/notarization and a tested rollback path before advertising seamless updates.
- Extend the existing doctor command with actionable, redacted diagnostics for
  runtime version, storage, model integrity, GPU placement, context, and integrations.
- Generate a tested capability matrix and stable installation guides from release
  metadata; clearly separate stable images from development/edge instructions.
- Pilot offline packs and trusted LAN model distribution, with signatures,
  checksums, explicit trust, license metadata, and resumable transfer.
- Keep unrestricted computer use, distributed inference, and fine-tuning out of
  the supported core until they have owners, threat models, and platform tests.

Exit criteria: install and upgrade from a clean supported machine, restore a
backup, complete a useful task, and diagnose a missing optional dependency without
special knowledge of the repository.

## Tracking and release evidence

Record completed slices below with commands run and limitations. Do not mark a
stage complete because its unit tests pass while its user workflow remains absent.

### 2026-09-16: Stage 1 first slice implemented locally

Implemented owner-scoped session operations, non-overwriting creation, shared
chat authorization/retrieval policy, and explicit unavailable knowledge storage.
Legacy data is preserved; no ownership migration runs automatically. Also fixed
the queue-size error-path race, synchronized cache enable/disable and cleanup
start/stop, and made cleanup shutdown wait for its worker to exit.

Validation passed:

- Windows: `go test ./...`.
- WSL/Linux: `go test -race ./internal/sessions ./internal/rag ./internal/server
  ./internal/users ./internal/cache`.
- Web: `npm run api:generate`, `npm run api:check`, `npm run check`, and
  `npm run build`.
- CI syntax: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
  -shellcheck= .github/workflows/ci.yml`.
- `git diff --check`.

The tests exercise cross-user requests, forged ownership, legacy access,
concurrent creates/appends, broken database/directory initialization, retrieval
failure/no-evidence behavior, and concurrent cache lifecycle. They do not prove
model answer quality, OS packaging, or real Hermes/OpenClaw tool execution.

At this checkpoint, durable approval/resume and drafts were still pending.
The next entry records their implementation; collection-level authorization and
end-to-end citations remain later-stage work.

### 2026-09-16: durable execution and draft recovery implemented

Built-in agent runs now use persisted native-tool checkpoints with actor-bound,
single-invocation expiring approvals. Every model-requested tool in a batch is
processed, not only the first. A checkpoint write must succeed before execution.
Startup never replays a side effect; interrupted calls become uncertain and
require a human-verified reconciliation. Concurrent approval attempts are
serialized; models release inference admission while waiting for human input.

Web and CLI controls inspect, approve, deny, cancel, resume, and reconcile the
same run. Returning to the Agents page restores the selected run from the server.
Legacy history is preserved but has no invented resumable checkpoint. Run
snapshots remain authoritative if the event-log projection fails. Storage
failures are explicit. CLI requests use the configured API key and render the
new committed-step/final-result stream.

Chat/task drafts are scoped to account and conversation in browser storage,
saved during editing, retained on failed sends, and protected from late response
clears. Storage failure keeps an in-memory draft with a warning. Identity must
resolve before rendering account-scoped state. Browser drafts are not encrypted
or synchronized across devices.

Validation passed before the container smoke test:

- Full Windows `go test ./...`, then affected CLI/agent/server packages again.
- WSL `go test -race ./internal/agents ./internal/runs ./internal/sessions
  ./internal/rag ./internal/server ./internal/users ./internal/cache`.
- OpenAPI generation/check, TypeScript check, production UI build.
- 16 Edge browser tests for workspace, functional pages, and recovery, including
  failed storage, failed sends, late responses, user switching, and approval/deny
  after navigation/reload. Model/tool interactions use controlled fixtures.
- CI actionlint, Electron main/preload JavaScript syntax, and whitespace checks.

Stage 1's listed implementation is complete; this is not a declaration of
production readiness for the whole roadmap. Stage 2 remains: opaque conversation
IDs/titles and pagination, durable streaming chat, collection isolation, and
stored source citations. Stage 3 needs measured hardware/quality baselines;
Stage 4 needs supported-platform release and real external-agent task evidence.
No commit, push, registry publication, or release is part of this local slice.

Local deployment verification:

- Built `offgrid-llm:reliability-20260916` from the working tree with runtime
  version `workspace-20260916-reliability` (not a published release tag).
- All 20 browser tests passed against an isolated image instance; real session
  create/reload/delete and navigation exercised the actual service, while model
  and approval scenarios used API fixtures. The isolated instance was removed.
- Replaced the local `offgrid` container on loopback port 11611, preserving
  `offgrid-models`, `offgrid-data`, and `OFFGRID_MAX_CONTEXT=65536`.
- Preserved a stopped rollback container `offgrid-pre-reliability-20260916` and
  a data snapshot at `D:\offgrid-backups\pre-reliability-20260916` before startup
  recovery changed old task statuses. Never run both containers against the data
  volume concurrently. The rollback container alone is not a data snapshot.
- Replacement is healthy and serves the new UI bundle. Seven navigation and
  recovery browser tests also passed against the replacement's built UI.
- This remains the existing CPU-only container configuration; no GPU backend,
  platform packaging, or new release publication is implied by this rebuild.
- A real `/v1/chat/completions` request to installed Phi 3.5 returned
  `OFFGRID_READY` (HTTP 200). Observed cold model load was 98.3 seconds and total
  request time 108.1 seconds with the preserved 65K context; a single sample is
  not a benchmark or a quality evaluation. This confirms that startup/memory
  profiling and GPU validation remain important Stage 3 work.

### 2026-09-16: responsive saved chat and context profiles implemented

Saved chat streams by default in the shared web/desktop renderer. Queue,
retrieval, loading, prompt processing, generation, and persistence are visible.
Stop cancels queue waits and inference. A completed turn is saved atomically
before the final event; transport failure or partial output is never reported
as a saved answer. Drafts and visibly provisional text survive failures.
The client batches rendering and follows output only while near the bottom.

Interactive chat defaults to an 8K context (bounded by the service limit),
while Extended and external-agent requests retain the configured service
context. Model/context switches wait for active leases. The runtime does not
silently truncate history, increase model residency using GGUF size alone,
or restart a cached process independently of inference admission. Backend
processes retain their environment, including CUDA loader/device settings.

The stream reports queue/load/first-text/generation timings and actual backend
token usage when available. Output budgets and context preferences persist per
account. The API contract and performance guide describe these semantics without
hardware-independent speed promises.

Validation passed:

- Full Windows `CGO_ENABLED=0 go test ./...`, then affected packages again.
- WSL `go test -race ./internal/server ./internal/inference ./internal/sessions`.
- OpenAPI generation, TypeScript check, and production UI build.
- All 23 Edge browser tests, both against the development build and the rebuilt
  image's packaged UI. Streaming tests cover incremental text, split UTF-8,
  premature EOF, cancellation, draft recovery, and saved completion. Model
  responses use controlled fixtures, not a hardware benchmark.
- Built `offgrid-llm:streaming-20260916` and confirmed its runtime version,
  health endpoint, and new UI asset hashes in an isolated temporary container.
  The temporary instance had no access to the user's model/data volumes and
  was stopped and automatically removed after testing.
- NVIDIA passthrough probe detected the RTX 5060 Laptop GPU with 8151 MiB VRAM.
  A device probe is not proof of GPU model inference.

Remaining limits: no replayable chat request ID or reconnect/resume protocol
yet. A disconnect during final persistence has an uncertain client outcome;
reload before retrying. Full hardware memory admission, fairness, and repeated
representative hardware benchmarks remain unverified. The source CUDA build
was interrupted by a large image-layer download error; local GPU validation
used the alternative runtime assembly described below.

Deployment checkpoint: the new CPU image is built, but replacing the running
`offgrid` container was blocked by the execution environment's policy. The
existing `offgrid-llm:reliability-20260916` service and its named volumes remain
unchanged. No registry publication, commit, push, or release was performed.

GPU validation follow-up:

- The released CUDA runtime failed to start its inference binary because
  `libgomp.so.1` was absent. Added `libgomp1` to `docker/Dockerfile.gpu` and a
  final-image linked-library check. Only `libcuda.so.1`, supplied by the NVIDIA
  container runtime, is permitted to be absent at build time.
- Built local `offgrid-llm:streaming-gpu-20260916` using the fresh CPU image's
  pure-Go binary and UI, plus the released CUDA runtime at immutable digest
  `sha256:b6290a2d4059859d3801cc4fbdb08e7030c0997971380fa648ab29f133f298b5`
  and the missing dependency. Verified its llama.cpp revision `e9fa078` matches
  the current source Dockerfiles. This local validation assembly is not a new
  published release or a successful from-source CUDA rebuild.
- In an isolated, auto-removed container with read-only models, separate
  temporary data, and a 1536 MiB RAM limit, actual Phi 3.5 saved-chat requests
  streamed text, reported a token-limit finish, committed the turn, and returned
  the same saved text on reload. NVIDIA VRAM usage reached roughly 3334 MiB.
- At 8192 context and a 32-token output limit, one cold request showed 8.41s
  model loading, 8.56s client-observed first text, and 9.12s through saved
  completion. A warm repeat showed 0.24s first text and 0.71s through completion.
  Backend-usage-derived decoding rates were about 63 and 69 tokens/s. These
  short identical-prompt samples are not representative benchmarks or a
  like-for-like comparison with the earlier CPU/65K test. The model returned
  the requested marker but continued with unwanted text; instruction-following
  quality is not established. Actual 65K GPU/agent inference remains untested.
- The temporary GPU container was stopped and automatically removed, including
  its disposable test conversations. The normal container on port 11611 remains
  unchanged because the replacement operation was blocked.
