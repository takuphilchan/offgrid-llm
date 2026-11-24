const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage, shell, nativeTheme, screen, utilityProcess, powerMonitor, safeStorage, globalShortcut } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { isTrustedPage, isTrustedSender, fingerprintUI, workspaceChannelAllowed } = require('./backend');
const { DesktopRuntime } = require('./runtime');
const { attachmentStore } = require('./attachment');
const { normalize: normalizePresentation, readCopy, languages, catalog: themeCatalog } = require('./presentation');
const {localizedCopy,overrides:localeOverrides}=require('./locale-messages');
const { createTrayFeedback } = require('./tray-feedback');
const { ComputerRuntime, parseComputerLink } = require('./computer-runtime');
const { computerPrincipal } = require('./computer-browser-control');
const { createApplicationLauncher } = require('./application-launcher');
const { ExecutionRuntime } = require('./execution-runtime');
const { FolderTrust } = require('./folder-trust');
const { folderTrustCopy } = require('./folder-trust-copy');
const { spawn } = require('node:child_process');
// Verification code belongs to the application, not the unverified payload.
const { verifyPack: verifyComputerPack } = require(app.isPackaged ? './computer-pack/pack.cjs' : '../computer/pack.cjs');

const APP_NAME = require('./branding.cjs').applyBranding(app);
// Test packages must remain isolated even when an elevated installer launches
// through Explorer, which does not inherit the installer's environment.
const installerTest = require('./package.json').name === 'offgrid-desktop-install-test';
const testArgument = name => installerTest ? process.argv.find(arg => arg.startsWith(name + '='))?.slice(name.length + 1) : undefined;
const customHome = testArgument('--offgrid-test-profile') || process.env.OFFGRID_DESKTOP_HOME;
if (installerTest && !customHome) throw new Error('Installer test package requires an isolated profile');
if (customHome && !path.isAbsolute(customHome)) throw new Error('OFFGRID_DESKTOP_HOME must be an absolute path');
const configRoot = customHome || path.join(app.getPath('home'), '.offgrid-llm');
// Explicit alternate profiles allow isolated qualification without attaching to
// or changing the installed desktop application's cookies, settings, or data.
if (customHome) app.setPath('userData', path.join(configRoot, 'electron'));
const quitForInstall = process.argv.includes('--offgrid-quit-for-install');
// A second process requests a normal, bounded app shutdown. If no desktop is
// running, this command exits without creating a window or starting a service.
if (!app.requestSingleInstanceLock({ quitForInstall }) || quitForInstall) app.exit(0);

