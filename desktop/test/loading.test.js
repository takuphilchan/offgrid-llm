const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const html = fs.readFileSync(path.join(__dirname, '../loading.html'), 'utf8');
const source = html.match(/<script>([\s\S]*?)<\/script>/)[1];

test('loading recovery shows incompatibility then navigates to the verified service', async () => {
  const elements = new Map();
  let destination;
  let backend = { state: 'incompatible', reason: 'Update this service first.' };
  const sandbox = vm.createContext({
    document: { getElementById(id) {
      if (!elements.has(id)) elements.set(id, { style: {}, classList: { add() {}, remove() {} }, textContent: '' });
      return elements.get(id);
    } },
    window: {
      electron: { getBackendInfo: async () => backend, getVersion: async () => 'test' },
      location: { assign(url) { destination = url; } }
    }
  });
  new vm.Script(source).runInContext(sandbox);
  await vm.runInContext('retryConnection()', sandbox);
  assert.equal(elements.get('error-message').textContent, backend.reason);
  assert.equal(destination, undefined);
  backend = { state: 'ready', url: 'http://127.0.0.1:11611' };
  await vm.runInContext('retryConnection()', sandbox);
  assert.equal(destination, 'http://127.0.0.1:11611/ui/');
});
