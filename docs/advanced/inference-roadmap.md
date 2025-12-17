# Inference Status / TODO (December 2025)

This document explains the current inference modes in OffGrid LLM and the remaining work to make **real inference** the default, production-grade experience.

## Current State

OffGrid LLM supports two inference paths:

1. **Mock Mode**
   - Used for development and UI/API testing.
   - Returns pre-programmed responses.
   - Implemented in `internal/inference/mock.go`.

2. **llama.cpp HTTP Proxy Mode (Recommended for Real Inference)**
   - OffGrid proxies OpenAI-compatible requests to a local `llama-server` process.
   - Implemented in `internal/inference/llama_http.go`.
   - OffGrid itself provides the OpenAI-compatible API surface and UX; `llama-server` does the actual token generation.

For setup instructions, see the llama.cpp integration guide:
- `docs/advanced/LLAMA_CPP_SETUP.md`

## TODO: Make Real Inference the Default

### 1) Reliable `llama-server` lifecycle management

- Automatically start/stop `llama-server` when the user selects/loads a model.
- Detect crashes and restart safely.
- Support clean shutdown and port reuse.

### 2) Model switching and load/unload correctness

- Ensure the system can switch models predictably (including concurrent requests).
- Track which model is actually loaded and expose accurate status to the UI and stats endpoints.

### 3) Streaming parity

- Ensure streaming (`stream=true`) behaves consistently across engines.
- Keep OpenAI-compatible Server-Sent Events (SSE) format stable.

### 4) Compatibility and packaging

- Provide prebuilt `llama-server` binaries per OS/arch where possible.
- Document CPU feature requirements (AVX/AVX2) and fallbacks.

## Known Pitfalls (Design Notes)

- `llama-server` typically serves **one model per process**; OffGrid should avoid sending the model name through to the backend when it can confuse the server.
- Inference can return 503 while a model loads; callers should retry for a bounded time.

## How to Help

If you’re contributing, the best starting points are:
- `internal/inference/llama_http.go`
- `internal/server/server.go` (chat completions + streaming)
- `internal/models/registry.go` (accurate model load/unload tracking)
