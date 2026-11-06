# OffGrid workspace tour

These screenshots show the 0.4.27 development build with isolated demonstration
data. The conversation and library entries are samples, not model-quality or
document-ingestion benchmark results. Your installed release may look different.

## Chat

Keep the conversation in the main workspace. The composer provides attachments,
response settings and voice controls without opening a separate setup page.

![OffGrid Chat showing a sample community workshop plan and its composer](../images/chat-workspace.png)

## Knowledge

Use Knowledge for documents you want to reuse across conversations. Files attached
directly in Chat stay with that conversation. See the [Knowledge guide](embeddings.md)
for supported preparation paths and source limitations.

![OffGrid Knowledge showing three sample documents, source controls and search](../images/knowledge-library.png)

## Desktop terminal

The Windows desktop terminal shares the main work area with Chat. Use **+** to
open OffGrid CLI, Agent or PowerShell in a folder you choose. Leaving Agent with
`/exit` returns to the shell; closing a tab stops its owned processes.

![OffGrid's native desktop terminal displaying command help in a demonstration folder](../images/desktop-terminal.png)

The terminal capture runs the actual packaged CLI. Its prompt is shortened for
the demonstration. See [desktop terminal](../setup/desktop-terminal.md) for
keyboard shortcuts, local permissions and platform availability.

## Refreshing the images

From `apps/web`, set `OFFGRID_DOC_CAPTURES=1` and run
`npx playwright test tests/documentation-captures.spec.ts --workers=1`.
For the terminal, use the same variable and run
`node scripts/test/test-desktop-terminal-packaged.mjs PATH_TO_PACKAGED_EXE`
from the repository root. Both write to `build/documentation-captures`.
Inspect the images before copying selected files into `docs/images`.
Use isolated sample state; do not publish personal conversations or credentials.
