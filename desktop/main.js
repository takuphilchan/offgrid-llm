const { app, BrowserWindow, Menu, Tray, dialog } = require('electron');
const { spawn } = require('child_process');
const path = require('path');
const http = require('http');

let mainWindow;
let serverProcess;
let tray;
const SERVER_PORT = 11611;
const SERVER_URL = `http://localhost:${SERVER_PORT}/ui/`;

// Determine the correct binary path based on platform and environment
function getServerBinaryPath() {
  const isDev = !app.isPackaged;
  const platform = process.platform;
  
  if (isDev) {
    // Development mode - use binary from parent directory
    const binaryName = platform === 'win32' ? 'offgrid.exe' : 'offgrid';
    return path.join(__dirname, '..', binaryName);
  } else {
    // Production mode - binary is in resources
    const binaryName = platform === 'win32' ? 'offgrid.exe' : 'offgrid';
    return path.join(process.resourcesPath, binaryName);
  }
}

// Start the Go server
function startServer() {
  return new Promise((resolve, reject) => {
    const serverBinary = getServerBinaryPath();
    
    console.log('Starting server from:', serverBinary);
    
    serverProcess = spawn(serverBinary, ['server'], {
      env: { ...process.env, PORT: SERVER_PORT.toString() }
    });

    serverProcess.stdout.on('data', (data) => {
      console.log(`Server: ${data}`);
    });

    serverProcess.stderr.on('data', (data) => {
      console.error(`Server Error: ${data}`);
    });

    serverProcess.on('error', (err) => {
      console.error('Failed to start server:', err);
      reject(err);
    });

    serverProcess.on('close', (code) => {
      console.log(`Server process exited with code ${code}`);
    });

    // Wait for server to be ready
    let attempts = 0;
    const maxAttempts = 30;
    
    const checkServer = setInterval(() => {
      attempts++;
      
      http.get(`http://localhost:${SERVER_PORT}/v1/health`, (res) => {
        if (res.statusCode === 200) {
          clearInterval(checkServer);
          console.log('Server is ready!');
          resolve();
        }
      }).on('error', (err) => {
        if (attempts >= maxAttempts) {
          clearInterval(checkServer);
          reject(new Error('Server failed to start after 30 seconds'));
        }
      });
    }, 1000);
  });
}

// Stop the server
function stopServer() {
  if (serverProcess) {
    console.log('Stopping server...');
    serverProcess.kill();
    serverProcess = null;
  }
}

// Create the main application window
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 1000,
    minHeight: 600,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true
    },
    icon: path.join(__dirname, 'icon.png'),
    title: 'OffGrid LLM',
    backgroundColor: '#0a0e1a',
    show: false // Don't show until ready
  });

  // Remove default menu
  Menu.setApplicationMenu(Menu.buildFromTemplate([
    {
      label: 'File',
      submenu: [
        {
          label: 'Reload',
          accelerator: 'CmdOrCtrl+R',
          click: () => mainWindow.reload()
        },
        { type: 'separator' },
        {
          label: 'Exit',
          accelerator: 'CmdOrCtrl+Q',
          click: () => app.quit()
        }
      ]
    },
    {
      label: 'View',
      submenu: [
        {
          label: 'Toggle Developer Tools',
          accelerator: 'CmdOrCtrl+Shift+I',
          click: () => mainWindow.webContents.toggleDevTools()
        }
      ]
    },
    {
      label: 'Help',
      submenu: [
        {
          label: 'About',
          click: () => {
            dialog.showMessageBox(mainWindow, {
              type: 'info',
              title: 'About OffGrid LLM',
              message: 'OffGrid LLM v0.1.0',
              detail: 'Local AI Assistant - Privacy First\n\nYour conversations are processed entirely on your device.'
            });
          }
        }
      ]
    }
  ]));

  // Show window when ready
  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
  });

  // Load the UI
  mainWindow.loadURL(SERVER_URL);

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

// Create system tray
function createTray() {
  tray = new Tray(path.join(__dirname, 'icon.png'));
  
  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Show OffGrid LLM',
      click: () => {
        if (mainWindow) {
          mainWindow.show();
        } else {
          createWindow();
        }
      }
    },
    { type: 'separator' },
    {
      label: 'Quit',
      click: () => app.quit()
    }
  ]);

  tray.setToolTip('OffGrid LLM');
  tray.setContextMenu(contextMenu);
  
  tray.on('click', () => {
    if (mainWindow) {
      mainWindow.show();
    }
  });
}

// App lifecycle
app.whenReady().then(async () => {
  try {
    console.log('Starting OffGrid LLM Desktop...');
    await startServer();
    createWindow();
    createTray();
  } catch (err) {
    console.error('Failed to start application:', err);
    dialog.showErrorBox(
      'Startup Error',
      `Failed to start OffGrid LLM server:\n\n${err.message}\n\nPlease check that the server binary is present.`
    );
    app.quit();
  }
});

app.on('window-all-closed', () => {
  // On macOS, keep app running in background
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('activate', () => {
  if (mainWindow === null) {
    createWindow();
  }
});

app.on('before-quit', () => {
  stopServer();
});

app.on('will-quit', () => {
  stopServer();
});

// Handle uncaught exceptions
process.on('uncaughtException', (err) => {
  console.error('Uncaught exception:', err);
  dialog.showErrorBox('Error', `An error occurred:\n\n${err.message}`);
});
