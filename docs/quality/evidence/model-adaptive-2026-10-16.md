## Follow-up: model-adaptive execution (2026-10-16)

### Authorized local replacement (2026-10-17)

- Deployed the current uncommitted 0.4.18 worktree as
  `offgrid-llm:model-adaptive-local-20261017`, container `58794539e4a1`.
  The prior container remains stopped with restart disabled. Only the replacement
  OffGrid service is running, on the existing loopback port 11611.
- Preserved the existing container configuration, GPU device assignment, data and
  model mounts. Verified the stopped workspace backup with the previous binary
  before activation; private backup: `/home/phil/offgrid-harness-backup-biQspn`.
  Before/after workspace identity, conversation/model/document inventories and
  every saved task's identity/status matched. No paused or uncertain work resumed.
- Replaced the installed Windows desktop with the freshly packaged 0.4.18 build.
  Verified staged files and retained the entire previous installation at
  `C:\Users\phil\AppData\Local\Programs\OffGrid LLM Desktop.rollback-harness-20261017`.
  Hash-verified local data/settings backup:
  `build/desktop-backups/before-harness-20261017` (private, ignored).
- Packaged startup/recovery and access-dialog tests passed, including 100%/200%
  zoom without granting consent. The installed-app check passed with an isolated
  profile, external-service ownership, bridge protocol 1 and matching UI identity
  `342480b5bf5019c7ebb9b1345755d661964bc969dcb635dceb5de0c5216b68c1`.
  Closing that test app left the external container healthy.
- The deployed Qwen2.5-7B-Instruct-Q4_K_M returned the exact synthetic response
  `OffGrid ready` (36 prompt / 4 completion tokens). Its native two-step tool
  check passed with 1 ms admission and 2,818 ms probe time, without desktop input.
  NVIDIA RTX 5060 detection and automatic GPU offload configuration were confirmed.
  The separate CPU runtime remains present and executable; CPU generation was
  not remeasured in this deployment pass.
- This is an unsigned local testing deployment, not a published release or
  completion of the harness program. Real native workflows, full runtime
  qualification and the remaining implementation tasks remain unverified or
  unfinished as recorded below. No commit, push or publication was performed.

The approved harness program is being tracked in `model-adaptive-execution` and
its dependent conversation/native changes. This is development evidence, not a
deployment or qualification claim.

- Frozen evaluation definition: `internal/agents/testdata/workflows-v1.json` has
  thirty cases (five each for conversation, browser, document, spreadsheet, files
  and collaboration), three repetitions and a 90% qualification threshold.
  Canonical JSON SHA-256:
  `ae3b4a2d00db4946daf589112dd9492afcebda319664ec99478df8f87e0e3e89`.
  Each case names setup, required evidence, forbidden effects and native-isolation
  scope. Changes require a new suite identity; results must record that identity.
- Windows source baseline, before runtime edits:
  `go test ./internal/agents ./internal/inference ./internal/server -count=1`
  passed (agents 69.982s, inference 12.087s, server 91.914s). These are isolated
  regression fixtures, not thirty completed real-model workflows.
- `go test ./internal/agents -run TestWorkflowEvaluationManifest -count=1`
  passed. This proves the evaluation definition's structure/digest only.
- Shared synthetic-check tests passed with
  `go test ./internal/server -run 'TestComputerModel(Check|Probe)|TestComputerSubmissionBlocksUnsupportedModel|TestSavedTaskPreflightFailure' -count=1`.
  They cover independent observer cancellation, last-observer cancellation,
  shutdown, late completion versus replacement, result-pointer isolation and
  fresh checks after completion. Existing admission/probe deadlines and held-task
  no-dispatch regressions remain covered.
- Real-model baseline, real native platform runs, performance comparisons and
  workflow qualification have not run for this program. No personal application
  was controlled and no installed service/package was replaced.
- API generation/check and TypeScript check passed after adding optional shared
  check admission/probe timings. Response fixtures cover completed negative
  checks and retryable runtime failures, retaining legacy result/error fields.
