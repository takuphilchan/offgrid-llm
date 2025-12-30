const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage, shell, nativeTheme } = require('electron');
const path = require('path');
const { spawn } = require('child_process');
const fs = require('fs');
const http = require('http');

// Single instance lock - prevent multiple instances
const gotTheLock = app.requestSingleInstanceLock();

if (!gotTheLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    // Focus the existing window when user tries to open second instance
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
    }
  });
}

let mainWindow = null;
let tray = null;
let offgridProcess = null;
let isQuitting = false;
let serverCheckInterval = null;

const APP_NAME = 'OffGrid LLM Desktop';
const SERVER_PORT = 11611;
const SERVER_URL = `http://localhost:${SERVER_PORT}`;

// Paths configuration
const paths = {
  getOffgridBinary() {
    if (app.isPackaged) {
      if (process.platform === 'win32') {
        return path.join(process.resourcesPath, 'bin', 'offgrid.exe');
      } else if (process.platform === 'darwin') {
        // Use architecture-specific binary for macOS
        const arch = process.arch === 'arm64' ? 'arm64' : 'amd64';
        return path.join(process.resourcesPath, 'bin', `offgrid-${arch}`);
      } else {
        return path.join(process.resourcesPath, 'bin', 'offgrid');
      }
    } else {
      // Development mode
      const platform = process.platform;
      const basePath = path.join(__dirname, '../build');
      
      if (platform === 'win32') {
        return path.join(basePath, 'windows/offgrid.exe');
      } else if (platform === 'darwin') {
        const arch = process.arch === 'arm64' ? 'arm64' : 'amd64';
        return path.join(basePath, `macos/offgrid-${arch}`);
      } else {
        return path.join(basePath, 'linux/offgrid');
      }
    }
  },
  
  getConfigDir() {
    return path.join(app.getPath('home'), '.offgrid-llm');
  },
  
  getModelsDir() {
    return path.join(this.getConfigDir(), 'models');
  },
  
  getDataDir() {
    return path.join(this.getConfigDir(), 'data');
  },
  
  getWindowStatePath() {
    return path.join(this.getConfigDir(), 'window-state.json');
  }
};

// Window state management - save and restore window position/size
const defaultWindowState = {
  width: 1400,
  height: 900,
  x: undefined,
  y: undefined,
  isMaximized: false
};

function loadWindowState() {
  try {
    const statePath = paths.getWindowStatePath();
    if (fs.existsSync(statePath)) {
      const data = fs.readFileSync(statePath, 'utf8');
      const state = JSON.parse(data);
      // Validate state has required fields
      if (typeof state.width === 'number' && typeof state.height === 'number') {
        return { ...defaultWindowState, ...state };
      }
    }
  } catch (err) {
    console.warn('Could not load window state:', err.message);
  }
  return { ...defaultWindowState };
}

function saveWindowState() {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  
  try {
    const isMaximized = mainWindow.isMaximized();
    const bounds = mainWindow.getBounds();
    
    const state = {
      width: bounds.width,
      height: bounds.height,
      x: bounds.x,
      y: bounds.y,
      isMaximized: isMaximized
    };
    
    // Ensure config directory exists
    const configDir = paths.getConfigDir();
    if (!fs.existsSync(configDir)) {
      fs.mkdirSync(configDir, { recursive: true });
    }
    
    fs.writeFileSync(paths.getWindowStatePath(), JSON.stringify(state, null, 2));
  } catch (err) {
    console.warn('Could not save window state:', err.message);
  }
}

// Wait for server to be ready with exponential backoff
function waitForServer(callback, maxAttempts = 60) {
  let attempts = 0;
  let delay = 500;
  const maxDelay = 3000;
  
  const check = async () => {
    const ready = await checkServer();
    if (ready) {
      callback();
    } else if (attempts < maxAttempts) {
      attempts++;
      delay = Math.min(delay * 1.2, maxDelay); // Exponential backoff
      setTimeout(check, delay);
    } else {
      // Server didn't start - show error
      if (mainWindow) {
        dialog.showMessageBox(mainWindow, {
          type: 'error',
          title: 'Server Timeout',
          message: 'OffGrid server did not respond in time',
          detail: 'The server may still be starting. Try refreshing the page in a few seconds.'
        });
      }
    }
  };
  setTimeout(check, 1000);
}

