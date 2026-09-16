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

## Migration rule

This redesign is incremental, but new screens must not extend the legacy visual
language. When materially changing a feature, move its styles into the
workspace layer, preserve the semantic tokens, include loading/empty/error and
unavailable states, and test its smallest supported width. Once every feature
has moved, remove the superseded declarations from `styles.css` rather than
maintaining two permanent implementations.
