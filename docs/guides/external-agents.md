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
selecting tools; test a real governed task before making it the default.

To use a 65,536-token context explicitly:

```bash
export OFFGRID_MAX_CONTEXT=65536
export OFFGRID_ADAPTIVE_CONTEXT=false
offgrid serve
```

Large contexts consume substantially more RAM or VRAM. Keep adaptive context
enabled when the machine cannot safely allocate the requested window; the
readiness endpoint will then report the effective value and remain honest
about whether the external agent is ready.

## Hermes Agent

The recommended path is one command:

```bash
offgrid hermes install
```

If Hermes is missing, OffGrid asks before downloading and running Nous
Research's official installer. It then installs the embedded OffGrid provider,
persists the endpoint, token, model, provider, and context through Hermes'
configuration CLI. No YAML editing or shell exports
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
as a warning without invalidating the configured core; rerun with
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
unattended install. Existing OpenClaw default models remain unchanged; OffGrid
selects the chosen model explicitly for `openclaw run` and `openclaw test`.
The plugin discovers only chat-capable OffGrid models, not embedding-only
models. Models use native references such as
`offgrid/phi-3.5-mini-instruct.Q4_K_M`.

For a nonstandard OpenClaw executable, set `OFFGRID_OPENCLAW_BIN` to its full
path. Lower-level `offgrid integrations install openclaw` copies only the
provider bundle; `offgrid integrations setup openclaw` renders configuration
for manual deployment. The smoke test uses OpenClaw's local `infer model run`
path without a Gateway. It verifies the provider/model connection with a
short prompt, but does not prove real-world agent or tool reliability. Test
governed tool tasks separately with `offgrid openclaw run` before relying on
OpenClaw for unattended automation. Small CPU-only models can pass the provider
smoke test while remaining too slow or weak for full agent tasks.

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

- `GET /v1/integrations` lists plugins and readiness.
- `GET /v1/integrations/{id}/setup` returns install, environment,
  configuration, verification, and warning data.

The web and Electron Agents pages use these same endpoints, so their setup
instructions reflect the model and effective context of the running service.
