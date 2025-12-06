package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LoRAAdapter represents a LoRA adapter configuration
type LoRAAdapter struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Path        string         `json:"path"`
	Scale       float32        `json:"scale"`                // Adapter strength (0.0 - 1.0)
	BaseModel   string         `json:"base_model,omitempty"` // Compatible base model
	CreatedAt   time.Time      `json:"created_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// LoRAAdapterStatus represents the status of a loaded adapter
type LoRAAdapterStatus struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Loaded   bool       `json:"loaded"`
	Scale    float32    `json:"scale"`
	LoadedAt *time.Time `json:"loaded_at,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// LoRAManager manages LoRA adapters for hot-loading
type LoRAManager struct {
	mu          sync.RWMutex
	adapters    map[string]*LoRAAdapter       // ID -> Adapter
	loaded      map[string]*LoRAAdapterStatus // ID -> Status
	activeStack []string                      // Stack of active adapter IDs (order matters)
	dataDir     string
	engine      Engine
	llamaServer *LlamaServerClient // For llama-server based loading
}

// LlamaServerClient represents a connection to llama-server for LoRA operations
type LlamaServerClient struct {
	BaseURL string
}

// NewLoRAManager creates a new LoRA manager
func NewLoRAManager(dataDir string, engine Engine) *LoRAManager {
	mgr := &LoRAManager{
		adapters:    make(map[string]*LoRAAdapter),
		loaded:      make(map[string]*LoRAAdapterStatus),
		activeStack: make([]string, 0),
		dataDir:     dataDir,
		engine:      engine,
	}
	mgr.load()
	return mgr
}

// SetLlamaServer sets the llama-server client for server-based LoRA loading
func (m *LoRAManager) SetLlamaServer(baseURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llamaServer = &LlamaServerClient{BaseURL: baseURL}
}

// RegisterAdapter registers a new LoRA adapter
func (m *LoRAManager) RegisterAdapter(id, name, path string, scale float32, description string) (*LoRAAdapter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("adapter file not found: %s", path)
	}

	// Verify it's a valid LoRA file
	if !isValidLoRAFile(path) {
		return nil, fmt.Errorf("invalid LoRA file format: %s", path)
	}

	if id == "" {
		id = generateAdapterID()
	}

	adapter := &LoRAAdapter{
		ID:          id,
		Name:        name,
		Description: description,
		Path:        path,
		Scale:       scale,
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]any),
	}

	m.adapters[id] = adapter
	m.save()

	return adapter, nil
}

// GetAdapter gets an adapter by ID
func (m *LoRAManager) GetAdapter(id string) (*LoRAAdapter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	adapter, ok := m.adapters[id]
	return adapter, ok
}

// ListAdapters lists all registered adapters
func (m *LoRAManager) ListAdapters() []*LoRAAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LoRAAdapter, 0, len(m.adapters))
	for _, adapter := range m.adapters {
		result = append(result, adapter)
	}
	return result
}

// ListLoadedAdapters lists all currently loaded adapters
func (m *LoRAManager) ListLoadedAdapters() []*LoRAAdapterStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LoRAAdapterStatus, 0, len(m.loaded))
	for _, status := range m.loaded {
		if status.Loaded {
			result = append(result, status)
		}
	}
	return result
}

// LoadAdapter hot-loads a LoRA adapter
func (m *LoRAManager) LoadAdapter(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adapter, ok := m.adapters[id]
	if !ok {
		return fmt.Errorf("adapter not found: %s", id)
	}

	// Check if already loaded
	if status, ok := m.loaded[id]; ok && status.Loaded {
		return nil // Already loaded
	}

	// Load the adapter based on backend
	if err := m.loadAdapterInternal(ctx, adapter); err != nil {
		m.loaded[id] = &LoRAAdapterStatus{
			ID:     id,
			Name:   adapter.Name,
			Loaded: false,
			Scale:  adapter.Scale,
			Error:  err.Error(),
		}
		return err
	}

	now := time.Now()
	m.loaded[id] = &LoRAAdapterStatus{
		ID:       id,
		Name:     adapter.Name,
		Loaded:   true,
		Scale:    adapter.Scale,
		LoadedAt: &now,
	}

	// Add to active stack
	m.activeStack = append(m.activeStack, id)

	return nil
}

// loadAdapterInternal loads an adapter using the appropriate backend
func (m *LoRAManager) loadAdapterInternal(ctx context.Context, adapter *LoRAAdapter) error {
	// If using llama-server, use the API
	if m.llamaServer != nil {
		return m.loadViaLlamaServer(ctx, adapter)
	}

	// For native llama.cpp binding, we need to reload the model with LoRA
	// Note: This depends on the specific llama.cpp Go bindings being used
	// Most bindings require specifying LoRA at load time, not hot-loading

	// For now, we'll return an error if native binding is used
	// as hot-loading requires specific llama.cpp API support
	return fmt.Errorf("hot-loading requires llama-server backend; native binding requires model reload")
}

// loadViaLlamaServer loads a LoRA adapter via llama-server API
func (m *LoRAManager) loadViaLlamaServer(ctx context.Context, adapter *LoRAAdapter) error {
	// llama-server supports LoRA loading via the /lora endpoint
	// This is a simplified implementation

	// Note: llama-server must be started with --lora-init-without-apply
	// to allow dynamic LoRA loading

	// The actual API depends on llama-server version
	// For newer versions, we'd use:
	// POST /lora with body: {"path": adapter.Path, "scale": adapter.Scale}

	return fmt.Errorf("llama-server LoRA API not yet implemented - see llama.cpp documentation")
}

