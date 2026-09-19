const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../startup.js'), 'utf8');
const translations = JSON.parse(fs.readFileSync(path.join(__dirname, '../../web/app/public/desktop-presentation.json'), 'utf8'));

test('startup shows recovery and subscribes once without authorizing navigation', async () => {
  const elements = new Map();
  let subscription;
  let probes = 0;
  const sandbox = vm.createContext({
    document: { documentElement: { dataset: {} }, querySelectorAll: () => [], getElementById(id) {
      if (!elements.has(id)) elements.set(id, { hidden: false, classList: { toggle() {} }, textContent: '', addEventListener() {} });
      return elements.get(id);
    } },
    window: {
      electron: {
        presentation: { locale: 'en', effectiveTheme: 'light', copy: translations.en },
        getBackendInfo: async () => { probes++; return { state: 'incompatible', reason: 'Versions differ', version: '0.4.3-history-dev', desktopVersion: '0.4.4', url: 'http://127.0.0.1:11611', canOpenBrowser: true }; },
        onStartupState: callback => { subscription = callback; return () => {}; }
      }, addEventListener() {},
      location: { assign() { assert.fail('Only main may authorize navigation'); } }
    }
  });
  new vm.Script(source).runInContext(sandbox);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(probes, 1);
  assert.equal(elements.get('service-version').textContent, '0.4.3-history-dev');
  assert.equal(elements.get('desktop-version').textContent, '0.4.4');
  assert.equal(elements.get('recovery').hidden, false);
  assert.equal(elements.get('browser').hidden, false);
  subscription({ state: 'starting', managedByDesktop: true });
  assert.equal(elements.get('recovery').hidden, true);
  subscription({ state: 'ready', managedByDesktop: true });
  assert.equal(elements.get('status-title').textContent, 'Connected');
  assert.equal(probes, 1);
});

test('startup has a strict local CSP without inline executable content', () => {
  const html = fs.readFileSync(path.join(__dirname, '../loading.html'), 'utf8');
  assert.ok(!html.includes('unsafe-inline'));
  assert.ok(!html.includes('onclick='));
  assert.match(html, /script src="startup.js" defer/);
});
