# Tasks

## 1. Freeze the server contract

- [x] 1.1 Record the pre-move `server.go` size, constructor/handler inventory and current dirty-file overlap in `baseline-evidence.md`; verify the recorded inventory against `rg -n '^func ' internal/server/server.go` and `git status --short` without touching installed state.
- [x] 1.2 Capture ordered route paths, handlers, permission/rate wrappers and outer middleware order in a checked fixture; add a comparison test that fails on omission or wrapper drift, then verify `go test ./internal/server -count=1` passes before moving code.
- [x] 1.3 Add isolated HTTP baseline cases for health, authenticated/forbidden routes, body limits, representative error payloads and SSE headers/flush behavior using disposable state; verify targeted server tests pass and record what native or live behavior they do not prove.

## 2. Constructor and lifecycle ownership

- [x] 2.1 Move `NewWithConfig` and its concrete-construction helpers into `construction_legacy.go` without changing call order or the `startupErr` compatibility behavior; verify focused construction/startup/close tests and `go test ./internal/server -count=1` pass.
- [x] 2.2 Move model-cache/loading and background-startup helpers into narrow same-package files, preserving cancellation and effect order; verify focused inference/server tests and `go test ./internal/server -count=1` pass.
- [x] 2.3 Keep `Server`, `Start` and `Close` in `server.go` with one constructor and listener; update architecture documentation to identify temporary `construction_legacy.go` ownership and the deferred Phase 7 handoff, then verify `node scripts/maintenance/check-docs.mjs` and focused lifecycle tests pass.

## 3. Router, middleware and exposure

- [x] 3.1 Extract `router.go` with private capability registration functions, preserving the frozen ordered route/wrapper manifest exactly; verify its comparison test and `go test ./internal/server -count=1` pass.
- [x] 3.2 Extract request/logging/status middleware into `middleware.go` and exposure/loopback checks into `exposure.go` without reordering the outer chain; verify isolated auth/body-limit/remote-exposure tests and the frozen manifest pass.
- [x] 3.3 Document the route-registration ownership and security wrapper review rule in the relevant architecture/contributing docs; verify `node scripts/maintenance/check-docs.mjs` reports zero problems and route tests still pass.

## 4. Read, inference and model handlers

- [x] 4.1 Move root, health, readiness, power, stats and system handlers to `health_handlers.go` and `system_handlers.go`; verify their existing focused tests plus `go test ./internal/server -count=1` pass with unchanged response fixtures.
- [x] 4.2 Move chat/completions, embeddings, SSE and terminal handlers to capability-named inference files without changing streaming or error mapping; verify streaming, cancellation, rate-limit and error-payload tests pass.
- [x] 4.3 Move model list/cache/catalog, quantization, hot-swap and load-balancer handlers to focused model files; verify model/package, cache and permission tests plus `go test ./internal/server -count=1` pass.

## 5. Effects, administration and remaining handlers

- [x] 5.1 Move download/verify/progress/cancel handlers to `download_handlers.go` and USB import/export plus filesystem handlers to focused files; verify download, cancellation, file-boundary and response tests pass without writing installed models.
- [x] 5.2 Move user/auth/quota and plugin handlers to `user_handlers.go` and `plugin_handlers.go`; verify auth, role, quota and plugin endpoint tests pass with unchanged status and payloads.
- [x] 5.3 Move UI/template, legacy agent tools/MCP, WebSocket, P2P and distributed-RAG handlers to existing or narrowly named capability files; verify their focused tests and `go test ./internal/server -count=1` pass with the frozen route manifest unchanged.
- [x] 5.4 Confirm every former `server.go` function has exactly one definition in `package server`, no generic catch-all file was created, and `server.go` is approximately 400 lines or fewer; verify `go test ./internal/server -count=1` and a source inventory recorded in `implementation-evidence.md`.

## 6. Integration review

- [x] 6.1 Run `go test ./... -count=1`, targeted `go test -race ./internal/server -count=1` where supported, `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid`, the frozen route comparison and `git diff --check`; record exact outcomes and unavailable checks in `verification-evidence.md`.
- [x] 6.2 Review constructor count, route/middleware/authority identity, lifecycle/error/SSE behavior, overlap with active provider/agent work, and no installed-user side effects; run `openspec validate server-mechanical-decomposition --strict --no-interactive` and record blocking findings before archive. Phase 7 composition, deployment, desktop replacement, commits and publication remain separate operations.
