# OffGrid LLM

**Run powerful AI models completely offline on your own computer.**

[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square)](https://github.com/takuphilchan/offgrid-llm/releases)

No cloud. No subscriptions. No data leaving your machine.

---

## Install

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

### Windows

Open PowerShell as Administrator and run:

```powershell
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

**That's it.** The installer downloads everything you need.

---

## Quick Start

### 1. Download a Model

After installation, open a terminal and download a model:

```bash
# For 8GB+ RAM systems (recommended)
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# For 4GB RAM systems (smaller model)
offgrid download-hf MaziyarPanahi/Llama-3.2-1B-Instruct-GGUF \
  --file Llama-3.2-1B-Instruct.Q4_K_M.gguf
```

### 2. Start Chatting

**Option A: Web Interface**

Open your browser to: `http://localhost:11611/ui/`

**Option B: Command Line**

```bash
offgrid run Llama-3.2-3B-Instruct-Q4_K_M
```

---

## What You Get

- **Desktop App** with system tray icon
- **Web Interface** at `http://localhost:11611/ui/`
- **Command Line** tool (`offgrid`)
- **OpenAI-compatible API** for developers

---

## Features

### For Everyone

| Feature | Description |
|---------|-------------|
| **Chat Interface** | Web UI with markdown rendering and code highlighting |
| **Session History** | Save and resume conversations |
| **Knowledge Base** | Upload documents for AI-powered Q&A |
| **Model Search** | Find models that fit your RAM |
| **USB Transfer** | Import/export models to external drives |

### For Developers

| Feature | Description |
|---------|-------------|
| **OpenAI API** | Drop-in replacement at `localhost:11611/v1/` |
| **Function Calling** | Tool use support |
| **Embeddings** | Vector embeddings for RAG |
| **Batch Processing** | Process multiple prompts in parallel |
| **JSON Output** | `--json` flag on all commands |

---

## System Requirements

| RAM | What You Can Run |
|-----|------------------|
| 4GB | Small models (1B-3B parameters) |
| 8GB | Medium models (7B parameters) |
| 16GB+ | Large models (13B+ parameters) |

**GPU is optional** but speeds things up significantly. Supports NVIDIA (CUDA), AMD (ROCm), Apple Silicon (Metal), and Vulkan.

---

## Common Commands

```bash
# List installed models
offgrid list

# Search for models
offgrid search llama --ram 8

# Download a model
offgrid download-hf <repo> --file <model>.gguf

# Start chatting
offgrid run <model-name>

# Check system status
offgrid info

# Start the server
offgrid serve
```

See [CLI Reference](docs/CLI_REFERENCE.md) for all commands.

---

## API Usage

OffGrid provides an OpenAI-compatible API:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:11611/v1",
    api_key="not-needed"
)

response = client.chat.completions.create(
    model="Llama-3.2-3B-Instruct-Q4_K_M",
    messages=[{"role": "user", "content": "Hello!"}]
)

print(response.choices[0].message.content)
```

See [API Reference](docs/API.md) for endpoints and examples.

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Quick Start](docs/QUICKSTART.md) | Get running in 5 minutes |
| [Model Setup](docs/guides/MODEL_SETUP.md) | Choosing and downloading models |
| [Features Guide](docs/guides/FEATURES_GUIDE.md) | All features explained |
| [4GB RAM Guide](docs/4GB_RAM.md) | Running on limited hardware |
| [API Reference](docs/API.md) | OpenAI-compatible endpoints |

**For Developers:** See [dev/CONTRIBUTING.md](dev/CONTRIBUTING.md) for building from source and contributing.

**Docker Users:** See [docs/DOCKER.md](docs/DOCKER.md) for container deployment.

---

## Support

- [Documentation](docs/README.md)
- [Issue Tracker](https://github.com/takuphilchan/offgrid-llm/issues)
- [Discussions](https://github.com/takuphilchan/offgrid-llm/discussions)

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**Built with [llama.cpp](https://github.com/ggerganov/llama.cpp)**
