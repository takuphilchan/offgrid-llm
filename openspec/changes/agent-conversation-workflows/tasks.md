# Tasks

UI coordination (2026-11-14): workspace-outcome-experience section 41 owns the
explicit document-chat handoff for canonical text conversations and stale-client
upload conflicts. Canonical attachment execution remains outside that recovery
fix; legacy-write guards remain enforced. Evidence:
`docs/quality/evidence/canonical-document-handoff-2026-11-14.md`.

The 2026-11-12 implementation request approves the replacement architecture in design.md. Sections 1-7 retain historical scope/evidence; replacement acceptance is tracked below and is not satisfied by historical checkmarks.

## 8. Reproduced regression repairs

- [x] 8.1 Remove profile-name forced delegation and English keyword capability gates; test negative and multilingual intent without dispatching native input.
- [x] 8.2 Apply conversation restrictions before resource readiness; retain restrictions across profile edits and restore selected configuration on reload. Verify unavailable disabled tools cannot block conversation and restrictions cannot widen authority.
- [x] 8.3 Preserve applied corrections and failed/cancelled outcomes in subsequent context; replace all-or-nothing history cutoff and provide scoped historical retrieval. Verify large outputs and revoked sources.
- [x] 8.4 Pin and expose authorized child skills; retrieve full child results with owner/graph checks and preserve useful partial outcomes.
- [x] 8.5 Support safe-boundary steering during delegation with instruction versions, stale-result fencing and no uncertain replay.

Repair evidence and remaining architectural limitations: [2026-11-12 verification record](../../../docs/quality/evidence/conversation-first-repairs-2026-11-12.md). Configuration restoration covers versioned accepted turns; it does not claim recovery of unsubmitted browser-only choices or completion of the canonical control store.

## 9. Canonical control model

In-progress foundation and verified boundaries are recorded in the
[2026-11-13 checkpoint](../../../docs/quality/evidence/conversation-control-foundation-2026-11-13.md).
These tasks remain unchecked until their complete contracts pass; the preview
API and internal migration do not complete the client/worker/resource phases.

- [x] 9.1 Add scoped projects, conversation configuration, work/run/worker/result/change-set identities and transactional command/event/outbox storage in the existing agent database.
- [x] 9.2 Add versioned session imports, source manifests, explicit canonical ownership, compatibility projections and paginated events; fault-test retries, concurrent commands, projection failure and upgrades.
- [x] 9.3 Implement preference provenance, authority intersections and optional non-secret project configuration without grants or hooks.

Current worker/control acceptance evidence and unverified boundaries:
[2026-11-14 checks](../../../docs/quality/evidence/conversation-worker-policy-2026-11-14.md).

## 10. Conversation-first clients

- [ ] 10.1 Add default assistant admission and scoped progressive discovery without profile setup; preserve explicit legacy request semantics and share attachment/model capability checks.
- [ ] 10.2 Add responsive conversation input, work focus, background observation and shared configuration to CLI/UI; align Go/TypeScript/Python contracts and legacy adapters.
- [ ] 10.3 Verify reload, cross-client controls, corrections, accessibility/locales, voice and configuration inspection in isolated service/browser tests.

## 11. Resumable research workers

- [x] 11.1 Add durable worker sessions/mailboxes, follow-up runs, explicit user-stopped continuation and accepted/delivered/applied input states.
- [x] 11.2 Implement scoped temporary assignments, versioned dependencies, complete result paging, worker questions and partial synthesis within shared budgets.
- [x] 11.3 Verify correction/pause/Stop/completion races, revocation, duplicate delivery and restart without implicit dispatch.

## 12. Local resource scheduling

- [x] 12.1 Extend budget accounting with graph-wide allocations/settlement, input/context/tool/process/storage costs and retained parent verification capacity.
- [ ] 12.2 Add foreground/background fairness, cancellation-safe queue admission and measured installed-model selection without weakening residency/memory safety.
- [ ] 12.3 Verify no budget reopening/starvation, unmeasured memory handling and matching-hardware single-agent performance.

## 13. Execution workers and integration

