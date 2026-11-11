## Purpose

Set observable visual and behavioral quality gates for the shared web and desktop experience without equating automated tests with product or hardware qualification.

## ADDED Requirements

### Requirement: Predictable settings and native controls
The sidebar gear SHALL navigate directly to Settings. Workspace status, refresh
and sign-out SHALL remain in a separately identified overflow menu. Language and
appearance controls SHALL have one settings location. Dropdowns SHALL preserve
native keyboard/type-ahead behavior with theme-aware geometry and RTL support.

#### Scenario: Gear navigation from a mobile draft
- **WHEN** a user opens the navigation drawer and activates its gear
- **THEN** Settings opens directly and the drawer closes
- **AND** returning to Chat preserves the unsent draft

#### Scenario: Dropdown accessibility
- **WHEN** text size, theme, direction or forced colors changes
- **THEN** dropdown text and its indicator remain separated and readable
- **AND** keyboard selection and persisted values remain functional

### Requirement: Theme-consistent decorative separation
Decorative rules SHALL blend with the selected palette and containing surface.
Lists and adjacent sections SHALL avoid redundant closing/opening rules. Control,
focus, selection and progress borders SHALL retain their distinct roles.

#### Scenario: Installed models followed by advanced import
- **WHEN** an installed package list precedes the Advanced import disclosure
- **THEN** the boundary presents at most one horizontal rule, with deliberate spacing
- **AND** changing palettes, locale or text size preserves readable controls and content

#### Scenario: Thematic breaks and accessibility
- **WHEN** a response contains a Markdown thematic break or system forced colors are enabled
- **THEN** the break renders as one explicit rule using theme or system colors
- **AND** keyboard focus and selected tabs remain distinguishable

### Requirement: Theme and language expansion remains lightweight and truthful
The workspace SHALL provide established themes through shared local data and
SHALL distinguish incomplete language previews from reviewed translations.

#### Scenario: Established theme follows the selected appearance mode
- **WHEN** a user saves an available named palette and changes system light/dark mode
- **THEN** the corresponding saved palette is restored, desktop and terminal consume the same colors, and no theme code or network asset is loaded

#### Scenario: An incomplete language is selected explicitly
- **WHEN** a user selects a preview language
- **THEN** the choice survives reload, a native-language notice identifies incomplete translation and pending review, and untranslated features use complete fallback text rather than missing labels
- **AND** interface language does not change voice configuration, models or permissions

#### Scenario: A preview language matches the operating system
- **WHEN** no explicit saved language exists and the browser advertises only an unreviewed preview language
- **THEN** the workspace uses the existing fallback language instead of silently enabling the preview

#### Scenario: A translation is incomplete or corrupt
- **WHEN** a language pack changes placeholders, contains empty messages or corrupted characters, or claims reviewed status without meeting the release gate
- **THEN** translation validation fails and does not certify the pack as production ready

### Requirement: Personalization preserves existing work
The renderer SHALL separate system/light/dark mode from versioned palette choices for each mode. Appearance preview SHALL be reversible without changing conversations, profiles, permissions or models. Unknown persisted versions SHALL fall back to the original appearance. Desktop storage SHALL accept only normalized presentation data.

#### Scenario: Preview and cancel
- **WHEN** a user previews a palette and cancels or leaves the appearance view
- **THEN** the last applied appearance returns without writing the preview
- **AND** appearance reset affects only appearance preferences

#### Scenario: Storage failure
- **WHEN** a presentation preference cannot be persisted
- **THEN** the current session remains usable and the user sees that persistence failed
- **AND** a later successful write clears only the relevant failure

#### Scenario: Direct settings navigation
- **WHEN** the user opens a supported settings deep link or selects a matching search result
- **THEN** the matching section opens and keyboard focus moves to the requested setting
- **AND** no service configuration or access grant changes as a result of navigation

