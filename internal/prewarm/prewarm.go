// Package prewarm provides model pre-warming on boot for faster first-request response.
// Essential for edge deployments where cold-start latency is unacceptable.
package prewarm

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// WarmupConfig configures model pre-warming
type WarmupConfig struct {
	ModelsDir    string        `json:"models_dir"`
	DefaultModel string        `json:"default_model"`
	WarmupModels []string      `json:"warmup_models"` // Models to pre-warm (in priority order)
	Timeout      time.Duration `json:"timeout"`
	MaxModels    int           `json:"max_models"`   // Max models to warm (memory limit)
	MemoryLimit  int64         `json:"memory_limit"` // Max memory for warmed models (bytes)
}

// DefaultWarmupConfig returns sensible defaults
func DefaultWarmupConfig(dataDir string) WarmupConfig {
	return WarmupConfig{
		ModelsDir:   filepath.Join(dataDir, "models"),
		Timeout:     5 * time.Minute,
		MaxModels:   1,                      // Only warm default model by default
		MemoryLimit: 2 * 1024 * 1024 * 1024, // 2GB
	}
}

// WarmupStatus represents the status of model warmup
type WarmupStatus struct {
	ModelName string        `json:"model_name"`
	Status    string        `json:"status"` // "warming", "ready", "failed", "skipped"
	Duration  time.Duration `json:"duration,omitempty"`
	Error     string        `json:"error,omitempty"`
	MemoryMB  int64         `json:"memory_mb,omitempty"`
}

// WarmupResult contains the results of pre-warming
type WarmupResult struct {
	StartTime     time.Time      `json:"start_time"`
	EndTime       time.Time      `json:"end_time"`
	TotalDuration time.Duration  `json:"total_duration"`
	Models        []WarmupStatus `json:"models"`
	Ready         int            `json:"ready"`
	Failed        int            `json:"failed"`
	Skipped       int            `json:"skipped"`
}

// ModelLoader interface for loading models
type ModelLoader interface {
	LoadModel(modelPath string) error
	UnloadModel(modelName string) error
	IsModelLoaded(modelName string) bool
	GetLoadedModels() []string
	GetModelMemory(modelName string) int64
}

// Prewarmer handles model pre-warming
type Prewarmer struct {
	config WarmupConfig
	loader ModelLoader
	result *WarmupResult
	mu     sync.RWMutex
}

// NewPrewarmer creates a new prewarmer
func NewPrewarmer(config WarmupConfig, loader ModelLoader) *Prewarmer {
	return &Prewarmer{
		config: config,
		loader: loader,
	}
}

// WarmDefault warms only the default model
func (p *Prewarmer) WarmDefault(ctx context.Context) (*WarmupResult, error) {
	if p.config.DefaultModel == "" {
		return nil, fmt.Errorf("no default model configured")
	}

	return p.WarmModels(ctx, []string{p.config.DefaultModel})
}