// UnloadAdapter unloads a LoRA adapter
func (m *LoRAManager) UnloadAdapter(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.loaded[id]
	if !ok || !status.Loaded {
		return nil // Already unloaded
	}

	// Unload based on backend
	if m.llamaServer != nil {
		// Call llama-server to unload
		// This would be: DELETE /lora/{id}
	}

	status.Loaded = false
	status.LoadedAt = nil

	// Remove from active stack
	newStack := make([]string, 0)
	for _, aid := range m.activeStack {
		if aid != id {
			newStack = append(newStack, aid)
		}
	}
	m.activeStack = newStack

	return nil
}

// SetAdapterScale updates the scale/strength of a loaded adapter
func (m *LoRAManager) SetAdapterScale(ctx context.Context, id string, scale float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scale < 0 || scale > 1 {
		return fmt.Errorf("scale must be between 0 and 1")
	}

	adapter, ok := m.adapters[id]
	if !ok {
		return fmt.Errorf("adapter not found: %s", id)
	}

	adapter.Scale = scale

	// Update loaded status if loaded
	if status, ok := m.loaded[id]; ok && status.Loaded {
		status.Scale = scale

		// If using llama-server, update via API
		if m.llamaServer != nil {
			// PATCH /lora/{id} with {"scale": scale}
		}
	}

	m.save()
	return nil
}

// DeleteAdapter removes an adapter registration
func (m *LoRAManager) DeleteAdapter(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.adapters[id]; !ok {
		return fmt.Errorf("adapter not found: %s", id)
	}

	// Unload if loaded
	if status, ok := m.loaded[id]; ok && status.Loaded {
		return fmt.Errorf("adapter is currently loaded; unload first")
	}

	delete(m.adapters, id)
	delete(m.loaded, id)
	m.save()

	return nil
}

// GetActiveAdapters returns the currently active adapter IDs in stack order
func (m *LoRAManager) GetActiveAdapters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, len(m.activeStack))
	copy(result, m.activeStack)
	return result
}

// GetStatus gets the full status of all adapters
func (m *LoRAManager) GetStatus() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registered := make([]map[string]any, 0, len(m.adapters))
	for _, adapter := range m.adapters {
		status := m.loaded[adapter.ID]
		isLoaded := status != nil && status.Loaded

		info := map[string]any{
			"id":         adapter.ID,
			"name":       adapter.Name,
			"path":       adapter.Path,
			"scale":      adapter.Scale,
			"loaded":     isLoaded,
			"created_at": adapter.CreatedAt,
		}
		if adapter.Description != "" {
			info["description"] = adapter.Description
		}
		if status != nil && status.Error != "" {
			info["error"] = status.Error
		}
		if status != nil && status.LoadedAt != nil {
			info["loaded_at"] = *status.LoadedAt
		}
		registered = append(registered, info)
	}

	return map[string]any{
		"adapters":       registered,
		"active_stack":   m.activeStack,
		"total_adapters": len(m.adapters),
		"loaded_count":   len(m.activeStack),
	}
}

// save persists adapters to disk
func (m *LoRAManager) save() {
	if m.dataDir == "" {
		return
	}

	data, err := json.MarshalIndent(m.adapters, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(m.dataDir, "lora_adapters.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0644)
}

// load loads adapters from disk
func (m *LoRAManager) load() {
	if m.dataDir == "" {
		return
	}

	path := filepath.Join(m.dataDir, "lora_adapters.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	json.Unmarshal(data, &m.adapters)
}

// isValidLoRAFile checks if a file appears to be a valid LoRA adapter
func isValidLoRAFile(path string) bool {
	ext := filepath.Ext(path)
	// Common LoRA file extensions
	validExtensions := map[string]bool{
		".gguf":        true, // GGUF format LoRA
		".ggml":        true, // Legacy GGML format
		".bin":         true, // Binary format
		".safetensors": true, // SafeTensors format
	}
	return validExtensions[ext]
}

// generateAdapterID generates a unique adapter ID
func generateAdapterID() string {
	b := make([]byte, 8)
	if _, err := os.ReadFile("/dev/urandom"); err == nil {
		f, _ := os.Open("/dev/urandom")
		f.Read(b)
		f.Close()
	}
	return fmt.Sprintf("lora-%x", b)
}

// LoRAConfig represents the LoRA configuration for model loading
type LoRAConfig struct {
	Adapters []LoRAAdapterConfig `json:"adapters"`
}

// LoRAAdapterConfig is the config for a single adapter in a load request
type LoRAAdapterConfig struct {
	Path  string  `json:"path"`
	Scale float32 `json:"scale"`
}

// ToLoadArgs converts the LoRA config to command-line arguments for llama-server
func (c *LoRAConfig) ToLoadArgs() []string {
	args := make([]string, 0)
	for _, adapter := range c.Adapters {
		args = append(args, "--lora", adapter.Path)
		if adapter.Scale != 1.0 {
			args = append(args, "--lora-scaled", fmt.Sprintf("%.2f", adapter.Scale))
		}
	}
	return args
}

// GetActiveLoRAConfig gets the current LoRA configuration from active adapters
func (m *LoRAManager) GetActiveLoRAConfig() *LoRAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]LoRAAdapterConfig, 0, len(m.activeStack))
	for _, id := range m.activeStack {
		if adapter, ok := m.adapters[id]; ok {
			if status, ok := m.loaded[id]; ok && status.Loaded {
				configs = append(configs, LoRAAdapterConfig{
					Path:  adapter.Path,
					Scale: adapter.Scale,
				})
			}
		}
	}

	return &LoRAConfig{Adapters: configs}
}