### Requirement: Consistent readable typography
The shared renderer SHALL use semantic size and line-height roles for titles,
sections, prose, controls, metadata and captions. Visible interface copy SHALL
not retain legacy sub-12px text. Exact technical identifiers MAY use monospace
without adopting a smaller unreadable size.

#### Scenario: Inspect primary pages and enlarged guidance
- **WHEN** the six primary pages and Help are rendered with representative content
- **THEN** visible text follows the shared type scale and localized line-height rules
- **AND** 200% desktop zoom retains readable content and reachable controls without clipped dialogs

### Requirement: Bounded accessible surface materials
The shared renderer SHALL use palette-derived sidebar, composer and floating
utility material as at most three decorative families per screen. Primary
actions SHALL use solid fills. Floating utilities and the command palette MAY
use at most 12px backdrop blur with a 96% surface-color fill; text SHALL remain
fully opaque. Answer/code surfaces and full-screen drawers SHALL NOT blur.
No animated gradient or wallpaper capture is permitted. The approved personalization extension permits validated named palettes while retaining the original default.

#### Scenario: Material fallback
- **WHEN** backdrop filtering is unavailable, transparency is reduced, contrast is increased, forced colors are active or the central material off switch is set
- **THEN** floating surfaces retain opaque readable fills and geometry without blur
- **AND** accessibility and off modes flatten decorative gradients without hiding controls or changing work state

#### Scenario: Popup readability and interaction
- **WHEN** a floating utility overlaps light or dark content in either theme
- **THEN** its ordinary text meets 4.5:1 contrast across the composited surface envelope
- **AND** focus remains visible, nested drawer Tab containment and Escape dismissal remain predictable, and unchanged 250ms fixture feedback gates pass

### Requirement: Whole-product experience qualification
The approved review SHALL be implemented and verified across all routes and
distributed editions. Screen hierarchy, field widths, settings, menus and status
feedback SHALL be consistent. Existing palette and 44px coarse-pointer targets
remain. Earlier 40px desktop references are superseded by the approved compact
fine-pointer geometry. Actual participant, screen-reader and installed-platform
evidence SHALL remain separate from structural and fixture checks.

#### Scenario: Candidate completion audit
- **WHEN** the whole-product implementation is considered complete
- **THEN** all group 16 outcomes have matching current-state evidence including the unchanged responsiveness gates
- **AND** missing platform, fresh-install or participant evidence is reported as incomplete rather than inferred from earlier checked tasks

### Requirement: Automatic desktop startup recovery
Standalone desktop installations SHALL start their bundled native service without
requiring Docker, WSL or Ubuntu. Remembered external endpoints SHALL receive a
bounded startup grace period followed by paced automatic reconnection when
offline or unresponsive. Reconnection SHALL retain compatibility and workspace
authority checks and SHALL NOT spawn a substitute service or replay work.

#### Scenario: A cleanup component fails during Quit
- **WHEN** preference saving or an owned input worker fails to finish during Quit
- **THEN** other owned cleanup still runs and preference saving cannot indefinitely block shutdown
- **AND** unconfirmed owned worker/service exits remain visible and retryable without claiming successful termination, discarding ownership or stopping an external service

#### Scenario: Service starts after desktop
- **WHEN** a remembered external endpoint becomes compatible and ready during or after startup grace
- **THEN** the desktop automatically opens that workspace without a Retry click
- **AND** waiting copy explains reconnection without reporting a completed connection

#### Scenario: Recovery is cancelled or incompatible
- **WHEN** the user quits, selects isolated recovery, or a probe reports incompatible identity
- **THEN** automatic checks of the previous endpoint stop
- **AND** no duplicate process or stale local authority is introduced

#### Scenario: Ordinary standalone installation
- **WHEN** a fresh desktop profile opens with a free configured port and bundled service
- **THEN** one native service starts with the desktop workspace and no Linux subsystem command

### Requirement: Palette-preserving gradient budget
Decorative gradients SHALL use only existing neutral palette tokens, be defined
centrally, and have a flat fallback and one central off switch. No screen SHALL
use more than three decorative treatment families. Answer and code text remain
on flat surfaces. Forced colors and increased contrast SHALL flatten treatments.

