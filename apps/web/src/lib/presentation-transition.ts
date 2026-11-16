// One presentation lane for voice, quick access and the full workspace. It
// never owns requests, drafts or capture. Reversing cancels only visual work.
let current: AbortController | undefined;
const reducedMotion = () => document.documentElement.dataset.reduceMotion === 'true' || matchMedia('(prefers-reduced-motion: reduce)').matches;
export function cancelPresentationTransition() { current?.abort(); }

export async function transitionPresentation(update: (reduced: boolean) => void | Promise<unknown>) {
  current?.abort();
  const owner = new AbortController(), {signal} = owner, root = document.documentElement;
  current = owner;
  const pause = (ms: number) => new Promise<void>(resolve => {
    const done = () => {clearTimeout(timer); signal.removeEventListener('abort', done); resolve();};
    const timer = setTimeout(done, ms);
    signal.addEventListener('abort', done, {once:true});
  });
  const paint = () => new Promise<void>(resolve => {
    let frame = 0;
    const done = () => {cancelAnimationFrame(frame); clearTimeout(timer); signal.removeEventListener('abort', done); resolve();};
    // Hidden windows throttle RAF; dismissal must not leave a faded workspace.
    const timer = setTimeout(done, 100);
    frame = requestAnimationFrame(() => {frame = requestAnimationFrame(done);});
    signal.addEventListener('abort', done, {once:true});
  });
  try {
    const reduced = reducedMotion() || document.hidden;
    root.dataset.windowTransition = reduced ? 'resizing' : 'leaving';
    if (!reduced) await pause(70);
    if (signal.aborted) return;
    root.dataset.windowTransition = 'resizing';
    await update(reduced);
    if (signal.aborted) return;
    await paint();
    if (signal.aborted) return;
    root.dataset.windowTransition = 'entering';
    if (!reduced && !reducedMotion()) await pause(120);
  } finally {
    if (current === owner) { delete root.dataset.windowTransition; current = undefined; }
  }
}
