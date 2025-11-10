# OffGrid LLM

**Edge Inference Orchestrator · Offline-First AI**

A self-contained LLM inference system for environments with limited connectivity. Built with Go and powered by [llama.cpp](https://github.com/ggerganov/llama.cpp), providing an OpenAI-compatible API with GPU acceleration.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Linux-FCC624?logo=linux)](https://www.linux.org)

## ✨ Features

### Core Capabilities
- 🔌 **Offline-First** - Complete functionality without internet after setup
- 🚀 **GPU Acceleration** - NVIDIA CUDA and AMD ROCm support with CPU fallback
- 🔄 **OpenAI Compatible** - Standard `/v1/chat/completions` API endpoints
- 🤗 **HuggingFace Integration** - Direct model search and download
- 📦 **Model Management** - Automatic detection, hot-reload, integrity verification
- 🎨 **Modern Web UI** - Clean HTML/CSS/JS interface with streaming chat, system monitoring, and API testing
- 🖥️ **Desktop App** - Cross-platform Electron app for Windows, macOS, and Linux (fully offline)

### New Features (v0.1.0-alpha)
- 💬 **Session Management** - Save, load, and export chat conversations
- 🏥 **Health Endpoints** - Kubernetes-ready `/health`, `/ready`, `/livez` probes
- ⌨️ **Shell Completions** - Bash, Zsh, and Fish tab completion
- 📊 **JSON Output Mode** - Machine-readable output for automation (`--json` flag)

### Productivity Tools
- 📝 **Prompt Templates** - 10 built-in templates (code-review, summarize, translate, etc.)
- ⚡ **Response Caching** - LRU cache with configurable TTL for faster queries
- 📚 **Batch Processing** - Process multiple prompts in parallel from JSONL files
- ⭐ **Aliases & Favorites** - Friendly names and starred models for quick access
- 💾 **Portable** - USB/SD card model import/export for air-gapped deployments

### Production Ready
- 🔒 **Security Hardening** - Localhost-only, dedicated user, restricted permissions
- 📈 **Monitoring** - Health checks, statistics, resource tracking
- 🔧 **Systemd Services** - Automatic startup and management
- 🐳 **Container Ready** - Docker support (coming soon)

## 🚀 Quick Start

### One-Line Installation

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm && sudo ./install.sh
```

The installer automatically:
- ✅ Detects GPU (NVIDIA/AMD) and configures drivers
- ✅ Compiles llama.cpp with optimal settings
- ✅ Installs systemd services
- ✅ Sets up security (localhost-only, restricted permissions)
- ✅ Installs shell completions (bash/zsh/fish)
- ✅ Creates model directory at `/var/lib/offgrid/models`

### First Steps

```bash
# 1. Search for a model
offgrid search llama --limit 5

# 2. Download a model
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# 3. Start chatting
offgrid run Llama-3.2-3B-Instruct-Q4_K_M

# 4. Access Web UI
firefox http://localhost:11611/ui

# 5. Check health
curl http://localhost:11611/health
```

### Desktop Application

Want a standalone app? Build the desktop version:

```bash
# Install Node.js dependencies
cd desktop
npm install

# Run in development mode
npm start

# Build for your platform
npm run build

# Build for all platforms
npm run build:all
```

Installers will be in `desktop/dist/`:
- **Windows**: `.exe` installer and portable version
- **macOS**: `.dmg` and `.zip`
- **Linux**: `.AppImage`, `.deb`, `.rpm`

The desktop app:
- ✅ Runs 100% offline (no internet needed)
- ✅ Auto-starts the Go server
- ✅ System tray integration
- ✅ Native menus and notifications
- ✅ Single-click installation

See [desktop/README.md](desktop/README.md) for details.

### Web UI Features

The web interface provides:
- **Interactive Chat** - Real-time streaming with markdown and code highlighting
- **Model Management** - View models and system resources
- **API Testing** - Interactive playground with request/response testing

## 📖 Usage

### Model Management

**Search HuggingFace:**
```bash
# Search with filters
offgrid search llama --quant Q4_K_M --sort downloads --limit 10

# JSON output for scripting
offgrid search llama --limit 5 --json | jq '.results[].name'
```

**Download Models:**
```bash
# From HuggingFace
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF \
  --file llama-2-7b-chat.Q4_K_M.gguf

# List installed
offgrid list

# JSON output
offgrid list --json | jq '.models[].name'
```

**Import/Export:**
```bash
# Import from USB
offgrid import /media/usb

# Export to USB
offgrid export tinyllama-1.1b-chat /media/usb

# Remove model
offgrid remove model-name
```

### Interactive Chat with Sessions

```bash
# Start chat and save conversation
offgrid run Llama-3.2-3B-Instruct-Q4_K_M --save my-project

# Continue previous conversation
offgrid run Llama-3.2-3B-Instruct-Q4_K_M --load my-project

# List all sessions
offgrid session list

# Show session details
offgrid session show my-project

# Export session to markdown
offgrid session export my-project output.md
```

### Prompt Templates

**Built-in templates for common tasks:**

```bash
# List available templates
offgrid template list

# Apply template
offgrid template apply summarize
offgrid template apply code-review
```

**Available templates:** summarize, code-review, translate, explain, brainstorm, debug, document, refactor, test, cli

### Automation & Scripting

**JSON Output Mode** - Perfect for CI/CD, monitoring, and automation:

```bash
# List models in JSON
offgrid list --json | jq '.count'

# Search with JSON output
offgrid search llama --limit 5 --json | jq '.results[].name'

# Get system info
offgrid info --json | jq '{cpu: .system.cpu, models: .models.count}'

# Session management
offgrid session list --json | jq '.sessions[].name'
```

**Batch Processing:**
```bash
# Process multiple prompts in parallel
cat > batch_input.jsonl << 'EOF'
{"id": "1", "model": "model.gguf", "prompt": "What is AI?"}
{"id": "2", "model": "model.gguf", "prompt": "Explain ML"}
EOF

offgrid batch process batch_input.jsonl results.jsonl --concurrency 4
```

### API Usage

**OpenAI-Compatible API:**

```bash
# Chat completion
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Llama-3.2-3B-Instruct-Q4_K_M",
    "messages": [
      {"role": "user", "content": "Explain quantum computing"}
    ]
  }'

