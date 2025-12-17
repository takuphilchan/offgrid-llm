# OffGrid LLM Desktop App

Native desktop application for OffGrid LLM using Electron.

## Features

- Native system tray integration
- Auto-start with system (optional)
- Native notifications
- Embedded OffGrid server
- Cross-platform (Linux, macOS, Windows)

## File Structure

```
desktop/
├── index.html          # UI entry point (synced from web/ui)
├── main.js             # Electron main process
├── preload.js          # Preload scripts
├── package.json        # Dependencies
├── css/
│   └── styles.css      # Synced from web/ui
├── js/                 # Synced from web/ui
│   ├── utils.js
│   ├── chat.js
│   └── ...             # (16 JS modules)
└── assets/
    └── ...             # App icons, etc.
```

## Syncing UI Changes

The desktop UI is synced from `web/ui`. After making changes to the web UI:

```bash
./scripts/sync-ui.sh
```

## Building

### Prerequisites

```bash
# Install Node.js 18+
# https://nodejs.org

# Install dependencies
cd desktop
npm install
```

### Development

```bash
npm start
```

### Build for Distribution

```bash
# Linux
npm run build-linux

# macOS
npm run build-mac

# Windows
npm run build-win
```

## Development

```bash
npm start
```

The app will start the embedded OffGrid server on localhost:11611.
