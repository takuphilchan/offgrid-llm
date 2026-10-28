# Native agent execution, October 25, 2026

Development evidence, not a released/deployed or generally qualified workflow.

## Implemented source

- Host-side execution contexts pin actor, service workspace, host, grant revision
  and admitted jobs. Cwd is session-local; no global directory mutation.
- Rooted bounded listing, raw revision-bound file pages, literal bounded search,
  exclusive creation, preconditioned replacement and independent read-back.
- Private strict framed stdio worker and durable operation journal. Duplicate
  mutation IDs return evidence/uncertainty, not replay. Unknown startup outcomes
  are not adopted by numeric PID. Standalone command dispatch requires the
  locally approved process grant and reviewed invocation policy.
- Process primitives retain actual exit codes, bounded output, minimal environment
  and Windows Job Object/Linux process-group cancellation. This is not an OS
  sandbox, and deliberately detached Unix daemons require stronger isolation.
- Default native CLI uses canonical profile conversations with explicit model selection,
  resume and profile budgets. Initial General-profile setup is confirmed and
  adds no authority. Ctrl+C targets only its exact submitted turn.
  Profile/model mismatch is rejected before creating a new conversation; the
  existing profile model policy is preserved rather than silently overridden.
- Canonical job/turn admission pins additive execution references. The effective
  catalog and dispatch check actor/workspace/grant independently, and selected
  workspace jobs cannot fall back to legacy service file tools.
- One authoritative reviewed workspace catalog supplies exact UTF-8 reads,
  original bytes, bounded search, preconditioned writes and read-back evidence.
  An isolated conversation fixture passes list → read → Unicode edit → read-back,
  preserves an unrelated file, and refuses new work after host revocation.
- The actor-scoped workspace catalog is observation-only. CLI `--workspace`
  selects a published reference and pins it once across interactive turns.
- A real built executor passes the digest-pinned private-stdio endpoint fixture:
  canonical-job admission, exact file read and refusal after job cancellation.
  Private protocol frames replace rather than retain a prior envelope's fields.

## Earlier milestone checks

- Windows: `go test ./internal/execution ./cmd/offgrid-executor` passed, including
  a real subprocess/private-stdio listing and cross-actor refusal.
- WSL Linux: `go test -race ./internal/execution ./cmd/offgrid-executor` passed.
- Windows: `go test ./internal/cli ./cmd/offgrid` passed after CLI integration.
- WSL: `go test -race ./internal/cli ./internal/execution ./cmd/offgrid-executor`
  passed after the interactive changes.
- macOS ARM64 cross-compilation of execution tests passed; no native macOS tests
  ran. Compilation is not platform qualification.
- Final Windows `go test ./...` passed after execution-context persistence and
  the profile-model mismatch guard. `go vet ./internal/execution
  ./cmd/offgrid-executor ./internal/agents ./internal/server ./internal/cli
  ./internal/serviceclient` also passed.
- WSL `go test -race ./internal/execution ./cmd/offgrid-executor ./internal/agents
  ./internal/server ./internal/cli ./internal/serviceclient` passed; CLI/client
  race checks were repeated after the final mismatch/error-projection tests.
- `go test ./internal/server -run
  '^TestCanonicalConversationExecutesSelectedWorkspaceAndExactEdit$' -count=3`
  passed all three isolated canonical list/read/edit/read-back runs. The caller
  is deterministic test code, not a local model; this is not model qualification.
- Web `npm run api:generate`, `npm run api:check`, `npm run check` and
  `npm run build` passed. The build retains its large-chunk warning; no claim of
  new UI performance qualification is made.
- Desktop `npm test`: 71 passed. Companion `npm test`: 73 passed, including its
  isolated Chromium fixtures. These are not installed-desktop qualification.
- Windows `python` was unavailable. The same SDK/worker check passed under WSL:
  `PYTHONPATH=sdk/python python3 -m pytest -q sdk/python/tests/test_client.py
  runtimes/python/tests` — 74 passed, with no package installation.
- Application/developer layout, repository hygiene, source-size, maintained-doc
  links, `git diff --check` and strict OpenSpec validation passed. The source-size
  gate initially caught growth in `computer_tools.go`; moving its unchanged
  vision-catalog helper beside task capability discovery resolved it without
  increasing the reviewed size baseline.

Unrun: real local-model native workflows, installed app/folder-grant UX, macOS
native execution, the frozen cross-platform workflow suite, new workspace UI
Playwright/accessibility/locale and 250 ms feedback measurements, executor release
packaging/installer qualification and the full release CI matrix. No external
agent, container, installed CLI, model or desktop state was changed in this pass.

## Required remaining work

Preview host enrollment, authenticated polling transport, reviewed command
policy/exact approval and the runner's process tools are implemented. Isolated
execution, app/desktop folder grants and matching
release packaging remain incomplete. These remain preview source capabilities,
not generally qualified native workflows or deployed application changes.
Skills/memory and scheduled automation remain required dependent changes in the
approved program. Existing profiles, installed service/CLI/desktop and model
configuration were not changed. No live native-agent/model workflow was run.

## Enrolled-host and command integration milestone

Implemented after the earlier checks above:

- Preview-gated, actor/workspace-bound single-use enrollment; separate random
  host credentials; no user API credential on host polling/reply/heartbeat routes.
  Readiness requires a native check of the exact locally approved scope. Lost
  operation acknowledgments are never redelivered; revocation closes authority.
- Native worker packs pin digest, version, protocol and platform/architecture.
  Worker grants and journal directories require account-private permissions/ACLs.
  No worker asset or grant is accepted from the model's workspace.
- CLI local folder review/supervision, read/edit/trusted session modes, published
  host selection and idle-input cancellation. Declining creates no pairing/state;
  closing an owned host reports unconfirmed termination instead of hiding it.
- Shared reviewed process start/output/wait/stop tools, exact invocation approval
  for commands outside a small routine-project grammar, and independent native
  policy checks. File grants and computer consent do not grant process authority.
  Trusted project code still runs with account-wide authority, not in a sandbox.
- Completion cannot use a start receipt as evidence of process exit. Root Stop
  waits for owned-tree termination; disconnect stays uncertain. Confirmed Stop
  does not erase unknown effects from an in-flight call. Terminal/error/shutdown
  cleanup does not depend on another model response.

Actual isolated checks at this milestone:

- `go test ./internal/server -run
  TestNativeApprovalStartsPinnedWorkerOverAuthenticatedBridge -count=1` passed
  using a real freshly built worker: decline/no state, one approval, pinned
  enrollment, exact Unicode write, actual `go test .` in a disposable folder,
  terminal exit/output, conflicting-command-ID rejection and closed-host refusal.
  This is a deterministic controller fixture, not real-model qualification.
- `go test ./internal/execution ./cmd/offgrid-executor ./internal/cli
  ./internal/serviceclient ./tools/executor-pack` passed. Targeted server command
  authorization/terminal-evidence tests and runner stopping tests passed.
- The final Windows `go test ./...` passed, including the new control and
  enrolled real-worker tests. This verifies source compatibility, not model or
  installed-host UX qualification.
- WSL `go test -race ./internal/execution ./cmd/offgrid-executor ./internal/agents
  ./internal/server ./internal/cli ./internal/serviceclient` passed, including
  enrolled HTTP/native worker fixtures. Final stopping-state changes were also
  race-tested separately with `go test -race ./internal/agents -run
  TestStopDoesNotAcknowledgeBeforeOwnedProcessTermination -count=1`.
- `go vet ./internal/execution ./cmd/offgrid-executor ./internal/agents
  ./internal/server ./internal/cli ./internal/serviceclient ./tools/executor-pack`
  passed; the same package vet set was repeated under WSL after the final
  stopping-state changes.
- OpenAPI generation/check, web TypeScript/build, maintained docs, source-size,
  application/developer layout and repository hygiene checks passed. The web
  build retains the existing large-chunk warning. Control functions were moved
  cohesively out of `durable.go` to satisfy the existing size ratchet without
  increasing its baseline.

No desktop UI setup, isolated-backend, skills/memory, scheduling, installed app
or real-model success is established by these results. No deployment, installed
configuration changes, model downloads, commits or publication were performed.

## Receipt recovery and isolated-runtime preflight milestone

- Canonical checkpoints pin the exact provider intent before dispatch and retain
  authenticated receipts before acknowledging native replies. Receipt lookup uses
  the original operation/payload digest, never `Claim` or dispatch. It works over
  owned stdio and the enrolled bridge, including after cancellation of that job.
- Lost service results and service restart can incorporate conclusive saved host
  evidence without replay or restoring host authority. Missing/partial/ongoing
  evidence remains uncertain. Recovered output is labelled historical and the
  job is interrupted for explicit review, not marked completed or auto-resumed.
- `agent recover-execution RUN_ID` and the matching owner-scoped POST accept no
  replacement result, action or arguments. The normal snapshot/event projection
  exposes readiness without exposing its private intent/transport credentials.
- Process output projection is one bounded page; raw native receipts are retained.
  Process settlement persistence failure revokes the session. Stale runner
  snapshots preserve concurrently saved receipts; human reconciliation clears
  pending intent without turning a user report into verified evidence.
- Isolated-Linux preflight/policy validates a pinned local image, local Unix
  daemon, resource support, explicit none/bridge network and exactly one approved
  bind mount. Implicit image volumes, host networking, special-file roots and
  user-environment forwarding are rejected. No process dispatch uses this backend
  yet: container identity/lifecycle and native setup integration remain required.

Checks actually run:

- Windows `go test ./...` passed after the main receipt and preflight additions;
  the final cleanup/history-label changes passed focused agents/server/CLI and
  execution tests. Windows `go vet ./internal/execution ./internal/agents
  ./internal/server ./internal/cli` passed.
- WSL full race checks passed for execution, executor, agents, server, CLI and
  serviceclient. Final focused race tests passed for receipt/restart/identity,
  canonical conversation, lost-result API, CLI recovery and isolation policy
  cases in agents/server/CLI/execution. One quoted WSL shell invocation failed
  before tests started; rerunning through direct Go argv passed.
- The real built-worker HTTP fixture now also inspects the original receipt
  through private stdio after a manual file change, confirming no write replay.
- Fault tests cover journal claim failure before effects, native receipt failure,
  asynchronous settlement failure, failed service receipt persistence before
  acknowledgment, lost results on both sides of receipt persistence, service
  restart, stale snapshots, repeated/conflicting identities and cancellation.
- Real local WSL Docker preflight passed using the already-installed Alpine image
  ID `sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b`.
  The opt-in test made read-only daemon/image queries; it did not start a
  container, download an image or qualify isolated command execution.
- OpenAPI generation/check, TypeScript, renderer build, source-size, hygiene,
  maintained docs, strict OpenSpec validation and whitespace checks passed.
  The existing renderer large-chunk warning remains.

No installed service/container, CLI, desktop, profiles, model files or user
documents were modified. The task remains an implementation program, not a
claim of real-model or native-app qualification.

## Integrated isolated execution, client selection and packaging batch

Implemented in this batch (not an installed release):

- Native isolated-Linux commands now use the shared process/receipt path. A
  durable name/owner precedes container creation; the verified exact ID is saved
  before start. Strict typed configuration mounts only the selected folder at
  `/workspace`, drops capabilities, uses non-root UID/GID, read-only rootfs,
  explicit none/bridge networking and memory/CPU/PID limits. No image pulls,
  inherited image volumes, credential forwarding or Docker socket mounts.
- Actual attached Unicode output, exit status, timeout and owned-container Stop
  are reported. Unknown start acknowledgments never replay. Ownership/state
  persistence failures prevent start or preserve uncertainty. Worker restart
  and the native stdio supervisor clean up only exact journaled objects; a
  mismatched owner/daemon is not killed. Cleanup does not certify completion.
- CLI host setup accepts explicit isolated image/network/resource selections
  and displays their scope before local approval. Linux paths are validated as
  Linux metadata even when the service runs on another OS. Trusted local mode
  and legacy local context JSON remain unchanged; no silent backend fallback.
- Chat in the shared browser/desktop renderer can choose an already enrolled
  host, inspect its folder/backend/file/command ceiling, and send only its
  reference with a canonical turn. Discovery/refresh does not create grants.
  Unavailable selection is retained and blocked; no service-cwd substitution.
  Readiness/error controls are translated in all nine locales.
- Native release, desktop before/after-pack and CPU/GPU/local-runtime Docker
  recipes now carry the matching executor pack. Desktop verification checks
  identity, platform, version and digest. The native installer retains versioned
  packs for rollback; old bundles without a pack remain installable without
  creating execution authority. A real Windows development pack was built and
  its digest verified, not installed.

Checks actually run:

- `go test ./...` and `go vet ./internal/execution ./internal/agents
  ./internal/server ./internal/cli ./cmd/offgrid-executor` passed on Windows.
- WSL `go test -race ./internal/execution ./cmd/offgrid-executor
  ./internal/agents ./internal/server ./internal/cli ./internal/serviceclient`
  passed. Final removed-root cleanup passed its focused real race test after
  the full run; no fixture result is represented as model qualification.
- Opt-in WSL `OFFGRID_TEST_ISOLATION_IMAGE=sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
  go test -race ./internal/execution -run TestRealIsolat -count=1 -v` passed:
  three Unicode create/read-back, exit failure, Stop and timeout runs, plus an
  abandoned-worker cleanup case. The follow-up removed-root crash case passed
  `-run TestRealIsolationWorkerCrashCleanup`. No owned test containers remained.
- Early crash-fixture attempts failed the private-directory check. The fixture
  now explicitly protects its own disposable native state, retaining production
  permission enforcement; it does not relax WSL filesystem privacy requirements.
- `npm run api:check`, `npm run check`, and `npm run build` in `apps/web`
  passed. The existing large renderer chunk warning remains (about 885 kB).
- Edge Playwright `tests/agent-chat.spec.ts` passed all 39 tests, including ten
  new execution/disconnect/no-duplicate/localization tests and existing consent,
  recovery, streaming and narrow-layout cases. `npm test` passed 72 desktop and
  73 companion tests. These are automated browser/renderer fixtures, not a real
  installed desktop folder-grant usability test.
- Actual `go run ./tools/executor-pack --output <repo>/build/execution-runtime/win-x64
  --version 0.4.20` and desktop `verify(...)` succeeded. No packaged installer or
  container was deployed. Full new CPU/GPU Docker images and signed native
  packages were not built/qualified in this batch.
- Installer entry-point/candidate/checksum/service fixtures, Bash syntax, layout,
  hygiene, source-size, maintained-doc and strict OpenSpec checks passed.
  Release CI gate test passed. Release-context test failed through Windows Bash
  path translation, then passed when run natively in WSL; the failure is not
  hidden by the successful rerun.

Remaining: desktop-native folder/grant IPC, richer command/file activity and
effective profile narrowing, all release-context artifact qualification, dependent
skills/memory and schedules, and full real-model/platform workflow qualification.
Only execution task 2.4 is newly checked; partial UI/packaging tasks stay unchecked.
No installed service, CLI, desktop, models, profiles or user documents changed.

## Native desktop IPC, policy and activity integration batch

Execution tasks 4.2, 4.3 and 4.4 are implemented and automatically checked; the
program is now 14/20. This is not real-model/platform release qualification.

- Native main owns folder picking, explicit account/folder/read-edit-command
  consent, current-principal checks and authenticated pairing. Preload accepts
  only a bounded access enum. A version/digest-verified helper receives a single
  framed private seed, not cookies/API keys, creates native-private grants/state,
  and supervises the same enrolled stdio executor as the CLI. EOF and local Stop
  close the lease. File/tray/recovery controls can stop without model inference.
  Late picker/consent/verification/pair replies cannot create authority after
  cancellation. Lock, suspend, account/workspace change and renderer failure
  revoke ownership. Unconfirmed termination is not acknowledged as stopped.
- The first real helper test found a redundant authenticated identity lookup in
  the credential-free helper. It failed before effects. The corrected native
  path relies on the authenticated main's reviewed pairing ticket and the
  service's independent version/actor/workspace/enrollment checks; authentication
  was not weakened and credentials were not forwarded.
- Optional profile execution ceilings and per-turn restrictions are persisted
  additively. Omission inherits an explicitly selected host ceiling; all-false
  disables its tools. Profile/turn limits cannot create a grant or widen a pinned
  profile. Catalog and dispatch enforce them independently of the live host's
  broader permissions. The profile editor and Chat Tools expose this policy.
- Shared Chat activity shows host/folder/backend, command arguments, true terminal
  exit code, bounded output and incomplete-output indicators. Start/running
  receipts never become completed-process claims. Configuration/readiness controls
  retain all nine locale dictionaries, existing popover/focus behavior and narrow
  layouts; native consent text remains English, as documented for native dialogs.
- Source admission was corrected to accept the reviewed `isolated_linux` backend
  instead of rejecting every non-local snapshot. Unknown backends still fail.
  Tests cover admission policy and independent dispatch denial, not just workers.

Checks actually run after the integrated changes:

- Windows `go test ./...` and affected `go vet` passed. WSL race checks passed for
  execution, executor, agents, server, CLI and serviceclient. The real built
  `--connect` helper fixture passed authenticated pairing, canonical dispatch,
  exact Unicode file read-back, credential non-forwarding and EOF revocation.
- Edge Playwright passed all 48 tests in `agent-chat.spec.ts` and
  `agent-profiles.spec.ts`; desktop passed 80 tests and companion passed 73.
  Final desktop actor/ticket tightening passed its eight focused lifecycle tests.
  API generation/check, TypeScript and renderer build passed; the existing large
  JS bundle warning remains (about 893 kB), not a measured feedback-latency claim.
- CPU and GPU Docker recipes both built their actual `go-builder` stages and
  emitted matching 0.4.20 Linux/amd64 executor packs. Read-only, network-disabled,
  capability-dropped disposable containers verified the actual binary digest
  against each manifest. These are build-only check images, not deployed services
  or GPU inference qualification. The first offline cold build lacked cached Go
  modules and failed; the normal build then succeeded. No models were downloaded.
- A macOS/arm64 executor pack cross-built and passed desktop digest verification;
  that is not native macOS execution evidence. Full signed desktop packages,
  final CPU/CUDA runtime images and human native-dialog usability remain unrun.
- Installer entry-point fixtures passed. The new isolated executor-install test
  passed versioned pack retention, old bundles without grants, and invalid
  version/path rejection before replacing files. It is now included in CI.
  Maintained docs, source-size, hygiene/layout, whitespace and strict OpenSpec
  checks passed. No source-size exception was enlarged.

Remaining approved work: dependent skills/scoped automatic memory, durable
automation, their integration/qualification, and the broader frozen native/model
suite. No installed service/container, CLI, desktop, profiles, models or personal
documents were replaced; no commit or publication occurred.

## Skills and scoped memory integration batch

Implemented the approved dependent `native-agent-skills-memory` change across
storage, canonical runner, authenticated APIs, CLI and shared profile UI. Skills
are immutable owned revisions with canonical byte digests, strict bounded imports
and progressive loading. Drafts retain job provenance but never activate or run
scripts. Profiles pin exact reviewed revisions; old profiles/tasks gain nothing.

Automatic memory defaults off and is scoped by actor/workspace/profile. Root jobs
pin live consent revisions; revocation/re-enable cannot reauthorize old jobs.
SQLite writer transactions serialize consent and writes across managers. Entry
CAS, payload-bound operation identities and deletion tombstones prevent stale
overwrites and deleted-response replay. Deletion clears live text/cached response
bodies, not historical tasks, backups or forensic SQLite remnants. No child gains
independent context-resource authority or an extra model/tool budget.

