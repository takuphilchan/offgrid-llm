# Tasks

## 1. Composed provider boundary

- [x] 1.1 Add context-aware policy-filtered registry discovery and task-first computer composition; preserve legacy restrictions, exclude local execution, recheck owner/policy on dispatch, and keep host/service paths separate. Add unit regressions and update the computer-task guide; run `go test ./internal/agents ./internal/server -run 'TestRegistryDiscovery|TestComposed' -count=1`.

## 2. Durable mixed-tool workflow

- [x] 2.1 Make observation ordering and approval records provider-specific, preserve conservative verification, and prove one durable MCP-plus-computer workflow with an isolated HTTP MCP server and controlled companion. Update the agent guide; run `go test ./internal/agents ./internal/server -run 'TestComposed|TestComputerTask|TestBrowserTools|TestNativeComputer' -count=1`.

## 3. Integration verification

- [x] 3.1 Run `go test ./...`, race checks for agents/server/computer/capabilities, `npm.cmd --prefix web/app run api:check`, `node dev/scripts/check-docs.mjs`, and `openspec validate agent-capability-composition --strict --no-interactive`; record checks and unrun real-app/platform workflows in the existing reliability plan.
