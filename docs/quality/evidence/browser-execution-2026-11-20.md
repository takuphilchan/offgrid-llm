# Browser execution reliability — 2026-11-20

Status: working-tree implementation and automated fixture evidence; not a release,
hardware or real-model qualification. Change:
[`browser-execution-reliability`](../../../openspec/changes/browser-execution-reliability/).
The [full transformation roadmap](../../ROADMAP.md) remains open beyond this increment.

## Implemented behavior

- Page labels cannot make clicks reversible; scoped-changes and full-task policies retain distinct authority.
- Pre-dispatch/read failures carry trusted stage codes, persist in the companion journal before acknowledgement, keep the session connected and enter at most three consecutive agent repair attempts. Unknown mutation exceptions never auto-replay.
- Completed input survives unavailable read-back/follow-up observation. Dispatch, verification and observation availability are separate. The agent consumes fresh action observations directly.
- Authenticated browser mutation receipts use existing durable service receipt storage. Restart recovers the recorded invocation without repeating input or declaring task success. Reviewed browser read intents restart without requiring mutation reconciliation; old unversioned intents remain conservative.
- Exact target/form freshness allows unrelated clock changes but rejects changed values, semantics, form destinations, replaced nodes and navigation. Captures and continuation retain whole-page revision checks.
- Negotiated `browser_inspect` pages controls, Unicode text and dropdown options. Hidden candidates do not permanently hide later controls. Replies stay under the 48 KiB transport limit. Legacy pairing, tools and synthetic compatibility probes retain their old shape.

## Checks

Environment: Windows workspace, Go 1.26.6, Node 24.18, installed Playwright 1.62.1 Chromium; isolated local fixtures. All tests below are source/contract tests, not claims about a model's reasoning or a personal browser session.

| Check | Recorded result |
|---|---|
| Initial `npm.cmd --prefix apps/web run check` | Passed |
| `npm.cmd --prefix apps/web run api:check` and `npm.cmd --prefix apps/web run build` | Passed; Vite reported existing large-chunk warnings |
| `npm.cmd --prefix apps/desktop test` | 235 tests: 234 passed, one skipped because directory symlinks were unavailable |
| Final `npm.cmd --prefix apps/computer test` | 85 passed, no skips; includes dropdown/read-back, same-URL reload and malformed Unicode regressions |
| Browser discovery/recovery/page regressions after dropdown pagination | 17 passed |
| Affected preflight/model-check regressions | Passed after correcting discovery negotiation |
| Browser restart/read-intent focused regressions | Passed after correcting the legacy-intent test fixture to omit the new read marker |
| First `go test ./...` and first full affected-package run | Failed in server package; no clean baseline claimed. The affected run exposed discovery leaking into the legacy nine-tool preflight. Fixed by capability filtering; final full run recorded below. |
| Final `go test ./...` | Passed after fixes; final authenticated browser receipt workflow also passed separately |
| Windows CI race command | Could not start: CGO disabled and no Windows gcc/clang found. This is not a Windows race pass. |
| CI race package list in Ubuntu WSL, `CGO_ENABLED=1` | Core packages passed. First run failed only `cmd/offgrid-executor/TestPinnedStdioEndpointRunsRealExecutor` when its subprocess build exceeded the fixture's 60-second deadline. Rerun of agents/server/executor passed (136.284 s / 278.498 s / 10.463 s). No race detector report occurred. |
| `CGO_ENABLED=1 go test -race ./internal/server -run TestBrowserToolsUseDurableApprovalsAndCannotEscapeRegistry -count=1` in Ubuntu WSL | Passed; authenticated browser receipt is persisted before acknowledgement, with dispatch distinguished from verified task success |
| `PYTHONPATH=sdk/python python3 -m pytest -q sdk/python/tests runtimes/python/tests` in Ubuntu WSL | 186 passed |
| Web `i18n:check`, `i18n:audit`, `i18n:complete`, `i18n:consistency` | Commands passed; native linguistic review remains pending, not certified by catalog completeness |
| Source-size ratchet, application layout, maintained docs, `git diff --check`, strict change validation | Passed; 416 Markdown files checked with zero broken local links |
| Documentation/source-size/hygiene guard unit tests | 14 passed |
| Repository hygiene against this working directory | Failed: pre-existing ignored `build/document-assessment-20261112` is discovered as a Go package. It was preserved. This is not a clean repository-hygiene gate. |

The race command covered sessions, RAG, server, users, cache, agents, runs,
storage, serviceclient, models, audio, computer, execution, inference,
integrations (including verification), CLI, power, degradation, offgrid and
offgrid-executor. The initial timeout and successful rerun are separate evidence;
do not present the first command as having passed. Logs are retained under
`build/browser-reliability-*.log` in this workspace.

## Regression coverage

`apps/computer/test/browser-recovery.test.mjs` checks successful dispatch plus
observation/read-back failure and unknown input exceptions. The session test
checks redelivered safe-error receipts, continued work, redaction and no duplicate
execution. Browser discovery tests traverse 410 hidden candidates, 105 visible
controls, long/malformed Unicode text and 130 dropdown options; they reject guessed/stale
cursors, same-URL document replacement and unobserved option IDs. Existing download dual-rejection, network
escape, secret masking and input-freshness tests remain applicable.

Go tests cover trusted descriptor identity, bounded/durable repair, absence of
error-string authority, hub continuation versus uncertain shutdown, feature
negotiation, legacy probes, mutation receipt restart and pinned read-only intent.
Source size was kept within the existing ratchet by extracting browser tool
schemas into their own module; the limit was not increased.

## Still unqualified or unfinished

No full rendered-UI/packaged-installer matrix, real-model 30-task browser/agent suite, matched Nativ comparison, native
Windows/macOS/Linux installation exercise, 72-hour soak, 8 GB/16 GB/Apple
Silicon/NVIDIA benchmark or user pilot was run here. Multi-origin browsing,
multi-tab ownership, selected signed-in Chrome/Edge attachment, frame/custom
controls, visual fallback, wider agent/product work, image generation, Anthropic
transport and distributed task execution remain in the approved roadmap.

The browser is still one managed origin. No personal profiles or installed
applications were controlled, no model weights were downloaded, and no commits,
publication or live installation replacement were performed.
