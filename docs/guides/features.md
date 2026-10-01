# Feature availability

OffGrid provides local model inference and a shared workspace through its web UI, desktop host, CLI, and API. Availability depends on installed components, permissions, model behavior, and hardware. This page is a navigation map, not a production certification.

## Everyday workflows

| Workflow | What to expect | Read next |
| --- | --- | --- |
| Chat and drafting | Local generation with an installed chat model; verify important claims | [Getting started](getting-started.md) |
| Model management | Catalog/search, downloads, installed models, and recovery | [Model discovery](model-discovery.md) |
| Document questions | Local embeddings and indexed sources; inspect evidence and extraction limits | [Knowledge](embeddings.md) |
| Agent tasks | Durable jobs, visible tool activity, approvals, recovery, and bounded artifacts | [Agents](agents.md) |
| External tools | Administrator-managed MCP connections; calls may leave the computer | [MCP](mcp.md) |
| Computer tasks | Preview browser and structured application operations within local consent | [Computer tasks](computer-tasks.md) |
| Shared access | Authentication and roles; not a guarantee of the entire planned project-sharing system | [Multi-user guide](multi-user.md) |

## Preview and optional capabilities

Computer drivers are not qualified for arbitrary applications or every model. Managed-browser vision is distinct from native desktop vision. Headless containers cannot control host apps by themselves. Follow the driver-specific limits rather than inferring capability from an enabled button.

Audio, P2P, LoRA adapter management, and external-agent integrations depend on extra components. Registering an adapter is not training a model. A benchmark command is not the planned versioned dataset/experiment workspace. Proposed features in roadmaps remain proposals until implemented and tested.

Hermes and OpenClaw use OffGrid inference but maintain their own agent runtimes and permissions. See [external agents](external-agents.md); their tools do not inherit OffGrid's approval guarantees.

## What local means

Model inference uses the configured local runtime. Installing software/models, searching public catalogs, using external MCP servers, and visiting public websites can use the internet. Review the destination and information sent before granting tool access. No cloud fallback should be inferred from a local model failing to load.

## Evidence and limits

The [reliability plan](../advanced/product-reliability-plan.md) records test evidence and remaining gates. Keep results scoped to their actual OS, hardware, model, driver, and workload. Do not convert unit tests, fixture success, or a published package into a universal quality claim.

For exact command syntax use the installed CLI help and [CLI reference](../reference/cli.md). For protocol details use the [API reference](../reference/api.md) and [OpenAPI contract](../../pkg/api/openapi.yaml).
