# Spec Delta

## Purpose

Allow a browser task to use several owned tabs and explicitly permitted sites while retaining local consent, exact target identity and reliable effect recovery.

## ADDED Requirements

### Requirement: Task network scope is explicit and enforced
The browser SHALL restrict every network request to locally consented HTTPS origins with pinned public destinations. Navigation, redirects and subresources MUST NOT widen that scope. Additional origins require renewed local consent.

#### Scenario: Research spans two approved sites
- **WHEN** the user grants two public HTTPS origins
- **THEN** navigation and resources on both work and an unapproved third origin is blocked

#### Scenario: Redirect attempts private network access
- **WHEN** an approved site redirects to a private address, plaintext endpoint or unapproved origin
- **THEN** no request reaches that destination, including through proxy bypass

### Requirement: Browser tabs have bounded task ownership
The browser SHALL expose stable task-owned tab identities and allow listing, opening, selecting and closing up to eight tabs. It MUST serialize input and reject unknown or closed identities without affecting unrelated browser tabs.

#### Scenario: Work across tabs
- **WHEN** the assistant opens a second approved page and selects the first tab again
- **THEN** the first tab retains its content and fresh observations identify the selected tab

#### Scenario: Capacity and closed references
- **WHEN** the assistant requests a ninth tab or selects a closed tab
- **THEN** the request fails before input and the remaining tabs stay usable

#### Scenario: Concurrent operations
- **WHEN** tab selection and input requests arrive concurrently
- **THEN** execution is serialized and references cannot cause input in a different tab

### Requirement: Observations remain bound to their tab and document
Every negotiated workspace observation SHALL identify its tab and document. Cross-tab references, replaced documents and closed tabs MUST fail before input. Unknown effects MUST retain the existing non-replay recovery contract.

#### Scenario: Reference from another tab
- **WHEN** input uses an observation from a different selected tab
- **THEN** the browser rejects it before dispatch even if element labels and indexes match

#### Scenario: Same-address replacement
- **WHEN** a tab reloads at the same URL
- **THEN** the document identity changes and earlier references are invalid

### Requirement: Workspace contracts require negotiation and local consent
Workspace tools and expanded scope SHALL require service/companion negotiation. CLI and desktop consent MUST display the actual approved sites. Older clients MUST retain their original scope and tools; unsupported expanded scope MUST fail visibly.

#### Scenario: Old service
- **WHEN** expanded scope is requested with an older service
- **THEN** startup rejects the unsupported request without opening a wider browser session

#### Scenario: Old companion
- **WHEN** a companion does not negotiate workspace support
- **THEN** workspace tools are unavailable and the existing one-origin contract remains valid
