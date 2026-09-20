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
		config.ComputerSession = "paired"
		config.ComputerExpectedText = "Saved"
		task, err := runner.Create("Save draft", "test", "alice", config)
		if err != nil {
			t.Fatal(err)
		}
		task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
		if err != nil || task.Status != TaskFailed || len(task.Steps) != 0 || task.PendingApproval != nil {
			t.Fatalf("unsafe response not rejected: %+v %v", task, err)
		}
	}
}