Evidence actually obtained:

- Windows `go test -json ./...` completed with zero failed events and 34 passing
  package summaries (log under ignored `build/native-context-go-tests.jsonl`).
  The final affected Go suite and `go vet` passed for agentcontext/agents/server/
  CLI/serviceclient/command/API packages as applicable. New fixture cases cover
  invalid paths/YAML, immutable revision selection, corrupt bytes, foreign owners,
  storage rejection, concurrent entry/consent CAS, live revoke/re-enable,
  archived-profile inspection, deletion and a transaction-abort failure.
- WSL race checks passed across agentcontext, agents, server, CLI and
  serviceclient using `-run 'TestContext|TestSkill|TestMemory|TestProfile|TestReadSkillDirectory'`.
  The serviceclient package had no matching focused tests; its complete normal
  suite passed separately. These are deterministic tests, not model quality.
- A real loopback HTTP CLI fixture imported a disposable folder, inspected and
  attached the exact skill revision, enabled/disabled consent, edited/inspected/
  deleted Unicode memory and created no execution job. A scripted canonical
  runner fixture loaded a skill, read/wrote memory and produced an inert draft
  in one root with its unchanged six-decision budget; no script was executed and
  review did not attach a draft automatically.
- Edge Playwright passed **58/58** across `agent-context.spec.ts`,
  `agent-profiles.spec.ts` and `agent-chat.spec.ts`. New cases cover attachment,
  Unicode inspection, explicit consent, entry edit/delete, no accidental job or
  profile submission and nine locale layouts at 320x480. Initial failures exposed
  an asynchronous checkbox test assumption and an unstable wrapping textarea
  label; tests now await service acknowledgment and controls have explicit IDs.
- API generation/check, TypeScript and renderer build passed. The existing large
  bundle warning remains (about 912 kB); no startup/250 ms performance claim is
  inferred from a successful build. Desktop tests passed 80/80; companion tests
  passed 73/73 using `npm --prefix apps/computer test`. An earlier incorrect
  companion glob selected zero tests and is not counted as evidence.
- Maintained docs, repository hygiene, application layout, source-size ratchet,
  whitespace and strict OpenSpec validation passed. The route fixture initially
  rejected the three added authenticated registrations; it was reviewed and
  updated without changing existing route/middleware order. No source-size
  allowance was increased.

Real-model skill following, memory usefulness, signed native packages, human
usability and the broader frozen native/backend suite remain unverified. This
batch did not download models, change installed data, deploy, replace the desktop
or CLI, commit or publish. Durable automation and final program integration remain
required work; this evidence does not mark the whole native-agent program done.

## Calendar scheduling and program integration batch — October 26

Implemented `native-agent-automation` in manager-owned SQLite and the existing
application-owned runner. No second executor, client timer or inference scheduler
was added. Policies pin actor/workspace, profile revision, model, resource/grant
identity and budgets. Typed offline IANA calendars skip gaps and select the first
fold, including Lord Howe. Claims/cursor advancement precede root creation;
stable request linkage precedes dispatch. Startup/clock gaps record missed work;
unsettled work blocks overlap. Recovery observes saved identities without creating
or continuing another task. Disable/re-enable cannot revive an old root's policy.

Occurrence admission rechecks current actor/profile, model asset/runtime, tools,
skills, memory consent, Knowledge IDs and host grant. Knowledge retrieval refreshes
bounded text under the pinned IDs; changed scope/model/budget is rejected before
claim. Preparation is separately cancellable, limited to one 30-second callback,
does not starve the normal job queue and drains on runner shutdown. Its completion
does not cancel an admitted root. Providers recheck their own current authority;
unrelated attachment failures do not erase independent partial results.

Added authenticated owned policy/history APIs, `agent schedules` CLI commands and
capability-gated Activity controls in nine locales. Review shows the exact profile,
model, host/folder/backend, permissions and budget. Changes clear confirmation;
creation/inspection/refresh never dispatch, and history links the canonical job.
No desktop grant, replay/run-now action or model-created schedule is exposed.

Automatic evidence collected in this batch:

- Windows `go test -json ./...` passed **35 package summaries**, zero fail events;
  **58 opt-in/platform tests skipped**, not counted as qualification. The ignored
  log is `build/native-schedule-go-tests.jsonl`. Subsequent focused schedule,
  authority, preparation, canonical-program and route tests passed after changes.
  Full affected packages `internal/agents`, `internal/server`, `internal/cli`,
  `internal/serviceclient` and `pkg/api` also passed. `go vet` passed for agents,
  server, automation, CLI, execution and agentcontext.
- WSL focused race checks passed for automation/agents/server/CLI calendar,
  occurrence, clock, profile-copy and scheduling scenarios. An earlier race run
  exposed shared pointer/slice mutation while JSON-unmarshalling saved profile and
  schedule settings into shallow copies. Deserialization now uses fresh detached
  structs; concurrent input-preservation regression and race checks pass.
- The broad CI race command passed the service/agent/storage/inference/integration
  packages but initially timed out in the two command packages while tests and
  browser workers competed. The actual failures were retained: isolated startup
  exceeded its 10-second fixture bound and executor compilation exceeded its
  60-second fixture bound. `go test -race -p 1 ./cmd/offgrid ./cmd/offgrid-executor
  -count=1` subsequently passed both. No product deadline or assertion was weakened.
- Canonical scripted fixtures used explicitly disposable approved local folders
  and independent `os.ReadFile` checks for exact Unicode. They verify scheduled
  write/read-back, duplicate inspection/ticks, host/model loss, live actor disable,
  pinned-profile updates, grant/policy revocation, unchanged budgets and blocked
  replay. Other tests exercise claim/create/link/restart boundaries, SQL failures,
  unknown effects and queued-root progress while preparation remains blocked.
  These local test folders are **not a security sandbox**.
- A researcher → writer → parent-save fixture passes for interactive and scheduled
  roots: the parent reads an exact selected source, researchers/writers receive
  bounded source/prerequisite context, children have no folder/desktop/memory or
  recursive authority, and only the parent writes and independently reads back
  the briefing. Normalized tool decisions and the original aggregate decision
  budget remain enforced. Its sources and model responses are synthetic; this
  does not qualify public research accuracy or real-model writing quality.
- Initial focused Edge scheduling tests passed **10/10**, including nine locales
  at 320×480 and management without job submissions. The first attempted run was
  blocked by the fixture's onboarding dialog; matching the established explicit
  onboarding-complete fixture state corrected the test setup.
- The four-worker disposable-service browser run reported **391 passed, 7 failed,
  10 skipped**. Failed contexts show `ERR_INSUFFICIENT_RESOURCES`, page-target
  crashes or navigation/screenshot timeouts. Their artifacts were retained in
  ignored `build/native-schedule-browser-parallel/results`; this run is not
  reported as green. The complete lower-concurrency rerun is recorded below.
- API generation/check, TypeScript/build, 271 maintained documentation links,
  source-size, repository-hygiene, application-layout and whitespace checks passed.
  The existing large-bundle warning remains (about 930 kB); a build is not a
  measured startup claim. Structural/scoring fixture tests passed 24/24; Unix
  installer entry-point and executor install/rollback fixtures passed in WSL.
- Desktop unit tests passed **80/80** and companion tests **73/73**. Python SDK/
  worker tests passed **74/74** through WSL Python. The Windows `python` alias was
  unavailable. The packaged desktop-theme script was mistakenly invoked without
  its required executable and exited before launch; it is not platform evidence.

Real-model/backend repetitions, installed macOS/Wayland/X11/native desktop control,
signed packaging and human usability remain separate qualification gates. No
models were downloaded; installed container, desktop, CLI, profiles and user data
were not replaced. No commit, push, publication or release was performed.

Final browser verification: `node scripts/test/test-web-workspace.mjs
build/native-schedule-offgrid.exe --workers=1` with Edge passed **398/398 selected
tests**, with **10 opt-in checks skipped** (408 discovered). The earlier seven
failures all passed without weakening assertions. Existing fixture feedback gates
also passed; for example streamed UI p95 was about 60 ms and local Stop feedback
11.5 ms, below the 250 ms fixture gate. Those timings are local fixture evidence,
not installed native or real-model throughput claims.

After adding explicit editor focus restoration, a rebuilt renderer's focused
scheduling suite passed **12/12**. The visual matrix captured light/dark
1440×900, 1280×720, 390×844 and 320×480; 200% zoom and keyboard return to the
invoking button passed. Light desktop and dark narrow captures were inspected.
Visual inspection exposed a browser-default time input and blue checkboxes:
the time control now consumes the authoritative shared control styling, while
schedule checkboxes use the existing monochrome token. No competing input theme
was introduced. Follow-up control-consistency verification is recorded below.

The rebuilt final renderer passed **23/23** focused scheduling/control-consistency
tests against another disposable service. This includes native time-control
sizing/radius, both visual matrices, all nine locales, keyboard restoration,
forced-colors select compatibility and cross-page containment. The entire
398-test suite was not repeated after this bounded control-style correction;
the directly affected suites and final build passed.

After the final admission/resource additions, full affected Go packages passed
again (`internal/cli`, `internal/server`, `internal/agents`, `pkg/api`), and focused
WSL race tests passed for `TestSchedule`, `TestNativeProgram` and the shared-input
profile regression. All 26 OpenSpec changes/specs strict-validated. Successful
checks do not close the explicit real-hardware task below.

### Program acceptance reconciliation

The following reconciles the execution change's approved scenarios and its two
dependencies. “Implemented” and “automatically tested” remain distinct from the
real-model/backend repetitions required by task 6.2.

| Acceptance | Implemented boundary and automatic evidence | Qualification limits |
| --- | --- | --- |
| Selected terminal folder; follow-up read | Canonical conversation execution references, explicit local grant, rooted raw file tools; CLI workspace/conversation and service execution fixtures | Real-model follow-up behavior is not established by scripted calls |
| Two independent workspaces | Provider-local directories and actor/workspace/job binding tests; CLI isolation and worker session tests | No ambient Windows/WSL/container path conversion is supported |
| Computer consent alone; changed grant | Execution admission/dispatch require their own current grant; forged, revoked and cross-owner references are rejected | Installed native-consent UX and human review remain separate |
| Large/changed reads, outside links, stale edits | Bounded revision-bound pages, traversal/link/type rejection, preimage replacement and exact Unicode read-back tests | Noncooperating external writers prevent a universal OS-level CAS promise |
| True command failure; foreground/background output | Supervised worker process tests retain exit, timeout, bounded output and incomplete/unknown states | Trusted local commands retain the signed-in account's OS authority |
| Duplicate dispatch, restart, Stop and PID reuse | Journal payload identities, real private-stdio executor fixture, owned-tree stop and unknown-effect recovery tests on Windows/WSL | Broader installed OS/model combinations and real emergency-stop journeys are not qualified |
| Routine trusted commands and isolated Linux | Reviewed routine grammar/per-call approvals; explicit isolated backend mounts/network/container ownership tests and prior disposable backend evidence in this record | Local folder fixtures are not isolated Linux; no isolation claim is inferred for personal-host execution |
| Old services, jobs and profiles | Capability gates, optional projections, strict private protocol/pack identity, text-only native adapter gates, old-job and no-implicit-grant tests | Unqualified MLX/Transformers tools/vision remain unavailable |
| Hidden local scripts, foreign host paths, legacy computer-only scope | Discovery and independent dispatch deny renamed local execution and service fallback; MCP/computer composition tests retain their narrower contracts | No broader specialist tool eligibility is inferred from MCP read-only hints |
| Skills and memory | Completed dependency, actual owned service/CLI/UI tests, progressive inert skill load/draft/review, opt-in scoped consent, CAS/idempotency/deletion and race checks | No claim of real-model skill following or memory usefulness |
| Scheduled work | Current dependency: IANA/DST, durable claims, missed/no-overlap/recovery, live resources, canonical parent/children, CLI/Activity and Unicode file fixtures | No OS wake-up, desktop grant restoration, catch-up or schedule replay; unattended model quality remains unqualified |
| Researcher → writer → parent result | Existing bounded dependent jobs integrated with selected-source read, normalized tool decisions, parent-only save/read-back and aggregate budgets for interactive/scheduled roots | Synthetic source/model fixture is not public-source research or a subjective report-quality evaluation |
| Calm application parity | Shared browser/desktop renderer, nine locale and RTL controls, response actions, follow-latest, safe recovery and full isolated-service browser suites | Browser fixtures do not establish installed native-platform usability or signed package qualification |

The qualification blocker is evidence, not permission to change enforcement:
three repetitions of the frozen real-model workflows on each advertised backend
are still required. Do not advertise every model/provider as reliable from these
fixtures, count a safe block as task success, or clear task 6.2 from compilation.

### October 26: foreground CLI correction and authorized installation

The user approved the one-shot CLI refinement and replacement of the container,
desktop and matching WSL CLI. Terminal `agent run` now offers its actual current
local folder through the existing supervisor and streams the same canonical root.
`--workspace-folder`, `--workspace-access read|edit|trusted` and `--detach` make
selection explicit. JSON and piped submissions retain asynchronous compatibility;
detached work cannot abandon a newly owned folder lease. Decline or unavailable
execution does not fall back to container files. Interrupted/disconnected streams
inspect and stop the existing root, without creating a replacement job.

The CLI now renders committed, typed directory observations separately from
unchanged assistant prose. Repeated snapshots are deduplicated; malformed,
uncommitted, oversized and incomplete observations cannot become invented full
listings. Unicode filenames and terminal escape handling have regression tests.

Live checks used only newly created `/tmp/offgrid-native-cli-smoke-*` directories
containing two synthetic files and an empty directory. No saved user job was
resumed. Earlier failed evidence is retained:

- `run-4740760d5e72702778b9b1f54030e858`: the correct local listing was recorded,
  but assistant prose omitted the filenames and the old renderer hid tool results.
- `run-355aac70df1ccf930a5afd38680ecd2f`: the model supplied `revision: "."`.
  The host refused it. The tool schema now rejects malformed revision values
  before dispatch using the existing bounded decision correction path.
- `run-b7fe88d6d2441e40e3b4c661f2551ae6`: the first corrected installed build
  completed, displayed all three observed names and repeated them in its answer.
- `run-6804b9d09e0110c8eb6e390a65470bf8`: a fresh repetition listed the right
  folder, then proposed an absolute child path, which the host rejected. The
  schema now also rejects absolute/drive/UNC/backslash paths before dispatch.
  It does not translate host paths or weaken independent rooted checks.
- `run-f743e7c9d4ce5364265e2fd0484848e8`: the path-constrained build completed
  and returned the actual three entries, including the Unicode filename.
- `run-7338d3f0eeabb01456be7feaaa659504`: the next repetition copied the
  parent's valid revision into a fresh child-directory read. The host correctly
  refused that mismatched revision. The final model-facing page schema separates
  fresh reads (offset omitted/zero, revision omitted/empty) from continuations
  (positive offset and revision required). Write preimages and the worker's
  legacy first-page precondition support remain intact. Structural checks do not
  certify that a continuation hash actually belongs to the selected target.

The first longer schema descriptions caused the 8K-context researcher/writer
fixture to stop after its recorded save. Repeated guidance was shortened instead
of increasing its context or removing checks. Adding the page relationship
constraint exposed this margin again; the repeated tool descriptions were
compacted further, retaining independent enforcement. `go test -p 1
./internal/execution ./internal/server -run 'TestWorkspace|TestExecution|TestNativeProgram'
-count=1` passed on the final catalog. The revision/path schema tests
exercise the real normalized decision validator, including valid nested Unicode
paths and invalid placeholders. A schema-shaped hash still does not establish
freshness: the host independently checks revisions, identity and confinement.

Installation uses the retained local GPU runtime, not a new inference engine or
model download. Consistent stopped-volume backups, old desktop directories and
CLI/executor assets remain under the restricted local deployment backup. One
attempt stopped at backup because Docker had removed the previous untagged image;
the helper restored the old container automatically. The subsequent backup used
the separately retained original runtime image. Execution preview is explicitly
enabled for controlled testing; installation grants no folder or process access.

Existing startup logs still report historical conversation-bookkeeping recovery
warnings and the existing authentication-disabled container-listener warning.
The published service port remains loopback-only. These are not represented as
an error-free log audit, and old work/configuration was not silently repaired.

The earlier full automatic evidence above remains applicable to unchanged UI and
subsystems. CLI-specific compatibility/cancellation fixtures, committed-observation
tests, actual Windows packaging and WSL race checks were additionally exercised.
Final installed-build identities and fresh repetitions are recorded below.

Final installation (local candidate, not Git publication):

| Item | Verified identity |
| --- | --- |
| Service | `v0.4.20`, revision `0161a68-dirty-cli-pages-20261026` |
| Running container | `7856c4949b2731ac8f1aaadcc46c2adbcc031495276b1d874365f4dee3dd25a6` |
| Image | `offgrid-llm:local-native-agents-20261026`, `sha256:8052dc41fd5756a58f36a326e24f9db269060e271aa0c3238e5430010f9e680c` |
| Previous container | `offgrid-rollback-native-20261026-023153`, stopped with automatic restart disabled |
| Workspace | `d8b307945e8e3a9ee6025092525318a4`, unchanged; all five model IDs preserved |
| Renderer | `f630485ce84e7db97345b9edf57d344c352a1094fe83b5363e3000f29e9ec5f5`, matching service and desktop |
| Installed desktop | `%LOCALAPPDATA%/Programs/OffGrid LLM Desktop`; bundled service SHA-256 `9660a3410e97b5d09e2a0a8dd90fbe5d10331b8b29a22a3ec9d99bae5b3ec744` |
| Windows executor | SHA-256 `508a7267fd618b9f41c72ffb0823bb98f79cec48cf58331b443ec0a3072d498a`; installed pack and companion verified |
| WSL CLI | `/home/phil/go/bin/offgrid`, SHA-256 `bd8d84c7825ea3af72a9c5ca2bcfe015dae8798af2235c1e7da5f8c3fca3031b`; login-shell command resolution verified |
| WSL executor | `/home/phil/go/bin/execution-runtime/0.4.20`, SHA-256 `ded235266ee0d541dd7bd4877161359e5f934c537dce31b5a88419bdcec426de` |
| Rollback data | Restricted `%LOCALAPPDATA%/OffGridDeploymentBackups/native-agents-20261026-010240`; consistent `offgrid-data-before-20261026-023153.tar.gz`, original/intermediate desktop and CLI assets retained |

The final installed desktop/service compatibility handshake returned `ready`.
The sole running OffGrid container publishes only `127.0.0.1:11611`; GPU access
still reports the RTX 5060 Laptop GPU (8,151 MiB). No container volume was deleted,
no configuration/model selection was replaced, and no saved work was replayed.

Additional commands/results:

- `go test -p 1 ./internal/execution ./internal/cli ./cmd/offgrid -count=1`
  passed; the complete agents package also passed. The earlier full server run
  captured the longer-schema context regression; after compacting the catalog,
  `go test -p 1 ./internal/server -count=1` passed (129.663 seconds).
- WSL `go test -race -p 1 ./internal/cli ./cmd/offgrid -count=1` passed, followed
  by `go test -race -p 1 ./internal/execution ./internal/server -run
  'TestWorkspace|TestExecution|TestNativeProgram' -count=1` on the final catalog.
