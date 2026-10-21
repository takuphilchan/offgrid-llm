# Tasks

## 1. Stable service contracts

- [x] 1.1 Separate durable contract support from storage health; add an isolated storage-failure identity regression proving task operations remain blocked. Update workspace documentation and run `go test ./internal/server ./internal/agents -run 'TestAgentContract|TestPublicSystem|TestTaskActivity' -count=1`.

## 2. Single current renderer

- [x] 2.1 Extract current management panels, remove legacy work UI and exclusively referenced setup/state/CSS helpers, preserve current drafts on transient identity failures, and add nine-locale recovery copy. Verify references and `npm.cmd --prefix web/app run check`; document removed presentation versus retained APIs/data.
- [x] 2.2 Add browser/desktop-renderer tests for cold unsupported service, healthy-to-unhealthy transition, failed identity refresh, management navigation and draft preservation; migrate obsolete page tests to current contracts. Run affected Playwright suites with isolated state and confirm no legacy work DOM or submission requests.

## 3. Integration evidence

- [x] 3.1 Run full Go tests, relevant race checks, API drift, renderer production build, real-service web suite and desktop unit tests; run documentation and strict OpenSpec checks. Record actual results and remaining installed-platform/storage recovery limits in the existing reliability plan.
