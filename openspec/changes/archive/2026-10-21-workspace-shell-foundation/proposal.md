## Why

The shared workspace shell repeats navigation categories, descriptions, and utility controls above every task. This second slice of the approved workspace redesign reduces that visual overhead without changing the monochrome identity or the recovered-state behavior delivered by workspace-state-recovery.

## What Changes

- Extract shared navigation and header presentation from App while preserving routes, authentication, page ownership, and refresh behavior.
- Introduce a compact header and one workspace-options popover for language, appearance, service details, and account actions.
- Add a remembered, collapsible desktop sidebar with accessible icon-only navigation; retain all destinations on mobile.
- Share dismissible, viewport-bounded popover behavior with voice settings, including keyboard dismissal and focus restoration.
- Consolidate shell geometry in its own stylesheet and verify keyboard, narrow-screen, RTL, light/dark, and draft-continuity behavior.
- Exclude feature-page redesigns, backend authorization changes, runtime changes, deployment, and publication from this slice.

## Capabilities

### New Capabilities
- `workspace-shell`: Compact, responsive, accessible shared navigation and utility controls that preserve active work.
- `workspace-popovers`: Shared nonmodal utility popovers with consistent dismissal, placement, and focus behavior.

### Modified Capabilities

None. No published OpenSpec capability currently describes this shell.

## Impact

React shell components, workspace styling, voice-settings presentation, localization, renderer tests, desktop smoke locators, and existing workspace/reliability documentation. No service API, stored user data, task approval, microphone authorization, or installed runtime contract changes.
