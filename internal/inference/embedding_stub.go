//go:build !llama
// +build !llama

package inference

import "fmt"

// StubEmbeddingImpl is a stub implementation when llama.cpp is not available
type StubEmbeddingImpl struct {
	dimensions int
}

// newEmbeddingImpl creates a stub embedding implementation
func newEmbeddingImpl() (EmbeddingImpl, error) {
	return &StubEmbeddingImpl{
		dimensions: 384, // Default embedding size
	}, nil
}

// Load simulates loading a model
func (s *StubEmbeddingImpl) Load(modelPath string, opts EmbeddingOptions) error {
	// Stub: just accept the load
	fmt.Printf("STUB: Would load embedding model from %s\n", modelPath)
	return nil
}

// Embed generates stub embeddings (random-ish values for testing)
func (s *StubEmbeddingImpl) Embed(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		// Generate deterministic "embedding" based on text hash
		embedding := make([]float32, s.dimensions)
		hash := simpleHash(text)

		for j := 0; j < s.dimensions; j++ {
			// Create pseudo-random but deterministic values
			hash = (hash*1103515245 + 12345) & 0x7fffffff
			embedding[j] = float32(hash%1000) / 1000.0
		}

		embeddings[i] = embedding
	}

	return embeddings, nil
}

// Unload does nothing in stub
func (s *StubEmbeddingImpl) Unload() error {
	fmt.Println("STUB: Unloading embedding model")
	return nil
}

// GetDimensions returns the stub dimension size
func (s *StubEmbeddingImpl) GetDimensions() int {
	return s.dimensions
}

// simpleHash creates a simple hash of a string
func simpleHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = (hash * 31) + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
