# Design

## Activity and attention reconciliation (2026-11-05)

The existing header truncates unordered jobs before prioritizing them, includes
terminal chat references in its popup, and polls full task histories separately
from Activity. Replace this with a compact metadata projection, deterministic
attention/running/history classification, and stable identity-based pagination.
The service enforces actor/source visibility before counting or returning rows;
summary metadata never authorizes mutation. Avoid copying transcripts or tool
payloads merely to calculate a badge. Legacy list endpoints remain compatible.

A workspace-scoped coordinator owns current summary reads and invalidation.
History pages request bounded results; commands trigger immediate revalidation.
Abort or generation-check superseded requests, prevent pre-mutation responses
from restoring removed records, and mark unavailable snapshots stale without
reclassifying real tasks as failed or silently hiding unknown effects. A linked
conversation and root job represent one work item; children remain subordinate.

Keep the header preview compact and attention-first with an explicit full-list
destination. Activity separates attention, running and history, explains the
next safe action and cleanup eligibility, and uses one scroll owner. Bulk removal
reviews a fixed snapshot and reports partial failures; stopping is never implied
by history removal. Preserve keyboard focus, locale/RTL, narrow layouts, saved
filters and drafts. Large-history checks measure payload, request count and UI
feedback independently from inference performance. Unavailable human/native
qualification remains unverified.

## Approved defaults-first interaction redesign (2026-11-01)

The default product journey is open, ask or add documents, receive a useful
answer with sources, and resume later. Existing advanced functionality remains
available, but ordinary users should not configure infrastructure each turn.

- First use checks service, hardware, installed chat/embedding capabilities and
  available offline packs. Recommend one compatible setup with size/time/privacy
  explanation and explicit install approval where needed. Keep selected working
  models and profiles unchanged; recommendations never authorize replacement.
- Return directly to saved work once ready. Do not show completed setup again.
  Missing capabilities produce one relevant action beside the attempted task,
  with drafts retained and no repeated detours through Models/Settings.
- Keep Chat and the document library primary. Group administrative destinations
  and optional tools while preserving all existing routes and keyboard access.
  Use consistent page headings and concise next actions, not another dashboard
  of status cards. No charts are introduced in this redesign.
- Composer prioritizes text/attachments, Send/Stop and active context. A compact
  settings entry owns overrides; model/agent details are disclosed on demand.
  Active permissions remain visible and revocable. Default profiles do not grant
  file, desktop, network or terminal authority.
- Knowledge uses Add documents -> per-file processing -> Ready/needs attention
  -> Ask. Source selection is a plain user decision. Embedding models, chunking,
  thresholds and reindexing are advanced administration with safe defaults and
  explicit index migration. New imports consume the linked ingestion contract.
- Agent creation starts with purpose/name and recommended existing model/tools;
  detailed budgets, routing, collaborators and qualification appear progressively.
  Saved revisions remain pinned and current authority restrictions are preserved.
- Terminal is an optional desktop tool governed by its own linked change. It
  must not become the workaround ordinary users need for broken document import.
- Freeze first-use, returning-user, offline/low-memory, failed-import and recovery
  journeys before implementation. Measure actions, exposed decisions, time to
  first useful answer, errors and assistance. Require ordinary chat without
  opening settings and a document answer without selecting an embedding model
  once setup is complete. Automated fixtures do not replace the pending human
  usability, screen-reader or language reviews.

This section supersedes prior no-setup-wizard wording only within the approved
scope; installation remains explicit and no existing data is silently migrated.

## Profile-first composer and contextual Help

Supersede the earlier always-visible pinned-model and Tools controls with one
Agent settings disclosure. Its read-only model/revision/decision-budget summary
comes from the selected profile. Resource and execution restrictions use existing
state and payloads; changing these does not mutate the saved profile. Provide a
clear route to edit the reusable profile. Keep active workspace and desktop
access visible and preserve on-demand access requests; no profile setting is a
grant. Ordinary Chat retains its model picker. No nested configuration popovers.
When the service supports on-demand folder access, omit the unused No host folder
selector until a workspace is attached. The task's existing access request owns
setup. Older services retain the explicit selector; active selections remain
visible and their readiness checks still block unavailable authority.

