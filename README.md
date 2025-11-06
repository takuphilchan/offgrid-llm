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
POST /v1/chat/completions    - Chat completions (OpenAI-compatible)
POST /v1/completions         - Text completions (OpenAI-compatible)
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

### Phase 2 (In Progress)
- [ ] llama.cpp integration
- [ ] Model loading from disk
- [ ] P2P model discovery & sharing
- [ ] USB model import API
- [ ] Multi-user support

### Phase 3
- [ ] Advanced quantization
- [ ] Bandwidth-aware syncing
- [ ] Web dashboard
- [ ] Mobile/ARM optimization
- [ ] Docker support

## 📖 Documentation

- [Model Setup Guide](docs/MODEL_SETUP.md) - Download and configure models
- [Quick Start Script](scripts/quickstart.sh) - Automated setup

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