- [ ] 13.1 Add independently admitted worker sessions, bounded private working copies, input/environment manifests and process ownership without transferring grants.
- [x] 13.2 Implement sealed change sets, combined validation, dependency candidates, conflict handling and per-file integration/quarantine receipts.
- [ ] 13.3 Add qualified displaced-content-preserving replacements with race/crash tests and unsupported-backend review fallback; never blind rollback.
- [ ] 13.4 Implement authorized background supervision while retaining client-owned grant lifetime; verify disconnect, expiry and Stop.

## 14. Qualification and rollout

- [ ] 14.1 Run full applicable Go/race, API/web/build/browser, desktop/executor, Python, docs and OpenSpec checks; record actual results and unrelated failures.
- [ ] 14.2 Run frozen model/backend/hardware and representative-user cases; qualify automatic behavior only where evidence supports it, with unrun cases explicitly unverified.
- [ ] 14.3 Reconcile public docs/evidence and admission feature flags; verify existing work remains observable/recoverable when disabled.

All tasks start unchecked. Planning validation is not implementation or test evidence. Use isolated test stores and services; never direct browser suites at the user's normal port 11611. This change does not complete the remaining `native-document-workflows` tasks. Native OS tests, training, model downloads, deployment, commits and publication are separate from the code checklist below.

## 1. Intent, completion and relevant capabilities

- [x] 1.1 Add sanitized runner/server regression fixtures for hello with Full access, verified OBS followed by an irrelevant calculator call, Settings outside the first 40 applications, and unsaved multilingual Notepad being replaced by a service artifact. Verify fixtures expose the current faults without launching native apps using `go test ./internal/agents ./internal/server` and record the baseline in `docs/advanced/product-reliability-plan.md`.
- [x] 1.2 Separate enabled desktop capability from requested host execution in job submission and workflow state; preserve message/clarification responses and acquire task access only when relevant host work is proposed. Verify zero tools, checks and access requests for conversational turns, plus unchanged grants and edit preconditions for action turns with `go test ./internal/agents ./internal/server ./internal/inference`.
- [x] 1.3 Replace desktop system-prompt overwrites/string substitutions with composable versioned protocol sections; retain configurable instruction space and explicit finish behavior after verified outcomes. Verify conversation, mixed requests, partial completion and unresolved-action rejection across native feature variants with `go test ./internal/agents ./internal/server ./internal/inference`; document behavior in `docs/guides/agents.md`.
- [x] 1.4 Add bounded full-catalog application lookup and explicit requested-application matching with host-issued identities, ambiguity handling and catalog invalidation. Verify Settings beyond entry 40, localized names, ambiguous matches and no Access substitution with `go test ./internal/server ./internal/computer`.
- [x] 1.5 Add authorized relevance-based capability discovery and structured execution location while preserving MCP/native composition; bound schema/result context including the private inference envelope. Verify revoked tools stay undiscoverable, long MCP results retain provenance, 8192-token fixtures retain task constraints and host documents cannot be satisfied by service artifacts with `go test ./internal/agents ./internal/server ./internal/inference`; update `docs/advanced/ARCHITECTURE.md` and `docs/advanced/PERFORMANCE.md`.

## 2. Effects and recovery

- [x] 2.1 Classify trusted argument/path/authorization rejection before any effects, including the protected service-path write case, and integrate eligible failures with bounded repair. Verify no unknown-outcome form is required, the limit is three consecutive repairs, and unrelated reads do not reset it using `go test ./internal/agents ./internal/server`.
- [x] 2.2 Preserve partial/unknown effects after directory creation, partial writes and dispatched native/remote input; add fault-injection tests showing no automatic mutation replay. Run `go test ./internal/agents ./internal/server ./internal/computer` and update effects semantics in `docs/reference/api.md`.
- [x] 2.3 Add trusted journal/read-only recovery for uncertain actions, with action-bound evidence and explicit unresolved states. Verify lost acknowledgment, unavailable target, misleading visible post-state and service disconnect with `go test ./internal/agents ./internal/server ./internal/computer`; retain explicit resume boundaries.
- [x] 2.4 Preserve human-reported reconciliation provenance and original call arguments without treating done as independent verification. Verify completed/partial/uncertain distinctions through job snapshots and APIs with `go test ./internal/agents ./internal/server`; update `docs/guides/agents.md` and `docs/guides/computer-tasks.md`.

## 3. Reusable profiles and configuration

