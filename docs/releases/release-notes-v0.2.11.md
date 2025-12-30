# Release Notes v0.2.11

**Release Date:** December 30, 2025

Version 0.2.11 is a significant performance and UX update focused on **model loading speed**, **low RAM optimization**, **CLI experience**, and **enterprise-grade features**. This release dramatically improves model switching times, adds real-time loading progress, enhances the interactive chat CLI, and introduces new enterprise capabilities.

## Highlights

- **5x Faster Model Loading** - Optimized for low RAM systems with aggressive polling and reduced delays
- **Real-Time Loading Progress** - SSE-based progress streaming with phase tracking (0-100%)
- **Enhanced CLI Chat Experience** - New commands, visual screen clear, status display, and slash command support
- **Hover Pre-warming** - Models warm into page cache when you hover over the dropdown
- **Fast Concurrent Warming** - 16MB chunks with 4 parallel readers for 3-5x faster warming
- **RAM-Aware Auto-Configuration** - Context size, batch size, and KV cache auto-tuned for available RAM
- **Loading Tracker API** - Server-side progress tracking with `/v1/loading/progress` endpoints
- **Enterprise Features** - LDAP/AD authentication, MCP marketplace, distributed RAG, plugin system

---

## Performance Optimizations

### Model Loading Speed (Critical for low RAM)

**Before:** Model switching took 60-120s on cold start, 15-30s even when warm
**After:** 5-15s when warm, significantly faster cold starts

#### Polling Interval Improvements
- Server startup detection: **500ms → 200ms** (2.5x faster detection)
- Model loading check: **1000ms → 500ms** (2x faster feedback)
- SSE progress stream: **200ms → 100ms** (smoother UI updates)

#### Eliminated Blocking Delays
- Removed 300ms warmup delay - model is ready immediately
- Background warmup no longer blocks the request
- First inference may be slightly slower, but model switch is instant

#### RAM-Aware Auto-Configuration
For 8-16GB RAM systems, the following are now auto-tuned:
- **Context size**: Automatically reduced to 2K-4K based on model size
- **Batch size**: Reduced to 64-128 for faster time-to-first-token
- **KV cache quantization**: Auto-enabled (`q8_0`) to reduce memory by ~50%

```go
// Example: low RAM with 5GB model
// Context: 2048 (vs 4096 default)
// Batch: 64 (vs 256 default)  
// KV Cache: q8_0 (reduces VRAM ~50%)
```

### Fast Pre-warming with Concurrent I/O

New `FastWarmModel()` uses aggressive read-ahead:
- **16MB chunks** instead of 4MB
- **4 parallel readers** for maximum disk throughput
- **3-5x faster** than sequential warming

```go
// New API endpoint for hover pre-warming
POST /v1/loading/prewarm
{"model_path": "/path/to/model.gguf"}
```

### Hover Pre-warming

When users hover over a model in the dropdown:
1. 300ms debounce to avoid spamming
2. Fast pre-warm triggered in background
3. Model is in page cache before user clicks
4. Switch feels nearly instant

---

## New Features

### Loading Progress Tracker

Server-side loading progress tracking with real-time SSE streaming:

```bash
# Get current progress (snapshot)
curl http://localhost:11611/v1/loading/progress

# Stream progress updates (SSE)
curl http://localhost:11611/v1/loading/progress/stream
```

**Response:**
```json
{
  "model_id": "llama3",
  "phase": "loading",
  "progress": 65,
  "message": "Loading model weights...",
  "elapsed_ms": 3500,
  "estimated_ms": 8000,
  "is_warm": true,
  "size_mb": 4096
}
```

**Loading Phases:**
| Phase | Progress | Description |
|-------|----------|-------------|
| `idle` | 0% | No loading in progress |
| `unloading` | 0-15% | Unloading previous model |
| `starting` | 15-40% | Starting inference server |
| `loading` | 40-90% | Loading model weights |
| `warmup` | 90-100% | Warming up inference pipeline |
| `ready` | 100% | Model ready to serve |
| `failed` | - | Loading failed |

### Enterprise Features

#### LDAP/Active Directory Authentication
Full LDAP support with group-to-role mapping:
```yaml
ldap:
  server: ldap.example.com
  port: 636
  use_tls: true
  base_dn: dc=example,dc=com
  user_filter: "(sAMAccountName=%s)"
  admin_groups: ["Domain Admins", "IT-Staff"]
  user_groups: ["Domain Users"]
```

#### MCP Server Marketplace
Browse and install popular MCP servers with one click:
- Filesystem, GitHub, Slack, PostgreSQL
- Brave Search, Puppeteer, Memory
- Auto-install via npm/pip/docker

#### Distributed RAG
Federated search across multiple OffGrid nodes:
```bash
# Add remote nodes
POST /v1/rag/nodes
{"id": "node2", "url": "http://192.168.1.100:11611"}

# Search spans all healthy nodes
POST /v1/rag/search
{"query": "...", "distributed": true}
```

#### Plugin System
Custom tool plugins in Python, Node, Go, or Shell:
```json
// plugins/my-tool/plugin.json
{
  "id": "my-tool",
  "name": "My Custom Tool",
  "type": "tool",
  "language": "python",
  "entry_point": "main.py"
}
```

### Multi-Agent Orchestration
New orchestration modes for complex workflows:
- **Sequential** - Agents run one after another
- **Parallel** - Agents run simultaneously
- **Debate** - Agents discuss and refine answers
- **Voting** - Agents vote on best answer
- **Hierarchy** - Supervisor delegates to workers

