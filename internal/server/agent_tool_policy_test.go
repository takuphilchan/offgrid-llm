package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestAgentRequestAdapterPreservesOldJobsAndFailsClosed(t *testing.T) {
	s := profileFixture(t)
	task := &agents.Task{Model: "model", Config: agents.AgentConfig{TaskFirst: true, ContextWindow: 8192}}
	request := &api.ChatCompletionRequest{}
	if err := s.configureAgentRequest(context.Background(), request, task); err != nil || !request.SingleStepTools || request.ParallelToolCalls == nil || *request.ParallelToolCalls {
		t.Fatal("old adapter changed", err)
	}
	task.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, EvidenceID: "untrusted", FingerprintID: "untrusted"}
	if err := s.configureAgentRequest(context.Background(), request, task); err == nil {
		t.Fatal("unqualified native pin enabled")
	}
	if !request.SingleStepTools {
		t.Fatal("failure changed request adapter")
	}
}

func TestQualifiedAgentRequestKeepsToolsAndChecksCurrentEvidence(t *testing.T) {
	s := profileFixture(t)
	ctx := context.Background()
	f := models.CapabilityFingerprint{ModelSHA256: strings.Repeat("a", 64), TemplateSHA256: strings.Repeat("b", 64), Quantization: "fixture", RuntimeBuild: "fixture", Adapter: inference.NativeAgentToolAdapter, ContextWindow: 8192, Backend: "cpu", HardwareID: "fixture"}
	q := &models.WorkflowQualification{SuiteSHA256: strings.Repeat("c", 64), Platform: "fixture", ArtifactID: strings.Repeat("d", 64), Cases: 30, Repetitions: 3, Verified: 90}
	f.RuntimeSHA256, f.RuntimeConfigSHA256 = strings.Repeat("e", 64), strings.Repeat("f", 64)
	store := models.NewCapabilityEvidenceStore(t.TempDir())
	if _, err := store.Record(ctx, models.CapabilityEvidence{Version: 1, ModelID: "fixture", CheckID: inference.NativeAgentToolAdapter, Fingerprint: f, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now(), Qualification: q}); err != nil {
		t.Fatal(err)
	}
	lookup := func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) {
		return &models.CapabilityEvidenceTarget{Fingerprint: f, SuiteSHA256: q.SuiteSHA256, Platform: q.Platform}, nil
	}
	policy, err := inference.NewAgentToolPolicy(store, lookup, lookup)
	if err != nil {
		t.Fatal(err)
	}
	s.agentToolPolicy = policy
	pin, err := policy.Select(ctx, "fixture", 8192)
	if err != nil || pin == nil {
		t.Fatal(pin, err)
	}
	task := &agents.Task{Model: "fixture", Config: agents.AgentConfig{TaskFirst: true, ContextWindow: 8192, ToolAdapter: pin}}
	request := &api.ChatCompletionRequest{Model: "fixture", Tools: []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "fixture_read"}}}}
	if err := s.configureAgentRequest(ctx, request, task); err != nil || request.SingleStepTools || request.ParallelToolCalls == nil || *request.ParallelToolCalls || len(request.Tools) != 1 || request.Tools[0].Function.Name != "fixture_read" {
		t.Fatal("native request changed tool contract", err)
	}
	f.TemplateSHA256 = strings.Repeat("e", 64)
	if err := s.configureAgentRequest(ctx, request, task); err == nil {
		t.Fatal("changed runtime accepted")
	}
}