# Health check
curl http://localhost:11611/health

# Readiness probe (Kubernetes)
curl http://localhost:11611/ready

# List models
curl http://localhost:11611/v1/models
```

**Python Example:**
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:11611/v1",
    api_key="not-needed"  # Local inference, no auth required
)

response = client.chat.completions.create(
    model="Llama-3.2-3B-Instruct-Q4_K_M",
    messages=[
        {"role": "user", "content": "Explain edge computing"}
    ]
)

print(response.choices[0].message.content)
```

## 🏗️ Architecture

### System Design

```
┌─────────────────────────────────────────────────────────────┐
│  Client (Browser / API / CLI)                               │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP :11611
┌───────────────────▼─────────────────────────────────────────┐
│  OffGrid LLM Server (Go)                                    │
│  • OpenAI-compatible routing    • Model registry           │
│  • Session management           • Health monitoring        │
│  • Statistics & caching         • Web UI serving           │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP (internal, localhost-only)
┌───────────────────▼─────────────────────────────────────────┐
│  llama-server (llama.cpp C++)                               │
│  • GGUF model loading           • GPU inference            │
│  • Context management           • Token generation         │
└─────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
/usr/local/bin/
├── offgrid                    # Main CLI
└── llama-server               # Inference engine

/var/lib/offgrid/
└── models/                    # Model storage
    └── *.gguf

/etc/offgrid/
├── active-model               # Currently loaded model
└── llama-port                 # Internal port

~/.offgrid/
└── sessions/                  # Saved chat sessions

/etc/systemd/system/
├── offgrid-llm.service        # Main service
└── llama-server.service       # Inference service
```

## ⚙️ System Requirements

### Minimum Configuration
- **CPU**: 2 cores, 2GHz+
- **RAM**: 2GB (for small Q4 models)
- **Disk**: 2GB available
- **OS**: Linux (Ubuntu 20.04+, Debian 11+)

