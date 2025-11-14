# OffGrid LLM Desktop Application

Desktop application for OffGrid LLM using Electron.

## Features

- Native desktop app (Windows, macOS, Linux)
- Auto-starts OffGrid and llama-server
- Uses your existing web UI
- Native file dialogs
- System tray integration

## Development

### Install

```bash
cd desktop
npm install
```

### Run

```bash
npm start
```

### Build

```bash
# Package for current platform
npm run pack

# Build distributable
npm run dist
```

## Structure

- `main.js` - Electron main process (starts servers, creates window)
- `preload.js` - Electron preload (exposes safe APIs to UI)
- `index.html` - Your existing web UI
- `package.json` - Config and dependencies

## How It Works

1. Starts llama-server (if available)
2. Starts OffGrid server
3. Opens window with your UI
4. UI connects to http://localhost:11611

Simple and clean - no React overhead!
