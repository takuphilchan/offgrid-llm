# Contributing

OffGrid is one product with several distribution surfaces. Keep behavior in
shared Go services and the React application rather than implementing another
copy for a particular installer or host.

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
```

Run the server with `go run ./cmd/offgrid serve`, then open
`http://127.0.0.1:11611/ui/`. For Electron development, keep the server running
and use `npm ci && npm run dev` in `desktop`.

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

```bash
go test ./...

cd web/app
npm run api:check
npm run check
npm run build
npm run test:e2e

cd ../../desktop
node --check main.js
node --check preload.js
```

See [Architecture](../docs/advanced/ARCHITECTURE.md) and
[Maintainability and generation](../docs/advanced/maintainability.md) for the
system boundaries and generation policy.
