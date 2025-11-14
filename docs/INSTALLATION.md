# OffGrid LLM Installation Guide

## Overview

OffGrid LLM features a professional, automated installer that handles everything from dependency detection to service configuration. The installation process is designed to be clear, organized, and informative with real-time progress tracking.

## Features

### Professional Installation Experience

- **Progress Tracking** - Visual progress bar with step count and percentage
- **Time Estimates** - Each step shows estimated completion time
- **Organized Output** - Clean, color-coded messages with clear sections
- **Error Handling** - Detailed error messages with troubleshooting hints
- **Pre-flight Checks** - Verifies system before starting installation
- **Installation Summary** - Comprehensive report at completion

### Installation Progress Display

```
╭────────────────────────────────────────────────────────────────────╮
│ Step 7/14 [████████████░░░░░░░░] 50% │ Elapsed: 05:32
╰────────────────────────────────────────────────────────────────────╯

◆ Building llama.cpp Inference Engine
────────────────────────────────────────────────────────
  Estimated time: ~5-10 minutes

  Configuring build with CMake...
  Found CUDA toolkit: 12.2 at /usr/local/cuda
  llama-server built successfully
  Libraries installed system-wide
```

## Quick Start

### Standard Installation

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
sudo ./install.sh
```

**Estimated Time:** 10-15 minutes

### Installation Options

```bash
# Auto-detect GPU (default)
sudo ./install.sh

# Force CPU-only mode
sudo ./install.sh --cpu-only

# Require GPU (fail if not detected)
sudo ./install.sh --gpu

# Show help
./install.sh --help
```

## Installation Steps

The installer performs these steps automatically:

### 1. System Checks (30 seconds)
- Verify required dependencies (curl, git, cmake, etc.)
- Detect system architecture (x86_64/arm64)
- Identify operating system and package manager
- Detect GPU hardware (NVIDIA/AMD/none)

### 2. Build Dependencies (2-3 minutes)
- Install build tools (gcc, g++, make, cmake)
- Install GPU-specific packages (CUDA/ROCm if needed)
- Update package lists

### 3. Go Installation (1-2 minutes)
- Download Go 1.21.13
- Install to `/usr/local/go`
- Configure PATH for persistent access
- Verify installation

### 4. GPU Configuration (1 minute)
- Install/verify NVIDIA drivers (if NVIDIA GPU)
- Load kernel modules
- Check CUDA toolkit availability

### 5. llama.cpp Build (5-10 minutes)
- Clone/update llama.cpp repository
- Configure CMake with GPU support
- Build llama-server binary
- Install shared libraries system-wide

### 6. OffGrid Build (2-3 minutes)
- Download Go dependencies
- Build OffGrid binary
- Run tests

### 7. System Configuration (1 minute)
- Create service user `offgrid`
- Set up directory structure at `/var/lib/offgrid`
- Configure file permissions
- Create model directory

### 8. Service Setup (1 minute)
- Create systemd service files
- Configure llama-server (internal, localhost-only)
- Configure OffGrid LLM (public API on port 11611)
- Enable auto-start on boot

### 9. Installation (30 seconds)
- Install binaries to `/usr/local/bin`
- Install shell completions
- Update library cache

### 10. Service Start (30 seconds)
- Start llama-server
- Start OffGrid LLM
- Verify health checks

## Installation Summary

Upon completion, you'll see a comprehensive summary:

```
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║              INSTALLATION COMPLETE                            ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝

╭─────────────────────────────────────────────────────────────────╮
│ SYSTEM INFORMATION
├─────────────────────────────────────────────────────────────────┤
│  Architecture     amd64
│  Operating System Ubuntu 22.04
│  GPU Type         nvidia
│  GPU Info         NVIDIA GeForce RTX 3080
│  Inference Mode   REAL LLM (via llama.cpp)
│  Install Time     12:34
╰─────────────────────────────────────────────────────────────────╯

╭─────────────────────────────────────────────────────────────────╮
│ SERVICE ENDPOINTS
├─────────────────────────────────────────────────────────────────┤
│  llama-server     http://127.0.0.1:52341 (internal only)
│  OffGrid API      http://localhost:11611
│  Web UI           http://localhost:11611/ui
╰─────────────────────────────────────────────────────────────────╯

╭─────────────────────────────────────────────────────────────────╮
│ SECURITY
├─────────────────────────────────────────────────────────────────┤
│  llama-server bound to 127.0.0.1 only (internal IPC)
│  Random high port 52341 not exposed externally
│  Only OffGrid port 11611 is publicly accessible
│  Same architecture as Ollama for security and isolation
╰─────────────────────────────────────────────────────────────────╯
```

## System Requirements

### Minimum Requirements
- **OS:** Ubuntu 20.04+, Debian 11+, Fedora 35+, or compatible Linux distribution
- **CPU:** x86_64 or ARM64 architecture
- **RAM:** 8 GB (for 7B models)
- **Disk:** 20 GB free space
- **Privileges:** sudo access

### Recommended for GPU Acceleration
- **NVIDIA GPU:** GTX 1060 or better, CUDA 11.7+
- **AMD GPU:** RX 580 or better, ROCm 5.0+
- **RAM:** 16 GB
- **VRAM:** 6 GB+ for 7B models, 24 GB+ for 70B models

## What Gets Installed

### Binaries
- `/usr/local/bin/offgrid` - Main OffGrid CLI
- `/usr/local/bin/llama-server` - llama.cpp inference server
- `/usr/local/bin/llama-server-start.sh` - Service startup script

### Directories
- `/var/lib/offgrid/` - Data directory (owned by `offgrid` user)
- `/var/lib/offgrid/models/` - Model storage (writable by `offgrid` group)
- `/var/lib/offgrid/web/ui/` - Web UI files
- `/etc/offgrid/` - Configuration files
- `$HOME/llama.cpp/` - llama.cpp source and build

### Services
- `llama-server.service` - Internal inference server (localhost-only)
- `offgrid-llm.service` - Public API service (port 11611)

### Configuration Files
- `/etc/offgrid/llama-port` - Internal port configuration
- `/etc/offgrid/active-model` - Currently loaded model
- `/etc/systemd/system/llama-server.service` - Service definition
- `/etc/systemd/system/offgrid-llm.service` - Service definition

## Verification

### Check Installation

```bash
# Verify binary installation
which offgrid
offgrid --version

