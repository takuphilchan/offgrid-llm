import type { Root } from 'hast';

type Job = { id: number; owner: symbol; content: string; accept: (tree?: Root) => void; cancelled?: boolean };
let worker: Worker | undefined, active: Job | undefined, serial = 0;
const pending = new Map<symbol, Job>();
let idle: ReturnType<typeof setTimeout> | undefined;
let deadline: ReturnType<typeof setTimeout> | undefined;
let unavailable = false;

function failWorker() {
  clearTimeout(deadline);
  const failed = active; active = undefined;
  worker?.terminate(); worker = undefined; unavailable = true;
  if (failed && !failed.cancelled) failed.accept();
  const queued = [...pending.values()]; pending.clear();
  for (const job of queued) job.accept();
}

// One shared local parser. Each mounted response owns at most one pending
// revision, so a fast stream cannot build a queue of obsolete whole documents.
function pump() {
  if (active) return;
  const next = pending.values().next().value as Job | undefined;
  if (!next) {
    clearTimeout(idle);
    idle = setTimeout(() => { worker?.terminate(); worker = undefined; }, 15_000);
    return;
  }
  pending.delete(next.owner);
  clearTimeout(idle);
  try {
    if (!worker) {
      worker = new Worker(new URL('./markdown-worker.ts', import.meta.url), { type: 'module' });
      worker.onmessage = (event: MessageEvent<{ id: number; tree?: Root; failed?: boolean }>) => {
        if (!active || active.id !== event.data.id) return;
        clearTimeout(deadline);
        const done = active; active = undefined;
        if (!done.cancelled) done.accept(event.data.failed ? undefined : event.data.tree);
        pump();
      };
      worker.onerror = failWorker;
      worker.onmessageerror = failWorker;
    }
    active = next;
    // A stalled worker must not leave an earlier partial rendering on screen
    // indefinitely. Retain the complete source and release the bounded queue.
    deadline = setTimeout(failWorker, 10_000);
    worker.postMessage({ id: next.id, content: next.content });
  } catch {
    if (!active) active = next;
    failWorker();
  }
}

export function parseMarkdown(owner: symbol, content: string, accept: Job['accept']) {
  if (unavailable) { accept(); return; }
  pending.set(owner, { id: ++serial, owner, content, accept });
  pump();
}

export function forgetMarkdown(owner: symbol) {
  pending.delete(owner);
  if (active?.owner === owner) active.cancelled = true;
  if (!active && !pending.size) pump();
}
