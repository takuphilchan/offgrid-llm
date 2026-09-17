# OffGrid LLM v0.4.4

OffGrid LLM v0.4.4 makes agent work visible while it runs, adds practical chat
and task-history controls, and strengthens workspace ownership, backups, CLI
failures, and desktop/backend compatibility. Web and Electron use the same UI.

## Live agents and manageable history

- Agent runs expose queued, model-loading, processing, generating, tool, and
  approval phases, with a bounded, persisted public response preview. Reconnecting
  views recover the saved snapshot without submitting the task again.
- Task setup and results have independent, bounded scrolling. Growing output
  does not displace the Run task controls or force readers back to the bottom.
- Chat history supports search, refresh, individual deletion, and confirmed
  deletion of the filtered history. Agent history supports search, loading older
  runs, copying results, and reusing prompts without silently replacing a draft.
- Owners can delete terminal completed, failed, and cancelled task history.
  Active work, pending approvals, and interrupted or uncertain tool outcomes stay
  protected until resolved. Durable deletion markers prevent old activity events
  from bringing deleted runs back after restart.
- Truncated model output is not treated as successful agent completion or a
  complete tool invocation. Response previews do not expose private reasoning or
  tool arguments.
- Controls are translated into the nine existing interface languages; independent
  speaker review remains pending. Composition input and blocked browser storage
  receive safer handling.

History deletion is not secure erasure and has no undo. Audit records, events,
artifacts, files created by tools, and backups may remain.

## Workspace and runtime reliability

- An exclusive data-directory lock prevents two current OffGrid services from
  writing one workspace. Shutdown drains workers before releasing ownership.
- SQLite connections consistently enable foreign keys and durability settings.
  The pinned SQLite implementation includes the applicable WAL fixes.
- New offline `offgrid workspace backup`, `verify`, and `restore` commands verify
  archive contents and digests. Backup requires a stopped workspace; restore
  uses a new destination and checks application-version compatibility.
- Inference admission has a bounded FIFO queue, defaults to one active request,
  and returns an explicit busy response when full. Slow progress subscribers do
  not block task execution. Corrupt persistent event logs fail visibly.

## Consistent clients

- Converted knowledge, conversation, export, and agent lifecycle CLI commands use
  the authenticated service rather than silently falling back to direct files.
  They share request deadlines, cancellation, structured failures, and meaningful
  exit statuses. Progress is separate from machine-readable output.
- `/api/v2/system` identifies the product, API, application revision, and UI build.
  Desktop checks compatibility before attaching to a backend and does not stop
  an externally managed service. IPC sender and navigation checks are stricter.
- Settings and recovery screens expose the active backend and actionable failure
  states. This does not complete the planned v2 conversations/jobs API migration.

## Upgrade and recovery

Back up the complete stopped data directory before upgrading. Preserve models,
external configuration, and credentials separately where they live outside that
directory. Keep the previous application/image with its matching backup. Never
start old and new services on the same writable data simultaneously: older
versions do not enforce the new ownership lock.

Older binaries do not understand the new task-history deletion markers. Do not
downgrade against a workspace modified by this version; restore a matched
application/data backup instead. A restored backup reflects its capture time,
not changes made afterward. The staged transactional workspace migration and
generated conversation IDs are not part of this release.

With authentication enabled, legacy unowned conversations still require
administrative access. Knowledge remains shared among authorized RAG users;
private collection isolation is not yet delivered. See the
[client contracts](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.4/docs/advanced/client-contracts.md)
and [readiness plan](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.4/docs/advanced/production-readiness.md)
for exact coverage and outstanding work.

## Downloads and containers

Release assets retain Windows x64, macOS Intel/Apple Silicon, Linux x64 desktop,
and Linux x64/ARM64 CLI editions and their existing CPU, Vulkan, or Metal variants.
Not every accelerator is available on every OS. Desktop applications remain
unsigned/not notarized; packaged-platform qualification is still in progress.

Download the matching asset and `checksums-v0.4.4.sha256`. On Linux:

```bash
sha256sum --ignore-missing -c checksums-v0.4.4.sha256
```

For a new CPU-container installation:

```bash
docker pull takuphilchan/offgrid-llm:0.4.4
docker run -d \
  --name offgrid \
  --init \
  --restart unless-stopped \
  --security-opt no-new-privileges=true \
  --cap-drop ALL \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-models:/var/lib/offgrid/models \
  -v offgrid-data:/var/lib/offgrid/data \
  takuphilchan/offgrid-llm:0.4.4
```

Open <http://127.0.0.1:11611/ui/> once healthy. Models are downloaded separately.
An existing container named `offgrid` requires a separate, backed-up replacement
that preserves its configuration and volumes; pulling alone does not update it.
Keep the loopback binding unless authentication and trusted TLS are configured.

CPU images support Linux AMD64 and ARM64. NVIDIA users use
`takuphilchan/offgrid-llm:0.4.4-gpu` with `--gpus all` on Linux AMD64 and working
NVIDIA container passthrough. See the
[Docker guide](https://github.com/takuphilchan/offgrid-llm/blob/v0.4.4/docs/setup/docker.md).

## Validation and remaining limits

Pre-release source checks passed Go tests on Windows, targeted Linux race tests,
CLI tests, API/type checks, production UI builds, 39 Edge browser tests, and six
desktop compatibility/security tests. Browser coverage combines real service
persistence/history checks with controlled model/task fixtures. Local isolated
container checks and a backed-up deployment verified health and GPU visibility;
these are not cross-hardware inference benchmarks or installed-desktop tests.

This is an incremental reliability release, not certification of the complete
production-readiness plan. Durable chat submission/replay, private collections,
full CLI convergence, signed updates, broader hardware qualification, independent
security review, soak testing, and the user pilot remain outstanding. OffGrid
approvals do not govern tools run independently by external agents.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.3...v0.4.4)
