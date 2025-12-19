# Release Notes v0.2.9

Version 0.2.9 focuses on performance optimizations for consumer hardware, making OffGrid LLM faster and more memory-efficient on typical desktop and laptop systems. This release also improves the installer experience and adds performance tuning options to the UI.

## Performance Optimizations

### Flash Attention
- Enabled flash attention (`-fa on`) for faster inference on supported hardware
- Reduces memory bandwidth requirements during attention computation
- Improves tokens/second on consumer GPUs

### KV Cache Quantization
- KV cache now uses q8_0 quantization (`--cache-type-k q8_0 --cache-type-v q8_0`)
- Reduces memory usage by ~50% with minimal quality impact
- Allows larger context windows on memory-constrained systems

### Cache Reuse
- Added `--cache-reuse 256` for faster follow-up responses
- Reuses KV cache entries from previous requests
- Significantly improves multi-turn conversation latency

### GPU Auto-Detection
- Automatically detects NVIDIA GPU via `nvidia-smi`
- Sets optimal `--n-gpu-layers` based on available VRAM
- Falls back to CPU gracefully when GPU unavailable

### Physical Core Detection
- Detects physical cores (not hyperthreads) from `/proc/cpuinfo`
- Sets optimal thread count for inference
- Avoids performance degradation from hyperthread contention

## UI Improvements

### Performance Mode Selector
- New dropdown in chat interface: Balanced / Fast / Quality
- **Fast**: Lower temperature, fewer tokens, aggressive caching
- **Balanced**: Default settings for most use cases
- **Quality**: Higher temperature, more tokens, better responses
- Settings persist across sessions

### Cache Status Indicator
- Shows current model cache state: Cold / Warm / Hot
- Updates when switching models or warming cache
- Helps users understand expected response latency

### Increased Defaults
- Default max_tokens increased from 1024 to 2048
- Better for longer responses without manual adjustment

## Python Client Improvements

### Cache Management
- `client.cache_stats()` - Get current cache statistics
- `client.warm_model(model_id)` - Pre-warm a model for faster first response
- `client.is_model_cached(model_id)` - Check if model is in cache

### Connection Optimization
- Added HTTP keep-alive for persistent connections
- Reduces connection overhead for multiple requests
- New `keep_alive` parameter (default: True)

### Example Script
- New `python/examples/performance.py` demonstrating cache warming and performance monitoring

## Installer Improvements

### Redesigned Interface
- Installer now matches CLI visual theme
- Uses same icons and formatting as `offgrid` commands
- Clean, professional appearance without emojis
- Proper section headers with `◈` symbol
- Checkmarks `✓` for selected components
- Arrow `→` for info messages

### Better Feedback
- Shows system info (OS, CPU features, version)
- Component selection with visual checkboxes
- Download progress with timing
- Clear next steps after installation

## Upgrade Notes

This is a drop-in upgrade from v0.2.8. No configuration changes required.

The new performance optimizations are enabled automatically. If you experience issues:
- Flash attention can be disabled by setting `OFFGRID_NO_FA=1`
- KV cache quantization can be disabled with `OFFGRID_KV_F16=1`

## What's Next

- v0.3.0 will focus on multi-model orchestration
- Improved RAG pipeline performance
- WebSocket streaming improvements
