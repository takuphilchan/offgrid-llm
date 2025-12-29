# OffGrid LLM Roadmap

> From "local LLM tool" to **the #1 private AI platform**.

**Last Updated:** December 2025  
**Current Version:** 0.2.11

---

## Vision Statement

**OffGrid LLM is not "poor man's Ollama" - it's the AI platform for people who can't or won't use the cloud.**

Our unique positioning:
- **True air-gapped operation** (USB transfer, P2P sharing)
- **Multi-user with audit logging** (enterprise compliance)
- **Agentic AI with MCP** (not just chat)
- **Built-in RAG** (no external vector DB needed)
- **Voice interface** (Whisper + Piper, 18 languages)

---

## Phase 1: Production-Ready Inference (CRITICAL)

**Goal:** Users should have working AI chat within 5 minutes of install.

### 1.1 Auto-Download llama.cpp Binaries
- [x] Auto-download `llama-server` on first run (via BinaryManager)
- [x] Linux x64 support (AVX2)
- [x] macOS ARM64 (Apple Silicon) support
- [x] macOS x64 (Intel) support
- [x] Windows x64 support
- [x] CUDA build for NVIDIA GPUs (Windows)
- [x] Progress bar during download

### 1.2 Automatic Server Lifecycle
- [x] Start `llama-server` when user runs `offgrid run <model>`
- [x] Graceful shutdown on model switch
- [x] Health checks before routing requests
- [x] Port conflict resolution
- [x] Crash detection and auto-restart (integrated into ModelCache)

**Files:** `internal/inference/binary_manager.go`, `internal/inference/model_cache.go`

### 1.3 Remove Mock Mode
- [x] Make llama.cpp the default engine (server uses LlamaHTTPEngine by default)
- [x] Remove mock responses from CLI commands (batch now uses real API)
- [x] Clear error messages when llama-server unavailable

### 1.4 GPU Auto-Detection
- [x] Detect NVIDIA GPU (check for `nvidia-smi`)
- [x] Detect AMD GPU (check for `rocm-smi`)
- [x] Auto-select appropriate llama-server binary (CUDA on Windows/Linux with NVIDIA)
- [x] Auto-enable GPU layers when GPU detected
- [x] Show GPU status in CLI and server banner

---

## Phase 2: User Experience Parity

**Goal:** Match or exceed Ollama's ease of use.

### 2.1 One-Command Model Run
```bash
# Current (complex)
offgrid serve &
# then open browser, download model, etc.

# Target (simple)
offgrid run llama3.2     # Downloads if needed, starts chat
offgrid run mistral      # Named aliases work
offgrid run ./my.gguf    # Local files work
```

- [x] `offgrid run <model>` command ✅ Already exists
- [x] Model name aliases (llama3.2 → Llama-3.2-3B-Instruct-Q4_K_M.gguf) ✅ Implemented v0.2.11
- [x] Auto-download from HuggingFace by alias ✅ Implemented v0.2.11
- [x] Interactive CLI chat mode - Already exists

### 2.2 Better Progress Feedback
- [x] Download progress bars in terminal (llama-server download)
- [x] Model loading progress in UI (ModelManager notifies with elapsed time)
- [x] Estimated time remaining - CLI and UI both calculate ETA from speed
- [x] Cancel in-progress downloads - Cancel button in download modal, server-side cancellation

### 2.3 First-Run Experience
- [x] Welcome wizard (already in v0.2.11, polish it) - Enhanced with RAM-based model recommendations
- [x] Recommended starter model download - Shown in `offgrid list` when empty
- [x] Hardware capability check - `offgrid init` and `offgrid doctor`
- [x] Quick tutorial - Onboarding wizard with 3-step guide

---

## Phase 3: Unique Value Propositions

**Goal:** Features Ollama doesn't have.

### 3.1 P2P Model Sharing (Polish)
- [x] CLI command for P2P status - `offgrid peers` command
- [x] Discovery UI in web interface - P2P Network tab shows discovered peers
- [x] One-click "Download from Peer" - Download button per model in P2P UI
- [x] Transfer progress UI - Active transfers section with progress bars
- [x] One-click "Share to Network" - All local models automatically shared when P2P enabled
- [x] P2P status API - `/v1/p2p/status` returns shared models and network info
- [x] Integrity verification display - Verify button shows SHA256 hash in P2P UI

### 3.2 USB/Offline Distribution (Polish)
- [x] Create USB packages - `offgrid package` command
- [x] Create USB packages from UI - Export to USB section in Models tab
- [x] Import from USB in UI - Import from USB section in Models tab
- [x] Include installer on USB - Checkbox to include platform-specific install scripts
- [x] Model signature verification - Ed25519 signed manifests with publisher keys

### 3.3 Enterprise Features
- [x] API key management - via config and CLI
- [x] Audit log export - JSON export via `/v1/audit/export`
- [x] Role-based access - admin/user roles with multi-user mode
- [x] API key management UI - Regenerate key in user details modal
- [x] Usage quotas per user - Set/view quotas in user details with progress bars
- [x] LDAP/Active Directory auth - Full LDAP support with group-to-role mapping

### 3.4 Agentic AI Improvements
- [x] Agent templates - researcher, coder, analyst, creative templates
- [x] MCP server configuration - via config file
- [x] Custom tool creation UI - Create Tool button with shell/HTTP tool builder
- [x] Agent memory persistence - Tasks saved to disk and restored on restart
- [x] MCP server marketplace - Browse and install popular MCP servers
- [x] Multi-agent orchestration - Sequential, parallel, debate, voting, hierarchy modes

