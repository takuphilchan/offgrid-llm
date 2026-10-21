package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestOverBudgetModelDecisionCannotDispatchTool(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	tools := &runTestTools{}
	runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		response := runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: `{}`}}}})
		response.Usage = api.Usage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8}
		return response, nil
	})
	config := DefaultAgentConfig()
	config.MaxTokens = 4
	task, err := runner.Create("write a file", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskFailed || !strings.Contains(task.Error, "output token budget") || tools.calls.Load() != 0 || task.PendingApproval != nil {
		t.Fatalf("over-budget decision was not rejected before dispatch: task=%+v calls=%d err=%v", task, tools.calls.Load(), err)
	}
	stored, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
	if !ok || stored.Status != TaskFailed || stored.Checkpoint.Iteration != 0 {
		t.Fatalf("over-budget rejection was not durable: %+v", stored)
	}
}

func TestDecisionUsageIsDurableAndChildReservationsConstrainAggregateBudget(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		response := runAnswer(api.ChatMessage{Role: "assistant", Content: "Done"})
		response.Usage = api.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}
		return response, nil
	})
	config := DefaultAgentConfig()
	config.MaxTokens = 4
	task, err := runner.Create("answer", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskCompleted {
		t.Fatalf("measured decision failed: %+v, %v", task, err)
	}
	stored, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
	if !ok || stored.Checkpoint.MeasuredCompletionTokens != 2 || stored.Checkpoint.UnmeasuredModelCalls != 0 {
		t.Fatalf("measured usage was lost on restart: %+v", stored)
	}

	root := &Task{Config: AgentConfig{MaxIterations: 3, MaxTokens: 4}, Checkpoint: &Checkpoint{ReservedChildIterations: 2, MeasuredCompletionTokens: 3}}
	if err := recordDecisionUsage(root, api.Usage{CompletionTokens: 2}); err == nil || root.Checkpoint.MeasuredCompletionTokens != 3 {
		t.Fatalf("child reservation did not protect aggregate output budget: %+v, %v", root.Checkpoint, err)
	}
	if err := recordDecisionUsage(root, api.Usage{}); err != nil || root.Checkpoint.UnmeasuredModelCalls != 1 {
		t.Fatalf("missing usage was misrepresented as measured zero: %+v, %v", root.Checkpoint, err)
	}
}
