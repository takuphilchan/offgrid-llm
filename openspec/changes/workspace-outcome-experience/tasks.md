# Tasks

## 22. Approved defaults-first workflow redesign

- [ ] 22.1 Freeze ordinary first-use, returning-user, document import/answer, offline/low-memory and error-recovery journeys with current action/configuration counts; capture isolated real-state screenshots and retain existing human gates.
- [ ] 22.2 Implement a single guided readiness flow using installed capabilities and explicit recommended setup/offline import; preserve working selections and verify no silent downloads, model switches or grants across reload/failure.
- [ ] 22.3 Simplify primary navigation and composer to ordinary work with advanced destinations/settings progressively disclosed; preserve all routes, active context/access, drafts and keyboard navigation. Pass localized normal/narrow/RTL/200% tests.
- [ ] 22.4 Integrate Add documents -> processing -> ready/needs attention -> Ask from document-ingestion-reliability; prove ordinary document questions require no embedding/chunk/index choices after setup, with real-service source/citation checks.
- [x] 22.5 Simplify profile creation around purpose and recommended existing capabilities, with advanced budgets/resources optional; test saved revision/permission compatibility and easy later overrides. Evidence: `docs/quality/evidence/profile-defaults-2026-11-02.md` (66 browser checks; human/platform gates remain open).
- [x] 22.6 Integrate the optional desktop-integrated-terminal entry without adding it to ordinary setup; test native capability absence, focus restoration and no automatic shell startup. Evidence: `docs/quality/evidence/workspace-readiness-2026-11-04.md` and the isolated packaged Windows terminal record; other platform qualification remains open.
- [ ] 22.7 Compare the frozen journeys for actions, exposed decisions, errors, retained drafts and first useful result; run full CI and installed-platform checks, document known gaps, and retain unperformed human/assistive-technology evaluations as unverified.

## 20. Corrective interaction and lifecycle review (2026-11-01)

- [x] 20.1 Replace oversized history utilities with compact action rows; simplify ordinary chat deletion while retaining typed active-work conflict choices, confirmation, exact snapshots and partial retry. Verify `history-controls.spec.ts`, `agent-chat.spec.ts` and narrow/RTL keyboard captures.
- [x] 20.2 Remove duplicate profile navigation, clarify workspace limits without changing permissions, and fix actual Activity list alignment and selected-view visibility. Verify profile and workspace-order fixtures plus shared control/accessibility regressions.
- [x] 20.3 Make owned desktop shutdown failure-isolated, exit-acknowledged and retryable; bound preference saving and verify no duplicate service on retry. Run desktop lifecycle tests and isolated native ownership checks without touching the user's active workspace.
- [x] 20.4 Inspect current before/after screenshots and run API/type/build, full isolated renderer, desktop, documentation/source-size and OpenSpec checks. Record exact results and remaining whole-product/manual/platform gaps; do not label this corrective subset complete product qualification.

## 19. Whole-app order and presentation follow-up (2026-11-01)

The renewed review found composition gaps despite passing earlier control tests.
These tasks implement the already approved whole-product hierarchy outcomes;
previous hardware, human and performance-baseline gates remain open.

- [x] 19.1 Align management-page leading edges and shared heading/action layouts; keep readable Chat and settings measures. Check wide, narrow, short and RTL layouts without changing routes.
- [x] 19.2 Consolidate occasional response and voice preferences while keeping text/image/microphone, active context and Stop directly available. Verify voice opt-in, focus return and draft preservation.
- [x] 19.3 Give profile identity/model and purpose a clear reading order, with explicit save/use actions accessible during configuration. Preserve drafts, revisions, validation and permissions.
- [x] 19.4 Give eligible installed models an explicit selection action and put verification/removal in labelled maintenance controls. Preserve package ineligibility and confirmation; selection must not load/download a model.
- [x] 19.5 Group Knowledge search and question actions into a library toolbar and keep selected-source actions contextual. Verify exact scope and draft handoff.
- [x] 19.6 Separate Activity's work, scheduling and diagnostic views; keep canonical inspection and filter restoration. Bound long artifact-name presentation with keyboard/touch access to the exact name and unchanged verified download.
- [x] 19.7 Put desktop startup technical metadata in its existing disclosure and retain immediate progress/recovery actions. Verify readiness, errors, cancellation and localized layouts.
- [x] 19.8 Strengthen realistic-content fixtures and density/discoverability assertions, inspect screenshots and run API/type/build, full isolated renderer, desktop, source-size and OpenSpec checks. Record exact results and remaining qualification limits in the existing evidence log.

