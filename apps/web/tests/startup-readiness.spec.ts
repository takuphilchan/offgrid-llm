import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {mkdirSync} from 'node:fs';
import {createRequire} from 'node:module';
const {messages:greetings}=createRequire(import.meta.url)('../../desktop/startup-welcome.js') as {messages:Record<string,string[]>};

function latch(){let release!:()=>void;const promise=new Promise<void>(resolve=>{release=resolve;});return {promise,release};}
async function boot(page:Page,theme='light',reduced=false){
  await craftFixture(page,'en',theme);
  await page.addInitScript(({theme,reduced})=>{
    Object.assign(window,{electron:{getSystemTheme:async()=>theme,onThemeChange:()=>()=>{},presentation:{locale:'en',effectiveTheme:theme,direction:'ltr',appearance:{reduceMotion:reduced},copy:{opening:'Opening your workspace',intro:'Your conversations, knowledge, and agents. On your computer.',retry:'Retry connection'}}}});
    localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'offgrid',dark:'offgrid',textSize:100,reduceMotion:reduced}));
  },{theme,reduced});
}

test('startup stays continuous until models, history and the selected turn have settled',async({page})=>{
  await boot(page);
  // Match the Windows preload's reserved caption area before initial reads finish.
  await page.addInitScript(()=>addEventListener('DOMContentLoaded',()=>{
    document.documentElement.style.setProperty('--native-chrome-height','32px');
    document.documentElement.style.setProperty('--workspace-viewport','calc(100dvh - 32px)');
    Object.assign(document.body.style,{paddingTop:'32px',boxSizing:'border-box'});
  }));
  const models=latch(),history=latch(),turn=latch();let historyRequested=false,turnRequested=false;
  await page.route('**/v1/models',async route=>{await models.promise;await route.fallback();});
  await page.route('**/v1/sessions',async route=>{historyRequested=true;await history.promise;await route.fallback();});
  await page.route('**/v1/sessions/*/turn',async route=>{turnRequested=true;await turn.promise;await route.fulfill({json:{turn:null}});});
  await page.goto('/ui/#/chat');
  const splash=page.locator('#workspace-startup');
  await expect(splash).toBeVisible();
  await expect(page.locator('#root')).toHaveAttribute('inert','');
  await expect(page.locator('.app-shell')).toHaveCount(0);
  expect(await page.evaluate(()=>document.documentElement.scrollHeight<=innerHeight)).toBe(true);
  await expect(splash).toHaveCSS('top','32px');
  models.release();await expect.poll(()=>historyRequested).toBe(true);
  await expect(splash).toBeVisible();await expect(page.locator('.composer textarea')).toBeHidden();
  await expect(splash.locator('.startup-stage')).toHaveText('Opening your workspace');
  await expect(splash.locator('.startup-stage')).not.toContainText('Chat');
  history.release();await expect.poll(()=>turnRequested).toBe(true);
  await expect(splash).toBeVisible();await expect(page.locator('.composer textarea')).toBeDisabled();
  turn.release();await expect(splash).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toBeVisible();await expect(page.locator('.composer textarea')).toBeEnabled();
  await expect(page.getByText('Prepare a concise briefing with an example.',{exact:true})).toBeVisible();
  await page.evaluate(()=>window.dispatchEvent(new Event('focus')));
  await expect(splash).toHaveCount(0);
  await expect(page.locator('#root')).not.toHaveAttribute('inert');
  expect(await page.evaluate(()=>document.documentElement.scrollHeight<=innerHeight)).toBe(true);
  expect(await page.evaluate(()=>getComputedStyle(document.documentElement).overflow)).not.toBe('hidden');
});

test('history failure reveals an actionable error rather than a stuck splash',async({page})=>{
  await boot(page);
  await page.route('**/v1/sessions',route=>route.fulfill({status:503,json:{error:{message:'History is temporarily unavailable'}}}));
  await page.goto('/ui/#/chat');
  await expect(page.locator('#workspace-startup')).toHaveCount(0);
  await expect(page.getByText('History is temporarily unavailable',{exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'Refresh history',exact:true})).toBeVisible();
});

