package inference

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// EmbeddingEngine handles text embedding generation
type EmbeddingEngine struct {
	mu             sync.RWMutex
	modelPath      string
	loaded         bool
	dimensions     int
	maxBatchSize   int
	implementation EmbeddingImpl // Platform-specific implementation
}

// EmbeddingImpl is the interface for platform-specific embedding implementations
type EmbeddingImpl interface {
	Load(modelPath string, opts EmbeddingOptions) error
	Embed(texts []string) ([][]float32, error)
	Unload() error
	GetDimensions() int
}

// EmbeddingOptions contains options for loading embedding models
type EmbeddingOptions struct {
	NumThreads    int    // Number of CPU threads
	NumGPULayers  int    // Number of layers to offload to GPU
	UseMmap       bool   // Use memory mapping
	UseMlock      bool   // Lock model in RAM
	ContextSize   int    // Context size (for longer inputs)
	NormalizeL2   bool   // Normalize embeddings to unit length
	PoolingMethod string // "mean", "cls", or "last" token pooling
}

// DefaultEmbeddingOptions returns sensible defaults optimized for low-end hardware
func DefaultEmbeddingOptions() EmbeddingOptions {
	return EmbeddingOptions{
		NumThreads:    0,      // 0 = auto-detect based on CPU cores
		NumGPULayers:  0,      // CPU by default
		UseMmap:       true,   // Memory-map for lower RAM usage
		UseMlock:      false,  // Don't lock RAM (safer for low-end systems)
		ContextSize:   512,    // Embeddings don't need large context
		NormalizeL2:   true,   // Standard for embeddings
		PoolingMethod: "mean", // Mean pooling is most common
	}
}

// NewEmbeddingEngine creates a new embedding engine
func NewEmbeddingEngine() *EmbeddingEngine {
	return &EmbeddingEngine{
		loaded:       false,
		maxBatchSize: 32, // Process up to 32 texts at once
	}
}

// Load loads an embedding model
func (e *EmbeddingEngine) Load(ctx context.Context, modelPath string, opts EmbeddingOptions) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.loaded {
		if err := e.unloadUnsafe(); err != nil {
			return fmt.Errorf("failed to unload previous model: %w", err)
		}
	}

	// Create platform-specific implementation
	impl, err := newEmbeddingImpl()
	if err != nil {
		return fmt.Errorf("failed to create embedding implementation: %w", err)
	}

	// Load the model
	if err := impl.Load(modelPath, opts); err != nil {
		return fmt.Errorf("failed to load embedding model: %w", err)
	}

	e.implementation = impl
	e.modelPath = modelPath
	e.dimensions = impl.GetDimensions()
	e.loaded = true

	return nil
}

// Unload unloads the current embedding model
func (e *EmbeddingEngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.unloadUnsafe()
}

func (e *EmbeddingEngine) unloadUnsafe() error {
	if !e.loaded {
		return nil
	}

	if e.implementation != nil {
		if err := e.implementation.Unload(); err != nil {
			return err
		}
		e.implementation = nil
	}

	e.loaded = false
	e.modelPath = ""
	e.dimensions = 0

	return nil
}

// IsLoaded returns whether a model is loaded
func (e *EmbeddingEngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// GetDimensions returns the embedding dimension size
func (e *EmbeddingEngine) GetDimensions() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.dimensions
}

// GenerateEmbeddings generates embeddings for the given request
func (e *EmbeddingEngine) GenerateEmbeddings(ctx context.Context, req *api.EmbeddingRequest) (*api.EmbeddingResponse, error) {
	e.mu.RLock()
	if !e.loaded {
		e.mu.RUnlock()
		return nil, fmt.Errorf("no embedding model loaded")
	}
	impl := e.implementation
	e.mu.RUnlock()

	// Parse input (can be string or []string)
	texts, err := e.parseInput(req.Input)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no input texts provided")
	}

	// Check dimension override
	if req.Dimensions != nil && *req.Dimensions != e.dimensions {
		return nil, fmt.Errorf("dimension override not supported for this model (requested %d, model has %d)",
			*req.Dimensions, e.dimensions)
	}

	// Process in batches
	allEmbeddings := make([][]float32, 0, len(texts))
	totalTokens := 0

	for i := 0; i < len(texts); i += e.maxBatchSize {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		end := i + e.maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]

		// Generate embeddings for this batch
		embeddings, err := impl.Embed(batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings: %w", err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)

		// Estimate tokens (rough approximation: ~1 token per 4 chars)
		for _, text := range batch {
			totalTokens += len(text) / 4
		}
	}

	// Build response
	response := &api.EmbeddingResponse{
		Object: "list",
		Model:  req.Model,
		Data:   make([]api.EmbeddingData, len(allEmbeddings)),
		Usage: api.EmbeddingUsage{
			PromptTokens: totalTokens,
			TotalTokens:  totalTokens,
		},
	}

	for i, embedding := range allEmbeddings {
		response.Data[i] = api.EmbeddingData{
			Object:    "embedding",
			Embedding: embedding,
			Index:     i,
		}
	}

	return response, nil
}

// parseInput converts the input (string or []string) to a slice of strings
func (e *EmbeddingEngine) parseInput(input interface{}) ([]string, error) {
	switch v := input.(type) {
	case string:
		// Single string
		return []string{v}, nil
	case []interface{}:
		// Array of mixed types - convert to strings
		texts := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("input array must contain only strings, got %T at index %d", item, i)
			}
			texts[i] = str
		}
		return texts, nil
	case []string:
		// Array of strings (shouldn't happen with JSON unmarshaling, but handle it)
		return v, nil
	default:
		return nil, fmt.Errorf("input must be a string or array of strings, got %T", input)
	}
}

// ValidateInput validates that input texts are within reasonable limits
func (e *EmbeddingEngine) ValidateInput(texts []string) error {
	const maxTextLength = 8192 // Max tokens per text

	for i, text := range texts {
		// Clean the text
		text = strings.TrimSpace(text)
		if text == "" {
			return fmt.Errorf("text at index %d is empty", i)
		}

		// Check length (rough approximation: 1 token ≈ 4 chars)
		if len(text) > maxTextLength*4 {
			return fmt.Errorf("text at index %d exceeds maximum length (%d chars)", i, maxTextLength*4)
		}
	}

	return nil
}

// GetModelInfo returns information about the loaded model
func (e *EmbeddingEngine) GetModelInfo() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"loaded":     e.loaded,
		"model_path": e.modelPath,
		"dimensions": e.dimensions,
		"batch_size": e.maxBatchSize,
	}
}
