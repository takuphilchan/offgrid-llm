## Purpose

Let people start ordinary conversations and governed tasks without navigating setup machinery, while keeping context, voice and permission choices explicit.

## ADDED Requirements

### Requirement: Defaults-first setup and ordinary use
The workspace SHALL offer one guided readiness flow, preserve working selections
and expose advanced configuration on demand. Completed setup SHALL NOT recur
on ordinary turns. Downloads, grants and model replacement SHALL remain explicit.

#### Scenario: First usable local setup
- **WHEN** a new user opens a workspace without required local capabilities
- **THEN** one recommended compatible setup explains prerequisites and offers explicit installation or offline import
- **AND** declining leaves the app usable for inspecting available work without repeated configuration prompts

#### Scenario: Returning user asks about documents
- **WHEN** setup is ready and a user adds supported documents then asks a question
- **THEN** the interface shows per-file readiness and attributable results without requiring embedding, chunking or runtime choices
- **AND** relevant errors preserve the draft and expose one next action with technical details secondary

### Requirement: Progressive advanced tools
The default workspace SHALL prioritize conversation and documents while keeping
administration, overrides and the optional desktop terminal discoverable without
opening them automatically or expanding authority.

#### Scenario: Ordinary task with no advanced configuration
- **WHEN** a user starts a supported conversation using saved defaults
- **THEN** they can submit without opening settings, selecting a profile or starting a terminal
- **AND** active source and access state remain understandable and revocable

### Requirement: Profile-first agent composition and discoverable Help
Chat SHALL use the selected pinned profile's model, resources and limits without
requiring repeated setup. One Agent settings disclosure SHALL show that identity
and offer existing scoped restrictions. Active access SHALL remain discoverable;
neither profile selection nor opening settings SHALL create a grant.

#### Scenario: Submit with existing profile defaults
- **WHEN** a user selects a saved profile and sends a request without overrides
- **THEN** the existing profile ID/revision and resource policy govern the turn
- **AND** ordinary conversation requires no folder grant, and required access is requested through the existing task path

#### Scenario: Inspect or narrow agent configuration
- **WHEN** a user opens Agent settings or narrows an attached resource
- **THEN** the pinned model and decision limit are visible, saved profiles remain unchanged, and the submitted restrictions cannot expand authority
- **AND** active folder/computer access and revocation remain reachable

#### Scenario: Learn and return to work
- **WHEN** a first-time or returning user opens Help from the shell or Quick actions
- **THEN** localized local examples, configuration guidance and recovery navigation are available without submitting work or changing settings
- **AND** dismissal restores focus, drafts survive, and narrow and RTL layouts remain usable

### Requirement: Scoped image input in ordinary Chat
The shared composer SHALL accept up to four PNG/JPEG images of at most 2 MiB
each through explicit selection, paste or drop. It SHALL show removable previews,
preserve text and voice input, and reject invalid formats, oversized or malformed
images without discarding the draft. Image URLs SHALL be local data only.

#### Scenario: Send and reopen images
- **WHEN** a user sends images to a compatible installed local vision model
- **THEN** the accepted bytes and names are saved with the actor/workspace-scoped turn and user message
- **AND** inference receives those images, history displays them after restart, and reconnect does not replay the submission

#### Scenario: Unavailable image inference
- **WHEN** the service, selected model or selected agent cannot support the image request
- **THEN** it explains the limitation and retains the draft
- **AND** it does not discard images, change models, download dependencies or grant access

#### Scenario: Scope and failure boundaries
- **WHEN** a conversation, actor or workspace changes or image storage fails
- **THEN** attachments cannot leak into another scope and unsaved state is explicitly reported
- **AND** unsupported URLs, spoofed formats and excessive images fail before inference

### Requirement: Ordinary CLI image parity
Plain and TUI `offgrid run` SHALL support explicit `--image PATH` and interactive
`/image PATH`, `/images`, `/remove-image [N|all]` with shared PNG/JPEG limits.

