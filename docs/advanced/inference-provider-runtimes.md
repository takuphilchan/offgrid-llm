# Inference provider runtimes

This is an in-progress provider contract, not an enabled multi-engine release.
Existing GGUF chat still uses the established llama.cpp service path. MLX and
Transformers text packages can be imported as metadata, but ordinary installs
cannot select them for Chat until trusted runtime installation and real-hardware
qualification are complete. A service-owned selector now has a gated path
through non-streaming Chat, session streams and OpenAI SSE, exercised only with
isolated fake providers. The experimental local workers share a typed request,
cancellation, streaming, and terminal-event protocol. Their source and protocol
fixtures live in `runtimes/python`, separate from the public `sdk/python`
client. Runtime activation still requires an explicitly pinned worker path and
reviewed digest; the source move does not create a fallback executable or
qualify MLX/Transformers hardware.

## Runtime states

These states are intentionally separate:

- **Installed** — model files and a manifest are present.
- **Runtime ready** — the declared provider, runner, adapter, and backend are
  available on this machine.
- **Smoke tested** — the worker completed an isolated protocol and generation
  check.
- **Qualified** — the provider passed the real hardware workflow suite for a
  specific model/runtime revision.

Installing files or passing a fake-worker test never implies qualification.
Vision and audio remain unavailable until their model-specific preprocessing,
projector, and tool-call checks pass.

## Provider selection

The intended selection is deterministic: an explicit profile/model provider wins, followed
by the provider declared by an installed model. OffGrid does not silently load
GGUF through MLX, convert safetensors through llama.cpp, download weights, or
fall back to a different provider after an uncertain external effect.

