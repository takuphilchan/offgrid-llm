# Documentation

Complete guide to OffGrid LLM.

---

## Start Here

| Guide | Time | Description |
|-------|------|-------------|
| [Quick Start](setup/quickstart.md) | 3 min | Install and run your first model |
| [CLI Reference](reference/cli.md) | - | All commands |
| [API Reference](reference/api.md) | - | REST endpoints |

---

## Setup

| Guide | Description |
|-------|-------------|
| [Quick Start](setup/quickstart.md) | Fastest path to running |
| [Installation](setup/installation.md) | All installation methods |
| [Docker](setup/docker.md) | Container deployment |
| [Autostart](setup/autostart.md) | Run as system service |

---

## Guides

### Core

| Guide | Description |
|-------|-------------|
| [Getting Started](guides/getting-started.md) | First-time walkthrough |
| [Models](guides/models.md) | Choosing and managing models |
| [Features](guides/features.md) | Feature overview |

### Features

| Guide | Description |
|-------|-------------|
| [AI Agents](guides/agents.md) | Autonomous task execution |
| [RAG](guides/embeddings.md) | Chat with your documents |
| [Audit Logs](guides/audit.md) | Security logging |
| [Multi-User](guides/multi-user.md) | User management |
| [Metrics](guides/metrics.md) | Monitoring |

---

## Reference

| Document | Description |
|----------|-------------|
| [CLI Reference](reference/cli.md) | Command-line usage |
| [API Reference](reference/api.md) | REST API endpoints |
| [Python SDK](../python/README.md) | Python library |

---

## Advanced

| Guide | Description |
|-------|-------------|
| [Architecture](advanced/architecture.md) | System design |
| [Performance](advanced/performance.md) | Optimization |
| [Low Memory](advanced/low-memory.md) | Running on 4GB RAM |
| [Building](advanced/building.md) | Build from source |
| [Deployment](advanced/deployment.md) | Production setup |
| [Inference TODO](INFERENCE_TODO.md) | Short checklist to enable real inference |

---

## Quick Reference

### Install

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

### Run

```bash
offgrid run llama3
```

### Server

```bash
offgrid serve
# Open http://localhost:11611
```

### Common Commands

```bash
offgrid list                 # Show installed models
offgrid search llama         # Search HuggingFace
offgrid doctor               # Check system
offgrid agent chat           # AI agent
offgrid kb add ./docs/       # Add documents to RAG
```

### API

```bash
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello"}]}'
```

---

## Links

- [Roadmap](ROADMAP.md) - Future development
- [Contributing](../dev/CONTRIBUTING.md) - How to contribute
- [GitHub](https://github.com/takuphilchan/offgrid-llm) - Source code
