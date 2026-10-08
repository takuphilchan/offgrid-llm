## Purpose

Make utility popovers predictable and accessible across voice settings and the shared workspace shell.

## ADDED Requirements

### Requirement: Consistent nonmodal dismissal
Utility popovers SHALL open from labelled keyboard-operable triggers, close on outside interaction or Escape, and expose their expanded state. Escape SHALL return focus to the trigger without trapping focus or treating the utility as a consequential approval dialog.

#### Scenario: Keyboard dismissal
- **WHEN** a user opens workspace or voice options by keyboard and presses Escape
- **THEN** the panel closes and focus returns to its trigger

#### Scenario: Switching utility panels
- **WHEN** a user opens another utility popover
- **THEN** the previous utility popover closes

### Requirement: Bounded placement
Utility popovers SHALL remain within narrow, short, and RTL viewports with scrollable contents where needed. Scrolling or resizing SHALL not leave an open panel detached from its trigger.

#### Scenario: Small viewport
- **WHEN** a utility panel opens in a 320-pixel-wide or short viewport
- **THEN** its controls remain reachable within viewport bounds

### Requirement: Voice settings compatibility
Voice settings SHALL retain installed-profile selection, error reporting, and cancellation of dismissed metadata requests while adopting shared popover behavior.

#### Scenario: Dismiss during metadata loading
- **WHEN** voice settings closes before its metadata request completes
- **THEN** the request is cancelled and its late response does not update a subsequently opened panel