The provider scheduler has isolated tests for resident reuse, load
deduplication, cancellation, idle-resident eviction before a model switch,
waiting for active inference before switching, and bounded pre-dispatch replacement after a
worker health failure. A generation now pins its newly loaded runtime before
exposing it to a concurrent model switch, including the interval between load
and dispatch when multiple inference slots are configured. Existing
llama-backed Chat, agent and model-probe
acquisition shares its generation admission queue. Native text Chat dispatch
exists behind a service-owned selector, but no qualified native runtime is
configured in ordinary installations, and agent tool routing remains on the
llama path. Startup, health and terminal replies carry an observed backend/device
and distinguish measured memory from unknown memory. The native text worker
installation can bind separate qualification targets to each provider/backend
pair; an absent pair is denied rather than borrowing another provider's
hardware evidence. The older single-target internal call remains compatible.
This is a source-level route, not a bundled runtime or a real-device pass.
The native text worker
counts tokens after applying its pinned chat template and rejects requests
that exceed its qualified context. Both native engines now generate from the
exact token IDs admitted by that check. The worker emits typed token usage;
the service rejects inconsistent counts or counts beyond the admitted context.
When the caller omits `max_tokens`, the worker reserves up to 512 output tokens
from the context remaining after tokenization; an explicit limit is never
silently reduced. Reaching that limit returns `finish_reason: length` through
the worker, service response, and Chat stream rather than falsely reporting
normal completion. A terminal event without measured token usage is also
rejected. Cancellation and a missing terminal event still fail the request
instead of producing a saved answer.
The shared scheduler independently checks a native request's output-token
ceiling against reported usage and bounds cumulative streamed text bytes before
passing another delta to Chat. Those local budget fields are not worker
commands; a worker that omits usage or exceeds either bound cannot produce a
completed answer. This does not replace root/child aggregate budgeting.
These are protocol tests, not live native-model evidence. Provider-wide memory
admission is unfinished.
Until mixed-engine concurrency has hardware evidence, native generation is
exclusive of llama generation even when an installation explicitly configures
multiple llama slots. Those existing llama slots remain concurrent with each
other. Waiting requests do not occupy numeric generation slots, can be stopped
without launching a worker, and contribute their wait to queue timing.
Legacy llama admission now retires idle native workers before allowing the
existing llama model loader to run. Compatible llama residents are retained.
If a native worker cannot be retired, the scheduler rejects later admissions
and model switches instead of assuming its device memory was released. This is
an isolated scheduler guarantee, not a measured device-memory budget or a
complete migration of the llama binary lifecycle into the provider interface.
The llama provider adapter now preserves terminal `stop` versus `length` and
measured usage when the existing HTTP engine exposes structured SSE chunks.
It rejects an incomplete stream, ambiguous JSON, undeclared tool catalogs, and
tool-call deltas in text-only requests. Engines with only the older token
callback retain that compatibility path. This does not move binary ownership
or embeddings into the provider contract.
Embedding loads and generation now acquire the same admission gate as llama
text and native workers. Before a native worker opens, the service retires an
idle embedding child as well as its llama chat process. Knowledge retains the
verified embedding identity and reloads on its next request only if the new
runtime reports the exact same identity; an explicit unload never silently
reloads. The hot-swap HTTP path also takes a cancellable scheduler admission
lease before it can start a llama model process. That lease is exclusive even
when parallel llama generation slots are configured, so cache eviction cannot
close a model that is still generating. Isolated concurrency and
switch fixtures verify these boundaries, but the embedding child is still
owned by its existing engine and measured cross-provider memory admission is
not complete.
The scheduler now acquires the existing llama lifecycle lease and invokes its
model/context loader as one admission operation, recording load versus queue
time without changing the cache's binary or HTTP ownership. Ordinary Chat
uses these scheduler timings for its loading and queue display. This is an
intermediate compatibility boundary, not completion of provider-owned llama
process lifecycle or device-memory accounting.
Resident worker health and resource checks hold a scheduler pin until their
transport calls return. Model switches and shutdown wait for that pin before
retirement; a failed health check remains tracked until worker cleanup is
confirmed. A cleanup failure latches unresolved residency instead of allowing
another provider load or reporting a clean shutdown. This is a source-level
concurrency guarantee, not measured device-memory qualification.
At startup, the scheduler adopts the idle llama cache and engine through one
registration. Restart admission, residency reporting, and final process
retirement are installed together; an active or partly registered cache is
rejected. The cache still implements the GGUF process map behind this owner.
Service hot-swap and model deletion now enter the scheduler's exclusive
maintenance callback, which exposes only bounded cache/process mutations while
the lease is held. Isolated transport fixtures attach the same owner. The
attached llama process also requires a live scheduler lease for activation or
retirement; a retained callback context expires when that operation returns.
Missing owned handles fail closed instead of constructing another process
adapter. The configured model-load timeout begins at the load callback, after
queue admission, so waiting behind other inference does not consume the load
budget. Maintenance load and unload durations are reported separately from
queue time. The underlying cache still manages llama child processes and the
service retains registry/knowledge transaction ordering. This is not complete
process-owner consolidation or GPU/Metal memory qualification.
Its internal memory snapshot also counts cache-owned llama entries separately
as unmeasured residents. It never adds those entries to measured native-worker
bytes or reports their memory as free capacity; a real CPU/GPU memory limit for
the legacy cache still requires separate qualification.
The browser-vision smoke pass is also bound to a cache process generation as
well as the model/projector file identity. A llama child replacement invalidates
that in-memory pass even when it reuses the same model files or loopback port;
the synthetic probe must run again before capture is offered. This is not
installed-platform visual-grounding qualification.
The service's GGUF activation and native-switch retirement sequence now goes
through a narrow inference-owned llama process adapter. It preserves the
existing cache's context, binary, CPU/GPU and projector behavior and keeps the
engine port connection in the same order. A failed connection clears the
service's fast-path identity so a later request cannot silently use a changed
HTTP port under the old model name. Cancellation after cache load is checked
before the engine is connected or the model is reported active. This adapter
still shares the legacy cache;
normal GGUF Chat generation has not been moved to ProviderRuntime events.
If the cache monitor replaces a crashed llama child, the next admission
compares its process generation with the engine's last bound generation. It
reconnects before generation even if the replacement reused the same port.
The synthetic vision check has separate admission and perception deadlines:
queueing or cold model loading cannot consume its bounded image probe time.
The typed admission path attributes this retirement to unload time rather than
queue time; the older caller still receives total admission wait for compatibility.
Cancellation or scheduler shutdown during retirement does not hand out a
legacy generation lease after the worker closes.
The reverse switch is also gated: the service connects its existing llama
model cache and engine to native admission, retires loaded llama processes,
invalidates the old lifecycle identity, and updates registry load state before
opening MLX or Transformers. A failed retirement latches an unresolved state
instead of retrying a native load. This preserves the existing llama process
owner for GGUF compatibility; full scheduler ownership and real-device memory
qualification remain unfinished.
Waiting for a concurrent model operation's service lock is cancellation-aware;
Stop can abandon that wait without unloading a model after cancellation.
A safe worker switch now treats intentional process retirement as cleanup, not
as a crash. A complete, validated worker error leaves the resident process
available for a later explicit request; malformed or truncated output still
terminates it. A cancelled terminal is reported as cancellation, never as a
successful answer. Startup and health events must retain the expected protocol
identities. Exact-token dispatch avoids special-token drift after chat-template
admission. [MLX-LM's `stream_generate` contract](https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/generate.py)
accepts integer token IDs and exposes generation counts. A queued or loading
request is tracked before provider dispatch, so Stop can cancel it without
opening a new worker. A generating request also receives bounded provider
cancellation; its elapsed cancellation time is recorded.
A failed generation is never silently replayed after a crash, which is an
operational failure rather than evidence of model incompatibility.
The older public HTTP streaming path also reports a pre-output EOF as a
failure; it no longer unloads and replays that request outside scheduler
admission. No visible tokens is not evidence that the prior generation had no
effect on the runtime. That path requires one supported terminal choice before
it emits the public `[DONE]` marker; missing, duplicate, or post-terminal
choices end with an error instead of a false completion.

## Local worker requirements

The experimental MLX and Transformers workers accept a local model path and typed
JSON-lines messages only. They have no protocol command for credentials or
desktop control, but OS-level filesystem and network isolation is **not yet
implemented**. Transformers loads with
`local_files_only=True`, `trust_remote_code=False`, and
`use_safetensors=True`; a missing safetensors loader cannot silently fall
back to pickle weights. A Transformers producer that does not stop after its
bounded join poisons the worker, and the next health check requires supervisor
replacement rather than admitting overlapping generation. MLX and PyTorch packages
must already exist locally. The native route checks pinned interpreter and
worker-script digests, reviewed adapter and tokenizer-template identity, and
exact installed `mlx`/`mlx-lm`/`transformers` or
`torch`/`transformers`/`safetensors` distribution versions before dispatch.
Their sorted version-lock digest is part of the qualification fingerprint, so a
changed pin makes old evidence stale. This checks distribution versions, not
every byte in a Python environment; it does not replace a pinned environment
image or OS-level process isolation. The route also requires current
model/hardware-specific workflow evidence. The
native activation gate rejects a missing or mismatched template digest before
launching a worker; metadata-only import remains possible. The
worker has a filtered environment and Python isolated mode, but OS-level
filesystem/network sandboxing still needs implementation and verification. No
runtime package, model weights, or cloud fallback are installed automatically.
MLX admission requires an explicitly selected Metal backend and the local
worker checks Metal availability, selects the GPU before MLX-LM creates its
generation stream, verifies that the selected device remains the GPU, and
reports the detected device name in its handshake. Protocol fixtures cover
selection, reject CPU placement and reject a missing device identity;
they are not Apple Silicon qualification. Transformers admits
only CPU, CUDA, or MPS at this implementation stage. These checks reject
unsupported selections; they do not establish real-device qualification.

## Qualification evidence

Record implementation, automatic test, and real-hardware qualification as
separate evidence in the product reliability plan. CPU, CUDA, Metal, and MPS
claims require measurements on the corresponding hardware. Until those checks
are recorded, the Models page must show the provider as unqualified even when
the manifest is valid.

The frozen 30-case manifest has a read-only scorer for independently collected
three-run results: `node scripts/test/score-provider-workflows.mjs RESULTS.json
EVIDENCE_DIR`. It checks profile and suite identity, all 90 trial slots,
distinct run-evidence files, per-family outcomes, safety incidents, memory and
timing fields. Its output is always marked `requires_independent_review`:
the scorer does not run models, inspect evidence contents, or qualify a provider.
Native cases still need disposable environments and real-platform execution.

The results file has `schema_version: 1`, the frozen `suite_digest`, one pinned
`profile` (`engine`, `model_digest`, `runner_revision`, `adapter_revision`,
`backend`, `platform`, `device`), and exactly three `trials` per manifest case.
Each trial records `case_id`, `repetition` (1–3), evaluator, outcome, setup and
distinct run evidence paths, an isolated fixture reference for native cases,
every named check and forbidden effect with its own evidence reference, the
four incident counts, and load/queue/generation/total milliseconds plus peak
memory bytes. Paths must resolve to files under `EVIDENCE_DIR`. A human reviewer
must inspect those files and confirm the pinned profile and environment before
any qualification decision; reported booleans and file existence are not proof.
