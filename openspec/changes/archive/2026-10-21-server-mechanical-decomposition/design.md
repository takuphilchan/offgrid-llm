# Design

## Context

See [proposal.md](proposal.md), [repository structure](../../../../docs/repository-structure.md) and [architecture](../../../../docs/advanced/ARCHITECTURE.md). `internal/server/server.go` currently contains the `Server` state, `NewWithConfig`, a 300-plus-line `Start` containing route registration and HTTP startup, `Close`, middleware and handlers through line 6,170. Other agent, audio, RAG, session, browser and computer handlers are already separate files. Phase 5 leaves `cmd/offgrid` using this constructor through a private adapter; Phase 7, not this change, will move concrete dependency composition into `internal/app`.

## Goals / Non-Goals

**Goals:** Make HTTP registration, its authority wrappers and lifecycle reviewable; keep functions in the same Go package with unchanged signatures; make `server.go` approximately 400 lines or fewer; establish repeatable before/after route and response evidence.

**Non-Goals:** Change route paths or methods, auth/rate/body-limit ordering, constructor behavior, engine scheduling, handler semantics, storage, API schemas, installed service state or public interfaces. No new generic handler/service/utils package and no premature `internal/app` shell.

## Decisions

1. **Freeze behavior before moving code.** Record the current route list, registration order, permission wrapper, rate-limit wrapper and outer middleware chain from `Start`. Use a checked fixture and focused HTTP cases for public, authenticated, forbidden, oversized-body, SSE and error-payload paths. Static route comparison catches omissions; HTTP tests catch behavioral mismatch. Compilation alone is insufficient.
2. **Move functions within `package server`, without rewriting algorithms.** Move `NewWithConfig` and constructor-owned helpers to `construction_legacy.go`; move handlers to capability-named files such as `health_handlers.go`, `system_handlers.go`, `inference_handlers.go`, `model_handlers.go`, `download_handlers.go`, `usb_handlers.go`, `user_handlers.go`, `plugin_handlers.go` and `ui_handlers.go`. Keep already-focused files intact. If a target would itself become an oversized mixed file, use a narrower capability name (for example agent MCP versus agent tools) rather than `common.go`.
3. **Extract registration without changing policy.** `router.go` builds the existing `http.ServeMux` through private `register*Routes` functions. Group names aid review; each `Handle`/`HandleFunc` line retains its exact path and wrappers, and registration order is frozen for overlapping patterns. Keep the outer chain explicit and in its present order: auth around mux, browser transport, request-body limit, then logging outermost. `middleware.go` owns the mechanics; `exposure.go` owns loopback/exposure checks.
4. **Keep one server constructor and one listener.** `server.go` retains the type and HTTP lifecycle. It calls the extracted router and unchanged startup/cleanup helpers. `construction_legacy.go` still calls the same concrete stores, model cache, registry and services; `cmd/offgrid` keeps its Phase 5 compatibility adapter. This avoids a second partially initialized runtime while Phase 7 is unfinished.
5. **Keep effectful behavior and recovery unchanged.** `Start` must still seal native installation before startup, reject `startupErr` and invalid exposure, build the same handler, start the same workers, bind once and serve. `Close` remains idempotent and preserves uncertain drain/lock ownership. Tests use disposable state; no running user service is recycled for this structural refactor.

## Risks / Trade-offs

- **A moved route loses a wrapper or changes order** → compare the frozen registration manifest and exercise status/auth/rate/body/SSE fixtures before and after each registration group.
- **Cross-file moves accidentally change side effects or startup ordering** → move one cohesive region at a time, use mechanical diffs and run `go test ./internal/server -count=1` between regions.
- **Hidden dependencies surface as package cycles** → stay in `package server`; do not introduce subpackages during extraction.
- **A short `server.go` merely relocates a monolith** → require focused file ownership and review file sizes; permit narrower capability files rather than a giant `handlers.go`.
- **Concurrent provider/agent edits overlap this file** → preserve those edits, inspect the dirty diff before each extraction, and rerun relevant package/full Go tests. No Git reset or installed-user state cleanup.

## Migration Plan

Capture source and isolated HTTP baselines. Extract constructor helpers, then route registration and middleware, then handlers by capability, then lifecycle helpers only as necessary for the file-size target. Format and run focused checks after each group; run the full Go suite and API-contract/documentation checks at the end. There is no storage migration or deployment in this change. Rollback, if needed before any deployment, is a scoped source revert of the affected extraction, not deletion of task data or state.
