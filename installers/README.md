# OffGrid LLM - Easy Installation# Installers Directory



One-command installation for all platforms. No compilation, no complex setup - just run and go!This directory contains platform-specific installation scripts for pre-compiled binaries.



---## Quick Reference



## 🚀 Quick Install| Script | Platform | Purpose |

|--------|----------|---------|

### Linux / macOS| `install.sh` | **Universal** | Auto-detects OS, downloads latest release |

```bash| `install-macos.sh` | macOS | Installs from extracted .tar.gz or .dmg |

curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash| `install-windows.ps1` | Windows | PowerShell installer with PATH setup |

```

## Usage

### Windows (PowerShell)

```powershell### Universal Installer (Recommended)

irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.ps1 | iex

```Downloads and installs the correct binary for your platform:



**Installation time:** 10-15 seconds ⚡```bash

# Linux / macOS

---curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash



## ✨ Features# Or download and run locally

wget https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh

### Automatic GPU Detectionchmod +x install.sh

- ✅ **Linux/macOS**: Detects NVIDIA, AMD, Intel GPUs → Installs Vulkan-accelerated binaries./install.sh

- ✅ **Windows**: Detects NVIDIA GPU + CUDA → Installs CUDA-accelerated binaries  ```

- ✅ **Fallback**: CPU-only if no GPU detected

### Platform-Specific Installers

### What Gets Installed

- **OffGrid LLM** - Main application (~10MB)Used when you've already downloaded and extracted a release:

- **llama.cpp** - Inference engine with GPU support (~15-50MB depending on variant)

- **All required libraries** - Shared libraries for GPU acceleration#### macOS

```bash

---# After downloading offgrid-darwin-arm64.tar.gz

tar -xzf offgrid-darwin-arm64.tar.gz

## 📋 System Requirementscd offgrid-darwin-arm64

bash install-macos.sh  # from extracted package

### Minimum```

- **OS**: Linux (Ubuntu 20.04+) / macOS 11+ / Windows 10+

- **RAM**: 4GB#### Windows

- **Storage**: 500MB + models```powershell

# After extracting offgrid-windows-amd64.zip

### For GPU Accelerationcd offgrid-windows-amd64

powershell -ExecutionPolicy Bypass -File install.ps1

#### Linux - NVIDIA GPU```

```bash

sudo apt-get install vulkan-tools libvulkan1#### Linux

# Then reinstall OffGrid```bash

```#### Linux

```bash

#### Linux - AMD GPU# After downloading offgrid-linux-amd64.tar.gz

```bashtar -xzf offgrid-linux-amd64.tar.gz

sudo apt-get install mesa-vulkan-drivers vulkan-tools# Binary is extracted directly, move to PATH

```sudo mv offgrid /usr/local/bin/

```

#### Windows - NVIDIA GPU

Install [CUDA Toolkit 12.4+](https://developer.nvidia.com/cuda-downloads)## Difference from Root `install.sh`



#### macOS**This directory** (`installers/`) contains scripts for **pre-built binaries**.

- **Apple Silicon**: Metal built-in ✅

- **Intel Mac**: CPU-only**Root `install.sh`** is for **building from source** on Linux with:

- llama.cpp compilation (~5-10 min)

---- GPU detection and configuration

- systemd service setup

## 🔧 Post-Installation- Full development environment



### Verify InstallationMost users should use the installers in this directory for faster installation.

```bash

offgrid version## What Gets Installed

offgrid info

```All installers place binaries in standard locations:



### Download Your First Model| Platform | Binary Location | Config Location |

```bash|----------|----------------|-----------------|

# Browse models| Linux | `/usr/local/bin/` | `~/.config/offgrid/` |

offgrid catalog| macOS | `/usr/local/bin/` or `/Applications/OffGrid.app/` | `~/Library/Application Support/OffGrid/` |

| Windows | `C:\Program Files\OffGrid\` | `%APPDATA%\OffGrid\` |

# Download small model (600MB)

offgrid download tinyllama-1.1b-chat Q4_K_M## Uninstallation

```

### Linux/macOS

### Start Using```bash

```bashsudo rm /usr/local/bin/offgrid

# Interactive chatsudo rm /usr/local/bin/llama-server  # if bundled

offgrid run tinyllama-1.1b-chatrm -rf ~/.config/offgrid              # config files

rm -rf ~/.local/share/offgrid         # data files

# API server (OpenAI-compatible)```

offgrid serve

```### Windows

```powershell

---# Run the uninstaller created during installation

powershell -ExecutionPolicy Bypass -File "C:\Program Files\OffGrid\Uninstall.ps1"

## 🐛 Troubleshooting```



### Linux: Permission denied## Troubleshooting

```bash

curl -fsSL ... | sudo bash### "Command not found" after installation

```

**Linux/macOS**: Restart terminal or run:

### Windows: Command not found after install```bash

Restart PowerShell terminal (PATH updated)export PATH="$PATH:/usr/local/bin"

```

### GPU not detected

**Linux:** Install vulkan-tools, then reinstall  **Windows**: Restart PowerShell or Command Prompt.

**Windows:** Install CUDA Toolkit, then reinstall

### Permission denied

---

**Linux/macOS**: Make sure to use `sudo` for system-wide installation.

## 🔄 Updating

**Windows**: Run PowerShell as Administrator.

Run the installer again - it replaces old binaries while keeping models/config.

### Already installed

---

The installers will detect and remove previous versions automatically.

## 🗑️ Uninstall

## Documentation

### Linux/macOS

```bash- Full build instructions: [../docs/BUILDING.md](../docs/BUILDING.md)

sudo rm -f /usr/local/bin/{offgrid,llama-server}- Distribution strategy: [../docs/DISTRIBUTION_STRATEGY.md](../docs/DISTRIBUTION_STRATEGY.md)

sudo rm -f /usr/local/lib/libggml*.so* /usr/local/lib/libllama*.so*- Quick reference: [../docs/QUICK_REFERENCE.md](../docs/QUICK_REFERENCE.md)

rm -rf ~/.offgrid-llm ~/.config/offgrid  # Optional: models/config
```

### Windows
```powershell
Remove-Item -Recurse "$env:LOCALAPPDATA\offgrid-llm"
Remove-Item -Recurse "$env:USERPROFILE\.offgrid-llm"  # Optional: models
```

---

## 📚 Documentation

- **Main README**: https://github.com/takuphilchan/offgrid-llm
- **API Reference**: https://github.com/takuphilchan/offgrid-llm/blob/main/docs/API.md
- **Build from Source**: https://github.com/takuphilchan/offgrid-llm/tree/main/dev
