package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Registry manages available models
type Registry struct {
	mu           sync.RWMutex
	models       map[string]*api.ModelMetadata
	modelsDir    string
	loadedModels map[string]interface{} // actual loaded model instances
}

// NewRegistry creates a new model registry
func NewRegistry(modelsDir string) *Registry {
	return &Registry{
		models:       make(map[string]*api.ModelMetadata),
		modelsDir:    modelsDir,
		loadedModels: make(map[string]interface{}),
	}
}

// ScanModels scans the models directory for available models
func (r *Registry) ScanModels() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing models to get fresh state
	r.models = make(map[string]*api.ModelMetadata)

	// Ensure models directory exists
	if err := os.MkdirAll(r.modelsDir, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	// Scan for .gguf files
	return filepath.Walk(r.modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip temporary download files
		if strings.HasSuffix(path, ".tmp") {
			return nil
		}

		// Check for supported model formats
		ext := filepath.Ext(path)
		if ext == ".gguf" || ext == ".ggml" || ext == ".bin" {
			modelID := r.generateModelID(path)
			modelType := r.detectModelType(modelID, path)
			metadata := &api.ModelMetadata{
				ID:           modelID,
				Name:         filepath.Base(path),
				Path:         path,
				Size:         info.Size(),
				Format:       ext[1:], // Remove the dot
				Quantization: r.detectQuantization(path),
				ContextSize:  4096, // Default, will be updated when loaded
				Parameters:   r.detectParameters(path),
				Type:         modelType,
				IsLoaded:     false,
			}

			// Check for projector
			projectorFilename := GetProjectorFilename(modelID, filepath.Base(path))
			if projectorFilename != "" {
				projectorPath := filepath.Join(filepath.Dir(path), projectorFilename)
				if _, err := os.Stat(projectorPath); err == nil {
					metadata.ProjectorPath = projectorPath
				}
			}

			r.models[modelID] = metadata
		}

		return nil
	})
}

// ListModels returns all available models
func (r *Registry) ListModels() []api.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]api.Model, 0, len(r.models))
	for _, meta := range r.models {
		// Format size in GB
		sizeGB := fmt.Sprintf("%.2f GB", float64(meta.Size)/(1024*1024*1024))
		if meta.Size < 1024*1024*1024 {
			sizeGB = fmt.Sprintf("%.0f MB", float64(meta.Size)/(1024*1024))
		}

		models = append(models, api.Model{
			ID:      meta.ID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "offgrid-llm",
			Type:    meta.Type,
			Size:    meta.Size,
			SizeGB:  sizeGB,
		})
	}

	return models
}

// CountLoadedModels returns how many models are currently marked as loaded.
func (r *Registry) CountLoadedModels() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, meta := range r.models {
		if meta != nil && meta.IsLoaded {
			count++
		}
	}
	return count
}

// DeleteModel removes a model from the registry and deletes the file
func (r *Registry) DeleteModel(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the model
	meta, exists := r.models[modelID]
	if !exists {
		return fmt.Errorf("model %s not found in registry (available: %v)", modelID, r.getModelIDs())
	}

	// Delete the file
	fmt.Printf("Attempting to delete file: %s\n", meta.Path)
	if err := os.Remove(meta.Path); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("File already deleted: %s\n", meta.Path)
		} else {
			return fmt.Errorf("failed to delete model file %s: %w", meta.Path, err)
		}
	} else {
		fmt.Printf("Successfully deleted file: %s\n", meta.Path)
	}

	// Remove from registry
	delete(r.models, modelID)
	delete(r.loadedModels, modelID)

	return nil
}

func (r *Registry) getModelIDs() []string {
	ids := make([]string, 0, len(r.models))
	for id := range r.models {
		ids = append(ids, id)
	}
	return ids
}

// GetModel retrieves a model by ID
func (r *Registry) GetModel(id string) (*api.ModelMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[id]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", id)
	}

	return model, nil
}

// LoadModel loads a model into memory
func (r *Registry) LoadModel(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[id]
	if !exists {
		return fmt.Errorf("model not found: %s", id)
	}

	if model.IsLoaded {
		return nil // Already loaded
	}

	// TODO: Actual model loading with llama.cpp
	// For now, just mark as loaded
	model.IsLoaded = true
	model.LoadedAt = time.Now()

	return nil
}

// UnloadModel unloads a model from memory
func (r *Registry) UnloadModel(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[id]
	if !exists {
		return fmt.Errorf("model not found: %s", id)
	}

	if !model.IsLoaded {
		return nil // Not loaded
	}

	// TODO: Actual model unloading
	delete(r.loadedModels, id)
	model.IsLoaded = false

	return nil
}

