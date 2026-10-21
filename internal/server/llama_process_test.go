package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

type llamaLifecycleFixture struct {
	calls      []string
	fail       bool
	generation uint64
}

func (f *llamaLifecycleFixture) Activate(_ context.Context, model, _, _ string, _ inference.LoadOptions) (int, error) {
	f.calls = append(f.calls, model)
	if f.fail {
		return 0, errors.New("engine connection failed after port selection")
	}
	f.generation++
	return 12345, nil
}
func (*llamaLifecycleFixture) Alive(string) bool                       { return true }
func (f *llamaLifecycleFixture) Generation(string) uint64              { return f.generation }
func (*llamaLifecycleFixture) RetireAll(context.Context) (bool, error) { return false, nil }

func TestFailedLlamaActivationCannotReuseOldModelFastPath(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"old.gguf", "new.gguf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	process := &llamaLifecycleFixture{fail: true}
	s := &Server{config: &config.Config{ModelsDir: dir}, registry: registry, llamaProcess: process, currentModelID: "old", currentContext: 4096, currentPort: 54321}
	if err := s.switchModelWithContextSize(context.Background(), "new", 4096); err == nil {
		t.Fatal("failed engine activation was reported as loaded")
	}
	if s.currentModelID != "" || s.currentContext != 0 || s.currentPort != 0 {
		t.Fatalf("failed activation retained stale fast-path identity: model=%q context=%d port=%d", s.currentModelID, s.currentContext, s.currentPort)
	}
	process.fail = false
	if err := s.switchModelWithContextSize(context.Background(), "old", 4096); err != nil {
		t.Fatal(err)
	}
	if len(process.calls) != 2 || process.calls[0] != "new" || process.calls[1] != "old" || s.currentPort != 12345 {
		t.Fatalf("old model was not reconnected after failed switch: calls=%v port=%d", process.calls, s.currentPort)
	}
}

func TestRestartedLlamaChildReconnectsBeforeGeneration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	scheduler, err := inference.NewProviderScheduler(inference.NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	process := &llamaLifecycleFixture{}
	s := &Server{
		config: &config.Config{ModelsDir: dir}, registry: registry,
		llamaProcess: process, inferenceLifecycle: inference.NewLifecycleGate(1),
		providerScheduler: scheduler,
	}
	for attempt := 0; attempt < 2; attempt++ {
		release, err := s.acquireInferenceContext(context.Background(), "model", 4096, nil)
		if err != nil {
			t.Fatal(err)
		}
		release()
		if attempt == 0 {
			// Simulate the cache monitor replacing an alive child, potentially
			// on the same port, before the next request is admitted.
			process.generation++
		}
	}
	if len(process.calls) != 2 || s.currentGeneration != process.generation {
		t.Fatalf("replacement was not reconnected: activations=%v bound=%d running=%d", process.calls, s.currentGeneration, process.generation)
	}
}
