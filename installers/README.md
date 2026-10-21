# OffGrid LLM - Installation Scripts

> **Note:** The recommended way to install OffGrid LLM is using the unified installer at the repository root. These individual scripts are kept for legacy compatibility.

---

## Unified Installer (Recommended)

**One command installs everything:**

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

The root installer selects a platform release bundle, installs the CLI and
included Web UI/audio assets, and offers the matching desktop build when one
is published. It does not currently present a component-selection menu.

To select a published release and opt into the Linux user service:
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | VERSION=v0.4.19 AUTOSTART=yes bash
```

The previously published `scripts/install.sh` raw URL remains a compatibility
wrapper. It preserves its GPU bundle selection and `AUTOSTART=ask` default.
The root installer has no systemd side effect unless `AUTOSTART` is selected.
Repository unit sources are under [`systemd/`](systemd/); the installer retains
the installed `offgrid@.service` name.

---

## Legacy Scripts

### Desktop Application (Linux / macOS)
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

### Windows (PowerShell as Administrator)
```powershell
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

---

## What Gets Installed

### CLI Installation
- Pre-built `offgrid` binary (~10MB)
- Auto-detection of GPU support
- PATH configuration for instant use
- Auto-start service on Linux (systemd - optional)

### Desktop Installation  
- Desktop application with UI
- System tray integration
- Bundled CLI binary
- Automatic server management
- Models stored in `~/.offgrid-llm/`

**Installation time:** ~1 minute for CLI, ~2-3 minutes for Desktop

---

## Production Install

**For servers with auto-start on boot:**

See [the source-build guide](../docs/advanced/BUILDING.md) and the explicit
[`install-from-source.sh`](../scripts/build/install-from-source.sh) helper for
the older source/GPU/systemd path. It installs and changes system services;
the root release installer is the maintained default.

---

## System Requirements

### Minimum

- **OS**: Linux (Ubuntu 20.04+) / macOS 11+ / Windows 10+
- **RAM**: 4GB
- **Storage**: 500MB + models

### GPU Acceleration

#### Linux
```bash
# NVIDIA (Vulkan)
sudo apt-get install vulkan-tools libvulkan1

# AMD
sudo apt-get install mesa-vulkan-drivers vulkan-tools
```

#### Windows
Install [CUDA Toolkit 12.4+](https://developer.nvidia.com/cuda-downloads)

#### macOS
- **Apple Silicon**: Metal built-in [Done]
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

# Download from HuggingFace
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf
```

### Start Using

```bash
# Interactive chat
offgrid run Llama-3.2-3B-Instruct-Q4_K_M

# Or access Web UI
open http://localhost:11611/ui
```

---

## Troubleshooting

### Command not found

```bash
# Linux/macOS - reload shell
source ~/.bashrc  # or ~/.zshrc

# Or specify full path
/usr/local/bin/offgrid --version
```

### Permission denied

```bash
# Make binary executable
chmod +x /usr/local/bin/offgrid
```

### GPU not detected

```bash
# Check GPU support
offgrid info

# Reinstall with GPU libraries
sudo apt-get install vulkan-tools libvulkan1  # Linux
```

---

## Uninstall

```bash
# Remove binary
sudo rm /usr/local/bin/offgrid

# Remove systemd service (Linux only)
sudo systemctl stop llama-server@$USER.service
sudo systemctl disable llama-server@$USER.service
sudo rm /etc/systemd/system/llama-server@.service
sudo rm /usr/local/bin/llama-server-start.sh

# Remove models and data (optional)
rm -rf ~/.offgrid-llm
```

---

## Next Steps

-  [Documentation](../docs/README.md)
-  [Quick Start Guide](../README.md#quick-start)
-  [CLI Reference](../docs/reference/cli.md)
-  [API Documentation](../docs/reference/api.md)
