package inference

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestNewEmbeddingEngine(t *testing.T) {
	engine := NewEmbeddingEngine()
	if engine == nil {
		t.Fatal("NewEmbeddingEngine returned nil")
	}

	if engine.IsLoaded() {
		t.Error("Newly created engine should not be loaded")
	}

	if engine.GetDimensions() != 0 {
		t.Errorf("Expected dimensions 0 for unloaded engine, got %d", engine.GetDimensions())
	}
}

func TestDefaultEmbeddingOptions(t *testing.T) {
	opts := DefaultEmbeddingOptions()

	if opts.NumThreads <= 0 {
		t.Error("NumThreads should be > 0")
	}

	if !opts.NormalizeL2 {
		t.Error("NormalizeL2 should be true by default")
	}

	if opts.PoolingMethod != "mean" {
		t.Errorf("Expected pooling method 'mean', got '%s'", opts.PoolingMethod)
	}
}

func TestParseInput(t *testing.T) {
	engine := NewEmbeddingEngine()

	tests := []struct {
		name        string
		input       interface{}
		expectedLen int
		shouldError bool
	}{
		{
			name:        "single string",
			input:       "Hello world",
			expectedLen: 1,
			shouldError: false,
		},
		{
			name:        "array of strings",
			input:       []interface{}{"Hello", "World"},
			expectedLen: 2,
			shouldError: false,
		},
		{
			name:        "invalid type",
			input:       123,
			expectedLen: 0,
			shouldError: true,
		},
		{
			name:        "mixed array",
			input:       []interface{}{"Hello", 123},
			expectedLen: 0,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			texts, err := engine.parseInput(tt.input)
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(texts) != tt.expectedLen {
					t.Errorf("Expected %d texts, got %d", tt.expectedLen, len(texts))
				}
			}
		})
	}
}

