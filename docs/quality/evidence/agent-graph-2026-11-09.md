# Agent graph execution and progress — 2026-11-09

This records source and isolated-fixture verification of the `agent-conversation-workflows` 7.5–7.6 extension. It does not qualify a model, simultaneous inference, or native desktop execution. OpenSpec task 7.3 remains open.

## Behavior checked

- Dependency-ready branches are admitted within the runner's shared capacity. A failed prerequisite blocks its descendants while an independent branch can complete.
- Parent state retains a bounded structured outcome for every branch, including status, run identity, errors, evidence step references where present, and `independently_verified: false`. Child state is persisted on parent links so a disconnected follower can show progress.
- Each child handoff persists the original request, even when a model proposes a short goal label. A three-specialist fixture checks that both analysts receive the supplied figures, the writer receives both completed analyses, and the parent receives the writer result. The old saved demo run cannot be retroactively repaired; it needs a fresh run after installation.
- CLI reports child transitions without repeating the entire parent snapshot. Chat shows persisted child state when the child endpoint is temporarily unavailable, with motion disabled under reduced-motion settings.

## Checks run

- `go test ./internal/agents -run 'TestDelegation(CarriesSuppliedFiguresThroughThreeSpecialistGraph|PersistsOriginalFactsForEveryChild|DurableDelegationDependenciesAndNarrowScope)' -count=3 -timeout 5m` — passed.
- `go test ./cmd/offgrid ./internal/agents ./internal/cli ./internal/server -count=1 -timeout 5m` — passed on 2026-11-09 after the handoff fix.
- `npm.cmd --prefix apps/web run test:e2e -- tests/work-results.spec.ts tests/agent-profiles.spec.ts --workers=1` — 55 passed after the profile and child-status UI fix.

- `go test ./...` — passed on Windows.
- `go test ./internal/agents -run 'TestDelegation' -count=3 -timeout 5m` — passed.
- `go test ./cmd/offgrid ./internal/agents ./internal/cli ./internal/server -count=1 -timeout 5m` — passed.
- `npm.cmd --prefix apps/web run check` — passed.
- `npm.cmd --prefix apps/web run api:check` — passed.
- `npm.cmd --prefix apps/web run build` — passed; Vite reported a large-chunk advisory.
- `npm.cmd --prefix apps/web run test:e2e -- tests/work-results.spec.ts tests/agent-chat.spec.ts --workers=1` — 103 passed with the progress fixture.
- `npm.cmd --prefix apps/web run test:e2e -- tests/work-results.spec.ts -g 'persisted child progress' --workers=1` — passed with the added fixture.
- `node scripts/maintenance/check-docs.test.mjs` — 7 passed.
- `node scripts/maintenance/check-docs.mjs` — 337 maintained Markdown files checked, 0 problems.
- `openspec validate agent-conversation-workflows --strict --no-interactive` — passed.

`go test -race ./internal/agents -count=1 -timeout 5m` could not run in this Windows environment because CGO is disabled and no GCC executable is available. Real-model and real-hardware qualification, including actual simultaneous inference throughput, remains unverified.

## Completion recovery and installed-model demo check — 2026-11-11

The interrupted supplied-figures demo exposed two separate boundaries: the parent
could propose an already completed graph again, and a successful generation could
omit the requested numerical rates. Completed identical graphs now reuse saved
results without creating children again. Rejected delegation proposals before
effects are returned as corrective tool results; uncertain effects, authority and
storage failures remain protected. Bounded answer-only correction checks the
supported supplied-figures pattern and requested briefing length, with no tools
available during correction. This is not a general semantic quality verifier.

The CLI previously rendered child transitions only while the parent was waiting
for children. The writer's final completion could therefore disappear when it
arrived with a running/completed parent snapshot. It is now rendered once before
the answer, including snapshot recovery.

Checks:

