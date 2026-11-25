import {expect, test, type Page} from '@playwright/test';
import {canonicalTalkFixture} from './fixtures/canonical-talk';
import {resolve} from 'node:path';
import {readFile, mkdir} from 'node:fs/promises';
import {createHash} from 'node:crypto';

async function continuousFixture(page: Page, echoCancellation: boolean | 'all' = true, noiseSuppression = true) {
  await page.addInitScript(() => {(window as any).nativeCaptureContext = window.AudioContext;});
  const fixture = await canonicalTalkFixture(page);
  await page.addInitScript(({echoCancellation, noiseSuppression}) => {
    const state = window as any;
    window.AudioContext = state.nativeCaptureContext;
    Object.defineProperty(navigator, 'mediaDevices', {configurable: true, value: {getUserMedia: async () => {
      state.mic.permissions++;
      const context = new state.nativeCaptureContext();
      const oscillator = context.createOscillator(), gain = context.createGain(), output = context.createMediaStreamDestination();
      oscillator.frequency.value = 440; gain.gain.value = 0;
      oscillator.connect(gain); gain.connect(output); oscillator.start();
      await context.resume();
      const track = output.stream.getAudioTracks()[0], stop = track.stop.bind(track);
      state.syntheticCapture = {context, gain, track, oscillator};
      track.stop = () => {state.mic.stops++; stop(); oscillator.stop(); void context.close();};
      // Synthetic configuration exercises gating; it is not measured acoustic AEC.
      track.getSettings = () => ({echoCancellation, noiseSuppression});
      return output.stream;
    }}});
  }, {echoCancellation, noiseSuppression});
  await page.reload();
  await expect(page.locator('.canonical-response-summary')).toBeVisible();
  await page.getByRole('button', {name: 'Talk', exact: true}).click();
  await expect(page.locator('[data-capture-phase=monitoring]')).toHaveCount(1);
  return fixture;
}

for (const theme of ['light','dark']) test(`capture documentation automatic Talk ${theme}`, async ({page}) => {
  test.skip(!process.env.OFFGRID_DOC_CAPTURES, 'Opt-in documentation capture with synthetic microphone input.');
  await page.setViewportSize({width:1440,height:900});
  await page.emulateMedia({reducedMotion:'reduce'});
  await continuousFixture(page);
  await page.evaluate(theme => {document.documentElement.dataset.theme=theme;},theme);
  const surface=page.locator('.talk-surface');
  await expect(surface.locator('.talk-surface-copy [role=status]')).toHaveText('Listening');
  await expect(surface.getByRole('button')).toHaveCount(1);
  await expect(surface).toHaveCSS('opacity','1');
  await page.evaluate(()=>document.fonts.ready);
  await mkdir(resolve('../../build/documentation-captures'),{recursive:true});
  await page.screenshot({path:resolve(`../../build/documentation-captures/voice-listening-${theme}.png`),animations:'disabled'});
  await page.keyboard.press('Escape');
  await expect(surface).toHaveCount(0);
});

