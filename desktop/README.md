# OffGrid LLM Desktop App

Cross-platform desktop application for OffGrid LLM.

## Quick Start

### Development Mode

1. Install dependencies:
```bash
cd desktop
npm install
```

2. Build the Go server (from root directory):
```bash
cd ..
make build
```

3. Run the desktop app:
```bash
cd desktop
npm start
```

### Build for Production

Build for your current platform:
```bash
npm run build
```

Build for specific platforms:
```bash
npm run build:win      # Windows (NSIS installer + portable)
npm run build:mac      # macOS (DMG + ZIP)
npm run build:linux    # Linux (AppImage, DEB, RPM)
```

Build for all platforms (requires appropriate build tools):
```bash
npm run build:all
```

## Output

Built applications will be in the `desktop/dist/` directory:

### Windows
- `OffGrid LLM-0.1.0-win-x64.exe` - NSIS installer
- `OffGrid LLM-0.1.0-win-x64-portable.exe` - Portable version

### macOS
- `OffGrid LLM-0.1.0-mac-x64.dmg` - DMG installer
- `OffGrid LLM-0.1.0-mac-x64.zip` - ZIP archive

### Linux
- `OffGrid LLM-0.1.0-linux-x64.AppImage` - AppImage (universal)
- `OffGrid LLM-0.1.0-linux-x64.deb` - Debian/Ubuntu package
- `OffGrid LLM-0.1.0-linux-x64.rpm` - RedHat/Fedora package

## Features

- ✅ **Fully Offline** - No internet connection required
- ✅ **System Tray** - Runs in background
- ✅ **Auto-start Server** - Go backend starts automatically
- ✅ **Native Menus** - Platform-specific menus
- ✅ **Auto-updates** - Can be configured for updates
- ✅ **Cross-platform** - Windows, macOS, Linux

## Architecture

The desktop app consists of:

1. **Electron Frontend** - Provides native window and system integration
2. **Go Backend** - Your existing OffGrid LLM server (bundled as resource)
3. **Web UI** - Your HTML/CSS/JS interface served by Go server

On startup:
1. Electron launches
2. Spawns the Go server process
3. Waits for server to be ready (health check)
4. Opens window pointing to `http://localhost:11611/ui/`
5. On exit, cleanly shuts down the server

## System Requirements

- **Windows**: Windows 7 or later
- **macOS**: macOS 10.11 (El Capitan) or later
- **Linux**: Modern distributions with glibc 2.17+

## Development Notes

The app runs in two modes:

- **Development**: Uses `../offgrid` binary from parent directory
- **Production**: Uses bundled binary from `resources/` folder

## Customization

### Change App Icon

Replace `desktop/icon.png` with your own icon (512x512 PNG recommended).

### Modify Window Size

Edit `desktop/main.js`:
```javascript
mainWindow = new BrowserWindow({
  width: 1400,   // Change width
  height: 900,   // Change height
  // ...
});
```

### Change Server Port

Edit `desktop/main.js`:
```javascript
const SERVER_PORT = 11611; // Change port
```

## Troubleshooting

### Server fails to start
- Ensure `offgrid` binary is built: `cd .. && make build`
- Check binary has execute permissions: `chmod +x offgrid`

### Build fails
- Install electron-builder globally: `npm install -g electron-builder`
- Ensure you have platform-specific build tools installed

### macOS build on non-Mac
- macOS apps can only be built on macOS
- Use GitHub Actions or similar for cross-platform builds

## License

Same as OffGrid LLM project.
