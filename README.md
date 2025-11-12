<div align="center">

# OffGrid LLM

### *Offline-First AI Inference · Edge Computing · Zero Dependencies*

<p align="center">
  <strong>Run powerful language models completely offline with GPU acceleration</strong>
</p>

[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square)](https://github.com/takuphilchan/offgrid-llm/releases)
[![llama.cpp](https://img.shields.io/badge/Powered%20by-llama.cpp-green.svg?style=flat-square)](https://github.com/ggerganov/llama.cpp)

<p align="center">
  <a href="#-installation">Installation</a> •
  <a href="#-repository-structure">Structure</a> •
  <a href="#-features">Features</a> •
  <a href="#-usage">Usage</a> •
  <a href="#-api-reference">API</a> •
  <a href="#-documentation">Docs</a> •
  <a href="#contributing">Contributing</a>
</p>

</div>

---

## Why OffGrid LLM?

Built for **edge environments**, **air-gapped systems**, and **privacy-conscious deployments** where internet connectivity is limited or prohibited.

**Cross-platform support**: Linux, macOS (Intel & Apple Silicon), and Windows with native installers.

<table>
<tr>
<td width="50%">

**100% Offline Operation**
- No internet required after setup
- Complete data sovereignty
- Air-gapped deployment ready

</td>
<td width="50%">

**Production Ready**
- OpenAI-compatible API
- GPU acceleration (CUDA/ROCm/Metal)
- Cross-platform service integration

</td>
</tr>
<tr>
<td width="50%">

**Developer Friendly**
- Modern web UI included
- CLI with shell completions
- JSON output for automation

</td>
<td width="50%">

**Security First**
- Localhost-only binding
- No telemetry or tracking
- Process isolation & hardening

</td>
</tr>
</table>

---

## � Repository Structure

**New here? Here's what's where:**

| Location | Purpose | For |
|----------|---------|-----|
| **`installers/`** | 📦 **Quick install scripts** | End users - [Start here!](installers/) |
| `install.sh` (root) | 🔧 Linux source build with GPU setup | Advanced Linux users |
| `docs/` | 📖 Complete documentation | Everyone |
| `cmd/offgrid/` | 💻 Application source code | Developers |
| `internal/` | ⚙️ Core implementation | Developers |
| `.github/workflows/` | 🤖 CI/CD & automated releases | Contributors |
| `Makefile` | 🛠️ Build automation | Developers |

<details>
<summary><b>💡 Quick Navigation</b></summary>

- **Want to install?** → Use [`installers/install.sh`](installers/)
- **Want to contribute?** → Read [`CONTRIBUTING.md`](CONTRIBUTING.md)
- **Want to build from source?** → Use root `install.sh` (Linux) or `Makefile`
- **Looking for docs?** → Check [`docs/`](docs/) directory
- **Need help?** → See [Troubleshooting](#troubleshooting) or [open an issue](https://github.com/takuphilchan/offgrid-llm/issues)

</details>

---

## �📦 Installation

### Quick Install (Now Available!)

Pre-built binaries are now available from [GitHub Releases](https://github.com/takuphilchan/offgrid-llm/releases).

#### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

#### Windows (PowerShell as Admin)
Download the latest release from [Releases](https://github.com/takuphilchan/offgrid-llm/releases), extract, and run:
```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```

### Build from Source (Advanced)

For developers or those who want GPU optimization and full control:

#### Linux

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Full installation with llama.cpp compilation and GPU support (~10-15 min)
sudo ./install.sh
```

The installer automatically:
- Detects and configures GPU (NVIDIA CUDA, AMD ROCm)
- Compiles llama.cpp with optimizations
- Sets up systemd service
- Creates config directories

#### macOS

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install dependencies
brew install go cmake

# Build OffGrid
make build

# Install llama.cpp
brew install llama.cpp

# Move binary to PATH
sudo mv offgrid /usr/local/bin/
```

#### Windows

```powershell
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install Go 1.21+ from https://go.dev/dl/

# Build
go build -o offgrid.exe ./cmd/offgrid

# Install llama.cpp from https://github.com/ggerganov/llama.cpp/releases
```

### Manual Download

Download pre-built packages from [GitHub Releases](https://github.com/takuphilchan/offgrid-llm/releases):
- **Linux**: `offgrid-linux-amd64.tar.gz` or `offgrid-linux-arm64.tar.gz`
- **macOS**: `offgrid-darwin-amd64.tar.gz` (Intel) or `offgrid-darwin-arm64.tar.gz` (Apple Silicon)
- **Windows**: `offgrid-windows-amd64.zip` or `offgrid-windows-arm64.zip`

Extract and add to your PATH.

For developers or those who want maximum customization:

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Full installation with llama.cpp compilation and GPU support
sudo ./install.sh
```

### Pre-built Binaries (Coming Soon)

Once [GitHub Releases](https://github.com/takuphilchan/offgrid-llm/releases) are available, you can use these quick installers:

#### Linux / macOS
```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Full installation with llama.cpp compilation and GPU support
sudo ./install.sh
```

The source installer (`./install.sh` in the root directory) features:
- GPU detection and auto-configuration (NVIDIA CUDA, AMD ROCm)
- llama.cpp compilation with optimizations
- systemd service integration
- Security hardening and process isolation
- Professional progress bars and time estimates
- ~10-15 minutes installation time

See [docs/BUILDING.md](docs/BUILDING.md) for detailed build instructions.

<details>
<summary><b>📁 Installer Files Explained</b></summary>

To avoid confusion, here's what each installer does:

| File | Purpose | When to Use |
|------|---------|-------------|
| `./install.sh` (root) | **Full Linux source build** - Compiles llama.cpp, sets up systemd | Advanced users, GPU optimization, custom builds |
| `installers/install.sh` | **Universal quick installer** - Downloads pre-built binaries | Quick installation (NOW WORKING!) |
| `installers/install-macos.sh` | **macOS binary installer** - Installs from extracted archive | Manual macOS installation |
| `installers/install-windows.ps1` | **Windows PowerShell installer** - Adds to PATH | Manual Windows installation |

**Recommended**: Use the quick install curl command above for fastest setup.

</details>

---

## ✨ Features

<table>
<tr>
<td>

### 🎯 Core Capabilities
- ✅ **Offline-First** - Complete functionality without internet
- ✅ **GPU Accelerated** - NVIDIA CUDA & AMD ROCm support
- ✅ **OpenAI Compatible** - Standard API endpoints
- ✅ **HuggingFace Integration** - Direct model downloads
- ✅ **Auto Hot-Reload** - Model changes detected automatically
- ✅ **Modern Web UI** - Clean, responsive interface

</td>
<td>

### � Production Features
- ✅ **Session Management** - Save & resume conversations
- ✅ **Health Monitoring** - Kubernetes-ready probes
- ✅ **Shell Completions** - Bash/Zsh/Fish support
- ✅ **JSON Output** - Automation & CI/CD friendly
- ✅ **Systemd Services** - Auto-start on boot
- ✅ **Security Hardening** - Process isolation

</td>
</tr>
<tr>
<td>

### 🛠️ Productivity Tools
- 📝 **Prompt Templates** - 10 built-in templates
- ⚡ **Response Caching** - LRU cache with TTL
- 📚 **Batch Processing** - Parallel JSONL processing
- ⭐ **Aliases & Favorites** - Quick model access
- 💾 **USB Import/Export** - Portable deployments

</td>
<td>

### 📊 Developer Experience
- � **Model Search** - Filter by size, quant, author
- 🏥 **Health Endpoints** - `/health`, `/ready`, `/livez`
- 📈 **Statistics API** - Per-model metrics
- 🔧 **Flexible Config** - ENV vars & YAML support
- 🧪 **API Playground** - Built-in testing UI

</td>
</tr>
</table>

---

## 🚀 Quick Start

### One-Command Installation

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm && sudo ./install.sh
```

> **Note:** The installer features a professional progress bar, time estimates, and organized output for a smooth installation experience.

<details>
<summary><b>What gets installed?</b></summary>

The installer automatically:
- Detects GPU - Auto-configures NVIDIA CUDA or AMD ROCm
- Builds llama.cpp - Optimized inference engine with GPU support
- Installs systemd services - Auto-start on boot with proper isolation
- Security hardening - Localhost-only binding, process restrictions
- Go environment - Persistent configuration across reboots
---

## 🚀 Quick Start

### First Steps After Installation

```bash
# Check installation
offgrid --version

# Search for a model
offgrid search llama --limit 5

# Download a model (example: 4GB)
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# Start chatting
offgrid run Llama-3.2-3B-Instruct-Q4_K_M

# Access Web UI
firefox http://localhost:11611/ui

# Check system health
curl http://localhost:11611/health
```

### Web UI Preview

The included web interface provides:

| Feature | Description |
|---------|-------------|
| **Interactive Chat** | Real-time streaming with markdown & code highlighting |
| **Model Management** | Browse installed models with system stats |
| **API Playground** | Test endpoints with request/response viewer |
| **Health Monitor** | CPU, RAM, GPU metrics in real-time |

**Access at:** `http://localhost:11611/ui`

---

## 💻 Usage

### Model Management

<details>
<summary><b>Search & Download Models</b></summary>

```bash
# Search HuggingFace with filters
offgrid search llama --quant Q4_K_M --sort downloads --limit 10

# JSON output for scripting
offgrid search llama --limit 5 --json | jq '.results[].name'

# Download from HuggingFace
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF \
  --file llama-2-7b-chat.Q4_K_M.gguf

# List installed models
offgrid list

# List with JSON output
offgrid list --json | jq '.models[].name'
```

</details>

<details>
<summary><b>Import/Export for Offline Transfer</b></summary>

```bash
# Import models from USB/SD card
offgrid import /media/usb

# Export model to USB for transfer
offgrid export tinyllama-1.1b-chat /media/usb

# Remove a model
offgrid remove model-name

# Create portable package
./scripts/create-usb-package.sh /media/usb model-name
```

</details>

### Interactive Chat with Sessions

```bash
# Start chat and auto-save conversation
offgrid run Llama-3.2-3B-Instruct-Q4_K_M --save my-project

# Continue previous conversation
offgrid run Llama-3.2-3B-Instruct-Q4_K_M --load my-project

# List all saved sessions
offgrid session list

# View session details
offgrid session show my-project

# Export session to markdown
offgrid session export my-project output.md
```

### Prompt Templates

Built-in templates for common tasks:

```bash
# List available templates
offgrid template list

# Apply a template
offgrid template apply summarize      # Summarize text
offgrid template apply code-review    # Code review assistant
offgrid template apply translate      # Translation helper
offgrid template apply debug          # Debug code issues
```

**Available templates:**
`summarize` • `code-review` • `translate` • `explain` • `brainstorm` • `debug` • `document` • `refactor` • `test` • `cli`

### Automation & Scripting

<details>
<summary><b>JSON Output Mode</b></summary>

Perfect for CI/CD, monitoring, and automation:

```bash
# List models in JSON
offgrid list --json | jq '.count'

# Search with structured output
offgrid search llama --limit 5 --json | jq '.results[].name'

# Get system information
offgrid info --json | jq '{cpu: .system.cpu, models: .models.count}'

# Session management
offgrid session list --json | jq '.sessions[].name'
```

</details>

<details>
<summary><b>Batch Processing</b></summary>

Process multiple prompts in parallel:

```bash
# Create batch input file
cat > batch_input.jsonl << 'EOF'
{"id": "1", "model": "model.gguf", "prompt": "What is AI?"}
{"id": "2", "model": "model.gguf", "prompt": "Explain ML"}
{"id": "3", "model": "model.gguf", "prompt": "What is NLP?"}
EOF

# Process in parallel (4 workers)
offgrid batch process batch_input.jsonl results.jsonl --concurrency 4

# Check results
cat results.jsonl | jq .
```

</details>

---

## API Reference

### OpenAI-Compatible Endpoints

<details>
<summary><b>Chat Completion API</b></summary>

```bash
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Llama-3.2-3B-Instruct-Q4_K_M",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantum computing in simple terms"}
    ],
    "temperature": 0.7,
    "max_tokens": 500
  }'
```

**Response:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1731234567,
  "model": "Llama-3.2-3B-Instruct-Q4_K_M",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Quantum computing uses quantum mechanics..."
    },
    "finish_reason": "stop"
  }]
}
```

</details>

<details>
<summary><b>Health & Monitoring</b></summary>

| Endpoint | Purpose | Response |
|----------|---------|----------|
| `GET /health` | Full diagnostics | System health, CPU, RAM, models |
| `GET /ready` | Kubernetes readiness | HTTP 200 if ready |
| `GET /livez` | Liveness probe | HTTP 200 if alive |
| `GET /stats` | Model statistics | Per-model metrics |
| `GET /cache/stats` | Cache metrics | Hit rate, size, entries |

```bash
# Health check with full details
curl http://localhost:11611/health | jq

# Quick readiness check
curl -f http://localhost:11611/ready && echo "Ready!"

# Model statistics
curl http://localhost:11611/stats | jq '.models'
```

**Health Response:**
```json
{
  "status": "healthy",
  "version": "0.1.0",
  "uptime": "2h15m",
  "system": {
    "cpu_percent": 3.1,
    "memory_mb": 5161,
    "memory_percent": 43.1,
    "goroutines": 4
  },
  "models": {
    "available": 1,
    "loaded": 1
  }
}
```

</details>

<details>
<summary><b>Python Example</b></summary>

```python
from openai import OpenAI

# Connect to local OffGrid instance
client = OpenAI(
    base_url="http://localhost:11611/v1",
    api_key="not-needed"  # Local inference, no auth
)

# Chat completion
response = client.chat.completions.create(
    model="Llama-3.2-3B-Instruct-Q4_K_M",
    messages=[
        {"role": "system", "content": "You are a coding assistant."},
        {"role": "user", "content": "Write a Python function to reverse a string"}
    ],
    temperature=0.7,
    max_tokens=500
)

print(response.choices[0].message.content)
```

</details>

<details>
<summary><b>JavaScript/Node.js Example</b></summary>

```javascript
// Using fetch API
async function chat(prompt) {
  const response = await fetch('http://localhost:11611/v1/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model: 'Llama-3.2-3B-Instruct-Q4_K_M',
      messages: [
        { role: 'user', content: prompt }
      ]
    })
  });
  
  const data = await response.json();
  return data.choices[0].message.content;
}

