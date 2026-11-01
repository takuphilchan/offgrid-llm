import { useEffect, useState } from 'react';
import { api, type WorkSearchResponse } from '../api/client';

export function useWorkSearch(query: string, enabled: boolean) {
  const needle = query.trim();
  const valid = enabled && [...needle].length >= 2;
  const [state, setState] = useState<{ query: string; data?: WorkSearchResponse; failed?: boolean }>({ query: '' });
  useEffect(() => {
    if (!valid) { setState({ query: '' }); return; }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      void api.searchWork(needle, controller.signal).then(data => {
        if (!controller.signal.aborted) setState({ query: needle, data });
      }).catch(() => { if (!controller.signal.aborted) setState({ query: needle, failed: true }); });
    }, 250);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [needle, valid]);
  // Hide previous private results synchronously, before effect cleanup/fetch.
  const current = valid && state.query === needle ? state : undefined;
  return { data: current?.data, failed: current?.failed, loading: valid && !current?.data && !current?.failed };
}
