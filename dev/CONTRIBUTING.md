# Contributing

OffGrid is one product with several distribution surfaces. Keep behavior in
shared Go services and the React application rather than implementing another
copy for a particular installer or host.

Read [AGENTS.md](../AGENTS.md) and the [OpenSpec workflow](../openspec/README.md)
before substantive features or refactors. Documentation corrections can use a
focused patch and checks without a full proposal. Release qualification remains
in the existing reliability documentation, not generated specifications.

## Repository map

- `cmd/offgrid`: CLI entry point and command wiring.
- `internal/server`: HTTP composition, middleware, and transport handlers.
- `internal/inference`: lifecycle for the local `llama-server` child process.
- `internal/models`: catalog, registry, download, verification, and model files.
- `internal/sessions`: durable conversation state.
- `internal/rag`, `internal/agents`, `internal/mcp`, `internal/audio`,
  `internal/p2p`: optional capability modules.
- `pkg/api/openapi.yaml`: stable HTTP contract.
- `web/app`: the only browser and desktop UI source.
- `desktop`: Electron lifecycle, preload boundary, and package metadata.
- `internal/computer`, `cmd/offgrid-computer`, `computer`: typed native control,
  host supervision, managed browser, and dispatch journal.
- `docker`: container and Compose deployment.

## Development setup

Use the Go toolchain declared in `go.mod` and Node.js 22:

```bash
go mod download
go test ./...

cd web/app
npm ci
npm run api:check
npm run check
npm run build
cd ../..
```

Use the [build guide](../docs/advanced/BUILDING.md) for versioned Bash/PowerShell
build commands and matching desktop dependencies. Keep test state separate using
`OFFGRID_DATA_DIR`, `OFFGRID_MODELS_DIR`, and an unused port. Do not run an
unversioned development service against your installed desktop workspace.

## Design rules

- Give mutable state one owner and store it below `OFFGRID_DATA_DIR` or
  `OFFGRID_MODELS_DIR`.
- Pass cancellation to child processes, downloads, and background work.
- Keep handlers focused on transport; put shared behavior in services.
- Make optional dependencies visible as unavailable capabilities.
- Do not expose raw system or child-process errors to API consumers.
- Add stable operations to OpenAPI before using them from the UI.
- Do not add static UI files under `desktop` or `web/ui`; both editions consume
  the generated `web/dist` bundle.
- Label incomplete platform-dependent behavior as experimental.

## API changes

```bash
cd web/app
npm run api:generate
npm run api:check
```

Commit the contract and generated schema together with the Go implementation.
Add a Go test for handler/service semantics and a Playwright test for a critical
visible workflow.

## Before review

Run from the repository root after installing dependencies:

```bash
go test ./...
node --test dev/scripts/check-docs.test.mjs
node dev/scripts/check-docs.mjs

npm run api:check --prefix web/app
npm run check --prefix web/app
npm run build --prefix web/app

node --check desktop/main.js
node --check desktop/preload.js
npm test --prefix desktop
git diff --check
```

After building the service, use `node dev/scripts/test-web-workspace.mjs bin/offgrid`
(`bin/offgrid.exe` on Windows) for isolated real-service browser tests. Install
Playwright's Chromium as described in the build guide first. Do not run the full
E2E suite against the normal workspace; it creates and deletes fixture data.

Run computer/native/installed-package checks appropriate to the affected layer
from [CI](../.github/workflows/ci.yml). Native side effects require an isolated
desktop and explicit test scope. A documentation link check is not runtime,
security, model, or platform qualification. Report unrun checks honestly.

Documentation conventions and checker limits are in the
[writing guide](../docs/templates/README.md). No check authorizes committing,
publishing, training, or replacing an installed application without a request.

See [Architecture](../docs/advanced/ARCHITECTURE.md) and
[Maintainability and generation](../docs/advanced/maintainability.md) for the
system boundaries and generation policy.
