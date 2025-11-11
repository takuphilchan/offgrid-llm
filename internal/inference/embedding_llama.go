//go:build llama
// +build llama

package inference

import (
	"fmt"

	llama "github.com/go-skynet/go-llama.cpp"
)

// LlamaEmbeddingImpl implements embeddings using llama.cpp
type LlamaEmbeddingImpl struct {
	model      *llama.LLama
	opts       EmbeddingOptions
	dimensions int
}

// newEmbeddingImpl creates a new llama.cpp embedding implementation
func newEmbeddingImpl() (EmbeddingImpl, error) {
	return &LlamaEmbeddingImpl{}, nil
}

// Load loads the embedding model using llama.cpp
func (l *LlamaEmbeddingImpl) Load(modelPath string, opts EmbeddingOptions) error {
	// Convert our options to llama.cpp options
	llamaOpts := []llama.ModelOption{
		llama.SetContext(opts.ContextSize),
		llama.SetGPULayers(opts.NumGPULayers),
		llama.SetThreads(opts.NumThreads),
		llama.EnableEmbeddings, // Enable embedding mode
	}

	if opts.UseMlock {
		llamaOpts = append(llamaOpts, llama.EnableMLock)
	}

	if opts.UseMmap {
		llamaOpts = append(llamaOpts, llama.EnableMemoryMapping)
	}

	// Load the model
	model, err := llama.New(modelPath, llamaOpts...)
	if err != nil {
		return fmt.Errorf("failed to load model with llama.cpp: %w", err)
	}

	l.model = model
	l.opts = opts

	// Get embedding dimensions from model
	// Note: This is model-dependent. Most embedding models are 384 or 768 dims
	// For now, we'll try to infer from the model or use a default
	l.dimensions = 384 // Default, will be updated after first embedding

	return nil
}

// Embed generates embeddings for multiple texts
func (l *LlamaEmbeddingImpl) Embed(texts []string) ([][]float32, error) {
	if l.model == nil {
		return nil, fmt.Errorf("model not loaded")
	}

	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		// Generate embedding using llama.cpp
		embedding, err := l.model.Embeddings(text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}

		// Convert []float64 to []float32 if needed
		embeddingF32 := make([]float32, len(embedding))
		for j, v := range embedding {
			embeddingF32[j] = float32(v)
		}

		// Update dimensions on first embedding
		if i == 0 && l.dimensions != len(embeddingF32) {
			l.dimensions = len(embeddingF32)
		}

		// Normalize if requested
		if l.opts.NormalizeL2 {
			embeddingF32 = normalizeL2(embeddingF32)
		}

		embeddings[i] = embeddingF32
	}

	return embeddings, nil
}

// Unload frees the model
func (l *LlamaEmbeddingImpl) Unload() error {
	if l.model != nil {
		l.model.Free()
		l.model = nil
	}
	return nil
}

// GetDimensions returns the embedding dimension size
func (l *LlamaEmbeddingImpl) GetDimensions() int {
	return l.dimensions
}

// normalizeL2 normalizes a vector to unit length (L2 normalization)
func normalizeL2(vec []float32) []float32 {
	var sumSquares float32
	for _, v := range vec {
		sumSquares += v * v
	}

	if sumSquares == 0 {
		return vec
	}

	magnitude := float32(1.0) / float32(sqrt64(float64(sumSquares)))

	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = v * magnitude
	}

	return normalized
}

// sqrt64 is a helper for square root calculation
func sqrt64(x float64) float64 {
	if x == 0 {
		return 0
	}
	// Newton-Raphson approximation
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
