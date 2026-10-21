# Application layout verification (2026-10-19)

This report covers the source move in the primary `D:\offgrid-llm` worktree. It does not qualify an installed application or authorize deployment. The separate `D:\offgrid-llm-reorg` worktree is not synchronized automatically and was not used to overwrite the primary tree.

## Preservation and compatibility

- Pre/post source manifests matched byte-for-byte after prefix mapping: web 193 tracked + 1 untracked, desktop 40 tracked + 2 untracked, companion 38 tracked. Digests and identities are in `baseline-evidence.md`.
- No tracked source remains under `web/app`, `desktop`, or `computer`. The old `desktop/dist` is ignored output from a running unpacked app and was left intact; no installed application was touched.
- `git diff --numstat` reports no lockfile content changes. A direct staged/current manifest comparison preserved all three package names and versions, desktop `appId`, product name, artifact name, protocols, and packaged resource destinations. `pkg/api/openapi.yaml` retained SHA-256 `7251A1E6CCF0B6CED676499F2533BDB82100FED3FA1830D65862F97CFBB43E67`.
- `go run ./cmd/offgrid version` returned 0.4.18, `windows/amd64`, CPU only, matching the pre-move baseline.

## Checks run

| Check | Result |
| --- | --- |
| `npm.cmd --prefix apps/web run api:check` | Pass |
| `npm.cmd --prefix apps/web run check` | Pass |
| `npm.cmd --prefix apps/web run build` | Pass; output at `apps/web/dist` |
| `npm.cmd --prefix apps/desktop test` | Pass, 71 tests |
| `node --check apps/desktop/main.js` and `preload.js` | Pass |
| `npm.cmd --prefix apps/computer test` | Pass, 73 tests |
| `go test ./cmd/offgrid ./internal/server` | Pass |
| `go test ./...` | Pass |
| `go test -count=1 ./...` | Pass in a fresh uncached run after the final path correction |
| `wsl docker compose -f docker/docker-compose.yml config --quiet` | Pass |
| `wsl bash docker/validate-docker.sh` | Pass for configuration; full service image smoke not requested or run |
| `wsl docker build --target ui-builder --pull=false -f docker/Dockerfile -t offgrid-ui-layout-test .` | Pass; proves the new UI and compatibility inputs enter the Docker context |
| `wsl bash scripts/maintenance/test-installer-entrypoints.sh` | Pass |
| `node --test scripts/maintenance/check-repository-hygiene.test.mjs` and its checker | Pass |
| `node --test scripts/maintenance/check-application-layout.test.mjs` and its checker | Pass; old source paths and command forms rejected |
| `node dev/scripts/check-docs.mjs` | Pass, 169 maintained files and 0 broken links after the final guide correction |
| `node dev/scripts/test-release-ci.mjs` | Pass |
| `wsl node dev/scripts/test-release-context.mjs` | Pass |
| `wsl node dev/scripts/test-release-containers.mjs` | Pass |
| `wsl node dev/scripts/test-finalize-release.mjs` | Pass |
| `wsl node dev/scripts/test-release-asset-access.mjs` | Pass after normalizing workflow CRLF in the test harness |
| Python/PyYAML parsing of CI and release workflow YAML in WSL | Pass |
| `npx.cmd electron-builder --dir --win --x64 --publish never` from `apps/desktop` | Pass; unpacked package contains renderer, native companion and `computer-pack/pack.cjs` in `app.asar` |
| `git diff --check` and `git diff --cached --check` | Pass; only Git line-ending warnings |
| `openspec validate application-monorepo-layout --strict` | Pass |

The direct Windows invocation of `test-release-context.mjs` could not resolve its Bash script path; the supported WSL invocation passed. An initial WSL `test-release-asset-access.mjs` run exposed CRLF-sensitive step parsing and passed after the test harness normalized line endings. The layout review also found a stale CI companion working directory and several old commands in maintained docs; these were corrected and added to the path-check fixture.

`go list ./...` passed and showed no `/build/` or `/_work/` packages. `go mod tidy -diff` could not complete because the required `github.com/go-skynet/go-llama.cpp` module download failed at `proxy.golang.org`; it made no source change. This is an unresolved network-limited check, not evidence of a tidy diff.

## Not established by these checks

- Installed Windows/macOS/Linux app behavior, signing, installer/reinstall/uninstall, real native control, and human usability are unverified. The Windows `--dir` build is packaging evidence, not an installation test.
- Linux/macOS desktop packaging and the complete service/GPU container image were not built in this pass. The Docker UI stage was built but no running service was replaced.
- `actionlint` was not installed. The pinned `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 ...` could not download from `proxy.golang.org` on this host; workflow YAML parsed and release fixture tests passed, but actionlint and hosted GitHub Actions execution remain unverified.
- The separate reorganization worktree remains a staging checkout. Its uncommitted contents will not merge or sync into `main` without an explicit reconciliation/commit operation.

## Scope review

A separate diff review compared the proposal, design, task list, original source inventory, staged renames, and unstaged path edits. It found no missing tracked source, changed lockfile content, package identity change, API-schema change, or release artifact-name change. The review found and corrected a stale CI companion working directory plus maintained `cd`/`--prefix` examples. A subsequent pass found a wrong `cd ../../desktop` example in the UI guide; it was corrected to `cd ../desktop`. Regression fixtures now reject those forms. The proposal was also reconciled with the locked ignored `desktop/dist` so the change does not falsely promise that all old generated files have vanished. No external peer review or installed-platform qualification is claimed.
