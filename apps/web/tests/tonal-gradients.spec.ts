import { test, expect } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

for (const theme of ['light', 'dark']) test(`tonal gradients preserve contrast and flatten completely: ${theme}`, async ({ page, browser }, info) => {
  await craftFixture(page, 'en', theme);
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('A test draft');
  const treatments=['.app-shell > .sidebar','.chat-workspace .composer','.composer-actions > button:last-child'];
  for (const selector of treatments) await expect(page.locator(selector)).toHaveCSS('background-image', /linear-gradient/);
  await expect(page.locator('.composer-actions .voice-button')).toHaveCSS('background-image','none');
  const contrast = await page.evaluate(() => {
    const probe=document.createElement('i'); document.body.append(probe);
    const canvas=document.createElement('canvas'); canvas.width=canvas.height=1;
    const ctx=canvas.getContext('2d', {willReadFrequently:true})!;
    const luminance=(color:string)=>{
      probe.style.color=color; ctx.clearRect(0,0,1,1);ctx.fillStyle=getComputedStyle(probe).color;ctx.fillRect(0,0,1,1);
      const rgb=ctx.getImageData(0,0,1,1).data;
      return [.2126,.7152,.0722].reduce((sum,w,i)=>{const c=rgb[i]/255;return sum+w*(c<=.04045?c/12.92:((c+.055)/1.055)**2.4);},0);
    };
    const ratio=(a:string,b:string)=>{const x=luminance(a),y=luminance(b);return(Math.max(x,y)+.05)/(Math.min(x,y)+.05);};
    const pairs:Record<string,number>={};
    // Sample the full neutral envelope, including points beyond the actual
    // light composer endpoint: a conservative bound, not just midpoint contrast.
    for(const foreground of ['text','muted','faint']) for(const [name,a,b] of [['sidebar','surface-0','surface-1'],['composer','surface-1','surface-2']]) {
      pairs[`${name}/${foreground}`]=Math.min(...Array.from({length:101},(_,p)=>ratio(`var(--${foreground})`,`color-mix(in oklab, var(--${a}) ${p}%, var(--${b}))`)));
    }
    pairs['primary/text']=Math.min(...Array.from({length:19},(_,p)=>ratio('var(--on-accent)',`color-mix(in oklab, var(--text) ${82+p}%, var(--surface-3))`)));
    pairs['progress/track']=ratio('color-mix(in oklab, var(--text) 82%, var(--surface-3))','var(--surface-3)');
    pairs['primary/boundary']=Math.min(...['surface-1','surface-2'].map(surface=>ratio('color-mix(in oklab, var(--text) 82%, var(--surface-3))',`var(--${surface})`)));
    probe.remove();return pairs;
  });
  for(const [pair,value] of Object.entries(contrast)) expect(value,pair).toBeGreaterThanOrEqual(pair==='progress/track'||pair.endsWith('/boundary')?3:4.5);
  console.log(JSON.stringify({theme,browser:browser.version(),contrast}));
  await info.attach(`contrast-${theme}.json`, {body:JSON.stringify(contrast,null,2),contentType:'application/json'});
  const geometry=await page.locator('.composer').boundingBox();
  await page.evaluate(()=>document.documentElement.dataset.gradients='off');
  for(const selector of treatments) await expect(page.locator(selector)).toHaveCSS('background-image','none');
  expect(await page.locator('.composer').boundingBox()).toEqual(geometry);
  await page.evaluate(()=>delete document.documentElement.dataset.gradients);
  await page.emulateMedia({contrast:'more'});
  for(const selector of treatments) await expect(page.locator(selector)).toHaveCSS('background-image','none');
  await expect(page.locator('html')).toHaveCSS('--gradient-progress','none');
  await page.emulateMedia({contrast:'no-preference',forcedColors:'active'});
  for(const selector of treatments) await expect(page.locator(selector)).toHaveCSS('background-image','none');
  await expect(page.locator('html')).toHaveCSS('--gradient-progress','none');
  await page.emulateMedia({forcedColors:'none',reducedMotion:'reduce'});
  await expect(page.locator('.composer')).toHaveCSS('background-image',/linear-gradient/);
  await page.locator('.composer-actions > button:last-child').focus();
  await expect(page.locator('.composer-actions > button:last-child')).toBeFocused();
});

