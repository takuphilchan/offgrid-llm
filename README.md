# OffGrid LLM

**Run powerful language models completely offline with GPU acceleration.**

[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-0078D4.svg?style=flat-square)](https://github.com/takuphilchan/offgrid-llm/releases)

**Perfect for edge environments, air-gapped systems, and privacy-conscious deployments.**

---

## Why OffGrid LLM?

✅ **100% Offline** - No internet required after setup  
✅ **GPU Accelerated** - CUDA, ROCm, Metal, and Vulkan support  
✅ **OpenAI Compatible** - Drop-in replacement for local inference  
✅ **Web UI & Desktop App** - Modern interfaces included  
✅ **Auto-Start Services** - Systemd integration for servers  
✅ **USB Transfer** - Portable model deployment  

---

## Installation

### One-Line Install ⚡

**Single command - services start automatically:**

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

**What it does:**
- ✅ Auto-detects your OS, architecture, and GPU
- ✅ Downloads optimized bundle from [GitHub releases](https://github.com/takuphilchan/offgrid-llm/releases)
- ✅ Installs both `offgrid` and `llama-server` binaries
- ✅ Verifies checksums for security
- ✅ **Starts services immediately** - ready to use!
- ✅ Enables auto-start on boot (Linux systemd)

**Installation time:** ~30 seconds  
**Ready to use:** Immediately after install

**Advanced options:**
```bash
# Auto-start services without prompts
AUTOSTART=yes bash <(curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh)

# Install without starting services
AUTOSTART=no bash <(curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh)

# Install specific version
VERSION=v0.1.0 bash <(curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh)
```

---

### Desktop App 🖥️

**Download native installers:**

- **Linux**: [AppImage](https://github.com/takuphilchan/offgrid-llm/releases/latest) - Make executable and run
- **macOS**: [DMG](https://github.com/takuphilchan/offgrid-llm/releases/latest) - Drag to Applications
- **Windows**: [Installer](https://github.com/takuphilchan/offgrid-llm/releases/latest) - Run .exe

**Features:**
- Self-contained with embedded servers
- Native file pickers for USB import/export
- No terminal required

---

### Manual Installation

**Download bundles from [releases](https://github.com/takuphilchan/offgrid-llm/releases/latest):**

Choose your platform:
- `offgrid-v0.1.0-linux-amd64-vulkan.tar.gz` (Linux with GPU)
- `offgrid-v0.1.0-darwin-arm64-metal.tar.gz` (macOS Apple Silicon)
- `offgrid-v0.1.0-windows-amd64-cpu.zip` (Windows)
- See [all releases](https://github.com/takuphilchan/offgrid-llm/releases) for more variants

Extract and run `install.sh` (or copy binaries to PATH on Windows).

---

### Build from Source (Developers)

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
sudo ./dev/install.sh  # Build with GPU optimization
```

See [dev/CONTRIBUTING.md](dev/CONTRIBUTING.md) for development setup.

---

## Quick Start

```bash
# Verify installation
offgrid --version

# Search for models on HuggingFace
offgrid search llama --limit 5

# Download a model (example: ~4GB)
offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF \
  --file Llama-3.2-3B-Instruct-Q4_K_M.gguf

# Start chatting
offgrid run Llama-3.2-3B-Instruct-Q4_K_M

# Or use the web interface
firefox http://localhost:11611/ui
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

📚 **[Complete Documentation](docs/README.md)**

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

## System Requirements

**Minimum:**
- OS: Linux (x64/arm64), macOS (Intel/Apple Silicon), Windows 10+
- RAM: 8GB (4GB models), 16GB (7B models), 32GB+ (13B+ models)
- Storage: 10GB+ for models

**Recommended:**
- GPU: NVIDIA (CUDA), AMD (ROCm), Apple (Metal), Vulkan-compatible
- RAM: 16GB+
- Storage: SSD for better model loading times

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

- 📖 [Documentation](docs/README.md)
- 🐛 [Issue Tracker](https://github.com/takuphilchan/offgrid-llm/issues)
- 💬 [Discussions](https://github.com/takuphilchan/offgrid-llm/discussions)

---

**Built with [llama.cpp](https://github.com/ggerganov/llama.cpp) for inference.**
