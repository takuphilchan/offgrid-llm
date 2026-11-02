'use strict';

// ACKs are issued by the trusted renderer only after xterm's write callback.
// Pausing the PTY is flow control, not a claim that its child process stopped.
function createTerminalOutput({ send, pause, resume, overflow, limit = 256 * 1024, windowBytes = 64 * 1024 }) {
  if (!Number.isSafeInteger(limit) || !Number.isSafeInteger(windowBytes) || windowBytes < 4 || limit < windowBytes) throw Error('terminal_invalid_output_budget');
  const queue = [], pending = new Map();
  let buffered = 0, inFlight = 0, sequence = 0, paused = false, disposed = false;
  const setPaused = value => { if (paused !== value) { paused = value; (value ? pause : resume)(); } };
  function flush() {
    while (!disposed && queue.length && inFlight + queue[0].bytes <= windowBytes) {
      const frame = queue.shift();
      const id = ++sequence;
      pending.set(id, frame.bytes); inFlight += frame.bytes;
      try { send({ sequence: id, text: frame.text }); }
      catch (error) { dispose(); throw error; }
    }
    if (!disposed) setPaused(queue.length > 0 || inFlight >= windowBytes);
  }
  function dispose() { disposed = true; queue.length = 0; pending.clear(); buffered = inFlight = 0; }
  return {
    push(text) {
      if (disposed) return;
      if (typeof text !== 'string') throw Error('terminal_invalid_output');
      const bytes = Buffer.byteLength(text, 'utf8');
      if (buffered + bytes > limit) {
        setPaused(true); dispose(); overflow(); return;
      }
      buffered += bytes;
      // Bound IPC messages without splitting a Unicode surrogate pair.
      let chunk = '', size = 0;
      for (const char of text) {
        const length = Buffer.byteLength(char, 'utf8');
        if (size + length > Math.min(16 * 1024, windowBytes)) {
          queue.push({ text: chunk, bytes: size }); chunk = ''; size = 0;
        }
        chunk += char; size += length;
      }
      if (size) queue.push({ text: chunk, bytes: size });
      flush();
    },
    ack(id) {
      if (disposed) return;
      if (!Number.isSafeInteger(id) || !pending.has(id)) throw Error('terminal_invalid_ack');
      const bytes = pending.get(id); pending.delete(id); inFlight -= bytes; buffered -= bytes;
      flush();
    },
    dispose,
    get bufferedBytes() { return buffered; },
  };
}
module.exports = { createTerminalOutput };
