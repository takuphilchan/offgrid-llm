const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createTerminalOutput } = require('../terminal-output');

function fixture(options = {}) {
  const frames = [], states = []; let overflow = 0;
  const output = createTerminalOutput({ send: frame => frames.push(frame), pause: () => states.push('pause'), resume: () => states.push('resume'), overflow: () => overflow++, ...options });
  return { output, frames, states, overflow: () => overflow };
}
test('slow renderer pauses output and valid acknowledgements release bounded frames', () => {
  const f = fixture({ windowBytes: 8, limit: 32 });
  f.output.push('abcdefghijklmnop');
  assert.deepEqual(f.frames, [{ sequence: 1, text: 'abcdefgh' }]);
  assert.equal(f.output.bufferedBytes, 16); assert.deepEqual(f.states, ['pause']);
  assert.throws(() => f.output.ack(2), /invalid_ack/);
  f.output.ack(1); assert.equal(f.frames[1].text, 'ijklmnop');
  assert.throws(() => f.output.ack(1), /invalid_ack/);
  f.output.ack(2); assert.equal(f.output.bufferedBytes, 0);
  assert.deepEqual(f.states, ['pause', 'resume']);
});
test('multibyte characters remain exact across frame boundaries', () => {
  const f = fixture({ windowBytes: 8 }); const value = 'café 日本語 🐝';
  f.output.push(value);
  for (let i = 0; i < f.frames.length; i++) f.output.ack(f.frames[i].sequence);
  assert.equal(f.frames.map(frame => frame.text).join(''), value);
  assert.ok(f.frames.every(frame => Buffer.byteLength(frame.text, 'utf8') <= 8));
});
test('overflow stops accepting output rather than growing without bound', () => {
  const f = fixture({ limit: 8, windowBytes: 8 });
  f.output.push('12345678'); f.output.push('9'); f.output.push('ignored');
  assert.equal(f.overflow(), 1); assert.equal(f.frames.length, 1);
  assert.equal(f.output.bufferedBytes, 0); assert.deepEqual(f.states, ['pause']);
});
test('a detached renderer cannot resume output by acknowledging stale frames', () => {
  const f = fixture(); f.output.push('pending'); f.output.dispose(); f.output.ack(1); f.output.push('ignored');
  assert.equal(f.output.bufferedBytes, 0); assert.equal(f.frames.length, 1);
});
