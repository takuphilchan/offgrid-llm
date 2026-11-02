# Use your OffGrid workspace

Once installed, the same workspace is available through the browser, matching desktop app, and service-aware CLI commands. For installation, start with the [quickstart](../setup/quickstart.md).

## Check which workspace you opened

Open **Settings** to inspect the connected service. Desktop-managed data normally lives under your user profile; a container uses its mounted volumes. These are not automatically the same workspace. If models or history appear missing, check the address and workspace identity before downloading again.

The service owns saved work. Closing or reloading a page does not cancel submitted tasks; use task controls to pause or stop them.

## Have a local conversation

If a chat model is already available, open **Chat** and begin with a short question.
Otherwise, the setup guide opens **Models** so you can review a download or an
offline import. Nothing is downloaded by opening the guide. Long documents,
large context windows, and several loaded models require more memory.

A missing model or unavailable runtime does not silently select a replacement.
Chat keeps your draft and offers **Choose a chat model**. Select another model
explicitly, or restore the original installation and refresh the workspace.

Check important answers. Fluent text is not evidence of current facts, file access, or a completed computer action. A local model does not automatically browse the internet. See [history management](history-management.md) for conversation rename, export, and deletion.

## Use voice without changing the task workflow

With a compatible ASR runtime and model, **Use microphone** records a draft in
Chat or Agents. Stop recording, review the transcription, then send/start it
yourself. Dictation does not approve tools or submit work automatically.

With a compatible TTS runtime and model, **Read aloud** plays an existing Chat
answer. **Speak responses** in the Chat composer opts into speech for new
answers. **Voice settings** selects ASR and TTS independently; close the panel
by clicking elsewhere or pressing Escape. See [voice setup and recovery](voice.md)
before downloading: model weights alone do not install the speech runtime.

## Ask about documents

For a one-conversation question, use **Add files** in Chat and wait for each file
to finish preparing. Supported small documents do not require a separate
embedding setup. Keep the selected model's context limit in mind; inspect source
details and any extraction warnings before relying on the answer.

For a reusable document library, open **Knowledge**. Complete its one-time setup,
then **Add documents**, wait for **Ready**, and choose **Ask**. Embedding settings
remain available when needed; ordinary imports do not require configuring them
each time. Inspect the cited source.

Extraction failures, scanned documents, unavailable indexes, and insufficient evidence need different remedies. Follow [knowledge and embeddings](embeddings.md); do not delete sources because an old index needs rebuilding.

## Delegate a task

Create a profile in **Agents** with a name and purpose, then choose **Use in Chat**.
Describe the outcome in Chat. Tools and context are optional profile settings;
folder or computer access is requested separately when needed. For example:

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
