import {expect,test,type Page} from '@playwright/test';
import {browserManualText} from '../src/i18n/browser-manual';
import {taskControls} from '../src/i18n/task-controls';
import {nativeAppFeedback} from '../src/i18n/native-app';
import {computerExperience} from '../src/i18n/computer-experience';
import {languageRegistry} from '../src/i18n/language';
import type {LocaleCode} from '../src/i18n';

const id='run-'+'f'.repeat(32);
async function fixture(page:Page,{locale='en' as LocaleCode,waiting=false,lostHandoff=false,lostResume=false,legacy=false,foreign=false,lostAttach=false,declineResume=false,closedResume=false}={}){
 let status=waiting?'waiting_for_input':'running',expired=waiting,attached=0;
 const actions:string[]=[],inputs:unknown[]=[];
 await page.addInitScript(options=>{
  localStorage.setItem('offgrid.locale',options.locale);localStorage.setItem('offgrid.onboarding.complete','true');
  const w=window as any;w.calls={takeover:[],resume:[],stop:[],start:[]};
  w.host={state:options.waiting?'manual':'ready',active:true,installed:true,browserManualProtocol:1,target:{id:options.foreign?'foreign-session':'old-browser',origin:'https://example.com'}};
  Object.assign(window,{electron:{isDesktop:true,platform:'win32',getApiUrl:async()=>location.origin,getVersion:async()=>'test',getServerStatus:async()=>true,getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},
   browserManualProtocol:1,getComputerStatus:async()=>w.host,
   takeoverComputerAccess:async(request:unknown)=>{
    w.calls.takeover.push(request);
    if(options.legacy)return {state:'unsupported',code:'computer_browser_manual_unsupported'};
    w.host.state='taking_over';
    if(options.lostHandoff)throw Error('computer_handoff_unconfirmed');
    await new Promise<void>(resolve=>{w.confirmHandoff=resolve;});w.host.state='manual';return w.host;
   },
   stopComputerAccess:async(request:any)=>{w.calls.stop.push(request);if(request.session&&request.session!==w.host.target?.id)return {state:'not_owned'};w.host={state:'stopped',installed:true};return w.host;},
   stopComputerBrowser:async()=>{w.calls.stop.push({all:true});w.host={state:'stopped',installed:true};return w.host;},
   startComputerBrowser:async(request:unknown)=>{w.calls.start.push(request);throw Error('session_active');},
   resumeComputerBrowser:async(request:unknown)=>{
    w.calls.resume.push(request);
    if(options.closedResume){w.host={state:'stopped',installed:true};return {state:'not_owned'};}
    if(options.declineResume&&w.calls.resume.length===1)throw Error('consent_declined');
    w.host.state='resuming';
    if(options.lostResume&&w.calls.resume.length===1)throw Error("Error invoking remote method 'computer-browser-resume': Error: computer_resume_unconfirmed");
    w.host={...w.host,state:'ready',target:{id:'new-browser',origin:'https://example.com'}};return w.host;
   },
  }});
 },{locale,waiting,lostHandoff,lostResume,legacy,foreign,declineResume,closedResume});
 const snapshot=()=>({run_id:id,task_id:id,prompt:'Continue the research in my browser',model:'test-model',status,steps:[],output:'',pending_approval:null,resumable:status==='interrupted',computer_session:attached?'new-browser':'old-browser',computer_session_expired:expired,last_access:{mode:'browser',target:'Research site',url:'https://example.com'},
  pending_input:status==='waiting_for_input'?{id:'input-browser-resume',call_id:'call-resume',kind:'computer',mode:'browser',target:'Research site',url:'https://example.com',previous_session:'old-browser'}:null,requested_computer_approval_mode:'scoped_changes'});
 await page.route('**/health',route=>route.fulfill({json:{status:'healthy'}}));
 await page.route(/\/(?:api\/v2|v1)\//,async route=>{
  const path=new URL(route.request().url()).pathname;let data:unknown={};
  if(path==='/api/v2/system')data={product:'offgrid',api_version:2,version:'test',workspace_id:'manual-workspace',capabilities:['task-first-agents-v2','durable-agent-events-v2','browser-workspace-v1','browser-task-pairing-v1']};
  else if(path==='/v1/system/config')data={version:'test',require_auth:false};
  else if(path==='/v1/users/me')data={user:null,authenticated:false,guest:true};
  else if(path==='/v1/models')data={data:[{id:'test-model',type:'chat',context_window:8192}]};
  else if(path==='/api/v2/jobs'||path==='/v1/agents/tasks')data=[];
  else if(path===`/api/v2/jobs/${id}/events`)return route.fulfill({contentType:'text/event-stream',body:`data: ${JSON.stringify({...snapshot(),type:'snapshot'})}\n\n`});
  else if(['takeover','reconnect','cancel'].some(action=>path===`/api/v2/jobs/${id}/${action}`)){
   const action=path.split('/').at(-1)!;actions.push(action);
   if(action==='takeover'){status='interrupted';expired=true;}
   if(action==='reconnect')status='waiting_for_input';
   if(action==='cancel')status='cancelled';
   data=snapshot();
  }else if(path===`/api/v2/jobs/${id}/input`){
   inputs.push(route.request().postDataJSON());
   if(lostAttach&&inputs.length===1)return route.abort('failed');
   attached++;status='running';expired=false;data=snapshot();
  }else if(path===`/api/v2/jobs/${id}`)data=snapshot();
  else if(path==='/api/v2/computer/sessions')data={sessions:[]};
  else if(path==='/v1/catalog')data={models:[]};
  else if(path==='/v1/sessions')data={sessions:[]};
  return route.fulfill({json:data});
 });
 await page.goto(`/ui/#/agents/task/${id}`);await expect(page.locator('.conversation-work')).toBeVisible();
 return {actions,inputs,attached:()=>attached,calls:()=>page.evaluate(()=>(window as any).calls)};
}

