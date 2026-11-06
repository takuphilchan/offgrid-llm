import {expect,test} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

for(const width of [390,1440]) test(`MCP stays above a large inventory and Knowledge menu remains compact at ${width}`,async({page},info)=>{
 const state=await craftFixture(page,'en','light');await page.setViewportSize({width,height:900});
 await page.route('**/v1/agents/tools?all=true',r=>r.fulfill({json:{tools:Array.from({length:80},(_,i)=>({name:`Tool ${i}`,description:'A reviewed tool',enabled:true,source:'native'})),enabled_count:80}}));
 await page.goto('/ui/#/agents/tools');
 const connections=page.locator('#agent-connections-panel'),tools=page.locator('#agent-tools-panel');
 await expect(tools.locator('.tool-list article')).toHaveCount(80);
 expect((await connections.boundingBox())!.y).toBeLessThan((await tools.boundingBox())!.y);
 await connections.getByRole('button',{name:'Connect',exact:true}).click();
 await expect(connections.getByRole('form')).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
 await page.goto('/ui/#/knowledge');
 await page.getByRole('button',{name:'Manage Knowledge',exact:true}).click();
 const menu=page.locator('.knowledge-management-menu');await expect(menu).toBeVisible();
 expect((await menu.boundingBox())!.width).toBeLessThanOrEqual(260);
 await expect(menu).toContainText('local-embedding');
 await expect(menu.getByRole('button',{name:'Disable knowledge',exact:true})).toBeVisible();
 await page.screenshot({path:info.outputPath('knowledge-menu.png')});
 await page.keyboard.press('Escape');expect(state.writes).toEqual([]);
});

test('discard is explicit, preserves installed models and keeps failed removal retryable',async({page})=>{
 await craftFixture(page,'en','light');let removed=false,fail=true;const mutations:string[]=[];
 const item={file_name:'unfinished.gguf',model_id:'unfinished',status:'cancelled',percent:3.6,bytes_done:1024,bytes_total:28000,repository:'fixture/model',source_file:'model.gguf'};
 await page.route('**/v1/models/download/progress',r=>r.fulfill({json:removed?{}:{'unfinished.gguf':item}}));
 await page.route('**/v1/models/download/discard',r=>{
   mutations.push(r.request().postData()!);
   if(fail)return r.fulfill({status:500,json:{error:{message:'Storage is unavailable'}}});
   removed=true;return r.fulfill({json:{success:true}});
 });
 await page.goto('/ui/#/models');
 await page.locator('.model-transfers > summary').click();
 const discard=page.getByRole('button',{name:'Discard partial data',exact:true});
 await discard.click();let dialog=page.getByRole('dialog');
 await dialog.getByRole('button',{name:'Cancel',exact:true}).click();expect(mutations).toEqual([]);
 await discard.click();await dialog.getByRole('button',{name:'Discard partial data',exact:true}).click();
 await expect(dialog.getByRole('alert')).toBeVisible();await expect(page.locator('.installed-model')).toHaveCount(1);
 fail=false;await dialog.getByRole('button',{name:'Discard partial data',exact:true}).click();
 await expect(dialog).toHaveCount(0);await expect(discard).toHaveCount(0);
 await page.reload();await expect(discard).toHaveCount(0);await expect(page.locator('.installed-model')).toHaveCount(1);
 expect(mutations).toEqual([JSON.stringify({file_name:'unfinished.gguf'}),JSON.stringify({file_name:'unfinished.gguf'})]);
});


for(const width of [390,1440]) test('Activity cleanup belongs to its search toolbar at '+width,async({page},info)=>{
 const state=await craftFixture(page,'en','light');await page.setViewportSize({width,height:900});
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'craft',capabilities:['task-first-agents-v2','work-summary-v1']}}));
 await page.route('**/api/v2/work?*',r=>r.fulfill({json:{items:[{id:'run-'+'b'.repeat(32),title:'A completed task',status:'completed',created_at:'2026-11-06T00:00:00Z',deletable:true}],counts:{all:1,attention:0,running:0,history:1,archived:0},removable_count:1,has_more:false,scope:'actor_jobs'}}));
 await page.goto('/ui/#/activity/all');
 const toolbar=page.locator('.work-history .workspace-history-toolbar'),clear=toolbar.getByRole('button',{name:'Review removable history',exact:true});
 await expect(clear).toBeVisible();const search=await toolbar.locator('.history-search').boundingBox(),button=await clear.boundingBox();
 if(width>600){expect(Math.abs(search!.y+search!.height/2-button!.y-button!.height/2)).toBeLessThan(3);expect(button!.x).toBeGreaterThanOrEqual(search!.x+search!.width);}
 else {expect(button!.y-search!.y-search!.height).toBeLessThan(12);expect(button!.y).toBeGreaterThanOrEqual(search!.y+search!.height);}
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
 await page.screenshot({path:info.outputPath('activity-toolbar.png')});expect(state.writes).toEqual([]);
});
