export type WorkBucket = 'attention' | 'running' | 'history';
export function workBucket(status: string, stale = false): WorkBucket {
  if (stale) return 'attention';
  if (['completed', 'failed', 'cancelled'].includes(status)) return 'history';
  if (['running', 'pending', 'queued', 'waiting_for_children'].includes(status)) return 'running';
  return 'attention';
}
