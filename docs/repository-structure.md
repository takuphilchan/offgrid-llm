# Repository structure

OffGrid is one product with a shared service and several clients. Use this map
to locate behavior before adding a second implementation.

| Path | Responsibility |
| --- | --- |
| `cmd/offgrid/` | CLI commands, argument parsing, service clients, local maintenance |
| `cmd/offgrid-computer/` | Host-native computer worker entry point |
| `internal/server/` | HTTP routes, authentication, transport, service composition |
| `internal/agents/` | Durable tasks, approvals, context, delegation, SQLite task snapshots/events |
| `internal/computer/` | Typed control protocol, native OS adapters, target/action validation |
| `computer/` | Host browser/native session supervision, Playwright, local dispatch journal |
| `internal/inference/` | Managed native llama-server lifecycle and inference transport |
| `internal/models/` | Model catalog, downloads, registry, verification |
| `internal/sessions/`, `internal/rag/`, `internal/storage/` | Conversation, knowledge, and storage primitives |
| `internal/users/`, `internal/serviceclient/` | Identity/permissions and authenticated client transport |
| `pkg/api/openapi.yaml` | Versioned HTTP contract; generated clients must match |
| `web/app/` | Shared React/TypeScript renderer for browser and Electron |
| `web/dist/` | Generated UI output; never hand-edit |
| `desktop/` | Electron main/preload trust boundary, startup, consent, packaging |
| `python/` | Python SDK and examples |
| `docker/`, `installers/` | Container definitions and platform installation |
| `scripts/`, `dev/scripts/` | Installation/build helpers and isolated test harnesses |
| `examples/`, `dev/examples/` | User examples and developer-only experiments |
| `docs/` | Setup, guides, references, architecture, historical release notes |
| `openspec/` | Scoped change intent/acceptance; not a substitute for tests or release evidence |

## Follow a change through its owners

A task crosses the UI or CLI, the HTTP job contract, the durable agent runner,
and its tool authorization. Computer actions additionally cross a host-local
consent/dispatch boundary before reaching the native or browser driver.
Changing a dropdown cannot change those authorization rules by itself.

Start with [architecture](advanced/ARCHITECTURE.md),
[client contracts](advanced/client-contracts.md), and the
[Computer Tasks implementation notes](../computer/README.md). The
[reliability plan](advanced/product-reliability-plan.md) owns evidence;
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

See [Contributing](../dev/CONTRIBUTING.md) for checks and
[OpenSpec](../openspec/README.md) for substantive development changes.