## 18. Approved surface-material refinement (2026-10-31)

- [x] 18.1 Implement shared generated sidebar/composer/floating-material tokens, solid primary/hover fills and bounded blur with opaque accessibility/unsupported fallbacks; preserve palette, text opacity, geometry and the three-family budget. Repair nested drawer focus containment and run type/build/generator checks.
- [x] 18.2 Verify composited contrast, material on/off and reduced-transparency/contrast/forced-color states, keyboard dismissal, all routes and light/dark narrow RTL captures; run applicable renderer regressions and inspect the captures.
- [x] 18.3 Measure floating material during streaming at 4x/6x CPU throttle and rerun unchanged Stop/frame-p95 gates; record exact commands, failures and current evidence in the maintained gradient/reliability guides. Do not close outstanding real-model, platform or human qualification gates.

## 17. Approved image-input extension (2026-10-31)

- [x] 17.1 Add bounded image records and validation to ordinary durable Chat, scoped persistence, inference history and request identity; reject unsupported agents/legacy image paths and retain text compatibility. Update API/schema clients and run session/server/API tests including cross-scope access, malformed/oversized input, deduplication and restart.
- [x] 17.2 Add one shared composer attachment control with PNG/JPEG upload/paste/drop, previews/removal, scoped drafts, visible readiness/storage errors and saved images; preserve voice, keyboard, narrow/RTL and all nine locales. Run API/type/build and image/text/voice/continuity renderer tests.
- [ ] 17.3 Verify packaged desktop and web image journeys with isolated data; record actual local vision-model evidence separately from fixtures, preserve failed evidence and leave unavailable real-model/platform qualification unchecked. Update API/workspace guides and reliability evidence; no deployment or publication under these tasks.
- [x] 17.4 Repair ordinary CLI image input and add shared `/image`, `/images`, `/remove-image` handling in plain/TUI sessions; preserve repeated flags, failure retention, exact saved/reloaded bytes and explicit service vision gates. Run CLI/TUI/API regressions for invalid, oversized, missing files, failed/truncated streams and successful history; document commands and actual vision qualification limits.

## 16. Whole-product review implementation (2026-10-30)

These tasks implement the full approved review; they do not inherit completion
from earlier presentation tasks. Use current paths: `npm.cmd --prefix apps/web
run check`, `npm.cmd --prefix apps/web run api:check`, `npm.cmd --prefix apps/web
run build`, `go build -o build/workspace-outcome-experience/offgrid.exe ./cmd/offgrid`
and `node scripts/test/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe
<specs> --workers=1 --reporter=line`. Record actual evidence in the reliability
documentation; leave unavailable installed-platform/participant gates open.

