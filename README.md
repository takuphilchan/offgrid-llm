# OffGrid LLM

OffGrid is a local-first AI runtime with a CLI, HTTP API, responsive web UI,
and Electron desktop host. It runs GGUF models through `llama-server` and keeps
models, conversations, knowledge indexes, users, and run history on storage you
control.

The project is being hardened from a prototype into a dependable local tool.
Core paths are tested; optional subsystems are exposed according to their
actual readiness rather than being presented as universally available.

Development is tracked against the [production-readiness gates](docs/advanced/production-readiness.md).
See [client contracts](docs/advanced/client-contracts.md) for the new CLI error
behavior, desktop compatibility checks, and their current coverage limits.

## Run with Docker

Use the versioned stable image for a repeatable installation:

```bash
docker pull takuphilchan/offgrid-llm:0.4.5

docker run -d \
  --name offgrid \
  --init \
  --restart unless-stopped \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:0.4.5
```

Open <http://127.0.0.1:11611/ui/>. The named volumes survive container
replacement. Keep the `127.0.0.1` binding unless authentication and a trusted
TLS reverse proxy are configured.

Useful container commands:

```bash
docker exec -it offgrid offgrid version
docker exec -it offgrid offgrid list
docker exec -it offgrid offgrid download phi-3.5-mini-instruct
docker logs -f offgrid
```

Stable releases publish `latest`, full semantic versions, `sha-*` tags,
provenance, SBOMs, and AMD64/ARM64 CPU manifests. NVIDIA images use the separate
`0.4.5-gpu` tag (Linux AMD64). The `edge` tag is for development, not a stable
upgrade channel. See [Docker deployment](docs/setup/docker.md) and the
[v0.4.5 release notes](docs/releases/release-notes-v0.4.5.md).

## Build and run from source

Saved chat streams by default in the web and desktop UI. Response settings
separate smaller interactive context from extended context without changing
external-agent allocations. See [inference performance](docs/advanced/PERFORMANCE.md)
for GPU setup, memory tradeoffs, and meaningful latency measurements.

Requirements: Go 1.25+ with the toolchain declared in `go.mod`, Node.js 22,
and npm. OffGrid uses a compatible `llama-server` on `PATH` when present;
otherwise the native CPU/Metal build downloads a checksum-pinned llama.cpp
runtime on first inference. For a custom or GPU build, set
`OFFGRID_LLAMA_SERVER_PATH` to its executable. `OFFGRID_BIN_DIR` selects the
native fallback install directory. The Docker image already bundles its
inference runtime.

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

cd web/app
npm ci
npm run api:check
npm run build
cd ../..

