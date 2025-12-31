# Release Notes v0.2.12

**Release Date:** December 31, 2025

Version 0.2.12 is a **UX polish and stability update** focused on **model switching reliability**, **preventing zombie processes**, and **improved dropdown consistency**. This release ensures robust model management even under rapid user interactions.

## Highlights

- **Request Deduplication** - Backend prevents duplicate model load requests, eliminating zombie processes
- **Rate-Limited Model Switching** - Frontend enforces 1-second cooldown between model switches
- **Fixed Progress Stuck at 90%** - Loading progress now uses asymptotic formula for smooth 0-100% progression
- **Restored Fast Polling** - Fixed polling intervals (200ms startup, 500ms loading) for responsive feedback
- **Consistent Model Dropdowns** - All dropdowns sorted consistently with model sizes displayed
- **Loading Modal with Progress Bar** - Visual feedback during model switching operations

---

## Stability Improvements

### Request Deduplication (Backend)

The model cache now tracks pending load requests to prevent duplicate operations:

```go
// If a model is already being loaded, wait for that load instead of starting a new one
type ModelCache struct {
    pendingLoads map[string]chan error  // Track in-flight load requests
}
```

**Benefits:**
- Rapid model switching no longer spawns zombie llama-server processes
- Memory usage stays controlled during user interactions
- Only one load operation runs per model at a time

### Rate-Limited Model Switching (Frontend)

Frontend now enforces a 1-second cooldown between model switch requests:

```javascript
// ModelManager with rate limiting
_switchCooldown: 1000,  // 1 second between switches
_lastSwitchTime: 0,
_cleanupPendingLoad()   // Cleanup SSE/intervals on new switch
```

**Benefits:**
- Prevents UI spam from creating multiple backend requests
- Cleans up previous SSE connections and polling intervals
- Provides clear feedback when switching too fast

---

## Bug Fixes

### Progress Stuck at 90%

**Problem:** Linear progress calculation capped at 90%, never reaching 100%

**Solution:** Asymptotic formula that smoothly approaches 100%:

```javascript
// Before: progress = Math.min(90, baseProgress)
// After: progress = 40 + 55 * (1 - Math.exp(-elapsed / 60))
```

The progress now starts at 40%, curves toward 95% over ~3 minutes of loading, ensuring users always see meaningful progress.

### Slow Adaptive Polling Removed

**Problem:** Adaptive polling intervals (500ms → 2000ms) made model loading feel slow

**Solution:** Restored fixed fast polling:
- Server startup: **200ms** (fixed)
- Model loading: **500ms** (fixed)

---

## UI Improvements

### Consistent Model Dropdowns

All model dropdown menus across the application now display:

1. **Consistent Sorting:**
   - LLM models: Larger models first, then alphabetically
   - Embedding models: Smaller models first, then alphabetically

2. **Size Display:**
   - Each option shows the model size: `Llama-3.2-3B (4.37 GB)`
   - Helps users quickly identify model resource requirements

```javascript
// Sorting logic
models.sort((a, b) => {
    // LLM first, then embeddings
    if (isLLM(a) !== isLLM(b)) return isLLM(b) - isLLM(a);
    // LLMs: larger first; Embeddings: smaller first
    if (isLLM(a)) return (b.size || 0) - (a.size || 0);
    return (a.size || 0) - (b.size || 0);
});
```

### Loading Modal with Progress Bar

When switching models, a modal now displays:
- Current operation status
- Progress bar with percentage
- Model name being loaded

---

## Technical Changes

### Files Modified

**Backend:**
- `internal/inference/model_cache.go` - Added `pendingLoads` map and `doLoad()` for request deduplication

**Frontend:**
- `web/ui/js/model-manager.js` - Rate limiting, cleanup helpers, sorted dropdowns with sizes
- `web/ui/js/models.js` - Legacy fallback loaders updated with same sorting/size logic
- `web/ui/js/chat-ui.js` - Loading modal and progress bar integration
- `web/ui/js/utils.js` - Helper functions for size formatting

---

## Upgrade Notes

This is a seamless upgrade from v0.2.11. No configuration changes required.

### For Developers

If you've customized model switching behavior:
- The `ModelManager` now has `_switchCooldown` (1000ms default)
- `_cleanupPendingLoad()` is called automatically on new switch requests
- Model dropdowns expect `size` (bytes) or `size_gb` (string) fields from API

---

## What's Next

- **v0.2.13** - Focus on agent improvements and MCP enhancements
- **v0.3.0** - Major release with distributed inference and advanced RAG

---

**Full Changelog:** https://github.com/takuphilchan/offgrid-llm/compare/v0.2.11...v0.2.12
