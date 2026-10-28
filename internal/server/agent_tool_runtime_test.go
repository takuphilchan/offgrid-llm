package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type observedAgentRuntime struct {
	llamaLifecycleFixture
	observation inference.ToolRuntimeObservation
	err         error
	calls       int
}

func TestInstalledAgentToolPolicyBindsEvidenceToOwnedRuntime(t *testing.T) {
	ctx := context.Background()
	digest := strings.Repeat("a", 64)
	fingerprint := models.CapabilityFingerprint{ModelSHA256: digest, Quantization: "fixture", RuntimeBuild: "fixture", TemplateSHA256: digest, Adapter: inference.NativeAgentToolAdapter, ContextWindow: 8192, Backend: "cpu", HardwareID: "fixture"}
	target := models.CapabilityEvidenceTarget{Fingerprint: fingerprint, SuiteSHA256: strings.Repeat("b", 64), Platform: "fixture"}
	fingerprint.RuntimeSHA256, fingerprint.RuntimeConfigSHA256 = strings.Repeat("d", 64), strings.Repeat("e", 64)
	target.Fingerprint = fingerprint
	owner := &observedAgentRuntime{observation: inference.ToolRuntimeObservation{ModelID: "model", Generation: 3, Info: inference.ToolRuntimeInfo{Build: "fixture", TemplateSHA256: digest, Context: 8192}}}
	owner.observation.ConfigSHA256 = fingerprint.RuntimeConfigSHA256
	s := &Server{config: &config.Config{DataDir: t.TempDir()}, llamaProcess: owner}
	lookup := func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) {
		copy := target
		return &copy, nil
	}
	if err := s.InstallQualifiedAgentTools(lookup, lookup); err != nil {
		t.Fatal(err)
	}
	if err := s.InstallQualifiedAgentTools(lookup, lookup); err == nil {
		t.Fatal("policy replaced after installation")
	}
	if pin, err := s.agentToolPolicy.Select(ctx, "model", 8192); err != nil || pin != nil || owner.calls != 0 {
		t.Fatal("unmeasured or cold admission queried a process", pin, err)
	}
	// Synthetic qualification lives only in this temporary test directory.
	store := models.NewCapabilityEvidenceStore(s.config.DataDir)
	_, err := store.Record(ctx, models.CapabilityEvidence{Version: 1, ModelID: "model", CheckID: inference.NativeAgentToolAdapter, Fingerprint: fingerprint, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now(), Qualification: &models.WorkflowQualification{SuiteSHA256: target.SuiteSHA256, Platform: target.Platform, ArtifactID: strings.Repeat("c", 64), Cases: 30, Repetitions: 3, Verified: 90}})
	if err != nil {
		t.Fatal(err)
	}
	pin, err := s.agentToolPolicy.Select(ctx, "model", 8192)
	if err != nil || pin == nil || owner.calls != 0 {
		t.Fatal("qualified admission changed", pin, err)
	}
	task := &agents.Task{Model: "model", Config: agents.AgentConfig{TaskFirst: true, ContextWindow: 8192, ToolAdapter: pin}}
	request := &api.ChatCompletionRequest{}
	if err := s.configureAgentRequest(ctx, request, task); err != nil || request.SingleStepTools || owner.calls != 2 {
		t.Fatal("matching live runtime rejected", err)
	}
	owner.err = context.DeadlineExceeded
	if err := s.configureAgentRequest(ctx, request, task); !errors.Is(err, context.DeadlineExceeded) || !request.SingleStepTools {
		t.Fatal("unobserved runtime admitted", err)
	}
	view, err := store.Latest(ctx, "model", inference.NativeAgentToolAdapter, target)
	if err != nil || view.State != "workflow_qualified" {
		t.Fatal("operational failure rewrote evidence", view, err)
	}
}

func (o *observedAgentRuntime) ObserveToolRuntime(context.Context, string, int) (*inference.ToolRuntimeObservation, error) {
	o.calls++
	copy := o.observation
	return &copy, o.err
}

func TestAgentToolMeasurementMatchesOwnedRuntime(t *testing.T) {
	for _, mutation := range []string{"none", "restart", "template", "build", "context", "configuration", "model", "missing_generation", "disabled_tools", "cancel"} {
		t.Run(mutation, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			owner := &observedAgentRuntime{observation: inference.ToolRuntimeObservation{ModelID: "model", Generation: 1, Info: inference.ToolRuntimeInfo{Build: "fixture", TemplateSHA256: strings.Repeat("a", 64), Context: 8192}}}
			owner.observation.ConfigSHA256 = strings.Repeat("f", 64)
			s := &Server{llamaProcess: owner}
			measure, err := s.observeAgentToolTarget(func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) {
				target := &models.CapabilityEvidenceTarget{Fingerprint: models.CapabilityFingerprint{RuntimeBuild: "fixture", TemplateSHA256: strings.Repeat("a", 64), ContextWindow: 8192}}
				target.Fingerprint.RuntimeConfigSHA256 = strings.Repeat("f", 64)
				switch mutation {
				case "restart":
					owner.observation.Generation++
				case "template":
					target.Fingerprint.TemplateSHA256 = strings.Repeat("b", 64)
				case "build":
					target.Fingerprint.RuntimeBuild = "another-build"
				case "context":
					target.Fingerprint.ContextWindow = 4096
				case "configuration":
					target.Fingerprint.RuntimeConfigSHA256 = strings.Repeat("e", 64)
				case "model":
					owner.observation.ModelID = "other"
				case "missing_generation":
					owner.observation.Generation = 0
				case "disabled_tools":
					no := false
					owner.observation.Info.SupportsTools = &no
				case "cancel":
					cancel()
				}
				return target, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := measure(ctx, "model", 8192)
			if mutation == "none" {
				if err != nil || got == nil || owner.calls != 2 {
					t.Fatal("valid observation rejected", got, err, owner.calls)
				}
			} else if err == nil || got != nil {
				t.Fatal("changed observation accepted", got, err)
			}
		})
	}
}

func TestAgentToolRuntimeObservationIsRequiredAndOperational(t *testing.T) {
	s := &Server{}
	lookup := func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) {
		t.Fatal("measurement ran without owned runtime")
		return nil, nil
	}
	if _, err := s.observeAgentToolTarget(lookup); err == nil {
		t.Fatal("missing process owner accepted")
	}
	owner := &observedAgentRuntime{err: context.DeadlineExceeded}
	s.llamaProcess = owner
	measure, err := s.observeAgentToolTarget(lookup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := measure(context.Background(), "model", 8192); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("operational error replaced", err)
	}
}
