## Purpose

Let people start ordinary conversations and governed tasks without navigating setup machinery, while keeping context, voice and permission choices explicit.

## ADDED Requirements

### Requirement: Prompt-led entry
Chat and Agents SHALL each present one labelled prompt editor and one primary submit action. A ready workspace SHALL NOT require a separate configuration or execution-mode screen before ordinary submission. Secondary controls SHALL remain discoverable without competing with the prompt.

#### Scenario: Ready workspace
- **WHEN** a user opens a new conversation or task with a compatible selected model
- **THEN** the user can type and submit directly
- **AND** no redundant task label, repeated instructional paragraph or history-maintenance policy dominates the entry area

#### Scenario: Missing prerequisite
- **WHEN** the selected model is missing or an explicitly requested capability is unavailable
- **THEN** the interface names the blocker and presents its relevant setup or repair action
- **AND** it neither silently substitutes a model nor silently removes requested context

### Requirement: Coherent composer controls
Model, Knowledge/context and voice controls SHALL be grouped with the composer. Voice input, playback opt-in and speech-model preferences SHALL remain distinguishable. Maintenance and diagnostic settings SHALL be secondary, with visible active choices and accessible button controls.

#### Scenario: Change voice or model preferences
- **WHEN** a user opens preferences while editing a draft
- **THEN** the draft, selection and composition are preserved
- **AND** preferences do not start capture, playback, submission or downloads

#### Scenario: Voice unavailable
- **WHEN** speech is unavailable but text inference is ready
- **THEN** typed submission remains usable and voice explains the relevant limitation only in context
- **AND** no permanent speech failure banner is inserted into every ordinary conversation

### Requirement: Safe contextual setup return
Setup handoffs SHALL preserve the originating draft and work identity, identify why setup is needed, and offer an explicit return. Returning SHALL recheck readiness without automatically sending the draft, enabling Knowledge or changing authority.

#### Scenario: Repair a prerequisite and return
- **WHEN** a user leaves a composer to install a model or configure Knowledge and returns
- **THEN** the original text and supported context choices remain intact
- **AND** the user explicitly submits only after readiness is established

### Requirement: Consistent history management
Chat and Agents SHALL use consistent history selection, search and management patterns. Bulk deletion and protection explanations SHALL be available contextually rather than occupying the empty-workspace primary path. Individual deletion SHALL remain discoverable and preserve service restrictions and confirmation.

#### Scenario: Empty history
- **WHEN** no saved work exists
- **THEN** history shows a concise empty state without disabled bulk-delete controls or a permanent deletion-policy paragraph

#### Scenario: Protected work
- **WHEN** the user attempts to manage a non-deletable entry
- **THEN** the interface explains the reason and preserves the entry without implying that hiding it cancels or deletes it

### Requirement: Usable empty and error states
Each composer SHALL distinguish empty, loading, unavailable and failed states. Known failures SHALL show a specific next action and retained-work status; technical diagnostics SHALL be secondary. Decorative examples SHALL NOT submit work until explicitly chosen and sent.

#### Scenario: Interrupted send
- **WHEN** acceptance or completion cannot be confirmed
- **THEN** the original draft and available partial output remain accessible with their actual status
- **AND** the recovery action checks accepted work before offering a new submission

### Requirement: Workspace-scoped local drafts and explicit legacy restoration
New persisted drafts and active-chat selections SHALL be scoped to actor and workspace. Legacy account-only drafts SHALL be preserved and SHALL NOT be shown in a composer until the user explicitly confirms restoration into that workspace. Restoration SHALL copy only to empty destinations, keep conflicting originals, and grant no execution authority.

#### Scenario: Restore previous drafts here
- **WHEN** legacy drafts exist and the user confirms the one-time restoration offer in an identified workspace
- **THEN** empty destination drafts receive copies while newer drafts and all legacy originals remain intact
- **AND** no task, chat turn, download or permission request is submitted

#### Scenario: Storage failure or unidentified workspace
- **WHEN** local storage fails or the connected workspace has no stable identity
- **THEN** the client retains recoverable drafts without claiming restoration or durable saving succeeded
- **AND** it does not expose legacy drafts or drafts from another identified workspace as a fallback

#### Scenario: Changing accounts or workspaces
- **WHEN** the actor or workspace identity changes
- **THEN** only that scope's drafts and active-chat selection are available
- **AND** the previous scope's restoration decision does not authorize restoration in the new scope
