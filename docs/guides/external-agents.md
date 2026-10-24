# External agent providers

OffGrid can act as the private model runtime for external agent systems. The
first supported integrations are Hermes Agent and OpenClaw. Each integration
registers a provider named `offgrid` in the external system and talks directly
to OffGrid's OpenAI-compatible `/v1` API.

These are first-party OffGrid provider plugins. Their provider identity,
configuration, discovery, readiness, and documentation are owned by OffGrid.
Run managed install commands on the host where the external agent should
live, even when OffGrid itself runs in Docker. If the CLI is only built locally,
use `./bin/offgrid` (or `.\bin\offgrid.exe` in PowerShell) in place of `offgrid`.

## Check readiness

Start OffGrid and install at least one chat model, then run:

```bash
offgrid integrations list
```

The readiness check verifies that the selected model exists and that the
running OffGrid service advertises enough effective context. It does not
measure the model's actual tool-use quality or throughput. Hermes
Agent currently requires at least 64,000 tokens and OffGrid recommends 65,536.
A small model may implement the wire-level tool protocol and still be poor at
selecting tools; test a disposable task under the external runtime's own tool
policy before making it the default. OffGrid supplies inference, not authority
over Hermes/OpenClaw tools.

The first-party provider hooks default to one native tool call per response, so
dependent operations can consume actual tool results before continuing. Explicit
parallel-call settings remain authoritative; this does not remove tools or change
the selected model. Update the trusted provider through managed setup to receive
new hooks. Source-built workers identify the native cardinality repair with
`-offgrid-seq1`; older installed workers are not rewritten automatically. This
structural repair cannot prove correct arguments or honest completion: the known
file-workflow failures remain a commit/release gate until independently retested.

## One managed lifecycle

### Experimental explicit file contracts

The shared [host verification core](../../internal/integrations/verification/contract.go)
checks explicit outcomes independently of assistant prose. Opt in for a single
foreground task with `--verify-contract <file.json>`. Ordinary runs are unchanged.
This is experimental: deterministic hook tests are not real-model qualification,
and earlier failed live workflows remain unresolved until rerun successfully.

For example, place this JSON **outside** a small disposable workspace:

```json
{"version":1,"kind":"exact_copy","workspace":"./workspace","source":"source.txt","destination":"copy.txt"}
```

The workspace is resolved relative to the contract file; source and destination
are slash-separated paths relative to that workspace. The source and destination's
parent must exist. An existing destination may be replaced only with exact source
bytes. For a greeting or other file-preserving task use
`{"version":1,"kind":"no_changes","workspace":"./workspace"}` instead.

```bash
offgrid openclaw run "Copy /absolute/workspace/source.txt to /absolute/workspace/copy.txt exactly. Read the source first, then read back the copy. Change nothing else." --verify-contract copy.json
offgrid hermes run "hello" --verify-contract unchanged.json
```

The CLI starts in the contract workspace, but reviewed file-tool proposals must
use **absolute local paths**: external tools may have their own working directory.
Remote/container file backends and interactive/resumed sessions are not supported
by these initial host-file adapters. Use a matching native path on Windows.

Contracted OpenClaw uses its supported `agent --local` path to initialize lifecycle
hooks; ordinary runs still use `agent exec`. It selects the configured default
agent (or the sole agent), uses a fresh session and private state directory, and
reads existing JSON5/config includes through a read-only overlay. Model selection,
tools and hook/sandbox permissions are inherited, not replaced. Bootstrap creation
is suppressed and agent/session paths are scoped to the invocation. Ambiguous
agent selection fails before launch. Private session state is retained at the
printed temporary path for inspection; it may contain transcripts, so treat it
as private. It is not a sandbox or a copy of personal auth/session stores. The
upstream runtime may warn that its isolated plugin provenance database has no
install record; OffGrid still requires the exact trusted bundle before launch.