// Usage
chat("Explain async/await in JavaScript")
  .then(console.log)
  .catch(console.error);
```

</details>

---

## Architecture

### System Design

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Layer                             │
│         Browser / CLI / API Client / Python SDK             │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP :11611 (OpenAI-compatible)
┌───────────────────▼─────────────────────────────────────────┐
│                 OffGrid LLM Server (Go)                     │
│  ┌──────────────┬──────────────┬──────────────────────┐    │
│  │ HTTP Router  │ Model Mgmt   │ Session Manager      │    │
│  │ Web UI       │ Health Check │ Response Cache       │    │
│  │ Statistics   │ API Adapter  │ Batch Processor      │    │
│  └──────────────┴──────────────┴──────────────────────┘    │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP (localhost-only, random port)
┌───────────────────▼─────────────────────────────────────────┐
│              llama-server (llama.cpp C++)                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  GGUF Loader  │  GPU Inference  │  Token Generation │  │
│  │  Context Mgmt │  KV Cache       │  Sampling         │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼
            [ GPU / CPU Hardware ]
```

### Directory Structure

```
/usr/local/bin/
├── offgrid              # Main CLI binary
└── llama-server         # llama.cpp inference engine

/var/lib/offgrid/
├── models/              # GGUF model storage
│   └── *.gguf
└── web/                 # Web UI files
    └── ui/index.html

/etc/offgrid/
├── active-model         # Current model config
└── llama-port           # Internal port config

~/.offgrid/
└── sessions/            # Saved chat sessions
    └── *.json

/etc/systemd/system/
├── offgrid-llm.service     # Main API service
└── llama-server.service    # Inference engine service
```

