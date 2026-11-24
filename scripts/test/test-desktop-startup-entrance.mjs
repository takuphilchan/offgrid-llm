// Hidden, disposable profile and synthetic HTTP service only. No workspace
// grants, commands, real service writes, models or personal application state.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtempSync,readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {resolve,join,extname,sep} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
const root=resolve(import.meta.dirname,'../..'),require=createRequire(join(root,'apps/web/package.json'));
const {_electron:electron,expect}=require('@playwright/test');
const binary=resolve(process.argv[2]),ui=resolve(binary,'../resources/ui');
const evidence=mkdtempSync(join(tmpdir(),'offgrid-startup-entrance-'));
const uiBuild=createHash('sha256').update(readFileSync(join(ui,'index.html'),'utf8').replace(/\r\n/g,'\n')).digest('hex');
let protocol=2,failAccess=true,accessGate,releaseAccess;
const writes=[],checks=[];
const server=createServer(async(req,res)=>{
 try{
  const p=new URL(req.url,'http://localhost').pathname;if(req.method!=='GET')writes.push(p);
  if(p.startsWith('/ui/')){const file=resolve(ui,p.slice(4)||'index.html');assert.ok(file.startsWith(ui+sep));res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.woff2':'font/woff2'})[extname(file)]||'application/octet-stream');res.end(readFileSync(file));return;}
  if(p==='/v1/users/me'){await accessGate;if(failAccess){res.writeHead(503,{'Content-Type':'application/json'}).end(JSON.stringify({error:{message:'Fixture workspace is temporarily unavailable.'}}));return;}}
  const data={'/api/v2/system':{product:'offgrid',version:'0.4.32',api_version:2,workspace_id:'startup-entrance-fixture',ui_build_id:uiBuild,desktop_bridge:{status:'ready',schema_version:1,protocol,ui_build_id:uiBuild},capabilities:['sessions-v1','chat-streaming-v1','durable-agent-runs-v1']},'/health':{status:'healthy'},'/v1/users/me':{authenticated:false,auth_required:false,user:null},'/v1/models':{data:[]},'/v1/sessions':{sessions:[]},'/v1/catalog':{models:[]},'/api/v2/models':{models:[]},'/api/v2/models/packages':{packages:[]},'/v1/agents/tasks':[], '/v1/documents':{documents:[],count:0},'/v1/rag/status':{enabled:false,stats:{}},'/v1/system/config':{version:'0.4.32',require_auth:false}};
  res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data[p]||{}));
 }catch{res.writeHead(500).end('{}');}
});
await new Promise(r=>server.listen(0,'127.0.0.1',r));
let app;
const deadline=setTimeout(()=>{console.error('Startup fixture deadline',evidence);app?.process().kill();process.exit(1);},180000);deadline.unref();
const capture=async(page,name)=>{
 await page.evaluate(()=>document.fonts.ready);
 await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
 const grab=()=>app.evaluate(async({BrowserWindow})=>(await BrowserWindow.getAllWindows()[0].webContents.capturePage(undefined,{stayHidden:true,stayAwake:true})).toPNG().toString('base64'));
 // A newly created hidden compositor may not have a surface on its first frame.
 await expect(async()=>{await grab();}).toPass({timeout:5000,intervals:[100,200,400]});
 await new Promise(r=>setTimeout(r,180));writeFileSync(join(evidence,name+'.png'),Buffer.from(await grab(),'base64'));
};
const geometry=page=>page.locator('.startup-brand').evaluate(el=>{const r=el.getBoundingClientRect(),svg=el.querySelector('svg').getBoundingClientRect();return{x:r.x,y:r.y,width:r.width,height:r.height,markWidth:svg.width,markHeight:svg.height,font:getComputedStyle(el).fontSize};});
try{
 for(const [locale,theme,zoom,width,height] of [['en','light',1,1000,720],['en','dark',1,1000,720],['ar','dark',2,1280,900]]){
  const name=locale+'-'+theme+'-'+zoom,profile=join(evidence,name);mkdirSync(profile);
  writeFileSync(join(profile,'desktop-presentation.json'),JSON.stringify({locale,theme,appearance:{version:1,light:'slate',dark:'offgrid',textSize:100,reduceMotion:false}}));
  protocol=2;failAccess=true;accessGate=new Promise(r=>releaseAccess=r);
  const env={...process.env,OFFGRID_DESKTOP_HOME:profile,OFFGRID_DESKTOP_TEST_HIDDEN:'1',OFFGRID_PORT:String(server.address().port)};delete env.ELECTRON_RUN_AS_NODE;
  app=await electron.launch({executablePath:binary,env,timeout:30000});const page=await app.firstWindow();
  await app.evaluate(({BrowserWindow},{width,height,zoom})=>{const w=BrowserWindow.getAllWindows()[0];w.show=()=>{};w.focus=()=>{};w.setContentSize(width,height);w.webContents.setZoomFactor(zoom);w.webContents.setBackgroundThrottling(false);},{width,height,zoom});
  await expect(page.locator('#recovery')).toBeVisible();
  await expect(page.locator('#technical')).toHaveCSS('border-top-width','0px');
  await expect(page.locator('.startup-brand svg')).toHaveCSS('animation-name','none');
  await expect(page.locator('.startup-welcome,.startup-elapsed')).toHaveCount(0);
  await capture(page,'native-recovery-'+name);
  await page.locator('#technical>summary').click();await expect(page.locator('#address')).toBeVisible();await page.locator('#technical>summary').click();
  // The busy frame uses the real packaged local assets. Synthetic presentation
  // state keeps it inspectable; subsequent retry/navigation uses the real host.
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].webContents.send('startup-state',{state:'starting',managedByDesktop:true}));
  await expect(page.locator('html')).toHaveAttribute('data-startup-state','loading');
  await expect(page.locator('.startup-brand svg')).toHaveCSS('animation-name','startup-breathe');
  for(const layer of await page.locator('.startup-brand svg path').all())await expect(layer).toHaveCSS('transform','none');
  const native=await geometry(page);assert.ok(Math.abs(native.markWidth-38)<1);
  await capture(page,'native-loading-'+name);
  protocol=1;await page.evaluate(()=>window.electron.retryStartup());await page.waitForURL(/\/ui\//);
  await expect(page.locator('#workspace-startup')).toBeVisible();
  // Zoom is a test stress condition, reapplied after the navigation creates a
  // new document; it is not a claim that an arbitrary DevTools zoom persists.
  await expect(page.locator('#root .boot-screen')).toBeAttached();
  await app.evaluate(({BrowserWindow},zoom)=>BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(zoom),zoom);
  await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
  const renderer=await geometry(page);assert.ok(Math.abs(native.y-renderer.y)<2,JSON.stringify({native,renderer}));
  assert.ok(Math.abs(native.markWidth-renderer.markWidth)<1);assert.equal(native.font,renderer.font);
  await expect(page.locator('#root')).toHaveAttribute('inert','');
  await capture(page,'renderer-loading-'+name);
  releaseAccess();await expect(page.locator('#workspace-startup')).toHaveAttribute('data-startup-state','recovery');
  await expect(page.locator('.startup-message')).toHaveText('Fixture workspace is temporarily unavailable.');
  await capture(page,'renderer-recovery-'+name);
  failAccess=false;await page.locator('#workspace-startup button').click();
  await expect(page.locator('#workspace-startup')).toHaveCount(0);await expect(page.locator('#root')).not.toHaveAttribute('inert');
  await expect(page.locator('.app-shell')).toBeVisible();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  await capture(page,'workspace-ready-'+name);
  checks.push({locale,theme,zoom,native,renderer,retryReachedWorkspace:true});
  await app.close();app=null;
 }
 assert.deepEqual(writes,[]);
 writeFileSync(join(evidence,'result.json'),JSON.stringify({status:'passed',binary,uiBuild,checks,serviceWrites:0,personalData:false,nativeBusyPresentation:'synthetic host event; real packaged assets',handoff:'real host retry and renderer readiness'},null,2));
 console.log(JSON.stringify({status:'passed',evidence,checks:checks.length}));
}finally{releaseAccess?.();clearTimeout(deadline);await app?.close();server.closeAllConnections();await new Promise(r=>server.close(r));}
