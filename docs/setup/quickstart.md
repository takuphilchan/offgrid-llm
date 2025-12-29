# Quick Start

Get OffGrid LLM running in 3 minutes.

---

## Install

**One command:**

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

**Or build from source:**

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm && go build -o bin/offgrid ./cmd/offgrid
sudo mv bin/offgrid /usr/local/bin/
```

---

## Run

```bash
offgrid run llama3
```

That's it. The model downloads automatically and you start chatting.

**Other models to try:**

| Command | Model | RAM Needed |
|---------|-------|------------|
| `offgrid run tiny` | TinyLlama 1.1B | 2 GB |
| `offgrid run phi` | Phi 3 Mini | 4 GB |
| `offgrid run llama3` | Llama 3.2 3B | 4 GB |
| `offgrid run qwen` | Qwen 2.5 3B | 4 GB |
| `offgrid run mistral` | Mistral 7B | 8 GB |
| `offgrid run codellama` | Code Llama 7B | 8 GB |

See all shortcuts: `offgrid alias list`

---

## Web UI

Start the server:

```bash
offgrid serve
```

Open http://localhost:11611

The web interface includes:
- Chat with model selection
- Model download and management
- Knowledge base (RAG) for documents
- Session history
- Voice input/output

---

## Common Commands

| Task | Command |
|------|---------|
| List installed models | `offgrid list` |
| Check system | `offgrid doctor` |
| Search HuggingFace | `offgrid search llama` |
| Download specific model | `offgrid download-hf TheBloke/Llama-2-7B-GGUF` |
| Start AI agent | `offgrid agent chat` |
| View P2P peers | `offgrid peers` |
| Export audit logs | `offgrid audit export-csv report.csv` |
| All commands | `offgrid --help` |

---

## Models by RAM

| Your RAM | Recommended Models |
|----------|-------------------|
| 4 GB | TinyLlama, SmolLM, Qwen 0.5B |
| 8 GB | Llama 3.2 3B, Phi 3, Gemma 2B |
| 16 GB | Mistral 7B, Qwen 7B, Code Llama 7B |
| 32 GB+ | Llama 3.1 8B, DeepSeek, Mixtral |

---

## Installation Options

<details>
<summary>Docker</summary>

```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm && docker-compose up -d
```

Open http://localhost:11611

See [Docker Guide](docker.md) for GPU support.

</details>

<details>
<summary>Desktop App (Electron)</summary>

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

**Windows (PowerShell as Admin):**
```powershell
irm https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.ps1 | iex
```

</details>

<details>
<summary>Python SDK only</summary>

```bash
pip install offgrid
```

```python
import offgrid

client = offgrid.Client()
response = client.chat("Hello!")
print(response)
```

</details>

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Model download fails | `offgrid doctor` to check connectivity |
| Server won't start | `lsof -i :11611` to check port usage |
| Out of memory | Use smaller model: `offgrid run tiny` |
| Slow responses | Check GPU: `offgrid version` |

---

## Next Steps

- [Full Installation Guide](installation.md) - All options and configuration
- [CLI Reference](../reference/cli.md) - Complete command documentation
- [AI Agents](../guides/agents.md) - Autonomous task execution
- [RAG Guide](../guides/embeddings.md) - Chat with your documents
- [Performance Tuning](../advanced/performance.md) - Optimize for your hardware 
