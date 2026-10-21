# Proposal

## Why

The Phase 4 split made the command source readable but left dispatch coupled to
global process state: 91 production `os.Exit` calls, 53 `os.Args` references,
global output mode, direct terminal/network/process access, and default server
construction inside the command package. This makes CLI behavior difficult to
test without a real process or installed workspace. The repository plan calls
for a single, injected CLI execution boundary before server decomposition.

## What Changes

- Introduce `internal/cli` with a typed execution environment, command result
  boundary, and interrupt-aware `Execute(ctx, args, env) int` entry point.
- Move command families from `cmd/offgrid` only after their I/O, process,
  configuration, and service dependencies are injected and covered by tests.
- Make `cmd/offgrid/main.go` the OS adapter and sole process-exit owner.
- Preserve command names/aliases, default server behavior, exit codes, JSON
  shapes, terminal text, cancellation, authentication, and existing data paths.
- Keep service-backed commands on `internal/serviceclient`; do not create a
  second CLI executor or reimplement server-owned state transitions.
- Preserve the existing local-administration boundary: `users`, `audit`, and
  `config` continue to operate on the selected local data/configuration paths,
  while `metrics` continues to read the existing local endpoint. Moving these
  commands does not create service APIs or change their authorization.

## Capabilities

### New Capabilities

None. This change is an internal refactor and opts out of delta specs.

### Modified Capabilities

None. The existing user-visible CLI contract remains unchanged.

## Impact

`cmd/offgrid`, new `internal/cli`, CLI tests, and contributor/architecture
documentation. No HTTP schema, persistence, model/provider, desktop IPC,
installation, or release behavior changes. Existing work in inference-provider
and agent/computer changes remains outside scope.
