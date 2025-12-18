package inference

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

// ModelInstance represents a running llama-server instance with a loaded model
type ModelInstance struct {
	ModelID    string
	Port       int
	Cmd        *exec.Cmd
	LastAccess time.Time
}

// ModelCache manages multiple llama-server instances for fast model switching
type ModelCache struct {
	instances      map[string]*ModelInstance // modelID -> instance
	portToModel    map[int]string            // port -> modelID for reverse lookup
	usedPorts      map[int]bool              // track which ports are in use
	maxInstances   int
	gpuLayers      int // Number of GPU layers to offload
	contextSize    int // Context window size (0 = auto-detect based on RAM)
	batchSize      int // Batch size for inference (lower = faster first token)
	mu             sync.RWMutex
	basePort       int
	binManager     *BinaryManager
	mmapWarmer     *MmapWarmer // Pre-warms models into page cache
	defaultModelID string      // Protected from eviction
	useMlock       bool        // Lock small models in RAM
	modelsDir      string      // Models directory for mmap warming
	totalRAMMB     int64       // System RAM in MB for smart mlock
}

// NewModelCache creates a new model cache with specified capacity
func NewModelCache(maxInstances int, gpuLayers int, binDir string) *ModelCache {
	if maxInstances < 1 {
		maxInstances = 1
	}
	if maxInstances > 10 {
		maxInstances = 10 // Safety limit
	}

	return &ModelCache{
		instances:    make(map[string]*ModelInstance),
		portToModel:  make(map[int]string),
		usedPorts:    make(map[int]bool),
		maxInstances: maxInstances,
		gpuLayers:    gpuLayers,
		contextSize:  0,   // 0 = auto-detect based on available RAM
		batchSize:    256, // Lower batch = faster time-to-first-token
		basePort:     42382,
		binManager:   NewBinaryManager(binDir),
		useMlock:     false, // Disabled by default, enabled for small models
		totalRAMMB:   0,     // Will be set by SetSystemRAM
	}
}