// Ensure directories exist
function ensureDirectories() {
  const dirs = [
    paths.getConfigDir(),
    paths.getModelsDir(),
    paths.getDataDir()
  ];
  
  for (const dir of dirs) {
    try {
      if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
      }
    } catch (err) {
      console.error(`Failed to create directory ${dir}:`, err.message);
    }
  }
}

// Check if server is running (with timeout and proper cleanup)
function checkServer() {
  return new Promise((resolve) => {
    const req = http.get(`${SERVER_URL}/v1/health`, (res) => {
      // Consume response data to free up memory
      res.resume();
      resolve(res.statusCode === 200);
    });
    
    req.on('error', () => resolve(false));
    req.setTimeout(2000, () => {
      req.destroy();
      resolve(false);
    });
  });
}

// Start OffGrid server
async function startOffgridServer() {
  const offgridBinary = paths.getOffgridBinary();
  
  if (!fs.existsSync(offgridBinary)) {
    console.error('OffGrid binary not found:', offgridBinary);
    dialog.showErrorBox(
      'Binary Not Found',
      `OffGrid binary not found at:\n${offgridBinary}\n\nPlease ensure the application is properly installed.`
    );
    return false;
  }

  // Check if server is already running
  const isRunning = await checkServer();
  if (isRunning) {
    console.log('OffGrid server is already running');
    return true;
  }

  console.log('Starting OffGrid server:', offgridBinary);
  
  try {
    // Make binary executable on Unix systems
    if (process.platform !== 'win32') {
      try {
        fs.chmodSync(offgridBinary, '755');
      } catch (err) {
        console.warn('Could not chmod binary:', err);
      }
    }

    offgridProcess = spawn(offgridBinary, ['server', 'start'], {
      stdio: 'pipe',
      env: {
        ...process.env,
        OFFGRID_PORT: SERVER_PORT.toString(),
        OFFGRID_HOME: paths.getConfigDir()
      },
      detached: false
    });

    offgridProcess.stdout.on('data', (data) => {
      console.log(`[OffGrid] ${data.toString().trim()}`);
    });

    offgridProcess.stderr.on('data', (data) => {
      console.error(`[OffGrid Error] ${data.toString().trim()}`);
    });

    offgridProcess.on('close', (code) => {
      console.log(`OffGrid server exited with code ${code}`);
      if (!isQuitting && mainWindow) {
        dialog.showMessageBox(mainWindow, {
          type: 'warning',
          title: 'Server Stopped',
          message: 'OffGrid server has stopped',
          detail: `Exit code: ${code}\n\nThe application may not function correctly.`
        });
      }
    });

    offgridProcess.on('error', (err) => {
      console.error('Failed to start OffGrid server:', err);
      dialog.showErrorBox(
        'Server Start Failed',
        `Failed to start OffGrid server:\n${err.message}`
      );
    });

    // Wait for server to be ready
    let attempts = 0;
    while (attempts < 30) {
      await new Promise(resolve => setTimeout(resolve, 500));
      const ready = await checkServer();
      if (ready) {
        console.log('OffGrid server is ready');
        return true;
      }
      attempts++;
    }

    console.warn('Server did not become ready in time');
    return false;

  } catch (err) {
    console.error('Error starting server:', err);
    return false;
  }
}

// Stop OffGrid server with proper cleanup
function stopOffgridServer() {
  return new Promise((resolve) => {
    if (!offgridProcess) {
      resolve();
      return;
    }
    
    console.log('Stopping OffGrid server...');
    
    // Set up a timeout for force kill
    const forceKillTimeout = setTimeout(() => {
      if (offgridProcess) {
        console.log('Force killing OffGrid server');
        try {
          offgridProcess.kill('SIGKILL');
        } catch (err) {
          // Process may already be dead
        }
        offgridProcess = null;
        resolve();
      }
    }, 5000);
    
    // Listen for the process to exit cleanly
    offgridProcess.once('exit', () => {
      clearTimeout(forceKillTimeout);
      offgridProcess = null;
      console.log('OffGrid server stopped');
      resolve();
    });
    
    // Send SIGTERM for graceful shutdown
    try {
      offgridProcess.kill('SIGTERM');
    } catch (err) {
      clearTimeout(forceKillTimeout);
      offgridProcess = null;
      resolve();
    }
  });
}