Add one persistent Help entry and a Quick actions entry. Help is a keyboard-safe
local dialog with short task examples, profile/default explanations, workspace
access boundaries, returning-work guidance and direct links to existing pages.
Opening Help or following guidance does not submit a task, erase a draft, alter
configuration or download a model. Closing restores focus. Localized nine-locale
copy, narrow/RTL layouts and automated no-write checks are required; human
usability and screen-reader qualification remain explicitly unverified.
The owner's typography follow-up requires checking computed styles, not merely
counting old CSS declarations: retain 12px captions, 13px metadata, 14px controls,
16px prose, 18px sections and 24px page titles with locale-aware line heights.
Replace confirmed undersized visible remnants; do not blindly enlarge every
legacy selector or collapse semantic roles to one size.

## Corrective review after installation (2026-11-01)

Utility panels distinguish compact action lists from configuration panels rather
than assigning every popup 340px. Single-history actions use one quiet row, with
keyboard focus and coarse-pointer targets retained. Single-chat confirmation uses
readable identity and singular copy. Default deletion sends no work policy; only
the service's typed `conversation_has_work`/`task_stop_pending` response reveals
radio choices. Selection alone never executes; confirmation is required, and a
new dialog resets the choice. Partial deletion retains only failed items. Plain
generation conflicts cannot offer unsupported job-retention semantics.

Profile setup relies on the global Chat/Activity navigation instead of duplicating
it beneath its heading. A direct limit checkbox replaces the two-option workspace
select without changing its permission intersection. Activity's selected view must
be visibly distinct and its actual work list must align with the page leading edge.

Quit attempts each owned input cleanup even if another rejects, then attempts the
owned service cleanup. Preference persistence is bounded and cannot prevent stop.
Service termination requires an exit acknowledgement; timeout retains ownership.
Any unconfirmed worker keeps Quit visibly failed and retryable instead of leaving
the application in a permanent quitting state. No externally attached service is
terminated. Process counts distinguish Electron helpers and embedding/chat workers
from actual duplicate or orphaned processes; a snapshot is not a shutdown test.

## Order and presentation follow-up (2026-11-01)

Apply the approved hierarchy consistently rather than adding another visual theme.
Management content shares a leading edge and bounded width; reading and settings
controls retain appropriate measures. Headings allocate room for their actions
before wrapping. Chat groups occasional response/voice preferences, while input,
attachments, microphone, active scope and Stop remain visible. Profile identity
and model precede purpose and capabilities, with explicit save/use actions.
Installed model selection is separate from maintenance; unqualified packages
remain ineligible. Knowledge groups filtering and question entry, keeping selected
sources explicit. Activity separates work, schedules and diagnostics without
changing canonical task identity. Long artifact names use a bounded summary and
an accessible full-name disclosure, never a changed download name. Startup puts
versions/address in technical details while recovery stays visible. Tests must
check useful density and discovery, not only absence of horizontal overflow.

All eight follow-up tasks reuse existing state, authority, localization and
controllers. No selection loads a model, no navigation submits work, and no
presentation state permits replay. Existing qualification gates are unchanged.

## Approved material refinement (2026-10-31)

The reviewed prototype uses three decorative families: a neutral directional
sidebar ramp, a lifted composer ramp, and floating utility material. Primary
actions become solid with palette-derived hover fills, so opening a popup never
adds a fourth decorative family. Progress retains its functional static ramp.
Keep all material definitions in the generated token block and reuse them in
the shared web/Electron renderer. No whole-window transparency or OS wallpaper
sampling is introduced. Answer, code, input text and focus indicators stay opaque.

Floating utilities and the bounded command palette use a 96% surface-color fill
and at most 12px backdrop blur when supported. A small neutral directional wash
adds depth without a new hue, border overlay or glow. Full-screen drawers and
reading surfaces do not blur. Unsupported browsers retain an opaque fill.
Reduced transparency, increased contrast, forced colors and the existing central
off switch disable the material and decorative gradients. Reduced motion keeps
static materials but removes existing entrance/press animation.

Measure foreground contrast across both themes and extreme black/white
underlays; test opening a utility during streaming at 4x/6x CPU throttling, plus
the unchanged Stop and frame-p95 gates. Nested nonmodal utilities retain their
parent drawer's Tab containment while owning Escape dismissal. Existing modal
confirmations retain their own focus scope. Capture viewport-sized overlays only
after their entrance is visible; full-page screenshots are not overlay evidence.
Earlier rejected-overlay statements describe the previous phase and are
superseded only for these explicit consumers. Physical/native and human checks
remain separate from browser fixtures.

