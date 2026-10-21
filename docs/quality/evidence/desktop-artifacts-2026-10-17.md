## Follow-up: remembered Full access in Chat (2026-10-17)

The native host already reuses session consent, but Chat's live-submission
handoff recognized only `desktop` input. Model-generated requests for installed
applications use `app`, leaving a new desktop-scoped task at an unnecessary
Allow desktop access button even with a valid Full access session.

- Reproduced with a browser regression: zero input attachments before the fix.
  Chat now accepts both request forms for a live, explicitly submitted,
  desktop-scoped task. The existing host-owned grant path performs attachment
  once, with zero new enable/consent calls while Full access remains valid.
- Saved-task navigation, selected-window/managed-browser requests, and recovery
  of a previous session do not automatically attach or widen authority. Stop,
  revocation, actor/workspace checks and per-task grants are unchanged.
- Windows checks passed: 41 Playwright tests in `agent-chat.spec.ts` and
  `task-access-intent.spec.ts`; all 71 desktop tests; 32 native/desktop companion
  contract tests; renderer API drift check, TypeScript and production build.
  The build retains its existing large-chunk warning. These are isolated mocked
  host/transport checks, not a real-model desktop workflow qualification.
- No native consent dialog was opened for these checks. No deployment, desktop
  replacement, production-task resume or Git publication was performed.

### Authorized Full access repair deployment (2026-10-17)

- Subsequently deployed local image `offgrid-llm:full-access-repair-20261017`
  (digest `sha256:b5da799cbfa5c0ad915245cf41905d4eae24026333a56fdfe333b0203569709d`)
  as container `5083dd27975b`, version 0.4.18 and revision
  `full-access-repair-20261017`. One service runs on loopback port 11611.
  Existing mounts, container configuration, workspace identity, models,
  conversations, documents and task identities/statuses matched after replacement.
- Verified the stopped workspace archive at
  `/home/phil/offgrid-full-access-backup-YxLO0L`; retained the stopped previous
  container `offgrid-before-full-access-1792221140570` with automatic restart
  disabled. No saved task was resumed and no computer access was granted.
- Replaced the existing Windows application after checking it was closed.
  All 606 staged package files matched their build hashes. The previous app is
  retained at `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop.rollback-full-access-repair-20261017`.
  Desktop-state backups are under `build/desktop-backups/before-full-access-repair-20261017`;
  184 profile/journal/ownership files were hash-verified. Live host journals and
  preferences were not restored or reset. The original uninstaller was retained.
- Windows bundled backend SHA-256 is
  `BDD35E456529C4DC777F389082675D7DA9BDCC6945D8EE2EC7AB50DE7727E2B7`.
  Pack integrity, packaged startup/recovery (`offgrid-desktop-startup-1MjGQa`)
  and installed-app connection (`offgrid-installed-harness-UYZsSU`) passed.
  The installed app uses the external container, matching renderer and bridge;
  closing the isolated desktop check leaves the container running.
- A live Qwen request returned `READY`; NVIDIA automatic offload was logged and
  RTX 5060 memory usage was 4666 MiB. The retained CPU worker version check passed.
  No model downloads or personal-desktop actions were performed.
- Re-ran Go tests for server, agents, computer, inference, models, config,
  resource and power; all passed. Race tests for server, agents, computer and
  inference passed. This deployment is local, not a Git publication or release;
  real shared-session browser research qualification remains unverified.

## Follow-up: access-request and native handoff failures (2026-10-17)

The inspected Edge tourism job failed before host dispatch: it proposed an
installed browser as a managed-browser session, then supplied an empty target.
The saved context report showed 3264/8192 tokens and no compaction; that failure
was not context exhaustion. The previous grammar repair did not resolve this
separate contract/feedback problem.

- Access schemas now encode the app/managed-browser alternatives, with precise
  authoritative field validation. Invalid requests grant nothing. Installed
  browser intent is not silently replaced by a separate managed browser.
- Inference context omits only complete service-rejected, no-effect access
  call/result pairs during access repair. Canonical history, constraints,
  corrections and dispatch evidence are preserved. A fixed access diagnostic
  no longer describes invalid access arguments as model incompatibility.
