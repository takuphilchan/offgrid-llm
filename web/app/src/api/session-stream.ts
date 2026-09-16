import type { ChatSession, SessionMessage } from './client';
import type { components } from './schema.generated';

export type ChatPhase = components['schemas']['ChatStreamPhase'];
export type ChatMetrics = components['schemas']['ChatTimings'];
export type SessionResult = { session: ChatSession; message: SessionMessage; metrics?: ChatMetrics; finish_reason?: string };
export type SessionEvent = Extract<components['schemas']['SessionStreamEvent'], { type: 'phase' | 'delta' }>;

export class StreamInterruptedError extends Error {}

// fetch is required for authenticated POST streams. Decode across arbitrary UTF-8
// and network boundaries; EOF without a persisted `done` event is never success.
export async function readSessionStream(response: Response, onEvent: (event: SessionEvent) => void): Promise<SessionResult> {
  if (!response.body || !response.headers.get('content-type')?.includes('text/event-stream')) throw new StreamInterruptedError('Session streaming is unavailable; update the OffGrid service.');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let data: string[] = [];
  const consume = (line: string): SessionResult | undefined => {
    if (line.endsWith('\r')) line = line.slice(0, -1);
    if (line.startsWith('data:')) data.push(line.slice(5).trimStart());
    if (line !== '' || data.length === 0) return;
    const event = JSON.parse(data.join('\n'));
    data = [];
    if (event.type === 'error') throw new Error(event.error || 'Generation failed');
    if (event.type === 'done') {
      if (!event.session?.name || !Array.isArray(event.session.messages) || event.message?.role !== 'assistant') throw new StreamInterruptedError('Invalid saved response');
      return event as SessionResult;
    }
    if (event.type === 'phase' && ['queued', 'retrieving', 'loading', 'processing', 'generating', 'saving'].includes(event.phase)) onEvent(event as SessionEvent);
    else if (event.type === 'delta' && typeof event.delta === 'string') onEvent(event as SessionEvent);
  };
  try {
    for (;;) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      if (buffer.length > 16 * 1024 * 1024) throw new StreamInterruptedError('Stream event is too large');
      let end: number;
      while ((end = buffer.indexOf('\n')) >= 0) {
        const line = buffer.slice(0, end);
        buffer = buffer.slice(end + 1);
        const result = consume(line);
        if (result) return result;
      }
      if (done) throw new StreamInterruptedError('Connection ended before the conversation was confirmed saved. Reload before retrying.');
    }
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}
