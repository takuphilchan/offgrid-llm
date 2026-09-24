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

## Agent task architecture

The interactive workflow is task-first: save an outcome, run the tool loop,
request missing access as a durable interruption, then continue the same job.
The model does not grant itself access and the renderer does not own execution.

| Responsibility | Implementation boundary |
| --- | --- |
| Admission, tool loop, checkpoints, exact approvals | `internal/agents/durable.go` |
| Saved input interruption and idempotent resolution | `internal/agents/input.go` |
| Pause, takeover, steering and recovery | `internal/agents/control.go` |
| Exact context archival and recorded plans | `context.go`, `runtime_tools.go` |
| Bounded durable child graph and scheduling | `coordination.go` |
| Snapshots, ordered activity, schema backup/migration | `sqlite_tasks.go`, `task_activity.go`, `input_migration.go` |
| HTTP task commands | `internal/server/agent_jobs.go`, `job_commands.go` |
| Private artifacts, parsing, read-back and owner downloads | `internal/server/task_artifacts.go` |
| Model request for an environment | `internal/server/agent_access.go` |
| Actual computer authorization and dispatch | `computer_tools.go`, `internal/computer`, local companion |
| Draft/history and commands | `TaskWorkspace.tsx` |
| Read-only snapshot/stream recovery | `useTaskRun.ts` |
| Contextual host consent and saved-task continuation | `TaskAccess.tsx` |

`POST /api/v2/jobs` stores an actor-scoped request ID and payload digest before
returning acceptance. Identical retries reuse the task; changed payloads return
409. Deletion removes content but retains minimal request identity in the
tombstone, preventing an old retry from resurrecting deleted work.
`request_computer_access` is a typed model tool that *interrupts before dispatch*.
Only `/api/v2/jobs/{id}/input`, after host consent, actor/driver checks and model
preflight, can attach a session and consume that pending call. Tools are selected
again after resolution. The first computer action must observe; no previous
planning step counts as a computer observation. Session permissions are immutable.

This separates three lifetimes: durable task history, model execution, and
ephemeral host authority. Schema 4 gates older binaries against the new task
states. Migration saves a verified prior-schema SQLite backup plus digest/count
manifest before activation; it does not dual-write. Task input waits survive a
restart, while connected host sessions and grants expire.

### Bounded durable coordination

The old `WorkflowEngine` and `Orchestrator` are not the production coordination
boundary: they retain in-memory workflow results and use `RunImmediate` rather
than durable, actor-scoped child jobs. The server no longer wires that orchestrator
as an executable API. Do not re-enable it or reuse its raw executor as a shortcut.

The root model can call `delegate_tasks` before acquiring computer access. It
creates 1–4 read-only child jobs per group, at most eight per task, one level deep.
Each child has ten model iterations, the parent's model/context allocation and
an explicit subset of enabled tools. Capability descriptors are pinned; a tool
replacement cannot widen the child grant. No child receives computer authority,
write tools, shell, further delegation, or its parent's exact-call approvals.

Parent, children, dependency edges and events are committed in one SQLite
transaction. Waiting parents release their runner slot. Ready children pass
through the same runner and inference admission as interactive tasks; there is
no second planner backend. Dependency results are bounded, marked as untrusted
excerpts and linked to full child jobs. Cycles and unknown dependencies fail
validation. The parent only synthesizes after all children complete; a failed
branch cannot silently become a successful parent. Pause/stop cascade, uncertain
calls never replay, and restart requires explicit parent resume. The UI links
subtasks with their individual approval/recovery controls.

This is bounded delegation, **not** arbitrary workflow registration, recursive
agent teams, autonomous child computer control or concurrent file writers.
Reusable workflow templates, multi-model scheduling and graph-wide adjustable
token/wall-time budgets are not implemented. The legacy workflow API stays retired.

`task_plan` stores model-reported progress with recorded step references. It is
not an outcome oracle. `context.go` archives whole completed call/result groups
without dropping user instructions. The model can retrieve exact archived
records through owner-task `task_history` pages. Byte estimates are explicitly
labelled estimates, anchored to runtime-reported prompt usage when available;
they are not a tokenizer or a guarantee that every template will fit. Oversize
uncompressible input fails before another tool action. Archives persist with
digest validation and are excluded from public snapshots/exports.

For computer tasks, a verified current application can hand off to a newly
consented target in the same job. The previous session is revoked, approvals are
not transferred, and fresh observation is mandatory. Workspace artifact writes
are bounded typed operations: the service rereads, hashes and parses text,
Markdown, JSON and CSV outputs. CSV formula-like values are rejected. A committed
task reference and current ownership are required for download; digest knowledge
alone grants nothing. These checks establish byte/format integrity, not factual
quality, a saved Office document, or semantic completion of every requested task.

Prefer single-agent execution. Qualify delegation benefits with identical task
evaluations rather than assuming extra agents improve quality.
Relevant primary references are [Anthropic's orchestrator/worker experience](https://www.anthropic.com/engineering/multi-agent-research-system),
including its limits on tightly coupled work, and [the separation of harness and execution environment](https://www.anthropic.com/engineering/managed-agents).
These inform the design; they do not qualify OffGrid models or justify a cloud
dependency. There is no universal September-2026 architecture that makes an
unreliable local model or unverified driver reliable merely by adding agents.

## Capability maturity

- **Core:** inference, CLI, OpenAI-compatible APIs, sessions, web/desktop
  UI, model download/resume/verify, and authentication for network use.
- **Optional:** RAG, MCP, governed agents, and audio when their local
  dependencies are configured.
- **Experimental:** computer use, P2P sharing, LoRA workflows, and advanced
  orchestration. These require additional platform and security validation.

Readiness is a product contract: the UI and API must disable or explain an
unavailable capability instead of presenting a control that silently fails.
