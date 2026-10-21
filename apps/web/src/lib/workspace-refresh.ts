import { useEffect, useRef } from 'react';

const eventName = 'offgrid:refresh-workspace';
type RefreshEvent = CustomEvent<Promise<unknown>[]>;

// Refresh mounted data in place. Never remount a page or discard its drafts.
export function useWorkspaceRefresh(refresh: () => Promise<unknown>) {
  const latest = useRef(refresh);
  latest.current = refresh;
  useEffect(() => {
    const listener = (event: Event) => {
      (event as RefreshEvent).detail.push(Promise.resolve().then(() => latest.current()));
    };
    window.addEventListener(eventName, listener);
    return () => window.removeEventListener(eventName, listener);
  }, []);
}

export async function refreshWorkspace() {
  const pending: Promise<unknown>[] = [];
  window.dispatchEvent(new CustomEvent(eventName, { detail: pending }));
  await Promise.all(pending);
}
