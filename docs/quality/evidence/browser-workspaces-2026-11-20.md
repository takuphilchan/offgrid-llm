# Managed browser workspaces — 2026-11-20

Status: scoped implementation and planned source/fixture checks complete, with
the preserved hygiene exception below. Full M2 and release qualification remain open.
This extends [browser execution reliability](browser-execution-2026-11-20.md)
and implements [browser-task-workspaces](../../../openspec/changes/browser-task-workspaces/).
The complete [transformation tracker](../TRANSFORMATION_TRACKER.md) remains open.

## Implemented behavior

- Up to eight task-owned managed tabs, opaque tab/document identities and
  serialized list/open/select/close. Exact observations remain page-local.
- Explicit initial site plus up to 15 additional public HTTPS origins, selected
  locally. Page routing and the CONNECT relay enforce the same destination set;
  each destination is pinned before launch and TLS verification remains enabled.
- Negotiated `browser-workspace-v1`, actor-owned scope and service/companion
  dispatch checks. Legacy companions retain their tools and one-site authority.
- CLI and desktop consent display the actual sites and tab limit. The worker
  verifies the capability version shown at consent; late changes cannot widen
  a one-tab consent to eight tabs. Prepared drivers must match the declared scope.
- Tab closure is consequential. A manually closed selected tab does not silently
  select another target for a pending navigation. Explicit selection recovers
  remaining tabs without granting additional sites.
- Tab inventory and action observations have independent response budgets.
  An action receipt is not lost by combining eight tab descriptions with a dense
  page observation. Failed creation/navigation remains uncertain after dispatch.

## Executed evidence

Windows, Node 24.18, Playwright 1.62.1 Chromium, Go 1.26.6; disposable loopback
fixtures and directories. Self-signed TLS is accepted only in the owned relay
test fixture; production browser certificate checks are unchanged.

| Check | Result |
|---|---|
| Initial network/workspace/download tests | Nine passed, one popup test checked before its page-created event. Corrected the test to wait for that event. |
| First full companion suite | 90 passed, no skips |
| Combined final companion suite | 95 passed, no skips |
| Latest workspace/session regression run | 15 passed, including negotiation, post-consent capability change and dense reply bounds |
| Latest workspace review regression run | Six passed, including prepare → manual tab close → rejected navigation with the other tab unchanged |
| `go test ./internal/computer ./internal/capabilities` | Passed |
| `go test ./internal/computer ./internal/server -run 'Browser\|ComputerPreflight\|TaskCapabilities' -count=1` | Passed; scope immutability, legacy negotiation, prepared-call bypass and tool-effect identity covered |
| Combined final `npm.cmd --prefix apps/desktop test` | 240 tests: 239 passed, one directory-symlink test skipped on this environment |
| Combined web API generation check, TypeScript check and build | Passed; existing large-chunk warning remains |
| Combined `go test ./...` | Passed, including server 275.027 s. This run began before the subsequent model-context and endpoint-collision review fixes; their focused and full affected-package checks are tracked separately. |
| Combined Ubuntu WSL race run: computer, agents, server, inference | Computer, agents and inference passed. Server failed its private-host fixture build deadline (90 s), then hit the package's existing ten-minute timeout while running dependent-worker tests. No race report was found. This is a failed command, not a pass; investigate/recheck separately after concurrent full Go work finishes. |
| Isolated Ubuntu WSL server race rerun | Failed again: the private-host fixture build reached 90 s; the package reached 600 s during `TestWorkSummaryPaginationCountsAndDeletedRecords`, which had run for only two seconds. No data-race report. The first concurrency hypothesis does not fully explain the repeated failure. |
| Final structural checks | Application layout and source-size passed; strict browser-workspace, model-adaptive and provider OpenSpec validation passed. Repository hygiene still reports the preserved, pre-existing ignored `build/document-assessment-20261112` Go package. |
| Full server race on verified Linux-native source snapshot | Passed, 534.658 s, with the unchanged ten-minute package and 90-second fixture deadlines. Every one of 3,820 captured source entries was checked against its manifest after extraction. |
| Strict change validation, source-size and maintained-doc checks | Passed at this checkpoint; integrated final checks below remain to be recorded |

Logs use `build/transformation-*-20261120.log`. The race failures occurred during concurrent
Windows full-Go and Linux race runs; resource contention is a hypothesis to test,
not an established excuse. No deadline or assertion was relaxed. A separate
server rerun also failed after the model-context and endpoint-routing review
corrections. A later full server race passed on Linux-native source storage with
unchanged deadlines. This supports using native filesystem checkouts for WSL
verification; it does not prove filesystem overhead was the sole cause. Cleanup
and intervening integration changes also differ from the earlier failed runs.
[Desktop/rendered UI evidence](browser-workspace-consent-2026-11-20.md)
records the actual isolated renderer checks.

The successful snapshot is `build/transformation-source-20261120093941576.tar`,
40,996,864 bytes, SHA-256
`a413f8df205316bf00cad68a7015a758d1d3358cc1955d796cf55d130e7967da`.
Its adjacent JSON records base Git HEAD and every file hash; the source was frozen
during capture and rechecked after extraction to
`/tmp/offgrid-race-20261120093941576`. It includes tracked and nonignored untracked
maintained source, excluding ignored build outputs; the original ignored user
artifact remains untouched. Command: `CGO_ENABLED=1 GOPROXY=off go test -race
./internal/server -count=1`. Result log:
`build/transformation-server-nativefs-race-20261120.log`.
Later metadata-discovery, managed-Ollama and backup review corrections are not
covered by this frozen snapshot and retain their own required checks.

## Review findings resolved

An independent collaborating agent found the combined tab-inventory/observation
reply could exceed 48 KiB after an action succeeded. The receipt now contains
only selected-tab/effect metadata plus its bounded observation; inventory is a
separate read. A real Chromium regression fills eight tabs and a dense Unicode
page and checks both response sizes.

The same review raised navigation retargeting after manual selected-tab closure.
The workspace now requires an explicit new selection. The regression proves the
other page is not navigated. These focused reviews are not a complete security
audit or production qualification.

## Still required

Selected signed-in Chrome/Edge attachment, frame/shadow/custom controls,
searchable discovery, qualified visual actions, richer takeover/scope expansion,
file provenance and the real-model browser corpus remain in M2. No personal
profiles or live installation were accessed. No model weights, commits, releases,
hardware certification or human usability results are implied by these fixtures.
