# Distribution Strategy for OffGrid LLM

## Current State Analysis

### What You Have Now
- Linux-focused installation script (`install.sh`)
- Go binary compilation
- Systemd service integration
- Docker support
- Source-based installation

### What's Missing for Cross-Platform Distribution
- macOS native installer (.pkg or .dmg)
- Windows installer (.exe or .msi)
- Pre-compiled binary releases
- Code signing for macOS/Windows
- Auto-update mechanism
- Platform-specific service managers (launchd for macOS, Windows Service)

---

## Recommended Distribution Strategy (Ollama-Style)

### 1. **Release Artifacts Per Platform**

#### **macOS Distribution**
```
offgrid-darwin-amd64-v0.1.0.dmg          # Intel Macs
offgrid-darwin-arm64-v0.1.0.dmg          # Apple Silicon (M1/M2/M3)
```

**What's Needed:**
- **Application Bundle** (`OffGrid.app`)
  - Binary at `OffGrid.app/Contents/MacOS/offgrid`
  - launchd plist for auto-start
  - Homebrew integration for llama.cpp
  
- **DMG Creator**: Use `create-dmg` or `appdmg` tools
  ```bash
  # Example structure
  OffGrid.app/
    Contents/
      MacOS/
        offgrid              # Main binary
        llama-server         # Bundled llama.cpp
      Resources/
        icon.icns
      Info.plist
      LaunchAgents/
        com.offgrid.llm.plist
  ```

- **Code Signing** (Required for macOS Gatekeeper):
  ```bash
  codesign --deep --force --verify --verbose \
    --sign "Developer ID Application: Your Name (TEAM_ID)" \
    OffGrid.app
  
  # Notarize with Apple
  xcrun notarytool submit offgrid.dmg \
    --apple-id "your@email.com" \
    --team-id "TEAM_ID" \
    --password "app-specific-password"
  ```

#### **Windows Distribution**
```
OffGridSetup-v0.1.0-x64.exe              # 64-bit Windows
OffGridSetup-v0.1.0-arm64.exe            # ARM64 Windows
```

**What's Needed:**
- **Installer Creator**: Use NSIS, Inno Setup, or WiX Toolset
  ```
  Program Files/
    OffGrid/
      offgrid.exe
      llama-server.exe
      uninstall.exe
  ```

- **Windows Service Integration**:
  ```go
  // Use golang.org/x/sys/windows/svc
  // Or NSSM (Non-Sucking Service Manager)
  ```

- **Code Signing** (Required for SmartScreen):
  ```powershell
  signtool sign /f certificate.pfx /p password /tr http://timestamp.digicert.com /td sha256 OffGridSetup.exe
  ```

#### **Linux Distribution**
```
offgrid-linux-amd64-v0.1.0.tar.gz        # Generic Linux
offgrid-linux-arm64-v0.1.0.tar.gz        # ARM64 Linux
offgrid-v0.1.0-1.x86_64.rpm              # RedHat/Fedora
offgrid_v0.1.0_amd64.deb                 # Debian/Ubuntu
```

**What's Needed:**
- Keep current `install.sh` approach
- Add `.deb` and `.rpm` packages
- AppImage for universal Linux support

---

## 2. **Build Pipeline Architecture**

### GitHub Actions Workflow Structure

```yaml
name: Release Builds

on:
  push:
    tags:
      - 'v*'

jobs:
  # Build llama.cpp binaries for all platforms
  build-llama-cpp:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        arch: [amd64, arm64]
    runs-on: ${{ matrix.os }}
    steps:
      - name: Build llama-server
        run: |
          # Clone and build llama.cpp with platform-specific flags
          
  # Build Go binaries
  build-go:
    needs: build-llama-cpp
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        arch: [amd64, arm64]
    steps:
      - name: Cross-compile Go binary
        run: |
          GOOS=${{ matrix.os }} GOARCH=${{ matrix.arch }} \
          go build -o offgrid-${{ matrix.os }}-${{ matrix.arch }}
          
  # Package for macOS
  package-macos:
    needs: build-go
    runs-on: macos-latest
    steps:
      - name: Create .app bundle
      - name: Create DMG
      - name: Code sign (if secrets available)
      - name: Notarize (if secrets available)
      
  # Package for Windows
  package-windows:
    needs: build-go
    runs-on: windows-latest
    steps:
      - name: Build installer with NSIS
      - name: Code sign (if certificate available)
      
  # Package for Linux
  package-linux:
    needs: build-go
    runs-on: ubuntu-latest
    steps:
      - name: Create .deb package
      - name: Create .rpm package
      - name: Create .tar.gz archive
```

---

