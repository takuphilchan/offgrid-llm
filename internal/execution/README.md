# Native execution operations

This package is a host-side operation provider, **not** an agent loop or an
generally qualified integration. `OpenSession` is an authority constructor for
a trusted local supervisor after operator approval. Do not construct one from
an HTTP request, model output or renderer-provided JSON.

Bindings include actor, service workspace, host, execution session, grant revision
and an admitted job. Old profiles and computer consent confer none of this authority.
Revocation closes eligibility. Service/job integration and enrolled authenticated
transport, app folder selection, isolated execution and matching worker packaging
are implemented behind the explicit native-execution preview. Real-model/backend
and release qualification remain tracked in
`native-agent-workspace-execution`.

File operations use Go rooted handles, provider-relative paths, regular-file checks,
bounded pages and SHA-256 revisions. They do not reinterpret Windows/WSL/container
paths. Page data is raw bytes, base64 in JSON, not line-numbered display content.
Directory pages are sorted and revision-bound; their revision represents the entry
inventory, not the contents of every file. Reading checks a whole bounded file's
digest before returning a page. No process-global cwd is changed.

Directory/file pages report their starting `offset`; directory pages also report
`total_entries`. `complete` means this page reaches the end, not that a nonzero
page repeats earlier content. Directory presentation supplies bounded quoted
names without changing raw receipts. The model's page menu offers only recorded
next offsets/revisions for up to four recent targets. Completion, cwd changes
and possible workspace mutations invalidate old continuations; a fresh read
remains available. Host revision/confinement checks remain authoritative.

Rooted handles prevent ordinary path/symlink escape but are not an OS sandbox:
mounts/hard links and hostile processes sharing the OS account require separate
isolation. Trusted local commands will run with that account's authority, not
folder confinement. Do not select protected/system trees as project workspaces.

Run isolated fixtures with `go test ./internal/execution`.

`Hosts` indexes already approved endpoints by actor/workspace/session/grant and
requires explicit canonical-job admission. Registration is a trusted constructor,
not a renderer/HTTP authority endpoint. `StdioEndpoint` starts only an absolute,
digest-pinned worker supplied by the trusted supervisor, using private credentials
and owned process cleanup. Its private supervisor messages admit/cancel jobs;
they are not model tools. The grant file and journal must be outside the approved
file root. A failed/timed-out private stream is closed, never reused for a retry.
Service restart does not restore this ephemeral host authority.

The shared reviewed catalog is owned here, not redefined by each client. The
service's canonical runner can list/read/search/edit an attached workspace and
retain exact text and read-back evidence; CLI selection pins the same reference
across turns. A matching CLI can locally approve and supervise a pinned worker;
the host's exact version, native architecture, digest and private grant/state are
checked before readiness. Native app grants and worker packs share this provider;
installed-platform qualification and signed releases remain separate gates.
Command authority must be separately present in the local grant. Trusted project
tests/builds use a small typed invocation grammar; other invocations require the
durable runner's exact approval, carried in a private supervisor field rather
than model arguments. Project code can exercise the approving account's full
authority. This policy is not arbitrary-command confinement or model qualification.

## Private worker protocol v1

`offgrid-executor --stdio --grant-file ABS --state-dir ABS` reads a local,
regular private grant file and requires a 64-character supervisor credential in
`OFFGRID_EXECUTOR_TOKEN`. The credential is never forwarded to command children.
This is owned-child stdio, not a public command service. Grant files and journal
directories require account-private permissions/Windows ACLs. The native
supervisor loads a digest-pinned `execution-pack.json` and independently checks
the worker handshake; the digest must come from trusted application assets, not
a project/model package. Release signing/packaging remains a separate gate.

Frames are four-byte big-endian lengths followed by one strict JSON object,
maximum 1 MiB. Duplicate fields, malformed Unicode, unknown fields, truncated
frames and unsupported protocol versions are rejected. Requests contain `version`,
`id`, `operation_id`, `binding`, `token`, `operation`, `arguments` and optional
supervisor-authored `approved` for an exact process invocation. Results
contain the request ID and a receipt or safe error code. Tokens are excluded from
journal payloads. The host admits jobs separately from model operations.

Implemented operations: `workspace_info`, `directory_change`, `file_list`,
`file_read`, `file_search`, `file_write`, plus supervised process primitives.
Process dispatch is fail-closed without the local process grant and reviewed
invocation policy. An approval bit in tool arguments is rejected, not authority.
Every mutating operation claims an identity/payload digest before dispatch;
duplicate claims return evidence or uncertainty, never replay. A worker restart
marks unfinished claims/dispatched processes uncertain without PID adoption.

The process provider reports actual exit codes and bounded output. Windows uses
a suspended start and kill-on-close Job Object; Linux observes exit without
reaping before serializing process-group cancellation, avoiding reused-PID kills.
macOS uses a separately compiled exit observer and remains unqualified until
native tests run. Deliberately detached Unix daemons are not a sandboxed process
tree; use real isolation for hostile code. `process_inspect` pages an append-only
raw output stream with offsets; running processes keep a continuation even when
no new bytes have arrived. `process_wait`/`process_stop` use bounded waits (default
250 ms, maximum 1000 ms) so stdio remains responsive to control requests. Enrolled
transport and app integration use the same provider. Supervisor shutdown closes
worker stdin first so the worker can stop owned children and settle receipts.
Forced shutdown after the bounded grace period is unconfirmed, never proof of
command termination.

