# OffGrid workspace tour

These screenshots show the 0.4.30 renderer with isolated demonstration
data. The conversation and document/model library entries are samples, not model-quality or
document-ingestion benchmark results. Your installed release may look different.

## Chat

Keep the conversation in the main workspace. The composer provides attachments,
response settings and voice controls without opening a separate setup page.

![OffGrid Chat showing a sample community workshop plan and its composer](../images/chat-workspace.png)

### Dark mode

The same conversation in dark mode. Choose a theme in Settings to suit your workspace.

![OffGrid Chat in dark mode with the same sample workshop plan](../images/chat-dark.png)

## Models

**Discover models** presents catalog information, estimated memory requirements
and a download review. Discovery uses the internet; an installed model's runtime
and image capabilities are checked separately.

![Model discovery showing Qwen3.5, SmolLM3 and Granite catalog cards](../images/models-discover.png)

**Installed** keeps local choices separate from discovery. This sample library
shows three language models and the current selection. Runtime readiness is
shown explicitly rather than inferred from a model's presence on disk.

![Installed model library showing three sample language models and the current selection](../images/models-installed.png)

## Knowledge

Use Knowledge for documents you want to reuse across conversations. Files attached
directly in Chat stay with that conversation. See the [Knowledge guide](embeddings.md)
for supported preparation paths and source limitations.

![OffGrid Knowledge showing three sample documents, source controls and search](../images/knowledge-library.png)

## Agent profiles

Save a model and instructions for a recurring task, then select the profile in
Chat. Tools and context are optional; configure only the resources that task needs.

![Workshop assistant profile showing its instructions, model and optional tools and context](../images/agent-profiles.png)

## Animated overview

This 16-second GIF cycles through four captured screens. It is a screenshot
slideshow, not a recording of model generation or task execution. The static
images above show the same views without animation.

![Screenshot slideshow of dark Chat, model discovery, Knowledge and an agent profile](../images/workspace-tour.gif)

## Desktop terminal

The Windows desktop terminal shares the main work area with Chat. Use **+** to
open OffGrid CLI, Agent or PowerShell in a folder you choose. Leaving Agent with
`/exit` returns to the shell; closing a tab stops its owned processes.

![Native OffGrid 0.4.30 terminal displaying CLI help on the isolated Demo drive](../images/desktop-terminal.png)

**Commands** shows the command, captured output and shell-reported completion
status. Shell reports help you inspect a command; they are not independent proof
of its effects.

![Terminal command details showing real CLI help output and successful shell exit](../images/terminal-command-context.png)

These terminal images use the actual locally packaged 0.4.30 CLI. A temporary
PowerShell drive named `Demo:` keeps the isolated capture directory readable;
the normal shell integration and command reporting remain enabled.

See [desktop terminal](../setup/desktop-terminal.md) for
keyboard shortcuts, local permissions and platform availability.

## Refreshing the images

From `apps/web`, set `OFFGRID_DOC_CAPTURES=1` and run
`npx playwright test tests/documentation-captures.spec.ts --workers=1`.
For the terminal, use the same variable and run
`node scripts/test/test-desktop-terminal-packaged.mjs PATH_TO_PACKAGED_EXE`
from the repository root. Both write to `build/documentation-captures`.
Inspect the images before copying selected files into `docs/images`.
Use isolated sample state; do not publish personal conversations or credentials.

To build the optional GIF from Chat, model discovery, Knowledge and agent captures,
run from the repository root:

```sh
npm install --prefix build/documentation-tools --no-save --ignore-scripts gifenc@1.0.3 pngjs@7.0.0
node scripts/maintenance/build-documentation-gif.cjs
```

The encoder writes `build/documentation-captures/workspace-tour.gif`; its tools
stay outside the application dependencies. Review each frame before publishing.
Superseded screenshots are retained in [old-images](../images/old-images/README.md)
and are not examples of the current interface.
