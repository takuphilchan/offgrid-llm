import { expect, test } from '@playwright/test';
import { api, setAgentReplayScope } from '../src/api/client';

test('replay scope changes discard old cursors and late private snapshots', async () => {
  const original = globalThis.fetch;
  const cursors: string[] = [], seen: string[] = [];
  let pending: ReadableStreamDefaultController<Uint8Array> | undefined;
  const event = (cursor: string) => new TextEncoder().encode(`event: snapshot\ndata: ${JSON.stringify({run_id:'shared-id',status:'completed',event_cursor:cursor,steps:[],output:''})}\n\n`);
  let calls = 0;
  globalThis.fetch = async (input, init) => {
    if (String(input) === '/api/v2/system') return Response.json({workspace_id:'workspace',capabilities:['durable-agent-events-v2']});
    cursors.push(new Headers(init?.headers).get('Last-Event-ID') ?? '');
    calls++;
    const headers = { 'Content-Type': 'text/event-stream' };
    if (calls === 2) return new Response(new ReadableStream({start(controller) { pending = controller; }}), {headers});
    return new Response(event(String(calls * 10)), {headers});
  };
  try {
    await api.systemIdentity(); setAgentReplayScope('alice/admin/workspace');
    const follow = () => api.streamAgent('shared-id', snapshot => { seen.push(snapshot.event_cursor!); }, () => {}, new AbortController().signal);
    await follow();
    const old = follow();
    await expect.poll(() => Boolean(pending)).toBe(true);
    setAgentReplayScope('bob/member/workspace');
    pending!.enqueue(event('999')); pending!.close(); await old;
    await follow();
    expect(cursors).toEqual(['', '10', '']);
    expect(seen).toEqual(['10', '30']);
    // Refresh within the same scope retains the applied cursor.
    setAgentReplayScope('bob/member/workspace'); await follow();
    expect(cursors[3]).toBe('30');
  } finally { globalThis.fetch = original; setAgentReplayScope('test-finished'); }
});
