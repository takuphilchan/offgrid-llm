# Conversation UI audit — 2026-11-14

The shared renderer was captured with isolated Playwright fixtures. No installed
conversations, grants, models or native processes were changed. The installed
service was unavailable at its previous loopback address, so these are renderer
captures, not evidence that the installed desktop was updated.

## Findings and changes

- Canonical chat exposed a full-width Send as selector outside its 800px composer
  and a second Response settings disclosure. Ordinary conversation now has no
  routing selector. Explicit overrides live in the existing options menu; an
  active override has a visible cancellable indicator and resets after admission.
- Configuration inspection previously exposed raw JSON as the first content and
  displaced the conversation. Context now lives inside composer options, shows
  the project name first, and keeps technical provenance under Advanced.
- Pending input cards were unbounded outside the scrolling transcript. Their
  region now scrolls independently, preserving the composer and Stop controls.
- Profile guidance implied selecting a profile was required to start work. The
  page now uses its existing localized reusable-profile introduction.
- The Tools screen rendered `undefined/0 enabled` when the response omitted its
  aggregate. It now derives the fallback from the returned tool states. This
  appeared in the fixture; it is not claimed as a reproduced server defect.

The narrow profile editor is below the profile list, but choosing a profile
already focuses its heading. No speculative editor rewrite was made.

## Evidence

- `build/ui-craft/audit-before`: desktop light and narrow dark captures across
  chat, profiles/editor, tools/connections, activity, knowledge, models/discovery,
  settings, loading/error/empty states, palette, workspace menu and onboarding;
  separate canonical chat/settings captures.
- `build/ui-craft/audit-after`: 16 theme/language/width combinations (light/dark,
  English/Arabic, 390/768/1024/1440). This intermediate version still showed the
  compact routing selector; it was superseded following user feedback.
- `build/ui-craft/audit-final/canonical`: final default composer at 390 and 1440,
  with routing controls inside options rather than on the default surface.
- 37 browser cases passed in the broad intermediate run; all 14 canonical
  conversation cases passed after the routing correction, including the new
  20-input narrow-viewport regression.
- TypeScript and production build passed after the final correction. Existing
  large-bundle warnings remain. Source-size ratchet passed. OpenSpec validation
  passed for workspace-outcome-experience.

This closes specific reproduced presentation defects, not the complete UI/UX
review or native/model qualification. Screenshots are generated evidence, not
human usability results. No installer was rebuilt in this audit.
