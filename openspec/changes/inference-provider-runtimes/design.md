# Design: Provider-neutral local inference runners

## Endpoint owner configuration workflow (task 8.4.4)

The complete administrator workflow mutates the existing owner configuration;
it does not introduce another endpoint registry or scheduler. A bounded prepare
request validates the proposed entry and returns an expiring, actor-bound review
identity with the exact origin, address pins, model/runtime identity, actor scope,
credential reference and expected whole-file digest. Commit accepts only that
review identity and an explicit request identity. It rechecks current admin
permission, installation-owned ceilings and the file digest under the owner
writer lease before publishing. Models and ordinary chat requests still accept
only an enrolled identity, never a URL or credential reference.

Credential references come from a host-configured allowlist; the renderer cannot
name arbitrary process environment variables or receive their values. Managed
runtime entries reference immutable host-provisioned templates. Browser fields
cannot introduce executable paths, arguments or environment variables. Remote
entries use the same exact-origin/IP/TLS validation as normal provider routing.
Preparation never probes an endpoint, starts a runtime, downloads a model or
changes the selected model.

One private durable journal records intent before owner-file publication and a
receipt afterward. The record binds actor, request/review identity, old/new file
digests and activation epoch; it is audit evidence, not an alternative enrollment
authority. A crash or uncertain write blocks affected endpoint access until an
administrator explicitly reconciles the actual old/new/other file digest. No
write or request is automatically replayed. A retry with the same request identity
returns the existing receipt or uncertainty, never applies a different draft.
Fresh restore epochs invalidate historical pending review and activation grants.

Changed and new entries remain unavailable until service restart installs their
exact revision, followed by fresh explicit access review. Committing configuration
does not implicitly grant endpoint access. Revocation cancels active request
leases without claiming already dispatched remote effects were undone.

When no external configuration path is selected, the existing configuration
factory resolves one workspace-owned owner file. Its absence is an empty initial
configuration, not implicit enrollment; it is created only after administrator
commit. Existing external files remain at their explicit installation path with
no silent migration. Writes require an installation-enabled writer and verified
private file/directory ownership plus a shared OS writer lease; unavailable
ownership leaves readable enrollment usable but configuration editing unavailable.
No live installation permissions are broadened to make editing succeed.

Acceptance includes stale drafts, changed administrator/credential/template
ceilings, duplicate request identities, before/after-publication failures,
restart/reconcile/restore, competing writers, FIFO/link substitution and an actual
isolated service-to-renderer creation/update journey. These checks do not qualify
real provider models or complete the remaining local-runtime/platform work.

## Endpoint administration foundation (existing task 8.4)

The first rendered administration increment selects only identities already
provisioned by the service owner. It does not accept a destination, credentials,
model digest, executable path or actor grant from model/browser requests. The
original trusted endpoint file remains the ceiling for every actor and outbound
request. Its file opener must reject replacement/special files without blocking
before its type check. Read-only listing must never contact a provider.

A service-owned durable policy can subtract access from a provisioned endpoint;
it cannot create or broaden a grant. Administrator mutations identify the exact
endpoint/revision and expected trusted-configuration/policy revisions, record the
acting identity, and publish through the existing durable storage primitives.
Corruption fails closed. Re-enable still requires the current trusted entry and
current actor permissions; neither backup restoration nor missing policy may
silently undo a revocation. Restore must have an explicit new activation epoch
rather than treating the backed-up workspace ID as fresh authorization.
The initial explicit review creates a nonempty durable activation epoch before
saving any grants. Read-only status creates no metadata; an existing policy with
a missing activation marker is unavailable, never bound to an implicit empty
original epoch.

Production composition must retain its existing OS workspace ownership lease
before policy installation and until drained cleanup completes. The policy mutex
serializes that single service writer; it does not claim hostile same-account
filesystem isolation. Durable policy publication syncs the private temporary
file and its replacement namespace (Unix parent fsync, Windows write-through).
Failure, including after rename, cancels admitted leases and poisons authority
for that service rather than assuming the old snapshot survived.

