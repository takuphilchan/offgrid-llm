const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage, shell, nativeTheme, screen, utilityProcess, powerMonitor } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { isTrustedPage, isTrustedSender, fingerprintUI } = require('./backend');
const { DesktopRuntime } = require('./runtime');
const { normalize: normalizePresentation, readCopy } = require('./presentation');
const { ComputerRuntime, isComputerLink } = require('./computer-runtime');
const { createApplicationLauncher } = require('./application-launcher');
// Verification code belongs to the application, not the unverified payload.
const { verifyPack: verifyComputerPack } = require(app.isPackaged ? './computer-pack/pack.cjs' : '../computer/pack.cjs');

const APP_NAME = 'OffGrid LLM Desktop';
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
const binaryRoot = app.isPackaged ? path.join(process.resourcesPath, 'bin') : path.join(__dirname, '../build', { win32: 'windows', darwin: 'macos', linux: 'linux' }[process.platform]);
const binaryName = process.platform === 'win32' ? 'offgrid.exe' : process.platform === 'darwin' ? 'offgrid-' + (process.arch === 'arm64' ? 'arm64' : 'amd64') : 'offgrid';
const workspace = root => ({ config: root, models: path.join(root, 'models'), data: path.join(root, 'data') });
const runtime = new DesktopRuntime({
  url: 'http://127.0.0.1:' + port, version: app.getVersion(), binary: path.join(binaryRoot, binaryName), uiDir,
  workspace: workspace(configRoot), isolatedWorkspace: workspace(path.join(configRoot, 'desktop-workspace')),
  uiBuildID: fs.existsSync(path.join(uiDir, 'index.html')) ? fingerprintUI(fs.readFileSync(path.join(uiDir, 'index.html'))) : null
});
let mainWindow = null;
let tray = null;
let quitting = false;
let shutdownComplete = false;
let saveTimer;
let saveQueue = Promise.resolve();
let windowCreation;
let computerRequested = process.argv.some(isComputerLink);
const computerRoot = app.isPackaged ? path.join(process.resourcesPath,'computer') : path.join(__dirname,'../build/computer-runtime',`${{win32:'win',darwin:'mac',linux:'linux'}[process.platform]}-${process.arch}`);
let computerCopy;
const computerText = () => computerCopy?.[presentation.locale] ?? computerCopy?.en;
const computer = new ComputerRuntime({
  root: computerRoot, directory:path.join(configRoot,'computer'), fork:(...args)=>utilityProcess.fork(...args),
  service:()=> { if(runtime.state.state!=='ready') throw Error('service_unavailable'); return runtime.url; },
  identity: async url => {
    const response=await fetch(url+'/api/v2/system',{redirect:'error',signal:AbortSignal.timeout(5000)});
    if(!response.ok) throw Error('service_unavailable'); return response.json();
  },
  verify: async root => { try { await verifyComputerPack(root); } catch { throw Error('pack_invalid'); } },
  pairing: async service => {
    if (!mainWindow || mainWindow.isDestroyed() || !isTrustedPage(mainWindow.webContents.getURL(),service,LOADING_URL)) throw Error('pairing_failed');
    // Use the authenticated desktop session without extracting cookies or
    // disclosing enrollment credentials to renderer storage/IPC arguments.
    const response=await mainWindow.webContents.session.fetch(service+'/api/v2/computer/pairing',{
      method:'POST',credentials:'include',redirect:'error',headers:{'Content-Type':'application/json',Origin:service},body:'{}',signal:AbortSignal.timeout(10000)});
    if(!response.ok) throw Error('pairing_failed');return response.json();
  },
  confirm: async (origin, service, networkMode, approvalMode) => {
    const text=computerText();
    const result=await dialog.showMessageBox(mainWindow,{type:'question',title:text.consentTitle,message:text.consentTitle,
      detail:`${origin}\n${service}\n\nApproval policy: ${approvalMode.replaceAll('_',' ')}\n\n${text.consentBody}${networkMode==='trusted-vpn'?'\n\n'+text.networkWarning:''}`,buttons:[text.cancel,text.allow],defaultId:0,cancelId:0,noLink:true});
    return result.response===1;
  }
});
// Installed-app discovery and launch stay in the trusted main process. The
// renderer receives opaque catalog IDs only; it can never provide a path or
// executable command to the host.
const applicationLauncher = createApplicationLauncher({ platform: process.platform, env: process.env, shell });
const uploadGrants = new Map();
async function digestFile(file) {
  return await new Promise((resolve,reject)=>{const hash=crypto.createHash('sha256');const stream=fs.createReadStream(file);stream.on('data',chunk=>hash.update(chunk));stream.once('error',reject);stream.once('end',()=>resolve(hash.digest('hex')));});
}
async function selectComputerUpload() {
  const result=await dialog.showOpenDialog(mainWindow,{title:'Select one file for this computer task',properties:['openFile','dontAddToRecent']});
  if(result.canceled || result.filePaths.length!==1)return null;
  const file=await fs.promises.realpath(result.filePaths[0]);const info=await fs.promises.stat(file);
  if(!info.isFile() || info.size>512*1024*1024)throw Error('computer_upload_invalid');
  const grant={id:crypto.randomUUID().replaceAll('-',''),path:file,name:path.basename(file),size:info.size,sha256:await digestFile(file)};
  uploadGrants.clear();uploadGrants.set(grant.id,grant);
  return {id:grant.id,name:grant.name,size:grant.size,sha256:grant.sha256};
}
computer.on('status',()=>{ if (app.isReady() && presentationCopy) updateMenus(); });
app.on('open-url',(event,url)=>{ event.preventDefault(); if(isComputerLink(url)){computerRequested=true;if(app.isReady()){showWindow();showWorkspace();}} });
const statePath = path.join(configRoot, 'window-state.json');
const presentationPath = path.join(configRoot, 'desktop-presentation.json');
let presentation = normalizePresentation({});
let presentationCopy;
let presentationQueue = Promise.resolve();
const copy = key => presentationCopy?.[presentation.locale]?.[key] ?? presentationCopy?.en?.[key] ?? key;
const effectiveTheme = () => presentation.theme === 'system' ? (nativeTheme.shouldUseDarkColors ? 'dark' : 'light') : presentation.theme;
const presentationSnapshot = () => ({ ...presentation, effectiveTheme: effectiveTheme(), copy: presentationCopy?.[presentation.locale] });

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
  const state = { ...mainWindow.getNormalBounds(), isMaximized: mainWindow.isMaximized() };
  saveQueue = saveQueue.then(async () => {
    await fs.promises.mkdir(configRoot, { recursive: true });
    await fs.promises.writeFile(statePath + '.tmp', JSON.stringify(state));
    await fs.promises.rename(statePath + '.tmp', statePath);
  }).catch(() => console.warn('Window geometry could not be saved.'));
  return saveQueue;
}

