import { useSyncExternalStore } from 'react';
import { getPreferenceSaveStates } from './preferences';

const subscribe = (notify: () => void) => {
  window.addEventListener('offgrid-preference-save', notify);
  return () => window.removeEventListener('offgrid-preference-save', notify);
};

export const usePreferenceSaveStates = () => useSyncExternalStore(subscribe, getPreferenceSaveStates);