test('capability discovery finishes before Chat is revealed',async({page})=>{
  await boot(page);let reads=0;const capabilities=latch();
  await page.route('**/api/v2/system',async route=>{if(++reads>1)await capabilities.promise;await route.fallback();});
  await page.goto('/ui/#/home');await expect.poll(()=>reads).toBeGreaterThan(1);
  await expect(page.locator('#workspace-startup')).toBeVisible();
  capabilities.release();await expect(page.locator('#workspace-startup')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toBeEnabled();
});

for(const theme of ['light','dark']) for(const reduce of ['none','app','system']) test(`startup motion, live elapsed time and recovery ${theme} ${reduce}`,async({page})=>{
  await boot(page,theme,reduce==='app');await page.emulateMedia({reducedMotion:reduce==='system'?'reduce':'no-preference'});
  await page.route('**/v1/models',()=>{});
  await page.goto('/ui/#/chat');const splash=page.locator('#workspace-startup');
  await expect(splash).toBeVisible();
  await expect(splash.locator('.startup-brand svg path').first()).toHaveCSS('animation-name',reduce==='none'?'startup-layer-top':'none');
  await expect(splash.locator('.startup-spinner')).toHaveCount(0);
  await expect(splash.locator('.startup-welcome')).toHaveText('Welcome to OffGrid.');
  if(reduce==='none') {
    const mark=splash.locator('.startup-brand svg path').first();
    const first=await mark.evaluate(el=>getComputedStyle(el).transform);
    await expect.poll(()=>mark.evaluate(el=>getComputedStyle(el).transform)).not.toBe(first);
  }
  const time=await splash.locator('.startup-elapsed').textContent();
  await expect(splash.locator('.startup-elapsed')).not.toHaveText(time!);
  await expect(splash.locator('.startup-welcome')).toHaveText('Bring your ideas to life.',{timeout:7000});
  if(reduce==='none') await expect(splash.locator('.startup-welcome')).toHaveCSS('transition-duration','0.2s, 0.2s');
  else expect(await splash.locator('.startup-welcome').evaluate(el=>parseFloat(getComputedStyle(el).transitionDuration))).toBeLessThanOrEqual(.00001);
  mkdirSync('../../build/ui-craft/startup',{recursive:true});
  await page.screenshot({path:`../../build/ui-craft/startup/${theme}-${reduce}.png`});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('a failed application bundle still has a local startup surface and a retry',async({page})=>{
  await boot(page);
  await page.route('**/src/main.tsx',route=>route.abort());
  await page.clock.install();await page.goto('/ui/#/home');
  await expect(page.locator('#workspace-startup')).toBeVisible();
  await page.clock.fastForward(30001);
  await expect(page.getByRole('button',{name:'Retry connection',exact:true})).toBeVisible();
  await expect(page.locator('#root')).toHaveAttribute('inert','');
});

test('all 29 startup languages rotate without overflow on narrow screens',async({page})=>{
  await boot(page,'light',true);await page.setViewportSize({width:360,height:740});
  // Isolate the local startup surface from network deadlines while advancing
  // several minutes of greeting rotation. Recovery after API timeouts is tested above.
  await page.route('**/src/main.tsx',route=>route.abort());
  await page.clock.install();await page.goto('/ui/#/chat');
  const welcome=page.locator('.startup-welcome');await expect(welcome).toBeVisible();
  for(const [locale,copy] of Object.entries(greetings)) {
    await page.evaluate(locale=>{
      document.documentElement.lang=locale;document.documentElement.dir=['ar','ur','fa','he'].includes(locale)?'rtl':'ltr';
      window.dispatchEvent(new CustomEvent('offgrid-workspace-startup',{detail:{locale}}));
    },locale);
    await expect(welcome).toHaveText(copy[0]);
    for(let i=1;i<3;i++) {
      await page.clock.fastForward(5200);await expect(welcome).toHaveText(copy[i]);
      expect(await welcome.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
    }
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  }
});

test('pointer scrolling never outlines a message; transcript keyboard navigation stays visible',async({page})=>{
  await boot(page,'dark');
  const session={name:'Scroll test',model_id:'Local-Research-7B-Instruct-Q4_K_M-long-model-name',messages:Array.from({length:30},(_,i)=>({role:i%2?'assistant':'user',content:`Message ${i}. `+'Read this passage. '.repeat(30)}))};
  await page.route('**/v1/sessions',route=>route.fulfill({json:{sessions:[session]}}));
  await page.route('**/v1/sessions/*/turn',route=>route.fulfill({json:{turn:null}}));
  await page.goto('/ui/#/chat');const pane=page.locator('.conversation');
  await expect(page.locator('#workspace-startup')).toHaveCount(0);
  await page.keyboard.press('Tab');await pane.focus();
  await expect(pane).toHaveCSS('outline-style','solid');
  await page.keyboard.press('Home');
  await expect(page.locator('.conversation-content').locator(':scope > *').first()).toHaveCSS('outline-style','none');
  await pane.hover();await page.mouse.wheel(0,200);
  await expect(pane).toHaveCSS('outline-style','none');
  await page.locator('.chat-latest').click();
  await expect(pane).toHaveCSS('outline-style','none');
  await pane.hover();await page.mouse.wheel(0,-1600);
  await expect(page.locator('.chat-latest')).toBeVisible();
  await page.screenshot({path:'../../build/ui-craft/startup/scroll-no-outline.png'});
});
