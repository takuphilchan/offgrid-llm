package inference

import (
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

// ModelInstance represents a running llama-server instance with a loaded model
type ModelInstance struct {
	ModelID       string
	Port          int
	Cmd           *exec.Cmd
	LastAccess    time.Time
	ModelPath     string // For restart purposes
	ProjectorPath string // For VLM restart
}

// ModelCache manages multiple llama-server instances for fast model switching
type ModelCache struct {
	instances      map[string]*ModelInstance // modelID -> instance
	portToModel    map[int]string            // port -> modelID for reverse lookup
	usedPorts      map[int]bool              // track which ports are in use
	maxInstances   int
	gpuLayers      int    // Number of GPU layers to offload
	contextSize    int    // Context window size (0 = auto-detect based on RAM)
	batchSize      int    // Batch size for inference (lower = faster first token)
	parallelSlots  int    // Number of parallel inference slots (-np flag)
	numThreads     int    // Number of CPU threads (0 = auto-detect)
	kvCacheType    string // KV cache quantization: f16, q8_0, q4_0 (reduces VRAM usage)
	flashAttention bool   // Enable flash attention (faster, less VRAM on GPU)
	cacheReuse     int    // KV cache reuse for chat sessions (0 = disabled)
	contBatching   bool   // Enable continuous batching for multi-request throughput
	// Speculative decoding settings
	draftModel     string // Path to draft model for speculative decoding
	draftTokens    int    // Number of draft tokens to generate (default: 8)
	draftMin       int    // Minimum draft tokens for acceptance (default: 5)
	mu             sync.RWMutex
	pendingLoads   map[string]chan error // Deduplicate concurrent load requests
	basePort       int
	binManager     *BinaryManager
	mmapWarmer     *MmapWarmer     // Pre-warms models into page cache
	loadingTracker *LoadingTracker // Tracks loading progress for UI feedback
	defaultModelID string          // Protected from eviction
	useMlock       bool            // Lock small models in RAM
	modelsDir      string          // Models directory for mmap warming
	totalRAMMB     int64           // System RAM in MB for smart mlock
	autoRestart    bool            // Auto-restart crashed processes
	stopMonitor    chan struct{}
}

// NewModelCache creates a new model cache with specified capacity
func NewModelCache(maxInstances int, gpuLayers int, binDir string) *ModelCache {
	if maxInstances < 1 {
		maxInstances = 1
	}
	if maxInstances > 10 {
		maxInstances = 10 // Safety limit
	}

	mc := &ModelCache{
		instances:      make(map[string]*ModelInstance),
		portToModel:    make(map[int]string),
		usedPorts:      make(map[int]bool),
		pendingLoads:   make(map[string]chan error),
		maxInstances:   maxInstances,
		gpuLayers:      gpuLayers,
		contextSize:    0,     // 0 = auto-detect based on available RAM
		batchSize:      256,   // Lower batch = faster time-to-first-token
		parallelSlots:  1,     // 1 slot for stability on low-end machines
		numThreads:     0,     // 0 = auto-detect based on CPU cores
		kvCacheType:    "",    // Empty = use llama.cpp defaults (f16), set to q8_0 if your build supports it
		flashAttention: false, // Disabled by default - can cause crashes on some systems
		cacheReuse:     0,     // Disabled by default - can cause EOF issues
		contBatching:   true,  // Enabled by default for multi-request throughput
		basePort:       42382,
		binManager:     NewBinaryManager(binDir),
		useMlock:       false, // Disabled by default, enabled for small models
		totalRAMMB:     0,     // Will be set by SetSystemRAM
		autoRestart:    true,  // Enable auto-restart by default
		stopMonitor:    make(chan struct{}),
	}

	// Start process monitor
	go mc.monitorProcesses()

	return mc
}

// NewModelCacheWithWarmer creates a model cache with mmap pre-warming support
func NewModelCacheWithWarmer(maxInstances int, gpuLayers int, binDir string, modelsDir string) *ModelCache {
	mc := NewModelCache(maxInstances, gpuLayers, binDir)
	mc.modelsDir = modelsDir
	mc.mmapWarmer = NewMmapWarmer(modelsDir)
	mc.loadingTracker = NewLoadingTracker()
	return mc
}

// SetDefaultModel sets the protected default model that won't be evicted
func (mc *ModelCache) SetDefaultModel(modelID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.defaultModelID = modelID
	log.Printf("Default model set to %s (protected from eviction)", modelID)
}

// SetSystemRAM sets the system RAM for smart mlock decisions
func (mc *ModelCache) SetSystemRAM(ramMB int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.totalRAMMB = ramMB
}

// GetLoadingProgress returns the current model loading progress
func (mc *ModelCache) GetLoadingProgress() *LoadingProgress {
	if mc.loadingTracker == nil {
		return &LoadingProgress{Phase: PhaseIdle}
	}
	return mc.loadingTracker.GetProgress()
}

// GetRecentModels returns recently used models for UI display
func (mc *ModelCache) GetRecentModels(limit int) []ModelHistory {
	if mc.loadingTracker == nil {
		return nil
	}
	return mc.loadingTracker.GetRecentModels(limit)
}

// PrewarmPredicted pre-warms models likely to be used based on history
func (mc *ModelCache) PrewarmPredicted(currentModel string) {
	if mc.loadingTracker == nil || mc.mmapWarmer == nil {
		return
	}

	candidates := mc.loadingTracker.ShouldPrewarm(currentModel)
	if len(candidates) == 0 {
		return
	}

	go func() {
		for _, modelID := range candidates {
			// Find model path (would need registry access)
			log.Printf("Predictive pre-warm candidate: %s", modelID)
		}
	}()
}

// PrewarmModel pre-warms a model into the OS page cache for faster loading
func (mc *ModelCache) PrewarmModel(modelPath string) error {
	if mc.mmapWarmer == nil {
		return fmt.Errorf("mmap warmer not initialized")
	}
	_, err := mc.mmapWarmer.WarmModel(modelPath)
	return err
}

// FastPrewarmModel uses aggressive read-ahead for immediate pre-warming.
// Call this when user hovers over a model or when a switch is imminent.
func (mc *ModelCache) FastPrewarmModel(modelPath string) error {
	if mc.mmapWarmer == nil {
		return fmt.Errorf("mmap warmer not initialized")
	}
	mc.mmapWarmer.WarmOnDemand(modelPath)
	return nil
}

// PrewarmAllModels pre-warms all models in the background
func (mc *ModelCache) PrewarmAllModels() {
	if mc.mmapWarmer == nil {
		log.Println("Warning: mmap warmer not initialized, skipping prewarm")
		return
	}
	go func() {
		_, err := mc.mmapWarmer.WarmAllModels()
		if err != nil {
			log.Printf("Warning: failed to prewarm models: %v", err)
		}
	}()
}

// SetContextSize sets the context window size (0 = auto-detect)
func (mc *ModelCache) SetContextSize(size int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.contextSize = size
}

// SetBatchSize sets the batch size for inference
func (mc *ModelCache) SetBatchSize(size int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if size < 1 {
		size = 256
	}
	mc.batchSize = size
}

// SetMaxInstances limits the number of concurrent model instances
func (mc *ModelCache) SetMaxInstances(max int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if max < 1 {
		max = 1
	}
	mc.maxInstances = max
}

// SetParallelSlots sets the number of parallel inference slots (-np flag)
// Lower values use less memory but can cause 503 errors under concurrent load
func (mc *ModelCache) SetParallelSlots(slots int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if slots < 1 {
		slots = 1
	}
	if slots > 4 {
		slots = 4 // Safety cap
	}
	mc.parallelSlots = slots
}

// SetUseMlock enables or disables mlock for all models
func (mc *ModelCache) SetUseMlock(enable bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.useMlock = enable
}

// SetNumThreads sets the number of CPU threads (0 = auto-detect)
func (mc *ModelCache) SetNumThreads(threads int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if threads < 0 {
		threads = 0
	}
	mc.numThreads = threads
}

// SetKVCacheType sets the KV cache quantization type (f16, q8_0, q4_0)
// q8_0 is recommended for good balance; q4_0 saves more VRAM but slight quality loss
func (mc *ModelCache) SetKVCacheType(cacheType string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	// Validate cache type
	switch cacheType {
	case "f16", "q8_0", "q4_0":
		mc.kvCacheType = cacheType
	default:
		mc.kvCacheType = "q8_0" // Default to q8_0 if invalid
	}
}

// SetFlashAttention enables or disables flash attention
// Flash attention reduces VRAM usage and speeds up inference on GPU
func (mc *ModelCache) SetFlashAttention(enable bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.flashAttention = enable
}

// SetCacheReuse sets the KV cache reuse window for chat sessions
// Higher values = faster follow-up messages in a conversation
func (mc *ModelCache) SetCacheReuse(tokens int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if tokens < 0 {
		tokens = 0
	}
	mc.cacheReuse = tokens
}

// SetContinuousBatching enables or disables continuous batching
// Continuous batching allows efficient handling of multiple concurrent requests
func (mc *ModelCache) SetContinuousBatching(enable bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.contBatching = enable
}

// SetSpeculativeDecoding configures speculative decoding with a draft model
// draftModelPath: path to a smaller model for generating draft tokens
// draftTokens: number of draft tokens to generate (recommended: 4-16)
// draftMin: minimum acceptable draft tokens (recommended: 3-8)
func (mc *ModelCache) SetSpeculativeDecoding(draftModelPath string, draftTokens, draftMin int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.draftModel = draftModelPath
	mc.draftTokens = draftTokens
	mc.draftMin = draftMin
}

// IsModelAlive checks if a specific model's llama-server process is still running
// This is a fast check that doesn't acquire the write lock
func (mc *ModelCache) IsModelAlive(modelID string) bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	instance, exists := mc.instances[modelID]
	if !exists {
		return false
	}

	if instance.Cmd == nil || instance.Cmd.Process == nil {
		return false
	}

	// Signal 0 checks if process exists without actually sending a signal
	if err := instance.Cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return true
}

