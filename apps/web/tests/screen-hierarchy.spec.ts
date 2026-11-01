import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { accessGuidance } from '../src/i18n/access-guidance';
import { profileSelectionPrompt } from '../src/i18n/agent-profiles';

test('profile selection guidance and empty search preserve the selected draft',async({page})=>{
 const state=await craftFixture(page,'en','light');
 await page.route('**/api/v2/agent-profiles/researcher/effective?*',r=>r.fulfill({json:{issues:[]}}));
 await page.goto('/ui/#/agents');
 await expect(page.locator('.profile-empty')).toHaveText(profileSelectionPrompt.en);
 await page.locator('.profile-list-item').click();const name=page.getByRole('textbox',{name:'Name',exact:true});
 await name.fill('Unsaved profile name');
 const search=page.getByRole('searchbox',{name:'Search agents'});await search.fill('absent profile');
 await page.locator('.profile-library-list .workspace-empty').getByRole('button',{name:'Clear search'}).click();
 await expect(search).toBeFocused();await expect(search).toHaveValue('');await expect(name).toHaveValue('Unsaved profile name');
 expect(state.writes).toEqual([]);
});

for(const width of [390,1440]) test(`Activity sections share a single content alignment at ${width}`,async({page})=>{
 await craftFixture(page,'en','light');await page.setViewportSize({width,height:900});await page.goto('/ui/#/activity');
 await expect(page.locator('.task-list-item')).toBeVisible();
 const boxes=await page.locator('.activity-page .task-workspace, .activity-page > .activity-diagnostics').evaluateAll(elements=>elements.map(el=>{const r=el.getBoundingClientRect();return {x:r.x,width:r.width};}));
 expect(boxes).toHaveLength(2);for(const box of boxes){expect(Math.abs(box.x-boxes[0].x)).toBeLessThanOrEqual(1);expect(Math.abs(box.width-boxes[0].width)).toBeLessThanOrEqual(1);}
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu']) for(const width of [320,1280]) test(`settings rows keep labels and bounded controls aligned: ${locale} ${width}`,async({page},info)=>{
 const state=await craftFixture(page,locale,width===320?'light':'dark');await page.setViewportSize({width,height:700});
 await page.goto('/ui/#/settings');await expect(page.locator('#settings-language')).toBeVisible();
 await expect(page.getByRole('heading',{level:1})).toHaveCount(1);
 const fields=await page.locator('.settings-group .settings-panel').evaluateAll(rows=>rows.filter(row=>row.children.length>1).map(row=>{
  const label=row.firstElementChild!.getBoundingClientRect(),control=row.lastElementChild!.getBoundingClientRect();
  return {label:{x:label.x,y:label.y,bottom:label.bottom,right:label.right},control:{x:control.x,y:control.y,width:control.width,right:control.right},columns:getComputedStyle(row).gridTemplateColumns};
 }));
 expect(fields.length).toBeGreaterThanOrEqual(4);
 if(width===1280){
  for(const field of fields){expect(field.control.width).toBeLessThanOrEqual(321);const edge=locale==='ar'?'right':'x';expect(Math.abs(field.control[edge]-fields[0].control[edge])).toBeLessThanOrEqual(1);}
 }else for(const field of fields)expect(field.control.y).toBeGreaterThanOrEqual(field.label.bottom);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
 await page.locator('#settings-language').focus();await expect(page.locator('#settings-language')).toBeFocused();
 expect(state.writes).toEqual([]);
 if(locale==='en'||locale==='ar')await page.screenshot({path:info.outputPath(`settings-rows-${locale}-${width}.png`),fullPage:true,animations:'disabled'});
});

