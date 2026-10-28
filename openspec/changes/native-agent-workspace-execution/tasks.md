# Tasks

## 1. Execution context and bounded local files

- [x] 1.1 Add versioned execution contexts, grant bindings and provider-local current directories; test invalid identity, revocation, narrowing and independent sessions with `go test ./internal/execution`. Document trusted-host versus rooted-file boundaries in the package README.
- [x] 1.2 Add rooted bounded directory listings and revision-bound raw file pages; test Unicode, changed pagination, links/traversal, special files and cancellation with `go test ./internal/execution`.
- [x] 1.3 Add exclusive creation, preconditioned replacement, search and independent read-back; test stale revisions, output bounds and unchanged unrelated files with `go test ./internal/execution`, documenting concurrent-writer limitations.

## 2. Supervised executor and process lifecycle

- [x] 2.1 Add strict bounded versioned worker envelopes and supervised stdio dispatch; test malformed/truncated frames, unknown fields, wrong actor/grant/job and cancellation with `go test ./internal/execution ./cmd/offgrid-executor`; document the protocol.
- [x] 2.2 Journal operation claims and payload identities before process dispatch; test duplicate/conflicting IDs, restart and persistence failures with `go test ./internal/execution` and race coverage; uncertainty must not replay.
- [x] 2.3 Implement bounded foreground/background process output, actual exit states, 180-second default deadlines, minimal environment and owned-tree cancellation on Windows/Unix; test processes only in disposable directories with `go test ./internal/execution` on both systems. Document missing platform evidence separately.
- [x] 2.4 Add enrolled authenticated worker transport, local grants, revocation, identity/version/pack checks and isolated Linux backend; test forged/cross-user messages, host loss, mount/network constraints and no Docker socket exposure; update host setup docs.

## 3. Existing runner and public contracts

- [x] 3.1 Add additive execution references to job/turn admission and pinned configurations, plus actor-scoped execution resources; test idempotency and old/new clients with `go test ./internal/agents ./internal/server ./internal/serviceclient` and update OpenAPI/API docs.
- [x] 3.2 Register reviewed workspace/process capabilities through the existing runner, effective catalog and independent grant checks; retain legacy shell denial and MCP/computer composition. Test reads, edits, processes, revoked authority and no-effect rejection with the same Go packages.
- [x] 3.3 Add process/event/evidence projections and reconciliation from host receipts without replay; test every dispatch/persistence boundary and cancellation using Go/race checks; update recovery docs.

## 4. CLI and application parity

- [x] 4.1 Consolidate default CLI interaction with canonical conversation functions, authenticated discovery, profile/model/resume controls and profile budgets; test history continuity, overrides, endpoint/auth handling and no duplicate submissions with `go test ./internal/cli ./cmd/offgrid`.
- [x] 4.2 Add trusted CLI folder selection and executor supervision, desktop folder/grant IPC and browser enrolled-host selection; test launch-directory preservation, no implicit grants and old-host compatibility; update CLI/desktop guides.
- [x] 4.3 Show effective host/folder/toolsets and useful command/file activity in current Chat/profile UI, with nine locales and accessible readiness errors; run web API/type/build, focused Playwright and desktop/companion tests.
- [x] 4.4 Package and verify matching executor assets across native/release/Docker contexts; run installer/layout/hygiene/pack tests and document upgrade/rollback without deploying.

- [x] 4.5 Complete foreground one-shot CLI UX through the existing runner: terminal detection, once-per-run folder approval, explicit folder/access/detach controls, clear help, canonical cancellation and no service-directory fallback. Test JSON/nonterminal compatibility, exact binding, stream failure/cancellation and lease cleanup with Go/race tests.
- [x] 4.6 Build and install the matching WSL CLI/executor after checks, preserve rollback, and exercise a real-model disposable-folder listing plus foreground output; record installed paths, actual results and remaining qualification limits without treating this smoke check as task 6.2 completion.

## 5. Broader approved native-agent program

- [x] 5.1 Scaffold and implement dependent native-agent-skills-memory: revisioned Agent Skills import/loading/drafts, opt-in automatic scoped memory, concurrent writes and user inspection/delete; verify actual service/CLI/UI behavior, injected-content boundaries and absence of implicit script authority.
- [x] 5.2 Scaffold and implement dependent native-agent-automation: pinned schedule grants, IANA times, durable occurrence identity, no overlap/catch-up/replay, blocked-host states and Activity delivery. Test restart, DST, revocation and unattended isolated jobs.
- [x] 5.3 Integrate qualified model adapters, authorized research and bounded collaboration through existing changes; run greeting, researcher/writer/file-save, skills/memory and scheduled cases without adding an executor or resetting aggregate budgets.

## 6. Integrated qualification

