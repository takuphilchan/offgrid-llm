import { useEffect, useRef, useState } from "react";
import { APIError, api, type AgentRun } from "../../api/client";
import { agentActive } from "../../api/agent-stream";

// One read-only follower owns snapshot recovery and stream lifetime. Unmounting
// aborts the subscription, never the task. Commands remain explicit callers.
export function useTaskRun(id: string) {
  const [run, setRun] = useState<AgentRun | null>(null);
  const [connection, setConnection] = useState<
    "connecting" | "live" | "reconnecting"
  >("connecting");
  const [error, setError] = useState("");
  const [revision, refresh] = useState(0);
  const latest = useRef<AgentRun | null>(null);
  useEffect(() => {
    if (latest.current?.run_id !== id) latest.current = null;
    setRun(latest.current);
    setError("");
    setConnection("connecting");
    if (!id) return;
    let disposed = false,
      retries = 0;
    let timer: ReturnType<typeof setTimeout>,
      watchdog: ReturnType<typeof setTimeout>,
      controller: AbortController;
    const accept = (next: AgentRun) => {
      if (disposed || next.run_id !== id) return false;
      const previous = latest.current;
      // Durable cursors order committed snapshots. A delayed reconnect must
      // not restore an older approval or result. Equal cursors may still carry
      // a fresher provisional preview; legacy services lack this guarantee.
      const cursor = /^\d{1,19}$/;
      if (previous?.event_cursor && next.event_cursor &&
          cursor.test(previous.event_cursor) && cursor.test(next.event_cursor) &&
          BigInt(next.event_cursor) < BigInt(previous.event_cursor)) return false;
      latest.current = { ...next, prompt: next.prompt ?? previous?.prompt, model: next.model ?? previous?.model };
      setRun(latest.current);
      return true;
    };
    const follow = async () => {
      try {
        controller = new AbortController();
        const snapshot = await api.job(id, controller.signal);
        if (disposed) return;
        accept(snapshot);
        setError("");
        setConnection("live");
        retries = 0;
        if (latest.current && agentActive(latest.current)) {
          const alive = () => {
            clearTimeout(watchdog);
            watchdog = setTimeout(() => controller.abort(), 20_000);
          };
          alive();
          await api.streamAgent(
            id,
            accept,
            alive,
            controller.signal,
          );
        }
        // Includes saved input/approval waits: another client may resolve them.
        if (!disposed && latest.current && !['completed', 'failed', 'cancelled'].includes(latest.current.status)) timer = setTimeout(() => void follow(), 2500);
      } catch (reason) {
        if (disposed) return;
        setConnection("reconnecting");
        if (
          reason instanceof APIError &&
          [401, 403, 404].includes(reason.status)
        ) {
          latest.current = null;
          setRun(null);
          setError(reason.message);
          return;
        }
        setError(reason instanceof Error ? reason.message : "");
        timer = setTimeout(
          () => void follow(),
          Math.min(10_000, 1000 * 2 ** retries++),
        );
      } finally {
        clearTimeout(watchdog);
      }
    };
    void follow();
    return () => {
      disposed = true;
      clearTimeout(timer);
      clearTimeout(watchdog);
      controller?.abort();
    };
  }, [id, revision]);
  return {
    // A selection change renders before its effect resets state. Never expose
    // the previous task's result or controls under the new task's identity.
    run: run?.run_id === id ? run : null,
    connection,
    error,
    refresh: () => refresh((value) => value + 1),
  };
}
