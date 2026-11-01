# Workspace UI architecture

OffGrid presents one product surface across the browser and the Electron
desktop application. The desktop application securely hosts the same compiled
React UI rather than maintaining a fork. A change to a workflow should therefore
work in both environments unless it is explicitly guarded by a desktop
capability.

Shared surface tokens provide neutral sidebar/composer depth and lightly frosted
utility menus and the command palette. Text remains opaque; primary actions use
solid fills. Reduced transparency, increased contrast, forced colors and browsers
without the required CSS support receive opaque surfaces. Effects are static and
bounded to three decorative surface families. See the maintained
[surface-material guide](../ux/gradients.md) for ownership and verification.

## Product structure

The primary navigation is organized by user intent:

- **Work** contains Chat and Agents.
- **Library** contains Knowledge and Models.
- **System** contains Activity and Settings.

Feature behavior belongs under `apps/web/src/features/<feature>`. `App.tsx` owns
only shell-level state: routing, authentication, service reachability, the
selected chat model, locale, theme, and onboarding. API access remains in
`apps/web/src/api/client.ts`; feature components should not duplicate fetch and
error-decoding logic.

The shared header shows the page title, quick actions and relevant work status.
One 248px sidebar combines navigation with contextual Chat history and an early
New chat action. It can collapse to a labelled 64px icon rail without disposing
the composer. History scrolls independently; row actions use labelled menus with
the existing deletion confirmation. The footer has one **Workspace options**
menu containing Settings, refresh, language, appearance and account controls,
with connection status below it. Below 1024px a labelled navigation button opens
the expanded sidebar as a drawer; Escape/outside click closes it and restores
focus. All six destinations remain available.
These controls do not grant permissions or restart active work.

Quick actions also finds saved work on services advertising `work-search-v1`.
It searches authorized **conversation names and root task requests**, not message
bodies, artifacts or file contents. Search starts after two characters, is
debounced and cancelled when replaced or closed, and returns at most 20 metadata
matches. A partial-search notice means a result, time, storage or scan limit was
reached; refine the query rather than treating an empty partial result as proof
that work does not exist. Older services retain command-only search.
Opening a result navigates through the existing conversation/task identity and
offers **Back to previous view**, preserving its draft and active work. Search
results and return state are discarded when the actor or workspace changes;
search never starts, resumes or replays execution.

The Workspace options Refresh control reloads the current page's data without remounting its
editor or clearing drafts. Normal lists do not repeat it in each section heading.
Scoped recovery controls remain beside failed reads, expired approvals and
uncertain operations, where their distinct purpose is needed.

`components/shell` owns header/navigation presentation and shell geometry;
`lib/navigation.ts` is the shared destination list for desktop, mobile, and quick
actions. Authentication, service refresh, and page identity stay in `App.tsx`.

Agent-capable services expose reusable profiles at `#/agents` and
`#/agents/profiles`. The editor selects an installed model, instructions,
authorized tool references, Knowledge documents and optional bounded research
profiles. Save pins an immutable revision; conflicts retain the draft instead
of overwriting another editor. Missing resources stay visible for repair rather
than silently changing providers or downloading models. Drafts survive navigation
in actor/workspace-scoped memory. Unsaved reload/close warns before leaving.

Chat selects a profile for future messages with a compact Agent control. Tools
can narrow its resources without changing the saved profile. The selected model
comes from that revision; editing the profile does not rewrite running work.
Linked activity observes the same canonical job as its secondary work inspector.
Activity owns the task list and full-width canonical inspector; it does not keep
a second task-history pane alongside the result. Old `#/agents/workspace` and
`#/agents/task/<id>` links remain observation-only aliases. `ConversationWork`
is shared between Chat and the inspector; lifecycle actions, approval expiry,
manual observation/reconciliation, evidence and steering retain their existing
contracts. Optional previews and technical steps live inside Activity details.
Legacy run events and runtime statistics remain available under Diagnostics.
Member inspection does not enumerate administrator-only job history.
Ordinary typed Chat retains its streaming path. An older service cannot silently
interpret a selected profile as plain Chat.

