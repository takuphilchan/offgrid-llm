package agents

import (
	"context"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"testing"
)

func TestComputerRejectsBatchedAndUnobservedCallsBeforeDispatch(t *testing.T) {
	for _, names := range [][]string{{"browser_observe", "browser_verify"}, {"browser_click"}} {
		manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
		registry := NewToolRegistry()
		caller := func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
			calls := []api.ToolCall{}
			for _, name := range names {
				calls = append(calls, api.ToolCall{ID: name, Type: "function", Function: api.FunctionCall{Name: name, Arguments: `{}`}})
			}
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{ToolCalls: calls}}}}, nil
		}
		runner := NewRunner(manager, registry, caller)
		config := DefaultAgentConfig()
		config.ContextWindow = 16384
		config.ComputerSession = "paired"
		config.ComputerExpectedText = "Saved"
		task, err := runner.Create("Save draft", "test", "alice", config)
		if err != nil {
			t.Fatal(err)
		}
		task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
		wantStatus, wantSteps := TaskInterrupted, (maxProposalRepairs+1)*len(names)
		if err != nil || task.Status != wantStatus || len(task.Steps) != wantSteps || task.PendingApproval != nil || len(task.Checkpoint.Calls) != 0 {
			t.Fatalf("unsafe response not rejected: %+v %v", task, err)
		}
		for _, step := range task.Steps {
			if step.Type != "rejected" {
				t.Fatal("batch reached dispatch")
			}
		}
	}
}
