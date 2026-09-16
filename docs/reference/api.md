# API reference

OffGrid exposes a local HTTP API at `http://127.0.0.1:11611`. The stable core
contract is [pkg/api/openapi.yaml](../../pkg/api/openapi.yaml) and is also
available from a running instance:

```bash
curl http://127.0.0.1:11611/openapi.yaml
```

The OpenAPI document is the source of truth for request and response fields.
This page explains the main workflows and persistence semantics.

## Health and readiness

```bash
curl http://127.0.0.1:11611/health
curl http://127.0.0.1:11611/ready
```

`/health` confirms the process is alive. Readiness additionally reflects
whether required runtime components can accept work. Kubernetes-style
`/livez` and `/readyz` aliases are available.

## Authentication

Loopback quick start is unauthenticated. Set `OFFGRID_REQUIRE_AUTH=true` for a
network deployment. Browser login uses an HTTP-only session cookie; API clients
can use a bearer API key:

```bash
curl -H "Authorization: Bearer og_YOUR_API_KEY" \
  http://127.0.0.1:11611/v1/models
```

Authentication does not replace network controls. Bind to a trusted interface,
use TLS at a reverse proxy, and grant only the permissions a client needs.

## Models

List installed models and discover the curated catalog:

```bash
curl http://127.0.0.1:11611/v1/models
curl http://127.0.0.1:11611/v1/catalog
```

Model lifecycle endpoints include:

- `POST /v1/models/download`
- `GET /v1/models/download/progress`
- `POST /v1/models/download/cancel`
- `POST /v1/models/verify`
- `POST /v1/models/delete`

Downloads retain a partial file when cancelled and resume through an HTTP range
request. A successful download is rescanned into the registry before it is
reported as installed. Verification hashes the model contents and can be
cancelled with the request context.

## Stateless chat

`POST /v1/chat/completions` accepts the OpenAI chat-completions shape:

```bash
curl http://127.0.0.1:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "phi-3.5-mini-instruct.Q4_K_M",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

Set `stream` to `true` for server-sent events. Structured streaming preserves
tool-call deltas and optional usage information for agent runtimes.

## Durable conversations

The UI uses server-owned sessions rather than browser-only message state:

- `GET /v1/sessions` lists conversations.
- `POST /v1/sessions` creates one.
- `GET /v1/sessions/{name}` loads its complete history.
- `DELETE /v1/sessions/{name}` removes it.
- `POST /v1/sessions/{name}/generate` generates and atomically persists a full
  user/assistant exchange.

If generation fails or the caller cancels before persistence, no half-turn is
written. Cancellation during the final save may still commit a complete turn;
reload the session before retrying. This route is the preferred application
workflow when conversation continuity matters.

The web/desktop chat requests `stream: true` and renders text as it arrives:

```json
{"content":"Explain this briefly","model_id":"installed-model-id","stream":true,"profile":"interactive","max_tokens":1024}
```

The response is `text/event-stream`, with JSON `data:` events:

- `phase`: `queued`, `retrieving`, `loading`, `processing`, `generating`, `saving`.
  These are stages, not estimated percentage progress. Heartbeat comments keep
  the connection active while a model loads or a request waits for a slot.
- `delta`: text to append to the provisional assistant response.
- `done`: `session`, `message`, `finish_reason`, and timing `metrics`. This event
  is emitted **after** atomic persistence, not simply after model generation.
- `error`: generation or storage failed; no completed exchange is acknowledged.

HTTP authorization/validation errors happen before streaming. Retrieval,
inference and persistence failures after headers use an SSE `error` event.
Treat EOF without `done` as interrupted/unknown, keep the draft, and reload the
session before retrying. There is no automatic replay or reconnect/resume of a
partially generated chat turn. Stop aborts the request, including queue waits.

`interactive` allocates the smaller of `OFFGRID_CHAT_CONTEXT` (default 8192) and
the effective service context. `extended` uses the effective service context.
Profile changes are exclusive runtime switches and may reload the model. They
do not lower `/v1/chat/completions` context or the Hermes/OpenClaw configuration.
`max_tokens` accepts 1–4096, defaults to 1024, and bounds the response, not history.
The UI identifies `finish_reason: length` as a response-token limit.

Metrics include `total_ms` (through saving), `ready_ms` (including queue/load),
`queue_ms`, `load_ms`, `prompt_ms` (ready to first text, including transport),
`first_text_ms`, `generation_ms`, `context_window`, and, when the backend reports
usage, `completion_tokens` and an estimated `tokens_per_second`. Chunk counts
are not passed off as token counts. Streaming improves time to visible output;
it does not itself make model computation faster.

With authentication required, sessions are owned by their creator and all
operations check ownership. `owner_id` is server-assigned; clients cannot set
it. The `sessions:all` permission grants administrative access, including to
legacy conversations without an owner. Ordinary users cannot read or claim
those legacy conversations. Unauthenticated local mode retains access to them.
Enabling authentication does not delete or reassign existing data.

Names are currently unique across the installation. Creating a name that
already exists returns `409`, never overwrites history, and does not disclose
its owner or contents. Reading or modifying another user's conversation
returns `404`. Missing authentication returns `401`; missing session permission
returns `403`. Per-session HTTP mutations are serialized with generation.

## Knowledge

Knowledge endpoints live under `/v1/documents` and `/v1/rag`. RAG is optional:
clients must read `/v1/rag/status` and present an unavailable state when a local
embedding model has not been configured. Setting `use_knowledge_base` on a chat
request explicitly requires retrieval. It is never silently ignored:

- Authenticated callers need both `chat` and `rag` permission (`403` otherwise).
- Disabled knowledge or a retrieval failure returns `503` before generation.
- Retrieval with no matching evidence returns `422` before generation.
- The caller can configure knowledge, add relevant documents, or explicitly
  retry with `use_knowledge_base: false` for an ordinary model answer.

These rules apply to streaming, non-streaming, and saved-conversation chat.
Successful retrieval supplies untrusted source context to the model; it does
not guarantee that every generated statement is supported. Structured, persisted
citations and source-only evaluation remain planned work.

The knowledge index is currently shared by callers granted RAG access. Session
ownership does not make uploaded documents private to their uploader; separate
collection-level access controls are planned. Do not use one instance as an
isolated multi-tenant document service.

The supported setup workflow is:

- `GET /v1/catalog` to choose an entry with `type: embedding`.
- `POST /v1/models/download` with its stable `model_id`, repository, and file.
- `GET /v1/models/download/progress` until that model is complete.
- `POST /v1/rag/enable` with the installed embedding model ID.
- `POST /v1/documents/ingest` to retain and index a source.
- `POST /v1/documents/reindex` when a retained source needs rebuilding.

Activation metadata is persisted even before the first document is indexed.

If knowledge storage cannot initialize, the server keeps ordinary chat available
but rejects knowledge operations with `503`. It does not substitute a temporary
in-memory index or acknowledge imports it cannot persist. `/v1/rag/status` stays
available and reports `enabled: false`, `stats.storage_available: false`, and an
actionable error. Repair disk/access/database problems and restart; retain a
backup before any database recovery. Do not delete the database as a routine fix.

## Agents and MCP

MCP and agent routes are governed, administrator-level surfaces. Agent
sandboxes start lazily only when a tool call needs them. Treat terminal,
computer-use, filesystem, and external MCP tools as privileged operations and
require explicit policy or user approval. The stable UI integration routes are:

- `POST /v1/agents/run` and `GET /v1/agents/tasks`
- `GET /v1/agents/tasks/{id}` returns authoritative run state, not the internal checkpoint
- `POST /v1/agents/tasks/{id}/{approve|deny|cancel|resume|reconcile}` acts on that run
- `GET/PATCH /v1/agents/tools`
- `GET/POST /v1/agents/mcp` and `POST /v1/agents/mcp/test`
- `GET /v1/capabilities`
- `GET /v1/computer/status`

Task state, tool enablement, and successful MCP connection configuration are
stored in the OffGrid data directory. Computer use remains unavailable until a
supported native driver is configured; callers must honor the status endpoint.
These routes are included in the versioned OpenAPI document.

For long-lived work use `async: true` on run creation (HTTP `202`) and poll the
returned `run_id`. `stream: true` emits SSE `status`, committed `step`, `done`,
`approval_required`, and `error` events; it does not stream speculative model
tokens. Closing the stream does not cancel the task. Use the cancel action.

Approve/deny bodies contain the exact pending `approval_id`; never resubmit the
prompt. The server binds approval to actor, run, invocation, canonical arguments,
tool capability, and expiry. `canonical_arguments` is the exact JSON string to
display (avoids browser rounding of large numbers). No preapproved tool list is
accepted on run creation. Duplicate, expired, or stale actions return `409`.
`resume` refreshes an expired pending approval without executing it. Actions
cannot substitute prompts or tool arguments.

An interrupted model call can resume safely. A crash/cancellation/error during
a tool call produces `uncertain`, even if no side effect actually happened.
Inspect the external target, then POST `reconcile` with `call_id` and a human-
verified `result`. This records the outcome without rerunning the tool; explicitly
resume to continue remaining work. Legacy tasks without checkpoints remain
read-only history (`resumable: false`).

An unreadable/corrupt task snapshot or failed write makes task operations return
`503`; fix storage and restart. Successful mutations are atomic snapshots in
`OFFGRID_DATA_DIR/agent_tasks`, not volatile acknowledgments. Run summaries use
these checkpoints even if the auxiliary event log is unavailable, reporting
`event_history_available: false`. Back up the data directory before recovery.
Full backup restoration is still the operator's responsibility.

## Errors

Stable JSON endpoints return a non-2xx status with a concise public error
message. Internal process addresses and wrapped system errors are logged by the
server rather than exposed in the UI. Clients should branch on HTTP status and
must not parse human-readable error text.

## Changing the contract

After editing `pkg/api/openapi.yaml`:

```bash
cd web/app
npm run api:generate
npm run api:check
npm run check
```

Commit the contract, generated types, handler implementation, and tests
together. CI rejects generated-client drift.