- Selected-window tools now stage observation before input and again after
  input. Small observed reference sets constrain opaque IDs in the model schema;
  large sets retain bounded discovery. These are presentation constraints,
  not replacements for host freshness, scope or consent checks.
- Installed Qwen2.5-7B-Instruct-Q4_K_M passed four initial access cases and two
  explicit-resume cases with disposable persisted rejected proposals. Each
  reached the correct access request in one generation, without host dispatch.
  A separate synthetic-observation runtime check passed exact reference/text
  selection and fresh observation after a simulated input receipt. This is
  narrower evidence than a completed browser workflow.
- Real, explicitly approved temporary signed-out Edge tests reached local
  consent and native observation, but did **not** complete the three-source
  research outcome. They exposed label-as-input, opaque-reference copying and
  repeated-input proposals. A broad-tool diagnostic additionally chose raw HTTP
  search HTML and exhausted its small context; it is not proof of equivalent
  browser execution. Full research, source quality and shared-session real-model
  qualification remain unverified. No failed production job was resumed.
- Repeated dialogs during this investigation came from separate isolated
  selected-window test workers. Those runs were stopped at the user's request.
  The normal Full access path retains its session-owned permission: 24 desktop
  runtime tests and 32 companion tests passed, including one owned host reused
  across tasks without another enable prompt. No consent bypass, durable grant
  storage or new product dialog was added.
- Go server tests and race tests for server, agents, inference and computer
  passed. Deployment, desktop replacement and publication were not performed.

## Follow-up: artifact grammar regression (2026-10-17)

The deployed artifact filename pattern contained literal control characters.
The Go schema validator accepted it, but the installed llama.cpp schema-to-grammar
converter produced an invalid grammar. Two interrupted jobs stopped at iteration
zero with no steps or pending dispatch. This was a service-authored schema defect,
not context exhaustion or evidence that the model cannot use tools. The earlier
plain-text deployment smoke check did not exercise this path and missed the defect.

- Reproduced the actual HTTP 400 (`failed to parse grammar`) using the production
  artifact schema and single-step adapter against the idle installed runtime.
- Replaced the pattern with a printable runtime-compatible form. Authoritative
  validation still rejects control characters, paths, invalid formats and byte
  limit violations; filename suffix and nonempty-content constraints remain.
- Added a durable, fixed `invalid_tool_grammar` diagnostic without raw schemas or
  private runtime details. It does not enter model repair or retry automatically;
  unknown effects retain their stronger reconciliation state.
- The opt-in `TestArtifactInstalledRuntimeSchema` passed nine checks with installed
  Qwen2.5-7B-Instruct-Q4_K_M: four formats in streaming and non-streaming modes,
  exact multilingual text, and ordinary conversation with the tool available but
  unused. These were synthetic generations, not tool dispatch or native workflows.
  Run against an explicitly idle local runtime using
  `OFFGRID_TEST_SINGLE_STEP_RUNTIME` and `OFFGRID_TEST_RUNTIME_MODEL`.
- Regression fixtures cover literal controls, authoritative control rejection,
  private-safe HTTP errors, durable interruption, no automatic retry and preserved
  uncertain effects. This repair does not qualify other models or native platforms.
- Validation passed: Windows `go test ./...`; WSL Go 1.26.6
  `go test -race ./internal/agents ./internal/inference ./internal/server
  ./internal/computer`; `npm --prefix web/app run api:check`; maintained-document
  checks; `git diff --check`; and strict validation of `model-adaptive-execution`.
  UI, packaging and native end-to-end suites were not rerun for this backend-only
  repair. The live schema tests above supplement, rather than replace, those gates.
- No installed service/app replacement, live task resumption, commit or publication
  is included. The failed jobs remain untouched; deployment is a separate operation.

### Authorized grammar repair deployment and desktop replacement (2026-10-17)

- On subsequent explicit approval, deployed local image
  `offgrid-llm:grammar-repair-20261017` as container `a0d55369291e` and installed the
  matching Windows desktop/bundled backend (version 0.4.18, revision
  `grammar-repair-20261017`). This is a local replacement, not a published release.