for(const theme of ['light','dark']) for(const locale of ['en','ar']) for(const width of [390,1440]) test(`gradient comparison ${theme} ${locale} ${width}`, async({page})=>{
  const state=await craftFixture(page,locale,theme);state.download=true;
  await page.setViewportSize({width,height:900});
  const dir=resolve('../../build/ui-craft/gradients',`${theme}-${locale}-${width}`);mkdirSync(dir,{recursive:true});
  for(const route of ['chat','models','agents','settings']){
    await page.goto(`/ui/#/${route}`);await expect(page.locator('.page-content')).toBeVisible();
    if(route==='chat')await page.locator('.composer textarea').fill('A draft stays unchanged.');
    if(route==='models')await expect(page.getByRole('progressbar').first()).toBeVisible();
    for(const mode of ['on','off']){
      await page.evaluate(mode=>document.documentElement.dataset.gradients=mode,mode);
      await page.screenshot({path:resolve(dir,`${route}-${mode}.png`),fullPage:true,animations:'disabled'});
      expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
      if(mode==='off'&&route==='models')await expect(page.locator('.download-state > div span').first()).toHaveCSS('background-image','none');
    }
    await page.evaluate(()=>delete document.documentElement.dataset.gradients);
  }
});

test('gradient paint comparison under 4x and 6x CPU throttling', async({browser})=>{
  test.skip(!process.env.OFFGRID_PAINT_MEASURE,'Explicit isolated performance run, one worker.');
  test.setTimeout(180_000);
  const results:unknown[]=[];
  for(const rate of [4,6]) for(const mode of ['off','on']) for(const workload of ['stream','download']){
    // Each sample gets fresh storage and request state. A stopped synthetic
    // conversation must not leak its recovery state into the next measurement.
    const context=await browser.newContext(), page=await context.newPage();
    const state=await craftFixture(page,'en','dark');
    const cdp=await context.newCDPSession(page);
    await cdp.send('Emulation.setCPUThrottlingRate',{rate});
    state.download=workload==='download';
    await page.goto(`/ui/#/${workload==='stream'?'chat':'models'}`);
    await page.evaluate(mode=>document.documentElement.dataset.gradients=mode,mode);
    if(workload==='stream'){
      await page.locator('.composer textarea').fill('Continue the briefing');await page.locator('.composer-actions > button:last-child').click();
      await expect(page.locator('.streaming-response')).toBeVisible();
    } else await expect(page.getByRole('progressbar').first()).toBeVisible();
    const events:{name:string;dur?:number}[]=[];
    const collect=({value}:{value:{name:string;dur?:number}[]})=>events.push(...value);
    cdp.on('Tracing.dataCollected',collect);
    await cdp.send('Tracing.start',{categories:'devtools.timeline',transferMode:'ReportEvents'});
    const frames=await page.evaluate(async(workload)=>{
      const frames:number[]=[];let last=performance.now(),active=true;
      const tick=(now:number)=>{frames.push(now-last);last=now;if(active)requestAnimationFrame(tick);};requestAnimationFrame(tick);
      for(let i=0;i<60;i++){
        if(workload==='stream')(window as unknown as {emitCraftDelta:(value:string)=>void}).emitCraftDelta('\n\nA long streamed paragraph remains readable. café 日本語. '.repeat(6));
        await new Promise(resolve=>setTimeout(resolve,55));
      }
      active=false;return frames.sort((a,b)=>a-b);
    },workload);
    const finished=new Promise<void>(resolve=>cdp.once('Tracing.tracingComplete',()=>resolve()));
    await cdp.send('Tracing.end');await finished;cdp.off('Tracing.dataCollected',collect);
    const paint=events.filter(e=>e.name==='Paint').reduce((sum,e)=>sum+(e.dur??0),0)/1000;
    let stopMs:number|undefined;
    if(workload==='stream'){
      stopMs=await page.evaluate(async()=>{const start=performance.now();(document.querySelector('.composer-actions > button:last-child') as HTMLButtonElement).click();await new Promise(requestAnimationFrame);return performance.now()-start;});
      expect.soft(stopMs, `${rate}x ${mode}: unqualified stress Stop feedback`).toBeLessThan(250);
    }
    results.push({rate,mode,workload,paintMs:paint,paintEvents:events.filter(e=>e.name==='Paint').length,frameP95:frames[Math.floor(frames.length*.95)],stopMs});
    mkdirSync(resolve('../../build/ui-craft'),{recursive:true});
    writeFileSync(resolve('../../build/ui-craft/gradient-paint.json'),JSON.stringify({browser:browser.version(),results},null,2));
    await context.close();
  }
  const report={browser:browser.version(),results};
  mkdirSync(resolve('../../build/ui-craft'),{recursive:true});writeFileSync(resolve('../../build/ui-craft/gradient-paint.json'),JSON.stringify(report,null,2));
  console.log(JSON.stringify(report));
});
