# 🌐 OffGrid LLM

**AI for Edge & Offline Environments**

OffGrid LLM is a lightweight, offline-first LLM orchestrator designed for environments with limited or intermittent internet connectivity. Built in Go for maximum performance and minimal resource usage.

## 🎯 Features

- ✅ **Offline-First**: Works without internet connectivity
- 🔄 **P2P Model Sharing**: Share models across local networks
- 💾 **USB Model Import**: Install models from USB drives/SD cards
- ⚡ **Low Resource**: Runs on devices with as little as 2GB RAM
- 🔌 **OpenAI-Compatible API**: Drop-in replacement for OpenAI API
- 🌍 **Edge-Ready**: Perfect for remote locations, ships, clinics, schools
- 📦 **Single Binary**: Easy deployment, no dependencies

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Quick start (builds and optionally downloads a model)
./scripts/quickstart.sh

# Or manually:
# Build
make build

# Run
./offgrid
```

Server will start on `http://localhost:8080`

See [Model Setup Guide](docs/MODEL_SETUP.md) for downloading models.

## 📚 API Endpoints

```
GET  /health                  - Health check
GET  /v1/models              - List available models
POST /v1/chat/completions    - Chat completions (OpenAI-compatible, supports streaming)
POST /v1/completions         - Text completions (OpenAI-compatible)
```

### Streaming Support

Enable streaming by setting `"stream": true` in your request:

```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tinyllama-1.1b-chat",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

## 🎯 CLI Commands

```bash
offgrid                          # Start server (default)
offgrid catalog                  # Browse available models
offgrid download <model-id>      # Download a model
offgrid list                     # List installed models
offgrid help                     # Show help
```

### Example: Download and Run

```bash
# Browse available models
offgrid catalog

# Download TinyLlama (638MB)
offgrid download tinyllama-1.1b-chat

# Start the server
offgrid

# Test in another terminal
curl http://localhost:8080/v1/models
```

## 🏗️ Architecture

```
offgrid-llm/
├── cmd/offgrid/           # Main application entry point
├── internal/
│   ├── server/           # HTTP server & API handlers
│   ├── models/           # Model management & registry
│   ├── inference/        # LLM inference engine
│   ├── resource/         # Resource monitoring & allocation
│   └── p2p/              # Peer-to-peer networking
├── pkg/api/              # Public API types
└── web/ui/               # Web dashboard (future)
```

## 🎯 Use Cases

- 🚢 **Maritime & Offshore** - Ships, oil rigs, research vessels
- 🏥 **Healthcare** - Rural clinics, mobile medical units
- 🏫 **Education** - Schools in low-bandwidth areas
- 🏭 **Industrial** - Factories, mines, warehouses
- 🔒 **High-Security** - Air-gapped networks
- 🏕️ **Field Research** - Remote scientific operations

## 🛣️ Roadmap

### Phase 1 ✅ (Completed)
- [x] Basic HTTP server
- [x] OpenAI-compatible API structure
- [x] Model registry and management
- [x] Resource monitoring
- [x] Configuration system
- [x] P2P discovery foundation
- [x] Unit tests
- [x] **Streaming support (SSE)** ⭐ NEW
- [x] **P2P file transfer** ⭐ NEW

### Phase 2 (In Progress)
- [ ] llama.cpp integration
- [ ] Model loading from disk
- [ ] P2P model discovery & sharing (discovery done, integration pending)
- [ ] USB model import API
- [ ] Multi-user support
- [ ] Web dashboard

### Phase 3
- [ ] Advanced quantization
- [ ] Bandwidth-aware syncing
- [ ] Web dashboard
- [ ] Mobile/ARM optimization
- [ ] Docker support

## 📖 Documentation

- [Model Setup Guide](docs/MODEL_SETUP.md) - Download and configure models
- [Architecture & Distribution Strategy](docs/ARCHITECTURE.md) - How offline distribution works
- [API Documentation](docs/API.md) - Complete API reference
- [Quick Start Script](scripts/quickstart.sh) - Automated setup

## 🔧 Scripts & Tools

```bash
./scripts/quickstart.sh              # Interactive setup with model download
./scripts/create-usb-package.sh      # Create offline USB installation
./scripts/example_client.sh          # Bash API examples
./scripts/example_client.py          # Python API examples
```

## 💾 Offline Distribution

### USB Package Creation

```bash
# Create complete offline package
./scripts/create-usb-package.sh /media/usb tinyllama-1.1b-chat

# Result: USB drive with binaries, models, docs, installers
# Works on Linux, Windows, macOS - no internet needed!
```

### Model Catalog

Built-in catalog with 4 recommended models:
- **TinyLlama 1.1B** - Lightweight (638MB, 2GB RAM)
- **Llama 2 7B** - Balanced quality (3.8GB, 8GB RAM) 
- **Mistral 7B** - Excellent for code (4.1GB, 8GB RAM)
- **Phi-2** - Efficient reasoning (1.5GB, 4GB RAM)

## 🧪 Testing

```bash
# Run tests
make test

# Build
make build

# Run in development mode
make dev
```

## 🤝 Contributing

Contributions welcome! This project aims to make AI accessible in underserved environments.

## 📄 License

MIT License - See LICENSE file for details

## 💡 Philosophy

**AI should work everywhere, not just where the internet is fast.**

OffGrid LLM brings powerful language models to edge environments, remote locations, and anywhere reliable internet connectivity isn't guaranteed.