// shouldUseMlock determines if mlock should be used for a model
// Smart mlock: enable if system RAM is at least 4x the model size
// AND the system has sufficient RLIMIT_MEMLOCK
func (mc *ModelCache) shouldUseMlock(modelPath string) bool {
	// If mlock is explicitly enabled, always use it
	if mc.useMlock {
		return true
	}

	// Check RLIMIT_MEMLOCK - if it's too low, mlock will fail and crash llama-server
	if !canUseMlock() {
		return false
	}

	// Smart mlock: check if we have enough RAM
	if mc.totalRAMMB <= 0 {
		return false // Can't determine, skip mlock
	}

	// Get model file size
	info, err := os.Stat(modelPath)
	if err != nil {
		return false
	}

	modelSizeMB := info.Size() / (1024 * 1024)

	// Use mlock if RAM is at least 4x the model size
	// This leaves room for OS, other processes, and KV cache
	if mc.totalRAMMB >= modelSizeMB*4 {
		log.Printf("Smart mlock: enabling for %s (model: %dMB, RAM: %dMB)",
			modelPath, modelSizeMB, mc.totalRAMMB)
		return true
	}

	return false
}

// autoDetectGPULayers determines optimal GPU layers based on available VRAM
// This enables automatic GPU acceleration on consumer hardware
func (mc *ModelCache) autoDetectGPULayers(modelPath string) int {
	// Try to detect NVIDIA GPU VRAM
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.free", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		// No NVIDIA GPU available
		return 0
	}

	// Parse free VRAM in MB
	freeVRAM, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || freeVRAM < 2000 {
		// Less than 2GB free, don't use GPU
		log.Printf("GPU auto-detect: Only %dMB free VRAM, need 2GB+ for stable GPU inference", freeVRAM)
		return 0
	}

	// Get model file size to estimate layers
	info, err := os.Stat(modelPath)
	if err != nil {
		return 0
	}

	modelSizeMB := info.Size() / (1024 * 1024)

	// Very conservative approach: only use GPU if we have 2x the model size in VRAM
	// This leaves plenty of room for KV cache growth during generation
	requiredVRAM := modelSizeMB * 2
	if freeVRAM < requiredVRAM {
		// Not enough headroom, use partial offload
		// Only offload a portion of layers to leave room for KV cache
		usableForModel := freeVRAM / 2 // Use only half of VRAM for model weights
		layerPercentage := float64(usableForModel) / float64(modelSizeMB)
		estimatedLayers := int(layerPercentage * 24) // Conservative 24 layer estimate

		if estimatedLayers < 8 {
			log.Printf("GPU auto-detect: %dMB VRAM insufficient for stable GPU inference with %dMB model", freeVRAM, modelSizeMB)
			return 0
		}

		log.Printf("GPU auto-detect: Partial offload - %dMB VRAM, %dMB model - using %d layers (conservative)",
			freeVRAM, modelSizeMB, estimatedLayers)
		return estimatedLayers
	}

	// Have 2x headroom, safe to use full GPU
	log.Printf("GPU auto-detect: %dMB VRAM >= 2x model (%dMB) - using all layers",
		freeVRAM, modelSizeMB)
	return 99
}

