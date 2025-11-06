# OffGrid LLM

> **Offline-First AI for Edge Environments**

OffGrid LLM is a production-ready, self-contained LLM orchestrator designed for environments with limited or no internet connectivity. Deploy powerful language models on edge devices, ships, remote clinics, factories, and air-gapped networks.

## Why OffGrid LLM?

Traditional LLM deployments require constant internet connectivity, cloud APIs, and significant bandwidth. OffGrid LLM eliminates these dependencies:

- **True Offline Operation** - Works completely disconnected from the internet
- **Resource Efficient** - Runs on devices with as little as 2GB RAM  
- **P2P Model Sharing** - Share models across local networks without internet
- **USB Distribution** - Install models from USB drives and SD cards
- **OpenAI Compatible** - Drop-in replacement for OpenAI API
- **Single Binary** - No complex dependencies or runtime requirements

## Quick Start

### Installation

**Recommended: One-Line Install**

```bash
# Clone and install system-wide
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
./install.sh

# Or install for current user only (no sudo)
./install.sh --user
```

**Manual Installation Options:**

<details>
<summary>Option 1: System-wide installation (Recommended)</summary>

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install system-wide (requires sudo)
make install-system

# Now use 'offgrid' from anywhere!
offgrid --version
```
</details>

<details>
<summary>Option 2: User installation (no sudo required)</summary>

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install to user's Go bin
make install

# Add Go bin to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH="$PATH:$(go env GOPATH)/bin"

# Reload shell
source ~/.bashrc

# Now use 'offgrid' from anywhere!
offgrid --version
```
</details>

<details>
<summary>Option 3: Build locally</summary>

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Build
make build

# Run from directory
./offgrid
```
</details>

Server starts on `http://localhost:8080`

### Your First Model

```bash
# Browse available models
offgrid catalog

# Learn about quantization levels
offgrid quantization

# Download TinyLlama (638MB, 2GB RAM minimum)
offgrid download tinyllama-1.1b-chat Q4_K_M

# Start server
offgrid serve
```

### Test the API

```bash
# Check health
curl http://localhost:8080/health

# List models
curl http://localhost:8080/v1/models

# Chat completion
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat",
    "messages": [{"role": "user", "content": "Explain quantum computing"}],
    "stream": false
  }'

# Streaming chat
curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat",
    "messages": [{"role": "user", "content": "Write a haiku about AI"}],
    "stream": true
  }'
```

## Features

### Core Capabilities

**Offline-First Design**
- Fully functional without internet connectivity
- All inference happens locally on your hardware
- No data sent to external servers

**Model Management**
- Download models from HuggingFace with SHA256 verification
- Import models from USB drives, SD cards, or network shares
- Support for GGUF format (llama.cpp compatible)
- Automatic quantization detection

**OpenAI-Compatible API**
- `/v1/chat/completions` - Chat interface with history
- `/v1/completions` - Direct text completion
- `/v1/models` - List available models
- Server-Sent Events (SSE) streaming support

**Web Dashboard**
- Modern, professional white-themed interface
- Interactive chat with streaming responses
- Model management and system monitoring
- API testing tools built-in
- No external dependencies - works completely offline

**Resource Monitoring**
- Real-time CPU, RAM, and disk usage tracking
- Pre-load validation to prevent OOM crashes
- Automatic quantization recommendations based on available RAM

**P2P Discovery**
- Automatic peer discovery on local networks
- JSON-based announcement protocol
- Share model availability across nodes
- Enable collaborative offline deployments

**Configuration Management**
- YAML/JSON configuration files
- Persistent settings for all options
- Environment variable overrides
- Generate sample configs with `offgrid config init`

### Advanced Features

**Quantization Guide**
- Comprehensive explanation of Q2_K through Q8_0 quantization
- Quality vs size tradeoffs for each level
- Smart recommendations based on your system
- Run `offgrid quantization` for details

**SHA256 Verification**
- All catalog models include verified SHA256 hashes
- Automatic integrity checking during download
- Prevent corrupted or tampered models

**Build Modes**
- `make build` - Mock mode for development/testing
- `make build-llama` - Real inference with llama.cpp (requires CGO)

## Commands

### CLI Reference

```bash
offgrid                          # Start server (default)
offgrid serve                    # Start server explicitly
offgrid catalog                  # Browse available models
offgrid quantization             # Learn about quantization levels
offgrid download <id> [quant]    # Download model from internet
offgrid import <path>            # Import from USB/SD card
offgrid list                     # List installed models
offgrid config init              # Generate configuration file
offgrid info                     # Show system information
offgrid help                     # Display help
```