for (const theme of ['light','dark']) for (const viewport of [{width:630,height:790},{width:360,height:640},{width:360,height:320}]) {
  test(`Talk controls stay separate with one status at ${viewport.width}x${viewport.height} ${theme}`, async ({page}) => {
    await page.setViewportSize(viewport);
    await page.emulateMedia({reducedMotion:'reduce'});
    const fixture = await continuousFixture(page);
    await page.evaluate(theme => {document.documentElement.dataset.theme=theme;},theme);
    const surface=page.locator('.talk-surface');
    const screenshot=async (phase:string) => page.screenshot({path:resolve(`../../build/voice-talk-layout-polish-20261124/${viewport.width}x${viewport.height}-${theme}-${phase}.png`),animations:'disabled'});
    const geometry=async()=>{
      await expect(surface.getByRole('button',{name:'Stop Talk',exact:true})).toBeInViewport();
      await expect(surface.getByRole('button',{name:'Stop Talk',exact:true})).toBeEnabled();
      await expect(surface.locator('[role=status]:visible')).toHaveCount(1);
      expect(await surface.evaluate(element=>element.scrollWidth<=element.clientWidth)).toBe(true);
      const boxes=await surface.locator('button:visible').evaluateAll(buttons=>buttons.map(button=>{
        const rect=button.getBoundingClientRect(),svg=button.querySelector('svg')?.getBoundingClientRect();
        return {x:rect.x,y:rect.y,width:rect.width,height:rect.height,icon:svg?{x:svg.x,y:svg.y,width:svg.width,height:svg.height}:null};
      }));
      for(const box of boxes){
        expect(box.width).toBeGreaterThanOrEqual(44);expect(box.height).toBeGreaterThanOrEqual(44);
        expect(box.x).toBeGreaterThanOrEqual(0);expect(box.y).toBeGreaterThanOrEqual(0);
        expect(box.x+box.width).toBeLessThanOrEqual(viewport.width);expect(box.y+box.height).toBeLessThanOrEqual(viewport.height);
        if(box.icon){expect(box.icon.width).toBeLessThanOrEqual(20);expect(box.icon.height).toBeLessThanOrEqual(20);expect(box.icon.x).toBeGreaterThanOrEqual(box.x);expect(box.icon.x+box.icon.width).toBeLessThanOrEqual(box.x+box.width);}
      }
      for(let i=0;i<boxes.length;i++)for(let j=i+1;j<boxes.length;j++){
        const a=boxes[i],b=boxes[j];expect(a.x+a.width<=b.x||b.x+b.width<=a.x||a.y+a.height<=b.y||b.y+b.height<=a.y).toBe(true);
      }
    };
    await expect(surface.getByRole('button')).toHaveCount(1);
    const stopPosition = await surface.getByRole('button',{name:'Stop Talk',exact:true}).boundingBox();
    expect(stopPosition!.x + stopPosition!.width / 2).toBeCloseTo(viewport.width / 2, 0);
    expect(await surface.locator('.talk-surface-controls').evaluate(element=>getComputedStyle(element).borderTopWidth)).toBe('0px');
    await geometry();await screenshot('monitoring');
    await page.keyboard.press('Tab');
    await expect(surface.getByRole('button',{name:'Stop Talk',exact:true})).toBeFocused();
    await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value=.15;});
    await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
    await expect(surface.getByRole('button')).toHaveCount(3);
    await expect(surface.locator('.voice-duration')).toHaveCount(0);
    expect(await surface.getByRole('button',{name:'Stop Talk',exact:true}).boundingBox()).toEqual(stopPosition);
    await geometry();await screenshot('recording');
    await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value=0;});
    await page.getByRole('button',{name:'Stop recording',exact:true}).click();
    await expect.poll(()=>page.evaluate(()=>(window as any).mic.pending.length)).toBe(1);
    await expect(surface.locator('.talk-surface-copy [role=status]')).toHaveText('Transcribing');
    await expect(surface.getByRole('button')).toHaveCount(1);
    await geometry();await screenshot('transcribing');
    await page.evaluate(() => (window as any).mic.pending[0].finish('Explain the offline option.'));
    await expect.poll(() => fixture.submissions().length).toBe(1);
    fixture.complete(1, 'The offline option keeps processing local.');
    await expect(surface.locator('.talk-surface-copy [role=status]')).toHaveText('Speaking');
    const again=surface.getByRole('button',{name:'Speak again',exact:true});
    await expect(again).toBeVisible();
    expect(await again.evaluate(element=>{const range=document.createRange();range.selectNodeContents(element);return range.getClientRects().length;})).toBe(1);
    expect(await surface.getByRole('button',{name:'Stop Talk',exact:true}).boundingBox()).toEqual(stopPosition);
    await geometry();await screenshot('speaking');
    await surface.getByRole('button',{name:'Stop Talk',exact:true}).click();
    await expect(surface).toHaveCount(0);
    await expect.poll(()=>page.evaluate(()=>(window as any).mic.stops)).toBe(1);
  });
}