// GetOrLoad returns an existing model instance or loads a new one
// For low-end machines, this uses a simple single-instance approach:
// 1. If requested model is already loaded and alive -> return it
// 2. If another request is already loading this model -> wait for it
// 3. Otherwise, unload everything and load the new model
func (mc *ModelCache) GetOrLoad(modelID, modelPath, projectorPath string) (*ModelInstance, error) {
	mc.mu.Lock()

	// Check if another request is already loading this model
	if pendingCh, exists := mc.pendingLoads[modelID]; exists {
		mc.mu.Unlock()
		log.Printf("Model %s already loading, waiting for existing request", modelID)
		// Wait for the other request to complete
		err := <-pendingCh
		if err != nil {
			return nil, err
		}
		// Model should now be loaded, get it
		mc.mu.Lock()
		instance, exists := mc.instances[modelID]
		mc.mu.Unlock()
		if exists {
			return instance, nil
		}
		return nil, fmt.Errorf("model %s load completed but instance not found", modelID)
	}

	// Check if model file exists and get its size
	modelInfo, err := os.Stat(modelPath)
	if err != nil {
		mc.mu.Unlock()
		return nil, fmt.Errorf("model file not found: %w", err)
	}
	modelSizeMB := modelInfo.Size() / (1024 * 1024)

	// Check if model will fit in available RAM (with 1GB headroom)
	if mc.totalRAMMB > 0 {
		requiredRAM := modelSizeMB + 1024 // Model + 1GB headroom for KV cache & OS
		if requiredRAM > mc.totalRAMMB {
			mc.mu.Unlock()
			return nil, fmt.Errorf("model too large: requires ~%dMB but only %dMB RAM available (try a smaller model or Q3_K quantization)",
				requiredRAM, mc.totalRAMMB)
		}
	}

	// Check if model is already loaded and process is alive
	if instance, exists := mc.instances[modelID]; exists {
		if instance.Cmd.Process != nil {
			// Simple liveness check - just see if process exists
			if err := instance.Cmd.Process.Signal(syscall.Signal(0)); err == nil {
				// Process is alive - return it
				instance.LastAccess = time.Now()
				log.Printf("Model %s already loaded on port %d", modelID, instance.Port)
				// Mark as ready immediately if already loaded
				if mc.loadingTracker != nil {
					mc.loadingTracker.StartLoading(modelID, modelSizeMB, true)
					mc.loadingTracker.Complete(modelID, true, "")
				}
				mc.mu.Unlock()
				return instance, nil
			}
		}
		// Process is dead, clean it up
		log.Printf("Model %s process died, cleaning up", modelID)
		mc.cleanupInstance(modelID)
	}

	// Mark this model as loading (for deduplication)
	pendingCh := make(chan error, 10) // Buffered to allow multiple waiters
	mc.pendingLoads[modelID] = pendingCh
	mc.mu.Unlock()

	// Ensure we clean up pending state when done
	defer func() {
		mc.mu.Lock()
		delete(mc.pendingLoads, modelID)
		mc.mu.Unlock()
		close(pendingCh)
	}()

	// Do the actual loading (not holding the lock during I/O)
	instance, err := mc.doLoad(modelID, modelPath, projectorPath, modelSizeMB)

	// Broadcast result to waiting requests
	if err != nil {
		// Non-blocking send to all waiters
		select {
		case pendingCh <- err:
		default:
		}
		return nil, err
	}

	// Success - send nil to waiters
	select {
	case pendingCh <- nil:
	default:
	}

	return instance, nil
}

