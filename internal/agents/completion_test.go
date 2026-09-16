package agents

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestIncompleteCompletionNeverExecutesToolsOrSucceeds(t *testing.T) {
	for _, reason := range []string{"length", "content_filter", "", "unknown"} {
		for _, withTool := range []bool{false, true} {
			name := reason
			if withTool {
				name += "/tool"
			}
			t.Run(name, func(t *testing.T) {
				message := api.ChatMessage{Role: "assistant", Content: "This looks like an answer but is incomplete"}
				if withTool {
					message.ToolCalls = []api.ToolCall{{ID: "complete-json", Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: `{"path":"notes","content":"hello"}`}}}
				}
				response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: reason}}}
				tools := &runTestTools{}
				dir := t.TempDir()
				manager := NewManagerWithPersistence(nil, nil, nil, dir)
				runner := NewRunner(manager, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
					return response, nil
				})
				task, err := runner.Create("make notes", "model", "alice", DefaultAgentConfig())
				if err != nil {
					t.Fatal(err)
				}
				task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
				if err != nil {
					t.Fatal(err)
				}
				if task.Status != TaskFailed || task.Result != "" || task.Error == "" || task.PendingApproval != nil || tools.calls.Load() != 0 {
					t.Fatalf("incomplete output accepted: %+v", task)
				}
				if len(task.Checkpoint.Messages) != 2 || len(task.Checkpoint.Calls) != 0 || len(task.Steps) != 1 || task.Steps[0].Type != "incomplete" {
					t.Fatalf("incomplete output entered completed context: %+v", task)
				}
				// The structured legacy path must enforce the same rule.
				executed := false
				agent := NewStructuredAgent(DefaultAgentConfig(), nil,
					func(context.Context, string, json.RawMessage) (string, error) { executed = true; return "done", nil },
					func(context.Context, []api.ChatMessage, []api.Tool, map[string]interface{}) (*api.ChatCompletionResponse, error) {
						return response, nil
					})
				if answer, err := agent.Run(context.Background(), "make notes"); err == nil || answer != "" || executed {
					t.Fatalf("legacy structured path accepted incomplete output: answer=%q err=%v executed=%v", answer, err, executed)
				}
			})
		}
	}
}

func TestToolCompletionRequiresCalls(t *testing.T) {
	if err := validateCompletion(api.ChatCompletionChoice{FinishReason: "tool_calls"}); err == nil {
		t.Fatal("accepted missing tool call")
	}
}