Complex features use a small second level of deep-linkable navigation. Agents
keeps profiles, tool administration, and MCP/external-agent connections in
`#/agents/profiles`, `#/agents/tools`, and `#/agents/connections`.
Browser history and reload must retain these views.

Activity owns one work inspector: `TaskWorkspace`. It lists saved jobs and
opens their detail, access, recovery, and evidence controls. It has no task
composer: Chat is the only normal place to start work, including tool-enabled
turns. `AgentPage` selects profiles or the separate `AgentManagement` panels;
there is no legacy Task/Result page, pre-task mode
picker, or duplicate execution follower. Old `#/agents/new` links show the
activity view without restoring the removed composer.

System capability names describe configured API contracts, not storage health.
Task storage failure must return a recoverable error and block execution, never
remove the task-first contract or switch the UI. Clients connected to an older
service that withdraws its flag retain the Chat draft and block tool-enabled
submission until status recovers. A cold unsupported service presents update/retry
guidance and management navigation instead of mounting an obsolete task runner.
Retry is read-only. Transient identity failure must not change the shell's
actor/workspace key; a real actor or workspace change still isolates state.

This retirement removes presentation-only setup, polling, unused client wrappers,
copy, styles and an orphaned syntax theme. It does not delete task history,
conversation data, model installations, legacy HTTP/SDK contracts, migration
readers, companion protocol compatibility, or current approval/stop controls.
`agent-availability.spec.ts` covers failure transitions in both browser and
desktop-renderer fixtures; the installed application still needs a matching
rebuilt renderer. UI removal does not repair an unhealthy task database.

## Visual system

`components/WorkspacePresentation.tsx` owns reusable section headings, action
groups, list search, empty states and scoped notices. These are presentation-only:
features retain request cancellation, draft state, deletion eligibility and
authorization. Chat, Agents and Activity use them; corresponding layout
lives in `components/workspace-presentation.css`. Empty, loading and failed reads
must remain distinct. Controls continue to use the final `controls.css` layer.

`tokens.css` owns the semantic palette, typography and spacing tokens.
`styles.css` retains feature layout while `workspace.css` owns component treatment
and feature responsive overrides. `components/shell/workspace-shell.css` owns shared shell
layout, including header/sidebar size variables used by Chat. New visual work
must use the semantic custom properties in the
workspace layer instead of introducing literal feature colors.

`controls.css` is the shared, final control layer for both renderer editions.
`layers.css` is imported before any application modules and declares layer
precedence once. Feature CSS must not establish layer order accidentally through
module import order (especially mobile overlays and lazy-loaded routes).
It owns button/field geometry and interaction states; feature rules must not
redefine those values. Default actions and fields use a 40px minimum height,
8px radius, 14px text and 8px action spacing. Text can wrap and controls can
grow; do not set fixed heights that clip translated labels.

- Use `primary-button` for the solid main action, `secondary-button` for tonal
  alternatives, and `danger-button` for destructive operations. `text-button`
  and `utility-button` are quiet, labelled actions with visible hover and keyboard
  focus. Do not add an outline to every action. Destructive actions still require
  their existing confirmations; forced-colors mode restores explicit boundaries.
- Route-changing setup/recovery actions and artifact downloads retain anchor
  semantics but use `secondary-button`, including its sizing and focus states.
  Navigation rows and actual content/reference links remain distinct; references
  use the monochrome text palette, not browser-default blue or visited purple.
- Use `action-group` to keep related actions together. Toolbars and per-document
  controls wrap rather than overflowing at narrow widths.
- Associate fields with visible labels (`field` provides the stacked layout).
  Do not rely on placeholders as the only explanation of a form field.
- Keep native disabled semantics and keyboard focus rings. Compound search
  controls have one focus surface, not nested borders. The chat editor, command
  palette and compact segmented navigation have explicit layout exceptions.
- Test long labels/model names, dark/light appearances and RTL. The control
  regression suite includes 320px/390px and desktop layouts; Electron packages
  consume the same compiled renderer, not a separate styling implementation.

