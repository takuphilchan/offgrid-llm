# Tasks

## 1. Durable intent and outcome

- [x] 1.1 Persist and validate proposed policy, bind it into idempotent submission, reject mismatched attachment before preflight/dispatch, and expose additive contracts; verify `go test ./internal/server -run 'TestTaskFirst|TestTaskAccessPolicy' -count=1` and document the API.
- [x] 1.2 Close prose-after-rejection completion, preserve bounded recovery and uncertainty, expose conservative answer-only outcomes, and document recovery; verify `go test ./internal/agents -run 'TestProposalRepair|TestTaskOutcome' -count=1` plus the exact Chrome server regression.

## 2. Current interface

- [x] 2.1 Add composer policy control and task-scoped persistence, remove repeated policy selection for new tasks, present answer-only results in all nine locales, regenerate contracts, and document host limitations; verify TypeScript/API drift and `task-first.spec.ts` / `task-access-intent.spec.ts` browser suites including reload, outside click, Escape and RTL.

## 3. Integration evidence

- [x] 3.1 Run applicable Go, Go race, web build, task browser and documentation checks plus strict OpenSpec validation; record actual results and outstanding task-wide host qualification in the existing reliability plan. No live desktop effects or deployment are implied.