- [x] 16.1 Permit navigation/new conversations during generation with origin-bound acceptance, view detachment, durable return, no replay, no cross-session output or speech; extend chat-streaming/work-continuity fixtures and workspace guidance.
- [x] 16.2 Add authenticated, bounded text/Markdown artifact previews with independent integrity validation, strict UTF-8, inert content, cancellation, revocation and unsupported/oversized fallback; extend work-results fixtures and output guidance.
- [x] 16.3 Present typed state-specific recovery and retained-result explanations; preserve exact approvals, uncertain effects and no-replay semantics; test lost acknowledgment, revoked access, partial result and resumed inspection.
- [x] 16.4 Fix measured heavy-stream responsiveness, preserve exact final output and immediate Stop feedback; pass repeated unchanged 250ms stress gates, record frame/input/bundle measurements and limitations.
- [ ] 16.5 Repair native runtime distribution dependencies and verify fresh install, first reply, restart, sleep/wake and update/recovery on Windows/macOS/Linux in isolated state; preserve hardware claims and retained user data.
- [x] 16.6 Implement purpose-led agent configuration, optional capability sections, examples and explicit use/test flow; preserve revision conflicts, dirty drafts, grants and save validation; test narrow/wide/RTL and real handoff.
- [x] 16.7 Implement searchable readable model selection with exact-ID inspection, actual readiness/capabilities and resident state where known; improve discovery decisions without invented fit, automatic download or substitution; test all categories and unavailable models.
- [x] 16.8 Simplify composer hierarchy with active context and labelled secondary controls; retain accessible Send/Stop, keyboard/IME, follow-latest, voice opt-in and explicit workspace scope; test plain and agent turns in all layouts.
- [x] 16.9 Improve history/navigation space and bounded cross-work search with truthful result scopes, authorization and preserved return state; test active work, logout, workspace switching and long histories.
- [x] 16.10 Implement document/collection question handoff with end-to-end enforced source scope and inspectable supporting passages; preserve drafts and explicit retrieval choice, test missing/revoked/unindexed sources and no unrelated-source fallback.
- [x] 16.11 Improve skill discovery, examples, review and attach/use workflow; keep revision/digest authority and provide actionable unavailable/rejected states; test real skill handoff without automatic execution.
- [x] 16.12 Add subordinate specialist summaries for actual state, dependencies/blockers and returned deliverables; retain one parent conversation and bounded authorized subscriptions; test partial/conflicting/failed branches.
- [x] 16.13 Simplify schedules with searchable time zones, readable next-run preview and optional budgets; preserve explicit resource review, disabled state and no missed-run replay; test calendar/DST/permissions and narrow layouts.
- [x] 16.14 Apply screen-level hierarchy, appropriate field widths and shared action/menu/setting representations across all routes; remove competing/repeated headings and containers while preserving palette, focus, touch targets and state ownership. The 2026-10-31 user-directed border/placement refinement uses solid/tonal/quiet roles; final renderer: 627 passed, 27 skipped; manual/native qualification remains in 16.16–16.18.
- [x] 16.15 Improve connection capability/setup feedback, voice readiness/recording/transcription, login context/recovery, permission explanations and actionable empty states; test capability absence, role restrictions, no implicit grants/capture and context-preserving return. Evidence: setup and whole-screen hierarchy sections in workspace-experience-2026-10-30.md; source/UI fixtures do not qualify real speech or native hardware.
- [ ] 16.16 Update all nine locales and maintained guides; run keyboard, screen-reader, RTL, reduced-motion, 200% zoom, short/narrow and long-label journeys; preserve missing manual evidence as open.
- [ ] 16.17 Run complete first-reply, document-grounded and reviewed-agent-output journeys with failure/cancel/navigation, plus all applicable existing CI and installed-edition checks; independently verify output and no unauthorized/duplicate effects.
- [ ] 16.18 Run the consented five-participant formative comparison required by 7.2, record completion/detours/help/recovery and iterate against its target; do not substitute screenshots or automation for users.

## Restart recovery follow-up (2026-10-30)

- [x] R1 Implement single-owner external startup grace and paced reconnect, preserving native standalone startup, compatibility, cancellation and workspace identity; pass `npm.cmd --prefix apps/desktop test`.
- [x] R2 Localize waiting in all nine desktop locales, test presentation and delayed packaged startup, and update desktop setup guidance. Record Windows evidence separately from unrun macOS/Linux/reboot qualification.

Evidence: `docs/quality/evidence/desktop-reconnection-2026-10-30.md`. The user's
container-to-native migration was verified separately in
`docs/quality/evidence/native-windows-migration-2026-10-30.md`; reconnect is not
external-service lifecycle management.

## 15. User feedback: compact controls and direct configuration (2026-10-30)

- [x] 15.1 Introduce compact fine-pointer actions, retaining larger coarse-pointer targets, keyboard focus and the existing palette.
- [x] 15.2 Use available editor width for identity/instructions and visible resource sections; move skills/collaborators out of nested Advanced disclosure without changing persistence or grants.
- [x] 15.3 Group Workspace options into compact aligned rows with stable hover/open treatment; refine the chat-history scrollbar without hiding scroll access.
- [x] 15.4 Verify creation, revision/conflict, dirty drafts, resource availability, popup keyboard/dismissal, responsive/RTL/touch layouts and scroll containment; capture affected surfaces and run applicable renderer/build checks.

Group 15: 444 renderer tests passed, 27 skipped; API/type/build, source-size,
palette and OpenSpec checks passed. Evidence is under `build/ui-craft/compact-*`.
Unchanged earlier hardware, participant and throttled-stream gates remain open.

## 14. Approved palette-preserving gradients (2026-10-30)

