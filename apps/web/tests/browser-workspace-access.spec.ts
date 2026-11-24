import {expect,test,type Page} from '@playwright/test';
import {additionalBrowserOrigins} from '../src/features/agents/browser-scope';
import {computerExperience} from '../src/i18n/computer-experience';
import type {LocaleCode} from '../src/i18n';

const id='run-'+'e'.repeat(32);
async function fixture(page:Page,locale:LocaleCode='en',legacy=false){
 let attached=0;
 await page.addInitScript(({locale,legacy})=>{
  localStorage.setItem('offgrid.locale',locale);localStorage.setItem('offgrid.onboarding.complete','true');
  Object.assign(window,{electron:{isDesktop:true,platform:'win32',getApiUrl:async()=>location.origin,getVersion:async()=>'test',getServerStatus:async()=>true,getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},getComputerStatus:async()=>({state:'idle',installed:true}),stopComputerAccess:async()=>({state:'stopped'}),
   startComputerBrowser:async(request:unknown)=>{(window as any).browserRequests=[...((window as any).browserRequests??[]),request];if(legacy&&(request as any).additionalOrigins?.length)throw Error("Error invoking remote method 'computer-start': Error: computer_browser_workspace_unsupported");return {state:'ready',target:{id:'browser-host',origin:'https://example.com'}};}
  }});
 },{locale,legacy});
 const snapshot=()=>({run_id:id,task_id:id,prompt:'Read the approved research sites',model:'test-model',status:attached?'running':'waiting_for_input',steps:[],output:'',pending_approval:null,resumable:false,pending_input:attached?null:{id:'input-browser',call_id:'call-browser',kind:'computer',mode:'browser',target:'Research sites',url:'https://example.com/article'},requested_computer_approval_mode:'scoped_changes'});
 await page.route('**/health',route=>route.fulfill({json:{status:'healthy'}}));
 await page.route(/\/(?:api\/v2|v1)\//,async route=>{
  const path=new URL(route.request().url()).pathname;let data:unknown={};
  if(path==='/api/v2/system')data={product:'offgrid',api_version:2,version:'test',workspace_id:'browser-workspace',capabilities:['task-first-agents-v2','durable-agent-events-v2',...(legacy?[]:['browser-workspace-v1'])]};
  else if(path==='/v1/system/config')data={version:'test',require_auth:false};
  else if(path==='/v1/users/me')data={user:null,authenticated:false,guest:true};
  else if(path==='/v1/models')data={data:[{id:'test-model',type:'chat',context_window:8192}]};
  else if(path==='/api/v2/jobs'||path==='/v1/agents/tasks')data=[];
  else if(path===`/api/v2/jobs/${id}/events`)return route.fulfill({contentType:'text/event-stream',body:`data: ${JSON.stringify({...snapshot(),type:'snapshot'})}\n\n`});
  else if(path===`/api/v2/jobs/${id}/input`){expect(route.request().postDataJSON()).toEqual({input_id:'input-browser',computer_session:'browser-host'});attached++;data=snapshot();}
  else if(path===`/api/v2/jobs/${id}`)data=snapshot();
  else if(path==='/api/v2/computer/sessions')data={sessions:[]};
  else if(path==='/v1/catalog')data={models:[]};
  else if(path==='/v1/sessions')data={sessions:[]};
  return route.fulfill({json:data});
 });
 await page.goto(`/ui/#/agents/task/${id}`);
 await expect(page.locator('.task-access')).toBeVisible();
 return {attached:()=>attached};
}

test('additional origin presentation normalizes only bare public HTTPS sites',()=>{
 expect(additionalBrowserOrigins(' https://CDN.example.com/\nhttps://research.example.org:8443\n','https://example.com/article')).toEqual(['https://cdn.example.com','https://research.example.org:8443']);
 expect(additionalBrowserOrigins('https://bücher.example.com','https://example.com')).toEqual(['https://xn--bcher-kva.example.com']);
 expect(()=>additionalBrowserOrigins(`https://${'a'.repeat(505)}.com`,'https://example.com')).toThrow('computer_browser_scope_invalid');
 for(const value of ['https://x.example.com/a','https://x.example.com?q=1','https://x.example.com#x','https://u:p@x.example.com','http://x.example.com','https://127.0.0.1','https://[::1]','https://0x7f000001','https://x.local','https://x.example.com\nhttps://x.example.com','https://example.com','https://*.example.com','https://safe.\texample.com',Array.from({length:16},(_,i)=>`https://x${i}.example.com`).join('\n')])expect(()=>additionalBrowserOrigins(value,'https://example.com')).toThrow('computer_browser_scope_invalid');
});

test('browser access passes only explicitly entered sites and attaches the saved task once',async({page})=>{
 const f=await fixture(page);const field=page.getByRole('textbox',{name:'Additional sites',exact:true});
 await expect(field).toHaveValue('');await expect(field).toHaveAccessibleDescription(/Up to 15/);
 expect(await page.evaluate(()=>(window as any).browserRequests??[])).toEqual([]);
 await field.fill('https://CDN.example.com/\nhttps://research.example.org');
 await page.locator('.task-access .primary-button').click();
 await expect.poll(()=>f.attached()).toBe(1);
 expect(await page.evaluate(()=>(window as any).browserRequests)).toEqual([{workspace:'browser-workspace',requestId:'input-browser',origin:'https://example.com/article',additionalOrigins:['https://cdn.example.com','https://research.example.org'],approvalMode:'scoped_changes',networkMode:'direct'}]);
});

test('invalid or unsupported extra scope preserves the selection and never attaches a task',async({page})=>{
 const f=await fixture(page,'en',true),field=page.getByRole('textbox',{name:'Additional sites',exact:true});
 await field.fill('https://example.org/path');await page.locator('.task-access .primary-button').click();
 await expect(page.locator('.task-access [role=alert]')).toHaveText(computerExperience('en').invalidScope);
 expect(await page.evaluate(()=>(window as any).browserRequests??[])).toEqual([]);expect(f.attached()).toBe(0);
 await field.fill('https://example.org');await page.locator('.task-access .primary-button').click();
 await expect(page.locator('.task-access [role=alert]')).toHaveText(computerExperience('en').scopeUnsupported);
 await expect(field).toHaveValue('https://example.org');expect(f.attached()).toBe(0);
});

for(const locale of ['ar','zh-Hans'] as LocaleCode[])test(`browser scope stays labelled and usable in ${locale} on a narrow screen`,async({page},testInfo)=>{
 await page.setViewportSize({width:390,height:740});await fixture(page,locale);
 const copy=computerExperience(locale),field=page.getByRole('textbox',{name:copy.additionalSites,exact:true});
 await expect(field).toBeVisible();await expect(field).toHaveAttribute('dir','ltr');await expect(field).toHaveAccessibleDescription(copy.additionalSitesHint);
 await field.focus();await page.keyboard.type('https://example.org');await expect(field).toHaveValue('https://example.org');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 await page.screenshot({path:testInfo.outputPath(`browser-scope-${locale}.png`),fullPage:true});
});
