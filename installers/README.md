# OffGrid LLM - Easy Installation

One-command installation for all platforms. No compilation, no complex setup - just run and go!

**Installation time:** 10-15 seconds  
**Service type:** Manual start (not a background service)

**Note:** This installer provides pre-built binaries for manual use. For automatic startup and systemd service integration, use the [production installer](../dev/) instead.

---

## Quick Install

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

### Windows (PowerShell as Administrator)

```powershell
iwr -useb https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.ps1 | iex
```

---

## Features

### Automatic GPU Detection

- **Linux/macOS**: Detects NVIDIA, AMD, Intel GPUs and installs Vulkan-accelerated binaries
- **Windows**: Detects NVIDIA GPU + CUDA and installs CUDA-accelerated binaries
- **Fallback**: CPU-only if no GPU detected

### What Gets Installed

- **OffGrid LLM** - Main application (~10MB)
- **llama.cpp** - Inference engine with GPU support (~15-50MB depending on variant)
- **All required libraries** - Shared libraries for GPU acceleration

**Important:** This installer does NOT create systemd services or auto-start functionality. You manually start the server with `offgrid server start` when needed.

For automatic startup and background service operation, use the [production installer](../dev/) which builds from source and creates systemd services.

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
offgrid version
offgrid info
```

### Download Your First Model

```bash
# Browse models
offgrid catalog

# Download small model (600MB)
offgrid download tinyllama-1.1b-chat Q4_K_M
```

### Start Using

```bash
# Interactive chat
offgrid run tinyllama-1.1b-chat

# Start API server manually (runs in foreground)
offgrid server start

# Or run API server in background
offgrid server start &

# Check server status
curl http://localhost:11611/health
```

**Note:** The server does NOT start automatically. You must run `offgrid server start` each time you want to use it.

For automatic startup on boot, use the [production installer](../dev/) which creates systemd services.

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