// Create main window with optimized settings
function createWindow() {
  // Load saved window state
  const windowState = loadWindowState();
  
  mainWindow = new BrowserWindow({
    width: windowState.width,
    height: windowState.height,
    x: windowState.x,
    y: windowState.y,
    minWidth: 1000,
    minHeight: 700,
    title: APP_NAME,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
      preload: path.join(__dirname, 'preload.js'),
      // Performance optimizations
      backgroundThrottling: true,
      spellcheck: false
    },
    icon: path.join(__dirname, 'assets/icon.png'),
    backgroundColor: '#1e1e1e',
    show: false,
    autoHideMenuBar: true
  });

  // Restore maximized state if applicable
  if (windowState.isMaximized) {
    mainWindow.maximize();
  }

  // Save window state on resize/move (debounced)
  let saveStateTimeout = null;
  const debouncedSaveState = () => {
    if (saveStateTimeout) clearTimeout(saveStateTimeout);
    saveStateTimeout = setTimeout(saveWindowState, 500);
  };
  
  mainWindow.on('resize', debouncedSaveState);
  mainWindow.on('move', debouncedSaveState);
  mainWindow.on('maximize', saveWindowState);
  mainWindow.on('unmaximize', saveWindowState);

  // Load the lightweight loading page first
  mainWindow.loadFile('loading.html');

  // Show window when ready (avoids white flash)
  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
    mainWindow.focus();
    
    // Once server is ready, load the actual web UI
    waitForServer(() => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.loadURL(`${SERVER_URL}/ui/`);
      }
    });
  });

  // Handle navigation errors gracefully
  mainWindow.webContents.on('did-fail-load', (event, errorCode, errorDescription) => {
    console.error('Page load failed:', errorCode, errorDescription);
    // Only show error for non-abort errors (user navigation cancels are -3)
    if (errorCode !== -3) {
      mainWindow.loadFile('loading.html');
    }
  });

  // Prevent close, minimize to tray instead
  mainWindow.on('close', (event) => {
    if (!isQuitting && process.platform !== 'darwin') {
      event.preventDefault();
      mainWindow.hide();
      
      // Show notification on first minimize (only once per session)
      if (!app.minimizedNotificationShown) {
        const { Notification } = require('electron');
        if (Notification.isSupported()) {
          new Notification({
            title: APP_NAME,
            body: 'Running in background. Click tray icon to restore.',
            silent: true
          }).show();
        }
        app.minimizedNotificationShown = true;
      }
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });

  // Memory optimization: reduce renderer memory when hidden
  mainWindow.on('hide', () => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.webContents.setBackgroundThrottling(true);
    }
  });

  mainWindow.on('show', () => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.webContents.setBackgroundThrottling(false);
    }
  });

  // Open DevTools only in development
  if (!app.isPackaged) {
    mainWindow.webContents.openDevTools();
  }
}

