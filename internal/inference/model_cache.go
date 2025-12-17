package inference

import (
	"fmt"
	"log"
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
	instances    map[string]*ModelInstance // modelID -> instance
	portToModel  map[int]string            // port -> modelID for reverse lookup
	usedPorts    map[int]bool              // track which ports are in use
	maxInstances int
	gpuLayers    int // Number of GPU layers to offload
	contextSize  int // Context window size (0 = auto-detect based on RAM)
	batchSize    int // Batch size for inference (lower = faster first token)
	mu           sync.RWMutex
	basePort     int
	binManager   *BinaryManager
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
	}
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
func (mc *ModelCache) evictLRU() error {
	var oldestModel string
	var oldestTime time.Time

	// Find least recently used
	for modelID, instance := range mc.instances {
		if oldestModel == "" || instance.LastAccess.Before(oldestTime) {
			oldestModel = modelID
			oldestTime = instance.LastAccess
		}
	}

	if oldestModel == "" {
		return fmt.Errorf("no models to evict")
	}

	log.Printf("Evicting model %s from cache (LRU)", oldestModel)
	return mc.Unload(oldestModel)
}

// Unload removes a specific model from cache
func (mc *ModelCache) Unload(modelID string) error {
	instance, exists := mc.instances[modelID]
	if !exists {
		return fmt.Errorf("model %s not in cache", modelID)
	}

	// Kill the llama-server process
	if instance.Cmd != nil && instance.Cmd.Process != nil {
		if err := instance.Cmd.Process.Kill(); err != nil {
			log.Printf("Warning: error killing llama-server for %s: %v", modelID, err)
		}
		// Wait for process to fully terminate
		instance.Cmd.Wait()
		// Give port time to be released
		time.Sleep(500 * time.Millisecond)
	}

	// Clean up tracking maps
	delete(mc.instances, modelID)
	delete(mc.portToModel, instance.Port)
	delete(mc.usedPorts, instance.Port)

	log.Printf("Model %s unloaded from cache (port %d released)", modelID, instance.Port)
	return nil
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
// This prevents blank responses when switching models too quickly
func (mc *ModelCache) waitForReady(port int) error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)

	// Phase 1: Wait for server process to start (up to 30 seconds for VLM models)
	serverStarted := false
	for i := 0; i < 60; i++ {
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
		return fmt.Errorf("llama-server on port %d did not start within 30 seconds", port)
	}

	// Phase 2: Wait for model to actually load (up to 120 seconds total)
	// Check /v1/models endpoint which returns 200 only when model is loaded
	modelsURL := fmt.Sprintf("http://localhost:%d/v1/models", port)

	maxAttempts := 120 // 120 seconds for model loading (increased for CPU/slow disks)
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

	return map[string]interface{}{
		"max_instances": mc.maxInstances,
		"current_count": len(mc.instances),
		"cached_models": models,
	}
}
