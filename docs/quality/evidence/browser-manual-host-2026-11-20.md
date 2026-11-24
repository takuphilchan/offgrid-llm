# Retained browser host and task-bound pairing

Date: 2026-11-20. Source: the shared uncommitted worktree in
`D:\offgrid-llm`, continuing `browser-manual-takeover` task 2.1.
This page records the trusted host and service enrollment slice. The composed
browser/task journey and full change qualification are separate requirements.

## Implementation and contract

`apps/desktop/computer-browser-control.js` extends the existing `ComputerRuntime`
owner. The preload advertises `browserManualProtocol: 1` and exposes
`takeoverComputerAccess({session})` and
`resumeComputerBrowser({workspace,runId,requestId,session})`. Resume accepts no
renderer-provided origin, path, credential, network mode, policy or upload grant.
Older browser/native hosts report an explicit unsupported result.

The host retains the exact child, generation, actor, workspace, service, origin
set, network mode and approval policy from the original consent. It validates
the authenticated saved task's browser input ID and previous session before and
after renewed consent and pairing. Maintenance admission holds prevent new
authority. Host-generated operation IDs correlate manual/ready acknowledgments;
duplicate renderer requests observe the same operation. Lost replies remain
unconfirmed without a second pairing or child resume.

`browser-task-pairing-v1` is an additive service capability. POST
`/api/v2/computer/pairing` accepts protocol 1 with `run_id`, `input_id`,
`previous_session` and `approval_mode`. The existing generic `{}` and native
protocol 3 requests retain their behavior. `PairBrowserTaskCode` copies the
stopped browser session's exact origins, feature versions and policy. Code
consumption checks that scope and publishes the new session with actor, run,
input and workspace binding atomically. Existing reservation, dispatch and task
input admission reject another task or pending input. Native enrollment cannot
consume a browser renewal code.

Confirmed authentication denial or a changed principal/workspace/service closes
the owned resource. Temporary network or service failure retains local cleanup
ownership and denies continuation. An unconfirmed child or host Stop retains
its exact target so the same session can retry cleanup; a foreign session cannot
stop it. Stop invalidates pending consent and stale child replies.

## Acceptance evidence

| Boundary | Actual test |
|---|---|
| Consent, exact task/policy/scope and fresh child correlation | `apps/desktop/test/computer-browser-control.test.js`: real `ComputerRuntime` with bounded fake utility transport and injected native consent; exact pairing request and immutable original scope asserted |
| Lost reply and duplicates | Delayed manual/ready replies and host timeout; one pairing, one child command, same result even when renderer object key order differs |
| Revocation and cleanup | Background principal monitor, actual HTTP `Response` status/body classification, changed actor/workspace/service, consent-time change, maintenance hold and local Stop during service outage |
| Failed Stop remains owned | Child `computer_stop_unconfirmed` followed by failed kill acknowledgment; exact-session retry succeeds and foreign-session retry is rejected |
| Service publication and isolation | `TestBrowserTaskPairingPinsAuthorityBeforePublication`: concurrent hub observation, detached origin slices, run/input/workspace checks and direct foreign dispatch denial |
| Protocol and policy preservation | `TestBrowserTaskPairingRejectsProtocolScopePolicyAndActorChanges`: live prior authority, different actor/run/policy, origin/feature expansion and native code use rejected |
| Saved task and HTTP companion integration | `TestBrowserTaskPairingActualSavedInputAndCompanion`: real persistent runner task, takeover/reconnect state, service enrollment handler and actual companion transport handler; renewed session has the original run and pending input |

## Checks and review findings

Commands: `node --test apps/desktop/test/*.test.js`,
`go test ./internal/computer ./internal/server -run 'Test(Browser|TaskAccessPolicy|NativePairing|Desktop.*Pair)' -count=1`,
and WSL Go 1.26.6 `go test -race ./internal/computer -count=1` followed by
`go test -race ./internal/server -run TestBrowser -count=1`.

- Focused new host suite: 10/10 passed, 0.327 s.
- Complete desktop suite: 278 passed, 4 skipped of 282, 34.439 s. Skips cover
  platform/explicit opt-in fixtures and are not qualification evidence.
- Windows affected browser/task/native compatibility selection: computer
  0.604 s and server 2.453 s, passed.
- Linux complete `internal/computer` race suite: passed, 5.752 s.
- Linux server browser-focused race: passed, 2.343 s.
- Three isolated composed service/host/Chromium journeys passed in 16.466 s;
  exact boundaries and the earlier unresolved cleanup failures are recorded
  below. This is functional integration evidence, not full change qualification.
- Source-size ratchet passed; documentation checker found zero problems across
  458 maintained Markdown files. Focused whitespace and JavaScript syntax checks
  passed. These structural checks are not behavioral evidence.

