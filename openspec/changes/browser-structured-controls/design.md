# Design

## Context

See proposal.md and the current architecture. The existing BrowserDriver already
retains exact ElementHandles, document handles, target/form digests, bounded
opaque cursors and typed safe failures. BrowserWorkspace owns eight isolated tabs.
The service negotiates discovery/workspace support and journals action receipts.

## Goals / Non-Goals

Extend those owners for structured evidence and exact frame selection. Do not
replace them with a selector-driven executor, broaden network authority, attach
personal profiles or treat a fixture as real-model qualification.

## Decisions

- Reuse page-local observation identities and opaque query-bound cursors. A
  search scans bounded candidates/text per call and returns bounded results;
  empty pages can continue. Literal normalized Unicode matching and a fixed
  role vocabulary prevent arbitrary executable query expressions. A separately
  supplied selector would bypass the observed-target contract and is rejected.
- Keep exact handles for effects, although Playwright recommends locators for
  ordinary test resilience. Automatic re-resolution after a node replacement
  could act on a newly inserted target. Locators remain suitable for discovery;
  dispatch revalidates the retained node and document.
- Walk open shadow roots for relevant value/semantic digests, including host
  ancestry and root-local accessible labels. Closed roots remain explicitly
  unsupported until another qualified observation method exists. Contenteditable
  read-back uses text rather than inputValue. Custom checkbox/switch state is
  checked before and after typed input; arbitrary option/button clicks retain
  consequential classification and are observed afresh after expansion.
- Add frame inventory and explicit frame selection to the page driver. The
  selected frame is an observation context, not a new grant. Keep tab navigation
  explicit and invalidate selected descendants on top-level navigation. Frame
  metadata/cursors bind the tab, frame and document; no fallback when a selected
  frame vanishes. Establish actual origin rather than trusting a base URL,
  element src attribute or page-provided origin string. Unknown opaque/inherited
  cases fail closed with visible coverage until their ownership is established.
- Capture masking must include nested frames and shadow-root protected fields.
  Mask an unavailable frame as a whole or reject capture; do not expose secrets
  because top-level selectors omit a child document.
- Add a versioned structured feature requiring existing discovery/workspace
  contracts. Older services never receive an unknown pair field; older companions
  never receive unsupported tools. Preserve current result and repair semantics.

## Primary-source findings

Read 2026-11-20. [Playwright locators](https://playwright.dev/docs/locators)
describe automatic re-resolution and open-shadow support, excluding closed roots.
[Frame API](https://playwright.dev/docs/api/class-frame) provides frame-scoped
discovery. [Input actions](https://playwright.dev/docs/input) support native and
contenteditable filling. The exact-target, scope and no-replay constraints above
are OffGrid engineering decisions, not guarantees supplied by Playwright.

[Chrome messaging guidance](https://developer.chrome.com/docs/extensions/develop/concepts/messaging)
treats content-script messages as less trustworthy than the extension worker.
Keep this boundary for the later signed-in bridge; a shared DOM contract does
not make website messages an authorization source.

## Risks / Trade-offs

- Large or continuously changing pages: bounded scans, visible coverage, stale
  cursors and existing three-read repair ceiling; never retry input automatically.
- Framework-specific widgets: support observable semantic contracts and report
  mismatch/unsupported controls; no fabricated universal widget success.
- Frame trees mutate independently: retain exact document handles and verify
  parent/selected identity at dispatch, including same-URL replacement tests.
- Richer evidence can exceed replies: retain separate inventory/observation
  budgets and dense Unicode fixtures below existing transport limits.

## Migration Plan

Additive feature negotiation; no stored-user migration or automatic task replay.
Keep the broader M2 tracker open for extension attachment, visual qualification,
file provenance, takeover and repeated real-model browser tasks.
