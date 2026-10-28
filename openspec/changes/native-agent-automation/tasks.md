# Tasks

## 1. Durable calendar and policy

- [x] 1.1 Add typed offline IANA calendar calculation with gap skipping, first-fold selection and bounded future-slot lookup; verify UTC, Harare, New York, Lord Howe, invalid input and backward/forward clocks with `go test ./internal/automation`; document the calendar contract in its README.
- [x] 1.2 Add owned immutable schedule revisions, pinned configurations and compare-and-swap enable/disable policy in manager-owned SQLite; test owner isolation, corruption, old data, concurrent writers and persistence failure with `go test ./internal/agents`; document policy and retention boundaries.
- [x] 1.3 Persist occurrence claims, missed ranges, job linkage and unsettled-overlap protection before dispatch; test duplicate ticks and every claim/create/link/restart boundary with Go and race checks, without replaying saved roots.

## 2. Canonical runner integration

- [x] 2.1 Recheck live actor/schedule/profile/model/context/host authority before admission and every scheduled tool dispatch; test revocation, re-enable, missing resources, no computer grant and unchanged budgets with `go test ./internal/agents ./internal/server`.
- [x] 2.2 Wire bounded occurrence coordination into the existing application-owned runner lifecycle and inference scheduler; test shutdown, waiting/settling jobs, missed startup slots and isolated unattended operations without a second executor; update docs/guides/agents.md.

## 3. Owned clients and Activity

- [x] 3.1 Add authenticated owned schedule/history APIs and CLI commands through existing service clients; run `go test ./internal/server ./internal/cli ./internal/serviceclient ./pkg/api` and web API generation/check, update docs/reference/api.md and CLI guidance.
- [x] 3.2 Add progressive Activity schedule management with explicit pinned-resource review, canonical links, nine locales and keyboard/narrow layout behavior; run TypeScript/build and focused Playwright tests, proving inspect/refresh cannot dispatch.

## 4. Integration evidence

- [x] 4.1 Run affected Go/race, API/build/browser and desktop compatibility suites, plus isolated backend scheduling fixtures; record commands, actual outcomes and unverified real-model/native qualification in the existing reliability evidence, validate this change strictly and reconcile parent task 5.2 only after client/service scenarios pass.
