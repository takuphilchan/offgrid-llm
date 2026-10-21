# Design

## Context

See proposal.md for the reproduced failure. The current `AgentPage.tsx` owns
both TaskWorkspace selection and the obsolete AgentWorkspace implementation;
the latter also hosts current connection/tool management. `ActivityReplayAvailable`
combines persistence configuration with health, and system identity uses it as
a feature switch. The shell keys draft providers partly by this volatile switch.
Follow docs/advanced/workspace-ui.md for current styling and component ownership.

## Goals / Non-Goals

Remove the alternate task UI and its exclusive state/helpers, retain the current
management UI, distinguish contract support from health, and prevent remounts on
transient reads. Do not clear storage errors, replay tasks, remove compatibility
APIs or broaden permissions. Live container repair is not part of source cleanup.

## Decisions

- Extract the existing tool/connection management into a focused component that
  does not read task history or computer readiness just to manage connections.
  Delete legacy composer, runner follower, submission, reconciliation and mode
  picker code rather than adding another feature flag around it.
- Remove ComputerSetup and useAgentOutputScroll if reference checks confirm
  they are exclusive to the removed page. Retain shared TaskAccess, activity,
  emergency-stop and approval components. Remove demonstrably exclusive CSS and
  copy/helpers; do not blanket-delete anything named legacy.
- Report durable contract support from configured persistent storage, not its
  momentary health. Existing task endpoint health checks remain authoritative.
- Preserve last known task-work support only for the same workspace on read
  failure; explicit workspace changes reevaluate support and clear scoped UI.
  Never cache permission or authority. An unconfirmed cold connection shows a
  localized unavailable/retry surface, not the obsolete work page.
- Retain existing API compatibility adapters used by external/legacy clients.
  Replace obsolete UI tests with equivalent current-workflow assertions, preserving
  security, approval, recovery, scrolling and accessibility coverage where relevant.

## Risks / Trade-offs

- Shared helper deletion can break current access flows: use reference checks,
  TypeScript and current task/management/voice suites before removing a helper.
- A healthy-looking feature flag could obscure unavailable storage: task reads
  keep their explicit failure state and mutations continue to fail closed.
- Some older services cannot serve current tasks: explain incompatibility rather
  than automatically enabling an old interaction model.

## Migration Plan

No user-data migration. Current saved tasks remain served by existing APIs. Build
both editions from the same renderer; cold-failure and downgrade tests guard the
route. Source validation does not update a running container or installed app.