- Verified the stopped workspace backup at
  `/home/phil/offgrid-grammar-backup-L2yxvo`; preserved the previous stopped container
  `offgrid-before-grammar-1792217596921`. Models, documents, session/task identities
  and statuses, workspace identity, mounts and runtime configuration matched after
  replacement. Only one OffGrid container runs, published on loopback port 11611.
- Preserved the previous desktop in
  `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop.rollback-grammar-repair-20261017`
  and verified private desktop-state backups under
  `build/desktop-backups/before-grammar-repair-20261017`. Installed and built bundled
  backend SHA-256 matched: `AF31AAF1CF015E621BBD1DBFC15456E6911E1F345EFE94FE89E1AD7025140FD0`.
- TypeScript/build and 71 desktop tests passed. Packaged startup/recovery passed
  (`offgrid-desktop-startup-3bceRS`); the installed read-only connection harness
  passed (`offgrid-installed-harness-5TfcNP`), with external service ownership and
  matching UI/bridge. Closing the test desktop left the container running.
- Added and passed four `TestArtifactDeployedServiceSchema` checks through the
  deployed public chat endpoint, using ordinary tools and the production artifact
  schema, without dispatch. An initial diagnostic incorrectly sent the runtime's
  private `response_format` to that endpoint and was correctly rejected; this was
  corrected in the test harness, not by weakening the public API. Nine direct
  runtime checks also passed again, including streaming and tool abstention.
- The deployed Qwen model generated with automatic NVIDIA offload; RTX 5060 GPU
  memory was about 4.7 GiB. The retained CPU worker's version check passed. No new
  models were downloaded, failed jobs resumed, native actions dispatched, or Git
  publication performed. Full native workflow/platform qualification remains open.

## Follow-up: artifact argument repair and pending application work (2026-10-17)

The later failed run proposed a text artifact with a filename lacking `.txt`
immediately after computer access was attached. The offered schema accepted that
shape, the service rejected it, and `invalid_task_artifact` terminal-failed the
task. No application action or artifact write occurred in that run. This is a
tool-contract/recovery defect, not evidence of context exhaustion.

- Format-specific schema branches now constrain filenames and nonempty content.
  Service validation retains byte limits, safe paths, format parsing and CSV
  formula restrictions, and returns specific reasons without private payloads.
- Pre-dispatch artifact argument failures use bounded, durable correction.
  Unrelated reads and unsupported completion claims cannot erase that repair;
  successful owned storage can. A dispatched storage failure remains uncertain
  and is not replayed. Existing stronger pre-effect obligations are preserved.
- Following a model-requested access operation, artifact saving is withheld
  until current-target inspection. Authorization also checks old queued proposals;
  a completion check prevents access plus a saved query from being accepted as
  finished application work. Preattached grants do not force desktop activity,
  and an explicit subsequent user correction can supersede the proposed operation.
- New failing-before-fix fixtures reproduced schema/validator disagreement,
  terminal argument failure and artifact-only false completion. Final Windows
  `go test ./...`, targeted artifact/observation regressions, API generation check,
  maintained-documentation check and strict OpenSpec validation passed. An earlier
  full run found the concurrently changing power/context test failing; it passed
  on the final run. This repair did not edit that subsystem.
- WSL Go 1.26.6 `go test -race ./internal/agents ./internal/server
  ./internal/inference ./internal/computer` passed on the final source. An earlier
  race invocation overlapped edits and failed compilation; it is not counted as
  passing evidence. UI, packaged-desktop and real-native suites were not rerun for
  this service/runner-only repair.
- These are synthetic-model and isolated-storage tests, not a live Edge research
  pass. The new inspection guard is a necessary prerequisite, not independent
  verification of source relevance, source count or factual claims. Installed
  model/grammar and native-browser end-to-end qualification remain unverified.
  No live task was resumed, no personal desktop was controlled, and no deployment,
  desktop replacement, commit or publication was performed. Broad OpenSpec program
  tasks remain open; this bounded repair does not complete the entire harness plan.

### Artifact repair local deployment and desktop replacement (2026-10-17)

