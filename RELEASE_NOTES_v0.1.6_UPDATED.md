# OffGrid LLM v0.1.6

**Release Update: UI Polish & CLI Fixes**

This update to v0.1.6 brings significant visual improvements to the Terminal UI and fixes CLI output formatting.

## 🎨 What's Changed

### Terminal UI Improvements
*   **Authentic Terminal Look**: Updated the terminal font stack to use `Consolas`, `Monaco`, and `Courier New` for a true terminal feel, replacing the previous coding font.
*   **Seamless Input Area**: The terminal input line is now fully integrated with the output window, removing visual separators and borders for a cleaner, more immersive experience.
*   **Visual Polish**: Adjusted padding and alignment for the command prompt and input text.

### CLI Enhancements
*   **Better Help Output**: The `offgrid help` command now uses a tab writer to perfectly align command descriptions, making the help menu much easier to read.

---

## Installation

### 🖥️ Desktop Application (Recommended for Most Users)
Easy installation with system tray integration:

**Linux:**
*   Download - Make executable and run: `chmod +x *.AppImage && ./OffGrid*.AppImage`
*   Or download `.deb` - Install: `sudo dpkg -i *.deb`

**macOS:**
*   Download `.dmg` - Mount and drag to Applications folder

**Windows:**
*   Download `*Setup*.exe` - Run installer

**One-line install:**
```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash

# Windows (PowerShell as Admin)
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

### 💻 CLI Installation (For Servers & Advanced Users)
**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```
Or download bundles below and extract.

---

## 📦 Desktop Applications

**Linux**
*   `OffGrid-LLM-Desktop-{version}-x86_64.AppImage` - Universal Linux (recommended)
*   `OffGrid-LLM-Desktop-{version}-amd64.deb` - Debian/Ubuntu
*   `OffGrid-LLM-Desktop-{version}-arm64.AppImage` - ARM64 Linux
*   `OffGrid-LLM-Desktop-{version}-arm64.deb` - ARM64 Debian/Ubuntu

**macOS**
*   `OffGrid-LLM-Desktop-{version}-arm64.dmg` - Apple Silicon (M1/M2/M3)
*   `OffGrid-LLM-Desktop-{version}-x64.dmg` - Intel Mac
*   `OffGrid-LLM-Desktop-{version}-universal.dmg` - Universal (both architectures)

**Windows**
*   `OffGrid-LLM-Desktop-Setup-{version}.exe` - Windows Installer (recommended)
*   `OffGrid-LLM-Desktop-{version}-Portable.exe` - Portable version (no install)

## 🔧 CLI Bundles
Choose your platform and GPU variant:

**Linux**
*   `offgrid-v0.1.6-linux-amd64-vulkan-avx2.tar.gz` - Vulkan GPU (recommended, Intel/AMD 2013+)
*   `offgrid-v0.1.6-linux-amd64-vulkan-avx512.tar.gz` - Vulkan GPU (newer Intel)
*   `offgrid-v0.1.6-linux-amd64-cpu-avx2.tar.gz` - CPU only
*   `offgrid-v0.1.6-linux-arm64-cpu-neon.tar.gz` - ARM64

**macOS**
*   `offgrid-v0.1.6-darwin-arm64-metal-apple-silicon.tar.gz` - Apple Silicon with Metal GPU
*   `offgrid-v0.1.6-darwin-amd64-cpu-avx2.tar.gz` - Intel Mac (CPU only)

**Windows**
*   `offgrid-v0.1.6-windows-amd64-cpu-avx2.zip` - CPU only

---

## 📋 What's Included

**Desktop App Includes:**
*   Beautiful UI with system tray
*   Automatic server management
*   Built-in CLI binary
*   Model management interface
*   Chat interface

**CLI Bundle Includes:**
*   `offgrid` - Main CLI application
*   `llama-server` - Inference engine (llama.cpp)
*   `install.sh` - Installation script (Linux/macOS)
*   `README.md` - Getting started guide
*   `checksums.sha256` - File verification

## ✅ Verification
Verify your download:
```bash
sha256sum -c checksums-v0.1.6.sha256
```

## 🚀 Getting Started

**Desktop App**
1.  Install using one of the methods above
2.  Launch "OffGrid LLM Desktop" from your applications
3.  Use the Models tab to download models
4.  Start chatting!

**CLI**
After installation:
```bash
# Verify
offgrid --version

# Search for models
offgrid search llama --limit 5

# Download a model
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# Start chatting
offgrid run Llama-3.2-3B-Instruct-Q4_K_M

# Or use Web UI
open http://localhost:11611/ui
```

## 📚 Documentation
*   [Complete Documentation](docs/README.md)
*   [Desktop App Guide](docs/guides/FEATURES_GUIDE.md)
*   [CLI Reference](docs/CLI_REFERENCE.md)
*   [API Documentation](docs/API.md)