#### Scenario: Failed image request
- **WHEN** a file is unreadable or invalid, or inference fails or ends incompletely
- **THEN** the CLI reports the error without submitting an unintended text-only request or recording a completed answer
- **AND** previously accepted pending attachments remain available for inspection or removal

#### Scenario: Continue a saved image conversation
- **WHEN** a successful image exchange is saved and later loaded
- **THEN** its original image bytes remain in typed inference history and terminal history identifies its attachments
- **AND** the selected service enforces local vision readiness without a silent model change

### Requirement: Purposeful control boundaries
The workspace SHALL use shared solid primary, tonal secondary and quiet labelled
utility actions rather than enclosing every action in an outline. It SHALL retain
visible keyboard focus, explicit input and floating-panel boundaries, selected
state, and minimum 44px coarse-pointer action targets. Profile, tool and skill
sections SHALL avoid redundant nested frames while remaining visibly grouped.

#### Scenario: Quiet composer and Knowledge actions
- **WHEN** the user visits Chat or Knowledge in either theme
- **THEN** the main action has distinct visual weight, contextual utilities remain labelled and keyboard reachable, and opening a control does not submit work, grant access or start microphone capture

#### Scenario: Accessibility modes preserve affordances
- **WHEN** the user navigates with a keyboard, coarse pointer or forced colors
- **THEN** focus and selected states remain visible, touch actions retain their minimum targets, and necessary control boundaries are restored in forced colors

### Requirement: Task-focused composer and work retrieval
The composer SHALL prioritize the request, active context and Send/Stop, with
labelled secondary configuration. Workspace search SHALL disclose the authorized
work types it searches and preserve origin/return state. It SHALL NOT imply that
navigation/theme commands constitute searching conversation or artifact content.

#### Scenario: Find previous work while another task runs
- **WHEN** the user searches and opens an authorized work result
- **THEN** the original work remains discoverable and no execution is replayed
- **AND** private results are disposed when the actor or workspace changes

### Requirement: Combined contextual navigation
Chat SHALL use one 248px application/history sidebar with an explicit compact-collapse control and existing state/controllers. Below 1024px the same sidebar SHALL be a dismissible modal drawer with focus restoration. Navigation and the consolidated workspace/settings footer SHALL remain reachable independently of history scrolling. New chat SHALL precede contextual history; history row actions SHALL be available through labelled keyboard/touch-accessible controls without permanent delete-icon clutter.

#### Scenario: Collapse navigation during work
- **WHEN** the user collapses and expands navigation with a draft or running task
- **THEN** selection, draft and canonical work identity remain unchanged
- **AND** navigation remains labelled and usable without submitting, cancelling or replaying work

#### Scenario: Browse history in a narrow workspace
- **WHEN** a user opens navigation, selects a conversation and closes the drawer
- **THEN** the same saved conversation is selected without submitting, approving or replaying work
- **AND** Escape and outside click dismiss the drawer without losing the draft

### Requirement: Progressive controls and intentional following
Chat SHALL keep agent/model, tools, access status and submission controls discoverable while disclosing detailed preferences progressively. Scrolling away from the latest output SHALL suspend following and expose an explicit Latest action.

#### Scenario: Read earlier output during streaming
- **WHEN** the user scrolls upward while output or activity expands
- **THEN** their reading position is preserved until they return to the bottom or choose Latest
- **AND** navigation and submission controls remain reachable without an additional page scrollbar

### Requirement: Prompt-led entry
Chat SHALL present one labelled prompt editor and one primary submit action for ordinary and agent-enabled conversations. Agents SHALL configure reusable profiles and expose canonical work inspection, not duplicate task entry. A ready workspace SHALL NOT require a separate configuration or execution-mode screen before ordinary submission. Secondary controls SHALL remain discoverable without competing with the prompt.

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