## Enrolled host bridge

The service preview issues a two-minute single-use ticket bound to actor/workspace
after local review. The native supervisor uses a separately random host token,
never the user's service API key, for the exact `/api/v2/execution/host/*` routes.
Only after the pinned worker independently confirms the approved context does
the service publish a ready reference. Polling delivers an operation at most once;
a lost response closes eligibility rather than redelivering a mutation. Repeated
identical acknowledgment is idempotent. Heartbeats expire after 30 seconds;
revocation, account-policy loss or transport failure closes local supervision.
No service-side path translation or restoration of grants on restart occurs.

The service runner uses separate start/output/wait/stop tools. Start is only a
dispatch receipt; an observed terminal process result is required before final
completion. Cancellation does not require inference: it closes that job's dispatch
and waits for its owned trees. A lost acknowledgment stays visibly uncertain.

Host mutation intent is separately pinned in the canonical service checkpoint
before dispatch. Authenticated replies are persisted there before native reply
acknowledgment. Only a matching original receipt can recover an uncertain call;
service restart can incorporate a saved conclusive receipt without connecting a
host or running inference. `supervisor_receipt`/the private `receipt` bridge kind
only read the native journal against the original payload digest. They never
claim or repeat an operation, including after job cancellation. Missing entries
are uncertain, not proof that no effects occurred. Still-dispatched processes
remain uncertain until terminal evidence is available.

The process-start model projection is bounded to one output page; raw journal
evidence remains unchanged and continuation is explicit. If final process receipt
persistence fails, the native session is revoked rather than accepting more work.

The opt-in isolated-Linux backend uses a pinned, already-installed image ID and
the native host's local Unix Docker socket. It never pulls an image. Commands run
as the current non-root UID/GID with read-only rootfs, dropped capabilities,
no-new-privileges, bounded memory/CPU/PIDs, a private tmpfs and exactly one
nonrecursive approved-folder mount at `/workspace`. Network is explicitly `none`
(default) or `bridge`. The service, command container and model receive neither
the Docker socket nor native journal/grant/credential files. This is Linux/WSL
execution, not a transparent Windows/macOS folder translation.

Container ownership/name is persisted before create, and the verified exact ID
before start. Attached output is bounded independently of the true exit status.
Stop/timeout verifies termination before removal. A lost start acknowledgment
remains uncertain, not a reason to create/start another command. Restart and the
native supervisor after worker exit clean up only journaled, matching-daemon,
matching-label objects; cleanup never starts a container or certifies task
completion. Unavailable daemon, ownership mismatch or persistence failure keeps
stopping uncertain and prevents a success acknowledgment. Simultaneous loss of
the supervisor/host is not a universal kill-on-power-loss guarantee; durable
ownership remains for explicit recovery with the original state directory.

Real disposable WSL/Alpine command tests cover Unicode read-back, failure exit,
Stop, timeout, three repetitions, and abandoned-worker ownership cleanup. Those
checks do not qualify model reasoning, every image, or installed native apps.

Writes create exclusively unless an exact preimage digest authorizes replacement.

`workspace_write_from_input` is a service-side reviewed projection onto the same
`file_write` host operation. It selects unique boundaries in this root's original
request or an applied user correction; tool results, prior conversations, pending
corrections and foreign jobs are not sources. Versioned new-job eligibility and
the existing write grant are both required. Markers are excluded; selected UTF-8
bytes, BOMs and line endings are preserved. Selection ambiguity fails before
dispatch. The durable intent contains resolved bytes, and completion/recovery
require the original host's matching digest, byte count and verified read-back.
Unconfirmed effects are not retried. Provenance identifies the selected input and
span; it does not claim that a model chose the right meaning or finished the
whole task. Older jobs and ordinary `workspace_write` keep their prior contract.

For reviewed file/info/list/search operations, a conclusive rejected read may
produce service-authored correction guidance. The durable root allows at most
three follow-up repairs, charges normal model iterations and retains each error.
Only an exact reviewed descriptor and a known local rejection qualify; provider
prose, missing host acknowledgments and outstanding mutations do not. A new
model decision may correct the read or answer without irrelevant file work.
This is not automatic replay, successful verification or broader authority.

Replacement stages a file in the same rooted directory, rechecks the preimage and
atomically replaces it, then reads back the bytes. This is optimistic concurrency,
not an OS compare-and-swap against arbitrary external writers; a user/process
modifying the target during the commit window can still conflict. Do not claim
exclusive access, universal undo or exactly-once external effects. Post-dispatch
errors preserve partial/unknown effects. Literal search skips links/special/binary
files, bounds scanned entries/bytes/files/matches and reports incompleteness.