- [x] 6.1 Run full Go/race, API, web build/browser, desktop/companion, SDK and packaging CI checks. Record commands/failures/skips and compatibility evidence in the existing reliability plan; validate this change with `openspec validate native-agent-workspace-execution --strict --no-interactive`.
- [ ] 6.2 Run basic selected-directory/follow-up/edit/test/Stop workflows three times per advertised real model/backend and the broader frozen suite at the approved completion/safety gates; retain unavailable native/hardware qualification as unchecked, not fixture success.
- [x] 6.3 Reconcile every approved program acceptance scenario with implemented, automatic and real-hardware evidence; no deployment, commit or release is implied by implementation progress.

## 7. Typed user-input writing

- [x] 7.1 Add bounded exact UTF-8 source selection and source provenance for root/applied-correction inputs; reject ambiguity, malformed fields and foreign/pending sources. Test Unicode, CRLF, empty/large spans and unchanged original inputs with `go test ./internal/execution ./internal/agents`.
- [x] 7.2 Register `workspace_write_from_input` under existing write authority, persist resolved bytes before effects and validate matching independent receipts in dispatch/completion/recovery. Test scoped grants, preimages, tampering, uncertain effects, restart and old-job/ordinary-tool compatibility with `go test ./internal/server ./internal/agents ./internal/execution` and affected race checks.
- [x] 7.3 Reuse CLI/app activity and existing localized edit presentation; document source-selection limits and recovery. Run applicable API/web/type/build, focused renderer, desktop presentation, documentation, source-size and strict OpenSpec checks without claiming native qualification.
- [x] 7.4 Exercise fresh Meta-Llama/Qwen exact-input-write/read/test workflows three times in disposable folders through installed compatible builds; record actual failures, source-byte equality, untouched files and canonical identities. Preserve broader task 6.2 and unavailable hardware gates as unchecked; no release or Git publication is implied.

## 8. Conversation-first access and client experience

- [x] 8.1 Add versioned requestable execution capability and durable workspace input/attachment on the existing canonical root. Test profile/actor/workspace ceilings, decline, duplicate resolution, cancellation, restart and old-client compatibility; update API/generated types.
- [x] 8.2 Defer CLI executor creation until actual local access is requested, retaining launch-directory identity, foreground streaming, conversation follow-ups and JSON/detached behavior. Test no prompt/executor for conversation and exactly one root for local work.
- [x] 8.3 Implement optional client-owned remembered folder trust with exact resource/identity/permission binding, inspect/revoke controls and fail-closed persistence. Integrate CLI and desktop without converting old grants or weakening native consent; test changed identities, stale roots, revocation and permission expansion.
- [x] 8.4 Repair shared capability/response context against the captured capability and story failures, preserving current user intent through tool results and recovery. Run deterministic contract tests and isolated live-model comparisons, keeping failed outcomes visible and avoiding prompt-specific routing.
- [x] 8.5 Simplify foreground CLI progress and integrate on-demand access/trust presentation in existing Chat/app controls. Preserve detailed inspection, errors, cancellation, JSON behavior, nine locales and accessibility; run CLI, API, web/type/build/renderer and desktop checks.
- [x] 8.6 Run greeting, identity, capabilities, story, explanation, file follow-up, exact edit and test-command cases with and without attached resources on Meta-Llama/Qwen. Record actual correctness, unnecessary actions, approval count and latency separately; broader platform qualification remains open. No deployment or Git publication is implied.
- [x] 8.7 Align negotiated foreground one-shot response allocation with interactive Chat, with explicit extended selection and legacy/API/idempotency compatibility; run Go/race, API generation, TypeScript and documentation checks.
- [ ] 8.8 Measure installed-model cold/warm response timing and repeat mixed conversation/file/command cases after the refinement, preserving failures and broad task 6.2 as unverified where platform evidence is unavailable.
- [x] 8.9 Complete pending owned-process verification through bounded, receipt-derived wait calls persisted through the ordinary authorization/dispatch boundary; test foreign sessions, unavailable capabilities, profile denial, restart identity, cancellation and no duplicate starts. Compare response adapters in isolated decision/workflow tests without changing production selection or treating fixture success as model qualification.
- [x] 8.10 Refine persistent CLI session presentation and UX: one welcome, readable profile/model, shared slash help/picker, pinned skills/collaborators/tools views and on-demand recovery details. Test no command submissions or grants, Unicode/paste/shared consent input, plain/narrow terminals, streaming and cancellation with `go test ./internal/cli ./cmd/offgrid`, race coverage and an isolated PTY fixture; document actual limitations.
- [x] 8.11 Preserve validated response-schema order across public API forwarding and use matching discriminator-first historical decisions. Cover exact numbers, ambiguous JSON, nested caller mutations and API/inference round trips with `go test ./pkg/api ./internal/inference ./internal/server`, race tests and applicable compatibility checks; distinguish corrected test transport from model/workflow qualification.
