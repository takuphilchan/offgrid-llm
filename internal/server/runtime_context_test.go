package server

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestResolveContextWindowHonorsExplicitContext(t *testing.T) {
	got := resolveContextWindow(65536, false, 4096, 8192)
	if got != 65536 {
		t.Fatalf("got %d, want 65536", got)
	}
}

func TestTaskContextWindowKeepsAdmissionPinned(t *testing.T) {
	s := &Server{config: &config.Config{MaxContextSize: 4096, AdaptiveContext: true}}
	task := &agents.Task{Model: "model", Config: agents.AgentConfig{ContextWindow: 65536}}
	if got := s.taskContextWindow(task); got != 65536 {
		t.Fatalf("active task context changed to %d", got)
	}
}

func TestResolveContextWindowAppliesAdaptiveLimits(t *testing.T) {
	got := resolveContextWindow(65536, true, 8192, 4096)
	if got != 4096 {
		t.Fatalf("got %d, want 4096", got)
	}
}

func TestResolveContextWindowUsesSafeDefault(t *testing.T) {
	got := resolveContextWindow(0, false)
	if got != 4096 {
		t.Fatalf("got %d, want 4096", got)
	}
}

func TestPlanContextWindowAdmitsCapableLongContextModel(t *testing.T) {
	plan := planContextWindow(contextPlanningInput{
		OperatorMax: 65536, Adaptive: true, KVCacheType: "q8_0", InferenceSlots: 1,
		Model:      &api.ModelMetadata{Size: 4700 << 20, ContextSize: 131072, KVElementsPerToken: 28672},
		Resources:  &resource.SystemResources{TotalRAM: 10000, AvailableRAM: 7600, GPUAvailable: true, GPUMemory: 8151},
		GPUEnabled: true,
	})
	if plan.Window != 65536 || plan.ModelMax != 131072 || plan.ResourceMax != 65536 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestPlanContextWindowUsesConservativeTierWhenUnmeasured(t *testing.T) {
	plan := planContextWindow(contextPlanningInput{OperatorMax: 65536, Adaptive: true})
	if plan.Window != 8192 {
		t.Fatalf("got %d, want conservative 8192", plan.Window)
	}
}

func TestPlanContextWindowHonorsKnownModelLimitInFixedMode(t *testing.T) {
	plan := planContextWindow(contextPlanningInput{OperatorMax: 65536, Adaptive: false, Model: &api.ModelMetadata{ContextSize: 32768}})
	if plan.Window != 32768 {
		t.Fatalf("got %d, want trained limit 32768", plan.Window)
	}
}

func TestPlanContextWindowRoundsCapacityDownToStableTier(t *testing.T) {
	model := &api.ModelMetadata{Size: 3 << 30, ContextSize: 131072, KVElementsPerToken: 32768}
	resources := &resource.SystemResources{TotalRAM: 10000, AvailableRAM: 7100}
	plan := planContextWindow(contextPlanningInput{OperatorMax: 131072, Adaptive: true, Model: model, Resources: resources, KVCacheType: "f16", InferenceSlots: 1})
	if plan.Window != 32768 {
		t.Fatalf("unexpected rounded plan: %+v", plan)
	}
}
