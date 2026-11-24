# Design

## Context

See [proposal](proposal.md), [current architecture](../../../docs/architecture/CURRENT_STATE.md) and the [dated primary-source matrix](../../../docs/quality/evidence/selected-browser-research-2026-11-20.md). Current `BrowserDriver` and `BrowserWorkspace` own Playwright browser resources. `CompanionSession`, `DispatchJournal`, service task receipts and browser hub authority already provide the execution path. The recent manual-handoff implementation deliberately retains an owned browser; borrowed tabs need a different resource finalizer under that same lifecycle.

No existing OpenSpec change owns selected-tab attachment. This change depends on browser execution reliability, task workspaces, structured-control evidence and task-bound manual renewal. It does not claim those implementations are fully model/hardware qualified.

## Goals / Non-Goals

**Goals:** a concrete selected-tab provider boundary, exact task/connection authority, preservation of borrowed state, same tool semantics, and an ordered feasibility-to-integration qualification path.

**Non-Goals:** another agent executor, an unrestricted CDP service, browser-wide access, profile cloning, importing cookies, modifying global proxy settings, remotely installing extensions, bypassing browser protections, or certifying real-account behavior from fixtures. Existing credentials/payment and consequential-action restrictions remain unchanged.

## Decisions

### 1. Use a browser action and native messaging, with independent desktop authorization

The MV3 extension receives the browser-owned selected `tabId` only from an explicit action. It keeps one owner per attached tab and sends a bounded selection offer to the native host. A fresh random attachment handle binds that offer to the current browser connection, tab ID and lifecycle generation; raw IDs are not renderer authority. Default-profile remote-debugging ports, profile copies and whole-profile automatic connections do not meet this ownership contract.

The desktop presents the pending saved task, exact origins, policy, selected tab's sanitized description and borrowed-tab semantics. It rechecks service identity, authenticated principal, pending access request, maintenance hold and the exact selection after consent. The extension action alone does not approve a task, and a renderer checkbox alone does not attach a browser. Further existing tabs need a new browser gesture; adding already approved origin tabs does not require another policy choice. Opening a task-created tab inside the existing scope uses the normal governed tool and cap.

Minimal proposed manifest permissions are `activeTab`, `debugger`, `nativeMessaging`, plus bounded local `storage` and DNR only if the feasibility implementation requires them. No blanket tabs/history/cookie/storage export permission, externally connectable web messaging or page-origin command channel is planned. Runtime code still enforces scope because debugger authority is broader than activeTab. Package review must verify the actual manifest, not this intention.

### 2. Keep the native host a transport adapter

The browser launches a dedicated packaged native-host entry point. It validates the registered caller and negotiates a versioned framing protocol; stdout contains framed protocol only. It connects to the trusted desktop owner over a user-restricted local IPC endpoint with a fresh challenge and verified peer/process identity. Endpoint discovery and platform checks are part of implementation, never an unauthenticated loopback CDP/WebSocket listener. Service cookies and pairing secrets remain in the existing trusted owner.

Exact extension IDs and native argv are not sufficient authentication. Bind the offer to the current owner connection and trusted local confirmation. A malicious process running with the user's full authority remains part of the documented local compromise threat boundary; do not promise to solve it with a static extension token. Web content cannot invoke native consent or mint a service pairing.

Control messages use the exact bounded metadata contract in [protocol.md](protocol.md): at most 64 KiB UTF-8 with duplicate-key rejection before semantic parsing. The existing 20 KiB journal argument and 48 KiB result bounds remain required for future dispatch. Large captures require a separately bounded, correlated chunk stream with a fixed total decoded-byte ceiling, sequence validation and cancellation, never an unbounded JSON string. The precise capture ceiling must match existing capture limits. Limit pending messages and one outstanding mutation for the workspace; reject unknown fields, versions, direction changes and duplicate sequence conflicts. EOF invalidates dispatch before reconnect handling. The implemented metadata codec is not yet an authenticated or installed host.

### 3. Add attachment identity to the existing task grant

Reserve service/desktop/host capability `browser-attachment-v1` and additive browser feature `attachment:1`, in addition to only the discovery/workspace/structured features actually supported. These are not currently advertised. Do not overload native desktop authority or interpret a generic browser pairing as attachment consent.

The saved access request remains browser mode. Add `browser_source` (`managed` or `attached`) to saved intent and its idempotency digest before asking the user to choose a tab. Omission preserves managed behavior. The trusted initial pairing path validates the saved pending request and binds actor, workspace, run ID, input ID, policy, origins, provider and attachment handle atomically before publication. The local owner separately pins connection/tab generation. Reconnect uses the existing previous-session semantics plus the same attached resource binding; missing prior authority is not synthesized. The current renewed-only `PairBrowserTaskCode` cannot simply be reused with an invented stopped session.

Wire metadata names and ID syntax are fixed in [protocol.md](protocol.md); service admission, stateful grant correlation and authentication remain unimplemented. Untrusted renderers never supply profile paths, raw CDP endpoints, native executable paths or origin expansion during reconnect. Old generic/managed/native pairing payloads remain unchanged and old components reject attachment before any debugger attach.

### 4. Share execution state and browser semantics through a provider adapter

Generalize the current workspace's resource interface narrowly: enumerate its own tabs, select/open/close under policy, dispatch typed browser actions, clear observations, drain, and release resources. Preserve `CompanionSession` and the single `DispatchJournal` as execution owners. Remove concrete-provider checks only where the equivalent lifecycle capability is explicitly negotiated and tested.