---

## System Requirements

### Hardware Requirements

<table>
<tr>
<th>Component</th>
<th>Minimum</th>
<th>Recommended</th>
</tr>
<tr>
<td><b>CPU</b></td>
<td>2 cores, 2GHz+</td>
<td>4+ cores, 3GHz+</td>
</tr>
<tr>
<td><b>RAM</b></td>
<td>2GB (Q4 models)</td>
<td>8GB+ (Q4-Q6 models)</td>
</tr>
<tr>
<td><b>Storage</b></td>
<td>2GB free space</td>
<td>10GB+ SSD</td>
</tr>
<tr>
<td><b>GPU</b></td>
<td>Optional (CPU fallback)</td>
<td>NVIDIA 6GB+ VRAM</td>
</tr>
<tr>
<td><b>OS</b></td>
<td>Ubuntu 20.04+</td>
<td>Ubuntu 22.04 LTS</td>
</tr>
</table>

### GPU Support

| Platform | Requirements | Status |
|----------|--------------|--------|
| **NVIDIA** | GTX 1050 Ti+, CUDA 12.0+ | ✅ Fully Supported |
| **AMD** | ROCm-compatible GPU | ⚠️ Experimental |
| **CPU** | Any x86_64 CPU | ✅ Supported (slower) |

### Model Size vs. Memory

