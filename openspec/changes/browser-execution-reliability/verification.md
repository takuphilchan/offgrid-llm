# Verification: browser-execution-reliability

Verified 2026-11-20 against proposal, design, delta specification, tasks, source
diff and executed tests. This review was performed by the implementing agent;
it is not independent review or release approval.

| Dimension | Result |
|---|---|
| Completeness | 11 implementation/check-record tasks completed; five added requirements mapped. The full M0–M8 program remains open. |
| Correctness | All 12 specified scenarios have implementation and automated coverage. Real models and installed hardware are not verified here. |
| Coherence | Existing companion journal, capability checks, runner budget and service receipt storage reused. Additive discovery negotiation preserves legacy tools and probes. No dependency or second executor added. |

## Requirement and scenario mapping

| Requirement / scenarios | Implementation | Executed coverage |
|---|---|---|
| Page content cannot grant authority / misleading Save label | `apps/computer/approval-policy.mjs` | Approval-policy and session suites; scoped/full-task and prohibited classes remain distinct |
| Separate effects and observations / failed follow-up read, unknown mutation, acknowledged receipt followed by restart | `apps/computer/browser.mjs:114`, `internal/server/computer_receipt.go`, `internal/agents/computer_receipt.go:32` | Browser recovery and download failure tests; authenticated approval/receipt workflow; browser receipt restart test with `verified:false` preserved |
| Safe durable recovery / stale control, redelivered failure, interrupted read and legacy intent | `apps/computer/session.mjs:101`, `internal/server/browser_recovery.go:19`, `internal/agents/browser_recovery.go`, `internal/agents/proposal_recovery.go`, `internal/agents/tool_failure.go` | Session redelivery; typed hub/service failure tests; bounded durable read and authorization repair; pinned descriptor rejection; read-intent restart tests |
| Bounded discovery / large page and later dropdown options | `apps/computer/browser-observation.mjs:47`, `internal/computer/browser_features.go:28`, service tool filtering | 410 hidden candidates, 105 visible controls, Unicode/malformed Unicode, 130 options; guessed/stale references; byte bounds; old/new feature and probe contracts |
| Target freshness / updating clock, silent form edit, same-address reload | `apps/computer/browser-observation.mjs:31` | Real Chromium tests for unrelated text, replaced targets, altered labels/form action, silent values and same-URL document replacement |

## Findings and limits

- **Warning — workspace hygiene:** `node scripts/maintenance/check-repository-hygiene.mjs` reports the pre-existing Go assessment under `build/document-assessment-20261112`. Preserve that artifact; arrange an explicit relocation/exclusion before claiming the working directory passes this gate.
- **Warning — test timing:** the first Linux race invocation hit the executor fixture's build deadline. The affected rerun passed; retain both results and investigate if the timeout repeats on CI. No production timeout or test deadline was relaxed.
- **Not verified:** full rendered UI and packaged/native matrices, physical hardware, real-model task corpus, soak, independent review and pilot. These remain in the roadmap/readiness contract and are not certified by this increment.

No missing implementation was found among this increment's five requirements.
Source checks and the passing rerun do not erase the hygiene failure or establish
release readiness. The change remains unarchived. Exact commands and outcomes:
[dated evidence](../../../docs/quality/evidence/browser-execution-2026-11-20.md).
