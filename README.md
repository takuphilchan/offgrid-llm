# OffGrid LLM

> **Edge Inference Orchestrator · Offline-First AI**

A self-contained LLM inference system designed for environments with limited or no internet connectivity. Built with Go and powered by [llama.cpp](https://github.com/ggerganov/llama.cpp), providing OpenAI-compatible API with GPU acceleration support.

Suitable for edge devices, air-gapped networks, bandwidth-constrained environments, and scenarios requiring data sovereignty.

## Key Features

### HuggingFace Hub Integration

Direct access to GGUF models from HuggingFace Hub:

```bash
# Search for models with filters
$ offgrid search llama --quant Q4_K_M --sort downloads

# Download directly from HuggingFace
$ offgrid download-hf <author>/<repo> --file <model>.gguf

# Interactive chat
$ offgrid run <model>.gguf
```

Access to thousands of community-published GGUF models without intermediary registries.

### Offline-First Architecture

- **Offline Operation** - Full functionality without internet connectivity after initial setup
- **GPU Acceleration** - NVIDIA CUDA and AMD ROCm support
- **CPU-Only Mode** - Optimized fallback for systems without GPU
- **Resource Efficient** - Operates on systems with 2GB+ RAM
- **P2P Model Sharing** - Local network model distribution
- **Portable Storage** - USB/SD card model import/export

### Developer Experience

- **OpenAI Compatible API** - Standard API endpoints for integration
- **CLI Interface** - Command-line tools for model and system management
- **Web Dashboard** - Browser-based UI for monitoring and interaction
- **Streaming Support** - Real-time token streaming via Server-Sent Events
- **Benchmark Tools** - Performance testing for hardware validation

### Production Features

- **Systemd Integration** - Service management for Linux systems
- **Resource Monitoring** - CPU, memory, and GPU utilization tracking
- **Usage Statistics** - Inference metrics and performance analytics
- **Security Hardening** - Localhost-only operation, no external telemetry
- **Health Checks** - System readiness and diagnostic endpoints

## Comparison

| Feature | OffGrid LLM | Ollama | Cloud APIs |
|---------|-------------|---------|------------|
| **Offline Operation** | Full | Partial | No |
| **HuggingFace Search** | Direct | Curated | N/A |
| **Model Discovery** | 10k+ models | ~100 models | Varies |
| **USB Import/Export** | Yes | No | No |
| **P2P Sharing** | Yes | No | No |
| **OpenAI Compatible** | Yes | Yes | Yes |
| **GPU Acceleration** | CUDA/ROCm | CUDA | Cloud GPUs |
| **Self-Hosted** | Yes | Yes | No |
| **Cost** | Free | Free | Pay-per-use |

## Quick Start

### System Requirements

**Minimum:**
- CPU: 2 cores
- RAM: 2GB (Q4 quantized models)
- Disk: 2GB available
- OS: Linux (Ubuntu 20.04+, Debian 11+, or compatible)

**Recommended:**
- CPU: 4+ cores
- RAM: 8GB+
- GPU: NVIDIA GPU with CUDA 12.0+ (optional, for acceleration)
- Disk: 10GB+ available

### Installation

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Auto-detect GPU and install
sudo ./install.sh

# CPU-only mode (skip GPU detection)
sudo ./install.sh --cpu-only

# Require GPU mode (fail if no GPU detected)
sudo ./install.sh --gpu
```

**Installation performs:**
- GPU detection and acceleration setup (NVIDIA/AMD)
- llama.cpp compilation with optimal configuration
- Binary compilation and installation
- Systemd service configuration
- Security hardening (localhost-only binding, restricted ports)
- Model directory creation at `/var/lib/offgrid/models`

**Quick reinstall:**

```bash
sudo ./reinstall.sh              # Auto-detect GPU
sudo ./reinstall.sh --cpu-only   # Force CPU-only
sudo ./reinstall.sh --gpu        # Require GPU
```

### Post-Installation

Services start automatically after installation:

```bash
# Service status
sudo systemctl status offgrid-llm llama-server

# View logs
sudo journalctl -u offgrid-llm -f

# Access web UI
http://localhost:11611/ui
```

### Example: TinyLlama Model

```bash
# Download model (638MB, 2GB RAM requirement)
wget https://huggingface.co/<author>/<repo>/resolve/main/<model>.gguf

# Install to model directory
sudo mv <model>.gguf /var/lib/offgrid/models/
sudo chown offgrid:offgrid /var/lib/offgrid/models/*.gguf

# Restart service to load model
sudo systemctl restart llama-server

# Verify
curl http://localhost:11611/health
```

**Alternative:** Use web UI at `http://localhost:11611/ui` for model management.

### API Testing

```bash
# Health check
curl http://localhost:11611/health

# List models
curl http://localhost:11611/v1/models

# Chat completion
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<model-name>",
    "messages": [{"role": "user", "content": "Explain inference optimization"}],
    "stream": false
  }'

# Streaming chat
curl -N http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<model-name>",
    "messages": [{"role": "user", "content": "Describe quantization techniques"}],
    "stream": true
  }'
```

## Architecture

Two-process architecture for security and stability:

```
┌─────────────────────────────────────────────────────────────┐
│  Client (Browser / API)                                     │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP :11611 (localhost)
┌───────────────────▼─────────────────────────────────────────┐
│  OffGrid LLM (Go)                                           │
│  - Request routing and validation                           │
│  - Model management                                         │
│  - Statistics and monitoring                                │
│  - Web UI serving                                           │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP (random port 49152-65535, localhost)
┌───────────────────▼─────────────────────────────────────────┐
│  llama-server (C++)                                         │
│  - llama.cpp inference engine                               │
│  - GPU acceleration (CUDA/ROCm)                             │
│  - Model loading and caching                                │
│  - Token generation                                         │
└─────────────────────────────────────────────────────────────┘
```

**Security:**
- Localhost-only binding (127.0.0.1)
- Random high port allocation for inter-process communication
- Systemd security directives (IPAddressDeny/IPAddressAllow)
- Dedicated non-privileged user (`offgrid`)
- Restricted file system access

## Features

### HuggingFace Hub Integration

```bash
# Search models
offgrid search <query> --sort downloads

# Download from HuggingFace
offgrid download-hf <author>/<repo> --quant Q4_K_M

# Interactive terminal chat
offgrid run <model>.gguf

# Benchmark performance
curl -X POST http://localhost:11611/v1/benchmark -d '{"model":"<model>.gguf"}'
```

Direct integration with HuggingFace Hub for model discovery and download.

📖 **[HuggingFace Integration Documentation](docs/HUGGINGFACE_INTEGRATION.md)**

### Core Capabilities

**Inference Engine**
- llama.cpp C++ backend
- GPU acceleration: CUDA (NVIDIA), ROCm (AMD)
- Automatic GPU detection and configuration
- CPU fallback mode
- Edge hardware optimization

**Offline Operation**
- No internet dependency after initial setup
- Local inference execution
- No external data transmission
- USB/SD card model distribution
- Pre-configured offline deployments

**Model Management**
- File-based model loading
- GGUF format support (llama.cpp)
- Automatic model detection in `/var/lib/offgrid/models`
- Hot-reload via service restart
- SHA256 integrity verification

**API Compatibility**
- `/v1/chat/completions` - Conversation interface
- `/v1/completions` - Text completion
- `/v1/models` - Model enumeration
- `/v1/search` - HuggingFace Hub search
- `/v1/benchmark` - Performance testing
- `/health` - System diagnostics
- Server-Sent Events streaming
- OpenAI client library compatibility

**Web Dashboard**
- Browser-based interface
- Interactive chat with streaming
- System monitoring (CPU, RAM, GPU)
- Model management
- API testing tools
- Offline-capable (no CDN dependencies)

**GPU Acceleration**
- NVIDIA GPU detection (nvidia-smi)
- CUDA 12.x support with auto-detection
- GPU layer offloading configuration
- CPU fallback on GPU unavailability
- Real-time utilization monitoring

**Production Features**
- Systemd service management
- Automatic startup on boot
- Graceful shutdown handling
- Process monitoring and restart
- Structured logging (journald)
- Health check endpoints

### Technical Details

**Quantization Support**
- Q2_K through Q8_0 quantization levels
- Automatic detection from model filename
- Documented quality/size tradeoffs
- RAM-based recommendations

**Security**
- Localhost-only binding (127.0.0.1)
- Random high ports (49152-65535) for inter-process communication
- Systemd security directives
- Non-privileged dedicated user
- No external network dependencies

**Installation Resilience**
- Multi-method GPU detection (nvidia-smi, lspci, /proc/driver/nvidia)
- CUDA toolkit multi-location search
- Fallback download mechanisms
- Retry logic for network operations
- Diagnostic error reporting

**Integrity Verification**
- SHA256 checksums for catalog models
- Automatic verification during download
- Tamper detection

## Commands

### CLI Reference

```bash
# Server
offgrid serve                    # Start HTTP server

# Model Discovery
offgrid catalog                  # Browse available models
offgrid search <query>           # Search HuggingFace Hub
offgrid list                     # List installed models
offgrid info                     # System information

# Model Management
offgrid download <id> [quant]    # Download from internet
offgrid download-hf <repo>       # Download from HuggingFace
offgrid import <path>            # Import from storage
offgrid export <id> <path>       # Export to storage
offgrid remove <id>              # Remove model
offgrid run <model>              # Interactive chat

# Configuration
offgrid config init              # Generate config file
offgrid config show              # Display configuration
offgrid config validate <path>   # Validate config

# Utilities
offgrid quantization             # Quantization information
offgrid help                     # Help documentation
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
  port: 11611
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

Example verified models with SHA256 checksums:

| Model | Size | RAM | Quantization | Use Case |
|-------|------|-----|--------------|----------|
| TinyLlama 1.1B | 638MB | 2GB | Q4_K_M | Resource-constrained systems |
| TinyLlama 1.1B | 768MB | 2GB | Q5_K_M | Better quality, same model |
| Llama 2 7B | 3.8GB | 8GB | Q4_K_M | General purpose |
| Llama 2 7B | 4.5GB | 8GB | Q5_K_M | Higher quality |
| Mistral 7B | 4.1GB | 8GB | Q4_K_M | Instruction following |
| Phi-2 | 1.7GB | 4GB | Q4_K_M | Efficient reasoning |

**Quantization Levels:**
- **Q4_K_M** - Balanced quality/size (recommended)
- **Q5_K_M** - Higher quality, +25% size
- **Q3_K_M** - Maximum compression
- **Q8_0** - Near-lossless quality

See `offgrid quantization` for detailed information.

## Offline Distribution

### USB Package Creation

```bash
# Create offline package for distribution
./scripts/create-usb-package.sh /media/usb <model-id>

# Package contains:
# - Compiled binaries (Linux/Windows/macOS)
# - Selected model with SHA256 verification
# - Documentation
# - Installation scripts
```

### Storage Import/Export

```bash
# Import from external storage
offgrid import /media/usb
offgrid import /media/usb/<model>.gguf

# Export to external storage
offgrid export <model-id> /media/usb

# Verify installation
offgrid list
```

Models are verified with SHA256 checksums during import.

## Use Cases

**Bandwidth-Constrained Environments**
- Limited or expensive internet connectivity
- Offline-first operations
- Data sovereignty requirements

**Air-Gapped Networks**
- High-security facilities
- Classified environments
- Compliance-driven deployments

**Edge Computing**
- Remote operations
- Industrial automation
- Field deployments

**Resource-Constrained Systems**
- Low-power devices
- Embedded systems
- Cost-sensitive infrastructure

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
- **Web UI** - Cyberpunk-themed dashboard with black/cyan design, real-time chat, streaming support

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

### Project Structure

```
offgrid-llm/
├── cmd/offgrid/          # Main application entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── inference/        # Inference engines (HTTP proxy to llama-server)
│   ├── models/           # Model management and registry
│   ├── p2p/             # P2P discovery and transfer
│   ├── resource/        # System resource monitoring
│   └── server/          # HTTP server and API handlers
├── pkg/api/             # Public API types
├── web/ui/              # Web dashboard (vanilla JS, no build step)
├── scripts/             # Installation and utility scripts
├── docs/                # Documentation
├── install.sh           # Main installation script
└── reinstall.sh         # Quick cleanup and reinstall
```

### Building from Source

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install dependencies (requires Go 1.21+)
go mod download

# Build Go binary
make build

# Run locally (for development)
./offgrid serve
```

### Full Installation (with llama.cpp)

```bash
# Run full installer - builds everything including llama.cpp
sudo ./install.sh

# This will:
# - Detect and configure GPU support
# - Download and build llama.cpp with CUDA
# - Build OffGrid LLM
# - Set up systemd services
# - Configure security
```

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/server -v

# With coverage
go test -cover ./...

# Server tests with verbose output
go test -v ./internal/server/...
```

## Troubleshooting

### Services Won't Start

```bash
# Check service status
sudo systemctl status offgrid-llm
sudo systemctl status llama-server

# View detailed logs
sudo journalctl -u offgrid-llm -n 50 --no-pager
sudo journalctl -u llama-server -n 50 --no-pager

# Check if binary exists
ls -la /usr/local/bin/llama-server
ls -la /usr/local/bin/offgrid

# Verify dependencies
ldd /usr/local/bin/llama-server
```

### GPU Not Detected

```bash
# Check GPU detection
nvidia-smi  # Should show your GPU

# Check CUDA installation
nvcc --version  # Should show CUDA version

# Verify llama-server was built with CUDA
ldd /usr/local/bin/llama-server | grep cuda

# Rebuild with GPU support
sudo ./reinstall.sh
```

### Connection Refused Errors

```bash
# Check if services are running
sudo systemctl status offgrid-llm llama-server

# Check port bindings
sudo netstat -tulpn | grep -E "(11611|offgrid|llama)"

# Check llama-server port file
cat /etc/offgrid/llama-port

# Test llama-server directly
curl http://localhost:$(cat /etc/offgrid/llama-port)/health
```

### Model Not Loading

```bash
# Check model directory
ls -la /var/lib/offgrid/models/

# Check permissions
sudo chown -R offgrid:offgrid /var/lib/offgrid/models/

# Verify model format (should be .gguf)
file /var/lib/offgrid/models/*.gguf

# Restart services
sudo systemctl restart llama-server
sudo systemctl restart offgrid-llm
```

### Build Failures

```bash
# Check build logs
tail -50 /tmp/llama_build*.log

# Verify CUDA toolkit
which nvcc
nvcc --version

# Check for missing dependencies
sudo apt-get install build-essential cmake git

# Clean and rebuild
sudo rm -rf /root/llama.cpp
sudo ./reinstall.sh
```

### Symbol Lookup Errors

```bash
# If you see errors like:
# "symbol lookup error: undefined symbol: llama_state_seq_get_size_ext"

# This means shared libraries aren't installed. Reinstall:
sudo ./reinstall.sh

# The installer will now properly install shared libraries to /usr/local/lib
# and update the library cache with ldconfig

# Verify shared libraries are installed:
ls -la /usr/local/lib/libllama.so*
ls -la /usr/local/lib/libggml*.so*

# Check library dependencies:
ldd /usr/local/bin/llama-server
```

### Quick Fixes

```bash
# Reinstall with GPU support (default)
sudo ./reinstall.sh

# Reinstall with CPU-only mode
sudo ./reinstall.sh --cpu-only

# Force GPU mode (fails if no GPU detected)
sudo ./reinstall.sh --gpu
```

## Environment Variables

```bash
# Systemd services read from /etc/offgrid/config
# For manual runs:

OFFGRID_PORT=11611                       # Server port (default)
OFFGRID_HOST=127.0.0.1                   # Server host (localhost only)
```

## Performance

**Resource Requirements:**

| Model | Quantization | RAM | Disk | CPU Speed | GPU Speed |
|-------|--------------|-----|------|-----------|-----------|
| TinyLlama 1.1B | Q4_K_M | 2GB | 638MB | 20-30 tok/s | 60-100 tok/s |
| Phi-2 2.7B | Q4_K_M | 4GB | 1.7GB | 10-15 tok/s | 40-60 tok/s |
| Llama 2 7B | Q4_K_M | 8GB | 3.8GB | 5-10 tok/s | 30-50 tok/s |
| Mistral 7B | Q4_K_M | 8GB | 4.1GB | 5-10 tok/s | 30-50 tok/s |

**GPU Acceleration:**
- NVIDIA GTX 1050 Ti and newer
- CUDA 12.0+ required
- Typical 3-5x speedup over CPU
- Automatic layer offloading

**Optimization:**
- Q4_K_M recommended for balanced performance
- GPU acceleration when available
- Allocate 2x model size in RAM
- SSD storage for faster model loading

## Roadmap

### Completed
- HTTP server with OpenAI-compatible API
- llama.cpp integration with GPU support
- CUDA detection and configuration
- Systemd service management
- Security hardening
- Model loading from filesystem
- Streaming support (SSE)
- Web dashboard
- Health monitoring
- Installation resilience
- HuggingFace Hub integration

### In Progress
- Model download via web UI
- P2P model transfer
- USB model import/export
- Quantization tools

### Planned
- Multi-model concurrent support
- Conversation persistence
- Model fine-tuning
- AMD ROCm support
- Container images
- Cross-platform installers

## Technology Stack

**Backend:**
- Go 1.21+ - HTTP server, API routing, system management
- llama.cpp - C++ inference engine with CUDA/ROCm
- Systemd - Service management

**Frontend:**
- Vanilla JavaScript - No build dependencies
- Server-Sent Events - Real-time streaming
- CSS3 - Responsive interface

**Infrastructure:**
- Linux (Ubuntu 20.04+, Debian 11+)
- NVIDIA CUDA 12.0+ (optional)
- CMake - Build system

**Security:**
- Localhost binding (127.0.0.1)
- Random port allocation
- Systemd security directives
- User isolation

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Development areas:**
- AMD ROCm GPU support
- Cross-platform installers
- Model management improvements
- Performance optimization
- Documentation

**Development setup:**
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
git checkout -b feature/<name>
go test ./...
```

## License

MIT License - see [LICENSE](LICENSE) for details

## Acknowledgments

- [llama.cpp](https://github.com/ggerganov/llama.cpp) - Inference engine
- [HuggingFace](https://huggingface.co) - Model distribution platform
- GGUF model community contributors

## Support

- **Documentation**: [docs/](docs/)
- **Issues**: [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)
- **Discussions**: [GitHub Discussions](https://github.com/takuphilchan/offgrid-llm/discussions)

---

**Designed for offline-first deployment.**
