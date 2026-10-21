# Contributing

OffGrid is one product with several distribution surfaces. Keep behavior in
shared Go services and the React application rather than implementing another
copy for a particular installer or host.

Read [AGENTS.md](AGENTS.md) and the [OpenSpec workflow](openspec/README.md)
before substantive features or refactors. Documentation corrections can use a
focused patch and checks without a full proposal. Release qualification remains
in the existing reliability documentation, not generated specifications.

Join the [OffGrid LLM Discord community](https://discord.gg/fc7EqyqHf) for
contributor discussion and project questions. Use
[GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues) for bugs and
reproducible technical problems so they remain trackable.

## Repository map

- `cmd/offgrid`: OS-backed CLI entry and compatibility adapters. `main.go`
  owns interrupt setup and the process exit; `root.go` still routes legacy
  commands; `server_command.go` retains default server startup. Do not add
  another command executor here.
- `internal/cli`: command parsing, typed outcomes, presentation, and migrated
  command families. Host-only process, terminal and local-file capabilities
  enter through explicit adapters in `cmd/offgrid`.
- `internal/app`: the production startup and cleanup owner; default and `serve`
  CLI paths both construct one `app.Runtime`.
- `internal/server`: HTTP lifecycle in `server.go`, ordered registration and
  outer wrappers in `router.go`, enforcement in `middleware.go` and
  `exposure.go`, capability-named handlers, and explicit disposable test
  dependencies. Review permission/rate
  wrappers and route order with `TestServerRouteRegistrationContract` whenever
  routes change. `HTTPConfig`/`Dependencies` borrows typed services from app;
  there is one production graph constructor in `internal/app`. Do not add
  routes that bypass `router.go` or treat a dependency
  pointer as authorization; retain the route/permission fixture and local IPC
  checks until the migration is fully verified.
- `internal/inference`: lifecycle for the local `llama-server` child process.
- `internal/models`: catalog, registry, download, verification, and model files.
- `internal/sessions`: durable conversation state.
- `internal/rag`, `internal/agents`, `internal/mcp`, `internal/audio`,
  `internal/p2p`: optional capability modules.
- `pkg/api/openapi.yaml`: stable HTTP contract.
- `apps/web`: the only browser and desktop UI source.
- `apps/desktop`: Electron lifecycle, preload boundary, and package metadata.
- `internal/computer`, `cmd/offgrid-computer`, `apps/computer`: typed native control,
  host supervision, managed browser, and dispatch journal.
- `docker`: container and Compose deployment.

## Development setup

Use the Go toolchain declared in `go.mod` and Node.js 22:

```bash
go mod download
go test ./...

cd apps/web
npm ci
npm run api:check
npm run check
npm run build
cd ../..
```

Use the [build guide](docs/advanced/BUILDING.md) for versioned Bash/PowerShell
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
- Do not add static UI files under `apps/desktop` or `web/ui`; both editions consume
  the generated `apps/web/dist` bundle.
- Label incomplete platform-dependent behavior as experimental.

## API changes

```bash
cd apps/web
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
node --test scripts/maintenance/check-docs.test.mjs
node scripts/maintenance/check-docs.mjs

npm run api:check --prefix apps/web
npm run check --prefix apps/web
npm run build --prefix apps/web

node --check apps/desktop/main.js
node --check apps/desktop/preload.js
npm test --prefix apps/desktop
git diff --check
```

After building the service, use `node scripts/test/test-web-workspace.mjs bin/offgrid`
(`bin/offgrid.exe` on Windows) for isolated real-service browser tests. Install
Playwright's Chromium as described in the build guide first. Do not run the full
E2E suite against the normal workspace; it creates and deletes fixture data.

Run apps/computer/native/installed-package checks appropriate to the affected layer
from [CI](.github/workflows/ci.yml). Native side effects require an isolated
desktop and explicit test scope. A documentation link check is not runtime,
security, model, or platform qualification. Report unrun checks honestly.

Documentation conventions and checker limits are in the
[writing guide](docs/templates/README.md). No check authorizes committing,
publishing, training, or replacing an installed application without a request.

Use the [documentation index](docs/README.md) for current user and maintainer
instructions, [main OpenSpec specs](openspec/specs/) for accepted requirements,
and [active changes](openspec/changes/) for proposed or unfinished work. The
[roadmap](docs/ROADMAP.md) is the only current product delivery order;
[quality evidence](docs/quality/README.md) distinguishes source implementation,
automatic tests and real-platform qualification. Archived changes and release
notes are historical, not current instructions or release approval.

See [Architecture](docs/architecture/overview.md) and
[Maintainability and generation](docs/advanced/maintainability.md) for the
system boundaries and generation policy.
