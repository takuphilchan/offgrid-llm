import {test,expect,type Page} from '@playwright/test';
import {workspaceRecovery} from '../src/i18n/workspace-recovery';
import type {LocaleCode} from '../src/i18n';
import languages from '../../desktop/language-registry.json' with {type:'json'};

async function fixture(page:Page,{locale='en',admin=true,available=true,bridge=true,protocol=true,readMode='ready'}:{locale?:LocaleCode;admin?:boolean;available?:boolean;bridge?:boolean;protocol?:boolean;readMode?:'ready'|'deferred'|'failed'|'invalid'}={}){
 await page.addInitScript(({locale,available,bridge,protocol,readMode})=>{
  localStorage.setItem('offgrid.locale',locale);localStorage.setItem('offgrid.onboarding.complete','true');
  let state:any={version:1,revision:0,phase:'idle',available,active:false,serviceStopped:false,canResume:false};const listeners=new Set<(value:any)=>void>();
  const emit=(update:any)=>{state={...state,revision:state.revision+1,...update};for(const listener of listeners)listener(state);return structuredClone(state);};
  Object.assign(window,{recoveryCalls:[],recoveryReads:0,recoveryReadMode:readMode,emitRecovery:emit,recoveryEvent:(value:any)=>{for(const listener of listeners)listener(value);}});
  if(bridge)Object.assign(window,{electron:{isDesktop:true,platform:'win32',workspaceRecoveryProtocol:protocol?1:undefined,getApiUrl:async()=>location.origin,getVersion:async()=>'fixture',getServerStatus:async()=>true,getSystemTheme:async()=> 'light',onThemeChange:()=>()=>{},getBackendInfo:async()=>({state:'ready',managedByDesktop:available}),getPaths:async()=>({models:'C:\\Fixture\\models',data:'C:\\Fixture\\data',config:'C:\\Fixture\\config'}),
   getWorkspaceRecovery:async()=>{(window as any).recoveryReads++;const mode=(window as any).recoveryReadMode;if(mode==='failed')throw Error('Read unavailable');if(mode==='invalid')return {version:2};if(mode==='deferred')return new Promise(resolve=>{(window as any).finishRecoveryRead=()=>resolve(structuredClone(state));});return structuredClone(state);},onWorkspaceRecovery:(callback:any)=>{listeners.add(callback);return()=>{listeners.delete(callback);};},
   startWorkspaceRecovery:async(request:any)=>{(window as any).recoveryCalls.push(request);return emit({...request,id:'a'.repeat(64),phase:'selecting',active:true,available:false});},
   cancelWorkspaceRecovery:async(request:any)=>{(window as any).recoveryCalls.push({cancel:request});return state;},
   resumeWorkspaceAfterRecovery:async(request:any)=>{(window as any).recoveryCalls.push({resume:request});return emit({phase:'idle',available:true,serviceStopped:false,canResume:false});}
  }});
 },{locale,available,bridge,protocol,readMode});
 await page.route(/\/(?:health|v1\/|api\/v2\/)/,route=>{
  const path=new URL(route.request().url()).pathname;
  const responses:Record<string,unknown>={'/health':{status:'healthy'},'/api/v2/system':{product:'offgrid',version:'fixture',api_version:2,workspace_id:'recovery-fixture'},'/v1/users/me':{auth_required:true,authenticated:true,user:{id:'user1',username:'fixture',role:admin?'admin':'user'}},'/v1/system/config':{version:'fixture',require_auth:true},'/v1/models':{data:[]},'/v1/sessions':{sessions:[]},'/v1/rag/status':{enabled:false},'/api/v2/computer/status':{active_sessions:0}};
  return route.fulfill({json:responses[path]??{}});
 });
 await page.goto('/ui/#/settings/recovery');await expect(page.locator('.workspace-recovery')).toBeVisible();
 return page.locator('.workspace-recovery');
}
test('renderer sends only enums, prevents duplicate admission and retains Stop plus explicit Resume',async({page})=>{
 const area=await fixture(page),copy=workspaceRecovery('en');await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeEnabled();
 await expect(area.locator('input')).toHaveCount(0);
 await area.getByRole('button',{name:copy.backup,exact:true}).evaluate(button=>{(button as HTMLButtonElement).click();(button as HTMLButtonElement).click();});
 await expect(area.getByRole('button',{name:copy.cancel,exact:true})).toBeVisible();
 expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([{operation:'backup',encryption:'passphrase'}]);
 await area.getByRole('button',{name:copy.cancel,exact:true}).click();await expect(area.getByRole('button',{name:copy.cancelling,exact:true})).toBeDisabled();
 await page.evaluate(()=>(window as any).emitRecovery({phase:'cancelled',active:false,serviceStopped:true,canResume:true}));
 await expect(area.getByRole('button',{name:copy.resume,exact:true})).toBeEnabled();await expect(area.getByRole('button',{name:copy.backup,exact:true})).toHaveCount(0);
 await area.getByRole('button',{name:copy.resume,exact:true}).click();await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeEnabled();
 expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([{operation:'backup',encryption:'passphrase'},{cancel:{id:'a'.repeat(64)}},{resume:{id:'a'.repeat(64)}}]);
});
test('failed and uncertain publication preserve truthful summaries and escaped retained paths without persistence',async({page})=>{
 const area=await fixture(page),copy=workspaceRecovery('en');const retained='C:\\isolated\\<img src=x onerror=alert(1)>\\'+ 'long'.repeat(60);
 await page.evaluate(path=>(window as any).emitRecovery({id:'b'.repeat(64),phase:'failed',active:false,available:false,serviceStopped:true,canResume:true,result:{files:3,bytes:4096,application_version:'fixture',encryption:'age-v1',authenticated:true,published:true,database_checked:false},retainedPaths:[{path,directory:true,contains_plaintext:true}]}),retained);
 await expect(area.getByRole('alert')).toHaveText(copy.failed);await expect(area.getByText(copy.published,{exact:true})).toBeVisible();await expect(area.getByText(copy.plaintext,{exact:true})).toBeVisible();await expect(area.locator('.recovery-retained code')).toHaveText(retained);await expect(area.locator('img')).toHaveCount(0);
 await page.evaluate(()=>(window as any).recoveryEvent({version:1,revision:0,phase:'succeeded',active:false,available:true,serviceStopped:false,canResume:false}));
 await expect(area.getByRole('alert')).toHaveText(copy.failed);
 await page.evaluate(()=>(window as any).emitRecovery({phase:'uncertain',active:true,canResume:false}));await expect(area.getByRole('status')).toHaveText(copy.uncertain);await expect(area.getByRole('button',{name:copy.resume})).toHaveCount(0);
 await expect(area.getByText(copy.stopped,{exact:true})).toHaveCount(0);
 expect(await page.evaluate(()=>JSON.stringify({...localStorage}))).not.toContain('isolated');
});
for(const context of [{admin:false},{available:false},{bridge:false},{protocol:false}])test('unsupported or unauthorized context does not admit recovery '+JSON.stringify(context),async({page})=>{
 const area=await fixture(page,context),copy=workspaceRecovery('en');
 const reason='admin' in context?copy.permissionRequired:'available' in context?copy.externalUnavailable:'bridge' in context?copy.browserUnavailable:copy.unsupported;
 await expect(area.getByRole('status')).toHaveText(reason);await expect(area.getByText(copy.idle,{exact:true})).toHaveCount(0);await expect(area.getByRole('button',{name:copy.backup,exact:true})).toHaveCount(0);await expect(area.getByRole('combobox')).toHaveCount(0);expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([]);
});