const portValue = testArgument('--offgrid-test-port') || process.env.OFFGRID_PORT || '11611';
const port = /^\d+$/.test(portValue) && Number(portValue) > 0 && Number(portValue) <= 65535 ? Number(portValue) : 11611;
const LOADING_URL = pathToFileURL(path.join(__dirname, 'loading.html')).href;
const uiDir = app.isPackaged ? path.join(process.resourcesPath, 'ui') : path.join(__dirname, '../web/dist');
const binaryRoot = app.isPackaged ? path.join(process.resourcesPath, 'bin') : path.join(__dirname, '../../build', { win32: 'windows', darwin: 'macos', linux: 'linux' }[process.platform]);
const binaryName = process.platform === 'win32' ? 'offgrid.exe' : process.platform === 'darwin' ? 'offgrid-' + (process.arch === 'arm64' ? 'arm64' : 'amd64') : 'offgrid';
const workspace = root => ({ config: root, models: path.join(root, 'models'), data: path.join(root, 'data') });
const runtime = new DesktopRuntime({
  ownedStdio: true,
  url: 'http://127.0.0.1:' + port, version: app.getVersion(), binary: path.join(binaryRoot, binaryName), uiDir,
  workspace: workspace(configRoot), isolatedWorkspace: workspace(path.join(configRoot, 'desktop-workspace')),
  attachments: attachmentStore(configRoot),
  onIdentityChange: async () => {
    uploadGrants.clear();
    await execution.stop();
    if(execution.child||execution.pending||execution.state==='uncertain')throw Error('Execution access could not be stopped for the changed workspace');
    await computer.stop();
    if (computer.child || computer.pending || computer.stopping) throw Error('Computer access could not be stopped for the changed workspace');
  },
  uiBuildID: fs.existsSync(path.join(uiDir, 'index.html')) ? fingerprintUI(fs.readFileSync(path.join(uiDir, 'index.html'))) : null
});
let mainWindow = null;
let terminal = null;
let workspaceNavigationPending = false;
let tray = null;
let quitting = false;
let shutdownComplete = false;
let saveTimer;
let saveQueue = Promise.resolve();
let windowCreation;
let computerRequested = process.argv.map(parseComputerLink).find(Boolean);
const computerRoot = app.isPackaged ? path.join(process.resourcesPath,'computer') : path.join(__dirname,'../../build/computer-runtime',`${{win32:'win',darwin:'mac',linux:'linux'}[process.platform]}-${process.arch}`);
let computerCopy;
const computerText = () => localizedCopy(computerCopy??{},presentation.locale,'i18n/computer-experience:copy');
const computer = new ComputerRuntime({
  root: computerRoot, directory:path.join(configRoot,'computer'), fork:(...args)=>utilityProcess.fork(...args),
  browserWorkspaceRoots:()=>{
    if(execution.pending||execution.child&&!execution.resource||execution.state==='uncertain')throw Error('execution_scope_unavailable');
    return execution.resource?[execution.resource.root]:[];
  },
  service:()=> { if(runtime.state.state!=='ready'||runtime.maintenanceAdmissionHold) throw Error('service_unavailable'); return runtime.url; },
  identity: async url => {
    const response=await fetch(url+'/api/v2/system',{redirect:'error',signal:AbortSignal.timeout(5000)});
    if(!response.ok) throw Error('service_unavailable'); return response.json();
  },
  verify: async root => { try { await verifyComputerPack(root); } catch { throw Error('pack_invalid'); } },
  pairing: async (service,request={}) => {
    if (!mainWindow || mainWindow.isDestroyed() || !isTrustedPage(mainWindow.webContents.getURL(),service,LOADING_URL)) throw Error('pairing_failed');
    // Use the authenticated desktop session without extracting cookies or
    // disclosing enrollment credentials to renderer storage/IPC arguments.
    const response=await mainWindow.webContents.session.fetch(service+'/api/v2/computer/pairing',{
      method:'POST',credentials:'include',redirect:'error',headers:{'Content-Type':'application/json',Origin:service},body:JSON.stringify(request),signal:AbortSignal.timeout(10000)});
    if(!response.ok) throw Error('pairing_failed');return response.json();
  },
  task: async (service,id) => {
    if(!mainWindow||mainWindow.isDestroyed()||!isTrustedPage(mainWindow.webContents.getURL(),service,LOADING_URL))throw Error('computer_scope_violation');
    const response=await mainWindow.webContents.session.fetch(service+'/api/v2/jobs/'+encodeURIComponent(id),{credentials:'include',redirect:'error',signal:AbortSignal.timeout(10000)});
    if(!response.ok)throw Error('computer_scope_violation');return response.json();
  },
  principal: async service => {
    if(!mainWindow||mainWindow.isDestroyed()||!isTrustedPage(mainWindow.webContents.getURL(),service,LOADING_URL))throw Error('computer_scope_violation');
    const response=await mainWindow.webContents.session.fetch(service+'/v1/users/me',{credentials:'include',redirect:'error',signal:AbortSignal.timeout(5000)});
    return computerPrincipal(response);
  },
  confirm: async (origin, service, networkMode, approvalMode, scope) => {
    const text=computerText();
    const result=await showAppMessage(mainWindow,{type:'question',title:text.consentTitle,message:text.consentTitle,
      detail:`${origin}\n${service}\n\n${text.website}\n${scope.origins.join('\n')}\n\n${copy('approvalPolicy')}: ${copy(({ask_every_time:'askEveryTime',scoped_changes:'scopedChanges',full_task:'fullTask'})[approvalMode]??'askEveryTime')}\n\n${scope.maxTabs===8?text.workspaceConsent:text.consentBody}${networkMode==='trusted-vpn'?'\n\n'+text.networkWarning:''}`,buttons:[text.cancel,text.allow],defaultId:0,cancelId:0,noLink:true});
    return result.response===1;
  }
});
// Installed-app discovery and launch stay in the trusted main process. The
// renderer receives opaque catalog IDs only; it can never provide a path or
// executable command to the host.
const executionRoot=app.isPackaged?path.join(process.resourcesPath,'execution-runtime'):path.join(__dirname,'../../build/execution-runtime',`${{win32:'win',darwin:'mac',linux:'linux'}[process.platform]}-${process.arch}`);
async function executionFetch(service,endpoint,method='GET'){
  if(!mainWindow||mainWindow.isDestroyed()||!isTrustedPage(mainWindow.webContents.getURL(),service,LOADING_URL))throw Error('execution_scope_changed');
  const response=await mainWindow.webContents.session.fetch(service+endpoint,{method,credentials:'include',redirect:'error',headers:method==='POST'?{'Content-Type':'application/json',Origin:service}:{},...(method==='POST'?{body:'{}'}:{}),signal:AbortSignal.timeout(10000)});
  if(!response.ok)throw Error('execution_service_unavailable');return response.json();
}
const folderTrust=new FolderTrust(path.join(configRoot,'folder-trust'),safeStorage);
const execution=new ExecutionRuntime({trust:folderTrust,root:executionRoot,directory:path.join(configRoot,'execution-hosts'),protectedDirectories:[path.join(configRoot,'computer')],version:app.getVersion(),spawn,
  service:()=>{if(runtime.state.state!=='ready'||runtime.maintenanceAdmissionHold)throw Error('execution_service_unavailable');return runtime.url;},
  identity:service=>executionFetch(service,'/api/v2/system'),
  principal:async service=>{const account=await executionFetch(service,'/v1/users/me');if(account.auth_required===false)return 'local-admin';if(account.auth_required===true&&account.authenticated===true&&typeof account.user?.id==='string'&&account.user.id)return account.user.id;throw Error('execution_scope_changed');},
  pairing:service=>executionFetch(service,'/api/v2/execution/pairing','POST'),
  verify:root=>require('./execution-pack.cjs').verify(root,app.getVersion(),process.platform,process.arch),
  choose:async()=>{const chosen=await dialog.showOpenDialog(mainWindow,{properties:['openDirectory','dontAddToRecent'],title:copy('chooseWorkspace')});return chosen.canceled||chosen.filePaths.length!==1?null:fs.promises.realpath(chosen.filePaths[0]);},
  confirm:async({folder,permissions,service,actor,canRemember})=>{const mode=copy(permissions.processes?'workspaceCommands':permissions.write?'workspaceWrite':'workspaceRead');const result=await showAppMessage(mainWindow,{type:'question',title:copy('workspaceAccessTitle'),message:copy('workspaceAccessTitle'),detail:`${folder}\n${service}\n${copy('account')}: ${actor}\n\n${mode}\n\n${copy('workspaceLifetime')}`,buttons:[copy('cancel'),copy('allowAccess')],defaultId:0,cancelId:0,noLink:true,...(canRemember?{checkboxLabel:folderTrustCopy(presentation.locale).remember,checkboxChecked:false}:{})});return result.response===1?(result.checkboxChecked?'remember':true):false;}
});
execution.on('status',()=>{if(app.isReady()&&presentationCopy)updateMenus();});
const applicationLauncher = createApplicationLauncher({ platform: process.platform, env: process.env, shell });
const uploadGrants = new Map();
async function digestFile(file) {
  return await new Promise((resolve,reject)=>{const hash=crypto.createHash('sha256');const stream=fs.createReadStream(file);stream.on('data',chunk=>hash.update(chunk));stream.once('error',reject);stream.once('end',()=>resolve(hash.digest('hex')));});
}
async function selectComputerUpload() {
  const result=await dialog.showOpenDialog(mainWindow,{title:copy('chooseUpload'),properties:['openFile','dontAddToRecent']});
  if(result.canceled || result.filePaths.length!==1)return null;
  const file=await fs.promises.realpath(result.filePaths[0]);const info=await fs.promises.stat(file);
  if(!info.isFile() || info.size>512*1024*1024)throw Error('computer_upload_invalid');
  const grant={id:crypto.randomUUID().replaceAll('-',''),path:file,name:path.basename(file),size:info.size,sha256:await digestFile(file)};
  uploadGrants.clear();uploadGrants.set(grant.id,grant);
  return {id:grant.id,name:grant.name,size:grant.size,sha256:grant.sha256};
}
computer.on('status',()=>{ if (app.isReady() && presentationCopy) updateMenus(); });
app.on('open-url',(event,url)=>{ event.preventDefault(); const request=parseComputerLink(url);if(request){computerRequested=request;if(app.isReady()){showWindow();showWorkspace();}} });
const statePath = path.join(configRoot, 'window-state.json');
const presentationPath = path.join(configRoot, 'desktop-presentation.json');
let presentation = normalizePresentation({});
let presentationCopy;
const terminalPalette = () => {const mode=effectiveTheme();const id=presentation.appearance?.[mode]??'offgrid';const tokens=themeCatalog.themes[id]?.[mode];return tokens?{...tokens,ansi:themeCatalog.terminal[id]?.[mode]}:undefined;};
let presentationQueue = Promise.resolve();
const copy = key => localizedCopy(presentationCopy??{},presentation.locale,'desktop/presentation')[key] ?? key;
const reportTrayBackground = createTrayFeedback({ platform: process.platform, tray: () => tray, copy });
const effectiveTheme = () => presentation.theme === 'system' ? (nativeTheme.shouldUseDarkColors ? 'dark' : 'light') : presentation.theme;
const showAppMessage = (parent,options) => require('./app-dialog').showAppDialog({BrowserWindow,ipcMain,screen},parent,{buttons:[copy('close')],...options},()=>({theme:effectiveTheme(),palette:terminalPalette(),locale:presentation.locale,direction:languages[presentation.locale].direction,reduceMotion:presentation.appearance?.reduceMotion===true}));
const presentationSnapshot = () => ({ ...presentation, palette: terminalPalette(), effectiveTheme: effectiveTheme(), direction: languages[presentation.locale].direction, windowControls: Object.values(localizedCopy(require('./window-controls.json'),presentation.locale,'desktop/window-controls')), copy: localizedCopy(presentationCopy??{},presentation.locale,'desktop/presentation'), startupOverrides:localeOverrides[presentation.locale]?.['desktop/startup-welcome'] });
let commandCopy = {};
const commandLabel = () => commandCopy[presentation.locale]?.[0] ?? commandCopy.en?.[0] ?? 'Quick window';
const commandPreferencePath = path.join(configRoot, 'command-window.json');
let commandPreferenceQueue = Promise.resolve();
const commandTransitions = require('./command-window-motion').createCommandTransitionBridge();
const commandMotion = require('./command-window-motion').createCommandWindowMotion({
  prepare: commandTransitions.prepare, reducedMotion: () => presentation.appearance?.reduceMotion === true
});
const commandWindow = require('./command-window').createCommandWindow({ window: () => mainWindow, screen, globalShortcut,
  ready: () => runtime.state.state === 'ready' && !quitting && !workspaceNavigationPending,
  hideTerminal: () => terminal?.hide(), motion: commandMotion,
  changed: value => { if (mainWindow && !mainWindow.isDestroyed()) mainWindow.webContents.send('command-window-state', value); }
});