Restore first verifies the historical archive. It then writes a fresh 32-byte
random epoch encoded as 64 lowercase hex characters into the private stage
through its retained root, syncs it and publishes only a new destination. The
returned manifest remains historical verification evidence, while the next
backup inventories the new marker bytes. A corrupt original policy is not
implicitly repaired; old enabled decisions do not match the fresh restore epoch.

Queued and new requests recheck this policy before dispatch. An already sent
remote request may already have disclosed its prompt; revocation does not claim
to retract or undo that effect. Stop retains the existing request cancellation
path. Durable disable also cancels admitted HTTP and whole-generation contexts,
including a generation blocked in a response callback; uncertain owned-tree
retirement cannot claim released capacity. A connection check is an explicit bounded operation using the shared
provider scheduler and lifecycle, never direct runtime creation or model pulling.
Its result establishes only a checked connection/model/runtime identity, not
tool, semantic, offline or hardware qualification.

Endpoint menu entries must use endpoint-owned controls, not legacy file Verify
or Delete. The UI exposes current availability, grant/revision conflicts and
restart requirements without displaying credential values. Complete owner
configuration creation/update remains required after this foundation, using
reviewed validation, audit, atomicity and activation semantics; it is ordinary
remaining program work, not excluded scope.

## ONNX embedding integration (approved roadmap M5)

The existing `EmbeddingEngine` and its shared provider scheduler retain ownership.
A failed load or invalid dimension handshake must run cleanup; if that fails,
the implementation stays tracked as unresolved rather than ready and all later
allocation is blocked. Service shutdown still attempts final cleanup. This is a
prerequisite for any additional embedding runtime.

The initial ONNX path is an explicitly installed `onnx-embedding` package with
one ONNX graph, a local tokenizer JSON and a pinned embedding-settings JSON.
Settings declare model family, input/output tensor names, maximum tokens,
dimensions, padding, pooling and normalization. The first adapter is CPU-only;
other execution providers and asymmetric query/document prefix models remain
explicitly unavailable until their own selection/identity behavior is implemented.
No package can self-certify semantic retrieval quality or runtime readiness.

A pinned local Python worker uses the same supervised process protocol and
retirement primitives as other workers. The embedding implementation calls that
worker under `EmbeddingEngine` admission, never a second inference scheduler.
Interpreter/script/dependency versions and package identity are verified. The
worker loads bytes from installed artifacts only, disables runtime fallback and
uses `CPUExecutionProvider` explicitly. Graph external tensor data, custom-domain
execution, runtime plugin libraries and graph-controlled filesystem paths are
rejected before ONNX Runtime loading in the first adapter. Larger/external-data
models need separate validated support; they are not silently treated as complete.

Tokenizer loading is local, with explicit special tokens/padding. Input token
overflow is rejected rather than silently truncating source text. Attention masks
govern pooling; output rank/count/dimensions, finite values, nonzero norms and
requested normalization are checked. Request bytes, tokens, tensor shapes, batch
size and result bytes have finite bounds. Measured token usage and preprocessing
identity are retained. Vectors are withheld until validated terminal success;
cancellation or malformed output cannot publish a partial index update.

The model package/menu and embedding API must resolve the same installed identity;
Knowledge retains its existing embedding-identity gate and never automatically
rebuilds an index after a provider change. Fixture protocol/process tests establish
contracts only. Real ONNX model semantic similarity/retrieval, CPU/platform,
offline operation, cancellation, performance and repeated frozen workflows remain
separate open evidence requirements.

The first reachable Windows factory uses explicit installation-owned
`ONNXEmbeddingRuntimeFile` configuration and the existing package store. A new
`embedding:<package-id>@<revision>` selector routes to verified package metadata;
it cannot fall back to a colliding legacy model or be selected for Chat. Import
and runtime enrollment do not select a default model. Prior saved Knowledge
selection may restore normally, subject to its existing identity checks.

