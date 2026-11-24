import { api, APIError } from '../../api/client';
import type { components } from '../../api/schema.generated';
import { clearSubmittedDraft, readDraft, writeDraft } from '../../lib/drafts';

type Schema = components['schemas'];
export type CanonicalInput = Schema['ConversationInboxEntry'];
export type CanonicalQuestion = Schema['ConversationQuestion'];
export type CanonicalInputMode = 'auto' | 'message' | 'new_work';

export function retainedCanonicalAnswer(key: string, id: string) {
  try {
    const command = JSON.parse(readDraft(`${key}:input`)) as Schema['ConversationInputCommand'];
    if (command.target_id === id && command.kind === 'submit_input' && command.payload?.intent === 'answer' && command.payload.question_id) return command;
  } catch { /* No unresolved answer. */ }
  return undefined;
}

export function intersectConversationRestrictions(current: Schema['AgentResourceRestrictions'], requested: Schema['AgentResourceRestrictions']): Schema['AgentResourceRestrictions'] {
  const result = {...requested};
  if (current.knowledge) result.knowledge = requested.knowledge ? requested.knowledge.filter(id => current.knowledge!.includes(id)) : current.knowledge;
  if (current.tools) result.tools = requested.tools ? requested.tools.filter(tool => current.tools!.some(other => other.provider === tool.provider && other.tool === tool.tool && other.schema_revision === tool.schema_revision && other.location === tool.location)) : current.tools;
  if (current.computer !== undefined) result.computer = current.computer && requested.computer !== false;
  if (current.execution) result.execution = {read: current.execution.read && requested.execution?.read !== false, write: current.execution.write && requested.execution?.write !== false, processes: current.execution.processes && requested.execution?.processes !== false};
  return result;
}

async function savedCommand(key: string, command: Schema['ConversationCommand']) {
  let stored: Schema['ConversationCommand'] | undefined;
  try { stored = JSON.parse(readDraft(key)); } catch { /* No pending command. */ }
  if (stored?.target_id === command.target_id && stored.kind === command.kind && JSON.stringify(stored.payload) === JSON.stringify(command.payload)) command = stored;
  const encoded = JSON.stringify(command); writeDraft(key, encoded);
  try {
    const result = await api.conversations.command(command);
    clearSubmittedDraft(key, encoded); return result;
  } catch (error) {
    // A definite conflict did not accept this exact command. Unknown transport
    // outcomes retain the complete envelope for the next explicit retry.
    if (error instanceof APIError && error.status === 409) clearSubmittedDraft(key, encoded);
    throw error;
  }
}