Chat groups the selected model, **Context & response**, and voice settings next to
the prompt. Context & response contains Knowledge readiness and optional response
limits; selected Knowledge remains visible on the trigger, and a failed check of
selected context stays visible outside the panel. Unused Knowledge setup does not
occupy the conversation. Response timing/context metrics are under **Response
details** in that saved answer's action footer, beside Copy and Read aloud.
The details popover follows the same outside-click/Escape contract as other
utilities. Metrics are never presented as belonging to an older or in-flight
answer; incomplete output warnings remain visible.
Playback opt-in is inside Voice settings; enabled speech and active playback
remain visible. Microphone and Send/Stop sit beside the compact configuration
row. Transcript and composer share an 800px reading width and 16px body text.
Scrolling upward suspends follow-latest; the explicit Latest button resumes it.
The page itself does not compete with the transcript for scrolling.
Agents configures profiles; it has no separate task composer. Chat owns the voice
settings popover: clicking outside or pressing Escape dismisses it without
resetting model selections. Escape returns keyboard focus to its trigger; the
panel stays within the viewport in narrow and RTL layouts.

`components/UtilityPopover.tsx` owns nonmodal utility placement and dismissal.
It uses the native auto-popover top layer, bounds its scrollable panel to the
visible viewport, and repositions for content, direction, scroll, and resize
changes. Consumers own metadata requests and cancel them when dismissed. Do not
reuse it for consequential confirmations or add a modal focus trap to utilities.

The palette is deliberately monochrome. Statuses may use restrained semantic
contrast where losing the distinction would be unsafe. IBM Plex Sans, IBM Plex
Sans Arabic, and IBM Plex Mono are bundled with the application, so typography
does not rely on a network request and remains consistent offline.

Assistant responses are rendered as CommonMark plus GitHub-flavored Markdown.
Raw HTML is not enabled, and remote Markdown images are replaced with inert
labels so a response cannot trigger an unexpected tracking or network request.
Links are limited to HTTP, HTTPS, and email schemes. Code blocks and complete
responses expose explicit copy actions.

Both dark and light appearances are supported. The default follows the
operating system; an explicit choice is stored as `offgrid.theme`. Electron
forwards operating-system theme changes through the constrained preload bridge.
The quick-action palette opens with `Ctrl+K` or `Command+K` and provides
keyboard-accessible navigation and appearance actions.

The interface ships with English, French, Spanish, German, Arabic, Kiswahili,
ChiShona, isiNdebele (Northern Ndebele), and isiZulu. Locale packs implement the
shared typed `Messages` contract, so adding a language cannot silently omit
interface copy. The selected locale is stored as `offgrid.locale` and is shared
by the web and Electron interfaces.

## Desktop boundary

The renderer does not infer physical-device locality from a loopback endpoint.
Shell, empty-chat, onboarding, and speech feedback describe the connected OffGrid
service/workspace. **Settings → Workspace** shows
the endpoint (and desktop backend information when available). A reachable
service is not a privacy certification; external tools retain their separate
disclosures and authorization. Authentication already identifies this service as
the recipient of credentials. No new routing or processing authority is granted.

Desktop-only behavior is represented by the optional typed bridge in
`platform.ts`. The browser must continue to work when `window.electron` is not
defined. Do not expose arbitrary filesystem access or command execution through
the bridge. The Settings page may display safe runtime paths returned by the
main process, while backend operations still go through the normal OffGrid API.

Electron's loading surface follows the same neutral dark/light palette and
switches to the React workspace only after the local service is ready. Window
background colors must match the selected system appearance to avoid a white or
dark flash during startup.

## First-run contract

Onboarding is an outcome, not a dismissed modal. It is complete only after:

1. the local service is reachable;
2. a non-embedding chat model is available; and
3. OffGrid returns a non-empty assistant response.

Closing the guide never marks setup complete. The Models and Chat pages retain
inline next-step guidance until the first successful reply. Embedding models
must never be selected as chat fallbacks.

