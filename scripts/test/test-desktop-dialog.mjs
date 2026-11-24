// Hidden Electron qualification. Disposable profile and folder; only a synthetic
// PowerShell output command is run. No user's sessions, service writes or models.
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
const binary=resolve(process.argv[2]),profile=await mkdtemp(join(tmpdir(),'offgrid-dialog-check-'));
const ui=join(root,'apps/web/dist'),writes=[],checks=[];
const uiBuild=createHash('sha256').update((await readFile(join(ui,'index.html'),'utf8')).replace(/\r\n/g,'\n')).digest('hex');
await writeFile(join(profile,'desktop-presentation.json'),JSON.stringify({locale:'en',theme:'light',appearance:{light:'slate',dark:'offgrid'}}));
const server=createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://localhost'),p=url.pathname;
  if(req.method!=='GET')writes.push(p);
  if(p.startsWith('/ui/')){const file=resolve(ui,p.slice(4)||'index.html');assert.ok(file.startsWith(ui+sep));res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.woff2':'font/woff2'})[extname(file)]||'application/octet-stream');res.end(await readFile(file));return;}
  const data={'/api/v2/system':{product:'offgrid',version:'0.4.32',api_version:2,workspace_id:'dialog-fixture',ui_build_id:uiBuild,desktop_bridge:{status:'ready',schema_version:1,protocol:1,ui_build_id:uiBuild},capabilities:['sessions-v1','chat-streaming-v1']},'/health':{status:'healthy'},'/v1/users/me':{authenticated:false,auth_required:false,user:null},'/v1/models':{data:[]},'/v1/sessions':{sessions:[]},'/v1/catalog':{models:[]},'/api/v2/models':{models:[]},'/api/v2/models/packages':{packages:[]},'/v1/agents/tasks':[], '/v1/documents':{documents:[],count:0},'/v1/rag/status':{enabled:false,stats:{}}};
  data['/v1/system/config']={version:'0.4.32',require_auth:false};
  data['/api/v2/system'].capabilities.push('durable-agent-runs-v1');
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
 const page=await app.firstWindow();app.context().setDefaultTimeout(12000);
 await app.evaluate(({BrowserWindow})=>{
  const parent=BrowserWindow.getAllWindows()[0];parent.setContentSize(1380,480);parent.webContents.setBackgroundThrottling(false);
  BrowserWindow.prototype.show=function(){this.__testShown=true;};BrowserWindow.prototype.focus=function(){};
 });
 await page.waitForURL(/\/ui\//);await dismissWorkspaceSetup(page,'dialog-fixture');
 await page.goto(`http://127.0.0.1:${server.address().port}/ui/#/settings/appearance`);
 await expect(page.locator('#setting-theme')).toBeVisible();
 await page.evaluate(()=>scrollTo(0,200));
 const position=await page.evaluate(()=>scrollY);assert.ok(position>0,'Use a genuinely scrollable underlying Settings page');
 await app.evaluate(({Menu,dialog},folder)=>{dialog.showOpenDialog=async()=>({canceled:false,filePaths:[folder]});Menu.getApplicationMenu().items.flatMap(x=>x.submenu?.items??[]).find(x=>x.accelerator==='CommandOrControl+`').click();},profile);
 await expect.poll(()=>app.context().pages().some(p=>p.url().endsWith('/terminal.html'))).toBe(true);
 const terminal=app.context().pages().find(p=>p.url().endsWith('/terminal.html'));
 await expect(page.locator('html')).toHaveAttribute('data-native-terminal-visible','');
 assert.equal(await page.locator('html').evaluate(el=>getComputedStyle(el).overflowY),'hidden');
 await page.mouse.move(10,300);await page.mouse.wheel(0,600);await new Promise(r=>setTimeout(r,100));
 assert.equal(await page.evaluate(()=>scrollY),position,'Covered route must not scroll');
 await terminal.locator('#start').click();
 const input=terminal.locator('.pane:not([hidden]) .xterm-helper-textarea');
 await expect(terminal.locator('.xterm-accessibility-tree')).toContainText('PS ',{timeout:15000});
 await input.pressSequentially("1..100 | ForEach-Object { Write-Output ('DIALOG_SCROLL_' + $_) }");await input.press('Enter');
 await expect(terminal.locator('.xterm-accessibility-tree')).toContainText('DIALOG_SCROLL_100');
 assert.ok(await terminal.evaluate(()=>tabs.get(current).term.buffer.active.baseY>0),'Terminal retains scrollback');
 await expect.poll(()=>terminal.evaluate(()=>tabs.get(current).commands.commands.at(-1)?.finished)).toBe(true);
 await terminal.locator('#chat').click();await expect(page.locator('html')).not.toHaveAttribute('data-native-terminal-visible','');
 assert.equal(await page.evaluate(()=>scrollY),position,'Returning at the same geometry preserves the page position');
 await app.evaluate(({Menu})=>Menu.getApplicationMenu().items.flatMap(x=>x.submenu?.items??[]).find(x=>x.accelerator==='CommandOrControl+`').click());
 await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setContentSize(1380,800));
 const terminalCatalog=require(join(root,'apps/desktop/theme-palettes.json'));
 for(const [locale,theme,palette,zoom]of [['en','light','slate',1],['ar','dark','offgrid',2]]){
  await app.evaluate(({webContents},{locale,theme,palette,zoom})=>{const wc=webContents.getAllWebContents().find(w=>w.getURL().endsWith('/terminal.html'));wc.send('terminal-event',{type:'appearance',locale,theme,palette,reduceMotion:true});wc.setZoomFactor(zoom);},{locale,theme,palette:{...terminalCatalog.themes[palette][theme],ansi:terminalCatalog.terminal[palette]?.[theme]},zoom});
  await terminal.locator('#commands').click();
  await expect(terminal.locator('#context-command')).toHaveCSS('direction','ltr');
  await expect(terminal.locator('#context-output')).toHaveValue(/DIALOG_SCROLL_100/);
  await terminal.locator('#context-output').scrollIntoViewIfNeeded();
  const outputGeometry=await terminal.locator('#context-output').evaluate(el=>({height:el.getBoundingClientRect().height,scroll:el.scrollHeight,client:el.clientHeight,viewport:innerHeight}));
  assert.ok(outputGeometry.height>=Math.min(96,outputGeometry.viewport*.36)-2,JSON.stringify(outputGeometry));
  assert.ok(outputGeometry.scroll>outputGeometry.client,'Long output retains its own scrollable preview');
  if(zoom===1)assert.ok(await terminal.locator('#terminal-commands').evaluate(el=>el.scrollHeight<=el.clientHeight+2),'At ordinary size only the output needs a scrollbar');
  await capture(terminal,`context-output-${locale}-${zoom}`);
  await terminal.locator('#context-question').fill('Explain this output');
  console.log('Question typography',await terminal.locator('#context-question').evaluate(el=>({family:getComputedStyle(el).fontFamily,size:getComputedStyle(el).fontSize})));
  await terminal.locator('#context-share').scrollIntoViewIfNeeded();
  await expect(terminal.locator('#context-share')).toBeEnabled();
  await capture(terminal,`context-question-${locale}-${zoom}`);
  await terminal.keyboard.press('Escape');
  checks.push(`native command context ${locale}/${theme}/${zoom*100}%: output edit area and reachable question/actions`);
 }
 await app.evaluate(({webContents})=>{const wc=webContents.getAllWebContents().find(w=>w.getURL().endsWith('/terminal.html'));wc.setZoomFactor(1);wc.send('terminal-event',{type:'appearance',theme:'light',locale:'en'});});
 await terminal.getByRole('button',{name:'Close OffGrid CLI',exact:true}).click();
 await expect.poll(()=>app.context().pages().some(p=>p.url().endsWith('/app-dialog.html'))).toBe(true);
 let dialog=app.context().pages().find(p=>p.url().endsWith('/app-dialog.html'));
 await expect(dialog.locator('#actions button').first()).toBeFocused();
 const size=await dialog.evaluate(()=>({height:innerHeight,body:document.querySelector('.body').getBoundingClientRect(),actions:document.querySelector('footer').getBoundingClientRect(),button:getComputedStyle(document.querySelector('.confirm')).backgroundColor,text:getComputedStyle(document.body).color}));
 assert.ok(size.height<240,JSON.stringify(size));assert.equal(size.button,size.text);
 assert.ok(size.actions.top-size.body.bottom<=18,'No large empty gulf before actions');
 await capture(dialog,'terminal-close-light');await cancelDialog(dialog);
 await expect.poll(()=>dialog.isClosed()).toBe(true);await expect(input).toBeVisible();
 await terminal.locator('#chat').click();await expect(page.locator('html')).not.toHaveAttribute('data-native-terminal-visible','');
 await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setContentSize(1380,480));
 // Height changes can clamp the route's old position, but a terminal toggle at
 // stable geometry was checked above and must not introduce a page scrollbar.
 assert.ok(await page.evaluate(()=>getComputedStyle(document.documentElement).overflowY!=='hidden'));
 checks.push('real disposable terminal: scrollback, cancel, outer-scroll containment and page return');
 // Exercise the installed host-owned dialog directly with long paths and translated
 // actions. No confirmation invokes a terminal or service action in this matrix.
 const catalog=require(join(root,'apps/desktop/theme-palettes.json')),copy=require(join(root,'apps/desktop/terminal-translations.js'));
 for(const [locale,theme,palette,zoom,long] of [['en','light','slate',1,false],['en','dark','offgrid',1,false],['ar','dark','offgrid',2,true],['de','light','slate',2,true]]){
  const options={title:copy.text(locale,'Close terminal?'),message:copy.text(locale,'Stop this terminal and its running commands?'),detail:long?'C:\\'+('long folder name\\'.repeat(35)):profile,buttons:[copy.text(locale,'Cancel'),copy.text(locale,'Stop and close')],defaultId:0,cancelId:0};
  await app.evaluate(({app,BrowserWindow,ipcMain,screen},{options,appearance})=>{
   const host=process.mainModule.require(app.getAppPath()+'/app-dialog.js');
   globalThis.__dialogResult=null;host.showAppDialog({BrowserWindow,ipcMain,screen},BrowserWindow.getAllWindows()[0],options,()=>appearance).then(result=>{globalThis.__dialogResult=result;});
  },{options,appearance:{theme,locale,direction:locale==='ar'?'rtl':'ltr',palette:catalog.themes[palette][theme]}});
  await expect.poll(()=>app.context().pages().some(p=>p.url().endsWith('/app-dialog.html'))).toBe(true);
  dialog=app.context().pages().find(p=>p.url().endsWith('/app-dialog.html'));
  await expect(dialog.locator('#actions button').first()).toBeFocused();
  await app.evaluate(({BrowserWindow},zoom)=>BrowserWindow.getAllWindows().find(w=>w.webContents.getURL().endsWith('/app-dialog.html')).webContents.setZoomFactor(zoom),zoom);
  await expect.poll(()=>dialog.evaluate(()=>Math.ceil(document.querySelector('footer').getBoundingClientRect().bottom)<=innerHeight)).toBe(true);
  assert.ok(await dialog.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await capture(dialog,`dialog-${locale}-${theme}-${zoom}`);
  await dialog.keyboard.press('Shift+Tab');await expect(dialog.locator('#close')).toBeFocused();
  await dialog.keyboard.press('Shift+Tab');await expect(dialog.locator('.confirm')).toBeFocused();
  await cancelDialog(dialog);await expect.poll(()=>app.evaluate(()=>globalThis.__dialogResult?.response)).toBe(0);
  checks.push(`${locale}/${theme}/${palette}/${zoom*100}% native zoom: fit, keyboard, cancel`);
 }
 // Stop only the test-created terminal by its ordinary explicit confirmation.
 await app.evaluate(({Menu})=>Menu.getApplicationMenu().items.flatMap(x=>x.submenu?.items??[]).find(x=>x.accelerator==='CommandOrControl+`').click());
 await terminal.getByRole('button',{name:'Close OffGrid CLI',exact:true}).click();
 await expect.poll(()=>app.context().pages().some(p=>p.url().endsWith('/app-dialog.html'))).toBe(true);
 dialog=app.context().pages().find(p=>p.url().endsWith('/app-dialog.html'));await dialog.locator('.confirm').click();
 await expect(terminal.getByRole('tab',{name:'OffGrid CLI',exact:true})).toHaveCount(0);
 assert.deepEqual(writes,[]);
 await writeFile(join(profile,'receipt.json'),JSON.stringify({status:'passed',binary,profile,uiBuild,checks,serviceWrites:writes,physicalOSInput:false,personalData:false},null,2));
 console.log(JSON.stringify({status:'passed',profile,checks}));
}catch(error){console.error('Evidence profile:',profile);for(const page of app?.context().pages()??[])console.error(page.url(),await page.locator('body').innerText().catch(()=>''));throw error;}
finally{clearTimeout(deadline);await app?.close();await new Promise(r=>server.close(r));}