## Whole-product review implementation (2026-10-30)

Continue the shared renderer rather than adding a second application. The owner
approved every outcome in the subsequent review. Earlier presentation-only
constraints yield to the following concrete behavior; authority, privacy, exact
identity, no uncertain replay and existing-data preservation remain invariant.

1. Separate conversation selection from generation. Use existing durable turn
   recovery and scoped work references; selecting another conversation detaches
   observation, never cancellation or submission. In-flight acceptance must stay
   correlated with its original session. Late results cannot populate another
   conversation. Return reconciles saved state without replaying speech.
2. Preview bounded supported textual artifacts on explicit request through the
   existing authenticated artifact API. Validate size and SHA-256 before display,
   decode UTF-8 strictly, render text inertly and Markdown through the existing
   safe renderer. Unknown/binary/oversized content keeps an honest download path.
   Preview integrity is not semantic correctness. Revoke/dispose private content
   on scope change, and do not execute embedded HTML or scripts.
3. Derive recovery from typed state and operation, not English error matching.
   Explain retained output and offer the appropriate existing status, reconnect,
   reconciliation or continuation action. Unknown cases retain diagnostics.
4. Bound streaming rendering and isolate input/control scheduling. Keep final
   output exact; do not hide loss or delay cancellation feedback behind Markdown.
   Run the existing throttled stress fixture without relaxing its 250ms gate.
5. Use readable reversible model identity and truthful readiness at selection;
   search includes exact IDs. Never invent hardware fit or replace the model.
6. Group agent creation around purpose, identity/model and chosen capabilities,
   with optional advanced budgets. Preserve drafts/revisions and explicit save;
   offer examples and an isolated test path without granting or running on select.
7. Keep the composer focused on the request, active context chips and Send/Stop.
   Move occasional configuration into labelled panels; preserve voice opt-in,
   microphone lifecycle, permission scope and context discoverability.
8. Expand work retrieval with bounded authorized search and explicit result types;
   preserve filters/return paths. Document questions must pin actual supported
   source scope end to end, with additive service enforcement if needed. Never
   imply a selected source restriction from a frontend chip alone.
9. Skills expose purpose/examples/use; collaborator summaries show actual returned
   state and deliverables, not fabricated progress. Schedules lead with task and
   calendar, searchable time zone and next-run preview; resource review remains.
10. Enforce consistent screen hierarchy, setting controls, menus, status and
    responsive layouts. Simplify connections/voice/login/permissions/empty states
    with action-specific guidance. Retain 44px coarse-pointer access and all nine
    locales. Shared semantic primitives replace repeated competing patterns.
11. Repair runtime distribution at its existing packaging owner, then verify
    fresh installation/startup/restart/update in isolated state on each supported
    platform. This machine's manually prepared GPU runtime is not release proof.

Verification combines regression tests, rendered state matrices, real-service
journeys, packaged installations and consented formative evaluation. All must
retain their evidence category. Missing human/platform evidence remains open.

## Compact control and configuration feedback (2026-10-30)

The user's follow-up supersedes uniform 40px desktop actions: use 32px compact
secondary actions, 36px primary/form controls where appropriate, and retain 44px
coarse-pointer targets. Keep the existing palette. Workspace options becomes a
small grouped utility panel with label/control rows rather than large stacked
buttons. History gets a stable thin scrollbar using existing neutral tokens,
with native forced-color behavior and no horizontal scrollbar or content shift.

Agent creation uses a narrower profile list and a two-column editor when space
permits. Identity/model/instructions and resources are simultaneously visible;
tools and Knowledge are not hidden behind disclosure clicks. Skills and approved
collaborators leave the Advanced nesting. Budget and execution policy remain
advanced. Resource selection, revision conflicts, dirty drafts, save validation
and authority continue through the existing handlers. Narrow screens stack in
reading order. No automatic grants, settings or resource selection are added.

## Color-frozen craft and usability extension (2026-10-30)

The approved briefs refine the existing composition. The baseline is commit
`7357acae431e40092fd40a5458207d3649d503e8`. A parser-backed inventory freezes
color-bearing CSS declarations and UI color literals, including desktop startup.
Existing palette, shadows and color pairings are retained. The subsequent approved
2026-10-31 border review below supersedes retention of every decorative outline. New work uses
geometry, typography, spacing and transform/opacity motion. The freeze takes
precedence over suggested new shadow, scrollbar or focus colors.

