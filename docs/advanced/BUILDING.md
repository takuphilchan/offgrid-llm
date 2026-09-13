# Building and Releasing OffGrid LLM

This document explains how to build, package, and release OffGrid LLM for all supported platforms.

## Quick Start

### Build CLI for Current Platform

```bash
# Using Go directly
go build -o offgrid ./cmd/offgrid

# Or using the build script
./build-all.sh --cli --platform current
```

### Build Desktop App

```bash
# Build for current platform
cd desktop && npm install && npm run build

# Or use master build script
./build-all.sh --desktop
```

### Build Everything

```bash
# Build CLI + Desktop for all platforms
./build-all.sh --all
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
├── build/                  # Built CLI binaries (git-ignored)
│   ├── linux/offgrid
│   ├── macos/offgrid
│   └── windows/offgrid.exe
├── desktop/                # Electron desktop app
│   ├── package.json        # With electron-builder config
│   ├── main.js             # Main process
│   ├── dist/               # Desktop installers (git-ignored)
│   └── assets/             # Icons and resources
├── installers/             # Installation scripts
│   ├── desktop.sh          # Desktop app installer (Linux/macOS)
│   └── desktop.ps1         # Desktop app installer (Windows)
├── install.sh              # CLI installer (root level)
├── build-all.sh            # Master build script
└── .github/
    └── workflows/
        └── release-unified.yml  # Automated CI/CD
```

## Manual Building

### 1. Build CLI Binaries

```bash
# Using build-all.sh (recommended)
./build-all.sh --cli --platform all

# Or manually with Go
GOOS=linux GOARCH=amd64 go build -o build/linux/offgrid ./cmd/offgrid
GOOS=darwin GOARCH=arm64 go build -o build/macos/offgrid ./cmd/offgrid
GOOS=windows GOARCH=amd64 go build -o build/windows/offgrid.exe ./cmd/offgrid

# Outputs in build/:
# - build/linux/offgrid
# - build/macos/offgrid
# - build/windows/offgrid.exe
```

### 2. Build Desktop Applications

```bash
# Prerequisites
cd desktop && npm install

# Build for all platforms
npm run build:all

# Or platform-specific
npm run build:linux   # Creates .AppImage and .deb
npm run build:mac     # Creates .dmg
npm run build:win     # Creates .exe installer

# Outputs in desktop/dist/:
# Linux:   OffGrid-LLM-Desktop-{version}-x86_64.AppImage
#          OffGrid-LLM-Desktop-{version}-amd64.deb
# macOS:   OffGrid-LLM-Desktop-{version}-arm64.dmg
# Windows: OffGrid-LLM-Desktop-Setup-{version}.exe
```

## Platform-Specific Packaging

### Desktop Application (All Platforms)

The desktop app uses **electron-builder** which automatically creates native installers.

#### Linux

```bash
cd desktop
npm run build:linux

# Creates:
# - AppImage (universal, portable)
# - .deb package (Debian/Ubuntu)
# Both x64 and arm64 architectures
```

#### macOS

```bash
cd desktop
npm run build:mac

# Creates .dmg installers:
# - x64 (Intel Macs)
# - arm64 (Apple Silicon)
# - universal (both architectures)
```

#### Windows

```bash
cd desktop
npm run build:win

# Creates:
# - NSIS installer (.exe)
# - Portable version (.exe)
```

### CLI Bundles (with llama.cpp)

CLI bundles are created by the GitHub Actions workflow. See `.github/workflows/release-unified.yml` for the complete process which includes:
1. Building llama.cpp from source with platform-specific optimizations
2. Bundling with the OffGrid CLI binary
3. Creating release archives (.tar.gz, .zip)

## Automated Releases (GitHub Actions)

The current process is documented in [Releasing OffGrid](releasing.md). A
`vX.Y.Z` tag starts the desktop/CLI release and Docker Hub publishing flows.
The GitHub release is complete only after its 14 expected assets and
`checksums-vX.Y.Z.sha256` are present; the matching Docker Hub image must also
be pullable. Repair branches can finish an existing tag without moving it or
rebuilding already verified native packages.

The GitHub assets cover Linux AMD64/ARM64, macOS Intel/Apple Silicon, and
Windows AMD64 runtime bundles, plus Linux, macOS, and Windows desktop
packages. See the release notes for the exact filenames of a version.

## Installation Instructions

Use the current [installation guide](../setup/installation.md) and the exact
asset names on the release page. The old v0.1.6 DMG and Windows ARM64
examples are not current release artifacts. Download only the packages for
your operating system and architecture, then verify the downloaded file
against `checksums-vX.Y.Z.sha256` before installation.

Desktop packages are currently unsigned. Do not claim that Windows installers
or macOS desktop archives are signed or notarized until the signing jobs and
their verification are part of the release workflow.

## Testing Releases

### Test Locally Before Pushing

```bash
go test ./...
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/*.yml
node dev/scripts/test-finalize-release.mjs
```

CI also builds the web UI, exercises its browser integration tests, and
packages Electron. See [Releasing OffGrid](releasing.md) for hosted artifact
and Docker Hub checks.

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
go version  # Should be 1.26.6 or later

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
- `v0.1.6-alpha` - First alpha
- `v0.1.6-beta.1` - First beta
- `v0.1.6-rc.1` - First release candidate
- `v0.1.6` - Stable release

## Support

For build issues:
- Check GitHub Actions logs
- Review `docs/DISTRIBUTION_STRATEGY.md`
- Open an issue on GitHub

For platform-specific questions:
- Linux: See `install.sh`
- macOS: See `build/macos/`
- Windows: See `build/windows/` and `installers/install-windows.ps1`
