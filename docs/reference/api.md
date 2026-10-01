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

## Durable jobs and computer access

First-party tasks use `/api/v2/jobs`, not a separate history or runner:

| Resource | Purpose |
| --- | --- |
| `GET/POST /api/v2/jobs` | List owner-scoped history or submit a saved task |
| `GET /api/v2/jobs/{id}` | Authoritative snapshot and event cursor |
| `GET /api/v2/jobs/{id}/events` | Ordered persisted activity and snapshot SSE |
| `POST /api/v2/jobs/{id}/input` | Attach independently consented computer access to a pending request |
| `POST /api/v2/jobs/{id}/{action}` | Approve, deny, pause, resume, cancel, take over, steer, reconnect, or reconcile as supported by the contract |
| `GET /api/v2/jobs/{id}/export` | Evidence export |
| `GET /api/v2/jobs/{id}/artifact` | Owner-authorized artifact download; parameters are defined in OpenAPI |
| `DELETE /api/v2/jobs/{id}` | Delete eligible history; not cancellation or undo |

Submit with an installed model ID and a caller-generated request ID:

```bash
curl http://127.0.0.1:11611/api/v2/jobs \
  -H "Content-Type: application/json" \
  -d '{"prompt":"Calculate 25 * 47","model":"YOUR_MODEL_ID","request_id":"docs-example-0001"}'
```

This Bash example assumes loopback unauthenticated mode; protected services also
need authentication. Use a new request ID for new work. Reuse it only when
retrying the identical submission. Actor-scoped identical retries return the
original task; conflicting reuse returns `409`. A `202` acknowledges persistence,
not completion. Follow the returned `run_id` and `Location`.

Computer setup is a durable interruption: `waiting_for_input` and
`pending_input` direct the user to local consent. Resolving that input attaches
an existing owner-scoped session; it does not grant OS permissions or invent a
target. The desktop bridge controls local selection/consent. See
[Computer Tasks](../guides/computer-tasks.md).

### Activity replay

Connect to `/api/v2/jobs/{id}/events` using `Last-Event-ID` to resume after a
persisted sequence. Keep IDs as decimal strings, not floating-point numbers.
The stream delivers `activity`, `snapshot`, and, when the cursor is expired or
ahead, an explicit `snapshot_recovery` with the current `event_cursor`.

Replace local state from that recovery snapshot; never resubmit the task.
The current retention window is 256 activity metadata entries per task; completed
snapshots remain. Persisted events precede delivery and slow/disconnected readers
do not authorize duplicate execution. Disconnect does not cancel a job. Terminal
or input-waiting snapshots may close the stream; reconnect after a lifecycle action.

Saved chat streaming has different semantics: the conversation section above
does **not** promise this job replay protocol.

### Approval and recovery

Approve/deny requests identify the exact pending `approval_id`. The durable
approval binds actor, run, invocation, canonical arguments, capability, and
expiry. Display `canonical_arguments` without lossy numeric conversion.
Stale, duplicate, or conflicting actions fail rather than resubmitting a prompt.
Computer session policies independently govern which typed actions may be
automatically approved; a client-provided blanket grant is not accepted.

A mutating call with no durable result may be `uncertain`. Inspect the target
and reconcile the exact `call_id` with an independently verified result.
Reconciliation records an outcome; it does not execute the action again.
Known read-only failures do not automatically imply uncertain side effects.
See [agent recovery](../guides/agents.md#restart-and-uncertain-outcomes).

Agent snapshots and replay events use transactional
`OFFGRID_DATA_DIR/agent-state.sqlite`. Legacy `agent_tasks` JSON files are
validated/imported with originals retained; they are not the current write
store. Corruption or failed persistence blocks task operations. Preserve the
workspace and follow [backup/migration recovery](../advanced/workspace-recovery.md).

### Compatibility routes

`/v1/agents/run`, `/v1/agents/tasks`, and their lifecycle endpoints remain for
older explicit-run clients. They reuse the durable runner. The old
`stream: true` run response is not a substitute for reconnectable v2 job events.
New clients should use the versioned job contract.

The implemented computer resources include `/api/v2/computer/capabilities`,
`/status`, `/sessions`, `/sessions/stop`, `/model-check`, `/stop`, and the
advanced `/pairing` endpoint. Check methods and fields in OpenAPI rather than
assuming the entire roadmap resource set exists. Public raw `/v1/computer`
mutation routes are retired; they do not bypass the job/consent flow.

## Tools and MCP

These administrator-managed surfaces configure the running service:

- `GET/PATCH /v1/agents/tools`: inspect/update enabled tools.
- `GET/POST /v1/agents/mcp`: list/save connections.
- `POST /v1/agents/mcp/test`: test a supported HTTP endpoint.
- `DELETE /v1/agents/mcp?name=URL_ENCODED_NAME`: remove a saved connection,
  including one currently offline.
- `GET /v1/capabilities`: inspect broader service capabilities.

A successful connection persists configuration; deletion persists removal
before detaching the runtime and tools. Failure is reported without claiming
removal. Neither operation erases remote data or undoes a request already sent.
See [MCP setup and troubleshooting](../guides/mcp.md).

Treat external tools and their responses as untrusted. Local model inference
does not keep tool arguments local when an external endpoint is selected.

## Errors

For v2 job errors, use the structured `error.code`, `message`, `retryable`,
and `request_id` envelope with its HTTP status. Older routes can retain their
legacy error shape; consult the contract and support both only where required.
Do not parse human-readable messages as machine codes.

A retryable transport/storage error is not permission to rerun an uncertain
side effect. Read the saved job or retry its identical submission with the
original request ID. Keep server logs private; omit tokens and private tool
data from issue reports.

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
