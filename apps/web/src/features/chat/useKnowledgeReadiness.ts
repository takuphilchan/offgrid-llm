import {useEffect, useRef, useState} from 'react';
import {api} from '../../api/client';
import {useWorkspaceRefresh} from '../../lib/workspace-refresh';

export function useKnowledgeReadiness(allowed: boolean) {
  const [state, setState] = useState<'checking'|'ready'|'unavailable'|'failed'>('checking');
  const pending = useRef<AbortController | null>(null);
  const refresh = async () => {
    pending.current?.abort();
    if (!allowed) {setState('unavailable'); return;}
    const check = new AbortController(); pending.current = check; setState('checking');
    try {
      const status = await api.ragStatus(check.signal);
      if (!check.signal.aborted) setState(status.enabled === true ? 'ready' : 'unavailable');
    } catch {if (!check.signal.aborted) setState('failed');}
  };
  useEffect(() => {void refresh(); return () => pending.current?.abort();}, [allowed]);
  useWorkspaceRefresh(refresh);
  return {state, refresh};
}
