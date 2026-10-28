import type { components, operations } from './schema.generated';
import { readSessionStream, type SessionEvent } from './session-stream';
import { readAgentStream } from './agent-stream';
import { workflowText } from '../i18n/workflow';

export type Model = components['schemas']['Model'];
export type ModelPackageManifest = components['schemas']['ModelPackageManifest'];
export type ModelPackageState = components['schemas']['ModelPackageState'];
export type ModelCategory = components['schemas']['ModelCategory'];
export type TypedCatalogModel = components['schemas']['TypedCatalogModel'];
export type InstalledTypedModel = components['schemas']['InstalledTypedModel'];
export type ModelResolution = components['schemas']['ModelResolution'];
export type PackageDiscovery = components['schemas']['PackageDiscovery'];
export type ModelOperation = components['schemas']['ModelOperation'];
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
export type AgentProfile = components['schemas']['AgentProfile'];
export type AgentProfileSettings = components['schemas']['AgentProfileSettings'];
export type AgentSkill = components['schemas']['AgentSkill'];
export type AgentSkillReference = components['schemas']['AgentSkillReference'];
export type AgentMemoryEntry = components['schemas']['AgentMemoryEntry'];
export type AgentMemoryConsent = components['schemas']['AgentMemoryConsent'];
export type AgentSchedule = components['schemas']['AgentSchedule'];
export type AgentScheduleWrite = components['schemas']['AgentScheduleWrite'];
export type AgentScheduleOccurrence = components['schemas']['AgentScheduleOccurrence'];
export type AgentSkillInspection = {skill:AgentSkill;bundle:components['schemas']['AgentSkillBundle'];executable:false};
export type EffectiveAgentProfile = components['schemas']['EffectiveAgentProfile'];
export type AgentCapabilityIdentity = components['schemas']['AgentCapabilityIdentity'];
export type AgentSelection = components['schemas']['AgentSelection'];
export type ExecutionReference = components['schemas']['ExecutionReference'];
export type ExecutionWorkspace = components['schemas']['ExecutionWorkspace'];
export type AgentResourceRestrictions = components['schemas']['AgentResourceRestrictions'];
export type LinkedAgentTurn = components['schemas']['LinkedAgentTurn'];
export type AgentTurnRequest = NonNullable<operations['generateSessionTurn']['requestBody']>['content']['application/json'] & { agent: AgentSelection; request_id: string };
export type AgentTool = { name: string; description: string; enabled: boolean; source: string; capability?: { name: string; namespace: string; source: string; kind: string; risk: string; description?: string } };
export type MCPServer = { name: string; url?: string; transport: string; tools: number; status: string };
export type RunSummary = { id: string; status: string; started_at: string; updated_at: string; event_count: number; deletable?: boolean; data?: Record<string, unknown> };
export type RunEvent = { id: string; run_id: string; sequence: number; type: string; time: string; data?: Record<string, any> };
export type RAGStatus = components['schemas']['RAGStatus'];
export type ComputerStatus = { available: boolean; emergency_stop: boolean; active_sessions: number };
export type ComputerModelCheck = components['schemas']['ComputerModelCheck'];
export type ComputerSession = { id: string; origin: string; driver?: 'browser' | 'windows-uia' | 'macos-accessibility' | 'linux-atspi'; approval_mode: 'ask_every_time' | 'scoped_changes' | 'full_task'; remaining_actions: number; expires_at?: string; state?: 'ready' | 'in_use' | 'finished' | 'exhausted' | 'awaiting_consent'; run_id?: string; input_id?:string; desktop?:{workspace:string;binding:Record<string,unknown>} };
export type ExternalIntegration = components['schemas']['IntegrationStatus'];
export type IntegrationSetup = components['schemas']['IntegrationSetup'];
export type SystemConfig = { version: string; inference_slots: number; multi_user_mode: boolean; require_auth: boolean; guest_access: boolean; features: Record<string, boolean> };
export type SpeechProfile = { id: string; revision: string; name: string; architecture: string; adapter: string; capabilities: string[]; languages?: string[]; voices?: { id: string; language: string }[]; available: boolean; smoke_tested: boolean; issue?: string };
export type AudioStatus = { profiles?: SpeechProfile[]; asr?: { available: boolean; whisper_path?: string; issue?: string }; tts?: { available: boolean; piper_path?: string; voices?: number; issue?: string } };

