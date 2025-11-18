package inference

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
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
	portPool     []int                     // available ports
	maxInstances int
	mu           sync.RWMutex
	basePort     int
	nextPortIdx  int
}

// NewModelCache creates a new model cache with specified capacity
func NewModelCache(maxInstances int) *ModelCache {
	if maxInstances < 1 {
		maxInstances = 1
	}
	if maxInstances > 10 {
		maxInstances = 10 // Safety limit
	}

	// Create port pool starting from 42382
	portPool := make([]int, maxInstances)
	for i := 0; i < maxInstances; i++ {
		portPool[i] = 42382 + i
	}

	return &ModelCache{
		instances:    make(map[string]*ModelInstance),
		portPool:     portPool,
		maxInstances: maxInstances,
		basePort:     42382,
		nextPortIdx:  0,
	}
}

// GetOrLoad returns an existing model instance or loads a new one
func (mc *ModelCache) GetOrLoad(modelID, modelPath string) (*ModelInstance, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if model is already loaded
	if instance, exists := mc.instances[modelID]; exists {
		instance.LastAccess = time.Now()
		log.Printf("Model %s already cached on port %d", modelID, instance.Port)
		return instance, nil
	}

	// Need to load new model
	if len(mc.instances) >= mc.maxInstances {
		// Cache full - evict least recently used
		if err := mc.evictLRU(); err != nil {
			return nil, fmt.Errorf("failed to evict model: %w", err)
		}
	}

	// Get next available port
	port := mc.getNextPort()

	// Start llama-server with this model
	log.Printf("Loading model %s on port %d (cache: %d/%d)", modelID, port, len(mc.instances)+1, mc.maxInstances)
	cmd := exec.Command("llama-server",
		"-m", modelPath,
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1",
		"-c", "2048",
		"-ngl", "0",
	)

	cmd.Env = append(cmd.Env, "NO_PROXY=*")

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
		cmd.Process.Kill()
		delete(mc.instances, modelID)
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
		instance.Cmd.Wait()
	}

	delete(mc.instances, modelID)
	log.Printf("Model %s unloaded from cache", modelID)
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

// getNextPort returns the next available port from the pool
func (mc *ModelCache) getNextPort() int {
	port := mc.portPool[mc.nextPortIdx]
	mc.nextPortIdx = (mc.nextPortIdx + 1) % len(mc.portPool)
	return port
}

// waitForReady waits for llama-server to be ready on specified port
func (mc *ModelCache) waitForReady(port int) error {
	url := fmt.Sprintf("http://localhost:%d/health", port)

	for i := 0; i < 120; i++ {
		time.Sleep(1 * time.Second)

		resp, err := httpClient.Get(url)
		if err == nil {
			var health map[string]interface{}
			if json.NewDecoder(resp.Body).Decode(&health) == nil {
				resp.Body.Close()
				if status, ok := health["status"].(string); ok && status == "ok" {
					return nil
				}
			}
			resp.Body.Close()
		}
	}

	return fmt.Errorf("llama-server on port %d did not become ready within 120 seconds", port)
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
