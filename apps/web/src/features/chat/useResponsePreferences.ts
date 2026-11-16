import {useEffect, useState} from 'react';
import {readPreferences} from './chat-presentation';
import {writePreference} from '../../lib/preferences';

export function useResponsePreferences(key: string) {
  const [value, setValue] = useState(() => readPreferences(key));
  useEffect(() => {
    const refresh = () => setValue(readPreferences(key));
    const saved = (event: Event) => {if ((event as CustomEvent<{key:string}>).detail.key === key) refresh();};
    const stored = (event: StorageEvent) => {if (event.key === key || event.key === null) refresh();};
    refresh();
    window.addEventListener('offgrid-preference-save', saved);
    window.addEventListener('storage', stored);
    return () => {window.removeEventListener('offgrid-preference-save', saved); window.removeEventListener('storage', stored);};
  }, [key]);
  return [value, (next: ReturnType<typeof readPreferences>) => {setValue(next); writePreference(key, JSON.stringify(next));}] as const;
}