// doLoad performs the actual model loading (called without holding the lock initially)
func (mc *ModelCache) doLoad(modelID, modelPath, projectorPath string, modelSizeMB int64) (*ModelInstance, error) {
	mc.mu.Lock()

	// Check if model is warm (in page cache)
	isWarm := false
	if mc.mmapWarmer != nil {
		stats := mc.mmapWarmer.Stats()
		if warmedCount, ok := stats["warmed_models"].(int); ok {
			isWarm = warmedCount > 0
		}
	}

	// Start loading progress tracking
	if mc.loadingTracker != nil {
		mc.loadingTracker.StartLoading(modelID, modelSizeMB, isWarm)
	}

	// For single-instance mode (maxInstances=1), unload everything first
	// This prevents port conflicts and memory issues on low-end machines
	if mc.maxInstances == 1 && len(mc.instances) > 0 {
		if mc.loadingTracker != nil {
			mc.loadingTracker.UpdatePhase(PhaseUnloading, 5, "Unloading previous model...")
		}
		log.Printf("Single-instance mode: unloading all models before loading %s", modelID)
		for id := range mc.instances {
			mc.cleanupInstance(id)
		}
		// Reduced delay - active port checking is faster
		time.Sleep(200 * time.Millisecond)
	} else if len(mc.instances) >= mc.maxInstances {
		// Multi-instance mode: evict LRU
		if err := mc.evictLRU(); err != nil {
			mc.mu.Unlock()
			return nil, fmt.Errorf("failed to evict model: %w", err)
		}
	}

	// Use a consistent port for single-instance mode
	port := mc.basePort
	if mc.maxInstances > 1 {
		port = mc.getNextAvailablePort()
	}

	// Ensure port is free before starting
	mc.killProcessOnPort(port)

	// Mark port as used
	mc.usedPorts[port] = true
	mc.portToModel[port] = modelID

	// Start llama-server with this model
	log.Printf("Loading model %s on port %d", modelID, port)

	// Determine context size - use configured value or auto-detect based on RAM
	contextSize := mc.contextSize
	if contextSize <= 0 {
		// Auto-detect based on available system RAM and model size
		// Smaller context = faster loading, less memory usage
		if mc.totalRAMMB > 0 {
			if mc.totalRAMMB < 8192 {
				contextSize = 2048 // 2K for systems with <8GB RAM
			} else if mc.totalRAMMB < 16384 {
				// 8-16GB RAM: balance context with model size
				if modelSizeMB > 4096 {
					contextSize = 2048 // Large models: 2K context
				} else {
					contextSize = 4096 // Small/medium models: 4K context
				}
			} else {
				contextSize = 8192 // 16GB+ RAM: 8K context
			}
			log.Printf("Auto-detected context size: %d (RAM: %dMB, model: %dMB)", contextSize, mc.totalRAMMB, modelSizeMB)
		} else {
			contextSize = 4096 // Default fallback
		}
	}

	// Use configured batch size - smaller = faster first token, less memory spikes
	batchSize := mc.batchSize
	if batchSize <= 0 {
		// Auto-scale batch size based on RAM and model size
		if mc.totalRAMMB > 0 && mc.totalRAMMB < 16384 {
			if modelSizeMB > 4096 {
				batchSize = 64 // Very aggressive for large models on 12GB
			} else {
				batchSize = 128 // Standard for medium models
			}
		} else {
			batchSize = 256 // Default for 16GB+ systems
		}
	}

	// Use configured parallel slots
	// Default to 1 for stability - each slot doubles KV cache memory
	// Set to 2 if you need concurrent requests and have enough VRAM
	parallelSlots := mc.parallelSlots
	if parallelSlots <= 0 {
		parallelSlots = 1
	}

	// Build args for inference - prefer stability over optimizations
	args := []string{
		"-m", modelPath,
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1",
		"-c", fmt.Sprintf("%d", contextSize), // Context size from config
		"-np", fmt.Sprintf("%d", parallelSlots), // Parallel slots
		"-b", fmt.Sprintf("%d", batchSize), // Batch size
		"--no-warmup", // Skip token generation warmup
		"-fit", "off", // Skip slow memory fitting (saves 20-30s on startup)
	}

	// Add CPU thread count (critical for performance)
	numThreads := mc.numThreads
	if numThreads <= 0 {
		// Auto-detect: use physical cores (not hyperthreads) for best perf
		numThreads = runtime.NumCPU()
		if numThreads > 8 {
			numThreads = 8 // Cap at 8 to avoid diminishing returns
		}
	}
	args = append(args, "-t", fmt.Sprintf("%d", numThreads))

	// KV cache quantization - enabled by default for 8-16GB RAM systems
	// Reduces KV cache memory by ~50% with minimal quality impact
	kvCacheType := mc.kvCacheType
	if kvCacheType == "" && mc.totalRAMMB > 0 && mc.totalRAMMB < 16384 {
		kvCacheType = "q8_0" // Auto-enable for memory-constrained systems
	}
	if kvCacheType != "" && kvCacheType != "f16" {
		args = append(args, "--cache-type-k", kvCacheType)
		args = append(args, "--cache-type-v", kvCacheType)
		log.Printf("Using KV cache quantization: %s (RAM: %dMB)", kvCacheType, mc.totalRAMMB)
	}

	// Continuous batching for multi-request throughput
	// Enabled by default - allows handling multiple concurrent requests efficiently
	if mc.contBatching {
		args = append(args, "-cb")
	}

	// Speculative decoding for faster inference
	// Uses a smaller draft model to propose tokens, verified in parallel by main model
	if mc.draftModel != "" {
		args = append(args, "--model-draft", mc.draftModel)
		if mc.draftTokens > 0 {
			args = append(args, "--draft", fmt.Sprintf("%d", mc.draftTokens))
		}
		if mc.draftMin > 0 {
			args = append(args, "--draft-min", fmt.Sprintf("%d", mc.draftMin))
		}
		log.Printf("Speculative decoding enabled with draft model: %s", mc.draftModel)
	}

	// Cache reuse - DISABLED by default for stability
	// Can cause issues on some systems
	// mc.cacheReuse is kept at 0 by default

	if projectorPath != "" {
		args = append(args, "--mmproj", projectorPath)
	}

	// Add GPU layers - auto-detect if not explicitly configured
	gpuLayersToUse := mc.gpuLayers
	if gpuLayersToUse == 0 {
		// Auto-detect GPU layers based on available VRAM
		gpuLayersToUse = mc.autoDetectGPULayers(modelPath)
	}
	if gpuLayersToUse > 0 {
		args = append(args, "-ngl", fmt.Sprintf("%d", gpuLayersToUse))
		// Enable flash attention for GPU inference (faster, less VRAM)
		if mc.flashAttention {
			args = append(args, "-fa")
			log.Printf("Flash attention enabled for GPU inference")
		}
		log.Printf("Using %d GPU layers for model %s", gpuLayersToUse, modelID)
	} else {
		args = append(args, "-ngl", "0")
		// For CPU-only, use mlock to keep model in RAM if beneficial
		if mc.useMlock || mc.shouldUseMlock(modelPath) {
			args = append(args, "--mlock")
			log.Printf("Using mlock for model %s (CPU-only, keeping in RAM)", modelID)
		}
	}

	log.Printf("Starting llama-server with args: %v", args)

	// Get llama-server binary path
	binaryPath, err := mc.binManager.GetLlamaServer()
	if err != nil {
		mc.mu.Unlock()
		return nil, fmt.Errorf("failed to get llama-server binary: %w", err)
	}

	cmd := exec.Command(binaryPath, args...)

	cmd.Env = append(cmd.Env, "NO_PROXY=*")

	// Redirect output to parent process for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Update loading tracker - starting server
	if mc.loadingTracker != nil {
		mc.loadingTracker.UpdatePhase(PhaseStarting, 15, "Starting inference server...")
	}

	if err := cmd.Start(); err != nil {
		mc.mu.Unlock()
		if mc.loadingTracker != nil {
			mc.loadingTracker.Complete(modelID, false, err.Error())
		}
		return nil, fmt.Errorf("failed to start llama-server: %w", err)
	}

	instance := &ModelInstance{
		ModelID:       modelID,
		Port:          port,
		Cmd:           cmd,
		LastAccess:    time.Now(),
		ModelPath:     modelPath,
		ProjectorPath: projectorPath,
	}

	mc.instances[modelID] = instance

	// Unlock before waiting (waitForReady does network I/O)
	mc.mu.Unlock()

	// Wait for model to be ready
	if err := mc.waitForReady(port, modelID); err != nil {
		// Cleanup on failure - re-acquire lock
		mc.mu.Lock()
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		delete(mc.instances, modelID)
		delete(mc.portToModel, port)
		delete(mc.usedPorts, port)
		mc.mu.Unlock()

		if mc.loadingTracker != nil {
			mc.loadingTracker.Complete(modelID, false, err.Error())
		}
		return nil, err
	}

	// Complete loading successfully
	if mc.loadingTracker != nil {
		mc.loadingTracker.Complete(modelID, true, "")
	}

	log.Printf("Model %s loaded successfully on port %d", modelID, port)
	return instance, nil
}

