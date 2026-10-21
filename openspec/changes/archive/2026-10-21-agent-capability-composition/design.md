# Design

## Context

See proposal.md for motivation. `browserRunTools.ToolsForTask` currently selects
one provider, and `Authorize` rejects all registry tools once a session exists.
The runner also requires observation before any non-runtime tool and labels all
high-risk calls with the computer approval mode. The capability broker already
accepts actor/run identities at execution. Existing guides describe the current
task-first runner and legacy computer sessions.

## Goals / Non-Goals

Goals: fix composition at discovery, authorization and dispatch; preserve the
existing durable intent/result, exact approval and uncertainty boundaries.
Non-goals: the remaining program slices listed in proposal.md. In particular,
this does not create a personal OAuth credential model or desktop-wide consent.

## Decisions

1. Add context-aware actor discovery to the registry and task adapter, retaining
   existing interfaces for library compatibility. Evaluate policy with no
   action arguments and without approval; approval-required tools are discoverable,
   denied tools are not. Execution remains the authoritative, argument-aware
   check. Snapshot before evaluating policy, avoiding callbacks under locks.
2. Compose registry tools only in task-first computer runs. Older computer-only
   runs remain restricted; a new provider must not broaden old consent.
   Reserved computer/access/artifact names never resolve to registry overrides.
3. Local `execute` capabilities stay excluded in task-first runs unless they are
   MCP tools, whose existing separate exact approval remains required. This
   closes the renamed shell/script bypass without treating MCP annotations as
   trusted classification. Service-file descriptions retain execution location.
4. Move computer error translation into the actual computer branch. Delegate
   registry authorization and execution without converting their errors into
   native errors. Dispatch by registered capability, not a name prefix.
5. Apply foreign-path checks even after computer attachment. Return corrective
   no-effects guidance in that state instead of replacing the current session or
   executing the service-file operation. Keep existing local-input routing before
   a session exists, including its independent registry policy check.
6. Scope first-observation ordering and computer approval labels to actual
   companion capabilities. Keep calls sequential while a computer session exists.
   Keep conservative completion checks: an MCP call after computer verification
   requires re-verification, since remote tools may change shared state.

## Risks / Trade-offs

- Larger combined catalogs can exceed small model contexts: context admission
  remains enforced; bounded relevance discovery is a separate required slice.
- Policy that denies discovery without arguments hides that tool conservatively;
  discovery never supplies invented arguments or executes providers.
- Public/token connection settings remain administrator-managed and workspace
  scoped until the actor-scoped connection lifecycle slice. This change applies
  existing actor policy, not a claim of isolated personal MCP credentials.
- Real app/model/platform workflows are unqualified here. Use isolated controlled
  tools and HTTP MCP fixtures, with explicit limitations in reliability evidence.

## Migration Plan

No persistence migration or API shape change. Existing exact approvals still
bind the current descriptor and arguments. Rollback restores exclusive provider
selection without deleting tasks or credentials. Do not deploy automatically.
