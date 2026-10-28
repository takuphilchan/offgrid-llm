# Proposal

## Why

Native agents cannot bind a terminal's current directory or a desktop-selected folder to a job. File tools operate inside the service, shell is excluded, and the default CLI interactive loop starts unrelated ten-decision tasks. This prevents basic useful work despite a functioning inference service.

## What Changes

- Add explicit actor/job-bound execution contexts and a supervised, enrolled `offgrid-executor`; local Windows, Linux/WSL and macOS hosts remain distinct from service files and isolated containers.
- Add bounded workspace files, preconditioned edits and supervised foreground/background processes through the existing durable runner, registry and scheduler.
- Keep ordinary trusted-host commands autonomous after explicit session approval, disclose user-account authority, and preserve a separate approval path for sensitive/destructive work. Folder selection is not a shell sandbox.
- Compose effective toolsets with profiles, model readiness and current host grants. Computer consent never grants shell or files; old jobs/profiles gain no authority.
- Integrate execution references with canonical conversations and CLI/apps; replace the default terminal's independent-task loop with the existing conversation path without a client-local agent engine.
- Correct the observed one-shot terminal gap: stream normal interactive `agent run` requests in place, offer one local current-folder approval, expose explicit folder/access and detach controls, and preserve JSON/unattended submission behavior without service-filesystem substitution.
- Add reviewed writing from a task-owned user-input span. The model selects unique boundaries in the root request or an applied user correction; code preserves the selected UTF-8 bytes, persists them in the existing action intent and checks independent host read-back. This addresses observed exact-text drift without retyping literals through model output, adding grants, replacing ordinary writing or changing older jobs.
- Preserve the whole approved native-agent program: skills and scoped automatic memory belong to dependent `native-agent-skills-memory`; unattended occurrence/job scheduling belongs to `native-agent-automation`; model adapter qualification remains in `model-adaptive-execution`. These dependencies are required for the program, not silently deferred completion claims.

Acceptance: list the selected folder rather than container cwd; retain context for a follow-up read; independently verify an edit; report a test process's true exit; stop only owned processes; preserve unknown effects without replay; keep unrelated workspaces and existing computer grants isolated. Qualification requires real-model/backend evidence separately from fixtures.

## Capabilities

### Conversational admission and workspace trust refinement

Ordinary conversation must not initialize a file executor or ask for filesystem
access. Offer a bounded workspace-access request when local work is needed, then
attach the client-approved resource to the same canonical root at a safe boundary.
An optional local remembered-trust decision binds the exact canonical folder,
host, service workspace/actor and permission ceiling; expose inspect/revoke and
never migrate session-only approvals into remembered trust. CLI and apps share
the service contract and preserve separate desktop consent. Present concise
activity and one final answer, with identifiers and detailed steps available for
inspection. Test capability questions, creative requests, genuine file work and
follow-ups rather than treating greeting success as conversational qualification.

### New Capabilities
The release-readiness refinement aligns new foreground one-shot CLI context
selection with Chat's interactive allocation, retaining explicit extended
selection, legacy omission semantics and immutable saved allocations. Measure
cold loading separately from generation and exercise mixed real-model workflows;
improved feedback and smaller allocations do not themselves certify readiness.

- `agent-execution-workspaces`: actor/host/session-bound workspace identity, file access, enrollment and compatibility.
- `agent-process-runtime`: supervised, observable and cancellable process execution with durable operation identities.

### Modified Capabilities
- `task-capability-composition`: allow reviewed execution-provider tools only under their own explicit authority, without widening computer grants or enabling legacy local scripts.

## Impact

Existing durable agents, service authorization/OpenAPI, profile and session configuration, CLI, shared renderer, desktop packaging and a new executor binary. Additive persistence and protocol negotiation preserve old clients/jobs. No inference engine replacement, cloud fallback, model download, external integration redesign, deployment, commit or publication. SSH/cloud execution and automatic OS wake-up remain excluded.

### Interactive session experience

Extend the existing persistent CLI conversation with a single compact welcome,
profile/model identity, a discoverable slash-command menu and keyboard selection.
Keep session IDs and exact resume commands in `/session` and the exit receipt.
Commands inspect pinned skills, collaborators and tool attachments; they never
submit model prompts or silently attach resources. Preserve foreground streaming,
on-demand consent, cancellation, plain output and shared input ownership.