> Use `offgrid quantization` to learn about quantization levels

| Model Size | Quant Level | RAM Required | GPU VRAM | Speed (tok/s)* |
|------------|-------------|--------------|----------|----------------|
| **1B** | Q4_K_M | ~2 GB | ~2 GB | 80-100 |
| **3B** | Q4_K_M | ~4 GB | ~4 GB | 35-45 |
| **7B** | Q4_K_M | ~8 GB | ~6 GB | 20-30 |
| **13B** | Q4_K_M | ~16 GB | ~12 GB | 10-15 |
| **70B** | Q4_K_M | ~48 GB | ~40 GB | 3-5 |

<sup>* RTX 4090 performance estimates</sup>

## Security

### Network Security
- Localhost-only binding (`127.0.0.1`)
- Random high ports for IPC
- No external network access
- Systemd security directives

### Process Isolation
- Dedicated `offgrid` system user
- Restricted file system access
- No root privileges for operation

### Data Privacy
- No telemetry or analytics
- No external API calls
- All inference runs locally
- Complete data sovereignty

---

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/ARCHITECTURE.md) | System design & components |
| [API Reference](docs/API.md) | Complete API documentation |
| [CLI Reference](docs/CLI_REFERENCE.md) | All CLI commands & flags |
| [Model Setup](docs/MODEL_SETUP.md) | Model installation guide |
| [Deployment](docs/DEPLOYMENT.md) | Production deployment guide |
| [HuggingFace Integration](docs/HUGGINGFACE_INTEGRATION.md) | Model search & download |
| [llama.cpp Setup](docs/LLAMA_CPP_SETUP.md) | Building the inference engine |
| [JSON Output](docs/JSON_OUTPUT.md) | Automation & scripting guide |
| [Features Guide](docs/FEATURES_GUIDE.md) | Feature walkthrough |
| [Quickstart](docs/QUICKSTART_HF.md) | Quick HuggingFace setup |

