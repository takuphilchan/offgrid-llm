import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve,sep,extname} from 'node:path';
import {totalmem,cpus} from 'node:os';
import {craftFixture} from './helpers/craft-fixture';

// Production bundles, identical isolated data, one worker, no CPU throttling.
// This measures this machine; emulation is not an 8 GB hardware qualification.
test('work redesign production navigation and memory against retained baseline',async({browser},info)=>{
 test.skip(!process.env.OFFGRID_REDESIGN_PERF,'Opt-in isolated production comparison');
 test.setTimeout(180000);
 const baseline=process.env.OFFGRID_BASELINE_UI;
 expect(baseline,'Provide the retained pre-redesign UI directory').toBeTruthy();
 const records:Record<string,{navigation:number[];p95:number;heap:number;requests:number;acknowledgment:number}>={};
 for(const [name,directory] of [['baseline',baseline!],['candidate',resolve('dist')]]){
  const root=resolve(directory),context=await browser.newContext({viewport:{width:1440,height:900}}),page=await context.newPage();
  try{
   const fixture=await craftFixture(page,'en','dark');let requests=0;
   page.on('request',request=>{if(/\/(?:v1|api\/v2)\//.test(request.url()))requests++;});
   await page.route('**/ui/**',async route=>{
    const pathname=decodeURIComponent(new URL(route.request().url()).pathname.slice(4));
    const file=resolve(root,pathname||'index.html');
    if(!file.startsWith(root+sep))return route.abort();
    const types:Record<string,string>={'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.woff2':'font/woff2','.svg':'image/svg+xml'};
    try{return await route.fulfill({body:await readFile(file),contentType:types[extname(file)]??'application/octet-stream'});}catch(error){console.error('Production fixture asset:',file,error);return route.abort();}
   });
   await page.goto('/ui/#/chat/new');await expect(page.locator('.composer textarea')).toBeEnabled();
   await page.locator('.composer textarea').fill('Private benchmark draft');
   const navigation:number[]=[];
   for(let i=0;i<42;i++){
    const settings=i%2===0;
    const elapsed=await page.evaluate(async({hash,selector})=>{
     const start=performance.now();location.hash=hash;
     await new Promise<void>((resolve,reject)=>{const poll=()=>{
      if(performance.now()-start>10000){reject(Error('Navigation did not render'));return;}
      const node=document.querySelector<HTMLElement>(selector);
      if(node&&node.getClientRects().length&&!document.querySelector('.deferred-page'))requestAnimationFrame(()=>resolve());else requestAnimationFrame(poll);
     };requestAnimationFrame(poll);});return performance.now()-start;
    },{hash:settings?'#/settings':'#/chat',selector:settings?'.settings-page':'.composer textarea'});
    if(i>1)navigation.push(elapsed);
   }
   await expect(page.locator('.composer textarea')).toHaveValue('Private benchmark draft');
   expect(fixture.writes).toEqual([]);
   const cdp=await context.newCDPSession(page);await cdp.send('Performance.enable');await cdp.send('HeapProfiler.collectGarbage');
   const metrics=await cdp.send('Performance.getMetrics'),heap=metrics.metrics.find(item=>item.name==='JSHeapUsedSize')!.value;
   const acknowledgment=await page.evaluate(async()=>{
    const start=performance.now();document.querySelector<HTMLButtonElement>('button[aria-label="Send"]')!.click();
    await new Promise<void>((resolve,reject)=>{const poll=()=>{if(performance.now()-start>10000){reject(Error('No immediate feedback'));return;}if(document.querySelector('button[aria-label="Stop"]'))requestAnimationFrame(()=>resolve());else requestAnimationFrame(poll);};requestAnimationFrame(poll);});return performance.now()-start;
   });
   await page.getByRole('button',{name:'Stop',exact:true}).last().click();
   // Stop acknowledges cancellation before the final saved-state read settles.
   // Include that read for both builds instead of sampling a timing-dependent count.
   await expect(page.getByRole('button',{name:'Send',exact:true})).toBeEnabled();
   await expect(page.locator('.page-failure')).toHaveCount(0);
   const sorted=[...navigation].sort((a,b)=>a-b);
   records[name]={navigation,p95:sorted[Math.ceil(sorted.length*.95)-1],heap,requests,acknowledgment};
  }finally{await context.close();}
 }
 await info.attach('production-comparison.json',{body:JSON.stringify({cpu:cpus()[0]?.model,memoryBytes:totalmem(),records},null,2),contentType:'application/json'});
 console.log(JSON.stringify({measurement:'production-work-redesign',records}));
 expect(records.candidate.acknowledgment).toBeLessThan(100);
 expect(records.candidate.p95).toBeLessThanOrEqual(records.baseline.p95*1.1);
 expect(records.candidate.heap).toBeLessThanOrEqual(records.baseline.heap*1.1);
 expect(records.candidate.requests).toBeLessThanOrEqual(records.baseline.requests);
});