test('one real AudioWorklet stream interrupts playback before transcription and retains canonical work', async ({page}) => {
  const fixture = await continuousFixture(page);
  const onset = async () => {
    await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
    await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  };
  const finish = async (text: string, index: number) => {
    await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = 0;});
    await page.getByRole('button', {name: 'Stop recording', exact: true}).click();
    await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(index + 1);
    await page.evaluate(({text, index}) => (window as any).mic.pending[index].finish(text), {text, index});
    await expect.poll(() => fixture.submissions().length).toBe(index + 1);
  };
  await onset(); await finish('Explain the offline option.', 0);
  fixture.complete(1, 'The offline option keeps processing local.');
  await expect.poll(() => fixture.spoken.length).toBe(1);
  await expect.poll(() => page.evaluate(() => (window as any).players.length)).toBe(1);
  await onset();
  expect(await page.evaluate(() => (window as any).players[0].paused)).toBe(true);
  expect(await page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  expect(fixture.commands.some(command => command.kind === 'stop')).toBe(false);
  await finish('Actually use the other option.', 1);
  expect(fixture.submissions()[1].payload.origin).toBe('dictation');
  expect(await page.evaluate(() => (window as any).mic.permissions)).toBe(1);
  await page.getByRole('button', {name: 'Stop Talk', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.stops)).toBe(1);
  expect(fixture.submissions()).toHaveLength(2);
});

test('quiet gapped speech interrupts and submits hands-free with all-mode AEC and no noise suppression', async ({page}) => {
  const fixture = await continuousFixture(page, 'all', false);
  const utterance = async () => {
    await page.evaluate(() => {
      const {context, gain} = (window as any).syntheticCapture;
      const now = context.currentTime;
      gain.gain.cancelScheduledValues(now);
      for (let i = 0; i < 8; i++) {
        gain.gain.setValueAtTime(.012, now + i * .18);
        gain.gain.setValueAtTime(0, now + i * .18 + .14);
      }
    });
    await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  };
  const recognized = async (text: string, index: number) => {
    // The production pause detector finishes capture; no recording control is clicked.
    await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(index + 1);
    await page.evaluate(({text, index}) => (window as any).mic.pending[index].finish(text), {text, index});
    await expect.poll(() => fixture.submissions().length).toBe(index + 1);
  };
  await utterance(); await recognized('Explain the offline option.', 0);
  fixture.complete(1, 'The offline option keeps processing local.');
  await expect.poll(() => page.evaluate(() => (window as any).players.length)).toBe(1);
  await utterance();
  await expect.poll(() => page.locator('.talk-visual').evaluate(element => Number((element as HTMLElement).style.getPropertyValue('--voice-energy')))).toBeGreaterThan(.1);
  expect(await page.evaluate(() => (window as any).players[0].paused)).toBe(true);
  expect(await page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  await recognized('Actually use the other option.', 1);
  expect(fixture.submissions()[1].payload.origin).toBe('dictation');
  expect(fixture.commands.some(command => command.kind === 'stop')).toBe(false);
  expect(await page.evaluate(() => (window as any).mic.permissions)).toBe(1);
  await page.getByRole('button', {name:'Stop Talk', exact:true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.stops)).toBe(1);
});

test('known recorded speech at reduced gain interrupts playback through the real capture graph', async ({page}, testInfo) => {
  test.skip(!process.env.OFFGRID_KNOWN_SPEECH_WAV, 'Opt in with an existing public speech recording; never records a microphone.');
  const wav = await readFile(process.env.OFFGRID_KNOWN_SPEECH_WAV!);
  expect(wav.length).toBeLessThan(12 * 1024 * 1024);
  await testInfo.attach('recording-provenance', {body: JSON.stringify({bytes:wav.length, sha256:createHash('sha256').update(wav).digest('hex'), gain:.05}), contentType:'application/json'});
  await page.route('**/known-voice-fixture.wav', route => route.fulfill({contentType:'audio/wav', body:wav}));
  const fixture = await continuousFixture(page);
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value=.15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value=0;});
  await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  await page.evaluate(() => (window as any).mic.pending[0].finish('Explain the offline option.'));
  await expect.poll(() => fixture.submissions().length).toBe(1);
  fixture.complete(1, 'The offline option keeps processing local.');
  await expect.poll(() => page.evaluate(() => (window as any).players.length)).toBe(1);
  await page.evaluate(async () => {
    const {context, gain, oscillator} = (window as any).syntheticCapture;
    oscillator.disconnect();
    const source = context.createBufferSource();
    source.buffer = await context.decodeAudioData(await (await fetch('/known-voice-fixture.wav')).arrayBuffer());
    source.connect(gain); gain.gain.value=.05; source.start();
  });
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  expect(await page.evaluate(() => (window as any).players[0].paused)).toBe(true);
  expect(await page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  expect(fixture.commands.some(command => command.kind==='stop')).toBe(false);
  await page.getByRole('button', {name:'Stop Talk', exact:true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.stops)).toBe(1);
});

