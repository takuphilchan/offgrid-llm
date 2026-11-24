# Enrolled Ollama and vLLM endpoints

OffGrid can route an explicitly selected remote text model through its existing
inference scheduler to an Ollama or vLLM server. Windows and restricted Linux installations support an
explicitly enrolled, owned Ollama process. These are opt-in development
capabilities. No endpoint is discovered automatically, no weights are downloaded,
and failure does not fall back to another server or local model.

## Enrollment

Set `inference_endpoints_file` in the service configuration, or set
`OFFGRID_INFERENCE_ENDPOINTS_FILE`, to an absolute path to a service-owner-managed
JSON file. Restart OffGrid after adding an endpoint or changing its identity.
Then open **Models → Inference endpoints** as an administrator, review the
destination and configured users, and choose **Review and enable**. Existing
endpoint installations migrating from the earlier file-only preview also need
this explicit review; an absent policy never means approval.
Removing or changing an existing enrollment revokes new requests immediately;
each outbound HTTP request rechecks the file. An already dispatched generation
may already have sent its prompt; use Stop to cancel it.

The endpoint panel has its own controls. It never sends endpoint identities to
legacy model-file verification or deletion. **Check connection** performs pinned
runtime/model metadata checks through the existing scheduler, without generating
a prompt. It can start an explicitly configured managed local runtime. Success
does not qualify model quality, tools, offline operation or hardware performance.
An administrator who is not also an enrolled chat actor cannot run the check.

**Disable access** saves a new workspace policy revision, rejects later request
admission and cancels admitted HTTP/generation contexts. A request which selected
the endpoint before waiting for capacity must recheck authority before dispatch.
The remote server may already have received a prompt and may continue its work;
disabling neither undoes those effects nor proves remote memory was released.
Changing the owner file rejects new requests immediately but does not itself
cancel already dispatched requests. Removing a configured credential also fails
new requests closed. The panel exposes credential availability, never its value
or environment-variable name.

Workspace decisions live in `inference-endpoint-policy.json`, with the exact
configuration digest, revision, reviewing actor and time. They can only subtract
from the owner file's actor and provider authority. The application holds its
existing OS workspace ownership lock before installing these routes and through
drain/cleanup; concurrent services cannot write the same policy. This is a
cooperative single-writer boundary, not protection from arbitrary same-account
or privileged filesystem tampering. Unreadable/corrupt policy blocks endpoint
requests. A failed snapshot publication blocks this service's endpoint authority
and cancels admitted requests even if rename may have occurred; restart and
review the actual stored state rather than assuming the attempted change failed
before publication. Preserve damaged policy for diagnosis instead of silently
resetting it.

The first explicit review creates a private, durable, nonempty workspace
activation epoch before saving the policy. Status reads create nothing. An
existing policy with a missing epoch fails closed; absence cannot be mistaken
for an original empty epoch and reactivate restored grants.

Restore authenticates and verifies historical archive bytes, then writes a new
32-byte random restore epoch encoded as 64 lowercase hexadecimal characters in
the private stage before publication. Archived endpoint approvals no longer
match that epoch and need new administrator review. The returned restore manifest
continues to describe the verified archive; it does not claim its historical
epoch digest describes the newly written marker. The next backup inventories
the actual marker and policy bytes. A malformed marker blocks activation of
grants; a malformed historical marker is replaced only after archive verification.

Administrators can now open **Models → Configure endpoints** to create or update
owner entries. Review the exact destination, IP addresses, model digest, runtime
version, users and approved credential reference, then save. Saving revokes
access; it does not probe the provider. Choose **Reconcile recorded outcome**,
restart the service, then use **Review and enable** for the new revision.
Updates require a new revision. With no external path configured, publication
creates the workspace's `inference-endpoints.json`; an external file stays at its
explicit path and requires installation delegation plus verified private storage.

If a save response is lost, inspect the request in **Configuration history** and
reconcile it. Do not repeat the save. Reconciliation compares actual stored
bytes with the recorded intent; it never reapplies configuration or grants
access. A file conflict needs owner recovery. Preserve the file and journal;
do not delete recovery records. The configuration panel remains discoverable
while unresolved publication blocks the ordinary endpoint list.