Foundation tokens: type 12/13/14/16/18/24px, weights 400/500/600, UI leading 1.4,
prose leading 1.6, Arabic leading 1.65 without Latin tracking, spacing 4/8/12/16/
24/32/48px, control radius 8px, panel radius 12px, editor/dialog radius 16px,
motion 120/180ms and small transform distances. Retain existing canvas/surface/
overlay colors and elevation; no new surface palette. Keep 800px chat reading
width and 248px/64px shell. Do not animate grid dimensions or reanimate tokens
on every streaming update. CSS-only entrances and press feedback respect reduced
motion. Static code language labels are presentation; Copy and Markdown safety
remain unchanged. Desktop uses equivalent inherited spacing where shared renderer
tokens are unavailable; no duplicated color palette or packaging dependency.

UX changes use existing navigation and components: contextual explanations of
model types, offline discovery, documents/indexing, agent configuration versus
execution, recovery, and keyboard actions. New copy enters the typed Messages
contract in all nine locales and requires native-speaker review. No automatic
model selection, grant, download, retry, request replay or first-run completion.

Baseline/after matrix uses isolated API fixtures at 390/768/1024/1440 in both
themes and English/Arabic, with 320px, short-height, zoom, German/isiNdebele,
keyboard, streaming, auth and failure coverage in existing/full regression suites.
Compare screenshot states and task paths; distinguish synthetic evidence, live
service evidence, installed-platform evidence and unavailable screen-reader/user
studies. Preserve the outstanding earlier qualification tasks.

## Additional approved gradient phase (2026-10-30)

Use top-lit neutral ramps from existing surfaces. Define gradients only in
tokens.css, with independently tuned light/dark definitions, flat @supports
fallbacks, data-gradients="off", forced-colors and increased-contrast flattening.
No gradient behind answer or code text. Initial retained candidates are sidebar,
composer and primary actions; static progress treatment is functional. Other
surfaces stay flat to keep the per-screen decorative budget at three. Prototype
screens precede production consumers. Reject blind scroll fades: overflow and
directional-edge state must be established without altering the protected state
model. Retain desktop startup flat rather than duplicating palette ownership.

Generate eased stops for any retained sheen. No animated full-surface backgrounds,
pointer effects, new image assets or dependencies. Test computed gradients,
kill switch, contrast extremes, forced colors, RTL, browser support, throttled
stream/download feedback and before/on/off captures. Report physical-display
banding and native platform checks as unverified when unavailable.

## Context

### Restart recovery (2026-10-30)

DesktopRuntime owns one startup attempt and one optional reconnect timer. A
remembered external endpoint receives the existing bounded startup grace period;
offline/timeouts remain checking rather than immediately becoming a fatal screen.
After grace expires, recoverable waiting exposes recovery actions and schedules
one read-only handshake every five seconds. No concurrent probes, process launch,
task replay or external restart is introduced. Incompatibility and storage errors
stop automatic retries. Quit aborts probes/timers; isolated recovery cancels the
external timer. Identity changes still revoke local dispatch before readiness.
All nine startup locales explain automatic waiting. Standalone installs retain
the native bundled service path, independent of Docker/WSL. Existing container
data is never implicitly migrated into a native workspace.

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

### Approved image-input extension

Keep the text composer central. An attachment control beside voice input accepts
file selection, paste and drop; thumbnails above the text have explicit remove
controls. Use at most four PNG/JPEG images, 2 MiB decoded each, with the existing
8192-edge/32-megapixel limits. Reject unsupported or malformed files without
discarding the text or existing images. Never fetch image URLs or run SVG.

Add bounded `images` records (display name and local data URL) to durable ordinary
Chat requests, turns and saved user messages. Preserve existing string content
and API clients. Store accepted image bytes atomically with the scoped session,
include them in request identity, and rebuild typed inference content for history.
Bound total encoded conversation images to 12 MiB, enforced atomically at turn
admission. Completed turns keep image bytes once, in their saved user message.
Keep unsent drafts scoped to actor/workspace/conversation; storage failure must
be visible and must not silently discard attachments. Reject unsupported agents,
non-durable legacy image submissions and absent local projector/runtime paths;
never drop images to answer as text. Text-only legacy submissions stay compatible.