### Recommended Configuration
- **CPU**: 4+ cores
- **RAM**: 8GB+
- **GPU**: NVIDIA GPU with 6GB+ VRAM (optional)
- **Disk**: 10GB+ SSD

### GPU Support
- **NVIDIA**: GTX 1050 Ti or newer, CUDA 12.0+
- **AMD**: ROCm-compatible GPU (experimental)

### Model RAM Requirements

| Model Size | Quantization | RAM Needed | GPU VRAM |
|------------|--------------|------------|----------|
| 1B params  | Q4_K_M      | ~2GB       | ~2GB     |
| 3B params  | Q4_K_M      | ~4GB       | ~4GB     |
| 7B params  | Q4_K_M      | ~8GB       | ~6GB     |
| 13B params | Q4_K_M      | ~16GB      | ~12GB    |

> **Tip**: Use `offgrid quantization` to learn about different quantization levels

## 📊 Performance

**Expected throughput (tokens/second):**

| Hardware | 1B Model | 7B Model | 13B Model |
|----------|----------|----------|-----------|
| CPU (4 cores) | 20-30 | 5-10 | 2-5 |
| RTX 3060 (12GB) | 80-100 | 35-45 | 20-25 |
| RTX 4090 (24GB) | 120-150 | 60-80 | 40-50 |

*Performance varies with architecture, quantization, and context length*

## 🔒 Security

### Network Security
- ✅ Localhost-only binding (`127.0.0.1`)
- ✅ Random high ports for IPC
- ✅ No external network access
- ✅ Systemd security directives

### Process Isolation
- ✅ Dedicated `offgrid` system user
- ✅ Restricted file system access
- ✅ No root privileges for operation

### Data Privacy
- ✅ No telemetry or analytics
- ✅ No external API calls
- ✅ All inference runs locally
- ✅ Complete data sovereignty

## 📚 CLI Reference

### Model Commands
```bash
offgrid list [--json]                  # List installed models
offgrid search <query> [--json]        # Search HuggingFace
offgrid download-hf <repo> --file <f>  # Download from HuggingFace
offgrid import <path>                  # Import from USB/storage
offgrid export <model> <dest>          # Export to USB/storage
offgrid remove <model>                 # Remove model
```

### Chat & Sessions
```bash
offgrid run <model>                    # Interactive chat
offgrid run <model> --save <name>      # Save session
offgrid run <model> --load <name>      # Load session
offgrid session list [--json]          # List sessions
offgrid session show <name>            # View session
offgrid session export <name> <file>   # Export to markdown
offgrid session delete <name>          # Delete session
```

### Productivity
```bash
offgrid template list                  # List prompt templates
offgrid template apply <name>          # Use template
offgrid alias set <name> <model>       # Create alias
offgrid favorite add <model>           # Star model
offgrid batch process <in> <out>       # Batch processing
```

### System
```bash
offgrid info [--json]                  # System information
offgrid serve                          # Start HTTP server
offgrid completions <bash|zsh|fish>    # Generate completions
offgrid help                           # Show help
```

### Global Flags
```bash
--json                                 # JSON output for scripting
```

## 🔧 Configuration

### Environment Variables

```bash
OFFGRID_PORT=11611                     # HTTP server port
OFFGRID_MODELS_DIR=/var/lib/offgrid/models
OFFGRID_NUM_THREADS=4                  # CPU threads
OFFGRID_MAX_CONTEXT=4096               # Context window
```

### Config File

Create `~/.offgrid-llm/config.yaml`:

```yaml
server:
  port: 11611
  host: "127.0.0.1"

models:
  directory: "/var/lib/offgrid/models"
  
inference:
  num_threads: 4
  context_size: 4096
  gpu_layers: 0  # 0 = auto-detect

logging:
  level: "info"
```

Load with:
```bash
export OFFGRID_CONFIG=~/.offgrid-llm/config.yaml
offgrid serve
```

## 🔍 API Reference

### Core Endpoints

**Health & Monitoring:**
```bash
GET /health          # Full system diagnostics
GET /ready           # Kubernetes readiness probe
GET /livez           # Liveness probe
GET /readyz          # Readiness alias
GET /stats           # Per-model statistics
GET /cache/stats     # Cache metrics
```

