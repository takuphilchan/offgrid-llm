import type { components } from './schema.generated';
import { clientMessage } from '../i18n/client-errors';
import { endpointHistoryAPI } from './endpoint-history';

type Schema = components['schemas'];
export type EndpointOwnerSnapshot = Schema['InferenceEndpointOwnerSnapshot'];
export type EndpointOwnerInput = Schema['InferenceEndpointOwnerInput'];
export type EndpointOwnerEntry = Schema['InferenceEndpointOwnerEntry'];
export type EndpointOwnerDraft = Schema['InferenceEndpointOwnerDraft'];
export type EndpointOwnerRecord = Schema['InferenceEndpointOwnerRecord'];
type Request = <T>(path: string, init?: RequestInit) => Promise<T>;
const base = '/api/v2/inference-endpoints/configuration';
const invalid = (): never => { throw Error(clientMessage('response')); };
const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0;
const digest = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const name = (value: unknown): value is string => typeof value === 'string' && /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(value);
const timestamp = (value: unknown) => text(value) && Number.isFinite(Date.parse(value));
const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

function origin(value: unknown, remote = false) {
  if (!text(value)) return false;
  try {
    const url = new URL(value);
    return (url.protocol === 'https:' || !remote && url.protocol === 'http:') && !url.username && !url.password && !url.search && !url.hash && url.pathname === '/';
  } catch { return false; }
}

function entry(value: EndpointOwnerEntry): EndpointOwnerEntry {
  if (!value || !name(value.id) || !name(value.revision) || !['ollama', 'vllm'].includes(value.provider ?? '') || typeof value.managed !== 'boolean' || typeof value.credential_configured !== 'boolean' || value.expected_revision !== '' || !origin(value.url, !value.managed) || !text(value.model) || !text(value.runtime_version) || !['gguf', 'mlx_safetensors', 'safetensors'].includes(value.format ?? '') || !/^sha256:[a-f0-9]{64}$/.test(value.model_digest ?? '') || value.deployment_attested !== true || !Number.isSafeInteger(value.context_length) || value.context_length! < 512 || value.context_length! > 1048576) invalid();
  if (!Array.isArray(value.actors) || !value.actors.length || value.actors.length > 128 || new Set(value.actors).size !== value.actors.length || value.actors.some(actor => !text(actor) || actor === '*' || actor.length > 256)) invalid();
  if (value.allowed_ips !== undefined && (!Array.isArray(value.allowed_ips) || value.allowed_ips.some(ip => !text(ip)))) invalid();
  if (value.credential_reference !== undefined && !name(value.credential_reference) || value.managed_template !== undefined && !name(value.managed_template)) invalid();
  if (value.managed && (value.provider !== 'ollama' || value.actors.length !== 1 || value.credential_configured || value.credential_reference)) invalid();
  return value;
}

export function validateEndpointOwnerRecord(value: EndpointOwnerRecord): EndpointOwnerRecord {
  if (!value || !name(value.request_id) || !digest(value.review_id) || !text(value.actor) || !digest(value.epoch) || !digest(value.before) || !digest(value.after) || !text(value.endpoint) || value.endpoint.split('@').length !== 2 || !value.endpoint.split('@').every(name) || !['pending', 'published', 'committed', 'not_applied', 'conflict'].includes(value.state) || !timestamp(value.created_at)) invalid();
  if (['committed', 'not_applied', 'conflict'].includes(value.state) && (!timestamp(value.resolved_at) || !text(value.resolved_by) || !digest(value.resolved_epoch))) invalid();
  if (['pending', 'published'].includes(value.state) && (value.resolved_at !== undefined || value.resolved_by !== undefined || value.resolved_epoch !== undefined)) invalid();
  return value;
}

export function validateEndpointOwner(value: EndpointOwnerSnapshot): EndpointOwnerSnapshot {
	if (value?.history_available !== undefined && typeof value.history_available !== 'boolean') invalid();
	if (value?.journal_revision !== undefined && !digest(value.journal_revision)) invalid();
	const maintenance = value?.policy_maintenance;
	if (maintenance !== undefined && (!maintenance || !digest(maintenance.policy_revision) || !Number.isInteger(maintenance.decision_count) || maintenance.decision_count < 0 || maintenance.decision_count > 64 || !Array.isArray(maintenance.retirable) || maintenance.retirable.length > maintenance.decision_count || new Set(maintenance.retirable).size !== maintenance.retirable.length || maintenance.retirable.some(key => typeof key !== 'string' || key.split('@').length !== 2 || !key.split('@').every(name)))) invalid();
  if (!value || !digest(value.file_revision) || !['workspace', 'external'].includes(value.source) || typeof value.write_available !== 'boolean' || !text(value.actor) || value.write_reason !== undefined && typeof value.write_reason !== 'string') invalid();
  for (const [items, limit] of [[value.entries, 64], [value.credential_references, 32], [value.managed_templates, 32], [value.records, 64]] as const) {
    if (!Array.isArray(items) || items.length > limit) invalid();
  }
  const entries = value.entries.map(value => { entry(value); return `${value.id}@${value.revision}`; });
	if (maintenance?.retirable.some(key => entries.includes(key))) invalid();
  const credentials = value.credential_references.map(value => {
    if (!value || !name(value.id) || !Array.isArray(value.origins) || !value.origins.length || value.origins.length > 16 || value.origins.some(url => !origin(url, true))) invalid();
    return value.id;
  });
  const templates = value.managed_templates.map(value => {
    if (!value || !name(value.id) || !entry(value.entry).managed) invalid();
    return value.id;
  });
  const records = value.records.map(value => validateEndpointOwnerRecord(value).request_id);
  for (const ids of [entries, credentials, templates, records]) if (new Set(ids).size !== ids.length) invalid();
  if (value.write_available && value.records.some(record => ['pending', 'published', 'conflict'].includes(record.state))) invalid();
  return value;
}