## Quality gates

### Status and interaction conventions

- A transfer bar exists only while downloading or preparing a model. Installed
  models use a compact status, not a disabled action. Installation and active
  knowledge retrieval are separate states; setup observes only the selected
  embedding model's activation work.
- Workspace options Refresh updates the mounted page through `useWorkspaceRefresh`.
  It must not remount the workspace, erase drafts, or resubmit running work.
  Failed optional status requests remain unknown, not disabled or healthy.
- Models separates Installed (local filtering) from Discover (online search and
  catalog). Filters and category choices survive navigation; explicit reviews
  stay with the originating row. Setup uses bounded controls and one primary action.
- Agent tabs implement arrow/Home/End navigation, tool switches have accessible
  names, and narrow-screen navigation exposes every destination without sideways
  scrolling. Results use the same safe Markdown renderer as chat; Activity keeps
  raw event payloads behind explicit technical-details disclosure.

For shell or workflow changes run:

```bash
cd apps/web
npm run api:check
npm run check
npm run build
npm run test:e2e

cd ../desktop
node --check main.js
node --check preload.js
```

Playwright starts an isolated Vite server for local runs. To exercise an
already-running packaged OffGrid server instead, set `OFFGRID_E2E_URL` to its
origin before running the suite. This keeps local tests deterministic while CI
can validate the UI assets served by the Go runtime.

`tests/workspace-experience.spec.ts` protects theme persistence, truthful
onboarding, the successful-first-reply boundary, Markdown rendering, safe image
handling, command navigation, and conversation access at narrow widths. Extend
it when changing those contracts. Feature-specific tests
should mock only the API boundary they exercise or run against the repository's
integration server in CI.

## Interaction and recovery contracts

Knowledge foregrounds documents, readiness, source inspection and Add document.
Embedding and reindexing controls live in named management panels. **Ask using
Knowledge** preserves the Chat draft and explicitly selects permitted-base
retrieval, not a single-document filter; submission remains a separate action.

Activity lists current/recent permitted work before optional Diagnostics. A known
job can open its canonical task; legacy run IDs are never guessed to be jobs.
The coordinator retains at most 200 link identities from its existing jobs read,
not another history store. Statistics failure does not hide available history.
Settings groups Preferences, Workspace and Diagnostics, with one service identity
section. Desktop-local paths are labelled separately from external service storage.
Tools and Connections share labelled headings, list states and action roles;
their administration remains outside the ordinary task composer.

**Known work** in the header links to existing tasks and conversations. It is a
bounded list, not a complete count of workspace activity: up to 16 nonterminal
parent tasks and 16 conversations seen in this tab, with task overflow linked to
history. `lib/active-work.tsx` holds only reference/status/title projections in
actor/workspace-scoped memory. It does not store execution state or authorization.
Task views supply their existing history reads; away from them, one paced read
loop refreshes jobs and at most four unmounted known chats per cycle. It never
enumerates all conversation turns. Mounted chats claim their own follower, and
terminal task followers stop polling. Logout, role/workspace changes and lost
access dispose scoped references and subscriptions. Opening work changes the view,
not the work; it never submits, approves, cancels or starts audio.

Voice preferences use typed nine-locale resources, show readiness before selection,
and keep removed model/voice choices visible as unavailable instead of silently
substituting another voice. Automatic selection still exposes the available voice
choices. Dismissing preferences cancels its status read, not accepted work.

Agent replay cursors are scoped to actor, role and workspace. Scope changes discard
the cursor cache and reject late snapshots from the previous scope; refreshing the
same scope retains its cursor. These are read-side guards, not authorization.

`WorkspaceDraftProvider` owns the actor/workspace draft namespace; `drafts.ts`
handles storage and cross-tab invalidation. New composer drafts and active-chat
selection never fall back to an account-only key. With unknown workspace identity,
drafts stay in session memory and the UI discloses the reload limitation.
**Restore previous drafts here** explicitly copies legacy chat/task text, preserving
originals. Live destination values (including cleared drafts) take precedence over
separate recovery copies, including during concurrent writes. The restoration
decision is recorded only after storage succeeds. No request, selection, approval
or computer-session record is restored. **Keep separate** dismisses the offer for
that account/workspace without deleting anything. Drafts remain unencrypted browser
data; this is scope isolation, not a new secret store.

