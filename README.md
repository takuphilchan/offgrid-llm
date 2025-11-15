# OffGrid LLM

**Run powerful language models completely offline with GPU acceleration.**

[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square)](https://github.com/takuphilchan/offgrid-llm/releases)

**Perfect for edge environments, air-gapped systems, and privacy-conscious deployments.**

---

## Why OffGrid LLM?

**100% Offline** - No internet required after setup  
**GPU Accelerated** - CUDA, ROCm, Metal, and Vulkan support  
**OpenAI Compatible** - Drop-in replacement for local inference  
**Web UI & Desktop App** - Modern interfaces included  
**Auto-Start Services** - Systemd integration for servers  
**USB Transfer** - Portable model deployment  

---

## Installation

### Quick Install (Recommended)

**Copy and paste this command into your terminal:**

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

**What happens:**
1. Automatically detects your computer (OS, CPU, GPU)
2. Builds and installs OffGrid LLM
3. Installs the web interface
4. Asks if you want to start it now
5. Sets up auto-start on boot (optional)

**Time required:** 5-10 minutes (downloads and builds from source)

**After installation, open your browser to:** `http://localhost:11611/ui/`

**Start without asking:**
```bash
AUTOSTART=yes bash <(curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh)
```



---

### Advanced Installation (For Developers)

**Build with full GPU optimization:**

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
sudo ./dev/install.sh
```

This builds llama.cpp from source with optimizations for your GPU.

See [dev/CONTRIBUTING.md](dev/CONTRIBUTING.md) for development setup.

---

## Getting Started

### Step 1: Verify Installation

```bash
offgrid --version
```

### Step 2: Download a Model

**Search for models that fit your RAM:**
```bash
offgrid search llama --ram 4     # 4GB RAM systems
offgrid search mistral --ram 8   # 8GB RAM systems
```

**Download a small model (works on 4GB+ RAM, ~2GB download):**
```bash
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf
```

**Even smaller for very limited RAM (~1GB download):**
```bash
offgrid search "1b llama" --ram 4
offgrid download-hf MaziyarPanahi/Llama-3.2-1B-Instruct-GGUF \
  --file Llama-3.2-1B-Instruct.Q4_K_M.gguf
```

### Step 3: Start Using

**Option A: Web Interface (Easiest)**

Open in your browser: `http://localhost:11611/ui/`

**Option B: Command Line**

```bash
offgrid run Llama-3.2-3B-Instruct-Q4_K_M
```

---

## Key Features

### For Users

**Chat & Sessions**
- Interactive CLI with streaming responses
- Save and resume conversations
- Export sessions to markdown
- Prompt templates for common tasks

**Model Management**
- RAM-aware search (--ram 4 shows models for 4GB systems)
- Search HuggingFace directly from CLI
- Download GGUF models automatically
- Import/export models via USB
- Model aliases and favorites

**Web Interface**
- Clean, responsive UI with Tailwind CSS
- Real-time markdown rendering
- Code syntax highlighting
- Model browser with system stats
- USB import/export with progress tracking

### For Developers

**OpenAI-Compatible API**
```bash
curl http://localhost:11611/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model.gguf",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Health & Monitoring**
- `/health` - Full system diagnostics
- `/ready` - Kubernetes readiness probe
- `/livez` - Liveness probe
- `/stats` - Per-model metrics

**Automation**
- JSON output mode for all commands
- Batch processing with JSONL
- Shell completions (bash/zsh/fish)
- Response caching with LRU

See [docs/API.md](docs/API.md) for complete API reference.

---

## Project Structure

```
offgrid-llm/
├── cmd/offgrid/        # Main application entry point
├── internal/           # Core implementation
│   ├── server/         # HTTP server & API endpoints
│   ├── models/         # Model management & HuggingFace
│   ├── inference/      # llama.cpp integration
│   ├── sessions/       # Conversation persistence
│   └── cache/          # Response caching
├── web/ui/             # Web interface (HTML/CSS/JS)
├── desktop/            # Electron desktop app
├── installers/         # Quick install scripts
├── dev/                # Build from source tools
└── docs/               # Complete documentation
    ├── guides/         # User guides
    └── advanced/       # Developer documentation
```

---

## Documentation

**[Complete Documentation](docs/README.md)**

**Getting Started:**
- [Installation Guide](docs/INSTALLATION.md)
- [CLI Reference](docs/CLI_REFERENCE.md)
- [API Reference](docs/API.md)

**User Guides:**
- [Features Overview](docs/guides/FEATURES_GUIDE.md)
- [Model Setup](docs/guides/MODEL_SETUP.md)
- [Embeddings](docs/guides/EMBEDDINGS_GUIDE.md)
- [HuggingFace Integration](docs/guides/HUGGINGFACE_INTEGRATION.md)

**Advanced:**
- [Architecture](docs/advanced/ARCHITECTURE.md)
- [Building from Source](docs/advanced/BUILDING.md)
- [Deployment](docs/advanced/DEPLOYMENT.md)
- [Performance Tuning](docs/advanced/PERFORMANCE.md)

---

## Usage Examples

### Model Management

```bash
# List installed models
offgrid list

# Search HuggingFace with filters
offgrid search llama --quant Q4_K_M --limit 10

# Import from USB
offgrid import /media/usb

# Export to USB
offgrid export model-name /media/usb
```

### Interactive Chat

```bash
# Start a session
offgrid run model-name --save my-project

# Continue later
offgrid run model-name --load my-project

# List sessions
offgrid session list
```

### API Usage (Python)

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:11611/v1",
    api_key="not-needed"
)

response = client.chat.completions.create(
    model="your-model.gguf",
    messages=[
        {"role": "user", "content": "Explain quantum computing"}
    ]
)

print(response.choices[0].message.content)
```

---

## What You Need

**Your Computer:**
- Works on: Linux, macOS, Windows 10 or newer
- Memory (RAM): 4GB minimum (1B-3B models), 8GB+ recommended (7B models)
- Storage: 10GB+ free space for AI models
- GPU: Optional but makes it faster (NVIDIA, AMD, or Apple)

**Runs on modest hardware:**
- 4GB RAM: Llama 1B-3B models
- 8GB RAM: Llama 7B-8B models
- 16GB+ RAM: Llama 13B+ models

See [4GB RAM Guide](docs/4GB_RAM.md) for budget hardware recommendations.

---

## Contributing

We welcome contributions! See [dev/CONTRIBUTING.md](dev/CONTRIBUTING.md) for:
- Development setup
- Code standards
- Testing guidelines
- Pull request process

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Support

- [Documentation](docs/README.md)
- [Issue Tracker](https://github.com/takuphilchan/offgrid-llm/issues)
- [Discussions](https://github.com/takuphilchan/offgrid-llm/discussions)

---

**Built with [llama.cpp](https://github.com/ggerganov/llama.cpp) for inference.**
