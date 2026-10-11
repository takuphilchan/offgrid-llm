# Proposal

## Why

The running 0.4.16 container exposes the current revision but a SQLite timeout
removed its task-first capability flag. The current renderer interpreted this
health failure as an old service and mounted the obsolete two-panel Agents UI.

## What Changes

- Delete the legacy agent composer/result/history implementation and exclusively
  used setup/state helpers. Keep current tools/connections management separately.
- Keep implemented service contracts stable when task storage is unhealthy;
  execution continues to fail closed with existing actionable errors.
- Use the current TaskWorkspace for work. A genuinely incompatible service gets
  a localized unavailable state, never an alternate execution UI.
- Preserve drafts/current task selection across transient identity/read failures;
  workspace or actor changes still reset scoped state.
- Add backend and browser/desktop-renderer regression coverage, including cold
  storage failure and capability loss after a page has mounted.

This retirement is limited to redundant Agents presentation/fallback code. It
does not remove saved histories, compatibility endpoints, consent enforcement,
data migrations or legacy-model support. It does not silently clear a task-store
failure or retry uncertain work. Runtime/storage recovery and deployment remain
separate from removing the UI fallback.

## Capabilities

### New Capabilities

- `agent-workspace-continuity`: one current agent workspace across readiness
  failures and service compatibility checks, without legacy UI resurrection.

### Modified Capabilities

None; no archived specs currently exist.

## Impact

Shared React Agents entry/management components and exclusive obsolete helpers,
system identity feature advertisement, API/renderer tests and workspace docs.
No visual redesign, credential/data deletion, native control, commit or release.