### Inference Load Balancer
Distribute load across multiple inference backends:
- Round-robin, weighted, least-connections, latency-based strategies
- Support for llama-server, Ollama, LocalAI, vLLM, OpenAI-compatible APIs
- Health checks with automatic failover

### On-Device Model Quantization
Quantize models locally without external tools:
```bash
POST /v1/quantize
{
  "input_path": "/models/llama3.gguf",
  "quant_type": "q4_k_m"
}
```
Supports 14 quantization types from Q8_0 to IQ2_XXS.

---

## UI Improvements

### Theme Support
- Light and dark themes with system preference detection
- 9 built-in color themes
- Smooth theme transitions

### P2P Network Tab
- Peer discovery UI
- One-click model download from peers
- Transfer progress tracking
- Model integrity verification

### Agent Templates
Pre-configured agent personas:
- Researcher, Coder, Analyst, Writer
- System Administrator, Project Planner

---

## CLI Improvements

### Enhanced Interactive Chat

The `offgrid run <model>` interactive chat has been significantly improved:

#### New Commands

| Command | Description |
|---------|-------------|
| `help` or `?` | Show all available chat commands |
| `status` | Display current session info (model, messages, RAG, GPU) |
| `clear` | Clear screen AND conversation history (fresh start) |
| `rag` | Toggle knowledge base on/off |
| `exit`, `quit`, or `q` | Exit the chat |

#### Slash Command Support
All commands now work with or without `/` prefix:
```
› help       ← works
› /help      ← also works
› ?          ← shortcut for help
› q          ← quick exit
```

#### Visual Screen Clear
The `clear` command now properly clears the terminal screen and reprints the header, giving you a fresh chat experience:

```
◉ OffGrid Chat

Model:   Llama-3.2-1B-Instruct-Q4_K_M.gguf
Status:  ✓ Ready
GPU:     NVIDIA GeForce GTX 1050 Ti

Commands:  exit · clear · rag · status · help

✓ Conversation cleared
```

#### Session Status Display
The new `status` command shows detailed session information:
```
◉ Session Status

  Model:       Llama-3.2-1B-Instruct-Q4_K_M
  Messages:    6 (3 user, 3 assistant)
  Session:     my-session
  RAG:         enabled
  GPU:         NVIDIA GeForce GTX 1050 Ti
```

---

## API Changes

### New Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/loading/progress` | GET | Current loading progress |
| `/v1/loading/progress/stream` | GET | SSE stream of progress |
| `/v1/loading/prewarm` | POST | Fast pre-warm a model |
| `/v1/mcp/marketplace` | GET | List available MCP servers |
| `/v1/mcp/install` | POST | Install MCP server |
| `/v1/rag/nodes` | GET/POST | Manage distributed RAG nodes |
| `/v1/quantize` | POST | Quantize a model |
| `/v1/plugins` | GET | List installed plugins |
| `/v1/orchestrate` | POST | Run multi-agent workflow |

### Configuration Changes

New environment variables:
```bash
# RAM-aware defaults (auto-detected)
OFFGRID_CONTEXT_SIZE=0     # 0 = auto-detect based on RAM
OFFGRID_BATCH_SIZE=0       # 0 = auto-detect based on RAM
OFFGRID_KV_CACHE_TYPE=     # Empty = auto-enable q8_0 on <16GB RAM

# LDAP
OFFGRID_LDAP_SERVER=
OFFGRID_LDAP_BIND_DN=
OFFGRID_LDAP_BASE_DN=
```

---

## Bug Fixes

- Fixed model loading progress not updating in UI
- Fixed hover pre-warming triggering too aggressively
- Fixed SSE connection not closing on model ready
- Fixed memory leak in loading tracker subscribers
- Fixed KV cache quantization crash on some llama.cpp builds

---

## Upgrade Guide

### From v0.2.10

1. **Stop the server**
   ```bash
   offgrid stop
   ```

2. **Update binary**
   ```bash
   # Linux/macOS
   curl -fsSL https://offgrid.dev/install.sh | bash
   
   # Or rebuild from source
   go build -o bin/offgrid ./cmd/offgrid
   ```

3. **Start server**
   ```bash
   offgrid serve
   ```

No configuration changes required. RAM-aware optimizations are automatic.

### For low RAM Systems

The new defaults should work well, but you can fine-tune:

```bash
# Force specific context size
export OFFGRID_CONTEXT_SIZE=2048

# Force smaller batch for faster first token
export OFFGRID_BATCH_SIZE=64

# Enable KV cache quantization (if not auto-enabled)
export OFFGRID_KV_CACHE_TYPE=q8_0
```

---

## Known Issues

- Hover pre-warming may not work in all browsers (dropdown hover events vary)
- LDAP sync with very large directories (>10,000 users) may timeout
- Distributed RAG search may return slightly slower results than local-only

---

## Contributors

Thanks to all contributors who made this release possible!

---

## What's Next (v0.3.0 Preview)

- **Bundled binaries** - Auto-download llama-server on first run
- **Auto-start inference** - Model loads automatically on selection
- **`offgrid run <model>`** - One-command model download and chat
- **Video demo** - 30-second hero video on landing page

---

**Full Changelog:** https://github.com/takuphilchan/offgrid-llm/compare/v0.2.10...v0.2.11
