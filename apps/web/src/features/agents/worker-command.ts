import {api, APIError} from '../../api/client';
import type {components} from '../../api/schema.generated';
import {clearSubmittedDraft, readDraft, writeDraft} from '../../lib/drafts';

type Command = components['schemas']['WorkerCommand'];

export function pendingWorkerCommand(key: string, id: string): Command | null {
  try {
    const saved = JSON.parse(readDraft(key)) as Command;
    if (saved.target_id === id && ['message', 'continue', 'stop', 'answer'].includes(saved.kind) && saved.request_id && saved.expected_revision > 0) return saved;
  } catch { /* No pending command. */ }
  return null;
}

// A retry must carry the accepted revision, even if the worker has moved on.
// Reloading this record never sends it; only an explicit button action does.
export async function sendWorkerCommand(key: string, command: Command) {
  try {
    const previous = JSON.parse(readDraft(key)) as Command;
    if (previous.target_id === command.target_id && previous.kind === command.kind && previous.payload.content === command.payload.content && previous.payload.question_id === command.payload.question_id) command = previous;
  } catch { /* No pending command. */ }
  const encoded = JSON.stringify(command); writeDraft(key, encoded);
  try {
    const receipt = await api.conversations.workerCommand(command);
    clearSubmittedDraft(key, encoded); return receipt;
  } catch (error) {
    if (error instanceof APIError && error.status === 409) clearSubmittedDraft(key, encoded);
    throw error;
  }
}
