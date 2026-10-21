# Tasks

## 1. Baseline and isolation

- [x] 1.1 Preserve the dirty primary worktree and record this clean worktree's commit/status plus CLI help, version, aliases, representative JSON errors, OpenAPI digest, app package identities, installer/unit names, and release artifact names in a baseline evidence file; verify each captured command can be rerun without installed-user data or port 11611.
- [x] 1.2 Run `go mod tidy -diff`, `go test ./cmd/offgrid`, `go test ./internal/server`, and `go test ./... -count=1` twice in the clean worktree; rerun the identified model-operation test at least three times and record exact results, including any flake, without changing product behavior.

## 2. Generated-output boundaries

- [x] 2.1 Add missing ignore entries for `.gocache/`, `.pytest_cache/`, `_work/`, and relevant generated state while retaining local `bin/` and `build/` output; verify `git check-ignore` reports representative generated paths and does not ignore source.
- [x] 2.2 Remove tracked `bin/offgrid` from the index without history rewriting or deleting unrelated local binaries; verify `git ls-files bin build dist` contains no output artifact.
- [x] 2.3 Replace any generation/copy of `.go` source under `build/` with ignored `_work/` or OS temporary staging; verify `rg` finds no maintained generator targeting build-source and `go list ./...` has no `/build/` or `/_work/` package.
- [x] 2.4 Add a tested cross-platform repository-hygiene checker under `scripts/maintenance/` and wire it into `.github/workflows/ci.yml`; verify it passes on the clean source tree and rejects fixtures for tracked output and discovered Go output packages with path-specific errors.

## 3. Discoverable repository guidance

- [x] 3.1 Move `dev/CONTRIBUTING.md` to root `CONTRIBUTING.md` and update maintained links and repository-map documentation; verify `node dev/scripts/check-docs.mjs` and `rg 'dev/CONTRIBUTING.md'` show no maintained stale link.
- [x] 3.2 Add root `SECURITY.md`, `CODE_OF_CONDUCT.md`, and `SUPPORT.md` with accurate private-reporting, issue, and discussion paths; verify every referenced destination exists or is an explicitly identified external project channel, and run the documentation checker.
- [x] 3.3 Add GitHub bug, feature, docs, and model/runtime issue templates plus a pull-request template covering tests, docs, compatibility, state, and security; verify template syntax/required fields with a local parser or fixture test and do not add `CODEOWNERS`.

## 4. Remove obsolete development surfaces

- [x] 4.1 Compare `dev/Dockerfile` and `dev/docker-compose.yml` against active Docker/CI flows, update any maintained links, and remove the obsolete Go 1.21/version 0.1.0 files only after coverage is confirmed; verify Docker configuration and documentation checks still pass.
- [x] 4.2 Confirm `dev/INSTALLER_UPDATE.md` has no maintained unique procedure, then remove it and verify maintained references are absent.
- [x] 4.3 Compare both `.github/workflows/*.disabled` snapshots with active release workflows, carry forward any unique maintained behavior, then remove the disabled snapshots; verify release workflow syntax and release-context tests pass and historical release notes remain unchanged.

## 5. Installer convergence

- [x] 5.1 Inventory options, environment variables, GPU/CPU selection, download names, installed paths, autostart, systemd behavior, and failures across root `install.sh`, `scripts/install.sh`, and `dev/install.sh`; add isolated compatibility fixtures that prove both local and piped legacy invocation without performing installation.
- [x] 5.2 Port any supported unique legacy behavior into root `install.sh` or focused development helpers, then replace `scripts/install.sh` with a streaming-safe compatibility wrapper; verify Bash syntax, isolated installer fixtures, and unchanged unit/install/asset names. Keep the old implementation until parity tests pass.
- [x] 5.3 Move maintained unit sources into `installers/systemd/`, update all maintained installer/helper references and docs to root `install.sh`, and verify unit contents and installed names against the baseline plus docs/installer smoke tests in isolated state.

## 6. Cross-layer integration and review

- [x] 6.1 Run `go test ./...`, focused race coverage, `go list ./...`, documentation/repository checkers, relevant installer/Docker/release tests, `git diff --check`, and `openspec validate repository-maintainability-baseline --strict`; record commands, outcomes, and unavailable platform tests in the evidence file.
- [x] 6.2 Review the Phase 1 diff against the recorded CLI/API/package/release baseline and both worktrees, confirm no user-facing contract or installed-user state changed, and record unresolved findings before starting `application-monorepo-layout`; no commit, push, deployment, or release is part of this task.
