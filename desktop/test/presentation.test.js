const { test } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const vm = require('node:vm');
const { normalize, readCopy } = require('../presentation');
const catalog = readCopy(path.join(__dirname, '../../web/app/public'));

test('desktop presentation validates preferences and all nine dictionaries have matching keys', () => {
  assert.deepEqual(normalize({ locale: '../../bad', theme: 'bad' }), { locale: 'en', theme: 'system' });
  assert.deepEqual(normalize({ locale: 'ar', theme: 'dark' }), { locale: 'ar', theme: 'dark' });
  assert.equal(Object.keys(catalog).length, 9);
  for (const row of Object.values(catalog)) {
    assert.deepEqual(Object.keys(row).sort(), Object.keys(catalog.en).sort());
    assert.ok(Object.values(row).every(value => typeof value === 'string' && value.trim()));
  }
});

for (const locale of Object.keys(catalog)) test(`startup applies ${locale} and saved dark theme before backend connection`, async () => {
  const elements = new Map();
  const documentElement = { dataset: {} };
  const sandbox = vm.createContext({
    document: { documentElement, querySelectorAll: () => [], getElementById(id) {
      if (!elements.has(id)) elements.set(id, { hidden: false, classList: { toggle() {} }, textContent: '', addEventListener() {} });
      return elements.get(id);
    } },
    window: { addEventListener() {}, electron: {
      presentation: { locale, effectiveTheme: 'dark', copy: catalog[locale] },
      onStartupState: () => () => {}, getBackendInfo: async () => ({ state: 'ready' })
    } }
  });
  new vm.Script(fs.readFileSync(path.join(__dirname, '../startup.js'), 'utf8')).runInContext(sandbox);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(documentElement.lang, locale);
  assert.equal(documentElement.dir, locale === 'ar' ? 'rtl' : 'ltr');
  assert.equal(documentElement.dataset.theme, 'dark');
  assert.equal(elements.get('status-title').textContent, catalog[locale].ready);
});
