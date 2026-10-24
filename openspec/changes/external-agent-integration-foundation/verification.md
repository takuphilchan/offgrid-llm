# Initial foundation verification: external-agent-integration-foundation

This records the original nine-task review. Later approved extensions are tracked
in [tasks.md](tasks.md), and their actual model/platform results are recorded in
[evidence.md](evidence.md). This initial review does not qualify those extensions.

## Summary

| Dimension | Result |
| --- | --- |
| Completeness | 9/9 implementation/check tasks; all 6 added requirements mapped |
| Correctness | All 12 specified scenarios have implementation and regression coverage; real-model tool qualification and native installed-runtime coverage remain unverified |
| Coherence | Shared lifecycle, authenticated client, registry-driven UI and bounded adapters follow the design; no separate inference executor added |

## Requirement mapping

- Shared lifecycle: `internal/integrations/host.go`, `configure.go`, shared
  `internal/cli/integration_command.go`; third-adapter manager, CLI and renderer
  fixtures demonstrate extension without new lifecycle branches.
- Selection/context: `contracts.go`, scoped `state.go`, `host.go`,
  `internal/power/power_test.go`, `internal/degradation/manager_test.go` and
  service context/selection tests cover growth, shrink, battery and heap-vs-OS
  pressure. Installed models are discovered; no model IDs are built into policy.
- Truthful readiness: service projections and `ExternalProvidersPanel` label
  service compatibility separately; host reports distinguish installation,
  configuration, smoke evidence and unverified tool work. Operational-failure
  fixtures do not turn service failure into an installer request.
- Configuration preservation: `state.go`, `environment.go`, adapter command
  fixtures and real installed-runtime fixture runs cover snapshots, private
  secrets, multiline dotenv values, untouched unrelated models/defaults,
  repeated setup, interrupted stages and changed-config rejection.
- Authentication/bounds: `client_test.go`, command-argument fixtures and
  cancellation tests cover authenticated discovery, redirect rejection,
  provider/model override rejection, explicit installation and cancellation of
  owned subprocesses. External agents retain their own tool authority.
- Evidence: `host_test.go` covers changed model, runtime, plugin, config and
  contract invalidation, including legacy hosts without managed configuration.
  `test-external-agents.py` independently checks results; the real 3B failure
  is recorded instead of inferred successful from process exit.

## Findings

No critical source findings remain in the checks that ran. Final review found
and corrected multiline environment preservation and stale-evidence edge cases;
targeted normal/race regression checks were rerun.

Two qualification warnings remain:

1. **Real-model tool workflows are not qualified.** The tested local 3B model
   passed transport smoke but did not create the file. Repeat the disposable
   workflow with a suitable installed model and sufficient measured context
   before claiming an external-agent demo is ready. See the
   [actual evidence](../../../docs/quality/evidence/external-integrations-2026-10-22.md).
2. **Native platform/fresh installer qualification is not verified.** The
   installed-runtime evidence is WSL with existing Hermes/OpenClaw and a
   deterministic fixture. Run the documented isolated checks on native
   Windows/macOS and fresh installations before advertising those combinations.

No archive, deployment, publication or production-readiness approval is implied.
This verification is a source/evidence review, not an independent human review.
