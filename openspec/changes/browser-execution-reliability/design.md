# Design

## Context

See proposal.md for motivation and docs/architecture/overview.md for service ownership. The companion journal already claims actions before execution and persists successful replies. Browser exceptions currently discard stage information; the hub treats unknown errors as uncertain. The durable runner already has bounded pre-effect repair. These existing authorities remain in place.

## Goals / Non-Goals

**Goals:** repair browser effect boundaries end to end, preserve compatibility, and remove arbitrary page-size blind spots through bounded discovery.

**Non-Goals:** this increment does not qualify native hardware or replace browser, inference or agent runtimes. The complete approved transformation remains in docs/ROADMAP.md; cross-origin browsing and signed-in attachment require their own authority/transport changes.

## Decisions

1. Use companion-authored typed failures, not substring interpretation of website or provider errors. Mark safe failure only before invoking input, or for allowlisted read operations. Preserve unknown on all other exceptions. Persist safe error replies before acknowledging them; journal tombstones continue to prevent replay across sessions.
2. Separate the action receipt from best-effort follow-up observation. Successful click dispatch is not task verification. A failed read returns a bounded status with no raw page error. Read-only recovery may inspect again; it cannot repeat the action.
3. Reuse the existing durable repair budget for trusted browser failures, explicitly pinning descriptor source/namespace and allowed recovery codes. Reject external/MCP claims of pre-effect safety. No additional executor or parallel browser mutation path.
4. Retain whole-form property digest for submission integrity, but replace global mutation freshness for inputs with target snapshots and exact DOM identity. Retain the observed document handle so a same-URL reload cannot reuse capture/continuation authority. Capture remains whole-observation-bound. Inspect target ancestry/form semantics so changed form destinations and labels invalidate approval. A clock outside the form is irrelevant. Browser actionability continues through Playwright.
5. Add negotiated continuation to observations with bounded text/control pages. Keep the zero-argument legacy observation contract. Use observation-bound continuation and expose a new discovery tool only to capable companions; old packs continue with legacy limits.
6. All clicks remain consequential absent trusted structured semantics. Labels may restrict authority; they cannot increase it. Existing full-task grants still apply.
7. Reuse authenticated service receipt storage for staged browser mutations. Legacy replies lacking stages are not retrospectively interpreted. Pin reviewed browser read intents separately so a lost read does not require mutation reconciliation; transport loss still stops the session and does not manufacture authority to retry.

## Risks / Trade-offs

- More confirmations under scoped-changes mode → explain exact-click approval; do not weaken task grants to compensate.
- Target validation cannot prove website intent → retain consequential click policy and independent verification.
- Dynamic pages can invalidate continuations → explicit stale response and bounded re-observation, never replay input.
- Transport changes may mix old/new companions → additive fields and capability negotiation, with compatibility tests.
- Existing broad program spans hardware and pilot gates → preserve outstanding evidence explicitly; source checks do not certify them.

## Migration Plan

No user-state migration is needed. Existing journal format stores bounded error replies unchanged. Deploy service and companion together for extended discovery; older companions retain current tools. Rollback retains journal tombstones and does not reinterpret failed legacy tasks. Verify packaged runtime membership when adding companion modules.
