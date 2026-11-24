# Managed Windows Ollama: implementation and qualification boundary

Date: 2026-11-20. Change: `inference-provider-runtimes`, partial task 8.4.
Task 8.4 and real-runtime task 8.5 remain open.

## Implemented boundary

- Optional `managed_ollama` enrollment reaches the existing model selector,
  provider registry, public model list and shared scheduler. Enrollment and every
  HTTP request recheck the exact pins and current actor permission. Listing is
  metadata-only and never marks installed/runtime-ready/smoke-tested/qualified.
- Explicit Windows, single-user-host and unauthenticated-loopback consent;
  exactly one actor and no inherited upstream credentials. Other platforms,
  unowned localhost URLs and local vLLM fail closed.
- Preprovisioned account-private separate runtime/model/state directories;
  complete runtime inventory; legacy GGUF manifest/config/layer SHA-256 checks;
  bounded metadata, file/tree enumeration and context-aware hashing; retained
  Windows read handles deny replacement/writes. No download/import/update.
- Exclusive state lock; restricted HOME/cache/temp/PATH and cloud-off settings;
  suspended process start assigned to a per-runtime kill-on-close Job Object.
- Before HTTP writes, exact established IPv4 TCP connection tuples are mapped
  to a retained process identity and verified against the same job. Occupied or
  replaced unowned listeners receive no HTTP. A random port is not identity proof.
- Failed readiness, generation failure, Stop and scheduler switching retire the
  full tree before successful capacity release. Uncertainty retains pinned-file
  leases even when startup never returns a provider; service exit remains the
  eventual job/file-handle cleanup boundary.

## Executed checks

All runtime tests use a newly built isolated Go HTTP/process fixture in temporary
private directories. No Ollama installation, model weights or personal state was
used. Fixture bytes are deliberately not a model and cannot qualify inference.

- `go test ./internal/inference -run '^TestManagedOllama' -count=1` passed
  initially in 29.512 s, then passed with additional regressions in 39.542 s.
  Cases cover parent/child exit, held-file write denial/release, restricted
  environment, occupied and replaced listeners before HTTP, corrupt/missing
  pins, unpinned runtime files, missing model, absent consent, post-verification
  revocation, failed readiness cleanup, concurrent state ownership, cancellation
  and shared-scheduler retirement before llama admission.
- Initial implementation compile check exposed a transitive import cycle when
  reusing `execution.CheckPrivatePath`; that dependency was removed and replaced
  with a local read-only Windows ACL check. Root/sibling checks observing the
  incomplete multi-file edit also failed compilation; these are preserved as
  concurrent-edit failures, not successful test runs.
- The subsequent inference compile check passed (0.686 s). Existing configured
  endpoint tests passed (1.133 s) after service enrollment wiring.
- Source-size ratchet passed. Later focused regression and structure results
  are recorded below when completed.
- The new service missing-runtime regression initially failed (server 1.067 s):
  the HTTP factory returned a typed nil pointer inside a non-nil interface on
  startup failure. The scheduler could then call `Close` on it. The factory now
  returns a true nil interface on errors; a regression drives both direct factory
  and scheduler failure paths without weakening the original service assertion.
- The new uncertain-start regression then reproduced another existing gap:
  a factory returning no runtime plus `errProviderStopUncertain` allowed a later
  provider factory to run (failed in 0.403 s). The shared scheduler now records
  that unresolved resource release even without a returned runtime. The same
  regression proves both another provider and legacy allocation stay blocked.
- The corrected factory/uncertainty regressions passed together (0.489 s).
  Managed/configured service enrollment tests passed after correction (0.659 s).
  HTTP/provider-process regressions passed (7.392 s) before the final factory and
  uncertainty fixes; those fixes have their focused checks above.
- Review identified that a blocked output callback could prevent owned-tree
  termination after cancellation. Managed cancellation now starts retirement
  independently of that consumer. If its ten-second wait expires, Close still
  attempts tree termination and preserves `errProviderStopUncertain` even after
  proven process exit. The live blocked-callback regression observes the actual
  descendant exit before unblocking the callback, and checks the retained error.
  It and both startup-failure regressions passed together (18.024 s).
