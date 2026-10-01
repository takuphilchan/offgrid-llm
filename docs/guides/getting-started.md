# Use your OffGrid workspace

Once installed, the same workspace is available through the browser, matching desktop app, and service-aware CLI commands. For installation, start with the [quickstart](../setup/quickstart.md).

## Check which workspace you opened

Open **Settings** to inspect the connected service. Desktop-managed data normally lives under your user profile; a container uses its mounted volumes. These are not automatically the same workspace. If models or history appear missing, check the address and workspace identity before downloading again.

The service owns saved work. Closing or reloading a page does not cancel submitted tasks; use task controls to pause or stop them.

## Have a local conversation

Open **Models**, install a chat model, then select it in **Chat**. Begin with a short question. Long documents, large context windows, and several loaded models require more memory.

Check important answers. Fluent text is not evidence of current facts, file access, or a completed computer action. A local model does not automatically browse the internet. See [history management](history-management.md) for conversation rename, export, and deletion.

## Ask about documents

Knowledge retrieval uses an embedding model as well as a chat model. Enable embeddings, import a supported file, wait for indexing, then ask a grounded question. Inspect the cited source.

Extraction failures, scanned documents, unavailable indexes, and insufficient evidence need different remedies. Follow [knowledge and embeddings](embeddings.md); do not delete sources because an old index needs rebuilding.

## Delegate a task

Open **Agents**, describe the outcome, and choose **Start task**. For example:

> Calculate the total of 18.50, 42.75, and 9.25. Save a small CSV showing each amount and the total.

The selected model needs reliable tool calling. Inspect the activity and artifact. A verified artifact means its bytes and supported format were checked; it does not guarantee factual or mathematical correctness.

For work inside an application, describe the outcome naturally. OffGrid may show **Access needed** and ask you to select the target locally. This consent is separate from the prompt. See [computer tasks](computer-tasks.md) and [agent controls](agents.md#pause-adjust-and-recover).

## Add external information deliberately

An [MCP connection](mcp.md) adds tools from another server. Connect only a trusted server, inspect its tools, and start with a non-sensitive query. Inference can remain local while tool arguments travel over the internet.

Hermes and OpenClaw are [external agents](external-agents.md), not the built-in runner. Their permissions and history are separate.

## Recover or remove unfinished work

- **Access needed:** grant the target or stop the task.
- **Paused or interrupted:** inspect the saved state, then resume or stop deliberately.
- **Outcome unknown:** inspect the affected target before recording what happened. Do not invent success or repeat a potentially mutating operation blindly.
- **Completed, failed, or cancelled:** use confirmed deletion when no unresolved execution or linked-child restriction remains.

Tasks and Activity share history. Deleting a task does not undo a file change or remote action; backups and audits are separate. See [agent recovery](agents.md#restart-and-uncertain-outcomes).

## Keep your work

Back up before upgrades. An image, installer, and Git checkout do not contain your saved workspace. Follow [backup and restore](../advanced/workspace-recovery.md); retain model files and host companion state separately where required.
