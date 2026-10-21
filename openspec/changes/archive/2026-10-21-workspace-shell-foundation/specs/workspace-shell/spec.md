## Purpose

Provide a quiet, consistent workspace frame that keeps navigation and essential utilities available without distracting from active work.

## ADDED Requirements

### Requirement: Compact shared header
The workspace SHALL display the current page title, quick actions, refresh, and workspace options without repeating the page category and description. Language, appearance, service details, and authenticated account actions SHALL remain accessible through workspace options.

#### Scenario: Change a workspace preference
- **WHEN** a user opens workspace options and changes language or appearance
- **THEN** the preference applies without discarding the current composer draft or restarting accepted work
- **AND** desktop presentation synchronization remains active

### Requirement: Collapsible desktop navigation
The desktop sidebar SHALL offer a remembered expanded or collapsed layout. All six destinations SHALL retain accessible names and current-page indication in either layout. Mobile navigation SHALL remain independent of desktop collapse state.

#### Scenario: Collapse and reload
- **WHEN** a user collapses the desktop sidebar and reloads
- **THEN** the sidebar remains collapsed and all destinations remain usable

#### Scenario: Narrow or RTL workspace
- **WHEN** the viewport is narrow or the selected locale is RTL
- **THEN** all navigation destinations and header controls remain reachable without horizontal document overflow

### Requirement: Preserve existing workspace behavior
Shell presentation changes SHALL preserve authentication, routes, refresh behavior, drafts, monochrome themes, and existing task authorization. Layout preferences SHALL NOT grant access or trigger microphone capture.

#### Scenario: Shell-only interaction
- **WHEN** a user changes sidebar layout or opens utility controls during an unfinished draft
- **THEN** the draft remains unchanged and no task, capture, or approval request is submitted
