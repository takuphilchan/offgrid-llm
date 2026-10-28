package server

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkspacePageDecisionsUseOnlyRecordedContinuations(t *testing.T) {
	for _, name := range []string{"workspace_list", "workspace_read"} {
		var base api.Tool
		for _, tool := range execution.Tools() {
			if tool.Function.Name == name {
				base = tool
			}
		}
		revision := strings.Repeat("a", 64)
		step := func(complete bool, host, session string) agents.Step {
			var next any
			if !complete {
				next = 2
			}
			data, _ := json.Marshal(map[string]any{"host": host, "session": session, "operation": executionOperation(name), "receipt": map[string]any{"state": "complete", "result": map[string]any{"revision": revision, "next_offset": next, "complete": complete}}})
			return agents.Step{Type: "action", ToolName: name, ToolArgs: `{"path":"."}`, ToolResult: string(data)}
		}
		for _, mode := range []string{"new", "complete", "partial", "finished", "mutated", "wrong-host", "wrong-session"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				task := &agents.Task{Config: agents.AgentConfig{Execution: &execution.Reference{Host: "host", Session: "session"}}}
				switch mode {
				case "complete":
					task.Steps = []agents.Step{step(true, "host", "session")}
				case "partial":
					task.Steps = []agents.Step{step(false, "host", "session")}
				case "finished":
					task.Steps = []agents.Step{step(false, "host", "session"), step(true, "host", "session")}
				case "mutated":
					task.Steps = []agents.Step{step(false, "host", "session"), {Type: "action", ToolName: "workspace_change_directory"}}
				case "wrong-host":
					task.Steps = []agents.Step{step(false, "other", "session")}
				case "wrong-session":
					task.Steps = []agents.Step{step(false, "host", "other")}
				}
				before, _ := json.Marshal(base)
				tool := workspacePageTool(base, task)
				after, _ := json.Marshal(base)
				if string(before) != string(after) {
					t.Fatal("base catalog mutated")
				}
				for _, tc := range []struct {
					path     string
					offset   int
					revision string
					valid    bool
				}{
					{".", 0, "", true}, {".", 2, revision, mode == "partial"}, {"child", 2, revision, false}, {".", 1, revision, false}, {".", 2, strings.Repeat("b", 64), false},
				} {
					args, _ := json.Marshal(map[string]any{"path": tc.path, "offset": tc.offset, "revision": tc.revision})
					response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: name, Arguments: string(args)}}}}, FinishReason: "tool_calls"}}}
					err := inference.ValidateToolDecision(&api.ChatCompletionRequest{Tools: []api.Tool{tool}}, response)
					var repair *inference.DecisionValidationError
					if tc.valid && err != nil || !tc.valid && !errors.As(err, &repair) {
						t.Fatalf("%s: %v", args, err)
					}
				}
			})
		}
	}
}
