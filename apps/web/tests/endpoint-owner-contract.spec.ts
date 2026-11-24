import { expect, test } from '@playwright/test';
import { endpointOwnerAPI, validateEndpointOwner, validateEndpointOwnerRecord, type EndpointOwnerInput, type EndpointOwnerEntry, type EndpointOwnerDraft, type EndpointOwnerSnapshot, type EndpointOwnerRecord } from '../src/api/endpoint-owner';

const pin = 'a'.repeat(64), after = 'b'.repeat(64), review = 'c'.repeat(64), epoch = 'd'.repeat(64);
const input: EndpointOwnerInput = { expected_revision: pin, id: 'lab', revision: 'r2', provider: 'ollama', url: 'https://inference.example.test', allowed_ips: ['192.0.2.10'], actors: ['administrator'], model: 'fixture', model_digest: 'sha256:' + 'e'.repeat(64), format: 'gguf', runtime_version: 'fixture-v1', context_length: 4096, deployment_attested: true };
const entry: EndpointOwnerEntry = { ...input, expected_revision: '', managed: false, credential_configured: false };
const draft: EndpointOwnerDraft = { review_id: review, expected_revision: pin, expires_at: '2030-01-01T00:00:00Z', entry, restart_required: true, access_review_required: true };
const record: EndpointOwnerRecord = { request_id: 'write-once', review_id: review, actor: 'administrator', epoch, before: pin, after, endpoint: 'lab@r2', state: 'published', created_at: '2026-11-21T00:00:00Z' };
const snapshot: EndpointOwnerSnapshot = { file_revision: pin, source: 'workspace', write_available: true, actor: 'administrator', entries: [entry], credential_references: [], managed_templates: [], records: [] };
const resolved = (state: 'committed' | 'not_applied'): EndpointOwnerRecord => ({ ...record, state, resolved_at: '2026-11-21T01:00:00Z', resolved_by: 'administrator', resolved_epoch: epoch });

test('decision retirement binds the exact obsolete identities and revisions without retrying uncertainty', async () => {
  const reviewed = { ...snapshot, policy_maintenance: { policy_revision: pin, decision_count: 3, retirable: ['lab@r1', 'retired@r0'] } };
  const result = { retired: ['retired@r0', 'lab@r1'], policy_revision: after, history_revision: review };
  let calls = 0;
  const api = endpointOwnerAPI(async <T>(path: string, init?: RequestInit) => {
    calls++; expect(path.endsWith('/configuration/retire-decisions')).toBe(true);
    expect(JSON.parse(String(init?.body))).toEqual({ expected_revision: pin, expected_policy_revision: pin, endpoints: reviewed.policy_maintenance.retirable });
    return result as T;
  });
  expect(await api.retireDecisions(reviewed)).toEqual(result);
  for (const maintenance of [undefined, { ...reviewed.policy_maintenance, retirable: [] }, { ...reviewed.policy_maintenance, retirable: ['lab@r2'] }, { ...reviewed.policy_maintenance, retirable: ['lab@r1', 'lab@r1'] }, { ...reviewed.policy_maintenance, decision_count: 1 }, { ...reviewed.policy_maintenance, policy_revision: 'bad' }]) {
    await expect(api.retireDecisions({ ...reviewed, policy_maintenance: maintenance })).rejects.toThrow();
  }
  expect(calls).toBe(1);
  for (const changed of [{ retired: ['lab@r2'] }, { retired: ['lab@r1'] }, { policy_revision: pin }, { history_revision: 'bad' }]) {
    const malformed = endpointOwnerAPI(async <T>() => ({ ...result, ...changed } as T));
    await expect(malformed.retireDecisions(reviewed)).rejects.toThrow();
  }
  let lostCalls = 0;
  const lost = endpointOwnerAPI(async () => { lostCalls++; throw Error('lost acknowledgement'); });
  await expect(lost.retireDecisions(reviewed)).rejects.toThrow('lost acknowledgement');
  expect(lostCalls).toBe(1);
});

test('archival validates the reviewed journal and never retries a lost acknowledgement', async () => {
  const reviewed = { ...snapshot, journal_revision: pin, records: [resolved('committed')] };
  let calls = 0;
  const api = endpointOwnerAPI(async <T>(path: string, init?: RequestInit) => {
    calls++; expect(path.endsWith('/configuration/archive')).toBe(true);
    expect(JSON.parse(String(init?.body))).toEqual({ expected_revision: pin, expected_journal_revision: pin });
    return { archived: 1, journal_revision: after } as T;
  });
  expect(await api.archive(reviewed)).toEqual({ archived: 1, journal_revision: after });
  expect(calls).toBe(1);
  for (const value of [{ ...reviewed, journal_revision: undefined }, { ...reviewed, records: [record] }]) {
    await expect(api.archive(value)).rejects.toThrow();
  }
  expect(calls).toBe(1);
  for (const value of [{ archived: 0, journal_revision: after }, { archived: 1, journal_revision: 'bad' }]) {
    const malformed = endpointOwnerAPI(async <T>() => value as T);
    await expect(malformed.archive(reviewed)).rejects.toThrow();
  }
  let lostCalls = 0;
  const lost = endpointOwnerAPI(async () => { lostCalls++; throw Error('lost acknowledgement'); });
  await expect(lost.archive(reviewed)).rejects.toThrow('lost acknowledgement');
  expect(lostCalls).toBe(1);
});