- [x] 14.1 Inventory existing treatments and support; document budget and capture four isolated prototypes before production wiring.
- [x] 14.2 Add centralized generated tonal/eased tokens, flat fallback, kill switch and accessibility flattening; preserve every palette token.
- [x] 14.3 Apply reviewed functional/surface/action treatments within the three-treatment budget; keep content and desktop startup flat.
- [ ] 14.4 Verify every treatment on/off, worst-point contrast, supported syntax, RTL/reflow, throttled stream/download paint and local feedback; run full applicable checks and record physical/native limits.
- [x] 14.5 Update evidence/token inventory/rejections and commit the verified gradient phases separately.

14.4 remains open for physical-display and screen-reader qualification. The
earlier three failing 4x/6x Stop samples are retained in `docs/ux/gradients.md`;
the later worker-parsing repair passed three repeated unchanged stress runs
(16.4), as recorded in the whole-product evidence. Installed-platform coverage
is still incomplete. The subsequent user request authorizes replacing the local
container and desktop for testing, with backups; it does not authorize release.

## 12. Approved color-frozen presentation refinement (2026-10-30)

- [x] 12.1 Freeze colors/source identity, capture the full before matrix and document the 12 aesthetic findings and token plan.
- [x] 12.2 Add semantic type/geometry/motion foundations; preserve colors and migrate superseded legacy geometry.
- [x] 12.3 Refine shell, sidebar/rail, header and footer within existing dimensions and focus behavior.
- [x] 12.4 Refine Chat prose, code header, composer and response actions without changing streaming or Markdown safety.
- [x] 12.5 Refine Models/Knowledge list rhythm, metadata, search and transfer presentation.
- [x] 12.6 Refine Agents/Activity/Settings headings, lists, disclosures and outcomes.
- [x] 12.7 Refine palette/popover/onboarding motion, geometry and loading with reduced-motion support.
- [x] 12.8 Refine desktop startup presentation using the frozen desktop colors and existing bridge.
- [x] 12.9 Verify color inventory, full isolated renderer, API/types/build, desktop syntax/tests, before/after matrix, docs and focused accessibility/performance checks. Record unavailable native/screen-reader checks honestly; commit each working phase.

## 13. Approved usability refinement (2026-10-30)

- [x] 13.1 Document three personas, fifteen task paths, heuristic findings/severity, baseline steps and protected-contract proposals in docs/ux.
- [x] 13.2 Add a typed nine-locale explanatory vocabulary and glossary, with native-review status.
- [x] 13.3 Improve shell/Chat/setup guidance and keyboard discoverability through existing controls and routes.
- [x] 13.4 Improve Models/Knowledge guidance, offline distinctions and empty/error next steps without automatic actions.
- [x] 13.5 Improve Agents/Activity/Settings and permission/recovery guidance without altering protected contracts.
- [x] 13.6 Verify task flows, role/offline boundaries and unchanged mutation contracts; report after step counts, remaining risks and evidence limits.

## 11. Approved composition-first refinement

- [x] 11.1 Freeze the current three representative fixtures and source/build identity; capture populated Chat, running work and blocked access before changing presentation, using disposable state.
- [x] 11.2 Implement the 248px collapsible shared shell, early New chat, compact contextual history/action menus and consolidated workspace footer. Preserve drawer/focus/scoped state and verify navigation/history tests.
- [x] 11.3 Consolidate Activity inspection and agent route aliases around canonical work presentation without broadening permissions or introducing duplicate followers. Preserve legacy read-only events, filters, deletion, steering, recovery and stopping; verify task/continuity/result tests.
- [x] 11.4 Apply explicit action hierarchy, restrained type/surfaces and progressive Chat composition at existing component owners. Review the three representative after fixtures and verify streaming, Latest, IME, notices, voice and aligned actions.
- [x] 11.5 Migrate profiles, integrations, Models, Knowledge and Settings to the shared hierarchy; remove superseded presentation rules only after consumer migration. Verify corresponding library/profile/role fixtures.
- [x] 11.6 Refine OffGrid-owned desktop startup/recovery/access dimensions and hierarchy while retaining native authority and IPC; run desktop presentation checks and record unavailable installed-platform evidence explicitly.
- [x] 11.7 Run API contracts, TypeScript/build, full isolated renderer, desktop and documentation checks; capture/review the six-page responsive/theme matrix and measure local feedback. Record actual evidence and remaining human/platform gates in the reliability plan.