- Maintained docs passed (446 files, zero problems), source-size ratchet passed,
  strict change validation passed, and diff whitespace checks passed with only
  pre-existing line-ending conversion warnings.
- Final affected Windows focused inference run passed (45.498 s), including
  managed startup/Stop, blocked callback, HTTP adapters, process ownership and
  scheduler uncertainty. Linux HTTP race passed (6.185 s), and ProcessProvider
  race passed (2.649 s). The subsequent scheduler-uncertainty race command hit
  an incomplete concurrent storage type-file edit (`BackupRetainedError` was
  undefined); its failed compile is retained. After that type file became stable,
  the unchanged scheduler-uncertainty race check passed (1.025 s).

## Real qualification procedure: not executed

Use an isolated Windows account/VM and an explicitly approved offline runtime
and legacy GGUF model. Provision private dedicated directories and record the
OS/build, CPU/GPU/driver, provider version, runtime file hashes, manifest/blob
hashes, context setting, OffGrid source revision and enrollment identity. Follow
the concrete enrollment format in `docs/advanced/http-inference-endpoints.md`.

1. Start the configured OffGrid service with isolated user/model/database state.
   List models and verify the enrolled identity is declared only. Record actual
   startup time and failures; do not infer readiness from configuration.
2. Use `/v1/chat/completions` with that explicit `endpoint:<id>@<revision>` for
   streaming and nonstreaming fixed prompts. Record outputs, input/output usage,
   first-token/total latency, context and output bounds, runtime/model identities
   and peak/steady RAM, commit and GPU allocations.
3. Cancel during load and generation; record the runtime/runner PIDs, completion
   times and post-exit memory over a measured interval. Switch to llama and back,
   confirm no surviving former runtime descendants and no overlapping allocation
   claimed as freed. Repeat with forced runtime and service exits.
4. Change an enrollment pin, revoke its actor, replace a model blob in a stopped
   test instance, and occupy the chosen port with an unrelated fixture. Each
   must fail without replay, fallback, prompting the wrong listener or modifying
   the preprovisioned runtime/model store.
5. On the isolated VM with explicitly approved network monitoring/blocking,
   repeat generation with outbound networking denied. Preserve packet/event
   evidence and successful local outputs; environment variables alone do not
   demonstrate offline operation. Check for all unexpected download/update/cloud
   attempts and record failures rather than waiving them.
6. Run the frozen workflow suite three times per advertised hardware/model
   profile, record quality failures, performance, soak behavior and uncertainty.
   Tool workflows remain disabled until their separate routing and qualification
   implementation is complete. Supply evidence to the master tracker; do not
   check off this change from fixture results.

## Remaining requirements and limits

Stock Ollama's loopback API is unauthenticated. Verified server identity does not
authenticate other local clients, prevent their model-control requests or prove
exclusive whole-host network access. This increment is confined to explicitly
acknowledged trusted single-user hosts. Authenticated multi-user runtime isolation
requires additional private transport or OS isolation. The service does not
claim protection from arbitrary hostile code running as its own Windows account.

Local vLLM, POSIX/macOS descendant/listener containment, additional Ollama model
formats, measured resource telemetry/admission, rendered endpoint administration,
qualified tool routing and all real model/hardware/offline/soak workflow evidence
remain unfinished. No completion or production-readiness claim is made.

## Primary-source basis

- [Ollama configuration and cloud disablement](https://docs.ollama.com/faq)
- [Ollama local API authentication](https://github.com/ollama/ollama/blob/main/docs/api/authentication.mdx)
- [Ollama environment configuration](https://github.com/ollama/ollama/blob/main/envconfig/config.go)
- [Ollama manifest formats](https://github.com/ollama/ollama/blob/main/manifest/manifest.go)
- [Ollama manifest paths](https://github.com/ollama/ollama/blob/main/manifest/paths.go)
- [Ollama layer identities](https://github.com/ollama/ollama/blob/main/manifest/layer.go)
- [Windows TCP owner table](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable)
- [Windows TCP owner rows](https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcptable_owner_pid)

The manifest source now includes list/v2 layouts. Rejecting those here is an
explicit missing adapter, not an assertion that all Ollama models use legacy GGUF.