func TestValidateInput(t *testing.T) {
	engine := NewEmbeddingEngine()

	tests := []struct {
		name        string
		texts       []string
		shouldError bool
	}{
		{
			name:        "valid texts",
			texts:       []string{"Hello", "World"},
			shouldError: false,
		},
		{
			name:        "empty text",
			texts:       []string{"Hello", ""},
			shouldError: true,
		},
		{
			name:        "whitespace only",
			texts:       []string{"   "},
			shouldError: true,
		},
		{
			name:        "very long text",
			texts:       []string{string(make([]byte, 40000))},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidateInput(tt.texts)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateEmbeddings_Stub(t *testing.T) {
	engine := NewEmbeddingEngine()

	// Load a stub model
	ctx := context.Background()
	opts := DefaultEmbeddingOptions()
	err := engine.Load(ctx, "/fake/model.gguf", opts)
	if err != nil {
		t.Fatalf("Failed to load stub model: %v", err)
	}

	if !engine.IsLoaded() {
		t.Error("Engine should be loaded")
	}

	// Create embedding request
	req := &api.EmbeddingRequest{
		Model: "test-model",
		Input: []interface{}{"Hello world", "Test embedding"},
	}

	// Generate embeddings
	response, err := engine.GenerateEmbeddings(ctx, req)
	if err != nil {
		t.Fatalf("Failed to generate embeddings: %v", err)
	}

	// Verify response
	if response.Object != "list" {
		t.Errorf("Expected object 'list', got '%s'", response.Object)
	}

	if len(response.Data) != 2 {
		t.Errorf("Expected 2 embeddings, got %d", len(response.Data))
	}

	for i, embedding := range response.Data {
		if embedding.Object != "embedding" {
			t.Errorf("Embedding %d: expected object 'embedding', got '%s'", i, embedding.Object)
		}

		if embedding.Index != i {
			t.Errorf("Embedding %d: expected index %d, got %d", i, i, embedding.Index)
		}

		if len(embedding.Embedding) != engine.GetDimensions() {
			t.Errorf("Embedding %d: expected dimensions %d, got %d",
				i, engine.GetDimensions(), len(embedding.Embedding))
		}

		// Check that embeddings are reasonable values
		for j, val := range embedding.Embedding {
			if val < 0 || val > 1 {
				t.Errorf("Embedding %d, dimension %d: value %f out of range [0, 1]", i, j, val)
				break
			}
		}
	}

	// Check usage stats
	if response.Usage.PromptTokens <= 0 {
		t.Error("Expected positive prompt tokens")
	}

	if response.Usage.TotalTokens != response.Usage.PromptTokens {
		t.Error("Total tokens should equal prompt tokens for embeddings")
	}
}

func TestEmbedding_SingleString(t *testing.T) {
	engine := NewEmbeddingEngine()
	ctx := context.Background()

	err := engine.Load(ctx, "/fake/model.gguf", DefaultEmbeddingOptions())
	if err != nil {
		t.Fatalf("Failed to load model: %v", err)
	}

	req := &api.EmbeddingRequest{
		Model: "test-model",
		Input: "Single text input",
	}

	response, err := engine.GenerateEmbeddings(ctx, req)
	if err != nil {
		t.Fatalf("Failed to generate embeddings: %v", err)
	}

	if len(response.Data) != 1 {
		t.Errorf("Expected 1 embedding for single string, got %d", len(response.Data))
	}
}

func TestEmbedding_EmptyInput(t *testing.T) {
	engine := NewEmbeddingEngine()
	ctx := context.Background()

	err := engine.Load(ctx, "/fake/model.gguf", DefaultEmbeddingOptions())
	if err != nil {
		t.Fatalf("Failed to load model: %v", err)
	}

	req := &api.EmbeddingRequest{
		Model: "test-model",
		Input: []interface{}{},
	}

	_, err = engine.GenerateEmbeddings(ctx, req)
	if err == nil {
		t.Error("Expected error for empty input")
	}
}

func TestUnload(t *testing.T) {
	engine := NewEmbeddingEngine()
	ctx := context.Background()

	// Load model
	err := engine.Load(ctx, "/fake/model.gguf", DefaultEmbeddingOptions())
	if err != nil {
		t.Fatalf("Failed to load model: %v", err)
	}

	if !engine.IsLoaded() {
		t.Error("Engine should be loaded")
	}

	// Unload
	err = engine.Unload()
	if err != nil {
		t.Errorf("Failed to unload: %v", err)
	}

	if engine.IsLoaded() {
		t.Error("Engine should not be loaded after unload")
	}

	if engine.GetDimensions() != 0 {
		t.Error("Dimensions should be 0 after unload")
	}
}

func TestGetModelInfo(t *testing.T) {
	engine := NewEmbeddingEngine()
	ctx := context.Background()

	modelPath := "/fake/model.gguf"
	err := engine.Load(ctx, modelPath, DefaultEmbeddingOptions())
	if err != nil {
		t.Fatalf("Failed to load model: %v", err)
	}

	info := engine.GetModelInfo()

	if info["loaded"] != true {
		t.Error("Model should be marked as loaded")
	}

	if info["model_path"] != modelPath {
		t.Errorf("Expected model_path '%s', got '%s'", modelPath, info["model_path"])
	}

	if info["dimensions"].(int) <= 0 {
		t.Error("Dimensions should be > 0")
	}
}

func BenchmarkGenerateEmbeddings(b *testing.B) {
	engine := NewEmbeddingEngine()
	ctx := context.Background()

	err := engine.Load(ctx, "/fake/model.gguf", DefaultEmbeddingOptions())
	if err != nil {
		b.Fatalf("Failed to load model: %v", err)
	}

	req := &api.EmbeddingRequest{
		Model: "test-model",
		Input: "This is a test sentence for benchmarking embedding generation performance.",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.GenerateEmbeddings(ctx, req)
		if err != nil {
			b.Fatalf("Failed to generate embeddings: %v", err)
		}
	}
}

func BenchmarkGenerateEmbeddingsBatch(b *testing.B) {
	engine := NewEmbeddingEngine()
	ctx := context.Background()

	err := engine.Load(ctx, "/fake/model.gguf", DefaultEmbeddingOptions())
	if err != nil {
		b.Fatalf("Failed to load model: %v", err)
	}

	// Create batch of 10 texts
	texts := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		texts[i] = "This is a test sentence for benchmarking embedding generation performance."
	}

	req := &api.EmbeddingRequest{
		Model: "test-model",
		Input: texts,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.GenerateEmbeddings(ctx, req)
		if err != nil {
			b.Fatalf("Failed to generate embeddings: %v", err)
		}
	}
}