Installed projector metadata permits image submission but is not a quality claim.
The shared inference path revalidates it before generation; missing/stale metadata
leaves the draft intact. No automatic model changes, downloads, new permissions,
camera capture or external uploads. UI capabilities are negotiated with the service.

Verification covers upload/paste/drop, replacement/removal, errors, size/type
limits, image-only prompts, multilingual labels, keyboard/RTL, text and voice
coexistence, scope changes, history/restart, request deduplication, cancellation,
unsupported model/agent paths and real model qualification separately.

CLI parity uses one bounded local-file loader and pending-image command handler
for plain and TUI chat. Read bytes when explicitly attached, not after an unrelated
file replacement; reject non-regular, malformed, unsupported and oversized input.
Keep attachments through failed or incomplete responses and clear them only after
a confirmed response. Preserve bytes in existing local saved sessions and rebuild
typed history on load. The service remains responsible for projector readiness;
the CLI never substitutes a model or strips images to retry. `/image PATH` accepts
paths with spaces, `/images` lists pending input, and `/remove-image [N|all]`
removes pending input explicitly. Repeated `--image PATH` uses the same four-file
limit. Local session storage errors remain visible rather than claiming a save.

### 1. Use one common interaction grammar, not one enormous page

Retain the six primary destinations and established routes. Share the pattern **start / current work / result / details**, but let collections such as Models and Knowledge use lists rather than copying a chat layout.

- **Chat:** conversation plus composer; model/context/voice adjacent to input; optional response details subordinate. A compact context button reveals Knowledge readiness and response settings. Show active choices without requiring the menu to be opened. Preserve ordinary conversation follow-up.
- **Agents:** reusable profile configuration and canonical work inspection. New work and supported steering belong in Chat; do not restore a separate task composer. Access and approval remain bound to the canonical task. Put results and artifacts ahead of logs and keep lifecycle controls reachable.
- **Models:** Installed and Discover views, one category selector and one search for the current intent. Discover puts curated suggestions and public search in one result area, clearly labelling origin. A distinct operations surface remains reachable while browsing either view.
- **Knowledge:** document list, readiness, source inspection and Add document; embedding/index controls under Manage Knowledge. An optional Ask with Knowledge handoff states that the current backend searches the knowledge base, not an invented single-document scope.
- **Activity:** current/recent work and outcomes first; diagnostics secondary. Link known run IDs to the canonical task route; unrecognized/legacy run types remain read-only event views.
- **Settings:** Preferences, Workspace, Advanced/diagnostics groups. One authoritative service/build section. Existing emergency stop remains directly discoverable when applicable.

Alternative rejected: merge Chat, Agents and every administrative screen into a universal chat. That would hide useful library operations and introduce routing/authorization scope beyond this change. Do not assume the leading products have no setup or permissions; improve when OffGrid presents them.

### 2. Keep controls proper while reducing visual competition

Use the current monochrome tokens, IBM Plex fonts and icons. The user's 2026-10-31
whole-app border review supersedes the earlier requirement for outlined actions:
solid primary actions, tonal secondary actions and quiet labelled utilities share
the same geometry and keyboard behavior. Preserve input, floating-panel, error and
focus boundaries; remove redundant nested profile/tool/skill frames. Composer
selectors sit inside one editor boundary. Navigation, selectable rows, statuses
and source references keep their own semantics. Forced-colors and coarse-pointer
behavior must remain usable; screenshots and interaction tests are separate checks.

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

### Approved shared-shell refinement (2026-10-15)

Use one 248px navigation/history sidebar, with a compact 64px collapsed rail and navigation/account footer outside independently scrolling history. Below 1024px the same content becomes a modal drawer with outside-click/Escape dismissal and focus restoration. Portal existing history presentation into a shell slot without introducing another session controller or mounting a duplicate Chat page. Preserve scope disposal and exact work identities. Collapse changes presentation only; it neither disposes Chat nor changes execution permissions.

Keep the reading measure and composer around 800px. Show agent/model, Tools, access, microphone and Send/Stop; detailed response/voice configuration belongs in labelled popovers with active choices visible. Retain manual scroll-up and offer Latest to resume following. Use one activity disclosure, with requests and safe recovery always visible. Adopt a single monochrome token source, IBM Plex, 14px controls and 16px reading text, 40px desktop/44px touch targets, and 4/8/12/16/24/32px spacing. Use the product logo consistently. Refine all six pages and local desktop presentation without changing IPC or consent authority. The remaining manual/platform gates stay unverified until performed.