An attached provider translates existing typed actions to a fixed reviewed CDP/extension implementation. Factor pure DOM projection, privacy filtering, semantic state digests, origin validation and outcome mapping where useful; do not create divergent copies of policy. Browser-local code is bundled with the extension. The model cannot submit JavaScript, selectors that escape host-issued targets, a CDP method or an arbitrary target ID. The extension checks the grant epoch and exact target again immediately before each effect.

A root debugger session is one selected tab. Track same-process execution contexts and proven child iframe targets independently; recursively establish OOPIF ancestry before using a child. Workers are not implicitly tool targets. Every navigation, document replacement, tab replacement, detach or manual epoch invalidates retained handles/cursors/captures. Actual document origin must be obtained through the same trustworthy isolated-world principle used by managed frames, not URL labels or opaque-origin precursors. Borrowed-tab closing is rejected by `browser_tab_close`; explicit local detach is the separate user action. Only proven task-created tabs can be closed by that tool.

Capture, upload/download and structured features are enabled only after provider-specific parity tests. Required but unavailable capability is reported as incomplete. The provider cannot broaden private-content exposure by returning raw CDP DOM or screenshots when the existing redacted contract fails.

### 5. Make network feasibility a prerequisite with explicit failure handling

The approved origin set stays canonical public HTTPS, at most 16 exact origins including resource origins; page-discovered hosts never auto-enroll. A candidate layered implementation installs exact selected-tab DNR session rules before enabling automated dispatch, verifies installed rules, and uses selected-target request interception for per-request policy and denial evidence. Rule ownership is connection-scoped and cleanup cannot remove another task's rules.

This is a **candidate**, not a demonstrated security boundary. A mandatory isolated feasibility fixture independently counts outbound requests for redirects, subresources, DNS/private rebinding, same/OOPIF contexts, dedicated/shared/service workers, caches, beacons, popups, WebSocket/WebTransport/WebRTC established before and after attachment, and rule loss. Test prevention before effects, not merely emitted network events. A successful page load or remote-IP notification is insufficient.

The first actual Chromium 151.0.7922.34 probe reproduced both a pre-existing WebSocket effect after DNR/Fetch installation and a URL-approved hostname reaching a private listener. See the dated evidence. Both strict containment assertions failed, so this candidate cannot enable production attachment. The broader task 1.1 matrix and a prevention solution remain open.

Do not alter the user's global proxy, service-worker registrations, cookies, browser security flags or unrelated tabs to pass the fixture. Prior autonomous user-profile activity is not OffGrid-owned; nonetheless an OffGrid action must not gain a channel outside its approved protection through that existing state. If required prevention cannot be established, the dependent execution feature stays disabled and the failure is presented for an architectural decision. The roadmap requirement remains open. Framing, selection and ownership components can be developed independently without advertising usable automation. No exception can be inferred from consent wording.

### 6. Retain no-replay and truthful cleanup across the extra process boundary

The companion journals dispatch before sending an effect to the extension. Envelopes include a protocol version, owner connection generation, attachment/grant epoch, canonical action ID and bounded payload digest. The extension permits each action once within the live generation and returns a correlated result. It does not independently retry; restart loses the grant and forces fresh binding. Host journal tombstones survive result erasure and authority renewal. A timeout after sending is uncertain even if the extension later vanishes.

Takeover first fences new work, drains admitted commands and detaches debugger control; only confirmed release enters manual state. No observations occur while manual. Explicit continuation revalidates the same task and exact selected resource, renews authority and discards all stale observations. Stop releases all task-owned debugger sessions, queues and rules; it never closes a borrowed tab or browser. Preserve failed-release handles for scoped retry and distinguish uncertain effect from unconfirmed detachment. Local extension Stop remains available during service outage. Browser/extension/host restart, debugger conflict and authority loss never trigger automatic reattachment.

### 7. Separate isolated development from installed qualification

First build a hermetic extension fixture in disposable Chrome for Testing/Chromium profiles and synthetic authenticated sites. Native transport can be tested through an exact framing subprocess before any registered-host fixture. Real native registration tests run only in a disposable user/VM/environment whose registration roots are provably isolated; do not change live HKCU or live browser manifests for convenience.

The final composed fixture uses real service handlers and saved tasks, trusted desktop consent, native framing, the production extension/provider and actual browser effects. Injection is limited to consent UI automation, fixture services/model responses and disposable paths. Verify preserved borrowed tabs/cookies plus an unrelated sentinel tab, server-counted effects, lost replies, fresh renewal and confirmed detach. Record exact binaries/versions and failures. Installed Windows/macOS/Linux Chrome/Edge, store identities/signatures, real account sessions and real-model trials remain separate gates with explicit authorization/resources.

## Risks / Trade-offs

- Existing-profile network isolation may be infeasible with available tab APIs → blocking probe first; retain requirement, report concrete failure, seek an architecture decision before exposing dependent automation.
- Browser extensions have broad debugger capability → narrow fixed commands, exact session graph, trusted task consent, package inspection and adversarial boundary tests.
- More components create uncertain outcomes → one canonical dispatch journal, per-generation fencing, no reconnect replay and independent effect counters.
- Browser lifecycle APIs and store IDs vary → pinned tested versions, explicit feature negotiation and platform-specific qualification; unsupported paths remain unavailable.
- Manual/Stop cleanup must preserve user work → separate borrowed/created ownership and test detach failures without browser termination as fallback.

## Migration Plan

Add capability contracts without changing legacy defaults. Ship no auto-install or auto-register step from this change's development workflow. Packaging work produces reviewable manifests/install/remove procedures that verify ownership; actual installation/publication remains separately authorized. Rollback disables new admission and detaches existing OffGrid-owned sessions while retaining browser state and durable uncertainty records. Removing the extension or host cannot make an old command executable again.