test('pending status cannot imply ready or enable an operation',async({page})=>{
 const area=await fixture(page,{readMode:'deferred'}),copy=workspaceRecovery('en');
 await expect(area.getByRole('status')).toHaveText(copy.checking);await expect(area.getByText(copy.idle,{exact:true})).toHaveCount(0);await expect(area.getByRole('button',{name:copy.backup,exact:true})).toHaveCount(0);
 await page.evaluate(()=>(window as any).finishRecoveryRead());await expect(area.getByRole('status')).toHaveText(copy.idle);await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeEnabled();
 expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([]);
});

test('recovery keeps the original directory card and separates its own padded content',async({page})=>{
 const area=await fixture(page,{protocol:false}),directory=page.locator('.desktop-local-paths');await expect(directory).toBeVisible();
 await expect(directory).toHaveClass(/settings-card/);await expect(directory.locator('dl>div')).toHaveCount(2);
 await expect(directory).toHaveCSS('border-top-width','1px');await expect(directory.locator('dl>div+div')).toHaveCSS('border-top-width','1px');
 expect(await directory.evaluate(el=>parseFloat(getComputedStyle(el).borderTopLeftRadius))).toBeGreaterThan(0);
 await expect(area).toHaveCSS('padding-top','24px');await expect(area).toHaveCSS('padding-bottom','24px');
 expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([]);
});

