import {test,expect,type Page} from '@playwright/test';
import {agentSchedulesText} from '../src/i18n/agent-schedules';
import type {LocaleCode} from '../src/i18n';

const profile={id:'reporter',workspace:'schedule-fixture',actor:'local-admin',revision:2,digest:'d'.repeat(64),settings:{name:'Reporter',model:'fixture-model',max_iterations:8},archived:false};
const schedule={id:'schedule-one',version:1,revision:1,digest:'a'.repeat(64),settings:{name:'Morning report',prompt:'Read the approved workspace.',calendar:{zone:'Africa/Harare',hour:9,minute:0,weekdays:[1,2,3,4,5]},enabled:true},next_at:'2026-10-27T07:00:00Z',created_at:'2026-10-26T00:00:00Z',profile:{id:profile.id,revision:profile.revision,digest:profile.digest,name:profile.settings.name},model:'fixture-model',max_iterations:8};
async function fixture(page:Page,locale:LocaleCode='en'){
  const writes:{path:string;body:any}[]=[];let items=[structuredClone(schedule)];
  await page.addInitScript(value=>{localStorage.setItem('offgrid.locale',value);localStorage.setItem('offgrid.onboarding.complete','true');},locale);
  await page.route('**/*',async route=>{
    const req=route.request(),path=new URL(req.url()).pathname;if(!path.startsWith('/api/')&&!path.startsWith('/v1/'))return route.continue();
    if(req.method()!=='GET')writes.push({path,body:req.postData()?req.postDataJSON():null});
    let data:unknown={};
    if(path==='/api/v2/system')data={product:'offgrid',api_version:2,version:'fixture',workspace_id:'schedule-fixture',capabilities:['task-first-agents-v2','durable-agent-events-v2','agent-profiles-v1','agent-schedules-v1']};
    else if(path==='/v1/system/config')data={require_auth:false};
    else if(path==='/v1/users/me')data={user:null,authenticated:false,guest:true};
    else if(path==='/v1/models')data={data:[{id:'fixture-model',type:'chat'}]};
    else if(path==='/v1/sessions')data={sessions:[]};
    else if(path==='/api/v2/jobs')data=[];
    else if(path==='/api/v2/computer/sessions')data={sessions:[]};
    else if(path==='/api/v2/agent-profiles')data={profiles:[profile]};
    else if(path==='/api/v2/agent-schedules'){
      if(req.method()==='POST'){const body=req.postDataJSON();const item={...schedule,id:'schedule-two',...body,profile:schedule.profile,revision:1};items=[...items,item];data=item;}else data={schedules:items};
    }else if(path==='/api/v2/agent-schedules/schedule-one/disable'){items=items.map(x=>x.id===schedule.id?{...x,revision:x.revision+1,settings:{...x.settings,enabled:false}}:x);data=items[0];}
    else if(path.endsWith('/occurrences'))data={occurrences:[{id:'occurrence-one',state:'missed',reason:'service_offline',due_at:schedule.next_at},{id:'occurrence-two',state:'completed',due_at:schedule.next_at,job_id:'run-'+'1'.repeat(32)}]};
    return route.fulfill({json:data});
  });
  await page.goto('/ui/#/activity');
  await expect(page.getByRole('region',{name:agentSchedulesText[locale].title,exact:true})).toBeVisible();
  return {writes,panel:page.getByRole('region',{name:agentSchedulesText[locale].title,exact:true})};
}

