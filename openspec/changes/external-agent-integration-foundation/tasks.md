# Tasks

## 1. Contracts and context

- [x] 1.1 Add shared descriptor, options, service assessment/client and context-bound checks; verify authenticated requests, model selection and context cases with `go test ./internal/integrations ./internal/server ./internal/power` and document contracts.
- [x] 1.2 Finish battery-context regression while preserving low-battery concurrency and custom limits; verify `go test ./internal/power` and update external-agent guidance.

## 2. Shared host lifecycle

- [x] 2.1 Implement shared inspection/configuration/test/run manager, scoped connection records, private snapshots/journals and evidence invalidation; verify injected failure, concurrency, idempotency and third-adapter fixtures with `go test ./internal/integrations`.
- [x] 2.2 Adapt Hermes and OpenClaw to the manager, preserving bundles and existing defaults and adding per-run selection; verify external-command fixtures with `go test ./cmd/offgrid ./internal/cli` and document compatibility boundaries.
- [x] 2.3 Route aliases and generic configure/status/test/run through one parser/authenticated client; verify error exits, auth, option order and legacy provider-only commands with CLI tests; update README and CLI guide.

## 3. API and presentation

- [x] 3.1 Add service-scope readiness fields and registry-defined IDs to the API and generated clients; verify `npm --prefix apps/web run api:check` and server contract tests; update API documentation.
- [x] 3.2 Remove integration-specific UI behavior and add localized truthful state presentation; verify `npm --prefix apps/web run check`, `npm --prefix apps/web run build` and renderer fixtures for setup/readiness.

## 4. Integration evidence

- [x] 4.1 Run applicable full Go, race, web, desktop, documentation, hygiene and OpenSpec checks; record exact results in evidence.md and the existing reliability plan.
- [x] 4.2 Exercise installed Hermes/OpenClaw in isolated homes against a disposable local test service or explicitly scoped existing inference endpoint: verify setup repetition, model reply, disposable-file tool workflow and cancellation; record runtime/model versions, actual results and unavailable platform checks separately from fixture evidence.

These are implementation and verification tasks, not claims of real-model or
cross-platform qualification. See [evidence.md](evidence.md) for the failed local
model workflow and the explicitly unverified native/platform gates.

## 5. Observed launch and admission regressions (approved 2026-10-22)

- [x] 5.1 Decouple battery and transient pressure from context; retain memory/model admission, stable resident allocations, runtime confirmation and truthful power detection. Test power transitions, unknown sources, pressure and smaller-request reuse with Go unit/race tests.
- [x] 5.2 Repair foreground terminal ownership and cancellation reporting for interactive integrations; cover PTY input, Ctrl+C, EOF, startup failure, process-tree cancellation and terminal restoration without installed-user state.
- [x] 5.3 Handle supported structured response formats without silently dropping constraints; verify request validation, backend forwarding, unsupported-provider rejection and Hermes title-generation interoperability.
- [ ] 5.4 Run applicable regression checks, record actual evidence, then separately perform the user-authorized recoverable container and matching WSL CLI replacement; preserve model/config/workspace data and verify installed behavior. The October 23 GPU r8/q8 replacement, experimental template adapter and matching CLI checks are recorded, but real Hermes/OpenClaw file workflows and OpenClaw greeting quality still fail independent verification, so installed behavior is not yet accepted.

## 6. Slow local inference and truthful streaming (approved 2026-10-22)

- [x] 6.1 Implement prompt stream startup, serialized keep-alive/progress and bounded cancellation without leaking partial tool decisions; test slow admission/output, errors, disconnects and native/llama compatibility with `go test ./internal/server ./internal/inference` and race coverage.
- [x] 6.2 Classify queue/load/generation cancellation and timeouts truthfully; use supported request-local local-runtime budget/compact-context controls without removing configured capabilities or editing defaults. Verify `go test ./internal/integrations ./cmd/offgrid` and preservation fixtures.
- [x] 6.3 Run isolated installed-runtime slow-stream/cancellation tests, applicable full regression checks and documentation/OpenSpec validation; record exact evidence, latency observations, real-model failures and unavailable qualification in the existing reliability evidence.

## 7. Shared model compatibility and latency (approved 2026-10-22)

