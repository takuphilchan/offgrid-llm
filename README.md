# OffGrid LLM

**Run powerful AI models completely offline on your own computer.**

[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square)](https://github.com/takuphilchan/offgrid-llm/releases)
[![PyPI](https://img.shields.io/pypi/v/offgrid-llm?style=flat-square&color=3776AB)](https://pypi.org/project/offgrid-llm/)

No cloud. No subscriptions. No data leaving your machine.

---

## Install

### Desktop App

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

**Windows** (PowerShell as Admin):
```powershell
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

### Python Library

```bash
pip install offgrid-llm
```

---

## Usage

### Python

```python
import offgrid_llm

# Chat
response = offgrid_llm.chat("Hello!")
print(response)

# Streaming
for chunk in offgrid_llm.chat("Tell me a story", stream=True):
    print(chunk, end="", flush=True)
```

### With Client

```python
from offgrid_llm import Client

client = Client()

# Chat with options
response = client.chat(
    "Explain quantum computing",
    system="You are a physics teacher",
    temperature=0.7,
    max_tokens=500
)

# List models
for model in client.list_models():
    print(model["id"])
```

### Model Management

```python
# Search HuggingFace
results = client.models.search("llama", ram=8)

# Download
client.models.download(
    "bartowski/Llama-3.2-3B-Instruct-GGUF",
    "Llama-3.2-3B-Instruct-Q4_K_M.gguf"
)

# Import/Export USB
client.models.import_usb("/media/usb")
client.models.export_usb("model-name", "/media/usb")
```

### Knowledge Base (RAG)

```python
# Add documents
client.kb.add("notes.txt")
client.kb.add("meeting", content="Meeting notes...")
client.kb.add_directory("./docs")

# Chat with context
response = client.chat("Summarize the meeting", use_kb=True)

# Search documents
results = client.kb.search("deadline")
```

### Embeddings

```python
# Single
embedding = client.embed("Hello world")

# Batch
embeddings = client.embed(["Hello", "World"])
```

---

## Web UI & CLI

After installing the desktop app:

**Web Interface:** `http://localhost:11611/ui/`

**Command Line:**
```bash
offgrid list                    # List models
offgrid search llama --ram 8    # Search HuggingFace
offgrid download-hf repo --file model.gguf
offgrid run model-name          # Interactive chat
offgrid serve                   # Start server
```

---

## System Requirements

| RAM | Models |
|-----|--------|
| 4GB | 1B-3B parameters |
| 8GB | 7B parameters |
| 16GB+ | 13B+ parameters |

GPU optional. Supports NVIDIA (CUDA), AMD (ROCm), Apple Silicon (Metal), Vulkan.

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Python Library](python/README.md) | Full Python API reference |
| [Quick Start](docs/QUICKSTART.md) | Get running in 5 minutes |
| [CLI Reference](docs/CLI_REFERENCE.md) | All commands |
| [API Reference](docs/API.md) | REST API endpoints |
| [Model Setup](docs/guides/MODEL_SETUP.md) | Choosing models |

**Docker:** [docs/DOCKER.md](docs/DOCKER.md) · **Contributing:** [dev/CONTRIBUTING.md](dev/CONTRIBUTING.md)

---

## License

MIT License - [LICENSE](LICENSE)

**Built with [llama.cpp](https://github.com/ggerganov/llama.cpp)**
