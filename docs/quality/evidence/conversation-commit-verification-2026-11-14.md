# Conversation preview commit verification

The conversation architecture and UI changes were separated from concurrent
document/OCR and packaging work using the Git index. Checks below used an
isolated export of that index, not the combined working tree. Shared API, schema,
route, client, translation and attachment-guard changes were staged selectively.
The TypeScript contract was regenerated from the separated OpenAPI contract.

Verified on Windows on 2026-11-14:

- TypeScript and production web build passed; existing bundle-size warnings remain.
- Conversation, worker and change inspection browser suites: 30 passed; one
  opt-in screenshot case skipped. The temporary Vite configuration permitted
  the existing dependency directory; this test-harness setting is not committed.
- Full Go package suites passed for agents, execution, CLI, serviceclient,
  inference, sessions and cmd/offgrid-executor with isolated temporary state.
- The server suite initially exposed omitted shared slices and an approval-test
  timing race. Capability reporting, attachment authority guards and route
  fixtures were included in the staged snapshot. All five affected tests passed
  afterward. The worker integration test was then changed to wait for the
  previous worker turn to become inactive and passed two repeated runs (74.847s).
- Translation checks, source-size ratchet, 362-file documentation link check,
  both affected OpenSpec validations and staged whitespace check passed.

The first parallel Go attempt exhausted Windows memory; serial package runs
with bounded Go memory completed. WSL failed to start, so Linux/race checks were
not rerun here. Python and desktop suites were not rerun for this commit.
Earlier evidence is retained separately and is not a claim that the entire CI
matrix passed against this exact staged tree. Native/model qualification and
remaining architecture tasks remain open. No installed application was changed.
