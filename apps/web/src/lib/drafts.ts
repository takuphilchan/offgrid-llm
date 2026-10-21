import { useCallback, useSyncExternalStore } from 'react';

const values = new Map<string, string>();
const failed = new Set<string>();
const listeners = new Set<() => void>();
const notify = () => listeners.forEach(listener => listener());

// Other tabs can edit a draft while its composer is unmounted for setup. Keep
// the cache invalidation listener alive independently of current subscribers.
window.addEventListener('storage', event => {
  if (event.key === null) { for (const key of values.keys()) if (persistent(key) && !failed.has(key)) values.delete(key); notify(); }
  else if (event.key.startsWith('offgrid.draft.')) {
    const key = event.key.replace('offgrid.draft.recovered.v2:', 'offgrid.draft.v2:');
    if (!failed.has(key)) values.delete(key);
    notify();
  }
});

export const workspaceDraftScope = (actor: string, workspace: string) => `@workspace:${JSON.stringify([actor, workspace])}`;
export const draftKey = (scope: string, kind: string, id = '') => scope.startsWith('@workspace:')
  ? `offgrid.draft.v2:${encodeURIComponent(scope.slice(11))}:${kind}:${encodeURIComponent(id)}`
  : scope.startsWith('@memory:') ? `offgrid.draft.memory:${encodeURIComponent(scope)}:${kind}:${encodeURIComponent(id)}`
    : `offgrid.draft.v1:${encodeURIComponent(scope)}:${kind}:${encodeURIComponent(id)}`;
const persistent = (key: string) => !key.startsWith('offgrid.draft.memory:');
const recoveryKey = (key: string) => key.replace('offgrid.draft.v2:', 'offgrid.draft.recovered.v2:');

export function readDraft(key: string): string {
  if (values.has(key)) return values.get(key)!;
  if (!persistent(key)) return '';
  try { return localStorage.getItem(key) ?? (key.startsWith('offgrid.draft.v2:') ? localStorage.getItem(recoveryKey(key)) : null) ?? ''; } catch { return ''; }
}

export function writeDraft(key: string, value: string): void {
  values.set(key, value);
  // Deliberate session-only mode is disclosed by DraftRecoveryNotice, not
  // reported a second time as a failed browser-storage write.
  if (!persistent(key)) { notify(); return; }
  try {
    // A cleared v2 draft is a tombstone: never resurrect an older restored copy.
    if (value || key.startsWith('offgrid.draft.v2:')) localStorage.setItem(key, value); else localStorage.removeItem(key);
    failed.delete(key);
  } catch { failed.add(key); }
  notify();
}

export function forgetDraftMemory(scope: string) {
  const prefix = draftKey(scope, '').slice(0, -1);
  for (const key of values.keys()) if (key.startsWith(prefix)) { values.delete(key); failed.delete(key); }
}

const restorationKey = (actor: string, workspace: string) => `offgrid.draft.restore.v2:${encodeURIComponent(JSON.stringify([actor, workspace]))}`;
function legacyDrafts(actor: string) {
  const prefix = `offgrid.draft.v1:${encodeURIComponent(actor)}:`;
  const drafts: { kind: string; id: string; value: string }[] = [];
  for (let index = 0; index < localStorage.length; index++) {
    const key = localStorage.key(index);
    if (!key?.startsWith(prefix)) continue;
    const match = /^(chat|agent-task):([^:]*)$/.exec(key.slice(prefix.length));
    if (!match) continue;
    try {
      const value = localStorage.getItem(key);
      if (value) drafts.push({ kind: match[1], id: decodeURIComponent(match[2]), value });
    } catch { /* Malformed legacy keys cannot select another draft. */ }
  }
  return drafts;
}

export function hasLegacyDrafts(actor: string, workspace: string): boolean {
  if (!workspace) return false;
  try { return !localStorage.getItem(restorationKey(actor, workspace)) && legacyDrafts(actor).length > 0; } catch { return false; }
}

export function resolveLegacyDrafts(actor: string, workspace: string, restore: boolean): void {
  if (!workspace) throw new Error('Workspace identity unavailable');
  if (restore) for (const draft of legacyDrafts(actor)) {
    const key = draftKey(workspaceDraftScope(actor, workspace), draft.kind, draft.id);
    // Never overwrite the live slot, even if a second tab writes between our
    // read and copy. That slot (including an empty tombstone) always wins.
    if (localStorage.getItem(key) === null && !values.has(key) && localStorage.getItem(recoveryKey(key)) === null)
      localStorage.setItem(recoveryKey(key), draft.value);
  }
  // No acknowledgment on partial storage failure. Retrying preserves both the
  // source and successful copies; no request/approval records are migrated.
  localStorage.setItem(restorationKey(actor, workspace), restore ? 'restored' : 'separate');
  notify();
}

export function clearSubmittedDraft(key: string, submitted: string): void {
  if (readDraft(key) === submitted) writeDraft(key, '');
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

// Writes happen during editing, not unmount cleanup. A failed send retains the
// exact text, and a late response cannot clear a newer draft or another account.
export function useDraft(scope: string, kind: string, id = '') {
  const key = draftKey(scope, kind, id);
  const value = useSyncExternalStore(subscribe, () => readDraft(key));
  const unsaved = useSyncExternalStore(subscribe, () => failed.has(key));
  const setValue = useCallback((next: string) => writeDraft(key, next), [key]);
  return { key, value, setValue, unsaved };
}
