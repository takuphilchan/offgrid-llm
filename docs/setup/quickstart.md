# Start using OffGrid

This guide takes you from installation to a first local answer. Download and startup time depend on your connection, model, and hardware; there is no fixed three-minute setup requirement.

## Choose one installation

For a personal computer, use the desktop package from the [release page](https://github.com/takuphilchan/offgrid-llm/releases). Choose your OS and architecture, and verify the checksum. See [installation](installation.md) for package types and verification commands.

Docker users should follow the [Docker quickstart](docker.md#quick-start). Developers should follow [building from source](../advanced/BUILDING.md). A packaged desktop app does not require you to install Go, Node.js, or Docker.

Do not start another service if you already have a workspace. Desktop can connect to a matching local service at `http://127.0.0.1:11611`; its version and UI build must match. Follow [desktop recovery](desktop-startup.md) if they differ, rather than creating a new workspace just to dismiss the message.

## Download a chat model

1. Open OffGrid Desktop, or <http://127.0.0.1:11611/ui/> for a running local service.
2. Open **Models** and choose a chat/instruction model that fits available memory. An embedding model is for document search, not chat.
3. Download it and wait for the installed state. Use **Find more models** if it is not in the catalog.
4. Open **Chat**, select the installed model, and ask: "Give me three ways to organize my work this week."

A successful reply confirms text generation on this installation. It does not qualify tool calling, retrieval, vision, or computer control. First inference can take longer while the runtime loads the model.

For offline use, install the necessary model and runtime before disconnecting. Test the actual workflow with networking disabled; search, downloads, remote tools, and external websites will not work offline.

## Use the CLI with an existing service

Keep the service running while using service-aware CLI commands. For a standalone CLI installation, start it in one terminal:

```bash
offgrid serve
```

In a second terminal:

```bash
offgrid version
offgrid list --catalog
offgrid download phi-3.5-mini-instruct
offgrid list
```

Copy the installed model ID from `list`. Use **Chat** for a first shared-workspace conversation. The interactive terminal path, `offgrid run YOUR_INSTALLED_MODEL_ID`, currently also looks up local model files. A host CLI does not automatically see models inside Docker volumes. Use the web UI or execute its CLI inside that container:

```bash
docker exec -it offgrid offgrid list
docker exec -it offgrid offgrid run YOUR_INSTALLED_MODEL_ID
```

`offgrid` must be on `PATH`; otherwise use the executable's path, such as `.\offgrid.exe` in PowerShell. For commands that use the service, `OFFGRID_SERVER_URL` selects its address and `OFFGRID_API_KEY` supplies authentication when enabled.

## Try useful work

- **Chat:** ask a question or request a draft, then check important facts yourself.
- **Knowledge:** install an embedding model, import a supported document, wait for indexing, and inspect sources. See [knowledge setup](../guides/embeddings.md).
- **Agents:** try "Calculate 25 times 47 and report the result." Reliable tool calling is required. See [agent tasks](../guides/agents.md).
- **Computer tasks:** request work in an application and grant local access when prompted. This is preview functionality, not universal app control. See [computer tasks](../guides/computer-tasks.md).

## If something stops you

| What you see | What to do |
| --- | --- |
| Desktop and service versions differ | Update the existing service and desktop to matching builds, then retry. Desktop installation alone does not update Docker. |
| No models | Check the connected workspace before downloading again. |
| Model loading fails or memory runs out | Try a smaller model or shorter context. File size is not total memory use. |
| Access needed | Review the target in desktop, or stop the task if you do not want to grant access. |
| Outcome unknown | Inspect the affected operation; do not resubmit it just to clear the warning. |

Continue with [getting started](../guides/getting-started.md) or the [documentation index](../README.md).