async function loadWindowState() {
  const defaults = { width: 1400, height: 900 };
  try {
    const state = JSON.parse(await fs.promises.readFile(statePath, 'utf8'));
    if (![state.width, state.height].every(Number.isFinite)) return defaults;
    const bounds = { width: Math.max(760, Math.min(3840, state.width)), height: Math.max(560, Math.min(2160, state.height)) };
    if ([state.x, state.y].every(Number.isFinite) && screen.getAllDisplays().some(({ workArea: a }) =>
      state.x + bounds.width > a.x && state.x < a.x + a.width && state.y + bounds.height > a.y && state.y < a.y + a.height)) {
      Object.assign(bounds, { x: state.x, y: state.y });
    }
    return { ...bounds, isMaximized: state.isMaximized === true };
  } catch { return defaults; }
}

function saveWindowState() {
  if (!mainWindow || mainWindow.isDestroyed()) return saveQueue;
  const fullState = commandWindow.fullState();
  const state = fullState ? { ...fullState.bounds, isMaximized: fullState.maximized } : { ...mainWindow.getNormalBounds(), isMaximized: mainWindow.isMaximized() };
  saveQueue = saveQueue.then(async () => {
    await fs.promises.mkdir(configRoot, { recursive: true });
    await fs.promises.writeFile(statePath + '.tmp', JSON.stringify(state));
    await fs.promises.rename(statePath + '.tmp', statePath);
  }).catch(() => console.warn('Window geometry could not be saved.'));
  return saveQueue;
}

