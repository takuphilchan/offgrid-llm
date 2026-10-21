# Design

## Context

See `proposal.md` for motivation and `docs/advanced/client-contracts.md` for CLI output, exit-code, authentication, and cancellation contracts. `cmd/offgrid/main.go` currently contains about 7,500 physical lines and all root dispatch, server startup, terminal formatting, model operations, legacy commands, and logs. Nearby files already separate newer service-backed commands; this change must leave them and concurrent agent/provider work intact. Phase 3 preserved Python/tooling paths and verified the CLI baseline.

## Goals / Non-Goals

**Goals:** Make individual responsibilities reviewable while retaining one `package main`, identical command names/aliases, default service startup, global flags, help text, JSON shapes, terminal fallback, and exits. Keep file imports local and preserve executable build behavior.

**Non-Goals:** No `internal/cli` package, dependency injection, command rewrites, server lifecycle redesign, `offgrid chat` fix, API/schema change, authorization change, installation, or release. Those belong to later phases or other changes.

## Decisions

1. **Capture an exact baseline before extraction.** Build the CLI from this worktree and record version, normalized help, invalid-usage JSON, aliases, and representative dry/safe commands. Reuse and extend existing `cmd/offgrid` tests. A mechanical diff and equal outputs are the acceptance evidence; line-count reduction alone is not.
2. **Move declaration blocks, not behavior.** Keep each extracted file in `package main`. Move complete top-level declarations and their adjacent comments intact; retain signatures, control-flow statements, error text, `os.Exit` calls, and global flag processing. `main.go` will hold the `Version` link and `main()` entry call; the old body can live in a root dispatch function with the same logic. `server_command.go` owns the current default/`serve` path without changing runtime construction yet. This separates presentation from runtime behavior without prematurely imposing the Phase 5 environment interface.
3. **Scope imports and globals carefully.** Use Go formatting/import checks after each group. Go's package initialization order can depend on declaration/file ordering, so keep stateful initialization together, inspect dependencies, and cover startup paths. Do not duplicate package globals or change when they are initialized.
4. **Choose responsibility-oriented groups.** Use the attached plan's target names where they fit, but do not split a single function merely to meet a visual limit. `handleRun` currently spans roughly 800 lines, so `run_chat.go` may temporarily exceed the ~600-line guide with an explicit reason; behavioral decomposition is deferred to Phase 5. Existing separate command files are not re-extracted into duplicate names.
5. **Preserve a reversible source boundary.** Perform a mechanical file split that Git can review as moves plus declarations, keep tests passing in increments, and make no persisted data or service migration. If a split changes behavior, repair or revert that split only; do not compensate by changing CLI semantics. No installed-user state is touched.

## Risks / Trade-offs

- **Imports or implicit initialization drift** -> Compile after each extraction, inspect globals and preserve declaration dependencies.
- **Hidden command/alias/output drift** -> Compare golden CLI captures and run existing command tests before and after; preserve the placeholder `chat` path as-is.
- **Large mechanical diff masks concurrent edits** -> Restrict edits to `cmd/offgrid` and the change's evidence/tests; keep byte-identical function bodies where possible and review `git diff` by responsibility.
- **Terminal or server side effects in tests** -> Use isolated configuration, temporary paths, and safe commands; do not run the default service against the installed workspace simply to capture output.

## Migration Plan

Record baseline and current dirty-file ownership. Extract version/terminal/progress/root/server groups, then model/run/diagnostics groups, then legacy administration/integration/audio/logs groups. After each group run `gofmt`, `go test ./cmd/offgrid`, and relevant CLI comparisons. Finish with `go test ./...`, builds and repository checks; record unavailable race/platform checks honestly. This is source migration only. Rollback is at the scoped source-diff level before any release; no stored-state rollback is needed.
