# Spec Delta

## Purpose

Find and inspect relevant controls in authorized native observation evidence
without flooding model context or widening computer-control authority.

## ADDED Requirements

### Requirement: Control search is bounded and deterministic

Authorized tasks SHALL be able to search saved native controls by literal name
query, role and supported control properties. Results SHALL use existing
observation-bound identities, deterministic ordering and bounded pagination.
Queries MUST NOT accept executable selectors, scripts or model-supplied paths.

#### Scenario: Useful control omitted from compact model view

- **WHEN** a control exists in canonical evidence but not in the initial compact view
- **THEN** matching search results return that control's exact identity and properties
- **AND** the canonical observation is unchanged

#### Scenario: Unicode names and ambiguous matches

- **WHEN** multiple controls match a case-insensitive Unicode name fragment
- **THEN** results preserve original names and return the matches without silently choosing or activating one
- **AND** exact identifiers remain required for later inspection and input

#### Scenario: Bounded pages

- **WHEN** matches exceed 16 controls or the 4096-byte serialized result budget
- **THEN** the reply supplies a continuation bound to the same observation and query, with explicit omissions
- **AND** paging cannot silently switch to a newer capture

### Requirement: Inspection preserves source limits

Control inspection SHALL return bounded stored properties and paged text where
available, preserving protected-field filtering, truncation and observation
provenance. It MUST distinguish model-view omissions from host-side omissions
and MUST NOT invent hierarchy, text or controls absent from the source.

#### Scenario: Host observation is incomplete

- **WHEN** the host truncated a control tree or text before storage
- **THEN** search and inspection report that coverage is incomplete
- **AND** an empty search does not assert that the application lacks that control

#### Scenario: Text paging

- **WHEN** stored non-protected text spans multiple result pages
- **THEN** the client can retrieve Unicode-safe pages of that saved text
- **AND** the result cannot recover text the host never supplied

### Requirement: Retrieval does not authorize input

Discovery SHALL enforce task ownership and native-session/observation binding.
References from another task or replaced target SHALL be rejected. Retrieval
MUST NOT refresh observation timestamps, focus a window, prepare or dispatch
input, grant access, or override existing freshness and revocation checks.

#### Scenario: Expired snapshot

- **WHEN** a permitted saved observation has aged beyond input freshness
- **THEN** a read-only reply labels its age and historical status
- **AND** later input still requires a valid fresh observation or the existing supported host revalidation path

#### Scenario: Target switched or permission revoked

- **WHEN** a discovery request targets an earlier surface/session after switching, revocation or replacement
- **THEN** live-task discovery rejects the reference without fallback to another target
- **AND** separately authorized history access retains its existing read-only semantics

#### Scenario: Cross-user reference

- **WHEN** another actor supplies an observation identity, alias or continuation
- **THEN** no control content or existence information is disclosed

### Requirement: Discovery contracts are portable and capability honest

The same read-only discovery semantics SHALL support existing Windows UIA,
macOS Accessibility and Linux AT-SPI observation formats. Availability SHALL
depend on supported evidence and current authorization, not an OS name alone.
Unavailable launch, capture, visual-input and Wayland capabilities MUST remain
unavailable until their own implementations and qualification are delivered.

#### Scenario: Existing native adapter

- **WHEN** equivalent observations originate from each supported adapter format
- **THEN** the same filters, pagination, identity validation and coverage reporting apply
- **AND** no v2 selected-window grant becomes task-wide desktop authority

#### Scenario: Unknown observation version

- **WHEN** an observation contains unsupported security-relevant structure or a newer schema
- **THEN** structured discovery returns an explicit unsupported result
- **AND** it does not drop unknown fields and fabricate a supported control view

#### Scenario: MCP composition and context budgeting

- **WHEN** native discovery is added to a task with authorized MCP tools
- **THEN** the MCP tools and their independent policies remain available
- **AND** complete prompt budgeting includes the new schemas without silently increasing model allocation