The reviewed revisions are Hermes 0.21.2/f364c197 and OpenClaw
2026.10.4/383c124. They require matching first-party bundles; other revisions fail
preflight rather than silently claiming support. OpenClaw hooks are part of its
provider bundle. OpenClaw also requires an explicitly enabled `offgrid` plugin,
`plugins.entries.offgrid.hooks.allowConversationAccess: true`, and prompt hooks
not explicitly disabled. These lifecycle hooks can receive conversation data;
the verifier ignores that data and sends only bounded file-operation metadata
and proposed write bytes to the local CLI. Review the plugin before granting
access; OffGrid never changes these permissions automatically. Hermes requires
a distinct, explicitly installed/enabled plugin:

```bash
offgrid integrations install hermes-verifier
hermes plugins enable offgrid-verifier
```

Review this executable plugin before enabling it. Run never installs or enables
plugins. Hermes safe mode disables the hooks. No verifier endpoint, token or
contract is persisted in external-agent configuration. Credentials are consumed
by the hook process rather than inherited by its tool subprocesses.

Its initial contracts cover an exact source-to-destination copy or an unchanged
small workspace. Exact copy requires correlated complete source-read, write and
destination-read events plus independent byte comparison. The core rejects
incorrect proposals rather than rewriting them. Missing completions, changed
sources, unexpected files, cancelled runs and contradictory receipts cannot pass.
Original tool results and model/user messages are untouched. A blocked proposal
does not itself prove that an external runtime honored the block.

Checks are read-only and bounded to 256 entries, 1 MiB per file, 8 MiB total,
16 directory levels and 256 observed calls. Links and special files are rejected.
Only outcome codes/counts are returned, not file content. This is not a sandbox,
an atomic filesystem snapshot or proof that unobserved transient changes never
occurred. Filesystem mounts and hostile same-user processes still require host
isolation. A passing file contract does not certify arbitrary task quality.

Missing handshake, unhealthy/missing lifecycle closure, a lost completion or
failed final check makes the CLI exit unsuccessfully and print a narrow file
contract receipt. This does not rewrite the agent's already printed answer.
Unknown tools retain their existing authority: this is not general tool governance
or proof of absence of external/transient effects. Inspect failures before retrying.
No model qualification is saved from a passing contract.

The Hermes hook adds scoped instructions for interpreting its numbered display
and requiring contract read-back. An explicit read limit can count as complete
when Hermes confirms that it covers the entire file without truncation. If a
proposal contains an extra display prefix, the verifier can identify that prefix
only when the remaining bytes match the immutable source digest. The model must
submit the corrected write; the verifier never changes file contents itself.

Offline checks: `go test ./internal/integrations/... ./cmd/offgrid`,
`node --test scripts/test/test-verification-hooks.mjs`, and
`python -m unittest scripts/test/test_verification_hooks.py`. Optional installed
hook checks use `OFFGRID_HERMES_TEST_ROOT` / `OFFGRID_OPENCLAW_TEST_ROOT` with
`go test ./internal/integrations -run TestInstalledVerificationHooks -v` on Linux.
They use disposable homes and deterministic file operations, not live model replies.

The full managed-launch test is
`python scripts/test/test-verification-launch.py <source-built-cli> --agent openclaw`
(or `--agent hermes`). It checks actual installed CLI registration, greeting,
exact copying, false-completion rejection, cancellation and saved configuration
preservation against deterministic local inference. Add explicit
`--live --server http://127.0.0.1:11611 --model <installed-model-id>` only for an
authorized real-model greeting/copy test; it refuses to begin when the service is
busy. Real tests use file-only disposable configuration and independent read-back,
not personal settings or an OS sandbox. Inspect the private greeting text as well
as the file receipt; one success is not general model/workflow qualification.

### Commands

Both adapters share these commands (replace `<id>` with `hermes` or `openclaw`):

```bash
offgrid integrations configure <id> --model <installed-model-id>
offgrid integrations status <id>
offgrid integrations test <id>
offgrid integrations run <id> "your task" --model <installed-model-id>
```

