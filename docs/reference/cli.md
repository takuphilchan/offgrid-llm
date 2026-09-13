# CLI Reference

Complete reference for all `offgrid` commands.

---

## Quick Start

```bash
offgrid run llama3           # Chat with a model
offgrid serve                # Start web UI server
offgrid list                 # Show installed models
offgrid doctor               # Check system health
offgrid appliance status     # Check hardware fit for an offline box
```

---

## Models

### Run a Model

```bash
offgrid run <model>
```

Start interactive chat. Supports aliases and auto-download:

```bash
offgrid run llama3           # Uses built-in alias
offgrid run mistral          # Auto-downloads if missing
offgrid run ./local.gguf     # Use local file
```

**Chat commands:** `exit` to quit, `clear` to reset conversation.

---

### List Models

```bash
offgrid list
```

Shows installed models with sizes.

---

### Search HuggingFace

```bash
offgrid search <query> [flags]
```

| Flag | Description |
|------|-------------|
| `--author` | Filter by author |
| `--sort` | Sort by: downloads, likes, recent |
| `--limit` | Max results (default: 10) |

```bash
offgrid search llama
offgrid search "code llama" --author TheBloke
offgrid search mistral --sort downloads --limit 5
```

---

### Download Models

**From HuggingFace:**
```bash
offgrid download-hf <repo-id> [--file <filename>]
```

```bash
offgrid download-hf TheBloke/Llama-2-7B-GGUF
offgrid download-hf Qwen/Qwen2.5-3B-Instruct-GGUF --file qwen2.5-3b-instruct-q4_k_m.gguf
```

Vision models auto-download matching projector files.

**From catalog:**
```bash
offgrid download <model-id> [quantization]
offgrid catalog              # Browse available
```

---

### Aliases

Model shortcuts for common models:

```bash
offgrid alias list           # Show all aliases
offgrid alias add mymodel TheBloke/MyModel-GGUF
offgrid alias remove mymodel
```

**Built-in aliases:**

| Alias | Model |
|-------|-------|
| `llama3` | Llama 3.2 3B |
| `llama3:8b` | Llama 3.1 8B |
| `qwen` | Qwen 2.5 3B |
| `mistral` | Mistral 7B |
| `phi` | Phi 3 Mini |
| `codellama` | Code Llama 7B |
| `deepseek` | DeepSeek Coder 6.7B |
| `gemma` | Gemma 2 2B |
| `tiny` | TinyLlama 1.1B |

Full list: `offgrid alias list`

---

### Import/Export

```bash
offgrid import <path>              # From USB/directory
offgrid export <model> <path>      # To USB/directory
```

```bash
offgrid import /media/usb
offgrid export llama3 /media/usb
```

---

### Remove Models

```bash
offgrid remove <model>
```

Prompts for confirmation.

---

## Server

### Start Server

```bash
offgrid serve [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--host` | Bind address | 127.0.0.1 |
| `--port` | Port number | 11611 |
| `--model` | Default model | auto |

```bash
offgrid serve
offgrid serve --port 8080
offgrid serve --host 0.0.0.0 --model llama3
```

Web UI: http://localhost:11611

---

### Health Check

```bash
curl http://localhost:11611/health
```

---

## Appliance

Plan and prepare offline AI boxes for schools, libraries, clinics, and community
networks.

```bash
offgrid appliance <command>
```

| Command | Description |
|---------|-------------|
| `status` | Show this machine's appliance fit |
| `plan [profile]` | Show hardware and deployment plan |
| `profile [name]` | Show profile tuning settings |
| `init [profile]` | Create local appliance metadata |

Profiles:

| Profile | Best Use |
|---------|----------|
| `lite` | Raspberry Pi class learning node |
| `hub` | Refurbished mini PC for schools and community centers |
| `gpu` | NVIDIA GPU lab machine |
| `jetson` | Jetson robotics, camera, and edge AI kit |

```bash
offgrid appliance status
offgrid appliance plan hub
offgrid appliance profile lite
offgrid appliance init hub
```

See [Appliance Deployment](../setup/appliance.md).

---

## AI Agents

### Interactive Agent

```bash
offgrid agent chat [flags]
```

| Flag | Description |
|------|-------------|
| `--model` | Model for agent |
| `--style` | `react` or `cot` |
| `--template` | Use preset template |
| `--max-steps` | Max reasoning steps |

```bash
offgrid agent chat
offgrid agent chat --template coder
offgrid agent chat --model qwen --style react
```

---

### Run Single Task

```bash
offgrid agent run "<task>" [flags]
```

```bash
offgrid agent run "Calculate factorial of 10"
offgrid agent run "Search for Python tutorials" --model llama3
```

---

### Templates

Pre-configured agent personas:

```bash
offgrid agent templates        # List available
```

| Template | Purpose |
|----------|---------|
| `researcher` | Information gathering, summarization |
| `coder` | Code writing, debugging, review |
| `analyst` | Data analysis, pattern recognition |
| `writer` | Content creation, editing |
| `sysadmin` | System administration, DevOps |
| `planner` | Task breakdown, project planning |

```bash
offgrid agent chat --template researcher
offgrid agent chat --template coder
```

---

### Agent Tools

```bash
offgrid agent tools            # List available tools
```

Built-in tools: calculator, web search, file operations, shell commands.

---

### MCP Servers

Model Context Protocol integration:

```bash
offgrid agent mcp list
offgrid agent mcp add <name> "<command>"
offgrid agent mcp remove <name>
offgrid agent mcp test "<command>"
```

