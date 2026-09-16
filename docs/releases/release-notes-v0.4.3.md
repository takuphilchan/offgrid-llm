# OffGrid LLM v0.4.3

OffGrid LLM v0.4.3 makes everyday chat more responsive and gives built-in agent
work a durable, inspectable lifecycle. The browser and Electron share these
improvements; the CLI exposes the same agent recovery controls.

## Streaming chat and useful feedback

- Saved conversations stream text by default, with visible queue, retrieval,
  model loading, prompt processing, generation, and saving phases.
- Stop cancels queued or active inference. Broken streams and partial answers
  are not presented as successfully saved responses. A completed turn is saved
  atomically before the completion event.
- Account-scoped drafts survive navigation, reloads, and failed sends. Rendering
  is batched, and automatic scrolling respects users reading earlier messages.
- Response settings separate an 8K interactive context from the configured
  extended context, bounded by the actual service limit. Output budgets are
  selectable; reaching a token limit is visible rather than looking like an
  unexplained interruption.
- Timings distinguish queueing, loading, first text, and generation. Token rates
  use backend token usage when available, not a count of stream fragments.
- The new controls are included in all nine existing interface languages.

## Durable agents and safer data access

- Built-in agent runs save checkpoints before tool side effects and restore
  their status, pending approvals, and committed results after navigation or
  restart.
- Approvals authorize one exact invocation, are bound to the actor and saved
  run, expire after 15 minutes, and cannot be reused for another call.
- Web and CLI controls inspect, approve, deny, cancel, resume, or reconcile the
  existing run. Unknown tool outcomes after interruption require review instead
  of automatic execution again.
- Session ownership is enforced across reads and writes when authentication is
  enabled. Creating an existing session no longer overwrites it.
- Knowledge-backed requests fail clearly if retrieval is unavailable or fails,
  or if no evidence is found; they do not silently fall back to an ungrounded
  answer. Failed persistent knowledge storage does not become temporary storage.

## Runtime and container reliability

- Fixed concurrent cache and request-queue lifecycle races.
- Coordinated model/context switches with active inference, propagated stream
  failures, and preserved child-process CUDA environment settings.
- Container Go binaries are portable, matching the native release approach.
- Fixed the missing OpenMP runtime dependency in the CUDA image and added a
  final-image shared-library check.
- CPU containers support Linux AMD64 and ARM64. The separate CUDA container
  supports Linux AMD64 and requires working NVIDIA container passthrough.

## Upgrade notes

Back up persistent data before upgrading and retain the previous image or
installer until verification is complete. Preserve existing model/data volumes;
do not start old and new servers against the same writable data simultaneously.
Pulling a new Docker image alone does not replace an existing container.

This release changes the built-in agent approval contract. Clients must
continue saved runs through their approve/deny/resume actions; resubmitting a
prompt with a list of preapproved tools is no longer accepted. See the
[agent guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.3/docs/guides/agents.md).

With authentication enabled, legacy conversations without an owner require
administrative `sessions:all` access. They are not automatically assigned to the
first user who requests them. Knowledge storage remains shared among authorized
RAG users, not isolated into per-tenant collections.

## Container quick start

For a new installation (an existing container named `offgrid` must be upgraded
separately, preserving its configuration and volumes):

```bash
docker pull takuphilchan/offgrid-llm:0.4.3

docker run -d \
  --name offgrid \
  --init \
  --restart unless-stopped \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:0.4.3
```

Open <http://127.0.0.1:11611/ui/> after the container becomes healthy. Models are
downloaded separately. Keep the loopback port binding unless authentication and
trusted TLS termination are configured.

For NVIDIA, use `takuphilchan/offgrid-llm:0.4.3-gpu` with `--gpus all` and the
same persistence/security options. See the
[Docker guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.3/docs/setup/docker.md).
Do not run CPU and GPU instances on the same port or writable data volume.

## Desktop and CLI downloads

Release assets include Linux x64 and ARM64 CLI bundles, Linux x64 desktop
packages, macOS Apple Silicon and Intel desktop/CLI packages, and Windows x64
desktop installer, portable desktop, and CLI packages. Desktop packages remain
unsigned; Windows SmartScreen and macOS Gatekeeper may require explicit approval.

Download the matching assets and `checksums-v0.4.3.sha256`. On Linux, verify
downloaded files with:

```bash
sha256sum --ignore-missing -c checksums-v0.4.3.sha256
```

## Validation and remaining boundaries

Source validation includes Go tests, race tests for state and request lifecycles,
OpenAPI/TypeScript checks, production UI builds, and 23 browser tests. Controlled
model/tool fixtures test failure and recovery paths. Local NVIDIA testing also
confirmed real streamed saved-chat inference at an 8K context; this is not a
hardware-independent benchmark or a guarantee of model quality.

Streaming improves feedback, not a model's inherent decoding speed. Model load,
context size, RAM/VRAM, and offload still matter. External agents retain their
configured context requirements; the interactive chat profile does not pretend
that a smaller context satisfies them.

Chat does not yet support reconnect/replay by durable request ID. If connection
loss occurs during final saving, reload the conversation before retrying.
Browser draft storage is local, not encrypted or synchronized. Durable tool
approvals govern OffGrid's built-in runner, not tools executed independently by
Hermes or OpenClaw. Broader hardware, external-agent task, and desktop platform
validation remains ongoing.

See the [performance guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.3/docs/advanced/PERFORMANCE.md)
and [full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.2...v0.4.3).