- [x] 3.1 Add actor/workspace-owned profile and immutable revision storage with optimistic concurrency, archive behavior and additive migrations. Verify cross-actor denial, conflicting edits, retained historical revisions and upgrade/restore fixtures with `go test ./internal/agents ./internal/storage ./internal/server`.
- [x] 3.2 Add authenticated profile CRUD/effective-configuration APIs with installed-model, Knowledge and connection-reference validation; pin the effective revision at admission. Verify no silent downloads, no credentials in responses, missing-resource explanations and mid-run edits with `go test ./internal/server ./internal/agents`; document contracts in `docs/reference/api.md`.
- [x] 3.3 Apply profile/conversation restrictions to discovery and dispatch using provider/tool/schema/location/policy identity; invalidate affected approvals on identity changes. Verify revoked connections, changed schemas, forged references, instruction injection and connection-use versus administration permissions with `go test ./internal/agents ./internal/capabilities ./internal/server`.
- [x] 3.4 Extend existing read-only delegation with pinned child profiles, capability subsets, cycle/depth/count and aggregate budget limits. Verify children cannot obtain desktop input, broader permissions or independent narration, including a dependent research-to-document-draft handoff, using `go test ./internal/agents ./internal/server`; update `docs/advanced/ARCHITECTURE.md` and `docs/guides/agents.md`.

## 4. Conversation and job coordination

- [x] 4.1 Add optional agent/profile/resource selection and canonical-job origin to session-turn and job contracts. Generate types with `npm.cmd --prefix web/app run api:generate`, verify `npm.cmd --prefix web/app run api:check`, and test old payload compatibility using `go test ./internal/server ./internal/sessions`; update `docs/reference/api.md`.
- [x] 4.2 Implement persisted turn intent, unique request/digest lookup, held job admission, durable association and idempotent release. Fault-inject before and after every persistence boundary, including concurrent same/different-payload submissions, with `go test ./internal/sessions ./internal/agents ./internal/server`; assert no unlinked dispatch or duplicate job.
- [x] 4.3 Project canonical job events into replayable conversation activity and persist final output once. Verify final-write failure recovery attaches the saved result without inference/tool replay, reconnect is observation-only and unauthorized event access is denied with `go test ./internal/sessions ./internal/server ./internal/agents`.
- [x] 4.4 Preserve bounded conversation history, authorized Knowledge, selected model, pinned profile and safe-boundary steering for tool-enabled turns; use the same path for voice-originated text. Verify existing text-only Chat and retrieval errors remain compatible with `go test ./internal/server ./internal/sessions ./internal/agents`.
- [x] 4.5 Implement active-work conversation deletion choices, content-minimal deduplication retention and restart bookkeeping without implicit resume. Verify stop confirmation, retained inspector access, deletion/retry races and interrupted/uncertain jobs using `go test ./internal/sessions ./internal/server ./internal/agents`; update `docs/guides/history-management.md` and `docs/advanced/workspace-recovery.md`.

## 5. Current UI and interaction design

- [x] 5.1 Build the profile library/editor within the current Agents route, with model, instructions, Knowledge, authorized connections/tools and advanced settings. Verify create/edit/duplicate/archive, conflict and unavailable-resource states in an isolated browser fixture; run `npm.cmd --prefix web/app run check` and update `docs/advanced/workspace-ui.md`.
- [x] 5.2 Add the compact Chat agent selector and shared Tools controls using current design tokens; preserve model, voice, drafts and Knowledge controls. Verify selection affects future turns, per-actor/workspace state is isolated, and outside click/Escape restore focus without submission in the browser suite; update `docs/guides/agents.md`.
- [x] 5.3 Render linked work as conversation messages, compact inline activity, result attachments and collapsed technical details. Cover waiting, running, approval, unavailable, failed, partial, uncertain and completed states; verify no duplicate full-prompt heading or normal completion textarea in browser assertions and inspected screenshots at desktop and narrow/short sizes.
- [x] 5.4 Retain canonical work inspection and old job links as secondary routes; keep shared-shell Pause/Take over/Stop visible across navigation. Verify navigation/reload creates no submit/approval/replay, stopping remains pending until acknowledged, and the retired agent renderer cannot reappear in browser/desktop tests; update `docs/advanced/client-contracts.md`.
- [x] 5.5 Complete all nine locale strings and verify both themes, RTL, IME, keyboard navigation, screen-reader status behavior, popover scrolling and parent-only speech activity. Run `npm.cmd --prefix web/app run check`, `npm.cmd --prefix web/app run build`, `npm.cmd --prefix desktop test`, and the isolated accessibility/voice browser scenarios; record actual visual evidence in the reliability plan.

