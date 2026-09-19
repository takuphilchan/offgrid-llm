package inference

import "context"

// Test vectors must never be selected by a production build.
type fixtureEmbedding struct{}

func (fixtureEmbedding) Load(ctx context.Context, _ string, _ EmbeddingOptions) error {
	return ctx.Err()
}
func (fixtureEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = make([]float32, 384)
		for j := range result[i] {
			result[i][j] = 0.5
		}
	}
	return result, nil
}
func (fixtureEmbedding) Unload() error      { return nil }
func (fixtureEmbedding) GetDimensions() int { return 384 }
func (fixtureEmbedding) UsageTokens() int   { return 7 }
func newTestEmbeddingEngine() *EmbeddingEngine {
	e := NewEmbeddingEngine()
	e.newImplementation = func() (EmbeddingImpl, error) { return fixtureEmbedding{}, nil }
	return e
}
