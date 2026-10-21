# Verification evidence — 2026-10-21

This is a structural working-tree check, not a release, native-control, or model qualification. Tests used source fixtures and existing ignored dependencies. No installed OffGrid service, user model store, or desktop consent state was changed.

## Before output quarantine

| Check | Result |
| --- | --- |
| `go test ./... -count=1` | Passed across all discovered packages. |
| `go list ./...` | Passed; no output-directory Go packages. |
| `npm.cmd --prefix apps/web run api:check` | Passed; generated TypeScript API matches `pkg/api/openapi.yaml`. |
| `npm.cmd --prefix apps/web run check` and `run build` | Passed; Vite reported its existing >500 kB chunk advisory. |
| `npm.cmd --prefix apps/desktop test` | Passed, 71 tests. |
| `npm.cmd --prefix apps/computer test` | Passed, 73 tests. |
| WSL `PYTHONPATH=sdk/python python3 -m pytest -q sdk/python/tests/test_client.py runtimes/python/tests` | Passed, 74 tests. WSL printed an unrelated localhost-proxy warning. |
| WSL `bash scripts/maintenance/test-installer-entrypoints.sh` | Passed local/streamed entry-point, checksum and isolated service fixtures; no installation occurred. |
| `node --test` for application/developer/repository/source-size/docs/workspace-cleanup checkers | Passed, 27 tests. |
| Application, developer and repository layout checks; source-size and docs checks | Passed; docs check found 0 missing local links among 242 maintained Markdown files. |
| `node scripts/release/test-release-ci.mjs` | Passed. |
| WSL `node scripts/release/test-release-context.mjs` and `test-release-containers.mjs` | Passed. The Windows-Node invocations failed for environment-specific Unix-shell path and temporary-directory cleanup behavior; these are Linux CI scripts, so the WSL results are the applicable local fixture checks. |
| `docker compose -f docker/docker-compose.yml config --quiet` | Unrun: Docker CLI is not installed on this machine. Docker image builds and package/installed-platform tests are likewise unrun. |

The representative Git ignore fixtures retained their effective results after metadata normalization; desktop compatibility source remains included by the Docker context rules, and `apps/web/src/lib` remains unignored. The public package IDs, SDK version, desktop app ID/artifact patterns and OpenAPI SHA-256 are recorded in [the inventory](inventory-evidence.md).

## Quarantine and post-move integrity

The first frozen inventory (`workspace-cleanup-nested-20261021-frozen.json`) was rejected **before any move**: nested `sdk/python/.pytest_cache` and `sdk/python/build` were not themselves ignored, although their contained files were. The root ignore rules were corrected without ignoring maintained `scripts/build` source; the workspace-cleanup fixture tests and affected layout checks passed again. This failed preflight is retained as evidence, not treated as a partial cleanup.

The replacement frozen inventory is `D:\offgrid-llm-backups\workspace-cleanup-nested-20261021-frozen-v2.json`: 1,569 source files, SHA-256 manifest digest `f05b83ce3941b73152af38ce61895634459a017914958b10947ee5210f82f268`, 16 ignored-output targets. The guarded same-volume move placed all 16 targets into `D:\offgrid-llm-backups\workspace-cleanup-nested-20261021-01`; its `quarantine-manifest.json` lists every original and restoration path, counts, bytes and `moved` state. Every target is absent at its source and present at its destination: 12,596 files, 1,426.3 MiB. No data was deleted or disk space reclaimed.

The independent post-move inventory at `D:\offgrid-llm-backups\workspace-cleanup-nested-20261021-post.json` reports the **same source digest** and zero remaining enumerated outputs. The earlier root cleanup backup, including its documentation source backup, still exists. Post-move dependency-free checks and Git delta review follow below.

Post-move application/developer/repository layout, documentation and source-size checks passed; the quarantine fixture suite passed 6/6. The maintained-doc check found zero missing local links among 243 Markdown files. Comparing baseline and post-move source manifests found 24 changed files—all are the cleanup's three repository metadata files, maintained documentation, and the two new cleanup tool files—and **zero added or removed source paths** outside this change's mutable OpenSpec record. The frozen and post-move source digests are identical. `git status --porcelain=v1` remains dirty by design (830 entries at final review versus 825 at baseline capture); no index entry was staged, reset or discarded by this cleanup.

Restoration: consult `D:\offgrid-llm-backups\workspace-cleanup-nested-20261021-01\quarantine-manifest.json`, restore only an entry whose original source path is absent, and verify its file count and bytes before using it. Do not merge it over newly regenerated outputs. The earlier root backup and other dated desktop backups remain outside this cleanup's deletion scope.

## Final review

`openspec validate repository-workspace-cleanup --strict --no-interactive` passed with the expected `skip_specs` information; strict validation of all active changes and main specs passed 22/22. `git diff --check` and `git diff --cached --check` exited successfully. Git emitted Windows LF→CRLF conversion warnings but no whitespace errors.

The cleanup's 24 source-file deltas are limited to repository metadata, documentation and the inventory/quarantine tool. The baseline/post manifest shows no cleanup edit to `pkg/api/openapi.yaml`, Go/CLI/agent/desktop permission implementation, Python SDK metadata, package identities, installer behavior, or installed-user state. Existing unrelated dirty feature work remains unchanged and uncommitted. Full Docker Compose/image checks, live installed-desktop/native-platform checks, model qualification, deployment, desktop replacement, Git commit/push, release and permanent deletion were not performed. A source check does not substitute for those gates.