- `go vet ./internal/execution ./internal/cli ./cmd/offgrid`, documentation
  (271 maintained files), repository hygiene, application layout, source-size,
  whitespace and strict OpenSpec validation passed.
- Windows packaging completed and copied assets were verified after packaging.
  `node scripts/test/test-desktop-theme.mjs <installed-executable>` passed against
  an isolated profile: light/dark/system, nine readable options, 200% zoom across
  pages and saved appearance after restart. Evidence is in the private temporary
  `offgrid-desktop-theme-8CNgJw` directory. The renderer is unchanged in the final
  contract-only rebuild; final installed pack/companion/renderer checks passed.

These are implementation, regression and installed smoke checks. Task 6.2's
broader real-model/backend and native-platform qualification remains unchecked.

Final installed PTY smoke command: WSL `python3
/mnt/d/offgrid-llm/build/test_native_cli_pty.py`, three fresh invocations. Each
launched the installed CLI with the unchanged prompt `list files in this
directory`, the explicitly selected Meta-Llama model, read-only access and an
eight-decision cap. The fixture accepted consent only for its own synthetic folder.

| Run | Foreground/local-folder evidence | Assistant summary quality |
| --- | --- | --- |
| `run-e90a6b8b4ad65211a5da9457c739ea1b` | Completed; all three actual names displayed; one approval; host released | Only `Directory: .`; incomplete |
| `run-c32e04b22f0c5f3c52273b9958860b29` | Completed; all three actual names displayed; one approval; host released | Omitted `alpha-cafe.txt` after an unnecessary later page; incomplete |
| `run-f8639ab11dbd26272d1d4086ac6e3fa2` | Completed; all three actual names displayed; one approval; host released | Only current directory identity; incomplete |

The smoke checks independently fetched each canonical job, checked the exact
pinned temporary root and committed listing, rejected service-file fallback,
checked filenames in terminal output, and confirmed lease removal on exit.
All three passed those CLI/host-flow assertions. **Zero of three assistant
summaries repeated the complete listing.** These are not three successful
model-quality workflow qualifications, and no assistant output was rewritten to
hide that failure. The independently observed listing is explicitly labelled in
the terminal. Model result synthesis/redundant pagination remains a known
qualification finding, rather than a hardware-only unknown. Task 6.2 remains open.

After the tests the service was healthy, with zero active/queued generations,
no model switch in progress and zero ready invocation-owned execution sessions.
No deployment backup, test failure evidence, installed model or user workspace
was deleted. This pass made no commit, push, publication or release.

## October 26: selected-folder workflow investigation

The preceding failed summaries remain evidence. This continuation found and
corrected several distinct defects; none was resolved by changing the selected
model, granting service-filesystem access or replaying a saved user job.

- Directory pages now identify their offset and total entry count. A completed
  continuation is explicitly distinguished from a complete first-page inventory.
  Bounded quoted names accompany raw receipts without changing journal evidence.
  The model receives only recorded next offsets/revisions for recent targets;
  writes, directory changes and process starts invalidate old continuation menus.
