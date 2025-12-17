# OffGrid LLM Documentation

> Complete guide to running AI language models completely offline.

---

## Documentation Structure

```
docs/
├── setup/          # Installation & configuration
├── guides/         # Feature tutorials & how-tos  
├── reference/      # API & CLI specifications
└── advanced/       # Architecture & optimization
```

---

## Getting Started

| Document | Time | Description |
|----------|------|-------------|
| [**Getting Started**](guides/getting-started.md) | 5 min | Complete beginner's guide |
| [**Quick Start**](setup/quickstart.md) | 3 min | Fastest path to first chat |
| [**Installation**](setup/installation.md) | 10 min | Full installation options |

---

## Setup & Installation

| Document | Description |
|----------|-------------|
| [quickstart.md](setup/quickstart.md) | Get running in 3 minutes |
| [installation.md](setup/installation.md) | All installation methods |
| [docker.md](setup/docker.md) | Docker deployment |
| [autostart.md](setup/autostart.md) | Systemd service setup |

---

## User Guides

### Core Features

| Guide | Description |
|-------|-------------|
| [getting-started.md](guides/getting-started.md) | First-time user walkthrough |
| [models.md](guides/models.md) | Download & configure models |
| [features.md](guides/features.md) | Feature overview |
| [huggingface.md](guides/huggingface.md) | HuggingFace integration |

### AI Capabilities

| Guide | Description |
|-------|-------------|
| [agents.md](guides/agents.md) | Autonomous AI agents |
| [embeddings.md](guides/embeddings.md) | RAG & document search |
| [multi-user.md](guides/multi-user.md) | User management |
| [metrics.md](guides/metrics.md) | Monitoring & statistics |

---

## Reference

| Document | Description |
|----------|-------------|
| [api.md](reference/api.md) | REST API endpoints |
| [cli.md](reference/cli.md) | Command-line usage |
| [json-output.md](reference/json-output.md) | Structured output format |
| [versioning.md](reference/versioning.md) | Version management |

---

## Advanced

### Architecture & Development

| Document | Description |
|----------|-------------|
| [architecture.md](advanced/architecture.md) | System design overview |
| [building.md](advanced/building.md) | Build from source |
| [deployment.md](advanced/deployment.md) | Production deployment |
| [distribution.md](advanced/distribution.md) | Offline distribution |
| [llama-cpp.md](advanced/llama-cpp.md) | llama.cpp backend setup |
| [inference-roadmap.md](advanced/inference-roadmap.md) | Development roadmap |

### Performance & Optimization

| Document | Description |
|----------|-------------|
| [performance.md](advanced/performance.md) | General optimization |
| [cpu-tuning.md](advanced/cpu-tuning.md) | CPU performance tuning |
| [cpu-support.md](advanced/cpu-support.md) | CPU compatibility info |
| [low-memory.md](advanced/low-memory.md) | Running on 4GB RAM |
| [benchmarking.md](advanced/benchmarking.md) | Model comparison |

---

## Quick Reference

### Installation Methods

| Method | Command | Time |
|--------|---------|------|
| One-liner | `curl -fsSL .../install.sh \| bash` | 5 min |
| Docker | `docker compose up -d` | 2 min |
| Python | `pip install offgrid` | 1 min |
| Source | `go build ./cmd/offgrid` | 15 min |

### Common Commands

```bash
# Start server
offgrid serve

# List models
offgrid models list

# Interactive chat
offgrid chat

# Download model
offgrid models pull <model-name>
```

### API Quick Start

```bash
curl http://localhost:11611/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "default", "messages": [{"role": "user", "content": "Hello"}]}'
```

---

## Contributing

| Document | Description |
|----------|-------------|
| [Contributing Guide](../dev/CONTRIBUTING.md) | How to contribute |
| [Web UI Development](../web/ui/README.md) | Frontend contribution |

---

## File Naming Convention

All documentation follows these conventions:

- **Lowercase with hyphens**: `getting-started.md`, `cpu-tuning.md`
- **Descriptive names**: `embeddings.md` not `EMBEDDINGS_GUIDE.md`
- **Organized by purpose**: setup/, guides/, reference/, advanced/