Group 11.7 automated checks and all 48 responsive/theme captures were completed and reviewed on 2026-10-16, with representative populated/running/blocked comparisons. This is fixture review, not exhaustive human review of every operational state. Remaining usability/platform qualification is recorded in the reliability plan. No deployment or publication is included.

Historical checks below describe earlier scope, not completion of group 11.

## 10. Approved calmer-workspace refinement (2026-10-15)

Historical groups below record earlier work; this group tracks the newly approved scope. No deployment, installation or Git publication is included.

- [x] 10.1 Implement one 280px shell/history sidebar, contextual slot and sub-1024px drawer, simplified header and footer settings. Preserve controller identity, navigation without mutation, drawer focus and scoped state. Verify shell, history, continuity and new refinement fixtures.
- [x] 10.2 Consolidate visual tokens, product mark, action/search/notice grammar and progressive Chat composer. Preserve scroll following/manual reading, add Latest, and retain aligned response actions. Verify control, search, streaming, voice and agent Chat fixtures.
- [x] 10.3 Refine profile editing, model/document lists, Activity, Settings and result/activity hierarchy. Preserve capabilities, approval/recovery semantics and canonical work links. Verify profile, library, Knowledge, work-results and recovery fixtures.
- [x] 10.4 Refine trusted desktop startup/recovery/access presentation without changing consent or IPC. Run desktop presentation/unit checks; distinguish available tests from unrun installed-platform checks.
- [x] 10.5 Capture/review the six-page visual/state matrix, responsive/locale/accessibility fixtures and performance measurements; run `npm.cmd --prefix web/app run api:check`, `npm.cmd --prefix web/app run check`, `npm.cmd --prefix web/app run build`, the full isolated renderer wrapper, `npm.cmd --prefix desktop test` and documentation checks. Update workspace guidance and reliability evidence. Manual screen-reader, participant and unavailable installed-platform checks remain tracked separately in 6.2, 7.2 and 8.4.

Implementation order follows `design.md`. Preserve the completed shell/recovery slices and unrelated work. Each feature group owns its regression tests and documentation; later integration does not substitute for those checks.

For the commands below, rebuild after source changes:

```powershell
npm.cmd --prefix web/app run check
npm.cmd --prefix web/app run build
go build -o build/workspace-outcome-experience/offgrid.exe ./cmd/offgrid
```

`UI(<files>)` below means the exact isolated command `node dev/scripts/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe <files> --workers=1 --reporter=line`. New named suites must be created by their owning task, not silently omitted. Tests must never target the installed user's workspace.

## 1. Baseline and reusable presentation

- [x] 1.1 Freeze the 12 journeys and pre-change screenshots/source identity described in design.md in the existing reliability plan; build the current source and run `UI(workspace-shell.spec.ts workspace-experience.spec.ts task-first.spec.ts control-consistency.spec.ts)`. Record actual baseline paths, dimensions, states, administrative detours and unrun human checks without discarding dirty source changes.
- [x] 1.2 Implement reusable page/list headings, action groups, empty states and scoped notices with consumers in two existing features, retaining monochrome tokens and outlined controls; document ownership in `docs/advanced/workspace-ui.md`. Pass TypeScript and `UI(control-consistency.spec.ts workspace-shell.spec.ts)` with primary/secondary distinction, keyboard focus and long-label fixtures.

## 2. Composer, history and contextual setup

- [x] 2.1 Reshape Chat around its editor and adjacent model/context/voice controls; remove duplicated instructions and move optional metrics/settings out of the primary reading path. Preserve draft selection, IME, speech opt-in and Stop; update workspace/voice guides and pass `UI(chat-streaming.spec.ts voice-playback.spec.ts workspace-feedback.spec.ts)`.
- [x] 2.2 Apply the same composer grammar to task-first Agents without restoring execution-mode setup; retain capability-aware legacy presentation, conditional access and exact submission identity. Update the agents guide and pass `UI(task-first.spec.ts agent-layout.spec.ts interaction-contracts.spec.ts)` with one-submission and missing-model scenarios.
- [x] 2.3 Share history search/selection/management presentation between Chat and Agents; remove empty-history bulk controls and permanent deletion-policy paragraphs while retaining discoverable individual delete and protected-item reasons. Update `docs/guides/history-management.md` and pass `UI(task-management.spec.ts interaction-contracts.spec.ts workspace-experience.spec.ts)` for selection, confirmation, partial deletion, focus and narrow layouts.
- [x] 2.4 Implement scoped setup-origin/return handling for model and Knowledge prerequisites without placing private drafts in URLs or auto-enabling capabilities. Scope new drafts/active-chat selection to actor/workspace and implement the confirmed one-time explicit restoration of legacy drafts, preserving originals and newer destination text with safe storage-failure/unknown-workspace handling. Add `workspace-handoffs.spec.ts` for return, reload limitation, concurrent draft edits, legacy restoration, permission loss and actor/workspace changes; update model/Knowledge/workspace guides and pass `UI(workspace-handoffs.spec.ts workspace-feedback.spec.ts)`.

