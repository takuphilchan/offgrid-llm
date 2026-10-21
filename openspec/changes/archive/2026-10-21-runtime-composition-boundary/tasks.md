# Tasks

## 1. Freeze ownership and startup behavior

- [x] 1.1 Record the current `construction_legacy.go` creation/cleanup order, `startupErr` branches, concrete resources and external callers in `baseline-evidence.md`; verify with `rg -n 'NewWithConfig|startupErr|\.Close\(' internal/server cmd/offgrid` and the Phase 6 route fixture.
- [x] 1.2 Add isolated startup failure, ownership release, listener conflict and idempotent-close baseline tests using `t.TempDir()` and unused ports; verify `go test ./internal/server ./cmd/offgrid -count=1` passes without installed-user state.

## 2. Introduce the typed transport boundary

- [x] 2.1 Define `server.HTTPConfig` and typed `server.Dependencies` from the actual handler/lifecycle consumers, with required-dependency validation and no `any` locator; verify focused constructor tests and `go test ./internal/server -count=1`.
- [x] 2.2 Add `Handler()` and context-aware `Serve(ctx)` around the existing router, middleware and one listener, retaining response, timeout and exposure behavior; verify the frozen route fixture, isolated HTTP/SSE tests and `go test ./internal/server -count=1`.
- [x] 2.3 Document the temporary old/new transport boundary and authority review rule in architecture/contributing docs; verify `node scripts/maintenance/check-docs.mjs` and router tests pass.

## 3. Move concrete graph composition into `internal/app`

- [x] 3.1 Add `app.Runtime` and a reverse-order cleanup stack with idempotent close; test failure injection at each acquisition boundary, joined errors and reverse-order release with `go test ./internal/app -count=1`.
- [x] 3.2 Move config validation, workspace ownership, data layout, registry/model cache, resource monitor and inference/provider setup from the current working-copy constructor into `app.New` without changing creation order; verify isolated ownership/model startup tests and `go test ./internal/app ./internal/server -count=1`.
- [x] 3.3 Move users, sessions, RAG, audio, agents, computer, integrations, metrics and optional-service wiring into `app.New`; verify cancellation, failed-initialization cleanup, no eager sandbox, and `go test ./internal/app ./internal/server -count=1`.
- [x] 3.4 Wire `Runtime.Run(ctx)` and `Runtime.Close()` to transport drain followed by component cleanup, preserving uncertain-drain ownership; verify crash/bind/cancel/repeated-close tests and `go test ./internal/app ./internal/server -count=1`.
- [x] 3.5 Update architecture and repository maps to name `internal/app` as current composition owner only after the move lands; verify `node scripts/maintenance/check-docs.mjs` reports zero problems.

## 4. Migrate callers and fixtures without duplicate composition

- [x] 4.1 Convert in-package HTTP tests to explicit disposable dependency fixtures, moving complete-graph integration cases to `internal/app` or external tests without an import cycle; verify `go test ./internal/server ./internal/app -count=1`.
- [x] 4.2 Migrate `cmd/offgrid` default and explicit `serve` paths to one `app.New`/`Run` lifetime while preserving Ctrl+C and exit/error behavior; verify `go test ./cmd/offgrid -count=1` and isolated CLI startup tests.
- [x] 4.3 Remove production `NewWithConfig`, `startupErr` and `construction_legacy.go` after all callers migrate; verify `rg -n 'NewWithConfig|startupErr' internal/server cmd/offgrid` has no production matches and `go test ./... -count=1` passes.
- [x] 4.4 Remove transitional docs, document the final import and ownership direction, and verify `node scripts/maintenance/check-docs.mjs` plus `go list ./...` pass.

## 5. Cross-surface verification and honest handoff

- [x] 5.1 Run `go test ./... -count=1` twice, `go test -race ./internal/app ./internal/server ./cmd/offgrid -count=1` where supported, and `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid`; record exact results and unsupported checks in `verification-evidence.md`.
- [x] 5.2 Run the frozen route/HTTP baseline, `npm.cmd --prefix apps/web run api:check`, relevant desktop compatibility tests, `node scripts/maintenance/check-docs.mjs`, and `git diff --check`; record no route/schema/user-data changes or specific findings.
- [x] 5.3 Review one constructor/one listener, cleanup ownership, import direction, no installed-user effects and active provider/agent overlap; run `openspec validate runtime-composition-boundary --strict --no-interactive` and record blocking findings before archiving. Deployment, desktop replacement, commit and publication remain separate actions.
