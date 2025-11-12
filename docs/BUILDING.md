# Building and Releasing OffGrid LLM

This document explains how to build, package, and release OffGrid LLM for all supported platforms.

## Quick Start

### Build for Current Platform

```bash
make build
```

### Build for All Platforms

```bash
make cross-compile
```

### Create Release Packages

```bash
make release VERSION=0.1.0
```

## Supported Platforms

| Platform | Architecture | Package Format |
|----------|--------------|----------------|
| Linux    | x86_64       | `.tar.gz`, `.deb`, `.rpm` |
| Linux    | ARM64        | `.tar.gz` |
| macOS    | Intel        | `.dmg`, `.tar.gz` |
| macOS    | Apple Silicon| `.dmg`, `.tar.gz` |
| Windows  | x86_64       | `.exe` installer, `.zip` |
| Windows  | ARM64        | `.zip` |

## Build System Overview

### Directory Structure

```
offgrid-llm/
├── build/                  # Build scripts and packaging
│   ├── macos/
│   │   ├── create-app-bundle.sh
│   │   └── create-dmg.sh
│   ├── windows/
│   │   └── installer.nsi
│   └── linux/
├── installers/             # Installation scripts
│   ├── install.sh          # Universal installer
│   ├── install-macos.sh
│   ├── install-windows.ps1
│   └── (install.sh is the original Linux installer)
├── dist/                   # Build outputs (generated)
└── .github/
    └── workflows/
        └── release.yml     # Automated CI/CD
```

## Manual Building

### 1. Cross-Compile Binaries

```bash
# Build for all platforms
make cross-compile

# Outputs in dist/:
# - offgrid-linux-amd64
# - offgrid-linux-arm64
# - offgrid-darwin-amd64
# - offgrid-darwin-arm64
# - offgrid-windows-amd64.exe
# - offgrid-windows-arm64.exe
```

### 2. Create Release Archives

```bash
# Create .tar.gz and .zip archives
make release VERSION=0.1.0

# Outputs:
# - offgrid-0.1.0-linux-amd64.tar.gz
# - offgrid-0.1.0-darwin-arm64.tar.gz
# - offgrid-0.1.0-windows-amd64.zip
# etc.
```

## Platform-Specific Packaging

### macOS DMG

```bash
# 1. Build binary
GOOS=darwin GOARCH=arm64 go build -o offgrid-darwin-arm64 ./cmd/offgrid

# 2. Download llama-server for macOS (or build from source)
# Place in same directory as offgrid binary

# 3. Create app bundle
cd build/macos
./create-app-bundle.sh ../../offgrid-darwin-arm64 ../../llama-server v0.1.0

# 4. Create DMG
./create-dmg.sh v0.1.0 arm64

# Output: offgrid-v0.1.0-darwin-arm64.dmg
```

### Windows Installer (NSIS)

Requirements:
- NSIS installed (`choco install nsis`)
- EnVar plugin for NSIS

```bash
# 1. Build Windows binary
GOOS=windows GOARCH=amd64 go build -o dist/offgrid-windows-amd64.exe ./cmd/offgrid

# 2. Place llama-server.exe in dist/

# 3. Build installer
cd build/windows
makensis installer.nsi

# Output: OffGridSetup-0.1.0.exe
```

### Linux Packages

#### Debian/Ubuntu (.deb)

```bash
# Coming soon - see build/linux/
```

#### RedHat/Fedora (.rpm)

```bash
# Coming soon - see build/linux/
```

## Automated Releases (GitHub Actions)

### Trigger a Release

#### Method 1: Git Tag (Recommended)

```bash
# Tag the release
git tag -a v0.1.0 -m "Release v0.1.0"

# Push tag to trigger workflow
git push origin v0.1.0
```

This automatically:
1. Builds llama.cpp for all platforms
2. Builds OffGrid binaries for all platforms
3. Creates platform-specific packages
4. Generates checksums
5. Creates GitHub Release with all artifacts

#### Method 2: Manual Workflow Dispatch

From GitHub:
1. Go to Actions → Release Build
2. Click "Run workflow"
3. Enter version (e.g., `v0.1.0`)
4. Click "Run workflow"

### What Gets Built

The workflow creates these artifacts:

#### Linux
- `offgrid-v0.1.0-linux-amd64.tar.gz`
- `offgrid-v0.1.0-linux-arm64.tar.gz`

Each contains:
- `bin/offgrid`
- `bin/llama-server`
- `install.sh`
- `README.md`
- `LICENSE`

#### macOS
- `offgrid-v0.1.0-darwin-amd64.dmg`
- `offgrid-v0.1.0-darwin-arm64.dmg`

Each DMG contains:
- `OffGrid.app` (application bundle)
- Symlink to Applications folder
- `README.txt`

#### Windows
- `offgrid-v0.1.0-windows-amd64.zip`
- `offgrid-v0.1.0-windows-arm64.zip`

