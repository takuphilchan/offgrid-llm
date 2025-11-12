# OffGrid LLM

**Offline-First AI Inference · Edge Computing · Zero Dependencies**

Run powerful language models completely offline with GPU acceleration

[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square)](https://github.com/takuphilchan/offgrid-llm/releases)
[![llama.cpp](https://img.shields.io/badge/Powered%20by-llama.cpp-green.svg?style=flat-square)](https://github.com/ggerganov/llama.cpp)

**Quick Links:** [Installation](#-installation) · [Structure](#-repository-structure) · [Features](#-features) · [Usage](#-quick-start) · [API](#-api-reference) · [Docs](#-documentation) · [Contributing](#contributing)

---

## Why OffGrid LLM?

Built for **edge environments**, **air-gapped systems**, and **privacy-conscious deployments** where internet connectivity is limited or prohibited.

**Cross-platform support**: Linux, macOS (Intel & Apple Silicon), and Windows with native installers.

**100% Offline Operation**
- No internet required after setup
- Complete data sovereignty
- Air-gapped deployment ready

**Production Ready**
- OpenAI-compatible API
- GPU acceleration (CUDA/ROCm/Metal)
- Cross-platform service integration

**Developer Friendly**
- Modern web UI included
- CLI with shell completions
- JSON output for automation

**Security First**
- Localhost-only binding
- No telemetry or tracking
- Process isolation & hardening

---

## 📁 Repository Structure

**New here? Here's what's where:**

| Location | Purpose | For |
|----------|---------|-----|
| **`installers/`** | **One-command install scripts** | Everyone - [Start here!](installers/) |
| `dev/` | Build from source with GPU optimization | Developers |
| `docs/` | Complete documentation | Everyone |
| `cmd/offgrid/` | Application source code | Developers |
| `internal/` | Core implementation | Developers |
| `.github/workflows/` | CI/CD & automated releases | Contributors |

**Quick Navigation:**

- **Want to install?** → Use [`installers/install.sh`](installers/) (Linux/Mac) or [`installers/install.ps1`](installers/) (Windows)
- **Want to contribute?** → Read [`CONTRIBUTING.md`](CONTRIBUTING.md)  
- **Want to build from source?** → Use [`dev/install.sh`](dev/)
- **Looking for docs?** → Check [`docs/`](docs/) directory
- **Need help?** → See [Troubleshooting](#troubleshooting) or [open an issue](https://github.com/takuphilchan/offgrid-llm/issues)

---

## Installation

Choose the installation method that fits your needs:

### Quick Install (Recommended for Most Users)

**Fast, pre-built binaries - ready in 10-15 seconds:**

#### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

#### Windows (PowerShell as Administrator)
```powershell
iwr -useb https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.ps1 | iex
```

**What you get:**
- ✅ Pre-built binaries (no compilation)
- ✅ Auto-detects GPU and installs optimized version
- ✅ Sets up PATH automatically
- ✅ Manual start: Run `offgrid server start` when you need it

**Best for:** Development, testing, or manual usage

For detailed instructions, see **[installers/README.md](installers/README.md)**.

---

### Production Install (Systemd Service)

**Build from source with automatic startup on boot:**

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Full build with GPU optimization (~10-15 min)
sudo ./dev/install.sh
```

**What you get:**
- ✅ Built from source with custom optimizations
- ✅ Systemd services (auto-start on boot)
- ✅ Background service operation
- ✅ Production-ready security hardening
- ✅ Automatic model hot-reload

**Best for:** Production servers, always-on deployments, automatic startup

See **[dev/README.md](dev/README.md)** for advanced build options.

---

## Features

### Core Capabilities

- **Offline-First** - Complete functionality without internet
- **GPU Accelerated** - NVIDIA CUDA & AMD ROCm support
- **OpenAI Compatible** - Standard API endpoints
- **HuggingFace Integration** - Direct model downloads
- **Auto Hot-Reload** - Model changes detected automatically
- **Modern Web UI** - Clean, responsive interface

### Production Features

- **Session Management** - Save & resume conversations
- **Health Monitoring** - Kubernetes-ready probes
- **Shell Completions** - Bash/Zsh/Fish support
- **JSON Output** - Automation & CI/CD friendly
- **Systemd Services** - Auto-start on boot
- **Security Hardening** - Process isolation

### Productivity Tools

- **Prompt Templates** - 10 built-in templates
- **Response Caching** - LRU cache with TTL
- **Batch Processing** - Parallel JSONL processing
- **Aliases & Favorites** - Quick model access
- **USB Import/Export** - Portable deployments

### Developer Experience

- **Model Search** - Filter by size, quant, author
- **Health Endpoints** - `/health`, `/ready`, `/livez`
- **Statistics API** - Per-model metrics
- **Flexible Config** - ENV vars & YAML support
- **API Playground** - Built-in testing UI

---

## Quick Start

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

## Usage

### Model Management

**Search & Download Models**

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

**Import/Export for Offline Transfer**

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

**JSON Output Mode**

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

**Batch Processing**

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

---

## API Reference

### OpenAI-Compatible Endpoints

**Chat Completion API**

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

**Health & Monitoring**

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

**Python Example**

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

**JavaScript/Node.js Example**

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

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| **CPU** | 2 cores, 2GHz+ | 4+ cores, 3GHz+ |
| **RAM** | 2GB (Q4 models) | 8GB+ (Q4-Q6 models) |
| **Storage** | 2GB free space | 10GB+ SSD |
| **GPU** | Optional (CPU fallback) | NVIDIA 6GB+ VRAM |
| **OS** | Ubuntu 20.04+ | Ubuntu 22.04 LTS |

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

**Service Management Issues**

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

**Model Loading Problems**

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

**GPU Not Detected**

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

**Port Already in Use**

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

---

## Security Features

### Network Security

- Localhost-only binding (`127.0.0.1`)
- Random high ports for IPC
- No external network access
- Systemd security directives
- Firewall-friendly (single port)

### Data Privacy

- No telemetry or analytics
- No external API calls
- All inference runs locally
- Complete data sovereignty
- GDPR/HIPAA friendly

### Process Isolation

- Dedicated system user
- Restricted file access
- No root privileges
- SELinux compatible

### Production Hardening

- Read-only system files
- Private temp directories
- Capability restrictions
- Resource limits

---

## Offline Deployment

### Air-Gapped Installation

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

### USB Model Transfer

```bash
# Create portable USB package
./scripts/create-usb-package.sh /media/usb model-name

# Import on destination system
offgrid import /media/usb
```

---

## Development

**Build from Source**

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

**Running Tests**

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

**Project Structure**

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

---

## Contributing

We welcome contributions!

**Please read [CONTRIBUTING.md](CONTRIBUTING.md) for:**
- Repository structure explained
- Development workflow
- How to build and test
- Creating releases
- Getting help

**Quick links:**
- [Report bugs](https://github.com/takuphilchan/offgrid-llm/issues)
- [Request features](https://github.com/takuphilchan/offgrid-llm/issues)
- [Improve docs](https://github.com/takuphilchan/offgrid-llm/tree/main/docs)
- [Submit code](https://github.com/takuphilchan/offgrid-llm/pulls)

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

[![GitHub](https://img.shields.io/badge/GitHub-Repository-181717?style=for-the-badge&logo=github)](https://github.com/takuphilchan/offgrid-llm)
[![Issues](https://img.shields.io/badge/GitHub-Issues-green?style=for-the-badge&logo=github)](https://github.com/takuphilchan/offgrid-llm/issues)
[![Discussions](https://img.shields.io/badge/GitHub-Discussions-blue?style=for-the-badge&logo=github)](https://github.com/takuphilchan/offgrid-llm/discussions)

---

**Built for offline-first deployment · Zero external dependencies · Complete data sovereignty**
