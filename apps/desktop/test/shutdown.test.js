const { test } = require('node:test');
const assert = require('node:assert/strict');
const { shutdownDesktop } = require('../shutdown');

test('terminal descendants must stop before quit succeeds', async () => {
  const {options}=fixture({terminal:{stop:async()=>{throw Error('terminal_stop_unconfirmed');}}});
  await assert.rejects(shutdownDesktop(options),/desktop_stop_unconfirmed/);
});

function fixture(overrides = {}) {
  const calls = [];
  const step = name => async () => { calls.push(name); };
  return { calls, options: { save: step('save'), computer: {stop: step('computer')}, execution: {stop: step('execution')}, runtime: {stop: step('service')}, ...overrides } };
}

test('preference failure does not strand input or the owned service', async () => {
  const {calls,options} = fixture({save: async () => { throw Error('disk full'); }});
  await shutdownDesktop(options);
  assert.deepEqual(calls, ['computer','execution','service']);
});

test('failed executor cleanup still attempts service cleanup and reports uncertainty', async () => {
  const {calls,options} = fixture({execution: {stop: async () => { throw Error('execution_stop_unconfirmed'); }}});
  await assert.rejects(shutdownDesktop(options), /desktop_stop_unconfirmed/);
  assert.deepEqual(calls, ['save','computer','service']);
});

test('unconfirmed native input is not reported as a successful quit; retry can settle', async () => {
  let stopping = true;
  const {calls,options} = fixture({computer: {stop: async () => stopping ? {code:'stop_unconfirmed'} : {state:'stopped'}}});
  await assert.rejects(shutdownDesktop(options), /desktop_stop_unconfirmed/);
  stopping = false;
  await shutdownDesktop(options);
  assert.equal(calls.filter(call => call === 'service').length, 2);
});

test('both input cleanup attempts settle before stopping the service', async () => {
  let finish; const events=[];
  const stopping = shutdownDesktop({save: async()=>{}, computer:{stop:()=>new Promise(resolve=>{finish=()=>{events.push('input');resolve();};})}, execution:{stop:async()=>{}}, runtime:{stop:async()=>events.push('service')}});
  await new Promise(resolve=>setImmediate(resolve));
  assert.deepEqual(events,[]);finish();await stopping;
  assert.deepEqual(events,['input','service']);
});

test('a stuck preference write cannot indefinitely hold an otherwise completed quit', async () => {
  const {calls,options}=fixture({save:()=>new Promise(()=>{}),saveTimeoutMs:5});
  await shutdownDesktop(options);
  assert.deepEqual(calls,['computer','execution','service']);
});
