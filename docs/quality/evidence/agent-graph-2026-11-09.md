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
