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

### Profile and conversation SDK contracts (development)

Python exposes `client.agent.profiles` for owned profile list/get/effective/
resources/create/update/archive. Updates and archival require the expected
revision. Profiles reference installed models and authorized tools; these calls
never download models or change connection credentials.

`client.sessions.submit_agent_turn(name, content, request_id=..., profile_id=...,
profile_revision=...)` returns the service's `turn` and canonical `job`. Use
`client.sessions.turn(name)` or `client.agent.job(id)` to observe work.
`recover_agent_turn(name, turn_id)` repairs bookkeeping or attaches a saved
answer without executing again. An interrupted job still needs explicit resume;
uncertain effects still need inspection. These methods do not use the legacy
SDK's client-side `chat_with_session` history-save sequence.

`client.agent.steer(id, instruction, request_id=...)` distinguishes accepted
`pending_instructions` from applied `instructions`. It does not cancel an action
already sent. `stop(id)` requests cancellation; the returned status is the
authority, not an assumption of rollback. Conversation deletion accepts
`work="retain"` or `work="stop"` and propagates a pending-stop conflict.

New mutation methods disable transport retries. After a lost acknowledgment,
inspect saved state, then explicitly retry the identical ID/payload if needed.
Never change the request ID to recover a missing result. Errors from unsupported
servers propagate; there is no fallback executor or silently selected model.
Generated web contracts and client methods use these same service endpoints.
The current React Agents page manages profiles; Chat submits profile-enabled
turns and observes the canonical job. Old job links open a secondary inspector,
not a second execution path. Navigation, reload and final-result attachment do
not approve, resume or replay work. Shared-shell computer controls remain present
across navigation, and an unacknowledged Stop remains visibly unconfirmed.

The service-backed CLI exposes `agent profiles`, pinned `agent run --profile`,
and `session send/chat/turn/recover`. Interactive profile conversations queue
corrections on their active job; they do not start a second job while awaiting
access, approval or reconciliation. `/exit` detaches without stopping work.
Both CLI and Python check the advertised capability before profile-enabled
submission so older services cannot silently ignore agent fields and run plain
Chat. Cross-client installed-package qualification remains separate from the
HTTP, terminal and isolated browser contract tests.

The opt-in cross-client regression uses actual CLI, Python SDK and headless
Chromium against disposable authenticated handlers and fixture inference:
`OFFGRID_CROSS_CLIENT_TESTS=1 go test ./internal/server -run
TestAgentCrossClientIntegration -count=1 -v` (Go, Python and Playwright Chromium
must already be installed). It checks one canonical identity, idempotent steering,
cross-actor denial and cancellation across clients; it does not use port 11611 or
qualify an installed model or native desktop.

## Backend identity and desktop attachment

`GET /api/v2/system` is a public, uncached identity document. It contains product,
version, source revision (or `unknown`), API version, UI build ID and capability
names. It exposes no local paths, credentials, account data or model inventory.
Only this exact GET is public; it does not bypass authentication on the v2 prefix.

The initial capabilities describe existing v1 session/stream/agent contracts.
An API version of 2 here does **not** mean every v2 operation is available.
`durable-agent-events-v2` specifically advertises owner-scoped job snapshots and
ordered activity replay; it does not advertise v2 submissions, native control or
vision. See [live agent progress](agent-live-progress.md).

Desktop attachment distinguishes **external service compatibility** from
**bundled package consistency**. External services can have different release
versions and renderer builds when their API capabilities and renderer/desktop
bridge protocol are supported. An owned bundled child must additionally match
the exact desktop package version and renderer fingerprint.

The additive `desktop_bridge` identity field has `status: missing | invalid |
ready`. Ready metadata includes `schema_version: 1`, `protocol`, and `ui_build_id`.
The service validates a bounded `desktop-compatibility.json` from its actual UI
root against that root's index. Invalid/unsupported metadata cannot fall back to
legacy handling. The UI build ID is SHA-256 of `index.html` with CRLF normalized
to LF (it references content-hashed assets): a compatibility identifier, **not** a
signature or complete artifact-integrity proof. Protocol compatibility never
grants authentication, computer access, or local consent.

Protocol 1 describes the existing bounded `window.electron` main/preload API.
Breaking argument, response, or permission semantics require a new protocol;
supporting an older protocol requires retained tests. The renderer build emits
its declaration, and packaging verifies the copied UI. Keep declarations and
tests aligned when changing desktop IPC; do not derive protocols from semver.

Legacy metadata absence is accepted only by `desktop/legacy-compatibility.json`:
service 0.4.14 or 0.4.15 with renderer fingerprint
`a7336324e4ea09e8a9f5e25aabf2a53c2f9cb39b81eb990a081ba9a84e66f617`.
This is the reviewed workspace-refresh renderer, not every 0.4.14 build. Its
main/preload/computer-runtime contracts are unchanged between those release tags.
The mapping survives a newer desktop renderer build; unknown legacy tuples
remain blocked. Old installed desktops retain their old strict policy until
updated. Explicit invalid metadata is never treated as legacy absence.

An incompatible or nonresponsive
occupied port is not replaced. An externally managed backend is never stopped
by Electron, and its paths are not misrepresented as desktop-local paths.

Settings shows the active service address, versions, API, UI build ID, attachment
basis, and ownership. The service supplies its own UI; updating desktop alone
does not update an external service's features. Build
the UI and runtime from the same checkout before testing desktop development;
restart/redeploy an old service explicitly rather than expecting new source to
change a running process or container.

## Native document feature negotiation (development)

Document support is an explicit `native_documents:1` contract, not inferred from
desktop protocol 3, a release number or whole-value replacement support. Pack
metadata declares the understood contract revision. Worker startup reports only
contracts implemented by its provider; the parent rejects a revision absent from
the verified pack, unknown feature keys, and malformed startup fields. A legacy
pack/startup has no document feature and remains limited to its existing tools.

The service advertises `native-document-negotiation-v1` only to indicate that it
understands this handshake. Companions send optional `native_features` during
pairing only when both service and worker understand it, and validate the echoed
selection. Missing support at either end yields no negotiated document tools.
Unknown revisions are refused, not silently downgraded. Features live in the
actor-owned session and do not extend its selected-window or desktop grant.

Negotiation is not execution readiness: the provider must additionally expose
the operation for the current, fully bound document. New task-first jobs use
workflow-evidence v2; document schemas appear only for a negotiated actor-owned
host and the current observation/read stage. Execution checks this again before
queuing input. Legacy adapters still advertise no document feature. Service and
synthetic-worker integration tests do not qualify UIA, AX or AT-SPI editing.

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
hardening of the existing run log. Agent job replay now separately uses the same
SQLite transaction as the task snapshot, with bounded metadata compaction and
explicit snapshot recovery. The old run log is not its authoritative replay store.