for(const readMode of ['failed','invalid'] as const)test('status '+readMode+' offers read-only retry without false readiness',async({page})=>{
 const area=await fixture(page,{readMode}),copy=workspaceRecovery('en');await expect(area.getByRole('alert')).toHaveText(copy.statusFailed);await expect(area.getByText(copy.idle,{exact:true})).toHaveCount(0);await expect(area.getByRole('combobox')).toHaveCount(0);
 const readsBeforeRetry=await page.evaluate(()=>(window as any).recoveryReads);
 await page.evaluate(()=>(window as any).recoveryReadMode='ready');await area.getByRole('button',{name:'Try again',exact:true}).click();await expect(area.getByRole('status')).toHaveText(copy.idle);
 expect(await page.evaluate(()=>(window as any).recoveryReads)).toBe(readsBeforeRetry+1);expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([]);
});

test('lost operation acknowledgment requires a status check without replay',async({page})=>{
 const area=await fixture(page),copy=workspaceRecovery('en');await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeEnabled();
 await page.evaluate(()=>{(window as any).electron.startWorkspaceRecovery=async(request:any)=>{(window as any).recoveryCalls.push(request);(window as any).emitRecovery({id:'c'.repeat(64),phase:'running',active:true,available:false});throw Error('Lost acknowledgment');};});
 await area.getByRole('button',{name:copy.backup,exact:true}).click();await expect(area.getByRole('alert')).toHaveText(copy.requestUnconfirmed);await expect(area.getByRole('button',{name:copy.cancel,exact:true})).toBeEnabled();
 await area.getByRole('button',{name:'Try again',exact:true}).click();await expect(area.getByRole('status')).toHaveText(copy.running);
 expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([{operation:'backup',encryption:'passphrase'}]);
});
test('keyboard selection distinguishes legacy reading from encrypted backup and keeps a narrow layout',async({page})=>{
 const area=await fixture(page),copy=workspaceRecovery('en');await page.setViewportSize({width:760,height:560});
 await area.getByRole('combobox',{name:copy.operation,exact:true}).selectOption('restore');await area.locator('.recovery-protection summary').click();await area.getByRole('combobox',{name:copy.encryption,exact:true}).selectOption('legacy');
 await area.getByRole('combobox',{name:copy.operation,exact:true}).selectOption('backup');await expect(area.getByRole('combobox',{name:copy.encryption,exact:true})).toHaveValue('passphrase');
 await area.getByRole('combobox',{name:copy.operation,exact:true}).focus();await page.keyboard.press('Tab');await expect(area.locator('.recovery-protection summary')).toBeFocused();await page.keyboard.press('Tab');await expect(area.getByRole('combobox',{name:copy.encryption,exact:true})).toBeFocused();
 await page.keyboard.press('Tab');await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeFocused();
 expect(await area.evaluate(element=>element.scrollWidth<=element.clientWidth+1)).toBe(true);
});

