# Client correctness contracts (development)

These changes are in the working development source, not a claim about the
published v0.4.3 artifacts. See [production readiness](production-readiness.md)
for remaining work.

## CLI service requests

Knowledge commands (`offgrid kb ...`), saved-conversation commands (`session`,
`export-session`), and agent lifecycle controls (`status`, `approve`, `deny`,
`cancel`, `resume`, `reconcile`) now use the shared authenticated service client.
They do not fall back to editing service files. Other CLI commands, including
interactive chat saving, are still being migrated and do not yet all implement
this contract.

- `OFFGRID_SERVER_URL` is an HTTP(S) origin, e.g. `http://127.0.0.1:11611`.
  Do not include `/v1`, user credentials, query parameters or fragments.
- `OFFGRID_API_KEY` supplies the service Bearer credential. Never pass a real key
  in a URL or paste it into an issue. Remote use requires trusted TLS.
- Redirects are rejected; model registries/download hosts do not inherit the key.
- Requests have deadlines and support Ctrl+C, including while reading a response.
- Exit codes: `0` command succeeded, `1` operational failure, `2` invalid usage,
  `130` user cancellation. Reading a failed task's status is a successful read;
  the snapshot's `status` tells you whether the task itself succeeded.
- `--json` writes one JSON result or `{ "error": { "code", "message", "retryable",
  "request_id"? } }` to stdout. Progress/interactive prompts go to stderr. Do not
  infer success from empty output or ignore the exit code.
- `kb clear` requires explicit `--yes` when noninteractive or using JSON. Partial
  deletion/import errors report how much work completed. Cancellation/timeouts
  are not a guarantee that a server-side mutation was undone: inspect state first.

Examples (use your installed binary path if `offgrid` is not on PATH):

```bash
offgrid kb status --json
offgrid kb list --json
offgrid kb search "Where is the emergency procedure?" --json
offgrid kb add "./documents/meeting notes.md"
offgrid agent status RUN_ID --json
offgrid session list --json
offgrid session show "Meeting notes" --json
offgrid export-session "Meeting notes" --format markdown
```

Conversation exports default to stdout; an explicit output file must not already
exist. Protected or unavailable services never fall back to local conversation
files. These commands still use the existing v1 session names; generated IDs and
duplicate-title support depend on the pending staged migration. `offgrid export`
is the separate model/USB export command, not a conversation export alias.

Local file upload is bounded to 50 MiB and streamed rather than buffered wholesale.
Directory imports skip symlinks and unsupported extensions and stop on failure.
This does not expand supported extraction formats or add OCR.

## Backend identity and desktop attachment

`GET /api/v2/system` is a public, uncached identity document. It contains product,
version, source revision (or `unknown`), API version, UI build ID and capability
names. It exposes no local paths, credentials, account data or model inventory.
Only this exact GET is public; it does not bypass authentication on the v2 prefix.

The initial capabilities describe existing v1 session/stream/agent contracts.
An API version of 2 here does **not** mean durable v2 conversation/job/collection
operations are available yet.

Electron requires the same product version, supported contracts, and matching UI
build when its local bundle is present. The UI build ID is SHA-256 of `index.html`
with CRLF normalized to LF (it references content-hashed assets): a compatibility identifier, **not** a
signature or complete artifact-integrity proof. An incompatible or nonresponsive
occupied port is not replaced. An externally managed backend is never stopped
by Electron, and its paths are not misrepresented as desktop-local paths.

Settings shows the active service address, versions, API and UI build ID. Build
the UI and runtime from the same checkout before testing desktop development;
restart/redeploy an old service explicitly rather than expecting new source to
change a running process or container.

## Incomplete agent output

The durable and structured agent runners validate the model's completion reason
before using its answer or tool calls. Token-limit, filtered, missing and unknown
reasons fail the task; even syntactically valid tool arguments cannot bypass this
check. Durable history keeps any partial text labelled `incomplete`, outside
completed model context. Prior committed tool actions remain recorded; failing
a later model turn does not undo them. Models must return a supported terminal
reason, not silently omit it.

This prevents false success; it does not verify that an otherwise complete answer
is correct. Artifact checks and representative agent evaluations remain pending.

## SQLite connection policy

The runtime pins `modernc.org/sqlite v1.48.2` / SQLite 3.51.3, which includes the
[upstream WAL-reset fix](https://www.sqlite.org/wal.html#walresetbug).
The shared opener applies `foreign_keys=ON`, `synchronous=FULL` and a bounded busy
timeout on every pooled connection, verifies WAL mode and rejects known unpatched
versions. Paths are URI-escaped independently from settings; UNC paths are rejected.
Use a local filesystem—mapped network drives are not automatically detected.

This hardens the existing knowledge database; it is **not** the new transactional
workspace migration. Exclusive workspace ownership and offline backup/restore
are implemented separately; see [workspace recovery](workspace-recovery.md).

## Inference admission and event delivery

For live agent phases, response previews, reconnect behavior and CLI presentation,
see [live agent progress](agent-live-progress.md).

The inference gate defaults to one active request and at most 32 waiting requests.
Admission is FIFO, including model switches, so a continuous stream targeting the
loaded model cannot bypass an older request for a different model. A full queue
returns HTTP 429 with `inference_queue_full` and `Retry-After: 2`. Health reports
active/queued counts and capacity. Cancelled waiters leave the queue promptly.
Per-user/workload fairness and coordinated indexing admission remain pending.

Agent events are persisted before publication. A slow subscriber is disconnected
instead of blocking execution and must replay from its last persisted event ID.
Invalid persisted sequences and uncertain write failures fail closed. This is
hardening of the existing run log, not the pending transactional v2 job/event
contract, compaction, or browser reconnection store.
