# UI integration and installed delivery — 23 November 2026

The owner authorized completing the current UI work and synchronizing it into
local `main` for a same-day version. The original isolated-worktree snapshot was
used as the merge base; the other thread's current files were the receiving side.
This records local integration, not a published release or full-program completion.

## Integration

125 UI/source/test/documentation paths were synchronized: 119 applied directly,
two merged automatically, and four were resolved explicitly. Chat retains both
the new model-menu setup handoff and the other thread's voice Stop condition.
Models retains endpoint-owner management and the UI transfer surface. The two
translation catalogs were merged recursively with no conflicting leaf values.
The other thread's subsequent changes were preserved. Git's index was unchanged;
no commit, push, tag or publication was performed.

The merge snapshot, exact before/after hashes and original main files are retained
at `D:/offgrid-ui-20261121/build/ui-main-sync-20261123/manifest.json` and
`before-main/`. Main was at `8ece0e4a7496d763b0fcec3944edbcbeaccbfe45` when the
integration began. The shared working tree continues to contain the other
thread's uncommitted backend work; these checks do not qualify all of it.

## Installed result

The verified presentation package was installed at 22:18 China time, after
confirming that OffGrid was idle. No processes were terminated and no user data
was changed. Actual installed startup/recovery and Help/Workspace/Settings checks
passed in isolated Windows profiles, including Arabic and native 200% zoom.
Installed startup, recovery without the divider, and Settings results were
visually inspected. The logo animation remains; reduced motion disables it.

Native archive: `cb64f41bf064a3d5a123784c659bed68bed03097d349b0faf2262786a8ec62d4`.
Renderer index: `15a95ea235d3cfb02ec05d67c9ea00f9339bd4041b5fc767213a68010e6c602b`.
All 67 renderer files and preserved runtime hashes were checked after installation.
Rollback: `D:/offgrid-ui-20261121/build/ui-install/rollback-startup-20261123-221802`.
See [startup delivery evidence](startup-entrance-2026-11-22.md).

The installed package retains the previously verified service/native backend.
The main-branch production build additionally includes the other thread's current
voice and endpoint work. They are distinct artifacts, not interchangeable release
qualification. No unfinished native recovery/backend was added to the installation.

## Main integration checks

Logs are under `D:/offgrid-ui-20261121/build/ui-main-sync-20261123`.

| Check | Actual result |
| --- | --- |
| Production TypeScript/build | Passed; `build-qualified.log` |
| Desktop unit suite | 292 passed, four opt-in skips; `desktop.log` |
| Combined UI suite | 316 passed, two failed initially; both corrected startup cases passed; `browser.log`, `startup-recheck.log` |
| Talk lifetime, canonical speech steering, continuous voice | 22 passed; `voice.log` |
| API, translation inventory/completeness/consistency | Passed; final inventory in `i18n-check-final.log` |
| Translation audit guards | 11 passed; `quality-guards.log` |
| Source size, docs and OpenSpec structure | Passed |
| Production responsiveness | Passed unchanged gates: baseline/candidate p95 28.3/29.1ms, heap 12,948,128/13,536,784 bytes, requests 180/180; acknowledgment 30.2ms |
| Go source suite | All 40 tested application packages passed, including server (305.317s), inference and execution; the maintained-source follow-up exited 0 (`go-maintained.log`) |

The two startup failures were a fixture defect: a Vite entry retry with a query
string bypassed the simulated bundle outage. The route now matches both bare and
cache-busted entry URLs; readiness assertions are unchanged. A production rebuild
found the new audio-worklet URL import lacked TypeScript declarations; a scoped
asset URL declaration fixes that without altering capture behavior. Ten internal
capture diagnostics were added to the exact literal inventory after verifying
that `ContinuousVoiceInput` maps them to existing localized failure messages.
The initial source-size failure disappeared after the other thread's ongoing Chat
refinement; no source-size limit was raised. An attempted guard command named a
nonexistent file; the actual full quality-guard inventory subsequently passed.

The initial `go test -p 2 ./...` reports mixed packages in old diagnostic `.go`
backups under ignored `build/`. Those files were preserved. Full-CI success,
other operating systems, real models and human accessibility/linguistic studies
are not inferred from these local UI results. Existing Vite advisories remain.

The follow-up `go test -p 2 ./cmd/... ./internal/... ./pkg/... ./examples/... ./tools/...`
passed with exit 0, recorded in `go-maintained.log` and `go-maintained.exit`.
It excludes generated diagnostics rather than omitting an application package.