---

## Troubleshooting

<details>
<summary><b>Service Management Issues</b></summary>

```bash
# Check service status
sudo systemctl status offgrid-llm llama-server

# View live logs
sudo journalctl -u offgrid-llm -f
sudo journalctl -u llama-server -f

# Restart services
sudo systemctl restart llama-server
sudo systemctl restart offgrid-llm

# Test connectivity
curl http://localhost:11611/health
```

</details>

<details>
<summary><b>Model Loading Problems</b></summary>

```bash
# Verify models exist
offgrid list
ls -lh /var/lib/offgrid/models/

# Fix permissions
sudo chown -R offgrid:offgrid /var/lib/offgrid/models/
sudo chmod 664 /var/lib/offgrid/models/*.gguf

# Verify GGUF format
file /var/lib/offgrid/models/*.gguf

# Force model reload
sudo systemctl restart llama-server
```

</details>

<details>
<summary><b>GPU Not Detected</b></summary>

```bash
# Check GPU availability
nvidia-smi              # NVIDIA
rocm-smi               # AMD

# Verify CUDA installation
nvcc --version

# Check llama-server compilation
ldd /usr/local/bin/llama-server | grep cuda

# Reinstall with GPU support
cd /path/to/offgrid-llm
sudo ./install.sh --gpu

# Force CPU-only mode
sudo ./install.sh --cpu-only
```

</details>

<details>
<summary><b>Port Already in Use</b></summary>

```bash
# Find process using port 11611
sudo lsof -i :11611
sudo netstat -tulpn | grep 11611

# Kill the process
sudo kill -9 <PID>

# Or change OffGrid port
export OFFGRID_PORT=11612
sudo systemctl restart offgrid-llm
```

</details>

---

## 🔒 Security Features

<table>
<tr>
<td width="50%">

### Network Security
- ✅ Localhost-only binding (`127.0.0.1`)
- ✅ Random high ports for IPC
- ✅ No external network access
- ✅ Systemd security directives
- ✅ Firewall-friendly (single port)

</td>
<td width="50%">

### Data Privacy
- ✅ No telemetry or analytics
- ✅ No external API calls
- ✅ All inference runs locally
- ✅ Complete data sovereignty
- ✅ GDPR/HIPAA friendly

</td>
</tr>
<tr>
<td width="50%">

### Process Isolation
- ✅ Dedicated system user
- ✅ Restricted file access
- ✅ No root privileges
- ✅ SELinux compatible