### 3.5 Advanced RAG
- [x] PDF parsing with layout awareness - pdftotext + basic fallback, image detection
- [x] Image extraction from documents - Detects and counts images in PDF, notes in metadata
- [x] Web page ingestion - Import from URL in Knowledge Base tab
- [x] Automatic chunking tuning - Analyzes document type and auto-adjusts chunk size/overlap
- [x] Hybrid search (semantic + keyword) - FTS5 + vector search with configurable alpha
- [x] Citation tracking - Source URLs and metadata in search results with [N] format

---

## Phase 4: Performance & Scale

### 4.1 Inference Optimization
- [x] Flash attention - llama-server uses flash attention by default
- [x] Continuous batching (enabled by default) - Multi-request throughput via -cb flag
- [x] KV cache quantization options - Configurable via OFFGRID_KV_CACHE_TYPE (q8_0, q4_0, f16)
- [x] Speculative decoding - Draft model support via config with --model-draft, --draft, --draft-min flags
- [x] Model quantization on-device - `/v1/quantize` API with 14 quantization types (Q8_0 to IQ2_XXS)

### 4.2 Multi-Model
- [x] Model pool for multi-user scenarios - ModelCache with multiple instances
- [x] Automatic model eviction (LRU) - via ModelCache max instances
- [x] Model prewarming - prewarm package with background loading
- [x] Hot-swap models without full reload - `/v1/models/hotswap` API with mmap pre-warming

### 4.3 Horizontal Scaling
- [x] Load balancer mode - Multi-backend load balancing with round-robin, weighted, least-connections, and latency strategies
- [x] Multiple inference backends - Support for llama-server, Ollama, LocalAI, vLLM, OpenAI-compatible APIs
- [x] Distributed RAG index - Federated search across multiple OffGrid nodes with result merging

---

## Phase 5: Community & Ecosystem

### 5.1 Marketing & Positioning
- [ ] New landing page with clear value prop
- [ ] Comparison table (vs Ollama, LM Studio, LocalAI)
- [ ] Video demo (30 second hero video)
- [ ] Blog posts on use cases

### 5.2 Ecosystem
- [x] Plugin system for custom tools - PluginManager with Python/Node/Shell/Go plugin support
- [x] Theme support for UI - 9 built-in themes (dark, light, midnight, forest, ocean, sunset, solarized, nord, high-contrast)
- [ ] Model format converters
- [ ] Integration guides (n8n, Langchain, etc.)

### 5.3 Community
- [ ] Discord server
- [ ] GitHub Discussions enabled
- [ ] Contributor guide improvements
- [ ] Office hours / community calls

---

## Success Metrics

| Metric | Current | Target (Q2 2025) |
|--------|---------|------------------|
| GitHub Stars | ? | 5,000 |
| PyPI Downloads/month | ? | 10,000 |
| Docker Pulls | ? | 50,000 |
| Time to first chat | ~15 min | < 5 min |
| User NPS | ? | > 50 |

---

## Competitive Advantages to Emphasize

| Feature | OffGrid | Ollama | LM Studio |
|---------|---------|--------|-----------|
| Air-gapped USB deploy | ✅ | ❌ | ❌ |
| P2P model sharing | ✅ | ❌ | ❌ |
| Built-in RAG | ✅ | ❌ | ❌ |
| Multi-user auth | ✅ | ❌ | ❌ |
| AI Agents + MCP | ✅ | ❌ | Partial |
| Voice (STT+TTS) | ✅ | ❌ | ❌ |
| Audit logging | ✅ | ❌ | ❌ |
| Python SDK | ✅ | ✅ | ✅ |
| OpenAI API compat | ✅ | ✅ | ✅ |

---

## Version Milestones

### v0.2.11 - "Core Complete" (Current)
- Model aliases (llama3.2, qwen2.5, etc.)
- GPU auto-detection and layer auto-enable
- P2P CLI command (`offgrid peers`)
- Agent templates (researcher, coder, analyst, creative)
- Audit log export (JSON)
- Model cache with crash detection and auto-restart
- Enhanced onboarding wizard

### v0.3.0 - "Production Ready"
- Bundled llama-server binaries (auto-download)
- Auto-start inference on model select
- P2P discovery UI in web interface
- `offgrid run <model>` with auto-download
- Mock mode removed from all commands

### v0.4.0 - "Enterprise Ready"
- Polished P2P and USB workflows
- Enhanced multi-user (quotas, LDAP)
- API key management UI
- CSV/Excel audit export
- Custom tool creation UI

### v0.5.0 - "Scale Ready"
- Multi-model hot-swap
- Load balancer mode
- Advanced RAG (PDF, web)
- Continuous batching

### v1.0.0 - "General Availability"
- Stable API guarantee
- Comprehensive docs
- Active community
- Enterprise support option

---

## How to Contribute

See [CONTRIBUTING.md](../dev/CONTRIBUTING.md) for development setup.

Priority areas:
1. **llama-server bundling** - Help with cross-compilation
2. **GPU detection** - Platform-specific code
3. **CLI improvements** - `offgrid run` command
4. **Documentation** - User-focused guides
5. **Testing** - Integration tests

---

*This roadmap is a living document. Last updated: December 2025*