// evictLRU removes the least recently used model from cache
// Protected models (default model) will not be evicted
func (mc *ModelCache) evictLRU() error {
	var oldestModel string
	var oldestTime time.Time

	// Find least recently used, excluding protected default model
	for modelID, instance := range mc.instances {
		// Never evict the default model
		if mc.defaultModelID != "" && modelID == mc.defaultModelID {
			continue
		}
		if oldestModel == "" || instance.LastAccess.Before(oldestTime) {
			oldestModel = modelID
			oldestTime = instance.LastAccess
		}
	}

	if oldestModel == "" {
		// All models are protected, try to evict default as last resort
		if mc.defaultModelID != "" {
			log.Printf("Warning: evicting protected default model %s (no other models to evict)", mc.defaultModelID)
			return mc.unloadInternal(mc.defaultModelID)
		}
		return fmt.Errorf("no models to evict")
	}

	log.Printf("Evicting model %s from cache (LRU)", oldestModel)
	return mc.unloadInternal(oldestModel)
}

// Unload removes a specific model from cache
func (mc *ModelCache) Unload(modelID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.unloadInternal(modelID)
}

// unloadInternal removes a model without acquiring lock (for internal use)
func (mc *ModelCache) unloadInternal(modelID string) error {
	instance, exists := mc.instances[modelID]
	if !exists {
		return fmt.Errorf("model %s not in cache", modelID)
	}

	port := instance.Port

	// Kill the llama-server process - try graceful first, then force
	if instance.Cmd != nil && instance.Cmd.Process != nil {
		// First try SIGTERM for graceful shutdown
		if err := instance.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("Warning: error sending SIGTERM to llama-server for %s: %v", modelID, err)
		}

		// Wait briefly for graceful shutdown
		done := make(chan error, 1)
		go func() {
			_, err := instance.Cmd.Process.Wait()
			done <- err
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(2 * time.Second):
			// Force kill if still running
			log.Printf("Force killing llama-server for %s (did not respond to SIGTERM)", modelID)
			if err := instance.Cmd.Process.Kill(); err != nil {
				log.Printf("Warning: error killing llama-server for %s: %v", modelID, err)
			}
			instance.Cmd.Wait()
		}
	}

	// Clean up tracking maps immediately
	delete(mc.instances, modelID)
	delete(mc.portToModel, port)
	delete(mc.usedPorts, port)

	// Wait for port to be released using active checking instead of fixed sleep
	// This is much faster than the old 500ms sleep
	mc.waitForPortRelease(port, 100*time.Millisecond, 5)

	log.Printf("Model %s unloaded from cache (port %d released)", modelID, port)
	return nil
}