## Migration Plan

### Composition-first implementation

New chat precedes destinations; contextual history uses one-line titles and
accessible row-action popovers rather than permanent delete icons/counts. One
workspace menu includes Settings and existing appearance/account controls.
The header summarizes only actual running/attention states, not every historical
record as active work. Preserve bounded discovery and its limitations.

Agents configures profiles/tools/connections. Activity owns work browsing and
inspection. Agent-task and workspace aliases retain canonical identities and
administrator/non-administrator boundaries; inspect is a read operation. A
selected task replaces the list in the content region rather than keeping a
second fixed-width history sidebar. Return restores scoped filtering/selection.
Reuse ConversationWork with explicit inspector presentation, preserving requests,
steering, uncertainty, evidence and stop semantics. Legacy runs remain read-only
event inspection; do not equate legacy runs with canonical jobs by guessing IDs.

Controls have primary, secondary, utility and navigation variants with 40px
desktop/44px touch targets. Utilities remain buttons with clear affordances, not
unlabelled links. Typography remains 16px reading/14px controls/12-13px metadata.
Simplify nested surfaces at their owning feature styles; do not add an override
theme. Shared search and action groups retain focus/RTL/locale behavior.

Capture current fixtures before mutation, then review populated Chat, running
agent work and blocked access before propagating the hierarchy across all six
pages. Validate light/dark, short/narrow windows, zoom and scaled layouts; do not
alter OS scale. One integrated pass uses internal gates, not partial deployments.
Use existing isolated UI and packaged desktop fixtures; never run native input
against personal documents. Human and unavailable OS qualification stays open.

Deliver in order: freeze baseline and shared primitives; composer/history; work continuity/results; model-library presentation; Knowledge/Activity/Settings; integrated qualification. Each feature slice includes its own tests and user documentation before proceeding.

Retain existing hashes, aliases and preference keys; introduce namespaced presentation/draft state with the explicit, non-destructive legacy restoration above. Keep the legacy agent compatibility branch readable without recreating old setup-first behavior on capable services. No service-side storage migration, runtime replacement or contract cutover is part of this change.

Build the shared renderer and matching service/desktop package together for qualification. Installed deployment remains separately authorized; never update an occupied service automatically or run two production containers as a UI workaround. Renderer rollback must not erase drafts, models or task state. Preserve evidence of unrun platform/pilot gates in the existing reliability plan.

### User-requested container replacement after implementation

The subsequent user message authorizes replacement for testing after implementation and automated verification, not replacement during this planning step. Missing participant/platform evidence must remain disclosed and does not become a production release claim.

Inspect the actual Docker/WSL context and existing OffGrid service before choosing commands; do not assume an old image name or Compose ownership from conversation history. Preserve image identity, mounts, data/model volumes, environment (without logging credentials), loopback bindings, GPU device requests, restart/security policies and proxy/VPN configuration. Build a matching image without starting a parallel serving container or mounting live data in build tests.

Check active work before the maintenance window. If users have in-flight work or downloads that cannot safely settle, request coordination rather than silently cancelling them. Use the existing documented stop/backup/verify flow for a consistent workspace backup, preserve the old image/configuration for rollback, stop the previous service before starting its replacement, and never let two services write the same volumes. Keep any rollback container stopped; do not delete installed data or model stores.

Verify health, system identity/UI build, served asset hashes and read-only access to existing model/history inventory. Inspect the new UI without submitting user tasks or recording audio. On failure, stop the replacement before restoring the compatible former service; do not run an old binary against changed storage. Record the backup location, image/build and count of running OffGrid services. A matching desktop installation is a separate action: report any build mismatch and never weaken its compatibility checks to attach it.
## Document workspace

Opening an attachment or citation reveals a right-hand reader beside Chat;
closing restores the reading width and trigger focus. Keep the composer in Chat,
with independent bounded scrolling. On narrow/zoomed layouts use Chat/Document
tabs. Never squeeze two unreadable columns. PDF controls include page navigation,
zoom, search of extracted text and a visible extraction-coverage notice. Other
formats show structured source text with original download available; do not
fabricate Office page fidelity. Citation navigation verifies revision and opens
the relevant source unit, with exact passage highlighting where supported.

