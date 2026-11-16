# Durable document comparison evidence — 2026-11-13

Implementation progress under document-ingestion-reliability 9.2. This delivers
the deterministic comparison backend and client contracts, not the full analysis
workflow or completion of the seven-phase upgrade.

## Behavior

- Shared analysis plans bind workflow, source document IDs, immutable extraction
  revisions, original hashes and every retained unit. Unknown/partial extraction,
  figures, unresolved review items, unavailable structure and text outside declared
  units remain explicit gaps. Source traversal is not original-file understanding.
- Owner-private SQLite jobs use the existing import lifecycle pattern: bounded
  queue, one worker, attempt leases, restart recovery, cancellation and explicit
  retry. Each unit pair publishes its result and progress in one transaction.
  Retry begins after committed checkpoints; stale/cancelled attempts cannot publish.
- Comparison computes exact line additions/removals using unique monotonic anchors
  and repeated-line prefix/suffix matching. It avoids a quadratic diff matrix.
  This is not a minimum-edit or semantic-alignment claim. Unique sheet names align
  native cell addresses; other units use labelled positional alignment. Renamed or
  ambiguous sheets remain unmatched. Cell metadata is compared separately, including
  formulas, cached results, raw values, formats and hidden/merge attributes.
- Full source graphs are verified once for worker traversal, rather than rehashed
  for each unit. Results are bounded pages of references. Delivery validates their
  source/unit/block/range and SHA-256 against retained bytes. Source reopening is
  limited to 256 KiB with explicit truncation and optional native cell metadata.
  No formulas, macros or external links execute, and no inference model is used.
- Current access to both sources is required at acceptance, checkpoints, status,
  results and source reopening. Job ownership is separate from collection access;
  administrators cannot read another actor's analysis. Owners can stop their own
  jobs after source revocation without receiving source details.
- Deleting either source cascades to the analysis and checkpoints. Existing legal
  holds protect the source and derived job. Existing databases receive a verified
  pre-upgrade snapshot before analysis tables are introduced; snapshots remain a
  separate retention domain. Creation audit events omit document bodies.
- HTTP exposes create/status/results/source/cancel/retry under
  `/v1/document-analyses`. The `document-comparison-jobs-v1` capability advertises
  this specific stage. CLI commands and Go clients are implemented; TypeScript
  contracts are generated. Python SDK methods and tests are authored but unrun.
  New renderer controls and analysis exports are not yet implemented.

The public workflow currently accepts comparison only. Summary and requirements
requests return an explicit unavailable response rather than pretending that a
text diff implements synthesis. The shared traversal plan supports their future
processors, which remain required work.

## Verification

- `build/document-analysis-go-final.log`: uncached document, RAG, server,
  serviceclient, CLI and API suites pass. This is the affected-package suite,
  not `go test ./...` or a race run.
- `build/document-analysis-final-focused.log`: traversal, comparison, job recovery,
  authenticated HTTP, CLI/client receipt and route-registration regressions pass.
- `build/document-analysis-office-recovery.log`: actual native DOCX and XLSX parser
  fixtures pass through persistent jobs and exact source reopening. The DOCX case
  changes a footnote exception; the XLSX case changes a formula while preserving
  its cached result. Both retain their format/coverage limitations.
- Recovery tests commit the first of two unit pairs, interrupt/restart, reject the
  old attempt and resume the second. An injected SQLite abort proves result/progress
  rollback without an orphan checkpoint. Review flags survive retries. Other tests
  cover cancellation, current-access revocation, outsider denial, idempotency,
  held-source retention, deletion cascade and forged saved-citation rejection.
- Migration tests preserve document revision identities and private collection
  bindings in the snapshot and live store. Native text, Unicode spans and exact
  numeric strings are not rewritten by comparison.
- `build/document-analysis-last-focused.log`: the final unassigned-text coverage
  regression and CLI comparison command pass after the full run.
- API generation/check, TypeScript check and renderer build pass in
  `document-analysis-api-generation.log`, `document-analysis-api-check.log`,
  `document-analysis-typescript.log` and `document-analysis-build.log`.
  The existing renderer bundle-size warning remains.
- Source-size, application/developer layout, strict OpenSpec and diff checks pass
  in the corresponding `build/document-analysis-*` logs.

The first route regression failed because five newly registered endpoints changed
the frozen route count from 168 to 173. Their Knowledge permission wrappers and
registration order were reviewed, and the count/manifest were updated together.
The final route regression passes; the earlier failure log is retained. An initial
`TestRoute` selector matched no tests and is not counted as verification.

## Open work and qualification

Model-written comparison explanations, summary/requirements processing and support
validation, private-attachment analysis integration, renderer controls, exports and
broader structural-relationship comparison remain open. Positional page matching
is not semantic section alignment. Large-library/hardware performance, representative
document quality, real-model answers, clean-install packaging, race tests, Python
execution, human linguistic review and departmental pilot gates are not established
by these fixtures. No installation, publication, model download or live Computer
Use was performed. The full upgrade and task 9.2 remain incomplete.