function showWindow() {
  if (!mainWindow || mainWindow.isDestroyed()) return void createWindow();
  commandWindow.expand();
}

function safeExternal(value) {
  try { const url = new URL(value); return !url.username && !url.password && ['https:', 'http:', 'mailto:'].includes(url.protocol); }
  catch { return false; }
}

function showWorkspace() {
  if (!mainWindow || mainWindow.isDestroyed() || runtime.state.state !== 'ready' || quitting || workspaceNavigationPending) return;
  const current = mainWindow.webContents.getURL();
  if (current.startsWith(LOADING_URL) || computerRequested) {
    const suffix=computerRequested ? (computerRequested.task ? `#/agents/task/${computerRequested.task}` : '#/agents') : ''; computerRequested=undefined;
    void mainWindow.loadURL(runtime.url + '/ui/'+suffix).catch(() => {});
  }
}

runtime.on('status', status => {
  if (!mainWindow || mainWindow.isDestroyed() || quitting) return;
  mainWindow.webContents.send('startup-state', status);
  if (status.state === 'ready') showWorkspace();
  else if (status.state === 'error' && !mainWindow.webContents.getURL().startsWith(LOADING_URL)) {
    commandWindow.expand();
    void mainWindow.loadFile(path.join(__dirname, 'loading.html'));
  }
});

function createWindow() {
  if (windowCreation) return windowCreation;
  if (mainWindow && !mainWindow.isDestroyed()) return Promise.resolve();
  windowCreation = createMainWindow().finally(() => { windowCreation = null; });
  return windowCreation;
}