Independent parent review reproduced two defects before their fixes: generic
child errors dropped the cleanup target, and the background monitor suppressed
confirmed principal revocation. The new regression command failed both behavior
assertions before the correction and passes afterward. Its third pre-fix failure
was the not-yet-added principal helper, not an independent product defect.
The scheduling agent separately reviewed the host and task-bound pairing code
without finding another blocker in that sampled path; this was a read-only
review and did not independently execute the suite.

These tests use isolated stores, synthetic credentials, injected native consent
and controlled transports. They do not establish installed Electron IPC,
personal browser attachment, real model ability, hardware coverage, usability
or full M2 completion. No installed application or personal browser was accessed.

## Composed fixture development record

`TestBrowserManualHostIntegration` and
`apps/desktop/test/fixtures/browser-manual-integration.mjs` compose actual
authenticated Go service handlers and durable task state with the production
`ComputerRuntime`, `ManagedBrowserController`, `CompanionSession` and headless
Chromium. Only native consent, the utility-process message bridge and disposable
browser launch are injected. Test-only routes supply controlled task/tool
requests; a deterministic engine exercises the normal two-step tool preflight,
and the runner's controlled model callback waits for cancellation. This does not
qualify any installed model.

The original 8192-token fixture window correctly failed the normal context
admission after attachment: estimated input 7006 plus output reservation did not
fit. The fixture now declares 32768 consistently at task, service and lifecycle
admission; no production guard changed. A transient compile attempt failed on
an unrelated in-progress ONNX constant and was rerun after its owner restored a
compiling checkpoint.

One complete journey passed in 4.547 s. A subsequent three-run command retained
one failure: the fixture had shortened the host Stop grace from the production
35 seconds to five seconds and deliberately refused to simulate a forced child
exit. At that deadline Chromium was disconnected but its close promise/child
cleanup was still pending, so the host correctly returned `stop_unconfirmed`.
The other runs confirmed Stop in 173 ms and 263 ms. The fixture now uses the
unchanged production grace and still requires actual close and child exit.
The corrected three-run command failed in 194.011 s: two cases reached the
90-second fixture deadline during cleanup, and the third confirmed Stop after
6082 ms. This remains unresolved; a disconnected browser is not accepted as
confirmed child cleanup. Fixture-only tracing now separates driver/browser
close, browser-process exit, journal disposal and controller promises. No
production deadline or cleanup guarantee was weakened. Task 3.1 remains open,
and none of these timings establish performance qualification.

### Isolated confirmation and remaining cleanup concern

After the recovery renderer/Electron test window ended, one instrumented
diagnostic passed in 6.732 s. It observed browser process/stdio close with exit
code zero and completed temporary-directory cleanup, rather than relying on
`browser.isConnected()`. Host Stop took 360 ms. Instrumentation is opt-in and
does not alter production source.

The subsequent command used passive Playwright logging only: all session,
workspace, browser and journal methods were unwrapped production methods. Set
`OFFGRID_BROWSER_MANUAL_INTEGRATION=1` and
`OFFGRID_BROWSER_MANUAL_DIAGNOSTICS=process`, then run
`go test ./internal/server -run '^TestBrowserManualHostIntegration$' -count=3 -v -timeout 4m`.
The command passed in 16.466 s. UTC trace observations:

| Owned PID | Graceful close began | Process/stdio close, exit 0 | Temporary cleanup finished | Host Stop |
|---|---|---|---|---|
| 33408 | 13:30:38.075 | 13:30:38.389 | 13:30:38.429 | 375 ms |
| 35216 | 13:30:43.560 | 13:30:43.756 | 13:30:43.794 | 255 ms |
| 46956 | 13:30:48.450 | 13:30:48.731 | 13:30:48.765 | 343 ms |

Each run independently counted exactly one HTTP effect and two enrollments,
preserved the exact selected Playwright `Page` and two task tabs, completed
synthetic form sign-in with an HTTP-only cookie, rejected automated reads while
manual, and rejected old-session and same-actor foreign-task dispatch. Renewed
local consent had identical scope; the real `/jobs/{id}/input` handler performed
the normal tool preflight and resolved the saved input before continuation.
The same resume request did not enroll again. Task cancellation and local Stop
then confirmed actual child cleanup. The final durable task was cancelled, with
its resolved input retained and exactly one controlled model callback.

These isolated successes do not explain or erase the two earlier 90-second
cleanup deadlines. Resource contention is only a hypothesis. The open cleanup
and performance concern remains part of task 3.2 and broader qualification; no
release, installed-platform, real-model or human-usability claim follows from
the successful functional journey.