- Application access distinguishes discovery from consent. Whole-desktop access
  uses trusted local consent; explicitly enabled session access can cover later
  tasks until revoked or the session ends. Restricted compatibility sessions may
  still select a window. Neither discovery nor inspection grants authority or
  submits another task. Native consent and service enforcement remain independent
  of renderer presentation.
- Chat distinguishes a pending Knowledge check from a failed check or disabled
  retrieval. Setup and retry appear next to the Knowledge control rather than as
  a permanent unavailable warning in ordinary chat. A previously selected
  Knowledge request stays selected through refresh failure; it never silently
  becomes ordinary inference. Checking again only reads status.

- Command palette height includes its top offset and header; only the result list
  scrolls. Search focus is an inset underline inside rounded chrome, not a square
  border across the shell. Arrow-key selection scrolls only the results viewport;
  pointer hover does not pull the scroll position. Search exposes combobox semantics
  and the Esc control is clickable. Small-height, narrow, RTL and dark/light layouts
  are covered in `tests/palette-layout.spec.ts`.
- Agent work starts in Chat using a reusable profile configured in Agents. Activity
  provides a full-width inspector for the same canonical job, not a second
  composer. Answers and artifacts lead; optional execution details are expandable,
  while actionable approvals and failures retain their existing recovery rules.

- History panels separate the heading/actions, search, guidance, and results with
  a shared vertical rhythm. Loading and failed reads are not empty histories.
- The command palette and mobile conversation drawer contain keyboard focus,
  respect composition input, support Escape, and restore focus on close.
- Workspace controls follow the service's authentication mode and current role.
  Restricted screens explain access requirements without querying administrator
  endpoints. This is presentation, not an authorization boundary: the service
  still validates every request. Current agent/MCP management remains admin-only.
- Model searches, connector drafts, history filters, and Activity selection persist
  across page navigation in account/workspace-scoped memory. They are cleared on sign-out
  or expired authentication; they are not persisted across reloads. Existing chat
  and task draft persistence uses actor/workspace-scoped keys. Connector URLs are not saved to disk.
- Changing models invalidates pending integration setup responses. Statistics and
  Activity history load independently; event failures retain their own error state.
- Chat recovery controls name their action: refresh history, check request status,
  or retry cancellation. Checking status never resubmits inference. Knowledge is
  enabled in chat only after a successful readiness check; unknown index status is
  never reported as ready.
- Bulk deletion operates on the confirmed snapshot and shows processed counts.
  Stop finishes the current request, then leaves remaining entries untouched.
- Desktop startup and custom menu labels share the nine-locale presentation
  resource in `apps/web/public/desktop-presentation.json`. Validated locale/theme
  preferences are saved by trusted host IPC, independently of backend availability.
  Technical startup failures remain available under details. Native OS menu roles
  follow the platform. Translation key coverage is not speaker qualification.

`tests/interaction-contracts.spec.ts` covers these renderer contracts alongside
the existing history, recovery, download, and layout suites. Desktop presentation
tests exercise startup state/locale/theme handling; they do not replace installed
application tests on Windows, macOS, and Linux.

## Migration rule

Returning to a known conversation reconciles its saved current turn; it never
resubmits a prompt or replays speech. A missing recovery snapshot retains the
draft and explains the limitation rather than making an uncertain Send available.
Switching conversations discards late events from the old view. Job followers
ignore older durable event cursors, clear controls on revoked/deleted work, and
stop polling terminal jobs. Navigation aborts only view subscriptions, not jobs;
Stop remains an explicit command. Legacy services without durable cursors cannot
provide the same ordered-replay guarantee.

