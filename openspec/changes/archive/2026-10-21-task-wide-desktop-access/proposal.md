# Proposal

## Why

The updated desktop still asks users to select or launch a window because its actual host authority is protocol-v2, bound to one window. Choosing Full task access only changes action approval policy; reinstalling that implementation cannot deliver the promised task-wide workflow.

## What Changes

- Introduce a distinct protocol-v3 task desktop grant, with affirmative local consent, a 60-minute/500-mutation limit, independent stopping, and dynamic execution targets. Never widen existing v2 grants.
- Follow-up approved 2026-10-10: explicitly enabling Full access may authorize successive full-access tasks for the current desktop session. Keep that authorization only in the live host, scoped to service/workspace/actor; reset it on quit, lock/suspend, account/workspace changes or revocation. No repeated task-consent popup while it is valid. A proposed default or stored radio value is never a grant.
- Use the existing persistent in-app Stop control without a duplicate native popup (explicitly requested 2026-10-10). Keep independent emergency-shortcut and OS-session revocation, and allow local Stop while the service is offline. Make the access popover genuinely scrollable with wheel, touch and keyboard, including short/zoomed/RTL viewports.
- Hold desktop-enabled tasks before model dispatch until the matching host grants access. Keep access policy chosen before Start, and automatically continue the same durable task after consent.
- Add durable installed-application discovery, catalog-identity launch, surface discovery/focus, and independent application-open verification. A task can switch ordinary applications without another window permission. Unknown or unsafe launch profiles fail closed; no model executable paths or command strings.
- Replace the normal application picker with task consent in the current TaskWorkspace. Preserve explicitly limited legacy-window access and managed-browser compatibility without treating those as desktop authority.
- Test the complete Chrome orchestration and cancellation/recovery boundaries, and document actual platform coverage. Keep production qualification separate from implementation and fixtures.
- Follow-up reported 2026-10-10: fix post-launch native context rejection with lossless model projections, measured prompt accounting and bounded archive references. Handle slow Windows inference through exact host re-observation before new structured preparation/readback, never by extending stale timestamps or replaying actions. Retain canonical evidence and existing v2 limits; record actual tokenizer/model/owned-fixture coverage without claiming public-search qualification.

- Follow-up approved 2026-10-10: replace the lossless-every-turn model projection with bounded progressive observations while retaining canonical evidence. Add scoped retrieval, compact references, durable application progress and real long-URL regressions. Generalize discovery through OS registrations with validated typed launch descriptors, not model commands. Test and deploy matching service/desktop builds only after applicable checks.

This change implements the host-access/application-lifecycle part of the approved program. It does not claim the separate Chat linkage, OAuth, native visual fallback, host file transfer, Wayland portal, or full workflow-qualification stages are finished. Those remain required stages of the approved program, not completion claims of this increment.

## Capabilities

### New Capabilities

- `task-desktop-authority`: Explicit task-wide host consent, dynamic targets, durable application lifecycle, and pre-execution access admission.

### Modified Capabilities

None; there are no archived capability specs yet. This supersedes the deliberately limited host behavior documented by the completed task-access-intent change, not its immutable policy or recovery guarantees.

## Impact

Go computer supervisor/worker, companion queue and protocol, agent task admission, Electron trusted IPC, native transport, current React task composer/access view, generated API contracts, tests and existing computer/reliability documentation. Requires a matching desktop build as well as service build; a container-only update cannot deliver host authority. No model or user-data migration, no automatic downloads, no permission grants on upgrade, no release/publication.