async function createMainWindow() {
  const state = await loadWindowState();
  if (quitting) return;
  const showOnCreate = process.env.OFFGRID_DESKTOP_TEST_HIDDEN !== '1';
  mainWindow = new BrowserWindow({
    ...state, minWidth: 760, minHeight: 560, title: APP_NAME, show: showOnCreate, autoHideMenuBar: true,
    icon: path.join(__dirname, 'assets/icon.png'),
    ...require('./branding.cjs').windowChrome(process.platform, effectiveTheme(), terminalPalette()),
    webPreferences: { nodeIntegration: false, contextIsolation: true, sandbox: true, preload: path.join(__dirname, 'preload.js'), backgroundThrottling: true, spellcheck: true, additionalArguments: ['--offgrid-presentation=' + Buffer.from(JSON.stringify(presentationSnapshot())).toString('base64')] }
  });
  terminal = require('./terminal-dock').createTerminalDock({admit:()=>!runtime.maintenanceAdmissionHold,electron:require('electron'),showMessage:showAppMessage,window:mainWindow,host:app.isPackaged?path.join(process.resourcesPath,'terminal-runtime','offgrid-terminal-host.exe'):path.join(__dirname,'../../build/terminal-runtime/win-x64/offgrid-terminal-host.exe'),cli:path.join(binaryRoot,binaryName),service:()=>runtime.url,presentation:()=>({theme:effectiveTheme(),locale:presentation.locale,palette:terminalPalette(),reduceMotion:presentation.appearance?.reduceMotion===true,translations:localeOverrides[presentation.locale]?.['desktop/terminal']??{}})});
  if (state.isMaximized) mainWindow.maximize();
  // A first launch can spend several seconds in Windows reputation scanning
  // and Chromium initialization. Put a real native surface on screen as soon
  // as Electron is ready instead of leaving the user with no feedback until
  // the renderer's first paint. The background color prevents a white flash.
  mainWindow.once('ready-to-show', () => {
    if (showOnCreate) mainWindow.focus();
  });
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (safeExternal(url)) void shell.openExternal(url);
    return { action: 'deny' };
  });
  mainWindow.webContents.on('will-navigate', (event, url) => {
    if (isTrustedPage(url, runtime.url, LOADING_URL)) return;
    event.preventDefault();
    if (safeExternal(url)) void shell.openExternal(url);
  });
  // Full document reloads can reach a replaced service. Hash navigation does
  // not make a network request. Block the bridge while reassessing a reload.
  mainWindow.webContents.session.webRequest.onBeforeRequest((details, callback) => {
    const contents = mainWindow?.webContents;
    if (!contents || details.webContentsId !== contents.id || details.resourceType !== 'mainFrame' ||
        !isTrustedPage(details.url, runtime.url, LOADING_URL) || details.url.startsWith('file:') ||
        contents.getURL().startsWith(LOADING_URL)) return callback({});
    workspaceNavigationPending = true;
    void runtime.connect().then(status => {
      callback({ cancel: status.state !== 'ready' });
      if (status.state !== 'ready' && mainWindow && !mainWindow.isDestroyed()) void mainWindow.loadFile(path.join(__dirname, 'loading.html'));
    }).catch(() => callback({cancel:true})).finally(() => { workspaceNavigationPending = false; });
  });
  mainWindow.webContents.on('did-fail-load', (_event, code, _description, _url, isMainFrame) => {
    if (code === -3 || !isMainFrame || quitting) return;
    runtime.publish({ state: 'error', reason: 'The workspace page could not load. Retry to check the service and reopen it.' });
    if (!mainWindow.webContents.getURL().startsWith(LOADING_URL)) void mainWindow.loadFile(path.join(__dirname, 'loading.html'));
  });
  mainWindow.webContents.on('render-process-gone', () => {
    recovery.invalidate();
    void computer.stop();
    void execution.stop();
    runtime.publish({ state: 'error', reason: 'The workspace window stopped responding. Retry to reopen it; the service and saved work have not been stopped.' });
    void mainWindow.loadFile(path.join(__dirname, 'loading.html'));
  });
  mainWindow.webContents.on('did-start-navigation',(_event,_url,isInPlace,isMainFrame)=>{if(isMainFrame&&!isInPlace)recovery.invalidate();});
  mainWindow.on('closed',()=>recovery.invalidate());
  for (const event of ['resize', 'move', 'maximize', 'unmaximize']) {
    mainWindow.on(event, () => { clearTimeout(saveTimer); saveTimer = setTimeout(saveWindowState, 500); });
  }
  mainWindow.on('close', event => {
    void saveWindowState();
    // Do not strand failed first launches invisibly in the tray.
    if (!quitting && process.platform !== 'darwin' && runtime.state.state === 'ready' && tray) {
      event.preventDefault();
      mainWindow.webContents.send('command-window-dismiss');
      mainWindow.hide();
      reportTrayBackground();
    }
  });
  mainWindow.on('closed', () => { mainWindow = null; });
  // Render first. Backend discovery/startup never blocks creation of the window.
  await mainWindow.loadFile(path.join(__dirname, 'loading.html'));
  showWorkspace();
}

async function manageFolderTrust(){
  const text=folderTrustCopy(presentation.locale);
  try{
    const records=await folderTrust.list();
    if(!records.length){await showAppMessage(mainWindow,{type:'info',message:text.empty});return;}
    for(const record of records){
      const level=['read','write','processes'].filter(key=>record.permissions[key]).map(key=>text.permissions[['read','write','processes'].indexOf(key)]).join(' · ');
      const answer=await showAppMessage(mainWindow,{type:'question',message:text.title,detail:`${record.root}\n${record.host}\n${record.service}\n${record.actor}\n${level}\n\n${text.notice}`,buttons:[text.done,text.forget,text.next],defaultId:0,cancelId:0,noLink:true});
      if(answer.response===0)return;if(answer.response===1)await folderTrust.revoke(record.id);
    }
  }catch{await showAppMessage(mainWindow,{type:'error',message:text.error});}
}