test('reviewed creation pins policy; management and history never submit or replay',async({page})=>{
  const {writes,panel}=await fixture(page),copy=agentSchedulesText.en;
  await panel.getByRole('button',{name:copy.history,exact:true}).click();await expect(panel.getByText(copy.missed,{exact:true})).toBeVisible();
  await expect(panel.getByRole('link',{name:copy.inspect})).toHaveAttribute('href','#/activity/task/run-'+'1'.repeat(32));
  await panel.getByRole('button',{name:copy.refresh}).click();expect(writes).toEqual([]);
  await panel.getByRole('button',{name:copy.create,exact:true}).click();const editor=panel.getByRole('form',{name:copy.edit});
  await editor.getByLabel(copy.name,{exact:true}).fill('Exact Unicode café 日本語');await editor.getByLabel(copy.prompt,{exact:true}).fill('Report findings only.');
  await editor.getByLabel(copy.profile,{exact:true}).selectOption('reporter:2');
  await editor.getByLabel(copy.zone,{exact:true}).fill('Africa/Harare');await editor.getByRole('checkbox',{name:copy.enabled,exact:true}).check();
  const save=editor.getByRole('button',{name:copy.save,exact:true});await expect(save).toBeDisabled();
  await editor.getByRole('checkbox',{name:copy.review,exact:true}).check();await expect(save).toBeEnabled();
  await editor.getByLabel(copy.time,{exact:true}).fill('10:30');await expect(save).toBeDisabled();
  await editor.getByRole('checkbox',{name:copy.review,exact:true}).check();await save.click();await expect(editor).not.toBeVisible();
  expect(writes).toHaveLength(1);expect(writes[0].body).toMatchObject({profile_id:'reporter',profile_revision:2,max_iterations:8,settings:{calendar:{zone:'Africa/Harare',hour:10,minute:30},enabled:true}});
  await panel.getByRole('button',{name:copy.disable,exact:true}).first().click();await expect.poll(()=>writes.length).toBe(2);
  expect(writes[1]).toEqual({path:'/api/v2/agent-schedules/schedule-one/disable',body:{expected_revision:1}});
  expect(writes.some(x=>x.path==='/api/v2/jobs'||x.path.includes('/resume')||x.path.includes('/input'))).toBe(false);
});

for(const locale of Object.keys(agentSchedulesText) as LocaleCode[])test(`schedule controls ${locale} remain readable at 320×480`,async({page})=>{
  await page.setViewportSize({width:320,height:480});const {panel,writes}=await fixture(page,locale),copy=agentSchedulesText[locale];
  expect(Object.values(copy).every(value=>typeof value==='string'&&value.length>0)).toBe(true);
  await panel.getByRole('button',{name:copy.create,exact:true}).click();const editor=panel.getByRole('form',{name:copy.edit});
  await expect(editor.getByLabel(copy.profile,{exact:true})).toBeVisible();await expect(editor.getByRole('checkbox',{name:copy.enabled,exact:true})).not.toBeChecked();
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await editor.getByRole('button',{name:copy.cancel,exact:true}).click();await expect(editor).not.toBeVisible();await expect(panel.getByRole('button',{name:copy.create,exact:true})).toBeFocused();expect(writes).toEqual([]);
});

for(const theme of ['light','dark'] as const)test(`schedule review visual matrix ${theme}`,async({page},info)=>{
  await page.addInitScript(value=>localStorage.setItem('offgrid.theme',value),theme);
  await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
  const {panel,writes}=await fixture(page),copy=agentSchedulesText.en;
  const start=panel.getByRole('button',{name:copy.create,exact:true});await start.focus();await start.press('Enter');
  const editor=panel.getByRole('form',{name:copy.edit});await expect(editor.getByLabel(copy.name,{exact:true})).toBeFocused();
  await expect(editor.getByLabel(copy.time,{exact:true})).toHaveCSS('min-height','40px');
  await expect(editor.getByLabel(copy.time,{exact:true})).toHaveCSS('border-radius','8px');
  await editor.getByLabel(copy.name,{exact:true}).fill('Reviewed report — café 日本語');await editor.getByLabel(copy.profile,{exact:true}).selectOption('reporter:2');
  for(const size of [{width:1440,height:900},{width:1280,height:720},{width:390,height:844},{width:320,height:480}]){
    await page.setViewportSize(size);await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await page.screenshot({path:info.outputPath(`schedule-${theme}-${size.width}.png`),fullPage:true});
  }
  await page.setViewportSize({width:1280,height:720});await page.evaluate(()=>document.documentElement.style.zoom='2');
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await editor.getByRole('button',{name:copy.cancel,exact:true}).click();await expect(start).toBeFocused();expect(writes).toEqual([]);
});
