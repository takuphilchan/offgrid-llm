# Verification evidence — 2026-10-21

This records source and isolated checks for the documentation/repository phase. It does not qualify installed desktop workflows, provider models, other operating systems, or a release.

| Check | Result |
| --- | --- |
| `node scripts/maintenance/split-reliability-record.mjs --check` | Passed: 10 parts reconstruct 434,888 source bytes, SHA-256 `08ac686fe4eee88bd2c8451e5e9584ed9017927d4734c39cc0491f9ad55e97da`, accounting for manifest-recorded link rebases. Original working copy retained under ignored `build/documentation-backups/reliability-plan-2026-10-21.md`. |
| `node --test` documentation, application-layout, developer-layout and source-size checker suites | Passed, 18 tests including staged removals in both layout and documentation inventories. |
| `node scripts/maintenance/check-docs.mjs` | Passed after archival and final evidence entry: 236 maintained Markdown files, zero missing local links. External URLs and heading anchors are not checked. |
| Application/developer/source-size/repository-hygiene checkers | Passed against the current dirty working tree. Historical evidence is excluded from obsolete source-path claims; new maintained paths are checked. |
| `go test ./... -count=1` | Passed; all 49 listed Go packages were evaluated, including `cmd/offgrid-computer` after its entrypoint split. |
| `go build -trimpath ./cmd/offgrid` | Passed. |
| `npm.cmd --prefix apps/web run api:check`, `check`, `build` | Passed; Vite retains its existing >500 kB chunk advisory. |
| `npm.cmd --prefix apps/desktop test` | Passed, 71 tests. |
| `npm.cmd --prefix apps/computer test` | Passed, 73 tests. |
| WSL `PYTHONPATH=sdk/python python3 -m pytest -q sdk/python/tests/test_client.py runtimes/python/tests` | Passed, 64 tests. Native Windows `python.exe` is only the Store alias here. |
| WSL `python3 -m compileall -q sdk/python runtimes/python` | Passed. |
| `go list ./...` | Passed; 49 package paths. |
| `git diff --check` | Passed; Git emitted line-ending conversion warnings, not whitespace errors. |
| `openspec validate repository-documentation-hygiene --strict --no-interactive` and strict main-spec validation | Passed; 14 main specs validated after nine completed changes were synced and archived. |

Windows native UI actions, macOS/Wayland installed tests, independent usability/accessibility review, real provider-model qualification and release-package testing were not run. `go test -race` is unavailable in this Windows environment because cgo is not configured; it was not used as a pass claim. No container, installed application, user workspace, model files, live task or consent state was changed.

Public CLI/API/desktop and installer source entrypoints were preserved by the layout phases. This phase changed maintained docs, checker policy/fixtures, OpenSpec records, and mechanically split the native-worker command entrypoint; it did not change public transport contracts or installer behavior. The exact docs split is reversible from the ignored local backup and tracked evidence parts.

The final read-only path check found `cmd/offgrid/main.go`, `cmd/offgrid-computer/main.go`, `pkg/api/openapi.yaml`, `apps/desktop/main.js`, `apps/computer/package.json`, `install.sh`, `installers/`, and all four new documentation indexes present. It does not substitute for an installed upgrade test.
