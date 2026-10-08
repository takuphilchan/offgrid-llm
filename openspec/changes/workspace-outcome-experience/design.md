# Design

## Context

See `proposal.md` for motivation. This is the next interaction layer over the completed `workspace-state-recovery` and `workspace-shell-foundation` changes, not a second shell redesign. Existing ownership and release boundaries are in `docs/advanced/workspace-ui.md`, `client-contracts.md` and `production-readiness.md`.

Observed in the source and current fixture screenshots:

| Surface | Existing implementation | Gap this change addresses |
| --- | --- | --- |
| Chat | `ChatPage.tsx` owns session selection, request following, drafts and response speech; current-turn recovery already exists | Model/context controls are remote from the composer; history management competes with entry; runtime metrics are prominent |
| Agents | `TaskWorkspace`, `useTaskRun`, `TaskAccess` and `TaskControls` already use durable jobs, steering and artifacts | Deletion-policy text dominates empty history; lifecycle/output/details need clearer hierarchy; work is hard to find from other pages |
| Models | `ModelsPage` and `PackageModels` expose legacy and package paths; shared service owns operations | Concurrent discovery/catalog searches; one empty-catalog branch uses installed-model wording; package and single-file rows feel different |
| Knowledge | Source dialog, document status, ingestion and reindexing already exist | Index/embedding administration is mixed with document browsing |
| Activity / Settings | Separate run/events/stats requests; role checks; version and backend identity | Runtime counters precede useful work; repeated service/version cards; maintenance dominates preferences |
| Shared UI | Shell and popovers extracted; final controls layer provides button boundaries | Feature layouts still own too many competing surfaces; voice preferences contain English literals; feature-state coordination remains distributed |

`voice-model-foundation` tasks 5-8 still own acquisition durability, public model operations and package integration evidence. This change consumes those contracts; it cannot complete them by restyling cards. Production-readiness documentation contains future API plans alongside implemented contracts: use `api/client.ts`, generated types and actual service capabilities for implementation, not roadmap wording as evidence of availability.

## Goals / Non-Goals

**Goals:** One understandable path from intention to work to result; preservation of existing state and authority; reusable presentation primitives and bounded read coordination; objectively testable behavior across all six main pages.

**Non-goals:** A new agent platform, unified replacement database, unrestricted computer authority, automatic model/capability switching, new editors, new document-scoped retrieval or hosted inference. Do not rewrite every feature merely to impose one abstract controller. Existing browser/desktop and CLI/API behavior remain compatible.

## Decisions

### 1. Use one common interaction grammar, not one enormous page

Retain the six primary destinations and established routes. Share the pattern **start / current work / result / details**, but let collections such as Models and Knowledge use lists rather than copying a chat layout.

- **Chat:** conversation plus composer; model/context/voice adjacent to input; optional response details subordinate. A compact context button reveals Knowledge readiness and response settings. Show active choices without requiring the menu to be opened. Preserve ordinary conversation follow-up.
- **Agents:** new-task prompt and current-task detail, not a permanent configuration/result split. Access and approval appear only when the bound task requests them. Put results and artifacts ahead of logs. Keep Pause, Take over and Stop contextual and reachable. Existing supported steering becomes the visible follow-up area; unsupported steering explains what an explicit new task would do.
- **Models:** Installed and Discover views, one category selector and one search for the current intent. Discover puts curated suggestions and public search in one result area, clearly labelling origin. A distinct operations surface remains reachable while browsing either view.
- **Knowledge:** document list, readiness, source inspection and Add document; embedding/index controls under Manage Knowledge. An optional Ask with Knowledge handoff states that the current backend searches the knowledge base, not an invented single-document scope.
- **Activity:** current/recent work and outcomes first; diagnostics secondary. Link known run IDs to the canonical task route; unrecognized/legacy run types remain read-only event views.
- **Settings:** Preferences, Workspace, Advanced/diagnostics groups. One authoritative service/build section. Existing emergency stop remains directly discoverable when applicable.

Alternative rejected: merge Chat, Agents and every administrative screen into a universal chat. That would hide useful library operations and introduce routing/authorization scope beyond this change. Do not assume the leading products have no setup or permissions; improve when OffGrid presents them.

### 2. Keep controls proper while reducing visual competition

Use the current monochrome tokens, IBM Plex fonts, icons and outlined action controls. Reduce visual weight through grouping, whitespace and conditional relevance, not unframed link-like action buttons. Navigation, selectable rows, statuses and source references keep their own semantics.

