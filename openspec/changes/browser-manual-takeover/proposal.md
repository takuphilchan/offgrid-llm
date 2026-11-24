# Proposal

## Why

The existing Take over action pauses the canonical job but revokes its connection
and closes the managed browser. Users cannot finish a sign-in and return to the
same task tabs. Roadmap M2.08 requires an effective human handoff and explicit
continuation with preserved context.

## What Changes

- Quiesce the existing managed workspace before confirming manual control; retain
  its isolated tabs/cookies while disabling agent reads and inputs.
- Preserve and finish recording already-dispatched outcomes where possible.
  Uncertain effects remain unresolved and never repeat automatically.
- Reuse the retained browser only after explicit local consent, renewed service
  pairing, actor/workspace checks and fresh observation identities.
- Keep Stop, actor/workspace change, host loss and explicit close effective during
  manual control. Preserve native/legacy behavior when the new host contract is
  unavailable, with truthful labels and errors.
- Integrate current Chat/Activity controls, keyboard/accessibility and all locales;
  validate actual manual cookie/tab continuity with disposable browser fixtures.

## Capabilities

### New Capabilities

- `browser-human-control`: safe handoff, manual browser lifetime and explicit
  consented continuation of an owned task browser.

### Modified Capabilities

None. Existing task pause/reconciliation, browser workspace authority and
attachment-policy contracts remain authoritative and are reused.

## Impact

Companion/workspace lifecycle, desktop computer IPC and host state, shared task
controls and the existing reconnect/access flow. No new executor or persistent
browser profile. Selected personal Chrome/Edge extension attachment, scope
expansion, general animated visual capture and real-model/human qualification
remain separate required roadmap work; this increment does not remove them.

Authorization is the active full-program goal: complete dependency-ready
increments continuously, without another approval pause. Tests use isolated pages
and synthetic authentication only; personal sessions and installed-app changes
remain outside this execution scope.
