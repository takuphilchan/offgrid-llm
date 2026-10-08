# Workspace UI architecture

OffGrid presents one product surface across the browser and the Electron
desktop application. The desktop application securely hosts the same compiled
React UI rather than maintaining a fork. A change to a workflow should therefore
work in both environments unless it is explicitly guarded by a desktop
capability.

## Product structure

The primary navigation is organized by user intent:

- **Work** contains Chat and Agents.
- **Library** contains Knowledge and Models.
- **System** contains Activity and Settings.

Feature behavior belongs under `web/app/src/features/<feature>`. `App.tsx` owns
only shell-level state: routing, authentication, service reachability, the
selected chat model, locale, theme, and onboarding. API access remains in
`web/app/src/api/client.ts`; feature components should not duplicate fetch and
error-decoding logic.

The compact shared header shows the page title, quick actions, refresh, and
**Workspace options** (the settings icon). Workspace options contains language,
appearance, and signed-in account controls. Settings is the single destination
for service, workspace and diagnostic details; the sidebar footer reports
connection status without another service-details button. The desktop sidebar
can be collapsed with its navigation button; `offgrid.sidebar.collapsed` stores
only this presentation choice. Icon-only links retain accessible names and
current-page indication. Mobile keeps all six destinations regardless of that
preference. These controls do not grant permissions or restart active work.

The header Refresh control reloads the current page's data without remounting its
editor or clearing drafts. Normal lists do not repeat it in each section heading.
Scoped recovery controls remain beside failed reads, expired approvals and
uncertain operations, where their distinct purpose is needed.

`components/shell` owns header/navigation presentation and shell geometry;
`lib/navigation.ts` is the shared destination list for desktop, mobile, and quick
actions. Authentication, service refresh, and page identity stay in `App.tsx`.

Complex features use a small second level of deep-linkable navigation. Agents,
for example, keeps task work, tool administration, and MCP/external-agent
connections in `#/agents/workspace`, `#/agents/tools`, and
`#/agents/connections`. Browser history and reload must retain these views.

## Visual system

`components/WorkspacePresentation.tsx` owns reusable section headings, action
groups, list search, empty states and scoped notices. These are presentation-only:
features retain request cancellation, draft state, deletion eligibility and
authorization. Chat, task-first Agents and Activity use them; corresponding layout
lives in `components/workspace-presentation.css`. Empty, loading and failed reads
must remain distinct. Controls continue to use the final `controls.css` layer.

`styles.css` retains the existing feature layout while `workspace.css` owns the
current tokens, typography, component treatment, and feature responsive
overrides. `components/shell/workspace-shell.css` exclusively owns shared shell
layout, including header/sidebar size variables used by Chat. New visual work
must use the semantic custom properties in the
workspace layer instead of introducing literal feature colors.

`controls.css` is the shared, final control layer for both renderer editions.
`layers.css` is imported before any application modules and declares layer
precedence once. Feature CSS must not establish layer order accidentally through
module import order (especially mobile overlays and lazy-loaded routes).
It owns button/field geometry and interaction states; feature rules must not
redefine those values. Default actions and fields use a 40px minimum height,
8px radius, 13px text and 8px action spacing. Text can wrap and controls can
grow; do not set fixed heights that clip translated labels.

- Use `primary-button` for the main action, `secondary-button` for alternatives,
  and `danger-button` for destructive operations. Legacy `text-button` call sites
  use the same outlined secondary treatment: actions have a visible boundary
  before hover. Destructive actions still require their existing confirmations.
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
Playback opt-in, microphone and Send/Stop stay in the composer footer.
Reading text is limited to a 760px column, with 16px/1.7 body typography and
an 800px compound composer. The 64px desktop header and 24px vertical page
spacing leave room for work while preserving the 40px control contract.
Task-first Agents uses the same editor/control/footer grammar, with a labelled
prompt, selected model, voice settings and Start task. It does not require an
execution mode or app selection before describing work. The new-task editor is
not nested inside another bordered card, and an empty history does not offer a
redundant New task action. Chat and Agents share the voice
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
- The toolbar Refresh updates the mounted page through `useWorkspaceRefresh`.
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
cd web/app
npm run api:check
npm run check
npm run build
npm run test:e2e

cd ../../desktop
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
Tools and Connections share labelled headings, list states and outlined actions;
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

- Application access distinguishes discovery from consent: **Choose an
  application** lists windows; **Allow access and continue** attaches only the
  selected, locally consented target to the existing task. An empty installed-app
  picker cannot launch or grant access. Neither discovery nor opening an app
  submits another task.
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
- Agent work is task-first: compact runtime facts, a prompt-led composer, secondary
  model/style settings, and more width for results. At roomy desktop sizes composer
  and result panels align; narrow layouts stack without shifting controls as output
  grows. Selected history has a visible state. Execution steps are expandable after
  completion, while live output and approvals retain their existing recovery rules.

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
  resource in `web/app/public/desktop-presentation.json`. Validated locale/theme
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
digest/size checks. All formats download as attachments, not embedded active
content. Errors stay with the affected file. A supported correction targets the
same task with its existing request ID; unsupported continuation explicitly
creates a new local draft without submitting or overwriting another draft.

This redesign is incremental, but new screens must not extend the legacy visual
language. When materially changing a feature, move its styles into the
workspace layer, preserve the semantic tokens, include loading/empty/error and
unavailable states, and test its smallest supported width. Once every feature
has moved, remove the superseded declarations from `styles.css` rather than
maintaining two permanent implementations.
