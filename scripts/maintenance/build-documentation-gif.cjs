// Optional documentation tooling; never installed in the application bundle.
// npm install --prefix build/documentation-tools --no-save --ignore-scripts gifenc@1.0.3 pngjs@7.0.0
const { createRequire } = require('node:module');
const { readFileSync, writeFileSync } = require('node:fs');
const { resolve } = require('node:path');
const root = resolve(__dirname, '../..');
const load = createRequire(resolve(root, 'build/documentation-tools/package.json'));
const { GIFEncoder, quantize, applyPalette } = load('gifenc');
const { PNG } = load('pngjs');
const output = resolve(root, 'build/documentation-captures');
const gif = GIFEncoder();
for (const name of ['chat-dark', 'knowledge-library', 'agent-profiles', 'chat-workspace']) {
  const { data, width, height } = PNG.sync.read(readFileSync(resolve(output, `${name}.png`)));
  if (width !== 1440 || height !== 900) throw new Error(`Unexpected capture dimensions: ${name}`);
  const palette = quantize(data, 256);
  gif.writeFrame(applyPalette(data, palette), width, height, { palette, delay: 4000, repeat: 0 });
}
gif.finish();
const bytes = gif.bytes();
if (bytes.length > 2 * 1024 * 1024) throw new Error('Keep the documentation GIF below 2 MiB');
writeFileSync(resolve(output, 'workspace-tour.gif'), bytes);
console.log(`Wrote four-frame screenshot tour (${bytes.length} bytes, 16 seconds).`);
