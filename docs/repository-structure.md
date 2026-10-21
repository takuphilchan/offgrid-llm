# Repository structure

OffGrid is one product with a shared service and several clients. Use this map
to locate behavior before adding a second implementation.

| Path | Responsibility |
| --- | --- |
| `cmd/offgrid/` | OS entry, legacy command dispatch and host capability adapters; default and explicit `serve` share one application startup adapter |
| `internal/app/` | Production runtime composition, workspace ownership, reverse-order cleanup and one transport lifetime |
| `internal/cli/` | Migrated command parsing, presentation, typed outcomes and service-backed command logic |
| `cmd/offgrid-computer/` | Host-native computer worker entry point |
| `internal/server/` | Borrowed HTTP transport; `router.go` registration, `middleware.go` and `exposure.go` policy, capability-named handlers, and explicit disposable test fixtures |
| `internal/agents/` | Durable tasks, approvals, context, delegation, SQLite task snapshots/events |
| `internal/computer/` | Typed control protocol, native OS adapters, target/action validation |
| `apps/computer/` | Host browser/native session supervision, Playwright, local dispatch journal |
| `internal/inference/` | Managed native llama-server lifecycle and inference transport |
| `internal/models/` | Model catalog, downloads, registry, verification |
| `internal/sessions/`, `internal/rag/`, `internal/storage/` | Conversation, knowledge, and storage primitives |
| `internal/users/`, `internal/serviceclient/` | Identity/permissions and authenticated client transport |
| `pkg/api/openapi.yaml` | Versioned HTTP contract; generated clients must match |
| `apps/web/` | Shared React/TypeScript renderer for browser and Electron |
| `apps/web/dist/` | Generated UI output; never hand-edit |
| `apps/desktop/` | Electron main/preload trust boundary, startup, consent, packaging |
| `sdk/python/` | Independently versioned public Python SDK and client examples |
| `runtimes/python/` | Trusted optional Python service workers and protocol fixtures; not part of the SDK wheel |
| `docker/`, `installers/` | Container definitions and platform installation |
| `scripts/{build,test,release,maintenance}/` | Owner-specific build helpers, isolated test harnesses, release checks, and maintenance tools |
| `examples/`, `examples/clients/` | User-facing and developer client examples |
| `tools/model-package-metadata/`, `docker/test/` | Metadata tooling and isolated native test image |
| `docs/` | Setup, guides, references, architecture, historical release notes |
| `openspec/` | Scoped change intent/acceptance; not a substitute for tests or release evidence |

`internal/app/` constructs the production service runtime. In-package HTTP
fixtures use explicit disposable dependencies; no server-owned graph constructor
remains. `server.go` retains one listener and drains transport before app releases its
owned components. Preserve the
ordered route/wrapper fixture when editing `router.go`.

## Follow a change through its owners

A task crosses the UI or CLI, the HTTP job contract, the durable agent runner,
and its tool authorization. Computer actions additionally cross a host-local
consent/dispatch boundary before reaching the native or browser driver.
Changing a dropdown cannot change those authorization rules by itself.

Start with [architecture](architecture/overview.md),
[client contracts](advanced/client-contracts.md), and the
[Computer Tasks implementation notes](../apps/computer/README.md). The
[quality index](quality/README.md) owns evidence navigation;
do not create another competing readiness checklist.

## Keep development predictable

- Preserve OS-specific build tags and small platform adapters.
- Put shared domain behavior in services, not another UI/CLI-specific runner.
- Keep generated binaries, models, secrets, caches, logs, and private test data
  out of Git.
- Regenerate API types from the contract rather than editing generated output.
- Use isolated test state, especially for browser, desktop, and migration tests.
- Preserve historical release notes; correct current instructions in maintained guides.
- Keep sibling projects such as fine-tuning labs and Edge Delegate outside this
  repository's product documentation unless an actual integration is documented.

See [Contributing](../CONTRIBUTING.md) for checks and
[OpenSpec](../openspec/README.md) for substantive development changes.