// Create system tray with optimized menu
function createTray() {
  const iconPath = path.join(__dirname, 'assets/icon.png');
  let trayIcon;
  
  try {
    trayIcon = nativeImage.createFromPath(iconPath);
    // Resize for optimal tray display
    if (!trayIcon.isEmpty()) {
      trayIcon = trayIcon.resize({ width: 16, height: 16 });
    }
  } catch (err) {
    console.warn('Could not load tray icon:', err.message);
    trayIcon = nativeImage.createEmpty();
  }

  tray = new Tray(trayIcon);
  
  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Show App',
      click: () => {
        if (mainWindow) {
          mainWindow.show();
          mainWindow.focus();
        } else {
          createWindow();
        }
      }
    },
    { type: 'separator' },
    {
      label: 'Server Status',
      enabled: false
    },
    {
      label: 'Check Server',
      click: async () => {
        const running = await checkServer();
        dialog.showMessageBox({
          type: running ? 'info' : 'warning',
          title: 'Server Status',
          message: running ? 'Server is running' : 'Server is not responding',
          detail: `Port: ${SERVER_PORT}`
        });
      }
    },
    { type: 'separator' },
    {
      label: 'Open Config Folder',
      click: () => {
        shell.openPath(paths.getConfigDir());
      }
    },
    {
      label: 'Open Models Folder',
      click: () => {
        shell.openPath(paths.getModelsDir());
      }
    },
    { type: 'separator' },
    {
      label: 'Quit',
      click: () => {
        isQuitting = true;
        app.quit();
      }
    }
  ]);

  tray.setToolTip(APP_NAME);
  tray.setContextMenu(contextMenu);

  tray.on('click', () => {
    if (mainWindow) {
      if (mainWindow.isVisible()) {
        mainWindow.hide();
      } else {
        mainWindow.show();
        mainWindow.focus();
      }
    } else {
      createWindow();
    }
  });
}

// IPC Handlers
ipcMain.handle('get-api-url', () => {
  return SERVER_URL;
});

ipcMain.handle('get-server-status', async () => {
  return await checkServer();
});

ipcMain.handle('select-directory', async () => {
  const result = await dialog.showOpenDialog(mainWindow, {
    properties: ['openDirectory'],
    title: 'Select Directory'
  });
  
  if (!result.canceled && result.filePaths.length > 0) {
    return result.filePaths[0];
  }
  return null;
});

ipcMain.handle('get-paths', () => {
  return {
    config: paths.getConfigDir(),
    models: paths.getModelsDir(),
    data: paths.getDataDir()
  };
});

// System theme support
ipcMain.handle('get-system-theme', () => {
  return nativeTheme.shouldUseDarkColors ? 'dark' : 'light';
});

// Notify renderer when system theme changes
nativeTheme.on('updated', () => {
  const theme = nativeTheme.shouldUseDarkColors ? 'dark' : 'light';
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.webContents.send('system-theme-changed', theme);
  }
});

// App lifecycle
app.whenReady().then(async () => {
  console.log(`${APP_NAME} starting...`);
  console.log(`Version: ${app.getVersion()} | Electron: ${process.versions.electron} | Platform: ${process.platform}`);
  console.log(`Packaged: ${app.isPackaged}`);
  
  // Set app user model ID for Windows notifications
  if (process.platform === 'win32') {
    app.setAppUserModelId(APP_NAME);
  }
  
  // Ensure directories exist
  ensureDirectories();
  
  // Create tray first (so user sees app is starting)
  createTray();
  
  // Start server
  const serverStarted = await startOffgridServer();
  
  if (!serverStarted) {
    console.warn('Server may not have started successfully');
  }
  
  // Create window (it will show loading page and wait for server)
  createWindow();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    } else if (mainWindow) {
      mainWindow.show();
    }
  });
});

app.on('window-all-closed', () => {
  // Keep running in tray on Windows/Linux
  // On macOS, only quit if explicitly quitting
  if (process.platform === 'darwin' && isQuitting) {
    app.quit();
  }
});

app.on('before-quit', async (event) => {
  isQuitting = true;
  
  // Clear any intervals
  if (serverCheckInterval) {
    clearInterval(serverCheckInterval);
    serverCheckInterval = null;
  }
});

app.on('will-quit', async (event) => {
  event.preventDefault();
  console.log('App quitting, stopping server...');
  await stopOffgridServer();
  
  // Destroy tray
  if (tray) {
    tray.destroy();
    tray = null;
  }
  
  app.exit(0);
});

// Handle uncaught errors gracefully
process.on('uncaughtException', (error) => {
  console.error('Uncaught exception:', error);
  // Only show dialog in packaged app (dev mode has better error handling)
  if (app.isPackaged) {
    dialog.showErrorBox('Application Error', `An unexpected error occurred:\n\n${error.message}`);
  }
});

process.on('unhandledRejection', (reason, promise) => {
  console.error('Unhandled promise rejection at:', promise, 'reason:', reason);
});
