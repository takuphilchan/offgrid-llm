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

Complex features use a small second level of deep-linkable navigation. Agents,
for example, keeps task work, tool administration, and MCP/external-agent
connections in `#/agents/workspace`, `#/agents/tools`, and
`#/agents/connections`. Browser history and reload must retain these views.

## Visual system

`styles.css` retains the existing feature layout while `workspace.css` owns the
current tokens, shell, typography, component treatment, and responsive
overrides. New visual work must use the semantic custom properties in the
workspace layer instead of introducing literal feature colors.

`controls.css` is the shared, final control layer for both renderer editions.
It owns button/field geometry and interaction states; feature rules must not
redefine those values. Default actions and fields use a 40px minimum height,
8px radius, 13px text and 8px action spacing. Text can wrap and controls can
grow; do not set fixed heights that clip translated labels.

- Use `primary-button` for the main action, `secondary-button` for alternatives,
  `text-button` for low-priority actions, and `danger-button` for destructive
  operations. Destructive actions still require their existing confirmations.
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
- Model discovery comes before the suggested catalog, which has its own filter.
  Setup panels use bounded controls and one primary action.
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
  across page navigation in account-scoped memory. They are cleared on sign-out
  or expired authentication; they are not persisted across reloads. Existing chat
  and task draft persistence is unchanged. Connector URLs are not saved to disk.
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

This redesign is incremental, but new screens must not extend the legacy visual
language. When materially changing a feature, move its styles into the
workspace layer, preserve the semantic tokens, include loading/empty/error and
unavailable states, and test its smallest supported width. Once every feature
has moved, remove the superseded declarations from `styles.css` rather than
maintaining two permanent implementations.
