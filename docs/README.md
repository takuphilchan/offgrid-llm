# OffGrid documentation

Start here to install OffGrid, finish a task, or maintain an existing workspace. These guides describe this checkout; use the notes for your installed release when behavior differs.

## Choose your next step

| I want to | Start with |
| --- | --- |
| Install and ask my first question | [Quickstart](setup/quickstart.md) |
| Choose desktop, CLI, or Docker | [Installation](setup/installation.md) |
| Use my existing workspace after updating desktop | [Desktop startup and recovery](setup/desktop-startup.md) |
| Update a container without losing data | [Docker upgrades](setup/docker.md#upgrade-an-existing-workspace) |
| Find and download a model | [Model discovery](guides/model-discovery.md) |
| Dictate or hear an answer aloud | [Voice setup and controls](guides/voice.md) |
| Ask questions about local documents | [Knowledge and embeddings](guides/embeddings.md) |
| Run a task or connect tools | [Agents](guides/agents.md) and [MCP connections](guides/mcp.md) |
| Understand computer access and its limits | [Computer tasks](guides/computer-tasks.md) |
| Stop work or remove old history | [History management](guides/history-management.md) |
| Back up or restore work | [Workspace recovery](advanced/workspace-recovery.md) |

## Understand the workspace

The browser and desktop display the same UI from one OffGrid service. The service owns models, conversations, tasks, and data. Desktop can use its bundled service or connect to a compatible local service. Docker is a separate installation: updating Windows does not replace a running container.

Local inference does not require a cloud model provider. Downloads, public model search, remote MCP tools, and external websites use the network when requested. A remote tool receives the arguments sent to it; do not treat those calls as offline.

[Getting started](guides/getting-started.md) explains the main workflows. [Feature availability](guides/features.md) separates available paths from preview capabilities. The [reliability plan](advanced/product-reliability-plan.md) is the qualification record; a passing build is not proof of every platform, model, or task.

## Install and operate

- [Installation](setup/installation.md), [Docker](setup/docker.md), and [autostart](setup/autostart.md)
- [Desktop recovery](setup/desktop-startup.md) and [deployment](advanced/DEPLOYMENT.md)
- [Models](guides/models.md), [model discovery](guides/model-discovery.md), and [Hugging Face downloads](guides/huggingface.md)
- [Voice](guides/voice.md): ASR/TTS selection, dictation, playback and troubleshooting
- [Multi-user access](guides/multi-user.md), [audit records](guides/audit.md), and [metrics](guides/metrics.md)
- [Performance](advanced/PERFORMANCE.md), [CPU support](advanced/cpu-support.md), and [low-memory tradeoffs](advanced/low-memory.md)
- [Appliance planning](setup/appliance.md) for shared offline installations

## Use the interfaces

- [CLI reference](reference/cli.md) and [JSON output](reference/json-output.md)
- [API reference](reference/api.md) and [OpenAPI source](../pkg/api/openapi.yaml)
- [Python client](../python/README.md)
- [External agents](guides/external-agents.md): Hermes and OpenClaw are separate runtimes, not OffGrid's governed runner

Replace `YOUR_MODEL`, `RUN_ID`, and `/path/to/...` with your own values. Copy an installed model ID from Models or `offgrid list`. Bash and PowerShell use different continuation and environment-variable syntax.

## Develop and qualify

- [Build from source](advanced/BUILDING.md) and [contributing](../dev/CONTRIBUTING.md)
- [Architecture](advanced/ARCHITECTURE.md), [repository map](repository-structure.md), and [client contracts](advanced/client-contracts.md)
- [Workspace UI](advanced/workspace-ui.md), [agent progress](advanced/agent-live-progress.md), and [maintainability](advanced/maintainability.md)
- [Releasing](advanced/releasing.md), [distribution](advanced/distribution.md), and [versioning](reference/versioning.md)
- [Documentation standards](templates/README.md)

## Plans and historical material

[Production readiness](advanced/production-readiness.md) and the [roadmap](ROADMAP.md) describe unfinished work, not current guarantees. [Release notes](releases/) describe the version named in each file; old commands in those archives are not current setup instructions.

Report documentation problems with the page, installed version, OS, and exact command or UI step. Remove credentials and private task contents before posting an [issue](https://github.com/takuphilchan/offgrid-llm/issues).
