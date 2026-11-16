## Purpose

Let people start ordinary conversations and governed tasks without navigating setup machinery, while keeping context, voice and permission choices explicit.

## ADDED Requirements

### Requirement: Continuous workspace startup
The desktop SHALL retain a consistent local startup surface until the initial workspace is usable or an actionable recovery screen is ready. Startup SHALL report real stages and elapsed time, with gentle brand motion that respects both system and application reduced-motion preferences.

#### Scenario: Service connects before Chat is ready
- **WHEN** the service is connected but model discovery, history, capability discovery or selected-conversation restoration is pending
- **THEN** the loading surface remains visible and the underlying workspace is unavailable to keyboard and pointer interaction
- **AND** it is removed only after these reads settle and the usable workspace or its recovery state has rendered
- **AND** active generation and later background refreshes do not re-enter startup

#### Scenario: Startup dependency fails
- **WHEN** an initial read fails or the application bundle does not initialize
- **THEN** the user receives an actionable recovery state or a local reload control
- **AND** elapsed time and status remain available without claiming successful readiness

#### Scenario: Welcoming startup feedback
- **WHEN** startup remains in progress
- **THEN** a larger animated OffGrid mark and gently changing welcome messages replace the small spinner and static introductory text
- **AND** the messages use the selected interface language and remain separate from actual loading status
- **AND** reduced motion disables movement and fades while keeping textual updates, and hidden windows or recovery suspend the rotation

#### Scenario: Scrolling a focused transcript
- **WHEN** a user scrolls with a pointer, touch or the Latest action
- **THEN** no focus border is drawn around a message
- **AND** keyboard entry to the transcript retains a visible focus indicator on the scroll region

### Requirement: Direct setup destinations and consistent controls
Models SHALL be directly accessible in the primary sidebar. Installed inventory,
curated discovery and explicit online search SHALL use stable visible navigation
across model categories without initiating transfers or searches on navigation.
Settings SHALL use compact section navigation, preserve section deep links and
search, and use shared themed dropdowns with keyboard and RTL support.

#### Scenario: Find models and return to a setting
- **WHEN** a user opens Models from the sidebar and switches inventory, discovery or search
- **THEN** type and filter context remain explicit and downloads require the existing review action
- **AND** typing a repository search does not send it until requested
- **WHEN** a user selects a Settings section and reloads or navigates back
- **THEN** the URL restores that section and its existing saved preferences

#### Scenario: Themed dropdowns
- **WHEN** a user opens a dropdown in a supported desktop host
- **THEN** options, selection, keyboard focus and disabled states use the shared theme
- **AND** RTL and constrained viewports retain usable selection controls
- **AND** hosts without customizable-select support retain native selection behavior

### Requirement: One desktop agent with compact and full presentations
The desktop SHALL expose a configurable global shortcut and tray entry for a
compact request surface using the same authenticated conversation, draft and
saved work as the full workspace and CLI. Presentation changes SHALL NOT create
a second executor, duplicate a request, confer permissions or start capture.
Settings and source inspection SHALL remain reachable in the full workspace.

#### Scenario: Summon, work and expand
- **WHEN** a user summons OffGrid while the full window is hidden
- **THEN** the compact input accepts text and explicit voice activation without requiring the full workspace to remain visible
- **AND** expanding preserves the active draft, work identity and full-window geometry
- **AND** the compact presentation grows for actual work or voice rather than showing all configuration controls at rest

#### Scenario: Bounded quick access with visible model selection
- **WHEN** Quick Access is opened from a maximized full workspace
- **THEN** it uses bounded input geometry and cannot be maximized or freely resized
- **AND** the current model is visible beside input actions, with pinned-profile restrictions retained
- **AND** opening a model or attachment menu temporarily provides enough room to use it
- **AND** expanding restores full-window geometry and native resizing without losing the draft

#### Scenario: Interchangeable voice and text views
- **WHEN** Talk is explicitly started in Quick Access or the full workspace
- **THEN** one voice surface replaces the text view without a visible chat background, dimmed card or modal outline
- **AND** text navigation and composition are hidden and inert while voice is active
- **AND** leaving voice restores the same conversation, draft and focus without replaying or submitting audio
- **AND** Stop and localized status remain visible at small sizes and reduced motion disables decorative movement

