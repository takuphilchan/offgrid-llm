const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage } = require('electron');
const path = require('path');
const { spawn } = require('child_process');
const fs = require('fs');
const http = require('http');

let mainWindow = null;
let tray = null;
let offgridProcess = null;
let isQuitting = false;

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
  }
};

// Wait for server to be ready (helper function)
function waitForServer(callback, maxAttempts = 60) {
  let attempts = 0;
  const check = async () => {
    const ready = await checkServer();
    if (ready) {
      callback();
    } else if (attempts < maxAttempts) {
      attempts++;
      setTimeout(check, 500);
    }
  };
  setTimeout(check, 1000); // Start checking after 1 second
}

// Ensure directories exist
function ensureDirectories() {
  const dirs = [


    paths.getConfigDir(),
    paths.getModelsDir(),
    paths.getDataDir()
  ];
  
  dirs.forEach(dir => {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
  });
}

// Check if server is running
function checkServer() {
  return new Promise((resolve) => {
    const req = http.get(`${SERVER_URL}/v1/health`, (res) => {
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

// Stop OffGrid server
function stopOffgridServer() {
  if (offgridProcess) {
    console.log('Stopping OffGrid server');
    offgridProcess.kill('SIGTERM');
    
    // Force kill after 5 seconds if not stopped
    setTimeout(() => {
      if (offgridProcess) {
        console.log('Force killing OffGrid server');
        offgridProcess.kill('SIGKILL');
      }
    }, 5000);
  }
}

// Create main window
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 1000,
    minHeight: 700,
    title: APP_NAME,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js')
    },
    icon: path.join(__dirname, 'assets/icon.png'),
    backgroundColor: '#0f172a',
    show: false,
    autoHideMenuBar: true
  });

  // Load the web UI from the server once it's ready
  // Show loading page first
  mainWindow.loadFile('index.html');

  // Show window when ready
  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
    mainWindow.focus();
    
    // Once server is ready, load the actual web UI
    waitForServer(() => {
      mainWindow.loadURL(`${SERVER_URL}/ui/`);
    });
  });

  // Prevent close, minimize to tray instead
  mainWindow.on('close', (event) => {
    if (!isQuitting && process.platform !== 'darwin') {
      event.preventDefault();
      mainWindow.hide();
      
      // Show notification on first minimize
      if (!app.minimizedNotificationShown) {
        const { Notification } = require('electron');
        if (Notification.isSupported()) {
          new Notification({
            title: APP_NAME,
            body: 'App is running in the background. Click the tray icon to open.'
          }).show();
        }
        app.minimizedNotificationShown = true;
      }
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });

  // Open DevTools in development
  if (!app.isPackaged) {
    mainWindow.webContents.openDevTools();
  }
}

// Create system tray
function createTray() {
  const iconPath = path.join(__dirname, 'assets/icon.png');
  let trayIcon;
  
  try {
    trayIcon = nativeImage.createFromPath(iconPath);
    if (trayIcon.isEmpty()) {
      // Fallback to default icon
      trayIcon = nativeImage.createEmpty();
    }
  } catch (err) {
    console.warn('Could not load tray icon:', err);
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
        require('electron').shell.openPath(paths.getConfigDir());
      }
    },
    {
      label: 'Open Models Folder',
      click: () => {
        require('electron').shell.openPath(paths.getModelsDir());
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

// App lifecycle
app.whenReady().then(async () => {
  console.log(`${APP_NAME} starting...`);
  console.log('App version:', app.getVersion());
  console.log('Electron version:', process.versions.electron);
  console.log('Platform:', process.platform);
  console.log('Packaged:', app.isPackaged);
  
  // Ensure directories exist
  ensureDirectories();
  
  // Create tray
  createTray();
  
  // Start server
  const serverStarted = await startOffgridServer();
  
  if (!serverStarted) {
    console.warn('Server may not have started successfully');
  }
  
  // Wait a bit for server to stabilize
  await new Promise(resolve => setTimeout(resolve, 2000));
  
  // Create window
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
  // Don't quit on window close - keep running in tray
  if (process.platform === 'darwin') {
    // On macOS, it's common to quit when all windows are closed
    if (isQuitting) {
      app.quit();
    }
  }
});

app.on('before-quit', () => {
  isQuitting = true;
});

app.on('will-quit', () => {
  console.log('App quitting, stopping server...');
  stopOffgridServer();
});

// Handle uncaught errors
process.on('uncaughtException', (error) => {
  console.error('Uncaught exception:', error);
  dialog.showErrorBox('Application Error', `An unexpected error occurred:\n\n${error.message}`);
});

process.on('unhandledRejection', (error) => {
  console.error('Unhandled promise rejection:', error);
});
