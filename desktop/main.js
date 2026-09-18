const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage, shell, nativeTheme, screen } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const { pathToFileURL } = require('node:url');
const { isTrustedPage, isTrustedSender, fingerprintUI } = require('./backend');
const { DesktopRuntime } = require('./runtime');

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
let connectionQueue = Promise.resolve();
let windowCreation;
let nextWorkspace = 'default';
const statePath = path.join(configRoot, 'window-state.json');
const connectionPath = path.join(configRoot, 'desktop-connection.json');

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
  if (current.startsWith(LOADING_URL)) void mainWindow.loadURL(runtime.url + '/ui/').catch(() => {});
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
    backgroundColor: nativeTheme.shouldUseDarkColors ? '#101011' : '#f7f7f6',
    webPreferences: { nodeIntegration: false, contextIsolation: true, sandbox: true, preload: path.join(__dirname, 'preload.js'), backgroundThrottling: true, spellcheck: true }
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

function rememberConnection(mode) {
  connectionQueue = connectionQueue.catch(() => {}).then(async () => {
    await fs.promises.mkdir(configRoot, { recursive: true });
    await fs.promises.writeFile(connectionPath + '.tmp', JSON.stringify({ mode }));
    await fs.promises.rename(connectionPath + '.tmp', connectionPath);
    nextWorkspace = mode;
    updateMenus();
  });
  return connectionQueue;
}

function connectionMenu() {
  const select = mode => void rememberConnection(mode).catch(() => {
    void dialog.showMessageBox(mainWindow, { type: 'warning', message: 'The next-launch preference could not be saved.', detail: 'The current workspace has not changed. Check that your desktop settings folder is writable.' });
  });
  return [
    { label: 'Applies after you quit and reopen OffGrid', enabled: false },
    { type: 'separator' },
    { id: 'connection-default', label: 'Configured local service', type: 'radio', checked: nextWorkspace === 'default', click: () => select('default') },
    { id: 'connection-isolated', label: 'Separate desktop workspace', type: 'radio', checked: nextWorkspace === 'isolated', click: () => select('isolated') }
  ];
}

function updateMenus() {
  Menu.setApplicationMenu(Menu.buildFromTemplate([
    ...(process.platform === 'darwin' ? [{ role: 'appMenu' }] : []),
    { label: 'File', submenu: [
      { label: 'Connection on next launch', submenu: connectionMenu() },
      { type: 'separator' },
      { role: process.platform === 'darwin' ? 'close' : 'quit' }
    ] },
    { role: 'editMenu' }, { role: 'viewMenu' }, { role: 'windowMenu' },
    { role: 'help', submenu: [{ label: 'Setup and recovery guide', click: () => void shell.openExternal('https://github.com/takuphilchan/offgrid-llm/blob/main/docs/setup/desktop-startup.md') }] }
  ]));
  tray?.setContextMenu(Menu.buildFromTemplate([
    { label: 'Open OffGrid', click: showWindow },
    { label: 'Connection on next launch', submenu: connectionMenu() },
    { type: 'separator' },
    { label: 'Quit OffGrid', click: () => app.quit() }
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
handleTrustedIPC('startup-retry', () => runtime.connect(), true);
handleTrustedIPC('startup-local', async () => {
  const status = await runtime.connect(true);
  if (status.state === 'ready' && status.workspaceMode === 'isolated') {
    await rememberConnection('isolated');
  }
  return status;
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
  const result = await dialog.showOpenDialog(mainWindow, { properties: ['openDirectory'], title: 'Select Directory' });
  return result.canceled ? null : result.filePaths[0] || null;
});
handleTrustedIPC('get-system-theme', () => nativeTheme.shouldUseDarkColors ? 'dark' : 'light');
nativeTheme.on('updated', () => {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  const theme = nativeTheme.shouldUseDarkColors ? 'dark' : 'light';
  mainWindow.setBackgroundColor(theme === 'dark' ? '#101011' : '#f7f7f6');
  mainWindow.webContents.send('system-theme-changed', theme);
});
app.on('second-instance', (_event, _argv, _directory, request) => {
  if (request?.quitForInstall === true) app.quit();
  else showWindow();
});
app.whenReady().then(async () => {
  if (process.platform === 'win32') app.setAppUserModelId('com.offgrid.llm.desktop');
  await createWindow();
  createTray();
  try { nextWorkspace = JSON.parse(await fs.promises.readFile(connectionPath, 'utf8')).mode === 'isolated' ? 'isolated' : 'default'; } catch {}
  updateMenus();
  void runtime.connect(nextWorkspace === 'isolated');
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
    await connectionQueue.catch(() => {});
    await runtime.stop(); // Only our child, never a Docker/external service.
    tray?.destroy();
    shutdownComplete = true;
    app.quit();
  })();
});
process.on('unhandledRejection', () => {
  if (!quitting) runtime.publish({ state: 'error', reason: 'A desktop operation failed. Retry the connection or close and reopen OffGrid.' });
});