// ImportFromUSB imports a model from a USB drive or external storage
func (r *Registry) ImportFromUSB(sourcePath, modelName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Verify source file exists
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

	// Copy to models directory
	destPath := filepath.Join(r.modelsDir, modelName)

	// Read source
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write model: %w", err)
	}

	// Re-scan to pick up the new model
	return r.ScanModels()
}

// SaveRegistry saves the registry to disk
func (r *Registry) SaveRegistry(path string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := json.MarshalIndent(r.models, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadRegistry loads the registry from disk
func (r *Registry) LoadRegistry(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No registry file yet, that's OK
		}
		return err
	}

	return json.Unmarshal(data, &r.models)
}

// Helper functions

func (r *Registry) generateModelID(path string) string {
	// Simple ID generation from filename
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}

func (r *Registry) detectQuantization(path string) string {
	name := filepath.Base(path)

	// Common quantization patterns (uppercase, specific first)
	quantizations := []string{
		"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L",
		"Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M",
		"Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M",
		"Q6_K", "Q8_0", "F16", "F32",
		"IQ1_S", "IQ1_M", "IQ2_XXS", "IQ2_XS", "IQ2_S", "IQ2_M",
		"IQ3_XXS", "IQ3_XS", "IQ3_S", "IQ3_M",
		"IQ4_XS", "IQ4_NL",
	}

	for _, quant := range quantizations {
		if contains(name, quant) {
			return quant
		}
	}

	// Handle simplified quant names (e.g., Microsoft's "q4" naming)
	lower := strings.ToLower(name)
	base := strings.TrimSuffix(lower, ".gguf")

	// Check for simplified patterns
	type simplifiedPattern struct {
		suffix string
		quant  string
	}
	simplifiedPatterns := []simplifiedPattern{
		// Check longer patterns first (more specific)
		{"-q4_0", "Q4_0"}, {"_q4_0", "Q4_0"}, {".q4_0", "Q4_0"},
		{"-q4_1", "Q4_1"}, {"_q4_1", "Q4_1"}, {".q4_1", "Q4_1"},
		{"-q5_0", "Q5_0"}, {"_q5_0", "Q5_0"}, {".q5_0", "Q5_0"},
		{"-q5_1", "Q5_1"}, {"_q5_1", "Q5_1"}, {".q5_1", "Q5_1"},
		{"-q8_0", "Q8_0"}, {"_q8_0", "Q8_0"}, {".q8_0", "Q8_0"},
		// Then simpler patterns
		{"-q2", "Q2_K"}, {"_q2", "Q2_K"}, {".q2", "Q2_K"},
		{"-q3", "Q3_K_M"}, {"_q3", "Q3_K_M"}, {".q3", "Q3_K_M"},
		{"-q4", "Q4_K_M"}, {"_q4", "Q4_K_M"}, {".q4", "Q4_K_M"},
		{"-q5", "Q5_K_M"}, {"_q5", "Q5_K_M"}, {".q5", "Q5_K_M"},
		{"-q6", "Q6_K"}, {"_q6", "Q6_K"}, {".q6", "Q6_K"},
		{"-q8", "Q8_0"}, {"_q8", "Q8_0"}, {".q8", "Q8_0"},
	}
	for _, p := range simplifiedPatterns {
		if strings.HasSuffix(base, p.suffix) {
			return p.quant
		}
		if strings.Contains(base, p.suffix+"-") || strings.Contains(base, p.suffix+"_") || strings.Contains(base, p.suffix+".") {
			return p.quant
		}
	}

	return "unknown"
}

func (r *Registry) detectParameters(path string) string {
	name := filepath.Base(path)

	// Common parameter sizes
	sizes := []string{"7B", "13B", "30B", "65B", "70B", "1B", "3B"}

	for _, size := range sizes {
		if contains(name, size) {
			return size
		}
	}

	return "unknown"
}

func (r *Registry) detectModelType(modelID, path string) string {
	name := strings.ToLower(filepath.Base(path))
	modelIDLower := strings.ToLower(modelID)

	// Check against catalog first
	catalog := DefaultCatalog()
	for _, entry := range catalog.Models {
		if strings.ToLower(entry.ID) == modelIDLower {
			if entry.Type != "" {
				return entry.Type
			}
		}
	}

	// Detect based on filename patterns
	embeddingKeywords := []string{"embed", "bge", "e5", "minilm", "nomic", "sentence", "gte"}
	for _, keyword := range embeddingKeywords {
		if strings.Contains(name, keyword) {
			return "embedding"
		}
	}

	// Detect VLM (Vision Language Models)
	vlmKeywords := []string{"llava", "bakllava", "yi-vl", "moondream", "vision", "minicpm-v", "qwen-vl", "mmproj"}
	for _, keyword := range vlmKeywords {
		if strings.Contains(name, keyword) {
			return "vlm"
		}
	}

	// Default to LLM
	return "llm"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
