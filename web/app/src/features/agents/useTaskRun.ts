import { useEffect, useState } from "react";
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
  useEffect(() => {
    setRun(null);
    setError("");
    setConnection("connecting");
    if (!id) return;
    let disposed = false,
      retries = 0;
    let timer: ReturnType<typeof setTimeout>,
      watchdog: ReturnType<typeof setTimeout>,
      controller: AbortController;
    const follow = async () => {
      try {
        const snapshot = await api.job(id);
        if (disposed) return;
        setRun(snapshot);
        setError("");
        setConnection("live");
        retries = 0;
        if (agentActive(snapshot)) {
          controller = new AbortController();
          const alive = () => {
            clearTimeout(watchdog);
            watchdog = setTimeout(() => controller.abort(), 20_000);
          };
          alive();
          await api.streamAgent(
            id,
            (next) => {
              if (!disposed)
                setRun((previous) => ({
                  ...next,
                  prompt: previous?.prompt ?? snapshot.prompt,
                }));
            },
            alive,
            controller.signal,
          );
        }
        // Includes saved input/approval waits: another client may resolve them.
        if (!disposed) timer = setTimeout(() => void follow(), 2500);
      } catch (reason) {
        if (disposed) return;
        setConnection("reconnecting");
        if (
          reason instanceof APIError &&
          [401, 403, 404].includes(reason.status)
        ) {
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
