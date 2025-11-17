# Desktop Application Build and Installation Guide

## Overview

OffGrid LLM Desktop is a cross-platform desktop application that provides a user-friendly interface for running large language models locally. Similar to Docker Desktop, it manages the backend server automatically and runs in the system tray.

## Features

- **Automatic Server Management**: Starts and stops the OffGrid backend automatically
- **System Tray Integration**: Minimize to tray and keep running in background
- **Cross-Platform**: Runs on macOS, Linux (multiple distros), and Windows
- **Easy Installation**: Double-click installers for each platform
- **Bundled Binary**: Includes the OffGrid CLI binary, no separate installation needed
- **Native Look**: Uses native OS controls and styling

## Quick Install

### Linux/macOS
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

### Windows (PowerShell as Administrator)
```powershell
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

## Manual Installation

### macOS

1. Download `OffGrid-LLM-Desktop-{version}-{arch}.dmg` from [releases](https://github.com/takuphilchan/offgrid-llm/releases)
2. Open the DMG file
3. Drag "OffGrid LLM Desktop" to Applications folder
4. Launch from Applications or Launchpad

**Note**: On first launch, you may need to right-click and select "Open" due to Gatekeeper.

### Linux

#### Ubuntu/Debian (recommended)
```bash
wget https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.3/OffGrid-LLM-Desktop-0.1.3-amd64.deb
sudo dpkg -i OffGrid-LLM-Desktop-0.1.3-amd64.deb
sudo apt-get install -f  # Install dependencies if needed
```

#### AppImage (universal)
```bash
wget https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.3/OffGrid-LLM-Desktop-0.1.3-x86_64.AppImage
chmod +x OffGrid-LLM-Desktop-0.1.3-x86_64.AppImage
./OffGrid-LLM-Desktop-0.1.3-x86_64.AppImage
```

#### Fedora/RHEL/CentOS
```bash
wget https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.3/OffGrid-LLM-Desktop-0.1.3-x86_64.rpm
sudo rpm -i OffGrid-LLM-Desktop-0.1.3-x86_64.rpm
```

### Windows

1. Download `OffGrid-LLM-Desktop-Setup-{version}.exe` from [releases](https://github.com/takuphilchan/offgrid-llm/releases)
2. Run the installer
3. Follow the installation wizard
4. Launch from Start Menu or Desktop shortcut

**Portable Version**: Also available as `OffGrid-LLM-Desktop-{version}-Portable.exe` (no installation required)

## Building from Source

### Prerequisites

- Node.js 16+ and npm
- Go 1.21+ (for building CLI binaries)
- Platform-specific build tools:
  - **macOS**: Xcode Command Line Tools
  - **Linux**: Standard build tools (gcc, make)
  - **Windows**: Visual Studio Build Tools or MinGW

### Build Everything

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Build everything (CLI + Desktop for all platforms)
./build-all.sh --all

# Or build for specific platform
./build-all.sh --platform linux --desktop
```

### Build Desktop Only

```bash
cd desktop

# Install dependencies
npm install

# Build for current platform
npm run build

# Build for specific platform
npm run build:linux   # Linux (AppImage, .deb)
npm run build:mac     # macOS (.dmg)
npm run build:win     # Windows (.exe installer)

# Build for all platforms
npm run build:all
```

### Development Mode

```bash
cd desktop
npm install
npm start
```

This will:
1. Use the binary from `../build/{platform}/offgrid`
2. Open DevTools automatically
3. Enable hot reload for UI changes

## Package Structure

After building, you'll find installers in `desktop/dist/`:

```
desktop/dist/
├── OffGrid-LLM-Desktop-0.1.3-x86_64.AppImage      # Linux universal
├── OffGrid-LLM-Desktop-0.1.3-amd64.deb            # Debian/Ubuntu
├── OffGrid-LLM-Desktop-0.1.3-x86_64.rpm           # Fedora/RHEL
├── OffGrid-LLM-Desktop-0.1.3-arm64.dmg            # macOS Apple Silicon
├── OffGrid-LLM-Desktop-0.1.3-x64.dmg              # macOS Intel
├── OffGrid-LLM-Desktop-Setup-0.1.3.exe            # Windows installer
└── OffGrid-LLM-Desktop-0.1.3-Portable.exe         # Windows portable
```

