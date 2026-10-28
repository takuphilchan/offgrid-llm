package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type measuredAgentRuntimeFixture struct {
	observedAgentRuntime
	measure func(context.Context, string, int, inference.ToolRuntimeArtifacts) (*inference.MeasuredToolRuntime, error)
}

func (f *measuredAgentRuntimeFixture) MeasureToolRuntimeArtifacts(ctx context.Context, model string, window int, files inference.ToolRuntimeArtifacts) (*inference.MeasuredToolRuntime, error) {
	return f.measure(ctx, model, window, files)
}

func TestMeasuredAgentStartupSelectsOnlyMatchingEvidence(t *testing.T) {
	ctx := context.Background()
	i, m, modelPath := measuredInstallationFixture(t)
	dir := t.TempDir()
	resources := &resource.SystemResources{OS: runtime.GOOS, Arch: runtime.GOARCH, TotalRAM: 8192, CPUCores: 4}
	hardware, err := resource.QualificationHardwareID(ctx, resources)
	if err != nil {
		t.Fatal(err)
	}
	i.Fingerprint.HardwareID = hardware
	metadata, err := m.model("fixture")
	if err != nil {
		t.Fatal(err)
	}
	registryJSON, _ := json.Marshal(map[string]*api.ModelMetadata{"fixture": metadata})
	registryPath := filepath.Join(dir, "registry.json")
	if err := os.WriteFile(registryPath, registryJSON, 0600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(filepath.Dir(modelPath))
	if err := registry.LoadRegistry(registryPath); err != nil {
		t.Fatal(err)
	}
	measured, err := m.live(ctx, "fixture", 8192, inference.ToolRuntimeArtifacts{})
	if err != nil {
		t.Fatal(err)
	}
	owner := &measuredAgentRuntimeFixture{observedAgentRuntime: observedAgentRuntime{observation: measured.Observation}, measure: m.live}
	s := &Server{config: &config.Config{DataDir: dir}, registry: registry, systemResources: resources, llamaProcess: owner}
	inventoryDir := filepath.Join(dir, "model-capabilities")
	if err := os.Mkdir(inventoryDir, 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "models": []agentToolInstallation{i}})
	if err := os.WriteFile(filepath.Join(inventoryDir, "native-agent-installations.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.InstallMeasuredAgentTools(); err != nil {
		t.Fatal(err)
	}
	if err := s.InstallMeasuredAgentTools(); err == nil {
		t.Fatal("installed policy replaced")
	}
	s.modelMutex.Lock()
	_, busyErr := s.agentToolPolicy.Select(ctx, "fixture", 8192)
	s.modelMutex.Unlock()
	if busyErr != inference.ErrRuntimeBusy {
		t.Fatal("cold measurement bypassed installation owner", busyErr)
	}
	if pin, err := s.agentToolPolicy.Select(ctx, "fixture", 8192); err != nil || pin != nil || owner.calls != 0 {
		t.Fatal("installation claimed qualification or loaded runtime", pin, err)
	}
	// Synthetic evaluator records are restricted to this test's temporary state.
	// They exercise startup wiring, never qualify a real model or installation.
	q := &models.WorkflowQualification{Scope: i.Scope, SuiteSHA256: i.SuiteSHA256, CapabilitySHA256: i.CapabilitySHA256, Platform: i.Platform, ArtifactID: strings.Repeat("c", 64), Repetitions: 3}
	cases, _ := models.NativeWorkspaceCasesForScope(i.Scope)
	q.Cases, q.Verified = len(cases), len(cases)*3
	for _, c := range cases {
		for repetition := 1; repetition <= 3; repetition++ {
			q.Results = append(q.Results, models.WorkflowCaseResult{CaseID: c.ID, Repetition: repetition, ArtifactID: q.ArtifactID, Verified: true})
		}
	}
	store := models.NewCapabilityEvidenceStore(dir)
	if _, err := store.Record(ctx, models.CapabilityEvidence{Version: 1, ModelID: "fixture", CheckID: inference.NativeAgentToolAdapter, Fingerprint: i.Fingerprint, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now(), Qualification: q}); err != nil {
		t.Fatal(err)
	}
	pin, err := s.agentToolPolicy.Select(ctx, "fixture", 8192)
	if err != nil || pin == nil || owner.calls != 0 {
		t.Fatal(pin, err)
	}
	task := &agents.Task{Model: "fixture", Config: agents.AgentConfig{TaskFirst: true, ContextWindow: 8192, ToolAdapter: pin}}
	request := &api.ChatCompletionRequest{}
	if err := s.configureAgentRequest(ctx, request, task); err != nil || request.SingleStepTools || owner.calls != 2 {
		t.Fatal("installed measurement path rejected", err)
	}
	owner.observation.Generation++
	owner.observation.ConfigSHA256 = strings.Repeat("f", 64)
	if err := s.configureAgentRequest(ctx, request, task); err == nil || !request.SingleStepTools {
		t.Fatal("changed owned process admitted")
	}
	view, err := store.Latest(ctx, "fixture", inference.NativeAgentToolAdapter, i.target())
	if err != nil || view.State != "scope_qualified" {
		t.Fatal("runtime mismatch overwrote evidence", view, err)
	}
	if err := os.WriteFile(modelPath, []byte("replacement model bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if current, err := s.agentToolPolicy.Select(ctx, "fixture", 8192); err != nil || current != nil {
		t.Fatal("new work reused stale evidence", current, err)
	}
	if err := s.configureAgentRequest(ctx, request, task); err == nil || !request.SingleStepTools {
		t.Fatal("old native job fell back after artifact change")
	}
}
