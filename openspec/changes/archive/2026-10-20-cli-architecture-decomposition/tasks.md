# Tasks

## 1. Baseline and extraction map

- [x] 1.1 Record current `cmd/offgrid` dirty state, `main.go` declaration inventory and physical line count, `go test ./cmd/offgrid`, a built CLI's `--no-color --help`, `version`, representative `--json` invalid-usage response/exit code, aliases, and safe `chat`/`run` behavior in `baseline-evidence.md`; verify the captured binaries come from this worktree and do not access the installed service.
- [x] 1.2 Map every top-level declaration in `cmd/offgrid/main.go` to a target file while noting existing sibling command files and initialization dependencies; verify no declaration is unassigned or assigned twice before moving code, and record the map in `baseline-evidence.md`.

## 2. Root and presentation extraction

- [x] 2.1 Move version linkage and `getVersion` into `version.go`, root dispatch into `root.go`, and current default/`serve` startup into `server_command.go` while retaining a minimal `main.go`; verify `gofmt -l (Get-ChildItem cmd/offgrid -Filter *.go).FullName` is empty in PowerShell, `go test ./cmd/offgrid` passes, and version/help/default-command fixtures match the baseline without starting a personal service.
- [x] 2.2 Move terminal formatting and progress declarations into `terminal_output.go` and `progress.go` without altering output or timing logic; verify `go test ./cmd/offgrid` and normalized help/terminal-output fixtures match the baseline.

## 3. Command responsibility extraction

- [x] 3.1 Move model download/import/remove/export/quantization/search and model helper declarations into `model_local.go`, `model_remote.go`, `model_transfer.go`, and existing `model_search.go` as appropriate; preserve existing separate model command files. Verify `go test ./cmd/offgrid`, representative model command aliases/JSON errors, and `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid`.
- [x] 3.2 Move run/chat and benchmark declarations into `run_chat.go` and `benchmark.go` without changing the `run` TUI/plain fallback or placeholder `chat`; document any indivisible >600-line function and verify `go test ./cmd/offgrid` plus safe run/chat/bench fixtures against the baseline.
- [x] 3.3 Move doctor/init/info/config/users/audit/metrics and related diagnostics into `diagnostics.go`, `config_command.go`, `users_command.go`, `audit_command.go`, and `metrics_command.go`; verify `go test ./cmd/offgrid` and diagnostic/config/user command routing and JSON-error fixtures.
- [x] 3.4 Move audio, legacy agent, legacy integration, logs and other remaining command declarations into `audio_command.go`, `agent_legacy.go`, `integration_legacy.go`, `logs_command.go` or narrowly named siblings; verify `go test ./cmd/offgrid`, source declaration inventory has no missing/duplicate symbol, and root aliases still route identically.

## 4. Integration and review

- [x] 4.1 Compare post-extraction golden output/exit codes and the safe route matrix with `baseline-evidence.md`; run `go test ./cmd/offgrid -count=1`, `go test ./...`, `go build -trimpath -o build/dev/offgrid.exe ./cmd/offgrid`, and `node scripts/maintenance/check-repository-hygiene.mjs`; record real outcomes and any unavailable check in `verification-evidence.md`.
- [x] 4.2 Update affected contributor/architecture docs only for source-file ownership, not user-visible command semantics; verify `node scripts/maintenance/check-docs.mjs` reports zero broken maintained links and `openspec validate cli-architecture-decomposition --strict --no-interactive` passes.
- [x] 4.3 Review the final declaration map, `main.go` line count, extracted file sizes, staged/unstaged diff, CLI compatibility, and absence of installed-user side effects; record exceptions and review findings in `verification-evidence.md` before considering archive. No commit, push, deployment, desktop replacement, or publication is authorized by this task.