function updateMenus() {
  const role=(name,key=name)=>({role:name,label:copy(key)});
  Menu.setApplicationMenu(Menu.buildFromTemplate([
    ...(process.platform === 'darwin' ? [{ role: 'appMenu' }] : []),
    { label: copy('file'), submenu: [
      { label: commandLabel(), click: () => commandWindow.open() },
      { label: computerText()?.stop ?? 'Stop browser assistance', enabled:!!computer.child || computer.pending, click:()=>void computer.stop() },
      { label:copy('stopWorkspace'),enabled:!!execution.child||execution.pending,click:()=>void execution.stop() },
      { label:folderTrustCopy(presentation.locale).manage,click:()=>void manageFolderTrust() },
      { type: 'separator' },
      role(process.platform === 'darwin' ? 'close' : 'quit')
    ] },
    { label:copy('editMenu'),submenu:[role('undo'),role('redo'),{type:'separator'},role('cut'),role('copy'),role('paste'),role('selectAll')] }, { label:copy('viewMenu'), submenu: [{ label: require('./terminal-translations').text(presentation.locale,'Terminals'), accelerator: 'CommandOrControl+`', click: () => void terminal?.toggle().catch(error => showAppMessage(mainWindow,{type:'error',message:error.message})) }, {type:'separator'}, role('reload'),role('toggleDevTools','developerTools'), {type:'separator'},role('resetZoom'),role('zoomIn'),role('zoomOut'),role('togglefullscreen','fullscreen')] }, { role: 'windowMenu',label:copy('windowMenu') },
    { role: 'help',label:copy('helpMenu'), submenu: [{ label: copy('guide'), click: () => void shell.openExternal('https://github.com/takuphilchan/offgrid-llm/blob/main/docs/setup/desktop-startup.md') }] }
  ]));
  tray?.setContextMenu(Menu.buildFromTemplate([
    { label: commandLabel(), click: () => commandWindow.open() },
    { label: copy('open'), click: showWindow },
    { label: computerText()?.stop ?? 'Stop browser assistance', enabled:!!computer.child || computer.pending, click:()=>void computer.stop() },
    { label:copy('stopWorkspace'),enabled:!!execution.child||execution.pending,click:()=>void execution.stop() },
      { label:folderTrustCopy(presentation.locale).manage,click:()=>void manageFolderTrust() },
    { type: 'separator' },
    { label: copy('quit'), click: () => app.quit() }
  ]));
}

function createTray() {
  try {
    const icon = nativeImage.createFromPath(path.join(__dirname, 'assets/icon.png')).resize({ width: 16, height: 16 });
    tray = new Tray(icon);
    tray.setToolTip(APP_NAME);
    tray.on('click', showWindow);
  } catch { console.warn('System tray is unavailable; closing the window will quit OffGrid.'); }
}

function handleTrustedIPC(channel, handler, startupOnly = false) {
  ipcMain.handle(channel, (event, ...args) => {
    if (!isTrustedSender(event, mainWindow?.webContents, runtime.url, LOADING_URL) ||
        (startupOnly && event.senderFrame.url !== LOADING_URL)) throw new Error('Untrusted desktop IPC sender');
    const loading = event.senderFrame.url === LOADING_URL;
    if (!workspaceChannelAllowed(channel, runtime.state.state === 'ready'&&!runtime.maintenanceAdmissionHold, loading)) throw new Error('Desktop workspace access is not ready');
    return handler(...args);
  });
}