- On subsequent explicit approval, built and deployed
  `offgrid-llm:artifact-repair-20261017` and replaced the Windows desktop with the
  matching current-worktree package. Version remains 0.4.18; this is a local
  testing build, not a published release. Service revision is
  `artifact-repair-20261017`, container `0a2083d7c5eb`, bound only to host
  `127.0.0.1:11611`. Exactly one OffGrid service container is running.
- Confirmed no active inference, model operations or computer-control sessions
  before replacement. Verified a stopped-workspace archive at
  `/home/phil/offgrid-artifact-backup-qajPmu` in WSL. Models, packages, conversations,
  documents, task IDs/statuses and workspace identity matched before/after;
  existing volume mounts, device configuration and restart policy were preserved.
  The predecessor `offgrid-before-artifact-1792216234808` remains stopped with
  restart disabled. No saved task was resumed or reconciled.
- Hash-verified desktop data/settings backups at
  `build/desktop-backups/before-artifact-repair-20261017/`. The intact previous
  installation remains at
  `C:/Users/phil/AppData/Local/Programs/OffGrid LLM Desktop.rollback-artifact-repair-20261017`.
  Existing uninstall integration and model directories were retained. The built
  and packaged Windows runtime hashes match; package hooks verified the copied
  renderer and native/browser pack. This local Windows package is unsigned.
- TypeScript/renderer build and 71 desktop unit tests passed. Packaged startup
  and recovery passed with disposable state; evidence is
  `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-startup-QMhfCt`.
  The replaced installation passed a read-only connection/bridge/UI check against
  the deployed service with an isolated desktop profile; evidence is
  `C:/Users/phil/AppData/Local/Temp/offgrid-installed-harness-asgeES`.
  Closing that test left the external service healthy and running.
- Both CPU and CUDA runtime binaries remain available. A bounded, unsaved Qwen
  completion returned exactly `OK`; the RTX 5060 reported 4666 MiB in use.
  This is not a fresh CPU benchmark or an end-to-end browser research pass.
  The initial packaging CLI output-option syntax failed before touching the
  installed app; packaging through the local builder API then passed.
  No personal desktop task, commit, push or release publication ran.

## Follow-up: observation rejection and workspace artifacts (2026-10-17)

The reported desktop task stopped after access attachment, before any recorded
application action. The old observation guard failed before persisting the model
proposal, so the exact rejected tool cannot be recovered from that historical
record. Current-code fixtures reproduced a concrete composition defect: the
offered service-owned artifact tool was incorrectly subject to desktop observation
and native expectation gates. This is a focused repair under the existing
model-adaptive execution/recovery work, not completion of the full program.

- Identity-bound service artifacts retain normal authorization, format checks
  and independent storage read-back without invented native observations/checks.
  Verified artifact-only results do not clear native expectations, unfinished
  plans or prior application actions.
- Missing-observation proposals now produce persisted, argument-redacted
  `computer_observation_required` records and bounded corrective feedback before
  dispatch. Iterations and existing repair budgets are charged; exhaustion pauses
  work. Observation or unrelated reads cannot claim the rejected edit completed.
- Regression coverage includes authorized/denied artifact policy, descriptor and
  legacy-scope boundaries, successful observe-then-act repair, false completion,
  repeated/alternating invalid decisions, restart, failed persistence and uncertain
  input. A service integration fixture exercises the real projected tool menu,
  normalized decision gate, profile, version-2 workflow and artifact store without
  connecting a native desktop.
- Windows Go 1.26.6: `go test ./...` passed. Initial runs exposed an existing test
  expecting permanent failure for missing observation; it now asserts bounded
  interruption with no dispatch. The final full-suite run passed.
- WSL Linux Go 1.26.6: `go test -race ./internal/agents ./internal/server
  ./internal/inference ./internal/computer` passed after the assertion update.
- `npm --prefix web/app run api:check`, `node dev/scripts/check-docs.mjs` and
  `git diff --check` passed. No API schema or renderer changes were required.
- No real-model/native workflow rerun, deployment, desktop replacement, commit or
  publication was performed. Installed-app and hardware qualification remains
  unverified; these fixtures do not establish that the tourism task completes.

