# OffGrid LLM

**Edge Inference Orchestrator · Offline-First AI**

A self-contained LLM inference system for environments with limited connectivity. Built with Go and powered by [llama.cpp](https://github.com/ggerganov/llama.cpp), providing an OpenAI-compatible API with GPU acceleration.

## Features

- **Offline-First** - Complete functionality without internet after setup
- **GPU Acceleration** - NVIDIA CUDA and AMD ROCm support with CPU fallback
- **OpenAI Compatible** - Standard `/v1/chat/completions` and `/v1/completions` endpoints
- **HuggingFace Integration** - Direct model search and download from HuggingFace Hub
- **Model Management** - Automatic detection, hot-reload, integrity verification
- **Production Ready** - Systemd services, health checks, monitoring, security hardening
- **Portable** - USB/SD card model import/export for air-gapped deployments
- **Web UI** - Browser-based dashboard with real-time streaming chat

## Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install (auto-detects GPU)
sudo ./install.sh

# Or force CPU-only mode
sudo ./install.sh --cpu-only
```

The installer handles:
- GPU detection (NVIDIA/AMD) and driver configuration
- llama.cpp compilation with optimal settings
- Binary installation and systemd service setup
- Security hardening (localhost-only, restricted permissions)
- Model directory creation at `/var/lib/offgrid/models`

### Basic Usage

```bash
# Check installation
offgrid list                     # Show installed models
curl http://localhost:11611/health

# Access web UI
firefox http://localhost:11611/ui

# Service management
sudo systemctl status offgrid-llm llama-server
sudo journalctl -u offgrid-llm -f
```

## Usage

### Model Management

**Download from HuggingFace:**
```bash
# Search models
offgrid search llama --quant Q4_K_M --sort downloads

# Download specific model
offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF \
  --file llama-2-7b-chat.Q4_K_M.gguf

# Verify installation
offgrid list
```

**Import from USB/Storage:**
```bash
# Import all models from USB
offgrid import /media/usb

# Import specific file
offgrid import /media/usb/model.gguf

# Export for distribution
offgrid export tinyllama-1.1b-chat /media/usb
```

**Interactive Chat:**
```bash
# Start chat with model
offgrid run tinyllama-1.1b-chat.Q4_K_M

# Models auto-switch and load
# Chat interface supports:
# - Multi-turn conversations
# - Streaming responses
# - 'exit' to quit, 'clear' to reset
```

### API Usage

**Chat Completion:**
```bash
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat.Q4_K_M",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantization"}
    ],
    "max_tokens": 500,
    "temperature": 0.7
  }'
```

**Streaming:**
```bash
curl -N http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat.Q4_K_M",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

**List Models:**
```bash
curl http://localhost:11611/v1/models
```

**Python Example:**
```python
import requests

response = requests.post(
    'http://localhost:11611/v1/chat/completions',
    json={
        'model': 'tinyllama-1.1b-chat.Q4_K_M',
        'messages': [
            {'role': 'user', 'content': 'Explain edge computing'}
        ],
        'max_tokens': 200
    }
)

print(response.json()['choices'][0]['message']['content'])
```

## Architecture

### System Design

```
┌─────────────────────────────────────────────────────────────┐
│  Client (Browser / API / CLI)                               │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP :11611 (localhost)
┌───────────────────▼─────────────────────────────────────────┐
│  OffGrid LLM Server (Go)                                    │
│  - OpenAI-compatible API routing                            │
│  - Model registry and management                            │
│  - Statistics tracking                                      │
│  - Web UI serving                                           │
│  - Health monitoring                                        │
└───────────────────┬─────────────────────────────────────────┘
                    │ HTTP (internal port, localhost)
┌───────────────────▼─────────────────────────────────────────┐
│  llama-server (llama.cpp C++)                               │
│  - GGUF model loading                                       │
│  - GPU-accelerated inference (CUDA/ROCm)                    │
│  - Context management                                       │
│  - Token generation                                         │
└─────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
/usr/local/bin/
├── offgrid                    # Main CLI binary
├── llama-server               # llama.cpp inference server
└── llama-server-start.sh      # Dynamic model loader script

/var/lib/offgrid/
└── models/                    # Model storage (system-wide)
    ├── tinyllama-1.1b-chat.Q4_K_M.gguf
    └── llama-2-7b-chat.Q4_K_M.gguf

/etc/offgrid/
├── active-model               # Currently loaded model
└── llama-port                 # Internal communication port

/etc/systemd/system/
├── offgrid-llm.service        # Main service
└── llama-server.service       # Inference service
```

