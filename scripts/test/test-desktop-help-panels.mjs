// Hidden Help and nested workspace-panel qualification. HTTP fixture hosts only;
// no permission grant, command, terminal, service write or personal profile access.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,readFile,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve,join,extname,sep} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {dismissWorkspaceSetup} from './desktop-onboarding.mjs';
const root=resolve(import.meta.dirname,'../..'),require=createRequire(join(root,'apps/web/package.json'));
const {_electron:electron,expect}=require('@playwright/test');
const binary=resolve(process.argv[2]),profile=await mkdtemp(join(tmpdir(),'offgrid-help-panels-'));
const ui=resolve(binary,'../resources/ui'),writes=[],checks=[];
const uiBuild=createHash('sha256').update((await readFile(join(ui,'index.html'),'utf8')).replace(/\r\n/g,'\n')).digest('hex');
await writeFile(join(profile,'desktop-presentation.json'),JSON.stringify({locale:'en',theme:'light',appearance:{light:'slate',dark:'offgrid'}}));
const server=createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://localhost'),p=url.pathname;
  if(req.method!=='GET')writes.push(p);
  if(p.startsWith('/ui/')){const file=resolve(ui,p.slice(4)||'index.html');assert.ok(file.startsWith(ui+sep));res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.woff2':'font/woff2'})[extname(file)]||'application/octet-stream');res.end(await readFile(file));return;}
  const data={'/api/v2/system':{product:'offgrid',version:'0.4.32',api_version:2,workspace_id:'help-panels-fixture',ui_build_id:uiBuild,desktop_bridge:{status:'ready',schema_version:1,protocol:1,ui_build_id:uiBuild},capabilities:['sessions-v1','chat-streaming-v1']},'/health':{status:'healthy'},'/v1/users/me':{authenticated:false,auth_required:false,user:null},'/v1/models':{data:[]},'/v1/sessions':{sessions:[]},'/v1/catalog':{models:[]},'/api/v2/models':{models:[]},'/api/v2/models/packages':{packages:[]},'/v1/agents/tasks':[], '/v1/documents':{documents:[],count:0},'/v1/rag/status':{enabled:false,stats:{}}};
  data['/v1/system/config']={version:'0.4.32',require_auth:false};
  data['/api/v2/system'].capabilities.push('durable-agent-runs-v1','native-execution-preview-v1','conversation-input-admission-v1');
  data['/api/v2/execution/workspaces']={protocol:1,workspaces:[{reference:{host:'TEST-HOST',session:'fixture-session',grant:'fixture-grant',revision:1},backend:'local',root:'C:/Disposable/example',directory:'.',permissions:{read:true,write:true,processes:false}}]};
  res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data[p]||{}));
 }catch{res.writeHead(500).end('{}');}
});
await new Promise(r=>server.listen(0,'127.0.0.1',r));
const env={...process.env,OFFGRID_DESKTOP_HOME:profile,OFFGRID_DESKTOP_TEST_HIDDEN:'1',OFFGRID_PORT:String(server.address().port)};delete env.ELECTRON_RUN_AS_NODE;
let app;
const cancelDialog=async page=>{try{await page.keyboard.press('Escape');}catch(error){if(!page.isClosed())throw error;}await expect.poll(()=>page.isClosed()).toBe(true);};
const deadline=setTimeout(()=>{console.error('Dialog check deadline',profile);app?.process().kill();process.exit(1);},180000);deadline.unref();
const capture=async(page,name)=>{
 await page.evaluate(()=>document.fonts.ready);
 await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
 const id=await page.evaluate(()=>document.title);
 await app.evaluate(async({webContents},title)=>{const wc=webContents.getAllWebContents().find(w=>w.getTitle()===title);await wc.capturePage(undefined,{stayHidden:true,stayAwake:true});},id);
 await new Promise(r=>setTimeout(r,180));
 const png=await app.evaluate(async({webContents},title)=>(await webContents.getAllWebContents().find(w=>w.getTitle()===title).capturePage(undefined,{stayHidden:true,stayAwake:true})).toPNG().toString('base64'),id);
 await writeFile(join(profile,name+'.png'),Buffer.from(png,'base64'));
};
try{
 app=await electron.launch({executablePath:binary,env,timeout:30000});
 const page=await app.firstWindow();app.context().setDefaultTimeout(15000);
 await app.evaluate(({BrowserWindow})=>{const win=BrowserWindow.getAllWindows()[0];win.show=()=>{};win.focus=()=>{};win.webContents.setBackgroundThrottling(false);});
 await page.waitForURL(/\/ui\//);await dismissWorkspaceSetup(page,'help-panels-fixture');
 const draft='Keep my draft while browsing Help and workspace access';
 await page.locator('.composer textarea').fill(draft);
 for(const [locale,theme,zoom,width,height]of [['en','light',1,754,560],['en','dark',2,1280,900],['ar','dark',2,1280,900]]){
  await app.evaluate(({BrowserWindow},{zoom,width,height})=>{const win=BrowserWindow.getAllWindows()[0];win.setContentSize(width,height);win.webContents.setZoomFactor(zoom);},{zoom,width,height});
  await page.evaluate(()=>{location.hash='#/settings/appearance';});
  await page.locator('#setting-theme button').filter({has:page.locator('.mode-preview-'+theme)}).click();
  await page.evaluate(()=>{location.hash='#/settings/general';});await page.locator('#settings-language').selectOption(locale);
  await page.evaluate(()=>{location.hash='#/home';});await expect(page.locator('.composer textarea')).toHaveValue(draft);
  await expect(page.locator('html')).toHaveAttribute('lang',locale);await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
  await page.locator('.help-trigger').click();
  const help=page.locator('.workspace-help'),heading=help.locator('.help-heading'),scroll=help.locator('.help-scroll');
  await expect(help.getByRole('searchbox')).toBeFocused();
  assert.ok((await help.locator('.help-search').boundingBox()).width<=421,'Help search should stay compact in a wide content area');
  await capture(page,`help-open-${locale}-${theme}-${zoom}`);
  await scroll.evaluate(el=>{el.scrollTop=180;});
  const before=await heading.boundingBox();
  const body=await scroll.boundingBox();assert.ok(body.y>=before.y+before.height-1,'Content must be clipped below the fixed heading');
  await expect(heading.getByRole('button')).toBeInViewport();
  await capture(page,`help-scrolled-${locale}-${theme}-${zoom}`);
  await help.locator('.help-topics button').first().click();await expect(help.locator('.help-article')).toBeFocused();
  assert.ok((await help.locator('.help-article h3').boundingBox()).y>=before.y+before.height-1);
  assert.equal(await page.evaluate(()=>scrollY),0);
  await capture(page,`help-topic-${locale}-${theme}-${zoom}`);
  await page.keyboard.press('Escape');await expect(help).toHaveCount(0);await expect(page.locator('.help-trigger')).toBeFocused();
  await expect(page.locator('.composer textarea')).toHaveValue(draft);
  await page.locator('.composer-attachments-menu > button').click();
  const workspace=page.locator('.chat-execution-workspace');await workspace.locator(':scope > button').click();
  const panel=page.locator('.execution-workspace-panel');await expect(panel).toBeVisible();
  await expect(panel.getByRole('radio',{name:/TEST-HOST/})).toBeVisible();
  const title=panel.locator('.execution-workspace-heading strong');
  if(locale==='en')assert.equal(await title.evaluate(el=>{const range=document.createRange();range.selectNodeContents(el);return new Set([...range.getClientRects()].map(r=>Math.round(r.top))).size;}),1);
  await panel.evaluate(el=>{el.scrollTop=0;});await capture(page,`workspace-overview-${locale}-${theme}-${zoom}`);
  await panel.locator('.execution-workspace-heading button').focus();await page.keyboard.press('Tab');
  const radio=panel.getByRole('radio').first();await expect(radio).toBeFocused();
  const focus=await panel.locator('.profile-option').first().evaluate(el=>{const style=getComputedStyle(el);return {outline:style.outlineStyle,width:style.outlineWidth,radius:style.borderRadius,visible:el.querySelector('input').matches(':focus-visible')};});
  await capture(page,`workspace-${locale}-${theme}-${zoom}`);
  assert.ok(focus.outline==='solid'&&parseFloat(focus.width)>0&&parseFloat(focus.radius)>=8,JSON.stringify(focus));
  assert.ok((await panel.locator('.profile-option').first().boundingBox()).y>=(await panel.boundingBox()).y+3,'Keyboard focus must leave the complete rounded row visible');
  assert.ok(await panel.evaluate(el=>el.scrollWidth<=el.clientWidth));
  await page.keyboard.press('Escape');await page.keyboard.press('Escape');await expect(panel).not.toBeVisible();
  await expect(page.locator('.composer textarea')).toHaveValue(draft);
  await page.evaluate(()=>{location.hash='#/settings/appearance';});
  const themeLabel=await page.locator('#setting-theme h3').innerText();
  await page.locator('.settings-search input').fill(locale==='en'?'he':themeLabel);
  const results=page.locator('.settings-results');await expect(results).toBeVisible();await expect(page.locator('.settings-groups')).toBeHidden();
  await expect(page.locator('.settings-search input')).toBeInViewport();
  const searchGeometry=await page.locator('.settings-search input').evaluate(el=>({top:el.getBoundingClientRect().top,chrome:document.querySelector('#offgrid-native-chrome')?.getBoundingClientRect().bottom??0}));
  assert.ok(searchGeometry.top>=searchGeometry.chrome,JSON.stringify(searchGeometry));
  const themeResult=results.locator('a[href="#/settings/theme"]');await expect(themeResult).toHaveCSS('text-decoration-line','none');
  await capture(page,`settings-search-${locale}-${theme}-${zoom}`);
  await themeResult.focus();await page.keyboard.press('Enter');await expect(page.locator('#setting-theme button[aria-pressed="true"]')).toBeFocused();
  await expect(page.locator('.settings-search input')).toHaveValue('');
  await page.evaluate(()=>{location.hash='#/home';});await expect(page.locator('.composer textarea')).toHaveValue(draft);
  checks.push(`${locale}/${theme}/${zoom*100}% native zoom: Help/search, article focus, workspace panel, settings result navigation and retained draft`);
 }
 assert.deepEqual(writes,[]);
 await writeFile(join(profile,'receipt.json'),JSON.stringify({status:'passed',binary,profile,uiBuild,checks,serviceWrites:writes,hostGrants:0,nativeTerminalSessions:0,personalData:false,physicalOSInput:false},null,2));
 console.log(JSON.stringify({status:'passed',profile,checks}));
}catch(error){console.error('Evidence profile:',profile);for(const page of app?.context().pages()??[])console.error(page.url(),await page.locator('body').innerText().catch(()=>''));throw error;}
finally{clearTimeout(deadline);await app?.close();await new Promise(r=>server.close(r));}