Preparation uses a subtle scan sweep on the file preview while an actual request
is active. Uploading, preparing, ready, partial and failed states are distinct;
no invented percentage or perpetual success animation. Source use during an
answer must reflect attached/retrieved sources, not imply the model read every
page. Reduced motion uses static status, opaque readable text and accessible
announcements. All nine locales, keyboard focus, 200% zoom, RTL, offline and
permission-loss paths are acceptance requirements.

### Installed audit implementation

Attachment prompts assign answer-local source identifiers (`S1`, `S1.1`) without
changing stored source bytes or existing selection records. The renderer binds
references only to that answer's persisted source ID, revision and exact ranges.
A new owner-scoped `GET /v1/sessions/{name}/attachments/{id}/passage` validates
revision, digest, UTF-8 boundaries and a 256 KiB response bound. Legacy filename
and byte references bind only when unambiguous and exactly selected. Ordinary
links and code are never interpreted as source references. Revocation or a
changed source fails closed. The reader finds a page only for one exact match in
one extraction unit; repeated/cross-page passages display retained text without
inventing a page. This verifies source identity, not claim entailment.

Activity uses shared running/attention/history classification and scoped UI
filter preferences. Header navigation selects the matching filter; changing
filters does not change jobs. Child work remains under its parent. Cancelled
downloads are collapsed but recoverable, active/failed transfers stay visible,
and the selected installed model has an explicit label. Knowledge explains its
reusable-library scope without publishing conversation attachments implicitly.

## Inactive work archival

Persist an additive archived_at timestamp on the task snapshot. Serialize archival with runner admission and teardown; reject active workers, pending approvals, and active children. Permit uncertain, interrupted and terminal root jobs. Keep all receipts, checkpoints and original states. Normal work counts exclude archived records; a separate archived count and paginated bucket expose them. Restore is explicit and never resumes execution. Preserve owner and Knowledge authorization, legacy local-admin ownership, restart persistence and immediate summary invalidation.

## Public brand and upgrade identity

OffGrid is the public product name. Display names and artifact filenames are separate
from package, bundle, uninstall, executable and storage identities. Retain the existing
Windows executable name and app ID for running-process/upgrade compatibility; pin
Electron userData before applying a display name. Keep CLI executable offgrid and
repository/module URLs unchanged. Desktop assets use OffGrid-VERSION-OS-ARCH with
Windows setup/portable suffixes. Linux package architecture conventions remain explicit.
Release reuse/finalization, installer selection and documentation must agree. Legacy
version downloads select their original names. macOS installation consumes the ZIP
actually published, preserving the existing bundle ID and user data. Windows upgrade
checks verify one install registration and updated shortcuts with existing data intact;
other native platform checks remain unverified unless executed on that platform.

## Management follow-up boundaries (2026-11-06)
The model-manager-only discard endpoint serializes with download admission, rejects active workers, removes only exact legacy or digest-pinned temporary files, and persists removal of the progress record. Published weights/projectors and receipts are retained. Missing partials make retries safe; persistence failure keeps the record visible. The UI confirms removal and guards against stale progress responses.
MCP connection management precedes tool discovery. External setup is a read-only modal with bounded scrolling and focus restoration to its initiating card; copying commands performs no setup. Knowledge uses a small menu with model identity and a separate action row. History cleanup aligns with search on wide layouts and wraps immediately below on narrow layouts.
Desktop navigation observes explicit sidebar activation as well as route changes, hiding the native terminal surface without closing processes. The terminal welcome offers CLI and Agent entry using the existing scoped folder selection and host lifecycle.

## Native installer presentation

Use the existing NSIS assisted installer and supported electron-builder hooks.
Retain native controls, DPI awareness, directory/scope selection, running-app
guards and deferred non-elevated launch. Use a full-width welcome/finish layout
with a small OffGrid mark, aligned headings and concise copy instead of a
decorative sidebar. Map layout dimensions through native dialog units for DPI
scaling. Progress remains tied to actual installation; diagnostic file output
stays available through Show details. The uninstall welcome explains application
removal separately from retained models and saved work. No data-erasure option,
automatic download or new installer engine is introduced. Render the actual
isolated wizard and verify preservation, not only source string assertions.