#### Scenario: Dismiss, stop and quit have distinct meanings
- **WHEN** the compact surface is dismissed or its shortcut is toggled
- **THEN** microphone capture and playback end and drafts are retained
- **AND** dismissal does not cancel accepted work or quit the service
- **AND** explicit Stop and Quit retain their existing cancellation and shutdown contracts

#### Scenario: Change presentation without exposing intermediate layouts
- **WHEN** a user switches between quick text and voice or expands to the full workspace
- **THEN** outgoing content fades before native resizing and the new view appears after its final geometry is painted
- **AND** rapid reversal or dismissal cancels stale visual callbacks without resubmitting work, replaying audio or losing the draft
- **AND** system and application reduced motion bypass native interpolation and decorative transitions
- **AND** an unavailable renderer acknowledgment cannot leave the native window permanently waiting

#### Scenario: Shortcut collision or unavailable service
- **WHEN** the selected shortcut cannot be registered or the service is not ready
- **THEN** the application reports the unavailable shortcut with a working tray fallback or shows the existing service recovery surface
- **AND** it does not take over another application's shortcut, conceal setup errors or claim readiness

### Requirement: Focused voice surface and purposeful motion
Explicit Talk activation SHALL open a focused, keyboard-accessible surface with
localized capture, permission, processing, playback and error states. Visual
input energy SHALL derive from microphone measurements. Processing motion SHALL
represent an activity state, never fabricated progress or hearing before consent.
Reduced motion SHALL preserve all controls, status and cancellation behaviour.

#### Scenario: Permission, utterance and response
- **WHEN** a user starts Talk and microphone permission is pending
- **THEN** the surface distinguishes pending permission from listening and remains cancellable
- **AND** one completed utterance enters the same saved work without a second concurrent recorder
- **AND** capture pauses during playback and resumes only while Talk remains active

#### Scenario: Escape or hidden surface
- **WHEN** the user presses Escape in Talk or hides OffGrid during permission or transcription
- **THEN** capture is cancelled, late permission releases tracks and late transcription cannot submit a request
- **AND** returning does not reopen the microphone automatically

### Requirement: Configuration is discoverable without occupying the composer
Models, voice profiles, response limits and desktop shortcut settings SHALL be
available in Settings with searchable destinations. The composer SHALL retain
only task context, attachments, input and immediate execution/playback actions.
The model library SHALL expose an explicit browse/back journey and a compact
category selector with independent retained filters and honest runtime readiness.

#### Scenario: Configure a model or voice and return
- **WHEN** the user follows configuration from a draft and changes a selection
- **THEN** returning preserves the draft and selected sources without submitting work
- **AND** local inventory is distinct from network discovery and no download begins without explicit review

### Requirement: Contextual work surface without compulsory project setup
The application SHALL prioritize entering a request and resuming recent work.
Document workflows SHALL remain contextual to selected sources or Library rather
than dominating the default home. A work surface SHALL present conversation, source or result
according to the task with at most two visible content panes. Existing canonical
identities SHALL remain authoritative; opening work SHALL NOT create a new
executor, security workspace or mandatory project. Library and Activity SHALL
remain discoverable independently from the assistant.

#### Scenario: Quick conversation from a ready application
- **WHEN** a user starts a conversation and sends a greeting
- **THEN** no project, folder or execution-mode selection is required
- **AND** one reply state and one primary Send/Stop location are presented without a duplicate prompt and Running card

#### Scenario: Inspect evidence while reviewing a result
- **WHEN** a user opens a citation from a saved result
- **THEN** result and pinned source can be inspected together and the assistant remains reachable without a third content pane
- **AND** switching surfaces preserves unsent text, review drafts, source position and active work identity

#### Scenario: Narrow or enlarged work surface
- **WHEN** the available width cannot support the minimum content pane widths, including at 200% zoom
- **THEN** tabs or drawers expose the same surfaces without squeezed columns or inaccessible primary actions
- **AND** keyboard focus, selected sources and reading position survive the transition

### Requirement: Canonical source inputs and explicit page scope
For advertised capabilities the composer SHALL submit supported text, document,
image and Knowledge references in the same conversation through canonical
admission. Viewed sources SHALL be distinct from selected input. A selected
page/block scope SHALL be visible before submission and pinned to an immutable
revision at acceptance. The server SHALL validate access and scope and recheck
access before generation and source delivery.

