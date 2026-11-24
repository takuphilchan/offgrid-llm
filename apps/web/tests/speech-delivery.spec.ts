import {expect, test, type Page} from '@playwright/test';

test.skip(!!process.env.OFFGRID_E2E_URL, 'Source-hook playback checks require isolated Vite; acoustic and packaged-service qualification are separate.');

async function mount(page: Page) {
  await page.route('**/v1/audio/status', route => route.fulfill({json: {asr: {available: true}, tts: {available: true}}}));
  await page.route('**/ui/speech-delivery.html', route => route.fulfill({contentType: 'text/html', body: `
    <div id="root"></div><script type="module">
      import React from '/ui/node_modules/.vite/deps/react.js';
      import ReactDOM from '/ui/node_modules/.vite/deps/react-dom_client.js';
      import RefreshRuntime from '/ui/@react-refresh';
      RefreshRuntime.injectIntoGlobalHook(window);
      window.$RefreshReg$ = () => {}; window.$RefreshSig$ = () => type => type;
      window.__vite_plugin_react_preamble_installed__ = true;
      const {useChatSpeech} = await import('/ui/src/features/chat/useChatSpeech.ts');
      const {useTalkMode} = await import('/ui/src/features/chat/useTalkMode.ts');
      window.requests = []; window.plays = []; window.pauses = 0; window.instances = [];
      const original = window.fetch.bind(window);
      window.fetch = (input, init) => {
        if (!String(input).endsWith('/v1/audio/speech')) return original(input, init);
        // Deliberately ignore abort to model late transport completion.
        return new Promise(resolve => window.requests.push({text: JSON.parse(init.body).input, signal: init.signal, complete: () => resolve(new Response(new Uint8Array(100), {headers: {'Content-Type':'audio/wav'}}))}));
      };
      // Explicit fake media sink: no browser decoding or audio device access.
      window.Audio = class extends EventTarget {
        constructor() {super(); this.addEventListener('ended', () => this.onended?.()); this.addEventListener('error', () => this.onerror?.());}
        play() {const entry = {player:this}; window.plays.push(entry); return window.deferPlay ? new Promise((resolve,reject) => {entry.resolve=resolve; entry.reject=reject;}) : Promise.resolve();}
        pause() {window.pauses++;} load() {} removeAttribute() {this.src='';} getAttribute() {return this.src || null;}
      };
      window.source = id => ({kind:'canonical',session:'fixture-session',conversationID:'fixture-conversation',turnID:id});
      function Harness() {
        window.talk = useTalkMode(); window.chatSpeech = useChatSpeech({asr:'',tts:'',voice:''}, 'en');
        return React.createElement('output', null, window.talk.state + ':' + window.chatSpeech.speechState);
      }
      window.begin = id => {
        window.talk.active.current = true; window.talk.setState('thinking');
        const speech = window.chatSpeech.beginSpeech(true, window.talk, {current:true}, {sessionKey:'fixture-session',requestID:id}, window.source(id));
        window.instances.push(speech); return speech;
      };
      window.root = ReactDOM.createRoot(document.getElementById('root')); window.root.render(React.createElement(Harness));
    </script>`}));
  await page.goto('/ui/speech-delivery.html');
  await expect(page.locator('output')).toHaveText('off:idle');
}
const receipt = (page: Page) => page.evaluate(() => (window as any).chatSpeech.delivery.current);
const count = (page: Page, key: 'requests'|'plays', n: number) => expect.poll(() => page.evaluate(key => (window as any)[key].length, key)).toBe(n);
const complete = (page: Page, n: number) => page.evaluate(n => (window as any).requests[n].complete(), n);
const ended = (page: Page, n: number) => page.evaluate(n => (window as any).plays[n].player.dispatchEvent(new Event('ended')), n);
const twoChunks = 'First sentence. ' + 'Second response '.repeat(12) + '.';

test('synthesis and play resolution do not establish device completion; ended re-arms the exact Talk turn', async ({page}) => {
  await mount(page);
  await page.evaluate(() => (window as any).begin('one').finish('Committed answer.'));
  await count(page, 'requests', 1);
  expect(await receipt(page)).toMatchObject({state:'active',committed:true,queued:{state:'synthesizing'},devicePlaybackCompleted:{chunks:0,text:''}});
  await complete(page, 0); await count(page, 'plays', 1);
  expect(await receipt(page)).toMatchObject({state:'active',synthesizedChunks:1,playing:{state:'playback_started'},devicePlaybackCompleted:{chunks:0,text:''}});
  await ended(page, 0);
  await expect(page.locator('output')).toHaveText('listening:idle');
  expect(await receipt(page)).toMatchObject({state:'completed',generatedCharacters:17,source:{turnID:'one'},devicePlaybackCompleted:{chunks:1,text:'Committed answer.'}});
});

test('Stop records unknown partial playback, discards lookahead, and fences late audio', async ({page}) => {
  await mount(page);
  await page.evaluate(text => (window as any).begin('one').finish(text), twoChunks);
  await count(page,'requests',1); await complete(page,0); await count(page,'plays',1); await count(page,'requests',2);
  await page.evaluate(() => (window as any).chatSpeech.stopSpeech());
  const stopped = await receipt(page);
  expect(stopped).toMatchObject({state:'interrupted',uncertainPartial:{text:'First sentence.',state:'partial_unknown'},discardedChunks:1,devicePlaybackCompleted:{chunks:0,text:''}});
  expect(stopped.playing).toBeUndefined(); expect(stopped.queued).toBeUndefined();
  expect(await page.evaluate(() => (window as any).requests.every((r:any)=>r.signal.aborted))).toBe(true);
  expect(await page.evaluate(() => (window as any).plays[0].player.getAttribute('src'))).toBeNull();
  await complete(page,1); await page.waitForTimeout(30);
  await count(page,'plays',1); expect(await receipt(page)).toEqual(stopped);
});

