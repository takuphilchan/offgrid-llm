# OffGrid LLM - Complete Documentation Index

Welcome to OffGrid LLM! This is your complete guide to running AI language models completely offline.

---

##  Getting Started (Pick One)

### For Beginners
- **[Quick Start Guide](QUICKSTART.md)** - 5-minute setup walkthrough
- **[Docker README](../DOCKER_README.md)** - Docker in 2 minutes (recommended)

### For Developers
- **[Installation Guide](INSTALLATION.md)** - Complete installation options
- **[Building from Source](advanced/BUILDING.md)** - Custom compilation

---

##  User Guides

### Essential Guides
- **[Model Setup](guides/MODEL_SETUP.md)** - Choosing and downloading models
- **[Features Guide](guides/FEATURES_GUIDE.md)** - All features explained
- **[HuggingFace Integration](guides/HUGGINGFACE_INTEGRATION.md)** - Finding models

### Advanced Usage
- **[API Reference](API.md)** - OpenAI-compatible endpoints
- **[Embeddings Guide](guides/EMBEDDINGS_GUIDE.md)** - Vector embeddings
- **[JSON Output Mode](JSON_OUTPUT.md)** - Structured responses
- **[CLI Reference](CLI_REFERENCE.md)** - Command-line usage

---

## 🐳 Docker Deployment

