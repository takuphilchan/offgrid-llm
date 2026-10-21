# Design

## Context

See `proposal.md` and `docs/advanced/client-contracts.md`. Phase 4 left a
6-line entry point and 24 responsibility files in `package main`. The root
still rewrites `os.Args`, and legacy handlers directly use process, console,
network, filesystem, and configuration globals. Newer authenticated commands
already use `internal/serviceclient` and a typed `reportCommandError` helper.
The current server starts and owns its signal loop through
`server.NewWithConfig(cfg).Start()`; there is no `internal/app` yet.

## Goals / Non-Goals

**Goals:** One callable, context-aware CLI executor with explicit dependencies;
an OS-only process entry; no command-owned process termination; preserved
outputs, aliases, exits, state paths and service authority. Tests can inject
terminal streams, environment, HTTP clients, process launch, and configuration
without touching installed-user state.

**Non-Goals:** No new command, corrected legacy `chat` implementation, changed
server constructor, provider lifecycle, API/storage migration, or release claim.
Phase 6 splits server handlers; Phase 7 introduces `internal/app` composition.

## Decisions

1. **Own the boundary in `internal/cli`.** Export `Environment` with `Stdin`,
   `Stdout`, `Stderr`, `Getenv`, `Setenv`, `HTTP`, `LongHTTP`, and a typed
   `CommandRunner`; expose `Execute(context.Context, []string, Environment) int`.
   Validate/default only optional fields at one entry. A command has the form
   `func(context.Context, []string, Environment) error`. A command registry
   maps aliases to one handler rather than duplicating root switches. A direct
   copy of the global `package main` state would be easier initially but would
   leave the test and authority problem intact.
2. **Migrate by capability with a compatible oracle.** Move one command family
   at a time only after direct and subprocess fixtures match original text,
   JSON, exit, cancellation and auth behavior. Shared presentation and client
   helpers move before handlers. Keep existing source in place until a family
   is wired, then remove its old route and declaration atomically. Do not leave
   two selectable executors or an internal package importing `cmd/offgrid`.
3. **Return typed outcomes instead of terminating inside commands.** Only
   `cmd/offgrid/main.go` calls `os.Exit`. Structured usage, cancellation and
   operational errors map to existing exit codes 2, 130 and 1 at the boundary.
   Legacy immediate-exit paths must be converted with explicit return and
   cleanup; panic/recover, a subprocess wrapper, or a fake `os.Exit` shim is
   rejected because it hides control flow and skips defers.
4. **Preserve task and service authority.** Service-backed commands keep the
   authenticated `internal/serviceclient` with redirect rejection and
   cancellation; the injected long HTTP client cannot make credentialed calls
   to a different origin. Direct model/download filesystem commands retain
   their current effects until a separate feature change. Process execution
   goes through a narrowly typed runner, never model-supplied arguments.
5. **Parse invocation without mutating the process.** Global flags become
   invocation options derived from a local copy of `args`. Output mode and
   color choice are per-execution, not global `os.Args`/`NO_COLOR` mutations.
   Existing external `NO_COLOR` behavior remains, and subprocess fixtures
   cover precedence. Shared mutable terminal state is removed or scoped before
   claiming concurrent `Execute` safety.
6. **Keep server composition in its owning phase.** Phase 5 retains a private
   compatibility adapter for the existing `server.NewWithConfig(cfg).Start()`
   path. Phase 7 replaces this adapter with `internal/app`. Creating a hollow
   `internal/app` wrapper now would contradict the plan's ownership rule and
   add a second half-initialized lifecycle.
7. **Preserve local-administration authority.** `users`, `audit`, and `config`
   keep their existing local stores and selected paths, and `metrics` keeps its
   existing local HTTP endpoint and access behavior. Inject these dependencies
   so they can be tested with disposable state. Do not invent actor-scoped
   audit/configuration endpoints or change `/metrics` authentication in this
   structural change. Any such policy or HTTP-contract change needs a separate
   proposal and acceptance scenarios.

## Risks / Trade-offs

- **91 direct exits and many global writes** → migrate family by family; no
  halfway route replacement; compare built-command bytes and exits.
- **Returning instead of exiting may run defers that previously did not** →
  inspect resource ownership and outcome timing on each converted path.
- **Signal behavior changes** → test Ctrl+C with isolated child processes;
  do not weaken Stop or service-owned shutdown.
- **Injected stdout/stderr diverges from helpers printing globally** → move
  presentation dependencies before wiring a command into `internal/cli`.
- **Concurrent execution touches shared globals** → treat concurrency as
  unsupported until mutable state is eliminated and race tests pass.

## Migration Plan

Freeze the Phase 4 safe-command binary and existing CLI tests. Introduce the
environment/result contract and unit tests, then migrate root flags and
read-only/service-backed families, then local model and external-process
families, then default server startup. After each family, compare exact safe
output/exit fixtures and run its tests. End with `go test ./...`, CLI subprocess
coverage, docs and CI checks. Rollback is a scoped source revert before any
deployment; no stored-state migration occurs. The installed service and desktop
are not used for tests.
