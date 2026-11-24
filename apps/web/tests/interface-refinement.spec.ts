import {test,expect} from '@playwright/test';
import {mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
import {craftFixture} from './helpers/craft-fixture';

const screenshots=resolve('../../build/ui-refinement-captures');
mkdirSync(screenshots,{recursive:true});
for (const theme of ['light','dark']) for(const locale of ['en','ar']) {
 test(`shared management and draft continuity ${theme}/${locale}`,async({page})=>{
  const state=await craftFixture(page,locale,theme);
  await page.setViewportSize({width:1440,height:900});
  await page.goto('/ui/#/chat/new');
  const input=page.locator('.composer textarea');
  await expect(input).toBeEnabled();
  await input.fill('A draft that must survive navigation.');
  await expect(page.locator('.work-home-recent')).toBeVisible();
  await expect(page.locator('.work-home-recent button')).toHaveCount(1);
  await expect(page.locator('.command-trigger > span')).toBeVisible();
  await page.screenshot({path:resolve(screenshots,`${locale}-${theme}-home.png`),animations:'disabled'});
  for(const route of ['models','knowledge','agents/profiles','activity','settings/appearance']) {
   await page.evaluate(hash=>{location.hash=hash;},'#/'+route);
   const selectors:Record<string,string>={models:'.models-workspace',knowledge:'.knowledge-workspace','agents/profiles':'.profile-library-layout',activity:'.activity-page','settings/appearance':'.appearance-settings'};
   await expect(page.locator(selectors[route])).toBeVisible();
   await expect(page.locator('.deferred-page')).toHaveCount(0);
   const content=page.locator('.page-content');
   await expect(content).not.toBeEmpty();
   await expect(page.locator('.workspace')).toBeVisible();
   await page.screenshot({path:resolve(screenshots,`${locale}-${theme}-${route.replaceAll('/','-')}.png`),animations:'disabled'});
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
   const nav=page.locator('.workspace-view-nav').first();
   if(await nav.count()&&route==='activity'){
    await expect(nav).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
    await expect(nav.locator('[aria-pressed=true]')).toHaveCSS('border-bottom-width','2px');
   }else if(await nav.count()){
    const geometry=await nav.evaluate(el=>({width:el.getBoundingClientRect().width,parent:el.parentElement!.getBoundingClientRect().width}));
    expect(geometry.width).toBeLessThan(geometry.parent);
    expect(await nav.locator('a,button').first().evaluate(el=>parseFloat(getComputedStyle(el).paddingInlineStart))).toBeGreaterThanOrEqual(12);
   }
  }
  await page.evaluate(()=>{location.hash='#/chat';});
  await expect(input).toHaveValue('A draft that must survive navigation.');
  await page.locator('.work-home-recent button').first().click();
  await expect(page.locator('.message.assistant')).toBeVisible();
  await page.evaluate(()=>{location.hash='#/chat/new';});
  await expect(input).toHaveValue('A draft that must survive navigation.');
  expect(state.writes).toEqual([]);
 });
 test(`drawer focus and narrow content ${theme}/${locale}`,async({page})=>{
  const state=await craftFixture(page,locale,theme);
  await page.setViewportSize({width:390,height:844});
  await page.goto('/ui/#/chat/new');
  await expect(page.locator('.composer textarea')).toBeEnabled();
  await page.locator('.navigation-trigger').click();
  const link=page.locator('.primary-nav a').first();
  await page.keyboard.press('Tab');
  await link.focus();
  await expect(link.locator('span')).toBeVisible();
  expect(await link.locator('span').evaluate(el=>getComputedStyle(el).position)).toBe('static');
  await page.screenshot({path:resolve(screenshots,`${locale}-${theme}-drawer.png`),animations:'disabled'});
  await page.keyboard.press('Escape');
  await expect(page.locator('.navigation-trigger')).toBeFocused();
  await page.evaluate(()=>{location.hash='#/models';});
  await expect(page.locator('.workspace-view-nav')).toBeVisible();
  await page.screenshot({path:resolve(screenshots,`${locale}-${theme}-narrow-models.png`),animations:'disabled'});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
  expect(state.writes).toEqual([]);
 });
}
test('recent work is bounded, keyboard operable, and empty history has no dead controls',async({page})=>{
 const state=await craftFixture(page,'en','dark');
 const sessions=Array.from({length:15},(_,i)=>({name:`Saved conversation ${i}`,updated_at:'2026-11-21T00:00:00Z',messages:[]}));
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions}}));
 await page.goto('/ui/#/chat/new');
 await expect(page.locator('.work-home-recent button')).toHaveCount(3);
 await expect(page.locator('#workspace-startup')).toHaveCount(0);
 await page.locator('.work-home-recent button').first().focus();
 await expect(page.locator('.work-home-recent button').first()).toBeFocused();
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[]}}));
 await page.reload();
 await expect(page.locator('.composer textarea')).toBeEnabled();
 await expect(page.locator('.work-home-recent')).toHaveCount(0);
 expect(state.writes).toEqual([]);
});