// WarmModels pre-warms specified models
func (p *Prewarmer) WarmModels(ctx context.Context, models []string) (*WarmupResult, error) {
	result := &WarmupResult{
		StartTime: time.Now(),
		Models:    make([]WarmupStatus, 0, len(models)),
	}

	p.mu.Lock()
	p.result = result
	p.mu.Unlock()

	var totalMemory int64

	for i, modelName := range models {
		// Check context
		select {
		case <-ctx.Done():
			result.EndTime = time.Now()
			result.TotalDuration = result.EndTime.Sub(result.StartTime)
			return result, ctx.Err()
		default:
		}

		// Check limits
		if p.config.MaxModels > 0 && i >= p.config.MaxModels {
			result.Models = append(result.Models, WarmupStatus{
				ModelName: modelName,
				Status:    "skipped",
				Error:     "max models limit reached",
			})
			result.Skipped++
			continue
		}

		// Find model file
		modelPath := p.findModel(modelName)
		if modelPath == "" {
			result.Models = append(result.Models, WarmupStatus{
				ModelName: modelName,
				Status:    "failed",
				Error:     "model not found",
			})
			result.Failed++
			continue
		}

		// Estimate memory
		info, err := os.Stat(modelPath)
		if err == nil {
			estimatedMemory := info.Size() / 2 // Rough estimate
			if p.config.MemoryLimit > 0 && totalMemory+estimatedMemory > p.config.MemoryLimit {
				result.Models = append(result.Models, WarmupStatus{
					ModelName: modelName,
					Status:    "skipped",
					Error:     "memory limit would be exceeded",
				})
				result.Skipped++
				continue
			}
		}

		// Check if already loaded
		if p.loader.IsModelLoaded(modelName) {
			result.Models = append(result.Models, WarmupStatus{
				ModelName: modelName,
				Status:    "ready",
				Error:     "already loaded",
			})
			result.Ready++
			continue
		}

		// Load model
		status := WarmupStatus{
			ModelName: modelName,
			Status:    "warming",
		}

		log.Printf("Pre-warming model: %s", modelName)
		warmStart := time.Now()

		err = p.loader.LoadModel(modelPath)
		status.Duration = time.Since(warmStart)

		if err != nil {
			status.Status = "failed"
			status.Error = err.Error()
			result.Failed++
			log.Printf("Failed to warm %s: %v", modelName, err)
		} else {
			status.Status = "ready"
			status.MemoryMB = p.loader.GetModelMemory(modelName) / (1024 * 1024)
			totalMemory += status.MemoryMB * 1024 * 1024
			result.Ready++
			log.Printf("Model %s warmed in %v", modelName, status.Duration)
		}

		result.Models = append(result.Models, status)
	}

	result.EndTime = time.Now()
	result.TotalDuration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// WarmMostUsed pre-warms the most recently/frequently used models
func (p *Prewarmer) WarmMostUsed(ctx context.Context, usageStats map[string]int) (*WarmupResult, error) {
	// Sort models by usage
	type modelUsage struct {
		name  string
		count int
	}
	var models []modelUsage
	for name, count := range usageStats {
		models = append(models, modelUsage{name, count})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].count > models[j].count
	})

	// Get top N models
	var toWarm []string
	for i := 0; i < p.config.MaxModels && i < len(models); i++ {
		toWarm = append(toWarm, models[i].name)
	}

	if len(toWarm) == 0 && p.config.DefaultModel != "" {
		toWarm = []string{p.config.DefaultModel}
	}

	return p.WarmModels(ctx, toWarm)
}

// WarmAll pre-warms all available models (careful with memory!)
func (p *Prewarmer) WarmAll(ctx context.Context) (*WarmupResult, error) {
	models, err := p.listModels()
	if err != nil {
		return nil, err
	}

	// Put default model first
	if p.config.DefaultModel != "" {
		var reordered []string
		reordered = append(reordered, p.config.DefaultModel)
		for _, m := range models {
			if m != p.config.DefaultModel {
				reordered = append(reordered, m)
			}
		}
		models = reordered
	}

	return p.WarmModels(ctx, models)
}

// GetStatus returns the current warmup status
func (p *Prewarmer) GetStatus() *WarmupResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.result
}

// findModel finds a model file by name
func (p *Prewarmer) findModel(name string) string {
	// Check common extensions
	extensions := []string{".gguf", ".GGUF", ""}

	for _, ext := range extensions {
		path := filepath.Join(p.config.ModelsDir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Search in subdirectories
	var found string
	filepath.Walk(p.config.ModelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, name) && strings.HasSuffix(strings.ToLower(base), ".gguf") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})

	return found
}

// listModels lists all available models
func (p *Prewarmer) listModels() ([]string, error) {
	var models []string

	err := filepath.Walk(p.config.ModelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			models = append(models, strings.TrimSuffix(info.Name(), filepath.Ext(info.Name())))
		}
		return nil
	})

	return models, err
}

// BootWarmup runs pre-warming as part of system boot
func BootWarmup(ctx context.Context, config WarmupConfig, loader ModelLoader) error {
	log.Println("Starting boot-time model pre-warming...")

	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	prewarmer := NewPrewarmer(config, loader)

	var result *WarmupResult
	var err error

	if len(config.WarmupModels) > 0 {
		result, err = prewarmer.WarmModels(ctx, config.WarmupModels)
	} else if config.DefaultModel != "" {
		result, err = prewarmer.WarmDefault(ctx)
	} else {
		log.Println(" No models configured for pre-warming")
		return nil
	}

	if err != nil {
		log.Printf("Pre-warming failed: %v", err)
		return err
	}

	log.Printf("Pre-warming complete: %d ready, %d failed, %d skipped in %v",
		result.Ready, result.Failed, result.Skipped, result.TotalDuration)

	return nil
}
