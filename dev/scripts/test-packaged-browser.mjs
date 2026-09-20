// Real packaged main/preload/UI/utility process. Only an owned local demo is
// controlled; fixture consent is explicitly intercepted in this test process.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,readFile,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve,extname,sep} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
const root=resolve(import.meta.dirname,'../..');
const require=createRequire(join(root,'web/app/package.json'));
const {_electron:electron}=require('playwright');
const {expect}=require('@playwright/test');
const binary=resolve(process.argv[2]||'');assert.ok(process.argv[2],'Pass the packaged executable');
const ui=join(root,'web/dist');
const version=JSON.parse(await readFile(join(root,'desktop/package.json'),'utf8')).version;
const uiBuild=createHash('sha256').update((await readFile(join(ui,'index.html'),'utf8')).replace(/\r\n/g,'\n')).digest('hex');
const directory=await mkdtemp(join(tmpdir(),'offgrid-packaged-browser-'));
const pairingCode='b'.repeat(64);let paired=false;let stopped=false;let next=null;const replies=[];
const session={id:'packaged-session',origin:'offgrid-demo://research',state:'ready',remaining_actions:100,expires_at:new Date(Date.now()+600000).toISOString()};
let finished,failed;const completion=new Promise((res,rej)=>{finished=res;failed=rej;});
completion.catch(()=>{}); // A UI failure may precede the awaited protocol result.
const server=createServer(async(req,res)=>{
 try {
  const url=new URL(req.url,'http://127.0.0.1');
  if(url.pathname.startsWith('/ui/')){
   const relative=url.pathname.slice(4)||'index.html';const file=resolve(ui,relative);assert.ok(file.startsWith(ui+sep));
   const mime={'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.woff':'font/woff','.woff2':'font/woff2'};
   res.setHeader('Content-Type',mime[extname(file)]||'application/octet-stream');res.end(await readFile(file));return;
  }
  let raw='';for await(const chunk of req)raw+=chunk;const body=raw?JSON.parse(raw):{};let value={};
  if(url.pathname==='/api/v2/system')value={product:'offgrid',api_version:2,version,ui_build_id:uiBuild,workspace_id:'packaged-fixture',capabilities:['sessions-v1','chat-streaming-v1','durable-agent-runs-v1']};
  else if(url.pathname==='/health')value={status:'healthy'};
  else if(url.pathname==='/v1/users/me'){res.setHeader('Set-Cookie','fixture-auth=owned; HttpOnly; SameSite=Strict; Path=/');value={authenticated:false,auth_required:false,user:null};}
  else if(url.pathname==='/v1/models')value={data:[{id:'fixture-model',type:'chat'}]};
  else if(url.pathname==='/v1/agents/tasks')value=[];
  else if(url.pathname==='/v1/agents/tools')value={tools:[],enabled_count:0};
  else if(url.pathname==='/v1/agents/mcp')value={servers:[]};
  else if(url.pathname==='/v1/integrations')value={integrations:[]};
  else if(url.pathname==='/v1/sessions')value={sessions:[]};
  else if(url.pathname==='/api/v2/computer/status')value={available:paired&&!stopped};
  else if(url.pathname==='/api/v2/computer/sessions')value={sessions:paired&&!stopped?[session]:[]};
  else if(url.pathname==='/api/v2/computer/pairing'){assert.match(req.headers.cookie||'',/fixture-auth=owned/);value={code:pairingCode,expires_seconds:120};}
  else if(url.pathname==='/api/v2/computer/stop'){stopped=true;value={status:'stopping'};}
  else if(url.pathname.endsWith('/companion/pair')){assert.equal(body.code,pairingCode);assert.equal(body.origin,session.origin);const first=!paired;paired=true;stopped=false;value={token:'packaged-fixture-token',session};next=first?{id:'read',kind:'browser_observe',arguments:{}}:null;}
  else if(url.pathname.includes('/companion/')){
   assert.equal(req.headers.authorization,'Bearer packaged-fixture-token');
   if(url.pathname.endsWith('/poll')){value={action:next};next=null;}
   else if(url.pathname.endsWith('/reply')){
    assert.equal(body.error,undefined);const view=JSON.parse(body.result);replies.push(body.id);
    if(body.id==='read')next={id:'write',kind:'browser_fill',arguments:{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Report title').id,text:'Packaged desktop test'}};
    else if(body.id==='write'){const control=view.elements.find(e=>e.label==='Report format');next={id:'select',kind:'browser_select',arguments:{observation_id:view.observation_id,element:control.id,option:control.options.find(o=>o.label==='Table').id}};}
    else if(body.id==='select'){assert.equal(view.check,'selected_option_matches');next={id:'check',kind:'browser_set_checked',arguments:{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Include sources').id,checked:true}};}
    else if(body.id==='check'){assert.equal(view.check,'checked_state_matches');next={id:'save',kind:'browser_click',arguments:{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Save draft').id}};}
    else if(body.id==='save'){assert.match(view.text,/Draft saved: Packaged desktop test/);next={id:'verify',kind:'browser_verify',arguments:{text:'Draft saved: Packaged desktop test'}};}
    else if(body.id==='verify'){assert.equal(view.verified,true);next={id:'verify-options',kind:'browser_verify',arguments:{text:'Format: Table; Sources: included'}};}
    else if(body.id==='verify-options'){assert.equal(view.verified,true);finished();}
   }
  }
  res.setHeader('Content-Type','application/json');res.end(JSON.stringify(value));
 }catch(error){failed(error);res.statusCode=500;res.end('{}');}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const env={...process.env,OFFGRID_DESKTOP_HOME:directory,OFFGRID_PORT:String(server.address().port)};delete env.ELECTRON_RUN_AS_NODE;
let app;const timeout=setTimeout(()=>failed(Error('Packaged browser task timed out')),60000);
try{
 app=await electron.launch({executablePath:binary,env,timeout:30000});
 const page=await app.firstWindow();page.setDefaultTimeout(15000);await page.waitForURL(/\/ui\//,{timeout:15000});
 console.log('Packaged workspace connected');
 await page.evaluate(()=>{localStorage.setItem('offgrid.locale','en');localStorage.setItem('offgrid.onboarding.complete','true');});
 await page.reload();
 // Navigate through the hydrated application. A same-document goto/reload
 // during initial React mounting can race its initial hash selection.
 await page.getByRole('navigation',{name:'Primary'}).getByRole('link',{name:'Agents',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Agents',exact:true})).toBeVisible();
 // Test-only native-dialog interception validates the scope before granting it.
 await app.evaluate(({dialog})=>{
   dialog.showMessageBox=async(...args)=>{
     const options=args.at(-1);
     if(options.title!=='Allow browser assistance?' || !options.detail.startsWith('demo\nhttp://127.0.0.1:')) throw Error('Unexpected native consent scope');
     if(!options.detail.includes('changes need approval')) throw Error('Missing consent boundary');
     return {response:1,checkboxChecked:false};
   };
 });
 await page.getByRole('checkbox',{name:'Use a browser · Preview'}).check();
 await page.getByRole('button',{name:'Try a practice page',exact:true}).click();
 console.log('Requested local browser session');
 await completion;
 assert.equal(await page.locator('.computer-pair-code').count(),0);
 await page.getByRole('combobox',{name:'Select a paired browser'}).waitFor();
 await page.evaluate(()=>window.scrollTo(0,0));await page.screenshot({path:join(directory,'desktop-browser-ready.png'),fullPage:true});
 await page.locator('.computer-setup').getByRole('button',{name:'Stop browser',exact:true}).click();
 // The owned runtime allows 35 seconds of graceful shutdown plus 5 seconds
 // for its kill acknowledgement. A default 5-second matcher is not that
 // contract; still require confirmed Stop, never accept an error as success.
 await expect(page.getByRole('button',{name:'Try a practice page',exact:true})).toBeEnabled({timeout:45000});
 assert.equal((await page.evaluate(()=>window.electron.getComputerStatus())).state,'stopped');
 assert.deepEqual(replies,['read','write','select','check','save','verify','verify-options']);
 // Starting a second session rechecks the installed manifest after actual use.
 await page.getByRole('button',{name:'Try a practice page',exact:true}).click();
 await page.getByRole('combobox',{name:'Select a paired browser'}).waitFor();
 await page.locator('.computer-setup').getByRole('button',{name:'Stop browser',exact:true}).click();
 await expect(page.getByRole('button',{name:'Try a practice page',exact:true})).toBeEnabled({timeout:45000});
 assert.equal((await page.evaluate(()=>window.electron.getComputerStatus())).state,'stopped');
 console.log(`PASS: packaged UI → native consent → private IPC → bundled Chromium → verified result → local Stop. Evidence: ${directory}`);
}catch(error){
 if(app){const page=await app.firstWindow();await page.screenshot({path:join(directory,'failure.png')}).catch(()=>{});console.error(await page.locator('body').innerText());console.error(await page.evaluate(()=>window.electron?.getComputerStatus?.()).catch(()=>null));}
 throw error;
}finally{
 clearTimeout(timeout);await app?.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));
}