Each contains:
- `offgrid.exe`
- `llama-server.exe`
- `install.ps1`
- `README.md`
- `LICENSE`

#### Checksums
- `checksums.txt` - SHA256 hashes for all files

## Installation Instructions

### End User Installation

#### One-Line Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

#### Manual Install

##### Linux/macOS
```bash
# Download and extract
wget https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.0/offgrid-v0.1.0-linux-amd64.tar.gz
tar -xzf offgrid-v0.1.0-linux-amd64.tar.gz
cd offgrid-v0.1.0-linux-amd64

# Install
sudo ./install.sh
```

##### macOS
```bash
# Download DMG
curl -LO https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.0/offgrid-v0.1.0-darwin-arm64.dmg

# Open and drag to Applications
open offgrid-v0.1.0-darwin-arm64.dmg

# Or run the install script
/Volumes/OffGrid\ LLM/OffGrid.app/Contents/Resources/install.sh
```

##### Windows
```powershell
# Download and extract
Invoke-WebRequest -Uri "https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.0/offgrid-v0.1.0-windows-amd64.zip" -OutFile offgrid.zip
Expand-Archive offgrid.zip -DestinationPath offgrid
cd offgrid

# Install (requires Admin)
powershell -ExecutionPolicy Bypass -File install.ps1
```

## Code Signing (Optional)

### macOS

```bash
# Sign the app
codesign --deep --force --verify --verbose \
  --sign "Developer ID Application: Your Name (TEAM_ID)" \
  OffGrid.app

# Notarize with Apple
xcrun notarytool submit offgrid-v0.1.0-darwin-arm64.dmg \
  --apple-id "your@email.com" \
  --team-id "TEAM_ID" \
  --password "app-specific-password"

# Staple notarization ticket
xcrun stapler staple OffGrid.app
```

Requirements:
- Apple Developer account ($99/year)
- Developer ID certificate
- App-specific password

### Windows

```powershell
# Sign the executable
signtool sign /f certificate.pfx /p password `
  /tr http://timestamp.digicert.com /td sha256 `
  offgrid.exe

# Sign the installer
signtool sign /f certificate.pfx /p password `
  /tr http://timestamp.digicert.com /td sha256 `
  OffGridSetup.exe
```

Requirements:
- Code signing certificate ($100-500/year)
- signtool (Windows SDK)

## Testing Releases

### Test Locally Before Pushing

```bash
# 1. Build everything
make cross-compile

# 2. Test on current platform
./dist/offgrid-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m) --version

# 3. Create packages
make release VERSION=0.1.0-test

# 4. Test installation
cd dist
tar -xzf offgrid-0.1.0-test-linux-amd64.tar.gz
cd offgrid-0.1.0-test-linux-amd64
sudo ./install.sh
```

### Test in Clean Environment

Use Docker for Linux:

```bash
# Test Ubuntu installation
docker run --rm -it ubuntu:22.04 bash
# Inside container:
curl -fsSL https://your-test-url/install.sh | bash
offgrid --version
```

## Troubleshooting

### Build Fails on macOS

```bash
# Install Xcode Command Line Tools
xcode-select --install

# Install Homebrew dependencies
brew install create-dmg
```

### Cross-Compilation Issues

```bash
# Ensure Go version is correct
go version  # Should be 1.21 or later

# Clean and rebuild
make clean
rm -rf dist/
make cross-compile
```

### Windows Installer Doesn't Build

```powershell
# Install NSIS
choco install nsis

# Install EnVar plugin manually:
# Download from: https://nsis.sourceforge.io/mediawiki/images/7/7f/EnVar_plugin.zip
# Extract to C:\Program Files (x86)\NSIS\Plugins\
```

## Release Checklist

Before creating a release:

- [ ] Update version in `Makefile`
- [ ] Update CHANGELOG.md
- [ ] Update README.md if needed
- [ ] Run tests: `make test`
- [ ] Build locally: `make cross-compile`
- [ ] Test on target platforms
- [ ] Create git tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
- [ ] Push tag: `git push origin vX.Y.Z`
- [ ] Monitor GitHub Actions workflow
- [ ] Verify release artifacts on GitHub
- [ ] Test installation from release
- [ ] Announce release

## Version Numbering

We use Semantic Versioning (semver):

- `vX.Y.Z` - Stable release
- `vX.Y.Z-alpha` - Alpha release
- `vX.Y.Z-beta` - Beta release
- `vX.Y.Z-rc.N` - Release candidate

Examples:
- `v0.1.0-alpha` - First alpha
- `v0.1.0-beta.1` - First beta
- `v0.1.0-rc.1` - First release candidate
- `v0.1.0` - Stable release

## Support

For build issues:
- Check GitHub Actions logs
- Review `docs/DISTRIBUTION_STRATEGY.md`
- Open an issue on GitHub

For platform-specific questions:
- Linux: See `install.sh`
- macOS: See `build/macos/`
- Windows: See `build/windows/` and `installers/install-windows.ps1`