#### Scenario: Low-power and accessible presentation
- **WHEN** gradients are unsupported, disabled or contrast preferences require flat surfaces
- **THEN** every treatment falls back without hiding content or changing geometry
- **AND** any animation uses only transform/opacity and respects reduced motion

#### Scenario: Measured quality
- **WHEN** gradient treatments are accepted
- **THEN** both themes pass worst-point text contrast, isolated on/off inspection and throttled stream/download feedback checks
- **AND** source palette values and application behavior remain unchanged

### Requirement: Frozen palette and bounded presentation motion
The refinement SHALL preserve every existing UI color declaration and literal.
The subsequently approved gradient phase MAY add generated neutral ramps and
their reviewed background-image consumers; it MUST NOT alter palette tokens.
New geometry, type and motion SHALL use additive semantic tokens. Animations
SHALL use transform or opacity, respect reduced motion and avoid replaying over
long streaming transcripts. Superseded legacy geometry SHALL be removed when
migrated; frozen color declarations remain at their original owners.

#### Scenario: Compare the refinement to its baseline
- **WHEN** the presentation refinement is verified
- **THEN** the flat refinement has zero authored color additions, removals or changes, and the later gradient phase has only its explicitly inventoried additive treatments
- **AND** before/after fixtures cover all main screens, streaming/code, overlays,
  empty/loading/error states, both themes and Arabic RTL at four viewport widths

#### Scenario: Reduced motion and slow hardware
- **WHEN** motion is reduced or a long answer is streaming
- **THEN** actions, focus and output remain available without animation-dependent timing
- **AND** transitions do not animate layout dimensions or apply large backdrop blur

### Requirement: Authoritative presentation foundations
The shared renderer SHALL use one monochrome token source, bundled IBM Plex fonts and consistent product mark. Controls SHALL preserve explicit boundaries and usable target sizes. OffGrid-owned desktop startup and consent surfaces SHALL follow the same hierarchy without moving local authorization into the service renderer.

#### Scenario: Presentation changes across editions
- **WHEN** the same workflow is opened in browser and desktop
- **THEN** shared controls, typography and responsive organization agree
- **AND** native consent, OS permissions, stopping and authentication remain independently enforced

### Requirement: Consistent visual hierarchy
Main pages SHALL retain the monochrome palette, visible action-button boundaries and shared typography, spacing, field and focus rules. One primary action SHALL dominate each active task region. Secondary maintenance SHALL use deliberate grouping rather than repeated nested cards or equal-weight controls.

#### Scenario: Scaled desktop composition
- **WHEN** the workspace has less content space because of display scaling, zoom or expanded navigation
- **THEN** work inspection uses the available content region rather than reserving a second history column
- **AND** readable text, visible keyboard focus and reachable 40px desktop/44px touch actions are retained without changing user zoom

#### Scenario: Distinct action priority
- **WHEN** primary, secondary and utility actions appear together
- **THEN** shared geometry and accessible names remain consistent but utilities do not compete visually with the primary action
- **AND** navigation selection is visually distinguishable from keyboard focus

#### Scenario: Compare main surfaces
- **WHEN** Chat, Agents, Models, Knowledge, Activity and Settings are rendered in equivalent states
- **THEN** corresponding controls share geometry and interaction states while primary work remains identifiable
- **AND** navigation links, actions, statuses and content references remain semantically distinct

#### Scenario: Realistic content density
- **WHEN** management pages contain long names and descriptions at narrow and desktop widths
- **THEN** headings reserve room for contextual actions, comparable pages share a leading edge, and long artifact names have a bounded summary with explicit full-name access
- **AND** maintenance does not compete with selection, work history does not require scrolling past schedule configuration, and technical startup metadata remains available in details

