import { expect, test, type Page } from '@playwright/test';

test.skip(!!process.env.OFFGRID_E2E_URL, 'Source-hook renderer tests require the isolated Vite fixture; packaged-service voice tests are separate.');

async function mountTalk(page: Page) {
  await page.route('**/v1/audio/status', route => route.fulfill({ json: { asr: { available: true }, tts: { available: true } } }));
  // This isolated React harness mounts the production hook, without Chat, a
  // service, microphone permission, audio playback, or model execution.
  await page.route('**/ui/talk-lifetime.html', route => route.fulfill({ contentType: 'text/html', body: `
    <div id="root"></div><script type="module">
      import React from '/ui/node_modules/.vite/deps/react.js';
      import ReactDOM from '/ui/node_modules/.vite/deps/react-dom_client.js';
      import RefreshRuntime from '/ui/@react-refresh';
      RefreshRuntime.injectIntoGlobalHook(window);
      window.$RefreshReg$ = () => {}; window.$RefreshSig$ = () => type => type;
      window.__vite_plugin_react_preamble_installed__ = true;
      const {useTalkMode} = await import('/ui/src/features/chat/useTalkMode.ts');
      window.pending = []; window.submitted = [];
      window.send = value => {window.submitted.push(value); return new Promise((resolve,reject)=>window.pending.push({resolve,reject}));};
      function Harness() {window.talk = useTalkMode(); return React.createElement('output', null, window.talk.state);}
      window.root = ReactDOM.createRoot(document.getElementById('root')); window.root.render(React.createElement(Harness));
    </script>` }));
  await page.goto('/ui/talk-lifetime.html');
  await expect(page.locator('output')).toHaveText('off');
}

async function startTalk(page: Page) {
  await page.evaluate(async () => {
    const fixture = window as any;
    await fixture.talk.start({ eligible: true, hasDraft: false, preferences: { asr: '', tts: '', voice: '' }, locale: 'en', unavailable: 'unavailable', draftError: 'draft', setError: () => {} });
  });
  await expect(page.locator('output')).toHaveText('listening');
}

async function submit(page: Page, value: string) {
  await page.evaluate(value => {
    const fixture = window as any;
    fixture.talk.acceptTranscript(value, 'isolated-talk-draft', fixture.send);
  }, value);
  await expect(page.locator('output')).toHaveText('thinking');
}

for (const outcome of ['not-sent', 'rejected', 'sent'] as const) {
  test(`old Talk ${outcome} settlement cannot change a restarted session`, async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await mountTalk(page);
    await startTalk(page);
    await submit(page, 'original utterance');
    await page.evaluate(() => (window as any).talk.reset());
    await expect(page.locator('output')).toHaveText('off');
    await startTalk(page);
    await submit(page, 'new utterance');
    await page.evaluate(async outcome => {
      const pending = (window as any).pending[0];
      if (outcome === 'rejected') pending.reject(new Error('late old send failure'));
      else pending.resolve(outcome === 'sent');
      await new Promise(resolve => setTimeout(resolve, 30));
    }, outcome);
    await expect(page.locator('output')).toHaveText('thinking');
    expect(await page.evaluate(() => (window as any).talk.active.current)).toBe(true);
    expect(await page.evaluate(() => (window as any).submitted)).toEqual(['original utterance', 'new utterance']);
    expect(errors).toEqual([]);
    await page.evaluate(() => (window as any).root.unmount());
  });
}

for (const reject of [false, true]) {
  test(`earlier utterance ${reject ? 'rejection' : 'not-sent result'} cannot pause a newer utterance in the same Talk session`, async ({page}) => {
    await mountTalk(page); await startTalk(page); await submit(page, 'earlier utterance');
    // Playback completion re-arms capture without restarting the Talk session.
    await page.evaluate(() => {const talk=(window as any).talk; talk.setState('listening'); talk.nextCycle();});
    await expect(page.locator('output')).toHaveText('listening');
    await submit(page, 'newer utterance');
    await page.evaluate(async reject => {
      const pending = (window as any).pending[0];
      if (reject) pending.reject(new Error('late earlier utterance failure')); else pending.resolve(false);
      await new Promise(resolve => setTimeout(resolve, 30));
    }, reject);
    await expect(page.locator('output')).toHaveText('thinking');
    expect(await page.evaluate(() => (window as any).submitted)).toEqual(['earlier utterance', 'newer utterance']);
    expect(await page.evaluate(() => (window as any).talk.active.current)).toBe(true);
  });

  test(`current Talk ${reject ? 'rejection' : 'not-sent result'} still pauses for review`, async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await mountTalk(page);
    await startTalk(page);
    await submit(page, 'retained utterance');
    await page.evaluate(reject => {
      const pending = (window as any).pending[0];
      if (reject) pending.reject(new Error('current send failure')); else pending.resolve(false);
    }, reject);
    await expect(page.locator('output')).toHaveText('needs_attention');
    expect(await page.evaluate(() => (window as any).talk.active.current)).toBe(false);
    expect(await page.evaluate(() => (window as any).submitted)).toEqual(['retained utterance']);
    expect(errors).toEqual([]);
    await page.evaluate(() => (window as any).root.unmount());
  });
}

test('one capture admits only one final transcript before React renders and rejects its stale callback after re-arm', async ({page}) => {
  await mountTalk(page); await startTalk(page);
  await page.evaluate(() => {
    const f=window as any; f.oldAccept=f.talk.acceptTranscript;
    f.oldAccept('first final','isolated-talk-draft',f.send);
    f.oldAccept('duplicate final','isolated-talk-draft',f.send);
  });
  await expect(page.locator('output')).toHaveText('thinking');
  expect(await page.evaluate(()=>(window as any).submitted)).toEqual(['first final']);
  await page.evaluate(() => {const f=window as any; f.talk.speakAgain(false,()=>{});});
  await expect(page.locator('output')).toHaveText('listening');
  await page.evaluate(() => {
    const f=window as any; f.oldAccept('late stale final','isolated-talk-draft',f.send);
    f.talk.acceptTranscript('next capture final','isolated-talk-draft',f.send);
  });
  expect(await page.evaluate(()=>(window as any).submitted)).toEqual(['first final','next capture final']);
});

test('late answer acknowledgment cannot re-arm a superseding capture or ended Talk session',async({page})=>{
  await mountTalk(page);await startTalk(page);await submit(page,'answer one');
  await page.evaluate(()=>{
    const f=window as any;f.oldAcknowledgment=f.talk.resumeAfterInput;f.talk.speakAgain(false,()=>{});
  });
  await expect(page.locator('output')).toHaveText('listening');
  await submit(page,'new question');
  await page.evaluate(async()=>{const f=window as any;f.oldAcknowledgment();await new Promise(resolve=>setTimeout(resolve,30));});
  await expect(page.locator('output')).toHaveText('thinking');
  await page.evaluate(async()=>{const f=window as any;f.talk.resumeAfterInput();f.talk.reset();await new Promise(resolve=>setTimeout(resolve,30));});
  await expect(page.locator('output')).toHaveText('off');
});

test('late capture phase cannot restore transcribing after its final transcript was admitted',async({page})=>{
  await mountTalk(page);await startTalk(page);
  await page.evaluate(()=>(window as any).oldPhase=(window as any).talk.capturePhase);
  await submit(page,'finalized transcript');
  await page.evaluate(()=>(window as any).oldPhase('transcribing'));
  await expect(page.locator('output')).toHaveText('thinking');
});
