# Tasks

## 1. Owned resource foundations

- [x] 1.1 Add bounded Agent Skills parsing, canonical digests and immutable owned import/load/list storage; verify paths, metadata, corruption, revision and ownership with `go test ./internal/agentcontext ./internal/agents`; document non-executable resource limits.
- [x] 1.2 Add versioned live consent, scoped memory entries, CAS updates, stable write identities and deletion; test concurrent managers, revoke/re-enable, replay, scope isolation and storage failures with the same Go packages and race checks; document retention semantics.
- [x] 1.3 Pin skill references and memory consent at profile/job admission and expose bounded tools through the existing runner, including inspectable skill drafts; test untrusted content, revoked consent, children and aggregate-budget preservation with `go test ./internal/agents ./internal/server`.

## 2. Authenticated clients

- [x] 2.1 Add owned resource/consent APIs and generated contracts; verify auth, strict inputs, cross-owner denial, old profiles/tasks and API checks with `go test ./internal/server ./internal/serviceclient` and `npm --prefix apps/web run api:check`; update docs/reference/api.md.
- [x] 2.2 Add explicit offline CLI imports, skill review/attachment and memory inspect/edit/delete/consent controls through the authenticated service; test disposable directories, hostile paths, errors and no implicit script execution with `go test ./internal/cli ./cmd/offgrid`; update docs/guides/agents.md.
- [x] 2.3 Add progressive shared profile resource controls and memory default-off/inspection/deletion in nine locales; verify TypeScript, build and focused renderer tests without changing desktop IPC or current design.

## 3. Integrated evidence

- [x] 3.1 Run Go/race, API/type/build, focused renderer and desktop compatibility checks; record actual commands, failures and unverified model usefulness in docs/quality/evidence/native-agent-execution-2026-10-25.md, validate this change strictly and reconcile parent task 5.1 only after service/CLI/UI scenarios pass.

- [ ] 3.2 Fix the reported repeated-load regression: retain the newest successfully loaded pinned skill file during compaction, preserve permission boundaries and original results, and verify one load followed by the requested checklist with the exact imported fixture on the installed Meta-Llama model. Cover restart, duplicates and mismatched references with Go/race tests; record live failures without claiming import success proves skill usefulness.