`offgrid hermes install/status/test/run` and `offgrid openclaw
install/status/test/run` remain aliases. `configure --dry-run` inspects the plan
without installing or changing configuration. The lower-level `integrations
install` still copies only the provider bundle; `integrations setup` still
renders manual instructions. Neither replaces managed configuration.

Model precedence is explicit `--model`, the managed connection's saved selection,
the external runtime's configured selection, then the installed-model catalog.
Model options work before or after the task. `--model` on `run`, `test` or `status`
is temporary; `configure` records a new default. No model is downloaded or
silently substituted.

Readiness has distinct meanings:

- `integrations list` and the browser report **service/model compatibility**;
  they cannot inspect your host from a remote container.
- `status <id>` checks runtime version, trusted plugin, provider endpoint,
  selected model and context without running inference.
- `test <id>` verifies a model reply and saves fingerprinted smoke evidence.
- Tool workflows need independent result checks. A process exit, setup or
  smoke test does not qualify the model for agent work.

The safe context relationship is `minimum <= configured <= service allocation`.
Extra capacity does not require reinstalling. Power transitions and transient
memory/queue pressure do not rewrite the configured context. Actual insufficient
memory at a new model load returns `insufficient_runtime_capacity` without
silently selecting a smaller allocation. Free resources or explicitly select a
smaller context/model; do not reinstall the agent or declare an inflated context.
Resource pressure uses OS measurements to control admission, not Go heap occupancy.

Configuration is locked and snapshotted under
`~/.offgrid/integrations/state/<id>/recovery/` (override with
`OFFGRID_INTEGRATIONS_DIR`). Reports identify the recovery directory. These
private snapshots can contain credentials: do not publish them. A failed step
does not blindly restore over newer settings. Inspect the journal and current
configuration before retrying. Status never replays agent work. Connection and
smoke records are scoped to service, credential identity and host profile;
credentials themselves are excluded. Alias JSON retains previous field types
and adds `assessment`; generic command JSON exposes the assessment directly.

Fresh installations use a 65,536-token operator ceiling with adaptive planning.
To request that ceiling explicitly while retaining hardware/model safeguards:

```bash
export OFFGRID_MAX_CONTEXT=65536
export OFFGRID_ADAPTIVE_CONTEXT=true
offgrid serve
```

Large contexts consume substantially more RAM or VRAM. Adaptive mode reads the
selected model's GGUF context/KV metadata and a startup hardware snapshot, then
plans a stable tier no larger than the ceiling. New model loads recheck available
memory after eviction (including exposed Linux container limits), and confirm
the worker's actual context before generation. A healthy larger allocation is
reused for smaller requests rather than unloaded and shrunk. Existing deployments that set
`OFFGRID_MAX_CONTEXT=8192` remain capped at 8K until the operator raises it.
Power telemetry is informational by default, including low/critical battery.
It neither blocks integrations nor shrinks context. Unknown power stays unknown;
it is not treated as evidence that the laptop is unplugged. Model/memory limits
still apply. Older builds could impose 4,096/2,048-token caps through battery or
degradation policy, including while on AC power.
Use `OFFGRID_ADAPTIVE_CONTEXT=false` only for a measured fixed allocation. The
readiness endpoint reports the actual planned value and remains honest about
whether the selected model is ready.

Automatic GPU placement accounts for split CPU/GPU allocations, with reserves
in both memory pools. After eviction, admission briefly rechecks memory while
the driver releases allocations; this never replays generation or tool work.
Keep the default `q8_0` KV cache for external-agent testing. A smaller cache
format can impair output even when model loading succeeds.

An `invalid_model_tool_response` means the model's proposed call failed the
runtime parser or schema validation. No call from that response was released;
earlier tool effects remain separate. Reinstallation does not repair a malformed
response. Smoke tests establish connectivity only; verify a disposable file task
with independent read-back before trusting the chosen model for agent work.

## Hermes Agent

The recommended path is one command:

