# Live agent progress

The Agents workspace shows live service phases: waiting for inference, loading
the model, preparing the prompt, generating a response, preparing a tool call,
executing an authorized tool, or waiting for approval. It shows the model-turn
number, elapsed time, and the age of the last saved progress update. These are
real runtime states, not estimated percentages or invented model reasoning.

Public model text appears in **Live response preview**, enabled by default. Turn
the checkbox off to hide it without stopping work. Completed results remain
separate. Truncated, cancelled, failed and interrupted output is not labelled as a
completed answer or added to completed model context. Existing exact-call tool
approval and uncertain-outcome reconciliation are unchanged.

Leaving the page, reloading, or losing the connection does not cancel an accepted
task. Returning to the selected run recovers its latest durable snapshot. A lost
connection shows a reconnection notice; the UI does not submit the task again.
Use **Cancel** to stop execution explicitly. Cancellation cannot undo tool side
effects. This feature does not add mid-run conversational steering.

## Workspace layout

The task form sizes independently from results, so growing output does not move
the **Run task** button. Narrow workspaces stack the panels; wider workspaces put
them side by side. Long model names, paths, and output wrap within their panel.

Results have one bounded, keyboard-focusable scrolling area. Status, **Cancel**,
and the preview toggle remain above it. Live output follows the bottom of this
area without scrolling the page or moving keyboard focus. Scroll upward to read
earlier text; return to the bottom to resume following. Approval and uncertain
outcome transitions return to the decision at the top rather than hiding it
below prior output. Completed output and steps use the same scrolling area.

## Service contract

- `GET /v1/agents/tasks/{id}` includes `progress` and `started_at`.
- `GET /v1/agents/tasks/{id}/events` is an authenticated, owner-scoped SSE stream
  of the latest snapshot and subsequent changed snapshots, with heartbeat
  comments. Terminal/approval snapshots end the stream. Browser requests use the
  existing same-origin session cookie, never a credential in the URL.
- Reconnection recovers a snapshot, **not** a complete event-history replay.
  This does not claim the future `/api/v2/jobs` contract or event compaction.
- The preview is capped at 64 KiB, preserving UTF-8. The first text and phase
  changes are saved immediately; later deltas are coalesced to roughly four
  writes per second, with a final flush. A crash may lose the most recent
  unflushed preview fragments, but they were never acknowledged as completed work.
- Streaming native tool-call fragments are assembled and bounded privately.
  Reasoning fields and provisional tool arguments are not streamed into the
  preview. Calls execute only after a valid terminal response and authorization.
- Engines without structured streaming retain the safe nonstreaming model-call
  path, with lifecycle progress but no token preview. Never discard native tool
  calls or invent a successful finish reason merely to display streaming.
- Slow viewers have bounded write deadlines and cannot block execution. A viewer
  disconnect never becomes the worker's cancellation context.

Interactive CLI agent streams show phase changes and provisional response text on
stderr; final results remain on stdout. Existing `agent status/approve/deny/cancel`
controls still address the same run ID. CLI connection loss reports the run ID for
inspection; automatic CLI reconnection is not implemented in this slice.

Web and Electron use the same renderer. New labels exist for the nine interface
languages; speaker review and installed-desktop qualification remain outstanding.
External Hermes/OpenClaw processes do not automatically gain an OffGrid-hosted
activity view from this change; their own runtime interfaces remain separate.