Task presentation is split into small existing-contract consumers:
`TaskApproval` handles visible expiry and exact review, `TaskResult` presents
saved output and owner-scoped bounded downloads, and `TaskContinuation` owns
instruction drafts/request recovery. `TaskControls` retains lifecycle dispatch
and evidence/context details. Each result/continuation/details sibling has a
distinct resource key so rerenders cannot duplicate panels or reset a draft.
Current phase and Stop stay visible; optional timing, preview and execution
payloads are subordinate. An expired approval's Refresh is strictly read-only.
Unknown operations preserve their exact canonical arguments instead of invented
friendly consequences. Pending and unconfirmed stopping are distinct from a
saved terminal state; no action is described as undone.

The result view distinguishes model output from file integrity. Artifact reads
retain authentication, cancellation, the current 128 KiB service limit and
digest/size checks. Text, Markdown, JSON and CSV files up to 64 KiB offer an
explicit in-place Preview. Preview checks the bytes again, decodes strict UTF-8
and uses inert text or the safe Markdown renderer. It does not execute HTML,
load remote images or fetch a file until requested. Closing clears the content
and cancels pending reads; reopening checks current authorization again.
Unsupported or larger files retain Download. All downloads remain attachments.
Errors stay with the affected file. A supported correction targets the
same task with its existing request ID; unsupported continuation explicitly
links back to Chat without resubmitting or overwriting another draft.

New chat and conversation selection remain available while a reply is running.
Navigation detaches the view, not the saved work; Stop is the explicit cancellation
action. Returning reconciles the original turn and refreshes its saved messages,
including when the background coordinator already observed completion. Late
events and Stop acknowledgments stay bound to the originating conversation.
History browsing is a separate presentation component; deletion remains guarded.

The model picker searches readable names and exact installed IDs. Each option
retains its exact ID and shows only reported runtime readiness, loaded state and
context capacity. Selection neither loads nor downloads a model. Unknown
readiness is not presented as qualified. Search, arrow keys, Enter and Escape
work within the bounded popover, including narrow and RTL layouts.

Recovery guidance follows saved task states rather than interpreting error prose.
Connection errors offer a read-only connection check. Interrupted work explains
its checkpoint or expired desktop access, and unavailable checkpoints are explicit.
Expired desktop access does not also offer a conflicting Resume button. Failed
work keeps its original reason and any saved output; Refresh is not offered as
an execution repair. Uncertain effects still require inspection and reconciliation.

Specialist activity shows each observed child's actual state under the parent,
with dependencies and returned output behind an individual disclosure. The first
four children reuse the existing authorized read-only followers; larger legacy
plans retain explicit inspect links for the others. A child result is shown only
when its returned parent identity matches. Child failure or completion never
changes the parent's displayed outcome or adds independent lifecycle controls.

This redesign is incremental, but new screens must not extend the legacy visual
language. When materially changing a feature, move its styles into the
workspace layer, preserve the semantic tokens, include loading/empty/error and
unavailable states, and test its smallest supported width. Once every feature
has moved, remove the superseded declarations from `styles.css` rather than
maintaining two permanent implementations.

### Presentation and learnability refinement (2026-10-30)

The additive semantic scale in `tokens.css` owns type roles (12/13/14/16/18/24),
400/500/600 weights, UI/prose line heights, spacing, panel/editor radii and
120/180 ms press/entrance motion. Arabic receives its own line-height and tracking
values. Control geometry remains 40px/8px/14px/8px; the composer is a composite
editor with a 16px radius and the existing 800px measure. Reduced motion disables
entrances and press movement. Streaming tokens never trigger a new entry effect.

Code headers derive a bounded language label from Markdown metadata; Copy still
copies only the source. Links, raw HTML handling and inert images retain their
existing safety contract. Popovers and onboarding use transform/opacity, without
backdrop blur. Palette and disclosure keyboard/focus behavior is unchanged.

Workspace options has an explicit visible label, with the signed-in identity
inside. Settings also exposes the existing language preference. Models explains
the selected category or online discovery; Knowledge explains indexing versus
actual use. Profiles link to Chat and Activity without starting work. Cold model
loading has a plain explanation and the existing Stop control. New copy lives
in `Messages.guidance` in all nine locales; native review remains outstanding.