## 3. **Dependency Management Strategy**

### Problem: llama.cpp is a C++ dependency

**Option A: Bundle Pre-compiled Binaries** (Recommended - Like Ollama)
```
Pros:
  Users don't need build tools
  Faster installation
  Consistent experience
  Works on locked-down systems
  
Cons:
  Larger download size
  Multiple binaries to maintain
  Need CI/CD for each platform
```

**Implementation:**
```bash
# In your releases, include:
offgrid-darwin-arm64.dmg
  └── Contains: offgrid binary + llama-server binary

# Installation just copies files, no compilation
```

**Option B: Install from Package Managers** (Current approach)
```
macOS:   brew install llama.cpp
Windows: choco install llama-cpp
Linux:   Script builds from source
```

**Option C: Hybrid Approach** (Recommended)
```
1. Try to detect existing llama.cpp installation
2. If not found, use bundled binary
3. Allow user to specify custom llama-server path
```

---

## 4. **Installation Script Improvements**

### Create Platform-Specific Installers

#### `install-macos.sh` (Simplified)
```bash
#!/bin/bash
# macOS Installation Script

# Install via Homebrew (preferred)
if command -v brew &> /dev/null; then
    brew tap takuphilchan/offgrid
    brew install offgrid-llm
else
    # Manual installation
    curl -fsSL https://offgrid-llm.io/install-macos.sh | sh
fi

# Install llama.cpp separately
brew install llama.cpp

# Setup launchd service
launchctl load ~/Library/LaunchAgents/com.offgrid.llm.plist
```

#### `install-windows.ps1` (PowerShell)
```powershell
# Windows Installation Script
# Download and run installer
$url = "https://github.com/takuphilchan/offgrid-llm/releases/latest/download/OffGridSetup.exe"
$installer = "$env:TEMP\OffGridSetup.exe"

Invoke-WebRequest -Uri $url -OutFile $installer
Start-Process $installer -Wait

# Install llama.cpp via chocolatey
choco install llama-cpp
```

#### Keep `install.sh` for Linux
- Your current script is excellent for Linux
- Consider splitting into:
  - `install-ubuntu.sh` (apt-based)
  - `install-fedora.sh` (dnf/rpm-based)
  - `install-arch.sh` (pacman-based)

---

## 5. **Packaging Tools You'll Need**

### macOS
```bash
# Install development tools
xcode-select --install

# DMG creation
npm install -g appdmg
# OR
brew install create-dmg

# Code signing
# Requires: Apple Developer account ($99/year)
```

### Windows
```powershell
# NSIS Installer
choco install nsis

# Or Inno Setup
choco install innosetup

# Code signing
# Requires: Code signing certificate (~$100-500/year from DigiCert, etc.)
```

### Linux
```bash
# Debian package tools
apt-get install dpkg-dev debhelper

# RPM package tools
dnf install rpm-build rpmdevtools

# AppImage
wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
```

---

## 6. **Project Structure Updates**

### Add These Directories
```
offgrid-llm/
├── build/                       # Build scripts and configs
│   ├── macos/
│   │   ├── create-dmg.sh
│   │   ├── Info.plist.template
│   │   └── entitlements.plist
│   ├── windows/
│   │   ├── installer.nsi        # NSIS script
│   │   └── service-install.ps1
│   └── linux/
│       ├── debian/              # .deb package configs
│       ├── rpm/                 # .rpm package specs
│       └── appimage/
├── installers/                  # Platform-specific install scripts
│   ├── install-macos.sh
│   ├── install-windows.ps1
│   └── install-linux.sh         # Rename current install.sh
├── scripts/
│   └── release.sh               # Automated release script
└── .github/
    └── workflows/
        ├── build.yml
        ├── release.yml
        └── package.yml
```

---

## 7. **Code Changes Needed**

### Platform Detection in Go
```go
// internal/platform/platform.go
package platform

import "runtime"

func GetPlatform() string {
    return runtime.GOOS
}

func GetDefaultInstallPath() string {
    switch runtime.GOOS {
    case "darwin":
        return "/Applications/OffGrid.app"
    case "windows":
        return "C:\\Program Files\\OffGrid"
    default:
        return "/usr/local/bin"
    }
}

func GetConfigPath() string {
    switch runtime.GOOS {
    case "darwin":
        return os.ExpandEnv("$HOME/Library/Application Support/OffGrid")
    case "windows":
        return os.ExpandEnv("%APPDATA%\\OffGrid")
    default:
        return os.ExpandEnv("$HOME/.config/offgrid")
    }
}
```

