# Design

## Context

See [proposal.md](proposal.md). The clean structural worktree starts at commit `68d36b2`; the primary worktree remains dirty with active provider, native-workflow, web, and desktop changes. Current `.gitignore` already excludes `bin/`, `build/`, `__pycache__/`, and `*.egg-info/`, but `bin/offgrid` is tracked and `.gocache/` and `.pytest_cache/` are not excluded. `dev/Dockerfile` uses Go 1.21 and `dev/docker-compose.yml` uses version `0.1.0-alpha`; active Docker files are under `docker/`. Two disabled release workflow snapshots exist beside active workflows. Root `install.sh`, `scripts/install.sh`, and `dev/install.sh` implement materially different flows. The current public docs still link to `scripts/install.sh` and `dev/CONTRIBUTING.md`.

The maintained architecture and readiness documents remain [ARCHITECTURE.md](../../../../docs/advanced/ARCHITECTURE.md), [maintainability.md](../../../../docs/advanced/maintainability.md), and [production-readiness.md](../../../../docs/advanced/production-readiness.md). This change does not replace them.

## Goals / Non-Goals

**Goals:** Establish a reproducible compatibility baseline; prevent tracked/generated output from reappearing; expose community guidance; reduce stale deployment entry points; preserve both public installer URLs and installed identities.

**Non-goals:** Move application, CLI, server, or Python source; alter an installer feature; migrate installed data; remove historical Git objects; publish or deploy. The subsequent five structural changes require separate acceptance and review.

## Decisions

### Isolate structural edits in a separate Git worktree

The main worktree's uncommitted changes are not moved, reset, stashed, or committed. The structural branch is based on the current committed baseline. This avoids overwriting active work, but it also means later integration must explicitly reconcile both sets of edits before application-path moves. Alternative: edit directly in the dirty main worktree; rejected because the plan prohibits moving overlapping files before safe isolation.

### Capture compatibility baselines before deleting or redirecting files

Record current CLI help/version/aliases and representative JSON errors, API schema digest, desktop and npm package identity, installer arguments and service names, and release artifact names in an evidence file with command, environment, and output digest. Run the named baseline checks in the isolated worktree; avoid installed-user state and port 11611. Alternative: infer compatibility from code review after edits; rejected because it cannot detect drift.

### Use an indexed-path and Go-package repository checker

Create a small cross-platform checker under `scripts/maintenance/` that inspects `git ls-files` and `go list ./...`, fails with explicit offending paths, and is run by CI. Ignore `_work/`, `.gocache/`, `.pytest_cache/`, Python bytecode/distribution outputs, and known generated state. Keep `build/` as an ignored artifact destination; diagnostic/generated source moves to `_work/` or the OS temporary directory. The checker must distinguish a local ignored file from a tracked path. Alternative: rely on `.gitignore` alone; rejected because it cannot remove or detect an already tracked binary.

### Remove obsolete files only after owner/reference checks

Compare disabled workflow snapshots with the active release workflows and references; remove snapshots only if no unique maintained action remains. Confirm no maintained build or documentation references `dev/Dockerfile`, `dev/docker-compose.yml`, or `dev/INSTALLER_UPDATE.md`. Do not rewrite release notes. Alternative: delete by filename; rejected because the historical files may contain still-referenced behavior.

### Treat installer convergence as compatibility work, not a rename

Inventory root, legacy public, and developer installer options and effects. Port unique supported functionality into the canonical root implementation or focused build helpers before replacing an entry point. The legacy `scripts/install.sh` wrapper must support both a checked-out file and `curl ... | bash` invocation; a wrapper that only computes `../install.sh` would fail the published raw URL. Unit source files move to `installers/systemd/`, while the installed unit names and configuration remain unchanged. Isolated stubbed-download tests verify command/URL/options without installation, plus shell syntax/static checks. Alternative: immediately make the legacy URL call root installer; rejected until GPU/systemd parity is proven.

### Community files are navigation, not new product contracts

Move `dev/CONTRIBUTING.md` to root and update maintained links. Add security, conduct, support, issue, and PR templates with truthful contact paths. No `CODEOWNERS` entries are invented. Alternative: leave the guidance under `dev/`; rejected because GitHub does not surface it as the conventional contributor entry point.

## Risks / Trade-offs

- [Clean branch omits active uncommitted provider work] → Keep all structural changes isolated; do not merge or move overlapping source until integration is reviewed against both worktrees.
- [Legacy installer behavior differs from root] → Gate wrapper replacement on an option/effect inventory and isolated compatibility tests; retain the old implementation if parity is not yet met.
- [Removing a disabled workflow loses unique release logic] → Compare against active workflows and preserve any unique maintained step before deletion.
- [Repository checker flags legitimate release output] → Inspect tracked paths and `go list` packages, not every ignored local artifact; test allowed and rejected fixtures.
- [Public documentation points to old contributor/unit paths] → Search maintained docs and test links; preserve historical release notes unchanged.

## Migration Plan

1. Capture read-only baselines and repeated Go test results in the clean worktree; record failures as failures rather than editing product behavior in this structural change.
2. Add hygiene checker/CI, ignore rules, and move generated-source staging. Remove the tracked binary from the index without deleting unrelated local output.
3. Add root community files and update maintained links.
4. Verify obsolete file replacements, then remove obsolete development deployment files, stale note, and disabled workflow snapshots.
5. Inventory installer behavior, port supported differences, test both entry points, relocate unit sources, and update maintained documentation.
6. Run applicable Go, repository, documentation, installer, Docker, and OpenSpec checks; record platform checks not run. Review before starting the next structural change.

Rollback is source-level: restore the prior tracked files and references on this isolated branch. No installed data, release artifacts, or running services are changed by this change.
