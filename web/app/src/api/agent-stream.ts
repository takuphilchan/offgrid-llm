import type { AgentRun } from './client';

export const agentActive = (run: AgentRun) => ['running', 'pending', 'waiting_for_children'].includes(run.status);

// Snapshots replace previews (never concatenate them); reconnecting cannot
// duplicate text or execute a task. Execution only ends through server state.
export async function readAgentStream(response: Response, id: string, onSnapshot: (run: AgentRun) => void, onHeartbeat: () => void): Promise<void> {
  if (!response.body || !response.headers.get('content-type')?.includes('text/event-stream')) throw new Error('Agent progress streaming unavailable');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '', data: string[] = [], size = 0;
  const consume = (line: string): boolean => {
    if (line.endsWith('\r')) line = line.slice(0, -1);
    if (line.startsWith(':')) onHeartbeat();
    if (line.startsWith('data:')) { data.push(line.slice(5).trimStart()); size += line.length; }
    if (size > 16 * 1024 * 1024) throw new Error('Agent progress event too large');
    if (line !== '' || !data.length) return false;
    const event = JSON.parse(data.join('\n')); data = []; size = 0;
    if (event.type === 'step' || event.type === 'activity') return false; // full snapshot carries committed steps
    if (event.event_cursor !== undefined && (typeof event.event_cursor !== 'string' || !/^\d{1,19}$/.test(event.event_cursor))) throw new Error('Invalid agent event cursor');
    if (event.run_id !== id || !['pending','running','waiting_for_approval','waiting_for_input','waiting_for_children','interrupted','uncertain','completed','failed','cancelled'].includes(event.status) || !Array.isArray(event.steps) || typeof event.output !== 'string') throw new Error('Invalid agent progress snapshot');
    if (event.progress && (typeof event.progress.preview !== 'string' || typeof event.progress.phase !== 'string' || typeof event.progress.iteration !== 'number')) throw new Error('Invalid agent progress preview');
    onSnapshot(event as AgentRun); onHeartbeat();
    return !agentActive(event);
  };
  try {
    for (;;) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      if (buffer.length > 16 * 1024 * 1024) throw new Error('Agent progress event too large');
      let end: number;
      while ((end = buffer.indexOf('\n')) !== -1) {
        const line = buffer.slice(0,end); buffer = buffer.slice(end+1);
        if (consume(line)) return;
      }
      if (done) throw new Error('Agent progress connection interrupted');
    }
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock(); }
}
