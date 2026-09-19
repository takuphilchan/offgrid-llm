//go:build !llama

package inference

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Opt-in qualification: no model downloads and no writes to an existing index.
func TestEmbeddingRealRuntime(t *testing.T) {
	model := os.Getenv("OFFGRID_TEST_EMBEDDING_MODEL")
	if model == "" {
		t.Skip("set OFFGRID_TEST_EMBEDDING_MODEL and OFFGRID_LLAMA_SERVER_PATH for real-runtime qualification")
	}
	if os.Getenv("OFFGRID_LLAMA_SERVER_PATH") == "" {
		t.Fatal("explicit runtime path required; this test never downloads dependencies")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	e := NewEmbeddingEngine(t.TempDir())
	defer e.Unload()
	if err := e.Load(ctx, model, DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	response, err := e.GenerateEmbeddings(ctx, &api.EmbeddingRequest{Model: "qualification", Input: []string{
		"A cat is sleeping on the sofa.", "A kitten is resting on a couch.", "The price of copper rose on the stock exchange.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	dot := func(a, b []float32) float64 {
		var sum float64
		for i := range a {
			sum += float64(a[i]) * float64(b[i])
		}
		return sum
	}
	a, b, c := response.Data[0].Embedding, response.Data[1].Embedding, response.Data[2].Embedding
	if math.Abs(dot(a, a)-1) > 1e-4 {
		t.Fatal("vector not normalized")
	}
	if dot(a, b) <= dot(a, c) {
		t.Fatalf("semantic paraphrase failed: related=%f unrelated=%f", dot(a, b), dot(a, c))
	}
	if e.GetModelInfo()["identity"] == "" || response.Usage.PromptTokens <= 0 {
		t.Fatal("missing provenance or measured usage")
	}
	t.Logf("dimensions=%d prompt_tokens=%d related=%.4f unrelated=%.4f", len(a), response.Usage.PromptTokens, dot(a, b), dot(a, c))
}
