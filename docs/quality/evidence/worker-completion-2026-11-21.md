# Worker completion ownership — 2026-11-21

Continues M1.05/M3.07 and
[Conversation execution requirements](../../../openspec/changes/agent-conversation-workflows/tasks.md).
This is a source/fixture correction, not complete native/model qualification.

## Observed defect and correction

`internal/server/execution_lifecycle.go` previously indexed completion by handle
text alone and did not require a complete terminal receipt. Six new assertions
reproduced acceptance of a different host, different session, uncertain/dispatched/
rejected terminal receipt, and a second started owner's matching handle erasing
the first owner's unfinished process. The unchanged assertions failed before the
fix (0.356 s, `build/worker-completion-owner-reproduced-20261121.log`).

Completion now keys each recorded process by host/session/handle and matches every
observation to that start. A terminal process also requires a complete receipt,
exit evidence and version-1 settled-descendant proof. Failed exit codes can still
be honestly reported. Pending observation retains the matching current execution
reference; old/foreign references do not acquire current authority. The existing
missing/unknown/future/invalid descendant-proof negatives remain unchanged.

## Integrated checks and retained failures

The broader Windows consumer run passed execution (20.161 s), agents (62.913 s)
and executor (4.202 s), but failed server (113.789 s):
`build/worker-environment-consumers-20261121.log`. Two older positive completion
fixtures lacked the now-required descendant proof. They now supply explicit
settled proof; separate negatives still reject absent proof. The third failure
was the route fixture's old count. Source review confirmed exactly two new
administrator-only endpoint registrations, with unchanged outer middleware.
The golden diff adds only those routes and the expected count changes 187 to 189.
Existing live-administrator/actor-scope tests remain present and passed in the
final full suite. No application guard was relaxed to make these tests pass.

The first new regression compile failed because of a local fixture variable
collision (`build/worker-completion-owner-baseline-20261121.log`); the corrected
test then exposed the six real behavioral failures above.

| Check | Result | Log |
| --- | --- | --- |
| New ownership/receipt regressions before fix | FAIL 0.356 s, all six cases | `build/worker-completion-owner-reproduced-20261121.log` |
| Reviewed route fixture update | PASS 0.378 s; exact two-route diff inspected | `build/worker-route-manifest-20261121.log` |
| Linux execution-completion race cases after fix | PASS 1.031 s | `build/worker-completion-owner-linux-20261121.log` |
| Full Windows server suite after fix | PASS 112.915 s | `build/worker-environment-server-final-20261121.log` |

Commands:

```text
go test -p 1 ./internal/execution ./internal/agents ./internal/server ./cmd/offgrid-executor -count=1 -timeout 6m
go test -p 1 ./internal/server -run TestExecutionCompletionRequiresSameStartedOwner -count=1 -timeout 1m
CGO_ENABLED=1 GOPROXY=off GOMAXPROCS=4 go test -race -p 1 ./internal/server -run ExecutionCompletion -count=1 -timeout 1m
go test -p 1 ./internal/server -count=1 -timeout 4m
```

Windows/WSL use Go 1.26.6. HTTP/native tests own their disposable services and
fixtures; loopback acquisition fixtures do not download real model weights.
No installed state, personal browser or remote account was used. The complete
Linux execution race failure remains open in
[ownership evidence](worker-job-watch-2026-11-21.md). Full combined CI, Windows
race, independent review, model/hardware and installed-product qualification are
not established by these passes.

Final source/fixture hashes for execution, agents, server and executor plus module
files are recorded in `build/worker-environment-source-20261121.json`. This is a
post-check identity manifest, not a frozen full-repository qualification snapshot.
Final documentation checks passed 486 maintained Markdown files; source-size,
strict Conversation OpenSpec validation and scoped whitespace checks also passed.