## 3. Work continuity, progress and results

- [x] 3.1 Add the bounded actor/workspace-scoped active-work read coordinator and shared entry surface; reuse actual job/session identity and do not enumerate every saved chat or persist another execution store. Add `work-continuity.spec.ts` for known-work counts, route selection, duplicate-follower bounds, logout and workspace switches; update workspace docs and pass `UI(work-continuity.spec.ts workspace-shell.spec.ts)`.
- [x] 3.2 Integrate conversation/job followers with navigation and service recovery, keeping explicit cancel separate from view disposal and never replaying speech or submissions. Extend continuity/streaming tests with detach/return, reload, lost acknowledgment, stale snapshots, removed work and unsupported recovery; update recovery guidance and pass `UI(work-continuity.spec.ts chat-streaming.spec.ts task-first.spec.ts voice-playback.spec.ts)`.
- [x] 3.3 Present current phase and lifecycle controls with pending input/approval in task context; move only nonessential metrics/traces into details. Reuse owned-computer stop logic and preserve exact identities/expiry; document states and pass `UI(task-first.spec.ts workspace-feedback.spec.ts work-continuity.spec.ts)` covering unknown tools, expired approval, uncertain effects and unconfirmed stopping.
- [x] 3.4 Build result-first presentation for saved answers, existing artifacts and available evidence using safe renderers and authorized source paths. Add `work-results.spec.ts` for generated-versus-verified copy, missing/revoked artifacts, unsupported formats and mobile layout; update agents/workspace docs and pass `UI(work-results.spec.ts task-first.spec.ts)`.
- [x] 3.5 Expose supported task correction/continuation alongside results, preserving steering request IDs and drafts; show an explicit new-task draft when continuation is unsupported and keep child work subordinate. Extend work-results tests with can_steer transitions, lost acknowledgment and repeated submission; update the agents guide and pass `UI(work-results.spec.ts task-first.spec.ts)` with no duplicate jobs.

## 4. Model-library presentation

- [x] 4.1 Add a capability-aware display projection and shared rows for legacy and package identities, explicit size meaning, unknown metadata and readiness; fix catalog-empty versus installed-empty wording. Update model guidance and pass TypeScript plus `UI(model-search.spec.ts speech-packages.spec.ts workspace-feedback.spec.ts)` with installed-but-unavailable speech and no chat-selector contamination.
- [x] 4.2 Introduce Installed/Discover views with retained category/filter state, one search for the current intent and a reachable operations surface; keep online discovery distinct from local filtering. Add `model-library-experience.spec.ts`, update discovery guidance and pass `UI(model-library-experience.spec.ts model-search.spec.ts)` for all four categories, empty/error states, navigation and role restrictions.
- [x] 4.3 Align legacy/package discovery with one anchored review/download decision, meaningful in-place variant choice, accessible source/license/space details and stable focus/scroll. Reuse existing resolvers/download authority; update discovery guidance and pass `UI(model-library-experience.spec.ts model-downloads.spec.ts speech-packages.spec.ts)` proving review/selection alone dispatches no transfer.
- [x] 4.4 Attach existing transfer/verification/repair/recovery state to its model and carry it across library navigation. Pass `UI(model-library-experience.spec.ts workspace-feedback.spec.ts)` plus isolated acquisition/restart fixtures in group 8; document any missing durable operation as a `voice-model-foundation` dependency, not a new implementation or a checked-off voice task. Do not mark this task complete if its specified recovery cannot be demonstrated.

## 5. Documents, activity, preferences and integrations