test('Stop Talk aborts recognition and fences its late transcript', async ({page}) => {
  const fixture = await continuousFixture(page);
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await page.getByRole('button', {name: 'Stop recording', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  await page.getByRole('button', {name: 'Stop Talk', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.stops)).toBe(1);
  await page.evaluate(() => (window as any).mic.pending[0].finish('Do not submit this late transcript.'));
  await expect(page.getByRole('button', {name: 'Talk', exact: true})).toBeVisible();
  expect(fixture.submissions()).toHaveLength(0);
  // Explicit Stop retains canonical job cancellation; automatic onset does not.
  await expect.poll(() => fixture.commands.filter(command => command.kind === 'stop').length).toBe(1);
  expect(fixture.commands.find(command => command.kind === 'stop').target_id).toBe('original-input');
});

test('device loss discards the partial utterance and releases its capture', async ({page}) => {
  const fixture = await continuousFixture(page);
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await page.evaluate(() => (window as any).syntheticCapture.track.dispatchEvent(new Event('ended')));
  await expect.poll(() => page.evaluate(() => (window as any).mic.stops)).toBe(1);
  await expect(page.locator('.talk-surface-error')).toBeVisible();
  expect(await page.evaluate(() => (window as any).mic.pending.length)).toBe(0);
  expect(fixture.submissions()).toHaveLength(0);
});

test('unverified echo cancellation prevents automatic interruption during playback', async ({page}) => {
  const fixture = await continuousFixture(page, false);
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = 0;});
  await page.getByRole('button', {name: 'Stop recording', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  await page.evaluate(() => (window as any).mic.pending[0].finish('Explain the offline option.'));
  await expect.poll(() => fixture.submissions().length).toBe(1);
  fixture.complete(1, 'The offline option keeps processing local.');
  await expect.poll(() => page.evaluate(() => (window as any).players.length)).toBe(1);
  const onset = await page.evaluate(() => {
    const capture = (window as any).syntheticCapture;
    capture.gain.gain.value = .15; return capture.context.currentTime;
  });
  // Exercise several onset windows, measured by the actual audio sample clock.
  await expect.poll(() => page.evaluate(() => (window as any).syntheticCapture.context.currentTime)).toBeGreaterThan(onset + .6);
  expect(await page.evaluate(() => (window as any).players[0].paused)).toBe(false);
  await expect(page.locator('.voice-interruption-hint')).toContainText('Choose Speak again');
  await expect(page.locator('[data-capture-phase=monitoring]')).toHaveCount(1);
  expect(fixture.submissions()).toHaveLength(1);
  await page.getByRole('button', {name: 'Stop Talk', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.stops)).toBe(1);
});

test('speech can interrupt silent synthesis without echo cancellation and cancels late audio', async ({page}) => {
  const fixture = await continuousFixture(page, false);
  let releaseSpeech: (() => Promise<void>) | undefined;
  await page.route('**/v1/audio/speech', route => new Promise<void>(resolve => {
    releaseSpeech = async () => { await route.fulfill({contentType:'audio/wav',body:Buffer.alloc(100)}); resolve(); };
  }));
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = 0;});
  await page.getByRole('button', {name:'Stop recording',exact:true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  await page.evaluate(() => (window as any).mic.pending[0].finish('Explain the first option.'));
  await expect.poll(() => fixture.submissions().length).toBe(1);
  fixture.complete(1, 'The first answer is being synthesized.');
  await expect.poll(() => !!releaseSpeech).toBe(true);
  expect(await page.evaluate(() => (window as any).players.length)).toBe(0);
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await releaseSpeech!();
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = 0;});
  await page.getByRole('button', {name:'Stop recording',exact:true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(2);
  await page.evaluate(() => (window as any).mic.pending[1].finish('Use the second option instead.'));
  await expect.poll(() => fixture.submissions().length).toBe(2);
  expect(await page.evaluate(() => (window as any).players.length)).toBe(0);
  await page.getByRole('button', {name:'Stop Talk',exact:true}).click();
});

test('creating the first saved conversation retains one capture and permits the next spoken turn', async ({page}) => {
  await continuousFixture(page);
  await page.getByRole('button', {name:'Stop Talk',exact:true}).click();
  await page.route('**/api/v2/system', route => route.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:[]}}));
  let created: any, creates = 0;
  await page.route('**/v1/sessions', route => {
    if (route.request().method() !== 'POST') return route.fallback();
    creates++;
    created={...route.request().postDataJSON(),messages:[]};
    return route.fulfill({json:created});
  });
  await page.reload();
  await page.getByRole('button', {name:'New chat',exact:true}).click();
  await page.evaluate(() => {
    const f=window as any, fetch=window.fetch.bind(window); f.generated=[];
    window.fetch=(input,init)=>{
      if(!String(input).endsWith('/generate'))return fetch(input,init);
      f.generated.push(String(input));
      const name=decodeURIComponent(String(input).split('/sessions/')[1].split('/generate')[0]);
      const message={role:'assistant',content:'A saved spoken reply.'};
      const done={type:'done',message,session:{name,messages:[message]},metrics:{first_text_ms:1,context_window:8192}};
      return Promise.resolve(new Response(`data: ${JSON.stringify(done)}\n\n`,{headers:{'Content-Type':'text/event-stream'}}));
    };
  });
  await page.getByRole('button', {name:'Talk',exact:true}).click();
  await expect(page.locator('[data-capture-phase=monitoring]')).toHaveCount(1);
  for(let index=0;index<2;index++){
    await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value=.15;});
    await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
    await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value=0;});
    await page.getByRole('button', {name:'Stop recording',exact:true}).click();
    await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(index+1);
    await page.evaluate(index => (window as any).mic.pending[index].finish(index?'And explain the other option.':'Start a new spoken conversation.'),index);
    await expect.poll(() => page.evaluate(() => (window as any).generated.length)).toBe(index+1);
    await expect.poll(() => page.evaluate(() => (window as any).players.length)).toBe(index+1);
  }
  expect(creates).toBe(1);
  expect(await page.evaluate(() => (window as any).mic.permissions)).toBe(1);
  expect(await page.evaluate(() => (window as any).players[0].paused)).toBe(true);
  expect(await page.evaluate(() => (window as any).generated)).toEqual(Array(2).fill(`/v1/sessions/${encodeURIComponent(created.name)}/generate`));
  await page.getByRole('button', {name:'Stop Talk',exact:true}).click();
});

test('empty recognition and discarded continuous utterances re-arm without sending or replay', async ({page}) => {
  const fixture = await continuousFixture(page);
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = 0;});
  await page.getByRole('button', {name: 'Stop recording', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  await page.evaluate(() => (window as any).mic.pending[0].finish(''));
  await expect(page.locator('[data-capture-phase=monitoring]')).toHaveCount(1);
  await expect(page.locator('.talk-surface-copy [role=status]')).toHaveText('Listening');
  expect(fixture.submissions()).toHaveLength(0);
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = .15;});
  await expect(page.locator('[data-capture-phase=recording]')).toBeVisible();
  await page.evaluate(() => {(window as any).syntheticCapture.gain.gain.value = 0;});
  await page.locator('.talk-surface .voice-discard').click();
  await expect(page.locator('[data-capture-phase=monitoring]')).toHaveCount(1);
  expect(await page.evaluate(() => (window as any).mic.pending.length)).toBe(1);
  expect(fixture.submissions()).toHaveLength(0);
  expect(await page.evaluate(() => (window as any).mic.permissions)).toBe(1);
  await page.getByRole('button', {name: 'Stop Talk', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).mic.stops)).toBe(1);
});