export class APIError extends Error {
  constructor(message: string, readonly status: number, readonly data?: Record<string, any>) { super(message); }
}

let durableAgentEvents = false;
let agentWorkspace = '';
const agentCursors = new Map<string, string>();
let agentReplayScope = '', agentReplayGeneration = 0;
export function setAgentReplayScope(scope: string) {
  if (scope === agentReplayScope) return;
  agentReplayScope = scope; agentReplayGeneration++; agentCursors.clear();
}

async function request<T>(path: string, init?: RequestInit, timeout = 30_000, decode?: (response: Response) => Promise<T>): Promise<T> {
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
      const issues = Array.isArray(parsed.issues) ? parsed.issues.map((issue: unknown) => {
        if (!issue || typeof issue !== 'object' || !('message' in issue) || typeof issue.message !== 'string') return '';
        return issue.message;
      }).filter(Boolean).slice(0, 4).join(' ') : '';
      const candidate = parsed.error?.message ?? parsed.error ?? parsed.message ?? (issues || undefined);
      if (typeof candidate === 'string') message = candidate;
    } catch { /* text response */ }
    if (response.status === 401 && !path.startsWith('/v1/auth/') && path !== '/v1/users/me') window.dispatchEvent(new Event('offgrid:unauthenticated'));
    throw new APIError(message, response.status, data);
  }
  if (decode) return await decode(response);
  if (!response.headers.get('content-type')?.includes('application/json')) throw new APIError('Unexpected service response. Check the backend address and version.', 502);
  return await response.json() as T;
  } catch (reason) {
    if (deadline.signal.aborted) throw new APIError(workflowText().timeout, 408);
    throw reason;
  } finally { clearTimeout(timer); }
}