export function validateEndpointOwnerDraft(value: EndpointOwnerDraft, input: EndpointOwnerInput): EndpointOwnerDraft {
  if (!value || !digest(value.review_id) || value.expected_revision !== input.expected_revision || !timestamp(value.expires_at) || value.restart_required !== true || value.access_review_required !== true) invalid();
  const proposed = entry(value.entry);
  if (proposed.id !== input.id || proposed.revision !== input.revision || !same(proposed.actors, input.actors) || proposed.deployment_attested !== input.deployment_attested) invalid();
  if (input.managed_template) {
    if (!proposed.managed || proposed.managed_template !== input.managed_template) invalid();
  } else {
    if (proposed.managed) invalid();
    for (const key of ['provider', 'url', 'model', 'model_digest', 'format', 'runtime_version', 'context_length'] as const) if (proposed[key] !== input[key]) invalid();
    if (!same(proposed.allowed_ips ?? [], input.allowed_ips ?? []) || (proposed.credential_reference ?? '') !== (input.credential_reference ?? '') || proposed.credential_configured !== Boolean(input.credential_reference)) invalid();
  }
  return value;
}

export function endpointOwnerAPI(request: Request) {
  const post = <T>(action: string, body: unknown) => request<T>(`${base}/${action}`, { method: 'POST', body: JSON.stringify(body) });
  return {
    ...endpointHistoryAPI(request),
	retireDecisions: async (snapshot: EndpointOwnerSnapshot) => {
	  validateEndpointOwner(snapshot);
	  const maintenance = snapshot.policy_maintenance;
	  if (!maintenance || maintenance.retirable.length === 0) return invalid();
	  const result = await post<Schema['InferenceEndpointPolicyMaintenanceResult']>('retire-decisions', { expected_revision: snapshot.file_revision, expected_policy_revision: maintenance.policy_revision, endpoints: maintenance.retirable });
	  if (!result || !digest(result.policy_revision) || result.policy_revision === maintenance.policy_revision || !digest(result.history_revision) || !Array.isArray(result.retired) || !same([...result.retired].sort(), [...maintenance.retirable].sort())) invalid();
	  return result;
	},
	archive: async (snapshot: EndpointOwnerSnapshot) => {
	  if (!digest(snapshot.journal_revision) || !digest(snapshot.file_revision) || snapshot.records.some(record => !['committed', 'not_applied'].includes(record.state))) invalid();
	  const result = await post<Schema['InferenceEndpointOwnerArchiveResult']>('archive', { expected_revision: snapshot.file_revision, expected_journal_revision: snapshot.journal_revision });
	  if (!result || !Number.isInteger(result.archived) || result.archived !== snapshot.records.length || !digest(result.journal_revision)) invalid();
	  return result;
	},
    read: async (signal?: AbortSignal) => validateEndpointOwner(await request<EndpointOwnerSnapshot>(base, { signal })),
    prepare: async (input: EndpointOwnerInput) => validateEndpointOwnerDraft(await post<EndpointOwnerDraft>('prepare', input), input),
    commit: async (draft: EndpointOwnerDraft, requestID: string, actor: string) => {
      const result = validateEndpointOwnerRecord(await post<EndpointOwnerRecord>('commit', { review_id: draft.review_id, request_id: requestID }));
      if (result.request_id !== requestID || result.review_id !== draft.review_id || result.actor !== actor || result.before !== draft.expected_revision || result.endpoint !== `${draft.entry.id}@${draft.entry.revision}` || !['published', 'committed', 'not_applied'].includes(result.state)) invalid();
      return result;
    },
    reconcile: async (record: EndpointOwnerRecord, expected: string, actor: string) => {
      const result = validateEndpointOwnerRecord(await post<EndpointOwnerRecord>('reconcile', { request_id: record.request_id, expected_revision: expected }));
      for (const key of ['request_id', 'review_id', 'actor', 'epoch', 'before', 'after', 'endpoint', 'created_at'] as const) if (result[key] !== record[key]) invalid();
      if (result.resolved_by !== actor || result.state !== (expected === record.after ? 'committed' : expected === record.before ? 'not_applied' : 'conflict') || result.state === 'conflict') invalid();
      return result;
    }
  };
}
