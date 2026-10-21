# Tasks

## 1. Freeze the current CLI contract

- [x] 1.1 Record the Phase 4 executable hash, safe output/exit fixtures, root aliases, default-startup path, `os.Exit`/`os.Args` inventory and preexisting dirty files in `baseline-evidence.md`; verify the fixture binary is built from this worktree and uses no personal service.
- [x] 1.2 Add isolated subprocess fixtures for global flag order, `NO_COLOR`, invalid usage, Ctrl+C, authenticated service errors and representative aliases; verify `go test ./cmd/offgrid -count=1` passes before boundary changes.

## 2. Environment and result contract

- [x] 2.1 Add `internal/cli.Environment`, `CommandRunner`, invocation options and `Execute(ctx,args,env) int` with explicit dependency validation; verify table tests cover missing/overridden streams, clients, getenv/setenv, execution and cancellation without touching OS state.
- [x] 2.2 Move command error, JSON and terminal presentation helpers behind injected streams/options; verify exact output/exit fixtures and `go test ./internal/cli ./cmd/offgrid -count=1`.
- [x] 2.3 Parse global flags from an immutable argument copy and dispatch aliases through one registry; verify all 47 original case labels, output hashes, flag permutations and no mutation of `os.Args` or process environment in direct tests.

## 3. Service-backed and read-only command families

- [x] 3.1 Move version/help/doctor/info/search and read-only model presentation into `internal/cli` using injected dependencies; verify direct unit tests and baseline subprocess captures for success and invalid usage.
- [x] 3.2 Move session/knowledge/agent/profile/computer service-backed commands, retaining `internal/serviceclient` authentication, redirect rejection and cancellation; verify isolated HTTP fixtures for 0/1/2/130 exits, JSON shapes and no local-file fallback.
- [x] 3.3 Move users/audit/metrics/config read and mutation routes with explicit error returns and injected local-store, configuration, and HTTP dependencies; preserve their current local authority, paths, metrics endpoint, output and exit behavior. Verify disposable HTTP/config fixtures and current command authorization tests. New actor-scoped service APIs are out of scope.

## 4. Local and external-effect command families

- [x] 4.1 Move model download/import/remove/export/quantization/verification and batch commands with injected filesystem, HTTP and terminal dependencies; verify disposable model-directory fixtures, no installed-state changes, and exact safe alias/error captures.
- [x] 4.2 Move run/chat, benchmark, audio and legacy agent/integration commands; replace direct exits with typed returns and preserve TUI/plain fallback, existing `chat` placeholder, cancellation and output. Verify isolated command tests and baseline captures.
- [x] 4.3 Inject process launching for Hermes, OpenClaw, systemd, browser and native helpers, with bounded reviewed arguments; verify fake-runner tests and no real process spawn in unit tests.

## 5. Process and server boundary

- [x] 5.1 Make `cmd/offgrid/main.go` construct the OS-backed environment, interrupt context and sole `os.Exit` call; remove migrated command implementations from `cmd/offgrid`. Verify `go test ./cmd/offgrid ./internal/cli -count=1`, root alias matrix and no production `os.Exit` elsewhere.
- [x] 5.2 Keep default `serve` behavior through a private legacy server-start adapter pending Phase 7 `internal/app`; verify an isolated no-user-data startup/shutdown fixture and existing server tests, with no second server constructor.
- [x] 5.3 Update `CONTRIBUTING.md` and relevant CLI architecture docs for actual ownership and the deferred `internal/app` handoff; verify `node scripts/maintenance/check-docs.mjs` reports zero problems.

## 6. Integration review

- [x] 6.1 Run `go test ./...`, targeted race coverage where supported, `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid`, full safe-command golden comparison, and `git diff --check`; record exact outcomes and unavailable checks in `verification-evidence.md`.
- [x] 6.2 Review no installed-user side effects, no command/alias/exit/JSON regressions, no duplicate executor, service-client authority, and interaction with active provider/agent changes; validate `openspec validate cli-execution-boundary --strict --no-interactive` and record blocking findings before archive. Deployment, desktop replacement, commit, push and release are separate operations.
