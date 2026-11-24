import { expect, test } from '@playwright/test';
import { createHash } from 'node:crypto';
import { endpointHistoryAPI, validateEndpointReceiptHistory, validateEndpointPolicyHistory, type EndpointReceiptHistory, type EndpointPolicyHistory } from '../src/api/endpoint-history';

const pin = 'a'.repeat(64), journal = 'b'.repeat(64), history = 'c'.repeat(64);
const hash = (id: string) => createHash('sha256').update(id).digest('hex');
const receipt = (id: string) => ({ version: 1 as const, archive_id: pin, archived_by: 'administrator', archived_at: '2026-11-22T00:00:00Z', record: { request_id: id, review_id: pin, actor: 'administrator', epoch: pin, before: pin, after: journal, endpoint: 'fixture@r1', state: 'committed' as const, created_at: '2026-11-21T00:00:00Z', resolved_at: '2026-11-21T00:00:01Z', resolved_by: 'administrator', resolved_epoch: pin } });
const receipts = Array.from({length: 32}, (_, index) => receipt('request-' + index)).sort((a,b) => hash(a.record.request_id).localeCompare(hash(b.record.request_id)));
const page: EndpointReceiptHistory = { journal_revision: journal, archive_id: pin, receipts, next: hash(receipts.at(-1)!.record.request_id) };
const policy: EndpointPolicyHistory = { revision: history, history: { version: 1, actor: 'administrator', at: '2026-11-22T00:00:00Z', configuration_revision: pin, snapshot: { version: 1, revision: journal, restore_epoch: pin, decisions: { 'fixture@old': { pin, enabled: true, actor: 'administrator', at: '2026-11-21T00:00:00Z' } } } } };

test('receipt history verifies lexical identity order and exact continuation without writes or retry', async () => {
 let calls=0;
 const api=endpointHistoryAPI(async <T>(path:string,init?:RequestInit)=>{calls++;expect(init?.method).toBeUndefined();expect(path).toBe('/api/v2/inference-endpoints/configuration/history?journal_revision='+journal);return page as T;});
 expect(await api.history(journal)).toEqual(page);expect(calls).toBe(1);
 await expect(api.history(undefined,pin)).rejects.toThrow();expect(calls).toBe(1);
 const next=endpointHistoryAPI(async <T>(path:string)=>{expect(path).toContain('after='+page.next);return {journal_revision:journal,archive_id:pin,receipts:[]} as T;});
 expect((await next.history(journal,page.next)).receipts).toHaveLength(0);
 let failed=0;const lost=endpointHistoryAPI(async()=>{failed++;throw Error('connection lost');});
 await expect(lost.history(journal)).rejects.toThrow('connection lost');expect(failed).toBe(1);
});

test('receipt history rejects mismatched journal, duplicate or reordered receipts and forged continuation', async () => {
 for(const changed of [
  {...page,journal_revision:pin}, {...page,archive_id:history}, {...page,receipts:[receipts[0],receipts[0]],next:undefined},
  {...page,receipts:[...receipts].reverse()}, {...page,next:pin}, {...page,receipts:receipts.slice(1)},
  {...page,receipts:[{...receipts[0],record:{...receipts[0].record,state:'published'}}],next:undefined},
  {...page,receipts:[{...receipts[0],archived_by:''}],next:undefined}
 ]) await expect(validateEndpointReceiptHistory(changed as EndpointReceiptHistory,journal)).rejects.toThrow();
 await expect(validateEndpointReceiptHistory(page,journal,page.next)).rejects.toThrow();
});

test('policy history binds the requested digest and validates historical authority fields', async () => {
 let calls=0;const api=endpointHistoryAPI(async <T>(path:string,init?:RequestInit)=>{calls++;expect(init?.method).toBeUndefined();expect(path.endsWith('?revision='+history)).toBe(true);return policy as T;});
 expect(await api.policyHistory(history)).toEqual(policy);expect(calls).toBe(1);
 expect(validateEndpointPolicyHistory(null)).toBeNull();expect(()=>validateEndpointPolicyHistory(null,history)).toThrow();
 const snapshot=policy.history.snapshot;
 for(const changed of [
  {...policy,revision:pin}, {...policy,history:{...policy.history,actor:''}},
  {...policy,history:{...policy.history,snapshot:{...snapshot,version:2}}},
  {...policy,history:{...policy.history,snapshot:{...snapshot,decisions:42}}},
  {...policy,history:{...policy.history,snapshot:{...snapshot,decisions:{'fixture@old':{...snapshot.decisions['fixture@old'],review_required:true}}}}},
  {...policy,history:{...policy.history,snapshot:{...snapshot,previous_snapshot:pin}}}
 ]) expect(()=>validateEndpointPolicyHistory(changed as EndpointPolicyHistory,history)).toThrow();
});