### Code Organization

```
offgrid-llm/
├── cmd/offgrid/              # CLI entry point
├── internal/
│   ├── config/              # Configuration (YAML/JSON/ENV)
│   ├── inference/           # llama.cpp HTTP proxy
│   ├── models/              # Registry, download, import/export
│   ├── server/              # HTTP API handlers
│   ├── resource/            # System monitoring
│   └── p2p/                 # Local network discovery
├── pkg/api/                 # Public API types
├── web/ui/                  # Web dashboard
├── docs/                    # Documentation
└── scripts/                 # Utilities
```

## Technical Details

### Model Format

**GGUF (GGML Universal File Format)**
- Efficient storage and loading
- Metadata embedded in file
- Multiple quantization levels
- Compatible with llama.cpp ecosystem

**Quantization Levels:**
- `Q2_K` - Maximum compression (~2 bits/weight)
- `Q3_K_M` - Good compression (~3 bits/weight)
- `Q4_K_M` - **Recommended** - Balanced quality/size (~4 bits/weight)
- `Q5_K_M` - Higher quality (~5 bits/weight)
- `Q6_K` - Near-original quality (~6 bits/weight)
- `Q8_0` - Minimal quality loss (~8 bits/weight)

```bash
# View quantization details
offgrid quantization
```

### System Requirements

**Minimum Configuration:**
- CPU: 2 cores, 2GHz+
- RAM: 2GB (for Q4 quantized small models)
- Disk: 2GB available
- OS: Linux (Ubuntu 20.04+, Debian 11+)

**Recommended Configuration:**
- CPU: 4+ cores
- RAM: 8GB+
- GPU: NVIDIA GPU with 6GB+ VRAM (optional)
- Disk: 10GB+ SSD

**GPU Requirements:**
- NVIDIA: GTX 1050 Ti or newer, CUDA 12.0+
- AMD: ROCm-compatible GPU (experimental)

**Model RAM Requirements:**

| Model Size | Quantization | RAM Needed | GPU VRAM |
|------------|--------------|------------|----------|
| 1B params  | Q4_K_M      | 2GB        | 2GB      |
| 3B params  | Q4_K_M      | 4GB        | 4GB      |
| 7B params  | Q4_K_M      | 8GB        | 6GB      |
| 13B params | Q4_K_M      | 16GB       | 12GB     |

### Performance

**Throughput (tokens/second):**

| Hardware | 1B Model | 7B Model | 13B Model |
|----------|----------|----------|-----------|
| CPU (4 cores) | 20-30 | 5-10 | 2-5 |
| RTX 3060 (12GB) | 80-100 | 35-45 | 20-25 |
| RTX 4090 (24GB) | 120-150 | 60-80 | 40-50 |

*Actual performance varies with model architecture, quantization, and prompt length.*

### Security

**Network Security:**
- Localhost-only binding (`127.0.0.1`)
- No external network access
- Random high ports for inter-process communication
- Systemd security directives

**Process Isolation:**
- Dedicated `offgrid` system user
- Restricted file system access
- No root privileges required for operation

**Data Privacy:**
- No telemetry or analytics
- No external API calls
- All inference runs locally
- Model files stay on disk

## API Reference

