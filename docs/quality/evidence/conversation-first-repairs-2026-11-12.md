# Conversation-first regression repairs — 2026-11-12

This records the first implementation increment of the replacement architecture
in `agent-conversation-workflows`. It is not qualification of the complete
architecture, a release approval, or a measured 9/10 product assessment.

## Implemented behavior

- Specialist names no longer force a delegation tool call or a two-child schema.
  Native document discovery no longer depends on English prompt matching. A
  negotiated document provider does not suppress ordinary browser observation;
  typed observations determine the active document/control protocol.
- Readiness checks operate on narrowed parent and child resources. Disabling an
  unavailable tool no longer prevents an otherwise usable conversation. Original
  immutable profile identities and capability ceilings remain intact.
- Accepted agent turns retain a versioned configuration in the existing agent
  database. Later turns inherit omitted restrictions, including across profile
  revisions. The renderer restores the saved specialization and restrictions.
  Unsubmitted drafts and missing historical browser preferences are not claimed
  to be saved configuration. The new canonical conversation store is still pending.
- Context assembly includes linked failed/cancelled attempts and applied or
  pending corrections, distinguishing accepted from applied instructions. The
  fixed 24,000-byte cutoff is removed. Model accounting can archive a long prior
  answer while retaining user constraints and exact retrievable answer bytes.
- Child profiles pin their authorized skill metadata and expose skill reads
  without inheriting memory consent or skill-draft writes. Child access requires
  a matching parent graph and immutable profile digest.
- Parent and dependent workers receive digest-pinned full-result references.
  Retrieval checks actor, graph/dependency membership, settled state, digest,
  byte offsets and current source access. Dependency-derived source identifiers
  propagate into the dependent task. Partial delegation includes useful recorded
  results, limitations and retrieval references, without claiming verification.
- Delegation corrections atomically fence child dispatch before cancellation.
  Application waits for active children to settle and never crosses an uncertain
  operation. Duplicate input is idempotent. Superseded results remain stale
  evidence; their budget allocations are not reopened. Paused work stays paused.
- Agent schema 5 fences older binaries from the new correction semantics. The
  existing transactional migration retains a verified database backup and manifest.

## Regression coverage

The tests cover negative specialist mentions, multilingual capability discovery,
disabled unavailable parent/child resources, configuration reload and profile
revision, failed-attempt context, corrected constraints, long answers and final
citations, unauthorized result retrieval, revoked source access, child skills,
late child generation, duplicate corrections, uncertain children, paused graphs,
schema upgrade backup, CLI steering and observation-only browser reload.

Relevant fixtures are in `internal/agents`, `internal/server`, `internal/cli`
and `apps/web/tests/agent-chat.spec.ts`. They use temporary stores and synthetic
models/hosts. Browser tests run on an isolated Vite fixture port, not the
installed service. They do not qualify real model decisions or native applications.

## Verification record

- The final Windows `go test ./... -timeout=300s` passed across all packages.
  An earlier uncached full run also passed across 38 packages.
- Targeted regression suites passed after configuration persistence and module
  extraction, including the two native-host bridge tests that build fixture binaries.
- API generation/checking, TypeScript checking, the web build, source-size ratchet,
  application layout, maintained-document checking and OpenSpec validation passed.
  Final API checking, web build, source-size and documentation reruns also passed.
- The full `agent-chat.spec.ts` browser suite passed: **157 tests**, zero failures,
  flakes or skips, including steering and configuration restoration across reload.
- A later full Go run passed 37 packages but encountered
  `TestLinkedEvidenceBoundsAndRevocation` in concurrently modified RAG work.
  Its isolated rerun and the subsequent full Windows suite passed. The failed
  invocation remains recorded; it is not counted as a successful run.
- The CI race package set ran under the existing WSL Ubuntu Go 1.26.6/GCC
  toolchain: 20 packages passed, including agents, server, CLI, execution,
  inference, storage and sessions. `cmd/offgrid-executor` initially hit its
  fixture's 60-second subprocess-build deadline during concurrent compilation;
  rerunning that complete package alone with `-race -count=1` passed in 31 seconds.
  No race was reported. All 21 packages therefore have passing race runs,
  although the first combined invocation failed at that build deadline.
- Native Windows race execution remains unrun (`CGO_ENABLED=0`). No compiler,
  model or runtime was installed for these checks; the WSL toolchain already existed.

## Still required

The following replacement increments remain unimplemented: canonical projects,
conversations and work identities; transactional general commands/events/outboxes
and session authority migration; the default assistant shared across CLI/UI;
resumable worker sessions/mailboxes; the expanded budget ledger and fair scheduler;
independent execution environments and change-set integration; background grant
lifetime changes; model/backend/hardware/usability qualification and rollout gates.

The repair still uses existing job configuration and legacy session linkage.
It does not provide the future separately editable conversation configuration
resource, complete preference provenance, partial authorized-context reconstruction
after revocation, arbitrary cross-conversation work linking, or new execution
worker capabilities. Existing execution and authorization boundaries remain in force.
