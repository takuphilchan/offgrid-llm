import { useCallback, useEffect, useRef, useState } from 'react';
import { api, type ModelOperation } from '../../api/client';
import { useI18n } from '../../i18n';
import { useWorkspace } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';

// One read-only subscription for the whole library. Category/search changes do
// not own a transfer's lifetime or replay its operation.
export function useModelOperations() {
  const { admin, scope, workspace } = useWorkspace();
  const { messages: text } = useI18n();
  const [operations, setOperations] = useState<ModelOperation[]>([]);
  const [error, setError] = useState('');
  const lifetime = useRef<AbortController | null>(null);
  const revision = useRef(0);
  const refresh = useCallback(async () => {
    const signal = lifetime.current?.signal;
    if (!admin || !signal || signal.aborted) return;
    const current = ++revision.current;
    try {
      const next = await api.modelOperations(signal);
      if (!Array.isArray(next)) throw new Error(text.common.error);
      if (!signal.aborted && revision.current === current) { setOperations(next); setError(''); }
    } catch (reason) {
      if (!signal.aborted && revision.current === current) setError(reason instanceof Error ? reason.message : text.common.error);
    }
  }, [admin, text.common.error]);
  useEffect(() => {
    const controller = new AbortController(); lifetime.current = controller;
    setOperations([]); setError('');
    if (admin) void refresh();
    let polling = false;
    const timer = admin ? window.setInterval(async () => {
      if (polling || controller.signal.aborted) return;
      polling = true;
      try { await refresh(); } finally { polling = false; }
    }, 2000) : undefined;
    return () => { controller.abort(); clearInterval(timer); revision.current++; };
  }, [admin, scope, workspace, refresh]);
  useWorkspaceRefresh(refresh);
  return { operations: admin ? operations : [], error: admin ? error : '', refresh };
}
