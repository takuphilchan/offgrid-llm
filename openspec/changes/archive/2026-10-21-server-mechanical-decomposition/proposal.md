# Proposal

## Why

`internal/server/server.go` is 6,170 lines and currently mixes construction, HTTP lifecycle, route registration, middleware, and many unrelated handlers. This makes security-sensitive route changes hard to review and obstructs the later runtime-composition phase.

## What Changes

- Split existing functions into responsibility-owned files **within** `internal/server`; retain their signatures and behavior.
- Extract route registration from `Start` into readable, private capability registration functions with permission and middleware wrappers visible at the registration point.
- Keep the current `NewWithConfig` constructor and server-owned dependencies in a temporary `construction_legacy.go` until the separately scoped `internal/app` phase.
- Reduce `server.go` to the `Server` type and HTTP lifecycle, targeting approximately 400 lines or fewer without hiding lifecycle code in generic helper packages.
- Freeze and compare route/middleware registrations and run existing server and repository tests before claiming the refactor complete.

No public command, HTTP route, permission, data format, storage location, inference behavior, or desktop behavior changes are intended. This is not the dependency-injection or runtime ownership change planned for Phase 7.

## Capabilities

### New Capabilities

None. This is a structural refactor; `.openspec.yaml` declares `skip_specs: true`.

### Modified Capabilities

None. Existing behavior and contracts must remain unchanged.

## Impact

The change affects `internal/server/server.go`, new files in that same package, focused server tests, and the architecture/contributing documentation. Existing HTTP and API clients, authentication and local IPC boundaries, server construction, persisted state, and installed-user data remain compatible. User-visible acceptance is that the same endpoints, status/error payloads, SSE behavior and authority checks continue to work, while the code layout becomes auditable. No deployment, installed-service replacement, commit, push or release is included.
