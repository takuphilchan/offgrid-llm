# Proposal

## Why

The completed server split makes HTTP routes reviewable, but `internal/server/construction_legacy.go` still creates databases, inference runtimes, background services and transport as one concrete constructor. This couples startup ownership to HTTP and leaves partially initialized `Server{startupErr: ...}` instances; the next repository phase needs one atomic, testable composition owner.

## What Changes

- Introduce `internal/app` as the sole broad runtime-composition package, owning concrete creation, reverse-order cleanup and one service lifetime.
- Make `internal/server` consume explicit dependencies and own HTTP routing, serving and shutdown, rather than constructing concrete stores and model runtimes.
- Migrate the default `offgrid serve` and no-command startup path to the composed runtime, with compatibility adapters only where tests or existing callers require them during the transition.
- Replace half-initialized startup objects with explicit errors after caller migration, and test failure cleanup and idempotent shutdown.
- Preserve all public CLI, HTTP, SSE, WebSocket, desktop, workspace, permission and installed-data behavior.

This is an internal refactor. It does not add a new executor, change providers, migrate storage, install models, deploy, or publish a release.

## Capabilities

### New Capabilities

None. This change is marked `skip_specs: true` because it must not change external behavior; maintained API and repository requirements remain authoritative.

### Modified Capabilities

None.

## Impact

The primary affected files are `internal/server/construction_legacy.go`, `internal/server/server.go`, `cmd/offgrid/legacy_server_start.go`, their fixtures, and new `internal/app` source. Architecture, repository-map and contributor documentation will describe the actual post-move owner. Tests use disposable workspace paths; installed user state and the running desktop/service are out of scope.
