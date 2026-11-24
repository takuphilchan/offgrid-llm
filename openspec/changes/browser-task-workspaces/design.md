# Design

## Context

See proposal.md. BrowserDriver owns one isolated page; context routing and a single-destination HTTPS CONNECT relay enforce its origin. CompanionSession serializes commands and journals results. BrowserHub and the durable runner provide task binding and no-replay receipts. See [current architecture](../../../docs/architecture/CURRENT_STATE.md).

## Goals / Non-Goals

Provide managed task workspaces through those existing boundaries. This increment does not attach personal profiles or certify real-model completion. Signed-in extension attachment and the other M2 requirements remain explicit in the program tracker.

## Decisions

- Compose a workspace from page-local drivers in one isolated context. Each driver retains exact element handles, observation state and document identity. A workspace queue serializes commands, including selection; the existing companion loop remains the transport owner. Keeping all commands serialized is deliberately stronger than the minimum per-tab mutation guarantee and fits current model scheduling.
- Issue opaque tab/document IDs, not browser indexes. Explicit selection invalidates the target observation; previous-page observations cannot be applied to identical controls in another tab. List at most eight tabs and bounded metadata. Close only owned pages; never a personal browser.
- Consent to an explicit bounded list of at most 16 origins, including resource domains. Resolve and validate every destination before launching; the CONNECT relay accepts only the resulting host/port pins. Chromium still verifies TLS. No wildcard, IP literal, plaintext or credentials. The existing trusted-VPN exception remains explicit and unchanged. Isolated loopback test support never becomes a production permission.
- Introduce additive `workspace:1` feature negotiation and a separately validated scope. Default legacy clients keep one origin. Updated clients can use task tabs at that same origin; wider origins must be displayed in local consent. Unknown fields/versions fail closed. Service discovery and direct dispatch both check negotiated support.
- Preserve the existing action receipt and repair contracts for tab operations; opening/closing/selecting are mutations, listing is read-only. A failed navigation after a tab was created is an uncertain mutation rather than a safe automatic replay.

## Risks / Trade-offs

- Resource domains can be numerous: expose blocked origins for a renewed explicit consent; never auto-enroll a domain from page content.
- Popups consume tabs: include owned popups within the same cap and scope; overflow is closed without affecting existing pages.
- A new-tab navigation can fail after creation: retain the tab and uncertainty rather than guessing whether the operation occurred.
- More tabs increase memory: the cap is eight, with model/hardware qualification still required before performance claims.

## Migration Plan

Additive negotiation preserves old pair payloads and default tools. Startup rejects requested wider scope if the service cannot enforce it. Service restart invalidates the workspace as it invalidates current computer sessions. No persistent user-data migration or automatic resume is introduced.
