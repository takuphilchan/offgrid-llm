# Preservation and ownership inventory — 2026-10-21

This is a dirty-worktree baseline, not a claim that the repository was committed or that unfinished features are qualified. The full per-file inventory is retained outside Git at `D:\offgrid-llm-backups\workspace-cleanup-nested-20261021-baseline.json`.

## Baseline

- 1,569 readable nonignored files, SHA-256 manifest digest `38d9bed23ac41eedf7e5715059f908abf2d4ba33fa508c347126bda4e6928615`. The inventory deliberately excludes this cleanup change's mutable evidence and task record from the digest.
- 825 `git status --porcelain=v1` entries at capture: 289 staged renames, 57 staged renames with unstaged edits, 162 unstaged modifications, 52 unstaged deletions, 9 staged deletions, 1 staged modification, and 255 untracked status entries. Git's collapsed directory display differs from the 463 individually enumerated untracked paths.
- 356 staged path changes; the worktree also contains 271 unstaged file changes and 463 individual untracked nonignored paths. These predate or include this change and remain user-owned.
- 16 explicit ignored-output targets contain 12,596 files and 1,426.3 MiB. They are application `node_modules`/`dist`, web test results and two local logs, SDK build/egg-info/pytest/bytecode, and runtime-worker bytecode. No target is source or user workspace data.
- The prior recoverable root cleanup remains at `D:\offgrid-llm-backups\workspace-cleanup-20261021-262801c9`. It contains the former documentation backup at `build\documentation-backups\reliability-plan-2026-10-21.md` (existence verified). This change will not purge it or the older desktop backups.

## Apparent deletions reconciled

- Staged `computer/`, `desktop/`, `web/app/`, `python/`, and `dev/` moves have current owners under `apps/`, `sdk/`, `runtimes/`, `scripts/`, `docker/`, `examples/`, `installers/`, and `tools/`. `git ls-files` reports no current indexed source at the retired roots. The two layout checkers pass against present staged/untracked source.
- The removed `cmd/offgrid/model_package_commands.go` has its command implementation under `internal/cli/model_package_command.go`; server data-layout code and tests have counterparts under `internal/storage`, and workspace ownership tests under `internal/app` and `internal/storage`.
- The removed `scripts/offgrid.service` has a maintained source under `installers/systemd/offgrid.service`. Removed disabled release workflows and obsolete `dev/` files were reviewed in the archived maintainability/layout changes; active release workflows remain under `.github/workflows/`.
- The deleted active OpenSpec directories for agent/desktop/workspace foundation changes exist under dated `openspec/changes/archive/2026-10-21-*` locations. The six active feature changes remain active and are not cleanup targets.

## Ownership and apparent redundancy

- `install.sh` is canonical and `scripts/install.sh` is a published compatibility URL, not a duplicate to remove. `apps/web` is one renderer shared by browser/Electron; `apps/desktop` and `apps/computer` have distinct host/companion duties. `sdk/python` is a public package; `runtimes/python` holds trusted service workers.
- `docs/advanced` short stubs preserve established links. `docs/quality` owns current readiness/evidence; dated `docs/releases` and archived changes retain historical statements. Installed `web/ui` paths are release/install destinations, not retired source paths.
- Byte-identical duplicate files were not found in the maintained non-generated inventory during the read-only audit. Similar names across tests, wrappers, platform adapters and evidence have distinct roles. Any overlapping runtime logic requiring semantic judgment stays outside this structural change.

The confirmed redundant entries were repeated `dist/`, `/build/`, and `*.so` ignore patterns, app-specific ignores already covered by generic patterns, obsolete root `dev/`/`python/` Docker exclusions, and `.gitattributes` entries for retired `web/ui/index.html` and `python/offgrid.egg-info`. Representative `git check-ignore -v --no-index` results were unchanged after normalization; `apps/web/src/lib/` remains explicitly unignored. The Docker `web/ui/` exclusion remains because it describes a release/runtime output destination, not former source. No maintained implementation file was deleted solely for having a similar name. Possible business-logic overlap in active server/CLI/provider work requires a separate behavior-scoped review.

## Compatibility identity at cleanup

- `VERSION`: `0.4.18`; web package `@offgrid/ui` `0.4.18`; desktop package `offgrid-llm-desktop` `0.4.18`; computer package `@offgrid/computer-preview` `0.1.0`.
- Desktop app ID `com.offgrid.llm.desktop`, product name `OffGrid LLM Desktop`; configured artifact names retain version/architecture and Windows Setup/Portable variants.
- Public Python distribution `offgrid` `0.2.12`, intentionally independently versioned. `pkg/api/openapi.yaml` SHA-256 `7251A1E6CCF0B6CED676499F2533BDB82100FED3FA1830D65862F97CFBB43E67`.
- Root `install.sh` remains canonical; `scripts/install.sh` still supports the published raw URL and checked-out invocation. Installed service paths and `/var/lib/offgrid/web/ui` were not changed.

## Boundary

No Git index changes, source deletion, installer behavior change, API change, model change, or native desktop operation is authorized by this inventory. Later source/config changes in this cleanup will be identified against this manifest, and generated outputs will be quarantined only after verification.