Shared primitives should cover page heading/action groups, list toolbars, scoped notices, empty states, result/artifact rows and history management. Reuse `UtilityPopover`, existing dialogs and the final control layer. Extract when two surfaces need the same behavior; avoid creating a component library without consumers.

Default composition:

- One primary action per active region; during work the lifecycle controls replace submission emphasis.
- Main reading text 15-16px; compact control text 13-14px; essential instructions never reduced to tiny metadata. Secondary labels remain legible and contrast-tested in both themes.
- Keep the current 40px minimum control height and 8px radius; use at least 44px targets in touch layouts where necessary. Do not enforce fixed heights that clip translations.
- Composer/content reading measure around 60-80 characters; list rows may use wider space. At 320px and 200% zoom, prefer one scroll owner, wrapped toolbars and accessible drawers over squeezed columns.
- Avoid card-inside-card-inside-card. Use section headings/dividers when a new surface adds no meaning. Empty pages offer a concise next action, not a dashboard of unavailable features.
- Put raw model IDs, context/token metrics, hashes and event payloads under identifiable details while retaining names, actual selected context and important readiness at the point of decision.
- Remove repeated Refresh buttons where they refresh the same state; preserve scoped retry and the shared refresh action. Bulk management appears in a named Manage history control, with individual removal discoverable by pointer and keyboard, not hover-only.

### 3. Coordinate work visibility without creating an execution store

Add a small workspace-level active-work coordinator above route views. It stores read models/references keyed by actor, service/workspace identity, resource kind and exact existing identifier. It never owns approvals, creates jobs on mount, or treats cache as durable service state.

Reuse `useTaskRun`/agent replay and conversation `currentTurn`/`followTurn`. The active-work control shows known active work, not an invented global count. Seed jobs from the existing permitted job listing and chat from submitted/selected/recovered turns; do not fan out across every saved conversation to discover activity. Bound the set, polling and retained payloads; show overflow via the existing history view. Keep at most one shared follower for a given visible/active resource, reference-count it, and stop unnecessary terminal followers.

For Chat, distinguish the submitted request, its view subscription and explicit cancellation. Navigating away may detach the follower, but must not issue `cancelTurn`; returning reconciles the service turn. Do not replay speech. Preserve existing draft/request recovery rather than manufacturing durable partial transcripts. Closing/reloading the browser is not a promise of new backend guarantees.

Existing `workspace-context.tsx` caches presentation by account, not durable jobs. Do not put execution objects into its module-global map. Scope new coordination to actor **and** workspace; clear private entries/subscriptions on logout, identity changes, revocation and incompatible backend. If workspace identity is unavailable, avoid persistent shared references and show the compatibility limitation. Browser storage is not a new authority or secret store.

Global Stop targets the visibly identified work and uses the same lifecycle functions as the task view. Preserve `stopOwnedComputer` ownership checks; stopping a chat is not stopping unrelated computer control. No new background microphone or playback ownership is added. Leaving the bound conversation/task still suspends capture and speech according to the existing privacy contract.

Alternative rejected: hidden mounted copies of all pages to preserve state. They would duplicate polling, retain private content and couple lifecycle to presentation. Also reject a new global state framework when bounded React subscriptions can reuse current contracts.

### 4. Preserve exact context through setup and result handoffs

Represent a handoff as an account/workspace-scoped origin route, existing work/draft identity and purpose such as missing model or requested Knowledge. Keep it in memory; do not put prompt text, connector URLs or tokens into hashes/query strings. The existing draft store remains authoritative for local drafts.

Offer Return to your draft after setup, recheck readiness and retain chosen context. Never silently enable retrieval, download a model, overwrite a draft or submit. Reload may restore the draft through existing persistence without claiming the transient handoff survived.

The owner confirmed explicit restoration of legacy drafts on 2026-10-08. New local draft keys and active-chat selections include both actor and workspace identity. Do not read account-only drafts into a composer automatically. Offer a one-time Restore previous drafts here confirmation, explain that these drafts have no known workspace, and preserve all original keys. Copy only into empty destination drafts; retain conflicts in their original keys, never merge or overwrite newer text. Record the restoration decision for that actor/workspace only after storage succeeds; failed storage retains recoverable originals and reports the failure. With missing workspace identity, use session-memory drafts only and disclose that persistence cannot be safely scoped. Never fall back to account-only private content. In-memory handoffs and draft views reset on actor/workspace/permission changes. Restoring drafts grants no authority and sends no requests.

