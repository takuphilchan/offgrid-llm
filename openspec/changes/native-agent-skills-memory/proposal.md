# Proposal

## Why

The approved native-agent program still lacks revision-bound reusable skills and
durable, opt-in scoped memory. Existing `internal/memory` compresses a transcript;
it is not an actor/profile memory store and must not be presented as one.

## What Changes

- Import user-supplied Agent Skills bundles into immutable owned revisions, pin
  profile attachments, disclose only bounded metadata until explicitly loaded,
  and retain non-executable drafts for human review.
- Add opt-in automatic memory scoped to actor/workspace/profile, with revisioned
  live consent, bounded model-facing reads/writes, concurrency checks and user
  inspection/deletion. Re-enabling consent cannot reauthorize an older job.
- Integrate resources with the existing canonical runner and authenticated
  service/CLI/shared UI, not another executor, conversation store or scheduler.
- Preserve existing jobs/profiles with no new skills, memory consent, grants or
  downloads. Skill tool hints, scripts and retrieved content never grant authority.

Acceptance: import/load the exact selected skill revision, reject traversal,
cross-owner references and changed digests, create an inspectable draft without
running code, retain a useful scoped memory only after explicit opt-in, and stop
retrieving/writing after revocation/deletion without erasing historical evidence.

## Capabilities

### New Capabilities

- `agent-skill-resources`: owned immutable imports, progressive loading and
  non-executable reviewed drafts.
- `agent-scoped-memory`: explicit live consent, profile scope, bounded automatic
  capture/retrieval, concurrency and user inspection/deletion.

### Modified Capabilities

None. The existing execution authority and task continuity contracts remain in
force; these context resources do not replace or broaden them.

## Impact

Depends on `native-agent-workspace-execution`. Add a focused context-resource store
and contracts, pinned profile/task metadata, owned authenticated APIs, CLI commands
and shared UI. Reuse application-owned SQLite and runner composition patterns.
No cloud inference, automatic downloads, package installation, automatic script
execution, sharing across actors/profiles, host grant restoration, deployment,
commits or publication. Real model usefulness remains separately qualified.
