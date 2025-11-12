# Installers Directory

This directory contains platform-specific installation scripts for pre-compiled binaries.

## Quick Reference

| Script | Platform | Purpose |
|--------|----------|---------|
| `install.sh` | **Universal** | Auto-detects OS, downloads latest release |
| `install-macos.sh` | macOS | Installs from extracted .tar.gz or .dmg |
| `install-windows.ps1` | Windows | PowerShell installer with PATH setup |

## Usage

### Universal Installer (Recommended)

Downloads and installs the correct binary for your platform:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash

# Or download and run locally
wget https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh
chmod +x install.sh
./install.sh
```

### Platform-Specific Installers

Used when you've already downloaded and extracted a release:

#### macOS
```bash
# After downloading offgrid-vX.Y.Z-darwin-arm64.tar.gz
tar -xzf offgrid-vX.Y.Z-darwin-arm64.tar.gz
cd offgrid-vX.Y.Z-darwin-arm64
./install.sh  # or use install-macos.sh
```

#### Windows
```powershell
# After extracting offgrid-vX.Y.Z-windows-amd64.zip
cd offgrid-vX.Y.Z-windows-amd64
powershell -ExecutionPolicy Bypass -File install.ps1
```

#### Linux
```bash
# After downloading offgrid-vX.Y.Z-linux-amd64.tar.gz
tar -xzf offgrid-vX.Y.Z-linux-amd64.tar.gz
cd offgrid-vX.Y.Z-linux-amd64
sudo ./install.sh
```

## Difference from Root `install.sh`

**This directory** (`installers/`) contains scripts for **pre-built binaries**.

**Root `install.sh`** is for **building from source** on Linux with:
- llama.cpp compilation (~5-10 min)
- GPU detection and configuration
- systemd service setup
- Full development environment

Most users should use the installers in this directory for faster installation.

## What Gets Installed

All installers place binaries in standard locations:

| Platform | Binary Location | Config Location |
|----------|----------------|-----------------|
| Linux | `/usr/local/bin/` | `~/.config/offgrid/` |
| macOS | `/usr/local/bin/` or `/Applications/OffGrid.app/` | `~/Library/Application Support/OffGrid/` |
| Windows | `C:\Program Files\OffGrid\` | `%APPDATA%\OffGrid\` |

## Uninstallation

### Linux/macOS
```bash
sudo rm /usr/local/bin/offgrid
sudo rm /usr/local/bin/llama-server  # if bundled
rm -rf ~/.config/offgrid              # config files
rm -rf ~/.local/share/offgrid         # data files
```

### Windows
```powershell
# Run the uninstaller created during installation
powershell -ExecutionPolicy Bypass -File "C:\Program Files\OffGrid\Uninstall.ps1"
```

## Troubleshooting

### "Command not found" after installation

**Linux/macOS**: Restart terminal or run:
```bash
export PATH="$PATH:/usr/local/bin"
```

**Windows**: Restart PowerShell or Command Prompt.

### Permission denied

**Linux/macOS**: Make sure to use `sudo` for system-wide installation.

**Windows**: Run PowerShell as Administrator.

### Already installed

The installers will detect and remove previous versions automatically.

## Documentation

- Full build instructions: [../docs/BUILDING.md](../docs/BUILDING.md)
- Distribution strategy: [../docs/DISTRIBUTION_STRATEGY.md](../docs/DISTRIBUTION_STRATEGY.md)
- Quick reference: [../docs/QUICK_REFERENCE.md](../docs/QUICK_REFERENCE.md)
