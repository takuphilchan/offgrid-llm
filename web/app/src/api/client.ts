import type { components } from './schema.generated';
import { readSessionStream, type SessionEvent } from './session-stream';
import { readAgentStream } from './agent-stream';
import { workflowText } from '../i18n/workflow';

export type Model = components['schemas']['Model'];
export type Document = components['schemas']['Document'];
export type ChatMessage = components['schemas']['ChatMessage'];
export type SessionMessage = components['schemas']['SessionMessage'];
export type SessionTurn = components['schemas']['SessionTurn'];
export type ChatSession = components['schemas']['ChatSession'];
export type CatalogModel = components['schemas']['CatalogModel'];
export type DiscoveredModel = components['schemas']['DiscoveredModel'];
export type DiscoveredFile = components['schemas']['DiscoveredFile'];
export type DownloadProgress = components['schemas']['DownloadProgress'];
export type Verification = components['schemas']['Verification'];
export type PublicUser = components['schemas']['PublicUser'];
export type ToolApproval = components['schemas']['ToolApproval'];
export type AgentStep = components['schemas']['AgentStep'];
export type AgentTask = components['schemas']['AgentTask'];
export type AgentRun = components['schemas']['AgentRunResponse'];
export type AgentTool = { name: string; description: string; enabled: boolean; source: string; capability?: { name: string; namespace: string; source: string; kind: string; risk: string; description?: string } };
export type MCPServer = { name: string; url?: string; transport: string; tools: number; status: string };
export type RunSummary = { id: string; status: string; started_at: string; updated_at: string; event_count: number; data?: Record<string, unknown> };
export type RunEvent = { id: string; run_id: string; sequence: number; type: string; time: string; data?: Record<string, any> };
export type RAGStatus = components['schemas']['RAGStatus'];
export type ComputerStatus = { available: boolean; emergency_stop: boolean; active_sessions: number };
export type ExternalIntegration = components['schemas']['IntegrationStatus'];
export type IntegrationSetup = components['schemas']['IntegrationSetup'];
export type SystemConfig = { version: string; inference_slots: number; multi_user_mode: boolean; require_auth: boolean; guest_access: boolean; features: Record<string, boolean> };

export class APIError extends Error {
  constructor(message: string, readonly status: number, readonly data?: Record<string, any>) { super(message); }
}

async function request<T>(path: string, init?: RequestInit, timeout = 30_000): Promise<T> {
  const deadline = new AbortController();
  const timer = setTimeout(() => deadline.abort(), timeout);
  try {
  const response = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    signal: init?.signal ? AbortSignal.any([init.signal, deadline.signal]) : deadline.signal,
    headers: { ...(init?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }), ...init?.headers }
  });
  if (!response.ok) {
    const body = await response.text();
    let message = `OffGrid returned HTTP ${response.status}. Refresh the workspace before retrying.`;
    let data: Record<string, any> | undefined;
    try {
      const parsed = JSON.parse(body);
      data = parsed;
      const candidate = parsed.error?.message ?? parsed.error ?? parsed.message;
      if (typeof candidate === 'string') message = candidate;
    } catch { /* text response */ }
    if (response.status === 401 && !path.startsWith('/v1/auth/') && path !== '/v1/users/me') window.dispatchEvent(new Event('offgrid:unauthenticated'));
    throw new APIError(message, response.status, data);
  }
  if (!response.headers.get('content-type')?.includes('application/json')) throw new APIError('Unexpected service response. Check the backend address and version.', 502);
  return await response.json() as T;
  } catch (reason) {
    if (deadline.signal.aborted) throw new APIError(workflowText().timeout, 408);
    throw reason;
  } finally { clearTimeout(timer); }
}

