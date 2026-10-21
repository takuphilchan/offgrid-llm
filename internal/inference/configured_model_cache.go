package inference

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

// NewConfiguredModelCache creates a model cache with performance settings from config.
func NewConfiguredModelCache(cfg *config.Config, resources *resource.SystemResources) *ModelCache {
	binDir := filepath.Join(cfg.ModelsDir, "..", "bin")

	// Never raise the configured residency limit from weight-file sizes alone:
	// KV caches, compute buffers, embeddings and other applications also need RAM.
	maxModels := cfg.MaxModels
	// Low memory mode is opt-in via config only
	// Auto-detection was removed as it can cause more problems than it solves
	lowMemoryMode := cfg.LowMemoryMode

	// Use the new warmer-enabled cache for faster model switching
	cache := NewModelCacheWithWarmer(maxModels, cfg.NumGPULayers, binDir, cfg.ModelsDir)
	// Runtime restarts must go through the lifecycle gate, including profiles.
	cache.SetAutoRestart(false)

	// Always set system RAM for auto-detection of context size
	if resources != nil {
		cache.SetSystemRAM(resources.TotalRAM)
	}

	// Apply performance tuning from config
	cache.SetContextSize(cfg.MaxContextSize)
	cache.SetBatchSize(cfg.BatchSize)
	cache.SetNumThreads(cfg.NumThreads)
	cache.SetGPUEnabled(cfg.EnableGPU)
	cache.SetKVCacheType(cfg.KVCacheType)
	cache.SetFlashAttention(cfg.FlashAttention)
	cache.SetContinuousBatching(cfg.ContBatching)
	cache.SetParallelSlots(cfg.InferenceSlots)

	// Configure speculative decoding if draft model specified
	if cfg.SpecDraftModel != "" {
		draftTokens := cfg.SpecDraftTokens
		if draftTokens <= 0 {
			draftTokens = 8
		}
		draftMin := cfg.SpecDraftMin
		if draftMin <= 0 {
			draftMin = 5
		}
		cache.SetSpeculativeDecoding(cfg.SpecDraftModel, draftTokens, draftMin)
		log.Printf("Speculative decoding configured with draft model: %s", cfg.SpecDraftModel)
	}

	// Apply low memory mode optimizations only when explicitly enabled
	if lowMemoryMode {
		log.Println("Low memory mode enabled (explicit): single model, reduced batch, 1 parallel slot, q4_0 KV cache")
		cache.SetBatchSize(128)      // Smaller batches
		cache.SetMaxInstances(1)     // Force single instance
		cache.SetParallelSlots(1)    // Single slot to reduce KV cache memory
		cache.SetKVCacheType("q4_0") // Max memory savings in low memory mode
		cache.SetCacheReuse(0)       // Disable cache reuse to save memory
		// Note: Context size is NOT reduced - let user control this via max_context_size config
	}

	// Set default model protection (won't be evicted from cache)
	if cfg.DefaultModel != "" && cfg.ProtectDefault {
		cache.SetDefaultModel(cfg.DefaultModel)
	}

	// Configure smart mlock based on system RAM
	if cfg.SmartMlock || cfg.UseMlock {
		if resources != nil {
			log.Printf("System RAM detected: %d MB (smart mlock enabled)", resources.TotalRAM)
		}
		if cfg.UseMlock {
			cache.SetUseMlock(true)
		}
	}

	return cache
}

// estimateAverageModelSize scans the models directory and returns average model size in MB
func estimateAverageModelSize(modelsDir string) int64 {
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return 2048 // Default assumption: 2GB per model
	}

	var totalSize int64
	var count int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".gguf") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sizeMB := info.Size() / (1024 * 1024)
		totalSize += sizeMB
		count++
	}

	if count == 0 {
		return 2048 // Default: 2GB if no models found
	}

	avgSize := totalSize / count

	// Add 20% overhead for runtime memory usage
	return avgSize * 120 / 100
}