test('keyboard takeover quiesces locally before service revocation and explicitly resumes the same task',async({page})=>{
 const f=await fixture(page),copy=browserManualText('en');
 const button=page.getByRole('button',{name:taskControls('en').takeover,exact:true});await button.focus();await page.keyboard.press('Enter');
 await expect.poll(()=>page.evaluate(()=>typeof (window as any).confirmHandoff)).toBe('function');
 expect(f.actions).toEqual([]);await page.evaluate(()=>(window as any).confirmHandoff());
 await expect.poll(()=>f.actions).toEqual(['takeover']);await expect(page.getByText(copy.manual+' '+copy.hint,{exact:true})).toBeVisible();
 await page.getByRole('button',{name:taskControls('en').reconnect,exact:true}).click();
 const access=page.locator('.task-access');await expect(access.getByRole('button',{name:copy.resume,exact:true})).toBeVisible();
 await expect(access.locator('input,textarea,select')).toHaveCount(0);
 await access.getByRole('button',{name:copy.resume,exact:true}).focus();await page.keyboard.press('Enter');
 await expect.poll(()=>f.attached()).toBe(1);
 expect((await f.calls()).resume).toEqual([{workspace:'manual-workspace',runId:id,requestId:'input-browser-resume',session:'old-browser'}]);
 expect((await f.calls()).stop).toEqual([]);expect((await f.calls()).start).toEqual([]);
 expect(f.inputs).toEqual([{input_id:'input-browser-resume',computer_session:'new-browser'}]);
});

test('lost local handoff still revokes the service and leaves scoped Stop available',async({page})=>{
 const f=await fixture(page,{lostHandoff:true});await page.getByRole('button',{name:taskControls('en').takeover,exact:true}).click();
 await expect.poll(()=>f.actions).toEqual(['takeover']);
 await expect(page.getByText(browserManualText('en').pending,{exact:true})).toBeVisible();
 await page.locator('.conversation-work header').getByRole('button',{name:taskControls('en').stop,exact:true}).click();
 await expect.poll(()=>f.actions).toEqual(['takeover','cancel']);expect((await f.calls()).stop).toContainEqual({session:'old-browser'});
});

test('legacy takeover uses scoped close and still revokes service access',async({page})=>{
 const f=await fixture(page,{legacy:true});await page.getByRole('button',{name:taskControls('en').takeover,exact:true}).click();
 await expect.poll(()=>f.actions).toEqual(['takeover']);expect((await f.calls()).stop).toEqual([{session:'old-browser'}]);
});

test('lost resume acknowledgement checks the same operation without new browser start or old-session stop',async({page})=>{
 const f=await fixture(page,{waiting:true,lostResume:true}),copy=browserManualText('en'),access=page.locator('.task-access');
 await access.getByRole('button',{name:copy.resume,exact:true}).click();await expect(access.getByRole('alert')).toHaveText(copy.unconfirmed);
 await access.getByRole('button',{name:copy.check,exact:true}).click();await expect.poll(()=>f.attached()).toBe(1);
 const calls=await f.calls();expect(calls.resume).toHaveLength(2);expect(calls.resume[0]).toEqual(calls.resume[1]);expect(calls.start).toEqual([]);expect(calls.stop).toEqual([]);
});

test('lost task attachment retries only the saved returned session',async({page})=>{
 const f=await fixture(page,{waiting:true,lostAttach:true}),access=page.locator('.task-access');
 await access.getByRole('button',{name:browserManualText('en').resume,exact:true}).click();await expect(access.getByRole('alert')).toBeVisible();
 await access.locator('.primary-button').click();await expect.poll(()=>f.attached()).toBe(1);
 expect(f.inputs).toHaveLength(2);expect(f.inputs[0]).toEqual(f.inputs[1]);expect((await f.calls()).resume).toHaveLength(1);
});

