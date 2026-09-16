import { useCallback, useSyncExternalStore } from 'react';

const values = new Map<string, string>();
const failed = new Set<string>();
const listeners = new Set<() => void>();
const notify = () => listeners.forEach(listener => listener());

export const draftKey = (scope: string, kind: string, id = '') => `offgrid.draft.v1:${encodeURIComponent(scope)}:${kind}:${encodeURIComponent(id)}`;

export function readDraft(key: string): string {
  if (values.has(key)) return values.get(key)!;
  try { return localStorage.getItem(key) ?? ''; } catch { return ''; }
}

export function writeDraft(key: string, value: string): void {
  values.set(key, value);
  try {
    if (value) localStorage.setItem(key, value); else localStorage.removeItem(key);
    failed.delete(key);
  } catch { failed.add(key); }
  notify();
}

export function clearSubmittedDraft(key: string, submitted: string): void {
  if (readDraft(key) === submitted) writeDraft(key, '');
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  const onStorage = (event: StorageEvent) => {
    if (event.key?.startsWith('offgrid.draft.v1:')) { values.delete(event.key); notify(); }
  };
  window.addEventListener('storage', onStorage);
  return () => { listeners.delete(listener); window.removeEventListener('storage', onStorage); };
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
