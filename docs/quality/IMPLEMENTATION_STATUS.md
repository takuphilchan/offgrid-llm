# Transformation implementation status

Updated: 2026-11-21. The complete transformation remains in progress.

The [master requirement tracker](TRANSFORMATION_TRACKER.md) records each M0–M8
requirement's implementation owner, tests/evidence, dependencies and remaining
work. The [roadmap](../ROADMAP.md) remains the product scope authority. Update
the tracker after every integrated increment; do not derive completion from
checked OpenSpec tasks alone.

The completed first increment is
[browser execution reliability](evidence/browser-execution-2026-11-20.md):
trusted safe failures, no uncertain replay, durable effect receipts, bounded
discovery, target/form/document freshness and conservative approval classification.
Its record explains affected files, regressions, actual checks and limitations.
It does not complete attached signed-in browsing, the wider agent work,
modalities, developer transports or qualification.

Current worktree increments have the following evidence and remaining work:

- [Worker environment and manifests](evidence/worker-environment-2026-11-21.md)
  now pin the minimal inherited template and publish local command evidence before
  dispatch. [Completion ownership](evidence/worker-completion-2026-11-21.md) fixes
  six reproduced foreign-owner/unconfirmed-receipt cases. Windows execution,
  agents, executor and server checks pass at their recorded source checkpoints;
  [Linux namespace ownership](evidence/worker-linux-namespace-2026-11-21.md) now
  passes the full execution/inference race suites and an authenticated native
  compiler journey, including shared ownership and private nested-runtime pins.
  Custom templates, toolchain/cache policy and background supervision remain open.

- [Managed task workspaces](evidence/browser-workspaces-2026-11-20.md): eight tabs,
  locally consented origins, independent egress enforcement and exact observation
  ownership; [consent UI](evidence/browser-workspace-consent-2026-11-20.md) has
  isolated renderer and 29-locale coverage. Planned source/fixture checks are
  complete, including the full server race suite on the recorded verified Linux
  source snapshot. Attached browsing and broader M2 remain open.
- [Structured browser controls](evidence/browser-structured-controls-2026-11-20.md):
  literal search, open-shadow/contenteditable/custom controls, exact nested and
  separate-process frame selection, protected capture coverage and additive
  negotiation. The complete companion suite passes 114/114; independent review
  and Go/client integration checks are underway. This does not qualify real-model
  browser tasks or signed-in attachment.
- [Local model policy](evidence/local-model-policy-2026-11-20.md): immutable
  candidate/evidence pins and explicit qualified fallback admission. Independent
  review found a final-context integration gap; the correction passed permanent
  HTTP/intent regressions, focused race tests and the independent reproduction.
  Automatic switching remains open.
- [HTTP inference endpoints](evidence/http-inference-endpoints-2026-11-20.md):
  opt-in enrolled Ollama/vLLM text routes with pinned transport and authority.
  Namespace collision and current background-actor revocation regressions pass.
  [Managed Windows Ollama](evidence/managed-ollama-windows-2026-11-20.md) has isolated
  process/connection ownership evidence; Linux lifecycle work is underway.
  Real-runtime qualification and tool-qualified routing remain open.
- [Endpoint owner configuration](evidence/endpoint-owner-configuration-2026-11-21.md):
  typed creation/update/review/reconciliation, exact credential/template controls,
  no automatic write replay and all 29 authored locale catalogs. Actual Windows
  service/UI create-update-lost-response-restart passes; automatic revocation is
  now distinct from human access review. Capacity maintenance, damaged-state
  recovery, installed/managed-runtime and human qualification keep task 8.4.4 open.
  The journal follow-up rejects malformed receipts and preserves settled history;
  Windows, renderer and actual service checks pass, while the latest full Linux
  server race exceeds its local eight-minute package deadline. The subsequent
  [task writer correction](evidence/task-writer-reservation-2026-11-21.md) passes
  full agents/API/config race checks but still reaches the server deadline. This is
  unresolved aggregate verification, not a green combined checkpoint.

Encrypted workspace backups now have storage and CLI implementations, independent
age interoperability, Windows private-file checks and isolated Linux/Windows
terminal tests. Independent review's memory-bound, temporary-state, publication,
ownership and secret-input corrections now pass full affected normal and Linux
race checks, with the final key-file opener regression also passing. Trusted
desktop maintenance integration is underway. [CLI evidence](evidence/encrypted-backup-cli-2026-11-20.md)
and [storage evidence](evidence/encrypted-workspace-backups-2026-11-20.md) retain
actual outcomes; administrator controls and real recovery qualification remain open.

Current failures and qualification gaps remain in
[known issues](KNOWN_ISSUES.md). Dated implementation and test records are indexed
in [verification evidence](VERIFICATION_EVIDENCE.md).