function showWindow() {
  if (!mainWindow || mainWindow.isDestroyed()) return void createWindow();
  if (mainWindow.isMinimized()) mainWindow.restore();
  mainWindow.show();
  mainWindow.focus();
}

function safeExternal(value) {
  try { const url = new URL(value); return !url.username && !url.password && ['https:', 'http:', 'mailto:'].includes(url.protocol); }
  catch { return false; }
}

function showWorkspace() {
  if (!mainWindow || mainWindow.isDestroyed() || runtime.state.state !== 'ready' || quitting) return;
  const current = mainWindow.webContents.getURL();
  if (current.startsWith(LOADING_URL) || computerRequested) {
    const suffix=computerRequested ? '#/agents' : ''; computerRequested=false;
    void mainWindow.loadURL(runtime.url + '/ui/'+suffix).catch(() => {});
  }
}

runtime.on('status', status => {
  if (!mainWindow || mainWindow.isDestroyed() || quitting) return;
  mainWindow.webContents.send('startup-state', status);
  if (status.state === 'ready') showWorkspace();
  else if (status.state === 'error' && !mainWindow.webContents.getURL().startsWith(LOADING_URL)) {
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
    backgroundColor: effectiveTheme() === 'dark' ? '#101011' : '#f7f7f6',
    webPreferences: { nodeIntegration: false, contextIsolation: true, sandbox: true, preload: path.join(__dirname, 'preload.js'), backgroundThrottling: true, spellcheck: true, additionalArguments: ['--offgrid-presentation=' + Buffer.from(JSON.stringify(presentationSnapshot())).toString('base64')] }
  });
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
  mainWindow.webContents.on('did-fail-load', (_event, code, _description, _url, isMainFrame) => {
    if (code === -3 || !isMainFrame || quitting) return;
    runtime.publish({ state: 'error', reason: 'The workspace page could not load. Retry to check the service and reopen it.' });
    if (!mainWindow.webContents.getURL().startsWith(LOADING_URL)) void mainWindow.loadFile(path.join(__dirname, 'loading.html'));
  });
  mainWindow.webContents.on('render-process-gone', () => {
    runtime.publish({ state: 'error', reason: 'The workspace window stopped responding. Retry to reopen it; the service and saved work have not been stopped.' });
    void mainWindow.loadFile(path.join(__dirname, 'loading.html'));
  });
  for (const event of ['resize', 'move', 'maximize', 'unmaximize']) {
    mainWindow.on(event, () => { clearTimeout(saveTimer); saveTimer = setTimeout(saveWindowState, 500); });
  }
  mainWindow.on('close', event => {
    void saveWindowState();
    // Do not strand failed first launches invisibly in the tray.
    if (!quitting && process.platform !== 'darwin' && runtime.state.state === 'ready' && tray) {
      event.preventDefault();
      mainWindow.hide();
    }
  });
  mainWindow.on('closed', () => { mainWindow = null; });
  // Render first. Backend discovery/startup never blocks creation of the window.
  await mainWindow.loadFile(path.join(__dirname, 'loading.html'));
  showWorkspace();
}