### Endpoints

**Health Check**
```
GET /health
```
Response:
```json
{
  "status": "ok",
  "timestamp": "2025-11-10T..."
}
```

**List Models**
```
GET /v1/models
```
Response:
```json
{
  "object": "list",
  "data": [
    {
      "id": "tinyllama-1.1b-chat.Q4_K_M",
      "object": "model",
      "created": 1731234567,
      "owned_by": "offgrid-llm"
    }
  ]
}
```

**Chat Completion**
```
POST /v1/chat/completions
```
Request:
```json
{
  "model": "tinyllama-1.1b-chat.Q4_K_M",
  "messages": [
    {"role": "system", "content": "You are helpful."},
    {"role": "user", "content": "Hello"}
  ],
  "temperature": 0.7,
  "max_tokens": 500,
  "stream": false
}
```

**Text Completion**
```
POST /v1/completions
```
Request:
```json
{
  "model": "tinyllama-1.1b-chat.Q4_K_M",
  "prompt": "Once upon a time",
  "max_tokens": 100
}
```

**OpenAI Client Compatibility:**
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:11611/v1",
    api_key="not-needed"  # Local inference, no auth required
)

response = client.chat.completions.create(
    model="tinyllama-1.1b-chat.Q4_K_M",
    messages=[
        {"role": "user", "content": "Explain quantum computing"}
    ]
)

print(response.choices[0].message.content)
```

## CLI Commands

### Model Management
```bash
offgrid list                        # List installed models
offgrid catalog                     # Browse model catalog
offgrid search <query>              # Search HuggingFace
offgrid download <id> [quant]       # Download from catalog
offgrid download-hf <repo>          # Download from HuggingFace
offgrid import <path>               # Import from storage
offgrid export <id> <dest>          # Export to storage
offgrid remove <id>                 # Remove model
```

### Inference
```bash
offgrid serve                       # Start HTTP server
offgrid run <model>                 # Interactive chat
offgrid benchmark <model>           # Performance test
```

### Configuration
```bash
offgrid config init                 # Generate config file
offgrid config show                 # Display config
offgrid config validate <path>      # Validate config
```

### Information
```bash
offgrid info                        # System information
offgrid quantization                # Quantization guide
offgrid help                        # Help documentation
```

## Configuration

### Environment Variables

```bash
# Server configuration
OFFGRID_PORT=11611              # HTTP server port
OFFGRID_HOST=127.0.0.1         # Bind address (localhost only)

# Model configuration
OFFGRID_MODELS_DIR=/var/lib/offgrid/models  # Model directory
OFFGRID_NUM_THREADS=4                        # CPU threads

# Inference settings
OFFGRID_MAX_CONTEXT=4096        # Context window size
OFFGRID_GPU_LAYERS=0            # GPU layer offloading (0=auto)
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

## Development

### Building from Source

```bash
# Prerequisites
sudo apt-get install build-essential cmake git golang-1.21

# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Build Go binary only
go build -o offgrid ./cmd/offgrid

# Run development server
./offgrid serve

# Build with llama.cpp (full installation)
sudo ./install.sh
```

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/server -v

# With coverage
go test -cover ./...

