# OffGrid LLM - Installation Guide

**Choose your installation method:**

---

## 🚀 Production Bundle Install (Recommended)

**Complete, zero-dependency installation with automatic GPU detection:**

```bash
curl -fsSL https://github.com/takuphilchan/offgrid-llm/releases/download/v1.0.1/install-bundle.sh | bash
```

### What You Get

- ✅ **Both binaries bundled**: `offgrid` + `llama-server` in one package
- ✅ **Zero external dependencies**: Everything statically compiled
- ✅ **Auto GPU detection**: Downloads Vulkan (Linux) or Metal (macOS) variant
- ✅ **Auto-starts servers**: Just run `offgrid run <model>` - everything starts automatically!
- ✅ **Production ready**: Tested, versioned releases
- ⚡ **Fast**: Installs in ~10 seconds

### Available Bundles

| Platform | GPU Support | Download Size |
|----------|-------------|---------------|
| Linux amd64 | Vulkan | ~50MB |
| Linux amd64 | CPU-only | ~35MB |
| macOS arm64 | Metal (built-in) | ~40MB |
| macOS amd64 | CPU-only | ~38MB |

**Installation time:** 10 seconds  
**Service type:** Auto-start on demand with `offgrid run`

---

## Quick Install (Go Binary Only)

**Fast install using pre-built Go binaries (requires separate llama-server setup):**

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

### Windows (PowerShell as Administrator)

```powershell
iwr -useb https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.ps1 | iex
```

**Installation time:** 10-15 seconds  
**Service type:** Manual start (not a background service)

**Note:** This installer provides the `offgrid` binary only. You'll need to install `llama-server` separately or use the production bundle above.

---

## 🔧 Production Install with Systemd (Advanced)

**Build from source with automatic startup on boot:**

See [dev/README.md](../dev/) for systemd service installation.

- ✅ Automatic startup on boot
- ✅ Security hardening with systemd
- ✅ Custom build flags and optimizations
- ✅ Full control over compilation

---

## Features

### Automatic GPU Detection (Bundle Installer)

The production bundle installer automatically detects your hardware:

- **Linux**: Checks for Vulkan support, downloads GPU or CPU variant
- **macOS Apple Silicon**: Automatically uses Metal-enabled bundle
- **macOS Intel**: CPU-only variant
- **Fallback**: Always downloads CPU version if GPU detection fails

### What Gets Installed (Bundle)

- **OffGrid LLM** - Main application binary (~15MB)
- **llama-server** - Statically compiled inference engine (~35-50MB)
- **No external dependencies** - Everything bundled and ready to run

### What Gets Installed (Quick Install)

- **OffGrid LLM** - Main application (~10MB)
- **llama.cpp** - Downloaded separately during first use (~15-50MB depending on variant)
- **Shared libraries** - GPU acceleration libraries if available

**Important:** Quick installer does NOT create systemd services. Use `offgrid run` to start manually.

---

## System Requirements

### Minimum

- **OS**: Linux (Ubuntu 20.04+) / macOS 11+ / Windows 10+
- **RAM**: 4GB
- **Storage**: 500MB + models

### For GPU Acceleration

#### Linux - NVIDIA GPU

```bash
sudo apt-get install vulkan-tools libvulkan1
# Then reinstall OffGrid
```

#### Linux - AMD GPU

```bash
sudo apt-get install mesa-vulkan-drivers vulkan-tools
```

#### Windows - NVIDIA GPU

Install [CUDA Toolkit 12.4+](https://developer.nvidia.com/cuda-downloads)

#### macOS

- **Apple Silicon**: Metal built-in
- **Intel Mac**: CPU-only

---

## Post-Installation

### Verify Installation

```bash
offgrid --version
```

### Download Your First Model

```bash
# Search for models
offgrid search llama --limit 5

# Download a model from HuggingFace
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf
```

### Start Using

```bash
# Interactive chat - auto-starts both servers!
offgrid run Llama-3.2-3B-Instruct-Q4_K_M

# Or start API server manually if needed
offgrid serve

# Check server status
curl http://localhost:11611/health

# Access Web UI
open http://localhost:11611/ui
```

**Note:** The `offgrid run` command automatically starts both the OffGrid API server AND llama-server if they're not running. Everything is zero-setup - just download a model and run!

---

## Installation Locations

| Platform | Binary Location | Config Location |
|----------|----------------|-----------------|
| Linux | `/usr/local/bin/` | `~/.config/offgrid/` |
| macOS | `/usr/local/bin/` | `~/Library/Application Support/OffGrid/` |
| Windows | `%LOCALAPPDATA%\offgrid-llm\bin\` | `%APPDATA%\OffGrid\` |

---

## Troubleshooting

### "Command not found" after installation

**Linux/macOS**: Restart terminal or run:

```bash
export PATH="$PATH:/usr/local/bin"
```

**Windows**: Restart PowerShell or Command Prompt.

### Permission denied

**Linux/macOS**: The installer requires sudo for system-wide installation.

**Windows**: Run PowerShell as Administrator.

### GPU not detected

**Linux**: Install vulkan-tools, then reinstall

**Windows**: Install CUDA Toolkit, then reinstall

---

## Updating

Run the installer again - it replaces old binaries while keeping models/config.

The installers will detect and remove previous versions automatically.

---

## Uninstall

### Linux/macOS

```bash
sudo rm -f /usr/local/bin/{offgrid,llama-server}
sudo rm -f /usr/local/lib/libggml*.so* /usr/local/lib/libllama*.so*
rm -rf ~/.offgrid-llm ~/.config/offgrid  # Optional: models/config
```

### Windows

```powershell
Remove-Item -Recurse "$env:LOCALAPPDATA\offgrid-llm"
Remove-Item -Recurse "$env:USERPROFILE\.offgrid-llm"  # Optional: models
```

---

## Documentation

- **Main README**: [../README.md](../README.md)
- **API Reference**: [../docs/API.md](../docs/API.md)
- **Build from Source**: [../dev/](../dev/)