### Configuration

Create a configuration file for persistent settings:

```bash
# Generate default config
offgrid config init

# Edit ~/.offgrid/config.yaml
vim ~/.offgrid/config.yaml

# Or use environment variable
export OFFGRID_CONFIG=/path/to/config.yaml
offgrid serve
```

Example configuration:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

models:
  directory: "./models"
  auto_load: true

inference:
  num_threads: 4
  context_size: 4096

p2p:
  enabled: true
  discovery_port: 8081
```

## Model Catalog

OffGrid LLM includes a curated catalog of verified models:

| Model | Size | RAM | Quantization | Use Case |
|-------|------|-----|--------------|----------|
| TinyLlama 1.1B | 638MB | 2GB | Q4_K_M | Low-resource environments |
| TinyLlama 1.1B | 768MB | 2GB | Q5_K_M | Better quality, same model |
| Llama 2 7B Chat | 3.8GB | 8GB | Q4_K_M | General purpose, balanced |
| Llama 2 7B Chat | 4.5GB | 8GB | Q5_K_M | Higher quality responses |
| Mistral 7B Instruct | 4.1GB | 8GB | Q4_K_M | Code, instruction following |
| Phi-2 | 1.7GB | 4GB | Q4_K_M | Efficient reasoning |

**Recommended Quantization Levels:**
- **Q4_K_M** - Best balance for most users (recommended)
- **Q5_K_M** - Higher quality, +25% size (production)
- **Q3_K_M** - Severe resource constraints (3-4GB RAM)
- **Q8_0** - Research/benchmarking (nearly lossless)

Run `offgrid quantization` for detailed explanations.

## Offline Distribution

### USB Package Creation

Distribute OffGrid LLM to offline environments via USB:

```bash
# Create complete offline package
./scripts/create-usb-package.sh /media/usb tinyllama-1.1b-chat

# Package includes:
# - Binary for Linux, Windows, macOS
# - Selected model with verified hash
# - Documentation
# - Installation scripts
```

### USB Model Import

Import models from external storage:

```bash
# Import all models from USB
offgrid import /media/usb

# Import specific model
offgrid import /media/usb/llama-2-7b-chat.Q4_K_M.gguf

# Windows
offgrid import D:\models

# Verify
offgrid list
```

Models are automatically verified with SHA256 checksums during import.

## Use Cases

**Maritime & Offshore**
- Ships, oil rigs, research vessels
- No reliance on satellite internet
- AI assistance for navigation, documentation, training

**Healthcare**
- Rural clinics, mobile medical units
- Medical reference and triage assistance
- Privacy-compliant patient data processing

**Education**
- Schools in low-bandwidth regions
- Offline tutoring and learning assistance
- No dependency on cloud services

**Industrial & Manufacturing**
- Factories, mines, warehouses
- Equipment documentation and troubleshooting
- Quality control and inspection assistance

**High-Security Environments**
- Air-gapped networks
- Government and defense facilities
- Complete data sovereignty

**Field Research**
- Remote scientific operations
- Environmental monitoring stations
- Field data analysis without connectivity

## Architecture

```
offgrid-llm/
├── cmd/offgrid/              # CLI application entry point
├── internal/
│   ├── config/              # Configuration management (YAML/JSON)
│   ├── server/              # HTTP server & API handlers
│   ├── models/              # Model registry, download, import
│   ├── inference/           # LLM inference engine (mock + llama.cpp)
│   ├── resource/            # CPU/RAM/disk monitoring with gopsutil
│   └── p2p/                 # Peer discovery and model sharing
├── pkg/api/                 # OpenAI-compatible API types
├── web/ui/                  # Web dashboard (pure HTML/CSS/JS)
├── docs/                    # Documentation
└── scripts/                 # Utilities and examples
```

**Key Components:**

- **Inference Engine** - Pluggable backend (mock for testing, llama.cpp for production)
- **Model Registry** - Track installed models, metadata, and availability
- **Resource Monitor** - Real-time system metrics with gopsutil
- **P2P Discovery** - JSON-based UDP announcements for local network discovery
- **API Server** - OpenAI-compatible HTTP endpoints with SSE streaming
- **Web UI** - Offline-capable dashboard with no external dependencies

## API Reference

### Endpoints

**GET /health**
```json
{
  "status": "ok",
  "timestamp": "2025-11-06T..."
}
```

**GET /v1/models**
```json
{
  "object": "list",
  "data": [
    {
      "id": "tinyllama-1.1b-chat",
      "object": "model",
      "owned_by": "local"
    }
  ]
}
```

**POST /v1/chat/completions**
```json
{
  "model": "tinyllama-1.1b-chat",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 500,
  "stream": false
}
```

**POST /v1/completions**
```json
{
  "model": "tinyllama-1.1b-chat",
  "prompt": "Once upon a time",
  "max_tokens": 100,
  "temperature": 0.7
}
```

Set `"stream": true` to enable Server-Sent Events streaming.

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install dependencies
go mod download

# Build (mock mode)
make build

# Build with real llama.cpp inference (requires CGO)
make build-llama

# Run tests
make test

# Development mode (auto-reload)
make dev
```

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/models -v