- Windows race testing could not start because CGO is disabled and no GCC is on
  its PATH. The existing WSL Go/GCC environment ran
  `go test -race ./internal/server ./internal/inference` successfully (server
  122.347s, inference 8.022s). This is Linux race evidence, not Windows native
  or real-model qualification.
- After adding shutdown draining and the evidence-store foundation,
  `go test ./...` passed (agents 77.349s, server 102.153s, models 11.747s).
  WSL `go test -race ./internal/server ./internal/inference ./internal/models`
  passed (server 153.786s, inference cached, models 2.752s).
- The versioned capability evidence store has restart, idempotence, concurrent
  writes, failed persistence, corrupt/unknown-version and identity invalidation
  coverage. `go test ./internal/models -run 'TestCapability' -count=1` passed
  (6.552s), including actual changed-file bytes with unchanged size/timestamp.
  Measurements hash an authorized open handle without altering its offset;
  callers must retain the installation/lifecycle lease throughout evaluation.
  Records contain structured identities/categories, not user prompts or raw
  inference output. Operational unavailability is not a failed measurement.
  Synthetic passes cannot imply workflow qualification; qualification additionally
  binds the suite/platform and evaluator evidence with the agreed numeric gates.
  The store is not yet wired into public readiness: trusted runtime identity,
  evaluator artifact validation and service/client integration remain unfinished.
  A configured GPU flag is not measured backend evidence. No persisted record
  substitutes for execution authorization or the fresh runtime check.
- API contract generation/check, TypeScript, web build and maintained-document
  checks passed. The web build retains its existing large-chunk warning.
- Native and constrained tool-enabled responses now share complete-decision and
  argument-schema validation before the runner accepts them. Fixtures cover
  unknown tools, unfinished/ambiguous JSON, duplicate keys, malformed Unicode,
  local versus external schema references, exact integer preservation and
  precision-loss rejection. Original dispatch arguments are not rewritten by
  schema validation. The existing computer whole-batch repair remains intact.
  Service-authored invalid decisions get persisted bounded correction, consuming
  iterations; provider-contract, network and persistence failures do not become
  model-repair retries. Counter restart, corrected conversation and zero-dispatch
  regression fixtures pass. This does not implement profile fallback routing.
- Follow-up verification on 2026-10-17: `go test ./...` passed (agents 23.866s,
  inference 16.561s, server 35.401s). WSL race checks for agents, computer,
  inference, server and models passed (19.581s, 3.368s, 5.446s, 54.332s, 1.621s).
  Focused gate/recovery checks were rerun after validation refinements.
- `npm --prefix desktop test`: 65 passed; `npm --prefix computer test`: 73 passed.
  These include isolated Chromium fixtures and native transport mocks, not real
  personal-desktop execution. Windows Python resolves to the unavailable Store
  launcher; WSL `PYTHONPATH=. python3 -m pytest tests/test_client.py -q` from
  `python/` passed 36 isolated client tests.
- The isolated built-service renderer wrapper ran `agent-chat.spec.ts`,
  `agent-streaming.spec.ts` and `agent-profiles.spec.ts`. The first run had
  33 passes and one geometry assertion failure: separate button-position reads
  could observe different transcript scroll offsets. The assertion now samples
  all action geometry in one browser turn, retaining the same alignment/spacing
  requirements. The targeted test passed five repetitions; the complete selected
  set then passed 34/34 in 26.4s. These suites use mocked agent/model responses;
  they do not qualify actual model workflows. No renderer styling was changed.
- Final affected-package rerun passed (inference 14.502s, agents 22.035s, server
  33.746s, models 5.348s); final WSL race rerun passed for agents/computer/
  inference/server/models (19.517s/cached/5.352s/54.188s/cached). OpenSpec strict
  validation, maintained-doc checks and `git diff --check` passed. The remaining
  runtime fingerprint integration, profile policy/switching, aggregate scheduling,
  dependent collaboration/native work and real qualification are not complete.
