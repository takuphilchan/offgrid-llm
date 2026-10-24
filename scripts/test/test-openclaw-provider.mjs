// Offline first-party hook checks, with SDK setup functions stubbed at import.
// Installed-runtime tests separately verify the SDK consumes this contract.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';
import test from 'node:test';

const require = createRequire(new URL('../../apps/web/package.json', import.meta.url));
const ts = require('typescript');
const source = readFileSync(new URL('../../internal/integrations/plugins/openclaw/offgrid/index.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } });
let provider;
const exports = {};
vm.runInNewContext(compiled.outputText, {
  exports,
  require(name) {
    if (name === 'openclaw/plugin-sdk/plugin-entry') return { definePluginEntry: entry => entry };
    if (name === 'openclaw/plugin-sdk/provider-setup') return {};
    if (name === './verification') return { registerVerification() {} };
    throw new Error(`Unexpected import: ${name}`);
  },
}, { filename: fileURLToPath(import.meta.url), timeout: 1000 });
exports.default.register({ registerProvider(value) { provider = value; } });

test('sequential default changes only absent cardinality and does not mutate input', () => {
  const callback = () => {};
  const original = Object.freeze({ temperature: 0.5, nested: { keep: true }, onPayload: callback });
  const result = provider.prepareExtraParams({ extraParams: original, modelId: 'newly-imported-model' });
  assert.equal(result.parallel_tool_calls, false);
  assert.equal(result.nested, original.nested);
  assert.equal(result.onPayload, callback);
  assert.equal(result.temperature, 0.5);
  assert.equal(original.parallel_tool_calls, undefined);
  assert.equal(provider.prepareExtraParams({}).parallel_tool_calls, false);
  assert.equal(provider.wrapStreamFn, undefined, 'normal stream/callback composition stays owned by the runtime');
});

test('explicit true, false, null and aliases stay authoritative', () => {
  for (const name of ['parallel_tool_calls', 'parallelToolCalls']) {
    for (const value of [true, false, null, 'invalid-left-for-runtime-validation']) {
      const original = Object.freeze({ [name]: value, other: 'kept' });
      assert.equal(provider.prepareExtraParams({ extraParams: original }), original);
    }
  }
  const both = { parallel_tool_calls: null, parallelToolCalls: true };
  assert.equal(provider.prepareExtraParams({ extraParams: both }), both);
});