// NewModelCacheWithWarmer creates a model cache with mmap pre-warming support
func NewModelCacheWithWarmer(maxInstances int, gpuLayers int, binDir string, modelsDir string) *ModelCache {
	mc := NewModelCache(maxInstances, gpuLayers, binDir)
	mc.modelsDir = modelsDir
	mc.mmapWarmer = NewMmapWarmer(modelsDir)
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

// PrewarmModel pre-warms a model into the OS page cache for faster loading
func (mc *ModelCache) PrewarmModel(modelPath string) error {
	if mc.mmapWarmer == nil {
		return fmt.Errorf("mmap warmer not initialized")
	}
	_, err := mc.mmapWarmer.WarmModel(modelPath)
	return err
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

// SetUseMlock enables or disables mlock for all models
func (mc *ModelCache) SetUseMlock(enable bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.useMlock = enable
}

// shouldUseMlock determines if mlock should be used for a model
// Smart mlock: enable if system RAM is at least 4x the model size
// This ensures the model stays in RAM without causing swapping
func (mc *ModelCache) shouldUseMlock(modelPath string) bool {
	// If mlock is explicitly enabled, always use it
	if mc.useMlock {
		return true
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

// GetOrLoad returns an existing model instance or loads a new one
func (mc *ModelCache) GetOrLoad(modelID, modelPath, projectorPath string) (*ModelInstance, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if model is already loaded
	if instance, exists := mc.instances[modelID]; exists {
		// Check if process is still alive AND healthy
		isHealthy := false
		if instance.Cmd.Process != nil {
			// First check if process exists
			if err := instance.Cmd.Process.Signal(syscall.Signal(0)); err == nil {
				// Process exists, check if it's responding
				if err := mc.checkHealth(instance.Port); err == nil {
					isHealthy = true
				} else {
					log.Printf("Model %s instance on port %d is unresponsive: %v", modelID, instance.Port, err)
				}
			}
		}

		if isHealthy {
			instance.LastAccess = time.Now()
			log.Printf("Model %s already cached on port %d", modelID, instance.Port)
			return instance, nil
		}

		log.Printf("Model %s instance on port %d is dead or unhealthy. Reloading...", modelID, instance.Port)
		// Clean up dead instance
		// We manually cleanup instead of calling Unload to avoid lock issues if Unload were to change
		if instance.Cmd != nil && instance.Cmd.Process != nil {
			instance.Cmd.Process.Kill()
			instance.Cmd.Wait()
		}
		delete(mc.instances, modelID)
		delete(mc.portToModel, instance.Port)
		delete(mc.usedPorts, instance.Port)
	}

	// Need to load new model
	if len(mc.instances) >= mc.maxInstances {
		// Cache full - evict least recently used
		if err := mc.evictLRU(); err != nil {
			return nil, fmt.Errorf("failed to evict model: %w", err)
		}
	}

	// Get next available port
	port := mc.getNextAvailablePort()

	// Mark port as used
	mc.usedPorts[port] = true
	mc.portToModel[port] = modelID

	// Start llama-server with this model
	log.Printf("Loading model %s on port %d (cache: %d/%d)", modelID, port, len(mc.instances)+1, mc.maxInstances)

	// Determine context size - use configured value or auto-detect
	contextSize := mc.contextSize
	if contextSize <= 0 {
		// Auto-detect based on available RAM
		// Default to 4096, but could be optimized further with runtime detection
		contextSize = 4096
	}

	// Use configured batch size
	batchSize := mc.batchSize
	if batchSize <= 0 {
		batchSize = 256
	}

	// Build optimized args for inference performance
	args := []string{
		"-m", modelPath,
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1",
		"-c", fmt.Sprintf("%d", contextSize), // Adaptive context size
		"-np", "1", // Limit to 1 parallel sequence for stability
		"--no-warmup",                      // Skip warmup to save memory/time on load
		"-b", fmt.Sprintf("%d", batchSize), // Adaptive batch size
		"-nr",                    // Disable weight repacking to save memory
		"--cont-batching",        // Continuous batching for better throughput
		"--cache-type-k", "q8_0", // Quantized KV cache - reduces memory ~50%
		"--cache-type-v", "q8_0", // with minimal quality loss
	}

	// Smart mlock: automatically enable for small models when RAM is sufficient
	// This keeps the model in RAM and prevents swapping, improving switch times
	if mc.useMlock || mc.shouldUseMlock(modelPath) {
		args = append(args, "--mlock")
		log.Printf("Using mlock for model %s (keeping in RAM)", modelID)
	}

	if projectorPath != "" {
		args = append(args, "--mmproj", projectorPath)
	}

	// Add GPU layers if configured
	if mc.gpuLayers > 0 {
		args = append(args, "-ngl", fmt.Sprintf("%d", mc.gpuLayers))
	} else {
		args = append(args, "-ngl", "0")
	}

	log.Printf("Starting llama-server with args: %v", args)

	// Get llama-server binary path
	binaryPath, err := mc.binManager.GetLlamaServer()
	if err != nil {
		return nil, fmt.Errorf("failed to get llama-server binary: %w", err)
	}

	cmd := exec.Command(binaryPath, args...)

	cmd.Env = append(cmd.Env, "NO_PROXY=*")

	// Redirect output to parent process for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start llama-server: %w", err)
	}

	instance := &ModelInstance{
		ModelID:    modelID,
		Port:       port,
		Cmd:        cmd,
		LastAccess: time.Now(),
	}

	mc.instances[modelID] = instance

	// Wait for model to be ready
	if err := mc.waitForReady(port); err != nil {
		// Cleanup on failure
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		delete(mc.instances, modelID)
		delete(mc.portToModel, port)
		delete(mc.usedPorts, port)
		return nil, err
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

	// Kill the llama-server process
	if instance.Cmd != nil && instance.Cmd.Process != nil {
		if err := instance.Cmd.Process.Kill(); err != nil {
			log.Printf("Warning: error killing llama-server for %s: %v", modelID, err)
		}
		// Wait for process to fully terminate
		instance.Cmd.Wait()
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

	for modelID := range mc.instances {
		mc.Unload(modelID)
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
// With mmap pre-warming, models load much faster (5-15s vs 60-120s)
// This prevents blank responses when switching models too quickly
func (mc *ModelCache) waitForReady(port int) error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)

	// Phase 1: Wait for server process to start (up to 10 seconds with pre-warming)
	serverStarted := false
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := httpClient.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			serverStarted = true
			log.Printf("llama-server on port %d started, waiting for model to load...", port)
			break
		}
	}

	if !serverStarted {
		return fmt.Errorf("llama-server on port %d did not start within 10 seconds", port)
	}

	// Phase 2: Wait for model to actually load
	// With mmap pre-warming, this should be fast (5-15s)
	// Check /v1/models endpoint which returns 200 only when model is loaded
	modelsURL := fmt.Sprintf("http://localhost:%d/v1/models", port)

	// Reduced from 120s to 30s - models should be in page cache
	maxAttempts := 30
	for attempt := 0; attempt < maxAttempts; attempt++ {
		time.Sleep(1 * time.Second)

		// Check if process is still running
		instance, exists := mc.instances[mc.portToModel[port]]
		if exists && instance.Cmd.ProcessState != nil && instance.Cmd.ProcessState.Exited() {
			return fmt.Errorf("llama-server on port %d exited unexpectedly", port)
		}

		resp, err := httpClient.Get(modelsURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				log.Printf("Model on port %d is fully loaded and ready", port)
				return nil
			}
		}
	}

	// If we get here, model didn't load in time
	return fmt.Errorf("model on port %d did not load within 120 seconds", port)
}

// checkHealth performs a quick health check on the llama-server instance
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
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
