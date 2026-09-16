import { useLayoutEffect, useRef } from 'react';
import type { AgentRun } from '../../api/client';
import { agentActive } from '../../api/agent-stream';

// Only the output pane follows a live run. Reading earlier text must never
// move focus, scroll the page, or get interrupted by the next streamed update.
export function useAgentOutputScroll(run: AgentRun | null, showPreview: boolean, view: string) {
  const ref = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const following = useRef(true);
  const previousRun = useRef<string | undefined>(undefined);
  const previousStatus = useRef<string | undefined>(undefined);
  useLayoutEffect(() => {
    const pane = ref.current;
    if (!pane) return;
    const changedRun = previousRun.current !== run?.run_id;
    const needsDecision = run && ['waiting_for_approval', 'uncertain', 'interrupted'].includes(run.status)
      && previousStatus.current !== run.status;
    if (changedRun || needsDecision) {
      following.current = true;
      pane.scrollTop = 0;
    }
    if (run && agentActive(run) && following.current) pane.scrollTop = pane.scrollHeight;
    previousRun.current = run?.run_id;
    previousStatus.current = run?.status;
  }, [run?.run_id, run?.status, run?.progress?.preview, run?.steps?.length, run?.output, showPreview, view]);

  // Reconnection notices and wrapped translations can resize the header even
  // when no token arrives. Keep the tail visible through those size changes.
  useLayoutEffect(() => {
    const pane = ref.current;
    if (!pane || !run || !agentActive(run)) return;
    const observer = new ResizeObserver(() => {
      if (following.current) pane.scrollTop = pane.scrollHeight;
    });
    observer.observe(pane);
    // Text wrapping, fonts and newly inserted steps can change scrollHeight
    // without resizing the pane itself. Observe the content as well.
    if (contentRef.current) observer.observe(contentRef.current);
    return () => observer.disconnect();
  }, [run?.status, view]);

  return { ref, contentRef, onScroll: () => {
    const pane = ref.current;
    if (pane) following.current = pane.scrollHeight - pane.clientHeight - pane.scrollTop < 48;
  } };
}
