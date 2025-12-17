# OffGrid LLM

<p align="center">
  <strong>Run powerful AI models completely offline on your own computer.</strong>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square" alt="License: MIT"></a>
  <a href="https://github.com/takuphilchan/offgrid-llm/releases"><img src="https://img.shields.io/badge/Version-0.2.7-blue.svg?style=flat-square" alt="Version"></a>
  <a href="https://github.com/takuphilchan/offgrid-llm/releases"><img src="https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square" alt="Platform"></a>
  <a href="https://pypi.org/project/offgrid/"><img src="https://img.shields.io/pypi/v/offgrid?style=flat-square&color=3776AB" alt="PyPI"></a>
</p>

<p align="center">
  No cloud. No subscriptions. No data leaving your machine.
</p>

<p align="center">
  <img src="docs/images/chat-page.png" alt="OffGrid LLM Chat Interface" width="800"/>
</p>

---

## Why OffGrid LLM?

| Problem | OffGrid Solution |
|---------|------------------|
| 🔒 **Privacy concerns** | All processing happens locally - your data never leaves your machine |
| 💰 **Expensive API costs** | Free forever after download - no subscriptions or per-token fees |
| 🌐 **Internet dependency** | Works completely offline - perfect for remote locations |
| 🏢 **Enterprise restrictions** | Air-gapped deployment for sensitive environments |
| 🎓 **Learning AI** | Experiment freely without cost or rate limits |

---

## Features at a Glance

```
┌─────────────────────────────────────────────────────────────────┐
│                        OffGrid LLM                               │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   🤖 AI Core    │   🎤 Voice      │   📚 Knowledge              │
│   ───────────   │   ─────────     │   ─────────────             │
│   • Chat UI     │   • Speech→Text │   • RAG/Embeddings          │
│   • Streaming   │   • Text→Speech │   • Document ingestion      │
│   • Sessions    │   • 18+ langs   │   • Semantic search         │
│   • AI Agent    │   • Whisper     │   • Context injection       │
├─────────────────┼─────────────────┼─────────────────────────────┤
│   🔧 Tools      │   📊 Ops        │   🔌 Integration            │
│   ───────────   │   ─────────     │   ─────────────             │
│   • Model Hub   │   • Metrics     │   • REST API                │
│   • Benchmarks  │   • Multi-user  │   • Python SDK              │
│   • Terminal    │   • Monitoring  │   • OpenAI compatible       │
│   • LoRA        │   • Auto-start  │   • USB transfer            │
└─────────────────┴─────────────────┴─────────────────────────────┘
```

---

## Quick Start

### 1. Install

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

<details>
<summary>📦 Other installation methods</summary>

**Interactive Install (choose components):**
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh -o install.sh
bash install.sh
```

**Docker:**
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm && docker-compose up -d
```

**Python library only:**
```bash
pip install offgrid
```

**From source:**
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm && go build -o bin/offgrid ./cmd/offgrid
```

</details>

### 2. Start Server

```bash
offgrid serve
```

### 3. Open Browser

Navigate to **http://localhost:11611**

That's it! Download a model from the Models tab and start chatting.

---

## Usage Examples

### Python Library

```python
import offgrid

# Connect to server
client = offgrid.Client()  # defaults to localhost:11611

# Simple chat
response = client.chat("Explain quantum computing in simple terms")
print(response)

# Streaming response
for chunk in client.chat("Tell me a story", stream=True):
    print(chunk, end="", flush=True)

# With options
response = client.chat(
    "Write a haiku about coding",
    model="Llama-3.2-3B-Instruct",
    temperature=0.9,
    max_tokens=100
)
```

### Knowledge Base (RAG)

```python
# Add your documents
client.kb.add("meeting_notes.txt")
client.kb.add_directory("./company_docs")

# Chat with document context
response = client.chat("What were the action items from the meeting?", use_kb=True)
```

### Voice Assistant

```python
# Speech to text
text = client.audio.transcribe("recording.wav")
print(text["text"])

# Text to speech
audio = client.audio.speak("Hello! How can I help you today?")
with open("greeting.wav", "wb") as f:
    f.write(audio)
```

### Command Line

```bash
offgrid list                           # List downloaded models
offgrid search "llama 3" --ram 8       # Search models for 8GB RAM
offgrid download-hf repo/model file    # Download from HuggingFace
offgrid run model-name                 # Interactive chat
offgrid audio transcribe recording.wav # Transcribe audio
```

---

## Web Interface

The browser-based UI at `http://localhost:11611` provides:

