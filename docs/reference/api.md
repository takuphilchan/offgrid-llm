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

Set `stream` to `true` for server-sent events. OffGrid also provides the
Ollama-compatible `/api/chat`, `/api/generate`, `/api/tags`, and `/api/embed`
routes for clients that use those conventions.

## Durable conversations

The UI uses server-owned sessions rather than browser-only message state:

- `GET /v1/sessions` lists conversations.
- `POST /v1/sessions` creates one.
- `GET /v1/sessions/{name}` loads its complete history.
- `DELETE /v1/sessions/{name}` removes it.
- `POST /v1/sessions/{name}/generate` generates and atomically persists a full
  user/assistant exchange.

If generation fails or the caller cancels, no half-turn is written. This route
is the preferred application workflow when conversation continuity matters.

## Knowledge

Knowledge endpoints live under `/v1/documents` and `/v1/rag`. RAG is optional:
clients must read `/v1/rag/status` and present an unavailable state when a local
embedding model has not been configured. Setting `use_knowledge_base` on a chat
request has an effect only when the RAG engine is enabled and indexed.

## Agents and MCP

MCP and agent routes are governed, administrator-level surfaces. Agent
sandboxes start lazily only when a tool call needs them. Treat terminal,
computer-use, filesystem, and external MCP tools as privileged operations and
require explicit policy or user approval.

These capabilities continue to evolve and are not all included in the stable
OpenAPI document yet. Their guides must be read together with the security
configuration before network use.

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
