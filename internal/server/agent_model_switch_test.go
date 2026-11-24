package server

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type switchEngineFixture struct {
	inference.Engine
	counted, generated []string
	revoke             func()
}

func (e *switchEngineFixture) CountPromptTokens(_ context.Context, request *api.ChatCompletionRequest) (int, error) {
	e.counted = append(e.counted, request.Model)
	return 256, nil
}

func (e *switchEngineFixture) ChatCompletion(_ context.Context, request *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	e.generated = append(e.generated, request.Model)
	if request.Model == "primary" {
		if e.revoke != nil {
			e.revoke()
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "invalid", Type: "function", Function: api.FunctionCall{Name: "not_registered", Arguments: "{}"}}}}}}}, nil
	}
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Verified fixture answer"}}}}, nil
}

func TestAdaptiveSwitchUsesPinnedLeaseAndRevalidatesBeforeCounting(t *testing.T) {
	for _, tc := range []struct {
		name              string
		revoke, canonical bool
	}{
		{"valid", false, false}, {"changed_evidence", true, false},
		{"conversation", false, true}, {"conversation_changed_evidence", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			revoke := tc.revoke
			dir := t.TempDir()
			for _, name := range []string{"primary", "fallback"} {
				if err := os.WriteFile(filepath.Join(dir, name+".gguf"), []byte("isolated model fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			registry := models.NewRegistry(dir)
			if err := registry.ScanModels(); err != nil {
				t.Fatal(err)
			}
			process, engine := &pinnedContextProcess{}, &switchEngineFixture{}
			s := &Server{config: &config.Config{MaxContextSize: 32768}, registry: registry, llamaProcess: process, engine: engine, inferenceLifecycle: inference.NewLifecycleGate(1)}
			store := models.NewCapabilityEvidenceStore(t.TempDir())
			targets := map[string]*models.CapabilityEvidenceTarget{}
			for i, name := range []string{"primary", "fallback"} {
				fingerprint := models.CapabilityFingerprint{ModelSHA256: strings.Repeat("a", 64), TemplateSHA256: strings.Repeat("b", 64), RuntimeSHA256: strings.Repeat("c", 64), RuntimeConfigSHA256: strings.Repeat("d", 64), Quantization: "fixture", RuntimeBuild: "fixture", Adapter: inference.NativeAgentToolAdapter, ContextWindow: 8192 << i, Backend: "cpu", HardwareID: "fixture"}
				q := &models.WorkflowQualification{SuiteSHA256: strings.Repeat("e", 64), Platform: "fixture", ArtifactID: strings.Repeat("f", 64), Cases: 30, Repetitions: 3, Verified: 90}
				targets[name] = &models.CapabilityEvidenceTarget{Fingerprint: fingerprint, SuiteSHA256: q.SuiteSHA256, Platform: q.Platform}
				if _, err := store.Record(context.Background(), models.CapabilityEvidence{Version: 1, ModelID: name, CheckID: inference.NativeAgentToolAdapter, Fingerprint: fingerprint, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now(), Qualification: q}); err != nil {
					t.Fatal(err)
				}
			}
			lookup := func(_ context.Context, name string, _ int) (*models.CapabilityEvidenceTarget, error) {
				return targets[name], nil
			}
			var err error
			s.agentToolPolicy, err = inference.NewAgentToolPolicy(store, lookup, lookup)
			if err != nil {
				t.Fatal(err)
			}
			settings := agents.ProfileSettings{Model: "primary", ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "adaptive", FallbackModels: []string{"fallback"}}}
			selection, issue := s.resolveProfileModelPolicy(context.Background(), settings, "primary", func(name string) int { return targets[name].Fingerprint.ContextWindow })
			if issue != nil {
				t.Fatal(issue)
			}
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst, cfg.DisableDelegation = true, true
			cfg.ModelSelection, cfg.ContextWindow, cfg.ToolAdapter = selection, 8192, selection.Candidates[0].ToolAdapter
			manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			runner := agents.NewRunner(manager, agents.NewToolRegistry(), s.callAgentModel)
			runner.ContextWindow, runner.CountContext = s.taskContextWindow, s.countAgentContext
			if revoke {
				engine.revoke = func() { targets["fallback"].Fingerprint.RuntimeBuild = "changed" }
			}
			var task *agents.Task
			if tc.canonical {
				scope := agents.ControlScope{Workspace: "switch-fixture", Actor: "alice"}
				created, createErr := manager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "create", Kind: "create_conversation"})
				if createErr != nil {
					t.Fatal(createErr)
				}
				cfg.ConversationConfiguration = &agents.ConversationConfiguration{Version: 1, Model: "primary"}
				admitted, admitErr := runner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: created.Conversation.ID, ExpectedRevision: created.Conversation.Revision, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Reply briefly."}}}, "primary", cfg)
				if admitErr != nil {
					t.Fatal(admitErr)
				}
				task, _ = manager.GetTask(admitted.Work.RunID)
			} else {
				task, err = runner.Create("Reply briefly.", "primary", "alice", cfg)
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if tc.canonical && result != nil && result.Config.ConversationConfiguration.Model != "primary" {
				t.Fatal("switch rewrote accepted conversation choice")
			}
			if err != nil || result.Model != "fallback" || len(result.Checkpoint.ModelSwitches) != 1 || !reflect.DeepEqual(process.allocations, []int{8192, 16384}) {
				t.Fatalf("lifecycle not pinned: %+v allocations=%v err=%v", result, process.allocations, err)
			}
			if revoke {
				if result.Status != agents.TaskFailed || !reflect.DeepEqual(engine.generated, []string{"primary"}) || !reflect.DeepEqual(engine.counted, []string{"primary"}) {
					t.Fatal("changed qualification reached fallback inference", result.Status, engine.generated, engine.counted)
				}
			} else if result.Status != agents.TaskCompleted || !reflect.DeepEqual(engine.generated, []string{"primary", "fallback"}) || !reflect.DeepEqual(engine.counted, []string{"primary", "fallback"}) {
				t.Fatal("qualified fallback did not complete through shared lease", result.Status, engine.generated, engine.counted)
			}
		})
	}
}