# Integration tests
go test -tags=integration ./...
```

### Project Structure

```
offgrid-llm/
├── cmd/offgrid/              # CLI application
│   └── main.go              # Entry point, command handling
├── internal/
│   ├── config/              # Configuration management
│   │   └── config.go        # ENV/YAML/JSON loading
│   ├── inference/           # Inference engines
│   │   ├── llama_http.go    # HTTP proxy to llama-server
│   │   ├── llama_stub.go    # Wrapper with port detection
│   │   └── mock.go          # Mock engine for testing
│   ├── models/              # Model management
│   │   ├── catalog.go       # Model catalog
│   │   ├── downloader.go    # HuggingFace downloads
│   │   ├── registry.go      # Installed model tracking
│   │   └── usb_importer.go  # USB import/export
│   ├── server/              # HTTP server
│   │   └── server.go        # API handlers, routing
│   ├── resource/            # System monitoring
│   │   └── monitor.go       # CPU/RAM/GPU tracking
│   └── p2p/                 # P2P networking
│       ├── discovery.go     # UDP broadcast
│       └── transfer.go      # Model sharing
├── pkg/api/                 # Public API types
│   └── types.go            # OpenAI-compatible structs
├── web/ui/                  # Web dashboard
│   └── index.html          # Single-page app
├── docs/                    # Documentation
├── scripts/                 # Utilities
└── install.sh              # Installation script
```

## Troubleshooting

### Service Issues

**Services won't start:**
```bash
# Check status
sudo systemctl status offgrid-llm llama-server

# View logs
sudo journalctl -u offgrid-llm -n 50
sudo journalctl -u llama-server -n 50

# Restart services
sudo systemctl restart llama-server
sudo systemctl restart offgrid-llm
```

**Connection refused:**
```bash
# Verify services are running
sudo systemctl is-active offgrid-llm llama-server

# Check ports
sudo netstat -tlnp | grep -E "(11611|offgrid|llama)"

# Test health endpoint
curl http://localhost:11611/health
```

### Model Issues

**Model not loading:**
```bash
# Check model directory
ls -la /var/lib/offgrid/models/

# Fix permissions
sudo chown -R offgrid:offgrid /var/lib/offgrid/models/
sudo chmod 664 /var/lib/offgrid/models/*.gguf

# Verify model format
file /var/lib/offgrid/models/*.gguf  # Should show "GGUF model file"

# Check active model
cat /etc/offgrid/active-model

# Force model reload
sudo systemctl restart llama-server
```

**Empty responses in chat:**
```bash
# Test llama-server directly
LLAMAPORT=$(cat /etc/offgrid/llama-port)
curl http://127.0.0.1:$LLAMAPORT/health

# Check for proxy interference
env | grep -i proxy

# Test without proxy
NO_PROXY='*' curl http://localhost:11611/v1/models

# Verify correct model name
offgrid list
# Use exact model ID from list in API calls
```

### GPU Issues

**GPU not detected:**
```bash
# Check GPU visibility
nvidia-smi                    # Should show GPU
nvcc --version               # Should show CUDA version

# Verify llama-server build
ldd /usr/local/bin/llama-server | grep cuda

# Check service logs for GPU initialization
sudo journalctl -u llama-server -n 100 | grep -i gpu

# Rebuild with GPU support
sudo ./reinstall.sh
```

**CUDA errors:**
```bash
# Check CUDA installation
ls -la /usr/local/cuda*/lib64/

# Update library paths
sudo ldconfig

# Verify driver
cat /proc/driver/nvidia/version

# Rebuild llama.cpp
sudo ./reinstall.sh
```

### Build Issues

**Compilation fails:**
```bash
# Check dependencies
sudo apt-get install build-essential cmake git

# View build logs
tail -100 /tmp/llama_build*.log

# Clean rebuild
sudo rm -rf ~/llama.cpp
sudo ./reinstall.sh

# Force CPU-only if GPU build fails
sudo ./reinstall.sh --cpu-only
```

**Go version too old:**
```bash
# Check Go version
go version  # Need 1.21+

# Install newer Go
sudo snap install go --classic

# Or download from golang.org
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### Performance Issues

**Slow inference:**
```bash
# Check if GPU is being used
nvidia-smi  # Monitor GPU utilization during inference

# Verify CPU threads
cat /proc/cpuinfo | grep processor | wc -l

# Check model quantization
offgrid list  # Look for Q4_K_M or higher

# Monitor resources
top
htop

# Test different quantization
offgrid download <model> Q4_K_M  # Smaller, faster
offgrid download <model> Q5_K_M  # Larger, better quality
```