### Docker Guides
- **[Docker Quick Start](../DOCKER_README.md)** - 2-minute setup
- **[Complete Docker Guide](DOCKER.md)** - Production deployment
- **[Docker Compose Examples](DOCKER.md#running-with-docker-compose)** - Different configurations

### Docker Files
```bash
docker-compose.yml          # Basic deployment
docker-compose.gpu.yml      # NVIDIA GPU support
docker-compose.prod.yml     # Production with SSL
nginx.conf.example          # Reverse proxy config
```

---

## ⚙️ Configuration & Optimization

### Performance
- **[Performance Tuning](advanced/PERFORMANCE.md)** - Speed optimization
- **[CPU Optimization](CPU_OPTIMIZATION.md)** - CPU-specific settings
- **[GPU Compatibility](CPU_COMPATIBILITY.md)** - GPU support

### System Setup
- **[Auto-Start Service](AUTO_START.md)** - Systemd configuration
- **[4GB RAM Guide](4GB_RAM.md)** - Low-memory systems
- **[Benchmark Comparison](BENCHMARK_COMPARE.md)** - Performance metrics

---

## 🏗️ Architecture & Development

### System Design
- **[Architecture](advanced/ARCHITECTURE.md)** - System overview
- **[llama.cpp Setup](advanced/LLAMA_CPP_SETUP.md)** - Backend integration
- **[Contributing Guide](../dev/CONTRIBUTING.md)** - Development workflow
- **[Version Management](VERSION_MANAGEMENT.md)** - Centralized versioning system

### Deployment
- **[Production Deployment](advanced/DEPLOYMENT.md)** - Scale and monitor
- **[Docker Production Setup](DOCKER.md#production-deployment)** - SSL, monitoring

---

##  Installation Methods Comparison

| Method | Best For | Setup Time | Complexity |
|--------|----------|------------|------------|
| **Docker** | Quick start, production | 2 min |  Easy |
| **Desktop App** | Non-technical users | 5 min |  Easy |
| **One-line Install** | CLI users | 5-10 min |  Moderate |
| **Build from Source** | Developers, custom builds | 15-30 min |  Advanced |

---

##  Common Use Cases

### By Scenario

**Quick Testing:**
```bash
# Docker - fastest way
docker-compose up -d
```

**Daily Use:**
```bash
# Desktop app - best UX
./installers/desktop.sh
```

**Production Server:**
```bash
# Docker with monitoring
docker-compose -f docker-compose.prod.yml up -d
```

**Air-Gapped Systems:**
```bash
# USB transfer after export
offgrid export /media/usb
```

**Development:**
```bash
# Build from source
git clone https://github.com/takuphilchan/offgrid-llm.git
sudo ./dev/install.sh
```

---

##  Finding Information

### By Task

**I want to...**
- **Get started quickly** → [Quick Start Guide](QUICKSTART.md)
- **Use Docker** → [Docker Guide](DOCKER.md)
- **Download models** → [Model Setup](guides/MODEL_SETUP.md)
- **Optimize performance** → [Performance Guide](advanced/PERFORMANCE.md)
- **Use the API** → [API Reference](API.md)
- **Deploy in production** → [Production Guide](advanced/DEPLOYMENT.md)
- **Transfer models offline** → [Features Guide](guides/FEATURES_GUIDE.md)
- **Build from source** → [Building Guide](advanced/BUILDING.md)
- **Understand the system** → [Architecture](advanced/ARCHITECTURE.md)
- **Contribute** → [Contributing](../dev/CONTRIBUTING.md)
- **Use AI Agents** → [Agent Guide](guides/AGENT_GUIDE.md)
- **Monitor metrics** → [Metrics Guide](guides/METRICS_GUIDE.md)
- **Enable multi-user mode** → [Multi-User Mode](guides/MULTI_USER_MODE.md)

### By Problem

**Common Issues:**
- Model won't load → [Performance Guide](advanced/PERFORMANCE.md)
- Out of memory → [4GB RAM Guide](4GB_RAM.md)
- Slow inference → [CPU Optimization](CPU_OPTIMIZATION.md)
- GPU not detected → [GPU Compatibility](CPU_COMPATIBILITY.md)
- Docker issues → [Docker Troubleshooting](DOCKER.md#troubleshooting)

---

**Ready to start?** → [Quick Start Guide](QUICKSTART.md) | [Docker README](../DOCKER_README.md)

**Complete documentation for installation, usage, and development.**

---

## Getting Started

| Document | Description | For |
|----------|-------------|-----|
| [**Installation**](INSTALLATION.md) | Complete installation guide with all methods | Everyone |
| [**CLI Reference**](CLI_REFERENCE.md) | Command-line interface documentation | Users |
| [**API Reference**](API.md) | REST API endpoints and usage | Developers |

---

## User Guides

**Learn how to use OffGrid LLM features:**

- [**Features Guide**](guides/FEATURES_GUIDE.md) - Complete feature overview
- [**Model Setup**](guides/MODEL_SETUP.md) - Download and manage models
- [**Embeddings Guide**](guides/EMBEDDINGS_GUIDE.md) - Using embeddings API
- [**HuggingFace Integration**](guides/HUGGINGFACE_INTEGRATION.md) - Direct HF model downloads

### New in v0.2.3

- [**AI Agent Guide**](guides/AGENT_GUIDE.md) - Autonomous AI agents with MCP support
- [**Metrics Guide**](guides/METRICS_GUIDE.md) - System monitoring and Prometheus metrics
- [**Multi-User Mode**](guides/MULTI_USER_MODE.md) - User management and authentication

---

## Advanced Topics

**For developers and advanced users:**

- [**Architecture**](advanced/ARCHITECTURE.md) - System design and components
- [**Building from Source**](advanced/BUILDING.md) - Compile and customize
- [**Deployment**](advanced/DEPLOYMENT.md) - Production deployment guide
- [**Performance Tuning**](advanced/PERFORMANCE.md) - Optimization tips
- [**llama.cpp Setup**](advanced/LLAMA_CPP_SETUP.md) - Backend configuration

---

## Additional Resources

- [**JSON Output**](JSON_OUTPUT.md) - Structured output for automation
- [**Auto-Start Guide**](AUTO_START.md) - Systemd service configuration

---

## Quick Links

- [Main README](../README.md)
- [Installation Scripts](../installers/)
- [Development Guide](../dev/CONTRIBUTING.md)
- [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)