test('completed device chunks remain separate from the interrupted next chunk', async ({page}) => {
  await mount(page);
  await page.evaluate(text => (window as any).begin('one').finish(text), twoChunks);
  await count(page,'requests',1); await complete(page,0); await count(page,'plays',1); await count(page,'requests',2);
  await complete(page,1); await ended(page,0); await count(page,'plays',2);
  await page.evaluate(() => (window as any).chatSpeech.stopSpeech());
  const stopped = await receipt(page);
  expect(stopped).toMatchObject({state:'interrupted',devicePlaybackCompleted:{chunks:1,text:'First sentence.'},uncertainPartial:{state:'partial_unknown',sequence:2}});
  expect(stopped.uncertainPartial.text).toBe(twoChunks.slice(16));
});

test('Stop before synthesis completion records no played or partial chunk', async ({page}) => {
  await mount(page);
  await page.evaluate(() => (window as any).begin('one').finish('Not yet played.'));
  await count(page,'requests',1);
  await page.evaluate(() => (window as any).chatSpeech.stopSpeech());
  await complete(page,0); await page.waitForTimeout(30);
  await count(page,'plays',0);
  const stopped = await receipt(page);
  expect(stopped).toMatchObject({state:'interrupted',synthesizedChunks:0,discardedChunks:1,devicePlaybackCompleted:{chunks:0,text:''}});
  expect(stopped.uncertainPartial).toBeUndefined();
});

test('old canonical answer, failure and late audio cannot finish or stop the next owner', async ({page}) => {
  await mount(page);
  await page.evaluate(() => (window as any).begin('old').finish('Old answer.'));
  await count(page,'requests',1);
  const result = await page.evaluate(() => {
    const f=window as any; f.begin('new');
    return [f.chatSpeech.finishSpeech('Old final.', f.source('old')), f.chatSpeech.stopSpeechFor(f.source('old')), f.instances[0].bindSource(f.source('new')), f.chatSpeech.finishSpeech('New answer.', {...f.source('new'),runID:'new-run'})];
  });
  expect(result).toEqual([false,false,false,true]);
  await count(page,'requests',2); await complete(page,0); await complete(page,1); await count(page,'plays',1);
  expect(await receipt(page)).toMatchObject({source:{turnID:'new',runID:'new-run'},playing:{text:'New answer.'},devicePlaybackCompleted:{chunks:0}});
  await ended(page,0); await expect(page.locator('output')).toHaveText('listening:idle');
});

test('completed old playback cannot re-arm a newer owner from its queued callback', async ({page}) => {
  await mount(page);
  await page.evaluate(() => (window as any).begin('old').finish('Old answer.'));
  await count(page,'requests',1); await complete(page,0); await count(page,'plays',1);
  await page.evaluate(async () => {
    const f=window as any; f.plays[0].player.dispatchEvent(new Event('ended'));
    await Promise.resolve(); await Promise.resolve(); f.begin('new');
  });
  await page.waitForTimeout(30);
  await expect(page.locator('output')).toHaveText('thinking:waiting');
  expect(await receipt(page)).toMatchObject({state:'active',source:{turnID:'new'},devicePlaybackCompleted:{chunks:0}});
});

test('late play promise rejection from an ended chunk cannot remove the next playback handlers', async ({page}) => {
  await mount(page);
  await page.evaluate(text => {const f=window as any; f.deferPlay=true; f.begin('one').finish(text);}, twoChunks);
  await count(page,'requests',1); await complete(page,0); await count(page,'plays',1); await count(page,'requests',2);
  await complete(page,1); await ended(page,0); await count(page,'plays',2);
  await page.evaluate(async () => {const f=window as any; f.plays[0].reject(new Error('late rejected play')); await Promise.resolve();});
  await ended(page,1); await expect(page.locator('output')).toHaveText('listening:idle');
  expect(await receipt(page)).toMatchObject({state:'completed',devicePlaybackCompleted:{chunks:2,text:twoChunks}});
});

test('Stop while play is unresolved retains uncertainty without claiming playback started', async ({page}) => {
  await mount(page);
  await page.evaluate(() => {const f=window as any; f.deferPlay=true; f.begin('one').finish('Pending playback.');});
  await count(page,'requests',1); await complete(page,0); await count(page,'plays',1);
  expect(await receipt(page)).toMatchObject({playing:{state:'playback_requested'},devicePlaybackCompleted:{chunks:0}});
  await page.evaluate(() => (window as any).chatSpeech.stopSpeech());
  const stopped = await receipt(page);
  expect(stopped).toMatchObject({state:'interrupted',uncertainPartial:{state:'partial_unknown',playbackEvidence:'playback_requested'},devicePlaybackCompleted:{chunks:0}});
  await page.evaluate(async () => {const f=window as any; f.plays[0].resolve(); await Promise.resolve();});
  expect(await receipt(page)).toEqual(stopped);
});

test('playback failure retains uncertainty and pauses Talk without committing delivery', async ({page}) => {
  await mount(page);
  await page.evaluate(() => {const f=window as any; f.deferPlay=true; f.begin('one').finish('Blocked playback.');});
  await count(page,'requests',1); await complete(page,0); await count(page,'plays',1);
  await page.evaluate(() => (window as any).plays[0].reject(new Error('Playback blocked')));
  await expect(page.locator('output')).toHaveText('needs_attention:idle');
  expect(await receipt(page)).toMatchObject({state:'failed',uncertainPartial:{playbackEvidence:'playback_requested'},devicePlaybackCompleted:{chunks:0}});
  expect(await page.evaluate(() => (window as any).chatSpeech.speechError)).toBe('Playback blocked');
});