test('a lost resume remains recoverable after navigation and a failed service session read',async({page})=>{
 const f=await fixture(page,{waiting:true,lostResume:true}),copy=browserManualText('en');
 await page.locator('.task-access').getByRole('button',{name:copy.resume,exact:true}).click();
 await expect(page.locator('.task-access').getByRole('alert')).toHaveText(copy.unconfirmed);
 // The original host operation completes after its renderer reply was lost.
 await page.evaluate(()=>{const w=window as any;w.host={...w.host,state:'ready',target:{id:'new-browser',origin:'https://example.com'}};});
 await page.goto('/ui/#/chat');await expect(page.locator('.task-access')).toHaveCount(0);
 await page.route('**/api/v2/computer/sessions',route=>route.abort('failed'));
 await page.goto(`/ui/#/agents/task/${id}`);
 const access=page.locator('.task-access');await expect(access.getByRole('button',{name:copy.check,exact:true})).toBeVisible();
 await expect(access.locator('input[type=url]')).toHaveCount(0);
 await access.getByRole('button',{name:copy.check,exact:true}).click();await expect.poll(()=>f.attached()).toBe(1);
 const calls=await f.calls();expect(calls.resume).toHaveLength(2);expect(calls.resume[0]).toEqual(calls.resume[1]);
 expect(calls.start).toEqual([]);expect(calls.stop).toEqual([]);
});

test('a different retained task is never offered as this task’s browser',async({page})=>{
 const f=await fixture(page,{waiting:true,foreign:true});
 await expect(page.locator('.task-access input[type=url]')).toBeVisible();
 await expect(page.getByRole('button',{name:browserManualText('en').resume,exact:true})).toHaveCount(0);
 expect((await f.calls()).resume).toEqual([]);expect((await f.calls()).stop).toEqual([]);
});

test('declined renewed consent keeps the manual browser and offers explicit resume again',async({page})=>{
 const f=await fixture(page,{waiting:true,declineResume:true}),access=page.locator('.task-access'),copy=browserManualText('en');
 await access.getByRole('button',{name:copy.resume,exact:true}).click();await expect(access.getByRole('alert')).toBeVisible();
 await expect(access.getByRole('button',{name:copy.resume,exact:true})).toBeEnabled();
 expect((await f.calls()).stop).toEqual([]);expect(f.attached()).toBe(0);
 await access.getByRole('button',{name:copy.resume,exact:true}).click();await expect.poll(()=>f.attached()).toBe(1);
 expect((await f.calls()).resume).toHaveLength(2);
});

test('a definitively closed retained browser returns to explicit access selection without opening another',async({page})=>{
 const f=await fixture(page,{waiting:true,closedResume:true}),access=page.locator('.task-access');
 await access.getByRole('button',{name:browserManualText('en').resume,exact:true}).click();await expect(access.getByRole('alert')).toBeVisible();
 await expect(access.locator('input[type=url]')).toBeVisible();expect(f.attached()).toBe(0);
 expect((await f.calls()).start).toEqual([]);expect((await f.calls()).stop).toEqual([]);
});

test('manual browser state and local Stop remain truthful across navigation and service outage',async({page})=>{
 const f=await fixture(page,{waiting:true}),copy=browserManualText('en');
 const banner=page.getByRole('complementary',{name:copy.manual,exact:true});await expect(banner).toBeVisible();
 await page.goto('/ui/#/chat');await expect(banner).toBeVisible();
 await page.route('**/api/v2/computer/**',route=>route.abort('failed'));
 await banner.getByRole('button',{name:nativeAppFeedback('en').stop,exact:true}).click();
 await expect.poll(async()=>(await f.calls()).stop).toContainEqual({all:true});
 expect(await page.evaluate(()=>(window as any).host.state)).toBe('stopped');
 const stopped=page.getByRole('complementary',{name:computerExperience('en').stopped,exact:true});await expect(stopped).toBeVisible();
 await expect(stopped.getByRole('alert')).toHaveText(computerExperience('en').localStoppedDisconnected);
 expect((await f.calls()).resume).toEqual([]);
});

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`manual browser controls stay localized and keyboard accessible in ${locale}`,async({page},info)=>{
 await page.setViewportSize({width:760,height:560});await fixture(page,{locale,waiting:true});
 const copy=browserManualText(locale),access=page.locator('.task-access');
 await expect(access).toContainText(copy.scope);await expect(access).toContainText(copy.hint);
 await expect(page.getByText(copy.manual+' '+copy.hint,{exact:true})).toHaveCount(0);
 const resume=access.getByRole('button',{name:copy.resume,exact:true});await resume.focus();await expect(resume).toBeFocused();
 await page.keyboard.press('Tab');await expect(access.getByRole('button',{name:nativeAppFeedback(locale).stop,exact:true})).toBeFocused();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 if(['en','ar','zh-Hans'].includes(locale))await page.screenshot({path:info.outputPath(`manual-${locale}.png`),fullPage:true});
});
