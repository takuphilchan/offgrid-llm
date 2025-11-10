# Desktop App Quick Start

## Prerequisites

- Node.js 16+ and npm
- Go 1.21+ (for building the server)
- Git

## Development Setup (5 minutes)

### 1. Clone and Build Server
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
make build
```

### 2. Install Desktop Dependencies
```bash
cd desktop
npm install
```

### 3. Run Desktop App
```bash
npm start
```

The app will:
1. Start the Go server automatically
2. Wait for it to be ready
3. Open the UI in a native window
4. Show a system tray icon

## Building Installers

### For Your Current Platform
```bash
npm run build
```

### For Specific Platforms
```bash
npm run build:win      # Windows
npm run build:mac      # macOS  
npm run build:linux    # Linux
```

### Using the Build Script
```bash
./build.sh              # Current platform
./build.sh windows      # Windows only
./build.sh all          # All platforms
```

## Platform-Specific Notes

### Windows
- Builds NSIS installer (.exe) and portable version
- Requires Windows 7 or later
- First run may show Windows Defender warning (normal for unsigned apps)

### macOS
- Builds DMG and ZIP
- Requires macOS 10.11 or later
- App needs to be "allowed" in Security & Privacy settings (unsigned)
- Cross-compilation from non-Mac requires Xcode tools

### Linux
- Builds AppImage (universal), DEB, and RPM
- AppImage works on most distributions
- DEB for Debian/Ubuntu
- RPM for Fedora/RHEL/CentOS

## Troubleshooting

### "Server failed to start"
- Make sure Go binary is built: `cd .. && make build`
- Check binary permissions: `chmod +x ../offgrid`

### "Module not found" errors
- Delete `node_modules` and reinstall: `rm -rf node_modules && npm install`

### Build fails
- Update electron-builder: `npm install -g electron-builder@latest`
- Check you have required build tools for your platform

### Port already in use
- Stop any running OffGrid server: `pkill offgrid`
- Or change port in `main.js` (default: 11611)

## File Structure

```
desktop/
├── main.js           # Electron main process
├── preload.js        # Preload script
├── package.json      # Dependencies & build config
├── build.sh          # Build helper script
├── icon.png          # App icon (512x512)
└── dist/            # Built installers (created during build)
```

## Customization

### Change Window Size
Edit `main.js`:
```javascript
width: 1400,   // Change width
height: 900,   // Change height
```

### Change App Icon
Replace `icon.png` with your own 512x512 PNG image.

### Change Server Port
Edit `main.js`:
```javascript
const SERVER_PORT = 11611; // Your custom port
```

## Distribution

After building, the installers in `dist/` can be:
- Shared directly with users
- Uploaded to GitHub Releases
- Distributed via your website
- Signed and notarized (for production)

**Note**: Unsigned apps will show security warnings. For production, sign your apps:
- Windows: Use code signing certificate
- macOS: Use Apple Developer ID
- Linux: Generally no signing required

## Next Steps

1. Add auto-updater (see Electron docs)
2. Code sign for production distribution
3. Set up CI/CD for automated builds
4. Add crash reporting
5. Implement app-specific features

## Support

- Desktop issues: [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)
- Electron docs: https://www.electronjs.org/docs
- electron-builder: https://www.electron.build/
