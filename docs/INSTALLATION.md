# OffGrid LLM Installation Guide

## Quick Install (Recommended)

**For most users, use the automated installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

**What it does:**
- Builds OffGrid LLM from source
- Optimized for your CPU and GPU
- Installs web interface automatically
- Sets up auto-start service (optional)
- Takes 5-10 minutes

**Skip prompts (auto-start enabled):**
```bash
AUTOSTART=yes bash <(curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh)
```

After installation, open: `http://localhost:11611/ui/`

---

## Advanced Installation

### Full Build from Source (Developers)

**For GPU optimization and development:**

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
sudo ./dev/install.sh
```

**This installer:**
- Builds llama.cpp with full GPU optimization
- Compiles OffGrid from source
- Sets up systemd services
- Installs web interface
- Takes 10-15 minutes

**Installation options:**

```bash
# Auto-detect GPU (recommended)
sudo ./dev/install.sh

# Force CPU-only mode
sudo ./dev/install.sh --cpu-only

# Require GPU (fail if not found)
sudo ./dev/install.sh --gpu

# Show help
./dev/install.sh --help
```

---

## What Gets Installed

### Binaries
- `/usr/local/bin/offgrid` - Main CLI tool
- `/usr/local/bin/llama-server` - Inference engine (from llama.cpp)

### Web Interface
- `/var/lib/offgrid/web/ui/` - Web UI files

### Models Directory
- `/var/lib/offgrid/models/` - Downloaded AI models stored here

### Systemd Services (Linux)
- `offgrid@<user>.service` - OffGrid API server
- Auto-starts on boot (if enabled during install)

---

## Installation Process

The installer performs these steps:

### 1. System Detection (~1 minute)
- Check dependencies (curl, git, cmake, gcc, etc.)
- Detect OS, architecture (x64/arm64)
- Identify GPU (NVIDIA, AMD, or none)
- Verify Go installation or install it

### 2. Build llama.cpp (~5-10 minutes)
- Clone llama.cpp repository
- Configure with CMake (GPU-optimized if available)
- Build llama-server binary
- Install system-wide

### 3. Build OffGrid (~2-3 minutes)
- Download Go dependencies
- Compile OffGrid binary
- Run basic tests

### 4. System Setup (~1 minute)
- Create `/var/lib/offgrid` directory
- Install web UI files
- Set up models directory
- Configure systemd service

### 5. Start Services (~30 seconds)
- Start OffGrid server
- Verify health endpoints
- Display access URLs

---

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
## After Installation

### Verify It Works

```bash
# Check if offgrid is installed
offgrid --version

# Check service status (Linux)
systemctl status offgrid@$USER

# Test health endpoint
curl http://localhost:11611/health
```

### Access the Web Interface

Open in your browser: `http://localhost:11611/ui/`

### Download Your First Model

```bash
# Search for models
offgrid search llama --limit 5

# Download a small model (~4GB)
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf
```

---

## Troubleshooting

### Installation Issues

**"Permission denied" error:**
```bash
# Run with sudo
sudo bash <(curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh)
```

**"GPU not detected" warning:**
- This is OK - it will use CPU mode
- Or install GPU drivers first and reinstall

**Build failed:**
```bash
# Check you have enough disk space (need 5GB+)
df -h

# Make sure you have build tools
sudo apt-get install build-essential cmake git
```

### Service Won't Start

```bash
# Check service status
systemctl status offgrid@$USER

# View logs
journalctl -u offgrid@$USER -n 50

# Restart service
systemctl restart offgrid@$USER
```

### Web UI Not Working

```bash
# Make sure service is running
systemctl status offgrid@$USER

# Check if UI files exist
ls -la /var/lib/offgrid/web/ui/

# If missing, reinstall or copy manually:
sudo mkdir -p /var/lib/offgrid/web/ui
sudo cp -r web/ui/* /var/lib/offgrid/web/ui/
```

---

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
## Uninstall

**Stop and remove services:**
```bash
# Stop service
systemctl stop offgrid@$USER

# Disable auto-start
systemctl disable offgrid@$USER

# Remove binaries
sudo rm /usr/local/bin/offgrid
sudo rm /usr/local/bin/llama-server
```

**Remove data (warning: deletes all models):**
```bash
sudo rm -rf /var/lib/offgrid
```

---

## Next Steps

- [Download models](MODEL_SETUP.md)
- [Learn CLI commands](CLI_REFERENCE.md)
- [Use the API](API.md)
- [Read feature guide](guides/FEATURES_GUIDE.md)

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