#### Scenario: Attach a supported source to an existing conversation
- **WHEN** the service advertises the corresponding input capability and the user adds a permitted source then submits
- **THEN** the original conversation receives one accepted input containing the text and validated references
- **AND** no new conversation handoff, legacy write fallback or silent context omission occurs

#### Scenario: Ask about the current page
- **WHEN** the user pins page 8 of a source revision and submits a question then navigates to page 9
- **THEN** accepted context and evidence receipts still identify the submitted revision and page 8
- **AND** unread figures or missing page text remain disclosed rather than inferred from the displayed preview

#### Scenario: Source unavailable or service too old
- **WHEN** a requested capability is absent, a source revision is unavailable or access has been revoked
- **THEN** the request fails with a specific recoverable state and retained draft/selection
- **AND** it does not substitute another revision, change models, drop the source or bypass the canonical write guard

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
The application SHALL use one collapsible application/contextual-history sidebar
with existing state/controllers. Width SHALL follow shared layout tokens and
content-pane minimums rather than a fixed Chat-only geometry. When available
width cannot support the sidebar and primary content, the same navigation SHALL
be a dismissible drawer with focus restoration. Navigation and Settings SHALL
remain reachable independently of history scrolling. New work/conversation entry
SHALL precede contextual history; row actions SHALL use labelled keyboard/touch
controls without permanent delete-icon clutter. Existing deep links SHALL resolve
to the same saved identities through route adapters.

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
### Requirement: Document reading beside conversation
Opening a supported attachment or verified citation SHALL show a source reader
beside Chat on wide screens and a reversible Chat/Document view on narrow screens.
The reader SHALL provide PDF page navigation, zoom and text search; other formats
SHALL identify structured text as distinct from original layout. Drafts and source
identity SHALL survive view switching without sending or changing permissions.

#### Scenario: Read and ask about a paper
- **WHEN** the user opens an attached PDF
- **THEN** Chat remains usable alongside its original pages and extraction-coverage status
- **AND** opening a citation navigates only to its verified matching source

#### Scenario: Preparing or blocked document
- **WHEN** preparation is active, finishes or fails
- **THEN** motion and status follow the actual operation and stop when it ends
- **AND** reduced-motion users receive equivalent static feedback without fake percentages

#### Scenario: Narrow screen or keyboard navigation
- **WHEN** the split layout cannot provide readable columns
- **THEN** accessible view switching preserves draft and reading position
- **AND** closing the reader restores focus without submitting work

### Requirement: Readable attachment references
Saved attachment answers SHALL expose compact source references bound to their
persisted extraction revision and selected ranges, without rewriting source text.

#### Scenario: Compact upload and source inspection
- **WHEN** a user adds files or opens a retained document
- **THEN** compact attachment tiles report actual preparation state and offer source inspection without sending a request
- **AND** page thumbnails and secondary reader controls open on demand without displacing the document canvas
- **AND** partial extraction remains explicitly identified, failures retain recovery, and reduced motion removes decorative activity without hiding status

#### Scenario: Switch documents without losing context
- **WHEN** a user opens multiple sources in the same conversation
- **THEN** the reader presents closable file tabs with distinct format, add, expand and split controls
- **AND** each source retains its own revision-bound page, zoom and search while only the active reader loads a full page
- **AND** closing a tab does not delete its attachment or submit the draft, and changing conversation scope clears open tabs

#### Scenario: Inspect thumbnails without selecting another page
- **WHEN** a user opens the filename control and scrolls through the page list
- **THEN** the list shows vertical thumbnails in an opaque themed panel with at most four decoded thumbnail rasters and one request at a time
- **AND** scrolling does not change the source page until a thumbnail is selected
- **AND** keyboard navigation, expand/restore and split view retain source identity and restore focus

#### Scenario: Valid retained passage
- **WHEN** the user opens a bound citation
- **THEN** the service checks owner access, revision, digest and UTF-8 boundaries and returns the exact passage
- **AND** the reader navigates to a page only for a unique exact match; otherwise it shows the retained passage without a guessed page

#### Scenario: Legacy or invalid citation
- **WHEN** an old byte citation uniquely matches a selected range it remains openable
- **THEN** duplicate names, forged ranges, code and ordinary links are not converted to source actions
- **AND** stale or revoked sources show an error without substituting another file
