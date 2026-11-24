package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDelegateInferencePinsSurviveRestart(t *testing.T) {
	for _, tc := range []struct {
		name               string
		conversation, pool bool
		model              string
		window             int
	}{
		{"conversation inherits", true, false, "primary", 8192},
		{"conversation explicit pool", true, true, "fallback-one", 16384},
		{"direct child profile", false, false, "fallback-one", 16384},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, r, scope, conversation := controlFixture(t)
			config := switchFixtureConfig()
			config.DisableDelegation, config.MaxIterations = false, 16
			config.Profile = &AgentProfile{ID: "parent", Revision: 1, Settings: ProfileSettings{Name: "Parent", Model: "primary", MaxIterations: 16}}
			child := DefaultAgentConfig()
			child.Profile = &AgentProfile{ID: "child", Revision: 1, Settings: ProfileSettings{Name: "Child", Model: "fallback-one", MaxIterations: 8}}
			child.ContextWindow, child.ToolAdapter = 16384, config.ModelSelection.Candidates[1].ToolAdapter
			if tc.pool {
				child.Profile.Settings.ModelPolicy = &ProfileModelPolicy{Version: 1, Mode: "adaptive", FallbackModels: []string{"primary"}}
				child.ModelSelection = &ModelPolicySelection{Version: 1, Mode: "adaptive", Primary: "fallback-one", Selected: "fallback-one", Reason: "primary_model", Candidates: []ModelCandidate{config.ModelSelection.Candidates[1], config.ModelSelection.Candidates[0]}}
			}
			config.DelegateProfiles = map[string]AgentConfig{"child": child}
			var root *Task
			var err error
			if tc.conversation {
				receipt, admissionErr := r.AdmitConversationWork(scope, ConversationCommand{RequestID: "delegate-model", TargetID: conversation.ID, ExpectedRevision: conversation.Revision, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Compare facts"}}}, "primary", config)
				if admissionErr != nil {
					t.Fatal(admissionErr)
				}
				root, _ = m.GetTask(receipt.Work.RunID)
			} else {
				root, err = r.Create("Compare facts", "primary", scope.Actor, config)
				if err != nil {
					t.Fatal(err)
				}
			}
			r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall("delegate_tasks", `{"tasks":[{"key":"review","goal":"Compare facts","depends_on":[],"tools":[],"profile_id":"child"}]}`), nil
			}
			root, err = r.Continue(context.Background(), root.ID, scope.Actor, "start", "", false)
			if err != nil || root.Status != TaskChildren || len(root.ChildHistory) != 1 {
				t.Fatalf("delegation: %+v %v", root, err)
			}
			worker, _ := m.GetTask(root.ChildHistory[0].ID)
			if worker.Model != tc.model || worker.Config.ContextWindow != tc.window || worker.Config.ToolAdapter == nil || !worker.Config.TaskFirst || !strings.Contains(worker.Checkpoint.Messages[0].StringContent(), nativeDecisionInstruction) {
				t.Fatal("worker lost qualified inference contract", worker)
			}
			if !worker.Config.DisableDelegation || worker.Config.ComputerSession != "" || len(worker.Config.AllowedCapabilities) != 0 {
				t.Fatal("worker expanded authority")
			}
			if tc.conversation && !tc.pool && (worker.Config.ModelSelection == nil || worker.Config.ModelSelection.Reason != "inherited_qualified_model") {
				t.Fatal("inherited model provenance missing")
			}
			r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				if task.Model != tc.model || task.Config.ContextWindow != tc.window {
					t.Fatal("inference differs from admitted worker")
				}
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "Compared the supplied facts."}), nil
			}
			worker, err = r.Continue(context.Background(), worker.ID, scope.Actor, "start", "", false)
			if err != nil || worker.Status != TaskCompleted {
				t.Fatalf("worker execution: %+v %v", worker, err)
			}
			if worker.Checkpoint.InputBudget.Window != 8192 || worker.Checkpoint.InputBudget.ChargedTokens != int64(tc.window) {
				t.Fatal("child widened aggregate token unit", worker.Checkpoint.InputBudget)
			}
			reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err := reloaded.StorageError(); err != nil {
				t.Fatal(err)
			}
			saved, _ := reloaded.GetTask(worker.ID)
			if saved.Model != tc.model || saved.Config.ContextWindow != tc.window || *saved.Config.ToolAdapter != *worker.Config.ToolAdapter {
				t.Fatal("restart changed admitted worker")
			}
			if worker.Config.ModelSelection != nil {
				worker.Config.ModelSelection.Candidates[0].Model = "mutated"
				parent, _ := m.GetTask(root.ID)
				if parent.Config.ModelSelection.Candidates[0].Model != "primary" {
					t.Fatal("child aliases parent policy")
				}
			}
			if tc.conversation {
				session, err := m.WorkerSession(scope, worker.Worker.SessionID)
				if err != nil {
					t.Fatal(err)
				}
				continued, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "model-followup", TargetID: session.ID, ExpectedRevision: session.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Expand the comparison"}})
				if err != nil {
					t.Fatal(err)
				}
				next, err := r.Continue(context.Background(), continued.Worker.CurrentRunID, scope.Actor, "start", "", false)
				if err != nil || next.Status != TaskCompleted || next.Model != tc.model || next.Checkpoint.InputBudget.Window != 8192 || next.Checkpoint.InputBudget.ChargedTokens != int64(tc.window) {
					t.Fatalf("followup changed admission or allowance: %+v %v", next, err)
				}
			}
		})
	}
}

func TestDelegateInferenceRejectsMissingOrMismatchedEvidence(t *testing.T) {
	for _, kind := range []string{"missing", "model", "window", "adapter", "version"} {
		t.Run(kind, func(t *testing.T) {
			parent := &Task{Model: "primary", Config: switchFixtureConfig(), WorkID: "work"}
			selected := switchFixtureConfig()
			selected.Profile = &AgentProfile{Settings: ProfileSettings{Model: "primary", ModelPolicy: &ProfileModelPolicy{Version: 1, Mode: "adaptive"}}}
			switch kind {
			case "missing":
				selected.ModelSelection = nil
			case "model":
				selected.Profile.Settings.Model = "other"
			case "window":
				selected.ContextWindow++
			case "adapter":
				selected.ToolAdapter = nil
			case "version":
				selected.ModelSelection.Version++
			}
			config := DefaultAgentConfig()
			if _, err := configureChildInference(&config, parent, &selected); err == nil {
				t.Fatal("invalid admitted evidence accepted")
			}
		})
	}
}