Installation-approved credentials and local templates come from
`inference_endpoint_owner_policy_file` / `OFFGRID_INFERENCE_ENDPOINT_OWNER_POLICY_FILE`.
Credential entries bind an ID and environment name to exact HTTPS origins;
the UI sees only the ID and origins. Managed templates contain the complete
owner-provisioned runtime entry; forms cannot supply executable paths, arguments
or environment variables. The policy format is version 1, with `credentials`
(`id`, `environment`, `origins`), `managed_templates` (`id`, `entry`) and optional
`editable_file` for a delegated external owner file. Installation policy is
never writable through this API. Missing credentials are not downloaded or
invented, and an existing credential cannot silently disappear during editing.

This is a verified development increment, not completion of endpoint
administration or provider qualification. Journal/decision capacity maintenance,
damaged-state recovery and installed-platform qualification remain open; see
the [owner configuration evidence](../quality/evidence/endpoint-owner-configuration-2026-11-21.md).

```json
{
  "version": 1,
  "endpoints": [
    {
      "id": "lab",
      "revision": "r1",
      "provider": "ollama",
      "url": "https://inference.example.test:8443",
      "allowed_ips": ["192.0.2.9"],
      "actors": ["local-admin"],
      "credential_env": "OFFGRID_LAB_TOKEN",
      "model": "your-installed-model:tag",
      "model_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "format": "gguf",
      "runtime_version": "replace-with-exact-server-version",
      "context_length": 8192,
      "deployment_attested": true
    }
  ]
}
```

The example address and digest are placeholders. Supply the actual deployment's
values and explicitly authorized OffGrid actor IDs. `local-admin` applies when
service authentication is disabled. Wildcard actors are rejected. Store the
upstream bearer credential in the named environment variable, not the JSON file.
Credential contents and origin URLs are not included in model descriptors.

For vLLM, use `provider: "vllm"`, the actual model format, the served model name
and a digest established from the deployed weights. `deployment_attested` is an
operator assertion about that specific deployment, not a workflow qualification
or cryptographic proof from `/v1/models`. vLLM's model-list API reports served
names, not weight digests. Use immutable deployments; a model name alone cannot
prove that a running server has not replaced its weights between requests.
The attestation also asserts that the endpoint executes on a separate resource
host. OffGrid rejects its own current interface addresses at enrollment and dial,
but client addressing alone cannot prove the placement behind NAT or forwarding.

Enrollment pins the exact origin and dial IP addresses. Remote endpoints require
HTTPS with ordinary certificate/hostname verification. Redirects, URL credentials,
environment proxies and link-local/loopback remote destinations are rejected.
Private remote addresses may be explicitly enrolled for a controlled LAN.
Ollama aliases reporting another upstream host/model are rejected, including
cloud aliases exposed through a local Ollama API.

## Selection and execution

Use `endpoint:lab@r1` as the model in the existing `/v1/chat/completions` request.
Both streaming and nonstreaming text requests use the shared provider queue.
Authorized enrollments appear in `/v1/models` and model refresh responses with
`backend: "remote"` or `"local-ollama"`; listing does not contact a provider or certify readiness.
Existing text-only agent profiles can select the same identity. Endpoint models
are excluded from automatic installed-local-model selection.

The `endpoint:` prefix is reserved for enrolled providers. A legacy local model
record with the same literal ID cannot override endpoint permissions or become a
fallback when enrollment is unavailable. It is excluded from the public model
menu; its registry record and files remain intact. Local-only preparation,
completion, embeddings, computer checks and model deletion reject this prefix.
Endpoint context counts do not reuse a local llama tokenizer; where exact counts
are unavailable, agent budgeting retains its conservative estimate and headroom.

Runtime version and model availability are checked before generation. Ollama's
reported model digest must match enrollment. Output must include valid terminal
status and token usage; partial text is provisional. Truncated streams, invalid
JSON, changed identities, unsupported controls and over-budget output fail
without replay. Stop cancels the particular HTTP request; it does not kill the
remote server or claim that its model memory was unloaded.

## Owned Ollama on Windows

This development path requires a trusted single-user host. **Ollama's local API
has no authentication.** Other local clients can reach its daemon and control
endpoints. OffGrid verifies that its established connection belongs to the exact
owned runtime before sending HTTP, but that does not authenticate other clients
to Ollama. This mode does not satisfy authenticated multi-user runtime isolation.

Provision separate existing private directories for a trusted offline Ollama
runtime, one legacy GGUF model store, and runtime state. Their Windows ACLs may
grant access only to the current account, SYSTEM and administrators. OffGrid
checks permissions and does not silently change them. Files must be regular,
without symbolic links, junctions or hard links. Directory roots cannot overlap.
The service has no install, pull, import or upgrade action on this path.

