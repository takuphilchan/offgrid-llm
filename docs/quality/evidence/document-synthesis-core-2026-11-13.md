# Document synthesis core evidence — 2026-11-13

Implementation progress under `document-ingestion-reliability` 9.2. This adds the
internal source batching, model-candidate protocol and guarded local-provider
boundary needed by summary and requirements jobs. It does **not** enable these
workflows in HTTP, CLI, Python or the app. Existing public workflow discovery
continues to advertise comparison only.

## Implemented

- Verified immutable plans produce contiguous, deterministic source-unit batches.
  Unicode byte offsets and all primary text survive traversal without loss or
  duplication. Newline boundaries are preferred where available. Primary batches
  are bounded to 1–8 KiB; this is an allocation bound, never a token estimate.
- Explicit structural dependencies retain separate exact citations. A distant
  footnote exception travels with its clause. Omitted dependencies, excessive
  link counts and unavailable full cell metadata remain recorded gaps. Native
  cells retain raw values, formulas and cached results separately; returned cell
  metadata cannot mutate the immutable source graph through shared pointers.
- Batch identities bind the source plan, protocol, workflow, exact ranges,
  evidence and gaps. Verification reconstructs the batch, rather than trusting a
  caller-recomputed digest. Source access remains the storage/service owner's
  responsibility; an internal source graph is not an authorization cache.
- A local-provider interface pins model SHA-256, runtime identity, output
  language, context allocation and protocol. Exact prompt counting and generation
  share one lease. Response tokens plus 256 tokens of headroom must fit. Missing
  token counts do not fall back to character estimates. The lease ends after the
  batch, enabling scheduling between batches once the service adapter is wired.
- Current-access guards run before admission, before/after model calls and before
  returning a receipt. Runtime changes, incomplete output, cancellation and failed
  guards withhold the receipt. There is no automatic generation retry.
- Model output is bounded strict JSON. Duplicate/case-aliased fields, unknown
  fields and missing evidence inspection identities fail. Each candidate needs
  exact, unique quotes and primary evidence; conditions/exceptions retain their
  own quotes. Unsupported numeric tokens are withheld rather than normalized.
- Those checks establish source identity and conservative number transcription,
  **not semantic support or arithmetic correctness**. Candidates are marked
  `source_identity_only` and require semantic review. Model-reported inspection
  and absence of requirements are explicitly not verification. Empty/rejected
  output retains gaps. No candidate is automatically published as an answer.
- Shared link lookup now uses an augmented interval index instead of scanning
  every block for each span. It retains overlapping containers and original
  block order. Dependencies with identical spans use a stable ID tie-breaker,
  preventing nondeterministic batch identities.

## Executed verification

| Evidence | Result |
| --- | --- |
| `build/synthesis-regression-final.log` | Full uncached `internal/document`, `internal/rag`, `internal/sessions` and `internal/server` suites pass. This covers the shared link-index change through existing private and library paths. |
| `build/synthesis-boundaries-final.log` | Synthesis and interval-boundary tests pass, including the final case-alias, ambiguous quote and mutable-cell regressions. |
| `build/synthesis-link-index-benchmark.log` | Sparse interval queries: 176.8 ns/op at 20,000 blocks and 226.3 ns/op at 100,000 blocks, 8 B/op, one allocation. Windows amd64, Intel Core 7 240H. This excludes index construction, graph parsing, dependency traversal, retrieval and model inference; it is not a hardware-profile or end-to-end latency qualification. |

New tests cover byte-exact multilingual traversal, distant exceptions, dependency
budgets, partial cells, invalid cursors, plan/batch tampering, exact source reopening,
duplicate JSON keys, invented/ambiguous citations, missing inspection identities,
numeric sign/precision/script changes, model no-findings limits, pinned runtime
identity, context admission, each access-guard boundary and prevention of replay.
Randomized interval queries are compared with the original full-scan predicate,
including overlapping and empty spans. Source-size/layout checks, strict OpenSpec
validation and diff checks pass.

## Still required

The provider tests use deterministic test doubles. No real model generated a
summary or requirement in this increment. The production local-runtime adapter,
durable synthesis acceptance/checkpoints/recovery, support assessment and final
section synthesis, API/client parity, UI/exports and real-model qualification remain
open. In particular, this code does not claim that citation validity proves the
associated prose or that a numeric match establishes a correct calculation.

No interface strings changed. Existing native translation review, packaged/offline
tests, race tests, representative corpus and 8/16 GB hardware gates remain open.
No installation, publication or deployment was performed.