go build -trimpath -o bin/offgrid ./cmd/offgrid
./bin/offgrid serve
```

Open <http://127.0.0.1:11611/ui/>. On Windows, use `bin\offgrid.exe`; in WSL,
use the Linux command above from `/mnt/d/offgrid-llm`.

The CLI is also the container entry point:

```bash
./bin/offgrid --help
./bin/offgrid search phi
./bin/offgrid download phi-3.5-mini-instruct
./bin/offgrid list
./bin/offgrid run phi-3.5-mini-instruct.Q4_K_M
```

## Desktop app

Electron hosts the same React application and local API used by the browser,
so navigation, persistence, translations, model management, and authentication
do not drift between editions. Release automation builds Windows, macOS, and
Linux packages with the matching OffGrid runtime and UI.

For desktop development, start an OffGrid server first (a local binary or the
Docker container), then run:

```bash
cd desktop
npm ci
npm run dev
```

If no server is already listening, the packaged desktop app starts its bundled
runtime. Desktop data defaults to `~/.offgrid-llm`.
If a different service is already running, the startup window explains the
mismatch and offers safe recovery choices. See [desktop setup and
recovery](docs/setup/desktop-startup.md), including Windows SmartScreen and macOS
signing requirements. Source changes do not update an already-installed app.

## Interfaces

- OpenAI-compatible: `POST /v1/chat/completions`, `POST /v1/embeddings`
- Durable conversations: `/v1/sessions`
- Model catalog and lifecycle: `/v1/catalog`, `/v1/models/*`
- MCP and governed agent endpoints for external agent integration
- Native `offgrid` provider plugins for Hermes Agent and OpenClaw
- Versioned OpenAPI contract: `GET /openapi.yaml`

The source contract is [openapi.yaml](pkg/api/openapi.yaml). UI types are
generated from it; CI fails when generated types drift.

External agents connect to OffGrid as a native `offgrid` provider. Hermes and
OpenClaw have guided managed setup:

```bash
offgrid hermes install
offgrid hermes
offgrid openclaw install
offgrid openclaw test
offgrid openclaw run "Summarize this directory"
```

Run these on the machine where the agent should live. If you built from source
and did not add the CLI to `PATH`, replace `offgrid` with `./bin/offgrid`
(`.\bin\offgrid.exe` in PowerShell). When OffGrid runs in Docker, keep the
service container running but run the managed agent installer from the host;
`docker exec` cannot install an agent into your host environment.

The install command checks the OffGrid service and model, installs Hermes with
its official installer when needed, installs the provider, persists its
configuration without pulling optional npm/browser dependencies. Verify actual
inference with `offgrid hermes test`; full diagnostics remain available through
`offgrid hermes doctor`. Hermes requires at least 64,000 context tokens, so
the command stops with guidance when the running OffGrid service cannot supply
that window. OpenClaw uses its official installer when missing and a local
plugin that discovers chat models only. Lower-level provider commands remain
available through `offgrid integrations`. See the
[external agents guide](docs/guides/external-agents.md).

## Persistent data

Two roots are authoritative:

| Setting | Native default | Container default | Contents |
| --- | --- | --- | --- |
| `OFFGRID_MODELS_DIR` | `~/.offgrid-llm/models` | `/var/lib/offgrid/models` | GGUF and projector files |
| `OFFGRID_DATA_DIR` | `~/.offgrid-llm/data` | `/var/lib/offgrid/data` | sessions, users, RAG, runs, audio, audit, artifacts |

Legacy state is copied into the data root without overwriting newer files. The
legacy source is retained so migration is recoverable.

## Capability maturity

| Capability | Status | Notes |
| --- | --- | --- |
| Local inference and model switching | Core | `llama-server` is lifecycle-managed and loopback-only |
| CLI, OpenAI-compatible API, web UI | Core | Shared runtime and contract |
| Durable chat sessions | Core | Complete turns are persisted atomically |
| Model catalog/download/resume/verify | Core | Cancellation keeps resumable partial files |
| Authentication and permissions | Core for network use | Disabled in loopback quick start |
| Knowledge/RAG | Optional | Requires a compatible local embedding model |
| Agents and MCP tools | Optional, governed | Tool calls are policy checked; risky calls require approval |
| Audio, computer use, LoRA, P2P | Experimental | Platform and deployment support varies; do not assume availability |

## Development checks

```bash
go test ./...

cd web/app
npm run api:check
npm run check
npm run build
# Starts an isolated UI server automatically.
npm run test:e2e

cd ../../desktop
node --check main.js
node --check preload.js
```

The CI workflow runs Go tests, contract generation checks, the UI build, live
browser integration tests, and native packaged-Electron startup/recovery checks. Set
`OFFGRID_E2E_URL` only when the browser suite should target an already-running
OffGrid service; local test runs manage their own UI server.

## Documentation

- [Architecture](docs/advanced/ARCHITECTURE.md)
- [CLI experience](docs/advanced/cli-experience.md)
- [Workspace UI](docs/advanced/workspace-ui.md)
- [Managing chat and agent history](docs/guides/history-management.md)
- [Workspace ownership, backup, and recovery](docs/advanced/workspace-recovery.md)
- [Maintainability and generation](docs/advanced/maintainability.md)
- [Product reliability plan and acceptance gates](docs/advanced/product-reliability-plan.md)
- [API reference](docs/reference/api.md)
- [CLI reference](docs/reference/cli.md)
- [Docker deployment](docs/setup/docker.md)
- [Model management](docs/guides/models.md)
- [Knowledge and embeddings](docs/guides/embeddings.md)
- [Agents](docs/guides/agents.md)
- [Repository structure](docs/repository-structure.md)

## Security

The unauthenticated quick start is for loopback use only. Remote deployments
must enable authentication, restrict permissions, terminate TLS at a trusted
proxy, and review agent/computer-use policies. Do not expose model management,
terminal, or tool execution endpoints directly to an untrusted network.

With authentication enabled, conversations are scoped to their owner. Existing
unowned/local conversations remain accessible to administrators with `sessions:all`,
not ordinary signed-in users; no data is automatically reassigned or deleted.
Conversation names currently remain unique within an installation; creating an
existing name returns a conflict instead of replacing its contents.

Knowledge requests require the `rag` permission as well as `chat`, including when
made through chat or saved conversations. If knowledge storage or retrieval is
unavailable, the request fails explicitly instead of silently generating without
sources. No matching evidence also produces an explicit error; users can turn off
knowledge for a general answer. Failed storage initialization disables ingestion
without disabling ordinary chat.

Agent approvals now continue a saved run rather than resubmit its prompt. Each
approval authorizes one exact invocation, expires after 15 minutes, and is bound
to the initiating account. Restarted work requires explicit resume; a tool with
an unknown outcome must be inspected and reconciled, never silently replayed.
The web UI and `offgrid agent status/approve/deny/cancel/resume/reconcile` use the
same durable state. See the [agent recovery guide](docs/guides/agents.md).

Chat and agent drafts are saved locally as you type, separately for each account.
They survive navigation and failed sends; browser storage failures show a warning.
Drafts are not encrypted or synchronized across devices. See the reliability plan
for remaining collection isolation, replayable chat progress, and release gates.

## License

MIT. See [LICENSE](LICENSE). OffGrid uses [llama.cpp](https://github.com/ggml-org/llama.cpp)
for local inference.