- `go test ./...` passed on Windows before the final CLI presentation patch.
- `go test ./cmd/offgrid -count=1` passed after that patch.
- `go test ./internal/agents -run 'TestCompletedGraphReuseThenAnswerRepairSurvivesReload|TestLateSuccessfulModelResultCannotCrossStop' -count=3 -timeout=3m` passed. The completion fixture checks durable reload, exactly three children, both dependency results, and no replay. The Stop fixture allows ten seconds for Windows startup/persistence contention; its cancellation assertion is unchanged.
- `go test ./internal/server -run '^TestSuppliedFiguresCollaborationInstalledModel$' -count=1` passed with the opt-in model test skipped when runtime environment variables are absent.
- Opt-in `TestSuppliedFiguresCollaborationInstalledModel` passed three repetitions against the installed NVIDIA Nemotron 3.5 Bolt E4B UD Q4_K_XL GGUF, using an isolated local GPU runner, `--jinja`, one inference slot, 8,192 context tokens and a 2,048-token response allowance. Each repetition persisted exactly three completed specialist jobs and a completed root, with the writer depending on both analysts, no external tool dispatch, all four expected rates, and 135–165 words for the requested 150-word briefing. Durations were 90.88, 89.07 and 90.01 seconds; answers contained 136, 139 and 141 words respectively. Inference was serialized, not simultaneous.

Reproduction uses a separately started local runtime and an isolated test store:
set `OFFGRID_TEST_COMPLETION_URL` to its `/v1/chat/completions` endpoint,
`OFFGRID_TEST_CONTEXT_MODEL` to the installed model ID and
`OFFGRID_TEST_DIRECT_RUNTIME=1`, then run the opt-in test with a 20-minute test
timeout. No models are downloaded and no personal workspace authority is passed.

Earlier fixture attempts with a 1,024-token response allowance failed: one
exhausted proposal repairs and another finished the children but returned a
134-word answer after correction. Those failures remain in local build logs.
The fixture was aligned with the production 2,048-token allowance, rather than
weakening the completion assertions. A separate preliminary three-repeat run
without explicit production runner flags also passed; the production-flags run
above is the relevant reported result.

These are finite real-model demo checks, not completion of OpenSpec task 7.3 or
the full frozen workflow suite. The length gate permits ten percent variance;
it does not guarantee exactly 150 words, correct attribution in every answer,
or absence of unsupported interpretation. macOS/Linux, race coverage and broader
native/human qualification remain unverified here. Existing failed user runs
were not resumed or rewritten.

## Combined local installation — 2026-11-11

At the user's explicit request, the installed build includes both the agent
completion fixes and the other thread's current Settings/theme/language changes.
A fixed build checkout captured 73 changed/untracked source files with SHA-256
hashes; the shared checkout remained untouched and matched the snapshot before
installation. The combined installer is in
`D:/offgrid-agent-install-20261111/build/package-combined/`.

- Web production build passed (existing large-chunk advisory).
- Desktop tests: 193 passed, one platform skip.
- Targeted work-results, settings-navigation and workspace-shell browser checks:
  100 passed. The development server reported font access warnings caused by the
  dependency junction; this run is functional evidence, not font-rendering proof.
- Packaged appearance checks passed, including 13 palette variants, 29 language
  options, restart persistence and 200% zoom. Capture inspected from
  `offgrid-desktop-theme-3N6p0a` in the local temporary directory.
- Packaged native terminal checks passed: copy/paste, Agent paste/drafts, cursor,
  tab switching, child cleanup and service identity (`offgrid-desktop-theme-tBdUeD`).
- Packaged startup/recovery passed (`offgrid-desktop-startup-ZUVsjp`).
- Silent installer returned zero. All 1,114 installed payload files matched the
  tested package. The earlier rollback backup remains at
  `D:/offgrid-backups/agent-completion-20261111-185838`.
- Saved-state comparison retained 471/475 exact file hashes. The four differences
  were window geometry and its temporary file, plus SQLite shared-memory/WAL
  files. Logical RAG comparison found only index_metadata.updated_at changed;
  all other stored rows and embedding identity were unchanged. Model inventory:
  22 files, unchanged sizes and modification timestamps.
- The installed service subsequently reported version 0.4.28 and UI fingerprint
  `86feb797b29781f7161b4571bf687a2fd0f4da42dbe3ba78d3930a6dff79557a`.

This is a combined local installation, not publication or broad qualification.
The other thread's changes remain uncommitted in the shared checkout.