## Authorized observation-repair deployment (2026-10-17)

After the separate deployment request and user-confirmed desktop quit, deployed
local build `observation-repair-20261017` (version 0.4.18) and replaced the installed
Windows desktop with its matching package. The existing Docker configuration,
loopback binding, GPU device request, data/model volumes and workspace identity
were preserved. A stopped-workspace backup was verified before replacement;
the old container remains stopped with restart disabled, and the old desktop and
verified desktop data/settings backup were retained. No failed task was resumed.

The initial deployment verifier stopped the replacement on an order-only mount
array difference. Comparing full mount records by destination confirmed identical
mounts; the same replacement was restarted and all verification checks passed.
Before/after model, conversation, task/status and document inventories matched.
Only the replacement OffGrid container runs. The CUDA runtime detects RTX 5060;
the CPU runtime version check passes. Qwen's unsaved chat-completion smoke returned
exactly `OK`, with automatic NVIDIA offload selected in service logs.

Renderer TypeScript/build and all 71 desktop tests passed. Packaged Windows
startup/recovery passed (`offgrid-desktop-startup-CjudJv`), and the replaced
installation passed the external ownership/bridge/UI connection check
(`offgrid-installed-harness-7CjijN`). The final ordinary app launch was blocked by
execution policy, so the user was asked to reopen it from the installed shortcut.
These deployment checks do not qualify the real-model Edge research workflow,
CPU inference performance or other platforms. No Git publication was performed.

## Follow-up: external desktop ownership across restart (2026-10-17)

The reported empty Models page reached a healthy bundled Windows backend rather
than the established Docker workspace. `DesktopRuntime.externalAttached` had
only session lifetime; an offline endpoint on relaunch therefore spawned a new
local backend. After the user quit that app, the Docker workspace remained
healthy inside WSL but initially unreachable from Windows localhost. Restarting
the idle existing container restored forwarding. Its original workspace identity
and four-model inventory were confirmed; no data/model migration or download ran.

This is a regression repair under `desktop-service-compatibility`'s external
ownership requirement, not completion of the separate harness program:

- Added bounded, versioned, endpoint/profile-scoped external ownership records.
  Relaunch/retry cannot spawn a local replacement for a remembered external
  endpoint. Invalid/read-failed/write-failed records fail closed. Explicit
  isolated recovery does not overwrite the default endpoint's record; first-use
  local startup still works. No grants, credentials or workspace contents are
  persisted in this record; every attachment still runs the live handshake.
- `npm --prefix desktop test`: 71 passed. Added fixtures cover persistence and
  endpoint/profile separation, corrupt/oversized/unknown records, failed writes,
  relaunch while offline, cancellation, isolated recovery and first-use startup.
- Packaged Windows startup test passed, including a real app relaunch with the
  external fixture stopped, Retry without a replacement listener, and reconnect
  to the returning fixture. Evidence: `offgrid-desktop-startup-F5wT2J` in the local
  temporary directory. First window 1,497 ms / recovery 1,700 ms are cold-start
  timings, not a claim of passing the 250 ms interaction-feedback gate.
- Packaged access test passed at 100%/200% zoom without granting consent
  (`offgrid-desktop-access-PYnHJ5`). Syntax, maintained-document checks and
  `openspec validate desktop-service-compatibility --strict --no-interactive`
  passed. No Go/service API or native action code changed in this repair; broader
  service, macOS/Linux, installer and native-workflow suites were not rerun.
- Installed the corrected unsigned 0.4.18 Windows app after hash-verified staging
  and data/settings backup (`build/desktop-backups/before-backend-affinity-20261017`).
  The prior application is retained at
  `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop.rollback-backend-affinity-20261017`.
  Installed external-attachment validation passed (`offgrid-installed-harness-88UW9W`).
  Normal app relaunch created the ownership record, retained the Docker workspace
  and four-model inventory, and left zero Windows `offgrid` backend processes.
  One Docker service remains running; no saved task was replayed. The unintended
  local workspace was preserved, not deleted or merged. No commit/publication.