test('installed inventory is not advertised as ready and clearing a filter preserves the inventory and focus',async({page})=>{
 const state=await craftFixture(page,'en','light');await page.goto('/ui/#/models');
 await expect(page.getByRole('heading',{name:'Installed · 1',exact:true})).toBeVisible();
 await expect(page.getByRole('heading',{name:'1 available',exact:true})).toHaveCount(0);
 const search=page.getByRole('searchbox',{name:'Filter installed models'});await search.fill('no such model');
 await expect(page.getByRole('heading',{name:'Installed · 1',exact:true})).toBeVisible();
 await page.locator('.workspace-empty').getByRole('button',{name:'Clear search'}).click();
 await expect(search).toHaveValue('');await expect(search).toBeFocused();
 await expect(page.locator('.installed-model')).toHaveCount(1);expect(state.writes).toEqual([]);
});

for(const category of ['Language','Embeddings','Speech recognition','Speech generation']) test(`empty ${category} inventory offers discovery without downloading`,async({page})=>{
 const state=await craftFixture(page,'en','light');await page.route('**/v1/models',r=>r.fulfill({json:{data:[]}}));
 await page.goto('/ui/#/models');await page.getByRole('button',{name:category,exact:true}).click();
 const empty=page.getByRole('tabpanel').locator('.workspace-empty').first();
 await empty.getByRole('button',{name:'Discover models',exact:true}).click();
 await expect(page.getByRole('tab',{name:'Discover models',exact:true})).toHaveAttribute('aria-selected','true');
 await expect(page.getByRole('tab',{name:'Discover models',exact:true})).toBeFocused();
 expect(state.writes).toEqual([]);
});

test('empty search in Knowledge clears only its filter and keeps document identity',async({page})=>{
 const state=await craftFixture(page,'en','light');await page.goto('/ui/#/knowledge');
 const name=await page.locator('.knowledge-documents h3').textContent(),search=page.getByRole('searchbox',{name:'Search documents'});
 await search.fill('missing document');await page.locator('.workspace-empty').getByRole('button',{name:'Clear search'}).click();
 await expect(search).toHaveValue('');await expect(search).toBeFocused();await expect(page.locator('.knowledge-documents h3')).toHaveText(name!);expect(state.writes).toEqual([]);
});

test('empty tool inventory links to connection setup without granting or running tools',async({page})=>{
 const state=await craftFixture(page,'en','light');await page.goto('/ui/#/agents/tools');
 await page.locator('.workspace-empty').getByRole('link',{name:'MCP connections',exact:true}).click();
 await expect(page.getByRole('form',{name:'Connect',exact:true})).toBeVisible();expect(state.writes).toEqual([]);
});

test('empty legacy Activity returns to Chat without submitting or discarding the draft',async({page})=>{
 const state=await craftFixture(page,'en','light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{workspace_id:'craft',api_version:2,capabilities:[]}}));
 await page.goto('/ui/#/chat');await page.locator('.composer textarea').fill('Keep this draft');
 await page.getByRole('link',{name:'Activity',exact:true}).click();
 await page.locator('.workspace-empty').getByRole('link',{name:'Chat',exact:true}).click();
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this draft');expect(state.writes).toEqual([]);
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as const) test(`permission recovery preserves the draft and issues no management calls: ${locale}`,async({page})=>{
 const state=await craftFixture(page,locale,'light'),blocked:string[]=[];
 await page.route('**/v1/users/me',r=>r.fulfill({json:{authenticated:true,auth_required:true,user:{id:'viewer',username:'viewer',role:'viewer'}}}));
 await page.route(/\/v1\/(?:agents\/tools|agents\/mcp|integrations)/,r=>{blocked.push(r.request().url());return r.fulfill({status:403,json:{error:'Forbidden'}});});
 await page.goto('/ui/#/chat');await page.locator('.composer textarea').fill('Keep my private draft');
 await page.evaluate(()=>{location.hash='/agents/tools';});
 const notice=page.locator('.permission-notice');await expect(notice).toContainText(accessGuidance[locale].body);
 await notice.getByRole('link').click();await expect(page.locator('.composer textarea')).toHaveValue('Keep my private draft');
 expect(blocked).toEqual([]);expect(state.writes).toEqual([]);
});
