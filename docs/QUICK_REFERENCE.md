# Quick Reference: Cross-Platform Distribution

## Installation Methods

### For End Users

#### Linux/macOS - One-Line Install
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

#### Windows - PowerShell
```powershell
# Download release, then:
cd offgrid
powershell -ExecutionPolicy Bypass -File install.ps1
```

### For Developers

```bash
# Build for all platforms
make cross-compile

# Create release packages
make release VERSION=0.1.0
```

## Directory Structure

```
Project Structure:
├── build/                      # Build & packaging scripts
│   ├── macos/
│   │   ├── create-app-bundle.sh
│   │   └── create-dmg.sh
│   ├── windows/
│   │   └── installer.nsi       # NSIS installer script
│   └── linux/                  # Future: .deb, .rpm
├── installers/
│   ├── install.sh              # Universal installer
│   ├── install-macos.sh
│   └── install-windows.ps1
├── internal/platform/          # Platform detection code
│   └── platform.go
└── .github/workflows/
    └── release.yml             # Automated builds
```

## Platform-Specific Paths

### Linux
- **Config**: `~/.config/offgrid/`
- **Data**: `~/.local/share/offgrid/`
- **Cache**: `~/.cache/offgrid/`
- **Logs**: `~/.local/state/offgrid/`
- **Install**: `/usr/local/bin/`

### macOS
- **Config**: `~/Library/Application Support/OffGrid/`
- **Data**: `~/Library/Application Support/OffGrid/Data/`
- **Cache**: `~/Library/Caches/OffGrid/`
- **Logs**: `~/Library/Logs/OffGrid/`
- **Install**: `/usr/local/bin/` or `/Applications/OffGrid.app/`

### Windows
- **Config**: `%APPDATA%\OffGrid\`
- **Data**: `%LOCALAPPDATA%\OffGrid\`
- **Cache**: `%LOCALAPPDATA%\OffGrid\Cache\`
- **Logs**: `%LOCALAPPDATA%\OffGrid\Logs\`
- **Install**: `C:\Program Files\OffGrid\`

## Release Process

### 1. Create Release Tag
```bash
# Update version
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

### 2. GitHub Actions Builds
- Automatically triggers on tag push
- Builds for all platforms
- Creates GitHub Release

### 3. What Gets Built

| Platform | Artifacts |
|----------|-----------|
| Linux x64 | `offgrid-v0.1.0-linux-amd64.tar.gz` |
| Linux ARM64 | `offgrid-v0.1.0-linux-arm64.tar.gz` |
| macOS Intel | `offgrid-v0.1.0-darwin-amd64.dmg` |
| macOS Silicon | `offgrid-v0.1.0-darwin-arm64.dmg` |
| Windows x64 | `offgrid-v0.1.0-windows-amd64.zip` |
| Windows ARM | `offgrid-v0.1.0-windows-arm64.zip` |

## Key Files

### Installers
- `installers/install.sh` - Universal (auto-detects OS)
- `installers/install-macos.sh` - macOS only
- `installers/install-windows.ps1` - Windows only

### Build Scripts
- `build/macos/create-app-bundle.sh` - Creates OffGrid.app
- `build/macos/create-dmg.sh` - Creates .dmg file
- `build/windows/installer.nsi` - NSIS installer config

### CI/CD
- `.github/workflows/release.yml` - Automated builds

### Platform Code
- `internal/platform/platform.go` - Cross-platform paths & detection

## Makefile Targets

```bash
make build              # Build for current platform
make cross-compile      # Build for all platforms
make release           # Create release packages
make clean             # Clean build artifacts
```

## Testing Locally

```bash
# 1. Build
make cross-compile

# 2. Test binary
./dist/offgrid-linux-amd64 --version

# 3. Test installer
cd dist
tar -xzf offgrid-0.1.0-linux-amd64.tar.gz
cd offgrid-0.1.0-linux-amd64
sudo ./install.sh

# 4. Verify
offgrid --version
```

## Common Issues

### macOS: "Unidentified Developer"
```bash
# Option 1: Sign the app (requires Apple Developer account)
codesign --deep --force --sign "Developer ID" OffGrid.app

# Option 2: Allow in System Preferences
# Right-click > Open > Open anyway
```

### Windows: SmartScreen Warning
```powershell
# Option 1: Sign the executable (requires code signing cert)
signtool sign /f cert.pfx offgrid.exe

# Option 2: Click "More info" > "Run anyway"
```

### Linux: Permission Denied
```bash
# Make installer executable
chmod +x install.sh

# Run with sudo for system-wide install
sudo ./install.sh
```

## Environment Variables

### Override Paths
```bash
export OFFGRID_CONFIG_DIR=/custom/config
export OFFGRID_DATA_DIR=/custom/data
export OFFGRID_CACHE_DIR=/custom/cache
export OFFGRID_INSTALL_DIR=/custom/install
```

### Version Selection
```bash
export OFFGRID_VERSION=v0.2.0  # Install specific version
```

## Documentation

- **Building**: `docs/BUILDING.md` - Full build instructions
- **Distribution**: `docs/DISTRIBUTION_STRATEGY.md` - Strategy overview
- **Installation**: `README.md` - User-facing instructions

## Quick Commands Cheat Sheet

```bash
# Development
make build                  # Build locally
make run                    # Build and run
make test                   # Run tests

# Release
make cross-compile          # All platforms
make release VERSION=X.Y.Z  # Package releases
git tag vX.Y.Z             # Tag release
git push origin vX.Y.Z     # Trigger CI

# Installation (End Users)
curl -fsSL ... | bash      # Linux/macOS
powershell install.ps1     # Windows

# Verification
offgrid --version
offgrid --help
offgrid doctor             # Check system
```

## Support Matrix

| OS | Version | Arch | Status |
|----|---------|------|--------|
| Ubuntu | 20.04+ | x64 | Supported |
| Ubuntu | 20.04+ | ARM64 | Supported |
| Debian | 11+ | x64 | Supported |
| macOS | 11+ | Intel | Supported |
| macOS | 11+ | Apple Silicon | Supported |
| Windows | 10+ | x64 | Supported |
| Windows | 11+ | ARM64 | Experimental |

## Next Steps

1. Cross-compilation working
2. GitHub Actions workflow configured
3. Platform-specific installers created
4. Test on all platforms
5. Get code signing certificates (optional)
6. Create first release
7. Publish to package managers (Homebrew, Chocolatey, etc.)