See [the heuristic findings](../ux/findings.md) and
[terminology and translation review](../ux/terminology.md). Palette freeze proof:
`node scripts/test/check-ui-colors.mjs 7357acae431e40092fd40a5458207d3649d503e8 --gradients --compact`.
Later explicitly approved gradients are audited separately; no palette token may
change. Source/API, permissions, task identity and recovery are not styling knobs.

See [tonal treatments and qualification](../ux/gradients.md) for the flat-mode
switch, contrast measurements and the outstanding throttled streaming limitation.

Installed-app feedback supersedes the uniform desktop control size above:
fine-pointer secondary actions are 32px and primary/form controls 36px, with
44px coarse-pointer targets retained. Workspace options is grouped into compact
rows. Agent editing uses two columns when space permits, with purpose examples
and identity/model fields alongside optional capabilities. Tools, Knowledge and
collaborators have labelled disclosures with selection counts; sections with
existing selections open initially, and collapsing them does not remove anything.
Skills/collaborators remain outside Advanced. Narrow layouts stack in reading
order. Save follows the fields without covering them. No resource, grant or draft
is changed merely by opening the form. Explicit Use in Chat selects the saved
revision and does not automatically submit a task.
Chat-history scrolling keeps the footer fixed and reserves its scrollbar lane.

Language/embedding discovery separates curated catalog browsing from Hugging Face
search. Catalog filters and comparison shortlists remain scoped to category and
workspace. Up to three models can be compared using supplied size, RAM estimate,
quantization and license metadata; missing values remain unknown. Catalog RAM
estimates are not measured runtime fit. Comparing or filtering does not download
or select a model, and filtering never hides ongoing downloads. Speech packages
retain their existing reviewed variant/installation flow.

Knowledge supports selecting one document or a temporary collection of up to
sixteen ready documents, then asking about those sources in Chat. This preserves
the current draft and submits nothing during navigation. The composer names the
selected sources; removing them turns retrieval off instead of silently expanding
to the whole library. Selection belongs to that conversation and workspace.
An older service without `scoped-knowledge-chat-v1` cannot accept this request.

The service checks the selection and fails if a source is unavailable or there
are no matching passages. Agent turns can only narrow documents already attached
to the pinned profile, including subordinate agents' effective resources. Saved
responses expose the actual bounded passages supplied to inference, with source
and chunk identities and content digests. Passages render as inert text; they
help the reader evaluate an answer, but do not prove every generated claim.

Skills can be searched by name or description before attachment. Inspect shows
the exact revision, inert source files and an example request. Drafts remain
inactive until explicitly reviewed; review does not attach or execute them.
Attach selects that revision in the profile draft, and Save agent persists it.
Use in Chat selects the saved agent without submitting the example or replacing
the conversation draft. Unavailable pinned revisions stay visible with refresh,
inspection and removal actions; a newer revision is never silently substituted.

Specialist activity shows each subordinate task's actual state, dependency names
and returned draft or files under the parent task. At most four child followers
run automatically. Inspection does not start, approve or retry a child, and
conflicting drafts remain attributable rather than becoming a claimed conclusion.

Streaming and long saved Markdown responses parse in a local worker with one
active parse and at most one pending revision per mounted response. This keeps
parsing off the main interface thread. Raw HTML and images remain inert, and
copying uses the complete canonical response. If the parser fails or stalls,
the source remains readable as plain text. This does not change inference speed.

The composer groups agent/model and workspace choices above a separate voice and
Send/Stop action row. Secondary settings retain visible labels but use quieter
controls. An agent's pinned model has a readable label and a read-only disclosure
of its exact identifier and profile revision; opening it cannot switch models.
Folder scope and tool restrictions remain explicit. Voice settings, microphone
capture and automatic speech opt-in remain distinct, with no automatic capture.

