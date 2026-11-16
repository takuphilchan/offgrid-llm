# Durable document candidate workflows

This increment connects source traversal to durable SQLite jobs, the owned local
llama runtime, HTTP, Go/CLI clients and a localized renderer workspace. It does
not complete the seven-phase document upgrade or certify generated claims.

## Implemented

- Summary and requirements jobs pin immutable sources, model digest, runtime
  identity, output language and context. Batch receipts and byte/unit progress
  commit together. Explicit retry resumes after the last committed batch.
- Cancellation, restart, stale attempts and source revocation prevent late
  publication. Legal holds protect originals; deletion cascades to job config,
  checkpoints and private retained responses. Migration snapshots retain prior
  comparison jobs and source ownership.
- The local adapter holds one inference lease across exact prompt counting and
  generation, yields between batches, records observed process/configuration,
  template and executable identities, and disallows runtime downloads. Requests
  have bounded responses, constrained JSON and no hidden generation retry.
  Prompts expose evidence IDs and text, keeping citation offsets/hashes owned by
  the service. Artifact hashing cost and broader model behavior still need
  qualification. Runtime identity is not a
  complete OS dynamic-library inventory.
- `/v1/document-analyses/synthesis`, `/synthesis/results` and `/synthesis/retry`
  expose private jobs and history; existing cancellation supports both families.
  These additive candidate endpoints preserve comparison clients. Chat and
  Knowledge authority are required to initiate model work.
- The analysis workspace selects sources and an installed model, offers summary
  or requirements, and displays draft sections beside digest-checked quotations.
  It includes history, cancellation/retry, source inspection at pinned revisions,
  narrow/RTL layouts and keyboard focus restoration. Read failures clear evidence.
- Go client and CLI add `synthesise`, `syntheses`, `synthesis`,
  `synthesis-results`, `retry-synthesis` and `synthesis-export`. Python `kb.synthesis` parity is
  authored; Python execution remains unavailable in this environment.
- Draft JSON/CSV/Markdown exports verify every persisted batch and its contiguous
  traversal, preserve conditions/exceptions and coverage, and fail without a
  partial artifact on tampering, size limits or cancellation. Access is rechecked
  and preparation audited without bodies. CSV protects formula-like cells while
  retaining exact values and references in metadata. Exports remain unreviewed.
- Whole-value calendar checks allow unambiguous numeric Chinese/Japanese/Korean
  and ISO dates to match cited native values, including English named-month
  sources. Wrong month/day/year, impossible dates and inferred years are rejected.
  A matched month never permits that numeral as an unrelated new quantity.
  This is a bounded equivalence check, not general date interpretation. New
  receipts pin `source-values-v2`; historical literal-number receipts retain
  their original decisions rather than being silently reinterpreted.

## Evidence and limitations

- `build/synthesis-durable-tests.log`: document synthesis and RAG analysis tests
  pass, including crash/restart, atomic rollback, tampering, cancellation,
  revocation, retention, migration and explicit model-call retry behavior.
- `build/synthesis-runtime-initial.log`: local-only missing-runtime and bounded
  transport tests pass, including refusal to retry a busy response.
- `build/synthesis-http-tests.log`: authenticated HTTP traversal/results/history
  and revocation pass using a deterministic provider fixture.
- `build/synthesis-regression.log`: full uncached document, RAG, inference,
  serviceclient, CLI and API packages pass. The server package's only failure was
  its hard-coded route count. `build/synthesis-server-corrected.log` passes the
  corrected route contract and affected analysis HTTP tests. The later full
  server suite passed in `build/synthesis-final-regression.log` (374.7 s), as did
  inference, serviceclient, CLI, document and API packages. A RAG database-open
  timeout occurred under competing CPU-heavy workloads. The complete RAG,
  document, serviceclient and CLI rerun passed in
  `build/synthesis-final-storage-clients.log` (RAG 18.0 s). These failures remain
  in the logs; no timeout limit was relaxed to obtain passing results.
- Initial renderer checks caught a model-selector bug excluding the valid `chat`
  type and a stale unknown-outcome warning; both were corrected. The original
  31-check synthesis suite and its 32-check export extension passed. The combined
  comparison/synthesis suite passed 104 checks with three initial startup
  timeouts; all three passed in isolation in `build/synthesis-comparison-recheck.log`
  (10.2 s). All 29 synthesis locale fixtures passed with model-identity validation.
  TypeScript and production build passed in `build/synthesis-build-final.log`;
  Vite retains its existing large-chunk advisory. Arabic desktop/dark/narrow and
  English/light layouts were visually inspected. Final screenshots are retained
  in `build/document-synthesis-visuals-20261113`; the two final locale checks pass.
- Translation inventory reports 1,458/1,458 authored messages per language.
  Reviewer packs are in `build/document-synthesis-translator-review-20261113`.
  Native linguistic approval is still pending; catalog completeness is not it.

## Actual local-model evidence

`TestRealDocumentSynthesisHTTP` uses disposable source/account/registry/cache
state and a read-only hard link to an explicitly supplied existing GGUF. It
exercises authenticated HTTP creation, the actual owned llama process, exact
token counting, constrained generation, durable receipts and CSV export. It does
not read installed conversations or change installed model bytes.

- Existing Qwen2.5 3B Q4_K_M; model SHA-256
  `626b4a6678b86442240e33df819e00132d3ba7dddfe1cdc4fbb18e0a9615c62d`.
- CPU, six threads, 8,192-token context, 2,048 maximum output tokens.
- Runtime/service identity SHA-256
  `30cabe6267476b86e32f80fd7996b1f632d8eb5f96633e22531b88f2fe87b25d`
  for this test binary; rebuilding the service changes this identity.
- `build/synthesis-actual-model-dates-verified.log`: English and Chinese both
  passed, approximately 29.84 s and 28.52 s for these tiny synthetic policies.
  This is not a latency/scale benchmark, nor an 8 GB qualification result.
- Earlier actual-model runs exposed copied citation formats and translated-date
  rejection. A numeric-regex grammar experiment caused runtime failure and was
  removed. The final implementation constrains the JSON envelope and validates
  complete date values after generation. Earlier failed logs remain.
- The fixture model grouped multiple requirements into one candidate and used
  broad quotation spans. Its semantic precision is **not approved**. No native
  reviewer, cross-language answer benchmark or representative pilot signed off
  these outputs. Synthetic test logs contain fixture text; production logs do
  not receive raw document responses.

`build/synthesis-final-focused.log` passes the affected document, RAG, inference,
Go client, CLI and server checks, including versioned receipt compatibility and
the 178-route contract. Source-size, source layout and UI literal checks pass.
Python tests were authored but not executed; race and hardware gates stay open.

Candidate traversal ends in `awaiting_review`, never an approved or exhaustive
answer. Exact quote/number checks establish source identity, not semantic support.
Human review decisions are implemented in the subsequent
[review increment](document-synthesis-review-2026-11-13.md).
Support assessment, final cross-section synthesis,
private-attachment analysis integration, actual-model quality, hardware/latency
and packaged/offline qualification
remain required. Existing broader OCR/layout, library and departmental gates are
also open. No installation, publication, or deployment was performed.
