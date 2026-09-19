import { createContext, useContext, useState, type Dispatch, type SetStateAction } from 'react';

// Presentation hints only. The service remains the authorization boundary.
export const WorkspaceContext = createContext({ scope: 'local', admin: false, knowledge: false });
export const useWorkspace = () => useContext(WorkspaceContext);

// Navigation state is session-local, account-scoped and never written to disk.
// In particular connector drafts may contain private addresses.
const state = new Map<string, unknown>();
export function clearWorkspaceState() { state.clear(); }
export function useWorkspaceState<T>(name: string, initial: T): [T, Dispatch<SetStateAction<T>>] {
  const { scope } = useWorkspace();
  const key = `${scope}:${name}`;
  const [value, update] = useState<T>(() => state.has(key) ? state.get(key) as T : initial);
  const setValue: Dispatch<SetStateAction<T>> = next => update(previous => {
    const resolved = typeof next === 'function' ? (next as (value: T) => T)(previous) : next;
    state.set(key, resolved);
    return resolved;
  });
  return [value, setValue];
}