// cleanupInstance removes a model instance without acquiring lock (caller must hold lock)
func (mc *ModelCache) cleanupInstance(modelID string) {
	instance, exists := mc.instances[modelID]
	if !exists {
		return
	}

	port := instance.Port

	// Kill the process
	if instance.Cmd != nil && instance.Cmd.Process != nil {
		instance.Cmd.Process.Kill()
		instance.Cmd.Wait()
	}

	// Clean up tracking maps
	delete(mc.instances, modelID)
	delete(mc.portToModel, port)
	delete(mc.usedPorts, port)
}

// killProcessOnPort kills any process using the specified port
func (mc *ModelCache) killProcessOnPort(port int) {
	// Check if port is in use
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err != nil {
		return // Port is free
	}
	conn.Close()

	// Try to kill llama-server on this port using pkill
	log.Printf("Port %d is in use, attempting to free it", port)
	exec.Command("pkill", "-9", "-f", fmt.Sprintf("llama-server.*--port.*%d", port)).Run()
	exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", port)).Run()

	// Wait for port to be released
	mc.waitForPortRelease(port, 200*time.Millisecond, 10)
}

// waitForPortRelease actively checks if a port is available
func (mc *ModelCache) waitForPortRelease(port int, interval time.Duration, maxAttempts int) {
	for i := 0; i < maxAttempts; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err != nil {
			// Port is free
			return
		}
		conn.Close()
		time.Sleep(interval)
	}
}

// UnloadAll removes all models from cache
func (mc *ModelCache) UnloadAll() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Collect model IDs first to avoid modifying map during iteration
	modelIDs := make([]string, 0, len(mc.instances))
	for modelID := range mc.instances {
		modelIDs = append(modelIDs, modelID)
	}

	// Unload each model using internal method (we already hold the lock)
	for _, modelID := range modelIDs {
		mc.unloadInternal(modelID)
	}
}

// getNextAvailablePort finds an available port that's not currently in use
func (mc *ModelCache) getNextAvailablePort() int {
	for i := 0; i < mc.maxInstances; i++ {
		port := mc.basePort + i
		if !mc.usedPorts[port] {
			return port
		}
	}
	// Fallback - reuse first port (shouldn't happen if eviction works)
	return mc.basePort
}

