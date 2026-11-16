// Native main/preload/renderer integration with disposable data and a local HTTP
// fixture. Windows stay hidden; show/focus are intercepted so this test cannot
// take over the user's desktop. OS shortcut delivery is deliberately not claimed.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {once} from 'node:events';
import {mkdtemp,readFile,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve,join,extname,sep,dirname} from 'node:path';
import {createRequire} from 'node:module';
import {setTimeout as delay} from 'node:timers/promises';

// An optional packaged executable exercises the exact installer payload.
const root=resolve(import.meta.dirname,'../..'),packaged=Boolean(process.argv[2]);
const binary=packaged?resolve(process.argv[2]):join(root,'apps/desktop/node_modules/electron/dist/electron.exe');
const dist=packaged?join(dirname(binary),'resources/ui'):join(root,'apps/web/dist');
const require=createRequire(join(root,'apps/web/package.json'));
const { _electron:electron, expect }=require('@playwright/test');
const {fingerprintUI}=require(join(root,'apps/desktop/compatibility.cjs'));
const {version}=JSON.parse(await readFile(join(root,'apps/desktop/package.json'),'utf8'));
const hash=fingerprintUI(await readFile(join(dist,'index.html')));
const profile=await mkdtemp(join(tmpdir(),'offgrid-command-'));
await writeFile(join(profile,'desktop-presentation.json'),JSON.stringify({locale:'en',theme:'dark'}));
const writes=[];
const identity={product:'offgrid',version,api_version:2,workspace_id:'command-fixture',ui_build_id:hash,
 desktop_bridge:{status:'ready',schema_version:1,protocol:1,ui_build_id:hash},
 capabilities:['sessions-v1','chat-streaming-v1','durable-agent-runs-v1']};
