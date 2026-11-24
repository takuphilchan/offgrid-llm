import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {mkdirSync} from 'node:fs';
import {startupCopy} from '../src/lib/workspace-startup';
import {languageRegistry,type LanguageCode} from '../src/i18n/language';

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

for(const theme of ['light','dark']) for(const reduce of ['none','app','system']) test(`compact startup preserves the mark and honors motion preferences ${theme} ${reduce}`,async({page})=>{
  await boot(page,theme,reduce==='app');await page.emulateMedia({reducedMotion:reduce==='system'?'reduce':'no-preference'});
  await page.route('**/v1/models',()=>{});
  await page.goto('/ui/#/chat');const splash=page.locator('#workspace-startup');
  await expect(splash).toBeVisible();const mark=splash.locator('.startup-brand svg');
  await expect(mark).toHaveCSS('animation-name',reduce==='none'?'startup-breathe':'none');
  await expect(mark).toHaveCSS('width','38px');
  for(const path of await mark.locator('path').all())await expect(path).toHaveCSS('transform','none');
  await expect(splash.locator('.startup-welcome,.startup-elapsed')).toHaveCount(0);
  await expect(splash.locator('.startup-stage')).toHaveText('Opening your workspace');
  await expect(splash.locator('button')).toBeHidden();
  if(reduce==='none') {
    const opacity=await mark.evaluate(el=>getComputedStyle(el).opacity);
    await expect.poll(()=>mark.evaluate(el=>getComputedStyle(el).opacity)).not.toBe(opacity);
  }
  const bounds=await splash.locator('.startup-content').boundingBox();expect(bounds!.height).toBeLessThan(100);
  mkdirSync('../../build/ui-craft/startup',{recursive:true});
  await page.screenshot({path:`../../build/ui-craft/startup/compact-${theme}-${reduce}.png`});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth&&document.documentElement.scrollHeight<=innerHeight)).toBe(true);
});

test('access failure keeps the same entrance and Retry reaches the ready workspace',async({page})=>{
  await boot(page);let failing=true;
  await page.route('**/v1/users/me',async route=>failing?route.fulfill({status:503,json:{error:{message:'The workspace is temporarily unavailable.'}}}):route.fallback());
  await page.goto('/ui/#/home');const splash=page.locator('#workspace-startup');
  await expect(splash).toHaveAttribute('data-startup-state','recovery');
  await expect(splash.locator('.startup-brand svg')).toHaveCSS('width','38px');
  await expect(splash.locator('.startup-brand svg')).toHaveCSS('animation-name','none');
  await expect(splash.locator('.startup-message')).toHaveText('The workspace is temporarily unavailable.');
  await expect(page.locator('#root')).toHaveAttribute('inert','');
  failing=false;await splash.getByRole('button',{name:'Retry connection',exact:true}).click();
  await expect(splash).toHaveCount(0);await expect(page.locator('#root')).not.toHaveAttribute('inert');
});

test('a failed application bundle still has a local startup surface and a retry',async({page})=>{
  await boot(page);
  // Vite may retry an entry with a cache-busting query after a source update.
  // Keep the simulated bundle outage in force for those requests as well.
  await page.route(/\/src\/main\.tsx(?:\?.*)?$/,route=>route.abort());
  await page.route(/\/ui\/assets\/index-[^/]+\.js(?:\?.*)?$/,route=>route.abort());
  await page.clock.install();await page.goto('/ui/#/home');
  await expect(page.locator('#workspace-startup')).toBeVisible();
  await page.clock.fastForward(30001);
  await expect(page.getByRole('button',{name:'Retry connection',exact:true})).toBeVisible();
  await expect(page.locator('#root')).toHaveAttribute('inert','');
});

test('all 29 startup languages fit narrow and short screens without a moving layout',async({page})=>{
  await boot(page,'light',true);
  await page.route(/\/src\/main\.tsx(?:\?.*)?$/,route=>route.abort());await page.route(/\/ui\/assets\/index-[^/]+\.js(?:\?.*)?$/,route=>route.abort());
  await page.clock.install();await page.goto('/ui/#/chat');
  const splash=page.locator('#workspace-startup');
  for(const [locale,definition] of Object.entries(languageRegistry)) {
    const copy=startupCopy(locale as LanguageCode);
    await page.evaluate(({locale,copy,direction})=>{
      document.documentElement.dir=direction;
      window.dispatchEvent(new CustomEvent('offgrid-workspace-startup',{detail:{locale,copy}}));
    },{locale,copy,direction:definition.direction});
    for(const size of [{width:360,height:740},{width:480,height:220}]) {
      await page.setViewportSize(size);await expect(splash.locator('.startup-stage')).toHaveText(copy.opening);
      expect(await splash.evaluate(el=>el.scrollWidth<=el.clientWidth&&el.scrollHeight<=el.clientHeight)).toBe(true);
    }
  }
  await page.clock.fastForward(30001);
  await expect(splash.locator('.startup-stage')).toHaveText(startupCopy('fil').reconnecting);
  await expect(splash.getByRole('button')).toBeVisible();
  await page.emulateMedia({forcedColors:'active'});
  await page.keyboard.press('Tab');await expect(splash.getByRole('button')).toBeFocused();
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
