package server

import (
	"context"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type contextCountingEngine struct {
	inference.Engine
	count func(context.Context, *api.ChatCompletionRequest) (int, error)
}

func (e *contextCountingEngine) CountPromptTokens(ctx context.Context, request *api.ChatCompletionRequest) (int, error) {
	return e.count(ctx, request)
}

func TestAgentContextCountingPinsModelAndReleasesLease(t *testing.T) {
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	s := &Server{config: &config.Config{MaxContextSize: 8192}, inferenceLifecycle: gate}
	task := &agents.Task{Model: "model", Config: agents.AgentConfig{TaskFirst: true, ComputerScope: "desktop", ComputerSession: "session", ComputerDriver: "windows-uia", MaxTokens: 1024}}
	s.engine = &contextCountingEngine{count: func(ctx context.Context, request *api.ChatCompletionRequest) (int, error) {
		if gate.Status().Active != 1 || gate.Status().CurrentModel != "model" || gate.Status().ContextWindow != 8192 {
			t.Fatal("count lacks model lease")
		}
		if request.Model != "model" || request.ToolChoice != "auto" || request.ParallelToolCalls == nil || *request.ParallelToolCalls || len(request.Messages) != 1 || len(request.Tools) != 1 || *request.MaxTokens != 1024 {
			t.Fatalf("request divergence: %+v", request)
		}
		return 4000, nil
	}}
	count, err := s.countAgentContext(context.Background(), task, []api.ChatMessage{{Role: "user", Content: "fixture"}}, nativeComputerTools()[:1])
	if err != nil || count != 4000 || gate.Status().Active != 0 {
		t.Fatalf("count=%d err=%v gate=%+v", count, err, gate.Status())
	}
	s.engine.(*contextCountingEngine).count = func(context.Context, *api.ChatCompletionRequest) (int, error) {
		return 0, inference.ErrPromptCountUnavailable
	}
	if _, err = s.countAgentContext(context.Background(), task, nil, nil); !errors.Is(err, agents.ErrContextCountUnavailable) || gate.Status().Active != 0 {
		t.Fatal("fallback or release failed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.countAgentContext(ctx, task, nil, nil); !errors.Is(err, context.Canceled) || gate.Status().Active != 0 {
		t.Fatal("count ignored cancellation", err)
	}
	task.Config.ComputerDriver = "browser"
	s.engine.(*contextCountingEngine).count = func(context.Context, *api.ChatCompletionRequest) (int, error) {
		t.Fatal("text count claimed multimodal budget")
		return 0, nil
	}
	if _, err = s.countAgentContext(context.Background(), task, nil, nil); !errors.Is(err, agents.ErrContextCountUnavailable) {
		t.Fatal(err)
	}
}
