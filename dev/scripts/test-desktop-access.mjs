// Packaged current UI + real preload/main on an isolated HTTP fixture/profile.
// No tasks, consent, microphone or user application control is triggered.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,readFile,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve,join,dirname,sep,extname} from 'node:path';
import {createRequire} from 'node:module';
const root=resolve(import.meta.dirname,'../..');
const require=createRequire(join(root,'web/app/package.json'));
const {_electron:electron,expect}=require('@playwright/test');
const {verifyUI}=require('../../desktop/compatibility.cjs');
const {verifyPack}=require('../../computer/pack.cjs');
assert.ok(process.argv[2],'Pass the packaged desktop executable');
const binary=resolve(process.argv[2]),resources=join(dirname(binary),'resources'),ui=join(resources,'ui');
const manifest=verifyUI(ui);await verifyPack(join(resources,'computer'),'win32','x64');
const version=JSON.parse(await readFile(join(root,'desktop/package.json'),'utf8')).version;
const evidence=await mkdtemp(join(tmpdir(),'offgrid-desktop-access-'));let writes=0;
const server=createServer(async(req,res)=>{
 try {
  if(req.method!=='GET'){writes++;res.writeHead(405);res.end('{}');return;}
  const pathname=new URL(req.url,'http://127.0.0.1').pathname;
  if(pathname.startsWith('/ui/')){
   const file=resolve(ui,pathname.slice(4)||'index.html');assert.ok(file.startsWith(ui+sep));
   res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.woff':'font/woff','.woff2':'font/woff2','.svg':'image/svg+xml'})[extname(file)]||'application/octet-stream');res.end(await readFile(file));return;
  }
  const data={
   '/api/v2/system':{product:'offgrid',version,api_version:2,workspace_id:'access-fixture',ui_build_id:manifest.ui_build_id,desktop_bridge:{status:'ready',...manifest},capabilities:['sessions-v1','chat-streaming-v1','durable-agent-runs-v1','task-first-agents-v2','task-desktop-sessions-v3','agent-profiles-v1','agent-conversations-v1']},
   '/health':{status:'healthy'},'/v1/users/me':{authenticated:false,auth_required:false,user:null},
   '/v1/models':{data:[{id:'test-model',type:'chat'}]},'/api/v2/jobs':[],
   '/v1/sessions':{sessions:[]},'/api/v2/computer/sessions':{sessions:[]},'/v1/agents/tasks':[],
   '/api/v2/agent-profiles':{profiles:[{id:'access-agent',revision:1,settings:{name:'Access fixture',model:'test-model',computer:true,tools:[],knowledge:[],delegates:[],max_iterations:10}}]},
  };
  res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data[pathname]||{}));
 }catch{res.writeHead(500);res.end('{}');}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const env={...process.env,OFFGRID_PORT:String(server.address().port),OFFGRID_DESKTOP_HOME:evidence,OFFGRID_DESKTOP_TEST_HIDDEN:'1'};delete env.ELECTRON_RUN_AS_NODE;
let app;
const deadline=setTimeout(()=>{console.error('Packaged access check timed out');app?.process().kill();server.closeAllConnections();server.close();process.exitCode=1;},90000);deadline.unref();
try{
 app=await electron.launch({executablePath:binary,env,timeout:30000});
 const page=await app.firstWindow();page.setDefaultTimeout(10000);
 await page.waitForURL(/\/ui\//);await page.evaluate(()=>localStorage.setItem('offgrid.onboarding.complete','true'));
 await page.goto(`http://127.0.0.1:${server.address().port}/ui/#/chat`);await page.reload();
 await page.getByRole('button',{name:'Agent',exact:true}).click();
 await page.getByRole('radio',{name:/Access fixture/}).check();
 await page.keyboard.press('Escape');
 assert.equal((await page.evaluate(()=>window.electron.getDesktopAccess())).enabled,false);
 for(const zoom of [1,2]){
  await app.evaluate(({BrowserWindow},zoom)=>{const win=BrowserWindow.getAllWindows()[0];win.setSize(960,600);win.webContents.setZoomFactor(zoom);win.webContents.setBackgroundThrottling(false);},zoom);
  const trigger=page.getByRole('button',{name:'Computer access',exact:true});await trigger.click();
  const panel=page.getByRole('region',{name:'Computer access',exact:true});await expect(panel).toBeVisible();
  await expect(panel.getByText('Full access is not enabled',{exact:true})).toBeVisible();
  await expect(panel.getByRole('radio')).toHaveCount(3);
  await expect(panel.getByRole('radio',{name:/Full task access/})).not.toBeChecked();
  await expect(panel.getByRole('button',{name:'Enable Full access',exact:true})).toHaveCount(0);
  await panel.focus();await page.keyboard.press('End');
  await expect(panel.locator('.workspace-secondary')).toBeInViewport();
  if(zoom===2)await expect.poll(()=>panel.evaluate(el=>el.scrollHeight>el.clientHeight&&el.scrollTop>0)).toBe(true);
  for(let retry=0;retry<10;retry++){
   try{const png=await app.evaluate(async({BrowserWindow})=>(await BrowserWindow.getAllWindows()[0].webContents.capturePage(undefined,{stayHidden:true,stayAwake:true})).toPNG().toString('base64'));await writeFile(join(evidence,`access-zoom-${zoom}.png`),Buffer.from(png,'base64'));break;}
   catch(error){if(!String(error).includes('UnknownVizError')||retry===9)throw error;await new Promise(r=>setTimeout(r,200));}
  }
  await page.keyboard.press('Escape');await expect(panel).toBeHidden();await expect(trigger).toBeFocused();
 }
 assert.equal((await page.evaluate(()=>window.electron.getComputerStatus())).active,false);
 assert.equal(writes,0,'smoke check must not submit, approve or mutate');
 console.log(JSON.stringify({packagedAccess:true,zoom:[1,2],explicitConsentNotGranted:true,ui:manifest.ui_build_id,evidence}));
}finally{clearTimeout(deadline);await app?.close();server.closeAllConnections();await new Promise(r=>server.close(r));}
