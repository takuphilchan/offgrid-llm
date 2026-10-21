// Preferences are conveniences, not workspace persistence. Browser storage can
// be disabled or full; keep the current tab usable without promising durability.
// Drafts use their separate storage helper, which visibly reports write failure.
const volatile = new Map<string, string | null>();

export function readPreference(key: string): string | null {
  if (volatile.has(key)) return volatile.get(key) ?? null;
  try { return localStorage.getItem(key); } catch { return null; }
}

export function writePreference(key: string, value: string | null): void {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
    volatile.delete(key);
  } catch { volatile.set(key, value); }
}
