import type { components } from './schema.generated';

type Schema = components['schemas'];
type Request = <T>(path: string, init?: RequestInit) => Promise<T>;
const path = (id: string) => `/api/v2/conversations/${encodeURIComponent(id)}`;

// Uses the ordinary authenticated transport. Reads never submit commands and
// failed commands retain the caller's request identity for an explicit retry.
export function conversationAPI(request: Request) {
  return {
    result: (id: string, offset = 0, limit = 4096, signal?: AbortSignal) => request<Schema['ConversationResultPage']>(`/api/v2/results/${encodeURIComponent(id)}?${new URLSearchParams({offset: String(offset), limit: String(limit)})}`, {signal}),
    workChanges: (id: string, after = '', signal?: AbortSignal) => request<Schema['WorkChangeList']>(`/api/v2/work-items/${encodeURIComponent(id)}/changes?${new URLSearchParams({after, limit: '32'})}`, {signal}),
    workChange: (id: string, change: string, view: Schema['WorkChangeDetail']['view'] = 'files', offset = 0, hash = '', signal?: AbortSignal, artifact = '') => request<Schema['WorkChangeDetail']>(`/api/v2/work-items/${encodeURIComponent(id)}/changes/${encodeURIComponent(change)}?${new URLSearchParams({view, offset: String(offset), hash, artifact, limit: view === 'content' || view === 'preserved' ? '65536' : '32'})}`, {signal}),
    userPreferences: (signal?: AbortSignal) => request<Schema['UserPreferences']>('/api/v2/user-preferences', {signal}),
    configureUserPreferences: (command: Schema['UserPreferencesCommand'], signal?: AbortSignal) => request<Schema['ConversationCommandReceipt']>('/api/v2/user-preferences/commands', {method: 'POST', body: JSON.stringify(command), signal}),
    worker: (id: string, signal?: AbortSignal) => request<Schema['WorkerSession']>(`/api/v2/workers/${encodeURIComponent(id)}`, {signal}),
    workerMessages: (id: string, after = 0, limit = 100, signal?: AbortSignal) => request<Schema['WorkerMessagePage']>(`/api/v2/workers/${encodeURIComponent(id)}/messages?${new URLSearchParams({after: String(after), limit: String(limit)})}`, {signal}),
    workerCommand: (command: Schema['WorkerCommand'], signal?: AbortSignal) => request<Schema['ConversationCommandReceipt']>(`/api/v2/workers/${encodeURIComponent(command.target_id)}/commands`, {method: 'POST', body: JSON.stringify(command), signal}),
    workCommand: (command: Schema['WorkControlCommand'], signal?: AbortSignal) => request<Schema['ConversationCommandReceipt']>(`/api/v2/work-items/${encodeURIComponent(command.target_id)}/commands`, {method: 'POST', body: JSON.stringify(command), signal}),
    project: (id: string, revision?: number, signal?: AbortSignal) => request<Schema['ConversationProject']>(`/api/v2/projects/${encodeURIComponent(id)}${revision === undefined ? '' : `?revision=${revision}`}`, {signal}),
    projectCommand: (command: Schema['ProjectCommand'], signal?: AbortSignal) => request<Schema['ConversationCommandReceipt']>(command.target_id ? `/api/v2/projects/${encodeURIComponent(command.target_id)}/commands` : '/api/v2/projects', {method: 'POST', body: JSON.stringify(command), signal}),
    list: (after = '', signal?: AbortSignal) => request<Schema['CanonicalConversationPage']>(`/api/v2/conversations?${new URLSearchParams({after})}`, {signal}),
    get: (id: string, signal?: AbortSignal) => request<Schema['CanonicalConversation']>(path(id), {signal}),
    inputs: (id: string, after = 0, limit = 100, signal?: AbortSignal) => request<Schema['ConversationInboxPage']>(`${path(id)}/inputs?${new URLSearchParams({after: String(after), limit: String(limit)})}`, {signal}),
    input: (id: string, inputId: string, signal?: AbortSignal) => request<Schema['ConversationInboxEntry']>(`${path(id)}/inputs/${encodeURIComponent(inputId)}`, {signal}),
    submitInput: (command: Schema['ConversationInputCommand'], signal?: AbortSignal) => request<Schema['ConversationCommandReceipt']>(`${path(command.target_id)}/inputs`, {method: 'POST', body: JSON.stringify(command), signal}),
    inputCommand: (id: string, command: Schema['ConversationInputControlCommand'], signal?: AbortSignal) => request<Schema['ConversationCommandReceipt']>(`${path(id)}/inputs/${encodeURIComponent(command.target_id)}/commands`, {method: 'POST', body: JSON.stringify(command), signal}),
    configuration: (id: string, signal?: AbortSignal) => request<Schema['EffectiveConversationPreferences']>(`${path(id)}/configuration`, {signal}),
    events: (id: string, after = 0, limit = 100, signal?: AbortSignal) => request<Schema['ControlEventPage']>(`${path(id)}/events?${new URLSearchParams({after: String(after), limit: String(limit)})}`, {signal}),
    command: (command: Schema['ConversationCommand'], signal?: AbortSignal) => request<Schema['ConversationCommandReceipt']>(command.target_id ? `${path(command.target_id)}/commands` : '/api/v2/conversations', {method: 'POST', body: JSON.stringify(command), signal}),
  };
}