Extend an endpoint entry with this structure, use `url: "http://127.0.0.1:<port>"`,
`allowed_ips: ["127.0.0.1"]`, exactly one actor, no `credential_env`, and
`provider: "ollama"`. Keep the exact runtime version, model manifest digest,
`format: "gguf"`, context limit and `deployment_attested: true` fields. Here the
deployment attestation concerns the explicitly provisioned local runtime; it
does not assert a separate remote host.

```json
{
  "managed_ollama": {
    "runtime_directory": "D:\\OffGridRuntime\\ollama",
    "executable": "ollama.exe",
    "runtime_files": {
      "ollama.exe": "sha256:replace-with-actual-64-hex-digest"
    },
    "model_directory": "D:\\OffGridRuntime\\models",
    "state_directory": "D:\\OffGridRuntime\\state",
    "trusted_single_user_host": true,
    "allow_unauthenticated_loopback": true
  }
}
```

Replace the example paths and digest. `runtime_files` must enumerate **every**
regular runtime file, including bundled DLLs and runner binaries, using relative
paths with `/` separators and SHA-256 digests. Missing or additional files reject
startup. The limit is 256 runtime files, 1,024 tree entries, 32 directory levels
and 4 GiB per runtime file. File enrollment itself is limited to 64 KiB.

The first supported model layout is
`manifests/registry.ollama.ai/library/<name>/<tag>` plus its referenced
`blobs/sha256-<digest>` files. Use an explicit simple `<name>:<tag>` model name.
The manifest's SHA-256 is `model_digest`. Its config and every referenced layer
are verified from local bytes before launch, with bounded counts and sizes.
The dedicated store must contain only this manifest and its referenced blobs;
multiple models, manifest lists, manifest-v2, Safetensors and multimodal layers
are unavailable on this path. They are additional implementation work, not
implicitly accepted as equivalent formats.

Startup retains read leases on the pinned files and an exclusive private state
lock, then starts the process suspended, assigns a separate Windows Job Object,
and resumes it. The runtime receives a dedicated HOME/cache/temp location and
a restricted environment with cloud routing disabled and one loaded model and
parallel generation. Operator credentials and environment proxy settings are
not inherited. The provider version and served model digest must also match.

The scheduler owns this runtime like other local providers. Stop or a managed
generation failure retires its complete process tree before that call settles;
switching to another local provider likewise waits for confirmed retirement.
An uncertain stop retains file leases and blocks capacity reuse. A service crash
closes the job and terminates its descendants. No direct-child-only kill is
reported as a complete unload. An unrelated process occupying or replacing the
port is rejected before it receives an HTTP request.

## Restricted owned Ollama on Linux

Linux uses the same explicit endpoint entry and single-user/unauthenticated
loopback acknowledgements. It requires a non-root service with no permitted,
effective, inheritable or ambient process capabilities, working unprivileged
user/PID/mount namespaces, procfs process/socket visibility and a reviewed
util-linux `unshare` executable. Use an ordinary Linux executable name and add
`namespace_launcher` (canonical absolute path) and `namespace_launcher_sha256`
(complete `sha256:` digest) to `managed_ollama`.

Host policy, including AppArmor, must permit these namespace operations. If it
does not, the provider remains unavailable; OffGrid does not change host policy
or fall back to an unsupervised process. A host administrator must explicitly
provision the required isolation facilities.

An administrator must provision the runtime and single-model store outside
user-writable directories. Every artifact and ancestor must be root-owned with
no group/other writes, symlinks or regular-file hardlinks. The separate state
directory must belong to the service account with no group/other access; its
parent and ancestors must meet the same root-owned, non-writable rule. For
example, a dedicated root-owned directory under `/opt` can hold separate
`runtime`, `models` and service-owned `state` children. OffGrid checks these
conditions and never installs files or changes ownership, permissions, mounts,
cgroups or host sysctls to make an enrollment work.

This restricted mode trusts privileged administrators and the base OS loader
and libraries. Root ownership protects pinned runtime/model bytes from the
service account; a read-only file descriptor or advisory lock alone would not.
The state flock coordinates cooperating OffGrid instances and cannot prevent
hostile same-account code deleting its own lock. The state lock file remains
after clean shutdown so normal reacquisition continues using the same inode.

Startup creates a separate user/PID namespace with its own proc mount. Before
HTTP writes, the exact established TCP tuple and socket inode must belong to a
live process in that namespace. Stop waits for namespace-init teardown and its
monitor's reap. Identity enumeration is bounded; unavailable or excessive procfs
data fails closed. This is process lifecycle containment, not a filesystem or
network sandbox, and it does not authenticate other local Ollama clients.

