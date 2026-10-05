# CLI reference

Use `offgrid --help` for the command inventory. This reference covers the main
supported workflows; optional and legacy commands have different prerequisites.
Examples assume the executable is on `PATH`. In PowerShell, use
`.\offgrid.exe` when running from its installation directory.

## Service connection

For typed speech/language/embedding discovery and managed speech package operations,
use `offgrid model help`. `model list|catalog --category asr|tts|language|embeddings`,
`discover`, `preview`, `install`, `status`, `cancel`, `resume`, `repair`, `verify`,
`discard`, and `remove` all use the existing authenticated service client. See the
[complete acquisition examples](../guides/model-discovery.md#service-backed-package-cli-and-python).
Speech package installation alone does not enable voice inference.

Start the service with `offgrid serve`, or use an already-running desktop/container
service. Do not start a second writer against its workspace.

Service-aware commands use `OFFGRID_SERVER_URL` (default
`http://127.0.0.1:11611`) and `OFFGRID_API_KEY` when authentication is enabled.
Set credentials privately in the client environment, not in shared command logs.

Configure the server through environment variables or `OFFGRID_CONFIG`, **not**
`serve --host`, `serve --port`, or `serve --model`; those are not parsed server
options in this implementation.

Bash:

```bash
OFFGRID_HOST=127.0.0.1 OFFGRID_PORT=11612 offgrid serve
```

PowerShell:

```powershell
$env:OFFGRID_HOST = '127.0.0.1'
$env:OFFGRID_PORT = '11612'
offgrid serve
```

Clients for that example need `OFFGRID_SERVER_URL=http://127.0.0.1:11612`.
The web UI is at `/ui/`. See [deployment](../advanced/DEPLOYMENT.md) for
authentication, storage, and service ownership.

## Models

```sh
offgrid list
offgrid list --catalog
offgrid search phi --limit 5
offgrid search "large model" --files --limit 5
offgrid download phi-3.5-mini-instruct
offgrid download OWNER/REPOSITORY --file MODEL.gguf
offgrid download-hf OWNER/REPOSITORY --file MODEL.gguf
```

Replace repository/file placeholders with actual results from search. Download
options include `--quant`, `--file`, `--detach`, `--yes`, and global `--json`.
Use `offgrid download --help` for its syntax. Quantization is an option, not a
second positional argument. Listing and downloads operate on the connected
service; they do not silently fall back to another local registry. CLI search
currently queries Hugging Face directly from the CLI process, so its network
environment can differ from the service's web-UI search and download environment.

Check [model discovery](../guides/model-discovery.md) for cancellation, resume,
and projector handling. A projector download is not proof of vision compatibility.

### Terminal chat and local model files

```sh
offgrid run YOUR_INSTALLED_MODEL_ID
offgrid alias list
offgrid alias set mymodel YOUR_INSTALLED_MODEL_ID
offgrid alias remove mymodel
```

Copy the model ID from `offgrid list`. The legacy `run` path also scans the local
models directory and resolves aliases there. A host CLI does not see files in a
container volume automatically: use the web UI or `docker exec -it offgrid offgrid
run YOUR_INSTALLED_MODEL_ID` for that deployment. Missing aliases may prompt for
a download; do not rely on unattended automatic selection.

Import/export/removal and alias configuration include local-file behavior.
Review [model management](../guides/models.md) before using them against an
installed workspace.

## Sessions

These commands use the connected service and its ownership checks:

```sh
offgrid session list
offgrid session show "Conversation name"
offgrid session export "Conversation name" conversation.md
offgrid export-session "Conversation name" --format json --output conversation.json
```

Exports default to stdout when no output path is given. An output file must not
already exist. To delete a conversation, use `offgrid session delete "Conversation
name"`; this removes saved history, not external files created during work.
Review the target and export it first if you need a copy.

See [history management](../guides/history-management.md).

## Agent tasks

Choose an installed tool-capable model, not an unverified alias:

```sh
offgrid agent run "Calculate 25 * 47 and report the result" --model YOUR_MODEL_ID --wait
offgrid agent tasks
offgrid agent status RUN_ID
offgrid agent chat --model YOUR_MODEL_ID
```

`agent run` accepts:

| Option | Meaning |
| --- | --- |
| `--model ID` | Required installed model ID |
| `--request-id ID` | 8–128 characters; reuse only for an identical submission retry |
| `--wait` | Follow task progress; input or approval may still be required |
| `--style react\|cot\|plan-execute` | Instruction style; `plan` aliases `plan-execute` |
| `--max-steps N` | 1–50 model iterations |
| `--json` | Return the saved submission snapshot; inspect its run ID for completion |

Without `--wait`, submission returns after saving the task. Reusing a request ID
with different work conflicts. `agent chat` accepts `--model`, not the old
`--template` examples. A style changes neither permissions nor model capability.

### Control and recover a saved task

```sh
offgrid agent approve RUN_ID APPROVAL_ID
offgrid agent deny RUN_ID APPROVAL_ID
offgrid agent pause RUN_ID
offgrid agent steer RUN_ID UNIQUE_REQUEST_ID "Focus on the three main findings."
offgrid agent resume RUN_ID
offgrid agent takeover RUN_ID
offgrid agent reconnect RUN_ID
offgrid agent cancel RUN_ID
offgrid agent export RUN_ID
```

Run only the operation intended for the current task, not this whole block.
Approval IDs come from its pending snapshot. Stop/cancel does not undo effects.
Use `agent reconcile RUN_ID CALL_ID "verified outcome"` only after independently
inspecting an uncertain operation; it records a result without rerunning it.
See [agent recovery](../guides/agents.md).

### Computer Tasks

```sh
offgrid computer status
offgrid computer targets
offgrid computer run "Read the selected document and summarize it" --model YOUR_MODEL_ID
offgrid computer setup RUN_ID
offgrid computer check YOUR_MODEL_ID
```

`setup RUN_ID` opens the installed desktop consent flow for the saved task.
`targets` reports existing computer sessions; it is not unrestricted OS discovery.
`computer pause|resume|takeover|stop RUN_ID` uses the same durable lifecycle.
The CLI cannot bypass local consent. See [Computer Tasks](../guides/computer-tasks.md)
for native/browser/vision limitations.

### Tools and MCP

Use **Agents → Available tools** and **Connections** for live service management.
The older CLI configuration commands are:

```sh
offgrid agent mcp list
offgrid agent mcp add https://learn.microsoft.com/api/mcp --name learn-docs
offgrid agent mcp disable learn-docs
offgrid agent mcp enable learn-docs
offgrid agent mcp remove learn-docs
```

These legacy commands edit local `tools.json`, not the connected service through
`OFFGRID_SERVER_URL`. Do not use them concurrently with a running service or
expect a Windows host edit to change a container. Prefer the UI; see
[MCP setup and removal](../guides/mcp.md). An `npx` command is not an HTTP URL,
and `agent mcp test` is not a supported CLI subcommand.

## Knowledge

```sh
offgrid kb status
offgrid kb enable YOUR_EMBEDDING_MODEL_ID
offgrid kb list
offgrid kb add ./notes.txt
offgrid kb search "What are the main decisions?"
offgrid kb disable
```

Install the embedding model first. `kb remove DOCUMENT_ID` deletes that indexed
document; `kb clear --yes` removes all documents available to the operation.
Do not include destructive commands in a routine smoke test.
See [embeddings](../guides/embeddings.md) and [shared-index limitations](api.md#knowledge).

## Workspace maintenance

Backup and restore are **offline** maintenance operations, not service requests.
Stop the writer first and preserve models/runtime/configuration separately.

```sh
offgrid workspace backup --data-dir /path/to/data --output /path/to/backups/workspace.zip
offgrid workspace verify /path/to/backups/workspace.zip
```

These are placeholders, not installation defaults. Follow
[backup and restore](../advanced/workspace-recovery.md) for prerequisites,
version matching, and restoration to a new directory.

## Configuration and diagnostics

```sh
offgrid version
offgrid info
offgrid doctor
offgrid config show
offgrid config validate /path/to/config.yaml
```

`config init PATH` writes a configuration file; use a new path rather than
overwriting an installed configuration. Set `OFFGRID_CONFIG` to load it.
There are no `config set` or `config reset` subcommands.

| Environment variable | Purpose |
| --- | --- |
| `OFFGRID_HOST`, `OFFGRID_PORT` | Service bind address and port |
| `OFFGRID_SERVER_URL`, `OFFGRID_API_KEY` | Service-aware client address/authentication |
| `OFFGRID_DATA_DIR`, `OFFGRID_MODELS_DIR` | Separate state and model roots |
| `OFFGRID_UI_DIR` | Matching generated UI |
| `OFFGRID_LLAMA_SERVER_PATH` | Explicit native inference runtime |
| `OFFGRID_REQUIRE_AUTH`, `OFFGRID_MULTI_USER` | Authentication and multi-user configuration; see the guide below |
| `NO_COLOR`, `OFFGRID_UNICODE` | Terminal presentation |
| `OFFGRID_TUI=0`, `OFFGRID_PLAIN=1` | Line-oriented terminal chat |

User administration uses `offgrid users list` and
`offgrid users create NAME ROLE`, where the role is positional. These legacy
commands access local storage; stop the service and use its actual data path.
See [multi-user setup](../guides/multi-user.md), not a copied admin password.

## Completions

Generate shell completions with `offgrid completions bash`,
`offgrid completions zsh`, or `offgrid completions fish`. Review output before
installing it into shell configuration. This is shell setup, not model inference.

## JSON and exit status

For the service-aware command path: `0` is successful command completion,
`1` operational failure, `2` invalid usage, and `130` cancellation.
A successful submission does not mean the agent task has completed successfully;
inspect its persisted status.

Legacy optional commands are not all covered by this contract. Do not infer
universal JSON support from the global flag. See [JSON output](json-output.md)
and [client contracts](../advanced/client-contracts.md).

## Optional commands

[External agents](../guides/external-agents.md),
[appliance planning](../setup/appliance.md), and [audit](../guides/audit.md) have
separate guides. Audio needs optional installed components. LoRA registration is
not a training workflow; P2P and native
computer control have separate preview/qualification boundaries. Consult the
[capability map](../guides/features.md) before treating their presence as readiness.
