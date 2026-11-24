# Proposal

## Why

The managed browser still closes every extra tab and blocks every second origin, preventing ordinary research and multi-site workflows. The approved M2 program requires bounded task tabs and explicit network scope without weakening the effect/recovery boundaries just repaired.

## What Changes

- Add up to eight task-owned managed tabs with stable identities, tab discovery, open/select/close operations and serialized execution.
- Admit an explicit locally consented list of HTTPS origins, including required resource origins, and pin every destination at the CONNECT relay. Preserve the one-origin legacy contract when no wider scope is selected.
- Bind tab/document observations to their exact target, reject stale or foreign references, and preserve completed action receipts across follow-up read failures.
- Negotiate workspace tools and scope through the existing companion transport; expose scope in CLI and desktop consent, with compatibility denial rather than silent downgrading.
- Record source/fixture evidence separately from the real-model browser qualification. Signed-in attachment, frame/custom-control support and visual fallback remain required subsequent work in M2, not removed or certified by this increment.

## Capabilities

### New Capabilities

- `browser-task-workspace`: task-owned browser tabs and explicit network scope.

### Modified Capabilities

None. Existing task authority and browser effect contracts remain applicable.

## Impact

Companion browser/network/session modules; Go browser session capability negotiation and tool contracts; desktop/CLI consent surfaces; tests and browser documentation. Reuses Playwright, the existing hub and durable runner. No model downloads, personal browser access, live installation or publication.
