# Conversation workers and immutable results: 2026-11-14

This record extends the [control foundation checkpoint](conversation-control-foundation-2026-11-13.md).
It records implementation checks, not model quality, release readiness or a
percentage of the complete architecture. The working tree also contains ongoing
document ingestion changes; these were preserved.

## Implemented contracts

- Schema 25 stores immutable, actor/workspace-scoped results and change-set
  records in the existing control database. Run settlement, record revisions,
  events and outbox writes share the task transaction. Upgrade backfills recorded
  outputs without inference or restored grants. New result identities bind bytes,
  instruction version, control epoch, outcome and source dependencies; legacy
  references retain their original identity contract.
- The result API pages exact UTF-8 output, including final citations, and checks
  ownership and live source access on every read. Go, Python, TypeScript, CLI
  `/result` and `session result`, and expandable UI results use that service API.
- Worker assignments persist expected results and delegation reasons. A pinned
  versioned policy defaults to four sessions, two active workers, one level and
  four initial calls. Follow-ups retain session identity. Budget requests move
  only unallocated root capacity, preserve parent verification capacity and
  commit with an idempotent receipt. Unknown costs never refund themselves.
- Durable worker questions are admitted by the conversation preview for generic
  tool runtimes. Native adapters retain their separate qualification gates.
  Question answers, side questions, corrections and new-work requests preserve
  their distinct control semantics and do not reopen existing work budgets.
- Foreground questions can continue using authorized context after a focused
  work source is revoked. Derived acceptance criteria and pending questions are
  withheld; corrections to inaccessible work remain blocked. Correction
  application rechecks source access after proposal preparation.
- Configuration inspection displays pinned project revision, provenance and
  restrictions using read-only requests. Project TOML accepts only versioned
  preferences and resource references; it cannot introduce credentials, grants
  or executable hooks.
- Historical images no longer block unrelated text or text-agent turns. The
  inference projection discloses unavailable image content and preserves user
  text. Saved bytes remain intact for a later vision turn. New image input still
  requires the installed vision capability and durable admission.

## Executed checks

Windows checks used isolated state under
`D:\offgrid-test-state\conversation-20261114`, with native backslash TEMP/TMP
paths. Earlier mixed-separator temporary paths caused path-comparison failures;
the corrected runs passed.

| Check | Observed result |
| --- | --- |
| Full Windows agents suite | Passed, 248.625 s |
| Full Windows server / CLI suites | Passed, 106.978 s / 1.652 s, before the last identity/question/image adjustments |
| Latest focused canonical result/integration server checks | Passed, 32.320 s |
| Historical-image continuity and existing image validation regressions | Passed, 1.302 s |
| Worker UI browser suite | 7 passed |
| Canonical conversation browser suite | 11 passed; one initial navigation timed out before assertions. After changing that navigation to DOMContentLoaded, its focused rerun passed. No claim of one clean combined run. |
| TypeScript check after configuration inspection | Passed |
| Python conversation API tests | 10 passed |
| Linux agents / CLI with race detector | Passed, 137.918 s / 4.429 s |
| Linux execution / inference / serviceclient with race detector | Passed, 76.476 s / 13.404 s / 0.176 s |
| Linux server with race detector | Passed, 277.843 s |
| Windows integration/recovery and worker integration after unsupported-backend repair | Passed, execution 16.441 s / agents 2.246 s |
| Generated API, renderer production build, source-size ratchet | Passed; build retains its existing large-chunk warning |
| Documentation and OpenSpec | 7 documentation tests passed; 384 maintained Markdown files checked, zero link problems; change validation passed |

Linux race command:

```sh
TMPDIR=/tmp/offgrid-conversation-20261114 GOMAXPROCS=4 CGO_ENABLED=1 \
go test -race -json -p 2 -parallel 4 -timeout 15m \
  ./internal/agents ./internal/server ./internal/cli \
  ./internal/execution ./internal/inference ./internal/serviceclient
```

The first Linux attempt incorrectly used `/mnt/d` as test temporary storage.
That filesystem did not provide the native permissions and replacement behavior
the tests require. Its failures are not product qualification. The corrected run
uses WSL's native filesystem; the distribution is stored at `D:\WSL\Ubuntu`.
The latest image and unsupported-backend changes were made after the broad race
binary was built and received separate focused Windows tests.

## Additional recovery repair

An unsupported replacement backend that refuses before any per-file dispatch
now produces an `abandoned` integration with its limitation and retained sealed
proposal. File downloads verify the retained digest; repeating the integration
command observes the recorded refusal without attempting writes. Uncertain
dispatched operations retain their existing conservative classification.

The Windows crash-helper test now pins `GOTMPDIR` as well as OS temp variables.
Go 1.26 testing.TempDir otherwise inherited the outer build directory and escaped
the parent fixture's directory. The corrected real-process crash tests passed.

A subsequent browser run passed 18/19 cases; its remaining reload waited for the
page load event. It now uses DOMContentLoaded plus the existing UI assertions.
A combined browser attempt then exhausted Node process memory before tests ran.
The extra Linux race pass completed sessions, users, cache, runs and storage,
but ended without a completion record for the remaining packages. Those extra
packages are not recorded as passing. A serial browser rerun follows; this does
not supersede the successful six-package core race run above.

The serial rerun completed successfully: **29 browser tests passed in 2.3 min**
across `canonical-conversation.spec.ts`, `worker-controls.spec.ts` and
`work-changes.spec.ts`. This includes the repaired reload case and verified
proposal downloads after an abandoned integration. The prior navigation and
memory failures remain recorded above to distinguish failed attempts from this
clean run. The source-size check and maintained-document link check also passed
after the recovery changes (385 Markdown files, zero link problems).

## Acceptance mapping

- **9.1:** control-store retry/concurrency/transaction tests; result transaction
  failure and upgrade tests; change-set HTTP records and continued-work inspection.
- **9.3:** preference precedence, restrictions, user-default provenance, project
  file rejection, host-folder binding and authority intersection tests.
- **11.2:** assignment metadata, worker dependency invalidation, retained result
  paging, question review/answer/recovery and partial synthesis tests.
- **11.3:** delegation correction fences, late Stop results, duplicate mailbox
  application, stopped-worker continuation, source revocation and recovery tests.
- **12.1:** input/context, output/inference, tool/process/storage reservation,
  child settlement, dynamic budget request and parent-reserve tests, including
  failed receipts and restart without invented refunds.
- **13.2:** sealed proposals, combined dependency candidates, validation receipts,
  overlap/current-version checks, per-file quarantine and preserved-file retrieval;
  native integration tests and real process exits after every file operation.

These mappings close these specific implementation tasks. They do not close
the remaining client, environment, scheduling or qualification tasks.

## Remaining limitations

Canonical inbox multimodal admission and complete document/Knowledge parity,
progressive project instruction loading, measured alternative-model selection,
environment template reuse/cleanup, background-grant setup and the complete
backend fault/rollout qualification remain open. Default workers still use the
current model. Representative-user studies and matching-hardware model workloads
have not been executed. Admission flags must not be interpreted as proof of
automatic delegation quality. Existing work remains subject to its pinned policy.