# Check services
sudo systemctl status offgrid-llm
sudo systemctl status llama-server

# Test health endpoint
curl http://localhost:11611/health

# View logs
sudo journalctl -u offgrid-llm -n 50
sudo journalctl -u llama-server -n 50
```

### Test API

```bash
# List models
offgrid list

# Download a model
offgrid download tinyllama

# Test chat
curl -X POST http://localhost:11611/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## Troubleshooting

### Installation Failed

**Check the error log:**
```bash
# The installer shows the exact line where it failed
# Look for error messages in red
```

**Common issues:**

1. **Missing sudo access**
   ```bash
   # Error: Permission denied
   # Solution: Run with sudo
   sudo ./install.sh
   ```

2. **Conflicting package managers**
   ```bash
   # Error: Could not determine package manager
   # Solution: Ensure you're on a supported Linux distribution
   cat /etc/os-release
   ```

3. **GPU not detected**
   ```bash
   # Warning: No GPU detected - will use CPU inference
   # Solution: Install GPU drivers first, or use --cpu-only
   sudo ./install.sh --cpu-only
   ```

4. **Build failed**
   ```bash
   # Error: Build failed with exit code 2
   # Solution: Check build log
   tail -50 /tmp/go_build.log
   ```

### Service Won't Start

**Check service status:**
```bash
sudo systemctl status offgrid-llm
sudo systemctl status llama-server
```

**View detailed logs:**
```bash
sudo journalctl -u offgrid-llm -n 100 --no-pager
sudo journalctl -u llama-server -n 100 --no-pager
```

**Common fixes:**

1. **Port already in use**
   ```bash
   # Check what's using port 11611
   sudo netstat -tlnp | grep 11611
   # Kill the process or change OffGrid port
   ```

2. **Model not found**
   ```bash
   # Download a model
   offgrid download tinyllama
   # Or check model directory
   ls -la /var/lib/offgrid/models/
   ```

3. **Permission issues**
   ```bash
   # Fix ownership
   sudo chown -R offgrid:offgrid /var/lib/offgrid
   sudo chmod 2775 /var/lib/offgrid/models
   ```

## Uninstallation

To completely remove OffGrid LLM:

```bash
# Stop services
sudo systemctl stop offgrid-llm llama-server
sudo systemctl disable offgrid-llm llama-server

# Remove service files
sudo rm /etc/systemd/system/offgrid-llm.service
sudo rm /etc/systemd/system/llama-server.service
sudo systemctl daemon-reload

# Remove binaries
sudo rm /usr/local/bin/offgrid
sudo rm /usr/local/bin/llama-server
sudo rm /usr/local/bin/llama-server-start.sh

# Remove data (WARNING: deletes all models and sessions)
sudo rm -rf /var/lib/offgrid
sudo rm -rf /etc/offgrid

# Remove user
sudo userdel -r offgrid

# Remove llama.cpp (optional)
rm -rf $HOME/llama.cpp

# Remove shell completions (optional)
rm -f ~/.bash_completion.d/offgrid
rm -f ~/.local/share/zsh/site-functions/_offgrid
rm -f ~/.config/fish/completions/offgrid.fish
```

## Advanced Configuration

### Custom Installation Path

The installer uses standard paths, but you can modify service files after installation:

```bash
# Edit service files
sudo systemctl edit offgrid-llm
sudo systemctl edit llama-server

# Reload
sudo systemctl daemon-reload
sudo systemctl restart offgrid-llm llama-server
```

### Custom Model Directory

```bash
# Create symlink to different location
sudo systemctl stop offgrid-llm llama-server
sudo mv /var/lib/offgrid/models /path/to/large/disk/models
sudo ln -s /path/to/large/disk/models /var/lib/offgrid/models
sudo systemctl start offgrid-llm llama-server
```

### Multiple Instances

To run multiple OffGrid instances:

```bash
# Copy and modify service file
sudo cp /etc/systemd/system/offgrid-llm.service \
       /etc/systemd/system/offgrid-llm-2.service

# Edit to use different port
sudo nano /etc/systemd/system/offgrid-llm-2.service
# Change: Environment="OFFGRID_PORT=11612"

# Start second instance
sudo systemctl enable offgrid-llm-2
sudo systemctl start offgrid-llm-2
```

## Next Steps

After installation:

1. **Visit the Web UI:** http://localhost:11611/ui
2. **Download models:** `offgrid download tinyllama`
3. **Read the docs:** `docs/FEATURES_GUIDE.md`
4. **Try the API:** `docs/API.md`
5. **Configure systemd:** `docs/DEPLOYMENT.md`

## Support

- **Documentation:** [docs/](../docs/)
- **Issues:** [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)
- **Discussions:** [GitHub Discussions](https://github.com/takuphilchan/offgrid-llm/discussions)
