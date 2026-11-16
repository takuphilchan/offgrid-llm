const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const {join} = require('node:path');
const vm = require('node:vm');
const {messages} = require('../startup-welcome');

test('startup greeting has three distinct messages in every selectable language', () => {
  const registry = require('../language-registry.json');
  assert.deepEqual(Object.keys(messages).sort(),Object.keys(registry).sort());
  for (const [locale,copy] of Object.entries(messages)) {
    assert.equal(copy.length,3,locale);
    assert.equal(new Set(copy).size,3,locale);
    assert.ok(copy.every(text=>typeof text==='string' && text.trim() && !text.includes('\uFFFD')),locale);
    if (locale!=='en') assert.ok(copy.every((text,i)=>text!==messages.en[i]),locale);
  }
});

function fixture(reduce=false) {
  let id=0;
  const intervals=new Map(),timeouts=new Map(),listeners=new Map(),classes=new Set();
  const element={textContent:'',classList:{add:name=>classes.add(name),remove:name=>classes.delete(name)}};
  const document={hidden:false,documentElement:{dataset:{reduceMotion:String(reduce)}},addEventListener:(type,fn)=>listeners.set(type,fn),removeEventListener:type=>listeners.delete(type)};
  const scope={window:{},document,matchMedia:()=>({matches:false}),setInterval:fn=>{intervals.set(++id,fn);return id;},clearInterval:key=>intervals.delete(key),setTimeout:fn=>{timeouts.set(++id,fn);return id;},clearTimeout:key=>timeouts.delete(key)};
  vm.runInNewContext(readFileSync(join(__dirname,'../startup-welcome.js'),'utf8'),scope);
  const tick=()=>[...intervals.values()].forEach(fn=>fn());
  const fade=()=>{const callbacks=[...timeouts.values()];timeouts.clear();callbacks.forEach(fn=>fn());};
  return {element,document,intervals,timeouts,listeners,classes,tick,fade,controller:scope.window.OffGridStartupWelcome.create(element,'en')};
}

test('welcome fades between messages, does not restart for repeated real status updates, and cleans up',()=>{
  const f=fixture();const timer=[...f.intervals.keys()][0];
  f.controller.setPaused(false);f.controller.setPaused(false);
  assert.equal([...f.intervals.keys()][0],timer);
  f.tick();assert.equal(f.element.textContent,messages.en[0]);assert.ok(f.classes.has('is-changing'));
  f.fade();assert.equal(f.element.textContent,messages.en[1]);
  f.tick();f.controller.dispose();f.fade();
  assert.equal(f.element.textContent,messages.en[1]);
  assert.equal(f.intervals.size+f.timeouts.size+f.listeners.size+f.classes.size,0);
});

test('reduced motion still changes copy; reviewed overrides and language changes apply',()=>{
  const f=fixture(true);f.tick();assert.equal(f.element.textContent,messages.en[1]);assert.equal(f.timeouts.size,0);
  f.controller.setLocale('fr',{'1':'Une traduction relue.'});
  assert.equal(f.element.textContent,messages.fr[0]);f.tick();assert.equal(f.element.textContent,'Une traduction relue.');
  f.controller.setLocale('ar');assert.equal(f.element.textContent,messages.ar[0]);
  f.controller.setLocale('unsupported');assert.equal(f.element.textContent,messages.en[0]);f.controller.dispose();
});

test('hidden windows and recovery suspend welcome transitions without losing the current message',()=>{
  const f=fixture();f.tick();
  f.document.hidden=true;f.listeners.get('visibilitychange')();
  assert.equal(f.intervals.size+f.timeouts.size,0);
  assert.equal(f.element.textContent,messages.en[0]);
  f.document.hidden=false;f.listeners.get('visibilitychange')();f.tick();f.fade();
  assert.equal(f.element.textContent,messages.en[1]);
  f.controller.setPaused(true);assert.equal(f.intervals.size,0);
  f.controller.setPaused(false);f.tick();f.fade();assert.equal(f.element.textContent,messages.en[2]);
  f.controller.dispose();
});