Windows read handles deny changes to existing files but do not prohibit new
children in a directory. An isolated regression demonstrated new importable files
in both the runtime root and `Lib/site-packages`. Final admission therefore
requires administrator-owned read-only runtime namespace ACLs, ancestors that
cannot be retargeted by ordinary users, and a non-elevated service token without
privileged ownership/filesystem/process overrides. Runtime files still retain
deny-write/delete leases and exact inventory hashes. This is a restricted
provisioned mode, not a completed ordinary desktop runtime installer. Privileged
administrator/OS mutation remains outside the boundary. The service never
changes owners or ACLs automatically. Explicit provisioning procedures operate
only on a fresh isolated fixture directory and are not executed by tests.

Python runs with isolated/no-site flags from the leased runtime directory.
Pinned adjacent `._pth` files cannot import site or escape the runtime; external
virtual-environment bases are rejected. The worker verifies its executable,
prefixes and import paths before loading dependencies from the fixed pinned
site-packages directory. Model artifacts contain no executable path authority.
The worker receives the verified manifest's exact graph/tokenizer/settings
digests as trusted arguments. It hashes the immutable byte buffers it will pass
to the graph/tokenizer parsers, never reopens a verified path for parsing, and
rejects changed bytes. Linux can therefore use the ordinary service-owned
package store while requiring its Python runtime/import namespace to be
root-owned and non-writable to the service. Its factory uses the existing
util-linux/PID/user namespace owner through ProcessProvider's private lifecycle
starter hook; there is no separate executor. Both pipe pairs are explicitly
owned, including child ends when namespace validation fails before process
start. Private writable state and root-owned parents follow the existing Linux
managed-runtime contract. A PID namespace alone does not make writable Python
imports immutable. Final provisioned Linux and Windows factory qualification
remains unrun; macOS ownership/activation and ordinary installed packaging remain
unfinished.