test('a late idle event does not silently dismiss an unconfirmed operation',async({page})=>{
 const area=await fixture(page),copy=workspaceRecovery('en');await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeEnabled();
 await page.evaluate(()=>{(window as any).electron.startWorkspaceRecovery=async(request:any)=>{(window as any).recoveryCalls.push(request);throw Error('Lost acknowledgment');};});
 await area.getByRole('button',{name:copy.backup,exact:true}).click();await expect(area.getByRole('alert')).toHaveText(copy.requestUnconfirmed);
 await page.evaluate(()=>(window as any).emitRecovery({phase:'idle',available:true,active:false}));
 await expect(area.getByRole('alert')).toHaveText(copy.requestUnconfirmed);await expect(area.getByRole('button',{name:copy.backup,exact:true})).toHaveCount(0);await expect(area.getByText(copy.idle,{exact:true})).toHaveCount(0);
 await area.getByRole('button',{name:'Try again',exact:true}).click();await expect(area.getByRole('status')).toHaveText(copy.idle);await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeEnabled();
 expect(await page.evaluate(()=>(window as any).recoveryCalls)).toEqual([{operation:'backup',encryption:'passphrase'}]);
});
test('intentional service outage preserves admitted recovery controls through focus refresh',async({page})=>{
 const area=await fixture(page),copy=workspaceRecovery('en');await area.getByRole('button',{name:copy.backup,exact:true}).click();
 await page.evaluate(()=>(window as any).emitRecovery({phase:'succeeded',active:false,available:false,serviceStopped:true,canResume:true}));
 await page.route('**/v1/users/me',route=>route.abort('connectionrefused'));
 await page.evaluate(()=>window.dispatchEvent(new Event('focus')));
 await expect(area.getByRole('button',{name:copy.resume,exact:true})).toBeVisible();await expect(page.locator('.boot-screen')).toHaveCount(0);
 await page.route('**/v1/users/me',route=>route.fulfill({status:401,json:{error:'session expired'}}));
 await page.evaluate(()=>window.dispatchEvent(new Event('focus')));await expect(area).toHaveCount(0);
});
test('late workspace onboarding identity does not obscure an explicit recovery route',async({page})=>{
 // A different workspace already consumed the legacy completion preference.
 // The new identity is pending, even though the browser-wide value is true.
 await page.addInitScript(()=>localStorage.setItem('offgrid.onboarding.legacyOwner','different-workspace'));
 const area=await fixture(page);await expect(area).toBeVisible();
 await expect(page.locator('.page-content')).not.toHaveAttribute('hidden','');
 expect(await page.evaluate(()=>localStorage.getItem('offgrid.onboarding.workspace:'+JSON.stringify(['user1','recovery-fixture'])))).not.toBe('complete');
});
for(const locale of Object.keys(languages) as LocaleCode[])test('recovery messages and narrow status remain accessible in '+locale,async({page})=>{
 const area=await fixture(page,{locale}),copy=workspaceRecovery(locale);await page.setViewportSize({width:760,height:560});
 if(locale!=='en')expect(copy.title).not.toBe(workspaceRecovery('en').title);
 await expect(area.getByRole('heading',{name:copy.title,exact:true})).toBeVisible();
 await expect(area.getByRole('combobox',{name:copy.operation,exact:true})).toBeVisible();
 await area.locator('.recovery-protection summary').click();await expect(area.getByRole('combobox',{name:copy.encryption,exact:true})).toBeVisible();
 await expect(area.getByRole('button',{name:copy.backup,exact:true})).toBeEnabled();
 expect(await area.evaluate(element=>element.scrollWidth<=element.clientWidth+1)).toBe(true);
 await expect(page.locator('html')).toHaveAttribute('dir',languages[locale].direction);
});
