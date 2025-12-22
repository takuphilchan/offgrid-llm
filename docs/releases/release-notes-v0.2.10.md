# Release Notes v0.2.10

Version 0.2.10 focuses on **stability and performance** improvements, especially for low-end and consumer hardware. This release fixes critical issues with mlock failures, reduces memory pressure, and optimizes hot paths throughout the codebase.

## Stability Fixes

### mlock Safety Check
- Added `canUseMlock()` function that checks `RLIMIT_MEMLOCK` before enabling mlock
- Prevents llama-server crashes on systems with low memory lock limits (common in containers, WSL, default Linux)
- Logs a helpful message when mlock is disabled due to insufficient limits

### Server Startup Reliability
- Increased llama-server startup timeout from 10s to 30s for larger models
- Added warmup request after model load to initialize inference pipeline
- Prevents 503 "slot busy" errors on first real request
- Reduced parallel slots to 1 by default for stability on low-end machines

### Graceful Error Handling
- Stream interruptions (EOF, OOM) now return partial response instead of error
- Exponential backoff retry for 503 errors (500ms → 1s → 2s → 3s cap)
- Better context cancellation handling throughout inference pipeline
- Graceful SIGTERM shutdown before force-killing llama-server processes

### Single-Instance Mode (Optional)
- Set `OFFGRID_MAX_MODELS=1` for low-RAM systems (<8GB)
- Automatic cleanup of existing instances before loading new model
- Port conflict detection and resolution
- Use `OFFGRID_LOW_MEMORY=true` for aggressive memory savings

## Performance Optimizations

### Server-Side
- **VERSION file caching**: Read once at startup instead of per-request disk I/O
- **Cache key generation**: Use `strings.Builder` instead of JSON marshal (~10x faster)
- **Session metadata caching**: New `ListMeta()` function with mod-time validation
- **RAG search**: O(n log k) min-heap for top-k selection instead of O(n²) bubble sort
- **Vector similarity**: 4x loop unrolling in `cosineSimilarity()` function
- **Metrics collection**: `strings.Builder` for efficient string concatenation
- **Rate limiter**: DDoS protection with max 10,000 buckets and LRU eviction
- **Whisper transcription**: Multi-threaded with `-t` flag (up to 4 threads)
- **Mmap warmer**: Adaptive chunk sizes based on available RAM

### Frontend
- **Power polling**: Pause monitoring when browser tab is hidden (saves CPU)
- **Models cache**: 60-second TTL cache to avoid redundant `/models` fetches
- **Model switching**: Non-blocking (removed confirmation modal)
- **JARVIS VAD**: Optimized polling interval (50ms vs 30ms), shorter timeouts

## Configuration Changes

### Default Values Updated
- `BatchSize`: 512 (optimized for throughput)
- `ContBatching`: true (continuous batching enabled)
- `ProtectDefault`: true (default model stays in cache)
- `ModelLoadTimeout`: 30 seconds
- `MaxTokens` slider: max 8192 → 16384

### New Settings
- `SetParallelSlots(n)` - Control parallel inference slots (1-4)
- `SetNumThreads(n)` - CPU thread count (0 = auto-detect)
- `SetKVCacheType(type)` - KV cache quantization (f16, q8_0, q4_0)
- `SetFlashAttention(bool)` - Enable flash attention for GPU
- `SetCacheReuse(tokens)` - KV cache reuse window

## Vision Model Improvements

### New Projector Fallbacks
Added automatic mmproj fallbacks for additional VLM architectures:
- LLaVA v1.6 Mistral 7B
- LLaVA v1.6 Vicuna 7B/13B
- Moondream2
- NanoLLaVA
- BakLLaVA

### Better Error Messages
- Vision adapter errors now explain how to fix (re-download with CLI)
- EOF/interrupted errors suggest reducing model size or context
- 503 errors give helpful "try again" message

## UI/UX Improvements

### JARVIS Mode
- Cleaner state labels without emojis (Ready, Listening, Thinking, etc.)
- Reduced timeouts (60s transcription, 45s chat) for faster failure detection
- Faster VAD response (500ms silence threshold, 4 samples to stop)
- 10s max recording time (faster transcription)

### Chat Interface
- Model switching no longer shows blocking confirmation modal
- Better error messages with markdown formatting and bullet points
- Max tokens slider extended to 16384

## Bug Fixes

- Fixed projector download using wrong repository ID for fallback sources
- Fixed rate limiter memory leak under DDoS conditions
- Fixed cache cleanup goroutine not stoppable (added `StopCleanupRoutine()`)
- Fixed `UnloadAll()` modifying map during iteration
- Fixed health check treating 503 (busy) as dead server
- Fixed start-server.sh not handling Ctrl+Z properly (now disabled)

## Upgrade Notes

For low-RAM systems (<8GB), you can enable single-instance mode:
```bash
export OFFGRID_MAX_MODELS=1
export OFFGRID_LOW_MEMORY=true
```

This prioritizes stability over fast model switching.

---

**Full Changelog**: https://github.com/takuphilchan/offgrid-llm/compare/v0.2.9...v0.2.10