- The page schema used valid JSON Schema composition that the pinned llama.cpp
  grammar converter handled differently: union branches were visited before
  sibling object properties, omitting the required read path from the generated
  grammar. Each branch now contains the complete object contract. This follows
  the behavior in the [pinned upstream converter](https://github.com/ggml-org/llama.cpp/blob/b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9/common/json-schema-to-grammar.cpp).
  Strict server validation and host revision checks remain in place.
- Native context counting previously requested the global 65,536-token setting
  even when generation used a task-pinned smaller allocation. Qwen's counting
  path repeatedly loaded/capped/unloaded the model before any action. Counting
  now acquires the same task allocation as generation. A regression covers
  8,192/32,768 allocations with the global setting unchanged; restoring the old
  line through an isolated Go overlay fails those assertions.
- Prompt guidance now distinguishes authorized workspace commands/files from
  desktop-only access, requests the latest turn's outcome and discourages
  re-running an already concluded operation. This is guidance, not proof that
  every model will follow it. Exact authority and effect checks remain code-owned.
- Interactive chat retains its conversation and local folder grant after a
  fresh canonical lookup proves the exact submitted turn is stopped. Unknown or
  still-running outcomes stop that same root; there is no resubmission. Six
  regression cases cover stopped, completed, live and mismatched identities.
- Missing host executables now have a bounded actionable diagnostic. Reviewed
  PATH command names are described in the schema; absolute/custom executables
  still require exact approval. An observed absolute Go proposal was cancelled
  in the test workspace, rather than bypassing that approval.

The stricter one-shot listing fixture requires the assistant's final answer to
contain every actual entry, as well as validating the committed host receipt,
exact temporary root, one local approval, absence of service-file fallback and
lease cleanup. Actual selected-model results:

| Model | Canonical roots | End-to-end seconds | Final answer |
| --- | --- | --- | --- |
| Meta-Llama 3.1 8B Q4_K_L | `run-e05b90a43cac1d92fadce683402cf823`, `run-4c2792a2f32da668489174ebbf9055dd`, `run-7f8efaa6aae156c09077e0bc11878a0b` | 71.289 cold; 24.279, 22.856 warm | 3/3 complete observed listings |
| Qwen2.5 7B Q4_K_M | `run-2f607308bcd4c5e48842e4994b03bedd`, `run-b01c163ddf5526e3675286102d11ab1e`, `run-4fc4f41c22798c3e0b9ad08205904ffd` | 18.727 cold; 6.671, 6.436 warm | 3/3 complete observed listings |

These are selected WSL/local-folder and RTX 5060-backed inference checks, not
qualification of every installed model/backend or the full frozen suite.

Fresh interactive tests then exposed additional findings:

| Disposable evidence directory | Observation |
| --- | --- |
| `/tmp/offgrid-native-conversation-vqsh2p91` | Listing passed; follow-up read exhausted three repairs under the incomplete grammar branches. |
| `/tmp/offgrid-native-conversation-qy9e_toj` | Listing, exact Unicode read and exact create/read-back passed after the grammar fix. Test launch lacked Go on its non-login PATH; the command did not start. |
| `/tmp/offgrid-native-conversation-xxwly2tz` | File checks passed. An absolute Go path correctly required exact approval; the test-owned pending root was explicitly cancelled. |
| `/tmp/offgrid-native-conversation-gws59j7q` | All file checks and real Go exit/output checks passed. The assistant unnecessarily ran the test twice and answered with filenames. This is a workflow-quality failure; the fixture now also rejects duplicate starts and a missing test-result answer. |

Each fixture owns only a synthetic temporary folder and disposable profile.
Profiles are archived, original files are hashed and checked, and captures are
outside the approved folder. Commands now run under a login-shell PATH matching
the user's terminal. Failed test roots are inspected/cancelled by exact identity;
no user task is restarted.

### Worker shutdown race

The live Stop fixture `run-3048875c5e8658c9c606cdeea247bdee` exposed an actual
supervisor shutdown defect: closing worker stdin was immediately followed by
killing the worker, racing its cleanup of separately grouped command children.
The test failed its five-second owned-process termination gate. It retained
failure evidence and used separately identified fixture-only cleanup; that
cleanup did not convert the run into a pass.

Shutdown now allows a bounded one-second EOF cleanup period before forced
termination. Worker exit errors are retained; forced termination reports
unconfirmed cleanup even when the worker itself exits. An actual compiled-worker
regression starts a child via Go, closes the endpoint and independently checks
the journal for a conclusive cancelled result with an exit code. A second test
ensures a forced shutdown cannot claim successful cleanup. Windows and WSL race
checks pass these tests.

The first updated live Stop fixture, `run-e74b2250e4569e7d508b7a2fb78bfafa`,
observed its exact owned process disappear in 20.63 ms, with no late write.
Its service root remained `uncertain`: host disconnection prevented remote stop
acknowledgment. Local process termination and canonical remote acknowledgment
are different evidence; no reconciliation or completion was invented.

Automatic checks for this continuation passed:

- Windows `go test -p 1 ./internal/execution ./cmd/offgrid-executor -count=1`;
  `go test -p 1 ./internal/agents ./internal/server ./internal/cli -count=1`.
- WSL `go test -race -p 1 ./internal/execution ./cmd/offgrid-executor
  ./internal/cli ./internal/server -run 'TestPinnedStdio|TestForcedWorker|
  TestInteractiveStream|TestMissingExecutable|TestWorkspacePage|TestNativeProgram|
  TestAgentContextCounter' -count=1` (one uninterrupted regular-expression argument).
- `go vet` for execution, CLI, server and command packages; documentation,
  source-size, application-layout and strict change validation. Repository
  hygiene initially found a Go negative-control file in ignored `build/`; its
  recoverable overlay source was renamed to `.go.txt`, and hygiene then passed.
- Matching application-container, Windows desktop and WSL CLI/executor builds.
  No new inference runtime or model download was introduced. The renderer did
  not change; earlier renderer/theme evidence is not relabelled as new testing.

The full Go suite and repeated strengthened conversation/Stop checks are recorded
below when finished. Main task 6.2 remains unchecked while model/backend/frozen
workflow qualification is incomplete. Unavailable native platform and hardware
checks remain unverified; there is no commit, publication or release in this pass.

### Longer conversation and stream transport checks

The strengthened `/tmp/offgrid-native-conversation-aguo23hn` run still failed:
`run-38f4bef81fa7fce57f31eac32a4f80c6` had real successful process evidence but
answered with filenames. General prompt guidance alone did not resolve that
finding. The inference view also carries committed process states for up to
four recent handles. It requires a
matching execution session, operation and original start before showing a
terminal state; original requests, tool messages and receipts remain unchanged.
Tests check exclusion of foreign sessions, repeated request text and invented handles and confirm that
projection does not mutate persisted state. This is model assistance, not
automatic semantic verification or an authorization source.

An experimental repeated current-request prefix was removed after a fresh
fixture (`/tmp/offgrid-native-conversation-5qky7gbh`) looped on already correct
write/read pairs. The canonical request remains in its original message; it is
not reissued as a fresh instruction after every tool result. That failed run is
preserved rather than counted as qualification.

The next longer fixture, `/tmp/offgrid-native-conversation-4mjqwu30`, exposed a
separate transport defect during `run-fe59efbe7377d1b97edada96bab56176`: the
ordinary authenticated HTTP client's total 30-second timeout also applied to an
agent event-stream body. The client stopped the exact submitted root after the
stream disconnected. There was no resubmission, but healthy longer work could
not finish through that client path.

Agent event following now uses an explicit authenticated GET stream method with
a bounded 30-second startup, caller/context cancellation and close cleanup,
without a total response-body timer. Ordinary JSON/mutation request timeouts,
selected origin, credentials and no-redirect policy remain unchanged. Regression
tests exercise a real stream longer than the ordinary timeout, cancellation
during body reading, bounded header wait, redirect rejection and foreign-origin
rejection, with no duplicate request. CLI/control tests and WSL race checks cover
both one-shot and conversation use of this transport.

`go test -p 1 ./... -count=1` passed across the repository after the shutdown fix.
The subsequent progress projection passed the full agents suite plus focused
WSL race tests. The event-transport change then passed service-client, CLI and
command suites; its first compile found an obsolete import, removed before the
passing run. These distinctions preserve the actual order and extent of testing.

### Final CLI continuation results

The governed single-step adapter now projects recorded single tool calls back
into its own decision envelope, with arguments retained as JSON objects. It does
not parse ordinary prose as a call, mutate canonical history, promote malformed
JSON/duplicate fields, or promote legacy/mixed batches to new decisions. Ordinary
assistant conversation text stays unchanged. An earlier candidate also wrapped
that prose and failed exact editing in both selected models; its failed evidence
is retained (`d3w93q4o`, `mbdvej_b`). The final adapter preserves prose.

With that adapter, Qwen2.5 7B passed three fresh four-turn conversations:

| Evidence directory | Listing / follow-up read / exact edit / real Go result roots |
| --- | --- |
| `/tmp/offgrid-native-conversation-r05p50gy` | `run-0f593fa08dc1aeec4008935b30c3c8cf` / `run-de710b0465286314c483c436b8315976` / `run-afb7e31ee5a56715d681fc32fa801a5f` / `run-167f9f1d5498bf0b71b1622e6d8905a2` |
| `/tmp/offgrid-native-conversation-oc_y2gtw` | `run-3f526b7a875bda254aca7e9f70285497` / `run-116ba9650700abacf692ecd9963d0480` / `run-06239ac19069811076e4d751d8bd4c7a` / `run-83ae87966de33e246e978d244c3cb93d` |
| `/tmp/offgrid-native-conversation-mjxlok2v` | `run-e160133cc39dcac3896f645401cd0aba` / `run-2005d33767d0a689f022301853ca387c` / `run-9deacadbe5b66a8e7407a2e634f24d00` / `run-147e7bbe0beb65ec1575db6e463fdf63` |

Each passing conversation retained one locally approved folder/session, preserved
the original files, independently checked exact UTF-8 bytes, started the requested
Go test only once, inspected its actual successful exit/output and gave a matching
final reply. Some file turns still made a redundant, conclusive same-content
replacement; these are not optimal action counts or replay of uncertain effects.
Generation/build activity overlapped some measurements, so these wall times are
not fair standalone performance benchmarks or a speed guarantee.

**Remaining observed model failure:** the fresh Meta-Llama comparison
`/tmp/offgrid-native-conversation-vj_2392z` passed listing/read, then
`run-e86565219b91e45db479872eefd73a27` wrote altered Unicode text. Exact-byte
validation rejected the fixture despite the model's completed status. This model
is not qualified for this exact-edit workflow. No default model/profile was
silently changed to conceal that result. Full task 6.2 remains open.

The installed command routing also still applied a legacy localhost client to
interactive chat. It now reserves that override for actual legacy commands.
Default chat, explicit chat/model options, workspaces, skills, memory, schedules
and execution recovery retain the configured authenticated service. An in-memory
transport test exercises default/chat/model entry points, rejects discovery
without authorization and proves that neither a real localhost service nor host
process is contacted. It passes on Windows and WSL with race detection.

The real fixtures are now maintained under `scripts/test/`, with explicit trusted
CLI/model arguments and a required disposable-workspace opt-in. The
[fixture guide](../native-agent-workflows.md) records scope, prerequisites,
private retained evidence and commands. Four boundary/golden-byte tests passed.
An initial copied fixture had an encoding error (`sx7w8noj`); it is excluded from
Unicode qualification. ASCII Unicode escapes plus an independent expected digest
now protect the fixture's exact intended bytes.

Final installed-build recheck using the corrected maintained conversation fixture:
`/tmp/offgrid-native-conversation-ncu18sua` passed all four turns, rooted at
`run-40f51f6d1fce4cf57d12f3c12d116119`,
`run-7ce2b6538c387ad8ffc0a6e55e5abf1f`,
`run-2e5f97f9d858949cccef3d560d464e98` and
`run-9449d90fdbebe736edce4de2498b8fb4`. The local approval prompt now identified
the selected `http://127.0.0.1:11611` endpoint. The cold listing took 61.356 seconds
and completed without the former 30-second stream cut-off; later turns took
3.550, 13.051 and 6.549 seconds. This is completion evidence, not a cold-start
latency acceptance claim.

Owned-process Stop passed three fresh Qwen fixtures after the shutdown fix:
`run-e74b2250e4569e7d508b7a2fb78bfafa` (20.63 ms),
`run-542f07233aee382bd05776b39e9e120b` (20.49 ms) and
`run-6696086419d22ab29cbe771b513bc528` (20.35 ms). The final maintained Stop
fixture `run-f855baeba62e0c3c54d1d91037a6ea1d` also passed (20.46 ms).
All observed their original process disappear, with no late write. All retained
an uncertain remote service acknowledgment; none claimed remote reconciliation.

Final installation, retaining the original runtime, volumes, models and rollback:

| Component | Identity |
| --- | --- |
| Service revision | `v0.4.20`, `0161a68-dirty-native-cli-final-20261026` |
| Container | `58213f286d5211cb2bccde9f9f0ce44b34a5ef95164ddafe8362f2778accd598` |
| Image | `sha256:38bbc999a6f17609e9f24fd304390135cf64f3e43f1a896c77dc0b97e60b6ccb` |
| Previous stopped container | `offgrid-rollback-native-20261026-103648`, restart disabled |
| Consistent data backup | Restricted deployment directory above; `offgrid-data-before-20261026-103648.tar.gz` |
| Windows bundled service | SHA-256 `73477493ad48e9377907c3b1180efb665e5a0dbf579c085d0a2d92cbeb67ba9c` |
| Windows executor | SHA-256 `3636f322effc6e1e3d700b8faf24e006e045ea752d7f82ea2d417cd6bc4966a4` |
| Installed WSL CLI | `/home/phil/go/bin/offgrid`, SHA-256 `8127c117ff1b99f2a5c8514580c548b481af39601f6ee9ea2bf82630f5c474bc` |
| Installed WSL executor | SHA-256 `de31a7343e31252f450be13f1a92224c735b4c127b5608ff72f37d580bebab7e` |

Installed worker/companion packs and renderer identity passed verification. The
desktop was reopened normally and displayed its OffGrid window; its matching
service compatibility check returned `ready`. One OffGrid container was running,
healthy, published only on loopback, with zero active/queued inference, no switch
and no remaining ready test-owned workspace leases. All five model IDs and the
workspace/renderer identities above were preserved. Recent logs contained no
panic, but retained the known historical-recovery and disabled-auth warnings;
this is not an error-free log or security-certification claim.

Final automatic verification: full `go test -p 1 ./... -count=1` passed after
the inference/stream changes, followed by the full command package and focused
WSL race test after the endpoint-selection correction. Focused inference,
agents, service-client and CLI race tests passed. The two desktop execution test
files passed 9/9 checks. Go vet, 272 maintained-document checks, source-size,
repository hygiene, whitespace and strict change validation passed. Previously
recorded full renderer/platform evidence remains separately scoped; unavailable
native platforms and broader frozen real-model workflows remain unverified.
No commit, push, release, backup deletion or user-job replay occurred.

The final endpoint audit also corrected authenticated legacy task-list aliases:
they retain their short timeout but now use `commandServerURL()` and the selected
key, instead of a hardcoded localhost origin. A separate in-memory regression
checks that origin/credential pair; the full command suite and WSL race check
passed. The subsequent service/WSL build is
`0161a68-dirty-native-endpoint-20261026`, container
`5a842a79adad76feef08be570e374e3fac9c2c1f1a0f9545a3079b947b6f5e7e`,
image `sha256:138d7c516045a62565ec62e5ee92338a456960c0492b9d03f93ccb22d53d5bb4`.
Rollback is `offgrid-rollback-native-20261026-105240`, with consistent
`offgrid-data-before-20261026-105240.tar.gz`. WSL CLI SHA-256 is
`bc3948c5b2ccc24c66843f342b192518c8f5119f361934b8dc7125770ef53535`;
its executor pack is unchanged. Models, workspace and renderer identities remain
unchanged. The matching desktop service binary is built (SHA-256
`e2ed03120df33932c48a740d2aa59920717870da42db3feba78ac1ba8e574191`).

Automatic approval review rejected the normal installer-quit command with
"blocked by policy". The previous compatible desktop remains running; its final
bundled binary replacement is pending manual File/tray Quit. No force-close,
alternative termination method or overwrite of the running app was attempted.

The endpoint-audit build also passed the user's original one-shot listing shape
with the explicitly selected Meta-Llama model: canonical root
`run-9f971fe49c05b8c4958a57dd7f66c912`, disposable folder
`/tmp/offgrid-native-cli-smoke-0olbkvvd`, 24.510 seconds, one actual list action,
one read-only local approval and all three observed names in the assistant's
answer. It followed the foreground run and released its host. This does not
override that model's failed exact-edit qualification.

## 2026-10-26: typed input writing and conversational tool abstention

After the user manually quit through the tray, the endpoint-audit desktop
components were backed up and replaced successfully. The installed bundled
service matched SHA-256
`e2ed03120df33932c48a740d2aa59920717870da42db3feba78ac1ba8e574191`;
the executor matched
`3636f322effc6e1e3d700b8faf24e006e045ea752d7f82ea2d417cd6bc4966a4`.
The app reopened with a compatible service handshake. This resolves the earlier
pending replacement entry; it does not qualify native model workflows.

A read-only frozen Meta-Llama payload probe retained in
`/tmp/offgrid-native-order-probe-qh_y_m5h` generated the expected literal in 3/3
content-first schema presentations and 0/3 path/revision-first presentations of
the same payload. It used only generation, never tool dispatch or file editing.
This is evidence of schema-order sensitivity, not a general ordering solution or
successful native execution. The approved remedy uses typed input references.

`workspace_write_from_input` selects exact bytes from this root's original
request or an applied correction. The existing write grant and worker operation
remain authoritative. New jobs pin feature version 1; old jobs and ordinary
inline writing retain their contracts. Tests cover Unicode/CRLF/empty/large
spans, duplicate and foreign sources (including a duplicate outside the bounded
menu), before-dispatch rejection, original-byte intent binding, receipt tampering
and restart recovery without an executor or replay. A source receipt proves its
selected span, not every semantic requirement of the request.

Automatic commands/results before live qualification:

- `go test -p 1 ./internal/execution ./internal/agents -count=1`: passed.
- `go test -p 1 ./internal/server -count=1`: passed after the fixture correction
  below; initial failures are retained in `build/native-input-server-tests.jsonl`.
- `go test -p 1 ./internal/inference ./internal/serviceclient ./internal/cli
  ./cmd/offgrid ./cmd/offgrid-executor -count=1`: passed after updating the
  token-count test's expected catalog position. Count and generation still use
  identical wire contracts; an initial assertion failure remains recorded.
- WSL race checks for input selection, canonical writes, recovery, paging,
  steering and collaborative parent-save workflows passed. Separate inference
  single-step/token-count race checks passed.
- Web API check and TypeScript/production build passed; the existing large-chunk
  warning remains. Ten focused Playwright tests passed, including source-write
  edit presentation in all nine locales. Desktop unit tests passed 80/80.
- Four maintained Python fixture boundary checks passed. Documentation links
  (272 maintained files), hygiene, source-size and strict OpenSpec checks passed.

The new catalog adds real context overhead. The deterministic researcher/writer
fixture initially exceeded its 8K allocation while reserving the default 2K
response. Its fixed short-response fixture now explicitly reserves 1K, keeping
the same 8K context and all outcome assertions. No production allocation,
overflow protection or user preference was increased or relaxed. This test is
orchestration evidence, not real-model context or quality qualification.

The user's saved greeting `run-692ac3bf3ce555e22c7c5b13422055df` was inspected,
not replayed or modified. Its actual actions were `workspace_info`, directory
listing, then an invalid file read of `integrations/`. Initial governed inference
placed its catalog after the user request. The adapter now places the catalog
before a current user request while retaining the final tool observation in
active exchanges. Strict decisions, schemas, tool availability, authority and
canonical history stay intact. Tests preserve conversational final answers,
current-request position, count/generation agreement and observation attachment.
This implementation observation alone does not establish that the model now
abstains correctly; fresh live tests are required.

## 2026-10-26: installed workspace workflow results and remaining limits

The subsequent live failures were retained and repaired without widening grants:

- A conclusive rejected workspace read now permits a bounded corrective decision.
  The persisted allowance is three corrections per root; the fourth rejection
  interrupts. Unknown effects, writes and outstanding mutation intents never use
  this path. Dependent calls in a rejected batch are not dispatched.
- Meta-Llama reversed the exact-source opening/closing markers. The service
  rejected the proposal before effects. Field descriptions and the existing
  bounded proposal-repair path now explain the distinction; code does not swap
  markers silently. The original failed run is
  `run-79932e503830b9fcb435734ac89e3e4b`.
- A command proposal put `go` in both the program and argument list. It remained
  approval-blocked and was cancelled in the owned fixture, without execution.
  Model-only field guidance now states that arguments exclude the executable;
  the canonical descriptor, permissions and actual arguments remain unchanged.
  Original run: `run-dc84a080d3aa1e5abb4cd345fb5e3c2a`.
- Command output previously presented both base64 bytes and readable text to the
  model; Meta-Llama echoed the encoding. The model projection now presents valid
  UTF-8 once, with its canonical observation reference and digest. Stored worker
  receipts retain original bytes; binary output stays explicitly binary. Exit,
  running, truncation and continuation state are preserved. CLI output presents
  independently observed command state/output before generated commentary.

The installed core matrix passed **24/24 turns**, three four-turn conversations
per explicitly selected model, in disposable WSL folders. Each conversation
listed its own folder, read a fixture, wrote the exact selected user-input bytes
with native independent read-back, and ran the fixture's Go test with observed
exit zero. It retained one approved host session across turns and one canonical
root per turn. Original-file hashes and the complete pre-test folder inventory
were checked. No service-directory substitution or model download was used.

| Model / repeat | Exact-write root | Command root | List/read/write/test seconds |
| --- | --- | --- | --- |
| Meta-Llama 8B / 1 | `run-6c998b8ff7a34d7586fdac4acc15dddb` | `run-83ca3ce8bf0a147930c8db072cb03383` | 25.950 / 4.884 / 7.136 / 11.392 |
| Meta-Llama 8B / 2 | `run-6363ca411439f651a27b269a98081a49` | `run-0775e4d011e28aae8f1aa1fb82900359` | 8.103 / 4.863 / 7.143 / 10.121 |
| Meta-Llama 8B / 3 | `run-d8056c2cf7b9367119205e7e4a5d17e3` | `run-41461a6a2ac99ecede9f366bdade9665` | 5.369 / 5.646 / 7.615 / 9.164 |
| Qwen 2.5 7B / 1 | `run-004a07ea12b3842d20eda79145b3216e` | `run-e16247024cd09c637d464a7c3893dcd0` | 85.638 / 4.617 / 8.360 / 7.930 |
| Qwen 2.5 7B / 2 | `run-6e36df43b80d6b570366037ba3da8d82` | `run-a0c6cb01e43482459ea9136762a3d16b` | 59.359 / 4.619 / 8.364 / 8.114 |
| Qwen 2.5 7B / 3 | `run-3b5fea5edd629a947e73dd44f5199c7f` | `run-123ec2257448d0f964c20316e19f53fc` | 60.874 / 6.385 / 11.635 / 9.370 |

Full installed model identities are
`Meta-Llama-3.1-8B-Instruct-Q4_K_L-f1e3673a9589` and
`Qwen2.5-7B-Instruct-Q4_K_M`. Private detailed results and all 24 root identities
are retained in `/tmp/offgrid-native-purpose-matrix-e0pm1pld/results.json`.
The maintained fixture command is
`python3 scripts/test/test-native-agent-conversation.py --cli /home/phil/go/bin/offgrid --model MODEL --endpoint http://127.0.0.1:11611 --allow-disposable-workspace --require-input-source`.
It was run three times for each model; the ignored matrix driver does not retry
failed conversations. These runs used the existing fixture's explicit 8K task
context on the RTX 5060 Laptop GPU. They do not qualify all profile contexts.

Earlier fixture failures remain preserved. For the source-write case, the oracle
was corrected to recognize the worker's close/reopen/exact-byte read-back receipt
plus a separate Python file read; it does not require a redundant model-requested
read before accepting that mechanical proof. The next conversation turn still
independently reads the file. The command-answer rubric was also corrected to
accept the truthful phrase "exited with code 0"; actual terminal exit/output
checks remain mandatory. A Unicode golden-digest regression prevents a previous
fixture encoding mistake. Five Python fixture boundary tests pass.

An earlier extended matrix retained in
`/tmp/offgrid-native-purpose-matrix-vls2o30a` added greeting and conceptual
explanation turns. Qwen passed all 18 turns, including zero tool calls for those
two prompts. Meta-Llama answered greetings without tools in 3/3 runs but still
performed an unnecessary workspace-info read for the path explanation. Its
strict abstention check remains failed. The final core matrix did not repeat or
claim that stricter abstention gate.

The Qwen listing latency is an unresolved quality/performance limitation. Runtime
logs show some decisions consuming the full 2,048-token response allowance before
bounded repair. Only one Qwen load occurred during these three conversations;
the repeated delay is not explained by repeated cold loading. No truncated
decision was accepted to improve the timing, and the raw rejected decision was
not captured here, so its exact generation failure is not established. Successful
file equality and command completion are not a claim that this interaction is
consistently fast.

Final automatic checks on the readable-process build:

- `go test -p 2 ./... -count=1`: passed, including the full native computer,
  inference, agents, server, CLI and executor packages. Optional real-model/native
  tests retain their explicit opt-in conditions.
- WSL `go test -race` covering the affected execution, agents, server and CLI
  source-write/read-repair/process-observation tests passed. Separate inference
  contract/count race checks passed earlier in this change.
- API/type/build checks and ten focused Playwright tests passed (nine locales
  for source-write activity plus command-state presentation). Desktop tests were
  repeated after installation and passed 80/80. Detailed logs are retained under
  ignored `build/native-input-web.log`, `build/native-readable-race.log`,
  `build/native-readable-final-all-go.log` and
  `build/native-readable-final-desktop.log`.

The user-authorized installed service and WSL CLI are
`0161a68-dirty-readable-process-20261026`, container
`630a40a0a709e6c4090db6726811a1889672676cda90996575095e42fd2309e9`,
image `sha256:a928815be98e10bb215d58a3d7dd579f18802df97abf481553887fc322963915`.
Rollback container `offgrid-rollback-native-20261026-140920` and consistent data
archive `offgrid-data-before-20261026-140920.tar.gz` remain retained. WSL CLI hash
is `adbccd68a1ba92b9a177f4fc771dfe082ee7110a6660dfb244a21ee73dd85e7d`.
After the user quit through the tray, matching desktop components were backed up
and replaced. Installed service hash is
`d649fc330ac0189fef5fd9e241ddf7be0c3936b5540efea5ee10e7433c8bf05f`;
executor hash is
`5580954b0d17fb0954c6b6f8e59cde8b5225faefa07b1326b150add1e6327e93`.
Desktop/service renderer identity is
`e97215e2f797fb1b371d44fa33dbd63a425b25cadf9a9e98db960e7f1eceaa0f`.
The desktop reopened and responded. Model files, profile selections, workspace
identity and previous jobs were preserved. No commit or publication occurred.

### Separate desktop-control investigation

The reported screenshot belongs to `run-17333b0856e65de520326fa6420a0bd9`,
created October 18, 2026 at 06:03:31 UTC (14:03:31 Asia/Shanghai), pinned to
bumblebee revision 7. Its six recorded steps end in a verified application launch
and then `computer_documents` failing with `computer_document_changed` at
06:27:14 UTC. No text insertion/save action or exact read-back is recorded.
The error can represent several provider consistency failures; the saved result
does not identify which one. It is not proof that the user changed a document.
The task is preserved as interrupted with expired computer access. After the
desktop replacement, the service reported no current computer sessions.

Workspace file/command results above do not qualify that native Notepad GUI
workflow. A fresh user-authorized diagnostic run is recorded separately as
`run-b0987f819280e9ee6f99de6084c7e9a7`; the old job was not resumed or replayed.
The fresh run reached a native desktop-access request. Automatic approval review
blocked opening its desktop deep link ("blocked by policy"); no alternate launch
or consent bypass was attempted. Fresh native outcome evidence remains pending.
Broader task 6.2, real desktop workflows, unavailable macOS/Wayland hardware,
strict Meta-Llama abstention and the latency limitations remain open.

## October 26: conversation-first access refinement (source candidate)

This section supersedes the earlier abstention result only for the cases below.
It does not supersede the native Notepad limitation or qualify general autonomy.
No container, installed CLI or desktop replacement, commit or publication occurred
for this refinement. Existing installed build hashes above remain historical.

The defects crossed client and harness boundaries: the CLI acquired folder access
before knowing whether it was needed; attached tools dominated conversational
responses; repair instructions could replace the user's objective. New clients
negotiate `execution-access-request-v1`, submit the conversation first, and attach
an approved folder to the same paused root only when requested. Exact launch-folder
identity is rechecked. Permission upgrades retain existing ceilings and receipt
history. JSON/detached callers do not receive surprise terminal prompts.

An ordinary approval remains session-only. Optional `remember`/the native Remember
checkbox stores exact folder/host/service/account/permission trust outside the
workspace, with inspection and revoke controls; each session still needs a fresh
authenticated grant. Desktop storage uses OS-protected encryption, rejects
plaintext fallback and degrades to session-only if unavailable. Desktop Full
access is separate. Foreground progress retains tools, errors and cancellation
while removing repeated prompt-preparation messages and duplicate answer previews.

Additional live failures led to bounded fixes: invalid HTTP schemes reject before
effects; file precondition recovery requires the matching persisted rejected host
receipt; exact-input repair retains source selectors and trailing newlines; known
running processes receive bounded observation opportunities before completion.
Unknown/partial effects cannot enter these repair paths. Historical references
only verify receipts and cannot authorize new operations after an upgrade.

### Verification method and results

Source runner tests used isolated task stores, disposable directories and the
actual installed Qwen2.5-7B Q4_K_M and Meta-Llama-3.1-8B Q4_K_L runtimes. They used
the native single-step adapter, not external Hermes/OpenClaw. A temporary loopback
transport forwarded bytes unchanged to the specifically selected already-loaded
runtime. No model downloads or persisted selection changes occurred. This tests
the new source runner with live inference; it is not an installed-client test.

Each model received five conversations with and without an attached workspace:
greeting, identity, capability explanation, story and general explanation.
Both models passed all ten cases with one decision and **zero tool calls**.
Answers were inspected as well as checked for the captured failure strings.
Qwen took 0.73–6.14 seconds per conversation; Meta-Llama took 3.62–30.57 seconds.
These are observed wall times, not controlled throughput claims or the UI's
250 ms feedback measurement. Meta-Llama remains noticeably slower.

Work follow-ups covered listing, Unicode reading, exact replacement including the
final newline, and `go test .` with actual process-exit evidence. Independent disk
read-back checks the requested bytes and preservation of a neighboring file.
Qwen passed all eight work cases. Unattached workflows requested access once;
already-attached workflows requested none. The fixture explicitly supplied a
read/write/process grant; it did not automate native human consent. Qwen work
times were 3.78–13.92 seconds. Final-answer prose is not itself exact-output proof:
one Qwen answer mistranscribed a character in command output while its raw receipt
and independently checked exit status remained correct.

The final Qwen rerun after the source-preserving repair change also passed all
eight work cases (`build/native-demand-qwen-repair-final.log`), with 3.67–40.52
second wall times. Its slowest command used 13 decisions, including redundant
read-only zero-wait polls of the same process; it started the command once and
verified exit code 0. Polling efficiency remains a known improvement, not a
second command dispatch or evidence of unknown-effect replay.

Meta-Llama initially failed both edits: after a proven precondition rejection it
switched to handwritten content, dropped the newline and repeated the write.
The run remained interrupted rather than falsely completing. After explicit
source-preserving repair guidance, all eight work cases passed, including both
edits' independent equality checks and both commands' actual exit code 0. Work
times were 9.00–164.10 seconds; the slowest command turn is an unresolved latency
observation, not concealed by the passing result. Concurrent build/race checks
mean these timings cannot establish a controlled model-performance comparison.
The earlier failed logs remain evidence; they were not relabeled as successes.

Local raw logs (ignored, disposable fixture data) include
`build/native-demand-qwen-qualified-fixture.log` (the filename does not mean
release qualification), `build/native-demand-meta-direct-final.log`, and
`build/native-demand-meta-repair.log`. Earlier bridge attempts failed because the
test transport omitted the model, reordered JSON discriminator fields or selected
an embedding runtime; those are test-infrastructure failures, not model
incompatibility evidence. A fixture initially omitted trusted-process policy and
used ambiguous source markers; both fixture errors were corrected explicitly.

Executed checks include:

- `go test -p 2 ./...`; targeted actor/grant/recovery/interactive/identity tests.
- WSL `go test -race -p 2 ./internal/agents ./internal/execution ./internal/cli ./internal/server ./internal/inference ./cmd/offgrid`, followed by agents/server race checks after final recovery changes.
- `npm test` in `apps/desktop`: 85/85, including protected-store doubles and trust revocation.
- `npx playwright test tests/agent-chat.spec.ts` in `apps/web`: 59/59, including nine locales, RTL, narrow layout, canonical attachment and duplicate prevention.
- Web `npm run api:check`, `npx tsc --noEmit`, `npm run build`: passed; existing large-bundle warning remains.
- `node scripts/maintenance/check-source-size.mjs`, `node scripts/maintenance/check-repository-hygiene.mjs`, documentation checks and strict OpenSpec validation.
- Live opt-in `go test ./internal/server -run '^TestNativeConversation(Work)?InstalledModel$' -count=1 -v`, with `OFFGRID_TEST_CONTEXT_MODEL`, `OFFGRID_TEST_COMPLETION_URL`, `OFFGRID_TEST_DISPOSABLE_EXECUTION=1` and the explicit direct-runtime fixture flag.

Final full-Go and changed-package race runs exited 0; logs are
`build/native-demand-go-release-check.log` and
`build/native-demand-race-release-check.log`. Their names do not authorize release.

These are one-pass case comparisons plus targeted reruns, not three-repeat
platform qualification. Real OS keychain behavior, native consent UI, installed
app usability, macOS/Wayland, general model accuracy and broader task 6.2 remain
unverified. No claim that arbitrary prompts or every workflow now succeed follows
from these checks.

### October 26 container-only deployment after explicit authorization

Replaced the idle container with `offgrid-llm:local-conversation-access-20261026`
(image `sha256:3d6528df12c301eaf3a4a12573c45145653c17847d2c8fdb080e48f274731ae6`),
application revision `0161a68d-dirty-conversation-access-20261026`, version v0.4.20.
The application/UI were rebuilt; the existing GPU runtime was retained through
`docker/Dockerfile.local-runtime`. The build retained its existing bundle-size
warning. No new native runtime qualification is claimed.

Before replacement, inference and jobs were idle and no computer sessions were
present. The stopped data volume and private configuration were backed up in WSL
at `/home/phil/.offgrid/deployment-backups/conversation-access-20261026-175750`.
Rollback container: `offgrid-rollback-conversation-20261026-175825` (stopped,
automatic restart disabled). One OffGrid service is running.

Verified healthy status, HTTP 200 for `/ui/`, new
`execution-access-request-v1`, all five model identities and all 187 previous job
identities. Workspace remains `d8b307945e8e3a9ee6025092525318a4`; renderer identity
is `f054039116190db873d989b59ec8e59bc5d6923ec9e4b448b54d4115c31c8c49`.
Mounts, GPU requests, network/port bindings, resource/security settings and
unrelated environment configuration match the prior container.

A fresh Qwen deployment smoke job,
`run-2eced983ce96ccd9c0d410720c374145`, answered `hello`, completed with zero tool
calls and acquired no workspace authority. Startup logs contained no error,
panic or fatal line. The existing authentication warning refers to listening on
`0.0.0.0` inside the container; Docker publishing remains
`127.0.0.1:11611`, unchanged.

The separately installed WSL CLI and desktop application were not replaced.
Browser testing can use the updated service UI after refresh. The new CLI
conversation-first prompt behavior and desktop remembered-trust controls require
matching client builds; container replacement alone does not install them.
No commit, push or release occurred.

### October 26 CLI presentation and matching WSL installation

The subsequently reported story run `run-ec5725d390c707182729a84893916456`
used the legacy installed CLI: no execution-request version, an upfront workspace
attachment and one `workspace_write`. Container deployment had not replaced that
client. Its created story file was not changed by this investigation.

Refined the existing concise-activity presentation: one OffGrid/model header for
foreground runs, readable action labels, a separated answer and one inspection
footer. Suppressed redundant request/run/preparation/status scaffolding in that
path while preserving request IDs in submission errors, detached/JSON contracts,
exact approval arguments and uncertain-effect recovery. Verified file-write
receipts present the observed destination on a separate line, not a path inferred
from model prose. The renderer preserves answer text rather than reformatting
source/code content or treating a model claim as verification.

Checks passed: full `go test -p 2 ./...`, WSL race tests for `internal/cli` and
`cmd/offgrid`, Linux binary/executor build, documentation/source-size/OpenSpec
checks and `git diff --check`. Added fixtures for separated outcomes, non-success
states, control-character sanitization and rejected/unverified/foreign write
receipts. The initial Linux build invocation had a Windows shell-quoting error
and then a missing login PATH; explicit Python subprocess arguments under the
WSL login environment produced the successful build.

An 80-column WSL pseudo-terminal ran the exact prompt `tell me a story` with
Meta-Llama in a disposable directory using the candidate binary. Run
`run-6319fe56d9dac50e4588113e74cde395` completed with zero tools, no approval prompt
and no files created. Its transcript contained one header, one answer and one
inspection footer, no preparation chatter or ANSI escapes in NO_COLOR mode.
Observed answer time was 42.0 seconds; this presentation pass does not resolve
Meta-Llama inference latency. This was a terminal transcript check, not a human
usability study or broad cross-terminal visual qualification.

After checks passed and no installed CLI/executor process was active, backed up
and replaced `/home/phil/go/bin/offgrid` and its adjacent executor pack.
CLI SHA-256: `be942d9b479639c333831eb465f39e446c6120ca950bffe0165ef98f9574d799`.
Executor SHA-256: `b75d26ecd611d39a6340d4748f37e3fca7efff09d57a4a4387d4f65cf2ac99c5`.
Rollback: `/home/phil/.offgrid/deployment-backups/cli-presentation-20261026-181552`.
The version remains 0.4.20; this is a local source build, not a new publication.
The container, desktop installation, models and user configuration were unchanged
during this CLI replacement. Broader task 6.2 remains open.

### October 26 live terminal feedback and organized help

The prior presentation build did not animate or append previews. A candidate
terminal check then exposed a second problem: the governed single-step adapter
buffered the complete JSON envelope before publishing normalized content. Run
`run-5abfd41448a00146008c09d7569522cb` completed, but failed the incremental-output
assertion. The failed transcript is retained locally in ignored build evidence.

The CLI now animates a single elapsed-time line on a terminal, clearing it before
other output and at termination. It appends provisional response snapshots without
duplicating the final answer. Failure, replacement and disconnection explicitly
end a draft. Piped/JSON output retains its existing result contract. Help defaults
to common workflows; `agent help all` groups the full reference and `agent
workspace help` works without a service connection.

For governed llama.cpp decisions, an internal context observer extracts only the
first literal `message` string as provisional text. It does not publish private
JSON, arguments or reasoning, alter model instructions, or authorize execution.
The original complete envelope still passes strict validation before any final
response or tool decision. Public inference streaming retains its validation
boundary. Tests cover every byte split of Unicode/escapes, private branches,
callback cancellation and invalid/truncated envelopes after a visible draft.

Passed commands:

- `go test ./internal/inference ./internal/server ./cmd/offgrid ./internal/cli`
- `go test ./...`
- WSL `go test -race ./internal/inference ./internal/agents ./internal/server ./internal/cli ./cmd/offgrid`
- Linux `CGO_ENABLED=0 go build -trimpath` and `go run ./tools/executor-pack`
- `node scripts/maintenance/check-source-size.mjs`
- `node scripts/maintenance/check-docs.mjs`
- `openspec validate native-agent-workspace-execution --strict --no-interactive`
- `git diff --check` (existing line-ending warnings, no whitespace errors)

Live disposable-directory PTY checks used the installed service and identical
candidate/installed CLI bytes. Both verified spinner redraws, response fragments
before completion, exact preservation of the final answer, one inspection footer,
zero tools, no folder approval and no created files:

| Model / width | Run | First draft | Total observed | Response updates |
| --- | --- | --- | --- | --- |
| Meta-Llama / 80 columns | `run-f0ac4136361d5979034e3ed459c491fd` | 85.0 s | 113.5 s | 93 |
| Qwen / 40 columns | `run-24119e96273b09d7080fcd7f882387e5` | 46.8 s | 50.7 s | 13 |

These include cold model loading; animation does not establish acceptable latency.
The initial batch safely stopped before its second submission when inference was
busy. Qwen was tested separately after the service became idle. The user's run
`run-13b127ce9bdc5a6bd868cd4d7928177e` still used the earlier installed renderer
while candidate verification was underway; it was not replayed or modified.

Replaced the idle service with `offgrid-llm:local-cli-live-20261026`, image
`sha256:deaacdad08b131e6c5e7e4cd0bd821565c26630a61948497d24871c6764dc020`,
preserving its prior CUDA runtime, UI, models, workspace, bindings and saved jobs.
Rollback container: `offgrid-rollback-cli-live-20261026-190501`.
Private data/configuration backup:
`/home/phil/.offgrid/deployment-backups/cli-live-20261026-190437`.

Installed WSL `/home/phil/go/bin/offgrid` SHA-256:
`67cf543d3d59553117212052acafaaded9c812325d5f393afcc172b3946dd0f4`.
Its executor pack remains
`b75d26ecd611d39a6340d4748f37e3fca7efff09d57a4a4387d4f65cf2ac99c5`.
CLI rollback: `/home/phil/.offgrid/deployment-backups/cli-live-20261026-190955`.
Normal WSL command resolution and installed/candidate SHA equality were checked.
Version remains 0.4.20; no publication, Git commit or desktop replacement occurred.
Native terminal appearance, screen-reader usability, other operating systems and
the broad workflow qualification gate remain unverified by these PTY checks.

## October 26: allocation, conversation lifetime and mixed-workflow follow-up

Release qualification remains open. Automated passes below do not erase the
live failures or establish that every installed model completes every workflow.

Foreground native `agent run` now negotiates `agent-response-profiles-v1` and
uses the same interactive response allocation as Chat. Explicit
`--response-profile extended` remains available. Omitted API fields, older
services, JSON/detached callers, global context configuration and external-agent
allocations retain their previous behavior. The selected allocation is pinned
to the canonical job and included in idempotency checks.

On this machine, Meta-Llama's 65,536-token allocation produced runtime loads of
60.9/92.1 seconds and a recorded 281-token generation in 28,918 ms. With the
interactive 8,192-token allocation, the runtime recorded a 9.2-second load,
2,585 prompt tokens in 1,566.78 ms and 303 generated tokens in 7,851.36 ms.
The fresh installed story run `run-016773c0b213a27786c6ab9106758942` produced its
first draft at 23.46 seconds and finished at 31.21 seconds, with 27 progressive
updates and no tools/files. The earlier story fixture took 85.0/113.5 seconds.
These are observed runs, not a controlled cache/concurrency benchmark or a
latency guarantee. Global maximum context remained 65,536.

Mixed conversational tests exposed additional defects:

- Attempt 1: the fixture expected the retired `Run:` heading. The greeting
  itself completed; the parser now also accepts the canonical Inspect footer.
- Attempt 2: the fixture did not recognize the new `y/remember/N` consent
  choices. It timed out and stopped its own invocation without resubmission.
- Attempt 3: a conversation retained a lease whose context was cancelled at
  the end of its first turn. The CLI now gives conversation-owned leases the
  conversation lifetime, retaining per-turn stream watchers and explicit exit
  cleanup. A regression test verifies reuse and outer-context cancellation.
- Attempt 4: Meta-Llama treated access attachment as the listing result.
  Response context now distinguishes setup from evidence of completed work.
- Attempt 5: greeting, explanation, listing and exact Unicode reading passed,
  but writing failed. Run `run-389b3c20f5c25c4d009dd3628477d0d0` attempted to read
  nonexistent `result.txt`, then returned repair guidance; no file was created.
  This is a failed outcome, despite the job's completed status. Private evidence:
  `/tmp/offgrid-native-conversation-d_njfstm`, matrix log
  `/tmp/offgrid-readiness-matrix-xno9ib04/Meta-1.log`.

The next candidate exposes the actual read/write/process grant in the model's
workspace context and explains permission upgrades only when the request-access
tool is actually offered. It does not grant authority or infer authorization
from a prompt. The mixed fixture permits a session replacement only for a
recorded strict permission upgrade on the same host/root/backend/directory;
it still rejects foreign scope, repeated same-scope approval, unexpected files,
incorrect bytes, missing independent read-back and duplicate process starts.

Checks passed before this last refinement: full `go test ./...`; race tests
for agents, CLI, server, execution and cmd/offgrid; web `api:generate`, `check`
and `api:check`; source-size, documentation, strict OpenSpec and diff checks.
`npm run typecheck` was initially attempted but is not a defined script;
`npm run check` is the applicable passing check. Initial CLI mocks failed on the
new identity request and were corrected to exercise older-server compatibility.
The latest focused grant-context tests pass, and the Python fixture boundary
suite passes 6 tests. Final candidate/live results are recorded below when run.

No version bump, commit, push, publication or desktop replacement was performed
for this follow-up. Local candidate service/CLI deployments retain rollback
backups; native platform, human usability and the broad frozen suite remain
separate qualification gates.

### Permission-context candidate: failed live gates

Candidate image `sha256:5c0cf7d2a7a00c16e027e8a835e7e5d212dd21ca1ab79561517c03a7cd28abfa`
and WSL CLI `8de442c12f18fa196090a869d402af3100e38dbe09957e8a4f8eaba013d88a49`
were installed with the same executor, five models, workspace and UI. Rollback
container: `offgrid-rollback-native-readiness-final-20261026-205853`; data backup:
`/home/phil/.offgrid/deployment-backups/native-readiness-final-20261026-205745`;
CLI backup: `.../native-readiness-final-20261026-205915`.
The first replacement attempt failed while resolving the prior Docker image for
the backup helper and restarted the unchanged old container. The second used the
verified retained base image to take the backup, then completed replacement.

`go test ./internal/agents ./internal/cli ./internal/server`, agents race tests,
six Python boundary tests, documentation/source-size checks, strict OpenSpec and
`git diff --check` passed for this candidate. Live results did not:

- Meta-Llama mixed workflow: greetings, explanation, list and read passed;
  edit failed again (`run-7286b087e6945aeb6b4a05fed72b6c79`, private evidence
  `/tmp/offgrid-native-conversation-quo1gsl0`). It changed the session directory
  to `.` and read an invented file instead of requesting edit access. No target
  file was created. No automatic replay or model substitution was performed.
- Qwen mixed workflow: edit returned the prior source file's content instead
  of creating `result.txt`; evidence `/tmp/offgrid-native-conversation-_udxnxt5`.
  Neither this nor Meta-Llama reached command verification in the mixed suite.
- Qwen Stop fixture: requested computer access for application `go` and exited
  at waiting-for-input (`run-5fd5e5648999b257062aef9243d3be15`). The fixture never
  started its process, so this is not a Stop pass. The test-owned pending job
  was explicitly cancelled; no desktop consent was approved.
- Meta-Llama Stop fixture: selected `http_get` with an invented example-domain
  Go-test URL (`run-b60b7c4bf247f63ca3ee96972f5b4d38`). It did not start the
  requested process. Stop behavior remains unqualified by this run.

Inspection of the failed task snapshot confirmed requestable workspace access
was pinned and no profile execution ceiling removed it. Thus the problem is
not simply an unavailable tool. These runs show incorrect tool/intent selection
and must remain release blockers, not be counted as completed work.

A further protocol correction now represents prior assistant text as the literal
`message` branch of the same single-step envelope used for tool decisions.
Previously only tool calls were projected consistently, leaving ordinary answers
as conflicting bare-text examples. JSON-looking prose is escaped as text, never
parsed into executable calls; canonical history and user instructions remain
unchanged. Automated round-trip, malformed/mixed-call and Unicode tests pass;
fresh live qualification is required before attributing any improvement to it.

### Final installed comparison: release still blocked

The history-envelope candidate passed `go test ./internal/inference
./internal/agents ./internal/server` and `go test -race ./internal/inference`.
Final documentation, source-size and strict OpenSpec checks passed. The live
comparison `/tmp/offgrid-envelope-comparison-epp_0243/results.json` did not:

| Model | Observed outcome | Qualification |
| --- | --- | --- |
| Meta-Llama 8B | First four turns passed. At the edit turn it explained that edit access was needed but never requested it or wrote the file. Run `run-ef47c060d2b5c2f1cd809a7847991285`; evidence `/tmp/offgrid-native-conversation-yem65ud9`. | Failed; command phase not reached |
| Qwen 2.5 7B | First four turns passed. At the edit turn it claimed the requested file had been created, but independent inspection found no such file. Run `run-7a961086942f32f221fe67e4140d1cab`; evidence `/tmp/offgrid-native-conversation-jlfvq5ey`. | Failed, including a false-completion claim; command phase not reached |

The comparison took 121.94 seconds for Meta-Llama and 139.56 seconds for Qwen,
including cold loading and all attempted turns, not just generation. Each case
used a fresh disposable workspace; failed tasks were not replayed. Repetitions
cannot be counted as a three-pass qualification while these gates fail. Tasks
6.2 and 8.8 remain unchecked. Prior checked implementation tasks do not supersede
this newer failed evidence. Outcome verification currently protects dispatched
typed writes but does not prove that a natural-language request was fulfilled
when the model avoids the write entirely; the Qwen result demonstrates that gap.

Installed container image:
`sha256:8fb995a7545527996f47cf72d02a1c89a6a5536aa492b137fb50996288d7fa11`.
Container binary, WSL CLI and source-built candidate all have SHA-256
`f70d87a9942768945ed040394573cee5998af8e418c3461798d5971f63565c57`.
Executor pack SHA-256:
`0eae79d6c3a885a5352aefe06e139e54a19b182d37af646a6c6f4c0c51d1b44e`.
Rollback container: `offgrid-rollback-native-readiness-final-20261026-212557`.
Private service backup:
`/home/phil/.offgrid/deployment-backups/native-readiness-final-20261026-212535`;
CLI backup: `.../native-readiness-final-20261026-212640`.
The service retains five model identities, its original workspace identity and
UI build. Version remains 0.4.20. This is an installed local test candidate,
not a newly published or qualified release; the desktop app was not replaced.

## Interactive session UX and process-observation follow-up — 2026-10-26

Source changes, not an installed release qualification:

- Persistent agent chat now emits one compact welcome with the selected profile
  and model. `/session` and the exit receipt retain exact recovery identities;
  normal successful turns no longer repeat Turn/Run/Inspect diagnostics.
- One registry drives `/help`, the keyboard slash picker and pinned configuration
  views (`/skills`, `/agents`, `/tools`). These views do not change profiles,
  grants, resources or jobs. Existing status/lifecycle/recovery operations remain
  explicit and service-owned.
- The terminal editor uses the same buffered reader as consent, restores terminal
  mode before dispatch and supports Unicode, bounded multiline paste, cursor
  editing and per-invocation history. Plain/redirected/small-terminal fallback
  remains line-oriented. The session writer preserves the actual terminal for
  spinner detection and incremental draft rendering.
- Input remains serialized with foreground turns. Concurrent steering input
  during generation, real Windows/macOS terminal behavior, screen-reader behavior
  and human visual/usability review remain unverified/unimplemented as applicable;
  no claim of full Claude/Hermes UX parity is made.

Validation executed:

- `go test ./internal/cli ./cmd/offgrid`: passed (0.336s / 28.786s), log
  `build/agent-session-ux-final-tests.log`.
- `go test -race ./internal/cli ./cmd/offgrid`: passed (3.130s / 57.897s), log
  `build/agent-session-ux-race.log`.
- `go test -c -o build/agent-session-ux.test ./cmd/offgrid` then
  `python3 scripts/test/test-agent-session-pty.py build/agent-session-ux.test`:
  six isolated Linux PTY scenarios passed. Covered keyboard selection, Unicode
  multiline paste, unknown-command dismissal, narrow and plain fallback, and
  a complete fake-service conversation. The conversation fixture verified the
  spinner remained animated through the session writer, draft bytes appeared
  before completion, the final answer was not repeated, and exactly one turn
  was submitted. No live service, model or real grant was used.
- Linux build and `GOOS=windows GOARCH=amd64 go build` passed. Compilation does
  not establish Windows terminal qualification.
- Source-size ratchet, maintained-doc links, strict OpenSpec validation and
  `git diff --check` passed (existing CRLF normalization warnings remain).
- The first two CLI test runs failed an old assertion requiring the startup
  budget line. The assertion now checks the intended one-header/model behavior;
  the budget is tested in the explicit session-details view. No execution or
  authority assertion was removed to obtain the pass.

The backend investigation also produced narrower, mixed evidence:

- Bounded process completion repair now persists an observation of an already
  owned process handle through the ordinary dispatch path. It cannot synthesize
  another process start or observe a foreign session. Agents/server Go tests and
  race tests passed (`native-completion-observation-tests2.log` and
  `native-completion-observation-race.log`). Additional dynamic revocation and
  cancellation coverage remains tracked by task 8.9.
- Qwen's opt-in native-format attached-workspace fixture passed list/read/edit/
  command work with independent file bytes and process exit verification, using
  one start and one harness-scheduled wait (`native-work-observation-qwen.log`).
  This was source runner code using installed inference, not newly deployed
  production adapter behavior or a repeated qualification suite.
- Meta-Llama's same experimental comparison failed. Without preattached access,
  list/read passed but edit returned a fabricated replacement/read-back narrative
  without changing the file; independent file comparison caught the failure.
  With access already attached, the list response was interrupted and no tool
  call was released (`native-work-observation-meta.log`). Production adapter
  selection remains unchanged. The release and mixed-workflow gates stay open.

No installed container, WSL CLI, desktop app, model configuration or version was
changed by this UX pass. No commit, push or release was performed.

Follow-up boundary checks: `go test ./internal/agents -run TestPendingProcess
-count=1` and the matching `-race` invocation passed (0.139s / 1.623s).
They persist the generated process observation first, then revoke the capability,
revoke host authorization or cancel before normal dispatch. Each case dispatches
zero tools and cannot report completion. Task 8.9's implementation/comparison
checks are complete; the observed Meta-Llama failure remains a separate open
workflow/release gate. Final isolated PTY rerun also passed all six cases after
using ASCII-only truncation/newline markers in the input viewport. Repository
hygiene check passed. Installed binaries remain unchanged.

### WSL CLI installation — 2026-10-26 22:59 local

At the user's explicit installation request, rebuilt the current CLI and executor
pack, then installed `/home/phil/go/bin/offgrid` by atomic rename. The complete
executor bundle matched the installed bundle byte-for-byte and was not replaced;
the open interactive chat was not terminated and continues on its original inode.
New invocations use CLI SHA-256
`19ce6160544335c89ce1e7d26fd348e9c67fea09e7ec2062ad83662d798ded65`.
The executor remains SHA-256
`0eae79d6c3a885a5352aefe06e139e54a19b182d37af646a6c6f4c0c51d1b44e`.
Backup: `/home/phil/.offgrid/deployment-backups/interactive-session-ux-20261026-225942`.

Verified executable lookup, installed version invocation and agent help. The
build retains version 0.4.20 with a dirty interactive-session revision; it is not
a new published release. Container, desktop, model configuration and saved work
were unchanged. Exit and restart the existing chat to activate the new client.
The initial direct WSL Python build invocation lacked Go on PATH; rerunning via
the login-shell toolchain succeeded before any installation. A diagnostic heredoc
also failed shell quoting; installation used the checked Python file instead.

### Matching container replacement — 2026-10-26 23:06 local

After the user's explicit container-update request, built
`offgrid-llm:local-interactive-session-ux-20261026` from the digest-pinned existing
image, retaining its CUDA runtime and renderer. The first admission check found
a connected execution host and changed nothing. After the user exited chat,
rechecked idle inference, jobs, desktop sessions and local execution hosts,
backed up the stopped data volume, and replaced the container.

- Active container: `5f80449f80d65b627ff7b637ee8527ff4b68ef0a896b142e542be0baa9912ac6`.
- Image digest: `sha256:925d939eedca8617880bdea6e15b06e6a083ebc0bfb3007e45523ecf4e2c0df2`.
- Container and installed WSL CLI both have binary SHA-256
  `19ce6160544335c89ce1e7d26fd348e9c67fea09e7ec2062ad83662d798ded65`.
- Workspace identity, all five installed model identities and preexisting job
  identities were preserved. Mounts, port binding, device requests, security
  options, dropped capabilities, privileged/network settings and restart policy
  match the previous container.
- Docker and service health checks passed. Reviewed startup logs contained zero
  ERROR/FATAL/panic matches across 96 lines; this is a startup check, not proof
  of every workflow. One post-install five-second health read timed out; a fresh
  read of the same running container succeeded without restart or redeployment.
- Rollback container: `offgrid-rollback-interactive-session-ux-20261026-230624`.
- Private cold backup and verification records:
  `/home/phil/.offgrid/deployment-backups/interactive-session-ux-20261026-230407`.

The desktop application was not replaced in this operation. The recorded
Meta-Llama workflow failures and release qualification gates remain open.

### User-requested skill smoke test — 2026-10-26 23:13 local

Imported the text-only `offgrid-checklist-test` skill through the installed CLI
and attached it to bumblebee as profile revision 8. Verified exact imported
SKILL.md bytes through the service read-back and confirmed that all profile
settings except the skill attachment were unchanged. The effective-profile
endpoint reports ready with no issues. Existing revision-7 conversations/jobs
remain pinned; new test sessions must select revision 8.

Skill reference: `skill-5c6cdb63abddbb85b893a3b842b64a3b`, revision 1, digest
`b36d2c2b1e9a82522a6e0f5526aeb2cdb84c827ae5e7521377798e0263d72fdb`.
The skill's body requests a recognizable checklist heading and three items for
an explicitly requested topic; it grants no tools or file/process authority.
Import/attachment is verified. Actual model loading/use remains for the user's
requested test, and is not claimed merely from profile readiness.

### Schema transport and reported skill-loop investigation — 2026-10-26

A new regression reproduced public API forwarding reordering schema properties:
`arguments` preceded the tool `name` after a map round trip. The constrained
runtime uses that order for generation. Retain validated wire order while the
decoded schema is unchanged; preserve exact numeric constants and reject
duplicate keys/malformed Unicode. Nested caller edits replace the retained wire
value. Historical calls now use the same name-first envelope.

The regression failed before the patch (`build/schema-order-before.log`). API,
inference and server tests then passed; API/inference race tests passed. The full
`go test -p 2 ./...` exited 0 (`build/schema-order-full-go.log`). API generation,
documentation, source-size and strict OpenSpec checks passed. An initial docs
command used the wrong working directory; the root-directory rerun passed.
This fixes transport, not model qualification; production adapter selection is
unchanged. This patch was not yet installed when the user reported the skill loop.

The user's checklist run returned the correct skill body but alternated 15
successful `skill_load` calls with 15 pending `task_plan` updates, exhausted its
30-decision budget and compacted context 27 times. No successful checklist is
claimed. The same-task compaction path did not retain loaded skill instructions.
The candidate retains the newest completed load of each pinned skill file,
allows older duplicate loads to be archived, and explains that loaded guidance
should now be used without acquiring authority. Unit checks cover compaction,
restart, supporting files, duplicate loads and foreign revisions. Live-model
verification is still in progress; the original failed run remains untouched.

Four isolated Windows-1252 em-dash bytes in this evidence file were converted to
their UTF-8 representation so strict text tools can read it. A byte-for-byte
pre-repair copy is retained in `build/native-agent-evidence-before-encoding-repair.bin`.

### Skill context and native decision comparison - 2026-10-27

The reported revision-8 run is preserved. Retention, immutable-file discovery,
structured-result projection and a consecutive-plan guard have unit/race coverage.
Skill creation remains discoverable but is separated from applying a loaded
skill. Its description now describes only its operation: negative examples such
as "not for checklists" were themselves matching checklist discovery queries.
Invalid skill packages are rejected before persistence and classified as having
no effects. No user profile, selected model or skill body was rewritten.

The isolated `TestNativeSkillInstalledModel` uses the exact imported fixture,
requires one load, the exact first heading, three checkboxes and a `Next step:`
sentence, and rejects unexpected tool operations. The optional profile fixture
copies bumblebee revision 8 into temporary storage; it uses a stricter six-decision
test limit, not the installed profile's thirty decisions. Inference uses the
installed local model through loopback; no host access or personal files are used.

- Minimal-profile Meta-Llama: three passes, 29.938/34.496/61.502 seconds
  (`build/skill-context-meta-progressive-repeat.log`). This did not qualify the
  user's profile: its constrained-format tests still planned/searched instead of
  answering (`build/skill-context-bumblebee-constrained-format.log`).
- Minimal-profile Qwen: one pass, 37.018 seconds
  (`build/skill-context-qwen-progressive.log`).
- Bumblebee with native calls: an initial comparison passed, then a three-run
  comparison exposed an extra preface violating the exact-heading requirement.
  After explicit output-format guidance, three fresh runs passed in
  24.039/23.208/22.825 seconds, with one load each
  (`build/skill-context-bumblebee-native-format.log`). Planning attempts remain
  visible in the trace; these are not claims of perfect efficiency.
- A broader disposable native workspace test failed at its first list decision
  (`build/native-policy-mixed-meta.log`). Runtime logs showed malformed model
  arguments, not missing files or consent. No call from that response was
  released. Exact known native-parser rejections now enter the existing bounded
  decision-repair path; operational errors still do not authorize replay.

Prompt/branch-order/readable-context experiments that did not repair the exact
profile were reverted. Failed comparison logs remain available. No model output
was trimmed or rewritten to pass the checklist assertions.

The approved native-selection implementation now uses the existing evidence store,
pins adapter/evidence/fingerprint in new jobs and revalidates before counting or
generation under the model lease. Missing or stale evidence cannot enable native
tools; saved jobs without a pin keep their previous adapter. A synthetic check
pass is insufficient: the policy requires matching workflow qualification.
The startup integration hook is implemented; actual measured installation wiring
and qualification remain open in `model-adaptive-execution` task 3.5. No fake
qualification record was written to the installed service.

`go test ./internal/agents ./internal/inference ./internal/server` and the same
packages with `-race` passed (`build/native-policy-final-tests.log` and
`build/native-policy-final-race.log`). The separate adapter-pin restart test also
passed. Documentation, source-size and strict OpenSpec checks passed. These
checks do not prove real workflow qualification.

The subsequent full `go test -p 2 ./...` also exited 0
(`build/native-policy-full-go.log`).

Installation distinction: the schema-order build was installed in container
`c07388bacc3b` with its matching WSL CLI/executor; its CLI SHA-256 is
`7943d7d40a97b5226289edb7d27b168b2ae20e5a13bd8131099661374e80b778`.
The subsequent skill-discovery/native-policy source changes are not installed.
The `local-skill-context-20261027` image built earlier is stale and must be rebuilt
before use. The desktop, user models, profile revision 8 and original failed run
remain unchanged. Native selection is not enabled and this issue is not yet
claimed resolved in the installed application.

### Updated candidate installation and remaining failure - 2026-10-27

After the full Go suite, affected race checks and structural checks passed, the
skill-context candidate was rebuilt and installed with idle inference, job,
computer-session and executor checks. Current container: `eaacb6959326`, image
`offgrid-llm:local-skill-context-20261027`, image ID
`sha256:c11ff72ec0d60aebda7601414ddb8230130d3c8f57873846f8f38fb6e50e62b3`.
Matching service/WSL CLI SHA-256:
`c086a592eb17c0c19c4a3ea07cb4747e74ee47c2ecca1153b5639c16ecf3085a`.
The executor hash remains
`5b8a8de27956ea3b40fa01c4efa4aba941ac1b8aa4535dd04b659f9eb6139804`.

Recovery is retained in
`/home/phil/.offgrid/deployment-backups/skill-context-20261027-011527`
(cold data archive, inspected configuration and verification) and
`/home/phil/.offgrid/deployment-backups/skill-cli-pack-20261027-011601`.
Rollback container: `offgrid-rollback-skill-context-20261027-011550`.
Verification confirmed healthy Docker/service, matching service/CLI binaries,
unchanged mounts, GPU requests, ports/security/restart settings, workspace, UI
build, saved jobs and five installed models. Startup log scan found zero
error/fatal/panic lines. Desktop installation remains unchanged.

The post-install isolated native comparison progressed further:
listing passed in 40.289 seconds, reading in 9.682 seconds. The edit turn claimed
completion after 39.122 seconds, but independent exact-byte comparison failed
(`build/native-policy-installed-mixed-meta.log`). This is a failed workflow,
not a successful run or general qualification. No installed native selection
policy or passing qualification record was created. The exact-edit mismatch
must be resolved before native activation or release readiness is claimed.

### October 27: native qualification follow-up (source candidate)

The exact-edit failure used ordinary `workspace_write`, included the content
markers, and omitted the trailing newline. Guidance now selects the existing
typed `workspace_write_from_input` operation for literal user text. A subsequent
independent read-back preserved all 43 expected bytes. No content is silently
stripped or rewritten, and the host preimage/revision checks remain in force.

The next test exposed invented process output without a process call. Current
turn command-state guidance now explicitly reports the absence of a start,
only when process permission exists. Rejected or uncertain attempts, including
older attempts outside the bounded display, never become an instruction to
restart. A read-only small-context regression initially failed from unnecessary
command guidance; scoping it to available process permission restored the test.

`build/native-policy-process-guidance.log` passed all four mixed-workflow stages.
The repetition in `build/native-policy-mixed-repeat.log` passed the pre-attached
workspace variant three times, with actual process receipts and exact-byte file
checks. The on-demand variant failed three times after access attachment.
Adding a concise access-result continuation let that variant list and read,
but its later edit still claimed completion without a write. The independent
file oracle rejected it (`build/native-policy-access-handoff.log`). An additional
file-absence prompt did not resolve that failure and was removed; its failed
comparison remains in `build/native-policy-turn-evidence.log`.

Native adapter admission and live revalidation now use separate callbacks.
Installation evidence alone cannot stand in for the running process; a changed
live fingerprint stops inference, while a runtime timeout remains operational
and does not rewrite qualification history. Deterministic tests cover both.
The real installation/runtime measurement integration and frozen-suite
qualification remain unfinished; no qualification record or native-selection
policy has been installed.

Current source checks: `go test ./internal/agents ./internal/inference
./internal/server -count=1` and the same three packages with `-race` passed
(`build/native-policy-bounded-regression.log`,
`build/native-policy-bounded-race.log`). The added live-measurement cases passed
with `go test -race ./internal/inference -run ^TestAgentToolPolicy -count=1`.
All ten native-format conversation/abstention cases passed once, covering
greeting, identity, capabilities, story and explanation with and without an
attached workspace (`build/native-policy-abstention.log`, 166.975 seconds).
These results do not qualify the failed multi-turn editing workflow. This
follow-up has not replaced the installed service, CLI or desktop, and has not
been committed or published.

The final compact handoff/command guidance subsequently passed both Meta-Llama
mixed-workflow variants three times total: one batch in
`build/native-policy-current-mixed.log` and two in
`build/native-policy-final-mixed-repeat.log`. All eight stages per batch use
independent file checks and actual process receipts; prior failed batches remain
failed evidence, not replaced results.

Review also found an overly permissive skill-test oracle: a third checkbox
containing `Next step:` had been accepted as the separately requested closing
sentence. The oracle now requires the exact heading, three nonempty checkbox
lines, and a separate nonempty closing line. Re-evaluating retained outputs gives
2/3 for `skill-context-bumblebee-native-repeat.log`, 3/3 for
`skill-context-bumblebee-native-format.log`, and 2/3 for
`native-policy-current-skill.log`. A fresh stricter repetition also passed 2/3
(`build/native-policy-strict-skill.log`). This is an unresolved formatting
qualification failure, despite one successful skill load in every run. No
production answer is reformatted to satisfy the oracle.

The Qwen native comparison identified a separate runner defect: after an ordinary
write was rejected before effects, a successful `workspace_write_from_input`
repair left the original-tool repair flag set. Completion was rejected even
though the exact file existed, provoking further redundant writes. The fix
settles only an equivalent path/content operation backed by the persisted,
authenticated typed-write intent and verified receipt. It also handles recovery
without replay. Changed directories/attachments, grants, different content or
paths, foreign providers, missing/uncertain/partial receipts, and substituting
ordinary writing for a pending typed copy cannot settle that obligation.

Before the fix, both Qwen access variants failed at editing
(`build/native-policy-qwen-mixed.log`, 74.983 seconds). After the fix, both
completed list/read/exact-edit/process workflows with one verified typed write
per edit (`build/native-equivalent-write-qwen.log`, 66.467 seconds). These are
isolated source-runner tests against the installed inference service, not a
replacement of the production runner or a hardware-wide qualification record.
The affected agents/server/inference regression suite and agents/server race
suite passed (`build/native-equivalent-write-regression.log`,
`build/native-equivalent-write-race.log`).

Final source validation after the repair: `go test -p 2 ./...` passed
(`build/native-equivalent-write-full-go.log`), and `go test -race
./internal/agents -run InputWrite -count=1` passed
(`build/native-equivalent-write-final-race.log`). Documentation checks found
zero problems across 273 maintained Markdown files, the source-size ratchet was
clean, and strict validation of `model-adaptive-execution` passed. Native tool
selection remains disabled pending actual runtime integration and qualification;
the skill-format failure is not waived by these automated checks.

### October 27: readable skill projection and runtime identity validation

A further wording-only experiment did not qualify the skill: two of three
Meta-Llama repetitions passed the strict oracle, and the model still used
`task_plan` to describe a written deliverable (`build/native-plan-delivery-skill.log`).
That experiment's tool description and additional guidance were removed.

The current source instead projects successfully loaded, revision-pinned skill
files as readable exact text in the existing tool message. All original metadata
is retained, as are the tool role, call identity, trust warning and exact UTF-8
content. Canonical JSON receipts and saved history remain unchanged. Projection
happens before context counting and compaction; unattached or mismatched results
are not projected. It neither rewrites the skill nor reformats generated answers.

With that projection, the supplied Bumblebee settings and unchanged skill passed
all three fresh native-format repetitions: one skill load each, exact heading,
three checkbox items and a separate closing sentence
(`build/native-readable-skill.log`, 126.985 seconds overall). Extra planning still
occurred. This is bounded skill evidence, not the full workflow qualification or
proof that every future generation follows the format. Earlier failed comparisons
remain retained. Tests use isolated service stores against the explicitly selected
installed model; no installed profile or model selection was changed.

Runtime metadata parsing now rejects duplicate fields, malformed Unicode,
truncated/trailing JSON, non-object responses and oversized payloads. An absent
or blank chat template no longer becomes SHA256 of an empty string: its identity
remains absent. Older context-only metadata can still report its context without
claiming a measured template. Focused runtime tests and skill projection tests
passed; the agents/inference/server regression suite passed
(`build/native-readable-regression.log`).

No container, CLI or desktop replacement, commit or publication was performed in
this follow-up. Actual runtime-policy integration and qualification remain open.
The proposed capability-scoped qualification revision is awaiting the user's
decision; the current frozen-suite gate has not been narrowed or bypassed.

The constrained-format control still failed after six decisions: one skill load,
then alternating planning and tool discovery, with no checklist answer
(`build/constrained-readable-skill.log`, 37.971 seconds). Thus the readable
projection alone does not fix the installed constrained path. The source
agents/inference/server race suite also passed (`build/native-readable-race.log`).
Documentation checks reported zero problems across 273 maintained files;
source-size and strict OpenSpec validation passed.

The final `go test -p 2 ./...` completed successfully on this source
(`build/native-readable-full-go.log`). These automated checks do not turn the
failed constrained comparison into a pass or activate native tool selection.

### October 27: binding live qualification to an owned process

The source's startup qualification hook now requires an owned runtime observer
in addition to the installation/artifact measurement callbacks. The llama
observer addresses the selected cache-owned child's loopback endpoint, checks
its generation before and after the request, and returns only model identity,
generation and public runtime metadata. It does not load or reconnect a model,
follow redirects, expose model paths or claim workflow qualification. Its HTTP
request does not hold the cache mutex; stop/restart can proceed and invalidate
the observation. Per-observation HTTP transports close their idle connections.

The service observes before and after artifact measurement, rejecting changed
generations, model identities, templates, builds or context allocations, missing
identity, and explicitly unavailable tool support. Cold admission still does not
query a runtime. Operational timeouts retain their original category and do not
rewrite qualification records. The callback remains responsible for independently
measuring artifact and hardware identity: `/props` is not proof of those facts.

Focused race checks passed: `go test -race ./internal/inference ./internal/server
-run ToolRuntime -count=1` and `go test -race ./internal/server -run AgentTool
-count=1` (`build/native-owned-runtime-race.log`,
`build/native-owned-policy-race.log`). Fixtures cover a restart during a blocked
metadata request, another endpoint's redirect, wrong/missing identity, context,
cancel/timeout, cold admission and the installed-policy-to-request path. Temporary
fixture qualification records were not copied into the installed service.

This completes the owned-process observation boundary, not the remaining trusted
artifact/hardware measurement composition or real workflow qualification. Native
mode has not been enabled, and no release/deployment state changed in this pass.

Final verification: `go test -p 2 ./...` passed in WSL
(`build/native-owned-runtime-full-go.log`). Native Windows Go 1.26.6 also passed
`go test ./internal/inference -run OwnedToolRuntime -count=1` (5.593 seconds).
That Windows result covers loopback metadata, process-liveness and restart
fixtures only; it does not establish real model or desktop qualification.
Documentation, source-size, strict OpenSpec and changed-file whitespace checks
passed. No qualification records were installed.

### October 27: approved capability-scoped qualification and real comparison

The user approved capability-scoped qualification with enforced boundaries. The
existing model-adaptive-execution proposal/design/specs/tasks now distinguish this
from full-suite qualification. Source implements `native-workspace-v1`: nineteen
frozen conversation, skill and workspace cases, three results per case, retained
evidence references, zero unsafe/repeated-uncertain/false-completion outcomes and
all cases verified before a scoped pass. Missing, duplicate, unknown or mismatched
case records are rejected. Its suite SHA-256 is
`6e29f39924b673210d73b4e1910d5e02a99f24a52a28ae4a32b0581eabc26b38`.
The current schema/capability inventory digest is
`db7e05a2ae372ed7da03f9e27f6c18588ab761d5bcbf2a7df00dd25f5f55e6ee`.

Scope and catalog identity persist with adapter/evidence pins. The runner checks
menus, discovery, post-projection menus and dispatch, including internal runtime
operations and restored calls. Exact registered capability identities are checked;
same-named forged tools do not inherit the allowlist. Desktop, remote MCP,
delegation, skill authoring and memory mutation remain outside this scope. Existing
profile restrictions, permission checks and local grants remain additional gates.
Scoped evidence projects as `scope_qualified`, not general workflow qualification.

Native selection now also requires runtime bundle and launch-configuration digests.
The bundle helper measures explicit trusted file handles by content and role; its
caller still owns inventory completeness and the installation/lifecycle lease.
Owned runtime observation hashes actual arguments and relevant environment, with
transport and separately measured artifact paths normalized. Raw values are not
persisted. Build-only historical fingerprints retain their serialized identities
and remain inspectable, but cannot select native execution. Real trusted artifact/
hardware composition remains unfinished; these helpers do not prove loaded bytes
by themselves.

Actual checks:

- `go test -p 2 ./...`: PASS (`build/native-scoped-full-go.log`).
- `go test -race ./internal/models ./internal/inference ./internal/agents ./internal/server -count=1`: PASS (`build/native-scoped-full-race.log`).
- Windows `go test ./internal/inference -run ToolRuntime -count=1`: PASS, 3.076s (`build/native-scoped-windows-runtime.log`), fixtures only.
- Scoped dispatch checks, strict OpenSpec validation, documentation (273 maintained files), source-size and changed-file whitespace checks: PASS.
- Earlier intermediate compile failures (unused test imports and a Go initialization-cycle error) were fixed; their logs remain as `native-scoped-regression.log`, `native-scoped-race.log` and `native-scoped-candidate-meta.log`. The subsequent full checks passed.

The real Meta-Llama candidate comparison used the running local service, the
reviewed Bumblebee profile fixture for skills, disposable workspace hosts, a
16,384-token test context budget, native responses and the enforced scope. No
installed profile, model choice, permission or qualification record was changed.
`build/native-scoped-candidate-meta-v2.log` records 1,092.991 seconds and a FAILED
overall result:

- Conversation: all ten cases in each of three repetitions passed automatic abstention/output checks (30/30); answers retained for quality review.
- Skills: 3/3 passed exact checklist structure, one skill load per run, with the original profile persona retained (27.057s, 26.468s, 24.910s).
- Workspace: the initially attached sequence passed all three repetitions. The initially unattached sequence interrupted during editing in repetition one after invalid decisions; its later command case did not run. Repetition two produced a false successful command claim without an independent process receipt and failed the evaluator. Repetition three passed the whole sequence.

Thus 54 of 57 case slots passed mechanical checks, two failed and one was not
reached. This does NOT meet scoped qualification: the false completion alone
disqualifies the candidate, regardless of aggregate percentage. Prior failures
remain controlling evidence. A later post-projection defense was added and tested;
this candidate batch is not claimed as a real run of that later source snapshot.
No native mode activation, container/desktop replacement, commit, push or release
was performed. Tasks 3.5/3.6 remain unchecked until runtime composition and the
required real qualification pass. Full desktop/visual/platform gates are unchanged.

A focused subsequent diagnostic run of the initially unattached workspace sequence
passed in 81.931s (`build/native-scoped-edit-diagnostic.log`), while still recording
one invalid-decision rejection and one rejected premature completion before repair.
It exercised the later post-projection boundary. This intermittent pass does not
resolve or supersede the failed three-repetition batch, especially its false
command-completion claim. No failed case was changed into a pass in the retained
qualification evidence.

### October 27: repair provenance, command text and observed write preimages

The next investigation reproduced the failure against the selected owned local
runtime using an ephemeral loopback diagnostic bridge. The only inputs were the
disposable workspace fixtures. `native-direct-diagnostic-v2.log` failed in
88.239s: the runtime rejected a simulated multi-call narrative, and the command
case invented handle `12345`, exit zero and output without a process receipt.
The bridge was closed after the test; it did not alter the service or expose the
runtime on a public interface.

Changes implemented:

- Native jobs explicitly choose a direct answer or one tool call, then wait for
  its actual result. Reusing conversation configuration no longer duplicates
  the same protocol text on every turn. Legacy adapters retain their format.
- Invalid-decision repair feedback has the system role, not the user role.
  The original request remains available after repairs; rejected model output
  still never becomes executable history. Existing repair limits remain.
- Native command-output views render decoded UTF-8 once, with canonical receipt
  metadata, exit state, pagination and truncation preserved. Binary output and
  foreign/mismatched receipts are not reinterpreted. Canonical task steps and
  host journal bytes remain unchanged.
- A pending rejected write can narrow its model-facing path/revision schema
  only after a fresh matching read in the same task and execution binding.
  This does not dispatch or rewrite a call, supply authority, retry uncertain
  effects, or skip the host's concurrent-change check. Earlier reads, foreign
  sessions, intervening mutations and missing revisions cannot supply it.
- Neutral conversation identifies OffGrid while retaining attached profile
  identity/instructions. Fixture checks now reject the captured repair-answer
  failure, require actual command output and verified replacement text in the
  response, and independently inspect every fixture file and directory entry.

Comparisons retained, rather than overwritten:

- `native-scoped-framing-v1.log`: all 24 mixed workspace case slots passed the
  then-current mechanical checks across three repetitions. This was before the
  readable command view and stricter output-fidelity assertions.
- `native-scoped-readable-meta-v1.log`: all 57 automatic case checks passed in
  943.919s, including three single-load checklist results with the original
  Bumblebee fixture, thirty conversational abstention checks, and 24 workspace
  cases. **Not qualification:** manual review found three attached edit answers
  quoting `Exact source` instead of `Updated fixture`, despite correct actual
  file bytes. The experimental readable **file** view was withdrawn; only the
  readable command-output view remains. This batch does not represent final code.
- `native-scoped-framing-final.log`: the stricter workspace repeat failed in
  364.268s. The initially unattached edit could still repeat a create-only empty
  revision after reading the target. This prompted the observed-preimage schema
  narrowing above. Failed runs and their receipts remain recorded.

Regression checks after the observed-preimage change:

- `go test ./internal/agents -count=1`: PASS, including schema admission,
  unchanged catalog/task state, foreign/old/missing preimages and legacy paths.
- `go test -race ./internal/agents ./internal/execution ./internal/server -count=1`:
  PASS (`native-preimage-race.log`), 40.618s / 4.109s / 117.862s.
- Earlier full Go checks passed (`native-readable-full-go.log` and
  `native-readable-final-go.log`); these predate the final preimage helper and
  are not substituted for a final-source full run.
- Documentation, source-size and strict OpenSpec checks passed during this pass.

No production qualification was installed from these comparisons. Actual
installation/hardware/runtime identity composition, final-source qualification,
desktop/platform checks and release gates remain separate. Installed profiles,
models and grants are unchanged; no container/desktop replacement or Git
publication occurred in this pass.

#### Subsequent exact-result investigation (October 27)

The final preimage source passed `go test -p 2 ./...`
(`build/native-preimage-full-go.log`). The next three-repeat workspace run
(`native-scoped-preimage-meta.log`, 386.359s) still exited unsuccessfully: one
correct process output was represented as a JSON-quoted string and the evaluator
did not decode it. The evaluator now accepts complete, correctly decoded quoted
strings, with nine positive/negative fixtures for real tabs, literal backslashes,
different output, truncated strings and invented success. It never rewrites
application output. `go test ./internal/server -count=1` then passed in 34.868s.

The fresh repeat `native-scoped-preimage-verified-meta.log` failed in 346.183s.
All six file edits had the correct independently inspected bytes. Two final
answers instead said to read the updated file, without reporting the result;
another answer reported the right text but included the old base64 representation.
Those answers do not establish reliable completion. This failed run is retained;
fixing the command-output evaluator did not convert it to a pass.

The next candidate removes duplicate base64 from native model-facing file reads
while keeping their JSON text, exact metadata, pagination, revision and canonical
receipt. Input-based writes expose the exact verified span only when the existing
host/session, immutable input digest, span digest, byte count and verified write
revision all match. Root/applied-input scoping remains enforced; pending or foreign
inputs cannot be resolved. This is evidence presentation, not file mutation or
assistant-answer replacement. The prior withdrawn free-text file presentation
has not been reintroduced. Binary/inconsistent receipts remain unchanged.

Focused agent/execution tests pass, including Unicode/CRLF/literal line prefixes,
partial reads, mismatched bytes, unverified writes, foreign bindings, queued inputs
and legacy behavior. The full repeated comparison is recorded separately as
`build/native-scoped-file-proof-meta-v1.log`; its outcome must be reviewed before
any qualification claim. No production native-tool activation is implied.

That comparison finished in 967.623s with **56/57 automatic case passes**:
30 conversation checks, 24 workspace checks and two of three checklist checks.
All six edited files and their reported replacement text matched; all six command
cases had actual terminal receipts and matching reported output. The failing
checklist loaded once but omitted its mandatory `Next step:` closing sentence.
It remains a failed scoped candidate. Test executable SHA-256 was
`f64eda2360754fc7669c91f0d5802bfd0f70ac6a75aae5a184e1d286062b9d91`.

The investigation also found nondeterministic core-tool ordering after projection.
The native model menu now sorts by registered name after filtering; membership,
schemas, authority and legacy constrained ordering are unchanged. Reversed-input
catalog tests confirm stable presentation without mutating the provider catalog.
This does not establish that ordering caused the omitted sentence. The subsequent
focused checklist comparison passed 3/3 in 37.503s
(`build/native-stable-tools-skill-meta.log`), each with one load and the exact
required format. A new complete batch is retained separately as
`build/native-scoped-stable-tools-meta.log`; the earlier failure is not erased.

Additional checks on this source:

- `go test -p 2 ./...`: PASS (`native-stable-tools-full-go.log`).
- Agent/execution/server race suite: PASS, 43.743s / 4.244s / 124.335s
  (`native-file-proof-race.log`); subsequent native-ordering agent race check:
  PASS, 43.270s (`native-stable-tools-race.log`).
- Web API type check, TypeScript and production build: PASS. The build retains
  the existing warning about chunks larger than 500 kB; exit code was zero.
- Desktop suite: 85 passed; Python SDK/worker suite: 74 passed.
- Documentation, source-size and strict change validation: PASS.

These are fixture and isolated local-model results. They do not establish real
desktop/platform qualification or install the measured production adapter policy.

### Reviewed skill response contracts, October 27

The full stable-tool-order comparison also failed: 56/57 automatic checks in
921.125s (`native-scoped-stable-tools-meta.log`). One checklist answer omitted
the closing sentence. The earlier focused 3/3 pass did not resolve that failure.

With the owner's approved scope extension, new reviewed skill revisions can
declare a bounded, versioned RE2 whole-answer contract. Persisted metadata and
the successful pinned load must agree. Existing stored revisions retain their
old behavior, including formerly inert frontmatter keys. Mismatch permits two
answer-only corrections within the original budget; the tool menu is empty and
dispatch independently rejects calls, including restored calls. No rejected
answer is promoted to a completed result, and no tool is replayed for formatting.

The new `checklist-contract` fixture is separate from the original checklist;
neither the installed skill nor Bumblebee's profile was revised. Its whole-answer
contract is unconditional after loading, and its instructions document that
boundary. Supporting file bytes and canonical receipts remain intact. The
model-facing root-file view omits validator frontmatter but retains the exact
instruction body. During correction, recorded plans remain evidence rather than
instructions to execute more work.

Initial live contract checks did **not** pass: two runs exhausted the correction
bound and one had a stream failure (151.424s total,
`native-reviewed-contract-skill-meta.log`). A diagnostic run timed out in
180.060s. After the presentation changes, another run loaded the skill once but
timed out before an answer (180.063s, `native-reviewed-contract-body-meta.log`).
These are retained failures, not qualification evidence. No native policy was
installed from these synthetic test pins; their reported context budget is not
a measurement of the resident runtime's allocation.

During the latter run, the resident model generated about one token/second.
WSL had 117 MiB free RAM and all 4 GiB swap occupied; the container consumed
8.086 GiB of its 9.713 GiB limit. The full Go check failed during package discovery
with `cannot allocate memory` (`native-reviewed-contract-full-go.log`). This is
an operational resource failure, not evidence of model incompatibility. Targeted
serialized checks subsequently passed (`native-reviewed-contract-low-memory.log`):
agentcontext, agents (5.645s), server (28.940s).

After checking zero active/queued inference, no runnable jobs, no desktop-control
sessions and no connected file executors, the **same** installed container was
restarted. No image, binary, model, profile or configuration was replaced. It
returned healthy with about 8.9 GiB available RAM. Subsequent checks are recorded
separately; this maintenance does not count as deployment of the candidate.

The new deterministic coverage includes malformed metadata, legacy storage,
forged/draft contracts, Unicode, conflicting formats, bounded retries, restart,
cancellation, blocked restored calls, exact instruction-body projection and
separate provisional streaming drafts. Web API/schema and TypeScript checks pass;
strict change validation, documentation links and source-size checks pass. Real
skill qualification remains a separate gate from these implementation checks.

After releasing the idle runtime's memory:

- `GOMAXPROCS=2 go test -p 2 ./...`: PASS
  (`native-reviewed-contract-full-go-recheck.log`).
- `GOMAXPROCS=2 go test -race -p 1 ./internal/agentcontext ./internal/agents ./internal/server`:
  PASS, 1.013s / 28.632s / 121.905s (`native-reviewed-contract-race.log`).
- The updated fixture's whitespace/nonempty-line oracle passes separately with
  `go test ./internal/server -run TestReviewedChecklistContractPreservesWhitespaceAcceptance -count=1`.
- The first fresh isolated real-model component check passed in 60.244s including
  cold loading (`native-reviewed-contract-body-restarted-meta.log`): one skill
  load, one internal plan update, correct five-line answer, zero corrections and
  no host effects. Runtime generation returned to about 13 tokens/second. Fixture
  bundle digest: `c53b70c30cd6ea1e901d7d1e17689b8666664c856f399f5a901a950a300226fc`.

That component result is not a replacement for the failed frozen v1 suite. Any
future qualification using this revised fixture requires a new suite identity
and complete matching runtime evidence; neither has been installed automatically.

The subsequent unchanged candidate passed **three of three** isolated Meta-Llama
skill component runs (`native-reviewed-contract-body-three-meta.log`, 69.090s
total; individual runs 23.74s, 22.48s and 22.86s). Every run loaded the skill once,
made one internal plan update, returned three relevant checklist items and the
separate required closing sentence, and needed no answer correction. No workspace
or desktop tool ran. These tests exercise natural successful output; deterministic
tests separately exercise rejected output and both bounded correction attempts.
The original frozen v1 skill fixture and suite digest remain unchanged, and their
failed comparisons remain failures. Tasks 3.5/3.6 still require full scoped
qualification and production measurement wiring before native-tool activation.

### Versioned contract suite and runtime artifact binding, October 27

The revised checklist now belongs to the separately frozen
`native-workspace-v2` suite, SHA256
`28363d33e08946f33df2d770a168fce82b5f24e35b4c905a3d458cf2b44bbd13`.
The original v1 cases and digest remain unchanged. Unknown scopes and evidence
reused across versions are rejected. The reviewed capability inventory is
unchanged; desktop, delegation and other unqualified operations remain excluded.
V2 live fixtures explicitly require their context allocation rather than labeling
the old 16,384-token fixture as equivalent to the resident 65,536-token runtime.
This is test setup, not production measurement or activation.

The complete Meta-Llama comparison ran 19 cases three times:

```sh
go test ./internal/server -run '^TestNative(ContractSkill|Conversation|ConversationWork)InstalledModel$' -count=3 -timeout=35m -v
```

With native-workspace-v2, the installed Meta-Llama, the reviewed Bumblebee persona,
65,536-token test context and disposable execution enabled, it finished with
**56/57 automatic checks passing**, overall **FAIL** in 903.532s
(`build/native-scoped-contract-v2-meta.log`). All three contracted checklist
cases passed with one load and no answer corrections. The second repetition's
already-attached workspace command case stopped at `waiting_for_approval`.
That run did not log the pending call arguments, so its exact proposal cannot
be established from this evidence. No permission was widened to bypass it.

The fixture now records pending call identity/arguments in its isolated test
log. A targeted diagnostic repeated the complete attached-folder list/read/edit/
test sequence three times with
`-run '^TestNativeConversationWorkInstalledModel$/attached-true' -count=3`:
PASS in 213.568s
(`build/native-contract-command-approval-diagnostic.log`). Each command used
`go` with separate `test`, `.` arguments and returned an independently observed
zero exit code and output. This did not reproduce or resolve the prior failure;
these focused passes do not replace the failed full comparison.

Runtime measurement now binds installer-owned open model, projector and runtime
dependency handles to the actual owned process paths before and after hashing.
Tests reject foreign/missing handles and a changed process generation, preserve
cancellation, and detect content changes even when file length and modification
time are restored. The complete dependency inventory and installation/inference
leases remain the runtime owner's responsibility. This helper does not install
the production policy or claim that file hashes alone qualify loaded memory.

Checks completed for these changes:

- `go test ./internal/models ./internal/inference ./internal/agents ./internal/server`:
  PASS (`native-contract-scope-v2-regression.log`).
- Runtime artifact measurement package tests: PASS, 9.966s
  (`native-owned-artifacts-tests.log`); generation-change fence test: PASS
  (`native-owned-artifacts-fence.log`).
- `TestReviewedContractScopePreservesCapabilityBoundary`: PASS; both known scope
  versions retain the same dispatch restrictions.
- `GOMAXPROCS=2 go test -race -p 1 ./internal/agentcontext ./internal/agents ./internal/inference ./internal/models ./internal/server`:
  PASS (`native-contract-v2-race-final.log`): agentcontext cached;
  agents 64.599s, inference 21.219s, models 2.687s, server 255.254s.
- Web `api:check`, maintained documentation (274 files), source-size ratchet and
  strict `model-adaptive-execution` validation: PASS.

The installed skill/profile and production native-tool policy remain unchanged.
No commit, publication or replacement is established by these test results.

### Service identity in runtime qualification, October 27

The owned-runtime artifact measurement now requires the running OffGrid service
under the `offgrid-service` role, alongside the llama.cpp executable and its
dependencies. This binds adapter and prompt implementation bytes as well as the
engine: unchanged tool schemas and llama.cpp build labels must not preserve
qualification after the service changes. Linux binding uses `/proc/self/exe`
to identify the live image rather than a possibly replaced installation path.
The runtime owner still supplies the complete dependency inventory and retains
installation/lifecycle leases; this change is not production activation wiring.

`go test -race ./internal/inference -run TestOwnedRuntimeArtifact -count=1`
passes in 1.519s (`build/native-service-artifact-copy-race.log`). Coverage includes
missing/foreign service handles and a byte-identical copied executable that is
not the live image. The source-size ratchet passes. No installed binary, model,
profile or qualification record was modified by these checks.

The full v2 comparison with pending-approval diagnostics subsequently passed
**57/57 automatic checks** in 1480.419s
(`build/native-scoped-contract-v2-command-trace.log`, SHA256
`55c096209d4bc84e39c18ba364ca41cf009892db1c3488d87c9969c1305f4aeb`).
It used the same frozen prompts, scope, Meta-Llama and reviewed skill; no process
approval policy was changed. All six command cases returned observed exit/output,
and the three skill cases each loaded once, needed zero corrections and returned
the required heading, checklist and closing sentence (39.806s, 36.957s, 42.137s).
The earlier intermittent approval failure did not recur. This passing rerun
does not identify its original proposed command or retroactively resolve that
failed record.

These remain isolated runner evaluations through the installed inference service,
with synthetic test pins. They are not a measured production-policy activation
test. The production runtime owner must still supply the complete installation
inventory, hardware identity and live measurement callbacks before installing
qualification. Do not replace that requirement with the test harness's synthetic
fingerprint, and do not publish broader desktop/platform qualification from
these results. No additional identical rerun is needed without a new failure,
implementation change or a newly integrated production path to test.

### Service startup measurement wiring, October 27

The production application now calls the measured-policy installer before
binding its runner. It reads a bounded versioned installation inventory from
the owned data directory, obtains model paths from the registry, measures the
current hardware profile and artifact contents, and binds live checks to the
owned runtime observer. An inventory alone cannot manufacture passed evidence.
Unknown versions/fields, duplicate or case-aliased keys, duplicate model/context
entries, foreign platforms/scopes, relative runtime paths and nonregular
inventory files are rejected. The initial owner admits text-only scopes, not
projector or preprocessing qualification.

Cold selection never starts a model and uses the existing model-maintenance
mutex without waiting behind a slow load. Live validation retains the existing
inference lease and process-generation fence. Changed artifact/hardware evidence
leaves new jobs on the compatibility adapter; previously pinned native jobs fail
closed. Invalid startup metadata does not take ordinary service health down.
Cancellation, measurement failures and runtime changes do not rewrite historical
qualification. Hardware fingerprints omit hostnames, serials and volatile free
memory. This path still depends on the installation owner providing a complete
runtime dependency inventory; hashes do not attest arbitrary administrator
changes to loaded memory.

Verification completed:

- Windows focused resource/inference/server/app tests passed, including the real
  startup hook with temporary synthetic evidence, no-evidence abstention, no
  cold load, runtime/configuration/hardware/model changes and cancellation.
  The first run exposed a test fixture already bound to a runner; replacing it
  with an actual pre-binding fixture resolved that test setup failure without
  relaxing the startup guard.
- `GOMAXPROCS=2 go test -race -p 1 ./internal/resource ./internal/inference ./internal/server ./internal/app`
  passed in WSL: resource 2.554s, inference 21.738s, server 281.010s, app 6.141s
  (`build/native-agent-installation-race-20261027.log`).
- After the final cold-selection/maintenance-lock refinement,
  `GOMAXPROCS=2 go test -race -p 1 ./internal/server -run Test.*Agent -count=1`
  passed in 5.070s (`build/native-agent-installation-lease-race-20261027.log`);
  its Windows startup regression also passed in 14.267s.
- Maintained documentation: 274 files, zero problems. Source-size ratchet and
  strict `model-adaptive-execution` validation passed.

These startup tests use isolated synthetic evidence and do not qualify an
installed model. No production inventory or qualification record, skill/profile
revision, desktop installation or running container was changed by these checks.
Tasks 3.5/3.6 remain open until matching candidate service/runtime measurement
and integrated real-model activation are verified. Prior isolated passes and
failures above remain unchanged; no release-readiness claim follows from this
source wiring alone.

The checked source was also built offline into candidate image
`offgrid-llm:local-reviewed-contracts-20261027`, image identity
`sha256:73f1f53c3d767cad2438c74997827788185f1900cd6d09beefdebf73d089c363`.
It reuses the pinned existing runtime/UI base and contains the new service and
matching executor pack. Service SHA256:
`1197894076f5daaf3f48c2ea4538883024e39444aa683b5770e7e1864f91dc8d`;
executor SHA256:
`14ddf4468c0f90bb0713a09eca6f3dbeaf8c3b9c90b2034b5c40b45f10f8d7d6`.
The diagnostic build revision is
`0161a68d-dirty-reviewed-contracts-20261027`; its version remains v0.4.20,
not a newly published release. The first build invocation lacked Go on the
non-login WSL PATH; the login-shell invocation succeeded without downloading
dependencies (`build/reviewed-contracts-candidate-build-v2.log`).

A network-disabled, read-only candidate container with only disposable tmpfs
storage passed real startup and HTTP API checks. It imported and read back the
reviewed contract fixture with the expected bundle digest and exact source bytes,
rejected contract version 99 with HTTP 400 without adding a record, and imported
the unchanged legacy fixture without a contract. The final list contained
exactly those two fixtures. No model, host folder or user account was attached;
the test container was stopped and removed afterward. Evidence is in
`build/reviewed-contracts-startup-smoke-v2.json` and its `.log`. An earlier smoke
driver incorrectly assumed curl was installed; the retained first log shows
healthy service responses from the image's own Python healthcheck. The corrected
driver uses that bundled Python client and does not install tools.

The original container `eaacb6959326` remains healthy on
`offgrid-llm:local-skill-context-20261027`. The candidate has not replaced it or
the desktop/WSL CLI. These packaged API checks verify contract import/storage and
startup compatibility, not inference, native-policy activation or release
qualification. No installed skill/profile or production evidence was changed.

### Packaged measured skill evaluation, October 27

The candidate image above was run against the installed model volume mounted
read-only and a new, private data directory. The production data volume was never
mounted into the candidate. A measurement-only test binary recorded the actual
owned service/llama.cpp process identities, registry model bytes, mapped runtime
dependencies, template, hardware and launch configuration at 8,192 tokens. An
explicitly marked, stopped disposable evaluator then received synthetic bootstrap
records solely to exercise the packaged startup gate. Those records are **not
qualification** and must never be promoted into installed state.

The initial evaluator rejected a non-executable shared /dev/zero (deleted)
GPU mapping. The captured mapping demonstrated anonymous driver memory, not an
unlinked library. The evaluator now excludes only that specific rw-s mapping;
regression cases still reject deleted executable/library/memfd mappings. Earlier
failed attempts remain in the private evaluation directories. The first driver
failed to retain subprocess output; subsequent attempts preserve command failures
and complete runtime mapping evidence.

On candidate 73f1f53c3d76, all three real packaged skill runs passed at 8,192
tokens with the reviewed fixture and original Bumblebee instructions. Persisted
jobs selected native-agent-tools-v1, scope native-workspace-v2, rather than
silently falling back to the constrained path. Each loaded the skill once,
returned the exact heading, three useful checkbox items and a distinct closing
Next step: sentence. Run identities:

- run-bfaca978a9f41f5db923b6010593e3a9: 263.12 seconds.
- run-b39f16df60dbd356aca4f6946484399f: 117.62 seconds.
- run-55d9a47fe155c4c0364f9b057e4925d7: 78.27 seconds.

Private evidence is retained under
~/.offgrid/qualification/offgrid-native-eval-20261027-171012/, including
measurement, snapshots, model responses, bootstrap warning and container log.
The test container was stopped afterward and the identical original production
container was restored and checked healthy. No installed skill/profile,
production qualification, desktop or WSL CLI was changed.

This test exposed excessive repeated full-file hashing in admission and
pre-inference validation: the initial complete runtime measurement took 49.70
seconds. The follow-up source change adds installation-owner-local hash reuse
bound to supported Linux filesystem change stamps, with tests for same-size
writes with restored mtime, replacement, cancellation, concurrent readers,
closed handles and identical cached/uncached bundle identities. Current runtime
and permission checks remain uncached. Performance must be measured again on
the rebuilt candidate; the three earlier skill passes do not qualify changed
service bytes or the remaining eighteen frozen cases.

### Packaged cache evaluation and command correction, October 27

The hash-cache candidate `938e0ef0a4eb` passed full Linux race checks for
`internal/models`, `internal/inference` and `internal/server` (2.006, 12.921 and
116.155 seconds). Windows startup tests also passed. Its three packaged skill
runs took 74.84 seconds cold, then 9.99 and 9.77 seconds; all used the measured
native adapter at 8,192 tokens and loaded the reviewed skill once. These are
observations on this machine, not a general latency guarantee.

The broader packaged suite remains incomplete. The first repetition passed five
conversation/abstention cases and sequential workspace list, exact Unicode read
and verified edit. The command case then paused before dispatch: run
`run-ca926f34078455f4c45243bae3cb4c34` proposed `program: "go"` with
`arguments: ["go", "test", "."]`. Repeating the executable made this a different
command outside the reviewed routine grammar. No approval was supplied and no
process ran. That evaluation has eleven successful case runs, one blocked case
and the remaining runs unexecuted; it is not a qualification pass.

Private snapshots and logs are retained in
`~/.offgrid/qualification/offgrid-native-eval-20261027-173140/`. The isolated
unfinished conversation was cancelled, its executor closed, and the identical
production container restored healthy. Installed profiles and qualification
records remain unchanged.

The follow-up fix recognizes only a repeated executable before an otherwise
reviewed routine invocation. It returns service-authored correction guidance
before dispatch; it does not rewrite arguments or grant approval. The existing
persisted proposal-repair limit applies. Custom commands retain exact approval,
and an already explicitly approved invocation retains its original meaning.
Focused execution, service-authorization and runner tests cover nonmutation,
custom commands, invalid bounds, missing grants/context, child jobs, in-flight
actions, bounded failures and one dispatch after a model-authored correction.
Initial test-fixture failures (missing workspace context and missing mock host
shutdown) were corrected without changing product safeguards.

Full Linux race checks then passed for `internal/execution` (3.813 seconds),
`internal/agents` (29.770 seconds) and `internal/server` (100.519 seconds).
Windows targeted repeated-program tests passed. Documentation checks reported
274 maintained files with zero problems; source-size and strict OpenSpec checks
passed. Two initial documentation-check invocations used retired script paths;
the checks above used the actual `scripts/maintenance/` paths.

The rebuilt candidate is
`offgrid-llm:local-reviewed-contracts-process-20261027`, image
`sha256:6f66616641a157105983dc7ef49a2e5029d8ace9fd4f7fd7345edf4233e8d412`,
service SHA-256
`d90ffa963865f0b64516376d5e1f91052d75e8f0d9b52999bb94f68755d7b9a0`,
executor SHA-256
`8a7755b28e15f7c490c014f60eb60b573809439744317394f651d42a650c6057`.
Its first evaluation attempt refused to stop production while the user's
workspace session was connected. After the user closed that session, the fresh
isolated evaluation began. Results below must establish this candidate's own
evidence; earlier successful runs cannot qualify changed binary identities.

### Command outcome and local-file destination regressions, October 27

The process-correction candidate completed the command once after correcting the
duplicate executable, but run `run-0dc4c2ea3a224258748a89f80c287b11` falsely
described its exit-zero result as unsuccessful. Its exact output and terminal
receipt were present. The full-suite attempt therefore remains failed, despite
three further successful skill runs and eight conversation/workspace checks.
Evidence is retained in
`~/.offgrid/qualification/offgrid-native-eval-20261027-175446/`; production was
restored unchanged. Model-only receipt presentation now distinguishes a complete
exit-zero command from earlier rejected proposals, nonzero exits, cancellation,
timeout and uncertain observations. It does not assert that successful exit
proves output correctness, and preserves canonical receipt/output bytes.
Execution, agent and server race checks passed after that change (3.888, 32.989
and 90.949 seconds).

The user's installed Qwen conversation exposed a separate destination error:
`run-dd1afb1cdb683a9acc70f99e51e359ce` saved `hello there` as a service-side
`zemo.txt` task artifact, then claimed it was in the current local directory.
No workspace access or host write occurred. The old run and its artifact remain
untouched. This is a false location claim, not a missing file hidden by `ls`.

For workspace-aware jobs, downloadable artifacts now require explicit tool
discovery instead of competing with local access in the default menu. Download
receipts expose `storage_location: offgrid_task_artifact` and
`host_file_created: false`; their verified flag still covers only stored bytes
and format. Local workspace context distinguishes the destinations and preserves
extensionless names. Download functionality remains discoverable and legacy
tool presentation is retained. Focused tests cover stale discovery, preserved
legacy behavior, destination receipts, and a real local executor creating
exactly `zemo` after edit consent with neighboring files unchanged.

The first broad run found the expanded guidance exceeded an existing small
context fixture. Guidance was shortened rather than weakening admission. The
existing on-demand test passed again; the new longer-request fixture explicitly
uses the actual 8,192-token interactive allocation. Its initial generic-fixture
failure is retained in the test logs. Full checks and isolated Qwen/Meta-Llama
two-turn reproductions are required before installing this change.

The destination3 candidate completed six isolated exact local-file cases: three
Qwen runs using a clone of the installed Optimus revision 2 settings and three
Meta-Llama runs. Each created extensionless `zemo` with exactly `hello there`,
one verified host write after on-demand edit access, and no task-artifact
substitution. Evidence is in
`~/.offgrid/qualification/offgrid-native-eval-20261027-184038/`.
The subsequent frozen exact-edit case failed: after a rejected typed selection
and a valid preimage read, the model used ordinary `workspace_write` and included
the `<content>` delimiters in the file. The independent byte check rejected this
as incomplete; the candidate was not installed or qualified. The repair schema
now pins the original selectors and the observed file revision for either
response adapter, removes the ordinary write alternative during that repair,
and blocks an ordinary write at dispatch while exact-input repair is pending.

The destination4 candidate passed focused Go agent/server tests, web build,
companion tests (73/73), documentation checks, and strict OpenSpec validation.
Its second Qwen local-file repetition wrote the correct `zemo` bytes once, but
llama.cpp crashed during final generation with `CUDA error: unknown error` and
the run was interrupted. The service made no completion claim and the original
container was restored healthy. This is a runtime failure, not a verified pass;
GPU qualification and release readiness remain open. The first targeted edit
retry stopped before its cases because the disposable image lacked the test
helper binary; the helper has since been built. No installed state changed.

The subsequent targeted GPU run measured the owned runtime and reached the
exact-edit case. Meta-Llama requested edit access and read `source.txt`, then
returned "Now, let's replace" without calling a write tool. The service
incorrectly accepted that answer as completed; the file remained unchanged and
the independent fixture check failed. The next candidate adds a bounded
completion check for an explicit edit-access grant: a verified current-session
write must occur after that grant. Read-only observation cannot satisfy it.
This check requests correction without dispatching an action itself and remains
subject to the existing repair limit. It does not infer that arbitrary edited
content matches the user's intent; the exact-byte fixture still must pass.

Candidate5's targeted run demonstrated why grant width cannot stand in for the
agent's request: the agent requested `edit`, but the disposable test client
attached a broader `trusted` workspace. The initial check missed this and
accepted a read-only final answer. The input-resolution record now persists the
validated requested access level; the completion check uses `edit` while still
validating the current host reference. The existing read-only access test
exposed the mistake during race coverage, and focused tests passed after the
correction. Candidate6 stopped the false completion, but Meta-Llama proposed
`<`/`>` as exact-input boundaries and exhausted the repair limit before a write.
No file was changed. Its full trace is retained in
`~/.offgrid/qualification/offgrid-native-eval-20261028-001543/`.
For a single uniquely paired tag in the immutable user request, the next
candidate presents that opening and closing marker as exact schema constants;
actual source selection and independent host read-back remain mandatory.
Ambiguous or incomplete pairs receive no marker hint. This is a model hint,
not a new authorization or automatic write.

Candidate7 selected the correct `<content>` boundaries and received a host
rejection for the missing preimage revision. After reading the preimage it kept
reading the same file and proposing completion instead of submitting the
corrected write. The saved task stopped at the repair limit with no file change;
see `~/.offgrid/qualification/offgrid-native-eval-20261028-002201/`. The next
candidate narrows the pending repair's model menu to the already chosen exact
write with its observed revision and original selectors. It still requires a
fresh model call, ordinary authority checks, the host's revision comparison,
and an independent write receipt. The Linux Go full suite and affected race
packages passed before this final menu refinement; focused agent/server tests
passed after it. Broad model qualification is still pending.

Candidate8's targeted edit produced a complete typed write for `source.txt`
after reading the current preimage. The evaluator independently checked exact
Unicode bytes, the expected digest, one verified write and unchanged neighbor
files; run `run-d4c65693635c3919f335963e19c24939` passed. The small
targeted test script subsequently failed its hardcoded 54-case total assertion;
that assertion was corrected for filtered runs. This is not a product failure,
and the isolated source/file/receipt evidence is retained at
`~/.offgrid/qualification/offgrid-native-eval-20261028-002745/`.
The full packaged candidate8 run is separate evidence in
`~/.offgrid/qualification/offgrid-native-eval-20261028-003219/`. Its three
Qwen and three Meta-Llama local-file repetitions, owned runtime measurement,
and three reviewed checklist-skill repetitions passed. Frozen conversation and
workspace cases reached the process-output check. Its receipt showed a real
`go test .` exit code 0 and output, and the answer reported that output using
literal `\\t` escape characters. The evaluator initially rejected this display
form; it was corrected to compare the escaped tab to the observed tab. The
original production service was restored healthy without a data change.

A fresh candidate8 evaluation at
`~/.offgrid/qualification/offgrid-native-eval-20261028-103632/` passed six
isolated exact host-file creations (three Qwen2.5 7B and three Meta-Llama 3.1
8B), three reviewed checklist-skill runs, and all 54 frozen conversation and
workspace cases across three repetitions. Each case recorded its own completed
run and mechanical checks; the persisted result has 54 distinct run IDs, 18
case IDs with three rows each, and no failed mechanical row. Exact Unicode
edits used one verified host write and left neighbor files unchanged; process
cases used an observed exit code and actual captured output. The final test
driver returned nonzero only because its count assertion included the separately
tested skill case in the 19-case manifest (19 × 3 instead of 18 × 3). That
assertion is corrected in the ignored evaluation helper; the 54 persisted case
results and three separate skill results were checked independently. The
production container was again restored by the driver's `finally` path and
verified healthy with its original image and ID.

For this disposable runtime, the 54 case times had a 6.209-second median and
42.077-second maximum, with 546.7 seconds of summed case time. Exact local-file
creation took 6.819–8.336 seconds on Meta-Llama and 6.96–9.795 seconds on
Qwen; model load, queue and generation are not yet separated in these numbers.
Representative answers were reviewed for relevance and false completion, but
the full suite's subjective answer-quality review, unseeded model qualification,
and native macOS/Wayland/Windows installed workflows remain unverified. The
evaluation-only capability seed was isolated and was never copied to installed
user state. These results support the scoped mechanical gate; they do not
qualify arbitrary agent work or remove the broader OpenSpec hardware gates.
