# Proposal

## Why

The current computer adapter replaces registry tools with native/browser tools,
so obtaining computer access removes MCP connections from the same task. This
prevents the approved documentation-to-application workflow before desktop input
even begins.

## What Changes

- Compose enabled registry capabilities with the granted computer tools in
  task-first runs; connecting either provider never grants the other authority.
- Apply the capability broker's actor policy during discovery and again before
  dispatch. Denied/disabled tools must not enter the model's catalog.
- Keep arbitrary local execution excluded, preserve service/host filesystem
  separation, and retain exact MCP approval independently of computer policy.
- Permit MCP work before the first computer observation; retain observation
  requirements for computer actions and conservative final verification.
- Preserve legacy computer-only runs and their narrower behavior. Update the
  existing guides, regression tests and reliability evidence, not the renderer.

This is the provider-composition slice of stage 2 of the approved program,
following `agent-access-recovery`. Revision-bound capability identities,
large-catalog discovery, user-facing task restrictions, shared Chat jobs,
actor-scoped connection credentials, OAuth and protocol-v3 desktop authority
remain required follow-on slices; this change does not claim those capabilities.

## Capabilities

### New Capabilities

- `task-capability-composition`: combined discovery and execution of independently
  authorized registry and computer tools in one durable task.

### Modified Capabilities

None. The repository has no archived capability specifications.

## Impact

`internal/agents` registry discovery and durable execution, `internal/server`
computer-tool composition and host-file routing, agent/computer guides and tests.
No new public endpoints, dependencies, companion protocol or UI layout changes.
No installed-state migration, native side-effect tests, deployment or release.
