# Release Notes v0.2.8

Version 0.2.8 delivers major performance improvements for model switching, bringing load times from 60+ seconds down to under 1 second for cached models. This release focuses on making OffGrid LLM competitive with Ollama's model switching speed while maintaining zero-configuration simplicity.

## Performance Improvements

### Fast Model Switching
- Model switching for cached models now completes in under 1 second (previously 60+ seconds)
- Pre-warmed models load in 3-5 seconds (previously 60-120 seconds)
- Reduced server startup timeout from 60 seconds to 10 seconds
- Reduced model load timeout from 120 seconds to 30 seconds
- Replaced fixed 500ms port release delay with active port checking

### Auto-Scaling Model Cache
- Model cache size now auto-scales based on available system RAM
- Scans model directory to calculate average model size
- Automatically determines optimal number of models to keep loaded
- Respects explicit user configuration when set
- Reserves 2GB for OS and other processes

### Intelligent Model Pre-Warming
- New mmap-based pre-warming system reads models into OS page cache on startup
- Background warming runs automatically when server starts
- Models are ready faster on first request after restart
- Pre-warming status tracked and reported in cache stats

### Smart Memory Management
- Smart mlock for small models (under 2GB) on systems with sufficient RAM
- Default model protection prevents frequently-used models from eviction
- Memory decisions based on actual system RAM detection

## UI Improvements

### Optimized Model Loading UX
- Added cache status detection before model switching
- Faster polling intervals for cached models (200ms vs 1000ms)
- Proactive model warming when opening chat tab
- Status badge shows "Warming" vs "Loading" based on cache state
- Smoother loading timer updates (100ms intervals)

### Better Feedback
- Shows "Switching to X..." for cached models (instant)
- Shows "Loading X..." for cold loads
- Displays model ready state immediately when cached

## API Enhancements

### Cache Statistics Endpoint
The `/v1/cache/stats` endpoint now includes additional information:
- `mmap_warmer` stats with total warmed models and GB
- `system_ram_mb` for memory-aware decisions
- `mlock_enabled` status
- `default_model` protection status

## Configuration

New configuration options (all enabled by default):

| Option | Default | Description |
|--------|---------|-------------|
| `prewarm_models` | true | Pre-warm models into page cache on startup |
| `smart_mlock` | true | Use mlock for small models on high-RAM systems |
| `protect_default` | true | Prevent default model from cache eviction |
| `fast_switch_mode` | true | Enable all fast-switch optimizations |

Environment variable overrides:
- `OFFGRID_PREWARM_MODELS`
- `OFFGRID_SMART_MLOCK`
- `OFFGRID_PROTECT_DEFAULT`
- `OFFGRID_FAST_SWITCH`

## Technical Details

### New Files
- `internal/inference/mmap_warmer.go` - Model pre-warming system

### Modified Components
- `internal/inference/model_cache.go` - Fast switching, smart mlock, default model protection
- `internal/config/config.go` - New fast-switch configuration options
- `internal/server/server.go` - Auto-scaling cache, pre-warming integration
- `web/ui/js/models.js` - Optimized polling, cache detection, background warming
- `desktop/js/models.js` - Same UI optimizations for desktop app

## Upgrade Notes

This is a drop-in upgrade with no breaking changes. All optimizations are enabled by default and work automatically based on your system's available RAM.

To disable fast-switch features (not recommended):
```bash
export OFFGRID_FAST_SWITCH=false
```

## Installation

### From Binary
```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

### From Docker
```bash
docker pull ghcr.io/takuphilchan/offgrid-llm:v0.2.8
docker run -p 11611:11611 ghcr.io/takuphilchan/offgrid-llm:v0.2.8
```

### From Source
```bash
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
go build -o bin/offgrid ./cmd/offgrid/
./bin/offgrid
```

## Contributors

Thanks to everyone who provided feedback on model switching performance.
