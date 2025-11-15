const { app, BrowserWindow, ipcMain, dialog } = require('electron');
const path = require('path');
const { spawn } = require('child_process');
const fs = require('fs');

let mainWindow;
let offgridProcess = null;
let llamaServerProcess = null;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 1000,
    minHeight: 700,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js')
    },
    icon: path.join(__dirname, 'assets/icon.png'),
    backgroundColor: '#0a0e1a',
    show: false,
    titleBarStyle: 'default',
    autoHideMenuBar: true
  });

  // Load the UI
  mainWindow.loadFile('index.html');

  // Show window when ready
  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
  });

  // Open DevTools in development
  if (!app.isPackaged) {
    mainWindow.webContents.openDevTools();
  }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

// Start OffGrid server
function startOffgridServer() {
  const offgridBinary = getOffgridBinaryPath();
  
  if (!fs.existsSync(offgridBinary)) {
    console.error('OffGrid binary not found:', offgridBinary);
    return;
  }

  console.log('Starting OffGrid server:', offgridBinary);
  offgridProcess = spawn(offgridBinary, ['server', 'start'], {
    stdio: 'pipe',
    env: { ...process.env }
  });

  offgridProcess.stdout.on('data', (data) => {
    console.log(`OffGrid: ${data}`);
  });

  offgridProcess.stderr.on('data', (data) => {
    console.error(`OffGrid Error: ${data}`);
  });

  offgridProcess.on('close', (code) => {
    console.log(`OffGrid server exited with code ${code}`);
  });
}

// Start llama-server
function startLlamaServer() {
  const llamaServerBinary = getLlamaServerBinaryPath();
  
  if (!fs.existsSync(llamaServerBinary)) {
    console.log('llama-server not found, skipping...');
    return;
  }

  const modelsDir = getModelsDir();
  if (!fs.existsSync(modelsDir)) {
    console.log('Models directory not found');
    return;
  }

  const models = fs.readdirSync(modelsDir).filter(f => f.endsWith('.gguf'));
  if (models.length === 0) {
    console.log('No models found');
    return;
  }

  const modelPath = path.join(modelsDir, models[0]);
  console.log('Starting llama-server with model:', modelPath);

  llamaServerProcess = spawn(llamaServerBinary, [
    '--model', modelPath,
    '--port', '42382',
    '--host', '127.0.0.1',
    '-c', '4096',
    '--threads', '4',
    '--metrics'
  ], {
    stdio: 'pipe'
  });

  llamaServerProcess.stdout.on('data', (data) => {
    console.log(`Llama: ${data}`);
  });

  llamaServerProcess.stderr.on('data', (data) => {
    console.error(`Llama Error: ${data}`);
  });

  llamaServerProcess.on('close', (code) => {
    console.log(`llama-server exited with code ${code}`);
  });
}

function getOffgridBinaryPath() {
  if (app.isPackaged) {
    const platform = process.platform;
    const ext = platform === 'win32' ? '.exe' : '';
    return path.join(process.resourcesPath, 'bin', `offgrid${ext}`);
  } else {
    const platform = process.platform;
    const basePath = path.join(__dirname, '../build');
    
    if (platform === 'win32') {
      return path.join(basePath, 'windows/offgrid.exe');
    } else if (platform === 'darwin') {
      return path.join(basePath, 'macos/offgrid');
    } else {
      return path.join(basePath, 'linux/offgrid');
    }
  }
}

function getLlamaServerBinaryPath() {
  const paths = [
    '/usr/local/bin/llama-server',
    '/usr/bin/llama-server',
    path.join(process.env.HOME || '', '.local/bin/llama-server')
  ];
  
  for (const p of paths) {
    if (fs.existsSync(p)) {
      return p;
    }
  }
  
  return 'llama-server';
}

function getModelsDir() {
  const home = process.env.HOME || process.env.USERPROFILE;
  return path.join(home, '.offgrid-llm/models');
}

// IPC Handlers
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

ipcMain.handle('get-api-url', () => {
  return 'http://localhost:11611';
});

// App lifecycle
app.whenReady().then(() => {
  // Start servers
  startLlamaServer();
  setTimeout(startOffgridServer, 2000);
  
  // Create window
  setTimeout(createWindow, 4000);

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('before-quit', () => {
  if (offgridProcess) {
    offgridProcess.kill();
  }
  if (llamaServerProcess) {
    llamaServerProcess.kill();
  }
});