# With coverage
go test -cover ./...
```

### Building for Production

For real LLM inference, build with llama.cpp support:

```bash
# Install dependencies (see docs/LLAMA_CPP_SETUP.md)
# Requires: CGO, C compiler, llama.cpp

# Build
make build-llama

# Result: offgrid binary with full inference capabilities
```

## Environment Variables

```bash
OFFGRID_CONFIG=/path/to/config.yaml    # Configuration file location
OFFGRID_PORT=8080                       # Server port
OFFGRID_HOST=0.0.0.0                    # Server host
OFFGRID_MODELS_DIR=./models             # Models directory
OFFGRID_NUM_THREADS=4                   # Inference threads
OFFGRID_CONTEXT_SIZE=4096               # Model context window
OFFGRID_P2P_ENABLED=true                # Enable P2P discovery
OFFGRID_P2P_PORT=8081                   # P2P discovery port
```

## Performance

**Resource Requirements (Minimum):**
- **TinyLlama 1.1B**: 2GB RAM, 1GB disk
- **Phi-2 2.7B**: 4GB RAM, 2GB disk  
- **Llama 2 7B**: 8GB RAM, 4GB disk
- **Mistral 7B**: 8GB RAM, 5GB disk

**Inference Speed** (approximate, CPU-only):
- TinyLlama: 20-30 tokens/sec on modern CPU
- Llama 2 7B Q4: 5-10 tokens/sec on modern CPU
- GPU acceleration available with llama.cpp build

## Roadmap

### Completed ✅
- [x] HTTP server with OpenAI-compatible API
- [x] Model registry and catalog
- [x] Model download with SHA256 verification
- [x] USB/SD card model import
- [x] Streaming support (Server-Sent Events)
- [x] Web dashboard with chat interface
- [x] P2P discovery protocol
- [x] Resource monitoring (CPU, RAM, disk)
- [x] Configuration management (YAML/JSON)
- [x] Quantization education system
- [x] llama.cpp integration framework

### In Progress 🚧
- [ ] llama.cpp CGO build setup and documentation
- [ ] P2P model transfer implementation
- [ ] Multi-user authentication

### Planned 📋
- [ ] Mobile/ARM optimization
- [ ] Docker containerization
- [ ] Model compression tools
- [ ] Bandwidth-aware model syncing
- [ ] Automatic model updates from USB
- [ ] Advanced quantization options
- [ ] Plugin system for custom inference engines

## Contributing

Contributions are welcome! This project aims to make AI accessible in underserved and offline environments.

**Areas for Contribution:**
- llama.cpp CGO build automation
- Additional model formats (ONNX, TensorFlow Lite)
- Mobile platform support (Android, iOS)
- Docker/Kubernetes deployment
- Documentation and tutorials
- Testing on edge hardware

## License

MIT License - See LICENSE file for details.

## Philosophy

**AI should work everywhere, not just where the internet is fast.**

OffGrid LLM democratizes access to large language models by eliminating the dependency on cloud infrastructure and constant connectivity. Whether you're on a ship in the Arctic, a clinic in rural Africa, or a factory floor with strict air-gap policies, you deserve access to modern AI capabilities.

## Acknowledgments

- Built with [llama.cpp](https://github.com/ggerganov/llama.cpp) for efficient inference
- Models from [TheBloke](https://huggingface.co/TheBloke) on HuggingFace
- Inspired by the needs of edge and offline environments worldwide

---

**Status:** Production-ready for offline deployment | Active development | MIT Licensed