// waitForReady waits for llama-server to start AND for the model to fully load
// Uses adaptive polling: fast at start, slower as time passes
// With mmap pre-warming, models load much faster (5-15s vs 60-120s)
// This prevents blank responses when switching models too quickly
func (mc *ModelCache) waitForReady(port int, modelID string) error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)

	// Pause background warming during active model loading to avoid I/O contention
	if mc.mmapWarmer != nil {
		mc.mmapWarmer.Pause()
		defer mc.mmapWarmer.Resume()
	}

	// Phase 1: Wait for server process to start (up to 15 seconds)
	// Fast 200ms polling for quick detection
	serverStarted := false
	startupDeadline := time.Now().Add(15 * time.Second)
	attempt := 0

	for time.Now().Before(startupDeadline) {
		// Update progress: 15-40% during server startup
		if mc.loadingTracker != nil {
			elapsed := time.Since(startupDeadline.Add(-15 * time.Second))
			progress := 15 + int(elapsed.Seconds()*25/15)
			if progress > 40 {
				progress = 40
			}
			mc.loadingTracker.UpdatePhase(PhaseStarting, progress, "Starting inference server...")
		}

		time.Sleep(200 * time.Millisecond)
		resp, err := httpClient.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			serverStarted = true
			log.Printf("llama-server on port %d started after %d attempts", port, attempt+1)
			break
		}
		attempt++
	}

	if !serverStarted {
		return fmt.Errorf("llama-server on port %d did not start within 15 seconds", port)
	}

	// Update tracker - server started, now loading model
	if mc.loadingTracker != nil {
		mc.loadingTracker.UpdatePhase(PhaseLoading, 40, "Loading model weights...")
	}

	// Phase 2: Wait for model to actually load
	// Fast fixed polling for responsive loading detection
	// With mmap pre-warming, this should be fast (5-15s)
	modelsURL := fmt.Sprintf("http://localhost:%d/v1/models", port)

	loadDeadline := time.Now().Add(300 * time.Second) // 5 minute max for large models
	loadStart := time.Now()

	for time.Now().Before(loadDeadline) {
		// Update progress: 40-95% during model loading (scaled to elapsed time)
		if mc.loadingTracker != nil {
			elapsed := time.Since(loadStart)
			// Use asymptotic progress: approaches 95% but never reaches it during loading
			// This gives more realistic feedback for slow loads
			progress := 40 + int(55*(1-math.Exp(-elapsed.Seconds()/60)))
			if progress > 95 {
				progress = 95
			}
			mc.loadingTracker.UpdatePhase(PhaseLoading, progress, "Loading model weights...")
		}

		// Fast 500ms polling - balance between responsiveness and CPU
		time.Sleep(500 * time.Millisecond)

		// Check if process is still running
		instance, exists := mc.instances[mc.portToModel[port]]
		if exists && instance.Cmd.ProcessState != nil && instance.Cmd.ProcessState.Exited() {
			return fmt.Errorf("llama-server on port %d exited unexpectedly", port)
		}

		resp, err := httpClient.Get(modelsURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				loadTime := time.Since(loadStart).Seconds()
				log.Printf("Model on port %d is fully loaded and ready (took %.1fs)", port, loadTime)

				// Skip warmup phase - model is ready to serve immediately
				if mc.loadingTracker != nil {
					mc.loadingTracker.UpdatePhase(PhaseReady, 100, "Model ready")
				}

				// Warmup in background - first user request may be slightly slower
				go func(p int) {
					if err := mc.warmupServer(p); err != nil {
						log.Printf("Background warmup (non-fatal): %v", err)
					}
				}(port)
				return nil
			}
		}
	}

	// If we get here, model didn't load in time
	return fmt.Errorf("model on port %d did not load within 5 minutes", port)
}

// checkHealth performs a quick health check on the llama-server instance
// Returns nil if the server is alive (even if busy with 503)
func (mc *ModelCache) checkHealth(port int) error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)

	// Create a client with a very short timeout for health checks
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 200 = healthy and ready
	// 503 = busy (all slots in use) - server is alive but busy, NOT dead
	// Only actual errors (connection refused, timeout) mean the server is dead
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable {
		return nil // Server is alive (busy is not dead)
	}

	return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
}

// warmupServer sends a minimal request to initialize the inference pipeline
// This prevents 503 errors on the first real request
func (mc *ModelCache) warmupServer(port int) error {
	url := fmt.Sprintf("http://localhost:%d/v1/chat/completions", port)

	// Very minimal request - just 1 token
	payload := `{"messages":[{"role":"user","content":"hi"}],"max_tokens":1,"temperature":0}`

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read and discard body
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusOK {
		log.Printf("Warmup request successful on port %d", port)
		return nil
	}

	return fmt.Errorf("warmup request returned status %d", resp.StatusCode)
}

// GetStats returns cache statistics
func (mc *ModelCache) GetStats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	models := make([]map[string]interface{}, 0, len(mc.instances))
	for modelID, instance := range mc.instances {
		models = append(models, map[string]interface{}{
			"model_id":    modelID,
			"port":        instance.Port,
			"last_access": instance.LastAccess,
		})
	}

	stats := map[string]interface{}{
		"max_instances": mc.maxInstances,
		"current_count": len(mc.instances),
		"cached_models": models,
		"default_model": mc.defaultModelID,
		"mlock_enabled": mc.useMlock,
		"system_ram_mb": mc.totalRAMMB,
	}

	// Add mmap warmer stats if available
	if mc.mmapWarmer != nil {
		stats["mmap_warmer"] = mc.mmapWarmer.Stats()
	}

	return stats
}

// monitorProcesses periodically checks if llama-server processes are alive
// and restarts them if they crash
func (mc *ModelCache) monitorProcesses() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopMonitor:
			return
		case <-ticker.C:
			mc.checkAndRestartCrashed()
		}
	}
}