```bash
offgrid hermes install
```

If Hermes is missing, OffGrid asks before downloading and running Nous
Research's official installer. It then installs the embedded OffGrid provider,
persists the endpoint, model, provider and context through Hermes'
configuration CLI and the token in its private `.env` without putting it in
process arguments. No YAML editing or shell exports
are required. For unattended installs, explicitly accept the external
installer and optionally choose a model:

```bash
offgrid hermes install --yes --model <model-id>
```

The default installs only the Hermes core: it does not wait on npm, Chromium,
or a full diagnostic probe of unrelated services. To add browser/computer-use
dependencies explicitly, or run full Hermes diagnostics, use:

```bash
offgrid hermes install --with-browser
offgrid hermes doctor
```

The optional browser stage can take several minutes. Its failure is reported
as a failed optional stage with core settings preserved; rerun with
`--with-browser` to repair it. `offgrid hermes test` checks for a real model
reply and exits nonzero if Hermes cannot reach OffGrid. If the service has less
than 64,000 effective context tokens, the installation stops before starting
the external installer; only raise that limit when the hardware can support it.

Check and use it with:

```bash
offgrid hermes status
offgrid hermes test
offgrid hermes
offgrid hermes -q "Summarize this directory"
```

`offgrid hermes` always opens Hermes chat through the `offgrid` provider.
On Unix, interactive launches hand the foreground terminal to the child and
restore it on exit. Ctrl+C cancellation is reported as cancelled (exit 130), not
as a still-running process. Launch progress appears before waiting for the agent.
Pipe/oneshot smoke tests do not establish interactive-terminal compatibility.
OffGrid selects an installed chat model whose **admitted** context meets
Hermes's requirement when installing without `--model`; `--model` keeps the
choice explicit. Downloading a model adds it to OffGrid's catalog, but does not
silently change an existing Hermes session or its saved default. Run
`offgrid hermes install --model <new-id>` to select it. `offgrid hermes status`
checks the managed model selection and configured context against OffGrid's
current allocation; a stale saved 65K value is not proof of 65K capacity.
Hermes' own commands remain accessible, for example `offgrid hermes model` and
`offgrid hermes sessions`. The provider is installed under
`$HERMES_HOME/plugins/model-providers/offgrid`; the default is
`~/.hermes/plugins/model-providers/offgrid` on Linux, macOS, and WSL, and the
Hermes Local AppData directory on native Windows.

The lower-level commands remain useful for custom deployments and debugging:

```bash
offgrid integrations install hermes
offgrid integrations setup hermes --model <model-id>
```

They install or render the provider only; unlike `offgrid hermes install`, they
do not install or configure the Hermes Agent application.

## OpenClaw

Install, configure, and verify OpenClaw through OffGrid:

```bash
offgrid openclaw install --model <model-id>
offgrid openclaw status
offgrid openclaw test
offgrid openclaw run "Summarize this directory"
```

If OpenClaw is missing, OffGrid asks before running its official installer
without onboarding. It then asks before registering the first-party local
plugin, checks that it loaded, dry-runs the config patch, and saves only the
`models.providers.offgrid` settings. Pass `--yes` for an explicitly approved
unattended install. The installer retains existing entries in OpenClaw's
`models.providers.offgrid.models` array when adding a selected model; it stops
if that array cannot be read safely. It places the chosen model first in the
OffGrid provider list for OffGrid CLI use, but does not replace OpenClaw's other
providers or global default model. OffGrid selects the chosen model explicitly
for `openclaw run` and `openclaw test`. The provider plugin discovers installed
chat models and their reported context from OffGrid's live `/v1/models` catalog;
newly installed models can appear on catalog refresh without a hardcoded model
name. Existing OpenClaw selections are not changed automatically. Rerun
`offgrid openclaw install --model <new-id>` to add a newly chosen model to the
saved provider configuration when needed.
The plugin discovers only chat-capable OffGrid models, not embedding-only
models. Models use native references such as
`offgrid/phi-3.5-mini-instruct.Q4_K_M`.

