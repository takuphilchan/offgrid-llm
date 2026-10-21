# Design

## Context

See [proposal.md](proposal.md), [architecture](../../../../docs/advanced/ARCHITECTURE.md) and [repository structure](../../../../docs/repository-structure.md). Phase 6 left `Server`, `Start`, `Close` and the single listener in a 314-line `server.go`, but `construction_legacy.go` still acquires workspace ownership and constructs the entire graph. `cmd/offgrid/legacy_server_start.go` calls `server.NewWithConfig(cfg).Start()`. In-package server tests use `newTestServer`, which currently calls that same concrete constructor and often mutates private fields. The ownership tests explicitly rely on a failed constructor returning a `Server` whose later `Start` reports `startupErr`.

This is a compatibility refactor, not permission to change routes, storage layout, scheduling or native authority. The route fixture and full Go suite from Phase 6 are the baseline.

## Goals / Non-Goals

**Goals:** `internal/app` owns the one concrete runtime graph and reverse-order cleanup; `internal/server` consumes typed dependencies and owns HTTP; startup fails atomically with an error; CLI default and `serve` use the same runtime. Keep existing tests expressive while moving them away from an all-purpose concrete constructor.

**Non-Goals:** New public API, new service process, new inference engine, storage migration, model download, desktop replacement, a generic dependency container or a second startup path. Platform qualification and deployment remain separate.

## Decisions

1. **One-way dependency direction.** `internal/app` imports `internal/server` and lower-level modules; `internal/server` never imports `internal/app`. A typed `server.Dependencies` (not `map[string]any`) carries only what HTTP handlers and the server lifecycle consume. Narrow interfaces live at the consuming boundary where practical; concrete types remain where existing behavior requires them. Do not create adapter layers merely to hide every field at once. Alternative—moving `Server` into `app`—would entangle transport with composition and break current route fixtures.
2. **Atomic composition and explicit failure.** `app.New(ctx,cfg)` validates configuration, acquires workspace ownership before mutable stores, then creates services in the current observed order. Every acquired resource registers a cleanup callback immediately; any failure runs callbacks in reverse order and joins cleanup errors with the original cause. A returned `Runtime` is fully usable. `server.New(HTTPConfig, Dependencies)` validates required dependencies without acquiring or starting concrete resources and returns `(*Server,error)`. Transitional `NewWithConfig` exists only while callers/tests migrate; it is removed before completion, together with `startupErr`.
3. **Ownership is singular.** `app.Runtime` owns stores, inference processes, monitors, provider scheduler, agents and their stop order. `server.Server` owns HTTP listener, handler chain, request draining and server-started workers. `Runtime.Close` first stops/awaits HTTP and its workers, then releases components in reverse construction order, including workspace ownership last. Both closes are idempotent; context cancellation never turns an uncertain drain into an acknowledged safe stop. Alternative—both layers closing the same components—is rejected because it creates races and double cleanup.
4. **Keep the HTTP contract frozen.** `Server.Handler()` returns the existing `buildRouter()` chain; `Serve(ctx)` builds one listener and handles cancellation while preserving the current timeouts, exposure checks and startup ordering. The existing `Start()` compatibility path is removed only after production and test callers migrate. Route registrations, wrappers, error payloads, SSE flush and WebSocket behavior stay unchanged. Treat the Phase 6 fixture as immutable unless an independently authorized behavior change is made.
5. **Tests are migrated by responsibility.** Pure handler tests build `server.Dependencies` with explicit disposable fixtures or fakes in `package server`; integration/ownership tests that need the complete graph move to `internal/app` or external `server_test` and call `app.New`. Do not create a broad test-only constructor that duplicates production composition. Existing direct private-field tests are reviewed and converted to explicit setup, not made to import `app` from `package server` (which would form a Go import cycle).
6. **Compatibility adapter is temporary and measurable.** Migrate `cmd/offgrid` last, after `app.Runtime` passes startup/cleanup tests. During transition keep exactly one actual construction implementation and compare old/new behavior in isolated tests; do not launch the installed service or port 11611. Remove the adapter and its `startupErr` pattern when all callers are switched. No saved user data changes are required.

## Risks / Trade-offs

- A large graph move can miss cleanup on a middle failure → instrument every constructor step in isolated fault-injection tests and assert reverse-order cleanup, ownership release and no child-process leak.
- Tests reach into the old concrete `Server` bag → migrate by test responsibility before deleting the compatibility constructor; use typed fixture helpers, not a second production composer.
- Starting workers before listener bind can leave effects after a bind failure → keep the observed order until tests capture it, then ensure `app.New`/`Run` cleanup drains all started workers without changing public status.
- A new `Dependencies` struct can become a service locator → group by actual consumer capability, prohibit `any`/string-keyed lookup, and review additions with handler ownership.
- Import cycles between `server`, `app` and test code → enforce one-way imports with `go list ./...` and package-external integration fixtures.
- The existing dirty worktree includes provider/agent changes → move the current source, not `HEAD`, and run the complete Go suite and frozen route check after each extraction stage.

## Migration Plan

Freeze constructor order, ownership and CLI startup fixtures. Add typed server dependency construction and `Handler`/`Serve` without changing routes. Move graph creation into `internal/app` in small owned clusters with a cleanup stack. Migrate server and CLI tests/callers; remove `construction_legacy.go` and `startupErr` only after all tests use the new path. Update architecture/contributing docs to the actual owner. Run full Go, race where available, build, API/desktop compatibility and isolated startup tests. No deployment or user-data migration is part of this source change; before deployment, the old committed executable remains the recoverable path.
