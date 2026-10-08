# Tasks

## 1. Truthful feedback and recovery

- [x] 1.1 Correct application discovery/launch/consent labels and empty selections, document the interaction in docs/advanced/workspace-ui.md, and verify `npm.cmd --prefix web/app run test:e2e -- task-first.spec.ts` including no access grant during discovery and stopped/late requests.
- [x] 1.2 Separate model metadata, inventory, and mutation error recovery; retry exact failed reads without duplicate installation or stale results. Document recovery in docs/guides/model-discovery.md and verify `npm.cmd --prefix web/app run test:e2e -- workspace-feedback.spec.ts` with search/discovery failures, polling, cancellation, and changed queries.
- [x] 1.3 Implement cancellable microphone phases and localized feedback without changing capture authority. Document states in docs/guides/voice.md and verify `npm.cmd --prefix web/app run test:e2e -- voice-playback.spec.ts` with delayed readiness/permission/transcription and late media cleanup using synthetic media only.
- [x] 1.4 Distinguish Knowledge readiness/check failure/setup without ordinary-chat warnings or silent fallback, document it in docs/advanced/workspace-ui.md, and verify `npm.cmd --prefix web/app run test:e2e -- workspace-feedback.spec.ts interaction-contracts.spec.ts` including stale responses and requested Knowledge preservation.
- [x] 1.5 Replace unconditional client-local claims with localized connected-service wording and discoverable details, document the boundary in docs/advanced/workspace-ui.md, and verify `npm.cmd --prefix web/app run test:e2e -- workspace-feedback.spec.ts workspace-experience.spec.ts` in English/RTL and both themes.

## 2. Integrated verification

- [x] 2.1 Run `npm.cmd --prefix web/app run api:check`, `npm.cmd --prefix web/app run check`, `npm.cmd --prefix web/app run build`, full isolated `npm.cmd --prefix web/app run test:e2e`, `npm.cmd --prefix desktop test`, documentation checks, and `openspec validate workspace-state-recovery --strict --no-interactive`; run the applicable fresh-built real-service browser tests and record results/unrun platform checks in docs/advanced/product-reliability-plan.md. Do not deploy, download models, record a real microphone, commit, or publish.