- [x] 5.1 Reshape Knowledge into document browsing/readiness/source inspection with Add document primary and indexing controls under management; implement the explicit Knowledge-to-Chat handoff with truthful retrieval scope. Add `knowledge-experience.spec.ts`, update embeddings/history guidance and pass `UI(knowledge-experience.spec.ts workspace-handoffs.spec.ts interaction-contracts.spec.ts)` for source revocation, setup failures, draft preservation and deletion.
- [x] 5.2 Put permitted current/recent work first in Activity, link known task identities to their canonical view and move runtime statistics/raw events into diagnostics. Update history/workspace docs and pass `UI(interaction-contracts.spec.ts task-management.spec.ts work-continuity.spec.ts)` with independent stats failure, legacy run types and administrator restrictions.
- [x] 5.3 Group Settings into preferences, workspace and diagnostics; remove repeated service/version presentations while preserving actual endpoint/build/storage information, compatibility guidance and emergency stop. Update workspace/desktop-startup docs and pass `UI(workspace-experience.spec.ts workspace-feedback.spec.ts control-consistency.spec.ts)` plus `npm.cmd --prefix desktop test`.
- [x] 5.4 Apply the shared list/action/state grammar to tools and connections, keeping labelled administration navigation, add/test/remove behavior and return-to-draft continuity. Update MCP/agents guidance and pass `UI(task-management.spec.ts interaction-contracts.spec.ts workspace-handoffs.spec.ts)` for role checks, failed tests, removal and responsive forms without auto-granting tool access.

## 6. Finish copy, accessibility and responsiveness

- [x] 6.1 Replace English literals in all touched workflow/voice surfaces with typed nine-locale resources and distinct state copy; preserve technical names. Update voice/workspace guidance and add locale-completeness assertions; pass TypeScript and `UI(voice-playback.spec.ts workspace-shell.spec.ts workspace-experience-quality.spec.ts)` with every locale, RTL and no mixed-language labels.
- [ ] 6.2 Add `workspace-experience-quality.spec.ts` checks for all six pages in both themes at 320/390px, short desktop height, 200% zoom, keyboard focus, IME, reduced motion, contrast and status announcements; inspect screenshots and run a manual screen-reader pass. Document actual browser/device and unrun assistive-technology coverage; pass `UI(workspace-experience-quality.spec.ts control-consistency.spec.ts palette-layout.spec.ts agent-layout.spec.ts)`.
- [ ] 6.3 Measure local feedback p95, large-history filtering, stream rendering and cold/warm route cost against group 1; bound subscriptions and rendering, investigate repeatable >15% regression and meet the 250ms fixture feedback gate. Record bundle raw/gzip sizes and inference-versus-UI timings in the existing reliability plan; pass the new quality/performance fixtures without claiming model latency improved from layout changes.

## 7. Complete-journey and usability evidence

- [x] 7.1 Add `workspace-journeys.spec.ts` connecting the 12 frozen workflows to independent state/output assertions across pages; use disposable service state, synthetic sources/media and explicit capability fixtures. Pass `UI(workspace-journeys.spec.ts)` with zero duplicate submissions, unauthorized effects or cross-scope leaks; record which paths are API fixtures versus actual service execution in the reliability plan.
- [ ] 7.2 Run the consented formative before/after study with at least five representative non-developer participants using the frozen tasks and disposable data; record completion, detours, timing, assistance and critical confusion. Meet the stated 90% unassisted-attempt target or iterate and retest; leave this task unchecked if participants/evidence are unavailable. Do not claim the existing production pilot or competitor parity has been achieved.

## 8. Integrated automated and installed-edition qualification