Use service-provided artifact metadata and same-origin authorized URLs. Render textual results with the existing safe Markdown renderer; unsupported artifact types receive named download/open actions, not executable HTML. No PDF/Office/website preview engine is added. Separate saved, generated and verified labels according to actual evidence. Source inspection uses existing document access checks, and references retain their citations.

### 5. Unify Models presentation while keeping acquisition ownership explicit

Create a typed display projection for identity, capability, source, size meaning, readiness and existing operation actions. It adapts current LLM/embedding and package responses; it does not collapse their different state semantics or discard exact IDs. Filename-derived friendly names remain reversible through details; unknown metadata is labelled unknown.

Installed, discovery-empty, search-failed, catalog-failed and category-empty copy are different. Discovery uses one explicit network search; local filtering does not pretend to query Hugging Face. Single variants expand directly to one review/download decision; multi-variant choice stays inside it. Maintain scroll anchor and focus, show required size/license/runtime conditions, and keep active progress attached to the model.

This change owns layout and cross-category display contracts. The existing voice change owns multi-artifact operation correctness, migration, install/repair/removal semantics and supported runtime claims. Any absent backend contract blocks that dependent experience; do not add fallback download code or mark those voice tasks done here. Local import remains an advanced/offline entry, not the default acquisition path.

### 6. Turn maintenance and errors into contextual workflows

Reuse `HistoryDeleteDialog` and exact service eligibility. Keep a selected snapshot, report partial deletion, preserve focus and update lists only after confirmed responses. No fake undo or deletion of output files when removing history. Model removal and document removal keep their distinct data consequences.

Show one contextual recovery with the affected object, what is retained and the next safe action. Read retry is distinct from checking a mutation's accepted state. A generic banner must not replace an actionable known error; an unknown error keeps safe details/request identity available. A successful optional read must not erase an unrelated error.

For approvals, summarize typed supported operations and consequences; retain exact arguments in details. Unknown tools must state uncertainty instead of inventing a harmless description. No auto-approval through UI choice, transcript or navigation. Move deletion policy to the management interaction; do not remove its explanation when it matters.

### 7. Apply localization, accessibility and performance as design constraints

Migrate English literals in touched voice and workflow surfaces to typed nine-locale resources. Preserve model names/technical identifiers. Test long labels, Arabic logical layout, IME composition, keyboard menus and focus restoration. Include native popup contrast, reduced motion, 200% zoom, screen-reader announcements and normal-text contrast of at least 4.5:1. Translation keys and automated accessibility checks do not replace speaker/manual review.

Avoid re-rendering the entire shell for every streamed token. Keep the existing bounded text update approach, isolate work summaries from large transcripts, defer technical details until opened and test large histories. Introduce list windowing only if measured; prefer existing bounds/pagination rather than adding a dependency blindly.

Record cold load, warm interactions, read latency, queue wait, generation and speech preparation separately. Local feedback p95 must stay within 250ms on named fixed fixtures. Investigate repeated >15% regression on the same fixture; do not blame model inference for an unresponsive button. The existing bundle warning is a tracked baseline: report raw/gzip growth and route parse cost; substantial growth requires a measured explanation, not an arbitrary size exemption.

### 8. Freeze complete journeys and compare against the starting build

Before changing feature behavior, record the current working-tree build (including the completed shell slice), source diff identity, seeded state, viewport, device, theme and locale. Do not reset user changes or use their live data. Capture empty, populated, busy, blocked, error and completed states for all six pages. Retain representative screenshots at 1440x900, 1280x720, 390x844, 320x480 and a short desktop height, plus 200% zoom.

Freeze these 12 journeys in the existing reliability plan with independent expected outcomes:

1. Ready Chat: type, send, inspect saved response, follow up.
2. First use: missing chat model, explicit setup, return to preserved draft, first successful reply.
3. Context: request Knowledge, encounter unavailable setup, return with honest retrieval scope and an inspectable source.
4. Voice: choose installed ASR/TTS, dictate an editable draft, opt into speech, stop playback; mock media in automation, separately consent real-device tests.
5. Task: submit an ordinary task with no up-front application setup; inspect its result/artifact.
6. Task access: grant only requested local scope, review a consequential action and return to progress.
7. Continuity: leave active work for another page, return, pause/stop or steer where supported without duplicate submission.
8. Recovery: lost acknowledgment/disconnection, inspect saved state and uncertain effects without automatic replay.
9. Models: choose category, search, inspect size/requirements, select a meaningful variant and download through existing authority; use synthetic sources in CI.
10. Model recovery: navigate away, resume/check a failed operation, remove an eligible model without deleting unrelated data.
11. Knowledge: add a permitted document, inspect source/readiness, handle unavailable indexing and delete explicitly.
12. Management: find canonical work from Activity, delete an eligible selected set, inspect preferences/service details and return without losing a draft.

