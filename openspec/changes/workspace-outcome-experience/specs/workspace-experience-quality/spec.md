## Purpose

Set observable visual and behavioral quality gates for the shared web and desktop experience without equating automated tests with product or hardware qualification.

## ADDED Requirements

### Requirement: Bounded accessible surface materials
The shared renderer SHALL use palette-derived sidebar, composer and floating
utility material as at most three decorative families per screen. Primary
actions SHALL use solid fills. Floating utilities and the command palette MAY
use at most 12px backdrop blur with a 96% surface-color fill; text SHALL remain
fully opaque. Answer/code surfaces and full-screen drawers SHALL NOT blur.
No animated gradient, wallpaper capture or new color palette is permitted.

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

### Requirement: Complete contextual localization
All new and touched user-facing workflow copy, including voice preferences, readiness and recovery, SHALL use typed resources for the nine existing interface locales. Stable identifiers and model names SHALL remain unchanged. Mixed-language UI SHALL NOT be accepted as completed localization.

#### Scenario: Change interface language
- **WHEN** a user switches locale with a draft, selected model or open utility
- **THEN** labels and announcements change without resetting work
- **AND** translated labels remain usable at narrow widths and in RTL

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
