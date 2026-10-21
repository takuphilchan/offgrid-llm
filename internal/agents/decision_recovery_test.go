package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func rejectedDecision() error {
	return inference.ValidateToolDecision(&api.ChatCompletionRequest{}, runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{ID: "fixture", Type: "function", Function: api.FunctionCall{Name: "not_registered", Arguments: `{"private":"not retained"}`}}}}))
}

func TestModelDecisionRecoveryBoundedAndDurable(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	tools := &runTestTools{}
	calls := 0
	runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return nil, rejectedDecision()
	})
	task, err := runner.Create("fixture", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskInterrupted || task.ErrorCode != "model_decision_repair_limit" || calls != 3 || tools.calls.Load() != 0 {
		t.Fatalf("invalid recovery: %+v calls=%d err=%v", task, calls, err)
	}
	if task.PendingApproval != nil || len(task.Checkpoint.Calls) != 0 || task.Checkpoint.Iteration != 3 || task.Checkpoint.DecisionRepairs != 3 {
		t.Fatal("rejected output became executable or escaped iteration budget")
	}
	for _, message := range task.Checkpoint.Messages {
		if strings.Contains(message.StringContent(), "not retained") || len(message.ToolCalls) != 0 {
			t.Fatal("rejected arguments entered context")
		}
	}
	restarted := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, ok := restarted.GetTask(task.ID)
	if !ok || saved.Checkpoint.DecisionRepairs != 3 || saved.Status != TaskInterrupted {
		t.Fatal("restart lost correction boundary", saved)
	}
}

func TestModelDecisionRecoveryAllowsCorrectedConversation(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{}
	calls := 0
	runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Hello"}), nil
	})
	task, err := runner.Create("hello", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskCompleted || task.Result != "Hello" || calls != 2 || tools.calls.Load() != 0 || task.Checkpoint.Iteration != 2 || task.Checkpoint.DecisionRepairs != 0 {
		t.Fatalf("corrected conversation failed: %+v %v", task, err)
	}
}

func TestModelDecisionRecoveryNeverRetriesOperationalOrPersistenceErrors(t *testing.T) {
	for _, failure := range []error{errors.New(rejectedDecision().Error()), errors.Join(rejectedDecision(), errors.New("persistence failed")), context.Canceled} {
		manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
		tools := &runTestTools{}
		calls := 0
		runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
			calls++
			return nil, failure
		})
		task, err := runner.Create("fixture", "model", "alice", DefaultAgentConfig())
		if err != nil {
			t.Fatal(err)
		}
		_, _ = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
		if calls != 1 || tools.calls.Load() != 0 {
			t.Fatal("non-classified failure retried", failure)
		}
	}
}