The Linux factory/service routing, namespace and listener boundaries have source
and isolated subprocess tests. Full administrator-provisioned factory success
and actual Ollama/model qualification have not been run; see the
[Linux evidence and runnable fixture procedure](../quality/evidence/provider-linux-namespace-2026-11-20.md).

## Current limits

In Models, open **Configure endpoints** to review **Archive settled receipts**
or **Retire obsolete decisions**. The dialog lists the exact affected records;
cancel to leave them unchanged. After a failed response, inspect refreshed state
before reviewing another action. **Archived receipts** and **Policy history**
show retained evidence without granting access. **Show more** reads the next
page; **Refresh** restarts browsing after history changes. **Export this page**
downloads the displayed records and their provenance as JSON. It does not export
the complete workspace or enable a historical grant. Preserve whole workspace
backups for complete recovery. See [history evidence and limits](../quality/evidence/endpoint-history-2026-11-22.md).

**Workspace recovery** remains reachable if endpoint configuration or its
journal is damaged. Preserve the original directory and verify a matching-version
backup, then use the [existing workspace recovery procedure](workspace-recovery.md)
to restore to a new folder. Restored access requires fresh review; duplicate
receipts do not reactivate grants. Do not delete damaged metadata to reset it or
merge an older backup into newer data. Explicitly external owner files need their
own retained copy. If no usable backup exists, preserve the original evidence
for investigation. See [damaged-state recovery checks and remaining gates](../quality/evidence/endpoint-damaged-recovery-2026-11-22.md).

Administrators can also explicitly archive settled owner-publication receipts through
`POST /api/v2/inference-endpoints/configuration/archive`. First read the owner
configuration and send its `file_revision` as `expected_revision` and
`journal_revision` as `expected_journal_revision`. Only reconciled `committed`
and `not_applied` records are eligible. A failed or lost response requires a fresh
read; do not automatically repeat the request. Configuration and access policy
are unchanged, and duplicate publication requests still retrieve their original
receipts. Preserve the receipt archive with the workspace; do not manually delete
it to reclaim space. See
[archival evidence and limitations](../quality/evidence/endpoint-owner-archive-2026-11-22.md).

The owner response also reports `policy_maintenance` when the access-policy
store can be read. To retire obsolete decisions, send the reviewed `file_revision`
as `expected_revision`, `policy_maintenance.policy_revision` as
`expected_policy_revision`, and the exact selected `retirable` identities as
`endpoints` to `POST /api/v2/inference-endpoints/configuration/retire-decisions`.
Current, installed and active identities are protected. The complete previous
policy is archived before retirement; historical grants never enable access.
Inspect a fresh response after any failure or lost acknowledgement. If policy
publication is uncertain, restart and review actual state before further use.
Preserve policy history with the workspace. The 64 live-decision limit remains;
explicit retirement releases obsolete entries without deleting their audit.
Plain/encrypted backup and restore retain this history while requiring fresh
access review. See [maintenance evidence and remaining gates](../quality/evidence/endpoint-policy-maintenance-2026-11-22.md).

File enrollment enables remote text and the owned Ollama paths above. Tool-agent routing, structured-output
qualification, multimodal requests, complete endpoint administration and live
readiness/qualification workflows remain unfinished. The transport adapters
validate normalized tool responses in isolated contract tests, but that does not
enable tool authority in service text routes.

Externally managed local URLs are not admitted through the configuration file.
Owned local vLLM, macOS containment, broader user-managed Linux installations, additional Ollama model layouts and
authenticated local-daemon isolation remain unfinished. Closing a client
connection cannot safely replace process ownership. Windows fixture tests are
not real Ollama, model quality, memory reclamation or offline qualification.

Source and fixture validation is recorded in the
[HTTP provider evidence](../quality/evidence/http-inference-endpoints-2026-11-20.md).
The owned-process increment and runnable qualification procedure are recorded in
[managed Ollama evidence](../quality/evidence/managed-ollama-windows-2026-11-20.md).
Real provider/platform/workflow qualification remains open in
[`inference-provider-runtimes`](../../openspec/changes/inference-provider-runtimes/tasks.md).

Protocol references: [Ollama chat](https://docs.ollama.com/api/chat),
[Ollama model list](https://docs.ollama.com/api/tags),
[Ollama cloud routing](https://docs.ollama.com/cloud),
[Ollama response types](https://github.com/ollama/ollama/blob/main/api/types.go),
and [vLLM OpenAI-compatible serving](https://docs.vllm.ai/en/latest/serving/openai_compatible_server/).
