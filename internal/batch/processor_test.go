package batch

import (
	"context"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"testing"
	"time"
)

type measurementEngine struct{ inference.Engine }

func (measurementEngine) Completion(context.Context, *api.CompletionRequest) (*api.CompletionResponse, error) {
	time.Sleep(20 * time.Millisecond)
	return &api.CompletionResponse{Usage: api.Usage{PromptTokens: 1000, CompletionTokens: 2, TotalTokens: 1002}}, nil
}
func TestBatchMetricsHaveHonestUnitsAndDenominator(t *testing.T) {
	r := NewProcessor(measurementEngine{}, 1).processRequest(context.Background(), &Request{ID: "test", Prompt: "hi"})
	if r.Duration < 10 || r.Duration > 5000 {
		t.Fatalf("duration_ms=%d", r.Duration)
	}
	if r.TokensPerSec > 200 || r.PromptTokens != 1000 || r.CompletionTokens != 2 {
		t.Fatalf("wrong metrics: %+v", r)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	_ = json.Unmarshal(data, &value)
	if value["duration_ms"] != float64(r.Duration) || value["throughput_basis"] != "completion_tokens_per_end_to_end_second" {
		t.Fatalf("ambiguous metrics: %s", data)
	}
}