**High memory usage:**
```bash
# Check model size
offgrid list

# Use smaller quantization
offgrid download <model> Q3_K_M  # More compressed

# Monitor memory
free -h
watch -n 1 free -h

# Restart services to clear cache
sudo systemctl restart llama-server
```

## Offline Deployment

### Air-Gapped Installation

**1. Prepare installation package on connected machine:**
```bash
# Download repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Download models
offgrid download-hf TheBloke/TinyLlama-1.1B-Chat-GGUF \
  --file tinyllama-1.1b-chat.Q4_K_M.gguf

# Package everything
tar czf offgrid-offline.tar.gz \
  offgrid-llm/ \
  ~/.offgrid-llm/models/*.gguf
```

**2. Transfer to air-gapped system:**
```bash
# Copy to USB
cp offgrid-offline.tar.gz /media/usb/

# On air-gapped machine
tar xzf /media/usb/offgrid-offline.tar.gz
cd offgrid-llm
sudo ./install.sh --cpu-only  # Or with GPU if available
```

**3. Import models:**
```bash
# Copy models
sudo cp ~/models/*.gguf /var/lib/offgrid/models/
sudo chown offgrid:offgrid /var/lib/offgrid/models/*.gguf

# Restart to load
sudo systemctl restart llama-server

# Verify
offgrid list
curl http://localhost:11611/v1/models
```

### USB Model Distribution

**Create portable package:**
```bash
# Script creates complete offline package
./scripts/create-usb-package.sh /media/usb tinyllama-1.1b-chat

# Package includes:
# - Model file with SHA256 checksum
# - Verification script
# - Documentation
# - Import instructions
```

**Import on destination:**
```bash
# Import from USB
offgrid import /media/usb

# Verify integrity (automatic)
# Models include embedded checksums
```

## Use Cases

**Edge Computing**
- Remote facilities without internet
- Industrial automation
- Field operations
- IoT gateways

**Air-Gapped Networks**
- Classified environments
- High-security facilities
- Compliance-driven deployments
- Government/military systems

**Resource-Constrained Deployments**
- Embedded systems
- Low-power devices
- Cost-sensitive infrastructure
- Limited bandwidth environments

**Privacy-Focused Applications**
- Healthcare (HIPAA compliance)
- Legal (attorney-client privilege)
- Financial (data sovereignty)
- Personal use (no data sharing)

## Technology Stack

**Backend:**
- Go 1.21+ (HTTP server, orchestration)
- llama.cpp (C++ inference engine)
- CUDA 12.0+ / ROCm (GPU acceleration)

**Infrastructure:**
- Systemd (service management)
- Linux (Ubuntu/Debian)

**Frontend:**
- Vanilla JavaScript (no build step)
- Server-Sent Events (streaming)
- CSS3 (responsive design)

**Dependencies:**
- CMake (build system)
- GCC/G++ 11+ (compiler)
- NVIDIA drivers (for GPU)

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

**Areas of interest:**
- AMD ROCm GPU support
- macOS/Windows support
- Performance optimization
- Documentation improvements
- Test coverage

**Development workflow:**
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
git checkout -b feature/your-feature
# Make changes
go test ./...
git commit -am "Description"
git push origin feature/your-feature
# Open pull request
```

## License

MIT License - See [LICENSE](LICENSE)

## Acknowledgments

- [llama.cpp](https://github.com/ggerganov/llama.cpp) - High-performance inference engine
- [HuggingFace](https://huggingface.co) - Model distribution and community
- GGUF format contributors

## Links

- **Documentation**: [docs/](docs/)
- **Architecture**: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **API Reference**: [docs/API.md](docs/API.md)
- **Model Setup**: [docs/MODEL_SETUP.md](docs/MODEL_SETUP.md)
- **Issues**: [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)

---

**Built for offline-first deployment · Zero external dependencies · Complete data sovereignty**
