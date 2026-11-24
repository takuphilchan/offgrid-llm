# Tasks

## 1. Owned workspace handoff

- [x] 1.1 Add immediate admission fencing, confirmed drain and evidence invalidation to the existing browser workspace; verify real Chromium effect/no-replay, read denial, cookie/tab continuity, manual closure and Stop with `node --test apps/computer/test/browser-takeover.test.mjs`, and document the lifecycle in apps/computer/README.md.
- [x] 1.2 Separate explicitly retained companion resources from ordinary Stop/failure; verify result-journal settlement, aborted polling, stale session callbacks, resume pairing and retained-resource cleanup through companion tests, retaining all uncertain outcomes.

## 2. Trusted host and task continuation

- [x] 2.1 Add versioned, exact-session host takeover/resume IPC in the existing runtime; validate saved pending task, previous session, actor/workspace, retained scope and renewed consent. Verify cancellation, lost reply, changed principal, duplicate commands, outage and host cleanup with desktop tests; document user behavior and unsupported older hosts.
- [x] 2.2 Integrate Chat/Activity Take over, visible manual state and explicit reconnect using current task contracts; preserve local and service Stop independently. Verify accessible keyboard states, all locales and cross-task rejection in real renderer fixtures plus `api:check`, `check` and `build`.

## 3. Integrated verification

- [x] 3.1 Run a composed disposable browser/session/host/task journey for manual authentication and renewed continuation, prove no automation while human control is active, and independently verify cookies/tabs and effect counts. Record actual native/installed/model/human limits separately.
- [ ] 3.2 Run applicable complete companion/desktop and affected Go/race, renderer and documentation/source checks, strict OpenSpec validation and independent review; update the master M2 tracker and dated evidence without claiming attached-browser or full M2 qualification.