export const api = {
  typedModels: async (category?: ModelCategory, signal?: AbortSignal) => (await request<{ models: InstalledTypedModel[] }>(`/api/v2/models?${new URLSearchParams(category ? { category } : {})}`, { signal })).models,
  typedCatalog: (category: ModelCategory, query?: string, signal?: AbortSignal) => request<{ models: TypedCatalogModel[]; repositories: { id: string; downloads: number; likes: number; size_bytes?: number }[] }>(`/api/v2/models/catalog?${new URLSearchParams(query === undefined ? { category } : { category, source: 'huggingface', q: query })}`, { signal }),
  discoverPackage: (repository: string, signal?: AbortSignal) => request<PackageDiscovery>(`/api/v2/models/discover?${new URLSearchParams({ repository })}`, { signal }, 180_000),
  resolvePackage: (input: components['schemas']['ResolveModelRequest'], signal?: AbortSignal) => request<ModelResolution>('/api/v2/models/resolve', { method: 'POST', body: JSON.stringify(input), signal }, 180_000),
  modelOperations: async (signal?: AbortSignal) => (await request<{ operations: ModelOperation[] }>('/api/v2/models/operations', { signal })).operations,
  installPackage: (resolution: string, requestID: string) => request<ModelOperation>('/api/v2/models/operations', { method: 'POST', body: JSON.stringify({ action: 'install', resolution_id: resolution, request_id: requestID }) }),
  repairPackage: (sourceOperationID: string, requestID: string) => request<ModelOperation>('/api/v2/models/operations', { method: 'POST', body: JSON.stringify({ action: 'repair', source_operation_id: sourceOperationID, request_id: requestID }) }),
  controlModelOperation: (id: string, action: 'cancel' | 'resume' | 'discard') => request<ModelOperation>(`/api/v2/models/operations/${encodeURIComponent(id)}/${action}`, { method: 'POST', body: '{}' }),
  modelPackages: async (signal?: AbortSignal) => (await request<{ packages: ModelPackageState[] }>('/api/v2/models/packages', { signal })).packages,
  importModelPackage: (body: FormData, signal: AbortSignal) => request<ModelPackageState>('/api/v2/models/packages', { method: 'POST', body, signal }, 30 * 60_000),
  verifyModelPackage: (id: string, revision: string, signal: AbortSignal) => request<ModelPackageState>(`/api/v2/models/packages/${encodeURIComponent(id)}/${encodeURIComponent(revision)}/verify`, { method: 'POST', body: '{}', signal }, 30 * 60_000),
  removeModelPackage: (id: string, revision: string, signal: AbortSignal) => request<{ removed: boolean }>(`/api/v2/models/packages/${encodeURIComponent(id)}/${encodeURIComponent(revision)}/remove`, { method: 'POST', body: '{}', signal }),
  searchModels: (query: string, signal: AbortSignal, category?: ModelCategory) => request<{ results: DiscoveredModel[]; total: number }>(`/v1/search?${new URLSearchParams(category ? { query, category } : { query })}`, { signal }),
  modelFiles: (repo: string, signal: AbortSignal) => request<{ repo: string; files: DiscoveredFile[] }>(`/v1/search/files?${new URLSearchParams({ repo })}`, { signal }),
  systemIdentity: async (signal?:AbortSignal) => {
    const identity = await request<components['schemas']['SystemIdentity']>('/api/v2/system',{signal});
    if (agentWorkspace !== (identity.workspace_id ?? '')) { agentCursors.clear(); agentReplayGeneration++; }
    agentWorkspace = identity.workspace_id ?? '';
    durableAgentEvents = Array.isArray(identity.capabilities) && identity.capabilities.includes('durable-agent-events-v2');
    return identity;
  },
  health: () => request<{ status: string; version?: string }>('/health'),
  audioStatus: (signal?: AbortSignal) => request<AudioStatus>('/v1/audio/status', { signal }),
  transcribeAudio: async (blob: Blob, filename: string, signal?: AbortSignal, selection?: { model?: string }) => {
    const form = new FormData(); form.append('file', blob, filename); form.append('response_format', 'json');
    if (selection?.model) form.append('model', selection.model);
    return request<{ text: string }>('/v1/audio/transcriptions', { method: 'POST', body: form, signal }, 300_000);
  },
  synthesizeSpeech: async (input: string, signal?: AbortSignal, selection?: { model?: string; voice?: string }) => {
    const response = await fetch('/v1/audio/speech', { method: 'POST', signal, credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ input, response_format: 'wav', speed: 1, ...selection }) });
    if (!response.ok) {
      const body = await response.text(); let message = 'Speech playback is unavailable.';
      try { const parsed = JSON.parse(body); message = parsed.error?.message ?? parsed.error ?? message; } catch { /* keep safe message */ }
      throw new APIError(message, response.status);
    }
    if (!response.headers.get('Content-Type')?.includes('audio/wav')) throw new APIError('Speech runtime returned an unsupported audio format.', 502);
    const blob = await response.blob();
    if (blob.size <= 44) throw new APIError('Speech runtime returned empty audio.', 502);
    return blob;
  },
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
  deleteSession: (name: string, work?: 'retain' | 'stop') => request<{ success: boolean; work_retained?: boolean }>(`/v1/sessions/${encodeURIComponent(name)}${work ? `?work=${work}` : ''}`, { method: 'DELETE' }),
  generateSession: (name: string, content: string, modelID: string, useKnowledgeBase: boolean, signal?: AbortSignal) => request<{ session: ChatSession; message: SessionMessage }>(`/v1/sessions/${encodeURIComponent(name)}/generate`, {
    method: 'POST', signal, body: JSON.stringify({ content, model_id: modelID, use_knowledge_base: useKnowledgeBase })
  }),
  currentTurn: (name: string, signal?: AbortSignal) => request<{ turn: SessionTurn | null; job?: AgentRun; recovery_required?: boolean }>(`/v1/sessions/${encodeURIComponent(name)}/turn`, { signal }),
  executionWorkspaces: (signal?: AbortSignal) => request<{protocol: number; workspaces: ExecutionWorkspace[]}>('/api/v2/execution/workspaces', {signal}),
  submitAgentTurn: async (name: string, data: AgentTurnRequest, signal?: AbortSignal) => {
    // An older generate endpoint may ignore additive fields. Never let a saved
    // agent selection silently turn into an unrestricted/plain Chat request.
    const identity = await request<components['schemas']['SystemIdentity']>('/api/v2/system', {signal});
    if (!identity.capabilities?.includes('agent-conversations-v1')) throw new APIError('This service does not support agent conversations. Update the service before retrying.', 409);
    if (data.execution && !identity.capabilities?.includes('native-execution-preview-v1')) throw new APIError('This service does not support the selected execution workspace. No work was submitted.', 409);
    return request<LinkedAgentTurn>(`/v1/sessions/${encodeURIComponent(name)}/generate`, { method: 'POST', signal, body: JSON.stringify(data) });
  },
  recoverAgentTurn: (name: string, id: string) => request<LinkedAgentTurn>(`/v1/sessions/${encodeURIComponent(name)}/turn/recover`, { method: 'POST', body: JSON.stringify({id}) }),
  agentProfiles: async (signal?: AbortSignal) => {
    const result = await request<{profiles: AgentProfile[] | null}>('/api/v2/agent-profiles', {signal});
    return result.profiles ?? [];
  },
  agentProfile: (id: string, revision?: number, signal?: AbortSignal) => request<AgentProfile>(`/api/v2/agent-profiles/${encodeURIComponent(id)}${revision ? `?revision=${revision}` : ''}`, {signal}),
  effectiveAgentProfile: (id: string, revision: number, signal?: AbortSignal) => request<EffectiveAgentProfile>(`/api/v2/agent-profiles/${encodeURIComponent(id)}/effective?revision=${revision}`, {signal}),
  agentProfileResources: (signal?: AbortSignal) => request<{tools: Record<string, AgentCapabilityIdentity>; computer_permitted: boolean; connection_scope: 'workspace-admin'}>('/api/v2/agent-profiles/resources', {signal}),
  createAgentProfile: (settings: AgentProfileSettings) => request<AgentProfile>('/api/v2/agent-profiles', {method:'POST',body:JSON.stringify({settings})}),
  updateAgentProfile: (id: string, expected_revision: number, settings: AgentProfileSettings) => request<AgentProfile>(`/api/v2/agent-profiles/${encodeURIComponent(id)}`, {method:'PUT',body:JSON.stringify({expected_revision,settings})}),
  archiveAgentProfile: (id: string, expected_revision: number) => request<{id:string; archived:true}>(`/api/v2/agent-profiles/${encodeURIComponent(id)}/archive`, {method:'POST',body:JSON.stringify({expected_revision})}),
  agentSchedules: (signal?:AbortSignal)=>request<{schedules:AgentSchedule[]}>('/api/v2/agent-schedules',{signal}),
  agentScheduleOccurrences: (id:string,signal?:AbortSignal)=>request<{occurrences:AgentScheduleOccurrence[]}>(`/api/v2/agent-schedules/${encodeURIComponent(id)}/occurrences`,{signal}),
  saveAgentSchedule: (body:AgentScheduleWrite,id?:string)=>request<AgentSchedule>(`/api/v2/agent-schedules${id?`/${encodeURIComponent(id)}`:''}`,{method:id?'PUT':'POST',body:JSON.stringify(body)}),
  disableAgentSchedule: (id:string,revision:number)=>request<AgentSchedule>(`/api/v2/agent-schedules/${encodeURIComponent(id)}/disable`,{method:'POST',body:JSON.stringify({expected_revision:revision})}),
  agentSkills: (signal?:AbortSignal)=>request<{skills:AgentSkill[]}>('/api/v2/agent-skills',{signal}),
  importAgentSkill: (files:components['schemas']['AgentSkillFile'][])=>request<AgentSkill>('/api/v2/agent-skills',{method:'POST',body:JSON.stringify({files})}),
  inspectAgentSkill: (ref:AgentSkillReference,signal?:AbortSignal)=>request<AgentSkillInspection>(`/api/v2/agent-skills/${encodeURIComponent(ref.id)}?revision=${ref.revision}&digest=${encodeURIComponent(ref.digest)}`,{signal}),
  reviewAgentSkill: (ref:AgentSkillReference)=>request<AgentSkill>(`/api/v2/agent-skills/${encodeURIComponent(ref.id)}/review`,{method:'POST',body:JSON.stringify({expected_revision:ref.revision,digest:ref.digest})}),
  agentMemory: (profile:string,signal?:AbortSignal)=>request<{consent:AgentMemoryConsent;entries:AgentMemoryEntry[];deletion_scope:string}>(`/api/v2/agent-memory/${encodeURIComponent(profile)}`,{signal}),
  changeAgentMemoryConsent: (profile:string,expected_revision:number,enabled:boolean)=>request<AgentMemoryConsent>(`/api/v2/agent-memory/${encodeURIComponent(profile)}/consent`,{method:'PUT',body:JSON.stringify({expected_revision,enabled})}),
  writeAgentMemory: (profile:string,write:components['schemas']['AgentMemoryWrite'])=>request<AgentMemoryEntry>(`/api/v2/agent-memory/${encodeURIComponent(profile)}`,{method:'POST',body:JSON.stringify(write)}),
  deleteAgentMemory: (profile:string,entry:AgentMemoryEntry)=>request<{deleted:true;historical_evidence_erased:false}>(`/api/v2/agent-memory/${encodeURIComponent(profile)}/${encodeURIComponent(entry.id)}?revision=${entry.revision}`,{method:'DELETE'}),
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
  downloadProgress: (signal?: AbortSignal) => request<Record<string, DownloadProgress>>('/v1/models/download/progress', { signal }),
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
  ragStatus: (signal?: AbortSignal) => request<RAGStatus>('/v1/rag/status', { signal }),
  enableRAG: (embeddingModel: string) => request<{ success: boolean; message: string }>('/v1/rag/enable', {
    method: 'POST', body: JSON.stringify({ embedding_model: embeddingModel })
  }),
  disableRAG: () => request<{ success: boolean }>('/v1/rag/disable', { method:'POST',body:'{}' }),
  stats: async () => (await request<Record<string, any> | null>('/v1/stats')) ?? {},
  systemConfig: () => request<SystemConfig>('/v1/system/config'),
  computerStatus: () => request<ComputerStatus>('/api/v2/computer/status'),
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
  emergencyStop: () => request<{ status: string }>('/api/v2/computer/stop', { method: 'POST', body: '{}' }),
  stopComputerSession: (session_id: string) => request<{ status: string }>('/api/v2/computer/sessions/stop', {method: 'POST', body: JSON.stringify({session_id})}),
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
  computerSessions: async () => {
    const data = await request<{ sessions: ComputerSession[] }>('/api/v2/computer/sessions');
    if (!Array.isArray(data.sessions) || data.sessions.some(s => !s || typeof s.id !== 'string' || typeof s.origin !== 'string' || !Number.isFinite(s.remaining_actions))) throw new APIError('Invalid browser session response. Check the service version.', 502);
    return data;
  },
  checkComputerModel: (model: string, signal: AbortSignal, driver='browser') => request<ComputerModelCheck>('/api/v2/computer/model-check', { method: 'POST', body: JSON.stringify({model,driver}), signal }, 600_000),
  job: (id: string, signal?: AbortSignal) => request<AgentRun>(`/api/v2/jobs/${encodeURIComponent(id)}`, { signal }),
  jobs: async (signal?: AbortSignal) => (await request<AgentTask[] | null>('/api/v2/jobs', { signal })) ?? [],
  deleteJob: (id: string) => request<{ success: boolean }>(`/api/v2/jobs/${encodeURIComponent(id)}`, {method: 'DELETE'}),
  exportJob: (id: string) => request<AgentRun>(`/api/v2/jobs/${encodeURIComponent(id)}/export`),
  taskArtifact: (id: string, digest: string, signal: AbortSignal) => request<Blob>(`/api/v2/jobs/${encodeURIComponent(id)}/artifact?digest=${encodeURIComponent(digest)}`, { signal }, 30_000, async response => {
    // Matches the existing bounded task-artifact contract, not arbitrary URLs.
    if (!response.headers.get('content-type')?.includes('application/octet-stream') || !response.body) throw new APIError('Unexpected artifact response.', 502);
    const reader = response.body.getReader(), chunks: Uint8Array<ArrayBuffer>[] = [];
    let length = 0;
    try {
      for (;;) {
        const { value, done } = await reader.read(); if (done) break;
        length += value.byteLength;
        if (length > 128 * 1024) throw new APIError('Artifact exceeds the supported size.', 502);
        chunks.push(value);
      }
      return new Blob(chunks, { type: 'application/octet-stream' });
    } finally { await reader.cancel().catch(() => {}); reader.releaseLock(); }
  }),
  jobAction: (id: string, action: 'approve' | 'deny' | 'cancel' | 'pause' | 'resume' | 'takeover' | 'steer' | 'reconnect' | 'reconcile', data: {approval_id?: string; call_id?: string; result?: string; request_id?: string; instruction?: string} = {}) => request<AgentRun>(`/api/v2/jobs/${encodeURIComponent(id)}/${action}`, {method: 'POST', body: JSON.stringify(data)}),
  submitJob: (prompt: string, model: string, request_id: string, computer_approval_mode?: ComputerSession['approval_mode'],computer_scope?:'desktop'|'selected_window') => request<AgentRun>('/api/v2/jobs', {method: 'POST', body: JSON.stringify({prompt, model, request_id, computer_approval_mode,computer_scope})}),
  // Default admission: 5 min, then tool check: 3 min, optional vision: 90 s.
  // Do not abort a cold load using the ordinary short API request deadline.
  resolveJobInput: (id: string, input_id: string, computer_session: string) => request<AgentRun>(`/api/v2/jobs/${encodeURIComponent(id)}/input`, {method: 'POST', body: JSON.stringify({input_id, computer_session})}, 600_000),
  resolveWorkspaceInput: (id: string, input_id: string, execution: ExecutionWorkspace['reference']) => request<AgentRun>(`/api/v2/jobs/${encodeURIComponent(id)}/input`, {method: 'POST', body: JSON.stringify({input_id, execution})}, 600_000),
  streamAgent: async (id: string, onSnapshot: (run: AgentRun) => void | boolean, onHeartbeat: () => void, signal: AbortSignal) => {
    const replay = durableAgentEvents;
    const generation = agentReplayGeneration;
    const headers: Record<string,string> = { Accept: 'text/event-stream' };
    const cursor = agentCursors.get(id);
    if (replay && cursor) headers['Last-Event-ID'] = cursor;
    const response = await fetch(`${replay ? '/api/v2/jobs' : '/v1/agents/tasks'}/${encodeURIComponent(id)}/events`, { credentials: 'same-origin', headers, signal });
    if (!response.ok) throw new APIError('Agent progress unavailable', response.status);
    return readAgentStream(response, id, run => {
      if (signal.aborted || generation !== agentReplayGeneration) return;
      if (onSnapshot(run) === false) return;
      if (replay && run.event_cursor) {
        agentCursors.delete(id);
        agentCursors.set(id,run.event_cursor);
        if (agentCursors.size > 100) agentCursors.delete(agentCursors.keys().next().value!);
      }
    }, onHeartbeat);
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
  removeMCP: (name: string) => request<{ status: string; server: string; tools_removed: number }>(`/v1/agents/mcp?name=${encodeURIComponent(name)}`, {
    method: 'DELETE'
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