For a nonstandard OpenClaw executable, set `OFFGRID_OPENCLAW_BIN` to its full
path. Lower-level `offgrid integrations install openclaw` copies only the
provider bundle; `offgrid integrations setup openclaw` renders configuration
for manual deployment. The smoke test uses OpenClaw's local `infer model run`
path without a Gateway. It verifies the provider/model connection with a
short prompt, but does not prove real-world agent or tool reliability. Test
disposable tool tasks separately with `offgrid openclaw run` before relying on
OpenClaw for unattended automation. Small CPU-only models can pass the provider
smoke test while remaining too slow or weak for full agent tasks.

### Slow local responses and cancellation

OffGrid starts an accepted inference stream before model loading/prompt processing
and sends separate progress keep-alives. A start event is not an answer. Installed
OpenClaw versions may not display those comments, so a quiet terminal can still
mean prompt processing. Large agent instructions and tool catalogs can take minutes
on a CPU even for a short user message. Context capacity is not processing speed.

Managed runs prepare the selected model through authenticated inference admission
before launching the external runtime. This avoids making a cold model load race
Hermes' short auxiliary-title timeout. Preparation runs no prompt or tools and is
not a smoke test; status remains read-only. Older services without preparation
emit a compatibility notice and retain external-runtime loading. Cancellation
releases the caller's demand without interrupting preparation needed by another
live caller.

The managed launcher retains OpenClaw's normal tool discovery and direct tool
mode. It no longer forces `--local-model-lean`, which removes some configured
capabilities in the installed runtime. It does not drop user instructions or change
saved model defaults to accelerate a greeting. To explicitly change the per-run agent
deadline (seconds), use:

```bash
offgrid openclaw run "hello" --timeout 900
```

Without this option, OpenClaw retains its own default. OffGrid accepts 1–86400
seconds, not an unlimited run; each streamed inference request remains bounded to
15 minutes by the service. Hermes retains its supported per-invocation
`--run-budget` option. Increasing either budget does not improve inference speed.
OffGrid no longer generates a separate fixed provider timeout. Older setups wrote
`models.providers.offgrid.timeoutSeconds: 300`; that override can still abort slow
prefill even with keep-alives or a longer `--timeout`. Configuration updates preserve
existing user settings. If that value came from the old OffGrid setup, back up your
private OpenClaw configuration and explicitly remove only that setting with
`openclaw config unset models.providers.offgrid.timeoutSeconds`. Do not remove a
deliberately configured limit without reviewing it. The finite agent and service
deadlines still apply.
Ctrl+C cancels the owned invocation. On Unix, the runtime gets a short opportunity
to abort requests/clean up before forced termination. Cancellation and timeout
are not successful completion, and OffGrid does not automatically replay a task.

## Experimental tool-template compatibility

`OFFGRID_TOOL_TEMPLATE_ADAPTER=llama31-optional-tools-v1` explicitly enables a
reviewed compatibility adapter when both the GGUF architecture/template digest
and the installed llama runtime revision match. It permits ordinary answers
when tools are available; it does not change the selected model, remove tools,
rewrite user instructions, or relax tool validation. Other templates keep their
native behavior. An unset value or `native` disables the override. Changing this
service environment requires a safe service restart; it is not a per-message
switch, and existing model files are not rewritten.

This is **experimental, not general tool-workflow qualification**. On the tested
Meta-Llama 8B configuration, greetings improved but sequential tool/file work
still failed independent checks. Neither installation nor a successful smoke
test certifies those workflows. Runtime diagnostics include the actual template
digest and adapter identity; token accounting uses the same worker template as
generation. Unknown adapters cannot be loaded from user-provided code.

