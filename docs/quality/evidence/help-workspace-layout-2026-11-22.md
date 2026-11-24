# Help and workspace panel layout — 22 November 2026

The owner reported overlapping Help content and a workspace-access popup whose
title split inside a word. Help's heading formerly shared its scrolling container
with topics and article content. Compact action-menu styles also reached buttons
inside nested popover forms, making Refresh hosts full width and squeezing the
workspace title.

Help now has a fixed heading above a separate content scroller. Topic selection
focuses without browser-induced outer scrolling, then aligns the article within
that scroller. Compact action styling applies only to direct menu actions and
nested triggers. Workspace host rows, refresh and folder controls retain their
own spacing. Native radio behavior is preserved; keyboard focus outlines the
rounded host row with scroll clearance around it.

The owner also rejected the stretched, tinted Help search pill and detached
topic counter. Search now uses a bounded 420 CSS pixel field, the shared rounded
rectangular shape, the page surface and a single border focus treatment. The
result count remains a live status for assistive technology without visual
clutter. Filtering, clearing and draft retention use the existing behavior.

Settings search now presents full result rows with section metadata and a
directional affordance. The editable section content is hidden while searching,
then revealed and focused on selection. Specific matches precede section-name
matches; descriptive section fallbacks remain available when there is no specific
match, including named palette queries. This removes unrelated description-only
sections for short queries such as `he`. Empty queries and no-match states remain
distinct. Existing settings, including the Models/Data directory card, keep their
markup and values.

## Checks

- Before-fix Help regression: two failures reproduced overlap in English and
  Arabic. `build/ui-completion/help-overlap-before.log`.
- `npm.cmd --prefix apps/web run test:e2e -- workspace-help.spec.ts agent-chat.spec.ts workspace-shell.spec.ts composer-hierarchy.spec.ts shared-control-polish.spec.ts --workers=1`:
  **212 passed**. `build/ui-completion/help-workspace-browser.log`.
  This covers all locale Help transitions, nested menus, execution permission
  boundaries, shell/composer continuity and shared controls.
- Three focused Help/workspace regressions passed after the focus-row change:
  `build/ui-completion/help-workspace-final-focused.log`.
- After the search-field refinement, **38 Help/shared-control cases passed**:
  `build/ui-completion/help-search-browser-final.log`.
- Settings discovery/navigation/organization: **128 passed, 3 failed** initially
  in `build/ui-completion/settings-result-rows.log`. Short queries exposed extra
  description-only section matches; those are now subordinate to specific and
  section-name matches. A test looking for a hidden voice control by accessible
  role was updated to inspect its pending readiness without making it visible.
  A storage-failure test was scoped to the Appearance status because the global
  preference-failure status can legitimately be present too. All **three
  corrected cases passed** in `build/ui-completion/settings-result-corrected.log`.
  The added regression also covers palette fallback, no-match state, clearing,
  and focus on a section destination. The known catalog theme Dracula replaces
  an unrecognized draft fixture label.
- Renderer API schema check and TypeScript/production build passed. Final build:
  `build/ui-completion/help-settings-qualified-build.log`.
- Final packaged native qualification passed in
  `C:\Users\phil\AppData\Local\Temp\offgrid-help-panels-lQxWj3`.
  Log: `build/ui-completion/help-settings-native-qualified.log`.
  It additionally verifies Settings search/result navigation and full visibility
  of the focused search field below the native title bar. Settings document
  scrolling now reserves that title-bar clearance.
- The final production comparison passed against `build/ui-baseline/dist`:
  navigation p95 **29.0 / 29.5 ms**, heap **12,844,416 / 13,274,944 bytes**,
  requests **180 / 180**, baseline/candidate respectively. Candidate immediate
  acknowledgment was **30.8 ms**. Existing +10% navigation/heap and <100 ms
  acknowledgment thresholds were unchanged. This was one comparison with 40
  navigation samples per build, not a physical hardware qualification.
  Log: `build/ui-completion/help-workspace-performance.log`.

## Staged delivery

The exact 67-file renderer matches `apps/web/dist`; its index SHA-256 is
`940693ba7e505dd924ff904d1e79324f391146317b3026d8ea3facf08a9ea43c`.
The native archive remains
`055da05a5d447a691601fd82bd057ebcf985b6b6382b3b72234c801eac3684f3`,
including all 416 native files and the previously delivered terminal redesign.
Executable, service and PDF/OCR worker hashes remain unchanged. The candidate
receipt is `build/ui-completion/package/receipt-help-workspace.json`.

Installation is pending the owner quitting the normal app and local workspace
sessions. The prepared installer preserves rollback copies, rechecks idle state
and expected installed hashes, and replaces only renderer assets. No user
process is terminated and no installed-user data is used by qualification.

The native harness `scripts/test/test-desktop-help-panels.mjs` serves the exact
candidate or installed renderer files with an isolated HTTP service and disposable
desktop profile. It opens Help and the nested workspace panel at native 100% and
200% zoom in English/light, English/dark and Arabic/dark, verifies scroll/focus
geometry, retains drafts and records screenshots. It does not grant folder
access, create terminals, execute commands or write service data. Appearance
changes use the app's Settings controls and affect only the disposable profile.

Intermediate fixture corrections are retained: an invalid line-height assertion
used a computed `normal` value; native radios ignore CSS border radius while
keeping platform appearance; native zoom rounds CSS outline widths; pointer
focus does not establish keyboard focus modality; and direct native presentation
updates do not change an existing renderer's saved theme choice. Final checks
use text line geometry, real keyboard traversal and Settings controls. Visual
inspection additionally found a clipped focus outline at 200%; scroll clearance
was added and is asserted explicitly.

All edits remain in `D:\offgrid-ui-20261121`. No changes are made to the concurrent
`D:\offgrid-llm` checkout. These layout fixes do not qualify the complete UI/UX
program, human/native-language review, real models, other platforms or full CI.