- [x] 8.1 Run `npm.cmd --prefix web/app run api:check`, TypeScript/build, a fresh native binary and the full isolated renderer suite (`node dev/scripts/test-web-workspace.mjs build/workspace-outcome-experience/offgrid.exe --workers=4 --reporter=line`), `npm.cmd --prefix desktop test`, `node --test dev/scripts/check-docs.test.mjs`, `node dev/scripts/check-docs.mjs` and `openspec.cmd validate workspace-outcome-experience --strict`. Record actual counts, failures/skips and screenshot evidence; structure validation is not product qualification.
- [x] 8.2 Build `go build -tags modeltestfixtures -o build/workspace-outcome-experience/offgrid-model-fixtures.exe ./cmd/offgrid`; run `node dev/scripts/test-web-workspace.mjs build/workspace-outcome-experience/offgrid-model-fixtures.exe --model-fixtures model-acquisition.spec.ts --workers=1` and the equivalent `model-restart.spec.ts` command. Preserve legacy model/embedding behavior; record missing backend gates in the owning voice change instead of marking synthetic inference qualified.
- [x] 8.3 Run applicable existing CI checks before claiming merge readiness, including `go test ./...`, the CI-listed race command in its supported environment, Python client tests and computer companion tests. Use the exact current commands in `.github/workflows/ci.yml` and the workflow guide; log results and unrun checks in the reliability plan. No claim of full CI passes is permitted from frontend checks alone.
- [ ] 8.4 Exercise matching installed web/desktop presentation and startup on Windows, macOS and Linux using the existing packaged desktop test scripts, including `test-desktop-theme.mjs`, with isolated homes and no uncontrolled native input. Verify preferences, popup contrast, returning work, service mismatch and rollback behavior; leave unavailable OS/installed evidence unchecked and retain the existing production-readiness gates.

## 9. User-requested container replacement for testing

This group records the user's subsequent request. Execute only after implementation and successful automated integration checks (8.1-8.3), not as part of proposal creation. Explicitly disclose unrun participant/installed-platform gates; a local testing deployment is not a production release. Do not commit, push, publish, update the desktop installation or download models under this group.

- [x] 9.1 Inspect the actual existing service/container, image and Docker/WSL context; capture a private rollback configuration and verify mounts, GPU, loopback ports, proxy/network and security settings. Build the matching image without launching another serving instance, check active work and create/verify a consistent backup using existing recovery guidance; record target identity and backup location without credentials before replacing anything.
- [x] 9.2 Stop the old service, retain it or its image/configuration as a stopped rollback target, and start only the replacement against the preserved volumes. Verify health, matching system/UI identity, served assets, model/history inventory and the redesigned routes; prove exactly one OffGrid service container is running. On failed health or compatibility, stop the replacement before safe rollback. Record the image/build, backup, results and any desktop mismatch in the reliability plan; do not remove installed data.
## 21. Approved profile-first composer and Help

- [x] 21.1 Consolidate pinned model, resources and limits into Agent settings using existing profile/restriction state; preserve active access and on-demand requests. Verify pinned revision, unchanged payloads, disabled controls and no automatic grants in agent-chat and composer tests.
- [x] 21.2 Add persistent Help and Quick actions access with local first-use examples, returning-work/recovery guidance and configuration explanations. Verify keyboard dismissal/focus restoration, draft preservation, navigation and zero implicit writes in all nine locales.
- [x] 21.3 Run API/type/build, the full isolated renderer suite and packaged Windows presentation checks; inspect realistic light/dark, narrow/RTL screenshots and update user/evidence docs. Retain the ten outstanding manual/platform gates; package/install only under the owner's separate authorization.
- [x] 21.4 Audit computed text sizes across the six primary pages and affected controls, replace confirmed undersized legacy declarations with semantic typography tokens, and verify normal/200% zoom and localized layout. Record fixture versus manual coverage separately.
## 23. Approved document reading workspace

- [x] 23.1 Add shared document reader with PDF pages, search/zoom/navigation, structured-format labels, source coverage and verified citation opening; integrate draft and saved attachments in Chat without automatic submission or Knowledge publication.
- [x] 23.2 Implement responsive Chat/document layout, focus/reading-position preservation and truthful preparation/source-use motion with reduced-motion and nine-locale copy. Verify text/voice/image and terminal switching remain intact.
- [x] 23.3 Run API/type/build and isolated document-reader/attachment/Knowledge/Chat suites; inspect light/dark, narrow/RTL/200% screenshots and record performance, failures and unavailable human checks.
- [x] 23.4 Correct attachment metadata collapse in narrow and split views, separate saved coverage notes, replace the conversation-wide focus box with a restrained keyboard indicator, and place actionable context recovery beside the composer. Verify retained drafts, response-settings opening, reload coverage and streaming/Stop feedback within the existing fixture budget.

Evidence for this increment: [document workspace verification](../../../docs/quality/evidence/document-workspace-2026-11-03.md). Source and automated checks only; installed/native, model-answer and human qualification remain separate.
