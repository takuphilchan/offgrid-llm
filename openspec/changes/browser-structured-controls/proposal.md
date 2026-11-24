# Proposal

## Why

The approved M2 scope requires finding and interacting with controls on modern
pages. The current managed workspace pages top-level native controls, but does
not provide literal search, explicit frame selection, complete shadow-root
freshness or verified contenteditable/custom-control editing.

## What Changes

- Extend existing bounded observation/continuation and exact-handle contracts
  with literal control/text discovery and explicit coverage.
- Support open shadow roots, contenteditable fields and semantic custom controls
  with state checks and independent read-back; preserve conservative click policy.
- Add task/tab-bound frame discovery and selection, exact frame/document identity,
  navigation invalidation and scope enforcement, including nested frames.
- Negotiate new tools and retain legacy behavior. Use the current companion,
  hub, durable receipts and repair budget; do not introduce another executor.

Signed-in extension attachment, visual action qualification, richer file
provenance and real-model acceptance remain required separate M2 work. This
increment does not claim that structured DOM fixtures complete the browser gate.

## Capabilities

### New Capabilities

- `browser-structured-controls`: scoped discovery and verified interaction with
  frame-owned structured page controls, extending the active browser-execution
  and browser-task-workspace contracts without replacing them.

### Modified Capabilities

None. Existing main-tree native-control-discovery governs native host evidence;
this change retains its bounded, literal, identity-preserving principles for
browser DOM evidence without altering native host grants.

## Impact

`apps/computer` page observation/driver/session and tests; Go computer feature
negotiation, capability/tool descriptors and recovery tests; companion packaging,
user guidance and requirement evidence. No model download or personal browser
access is required for isolated Chromium verification.