`llama31-result-fidelity-v2` is a separate opt-in revision that adds guidance for
interpreting display-only line numbers, genuine source prefixes, Unicode,
truncation and contradictory read-back. It preserves original result content;
it does not strip file text, rewrite tool arguments or independently govern an
external runtime's tools. A write receipt is not proof that the requested
outcome is correct. The revision has its own fingerprint and must pass separate
synthetic and real-workflow checks; selecting it is not a reliability guarantee.
The existing `llama31-optional-tools-v1` selection remains unchanged.

The October 24 comparison **rejected v2**: it passed 2/7 frozen synthetic cases
and still failed the Hermes exact-file workflow with a false completion claim.
One OpenClaw/Meta-Llama run passed, but that does not close the Hermes or
cross-case failures. The local activation was rolled back. Do not use this
revision as a production reliability fix; see the dated evidence below.

## Docker networking

If Hermes or OpenClaw runs on the Docker host and OffGrid publishes
`127.0.0.1:11611:11611`, keep `http://127.0.0.1:11611/v1`. If both run in the
same Compose network, use the OffGrid service name, for example
`http://offgrid:11611/v1`, and generate setup with:

```bash
offgrid integrations setup hermes \
  --model <model-id> \
  --base-url http://offgrid:11611
```

Do not expose an unauthenticated OffGrid service to a shared network. Enable
authentication and put TLS at a trusted reverse proxy before using a remote
base URL.

## API

- `POST /v1/inference/prepare` prepares an explicitly selected installed GGUF chat
  model with chat permission and normal memory/admission checks; no generation,
  default selection change or workflow qualification. Native provider preparation
  is not yet supported by this endpoint.
- `GET /v1/integrations` lists plugins and readiness.
- `GET /v1/integrations/{id}/setup` returns install, environment,
  configuration, verification, and warning data.

The web and Electron Agents pages use these same endpoints, so their setup
instructions reflect the model and effective context of the running service.

## Extending and testing integrations

`internal/integrations` owns descriptors, authenticated discovery, readiness,
selection, snapshots, connection state and the shared manager. A new supported
runtime adds a render adapter, a trusted provider bundle where needed, and a
host adapter implementing inspection/configuration/smoke/launch. Register the
host adapter at the CLI composition boundary; do not add another lifecycle or
integration-specific UI branch. A third-adapter fixture tests this boundary.
These are reviewed first-party extensions, not arbitrary executable manifests.

On Linux/WSL or macOS, the opt-in harness uses newly created private homes and a
disposable directory, never your installed agent configuration:

```bash
python3 scripts/test/test-external-agents.py /path/to/offgrid \
  --agent openclaw --model <installed-model-id> --server http://127.0.0.1:11611 --workflow
```

Add `--fixture --model fixture-tools` to test installed-runtime interoperability
with deterministic local responses, including cancellation. That is **not**
real-model qualification. The harness enables only file toolsets for the copy
test and checks its output independently; it is not an OS sandbox. Evidence is
retained in the printed private temporary directory. Existing runtimes must be
installed beforehand; the harness does not download models or agents.
Add `--greeting` for an actual external-agent greeting with the fixture's file
tools still available. Review `greeting.log`: process exit alone does not prove
answer quality. These restricted tests do not qualify the user's complete tool
catalog, and they never change that catalog.

To discover installed chat models and measure preparation, repeated replies,
streaming, native tool decisions and disconnect cancellation through the same
service, run:

```bash
python3 scripts/test/external_model_matrix.py --run --server http://127.0.0.1:11611
```

This consumes local inference and stops before starting if the service is busy.
It does not execute its synthetic tool, change model defaults, or download models.
Each failure stays in the private evidence; readiness is not a quality guarantee.
The isolated Hermes fixture disables its own background update checks so GitHub
availability does not contaminate local inference timing. Hermes' `--version`
command may also include an update check; OffGrid probes that version with a
disposable Hermes home and leaves the installed user's update, model, provider
and endpoint settings unchanged. This version probe does not establish model
reply speed or tool-workflow quality.

See [the dated evidence](../quality/evidence/external-integrations-2026-10-22.md)
for actual checks, failed model workflows and platform limitations.
