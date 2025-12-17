# Release Notes - v0.1.9

**Release Date:** November 25, 2025

## Overview

Version 0.1.9 brings UI refinements, improved terminal output formatting, and critical bug fixes for both the web interface and desktop application.

---

## What's New

### UI Improvements

**True Neutral Dark Theme**
- Removed blue tint from dark mode, now uses pure grays and blacks
- Consistent styling across sidebar, headers, and card backgrounds
- Improved text contrast and readability

**Modern Modal System**
- Custom theme-aware dialogs replace native browser alerts
- Unified design for error messages, confirmations, and input prompts
- Enhanced export dialog with clear format descriptions

**Terminal Output Formatting**
- Fixed column alignment in web and desktop terminal displays
- Consistent spacing between CLI and web UI outputs
- Improved monospace font rendering

### Bug Fixes

- Fixed HTTP 500 errors when executing terminal commands
- Resolved JavaScript syntax errors affecting tab switching
- Fixed chat input session reference errors
- Improved Windows USB utility compatibility

---

## Installation

### Desktop Application

The desktop app includes a bundled server and provides the easiest setup experience.

**Linux**
```bash
# One-line install
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash

# Or download directly:
# - AppImage (universal): OffGrid-LLM-Desktop-0.1.9-x86_64.AppImage
# - Debian/Ubuntu: OffGrid-LLM-Desktop-0.1.9-amd64.deb
```

**macOS**
```bash
# One-line install
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash

# Or download DMG:
# - Apple Silicon: OffGrid-LLM-Desktop-0.1.9-arm64.dmg
# - Intel: OffGrid-LLM-Desktop-0.1.9-x64.dmg
```

**Windows**
```powershell
# PowerShell (Run as Administrator)
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex

# Or download: OffGrid-LLM-Desktop-Setup-0.1.9.exe
```

### CLI Installation

For servers, containers, or users who prefer command-line tools.

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/scripts/install.sh | bash
```

**Pre-built Bundles**

| Platform | GPU Support | Download |
|----------|-------------|----------|
| Linux x64 | Vulkan | offgrid-v0.1.9-linux-amd64-vulkan-avx2.tar.gz |
| Linux x64 | CPU only | offgrid-v0.1.9-linux-amd64-cpu-avx2.tar.gz |
| Linux ARM64 | CPU | offgrid-v0.1.9-linux-arm64-cpu-neon.tar.gz |
| macOS ARM64 | Metal | offgrid-v0.1.9-darwin-arm64-metal-apple-silicon.tar.gz |
| macOS x64 | CPU | offgrid-v0.1.9-darwin-amd64-cpu-avx2.tar.gz |
| Windows x64 | CPU | offgrid-v0.1.9-windows-amd64-cpu-avx2.zip |

---

## Quick Start

```bash
# Verify installation
offgrid --version

# Search for models
offgrid search llama --ram 4

# Download a model
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# Start chatting
offgrid run Llama-3.2-3B-Instruct-Q4_K_M

# Or use the web interface
offgrid serve
# Open http://localhost:11611/ui
```

---

## Verification

Verify downloads using the provided checksums:

```bash
sha256sum -c checksums-v0.1.9.sha256
```

---

## Documentation

- [Installation Guide](https://github.com/takuphilchan/offgrid-llm/blob/main/docs/INSTALLATION.md)
- [Desktop App Guide](https://github.com/takuphilchan/offgrid-llm/blob/main/desktop/DESKTOP_INSTALL.md)
- [CLI Reference](https://github.com/takuphilchan/offgrid-llm/blob/main/docs/CLI_REFERENCE.md)
- [API Documentation](https://github.com/takuphilchan/offgrid-llm/blob/main/docs/API.md)

---

## Upgrade Notes

If upgrading from v0.1.8:
- No breaking changes
- Existing models and sessions are preserved
- Desktop app will auto-update if installed via the installer scripts