Record clicks/decisions, backtracks, time to first useful result, mistakes, recovery time and moderator help. For a ready workspace, entering Chat/Agents to submission needs only composing and Send/Start; switching views must not add setup confirmations. Returning to known active work must require at most opening the work list and selecting the item. Actual OS permissions and consequential approvals are recorded separately, not removed or hidden to improve a score.

All automated journeys must pass with zero duplicate submissions, cross-scope leaks or unauthorized effects. Run a formative comparison with at least five representative non-developer participants; freeze tasks before observation, counterbalance before/after order, obtain consent and use disposable data. Target at least 90% unassisted completion across assigned attempts, no critical confusion about running/stopped/saved states, and reduced administrative detours versus baseline. Report the small sample honestly. This is not a replacement for the existing 30-day/15-person production pilot or evidence of universal competitor parity.

## Risks / Trade-offs

- Hidden controls become hard to find -> use labelled context menus, visible active settings, keyboard access and journey discovery tests; never hide Stop or pending consequential requests.
- Shared state leaks between accounts/workspaces -> identity-keyed subscriptions, immediate disposal and revoked-access fixtures.
- Scope grows into unfinished runtime work -> preserve the API and ownership table above; record actual blockers in the voice/reliability plans instead of mocking a completed workflow.
- A result-first layout falsely certifies output -> distinguish saved/generation status from independent evidence and preserve uncertainty.
- Different legacy and package behavior persists -> common presentation projection with explicit missing capabilities, no invented state equivalence.
- Local inference is slower than hosted products -> separate UI feedback from model timing; no unmeasured performance comparison or silent model substitution.
- Controls become monotonous after consistency work -> vary hierarchy and placement while preserving proper buttons and the user's palette.
- UI tests pass but first-time users remain confused -> require observed complete journeys and keep human/installed gates unchecked until supplied.

## Migration Plan

Deliver in order: freeze baseline and shared primitives; composer/history; work continuity/results; model-library presentation; Knowledge/Activity/Settings; integrated qualification. Each feature slice includes its own tests and user documentation before proceeding.

Retain existing hashes, aliases and preference keys; introduce namespaced presentation/draft state with the explicit, non-destructive legacy restoration above. Keep the legacy agent compatibility branch readable without recreating old setup-first behavior on capable services. No service-side storage migration, runtime replacement or contract cutover is part of this change.

Build the shared renderer and matching service/desktop package together for qualification. Installed deployment remains separately authorized; never update an occupied service automatically or run two production containers as a UI workaround. Renderer rollback must not erase drafts, models or task state. Preserve evidence of unrun platform/pilot gates in the existing reliability plan.

### User-requested container replacement after implementation

The subsequent user message authorizes replacement for testing after implementation and automated verification, not replacement during this planning step. Missing participant/platform evidence must remain disclosed and does not become a production release claim.

Inspect the actual Docker/WSL context and existing OffGrid service before choosing commands; do not assume an old image name or Compose ownership from conversation history. Preserve image identity, mounts, data/model volumes, environment (without logging credentials), loopback bindings, GPU device requests, restart/security policies and proxy/VPN configuration. Build a matching image without starting a parallel serving container or mounting live data in build tests.

Check active work before the maintenance window. If users have in-flight work or downloads that cannot safely settle, request coordination rather than silently cancelling them. Use the existing documented stop/backup/verify flow for a consistent workspace backup, preserve the old image/configuration for rollback, stop the previous service before starting its replacement, and never let two services write the same volumes. Keep any rollback container stopped; do not delete installed data or model stores.

Verify health, system identity/UI build, served asset hashes and read-only access to existing model/history inventory. Inspect the new UI without submitting user tasks or recording audio. On failure, stop the replacement before restoring the compatible former service; do not run an old binary against changed storage. Record the backup location, image/build and count of running OffGrid services. A matching desktop installation is a separate action: report any build mismatch and never weaken its compatibility checks to attach it.