Primary interface basis reviewed 2026-11-20:
[ONNX Runtime sessions and execution providers](https://onnxruntime.ai/docs/api/python/api_summary.html),
[ONNX external data](https://onnx.ai/onnx/api/external_data_helper.html), and
[Hugging Face Tokenizers encoding/masks](https://www.huggingface.co/docs/tokenizers/python/latest/api/reference.html).

## Memory-pressure follow-up (2026-11-15)

Use current OS physical availability and, on Windows, separately measured commit
headroom. CPU budgets cannot exceed either pool; GPU availability cannot bypass
the host reserve needed by the process/driver. Failed measurement is not spare
capacity. Prewarming is speculative: at most one operation, 512 MiB per batch,
30 seconds, a 1 MiB read buffer, 32 history entries, and pressure checks between
reads. Pause cancels the current operation even if resumed before its next read.
Partial reads never claim the complete model is warm; page-cache residency is
an OS-controlled hint, not verified retention. No active inference is killed and
no model, context, download or user document is changed by these controls.

## Architecture

The service keeps one agent-facing inference contract and delegates native work
to a provider selected from immutable model metadata:

```text
Agent / chat / capability probe
             |
      provider-aware scheduler
             |
  llama.cpp | MLX worker | Transformers worker
             |
 CPU/CUDA/Metal/ROCm/Vulkan/MPS local resources
```

The scheduler owns admission, safe model switching, budgets, process ownership,
and cancellation. Providers own native loading and transport details. Workers
never receive desktop authority, credentials, or unrestricted filesystem access.

## Provider contract

Add an internal provider interface with load/start, capability handshake,
streaming completion, normalized tool decision, token accounting, memory/device
metrics, cancellation, health, and close operations. Non-llama providers use a
versioned authenticated loopback protocol containing protocol/provider/runner
revisions, request and cancellation IDs, model digest, typed stream events,
terminal status, and capability metrics.

The service supervises worker processes with bounded startup, request, shutdown,
and restart limits. A crash is operational unavailability, not model
incompatibility. No worker can execute arbitrary commands or install packages.

## Providers

- **llama.cpp:** adapt the existing binary manager and HTTP engine with no
  behavioral change; GGUF models without engine metadata default here.
- **MLX:** pinned worker for MLX/safetensors text models, Metal-first, with
  other backends admitted only after measured capability evidence.
- **Transformers:** pinned local Python/PyTorch worker with remote code disabled,
  explicit device selection, and pinned model-family adapters.

VLM/audio metadata is additive, but dispatch requires qualified preprocessing,
projector/encoder identity, prompt assembly, and tool-call evidence.

## Model metadata and trust

Extend manifests and installed projections with engine, format, backend,
runner/adapter revisions, modalities, tool-call status, context/template
identity, preprocessing dependencies, and separate installed/runtime-ready/
smoke-tested/qualified states. Runtime binaries are trusted bundled assets or
explicitly imported offline providers. Model artifacts never carry executable
provider code. Unknown families remain metadata-only until a pinned adapter is
qualified.

## Scheduling and migration

One admission scheduler serves parent/child/perception/probe/chat inference.
It prefers a resident compatible runner, serializes unqualified loads, measures
load/queue/generation/cancel/unload separately, and preserves Stop. Provider
changes occur only at persisted safe boundaries and never retry unknown effects.

Existing GGUF records receive the llama.cpp default during an additive
migration. Existing profiles, tasks, revisions, permissions, API fields, and
model files remain valid. No download or automatic re-import occurs.

## Compatibility and rollout

The public API adds provider metadata and readiness fields while preserving old
responses. Feature flags expose MLX/Transformers only after worker contract and
real hardware checks pass. Text is qualified independently from VLM/audio.

## Enrolled HTTP providers (2026-11-20)

Ollama `/api/chat` NDJSON and vLLM `/v1/chat/completions` SSE normalize into the
existing provider event gate. Only a complete terminal response with usage can
finish; tools are withheld until the complete validated decision. Requests never
retry or select another endpoint automatically. Ollama model digests and runtime
versions are read back; vLLM model identity additionally needs a service-owned
deployment attestation because `/v1/models` does not attest weight bytes.

Enrollment is installation configuration, not model output or arbitrary public
request URLs. It pins exact HTTPS origin, permitted dial IPs, upstream model,
runtime/adapter/model identity, actor allowlist and an environment credential
reference. Every admission and HTTP request rechecks current enrollment and
actor scope; redirects and environment proxies are disabled. No credential is
returned in diagnostics. Loopback test fixtures use an explicit lifecycle lease.
Externally managed local endpoints require a supervisor lease proving resource
release; disconnect alone cannot claim that local model memory was unloaded.
Remote endpoints describe remote resources, never local free memory.

Explicit remote text models use `endpoint:<id>@<revision>` in existing chat/API
and text-only agent paths. They do not enter automatic installed-model pools.
The `endpoint:` namespace takes precedence over local registry records. Local
metadata, tokenizer, projector and cached-lease admission reject this namespace,
and a legacy local collider is omitted from the public model menu without
changing its registry record or files. Endpoint deletion cannot remove local
artifacts. Missing or revoked enrollment always fails the explicit selection.
Authorized remote entries appear in existing model lists without readiness
claims. Local process ownership, tool-qualified agent routing, rendered endpoint
management/readiness and real runtime/hardware qualification remain explicit
subsequent tasks, not satisfied by fixture evidence.

### Owned-local HTTP lifecycle follow-up (task 8.4)

The first concrete owned-local enrollment is Windows Ollama with a preprovisioned
legacy GGUF manifest, dedicated private runtime/model/state directories and a
complete SHA-256 runtime-file inventory. Manifest/config/layer bytes are verified
before launch and retained under read leases. New manifest-list/v2 layouts fail
closed until their format adapters are implemented. This increment does not
complete local vLLM, POSIX containment, tool qualification or task 8.4 as a whole.

The operator must explicitly acknowledge a trusted single-user host and Ollama's
unauthenticated loopback API. Exact TCP connection ownership is checked against
the runtime's Windows Job Object before HTTP writes; this prevents sending a
prompt to an unrelated listener but does not authenticate other local clients to
Ollama. Authenticated multi-user runtime isolation remains an unmet requirement.
No proxy, random port or loopback address is described as satisfying it.

The existing provider factory and `HTTPEndpoint.AcquireLocal` boundary will own
local server startup; no second scheduler or agent executor is introduced. The
trusted launch record must bind provider/runtime identity, executable and model
bytes, a service-owned data directory, backend/context limits and a restricted
environment. Model responses and public request fields cannot supply commands,
arguments, runtime paths or arbitrary local URLs. Startup rechecks actor and
enrollment authority before acquiring a process lease. Missing model files or
changed pins fail without downloads, imports, updates or fallback.

An owned runtime is a process tree, not just the direct child PID. Windows needs
a distinct kill-on-close job per runtime, with ownership established before
descendants can escape; POSIX needs containment that includes descendants which
create new process groups, with bounded exit verification. Per-runtime Windows
jobs now implement this boundary; a service-wide job or direct-child `Kill`/`Wait`
alone cannot prove that Ollama runners or vLLM workers released resources.
Termination uncertainty must
retain the lease and poison admission instead of reporting freed capacity.

The listener must be local and tied to this owned instance. A successful health
response from a pre-existing or replacement process is not ownership evidence.
Binding/connection ownership and startup/retirement races require platform
checks; unsupported ownership checks must keep the local provider unavailable.
Use the provider's private authenticated IPC where available and preserve the
existing local service boundary. A random port or the vLLM API key alone is not
sufficient protection for every upstream control endpoint.

Ollama launch sets a dedicated model/home directory and disables cloud features
(`OLLAMA_NO_CLOUD=1`), while catalog and stream checks continue rejecting remote
aliases. vLLM launch uses verified local model/tokenizer paths, offline cache
settings, disabled usage telemetry and no remote-code trust, remote model fetch,
automatic plugin discovery, external tool servers or multimodal URL fetching.
Do not inherit credentials or user cache paths from the operator environment.
These controls need provider-version tests and an offline workflow run; setting
environment variables alone is not offline qualification.

Validation will launch only isolated test executables from temporary state:
parent/child/grandchild survival, concurrent close, crash during startup,
revocation before launch, occupied/replaced listener, cancellation, failed kill,
and retained admission after uncertain shutdown. Real Ollama/vLLM process trees,
memory reclamation, hardware throughput and workflow quality remain task 8.5.
The model menu may advertise only the installed and evidenced state actually
observed; local supervision does not automatically qualify tool agents.

### Restricted Linux lifecycle and enrollment (partial task 8.4)

Linux must prove descendant lifetime independently of process groups: a runner
can create a new session. A dedicated user/PID namespace can provide this proof,
because termination of its PID 1 tears down the namespace's remaining tasks.
The current host supports an isolated unprivileged namespace probe; its cgroup
root is not delegated, so the implementation must not change host cgroup/sysctl
policy to obtain that proof.

The installed Go syscall implementation's parent-death check compares getppid
with the original host PID. It cannot be combined naively with CLONE_NEWPID,
whose child sees an external parent as PID 0. The shared process owner now
creates the exact init with a kernel-returned pidfd, supplies private procfs and
launches only the approved arguments through a pinned constant activation gate.
EOF before activation prevents target execution, covering parent loss before
PDEATHSIG is armed. The creating OS thread stays alive through Wait. Live identity
checks verify namespace/init/mappings; retained pidfd exit handles fast commands
without delayed PID discovery. Unavailable kernel/launcher proofs fail closed
without a process-group fallback. This replaces the earlier external-monitor
implementation below; source review and actual failure/fix results are in
`docs/quality/evidence/worker-linux-namespace-2026-11-21.md`. Live isolated Linux race tests passed for setsid descendants,
independent namespaces, init/monitor/service-owner death, startup failure and
concurrent close. The launcher opener rejects a regular-file-to-FIFO substitution
without blocking. Results and the corrected EINTR test failure are recorded in
`docs/quality/evidence/provider-linux-namespace-2026-11-20.md`.
This is process lifecycle containment, not a filesystem/network sandbox or
authenticated daemon IPC. Linux enrollment now requires the restricted
lease/listener integration below. A read-only Unix file descriptor pins an inode
but does not prevent another writer changing its bytes; advisory locks and
read-only bind mounts do not provide an equivalent to Windows deny-write leases.
Runtime/model immutability must be enforced before enabling Linux enrollment.

The first Linux integration is restricted to an unprivileged service with no
effective/permitted/inheritable/ambient capabilities and administrator-provisioned
root-owned runtime/model trees. Every ancestor and artifact must reject group or
other writes, symbolic links and regular-file hardlinks. A separately pinned
root-owned util-linux launcher uses the same ancestor checks. The private writable
state belongs to the service account, with its parent and ancestors root-owned
and non-writable to that account. Opened state identity/owner/mode and ancestry
are checked before locking. Its flock is cooperative and does not protect against
hostile same-account code deleting the lock. No chown, chmod, runtime installation or
host policy change is performed. Privileged administrator mutation and the trusted
base OS loader/libraries are outside this integrity boundary. Broader user-managed
installations remain unsupported until equivalent integrity controls exist.

Linux connection admission must map the exact established server/client TCP tuple
to its socket inode, then retain a pidfd for a process in this runtime's namespace
which owns that socket. Recheck tuple/inode and process/namespace identity before
HTTP writes; bounded procfs enumeration, missing procfs access or a non-owned
listener fails closed. Existing shared scheduler and uncertainty semantics remain
the only admission owner. This still requires explicit trusted single-user and
unauthenticated-loopback consent, not a multi-user daemon-isolation claim.

Primary lifecycle basis:
[Linux PID namespace semantics](https://man7.org/linux/man-pages/man7/pid_namespaces.7.html),
[kernel namespace exit proof](https://github.com/torvalds/linux/blob/master/kernel/pid_namespace.c),
[util-linux namespace launcher](https://github.com/util-linux/util-linux/blob/master/sys-utils/unshare.c),
and [cgroup delegation](https://docs.kernel.org/admin-guide/cgroup-v2.html).

Primary references reviewed 2026-11-20:
[Ollama local configuration and cloud disablement](https://docs.ollama.com/faq),
[vLLM server arguments and authentication limits](https://docs.vllm.ai/en/latest/cli/serve/),
and [vLLM environment controls](https://docs.vllm.ai/en/latest/configuration/env_vars/).

## Memory-pressure follow-up (2026-11-15)

- Replaced Go-heap-as-free-RAM prewarming estimates with current OS memory.
  Windows commit headroom is measured separately and used in llama load admission;
  free VRAM cannot bypass the host commit reserve. macOS no longer assumes 80%
  of installed RAM is free. Failed monitor samples are marked unmeasured.
- Prewarming now has one operation, one 1 MiB buffer, a shared maximum 512 MiB
  byte budget, 30-second deadline, bounded 32-entry history and no producer queue.
  Pause/resume invalidates old work; partial/truncated reads cannot claim completion.
- Full `go test ./internal/resource ./internal/inference -count=1` passed
  (`build/memory-resource-inference-full.log`). Focused server context/capacity
  tests passed (`build/memory-pressure-tests.log`; its initial warmer history test
  exposed equal-timestamp eviction, fixed with ordered bounded history before the
  full successful run). Race instrumentation is unavailable without CGO/compiler;
  the installed WSL distribution also failed startup. No race pass claimed.
- Real model quality and sustained memory behaviour across supported hardware
  remain separate qualification; these checks do not establish a hard total-PC
  memory limit. No installed model/context preference was silently changed.
