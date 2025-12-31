# OffGrid LLM

Run AI models locally. No cloud. No API keys. Complete privacy.

[![Version](https://img.shields.io/badge/Version-0.2.12-blue.svg)](https://github.com/takuphilchan/offgrid-llm/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-10b981.svg)](LICENSE)
[![PyPI](https://img.shields.io/pypi/v/offgrid)](https://pypi.org/project/offgrid/)

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

## Run

```bash
offgrid run llama3
```

That's it. The model downloads automatically and you start chatting.

---

## Why OffGrid?

| Feature | OffGrid | Ollama | LM Studio |
|---------|:-------:|:------:|:---------:|
| Air-gapped USB deployment | Yes | - | - |
| P2P model sharing | Yes | - | - |
| Built-in RAG | Yes | - | - |
| Multi-user + audit logs | Yes | - | - |
| AI Agents with MCP | Yes | - | Partial |
| Voice assistant | Yes | - | - |
| OpenAI-compatible API | Yes | Yes | Yes |

**Built for:**
- Healthcare (HIPAA) - patient data never leaves
- Government - air-gapped deployment
- Enterprise - audit logging and multi-user
- Remote sites - ships, rigs, expeditions
- Research - unlimited use, zero API costs

---

## Quick Start

### Web UI

```bash
offgrid serve
```

Open http://localhost:11611

### CLI Chat

```bash
offgrid run llama3           # Chat with Llama 3
offgrid run mistral          # Chat with Mistral
offgrid run codellama        # Chat with Code Llama
```

### Python

```bash
pip install offgrid
```

```python
import offgrid

client = offgrid.Client()
response = client.chat("Hello!")
print(response)
```

---

## Models

Built-in shortcuts:

| Alias | Model | RAM |
|-------|-------|-----|
| `tiny` | TinyLlama 1.1B | 2 GB |
| `phi` | Phi 3 Mini | 4 GB |
| `llama3` | Llama 3.2 3B | 4 GB |
| `qwen` | Qwen 2.5 3B | 4 GB |
| `mistral` | Mistral 7B | 8 GB |
| `codellama` | Code Llama 7B | 8 GB |

```bash
offgrid alias list           # See all shortcuts
offgrid search llama         # Search HuggingFace
offgrid list                 # Show installed
```

---

## Features

### AI Agents

Autonomous task execution with tool use:

```bash
offgrid agent chat --template coder
offgrid agent run "Analyze this data and create a chart"
```

Templates: `researcher`, `coder`, `analyst`, `writer`, `sysadmin`, `planner`

### Knowledge Base (RAG)

Chat with your documents:

```bash
offgrid kb add ./documents/
offgrid kb search "project requirements"
```

### Voice

Speech-to-text and text-to-speech:

```bash
offgrid audio transcribe recording.wav
offgrid audio speak "Hello world" --output hello.wav
```

### Offline Transfer

Export models to USB for air-gapped systems:

```bash
offgrid export llama3 /media/usb
offgrid import /media/usb
```

### Audit Logs

Tamper-evident security logging:

```bash
offgrid audit show
offgrid audit export-csv report.csv
offgrid audit verify
```

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Quick Start](docs/setup/quickstart.md) | Get running in 3 minutes |
| [CLI Reference](docs/reference/cli.md) | All commands |
| [API Reference](docs/reference/api.md) | REST endpoints |
| [Python SDK](python/README.md) | Python library |
| [Agents](docs/guides/agents.md) | AI agent system |
| [RAG](docs/guides/embeddings.md) | Knowledge base |
| [Docker](docs/setup/docker.md) | Container deployment |

---

## System Requirements

| RAM | Models |
|-----|--------|
| 4 GB | TinyLlama, SmolLM, Phi 3 Mini |
| 8 GB | Llama 3.2 3B, Qwen 2.5 3B, Gemma 2B |
| 16 GB | Mistral 7B, Llama 3.1 8B, Code Llama |
| 32 GB+ | Llama 3 70B, Mixtral, DeepSeek |

GPU optional. Supports NVIDIA (CUDA), AMD (ROCm), Apple Silicon (Metal).

---

## Contributing

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
go build -o bin/offgrid ./cmd/offgrid
./bin/offgrid serve
```

See [Contributing Guide](dev/CONTRIBUTING.md).

---

## License

MIT - See [LICENSE](LICENSE)

Built with [llama.cpp](https://github.com/ggerganov/llama.cpp)

---

[Documentation](docs/README.md) | [Issues](https://github.com/takuphilchan/offgrid-llm/issues) | [Roadmap](docs/ROADMAP.md)
