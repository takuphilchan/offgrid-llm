# Architecture

OffGrid is a local-first application, not a collection of independently
deployed microservices. One Go process owns the HTTP API, model lifecycle,
durable state, and optional subsystems. The browser, Electron application, CLI,
and external agents are clients of that same runtime.

## Product boundaries

```text
Browser UI ────────┐
Electron UI ───────┼── HTTP / WebSocket ── OffGrid runtime ── llama-server
CLI ───────────────┤                           │
External clients ──┘                           ├── model files
                                               └── application data
```

- `cmd/offgrid` is the executable and CLI entry point.
- `internal/server` composes services and owns HTTP transport concerns.
- `internal/inference` manages one loopback-only `llama-server` process and
  normalizes its responses.
- `internal/models` owns the catalog, registry, resumable downloads, integrity
  checks, and model-file operations.
- `internal/sessions` owns durable conversation data.
- `internal/rag`, `internal/agents`, `internal/mcp`, `internal/audio`, and
  `internal/p2p` are capability modules. They must remain optional and report
  unavailable states instead of preventing the core runtime from starting.
- `pkg/api/openapi.yaml` is the checked-in contract for stable product APIs.
- `web/app` is the React source; `web/dist` is its generated static bundle.
- `desktop` is a thin Electron host for the same served React application.

## Request and inference lifecycle

For a non-streaming chat request:

1. HTTP middleware applies request IDs, authentication, authorization, rate
   limits, and observability.
2. A transport handler validates the request and calls the shared chat service.
3. The service verifies the requested model and acquires the inference gate.
4. The runtime starts or switches the loopback `llama-server` process and waits
   for readiness on `127.0.0.1`.
5. Optional RAG context is added only when requested and available.
6. The response is normalized to the public API type and metrics are recorded.
7. A session request atomically appends the user and assistant messages only
   after generation succeeds.

Failed or cancelled generation never writes half of a conversation turn.
Internal runtime errors are logged server-side; API clients receive a stable,
user-facing error message.

## State ownership

OffGrid uses two authoritative roots:

```text
OFFGRID_MODELS_DIR/
  *.gguf
  model projectors

OFFGRID_DATA_DIR/
  sessions/
  rag/
  users and quotas
  runs and artifacts
  audio/
  audit/
  tools and other service state
```

Native defaults are under `~/.offgrid-llm`; container defaults are mounted at
`/var/lib/offgrid`. Startup migrates recognized legacy files by copying them
without overwriting newer data. The legacy source is retained for recovery.

State writes that represent a complete object use temporary files plus atomic
rename. Components must not invent another top-level persistence location.

## Browser and desktop UI

The browser and Electron editions intentionally use one React application.
Electron starts the bundled OffGrid runtime when required, waits for its health
endpoint, and loads `http://127.0.0.1:<port>/ui/`. If a separately managed local
server already exists, Electron connects to it without starting a second one.

This keeps navigation, authentication, model actions, translations, responsive
layouts, and session persistence identical. Desktop-only capabilities are
exposed through a small context-isolated preload bridge; Node integration is
disabled and untrusted navigation is opened outside the application window.

## API contract and generation

The stable contract lives in `pkg/api/openapi.yaml` and is served at
`GET /openapi.yaml`. React types are generated into
`web/app/src/api/schema.generated.ts`.

```bash
cd web/app
npm run api:generate
npm run api:check
```

`api:check` is run in CI. Change the OpenAPI document and implementation in the
same review, regenerate the client types, and add a handler or browser test for
new behavior. Experimental endpoints may exist before they enter this stable
contract, but documentation must label them as experimental.

## Process ownership and shutdown

The server owns a root context. Background workers, streaming handlers,
downloads, RAG, metrics, P2P, and agent runs derive from it. Shutdown cancels
that context, stops owned child processes, and waits for component cleanup.

Agent sandboxes are lazy: starting OffGrid does not create idle Python
containers. A sandbox is created only when a governed tool call actually needs
one, and the owner is responsible for stopping it.

## Framework policy

The runtime remains framework-light. A wholesale migration to a microservice
framework would add service discovery, RPC, and generated-handler conventions
that do not match a single-device, long-lived inference process. Maintainability
comes from explicit package boundaries, a transport-neutral service layer,
OpenAPI generation, deterministic build scripts, and integration tests.

If a future hosted control plane is split from the local runtime, a framework
such as Go-Zero can be evaluated for that independently deployed boundary. It
should not dictate the local engine architecture.

## Capability maturity

- **Core:** inference, CLI, OpenAI-compatible APIs, sessions, web/desktop
  UI, model download/resume/verify, and authentication for network use.
- **Optional:** RAG, MCP, governed agents, and audio when their local
  dependencies are configured.
- **Experimental:** computer use, P2P sharing, LoRA workflows, and advanced
  orchestration. These require additional platform and security validation.

Readiness is a product contract: the UI and API must disable or explain an
unavailable capability instead of presenting a control that silently fails.