Model choices use readable names and searchable lists, while preserving exact
model IDs and package revisions for inspection. Speech preferences, embedding
setup and multi-variant discovery use inline lists that remain keyboard-accessible
inside their existing panels. Search and selection never start a transfer, capture
audio or submit a Chat draft. Escape closes the list before its containing panel.
Unavailable saved choices remain visible rather than silently selecting another.
Installed, runtime-ready, loaded and smoke-tested states remain distinct; a speech
smoke test does not establish quality or latency. Voice controls and recovery
messages now have explicit resources for all nine locales; independent speaker
review remains outstanding.

Sign-in identifies the connected service and distinguishes its accounts from the
computer's login. Desktop metadata contributes only the service origin; URL
credentials, paths and query parameters are not displayed. Password visibility is
an explicit local toggle. Failed sign-in preserves the entered details and offers
service/account guidance; it does not imply that OffGrid has an automatic password
reset flow. The authentication API and credential destination are unchanged.

Tools loads its own inventory independently of optional external-agent setup.
Connections keeps MCP inventory errors separate from external-provider readiness
errors, with a read-only retry beside the failed section. Editing a connection
draft clears the previous test result; pending test/connect operations hold the
submitted fields stable. None of these views grants tools to an agent implicitly.

The 2026-10-31 review replaces blanket outlined actions with shared visual roles.
Primary actions stay solid, secondary actions use an existing tonal surface, and
text/utility actions keep visible labels with hover and keyboard focus. Input
fields and floating panels retain their boundaries. Forced-colors mode restores
explicit secondary/selected boundaries; coarse-pointer actions retain 44px targets.
No palette color was added. Removing decoration does not remove grouping:

| Surface | Organization and boundaries |
| --- | --- |
| Chat | The editor has one outer boundary. Agent/model/workspace choices are grouped together; context and voice settings share a lower utility group opposite microphone and Send/Stop. Response actions are quiet. |
| Agents | A wider profile list sits beside the unboxed editor; instructions and resources share the available desktop width. Existing profiles prompt selection rather than creation. Tool and skill rows use separators instead of nested cards. |
| Knowledge | Add document and library management sit in the heading. Ask using Knowledge has its retrieval-scope explanation immediately beneath it. Document inspection and management remain beside the source they affect. Empty filters clear locally and restore focus. |
| Models | Installed counts describe inventory, not readiness. Selected tabs/categories use surface and weight, with explicit accessibility states. Comparison remains compact until there are selected models. Discovery and downloads remain separate. |
| Activity | Recent tasks, schedules and diagnostics align to one content width. Empty history can return to Chat without submitting the retained draft. |
| Settings | Labels and bounded controls align in two columns when space allows and stack on narrow screens. Service identity is shown once; account permissions and diagnostics retain their existing scope. |
| Sign-in and restricted areas | The service address stays visible without an extra nested card. Permission notices explain who can help and provide a draft-preserving return to Chat. |

Empty/search/error states remain distinct. Filtering a model, source or profile
never removes the underlying record, submits work or changes an existing editor
draft. These renderer checks do not qualify native screen readers or platforms.

### Images alongside text and voice

Ordinary Chat accepts PNG/JPEG files through **Add images**, clipboard paste or
drag-and-drop. Previews and remove controls stay above the text input; attachment
and voice controls share the composer. Up to four images of 2 MiB each are allowed
per turn (8192 pixels per edge, 32 megapixels). A conversation is additionally
bounded to 12 MiB of encoded image data; start a new conversation if it fills.

Images require an installed local vision model and matching projector. File
presence is not proof of visual accuracy. Unsupported models, older services and
agent turns show a limitation without discarding images, switching models or
downloading anything. Plain text and voice remain available for text-only chats.

Accepted images are stored with the scoped conversation and displayed after
reload. Unsent images use the same actor/workspace/conversation draft boundaries
as text. If browser storage fills, an explicit warning says to keep the window
open: the preview remains in memory but cannot be promised after a restart.
Images are sent to the connected OffGrid service only on Send; remote image URLs,
SVG and camera capture are not supported. Do not confuse an attached image with
permission for desktop control or a qualified visual-agent workflow.