### Requirement: Complete contextual localization
All new and touched user-facing workflow copy, including voice preferences, readiness and recovery, SHALL use typed resources for the nine existing interface locales. Stable identifiers and model names SHALL remain unchanged. Mixed-language UI SHALL NOT be accepted as completed localization.

#### Scenario: Change interface language
- **WHEN** a user switches locale with a draft, selected model or open utility
- **THEN** labels and announcements change without resetting work
- **AND** translated labels remain usable at narrow widths and in RTL

#### Scenario: Audit and review every selectable language
- **WHEN** localization is audited or exported for review
- **THEN** all 29 registered languages include the complete contextual message inventory, source, placeholders and authored-versus-fallback status
- **AND** a readiness check rejects missing translations and missing or stale human review records
- **AND** reviewed import rejects changed sources, concurrent edits, duplicate or missing entries and broken placeholders before replacing source files

#### Scenario: Selected language reaches desktop-owned copy
- **WHEN** translated contextual overrides exist for the selected language
- **THEN** startup, native menus, computer-access copy, remembered-folder prompts and terminal controls use them before per-message fallback
- **AND** packaging verifies the exact shared override asset, while user names, paths, commands and terminal output remain unchanged

#### Scenario: Audit rendered elements and live terminal utilities
- **WHEN** localization is checked across routes and expanded utility states
- **THEN** the audit records all rendered text nodes and user-facing attributes, including messages outside the known catalogs, instead of selecting a few expected labels
- **AND** changing language updates open terminal utilities and their status text without clearing questions, redacted output, search queries, terminal selection or process state
- **AND** missing translations, unclassified first-party runtime copy and human review remain distinct unresolved findings

### Requirement: Accessible responsive interaction
The experience SHALL support keyboard use, visible focus, semantic names, IME, RTL, reduced motion, text zoom and screen-reader status announcements. Work must remain reachable at 320-pixel width and short desktop heights. Optional utilities SHALL NOT obscure the focused control or remove access to Stop.

#### Scenario: Keyboard and zoom
- **WHEN** the user completes a core journey at 200 percent zoom or by keyboard alone
- **THEN** controls remain reachable, focus returns predictably and content is not clipped
- **AND** status updates do not repeatedly announce every generated token

### Requirement: Immediate truthful interaction feedback
Local input and lifecycle controls SHALL provide visible feedback within 250 milliseconds at p95 on frozen qualified UI fixtures. This SHALL NOT be presented as backend acknowledgment or inference completion. Model loading, queueing, generation, speech preparation and connection recovery SHALL remain distinguishable.

#### Scenario: Slow inference
- **WHEN** the service is slow or queued
- **THEN** editing, navigation and supported Stop controls remain responsive
- **AND** no invented percentage or model-latency guarantee is displayed

### Requirement: Evidence-backed experience acceptance
The change SHALL be evaluated against frozen complete journeys and the prior build, not screenshot attractiveness alone. Results SHALL record task success, detours, time, recovery and participant assistance, with zero duplicate submissions or unauthorized actions in regression scenarios. Automated, real-service, installed-edition and human evidence SHALL be distinguished.

#### Scenario: Missing qualification
- **WHEN** required installed-edition or usability evaluation has not run
- **THEN** the evidence log marks that gate unrun and the experience is not claimed equivalent to leading products
- **AND** the existing production-readiness, soak and pilot requirements remain authoritative

### Requirement: Clear native installation and removal
The Windows assisted wizard SHALL present a compact welcome, truthful progress,
concise completion and explicit uninstall retention guidance. It SHALL preserve
scope/directory selection, application identity, saved data, native keyboard and
high-contrast behavior, and running-app protection.

#### Scenario: Install or repair
- **WHEN** a user installs or repairs OffGrid
- **THEN** native controls remain reachable and progress reflects actual work
- **AND** models and saved work remain unchanged, with optional launch deferred until the wizard closes

#### Scenario: Remove the application
- **WHEN** a user opens the uninstaller
- **THEN** it explains which application files are removed and that models and saved work are retained
- **AND** removal does not stop unrelated services or erase the retained workspace
