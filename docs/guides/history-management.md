# Managing chat and agent history

Web and desktop share these controls. Deletion always requires confirmation and
applies to the listed items captured when the confirmation opens, not new items
created in another tab afterwards. Nothing is deleted just by opening the dialog.

In the updated development renderer, Chat and task-first Agents group bulk actions
under **Manage history** beside history search. Empty histories show a next-step
message without disabled bulk controls; a failed read still offers Retry. Trash
buttons remain visible on individual rows. Manage history explains protected task
states; these restrictions have not changed. Older compatible agent clients may
retain the previous toolbar layout.

## Conversations

- Use the visible trash button beside a conversation to delete it.
- Search conversations by title. **Delete listed chats** removes the conversations
  matching that search; clear the search first to remove all listed conversations.
- The first twenty matching conversations are rendered; **Show more** reveals
  the next twenty. Search and confirmed bulk deletion cover all loaded matches,
  not only the rows currently rendered. This is not server-side pagination.
- Confirmation shows the count and previews titles. Cancel or Escape closes it.
- Deleting a conversation also clears its draft on this browser. Drafts on other
  devices and existing backups are not erased. Deleting the selected conversation
  opens an empty composer, without silently selecting another conversation.
- A conversation generating in another client returns HTTP 409 immediately;
  deletion does not wait for generation to finish and then erase the new answer.

## Agent tasks

- Search all loaded task history by prompt or run ID; **Show more** reveals older
  tasks beyond the initial twenty. This is client-side browsing, not a claim of
  server-side paginated storage.
- **Delete task** removes an eligible run. **Clear removable tasks** removes the
  deletable runs matching the current search, including those beyond Show more.
- Completed, failed or cancelled runs owned by the current actor are eligible.
  Interrupted records with **no checkpoint and no pending approval** can also be
  deleted: they have no recovery path. Unowned legacy records remain removable
  only by the local administrator, not the first signed-in user.
  Pending, running, approval-waiting, resumable interrupted and uncertain runs are protected.
  Stop active work explicitly; inspect/reconcile unknown tool outcomes first.
  A worker still settling cancellation can briefly return 409: refresh and retry.
- **Reuse task** copies a prompt into an empty draft; it never executes it or
  overwrites an unsent draft. **Copy result** copies the selected saved output.
- Removing a selected run clears its result and saved selection. It does not clear
  an independent task draft. Removed runs also disappear from Activity after reload.

Bulk operations report partial failure honestly and keep only failed items in
the dialog for retry. Successful items are not sent again. Normal ownership and
administrator-only agent permissions still apply; these controls grant no access.

## Retention and recovery boundaries

Activity shows available current/recent work first. Select an item to inspect its
events; **Open task** appears only when its durable job identity is known. It opens
the same task, never a new run. Legacy event records may have no task link.
Runtime counters live under **Diagnostics**; their failure does not hide history.

Knowledge documents have separate **Manage document** controls for reindexing and
confirmed deletion. **View source** reads retained source with current permissions;
it cannot reconstruct an unretained original. Deleting a history entry is not
document deletion, and **Ask using Knowledge** only prepares the existing Chat
composer—it does not ingest, submit, or delete anything.

History deletion is not secure erasure and cannot undo actions performed by a
tool. Tool-created files, separate audit/event logs, artifact storage and existing
backups remain. Restore from a matched workspace backup if recovery is needed;
there is no in-app undo for confirmed deletion.

Agent deletion atomically replaces the task snapshot with a minimal tombstone
(identity, actor, status and timestamps), clearing its prompt, result, approval,
progress and checkpoint. The tombstone prevents retained event projections from
recreating a deleted history entry, including after restart. Deleted run events
are no longer returned by the Activity API. Do not downgrade an application/data
pair without a matched backup: older versions do not understand these tombstones.

`DELETE /v1/agents/tasks/{id}` returns 200 on success, 404 for absent/other-owner
runs, 409 for protected states or unsettled workers, and 503 on storage failure.
This feature is scoped to first-party OffGrid history; external Hermes/OpenClaw
session stores are not modified.