test('journal responses reject malformed proof and contradictory resolution state', () => {
  for (const changed of [
    { ...record, review_id: 'z'.repeat(64) }, { ...record, before: 'z'.repeat(64) },
    { ...record, endpoint: 'lab@r2@other' }, { ...record, created_at: 'not-a-time' },
    { ...record, resolved_at: '2026-11-21T01:00:00Z' }, { ...resolved('committed'), resolved_by: '' },
    { ...resolved('committed'), resolved_epoch: 'z'.repeat(64) }, { ...resolved('committed'), resolved_at: undefined }
  ]) expect(() => validateEndpointOwnerRecord(changed)).toThrow();
});

test('owner reads do not mutate and ambiguous or duplicate snapshots fail closed', async () => {
  let calls = 0;
  const api = endpointOwnerAPI(async <T>(path: string, init?: RequestInit) => {
    calls++; expect(path).toBe('/api/v2/inference-endpoints/configuration'); expect(init?.method).toBeUndefined(); return structuredClone(snapshot) as T;
  });
  expect(await api.read()).toEqual(snapshot); expect(calls).toBe(1);
  for (const changed of [
    { ...snapshot, entries: [entry, entry] },
    { ...snapshot, records: [record], write_available: true },
    { ...snapshot, records: [record, record], write_available: false },
    { ...snapshot, file_revision: 'changed' },
    { ...snapshot, credential_references: [{ id: 'credential', origins: ['https://user:secret@example.test'] }] }
  ]) expect(() => validateEndpointOwner(changed)).toThrow();
});

test('prepare verifies exact origin, identity and actor scope without committing or retrying', async () => {
  for (const change of [
    { expected_revision: after }, { restart_required: false },
    { entry: { ...entry, url: 'https://other.example.test' } },
    { entry: { ...entry, model_digest: 'sha256:' + 'f'.repeat(64) } },
    { entry: { ...entry, actors: ['another-actor'] } },
    { entry: { ...entry, allowed_ips: ['192.0.2.11'] } },
    { entry: { ...entry, credential_reference: 'unexpected', credential_configured: true } }
  ]) {
    let calls = 0;
    const api = endpointOwnerAPI(async <T>(path: string, init?: RequestInit) => {
      calls++; expect(path.endsWith('/configuration/prepare')).toBe(true); expect(JSON.parse(String(init?.body))).toEqual(input); return { ...draft, ...change } as T;
    });
    await expect(api.prepare(input)).rejects.toThrow(); expect(calls).toBe(1);
  }
});

test('commit matches the reviewed operation and never retries an unknown outcome', async () => {
  for (const changed of [{ request_id: 'other' }, { review_id: after }, { before: after }, { endpoint: 'lab@r3' }, { actor: 'other' }, { state: 'pending' }]) {
    let calls = 0;
    const api = endpointOwnerAPI(async <T>(path: string, init?: RequestInit) => {
      calls++; expect(path.endsWith('/configuration/commit')).toBe(true); expect(JSON.parse(String(init?.body))).toEqual({ review_id: review, request_id: 'write-once' }); return { ...record, ...changed } as T;
    });
    await expect(api.commit(draft, 'write-once', 'administrator')).rejects.toThrow(); expect(calls).toBe(1);
  }
  let calls = 0;
  const unavailable = endpointOwnerAPI(async () => { calls++; throw Error('publication outcome unknown'); });
  await expect(unavailable.commit(draft, 'write-once', 'administrator')).rejects.toThrow('unknown'); expect(calls).toBe(1);
});

test('reconciliation accepts only the actual digest outcome and preserves original intent', async () => {
  for (const state of ['committed', 'not_applied'] as const) {
    const expected = state === 'committed' ? after : pin;
    const api = endpointOwnerAPI(async <T>(path: string, init?: RequestInit) => {
      expect(path.endsWith('/configuration/reconcile')).toBe(true); expect(JSON.parse(String(init?.body))).toEqual({ request_id: record.request_id, expected_revision: expected }); return resolved(state) as T;
    });
    expect((await api.reconcile(record, expected, 'administrator')).state).toBe(state);
  }
  for (const changed of [{ state: 'not_applied' }, { after: pin }, { resolved_by: 'other' }, { review_id: after }, { resolved_epoch: '' }]) {
    let calls = 0;
    const api = endpointOwnerAPI(async <T>() => { calls++; return { ...resolved('committed'), ...changed } as T; });
    await expect(api.reconcile(record, after, 'administrator')).rejects.toThrow(); expect(calls).toBe(1);
  }
});

test('managed drafts retain the exact installation template and never accept a remote substitute', async () => {
  const managedInput: EndpointOwnerInput = { id: input.id, revision: input.revision, actors: input.actors, expected_revision: pin, deployment_attested: true, managed_template: 'installed-template' };
  const managedEntry: EndpointOwnerEntry = { ...entry, url: 'http://127.0.0.1:11435', managed: true, managed_template: 'installed-template' };
  const api = endpointOwnerAPI(async <T>() => ({ ...draft, entry: managedEntry } as T));
  expect((await api.prepare(managedInput)).entry.managed).toBe(true);
  const foreign = endpointOwnerAPI(async <T>() => ({ ...draft, entry: { ...managedEntry, managed_template: 'other' } } as T));
  await expect(foreign.prepare(managedInput)).rejects.toThrow();
});