### Service Management Abstraction
```go
// internal/service/manager.go
package service

type ServiceManager interface {
    Install() error
    Uninstall() error
    Start() error
    Stop() error
    Status() (string, error)
}

// Implementations:
// - systemd.go (Linux)
// - launchd.go (macOS)
// - windows.go (Windows Service)
```

---

## 8. **Release Process Automation**

### `scripts/release.sh`
```bash
#!/bin/bash
# Automated release script

VERSION=$1

# 1. Tag release
git tag -a "v${VERSION}" -m "Release v${VERSION}"

# 2. Trigger GitHub Actions
git push origin "v${VERSION}"

# 3. Wait for builds to complete
# 4. Create GitHub release with artifacts
gh release create "v${VERSION}" \
  --title "OffGrid LLM v${VERSION}" \
  --notes "Release notes here" \
  dist/*
```

---

## 9. **Minimum Viable Distribution (Start Here)**

If you want to start simple and iterate:

### Phase 1: Basic Cross-Platform Binaries
```bash
# Add to Makefile
cross-compile:
    GOOS=linux GOARCH=amd64 go build -o dist/offgrid-linux-amd64
    GOOS=darwin GOARCH=amd64 go build -o dist/offgrid-darwin-amd64
    GOOS=darwin GOARCH=arm64 go build -o dist/offgrid-darwin-arm64
    GOOS=windows GOARCH=amd64 go build -o dist/offgrid-windows-amd64.exe
```

### Phase 2: Add Install Scripts
- `curl https://offgrid-llm.io/install.sh | sh` (auto-detects platform)

### Phase 3: Add Native Installers
- macOS .dmg
- Windows .exe

### Phase 4: Code Signing & Auto-updates
- Sign binaries
- Add update checker

---

## 10. **Cost Considerations**

### Free Options
- GitHub Actions (2,000 minutes/month free)
- GitHub Releases (unlimited)
- Self-signed certificates (dev only)

### Paid Requirements
- Apple Developer ($99/year) - Required for macOS code signing
- Code Signing Certificate ($100-500/year) - Required for Windows SmartScreen
- Notarization (included in Apple Developer)

### Without Code Signing
- macOS: Users see "unidentified developer" warning (can bypass)
- Windows: SmartScreen warning (can bypass)
- Linux: No issues

---

## 11. **Next Steps - Implementation Checklist**

### Immediate (This Week)
- [ ] Create `build/` directory structure
- [ ] Add cross-compilation to Makefile
- [ ] Create basic install scripts for each platform
- [ ] Test manual builds on macOS/Windows

### Short-term (This Month)
- [ ] Set up GitHub Actions for cross-platform builds
- [ ] Create basic .dmg for macOS
- [ ] Create basic .exe for Windows
- [ ] Add platform detection to Go code
- [ ] Create unified install script (auto-detects OS)

### Medium-term (Next Quarter)
- [ ] Implement service management abstraction
- [ ] Add auto-update mechanism
- [ ] Get code signing certificates
- [ ] Publish to package managers (Homebrew, Chocolatey, apt repo)

### Long-term (Future)
- [ ] GUI installer for Windows/macOS
- [ ] Built-in model marketplace
- [ ] Automatic GPU driver installation
- [ ] Enterprise deployment tools

---

## 12. **Recommended Architecture Change**

### Current: Source-based Installation
```
User runs install.sh → Compiles llama.cpp → Builds Go → Installs
(~30 minutes, requires build tools)
```

### Recommended: Binary Distribution
```
User downloads installer → Copies pre-built binaries → Configures
(~2 minutes, no build tools needed)
```

### Implementation:
```
Release artifacts:
  - offgrid binary (pre-compiled)
  - llama-server binary (pre-compiled)
  - Optional: model downloader
  
Installation:
  1. Extract binaries to install location
  2. Set up config files
  3. Register service
  4. Done
```

---

## Summary: What You Need to Do

### Critical Path
1. **Add cross-compilation** to your build process
2. **Bundle llama.cpp binaries** in releases (don't make users compile)
3. **Create platform-specific installers** (.dmg, .exe, .deb)
4. **Set up CI/CD** (GitHub Actions) for automated builds
5. **Simplify installation** from 30 minutes to <5 minutes

### Your Current install.sh is Good For:
- Linux power users
- Development environments
- Custom configurations
- Building from source

### But You Also Need:
- Binary releases for casual users
- Native installers (double-click to install)
- Fast installation (no compilation)
- Code-signed binaries (trust)

### Files to Create
```
build/macos/create-dmg.sh
build/windows/installer.nsi
installers/install-macos.sh
installers/install-windows.ps1
.github/workflows/release.yml
docs/BUILDING.md
docs/RELEASING.md
```

Would you like me to start implementing any of these components?
