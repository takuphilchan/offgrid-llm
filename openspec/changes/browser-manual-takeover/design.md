# Design

## Context

See proposal.md. `BrowserWorkspace` already serializes operations and owns only
its isolated context. `CompanionSession` records dispatch results, but its stop
path always closes that context. `ComputerRuntime` owns one utility process.
The shared `ConversationWork` currently calls service takeover and host Stop in
parallel. The canonical runner already expires a takeover grant and exposes
reconnect and uncertain-call reconciliation.

## Goals / Non-Goals

Retain the browser resource while surrendering automation authority. Reuse the
existing runner, journal and reconnect path; browser retention is not a persisted
grant. Personal profile attachment and live changes to the original origin/egress
set remain separate increments.

## Decisions

- Add a synchronous workspace admission fence followed by a bounded drain of its
  serial queue. In-flight input is not undone. Manual-ready is published only
  after the queue and companion result handling settle. Reject reads, captures,
  preparation and input during handoff/manual control; dispose old observations,
  cursors and handles after draining.
- Separate resource close from explicit handoff. Only a trusted host takeover
  request can retain the driver/demo; ordinary disconnect, failure and Stop keep
  existing cleanup behavior. The utility process stays owned and can close all
  retained resources even after the former session has ended.
- Ask the host to quiesce first, then pause/revoke through the existing service
  takeover command. Always attempt service revocation even if local confirmation
  fails. A timeout is unconfirmed, never permission to type concurrently. This
  permits completed replies to persist before revocation without creating a
  second durable controller. Already uncertain work still needs reconciliation.
- Resume through a new locally consented pairing on the same service, actor and
  workspace. Validate the saved task's pending access and previous-session ID
  through the authenticated service. Carry exact retained origin/network scope
  from the host, not renderer claims. Do not automatically navigate, recreate a
  closed tab, reuse an observation or replay a pending action. Renewed authority
  is bound through the existing task attachment path.
- Negotiate `browser-task-pairing-v1` on the service and
  `browserManualProtocol: 1` on the desktop/utility bridge. Add task-bound browser
  protocol 1 enrollment with `run_id`, `input_id`, `previous_session` and
  `approval_mode`; preserve generic `{}` and native protocol 3. Copy the stopped
  session's exact scope and publish the renewed actor/run/input/workspace binding
  atomically. Another task cannot reserve the renewed grant before attachment.
- Retain at most one manual workspace in its existing child process. Stop,
  renderer/host loss, confirmed actor/workspace change and close remain terminal.
  An offline service cannot resume automation. A lost resume reply is recovered
  by observing host/service state, not pairing twice.
- A failed browser close retains cleanup ownership and reports unconfirmed Stop.
  Only confirmed cleanup acknowledges Stop or exits successfully. Preserve the
  exact target through host errors so explicit scoped retry remains available.
  Confirmed login denial retires the browser; transport outage alone does not
  erase the resource or authorize continuation.
- Advertise an additive desktop bridge version. Native control and older hosts
  keep their existing supported takeover semantics. No page or model tool can
  invoke manual resume, grant consent, supply a native path or change the origin
  allowlist. Human credential entry stays in the browser and is never copied
  through renderer, companion messages or journal records.

## Risks / Trade-offs

- Takeover while input is running: drain first, preserve its recorded result;
  unconfirmed retirement or service uncertainty stays visible and non-replayable.
- A human replaces/closes tabs: clear all issued evidence; require fresh explicit
  selection where the current tab disappeared. No fallback to another user page.
- Lost IPC/consent replies: correlate requests with child/generation/session;
  reject stale responses and retain ownership until Stop is confirmed.
- New authentication redirect outside scope: existing egress policy still
  blocks it. Show the exact retained scope; do not silently permit all sites.
- Manual state during service outage: local Stop still works; renewed pairing
  requires current service and principal proof.

## Migration Plan

Additive host capability and runtime payload. Existing sessions/installed state
are not migrated or automatically resumed. Rollback loses the new manual-resume
feature, not saved task history. Record real Chromium/host/renderer evidence and
all unavailable native, model and human qualification in the M2 tracker.

## Primary interfaces

[Playwright context lifecycle](https://playwright.dev/docs/api/class-browsercontext)
and [frame lifecycle](https://playwright.dev/docs/api/class-frame) provide resource
ownership and observation handles. OffGrid's explicit handoff, no-replay and
renewed-consent guarantees require the tests in this change; those interfaces do
not provide them automatically.
