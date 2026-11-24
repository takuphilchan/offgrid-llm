# Browser and cross-client qualification recheck — 2026-11-22

The disposable CLI/Python/Chromium journey and composed manual-browser journey
passed on Windows and under Linux race detection. These use real client and
browser processes with synthetic service/model responses and isolated state.
They do not establish model quality, personal Chrome/Edge attachment, installed
Electron IPC, human usability or completion of M2/M3.

## Repaired qualification path and CI coverage

`TestAgentCrossClientIntegration` still referenced the removed
`dev/scripts/test-agent-cross-clients.mjs`. With the existing Windows Python
3.12.14 installation selected only in the test process PATH, the actual opt-in
test failed at Node launch: `MODULE_NOT_FOUND`, package elapsed 3.614 s.
The original failure is retained in
`build/agent-cross-client-before-20261122.log`.

The test now resolves the maintained `scripts/test/test-agent-cross-clients.mjs`.
No client assertions, authentication checks or model-call counts were removed.
The CI workflow explicitly enables both integration flags after installing the
web and companion Chromium dependencies, so its normal Go test skips no longer
leave these two journeys unexecuted throughout the workflow:

```sh
OFFGRID_CROSS_CLIENT_TESTS=1 OFFGRID_BROWSER_MANUAL_INTEGRATION=1 \
go test -race ./internal/server -run '^Test(AgentCrossClientIntegration|BrowserManualHostIntegration)$' -count=1 -v -timeout=5m
```

This command was run locally under Linux; the hosted workflow has not been run
or published. The Go layout scanner did not catch the old path because it was
assembled from separate `filepath.Join` arguments. Executing the integration
journey now provides the relevant regression gate.

## Actual results

| Check | Result | Evidence log under `build/` |
|---|---|---|
| Windows cross-client journey, after repair | PASS; test 3.88 s, package 4.413 s | `agent-cross-client-fixed-20261122.log` |
| Windows composed manual browser journey | PASS; test 3.16 s, package 3.696 s; confirmed Stop 221 ms | `browser-manual-recheck-20261122.log` |
| Linux exact new CI command, race enabled | Both PASS; cross-client 12.20 s, manual 6.86 s, package 20.084 s; confirmed Stop 36 ms | `browser-agent-linux-race-20261122.log` |
| Full companion, default concurrency, `npm --prefix apps/computer test` | 219 passed, 0 failed, 5 skipped; 30.161 s | `browser-companion-recheck-20261122.log` |
| Full desktop, `npm --prefix apps/desktop test` | 288 passed, 0 failed, 4 skipped; 8.707 s | `browser-desktop-recheck-20261122.log` |

The Linux run restored and byte-verified 4,171 maintained source files from
`build/browser-agent-checkpoint-20261122.tar`, SHA-256
`285c35496c80a66d1a41043260374930483441f2d6f6eaa48d0daa9f6275d497`.
It used Go 1.26.6, Node 24.22.0, Python 3.14.4, CGO enabled, GOPROXY off,
GOMAXPROCS=4, fresh ext4 test state, existing Playwright dependencies and the
already installed isolated test Chromium. No dependencies or weights were
downloaded. Windows used Node 24.18.0 and the existing Python 3.12.14 runtime.

The cross-client journey proves a CLI-created session, one SDK-created canonical
job, browser steering with duplicate-request idempotency, foreign-actor rejection
for inspect/steer/cancel, CLI status/Stop and SDK observation of that same stopped
job. Exactly one fixture model call and one saved canonical task are asserted.

The manual journey proves one website effect, exactly two consented pairings,
retention of the same selected page and two tabs, a synthetic HttpOnly sign-in
cookie, denied automation during manual control, resolved task input and
confirmed browser cleanup. The host/controller bridge runs in the fixture's
process; this is not installed Electron utility-process or real human evidence.
Individual Stop timings are observations, not latency qualification.

## Remaining limits and retained failures

The companion skips are two selected-tab network-boundary feasibility probes,
the maximum-size upload, its 512 MiB transport diagnostic and the native
URLPattern network-policy diagnostic. Their previously documented failures are
not resolved by this suite pass. Desktop platform/explicit opt-in recovery tests
also remain outside this default run.

The earlier default-concurrency navigation/capture failures and composed cleanup
timeouts in [manual takeover evidence](browser-manual-takeover-2026-11-20.md)
remain recorded. This changed checkpoint passes, but does not establish their
cause or reproducibility under controlled load. Independent review and complete
installed/model/human qualification remain open; task 3.2 stays unchecked.

Developer layout, source-size, strict `browser-manual-takeover` validation and
scoped whitespace checks passed. The earlier complete Go and CI race-package
results remain in [server race evidence](server-race-recheck-2026-11-22.md).

## Requested main-branch checkpoint

The separately authorized local commit `108c2d42e5a505800963a542fe1c92f9a0b3fae0`
was created on `main` at 2026-11-22 02:00:19 +0800, before 11:30. Its subject is
`fix(agents): reserve task writer without rewriting snapshots`. It contains only
the writer-reservation implementation and regression/benchmark file. The full
agents suite passed on an isolated main checkout with only those two files
applied, 59.224 s (`build/task-writer-commit-agents-20261122.log`). The unchanged
baseline also passed, 62.334 s; this pair is not a performance benchmark.
The commit was not pushed. The cross-client/CI changes described above are later
uncommitted work, along with the preserved program changes.