function updateMenus() {
  Menu.setApplicationMenu(Menu.buildFromTemplate([
    ...(process.platform === 'darwin' ? [{ role: 'appMenu' }] : []),
    { label: copy('file'), submenu: [
      { label: computerText()?.stop ?? 'Stop browser assistance', enabled:!!computer.child || computer.pending, click:()=>void computer.stop() },
      { type: 'separator' },
      { role: process.platform === 'darwin' ? 'close' : 'quit' }
    ] },
    { role: 'editMenu' }, { role: 'viewMenu' }, { role: 'windowMenu' },
    { role: 'help', submenu: [{ label: copy('guide'), click: () => void shell.openExternal('https://github.com/takuphilchan/offgrid-llm/blob/main/docs/setup/desktop-startup.md') }] }
  ]));
  tray?.setContextMenu(Menu.buildFromTemplate([
    { label: copy('open'), click: showWindow },
    { label: computerText()?.stop ?? 'Stop browser assistance', enabled:!!computer.child || computer.pending, click:()=>void computer.stop() },
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
    return handler(...args);
  });
}

handleTrustedIPC('get-api-url', () => runtime.url);
handleTrustedIPC('get-app-version', () => app.getVersion());
handleTrustedIPC('get-server-status', () => runtime.state.state === 'ready');
handleTrustedIPC('get-backend-info', () => runtime.snapshot());
handleTrustedIPC('computer-status', () => ({...computer.state, installed:fs.existsSync(path.join(computerRoot,'manifest.json'))}));
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
handleTrustedIPC('computer-launchable-apps', () => ({ state: 'selecting', targets: applicationLauncher.discover() }));
handleTrustedIPC('computer-launch-app', request => applicationLauncher.launch(request?.id));
handleTrustedIPC('computer-stop', () => computer.stop());
handleTrustedIPC('get-presentation', () => presentationSnapshot());
handleTrustedIPC('set-presentation', value => {
  // Only appearance/language preferences: no caller-provided file paths or commands.
  if (!value || !['light', 'dark', 'system'].includes(value.theme) || typeof value.locale !== 'string' || !Object.hasOwn(presentationCopy, value.locale)) throw new Error('Invalid presentation preferences');
  const next = normalizePresentation(value);
  presentationQueue = presentationQueue.catch(() => {}).then(async () => {
    await fs.promises.mkdir(configRoot, { recursive: true });
    await fs.promises.writeFile(presentationPath + '.tmp', JSON.stringify(next), { mode: 0o600 });
    await fs.promises.rename(presentationPath + '.tmp', presentationPath);
    presentation = next;
    nativeTheme.themeSource = presentation.theme;
    mainWindow?.setBackgroundColor(effectiveTheme() === 'dark' ? '#101011' : '#f7f7f6');
    updateMenus();
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
  mainWindow.setBackgroundColor(effectiveTheme() === 'dark' ? '#101011' : '#f7f7f6');
  mainWindow.webContents.send('system-theme-changed', theme);
});
app.on('second-instance', (_event, _argv, _directory, request) => {
  if (request?.quitForInstall === true) app.quit();
  else { if(_argv.some(isComputerLink)) computerRequested=true; showWindow(); showWorkspace(); }
});
app.whenReady().then(async () => {
  if (process.platform === 'win32') app.setAppUserModelId('com.offgrid.llm.desktop');
  presentationCopy = readCopy(uiDir);
  computerCopy = JSON.parse(await fs.promises.readFile(path.join(uiDir,'computer-experience.json'),'utf8'));
  try { presentation = normalizePresentation(JSON.parse(await fs.promises.readFile(presentationPath, 'utf8')), app.getLocale().split('-')[0]); }
  catch { presentation = normalizePresentation({}, app.getLocale().split('-')[0]); }
  // Chromium popup controls and the renderer must use the same appearance,
  // including when OffGrid's explicit choice differs from the OS preference.
  nativeTheme.themeSource = presentation.theme;
  await createWindow();
  // Qualification/alternate profiles must not change the user's default handler.
  if(app.isPackaged && !installerTest && !customHome) app.setAsDefaultProtocolClient('offgrid');
  powerMonitor.on('lock-screen',()=>void computer.stop());
  powerMonitor.on('suspend',()=>void computer.stop());
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
  clearTimeout(saveTimer);
  void (async () => {
    await saveWindowState();
    await presentationQueue.catch(() => {});
    await computer.stop();
    await runtime.stop(); // Only our child, never a Docker/external service.
    tray?.destroy();
    shutdownComplete = true;
    app.quit();
  })();
});
process.on('unhandledRejection', () => {
  if (!quitting) runtime.publish({ state: 'error', reason: 'A desktop operation failed. Retry the connection or close and reopen OffGrid.' });
});
