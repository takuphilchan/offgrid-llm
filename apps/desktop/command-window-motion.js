'use strict';

// Native bounds interpolation works on Windows too (setBounds' animate flag is
// macOS-only). The renderer fades before this starts; no screenshot or second
// window is retained. All work is cancellable and tied to the original window.
function createCommandWindowMotion({ prepare, reducedMotion = () => true, now = () => performance.now(), schedule = setTimeout, unschedule = clearTimeout } = {}) {
  let active = null, settled = Promise.resolve();
  function cancel() { active?.abort(); active = null; }
  function run(target, update, options = {}) {
    cancel();
    const controller = new AbortController(), { signal } = controller;
    active = controller;
    const valid = () => !signal.aborted && !target.isDestroyed();
    const pause = ms => new Promise(resolve => {
      const done = () => { unschedule(timer); signal.removeEventListener('abort', done); resolve(); };
      const timer = schedule(done, ms);
      signal.addEventListener('abort', done, { once: true });
    });
    settled = (async () => {
      let reduced = reducedMotion() || options.reduceMotion === true || !target.isVisible() || target.isMinimized();
      if (!options.prepared && prepare && !reduced) reduced = await prepare(target, signal);
      if (!valid()) return;
      const from = target.getBounds?.() ?? target.getNormalBounds();
      let destination, finish;
      update(bounds => { destination = bounds; }, callback => { finish = callback; });
      if (destination) {
        const start = now(), duration = reduced ? 0 : 180;
        let progress = 0;
        do {
          if (!valid()) return;
          // Preferences can change during a transition. Finish without movement.
          progress = duration && !reducedMotion() ? Math.min(1, (now() - start) / duration) : 1;
          const ease = 1 - Math.pow(1 - progress, 3);
          const frame = Object.fromEntries(['x', 'y', 'width', 'height'].map(key => [key, Math.round(from[key] + (destination[key] - from[key]) * ease)]));
          target.setBounds(progress === 1 ? destination : frame);
          if (progress < 1) await pause(16);
        } while (progress < 1);
      }
      if (valid()) finish?.();
    })().finally(() => { if (active === controller) active = null; });
    return settled;
  }
  return { run, cancel, get active() { return active !== null; }, get settled() { return settled; } };
}

// Only an acknowledgment from the existing trusted main frame releases a
// prepared transition. A missing renderer cannot strand native navigation.
function createCommandTransitionBridge({ timeout = 500 } = {}) {
  let sequence = 0, pending = null;
  function prepare(target, signal) {
    pending?.release(true);
    if (signal.aborted || target.isDestroyed() || target.webContents.isDestroyed?.()) return Promise.resolve(true);
    return new Promise(resolve => {
      const id = ++sequence;
      const release = reduced => {
        clearTimeout(timer); signal.removeEventListener('abort', abort);
        if (pending?.id === id) pending = null;
        resolve(reduced);
      };
      const abort = () => release(true), timer = setTimeout(abort, timeout);
      pending = { id, release };
      signal.addEventListener('abort', abort, { once: true });
      try { target.webContents.send('command-window-transition', { id }); }
      catch { release(true); }
    });
  }
  function acknowledge(value) {
    if (!Number.isSafeInteger(value?.id) || value.id !== pending?.id) return false;
    pending.release(value.reduceMotion === true); return true;
  }
  return { prepare, acknowledge };
}
module.exports = { createCommandWindowMotion, createCommandTransitionBridge };
