// Preferences are conveniences, not workspace persistence. Browser storage can
// be disabled or full; keep the current tab usable without promising durability.
// Drafts use their separate storage helper, which visibly reports write failure.
const volatile = new Map<string, string | null>();
export type PreferenceSaveState = 'pending' | 'saved' | 'unsaved';
let saveStates: Readonly<Record<string, PreferenceSaveState>> = Object.freeze({});

// Session-local outcomes survive route changes; no preference values are exposed.
export const getPreferenceSaveStates = () => saveStates;
export function reportPreferenceSave(key: string, state: PreferenceSaveState): void {
  if (saveStates[key] !== state) saveStates = Object.freeze({ ...saveStates, [key]: state });
  // Continue notifying value subscribers even when the save state is unchanged.
  window.dispatchEvent(new CustomEvent('offgrid-preference-save', { detail: { key, saved: state === 'saved', state } }));
}

export function readPreference(key: string): string | null {
  if (volatile.has(key)) return volatile.get(key) ?? null;
  try { return localStorage.getItem(key); } catch { return null; }
}

export function writePreference(key: string, value: string | null): boolean {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
    volatile.delete(key);
    reportPreferenceSave(key, 'saved');
    return true;
  } catch { volatile.set(key, value); reportPreferenceSave(key, 'unsaved'); return false; }
}