## 6. CLI and Python parity

- [x] 6.1 Add service-backed `offgrid agent profiles` commands and profile selection to existing run/conversation commands, preserving current flags and legacy adapters. Verify interactive and non-interactive pending-access, uncertainty, conflict, streaming and cancellation behavior with `go test ./cmd/offgrid ./internal/serviceclient`; update `docs/reference/cli.md` and `docs/advanced/cli-experience.md`.
- [x] 6.2 Add Python profile and linked-turn methods and generated web contracts without client-local execution. Run `python -m pytest python/tests/test_client.py` and `npm.cmd --prefix web/app run api:check`; document request identity and recovery in `docs/advanced/client-contracts.md`.
- [x] 6.3 Add cross-client fixtures that create work in one client and inspect/steer/stop it in another, asserting a single canonical identity and actor isolation. Run `go test ./internal/server ./internal/serviceclient ./cmd/offgrid` plus the isolated browser/SDK integration fixture; record any unsupported old-server capability explicitly.

## 7. Integrated acceptance and evidence

- [x] 7.1 Run `go test ./...` and the CI race coverage: `go test -race ./internal/sessions ./internal/rag ./internal/server ./internal/users ./internal/cache ./internal/agents ./internal/runs ./internal/storage ./internal/serviceclient ./internal/models ./internal/audio ./internal/computer ./internal/inference` and `go test -race ./cmd/offgrid-computer`. Record command results and environment-specific omissions; do not count unavailable race/native environments as passing.
- [x] 7.2 Build `go build -o bin/offgrid.exe ./cmd/offgrid` and run `node dev/scripts/test-web-workspace.mjs bin/offgrid.exe --workers=1`, `npm.cmd --prefix desktop test` and `npm.cmd --prefix computer test`. Verify the new profile-to-Chat journey plus existing typed Chat, Knowledge, voice, models, history and legacy-link behavior against disposable services only. On 2026-10-15 the migrated full browser suite passed 348 tests with 10 opt-in checks skipped (both one-worker and four-worker runs); desktop passed 65/65 and companion passed 73/73. The seven model-acquisition/restart opt-in checks also passed separately against the fixture-tagged disposable service. Installed-task UI and seeded task-service opt-ins remain outside this fixture run; native/model qualification remains 7.3.
- [ ] 7.3 Run the frozen greeting, OBS, full-catalog Settings, mixed conversation/action, MCP-plus-native, exact multilingual document and recovery cases against an already installed tool-calling model with a controlled host fixture, three repetitions each. Record failures and actual call counts, context consumption, queue/model/tool/UI latency separately; record real native Windows/macOS/Linux qualification as unverified when hardware or disposable accounts are unavailable. Do not label synthetic results as OS completion.
- [x] 7.5 Persist structured, bounded child outcomes with evidence references and uncertainty; schedule dependency-ready branches within admitted runner capacity, retain independent results after branch failure, and block descendants without replay. Verify dependency cycles, budget exhaustion, restart, cancellation and partial outcomes with `go test ./internal/agents ./internal/server`.
- [x] 7.6 Show canonical child state and live progress in Chat and CLI, with reduced-motion and noninteractive fallbacks. Verify running, queued, blocked, failed and completed states, reconnect and narrow layout using the isolated web suite and CLI tests; do not claim simultaneous inference without hardware qualification.
- [x] 7.4 Reconcile implementation evidence and limitations in `docs/advanced/product-reliability-plan.md`, validate with `openspec validate agent-conversation-workflows --strict --no-interactive`, `node --test dev/scripts/check-docs.test.mjs`, `node dev/scripts/check-docs.mjs` and `git diff --check`. Confirm all acceptance scenarios map to recorded checks, unfinished dependent native tasks remain visible, and no deployment or release readiness is inferred solely from checked tasks. The 2026-10-15 remaining-work audit records the six acceptance mappings, passing checks, skipped fixtures and unresolved model/native/usability gates.
