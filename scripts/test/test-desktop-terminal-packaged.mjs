// Installed Electron main/preload/renderer check, with an isolated profile and
// local HTTP fixture. This never changes OS appearance or the user's workspace.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {realpathSync} from 'node:fs';
import {mkdtemp, mkdir, readFile, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve, sep, extname} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const root = resolve(import.meta.dirname,'../..');
const require = createRequire(join(root, 'apps/web/package.json'));
const {_electron: electron} = require('playwright');
const {expect} = require('@playwright/test');
assert.ok(process.argv[2], 'Pass the packaged desktop executable');
const binary = resolve(process.argv[2]);
const ui = join(root, 'apps/web/dist');
const version = JSON.parse(await readFile(join(root, 'apps/desktop/package.json'), 'utf8')).version;
const uiBuild = createHash('sha256').update((await readFile(join(ui, 'index.html'), 'utf8')).replace(/\r\n/g, '\n')).digest('hex');
// Windows runner TEMP may use an 8.3 alias; PowerShell expands it on entry.
// Compare the same canonical directory identity after Agent exits.
const evidence = realpathSync.native(await mkdtemp(join(tmpdir(), 'offgrid-desktop-theme-')));
await writeFile(join(evidence, 'desktop-presentation.json'), JSON.stringify({locale:'en',theme:'dark'}));
const savedSessions=new Map();let submittedTurns=0;
const server = createServer(async (request, response) => {
  try {
    const pathname = new URL(request.url, 'http://127.0.0.1').pathname;
    if (pathname.startsWith('/ui/')) {
      const file = resolve(ui, pathname.slice(4) || 'index.html');
      assert.ok(file.startsWith(ui + sep));
      const types = {'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.woff2':'font/woff2'};
      response.setHeader('Content-Type', types[extname(file)] || 'application/octet-stream');
      response.end(await readFile(file)); return;
    }
    if(pathname==='/v1/sessions'&&request.method==='POST'){
      let body='';for await(const part of request)body+=part;const value=JSON.parse(body);
      const session={name:value.name,model_id:value.model_id,created_at:new Date().toISOString(),updated_at:new Date().toISOString()};savedSessions.set(value.name,session);
      response.setHeader('Content-Type','application/json');response.end(JSON.stringify(session));return;
    }
    if(pathname.startsWith('/v1/sessions/')){
      const name=decodeURIComponent(pathname.split('/')[3]);if(pathname.endsWith('/generate'))submittedTurns++;
      response.setHeader('Content-Type','application/json');response.end(JSON.stringify(savedSessions.get(name)||{}));return;
    }
    if(/^\/api\/v2\/agent-profiles\/profile-0+[12]$/.test(pathname)) {
      const id=pathname.split('/').at(-1),n=Number(id.slice(-1));
      response.setHeader('Content-Type','application/json');response.end(JSON.stringify({id,revision:1,settings:{name:'Terminal fixture '+n,model:'theme-fixture'}}));return;
    }
    const data = {
      '/api/v2/system': {product:'offgrid',version,api_version:2,workspace_id:'packaged-theme-fixture',ui_build_id:uiBuild,desktop_bridge:{status:'ready',schema_version:1,protocol:1,ui_build_id:uiBuild},capabilities:['sessions-v1','chat-streaming-v1','durable-agent-runs-v1','task-first-agents-v2','agent-profiles-v1','agent-conversations-v1']},
      '/health': {status:'healthy'}, '/v1/users/me': {authenticated:false,auth_required:false,user:null},
      '/v1/models': {data:[{id:'theme-fixture',type:'chat'}]}, '/v1/sessions': {sessions:[]},
      '/v1/catalog': {models:[]}, '/v1/models/download/progress': {},
      '/v1/rag/status': {enabled:false,stats:{}}, '/v1/documents': {documents:[],count:0},
      '/api/v2/models': {models:[]}, '/api/v2/models/packages': {packages:[]},
      '/v1/agents/tasks': [], '/v1/agents/tools': {tools:[],enabled_count:0},
      '/api/v2/jobs': [],
      '/api/v2/agent-profiles': {profiles:[1,2].map(n=>({id:'profile-'+String(n).padStart(32,'0'),revision:1,settings:{name:'Terminal fixture '+n,model:'theme-fixture'}}))},
      '/api/v2/agent-profiles/resources': {tools:{},computer_permitted:false,connection_scope:'workspace-admin'},
      '/v1/agents/mcp': {servers:[]}, '/v1/integrations': {integrations:[]},
      '/api/v2/computer/sessions': {sessions:[]}
    };
    response.setHeader('Content-Type','application/json');
    response.end(JSON.stringify(data[pathname] || {}));
  } catch { response.statusCode = 500; response.end('{}'); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const env = {...process.env,OFFGRID_DESKTOP_HOME:evidence,OFFGRID_PORT:String(server.address().port),APPDATA:join(evidence,'config'),XDG_CONFIG_HOME:join(evidence,'config')};
// Capture the normal styled terminal, even when the test runner disables color
// for its own logs. Plain/no-color behavior has separate CLI contract tests.
env.NO_COLOR='';env.FORCE_COLOR='1';
delete env.ELECTRON_RUN_AS_NODE;
let app;
// Some OS-owned UI blocks Electron's protocol rather than rejecting a call.
// Bound the harness itself and stop only the isolated process it launched.
const deadline = setTimeout(() => {
  console.error('Packaged theme check exceeded its three-minute deadline');
  const child=app?.process();
  if(child && child.exitCode===null){
    try{if(process.platform==='win32')execFileSync('taskkill.exe',['/PID',String(child.pid),'/T','/F'],{windowsHide:true,stdio:'ignore'});else child.kill('SIGKILL');}catch{}
  }
  process.exit(1);
}, 180000);
deadline.unref();
async function launch() {
  console.log('Launching isolated packaged theme check');
  app = await electron.launch({executablePath:binary,env,timeout:30000});
  app.context().setDefaultTimeout(15000);
  app.context().setDefaultNavigationTimeout(15000);
  const page = await app.firstWindow();
  await page.waitForURL("**/ui/**",{timeout:30000});
  // The runner's virtual display may start below the navigation breakpoint.
  // Exercise the expanded-sidebar case at an explicit size, independently of
  // that display. The narrow/200% terminal case is checked separately below.
  await app.evaluate(({BrowserWindow}) => {
    const win=BrowserWindow.getAllWindows()[0];
    win.unmaximize();win.setContentSize(1280,900);win.webContents.setZoomFactor(1);
  });
  await expect.poll(()=>page.evaluate(()=>innerWidth)).toBeGreaterThan(1023);
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete','true'));
  await page.evaluate(() => localStorage.setItem('offgrid.onboarding.complete','true'));
  await page.reload();

  console.log('Packaged theme fixture connected');
  return page;
}

try {
 const page=await launch();
 page.setDefaultTimeout(15000);
 await page.waitForURL('**/ui/**',{timeout:30000});
 // Windows chrome is one quiet, native-control strip, including at browser zoom.
 if(process.platform==='win32') {
   await expect(page.locator('#offgrid-native-chrome')).toHaveCount(1);
   assert.equal(await page.locator('#offgrid-native-chrome').innerText(),'');
   assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].getTitle()),'OffGrid');
   for(const zoom of [1,1.25,2]) {
     await app.evaluate(({BrowserWindow},factor)=>BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(factor),zoom);
     await expect.poll(()=>page.evaluate(()=>Math.round(document.querySelector('#offgrid-native-chrome').getBoundingClientRect().height*devicePixelRatio))).toBeGreaterThan(0);
     await expect.poll(()=>page.evaluate(()=>document.querySelector('main.workspace').getBoundingClientRect().top)).toBeCloseTo(36/zoom,0);
     const fit=await page.evaluate(()=>({height:innerHeight,bottom:document.querySelector('main.workspace').getBoundingClientRect().bottom,color:getComputedStyle(document.querySelector('#offgrid-native-chrome')).backgroundColor,sidebar:getComputedStyle(document.querySelector('.sidebar')).backgroundColor}));
     assert.ok(Math.abs(fit.bottom-fit.height)<2,JSON.stringify(fit));
     assert.equal(fit.color,fit.sidebar);
   }
   await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(1));
   await page.screenshot({path:join(evidence,'native-chrome-dark.png')});
 }
 assert.equal(await page.evaluate(()=>typeof window.terminalDesktop),'undefined');
 await app.evaluate(({dialog},folder)=>{dialog.showOpenDialog=async()=>({canceled:false,filePaths:[folder]});dialog.showMessageBox=async()=>({response:1});},evidence);
 console.log('Opening dock');
 await app.evaluate(({Menu})=>Menu.getApplicationMenu().items.find(x=>x.role==='viewmenu').submenu.items.find(x=>x.label==='Terminal').click());
 await expect.poll(()=>app.context().pages().some(x=>x.url().endsWith('/terminal.html'))).toBe(true);
 const terminal=app.context().pages().find(x=>x.url().endsWith('/terminal.html'));
 terminal.setDefaultTimeout(15000);
 // Navigation can report its URL before open() applies the visible view bounds.
 if(process.platform==='win32') await expect.poll(()=>app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].contentView.children.at(-1).getBounds().y)).toBeGreaterThanOrEqual(36);
 terminal.on('pageerror',error=>console.error('TERMINAL PAGE ERROR',error));
 await expect(terminal.locator('#start-agent')).toBeVisible();
 await terminal.screenshot({path:join(evidence,'terminal-welcome.png')});
 // The sidebar must reveal Chat even when its route is already selected.
 await page.getByRole('link',{name:'Chat',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>document.documentElement.hasAttribute('data-native-terminal-visible'))).toBe(false);
 await page.getByRole('button',{name:'Open terminal',exact:true}).click();
 await page.getByRole('button',{name:'New chat',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>document.documentElement.hasAttribute('data-native-terminal-visible'))).toBe(false);
 await page.getByRole('button',{name:'Open terminal',exact:true}).click();
 if(process.env.OFFGRID_DOC_CAPTURES){
   const demoFolder=join(evidence,'OffGrid Demo');await mkdir(demoFolder);
   await app.evaluate(({dialog},folder)=>{dialog.showOpenDialog=async()=>({canceled:false,filePaths:[folder]});},demoFolder);
 }
 await terminal.locator('#start').click();
 await expect(terminal.getByRole('tab',{name:'OffGrid CLI',exact:true})).toHaveCount(1);
 const input=terminal.locator('.pane:not([hidden]) .xterm-helper-textarea');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('PS ');
 await input.pressSequentially('offgrid --help');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('agent');
 if(process.env.OFFGRID_DOC_CAPTURES){
   await input.pressSequentially("function prompt { 'PS OffGrid Demo> ' }; Clear-Host; offgrid --help");await input.press('Enter');
   await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('PS OffGrid Demo>');
   await expect.poll(()=>terminal.evaluate(()=>{const t=tabs.get(current).term;return t.buffer.active.getLine(t.buffer.active.baseY+t.buffer.active.cursorY)?.translateToString(true)||'';})).toBe('PS OffGrid Demo> ');
   await terminal.screenshot({path:join(root,'build/documentation-captures/desktop-terminal.png')});
   await app.evaluate(({dialog},folder)=>{dialog.showOpenDialog=async()=>({canceled:false,filePaths:[folder]});},evidence);
 }
 await terminal.getByRole('button',{name:'Close OffGrid CLI',exact:true}).click();
 await terminal.getByTitle('New terminal',{exact:true}).click();
 await terminal.locator('#shell').click();
 await expect(terminal.getByRole('tab',{name:'PowerShell'})).toHaveCount(1);
 await input.pressSequentially("[Console]::WriteLine(('CHECK_' + [char]0x65e5 + [char]0x672c + [char]0x8a9e))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('CHECK_\u65e5\u672c\u8a9e');
 await terminal.getByTitle('New terminal',{exact:true}).click();
 await terminal.locator('#shell').click();
 await expect(terminal.getByRole('tab',{name:'PowerShell'})).toHaveCount(2);
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('PS ');
 await input.pressSequentially("[Console]::WriteLine(('SECOND_' + 'TAB'))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('SECOND_TAB');
 await app.evaluate(({clipboard})=>{global.__terminalCopy=[];clipboard.writeText=text=>global.__terminalCopy.push(text);});
 await terminal.getByTitle('Search terminal output').click();await terminal.getByRole('searchbox',{name:'Search terminal output'}).fill('SECOND_TAB');
 await expect(terminal.locator('#query')).toHaveCSS('outline-style','none');
 // Electron rounds a one-CSS-pixel border to device pixels at Windows scaling.
 const searchBorder=await terminal.locator('#query').evaluate(el=>parseFloat(getComputedStyle(el).borderTopWidth));
 assert.ok(searchBorder>=.5&&searchBorder<=1, `Unexpected search boundary: ${searchBorder}`);
 await terminal.emulateMedia({forcedColors:'active'});
 await expect(terminal.locator('#query')).toHaveCSS('outline-width','2px');
 await terminal.emulateMedia({forcedColors:'none'});
 await expect.poll(()=>terminal.evaluate(()=>[...tabs.values()].find(tab=>!tab.pane.hidden).term.getSelection())).toBe('SECOND_TAB');
 await input.focus();await input.press('Control+Shift+C');
 await expect.poll(()=>app.evaluate(()=>global.__terminalCopy)).toEqual(['SECOND_TAB']);
 await terminal.getByRole('button',{name:'Close search',exact:true}).click();
 await input.pressSequentially("[Console]::Write(([char]27)+']52;c;dW50cnVzdGVk'+([char]7)); [Console]::WriteLine(('ESCAPE_'+'DONE'))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('ESCAPE_DONE');
 assert.deepEqual(await app.evaluate(()=>global.__terminalCopy),['SECOND_TAB']);
 // Exercise a prompt+draft exactly one row wide. ConPTY can move the cursor
 // onto an empty next row; that is valid wrapping, not missing input.
 const draftCommand="[Console]::WriteLine(('TAB_'+'DRAFT'))";
 const promptPadding=await terminal.evaluate(length=>tabs.get(current).term.cols-length-5,draftCommand.length);
 assert.ok(promptPadding>0);
 await input.pressSequentially(`function prompt { 'PS '+('x'*${promptPadding})+'> ' }`);await input.press('Enter');
 await expect.poll(()=>terminal.evaluate(()=>{const b=tabs.get(current).term.buffer.active;return b.getLine(b.baseY+b.cursorY)?.translateToString().trim();})).toBe('PS '+'x'.repeat(promptPadding)+'>');
 await terminal.getByTitle('Return to Chat',{exact:true}).click();
 await page.getByRole('button',{name:'Open terminal',exact:true}).click();
 await expect(terminal.getByRole('tab',{name:'PowerShell'})).toHaveCount(2);
 // Existing tabs remain in the DOM while hidden. Wait for the native visibility
 // event to re-enable input and restore focus before sending a single command.
 await expect.poll(()=>terminal.evaluate(()=>visible&&!tabs.get(current).term.options.disableStdin)).toBe(true);
 await expect(input).toBeFocused();
 await expect.poll(()=>terminal.evaluate(()=>{const b=tabs.get(current).term.buffer.active;return b.getLine(b.baseY+b.cursorY)?.translateToString().trim();})).toMatch(/^PS .*>$/);
 await input.pressSequentially(draftCommand);
 const draftSnapshot=()=>terminal.evaluate(()=>{const b=tabs.get(current).term.buffer.active;return {x:b.cursorX,y:b.cursorY,baseY:b.baseY,lines:Array.from({length:3},(_,i)=>b.getLine(b.baseY+b.cursorY-2+i)?.translateToString(true)||'')};});
 await expect.poll(async()=> (await draftSnapshot()).lines.join('')).toContain('PS '+'x'.repeat(promptPadding)+'> '+draftCommand);
 const draftCursor=await draftSnapshot();
 for(let n=0;n<3;n++){await terminal.getByRole('tab',{name:'PowerShell',exact:true}).first().click();await terminal.getByRole('tab',{name:'PowerShell',exact:true}).last().click();}
 await expect(input).toBeFocused();
 await expect.poll(draftSnapshot).toEqual(draftCursor);
 await input.press('Enter');await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('TAB_DRAFT');
 await input.pressSequentially("$p=Start-Process powershell.exe -ArgumentList '-NoProfile','-Command','Start-Sleep -Seconds 300' -WindowStyle Hidden -PassThru; [Console]::WriteLine(('CHILD' + 'PID:' + $p.Id))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText(/CHILDPID:[0-9]+/);
 const output=await terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree').innerText();const pid=Number(output.match(/CHILDPID:([0-9]+)/)[1]);
 await terminal.screenshot({path:join(evidence,'terminal-dark.png')});
 await page.evaluate(()=>window.electron.setPresentation({theme:'light',locale:'en'}));
 await expect(terminal.locator('html')).toHaveAttribute('data-theme','light');
 await terminal.screenshot({path:join(evidence,'terminal-light.png')});
 await terminal.getByRole('button',{name:'Close PowerShell',exact:true}).last().click();
 await expect(terminal.getByRole('tab',{name:'PowerShell'})).toHaveCount(1);
 const {execFileSync}=require('node:child_process');const alive=()=>execFileSync('powershell.exe',['-NoProfile','-NonInteractive','-Command',`if(Get-Process -Id ${pid} -ErrorAction SilentlyContinue){'alive'}else{'gone'}`],{encoding:'utf8',windowsHide:true}).trim();
 assert.equal(alive(),'gone');
 await input.pressSequentially("$p=Start-Process powershell.exe -ArgumentList '-NoProfile','-Command','Start-Sleep -Seconds 300' -WindowStyle Hidden -PassThru; [Console]::WriteLine(('RELOAD' + 'PID:' + $p.Id))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText(/RELOADPID:[0-9]+/);
 const reloadOutput=await terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree').innerText(),reloadPID=Number(reloadOutput.match(/RELOADPID:([0-9]+)/)[1]);
 await terminal.reload();await expect(terminal.locator('#start')).toBeEnabled();
 assert.equal(execFileSync('powershell.exe',['-NoProfile','-NonInteractive','-Command',`if(Get-Process -Id ${reloadPID} -ErrorAction SilentlyContinue){'alive'}else{'gone'}`],{encoding:'utf8',windowsHide:true}).trim(),'gone');
 await expect(terminal.getByRole('tab')).toHaveCount(0);
 await terminal.getByTitle('New terminal',{exact:true}).click();
 await terminal.locator('#shell').click();
 await expect(terminal.getByRole('tab',{name:'PowerShell'})).toHaveCount(1);
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('PS ');

 // Keep unfinished input through Chat/Terminal changes. No new shell or key replay.
 await input.pressSequentially("[Console]::WriteLine(('RETURN_' + 'EXACT'))");
 await input.press('ArrowLeft');
 await terminal.getByTitle('Return to Chat',{exact:true}).click();
 await expect(page.locator('main.workspace')).not.toHaveAttribute('inert');
 await page.getByRole('button',{name:'Open terminal',exact:true}).click();
 await expect(input).toBeFocused();
 await input.press('ArrowRight');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('RETURN_EXACT');
 const geometry=await app.evaluate(({BrowserWindow})=>{const win=BrowserWindow.getAllWindows()[0];return {view:win.contentView.children[0].getBounds(),size:win.getContentSize()};});
 const workspace=await page.locator('main.workspace').boundingBox();
 assert.ok(Math.abs(geometry.view.x-workspace.x)<=1,'terminal covers sidebar');
 assert.equal(geometry.view.y,Math.round(workspace.y));assert.equal(geometry.view.height,geometry.size[1]-geometry.view.y);
 // Web menus must never be cut off by the native terminal's compositing layer.
 const nativeVisible=()=>app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].contentView.children[0].getVisible());
 const menuDraft="[Console]::WriteLine(('MENU_'+'EXACT'))";
 const menuPadding=await terminal.evaluate(length=>tabs.get(current).term.cols-length-3,menuDraft.length);
 assert.ok(menuPadding>0);
 await input.pressSequentially(`function prompt { 'PS '+('x'*${menuPadding})+'> ' }`);await input.press('Enter');
 await expect.poll(()=>terminal.evaluate(()=>{const b=tabs.get(current).term.buffer.active;return b.getLine(b.baseY+b.cursorY)?.translateToString().trim();})).toBe('PS '+'x'.repeat(menuPadding)+'>');
 await input.pressSequentially(menuDraft);await input.press('ArrowLeft');
 await expect.poll(async()=> (await draftSnapshot()).lines.join('')).toContain('PS '+'x'.repeat(menuPadding)+'> '+menuDraft);
 const cursorBeforeMenu=await draftSnapshot();
 const options=page.locator('.workspace-options > button'),optionsPanel=page.locator('.workspace-options-panel');
 await options.click();await expect(optionsPanel).toBeVisible();
 const optionsBox=await optionsPanel.boundingBox(),sidebarBox=await page.locator('.sidebar').boundingBox();
 assert.ok(optionsBox.x+optionsBox.width<=sidebarBox.x+sidebarBox.width,'menu crosses native terminal');
 assert.equal(await nativeVisible(),true);
 await page.screenshot({path:join(evidence,'terminal-sidebar-options.png')});
 await page.keyboard.press('Escape');await expect(options).toBeFocused();
 // Collapsed-sidebar and modal overlays cannot fit beside the view. Suspending
 // its display must retain the same shell/buffer without stealing dismiss focus.
 await page.evaluate(()=>{const panel=document.createElement('div');panel.id='terminal-overlay-fixture';panel.popover='auto';panel.style.cssText='position:fixed;inset:100px auto auto 100px;width:450px;height:200px;background:Canvas';panel.textContent='Overlay fixture';document.body.append(panel);panel.showPopover();});
 await expect.poll(nativeVisible).toBe(false);
 await page.evaluate(()=>document.getElementById('terminal-overlay-fixture').hidePopover());
 await expect.poll(nativeVisible).toBe(true);
 await page.evaluate(()=>document.getElementById('terminal-overlay-fixture').remove());
 assert.deepEqual(await draftSnapshot(),cursorBeforeMenu);
 await input.click();await input.press('ArrowRight');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('MENU_EXACT');
 await input.pressSequentially("[Console]::WriteLine(('SERVICE:' + $env:OFFGRID_SERVER_URL))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('SERVICE:http://127.0.0.1:'+server.address().port);
 await terminal.getByTitle('New terminal',{exact:true}).click();
 await terminal.getByTitle('Open OffGrid Agent and choose a profile').click();
 await expect(terminal.getByRole('tab',{name:'Agent',exact:true})).toHaveCount(1);
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Search or number:',{timeout:15000});
	await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setSize(1440,900));
	await expect.poll(()=>terminal.evaluate(()=>innerWidth)).toBeGreaterThan(1100);
	assert.equal(await terminal.evaluate(()=>{const pane=document.querySelector('.pane:not([hidden])'),surface=pane.querySelector('.terminal-surface');const a=pane.getBoundingClientRect(),b=surface.getBoundingClientRect();return Math.abs(b.left-a.left-24)<2&&Math.abs(b.right-a.right+24)<2;}),true,'terminal content does not use the available pane width');
 await terminal.screenshot({path:join(evidence,'agent-profile-picker.png')});
	await terminal.evaluate(()=>appearance({theme:'dark'}));
	await terminal.screenshot({path:join(evidence,'agent-profile-picker-dark.png')});
	await terminal.evaluate(()=>appearance({theme:'light'}));
 await input.pressSequentially('0');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Choose a listed profile number');
 const closedAgentTab=await terminal.evaluate(()=>current);
 await terminal.getByRole('button',{name:'Close Agent',exact:true}).click();
 await terminal.getByTitle('New terminal',{exact:true}).click();
 await terminal.getByTitle('Open OffGrid Agent and choose a profile').click();
 await expect.poll(()=>terminal.evaluate(()=>current)).not.toBe(closedAgentTab);
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Search or number:');
 await expect(input).toBeFocused();
 await input.pressSequentially('fixture 1');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Search or number: fixture 1');
 await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Ask a question or describe a task.');
	await input.pressSequentially('/');
	await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Tab complete');
	await expect.poll(()=>terminal.evaluate(()=>{
	 const buffer=tabs.get(current).term.buffer.active;
	 for(let y=0;y<buffer.length;y++){
	  const line=buffer.getLine(y);if(!line?.translateToString().includes('> /help'))continue;
	  for(let x=0;x<line.length;x++)if(line.getCell(x)?.isInverse())return true;
	 }
	 return false;
	})).toBe(true);
	await terminal.screenshot({path:join(evidence,'agent-slash-menu.png')});
	await input.press('Escape');
	await input.pressSequentially('help');await input.press('Enter');
	await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('All commands and recovery controls');
 await input.pressSequentially('/clear');await input.press('Enter');
 await expect.poll(()=>terminal.evaluate(()=>{const t=tabs.get(current).term;return Array.from({length:t.buffer.active.length},(_,i)=>t.buffer.active.getLine(i)?.translateToString()).join('\n');})).not.toContain('Search or number:');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Terminal fixture 1');
 await input.pressSequentially('/ses');
 await terminal.getByRole('tab',{name:'PowerShell',exact:true}).click();
 await terminal.getByRole('tab',{name:'Agent',exact:true}).click();
 await expect(input).toBeFocused();await input.pressSequentially('sion');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Current session');
 assert.equal(savedSessions.size,1);assert.equal(submittedTurns,0);
 await input.pressSequentially('/default');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Remembered Terminal fixture 1');
 await input.pressSequentially('/agent');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Search or number:');
 await input.press('Escape');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Continuing this conversation');
 assert.equal(savedSessions.size,1);
 await input.pressSequentially('/agent');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Search or number:');
 await input.press('ArrowDown');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Starting a separate conversation');
 await expect.poll(()=>savedSessions.size).toBe(2);assert.equal(submittedTurns,0);
 await input.pressSequentially('/default clear');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Default forgotten');
 await terminal.screenshot({path:join(evidence,'agent-profile-continuity.png')});
 await input.pressSequentially('/exit');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Conversation saved');
 await expect.poll(()=>terminal.evaluate(()=>{const b=tabs.get(current).term.buffer.active;return b.getLine(b.baseY+b.cursorY)?.translateToString().trim();})).toMatch(/^PS .*>$/);
 await input.pressSequentially("[Console]::WriteLine(('AFTER_'+'AGENT:'+(Get-Location).Path))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('AFTER_AGENT:'+evidence);
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).not.toContainText('[Process exited:');
 await input.pressSequentially('offgrid --help');await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('Start here');
 await terminal.screenshot({path:join(evidence,'agent-returned-to-shell.png')});
 await terminal.getByRole('button',{name:'Close Agent',exact:true}).click();
	await expect(input).toBeFocused();

 await input.pressSequentially("$p=Start-Process powershell.exe -ArgumentList '-NoProfile','-Command','Start-Sleep -Seconds 300' -WindowStyle Hidden -PassThru; [Console]::WriteLine(('QUIT' + 'PID:' + $p.Id))");await input.press('Enter');
 await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText(/QUITPID:[0-9]+/);
 const quitOutput=await terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree').innerText(),quitPID=Number(quitOutput.match(/QUITPID:([0-9]+)/)[1]);
 await app.evaluate(({webContents,BrowserWindow})=>{BrowserWindow.getAllWindows()[0].setSize(960,720);webContents.getAllWebContents().find(w=>w.getURL().endsWith('/terminal.html')).setZoomFactor(2);});
 await expect(terminal.locator('#resize')).toHaveCount(0);
 await expect.poll(()=>terminal.evaluate(()=>Array.from(document.querySelectorAll('.tools button')).every(button=>{const r=button.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth;}))).toBe(true);
 assert.equal(await terminal.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
 const capture=await app.evaluate(async({webContents})=>(await webContents.getAllWebContents().find(w=>w.getURL().endsWith('/terminal.html')).capturePage()).toPNG().toString('base64'));
 await writeFile(join(evidence,'terminal-narrow-200.png'),Buffer.from(capture,'base64'));
 await app.close();app=undefined;
 assert.equal(execFileSync('powershell.exe',['-NoProfile','-NonInteractive','-Command',`if(Get-Process -Id ${quitPID} -ErrorAction SilentlyContinue){'alive'}else{'gone'}`],{encoding:'utf8',windowsHide:true}).trim(),'gone');
 console.log(JSON.stringify({passed:true,evidence,reloadChild:true,search:true,copy:true,oscClipboardBlocked:true,cliInteractive:true,agentClear:true,agentDraft:true,cursorPreserved:true,mainWorkspace:true,zoom200:true,tabs:true,unicode:true,hiddenRetained:true,closeChild:true,quitChild:true,serviceIdentity:true}));
}finally{await app?.close();server.close();clearTimeout(deadline);}
