package inference

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// EmbeddingEngine handles text embedding generation
type EmbeddingEngine struct {
	mu                sync.RWMutex
	modelPath         string
	loaded            bool
	dimensions        int
	maxBatchSize      int
	implementation    EmbeddingImpl // Platform-specific implementation
	newImplementation func() (EmbeddingImpl, error)
}

// EmbeddingImpl is the interface for platform-specific embedding implementations
type EmbeddingImpl interface {
	Load(ctx context.Context, modelPath string, opts EmbeddingOptions) error
	Embed(ctx context.Context, texts []string) ([][]float32, error)
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
	BinDir        string // Managed runtime directory; OFFGRID_BIN_DIR overrides this.
	RuntimePath   string // Explicit preinstalled runtime, used by offline maintenance.
}

// DefaultEmbeddingOptions returns sensible defaults optimized for low-end hardware
func DefaultEmbeddingOptions() EmbeddingOptions {
	threads := runtime.NumCPU() / 2
	if threads < 1 {
		threads = 1
	}

	return EmbeddingOptions{
		NumThreads:    threads,
		NumGPULayers:  0,     // CPU by default
		UseMmap:       true,  // Memory-map for lower RAM usage
		UseMlock:      false, // Don't lock RAM (safer for low-end systems)
		ContextSize:   512,   // Embeddings don't need large context
		NormalizeL2:   true,  // Standard for embeddings
		PoolingMethod: "",    // Use the model's pooling metadata unless explicitly overridden.
	}
}

// NewEmbeddingEngine creates a new embedding engine
func NewEmbeddingEngine(binDir ...string) *EmbeddingEngine {
	directory := ""
	if len(binDir) > 0 {
		directory = binDir[0]
	}
	return &EmbeddingEngine{
		loaded:            false,
		maxBatchSize:      32, // Process up to 32 texts at once
		newImplementation: func() (EmbeddingImpl, error) { return newEmbeddingImpl(directory) },
	}
}

// Load loads an embedding model
func (e *EmbeddingEngine) Load(ctx context.Context, modelPath string, opts EmbeddingOptions) error {
	if err := e.lockContext(ctx); err != nil {
		return err
	}
	defer e.mu.Unlock()
	return e.loadLocked(ctx, modelPath, opts)
}

func (e *EmbeddingEngine) loadLocked(ctx context.Context, modelPath string, opts EmbeddingOptions) error {

	if e.loaded {
		if err := e.unloadUnsafe(); err != nil {
			return fmt.Errorf("failed to unload previous model: %w", err)
		}
	}

	// Create platform-specific implementation
	impl, err := e.newImplementation()
	if err != nil {
		return fmt.Errorf("failed to create embedding implementation: %w", err)
	}

	// Load the model
	if err := impl.Load(ctx, modelPath, opts); err != nil {
		_ = impl.Unload()
		return fmt.Errorf("failed to load embedding model: %w", err)
	}
	if impl.GetDimensions() <= 0 {
		_ = impl.Unload()
		return fmt.Errorf("embedding runtime returned no dimensions")
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
	return e.GenerateEmbeddingsForIdentity(ctx, req, "")
}

// GenerateEmbeddingsForIdentity prevents a concurrent API model switch from
// silently contaminating a knowledge index built with a different model.
func (e *EmbeddingEngine) GenerateEmbeddingsForIdentity(ctx context.Context, req *api.EmbeddingRequest, identity string) (*api.EmbeddingResponse, error) {
	if err := e.lockContext(ctx); err != nil {
		return nil, err
	}
	defer e.mu.Unlock()
	return e.generateLocked(ctx, req, identity)
}

// GenerateEmbeddingsForModel binds loading and execution to the same lock so
// concurrent callers cannot return vectors produced by another caller's model.
func (e *EmbeddingEngine) GenerateEmbeddingsForModel(ctx context.Context, path string, opts EmbeddingOptions, req *api.EmbeddingRequest) (*api.EmbeddingResponse, error) {
	if err := e.lockContext(ctx); err != nil {
		return nil, err
	}
	defer e.mu.Unlock()
	if !e.loaded || e.modelPath != path {
		if err := e.loadLocked(ctx, path, opts); err != nil {
			return nil, err
		}
	}
	return e.generateLocked(ctx, req, "")
}

func (e *EmbeddingEngine) generateLocked(ctx context.Context, req *api.EmbeddingRequest, identity string) (*api.EmbeddingResponse, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no embedding model loaded")
	}
	impl := e.implementation
	if identity != "" && embeddingIdentity(impl) != identity {
		return nil, fmt.Errorf("embedding runtime changed; reload the knowledge model before retrying")
	}
	if req == nil {
		return nil, fmt.Errorf("embedding request is required")
	}
	if req.EncodingFormat != "" && req.EncodingFormat != "float" {
		return nil, fmt.Errorf("only float embedding encoding is supported")
	}

	// Parse input (can be string or []string)
	texts, err := e.parseInput(req.Input)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no input texts provided")
	}
	if err := e.ValidateInput(texts); err != nil {
		return nil, err
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
		embeddings, err := impl.Embed(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings: %w", err)
		}
		if len(embeddings) != len(batch) {
			return nil, fmt.Errorf("embedding runtime returned %d vectors for %d inputs", len(embeddings), len(batch))
		}
		for _, vector := range embeddings {
			if len(vector) != e.dimensions {
				return nil, fmt.Errorf("embedding dimension changed: got %d, expected %d", len(vector), e.dimensions)
			}
			var norm float64
			for _, value := range vector {
				if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
					return nil, fmt.Errorf("embedding runtime returned non-finite values")
				}
				norm += float64(value) * float64(value)
			}
			if norm == 0 {
				return nil, fmt.Errorf("embedding runtime returned a zero vector")
			}
		}

		allEmbeddings = append(allEmbeddings, embeddings...)

		// Never present character estimates as measured tokenizer usage.
		if measured, ok := impl.(interface{ UsageTokens() int }); ok {
			totalTokens += measured.UsageTokens()
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
		"identity":   embeddingIdentity(e.implementation),
	}
}

func embeddingIdentity(impl EmbeddingImpl) string {
	if verified, ok := impl.(interface{ Identity() string }); ok {
		return verified.Identity()
	}
	return ""
}

func (e *EmbeddingEngine) lockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.mu.TryLock() {
		return nil
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if e.mu.TryLock() {
				return nil
			}
		}
	}
}
