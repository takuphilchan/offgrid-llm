# OffGrid LLM - Installation Scripts

**Simple one-line installers for all platforms.**

---

## Quick Install (Recommended)

**Pre-built binaries with auto-detection:**

#### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

#### Windows (PowerShell as Administrator)
```powershell
iwr -useb https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.ps1 | iex
```

**What gets installed:**
- Pre-built `offgrid` binary (~10MB)
- Auto-detection of GPU support
- PATH configuration for instant use
- Auto-start service on Linux (systemd)

**Installation time:** ~1 minute

---

## Production Install

**For servers with auto-start on boot:**

See [../dev/README.md](../dev/README.md) for building from source with systemd services.

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

- 📖 [Documentation](../docs/README.md)
- 🚀 [Quick Start Guide](../README.md#quick-start)
- 💻 [CLI Reference](../docs/CLI_REFERENCE.md)
- 🌐 [API Documentation](../docs/API.md)