export async function submitCanonicalSession(options: {
  name: string; id: string; key: string; content: string; model: string;
  responseProfile?: 'interactive' | 'extended'; maxTokens?: number;
  profile?: {id: string; revision: number}; restrictions: Schema['AgentResourceRestrictions'];
  folder?: Schema['ExecutionWorkspace'] | null;
  folderApplied?: () => void;
  origin: 'typed' | 'dictation'; error: string; imported: (id: string) => void;
  question?: CanonicalQuestion | null;
  intent?: 'message' | 'new_work';
  attachments?: Schema['ConversationAttachment'][];
  images?: Schema['SessionImage'][];
  knowledgeDocuments?: string[];
  knowledgeSelections?: Schema['ConversationKnowledgeSelection'][];
  useKnowledge?: boolean;
}) {
  let id = options.id;
  if (!id) {
    const receipt = await savedCommand(`${options.key}:import`, {request_id: crypto.randomUUID(), target_id: '', expected_revision: 0, kind: 'import_conversation', payload: {legacy_name: options.name}});
    id = receipt.conversation?.id ?? '';
    if (!id) throw new Error(options.error);
    options.imported(id);
  }
  const key = `${options.key}:input`;
  const payload: Schema['ConversationInboxInput'] = {content: options.content, model: options.model, origin: options.origin};
  if(options.responseProfile)payload.response_profile=options.responseProfile;
  if(options.maxTokens!==undefined)payload.max_tokens=options.maxTokens;
  if (options.attachments?.length) Object.assign(payload, {source_session: options.name, attachments: options.attachments});
  if (options.images?.length) payload.images = options.images;
  if(options.knowledgeSelections?.length) payload.knowledge_selections=options.knowledgeSelections;
  if (options.knowledgeDocuments?.length) payload.knowledge_documents = options.knowledgeDocuments;
  if (options.useKnowledge) payload.use_knowledge = true;
  if (options.intent) payload.intent = options.intent;
  if (options.question) Object.assign(payload, {intent: 'answer', question_id: options.question.id, focused_work_id: options.question.work_id});
  // Recover the original dispatch before preparing any new configuration. A
  // lost acknowledgment must not turn retry into a second settings mutation.
  let pending: Schema['ConversationInputCommand'] | undefined;
  let unresolved: Schema['ConversationInputCommand'] | undefined;
  try {
    const stored = JSON.parse(readDraft(key)) as Schema['ConversationInputCommand'];
    const sameText = stored.payload?.content === options.content && stored.payload?.model === options.model && stored.payload?.origin === options.origin
      && stored.payload?.response_profile === payload.response_profile && stored.payload?.max_tokens === payload.max_tokens
      && JSON.stringify(stored.payload?.attachments ?? []) === JSON.stringify(payload.attachments ?? [])
      && JSON.stringify(stored.payload?.images ?? []) === JSON.stringify(payload.images ?? [])
      && JSON.stringify(stored.payload?.knowledge_selections ?? []) === JSON.stringify(payload.knowledge_selections ?? [])
      && JSON.stringify(stored.payload?.knowledge_documents ?? []) === JSON.stringify(payload.knowledge_documents ?? [])
      && !!stored.payload?.use_knowledge === !!payload.use_knowledge;
    if (stored.target_id === id && stored.kind === 'submit_input' && !sameText) unresolved = stored;
    if (stored.target_id === id && stored.kind === 'submit_input' && sameText) pending = stored;
  } catch { /* No unacknowledged input. */ }
  if (unresolved) {
    // Editing the draft or changing its source is not confirmation that the
    // preceding request failed. Observe its receipt before admitting new input.
    const observed=(await canonicalInputs(id)).find(input=>input.request_id===unresolved.request_id);
    if (!observed || JSON.stringify(observed.input)!==JSON.stringify(unresolved.payload)) throw new Error(options.error);
    clearSubmittedDraft(key,JSON.stringify(unresolved));
  }
  if (pending) return dispatchInput(key, pending, options.error);
  let conversation = await api.conversations.get(id);
  if (options.question) return dispatchInput(key, {request_id: crypto.randomUUID(), target_id: id, expected_revision: conversation.revision, kind: 'submit_input', payload}, options.error);
  const preferences: Schema['ConversationPreferences'] = {...conversation.preferences, model: options.model, profile_id: options.profile?.id ?? '', profile_revision: options.profile?.revision ?? 0, restrictions: intersectConversationRestrictions(conversation.preferences.restrictions ?? {}, options.restrictions)};
  if (readDraft(`${options.key}:configuration`) || JSON.stringify(conversation.preferences) !== JSON.stringify(preferences)) {
    await savedCommand(`${options.key}:configuration`, {request_id: crypto.randomUUID(), target_id: id, expected_revision: conversation.revision, kind: 'configure_conversation', payload: {preferences}});
    conversation = await api.conversations.get(id);
  }
  if (options.folder === null || options.folder?.project_context) {
    const receipt = await savedCommand(`${options.key}:folder`, {request_id: crypto.randomUUID(), target_id: id, expected_revision: conversation.revision, kind: options.folder ? 'use_project_folder' : 'clear_project_folder', payload: options.folder ? {folder: options.folder.reference} : {}});
    if (!receipt.conversation) throw new Error(options.error);
    conversation = receipt.conversation;
    options.folderApplied?.();
  }
  return dispatchInput(key, {request_id: crypto.randomUUID(), target_id: id, expected_revision: conversation.revision, kind: 'submit_input', payload}, options.error);
}

async function dispatchInput(key: string, command: Schema['ConversationInputCommand'], invalidResponse: string) {
  const encoded = JSON.stringify(command); writeDraft(key, encoded);
  try {
    const receipt = await api.conversations.submitInput(command);
    if (!receipt.input) throw new Error(invalidResponse);
    clearSubmittedDraft(key, encoded); return receipt.input;
  } catch (error) {
    if (error instanceof APIError && error.status === 409) clearSubmittedDraft(key, encoded);
    throw error;
  }
}

export async function canonicalInputs(id: string, signal?: AbortSignal) {
  const inputs: CanonicalInput[] = [];
  for (let after = 0; ;) {
    const page = await api.conversations.inputs(id, after, 100, signal);
    inputs.push(...page.items);
    if (!page.has_more) return inputs;
    after = page.next_cursor;
  }
}