handleTrustedIPC('get-api-url', () => runtime.url);
const recovery=require('./workspace-maintenance-host').createWorkspaceRecovery({runtime,dialog,window:()=>mainWindow,showMessage:showAppMessage,locale:()=>presentation.locale,computer,execution,terminal:()=>terminal,handle:handleTrustedIPC});
// Only the isolated preload caption exposes these buttons. No contextBridge
// window-control method is provided to service content.
handleTrustedIPC('window-chrome-action', action => require('./branding.cjs').controlWindow(mainWindow, action, commandWindow.snapshot().mode === 'compact'));
handleTrustedIPC('command-window-status', () => commandWindow.snapshot());
handleTrustedIPC('command-window-open', () => commandWindow.open());
handleTrustedIPC('command-window-expand', () => commandWindow.expand());
handleTrustedIPC('command-window-dismiss', () => commandWindow.dismiss());
handleTrustedIPC('command-window-layout', value => commandWindow.setLayout(value?.layout, {prepared:value?.prepared === true, reduceMotion:value?.reduceMotion === true}));
handleTrustedIPC('command-window-transition-ready', value => {
  const settled = commandMotion.settled;
  return commandTransitions.acknowledge(value) ? settled : undefined;
});
handleTrustedIPC('command-window-configure', value => {
  if (!value || typeof value.shortcut !== 'string') throw Error('invalid_command_shortcut');
  // Serialize native registration with the preference write. Report disk failure
  // instead of claiming the shortcut survives restart.
  commandPreferenceQueue = commandPreferenceQueue.catch(() => {}).then(async () => {
    if (quitting) throw Error('desktop_shutting_down');
    return require('./command-window').saveShortcut(value.shortcut, async shortcut => {
      await fs.promises.mkdir(configRoot, {recursive: true});
      await fs.promises.writeFile(commandPreferencePath + '.tmp', JSON.stringify({shortcut}));
      await fs.promises.rename(commandPreferencePath + '.tmp', commandPreferencePath);
    }, shortcut => quitting ? commandWindow.snapshot() : commandWindow.configure(shortcut));
  });
  return commandPreferenceQueue;
});
handleTrustedIPC('get-app-version', () => app.getVersion());
handleTrustedIPC('get-server-status', () => runtime.state.state === 'ready');
handleTrustedIPC('get-backend-info', () => runtime.snapshot());
handleTrustedIPC('execution-start', request=>execution.start(request));
handleTrustedIPC('execution-status', ()=>execution.status());
handleTrustedIPC('execution-stop', ()=>execution.stop());
handleTrustedIPC('computer-status', () => ({...computer.state,...computer.browserControl.status(), active:!!computer.child||!!computer.pending, installed:fs.existsSync(path.join(computerRoot,'manifest.json'))}));
handleTrustedIPC('computer-takeover-access', request => computer.browserControl.takeover(request));
handleTrustedIPC('computer-browser-resume', request => computer.browserControl.resume(request));
handleTrustedIPC('computer-start', request => {
  const upload=request?.uploadGrant ? uploadGrants.get(request.uploadGrant) : undefined;
  if(request?.uploadGrant && !upload)throw Error('computer_upload_invalid');
  const {uploadGrant,...safe}=request??{};
  if(uploadGrant)uploadGrants.delete(uploadGrant);
  return computer.start({...safe,upload});
});
handleTrustedIPC('computer-select-upload', () => selectComputerUpload());
handleTrustedIPC('computer-targets', request => computer.discoverNative(request));
handleTrustedIPC('computer-native-start', request => computer.startNative(request));
handleTrustedIPC('computer-desktop-start', request => computer.startDesktop(request));
handleTrustedIPC('computer-desktop-enable', request => computer.enableDesktopAccess(request));
handleTrustedIPC('computer-desktop-access', () => computer.desktopAccessStatus());
handleTrustedIPC('computer-launchable-apps', () => ({ state: 'selecting', targets: applicationLauncher.discover() }));
handleTrustedIPC('computer-launch-app', request => applicationLauncher.launch(request?.id));
handleTrustedIPC('computer-stop', () => computer.stop());
handleTrustedIPC('computer-stop-access', request => computer.stopAccess(request));
handleTrustedIPC('terminal-open', async () => {try{return await terminal.open();}catch(error){if(!quitting&&mainWindow&&!mainWindow.isDestroyed()&&error.message!=='terminal_stopping')await showAppMessage(mainWindow,{type:'error',message:copy('terminalFailure'),detail:error.message});return {visible:false};}});
handleTrustedIPC('terminal-status', () => ({available:terminal?.sessions.available()===true}));
handleTrustedIPC('terminal-layout', value => terminal.setWorkspaceBounds(value));
handleTrustedIPC('terminal-overlay', value => terminal.setOverlayActive(value));
handleTrustedIPC('terminal-hide', () => terminal.hide());
handleTrustedIPC('terminal-context-response', (id,accepted) => terminal.acknowledgeContext(id,accepted));
handleTrustedIPC('get-presentation', () => presentationSnapshot());
handleTrustedIPC('set-presentation', value => {
  // Only appearance/language preferences: no caller-provided file paths or commands.
  if (!value || !['light', 'dark', 'system'].includes(value.theme) || typeof value.locale !== 'string' || !Object.hasOwn(languages, value.locale)) throw new Error('Invalid presentation preferences');
  const next = normalizePresentation(value);
  presentationQueue = presentationQueue.catch(() => {}).then(async () => {
    await fs.promises.mkdir(configRoot, { recursive: true });
    await fs.promises.writeFile(presentationPath + '.tmp', JSON.stringify(next), { mode: 0o600 });
    await fs.promises.rename(presentationPath + '.tmp', presentationPath);
    presentation = next;
    nativeTheme.themeSource = presentation.theme;
    require('./branding.cjs').updateChrome(mainWindow, process.platform, effectiveTheme(), terminalPalette());
    updateMenus();
    terminal?.appearance();
  });
  return presentationQueue;
});
handleTrustedIPC('startup-retry', () => runtime.connect(), true);
handleTrustedIPC('startup-local', async () => {
  return runtime.connect(true);
}, true);
handleTrustedIPC('startup-browser', async () => {
  if (!runtime.state.canOpenBrowser) throw new Error('No identified external OffGrid workspace is available');
  await shell.openExternal(runtime.url + '/ui/');
}, true);
handleTrustedIPC('startup-help', () => shell.openExternal('https://github.com/takuphilchan/offgrid-llm/blob/main/docs/setup/desktop-startup.md'), true);
handleTrustedIPC('get-paths', () => {
  if (!runtime.child) throw new Error('Paths belong to the externally managed service, not the desktop host');
  return { ...runtime.workspace };
});
handleTrustedIPC('select-directory', async () => {
  const result = await dialog.showOpenDialog(mainWindow, { properties: ['openDirectory'], title: copy('directory') });
  return result.canceled ? null : result.filePaths[0] || null;
});
handleTrustedIPC('get-system-theme', () => nativeTheme.shouldUseDarkColors ? 'dark' : 'light');
nativeTheme.on('updated', () => {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  const theme = nativeTheme.shouldUseDarkColors ? 'dark' : 'light';
  require('./branding.cjs').updateChrome(mainWindow, process.platform, effectiveTheme(), terminalPalette());
  mainWindow.webContents.send('system-theme-changed', theme);
  terminal?.appearance();
});
app.on('second-instance', (_event, _argv, _directory, request) => {
  if (request?.quitForInstall === true) app.quit();
  else { const request=_argv.map(parseComputerLink).find(Boolean);if(request)computerRequested=request; showWindow(); showWorkspace(); }
});
app.whenReady().then(async () => {
  if (process.platform === 'win32') app.setAppUserModelId('com.offgrid.llm.desktop');
  presentationCopy = readCopy(uiDir);
  try { commandCopy = JSON.parse(await fs.promises.readFile(path.join(uiDir, 'desktop-command.json'), 'utf8')); } catch { /* Older UI retains the tray entry and explicit settings capability. */ }
  computerCopy = JSON.parse(await fs.promises.readFile(path.join(uiDir,'computer-experience.json'),'utf8'));
  try { presentation = normalizePresentation(JSON.parse(await fs.promises.readFile(presentationPath, 'utf8')), app.getLocale().split('-')[0]); }
  catch { presentation = normalizePresentation({}, app.getLocale().split('-')[0]); }
  // Chromium popup controls and the renderer must use the same appearance,
  // including when OffGrid's explicit choice differs from the OS preference.
  nativeTheme.themeSource = presentation.theme;
  await createWindow();
  let shortcut = 'CommandOrControl+Shift+Space';
  try { const preference = JSON.parse(await fs.promises.readFile(commandPreferencePath, 'utf8')); if (preference.shortcut === '' || require('./command-window').shortcuts.includes(preference.shortcut)) shortcut = preference.shortcut; } catch {}
  // Isolated tests never register a shortcut on the user's real desktop.
  if (!customHome && !installerTest) commandWindow.configure(shortcut);
  // Qualification/alternate profiles must not change the user's default handler.
  if(app.isPackaged && !installerTest && !customHome) app.setAsDefaultProtocolClient('offgrid');
  powerMonitor.on('lock-screen',()=>{recovery.invalidate();void computer.stop();void execution.stop();});
  powerMonitor.on('suspend',()=>{recovery.invalidate();void computer.stop();void execution.stop();});
  // Cookie removal/replacement includes logout and account changes. Do not let
  // an old actor's host permission survive until a renderer happens to refresh.
  mainWindow.webContents.session.cookies.on('changed',(_event,cookie)=>{
    if(cookie.name==='session')recovery.invalidate();
    if(cookie.name==='session'&&(computer.desktopAccess||computer.pending||computer.browserControl.context))void computer.stop();
    if(cookie.name==='session')void execution.stop();
  });
  mainWindow.on('unresponsive', () => { void computer.stop();void execution.stop(); });
  createTray();
  updateMenus();
  // A normal launch always reconnects to the authoritative local workspace.
  // An isolated workspace is an explicit, session-scoped recovery action only;
  // it must never silently replace the user's service, models or history.
  void runtime.connect(false);
  app.on('activate', showWindow);
});
app.on('window-all-closed', () => {
  if (runtime.state.state !== 'ready' || !tray) app.quit();
});
app.on('before-quit', event => {
  if (shutdownComplete) return;
  event.preventDefault();
  if (quitting) return;
  quitting = true;
  commandWindow.dispose();
  clearTimeout(saveTimer);
  void (async () => {
    await require('./shutdown').shutdownDesktop({
      save: async () => { await saveWindowState(); await presentationQueue; await commandPreferenceQueue.catch(() => {}); },
      computer, execution, terminal, runtime, recovery,
    });
    tray?.destroy();
    shutdownComplete = true;
    app.quit();
  })().catch(() => {
    quitting = false;
    commandWindow.configure(commandWindow.snapshot().shortcut);
    runtime.publish({ state: 'error', reason: 'OffGrid could not confirm that all local work stopped. Quit again to retry. No new service was started.' });
    showWindow();
  });
});
process.on('unhandledRejection', () => {
  if (!quitting) runtime.publish({ state: 'error', reason: 'A desktop operation failed. Retry the connection or close and reopen OffGrid.' });
});
