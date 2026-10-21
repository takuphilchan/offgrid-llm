package server

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type waitingBenchmarkEngine struct {
	inference.Engine
	entered chan struct{}
}

func (e *waitingBenchmarkEngine) ChatCompletion(ctx context.Context, _ *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	close(e.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestBenchmarkCancellationReleasesModelAdmission(t *testing.T) {
	modelsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modelsDir, "benchmark.gguf"), []byte("isolated fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(modelsDir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	gate := inference.NewLifecycleGate(1)
	initial, err := gate.AcquireWithContext(context.Background(), "benchmark", 4096, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	initial()
	engine := &waitingBenchmarkEngine{Engine: inference.NewMockEngine(), entered: make(chan struct{})}
	s := &Server{registry: registry, inferenceLifecycle: gate, engine: engine, monitor: resource.NewMonitor(time.Second)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest("POST", "/api/benchmark", strings.NewReader(`{"model":"benchmark","prompt_tokens":1,"output_tokens":1,"iterations":2}`)).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleBenchmark(httptest.NewRecorder(), request)
	}()
	select {
	case <-engine.entered:
	case <-time.After(time.Second):
		t.Fatal("benchmark did not enter generation")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled benchmark retained its generation and model lease")
	}
	ready, err := gate.AcquireWithContext(context.Background(), "benchmark", 4096, func(context.Context, string) error { t.Fatal("benchmark changed the model"); return nil })
	if err != nil {
		t.Fatalf("benchmark did not release model admission: %v", err)
	}
	ready()
}