| Tab | Description |
|-----|-------------|
| **Chat** | Conversational AI with session history and markdown rendering |
| **Voice** | Push-to-talk voice assistant with transcription |
| **Agent** | Autonomous AI that can execute multi-step tasks |
| **Models** | Browse HuggingFace, download, and manage models |
| **Knowledge** | Upload documents for RAG-powered conversations |
| **LoRA** | Load fine-tuned adapters for specialized tasks |
| **Benchmark** | Compare model performance metrics |
| **Terminal** | Run CLI commands from the browser |
| **Users** | Multi-user management with API keys |
| **Metrics** | Real-time server statistics and monitoring |

---

## System Requirements

| RAM | Recommended Models | Use Case |
|-----|-------------------|----------|
| **4GB** | TinyLlama, Phi-2 | Basic tasks, low-end devices |
| **8GB** | Llama 3.2 3B, Mistral 7B | General use, most users |
| **16GB** | Llama 3 8B, CodeLlama 13B | Professional work, coding |
| **32GB+** | Llama 3 70B, Mixtral | Research, complex tasks |

**GPU:** Optional but recommended. Supports NVIDIA (CUDA), AMD (ROCm), Apple Silicon (Metal), and Vulkan.

---

## Project Structure

```
offgrid-llm/
├── cmd/offgrid/        # CLI application entry point
├── internal/           # Core Go packages (30+ modules)
│   ├── server/         # HTTP API server
│   ├── inference/      # LLM inference engine
│   ├── agents/         # AI agent orchestration
│   ├── rag/            # Vector search & embeddings
│   └── ...             # Audio, metrics, config, etc.
├── web/ui/             # Browser interface (modular JS)
├── desktop/            # Electron desktop app
├── python/             # Python SDK
├── docs/               # Documentation
├── scripts/            # Build & deployment scripts
└── docker/             # Container configurations
```

---

## Documentation

### Getting Started
- [Quick Start](docs/setup/quickstart.md) - 3-minute setup
- [Getting Started Guide](docs/guides/getting-started.md) - Complete walkthrough
- [Installation](docs/setup/installation.md) - All installation methods

### User Guides
- [Python Library](python/README.md) - Full Python API
- [API Reference](docs/reference/api.md) - REST endpoints
- [CLI Reference](docs/reference/cli.md) - Command-line usage
- [Models](docs/guides/models.md) - Choosing models
- [Features](docs/guides/features.md) - All features
- [Embeddings](docs/guides/embeddings.md) - RAG setup
- [Agents](docs/guides/agents.md) - AI agent usage

### Advanced
- [Architecture](docs/advanced/architecture.md) - System design
- [Performance](docs/advanced/performance.md) - Optimization
- [Docker](docs/setup/docker.md) - Container setup
- [Building](docs/advanced/building.md) - Custom builds

### Contributing
- [Contribution Guide](dev/CONTRIBUTING.md) - How to contribute
- [Code Style](dev/CONTRIBUTING.md#code-style-guide) - Coding standards
- [Web UI Guide](web/ui/README.md) - Frontend development

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| **FUSE error (Linux)** | `sudo apt install libfuse2` |
| **Voice not working** | `rm -rf ~/.offgrid-llm/audio && offgrid audio setup whisper` |
| **Model loading slow** | Use quantized models (Q4_K_M recommended) |
| **Out of memory** | Try smaller model or increase swap |
| **Server won't start** | Check if port 11611 is in use |

See [docs/setup/installation.md](docs/setup/installation.md#troubleshooting) for detailed troubleshooting.

---

## Contributing

We welcome contributions! See our [Contributing Guide](dev/CONTRIBUTING.md) for details.

```bash
# Quick setup for contributors
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
go mod download
make build
./bin/offgrid serve --verbose
```

---

## License

MIT License - See [LICENSE](LICENSE)

**Built with [llama.cpp](https://github.com/ggerganov/llama.cpp)**

---

<p align="center">
  <a href="docs/setup/quickstart.md">Quick Start</a> •
  <a href="docs/README.md">Documentation</a> •
  <a href="https://github.com/takuphilchan/offgrid-llm/issues">Issues</a> •
  <a href="dev/CONTRIBUTING.md">Contributing</a>
</p>
