import { useEffect, useRef, useState } from 'react';

type ReadAction = { label: string; run: (signal: AbortSignal) => Promise<void> };
type ReadFailure = { message: string; action: ReadAction };

// Read callbacks only. Mutations recover by saved identity, never by replay here.
export function useRecoverableRead(fallback: string) {
  const current = useRef<AbortController | null>(null);
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<ReadFailure | null>(null);
  useEffect(() => () => current.current?.abort(), []);
  const cancel = () => {
    current.current?.abort(); current.current = null;
    setPending(false); setFailure(null);
  };
  const run = async (action: ReadAction) => {
    current.current?.abort();
    const controller = new AbortController(); current.current = controller;
    setPending(true); setFailure(null);
    try { await action.run(controller.signal); }
    catch (reason) {
      if (!controller.signal.aborted) setFailure({ message: reason instanceof Error ? reason.message : fallback, action });
    } finally {
      if (!controller.signal.aborted && current.current === controller) {
        current.current = null; setPending(false);
      }
    }
  };
  return { pending, failure, run, cancel };
}
