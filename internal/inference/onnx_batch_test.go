package inference

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

type onnxBatchRuntime struct {
	fixtureProvider
	inputs []string
	sizes  []int
}

func (p *onnxBatchRuntime) Generate(_ context.Context, request ProviderRequest, callback func(ProviderEvent) error) error {
	if _, err := validateONNXEmbeddingPrompt(request, p.descriptor); err != nil {
		return err
	}
	var prompt struct {
		Input []string `json:"input"`
	}
	if err := json.Unmarshal(request.Prompt, &prompt); err != nil {
		return err
	}
	p.inputs = append(p.inputs, prompt.Input...)
	p.sizes = append(p.sizes, len(request.Prompt))
	vectors := make([][]float32, len(prompt.Input))
	for i := range vectors {
		vectors[i] = []float32{1, 0}
	}
	if err := callback(ProviderEvent{Type: ProviderEventEmbeddings, Embeddings: vectors}); err != nil {
		return err
	}
	return callback(ProviderEvent{Type: ProviderEventDone, FinishReason: "stop", Usage: &UsageMetrics{PromptTokens: len(vectors), TotalTokens: len(vectors)}})
}

func TestONNXEmbeddingBatchesByEncodedBytesWithoutTruncation(t *testing.T) {
	for _, text := range []string{strings.Repeat(" ", 32768), strings.Repeat("<", 32768)} {
		provider := &onnxBatchRuntime{fixtureProvider: fixtureProvider{onnxDescriptorFixture()}}
		impl := &ONNXEmbeddingImpl{runtime: provider, descriptor: provider.descriptor, settings: models.ONNXEmbeddingSettings{Dimensions: 2, MaxTokens: 8, NormalizeL2: true}}
		inputs := make([]string, 32)
		for i := range inputs {
			inputs[i] = text
		}
		vectors, err := impl.Embed(context.Background(), inputs)
		if err != nil || len(vectors) != len(inputs) || impl.UsageTokens() != len(inputs) {
			t.Fatalf("individually valid inputs failed: vectors=%d usage=%d err=%v", len(vectors), impl.UsageTokens(), err)
		}
		if len(provider.sizes) < 2 || !reflect.DeepEqual(inputs, provider.inputs) {
			t.Fatalf("batching changed or omitted input bytes: batches=%v", provider.sizes)
		}
		for _, size := range provider.sizes {
			if size > 768<<10 {
				t.Fatalf("oversized worker batch: %d", size)
			}
		}
	}
}

func TestONNXEmbeddingRejectsOversizeSingleInputWithoutInvocation(t *testing.T) {
	provider := &onnxBatchRuntime{fixtureProvider: fixtureProvider{onnxDescriptorFixture()}}
	impl := &ONNXEmbeddingImpl{runtime: provider, descriptor: provider.descriptor, settings: models.ONNXEmbeddingSettings{Dimensions: 2, MaxTokens: 8}}
	if vectors, err := impl.Embed(context.Background(), []string{strings.Repeat("x", 32769)}); err == nil || vectors != nil || len(provider.inputs) != 0 {
		t.Fatalf("oversized input was truncated or invoked: vectors=%v err=%v inputs=%d", vectors, err, len(provider.inputs))
	}
}