```bash
offgrid agent mcp add filesystem "npx -y @modelcontextprotocol/server-filesystem /tmp"
offgrid agent mcp add memory "npx -y @modelcontextprotocol/server-memory"
```

---

## External Agents

Install, configure, verify, and launch Hermes Agent through OffGrid:

```bash
offgrid hermes install                    # Complete guided setup
offgrid hermes install --with-browser     # Also install optional browser/computer-use tools
offgrid hermes status                     # Inspect setup and model-context preflight
offgrid hermes test                       # Run an inference smoke test
offgrid hermes                            # Start interactive Hermes chat
offgrid hermes -q "Summarize this repo"   # Start with a prompt
```

The install command uses Hermes' official installer when the runtime is not
present, installs the native OffGrid provider, and persists configuration
through Hermes' own configuration CLI. Use `--yes` for an unattended install.
The default skips npm/browser tools and unrelated full diagnostics; use
`--with-browser` or `--doctor` to opt in. Optional npm/browser failures are
reported as warnings after the working core is configured. Hermes requires a
real OffGrid context of at least 64,000 tokens; do not inflate the Hermes
setting beyond what OffGrid actually allocates.

Lower-level provider inspection and configuration remain available:

```bash
offgrid integrations list
offgrid integrations setup hermes --model <model-id>
offgrid openclaw install                   # Install and configure OpenClaw
offgrid openclaw status                    # Check provider and model readiness
offgrid openclaw test                      # Headless model-response smoke test
offgrid openclaw run "Summarize this repo" # Run an isolated local agent turn
```

`offgrid integrations install openclaw` only copies the provider bundle; use
`offgrid openclaw install` for complete setup. An installed OpenClaw runtime
is left at its current version. Missing runtimes use OpenClaw's official
installer after confirmation; pass `--yes` only when you trust that installer
and the local provider. The managed setup updates only the `offgrid` provider
in OpenClaw's config and does not change an existing default model.

---

## Knowledge Base (RAG)

Chat with your documents:

```bash
offgrid kb status              # Show status
offgrid kb enable <model>      # Enable with an embedding model
offgrid kb disable             # Disable retrieval
offgrid kb list                # List documents
offgrid kb add <path>          # Add file or directory
offgrid kb search "<query>"    # Search
offgrid kb remove <id>         # Remove document
offgrid kb clear               # Clear all
```

```bash
offgrid download bge-small-en-v1.5
offgrid kb enable bge-small-en-v1.5
offgrid kb add ./docs/manual.pdf
offgrid kb add ./notes/
offgrid kb search "how to configure"
```

---

## P2P Network

View beta peer discovery and local-network model transfer status.

P2P is useful for trusted local labs, but USB import/export is still recommended
for critical offline deployments.

```bash
offgrid peers                  # List connected peers
```

---

## LoRA Adapters

Register LoRA adapter metadata.

Runtime hot-loading is experimental and is not available for the current
backend yet.

```bash
offgrid lora list
offgrid lora register <name> <path> [scale]
offgrid lora info <id>
offgrid lora scale <id> <value>
offgrid lora remove <id>
```

---

## Audit Logs

Security audit logging with tamper-evident chain:

```bash
offgrid audit show [--limit N] [--type TYPE]
offgrid audit stats
offgrid audit verify
offgrid audit export-json <file>
offgrid audit export-csv <file>
```

| Subcommand | Description |
|------------|-------------|
| `show` | Display recent events |
| `stats` | Show audit statistics |
| `verify` | Verify chain integrity |
| `export-json` | Export to JSON file |
| `export-csv` | Export to CSV file |

```bash
offgrid audit show --limit 50
offgrid audit show --type auth
offgrid audit export-csv /tmp/audit-report.csv
offgrid audit verify
```

---

## Users

Multi-user mode (requires `OFFGRID_MULTI_USER=true`):

```bash
offgrid users                  # List users
offgrid users create <name> [--role admin|user]
offgrid users delete <id>
```

---

## System

### Version

```bash
offgrid version
```

Shows version, platform, and GPU detection.

---

### Doctor

```bash
offgrid doctor
```

Checks:
- Models directory
- Server connectivity
- Disk space
- GPU availability
- llama-server binary

---

### Info

```bash
offgrid info
```

Shows system information and configuration.

---

### Quantization Guide

```bash
offgrid quantization
```

Explains quantization levels (Q4_K_M, Q5_K_M, etc.) and trade-offs.

---

### Benchmark

```bash
offgrid benchmark <model>
```

Measures inference speed, memory usage, and latency.

---

## Configuration

```bash
offgrid config show            # Show current config
offgrid config set <key> <val> # Set value
offgrid config reset           # Reset to defaults
```

| Key | Description | Default |
|-----|-------------|---------|
| `models_dir` | Model storage path | ~/.offgrid-llm/models |
| `server_port` | Default server port | 11611 |
| `default_model` | Default model for serve | auto |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OFFGRID_PORT` | Server port |
| `OFFGRID_HOST` | Server bind address |
| `OFFGRID_MODELS_DIR` | Models directory |
| `OFFGRID_MULTI_USER` | Enable multi-user mode |
| `OFFGRID_LOG_LEVEL` | Log level (debug, info, warn, error) |
| `NO_COLOR` | Disable colored output |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Model not found |
| 4 | Connection error |
| 5 | Permission denied |

---

## See Also

- [Quick Start](../setup/quickstart.md)
- [Agents Guide](../guides/agents.md)
- [API Reference](api.md)
- [Configuration](../advanced/configuration.md)
