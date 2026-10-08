# Proposal

## Why

The completed state/recovery and shell slices make OffGrid more consistent, but everyday work still competes with setup, maintenance and repeated status controls. This follow-up makes the existing shared renderer outcome-led across Chat, Agents, Models, Knowledge, Activity and Settings, with observable workflow and usability gates rather than an unsupported claim of parity with leading AI products.

## What Changes

- Compose Chat and task entry around one prompt and one primary action, with context, model and voice controls grouped nearby. Keep visible button boundaries; reduce competing actions through placement and progressive disclosure, not link-like styling or hidden safety controls.
- Keep accepted work discoverable across navigation with a compact, actor/workspace-scoped active-work surface. Reuse existing session-turn and job snapshots/events, drafts, request IDs and lifecycle actions; never start a second task to resume a view.
- Scope local drafts to actor and workspace. As confirmed by the owner, offer one explicit restoration of legacy account-only drafts into the chosen workspace; preserve originals and never overwrite a newer draft or automatically restore private text.
- Present agent results, existing artifacts, evidence and supported follow-up ahead of execution details. Put exact access/approval requests in the work context, preserving consequential confirmations and uncertain-outcome handling.
- Unify the visual organization of installed models, discovery and operations across model categories. Fix contradictory catalog/installed empty states, show sizes and honest readiness, and retain one in-context acquisition decision. Existing speech acquisition implementation remains owned by `voice-model-foundation`, not a second downloader or duplicated acceptance checklist.
- Make Knowledge document-first, Activity work-first and Settings preference-first. Put indexing, diagnostics, service maintenance and bulk history management in clearly named secondary surfaces without removing them.
- Finish shared action hierarchy, page density, readable typography, semantic copy, nine-locale coverage, accessible keyboard/mobile behavior, scoped recovery and useful empty states. Preserve monochrome colors and the current visual identity.
- Validate complete journeys, not just isolated controls: start, navigate away, return, approve, stop, recover, inspect output, reuse and delete. Measure frontend responsiveness separately from model/runtime latency and compare observed usability with the pre-change build.

### Acceptance and boundaries

With an already-ready workspace, ordinary Chat and task creation require no separate setup screen or mode-selection detour. Navigation must retain the selected work/draft and must not submit, approve, cancel or replay work. Every blocked action has a specific reason and relevant recovery; completed output is visually distinct from provisional or uncertain output. Every main page is covered in both themes, narrow/short layouts, keyboard use and all nine locales.

This is one cross-cutting renderer interaction change, delivered in reviewable feature slices. It depends on the implemented `workspace-state-recovery` and `workspace-shell-foundation` source changes and does not reopen their scope. Speech package lifecycle/runtime gaps remain in `voice-model-foundation`; completing this change cannot check those tasks off.

Exclude new model/runtime adapters, universal native control, Talk mode, new project/collection APIs, a database migration, new document editors or artifact generators, backend authorization changes, and a new UI/router/state framework. Do not invent document-scoped retrieval, verification, streaming replay or agent-steering support that the service does not expose. Model latency, hardware qualification and release readiness remain separate evidence. No deployment, model download, user recording, commit, push or release is authorized by this proposal.

## Capabilities

### New Capabilities

- `workspace-composition`: Prompt-led entry and coherent context, voice and setup handoffs for existing Chat and task workflows.
- `workspace-work-continuity`: Scoped active-work navigation, lifecycle presentation, results and supported follow-up without duplicate execution.
- `model-library-presentation`: Shared model inventory/discovery/operation hierarchy with truthful identity, size, readiness and recovery.
- `workspace-management-presentation`: Document-first Knowledge, work-first Activity and grouped Settings with contextual maintenance and deletion.
- `workspace-experience-quality`: Cross-surface visual, localization, accessibility, responsiveness and measured journey acceptance.

### Modified Capabilities

None. No main capabilities have yet been published under `openspec/specs`; this change references rather than duplicates the completed shell/popover and pending model-package changes.

## Impact

Shared React components and presentation state; Chat, task-first Agents and legacy compatibility presentation; Models, Knowledge, Activity and Settings; existing API client consumers; localization; CSS ownership; Playwright and desktop presentation checks; workspace, workflow and reliability documentation. Existing HTTP/CLI/Python contracts and desktop IPC trust remain unchanged. No additional production dependency is assumed.

Evidence continues in `docs/advanced/product-reliability-plan.md`; `docs/advanced/production-readiness.md` remains the release authority. This proposal is implementation intent, not a completed redesign or usability result.

### Subsequent deployment request (2026-10-08)

The user separately requested container replacement after the improvements. Record a post-implementation testing deployment: inspect and preserve the actual container configuration, back up consistently, replace the existing service with exactly one running OffGrid container, and verify or roll back. This is not authorization to publish a release, replace the desktop app, bypass compatibility checks or claim the unfinished usability/platform gates passed. Planning alone does not execute this request.