**OpenAI-Compatible:**
```bash
POST /v1/chat/completions    # Chat completion
POST /v1/completions         # Text completion
GET  /v1/models              # List available models
```

### Health Response

```json
{
  "status": "healthy",
  "version": "0.1.0-alpha",
  "uptime": "running",
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

### Chat Completion

**Request:**
```json
{
  "model": "Llama-3.2-3B-Instruct-Q4_K_M",
  "messages": [
    {"role": "system", "content": "You are helpful."},
    {"role": "user", "content": "Hello"}
  ],
  "temperature": 0.7,
  "max_tokens": 500,
  "stream": false
}
```

**Response:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1731234567,
  "model": "Llama-3.2-3B-Instruct-Q4_K_M",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ]
}
```

## 🛠️ Troubleshooting

### Service Issues

```bash
# Check status
sudo systemctl status offgrid-llm llama-server

# View logs
sudo journalctl -u offgrid-llm -f
sudo journalctl -u llama-server -f

# Restart services
sudo systemctl restart llama-server
sudo systemctl restart offgrid-llm

# Test health
curl http://localhost:11611/health
```

### Model Issues

```bash
# List models
offgrid list

# Check permissions
sudo chown -R offgrid:offgrid /var/lib/offgrid/models/
sudo chmod 664 /var/lib/offgrid/models/*.gguf

# Verify model format
file /var/lib/offgrid/models/*.gguf

# Force reload
sudo systemctl restart llama-server
```

### GPU Issues

```bash
# Check GPU
nvidia-smi
nvcc --version

# Verify CUDA in llama-server
ldd /usr/local/bin/llama-server | grep cuda

# Rebuild with GPU support
sudo ./reinstall.sh

# Force CPU-only
sudo ./reinstall.sh --cpu-only
```

## 📦 Offline Deployment

### Air-Gapped Installation

**1. Prepare on connected machine:**
```bash
# Clone and package
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Download models
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# Create package
tar czf offgrid-offline.tar.gz offgrid-llm/ ~/.offgrid-llm/models/
```

**2. Transfer and install:**
```bash
# On air-gapped machine
tar xzf offgrid-offline.tar.gz
cd offgrid-llm
sudo ./install.sh --cpu-only  # Or with GPU

# Import models
sudo cp models/*.gguf /var/lib/offgrid/models/
sudo chown offgrid:offgrid /var/lib/offgrid/models/*.gguf
sudo systemctl restart llama-server
```

### USB Model Distribution

```bash
# Create portable package
./scripts/create-usb-package.sh /media/usb model-name

# Import on destination
offgrid import /media/usb
```

## 💻 Development

### Building from Source

```bash
# Prerequisites
sudo apt-get install build-essential cmake git golang-1.21

# Clone and build
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
go build -o offgrid ./cmd/offgrid

# Run development server
./offgrid serve
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/server -v
```

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

**Areas of interest:**
- AMD ROCm GPU support
- macOS/Windows support  
- Performance optimization
- Documentation improvements

## 📄 License

MIT License - See [LICENSE](LICENSE)

## 🙏 Acknowledgments

- [llama.cpp](https://github.com/ggerganov/llama.cpp) - High-performance inference engine
- [HuggingFace](https://huggingface.co) - Model distribution and community
- GGUF format contributors

## 📖 Documentation

- [Architecture](docs/ARCHITECTURE.md) - System design and components
- [API Documentation](docs/API.md) - Complete API reference
- [JSON Output Mode](docs/JSON_OUTPUT.md) - Automation and scripting
- [Model Setup](docs/MODEL_SETUP.md) - Model installation guide
- [CLI Reference](docs/CLI_REFERENCE.md) - Complete command reference

## 🔗 Links

- **Repository**: [github.com/takuphilchan/offgrid-llm](https://github.com/takuphilchan/offgrid-llm)
- **Issues**: [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)
- **Discussions**: [GitHub Discussions](https://github.com/takuphilchan/offgrid-llm/discussions)

---

**Built for offline-first deployment · Zero external dependencies · Complete data sovereignty**
