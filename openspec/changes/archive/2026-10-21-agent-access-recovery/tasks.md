# Tasks

## 1. Access proposal validation

- [x] 1.1 Return actionable invalid-access errors, reject trailing JSON, clarify installed-app routing, and add empty-URL/invalid-input regressions; verify with `go test ./internal/server -run TestAccessProposal -count=1` and update the computer-task guide.

## 2. Durable bounded repair

- [x] 2.1 Implement and persist bounded pre-dispatch repair, pause on exhaustion, preserve hard denials/uncertainty, and add restart, ownership, and explicit-recovery regressions; verify with `go test ./internal/agents -run 'TestProposalRepair|TestComputerTaskRepairs' -count=1` and update the agent recovery guide.
- [x] 2.2 Exercise the exact malformed Chrome request through the real server adapter and durable runner using an isolated controlled caller; verify `go test ./internal/server -run TestAccessProposalRunner -count=1` reaches local access in the original run without dispatch.

## 3. Integration verification

- [x] 3.1 Run `go test ./...`, relevant Go race checks including `./internal/computer`, `npm.cmd --prefix web/app run api:check`, `node dev/scripts/check-docs.mjs`, and `openspec validate agent-access-recovery --strict --no-interactive`; record actual evidence and unrun platform/model checks in the existing reliability plan.