const bodies={
 '/health':{status:'healthy'},'/api/v2/system':identity,'/v1/system/config':{version,require_auth:false},
 '/v1/users/me':{authenticated:false,auth_required:false},'/v1/models':{data:[{id:'fixture-model',type:'chat'}]},
 '/v1/sessions':{sessions:[]},'/v1/rag/status':{enabled:false},'/api/v2/agent-profiles':{profiles:[]},
 '/api/v2/models':{models:[]},'/api/v2/models/operations':{operations:[]},'/v1/audio/status':{profiles:[]},
 '/api/v2/jobs':[], '/v1/agents/tasks':[], '/api/v2/computer/status':{available:false},
};
let holdHistory=false,releaseHistory;
const historyGate=new Promise(resolve=>{releaseHistory=resolve;});
const server=createServer(async(request,response)=>{
 try {
  const pathname=new URL(request.url,'http://localhost').pathname;
  if(request.method!=='GET')writes.push(pathname);
  if(pathname==='/v1/sessions'&&holdHistory)await historyGate;
  if(pathname.startsWith('/ui/')) {
   const file=resolve(dist,decodeURIComponent(pathname.slice(4))||'index.html');
   if(!file.startsWith(dist+sep)){response.writeHead(403).end();return;}
   const body=await readFile(file);
   const mime={'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.woff2':'font/woff2','.svg':'image/svg+xml'};
   response.writeHead(200,{'Content-Type':mime[extname(file)]||'application/octet-stream'}).end(body);return;
  }
  response.writeHead(200,{'Content-Type':'application/json'}).end(JSON.stringify(bodies[pathname]??{}));
 }catch{response.writeHead(404).end();}
});
server.listen(0,'127.0.0.1');await once(server,'listening');
const env={...process.env,OFFGRID_DESKTOP_HOME:profile,OFFGRID_DESKTOP_TEST_HIDDEN:'1',OFFGRID_PORT:String(server.address().port)};
delete env.ELECTRON_RUN_AS_NODE;
let app;
try {
 app=await electron.launch({executablePath:binary,args:packaged?[]:[join(root,'apps/desktop')],cwd:dirname(binary),env,timeout:30000});
 const page=await app.firstWindow();
 await page.waitForURL(/\/ui\//);
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isVisible()),false);
 await app.evaluate(({BrowserWindow})=>{
  const window=BrowserWindow.getAllWindows()[0];
  window.show=()=>{};window.focus=()=>{};window.webContents.setBackgroundThrottling(false);
 });
 await page.evaluate(()=>{
  localStorage.setItem('offgrid.onboarding.complete','true');localStorage.setItem('offgrid.locale','en');localStorage.setItem('offgrid.theme','dark');
 });
 holdHistory=true;
 await page.reload();
 await expect(page.locator('#workspace-startup')).toBeVisible();
 await expect(page.locator('.composer textarea')).toBeHidden();
 for(const zoom of [1,1.5,2]) {
  await app.evaluate(({BrowserWindow},factor)=>BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(factor),zoom);
  await expect.poll(()=>page.evaluate(()=>{
   const splash=document.querySelector('#workspace-startup').getBoundingClientRect();
   const caption=document.querySelector('#offgrid-native-chrome').getBoundingClientRect();
   return Math.abs(splash.top-caption.bottom)<1&&document.documentElement.scrollHeight<=innerHeight&&document.documentElement.scrollWidth<=innerWidth;
  })).toBe(true);
 }
 await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(1));
 releaseHistory();holdHistory=false;
 await expect(page.locator('#workspace-startup')).toHaveCount(0);
 await expect(page.locator('.composer textarea')).toBeVisible();
 const original=await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].getNormalBounds());
 // Hidden native windows can reject CDP keyboard input without OS focus. Inject
 // a DOM input event instead; ordinary keyboard/IME is covered in renderer tests.
 // Confirm React persisted it before switching. Never rewrite it after a switch.
 await expect(page.locator('.composer textarea')).toBeEditable();
 await page.locator('.composer textarea').evaluate(input=>{
  Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(input,'Keep this unsent request');
  input.dispatchEvent(new Event('input',{bubbles:true}));
 });
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this unsent request');
 assert.equal(await page.evaluate(()=>Object.keys(localStorage).some(key=>key.startsWith('offgrid.draft.v2:')&&key.endsWith(':chat:')&&localStorage.getItem(key)==='Keep this unsent request')),true);
 await page.evaluate(()=>window.electron.openCommandWindow());
 await expect(page.locator('.app-shell')).toHaveClass(/command-window/);
 await page.evaluate(()=>window.electron.setCommandLayout('input'));
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this unsent request');
 // The native resize reply can precede Chromium's resize event and the
 // renderer viewport update. Measure the settled layout, not that old frame.
 await expect.poll(()=>page.evaluate(()=>{
  const composer=document.querySelector('.composer').getBoundingClientRect();
  const chrome=document.querySelector('#offgrid-native-chrome').getBoundingClientRect();
  return composer.top>=chrome.bottom&&composer.bottom<=innerHeight;
 }),{timeout:2000}).toBe(true);
 const geometry=await page.evaluate(()=>{
  const composer=document.querySelector('.composer').getBoundingClientRect();
  const chrome=document.querySelector('#offgrid-native-chrome').getBoundingClientRect();
  return {bottom:composer.bottom,top:composer.top,height:innerHeight,chrome:chrome.bottom,overflow:document.documentElement.scrollWidth>innerWidth};
 });
 assert.ok(geometry.top>=geometry.chrome&&geometry.bottom<=geometry.height,JSON.stringify(geometry));
 assert.equal(geometry.overflow,false);
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isMaximizable()),false);
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isResizable()),false);
 await expect.poll(()=>page.locator('.composer').evaluate(node=>node.scrollHeight<=node.clientHeight+1)).toBe(true);
 await expect(page.locator('.composer-actions .model-select-trigger')).toBeInViewport();
 await page.evaluate(()=>window.electron.dismissCommandWindow());
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isVisible()),false);
 await page.evaluate(()=>window.electron.openCommandWindow());
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this unsent request');
 await page.evaluate(()=>window.electron.expandCommandWindow());
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isResizable()),true);
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isMaximizable()),true);
 const restored=[];
 for(let cycle=0;cycle<5;cycle++) {
  await page.evaluate(()=>window.electron.openCommandWindow());await page.evaluate(()=>window.electron.expandCommandWindow());
  restored.push(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].getNormalBounds()));
 }
 // Windows rounds device-independent bounds at fractional display scaling.
 // Allow one rounding step, but never cumulative growth on repeated reopening.
 for(const bounds of restored)for(const key of ['x','y','width','height'])assert.ok(Math.abs(bounds[key]-original[key])<=2,JSON.stringify({original,restored}));
 // Exercise actual native resizing and the trusted renderer handshake while
 // still preventing any OS window from appearing. Visibility is simulated only
 // for the animation eligibility check; native bounds, IPC and paints are real.
 await app.evaluate(({BrowserWindow})=>{
  const window=BrowserWindow.getAllWindows()[0];
  window.__realVisible=window.isVisible.bind(window);window.isVisible=()=>true;
  window.__boundsFrames=[];const setBounds=window.setBounds.bind(window);
  window.setBounds=(bounds,...args)=>{window.__boundsFrames.push(bounds);return setBounds(bounds,...args);};
 });
 await page.evaluate(()=>{
  Object.defineProperty(document,'hidden',{configurable:true,get:()=>false});
  document.documentElement.dataset.reduceMotion='false';
  window.__transitionPhases=[];
  new MutationObserver(()=>window.__transitionPhases.push(document.documentElement.dataset.windowTransition??'settled')).observe(document.documentElement,{attributes:true,attributeFilter:['data-window-transition']});
 });
 await page.evaluate(()=>window.electron.openCommandWindow());
 await expect(page.locator('html')).not.toHaveAttribute('data-window-transition',/./);
 const animatedFrames=await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].__boundsFrames.length);
 assert.ok(animatedFrames>3,'A visible summon should interpolate native bounds');
 assert.deepEqual(await page.evaluate(()=>[...new Set(window.__transitionPhases)]),['leaving','resizing','entering','settled']);
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this unsent request');
 await page.evaluate(()=>window.electron.setCommandLayout('voice',{prepared:true,reduceMotion:false}));
 assert.equal((await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].getNormalBounds())).width,420);
 await page.evaluate(async()=>{
  const voice=window.electron.setCommandLayout('input',{prepared:true,reduceMotion:false});
  await new Promise(resolve=>setTimeout(resolve,35));
  await window.electron.setCommandLayout('work',{prepared:true,reduceMotion:false});await voice;
 });
 assert.ok(Math.abs((await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].getNormalBounds())).height-480)<=2);
 await page.evaluate(()=>window.electron.expandCommandWindow());
 await expect(page.locator('html')).not.toHaveAttribute('data-window-transition',/./);
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this unsent request');
 await page.evaluate(()=>{document.documentElement.dataset.reduceMotion='true';});
 await app.evaluate(({BrowserWindow})=>{BrowserWindow.getAllWindows()[0].__boundsFrames=[];});
 await page.evaluate(()=>window.electron.openCommandWindow());
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].__boundsFrames.length),1,'Reduced motion must skip bounds interpolation');
 await expect(page.locator('html')).not.toHaveAttribute('data-window-transition',/./);
 await app.evaluate(({BrowserWindow})=>{const window=BrowserWindow.getAllWindows()[0];window.isVisible=window.__realVisible;});
 await page.evaluate(()=>{delete document.hidden;});
 await page.evaluate(()=>window.electron.setCommandShortcut(''));
 assert.deepEqual(JSON.parse(await readFile(join(profile,'command-window.json'),'utf8')),{shortcut:''});
 assert.match(await page.evaluate(async()=>{try{await window.electron.setCommandShortcut('unbounded-value');return 'accepted';}catch(error){return error.message;}}),/invalid_command_shortcut/);
 await page.evaluate(()=>window.electron.openCommandWindow());
 await page.evaluate(()=>window.electron.setCommandLayout('input',{prepared:true,reduceMotion:true}));
 await expect(page.locator('html')).not.toHaveAttribute('data-window-transition',/./);
 await expect(page.locator('.app-shell')).toHaveCSS('opacity','1');
 await expect(page.locator('.composer textarea')).toBeInViewport();
 await expect.poll(()=>page.evaluate(()=>innerHeight)).toBeLessThanOrEqual(166);
 await delay(200); // Let the hidden compositor publish the final revealed frame.
 for(let attempt=0;attempt<10;attempt++) {
  try {
   const png=await app.evaluate(async({BrowserWindow})=>(await BrowserWindow.getAllWindows()[0].webContents.capturePage(undefined,{stayHidden:true,stayAwake:true})).toPNG().toString('base64'));
   await writeFile(join(profile,'compact-native.png'),Buffer.from(png,'base64'));break;
  }catch(error){if(!String(error).includes('UnknownVizError')||attempt===9)throw error;await delay(200);}
 }
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows().length),1);
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isVisible()),false);
 assert.deepEqual(writes,[],'Presentation and draft edits must not submit work');
 console.log(JSON.stringify({status:'passed',version:await app.evaluate(({app})=>app.getVersion()),packaged,profile,geometry,animatedFrames,checks:['one trusted window','draft retained','hidden capture','native chrome clearance','bounds restored','trusted transition acknowledgment','native eased frames and reversal','reduced motion','shortcut persistence and validation','no work submitted'],limits:[`${packaged?'packaged':'source'} Electron with HTTP fixtures`,'show/focus intercepted; DOM input injection','visibility simulated for motion eligibility; window stays hidden','no OS shortcut delivery or real model execution']}));
}finally{if(app)await app.close();server.close();server.closeAllConnections();}