## Configuration

### Config Directory

The app stores configuration and models in:
- **macOS/Linux**: `~/.offgrid-llm/`
- **Windows**: `%USERPROFILE%\.offgrid-llm\`

Structure:
```
~/.offgrid-llm/
├── models/          # Downloaded GGUF models
├── data/            # Application data
└── config.json      # Configuration file
```

### Server Settings

The app starts the OffGrid server on port `11611` by default. You can change this by setting the `OFFGRID_PORT` environment variable before launching the app.

## Usage

### System Tray

The app runs in the system tray with the following options:
- **Show App**: Open the main window
- **Check Server**: Verify backend server status
- **Open Config Folder**: Open config directory in file manager
- **Open Models Folder**: Open models directory
- **Quit**: Stop server and quit application

### Minimize to Tray

Close the window to minimize to tray (app keeps running). Right-click the tray icon to quit completely.

### Download Models

Use the Models tab in the app or the CLI:
```bash
# If CLI is installed
offgrid download llama-2-7b-chat

# Models will be stored in ~/.offgrid-llm/models/
```

## Troubleshooting

### App won't start

**Check if port is in use**:
```bash
# Linux/macOS
lsof -i :11611

# Windows
netstat -ano | findstr :11611
```

**Check logs**:
- **macOS**: Console.app → filter for "OffGrid"
- **Linux**: Run from terminal to see output
- **Windows**: Event Viewer → Application logs

### Binary not found error

This means the bundled binary is missing or not executable:

1. Check if binary exists:
   ```bash
   # macOS
   ls -la "/Applications/OffGrid LLM Desktop.app/Contents/Resources/bin/"
   
   # Linux (AppImage)
   # Extract and check: ./OffGrid-LLM-Desktop.AppImage --appimage-extract
   
   # Windows
   dir "%ProgramFiles%\OffGrid LLM Desktop\resources\bin\"
   ```

2. Ensure it's executable (Linux/macOS):
   ```bash
   chmod +x ~/.local/bin/OffGrid-LLM-Desktop.AppImage
   ```

### Server fails to start

1. Check if another instance is running
2. Look for port conflicts (port 11611)
3. Check file permissions on config directory
4. Try running the CLI directly to see errors:
   ```bash
   # macOS/Linux
   ~/.offgrid-llm/bin/offgrid server start
   
   # Windows
   %PROGRAMFILES%\OffGrid\offgrid.exe server start
   ```

### Models don't load

1. Ensure models are in `~/.offgrid-llm/models/`
2. Verify they are in GGUF format
3. Check file permissions
4. Try downloading through the app's Models tab

## Uninstallation

### macOS
```bash
# Remove application
rm -rf "/Applications/OffGrid LLM Desktop.app"

# Remove config (optional)
rm -rf ~/.offgrid-llm
```

### Linux

**Debian/Ubuntu**:
```bash
sudo apt-get remove offgrid-llm-desktop
rm -rf ~/.offgrid-llm  # Optional
```

**AppImage**:
```bash
rm ~/.local/bin/OffGrid-LLM-Desktop.AppImage
rm ~/.local/share/applications/offgrid-llm-desktop.desktop
rm -rf ~/.offgrid-llm  # Optional
```

**RPM**:
```bash
sudo rpm -e offgrid-llm-desktop
rm -rf ~/.offgrid-llm  # Optional
```

### Windows
1. Go to Settings → Apps → Apps & features
2. Find "OffGrid LLM Desktop"
3. Click Uninstall
4. Optionally delete `%USERPROFILE%\.offgrid-llm`

## Support

- **Issues**: https://github.com/takuphilchan/offgrid-llm/issues
- **Discussions**: https://github.com/takuphilchan/offgrid-llm/discussions
- **Documentation**: https://github.com/takuphilchan/offgrid-llm

## License

See [LICENSE](../LICENSE) file in the main repository.
