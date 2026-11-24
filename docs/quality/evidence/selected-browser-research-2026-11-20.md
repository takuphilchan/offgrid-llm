# Selected browser attachment: primary-source reconnaissance

Date: 2026-11-20. Status: planning evidence only. No extension was installed, no native host was registered, no personal profile was opened, and no attached-browser execution was tested. Mutable upstream documentation was read on this date; an implementation must pin and record the versions actually exercised.

## Existing OffGrid boundary

[Roadmap M2](../../ROADMAP.md) and tracker M2.02 require selected signed-in Chrome/Edge tabs through extension/native messaging. The sampled implementation owns newly launched Chromium contexts: `apps/computer/browser.mjs` and `browser-workspace.mjs`. `session.mjs` and `journal.mjs` own dispatch, replay protection and receipts. `internal/computer/browser_task_pairing.go` binds renewed managed sessions to the previous stopped session and the same task. Initial attachment to a borrowed tab needs an additive binding, not a fabricated previous session or generic pairing shortcut.

The existing browser changes explicitly leave selected-profile attachment open. Neither structured CDP frame inspection inside an owned context nor managed manual handoff implements it. This inspection is not a complete security audit of all browser or native code.

## Source findings and concrete implications

| Primary source, inspected 2026-11-20 | Observed behavior | OffGrid implication / required validation |
| --- | --- | --- |
| [Chrome debugger API](https://developer.chrome.com/docs/extensions/reference/api/debugger) | Tab IDs identify debugger roots; allowed CDP domains are restricted. Flattened child sessions are supported from Chrome 125. OOPIF targets and same-process frame execution contexts differ. Detach can occur on tab close or DevTools use. | Prove each descendant belongs to the selected root; route by exact connection/session/frame generation. Negotiate required methods and fail closed on detach or unsupported versions. Test real same-process and out-of-process frames. |
| [Chrome activeTab](https://developer.chrome.com/docs/extensions/develop/concepts/activeTab) | A user gesture grants temporary access to the selected tab; cross-origin navigation or closure revokes that grant. | Use the browser action for explicit selection. Do not infer that activeTab constrains debugger's broader authority or implements OffGrid's multi-origin consent. |
| [Chrome native messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging) | Browser-spawned host uses framed JSON on stdin/stdout and exact extension IDs. Browser/host message limits differ. Host registration paths depend on platform/browser, including separate Chrome for Testing paths from version 146. | Strict smaller application limits, no credentials in argv/logs, EOF revocation and versioned framing tests. Synthetic registration must target a disposable environment; never silently install into live browser state. |
| [Chromium extensions security FAQ](https://chromium.googlesource.com/chromium/src/+/main/extensions/docs/security_faq.md) | Debugger permission is powerful and can bypass ordinary host restrictions. Native messaging's allowed extension ID is not a secure channel against another local binary or an unpacked extension using that ID. | Exact IDs are necessary but insufficient. Bind trusted desktop consent, owner process/connection identity and a fresh task-scoped challenge. State the remaining same-user local compromise boundary honestly; do not call an extension ID cryptographic identity. |
| [Chrome worker lifecycle](https://developer.chrome.com/docs/extensions/develop/concepts/service-workers/lifecycle) | MV3 workers can terminate and lose globals. Native ports and debugger sessions affect lifetime, but failure still needs handling. | Treat transport loss as revocation. Retain action tombstones in the existing host journal; reconnecting a port cannot recreate authority or retry an uncertain input. |
| [Chrome declarativeNetRequest](https://developer.chrome.com/docs/extensions/reference/api/declarativeNetRequest) | Tab IDs work in session rules; those rules are not durable across browser restart/update. Rules affect network-stack requests, not all service-worker-generated or CacheStorage responses. Background requests may lack a tab association. | Probe exact per-tab rule coverage and lifecycle. DNR can be one layer, not proof of complete isolation. Test service workers, background/worker requests and stale rule cleanup without affecting unrelated tabs. |
| [Chrome webRequest](https://developer.chrome.com/docs/extensions/reference/api/webRequest) | Ordinary MV3 extensions lack blocking webRequest permission. WebSocket/WebTransport handshakes differ from established sessions; established payloads are outside this API's interception boundary. | Do not substitute passive request observation for prevention. Explicitly test pre-existing sockets and automation-triggered sends; an observed packet after dispatch is too late to establish denial. |
| [Chrome proxy API](https://developer.chrome.com/docs/extensions/reference/api/proxy) | Proxy configuration changes browser settings, with regular/incognito scopes rather than an individual selected-tab contract. | Do not change the user's profile proxy to imitate managed Chromium's dedicated CONNECT relay. Preserve unrelated traffic and settings. |
| [Chrome default-profile remote-debugging change](https://developer.chrome.com/blog/remote-debugging-port) | From Chrome 136, remote debugging switches do not apply to the default data directory without a different user-data directory; the post recommends Chrome for Testing for automation. | Avoid a default-profile debugging-port shortcut and profile-copy/cookie-export workaround. Synthetic fixtures use explicitly disposable state. |
| [Edge API support](https://learn.microsoft.com/en-us/microsoft-edge/extensions/developer-guide/api-support) and [Edge native messaging](https://learn.microsoft.com/en-us/microsoft-edge/extensions/developer-guide/native-messaging) | Edge documents debugger, DNR and native messaging, but browser/platform support and registration differ; published IDs differ from unpacked IDs and Windows host discovery can fall back to other Chromium registrations. | Qualify Chrome and Edge separately. Pin exact IDs/manifests and preserve foreign registrations; no broad registry cleanup or assumed compatibility. |
| [Playwright extension README](https://github.com/microsoft/playwright/blob/main/packages/extension/README.md) and [manifest](https://github.com/microsoft/playwright/blob/main/packages/extension/manifest.json) | The extension connects existing authenticated tabs with explicit tab selection and per-client ownership. Its current manifest declares broader tab/host permissions than activeTab alone. | Adopt explicit selection and single-client ownership as usability references, not security proof. OffGrid must enforce its own task scope; grouping or a page title cannot grant access. |
| [Chrome DevTools MCP advanced usage](https://github.com/ChromeDevTools/chrome-devtools-mcp/blob/main/docs/advanced-usage.md) | The documented automatic connection prompts for permission but provides access to all open windows in the chosen profile. | Useful for whole-profile developer inspection; it does not satisfy OffGrid's selected-tab authority contract as-is. Do not use it as a hidden fallback. |
| [Browser Use README](https://github.com/browser-use/browser-use/blob/main/README.md) | Current code documentation describes raw-CDP/Python execution, existing-browser or copied-profile modes, and domain restrictions as a guard rather than a sandbox. | Comparison must reflect the current architecture. OffGrid retains typed dispatch, local authority and no cookie/profile copying; a peer's domain feature is not evidence of an enforced egress boundary. |

## Design inference and unresolved gate

The sources support a selected-tab extension transport. They do not establish that an extension can impose managed-browser-equivalent IP-pinned egress on an already active signed-in tab while preserving its profile and unrelated tabs. That is an engineering inference from the documented boundaries, not a demonstrated universal impossibility.

The new [change](../../../openspec/changes/browser-selected-tab-attachment/proposal.md) therefore starts with executable feasibility probes. Separate three claims: exact host/tool target authority; prevention of newly initiated scoped page requests; containment of pre-existing/shared/background traffic and DNS destinations. Evidence for one cannot stand in for the others. Navigation success, post-request remote-IP observation, URL allowlists and browser local-network prompts do not by themselves prove the last two.

If the probes show the approved protection cannot be met, retain that requirement as open and present the concrete failing case and architectural alternatives for review. Do not silently weaken the roadmap, break the user's browser settings, or ship a capability called equivalent because ordinary demos pass. Independently implementable framing/authority/ownership work can proceed only with its production execution path disabled until the dependent gate is satisfied.

## Acceptance evidence to collect

Use an ephemeral profile and synthetic first-party authentication state, with independent server-side effect counters and an unrelated sentinel tab. Record exact browser/extension/native-host/service versions, OS, profile root ownership, permitted fixture origins, command IDs and expected effects. No secret cookie/header/page payload belongs in retained diagnostics.

Required adversarial probes include redirected/private/rebinding destinations; approved and foreign OOPIFs; worker/cache/socket behavior established before and after attachment; popup creation; browser update/restart and extension disable; host/worker EOF; stale document and changed consent; same-actor wrong-task dispatch; lost effect reply; manual takeover and fresh attachment; cleanup that leaves all borrowed tabs, cookies and unrelated settings intact.

Real branded-browser installation, native-host registration/unregistration, store identity/signing, actual account/SSO behavior and real signed-in profiles are distinct external gates. They require explicit side-effect authorization and suitable resources. Synthetic success never substitutes for those or for the roadmap's repeated model/workflow qualification.

## Planning checks

- `openspec --version`: 1.14.0. Existing project root and capability inventory were inspected before creating the new change.
- `openspec validate browser-selected-tab-attachment --strict --no-interactive`: PASS.
- `openspec status --change browser-selected-tab-attachment --json`: all four planning artifacts exist. Its completion flag describes planning, not implemented features; all 12 implementation/qualification tasks remain unchecked.
- `node scripts/maintenance/check-docs.mjs`: PASS, 465 maintained Markdown files, zero problems. This checks local references, not external URLs or heading anchors.
- `git diff --check -- openspec/changes/browser-selected-tab-attachment docs/quality/evidence/selected-browser-research-2026-11-20.md`: exit zero; new untracked files are not covered by this diff check. Strict spec and documentation checks above inspected the created artifacts.
- No application suite, extension runtime probe or browser qualification was run for this planning-only increment. No production files, existing browser-manual artifacts or authoritative roadmap requirements were edited.

## Task 1.1: first runtime feasibility counterexamples

The planning-only statement above describes the earlier checkpoint. After root design review, two strict opt-in runtime probes were added under `apps/computer/test/attachment-network-feasibility.test.mjs`, with a fixture-only MV3 extension and independent HTTPS/WebSocket server. The installed Playwright 1.62.1 full Chromium revision 1234 reports **151.0.7922.34**. The extension loaded in its owned empty persistent profile; no download or native-host registration was needed. The [official Playwright extension guide](https://playwright.dev/docs/chrome-extensions) describes this persistent-context/headless Chromium path; this does not qualify installed Chrome or Edge.

Command (PowerShell, repository root; the fixture README includes exit-code-preserving cleanup):

```powershell
$env:OFFGRID_ATTACHMENT_NETWORK_PROBE = '1'
node --test --test-concurrency=1 apps/computer/test/attachment-network-feasibility.test.mjs
Remove-Item Env:OFFGRID_ATTACHMENT_NETWORK_PROBE
```

Result: **FAIL, two tests, zero passed, 2.632 seconds total**. These are required zero-effect assertions; they have not been weakened into tests that pass when a leak occurs.

| Probe | Verified controls | Actual forbidden result |
| --- | --- | --- |
| WebSocket established before attachment | Three selected-tab DNR session rules installed; an allowed HTTP request actually paused and continued through extension Fetch; a new foreign fetch and new foreign WebSocket handshake were denied; the unrelated sentinel tab still fetched the foreign fixture site | A normal button click sent `effect-after-install` on the already-open foreign socket. The server received one message after both controls were enabled. Required zero-message assertion failed. |
| Approved HTTPS hostname resolving privately | The same installed rule and Fetch checks passed. The request's exact origin was in the candidate allowlist. The unrelated tab remained usable. | A button-triggered POST to `private.offgrid-fixture.example` reached the loopback listener. Server destination and CDP `remoteIPAddress` both reported `127.0.0.1`; one effect was recorded. Required zero-effect assertion failed. |

Both tests awaited browser closure and removed only their validated temporary profile roots. A subsequent process query found zero Chromium processes using the owned `offgrid-attachment-network-` profile prefix. There were no extension API errors in either diagnostic record.

The private-address fixture uses an owned Chromium resolver mapping and self-signed test certificate. It proves this exact URL-filtering candidate does not constrain the connection IP. It does **not** establish public-to-private local-network permission behavior, actual DNS rebinding, or the impossibility of a stronger architecture. The WebSocket counterexample is independent of DNS filtering: a request-time boundary cannot retroactively intercept the established channel's payload merely because a new handshake would be blocked.

Task 1.1 remains unchecked: redirects, additional frame/worker/cache paths, WebTransport/WebRTC, browser lifecycle and a complete prevention solution remain outstanding. Production attachment negotiation and automation are still absent/disabled. These failures block calling DNR plus Fetch an equivalent replacement for the managed CONNECT boundary; they do not justify changing the approved requirement or applying profile-wide settings. Independent bounded framing work can proceed under the reviewed plan without granting browser authority.
