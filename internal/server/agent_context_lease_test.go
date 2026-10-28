package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type pinnedContextProcess struct {
	llamaLifecycleFixture
	allocations []int
}

func (p *pinnedContextProcess) Activate(ctx context.Context, model, file, projector string, options inference.LoadOptions) (int, error) {
	p.allocations = append(p.allocations, options.ContextSize)
	return p.llamaLifecycleFixture.Activate(ctx, model, file, projector, options)
}

type agentPromptCounterFixture struct {
	inference.Engine
	calls int
}

func (e *agentPromptCounterFixture) CountPromptTokens(context.Context, *api.ChatCompletionRequest) (int, error) {
	e.calls++
	return 123, nil
}

func TestAgentContextCounterUsesSamePinnedAllocationAsGeneration(t *testing.T) {
	for _, window := range []int{8192, 32768} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "selected.gguf"), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		registry := models.NewRegistry(dir)
		if err := registry.ScanModels(); err != nil {
			t.Fatal(err)
		}
		process := &pinnedContextProcess{}
		engine := &agentPromptCounterFixture{}
		s := &Server{config: &config.Config{MaxContextSize: 65536}, registry: registry, llamaProcess: process, engine: engine, inferenceLifecycle: inference.NewLifecycleGate(1)}
		task := &agents.Task{Model: "selected", Config: agents.AgentConfig{TaskFirst: true, ContextWindow: window, MaxTokens: 512}}
		count, err := s.countAgentContext(context.Background(), task, []api.ChatMessage{{Role: "user", Content: "hello"}}, nil)
		if err != nil || count != 123 || engine.calls != 1 || len(process.allocations) != 1 || process.allocations[0] != window {
			t.Fatalf("count used wrong allocation: count=%d error=%v allocations=%v", count, err, process.allocations)
		}
		lease, err := s.acquireInferenceLease(context.Background(), task.Model, s.taskContextWindow(task), nil)
		if err != nil {
			t.Fatal(err)
		}
		lease.Release()
		if len(process.allocations) != 1 || s.currentContext != window || s.config.MaxContextSize != 65536 {
			t.Fatal("generation reloaded or mutated operator context", process.allocations, s.currentContext)
		}
	}
}
