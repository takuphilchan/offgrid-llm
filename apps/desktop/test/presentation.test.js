const { test } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const vm = require('node:vm');
const { normalize, readCopy } = require('../presentation');
const catalog = readCopy(path.join(__dirname, '../../web/public'));

test('missing or damaged optional presentation assets retain safe startup defaults',()=>{
 const {loadAssets}=require('../presentation');
 const dir=fs.mkdtempSync(path.join(require('node:os').tmpdir(),'offgrid-presentation-assets-'));
 const missing=loadAssets(dir);
 assert.deepEqual(missing.catalog,{themes:{},terminal:{}});
 assert.equal(missing.languages.en.fallback,'en');assert.equal(missing.languages.ar.direction,'rtl');
 fs.writeFileSync(path.join(dir,'theme-palettes.json'),'{bad');
 fs.writeFileSync(path.join(dir,'language-registry.json'),'{}');
 assert.deepEqual(loadAssets(dir),missing);
});
test('pack verification catches the exact missing-catalog failure before release',()=>{
 const {verify,required,mapped}=require('../presentation-pack.cjs');
 const archive={extractFile:(_archive,name)=>fs.readFileSync(path.join(__dirname,'..',mapped[name]??name))};
 assert.doesNotThrow(()=>verify('fixture',archive));
 for(const missing of [...required,...Object.keys(mapped)])assert.throws(()=>verify('fixture',{extractFile:(a,name)=>{if(name===missing)throw Error('missing');return archive.extractFile(a,name);}}),/missing from app.asar/);
 assert.throws(()=>verify('fixture',{extractFile:()=>Buffer.from('{}')}),/differs from source/);
 const manifest=require('../package.json');for(const file of required)assert.ok(manifest.build.files.includes(file));
 for(const [to,from] of Object.entries(mapped))assert.ok(manifest.build.files.some(entry=>entry.to===path.posix.dirname(to)&&entry.from===path.posix.dirname(from)&&entry.filter?.includes(path.posix.basename(from))));
});

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
  vm.runInContext("render({state:'checking',autoReconnect:true})",sandbox);
  assert.equal(elements.get('status-title').textContent,catalog[locale].reconnecting);
  assert.equal(elements.get('status-text').textContent,catalog[locale].reconnectingBody);
  assert.equal(elements.get('recovery').hidden,true);
  vm.runInContext("render({state:'unavailable',autoReconnect:true})",sandbox);
  assert.equal(sandbox.document.title,'OffGrid');
  assert.equal(elements.get('recovery').hidden,false);
});

test('appearance normalization rejects executable and unbounded preferences',()=>{
 const value=normalize({locale:'ar',theme:'system',appearance:{version:1,light:'forest',dark:'../bad',textSize:999,reduceMotion:true,script:'bad'}});
 assert.deepEqual(value.appearance,{version:1,light:'forest',dark:'offgrid',textSize:100,reduceMotion:true});
 assert.equal(normalize({appearance:{version:999}}).appearance,undefined);
});
