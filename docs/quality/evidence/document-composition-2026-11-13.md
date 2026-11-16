# Document summary composition — 2026-11-13

This source increment connects section drafts to a composed summary. It does not
close the full document upgrade or establish public-sector readiness.

## Implemented behavior

- Summary composition uses a deterministic balanced merge tree. Every input pins
  its original analysis/batch/candidate digest, review revision and keep/exclude
  decision. Requirements remain individual rows rather than compressed summaries.
- Each merge counts the exact prompt and generates under the same pinned local
  model lease. Results are bounded to four sections and checked for input identity,
  original-source numeric consistency and explicit omissions. Newly written prose
  never inherits human approval. Omission gaps survive subsequent merge levels.
- Atomic checkpoints persist validated receipts and private raw responses.
  Restart/retry replays committed receipts without repeating completed inference.
  Cancellation, current source access, review changes and model identity are
  checked at work boundaries. One shared worker and bounded queue limit contention.
- SQLite migration snapshots preserve existing analyses and review histories.
  Source deletion cascades to compositions/checkpoints; legal holds prevent it.
  Exports and snapshots retain their separate retention boundaries.
- Authenticated composition status/history, report, evidence, cancel/retry and
  export endpoints use current source access and private ownership. Administrative
  permissions do not grant private report access. Report responses are compact;
  original-source candidates are paginated eight at a time.
- Reports preserve extraction, candidate and composition gaps, unrepresented
  source-batch counts and stale-review state. Bounded JSON/CSV/Markdown exports
  retain original candidates, including exclusions, source identities and review
  snapshots. Source access is rechecked and preparation audited without bodies.
- The existing analysis workspace has report and section-draft views, an original
  evidence panel, progress/cancellation/retry, history and exports. It shares theme
  tokens and the source reader, restores keyboard focus and clears revoked data.
- Go/CLI support is implemented. Python parity and tests are authored; no working
  Python interpreter was available here, so those tests were not executed.

## Real-model findings

The opt-in fixture used existing local assets through a disposable registry and
database, with no downloads or changes to installed-user metadata. Model SHA-256:
`626b4a6678b86442240e33df819e00132d3ba7dddfe1cdc4fbb18e0a9615c62d`.
Runtime executable SHA-256:
`5f3fcc38bf345acb565381ac43ad712ab6302e8ab68f9423d8a3446b5030043d`.
The service test binary is separately pinned in runtime identity and changes when
rebuilt. Context was 8,192 tokens with six CPU threads.

Initial runs exposed three real failures: summary generation populated fields
that the existing compiler rejects; a merge both used and omitted the same input;
and Chinese output translated a quotation that must remain native. Failed logs
are retained in `build/composition-real-runtime.log` and
`build/composition-real-runtime-fixed.log`.

The generation schema now matches the existing summary field constraints.
One/two-input merges constrain disjoint used/omitted sets in their grammar;
larger merges retain strict post-generation validation. Short source batches
(at most 4 KiB) offer bounded exact native quotation choices, including complete
evidence and sentence passages. Larger batches retain exact-quote validation;
this increment does not claim their multilingual generation reliability.

`build/composition-real-runtime-native-quotes.log` passes English and Chinese
source traversal, local composition, original evidence readback and export.
The tiny fixture took approximately 74 and 58 seconds respectively; its Chinese
merge took about 26 seconds. Both outputs retain pending review and extraction
gaps. This is one small synthetic policy, not a scale, hardware or semantic
accuracy benchmark. The model still uses broad quotations and needs review.

## Automated evidence

- Core, durable-store, HTTP and client targeted checks pass in
  `build/composition-focused-tests.log`. They include pinned source/model/review
  identity, omissions through multiple levels, tampering, rollback, restart,
  cancellation, concurrent revocation, private ownership, idempotency and export.
- The initial 32-test renderer suite passes in `build/composition-ui-tests.log`,
  including all 29 locale controls, RTL/narrow layouts, source focus, forged-source
  rejection, uncertain-create recovery and revoked-cache removal. Visual inspection
  then identified and corrected report padding and duplicated status framing.
- Catalog inventory has 1,476 authored messages per locale and zero missing
  messages. Native approval remains pending. Updated translator packs are in
  `build/document-composition-translators-20261113`.
- Production renderer build, generated API check, source-size/layout checks,
  translation consistency and both OpenSpec validations pass. The build retains
  the existing large-chunk advisory. A final TypeScript build caught an improperly
  narrowed history-state lookup; the lookup is corrected and the final build
  passes in `build/composition-build-final.log`.
- Full affected document, RAG, service-client, CLI and API suites pass in
  `build/composition-full-go.log`. The full server run failed a process-accounting
  deadline during concurrent browser work. A subsequent server-only run failed
  the workspace-edit completion deadline instead. Both tests pass together in
  isolation in `build/composition-server-isolated-recheck.log` (2.0 seconds).
  These reruns do not constitute a clean full-server run; the original failures
  remain recorded in `build/composition-full-go.log` and
  `build/composition-server-isolated.log`.
- The combined comparison/synthesis/review/composition renderer run passed 136
  of 141 tests; five browser startup/loading checks timed out. All five pass in
  the isolated rerun, alongside final English/Arabic layout checks. Logs are
  `build/composition-full-ui.log` and `build/composition-ui-isolated.log`.
  The additional keyboard/enlarged-display test passes in
  `build/composition-zoom-status-final.log`, including cancellation, retry,
  source focus and the history selector's live completed status. Thus all 142
  cases have passing evidence, but not in one clean combined run.
- Final desktop/narrow English/Arabic and enlarged-display screenshots were
  inspected in `build/composition-visuals-20261113-final`. Padding, source-panel
  stacking and completed history status are corrected. No live computer control
  was used. This is fixture-based presentation evidence, not human accessibility
  certification or native linguistic review.

## Still open

Independent support assessment, private-attachment analysis integration,
representative multilingual/model qualification and the broader OCR/layout,
library/departmental and archive phases remain open. Human review, 8/16 GB
hardware and scale qualification, packaged offline installations, race checks and
the departmental pilot are not established by these fixtures. No installation,
publication or deployment was performed.
