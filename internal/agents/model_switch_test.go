package agents

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func switchFixtureConfig() AgentConfig {
	config := DefaultAgentConfig()
	config.TaskFirst, config.DisableDelegation = true, true
	config.ContextWindow = 8192
	config.ModelSelection = &ModelPolicySelection{Version: 1, Mode: "adaptive", Primary: "primary", Selected: "primary", Reason: "primary_model"}
	for i, name := range []string{"primary", "fallback-one", "fallback-two"} {
		pin := &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, EvidenceID: strings.Repeat(string(rune('a'+i)), 64), FingerprintID: strings.Repeat("f", 64)}
		config.ModelSelection.Candidates = append(config.ModelSelection.Candidates, ModelCandidate{Model: name, ContextWindow: 8192 << i, ToolAdapter: pin})
	}
	config.ToolAdapter = config.ModelSelection.Candidates[0].ToolAdapter
	return config
}

func TestModelSwitchPersistsBeforeInferenceAndKeepsRepairBudget(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{}
	var models []string
	runner := NewRunner(manager, tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		models = append(models, task.Model)
		saved, ok := manager.GetTask(task.ID)
		if !ok || saved.Model != task.Model || saved.Config.ContextWindow != 8192<<(len(models)-1) || len(saved.Checkpoint.ModelSwitches) != len(models)-1 || saved.Checkpoint.DecisionRepairs != len(models)-1 || saved.Checkpoint.BudgetedModelCalls != len(models) {
			t.Fatalf("switch or cumulative budget not persisted before inference: call=%d model=%s window=%d switches=%d repairs=%d budget=%d", len(models), saved.Model, saved.Config.ContextWindow, len(saved.Checkpoint.ModelSwitches), saved.Checkpoint.DecisionRepairs, saved.Checkpoint.BudgetedModelCalls)
		}
		if *saved.Config.ToolAdapter != *saved.Config.ModelSelection.Candidates[len(models)-1].ToolAdapter {
			t.Fatal("destination evidence changed")
		}
		return nil, rejectedDecision()
	})
	task, err := runner.Create("Keep all original constraints.", "primary", "alice", switchFixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskInterrupted || task.ErrorCode != "model_decision_repair_limit" || !reflect.DeepEqual(models, []string{"primary", "fallback-one", "fallback-two"}) || tools.calls.Load() != 0 || task.Checkpoint.Iteration != 3 {
		t.Fatalf("bounded adaptation failed: %+v models=%v err=%v", task, models, err)
	}
	_, err = runner.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || len(models) != 3 {
		t.Fatal("explicit resume replenished exhausted correction budget", err)
	}
}

func TestModelSwitchRestartKeepsSelectionAndCharges(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return nil, rejectedDecision()
	})
	runner.Observer = func(task *Task) {
		if len(task.Checkpoint.ModelSwitches) == 1 {
			cancel()
		}
	}
	task, err := runner.Create("preserve this request", "primary", "alice", switchFixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = runner.Continue(ctx, task.ID, "alice", "start", "", false)
	restarted := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, ok := restarted.GetTask(task.ID)
	if !ok || saved.Model != "fallback-one" || saved.Status != TaskInterrupted || saved.Checkpoint.DecisionRepairs != 1 || len(saved.Checkpoint.ModelSwitches) != 1 || saved.Checkpoint.BudgetedModelCalls != 1 {
		t.Fatalf("lost durable switch: %+v", saved)
	}
	calls := 0
	resumed := NewRunner(restarted, &runTestTools{}, func(_ context.Context, current *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if current.Model != "fallback-one" || current.Checkpoint.DecisionRepairs != 1 || current.Checkpoint.BudgetedModelCalls != 2 || messages[1].StringContent() != "preserve this request" {
			t.Fatal("restart reset model, context or budget")
		}
		return runAnswer(api.ChatMessage{Content: "Done"}), nil
	})
	finished, err := resumed.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || finished.Status != TaskCompleted || calls != 1 || len(finished.Checkpoint.ModelSwitches) != 1 {
		t.Fatalf("resume: %+v %v", finished, err)
	}
}

func TestModelSwitchDoesNotTreatOperationalErrorsAsRepair(t *testing.T) {
	for _, failure := range []error{errors.New("model missing"), errors.Join(rejectedDecision(), errors.New("failed persistence")), context.Canceled} {
		manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
		calls := 0
		runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
			calls++
			return nil, failure
		})
		task, err := runner.Create("fixture", "primary", "alice", switchFixtureConfig())
		if err != nil {
			t.Fatal(err)
		}
		_, _ = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
		saved, _ := manager.GetTask(task.ID)
		if calls != 1 || saved.Model != "primary" || len(saved.Checkpoint.ModelSwitches) != 0 {
			t.Fatal("operational failure switched models")
		}
	}
}

func TestModelSwitchScopeAndSavedPolicyRejections(t *testing.T) {
	for name, mutate := range map[string]func(*AgentConfig){
		"version":     func(c *AgentConfig) { c.ModelSelection.Version = 99 },
		"selection":   func(c *AgentConfig) { c.ModelSelection.Selected = "other" },
		"missing_pin": func(c *AgentConfig) { c.ModelSelection.Candidates[1].ToolAdapter = nil },
		"scope": func(c *AgentConfig) {
			c.ModelSelection.Candidates[1].ToolAdapter.Scope = "native-workspace-v1"
			c.ModelSelection.Candidates[1].ToolAdapter.CapabilitySHA256 = NativeWorkspaceCapabilitySHA256()
		},
	} {
		t.Run(name, func(t *testing.T) {
			manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			config := switchFixtureConfig()
			mutate(&config)
			calls := 0
			runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				return nil, rejectedDecision()
			})
			task, err := runner.Create("fixture", "primary", "alice", config)
			if err != nil {
				t.Fatal(err)
			}
			got, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
			wantCalls := 0
			if name == "scope" {
				wantCalls = 1
			}
			if err != nil || calls != wantCalls || got.Status != TaskInterrupted || got.Model != "primary" || len(got.Checkpoint.ModelSwitches) != 0 {
				t.Fatalf("invalid policy executed fallback: %+v %v", got, err)
			}
		})
	}
}

func TestModelSwitchUnsafeOrUnapprovedBoundary(t *testing.T) {
	for name, mutate := range map[string]func(*Task){
		"child":   func(task *Task) { task.ParentID = "root" },
		"guided":  func(task *Task) { task.Config.ModelSelection.Mode = "guided" },
		"effect":  func(task *Task) { task.Checkpoint.ExecutingCall = "uncertain" },
		"receipt": func(task *Task) { task.Checkpoint.ComputerReceipt = &ComputerReceipt{} },
		"read":    func(task *Task) { task.Checkpoint.ReadOnlyCall = "unsettled" },
		"control": func(task *Task) { task.ControlPending = true },
		"queued":  func(task *Task) { task.Checkpoint.Calls = []api.ToolCall{{ID: "undispatched"}} },
		"stopped": func(task *Task) { task.Status = TaskCancelled },
	} {
		t.Run(name, func(t *testing.T) {
			task := &Task{Model: "primary", Status: TaskRunning, Config: switchFixtureConfig(), Checkpoint: &Checkpoint{Iteration: 1, DecisionRepairs: 1}}
			mutate(task)
			adaptRejectedDecision(task)
			if task.Model != "primary" || len(task.Checkpoint.ModelSwitches) != 0 {
				t.Fatal("unsafe switch")
			}
		})
	}
}