export const api = {
  searchModels: (query: string, signal: AbortSignal) => request<{ results: DiscoveredModel[]; total: number }>(`/v1/search?${new URLSearchParams({ query })}`, { signal }),
  modelFiles: (repo: string, signal: AbortSignal) => request<{ repo: string; files: DiscoveredFile[] }>(`/v1/search/files?${new URLSearchParams({ repo })}`, { signal }),
  systemIdentity: () => request<components['schemas']['SystemIdentity']>('/api/v2/system'),
  health: () => request<{ status: string; version?: string }>('/health'),
  currentUser: () => request<components['schemas']['CurrentUser']>('/v1/users/me'),
  login: (username: string, password: string) => request<{ user: PublicUser; expires_at: string; auth_method: string }>('/v1/auth/login', {
    method: 'POST', body: JSON.stringify({ username, password })
  }),
  logout: () => request<{ status: string }>('/v1/auth/logout', { method: 'POST', body: '{}' }),
  models: async () => {
    const result = await request<{ data: Model[] | null }>('/v1/models');
    return Array.isArray(result.data) ? result.data : [];
  },
  sessions: async () => {
    const result = await request<{ sessions: ChatSession[] | null }>('/v1/sessions');
    return Array.isArray(result.sessions) ? result.sessions : [];
  },
  session: (name: string) => request<ChatSession>(`/v1/sessions/${encodeURIComponent(name)}`),
  createSession: (name: string, modelID: string, signal?: AbortSignal) => request<ChatSession>('/v1/sessions', {
    method: 'POST', signal, body: JSON.stringify({ name, model_id: modelID })
  }),
  deleteSession: (name: string) => request<{ success: boolean }>(`/v1/sessions/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  generateSession: (name: string, content: string, modelID: string, useKnowledgeBase: boolean, signal?: AbortSignal) => request<{ session: ChatSession; message: SessionMessage }>(`/v1/sessions/${encodeURIComponent(name)}/generate`, {
    method: 'POST', signal, body: JSON.stringify({ content, model_id: modelID, use_knowledge_base: useKnowledgeBase })
  }),
  currentTurn: (name: string) => request<{ turn: SessionTurn | null }>(`/v1/sessions/${encodeURIComponent(name)}/turn`),
  cancelTurn: (name: string, id: string) => request<{ success: boolean }>(`/v1/sessions/${encodeURIComponent(name)}/turn/cancel`, { method: 'POST', body: JSON.stringify({ id }) }),
  followTurn: async (name: string, id: string, onEvent: (event: SessionEvent) => void, signal: AbortSignal) => {
    const response = await fetch(`/v1/sessions/${encodeURIComponent(name)}/turn/events?id=${encodeURIComponent(id)}`, { credentials: 'same-origin', signal });
    if (!response.ok) throw new APIError('Conversation progress unavailable. Refresh to reconnect.', response.status);
    return readSessionStream(response, onEvent);
  },
  streamSession: async (name: string, content: string, modelID: string, useKnowledgeBase: boolean, profile: string, maxTokens: number, onEvent: (event: SessionEvent) => void, signal: AbortSignal, requestID = crypto.randomUUID()) => {
    const response = await fetch(`/v1/sessions/${encodeURIComponent(name)}/generate`, {
      method: 'POST', credentials: 'same-origin', signal,
      headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
      body: JSON.stringify({ content, model_id: modelID, use_knowledge_base: useKnowledgeBase, stream: true, durable: true, request_id: requestID, profile, max_tokens: maxTokens })
    });
    if (!response.ok) {
      const body = await response.text();
      let message = `${response.status} ${response.statusText}`;
      try { const error = JSON.parse(body); message = error.error?.message ?? error.error ?? message; } catch { /* Do not show an HTML proxy page. */ }
      throw new APIError(message, response.status);
    }
    return readSessionStream(response, onEvent);
  },
  catalog: async () => {
    const result = await request<{ models: CatalogModel[] | null }>('/v1/catalog');
    return Array.isArray(result.models) ? result.models : [];
  },
  downloadModel: (model: Pick<CatalogModel, 'id' | 'repo' | 'file' | 'quant'>, enableKnowledge = false) => request<{ success: boolean; exists?: boolean; status: string; file_name: string }>('/v1/models/download', {
    method: 'POST', body: JSON.stringify({ model_id: model.id, repository: model.repo, file_name: model.file, quantization: model.quant, enable_knowledge: enableKnowledge })
  }),
  downloadProgress: () => request<Record<string, DownloadProgress>>('/v1/models/download/progress'),
  cancelDownload: (fileName: string) => request<{ success: boolean }>('/v1/models/download/cancel', {
    method: 'POST', body: JSON.stringify({ file_name: fileName })
  }),
  deleteModel: (modelID: string) => request<{ success: boolean }>('/v1/models/delete', {
    method: 'DELETE', body: JSON.stringify({ model_id: modelID })
  }),
  verifyModel: (modelID: string) => request<Verification>(`/v1/models/verify?model=${encodeURIComponent(modelID)}`),
  documents: async () => {
    const result = await request<{ documents: Document[] | null; count: number }>('/v1/documents');
    return { ...result, documents: Array.isArray(result.documents) ? result.documents : [] };
  },
  ragStatus: () => request<RAGStatus>('/v1/rag/status'),
  enableRAG: (embeddingModel: string) => request<{ success: boolean; message: string }>('/v1/rag/enable', {
    method: 'POST', body: JSON.stringify({ embedding_model: embeddingModel })
  }),
  disableRAG: () => request<{ success: boolean }>('/v1/rag/disable', { method:'POST',body:'{}' }),
  stats: async () => (await request<Record<string, any> | null>('/v1/stats')) ?? {},
  systemConfig: () => request<SystemConfig>('/v1/system/config'),
  computerStatus: () => request<ComputerStatus>('/v1/computer/status'),
  integrations: async (modelID?: string) => {
    const query = modelID ? `?model=${encodeURIComponent(modelID)}` : '';
    const result = await request<{ provider: string; base_url: string; integrations: ExternalIntegration[] | null }>(`/v1/integrations${query}`);
    return { ...result, integrations: Array.isArray(result.integrations) ? result.integrations : [] };
  },
  integrationSetup: (id: string, modelID?: string) => {
    const query = new URLSearchParams({ base_url: window.location.origin });
    if (modelID) query.set('model', modelID);
    return request<{ integration: ExternalIntegration; setup: IntegrationSetup }>(`/v1/integrations/${encodeURIComponent(id)}/setup?${query}`);
  },
  emergencyStop: () => request<{ status: string }>('/v1/computer/stop', { method: 'POST', body: '{}' }),
  runs: async () => {
    const result = await request<{ runs: RunSummary[] | null }>('/v1/runs');
    return Array.isArray(result.runs) ? result.runs : [];
  },
  runEvents: async (runID: string) => {
    const result = await request<{ events: RunEvent[] | null }>(`/v1/runs/${encodeURIComponent(runID)}/events`);
    return Array.isArray(result.events) ? result.events : [];
  },
  chat: async (model: string, messages: ChatMessage[], useKnowledgeBase: boolean, signal?: AbortSignal) => {
    const result = await request<{ choices: Array<{ message: { content: string } }> }>('/v1/chat/completions', {
      method: 'POST', signal, body: JSON.stringify({ model, messages, use_knowledge_base: useKnowledgeBase })
    });
    return result.choices[0]?.message.content ?? '';
  },
  runAgent: (model: string, prompt: string, style: string) => request<AgentRun>('/v1/agents/run', {
    method: 'POST', body: JSON.stringify({ model, prompt, style, max_iterations: 12, async: true })
  }),
  agentRun: (id: string) => request<AgentRun>(`/v1/agents/tasks/${encodeURIComponent(id)}`),
  deleteAgentRun: (id: string) => request<{ success: boolean }>(`/v1/agents/tasks/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  streamAgent: async (id: string, onSnapshot: (run: AgentRun) => void, onHeartbeat: () => void, signal: AbortSignal) => {
    const response = await fetch(`/v1/agents/tasks/${encodeURIComponent(id)}/events`, { credentials: 'same-origin', headers: { Accept: 'text/event-stream' }, signal });
    if (!response.ok) throw new APIError('Agent progress unavailable', response.status);
    return readAgentStream(response, id, onSnapshot, onHeartbeat);
  },
  agentAction: (id: string, action: 'approve' | 'deny' | 'cancel' | 'resume' | 'reconcile', data: { approval_id?: string; call_id?: string; result?: string } = {}) => request<AgentRun>(`/v1/agents/tasks/${encodeURIComponent(id)}/${action}`, { method: 'POST', body: JSON.stringify({ ...data, async: true }) }),
  agentTasks: async () => {
    const result = await request<AgentTask[] | null>('/v1/agents/tasks');
    return Array.isArray(result) ? result : [];
  },
  agentTools: async () => {
    const result = await request<{ tools: AgentTool[] | null; total: number; enabled_count: number }>('/v1/agents/tools?all=true');
    return { ...result, tools: Array.isArray(result.tools) ? result.tools : [] };
  },
  setAgentToolEnabled: (name: string, enabled: boolean) => request<{ status: string; tool: string; enabled_count: number }>('/v1/agents/tools', {
    method: 'PATCH', body: JSON.stringify({ name, enabled })
  }),
  mcpServers: async () => {
    const result = await request<{ servers: MCPServer[] | null }>('/v1/agents/mcp');
    return Array.isArray(result.servers) ? result.servers : [];
  },
  testMCP: (url: string) => request<{ status: string; tools_count: number }>('/v1/agents/mcp/test', {
    method: 'POST', body: JSON.stringify({ url })
  }),
  connectMCP: (name: string, url: string) => request<{ status: string; server: string; tools_added: number }>('/v1/agents/mcp', {
    method: 'POST', body: JSON.stringify({ name, url })
  }),
  reindexDocument: (documentID: string) => request<{ success: boolean; document: Document }>('/v1/documents/reindex', {
    method: 'POST', body: JSON.stringify({ document_id: documentID })
  }, 5 * 60_000),
  deleteDocument: (id: string) => request<{ success: boolean }>(`/v1/documents/delete?id=${encodeURIComponent(id)}`, { method:'DELETE' }),
  documentSource: (id: string) => request<{ document: Document; content: string; truncated: boolean }>(`/v1/documents/source?id=${encodeURIComponent(id)}`),
  ingest: (file: File) => {
    const form = new FormData();
    form.append('file', file);
    return request<{ success: boolean; document: Document }>('/v1/documents/ingest', { method: 'POST', body: form }, 5 * 60_000);
  }
};