</td>
<td width="50%">

### Production Hardening
- ✅ Read-only system files
- ✅ Private temp directories
- ✅ Capability restrictions
- ✅ Resource limits

</td>
</tr>
</table>

---

## 📦 Offline Deployment

### Air-Gapped Installation

<details>
<summary><b>Step-by-step guide</b></summary>

**On internet-connected machine:**
```bash
# 1. Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# 2. Download models
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# 3. Create offline package
tar czf offgrid-package.tar.gz \
  offgrid-llm/ \
  ~/.offgrid/models/

# 4. Transfer to offline system (USB, etc.)
```

**On air-gapped machine:**
```bash
# 1. Extract package
tar xzf offgrid-package.tar.gz

# 2. Install
cd offgrid-llm
sudo ./install.sh --cpu-only  # Or --gpu if GPU available

# 3. Import models
sudo cp ~/.offgrid/models/*.gguf /var/lib/offgrid/models/
sudo chown offgrid:offgrid /var/lib/offgrid/models/*.gguf

# 4. Restart services
sudo systemctl restart llama-server offgrid-llm

# 5. Verify
curl http://localhost:11611/health
```

</details>

### USB Model Transfer

```bash
# Create portable USB package
./scripts/create-usb-package.sh /media/usb model-name

# Import on destination system
offgrid import /media/usb
```

---

## 💻 Development

<details>
<summary><b>Build from Source</b></summary>

```bash
# Install prerequisites
sudo apt-get update
sudo apt-get install -y build-essential cmake git

# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Build binary
go build -o offgrid ./cmd/offgrid

# Run development server
./offgrid serve
```

</details>

<details>
<summary><b>Running Tests</b></summary>

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./internal/server

# Specific test
go test -run TestHealthEndpoint ./internal/server
```

</details>

<details>
<summary><b>Project Structure</b></summary>

```
offgrid-llm/
├── cmd/offgrid/          # Main CLI entry point
├── internal/
│   ├── server/           # HTTP server & API
│   ├── inference/        # llama.cpp integration
│   ├── models/           # Model management
│   ├── sessions/         # Session storage
│   ├── cache/            # Response cache
│   └── templates/        # Prompt templates
├── pkg/api/              # Public API types
├── web/ui/               # Web interface
├── docs/                 # Documentation
├── scripts/              # Helper scripts
└── install.sh            # Installation script
```

</details>

---

## Contributing

We welcome contributions! 🎉

**Please read [CONTRIBUTING.md](CONTRIBUTING.md) for:**
- 📁 Repository structure explained
- 🛠️ Development workflow
- 🚀 How to build and test
- 📦 Creating releases
- ❓ Getting help

**Quick links:**
- 🐛 [Report bugs](https://github.com/takuphilchan/offgrid-llm/issues)
- 💡 [Request features](https://github.com/takuphilchan/offgrid-llm/issues)
- 📖 [Improve docs](https://github.com/takuphilchan/offgrid-llm/tree/main/docs)
- 💻 [Submit code](https://github.com/takuphilchan/offgrid-llm/pulls)

**Priority areas:**
- Performance optimization & benchmarking
- Platform support (macOS/Windows)
- Web UI improvements
- Documentation & tutorials

---

## License

MIT License - See [LICENSE](LICENSE) for details

---

## Acknowledgments

This project builds on excellent work from:

- [**llama.cpp**](https://github.com/ggerganov/llama.cpp) - High-performance C++ inference engine
- [**HuggingFace**](https://huggingface.co) - Model hosting and GGUF format
- [**GGUF Format**](https://github.com/ggerganov/ggml) - Efficient model serialization
- **Open Source Community** - For making AI accessible

---

## Links

<div align="center">

[![GitHub](https://img.shields.io/badge/GitHub-Repository-181717?style=for-the-badge&logo=github)](https://github.com/takuphilchan/offgrid-llm)
[![Issues](https://img.shields.io/badge/GitHub-Issues-green?style=for-the-badge&logo=github)](https://github.com/takuphilchan/offgrid-llm/issues)
[![Discussions](https://img.shields.io/badge/GitHub-Discussions-blue?style=for-the-badge&logo=github)](https://github.com/takuphilchan/offgrid-llm/discussions)

</div>

---

<div align="center">

**Built for offline-first deployment · Zero external dependencies · Complete data sovereignty**

</div>
