# Design

## Context

See proposal.md for motivation. The current agent manager owns durable tasks and
immutable profile revisions in application-owned SQLite. The HTTP server borrows
the manager, authenticates actor/workspace and composes model tools. Existing
`internal/memory` is transcript compression, not consented profile memory.

## Goals / Non-Goals

Keep one runner, application storage owner and model scheduler. Add context data,
not a filesystem executor or skill package installer. Do not infer permission
from imported frontmatter, model output, paths or previous memory content.

## Decisions

### Additive resource tables behind the agent manager

Use the manager's existing database/lifecycle rather than a second task store.
Factor bounded skill parsing and canonical hashing into `internal/agentcontext`;
the manager owns persistence and actor/workspace/profile checks. Revision tables
store immutable skill manifests and bytes, with stable opaque IDs. Hash the
canonical sorted file list and bytes; verify on load. Resource limits: 64 files,
512 KiB total, 64 KiB each, SKILL.md at most 32 KiB. Paths are portable relative
POSIX names; reject traversal, backslashes, controls and case-fold collisions.

Import API takes an explicit list of file bytes, not service paths or executable
archives. CLI/UI read files only after user selection. Directory import skips no
files silently: links/special files or a limit failure reject the whole import.
This avoids archive extraction and its separate decompression/permission risks.
SKILL.md validates required Agent Skills name/description metadata and name
syntax. Optional metadata is inert. Supporting scripts are stored as text/data,
never invoked by loading. Unknown frontmatter fields are not authority.

### Pin references; disclose progressively

Profile settings gain bounded immutable skill references. Admission validates
ownership, digest and revision before persisting the pinned profile. Task context
gets only bounded names/descriptions/reference metadata. A reviewed context tool
loads SKILL.md or one named supporting file after independent pinned-reference
and current ownership checks. No implicit full bundle injection, extra tools,
outbound transfers, or host path interpretation. Skill drafting produces a
bounded non-executable owned draft with task ID; explicit review imports it as a
new immutable resource, then explicit profile revision attachment enables use.

### Separate live memory consent from immutable profile settings

A consent row is keyed by actor/workspace/profile, defaults disabled and has a
monotonic revision. User updates use compare-and-swap. Admission pins an enabled
revision; each tool call rechecks the exact revision and current profile state.
Disable/re-enable advances it, so neither an old job nor restored checkpoint
regains authority. Use a single transaction for consent checks and memory writes
to prevent a revoke/write race. User inspection/deletion remains available when
automatic use is disabled. Children have no independent memory-write consent.

Entries use opaque IDs, a short label, bounded text (8 KiB), provenance and CAS
revision. Cap scope size at 128 entries / 512 KiB and retrieval at 16 KiB.
Operation records bind stable canonical-job/tool-call identities to payload
digests and results; exact duplicates return the original result, conflicting
reuse fails. Deleted entries become tombstones and cannot be resurrected through
replay. Model writes consume the existing runner decision/tool budget: no extra
post-run inference, background memory agent or budget reset.

### Additive authenticated clients

Expose owned skill and profile-memory endpoints through existing Chat policy.
CLI and shared profile UI use those APIs; neither accesses service storage
directly. Memory has an explicit default-off toggle, inspect/edit/delete and clear
historical-evidence wording. Existing renderer layouts/locales and API clients
remain compatible. Profile edits affect future jobs, not live authority grants.

## Risks / Trade-offs

- Untrusted instructions or secrets in resources → clear provenance, bounded
  loading and unchanged authorization/outbound policy; content heuristics are not
  an authorization boundary.
- Useful automatic memory is model-dependent → deterministic consent, storage,
  concurrency and isolation tests first; actual recall quality remains separately
  measured, never inferred from successful persistence.
- Tombstones retain private content if snapshots are copied → deletion clears
  stored live content and operation response bodies; retain only bounded identity,
  revision and digest metadata. Historical tasks remain subject to their existing
  evidence-retention controls, not a promise of retroactive erasure.
- Existing manager file sizes → separate resource persistence files, no increase
  to source-size exceptions and no execution-loop duplication.

## Migration Plan

Create additive tables lazily through the manager storage boundary. Nil skill
attachments and absent memory pins preserve old profiles/tasks. Failed migration
leaves resources unavailable, never silently creates replacement task history.
Older binaries can read old jobs; rollback does not delete resource tables or
claim old software enforces new resource features. No deployment is included.
