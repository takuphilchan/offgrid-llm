import type { components } from './schema.generated';
import { clientMessage } from '../i18n/client-errors';
import { validateEndpointOwnerRecord } from './endpoint-owner';

type Schema = components['schemas'];
export type EndpointReceiptHistory = Schema['InferenceEndpointOwnerHistoryPage'];
export type EndpointPolicyHistory = Schema['InferenceEndpointPolicyHistoryPage'];
type Request = <T>(path: string, init?: RequestInit) => Promise<T>;
const base = '/api/v2/inference-endpoints/configuration';
const invalid = (): never => { throw Error(clientMessage('response')); };
const digest = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const actor = (value: unknown) => typeof value === 'string' && value.length > 0 && new TextEncoder().encode(value).length <= 256;
const time = (value: unknown) => typeof value === 'string' && value.length <= 64 && Number.isFinite(Date.parse(value));
const identity = (value: string) => /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}@[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(value);

async function requestDigest(request: string) {
 const value = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(request));
 return Array.from(new Uint8Array(value), byte => byte.toString(16).padStart(2, '0')).join('');
}

export async function validateEndpointReceiptHistory(page: EndpointReceiptHistory, expected?: string, after = '') {
 if (!page || !digest(page.journal_revision) || expected !== undefined && page.journal_revision !== expected || !Array.isArray(page.receipts) || page.receipts.length > 32 || page.archive_id !== undefined && !digest(page.archive_id) || page.next !== undefined && (!digest(page.next) || page.receipts.length !== 32)) return invalid();
 let last = after;
 for (const receipt of page.receipts) {
  if (!receipt || receipt.version !== 1 || !digest(receipt.archive_id) || receipt.archive_id !== page.archive_id || !actor(receipt.archived_by) || !time(receipt.archived_at)) return invalid();
  const record = validateEndpointOwnerRecord(receipt.record);
  if (!['committed', 'not_applied'].includes(record.state) || new TextEncoder().encode(JSON.stringify(receipt)).length > 4096) return invalid();
  const key = await requestDigest(record.request_id);
  if (key <= last) return invalid();
  last = key;
 }
 if (page.next !== undefined && page.next !== last) return invalid();
 return page;
}

export function validateEndpointPolicyHistory(page: EndpointPolicyHistory | null, expected?: string) {
 if (page === null) { if (expected) return invalid(); return null; }
 if (!page || !digest(page.revision) || expected !== undefined && page.revision !== expected || !page.history) return invalid();
 const history = page.history, snapshot = history.snapshot;
 if (history.version !== 1 || !actor(history.actor) || !time(history.at) || !digest(history.configuration_revision) || !snapshot || ![1,2].includes(snapshot.version) || !digest(snapshot.restore_epoch) || !digest(snapshot.revision) || !snapshot.decisions || typeof snapshot.decisions !== 'object' || Array.isArray(snapshot.decisions) || Object.keys(snapshot.decisions).length > 64) return invalid();
 if (snapshot.version === 1 && snapshot.previous_snapshot !== undefined || snapshot.version === 2 && !digest(snapshot.previous_snapshot)) return invalid();
 for (const [key, value] of Object.entries(snapshot.decisions)) {
  if (!identity(key) || !value || !digest(value.pin) || typeof value.enabled !== 'boolean' || !actor(value.actor) || !time(value.at) || value.review_required !== undefined && typeof value.review_required !== 'boolean' || value.enabled && value.review_required) return invalid();
 }
 return page;
}

export function endpointHistoryAPI(request: Request) {
 return {
  history: async (revision?: string, after?: string, signal?: AbortSignal) => {
   if (revision !== undefined && !digest(revision) || after !== undefined && (!digest(after) || !revision)) return invalid();
   const query = new URLSearchParams(); if (revision) query.set('journal_revision', revision); if (after) query.set('after', after);
   return validateEndpointReceiptHistory(await request<EndpointReceiptHistory>(`${base}/history${query.size ? '?' + query : ''}`, { signal }), revision, after);
  },
  policyHistory: async (revision?: string, signal?: AbortSignal) => {
   if (revision !== undefined && !digest(revision)) return invalid();
   return validateEndpointPolicyHistory(await request<EndpointPolicyHistory | null>(`${base}/policy-history${revision ? '?revision=' + revision : ''}`, { signal }), revision);
  }
 };
}