- [x] 7.1 Separate native external sequential-tool transport from explicitly selected governed single-step transport; validate complete native decisions and stream boundaries, preserve text streaming and token-count parity; test malformed/truncated/multiple calls, cancellation and internal regression fixtures.
- [x] 7.2 Implement cancellation-safe shared model preparation and verify auxiliary cancellation, same-model waiters, all-waiter cancellation, exclusive switching, memory admission and shutdown with unit/race tests.
- [x] 7.3 Remove implicit capability-reducing launch controls; investigate and address supported Hermes auxiliary/prompt overhead without hidden tool removal, changed defaults or upstream patches; verify host/provider and generic-client preservation fixtures.
- [x] 7.4 Run a dynamic eligible-installed-model reply/tool/cancellation matrix and isolated Hermes/OpenClaw workflows with cold/warm timing; run applicable regression/contract/docs checks and record failures and unverified hardware separately. Do not declare all models qualified from fixtures or deploy as part of implementation. Both real file workflows failed independent verification; see evidence.md.

## 8. Tool-template compatibility (approved 2026-10-23)

- [x] 8.1 Implement pinned runtime/template-selected compatibility with native instruction/tool/result preservation, unknown-template passthrough, cache invalidation and token-count parity. Verify `go test ./internal/inference ./internal/server` including stream/nonstream, required/optional tools, malformed/truncated output and governed-protocol regressions.
- [x] 8.2 Run synthetic greeting/abstention and sequential multilingual tool-result cases, then isolated Hermes/OpenClaw exact-file workflows. Run applicable Go/race and documentation/OpenSpec checks; record actual adapter/runtime/model identities, failures and unverified qualification in the existing evidence and guide. Never mark a failed workflow qualified.

## 9. Native sequencing repair (approved 2026-10-23)

- [x] 9.1 Add a revision-bound bundled llama.cpp parser patch and CPU/CUDA build integration. Verify optional/required sequential and parallel native JSON grammar/parser cases with the pinned runtime's parser tests, plus patch mismatch tests and Docker-context checks; document the runtime patch identity. Revision 2 includes the untagged native JSON shape and schema-range enforcement: CPU/CUDA suites each pass 117 tests/583 assertions; reverting the new generator fix reproduces two failures. Real-model activation and workflow verification remain in 9.3.
- [x] 9.2 Add supported Hermes/OpenClaw provider defaults for sequential calls, preserving explicit true/false/null controls, text/auxiliary requests, callbacks, native schemas and all capabilities. Run provider Python/Node tests and `go test ./internal/integrations ./cmd/offgrid` trusted-bundle/host regressions.
- [ ] 9.3 Run applicable Go/race, provider, documentation/hygiene and OpenSpec checks, then separately authorized isolated real greetings and exact-file workflows with the pinned runtime/provider candidate. Record per-model results and unresolved failures; no commits/publication until known workflow failures pass independent verification.

## 10. Result interpretation and outcome fidelity (approved 2026-10-24)

- [x] 10.1 Add an explicit versioned pinned semantic-framing adapter with unchanged native result serialization, schemas and user/system content; preserve v1/native behavior and verify selection, digest, token-count parity and unknown-identity tests with Go unit/race checks.
- [x] 10.2 Freeze and run same-model baseline/candidate synthetic cases for display prefixes, genuine numbered text, Unicode, multiline content, truncation and contradictory read-back/receipts without executing returned tools. Record actual successes/failures; no prompt-only qualification claims.
- [ ] 10.3 Run fresh separately authorized installed Hermes/OpenClaw greeting and exact-file workflows with independent byte/workspace checks, applicable regression/docs/OpenSpec checks, and truthful revision-specific evidence. Keep known failures and the commit gate open until verified.

## 11. Independent host verification (approved 2026-10-24)

- [x] 11.1 Implement a shared read-only verification core with explicit versioned expected outcomes, bounded workspace snapshots, immutable source evidence, correlated proposed/completed file operations and independent final read-back. Test Unicode, genuine prefixes, serialized commands, truncation, scope escape/links, changed sources, event replay, missing hooks, cancellation and false completion using `go test ./internal/integrations/...` and race coverage. Document narrow guarantees and limitations without adding unagreed run options.
- [x] 11.2 Integrate supported Hermes/OpenClaw hook adapters and the approved `--verify-contract <file.json>` invocation interface without upstream patches, changed ordinary runs, silent plugin installation or tool-result rewriting. Use the supported hook-initialized local launch with isolated session state specifically for contracted OpenClaw runs. Test strict contract loading, authenticated loopback transport, actual installed managed-launch lifecycle delivery, hook payloads, fail-open/absent hooks, terminal nonzero outcomes, cancellation, permissions and configuration preservation; run provider and CLI tests and update maintained guidance.
- [x] 11.3 Exercise fresh isolated real greetings and exact-file workflows through the supported hooks and independent oracle; record runtime/model/contract identities, failures and unavailable platforms. Run applicable regression checks. Keep existing known-failure and commit gates open until the corresponding workflows genuinely pass; no deployment or publication is implied.
