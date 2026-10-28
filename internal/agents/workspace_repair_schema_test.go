package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func repairSchemaFixture(t *testing.T) (*Task, []api.Tool) {
	t.Helper()
	ref := execution.Reference{Host: "host", Session: "session"}
	raw, _ := json.Marshal(execution.FilePage{Location: execution.Location{Host: "host"}, Revision: strings.Repeat("a", 64), Data: []byte("old"), Complete: true})
	read, err := execution.PresentReceipt(ref, "file_read", execution.Receipt{State: "complete", Result: raw})
	if err != nil {
		t.Fatal(err)
	}
	return &Task{Prompt: "<content>new</content>", Config: AgentConfig{Execution: &ref}, Checkpoint: &Checkpoint{PreEffectRepair: true, PreEffectRepairTool: execution.InputWriteTool}, Steps: []Step{
		{Type: "rejected", ToolName: execution.InputWriteTool, ToolArgs: `{"path":"source.txt","revision":"","input":"request","after":"<content>","before":"</content>"}`, ToolResult: `{"error":"execution_write_precondition","effects":"none"}`},
		{Type: "action", ToolName: "workspace_read", ToolArgs: `{"path":"./source.txt"}`, ToolResult: read},
	}}, execution.ModelTools()
}

func TestWorkspaceWriteRepairSchemaRequiresObservedRevision(t *testing.T) {
	task, tools := repairSchemaFixture(t)
	before, _ := json.Marshal(tools)
	state, _ := json.Marshal(task)
	projected := projectWorkspaceWriteRepair(task, tools)
	if len(projected) != 1 || projected[0].Function.Name != execution.InputWriteTool {
		t.Fatal("unrelated or repeated-read tools offered during exact-input repair", projected)
	}
	request := &api.ChatCompletionRequest{Tools: projected}
	for _, tc := range []struct {
		path, revision, after string
		valid                 bool
	}{
		{"source.txt", strings.Repeat("a", 64), "<content>", true}, {"source.txt", "", "<content>", false}, {"other.txt", strings.Repeat("a", 64), "<content>", false}, {"source.txt", strings.Repeat("b", 64), "<content>", false}, {"source.txt", strings.Repeat("a", 64), "<wrong>", false},
	} {
		args, _ := json.Marshal(map[string]string{"path": tc.path, "revision": tc.revision, "input": "request", "after": tc.after, "before": "</content>"})
		call := api.ToolCall{ID: "new", Type: "function", Function: api.FunctionCall{Name: execution.InputWriteTool, Arguments: string(args)}}
		message := api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}}
		response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Index: 0, FinishReason: "tool_calls", Message: message}}}
		if err := inference.ValidateToolDecision(request, response); (err == nil) != tc.valid {
			t.Fatal("incorrect repair admission", tc, err)
		}
	}
	after, _ := json.Marshal(tools)
	saved, _ := json.Marshal(task)
	if string(after) != string(before) || string(saved) != string(state) {
		t.Fatal("projection changed catalog, authority or saved calls")
	}
}

func TestWorkspaceWriteRepairSchemaCannotInventPreimage(t *testing.T) {
	for _, name := range []string{"not_repairing", "no_read", "different_target", "foreign_host", "foreign_session", "malformed_revision", "uncertain_write", "intervening_write", "earlier_read"} {
		t.Run(name, func(t *testing.T) {
			task, tools := repairSchemaFixture(t)
			switch name {
			case "not_repairing":
				task.Checkpoint.PreEffectRepair = false
			case "no_read":
				task.Steps = task.Steps[:1]
			case "different_target":
				task.Steps[1].ToolArgs = `{"path":"other.txt"}`
			case "foreign_host":
				task.Config.Execution.Host = "different"
			case "foreign_session":
				task.Config.Execution.Session = "different"
			case "malformed_revision":
				task.Steps[1].ToolResult = strings.ReplaceAll(task.Steps[1].ToolResult, strings.Repeat("a", 64), "invalid")
			case "uncertain_write":
				task.Steps[0].ToolResult = `{"error":"execution_write_precondition","effects":"unknown"}`
			case "intervening_write":
				task.Steps = append(task.Steps, Step{Type: "action", ToolName: "workspace_write"})
			case "earlier_read":
				task.Steps[0], task.Steps[1] = task.Steps[1], task.Steps[0]
			}
			before, _ := json.Marshal(tools)
			after, _ := json.Marshal(projectWorkspaceWriteRepair(task, tools))
			if string(after) != string(before) {
				t.Fatal("unobserved preimage admitted")
			}
		})
	}
}