// checkAndRestartCrashed checks all instances and restarts any that have crashed
func (mc *ModelCache) checkAndRestartCrashed() {
	mc.mu.Lock()

	if !mc.autoRestart {
		mc.mu.Unlock()
		return
	}

	// Collect crashed models
	var crashed []struct {
		modelID       string
		modelPath     string
		projectorPath string
	}

	for modelID, instance := range mc.instances {
		if instance.Cmd == nil || instance.Cmd.Process == nil {
			continue
		}

		// Check if process is still running by sending signal 0
		err := instance.Cmd.Process.Signal(syscall.Signal(0))
		if err != nil {
			// Process has crashed
			log.Printf("Detected crashed llama-server for model %s, will restart...", modelID)
			crashed = append(crashed, struct {
				modelID       string
				modelPath     string
				projectorPath string
			}{modelID, instance.ModelPath, instance.ProjectorPath})

			// Clean up the crashed instance
			mc.cleanupInstance(modelID)
		}
	}
	mc.mu.Unlock()

	// Restart crashed models outside of lock
	for _, c := range crashed {
		log.Printf("Restarting model %s...", c.modelID)
		_, err := mc.GetOrLoad(c.modelID, c.modelPath, c.projectorPath)
		if err != nil {
			log.Printf("Failed to restart model %s: %v", c.modelID, err)
		} else {
			log.Printf("Successfully restarted model %s", c.modelID)
		}
	}
}

// SetAutoRestart enables or disables automatic restart of crashed processes
func (mc *ModelCache) SetAutoRestart(enable bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.autoRestart = enable
}

// StopMonitor stops the process monitor goroutine
func (mc *ModelCache) StopMonitor() {
	close(mc.stopMonitor)
}

// HotSwapResult contains the result of a hot-swap operation
type HotSwapResult struct {
	Success      bool   `json:"success"`
	FromModel    string `json:"from_model,omitempty"`
	ToModel      string `json:"to_model"`
	SwapTimeMS   int64  `json:"swap_time_ms"`
	Method       string `json:"method"` // "preloaded", "warm", "cold"
	Port         int    `json:"port"`
	ErrorMessage string `json:"error,omitempty"`
}

// HotSwap performs a fast model switch with minimal downtime
// It uses multiple strategies to minimize load time:
// 1. If model is already loaded (preloaded), returns immediately
// 2. If model is in mmap cache (warm), loads faster
// 3. Otherwise performs cold load with optimizations
func (mc *ModelCache) HotSwap(toModelID, toModelPath, toProjectorPath string) (*HotSwapResult, error) {
	startTime := time.Now()
	result := &HotSwapResult{
		ToModel: toModelID,
	}

	mc.mu.Lock()

	// Check what's currently loaded
	var currentModelID string
	for id := range mc.instances {
		currentModelID = id
		break
	}
	result.FromModel = currentModelID

	// Strategy 1: Already loaded - instant return
	if instance, exists := mc.instances[toModelID]; exists {
		if instance.Cmd != nil && instance.Cmd.Process != nil {
			if err := instance.Cmd.Process.Signal(syscall.Signal(0)); err == nil {
				instance.LastAccess = time.Now()
				result.Success = true
				result.Method = "preloaded"
				result.SwapTimeMS = time.Since(startTime).Milliseconds()
				result.Port = instance.Port
				mc.mu.Unlock()
				log.Printf("Hot-swap: %s already loaded (instant)", toModelID)
				return result, nil
			}
		}
		// Process dead, clean up
		mc.cleanupInstance(toModelID)
	}
	mc.mu.Unlock()

	// Strategy 2: Check if model is in mmap cache (warm)
	isWarm := false
	if mc.mmapWarmer != nil {
		stats := mc.mmapWarmer.Stats()
		if warmedModels, ok := stats["models"].([]string); ok {
			for _, path := range warmedModels {
				if path == toModelPath {
					isWarm = true
					break
				}
			}
		}
	}

	if isWarm {
		result.Method = "warm"
		log.Printf("Hot-swap: %s in mmap cache (warm load)", toModelID)
	} else {
		result.Method = "cold"
		// Pre-warm in background for faster subsequent loads
		if mc.mmapWarmer != nil {
			go mc.mmapWarmer.WarmModel(toModelPath)
		}
		log.Printf("Hot-swap: %s cold load (no cache)", toModelID)
	}

	// Perform the actual load
	instance, err := mc.GetOrLoad(toModelID, toModelPath, toProjectorPath)
	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	result.Success = true
	result.SwapTimeMS = time.Since(startTime).Milliseconds()
	result.Port = instance.Port

	log.Printf("Hot-swap complete: %s -> %s in %dms (%s)",
		result.FromModel, toModelID, result.SwapTimeMS, result.Method)

	return result, nil
}

// PrepareHotSwap pre-warms a model for faster hot-swap later
// Call this ahead of time when you know you'll need a model soon
func (mc *ModelCache) PrepareHotSwap(modelPath string) error {
	if mc.mmapWarmer == nil {
		return fmt.Errorf("mmap warmer not initialized")
	}

	log.Printf("Preparing model for hot-swap: %s", modelPath)
	_, err := mc.mmapWarmer.WarmModel(modelPath)
	return err
}

// GetHotSwapStatus returns information about hot-swap readiness for models
func (mc *ModelCache) GetHotSwapStatus() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Currently loaded models
	loaded := make([]string, 0, len(mc.instances))
	for id := range mc.instances {
		loaded = append(loaded, id)
	}

	// Warm models (in mmap cache)
	var warm []string
	if mc.mmapWarmer != nil {
		stats := mc.mmapWarmer.Stats()
		if warmedModels, ok := stats["models"].([]string); ok {
			warm = warmedModels
		}
	}

	return map[string]interface{}{
		"loaded_models":  loaded,
		"warm_models":    warm,
		"max_instances":  mc.maxInstances,
		"current_count":  len(mc.instances),
		"hot_swap_ready": len(warm) > 0 || mc.mmapWarmer != nil,
	}
}