test('empty and failed library states retain explicit recovery and keyboard access',async({page})=>{
 const state=await craftFixture(page,'en','dark');state.mode='empty';
 await page.goto('/ui/#/knowledge');
 await expect(page.locator('.workspace-empty,.empty-state').first()).toBeVisible();
 await page.screenshot({path:resolve(screenshots,'knowledge-empty.png'),animations:'disabled'});
 state.mode='error';await page.reload();
 await expect(page.getByRole('alert').first()).toContainText('temporarily unavailable');
 await page.screenshot({path:resolve(screenshots,'knowledge-error.png'),animations:'disabled'});
 expect(state.writes).toEqual([]);
});
test('management selection remains distinguishable with forced colors and enlarged text',async({page})=>{
 const state=await craftFixture(page,'ar','dark');
 await page.emulateMedia({forcedColors:'active',reducedMotion:'reduce'});
 await page.setViewportSize({width:720,height:900});
 await page.goto('/ui/#/models');
 const nav=page.locator('.workspace-view-nav');await expect(nav).toBeVisible();
 await page.evaluate(()=>{document.documentElement.style.fontSize='200%';});
 const selected=nav.locator('[aria-current=page]');await selected.focus();
 await expect(selected).toBeFocused();
 expect(await selected.evaluate(el=>getComputedStyle(el).outlineStyle)).not.toBe('none');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
 await page.screenshot({path:resolve(screenshots,'ar-forced-colors-enlarged.png'),animations:'disabled'});
 expect(state.writes).toEqual([]);
});

test('saved answers keep verification visible and profile provenance available in Details',async({page})=>{
 const state=await craftFixture(page,'en','dark');
 const id='run-'+'d'.repeat(32),model='Local-Research-7B-Instruct-Q4_K_M-long-model-name';
 const run={id,run_id:id,prompt:'Prepare a briefing',model,status:'completed',created_at:'2026-11-21T00:00:00Z',
  output:'A saved briefing ready for review.',profile:{id:'researcher',name:'Researcher',revision:3},steps:[],
  verification:{version:2,revision:1,instruction_id:'initial',instruction_sha256:'a'.repeat(64),state:'unverified',checks:[]}};
 const turn={id:'request-refinement',prompt:run.prompt,model,status:run.status,output:run.output,job_id:id,job_admission:'linked',agent:{id:'researcher',revision:3}};
 const session={name:'Saved briefing',model_id:model,updated_at:run.created_at,jobs:[{turn_id:turn.id,job_id:id}],messages:[
  {role:'user',content:run.prompt,turn_id:turn.id,job_id:id},{role:'assistant',content:run.output,turn_id:turn.id,job_id:id}]};
 await page.route('**/api/v2/jobs',r=>r.fulfill({json:[run]}));
 await page.route(`**/api/v2/jobs/${id}`,r=>r.fulfill({json:run}));
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[session]}}));
 await page.route('**/v1/sessions/Saved%20briefing',r=>r.fulfill({json:session}));
 await page.route('**/v1/sessions/*/turn',r=>r.fulfill({json:{turn}}));
 await page.goto('/ui/#/chat/new');
 await page.locator('.work-home-recent button').first().click();
 const work=page.locator('.conversation-work');
 await expect(page.locator('.message.assistant')).toContainText(run.output);
 await expect(work.locator('.work-status')).toHaveText('Not independently verified');
 await expect(work.getByRole('link',{name:'Inspect work',exact:true})).toBeVisible();
 const profile=work.locator('.conversation-work-profile');await expect(profile).toBeHidden();
 await page.screenshot({path:resolve(screenshots,'saved-unverified-work.png'),animations:'disabled'});
 await work.locator('.conversation-work-details > summary').click();
 await expect(profile).toContainText('Researcher');await expect(profile).toContainText('3');
 expect(state.writes).toEqual([]);
});
