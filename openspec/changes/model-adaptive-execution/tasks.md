# Tasks

## 1. Frozen evaluation and baseline

- [x] 1.1 Add a versioned thirty-case manifest covering six families, explicit isolation/setup/evidence/prohibited-effects fields and digest validation; verify `go test ./internal/agents -run TestWorkflowEvaluationManifest -count=1` and document fixture versus qualification semantics.
- [x] 1.2 Record current isolated agent/inference/service-check baseline with `go test ./internal/agents ./internal/inference ./internal/server -count=1`; record failures and unavailable real-model/native baseline separately in the reliability plan, never as successful runs.

## 2. Isolated shared synthetic checks

- [x] 2.1 Implement per-service in-flight synthetic check deduplication, independent cancellation, last-observer cancellation and late-completion protection; verify deterministic concurrency/no-result-cache tests with `go test ./internal/server -run 'TestComputerModel(Check|Probe)' -count=1`.
- [x] 2.2 Add additive admission/probe timing projection to successful and operational check responses, document semantics in the API reference and regenerate OpenAPI types; verify service response tests and `npm --prefix web/app run api:check`.
- [x] 2.3 Verify cancellation/admission integration with `go test -race ./internal/server ./internal/inference` and existing computer submission/held-task regressions; record actual environment/results in the reliability plan.

## 3. Measured capability records

- [x] 3.1 Implement versioned model/runtime/projector fingerprint evidence storage with stale/unknown-version handling and operational versus measured states; verify restart, changed-file/template/context and failed-persistence tests; document evidence lifecycle.
- [ ] 3.2 Integrate synthetic-check evidence with existing model APIs and authenticated CLI/Python/generated clients; verify actor access, cancellation, no private prompt capture and legacy response compatibility.
- [x] 3.3 Normalize native tool-call adapters through the same complete-response gate as constrained output; verify malformed, truncated, duplicate-key and unknown-tool fixtures never dispatch; document supported adapter selection.

## 4. Explicit local model policy

- [ ] 4.1 Add immutable profile execution settings for guided/adaptive mode and installed fallback/perception references without broadening old profiles; verify revision/migration, unavailable-resource and API/client tests and document defaults.
- [ ] 4.2 Persist safe-boundary local switching with two-switch/three-repair root limits and no uncertain-effect replay; verify crash boundaries, missing models, denial, stop and parent/child budget tests; document user-visible switch reasons.
- [ ] 4.3 Integrate measured context projection, parent budget reservations and local admission for parent/child/perception calls; verify CPU/GPU fixtures, one-generation default, cancellation and no cross-actor cache reuse; document latency breakdown.
- [ ] 4.4 Add compact advanced profile/readiness and Chat switch presentations preserving current design, locales and accessibility; verify TypeScript/build and renderer fixtures at narrow/short sizes without duplicate actions.
- [x] 4.5 Replace the fixed normal-operation context clamp with per-model adaptive planning from bounded GGUF metadata, local memory, cache type, slots, power and pressure; pin task allocations and project the same tier through model/integration APIs; verify parser, planner, lifecycle, integration and conversation tests and document fixed versus adaptive operation.

## 5. Integration and qualification

- [ ] 5.1 Run applicable existing CI checks (`go test ./...`, affected race packages, API/check/build, desktop/companion, CLI/Python and isolated service suites), fix regressions and record actual results.
- [ ] 5.2 Run the frozen suite three times per advertised real local model/platform profile, compare single/multi-agent cases, and record success/unsafe/false-completion/latency/memory evidence. Unavailable machines and model runs remain unverified; no release qualification from mocks or compilation.
