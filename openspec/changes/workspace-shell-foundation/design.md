## Context

See proposal.md for motivation. App.tsx currently owns navigation markup and page lifecycle together. Shell geometry is split across styles.css and workspace.css. VoiceSettings already uses native auto popovers but owns its positioning. Existing renderer and desktop theme checks expect the language control directly in the header.

## Goals / Non-Goals

**Goals:** Separate presentation ownership, centralize navigation metadata, and share accessible utility-panel behavior while preserving the state/recovery fixes from the previous slice.

**Non-Goals:** Do not replace routing, page state, consequential dialogs, authentication, runtime selection, or service transport. Do not introduce a new UI library or change the monochrome palette, fonts, or icon style.

## Decisions

- Extract navigation/header components, leaving service and page lifecycle in App. A new routing or global-state framework would increase migration risk without improving this slice.
- Use one native auto-popover component with viewport-clamped placement and resize/scroll updates. Keep browser light dismissal and focus semantics rather than adding document-wide click handlers or modal traps. Consumers retain their own loading and cancellation state.
- Consolidate shell rules in an explicitly owned shell stylesheet and use shared header/sidebar size variables for dependent chat layout. Remove superseded declarations rather than stacking another override layer. Preserve feature-specific layouts.
- Store only the non-sensitive sidebar layout in the existing preference helper. Default expanded, retain labels for screen readers in collapsed mode, and keep mobile navigation independent.
- Put language, theme, service details, and account actions in a labelled workspace-options panel. Keep quick actions and refresh visible. Preserve Electron presentation updates through the existing App effect.
- Add shell strings in all nine locales with typed contracts. Adapt existing tests to open workspace options before changing language; keep their assertions intact.

## Risks / Trade-offs

- Moving preferences reduces immediate visibility → use a labelled, titled header control and retain existing quick actions/settings routes.
- Native popover positioning can overflow on short screens → clamp both axes, bound height, observe content changes, and test 320px width, short height, and RTL.
- CSS cleanup can alter page geometry → verify chat height/history drawer, mobile navigation, both themes, all locales, and existing feature suites.
- Refactoring shell markup can remount feature pages → keep the page boundary and page identity in App and test draft continuity.

## Migration Plan

No server or data migration. Build the shared renderer for both clients; existing installations receive the changes only during a separately authorized deployment. Rollback restores prior renderer assets; the optional sidebar preference can be ignored by older versions. Record checks in existing reliability documentation, not as production qualification.
