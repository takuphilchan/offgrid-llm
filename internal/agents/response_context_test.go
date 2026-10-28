package agents

import (
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"strings"
	"testing"
)

func TestWorkspaceUpgradeGuidanceRequiresOfferedAccessTool(t *testing.T) {
	task := &Task{Checkpoint: &Checkpoint{}, Config: AgentConfig{ExecutionRequestVersion: 1, ExecutionWorkspace: &execution.Resource{Permissions: execution.Permissions{Read: true}}}}
	tools := []api.Tool{{Function: api.FunctionDef{Name: execution.AccessRequestTool}}}
	text := responseContext(task, tools)
	if !strings.Contains(text, "CURRENT access") || !strings.Contains(text, "edit enables") || !strings.Contains(text, "trusted enables") {
		t.Fatal("missing upgrade explanation")
	}
	if strings.Contains(responseContext(task, nil), "edit enables") {
		t.Fatal("suggested unavailable access request")
	}
	if task.Config.ExecutionWorkspace.Permissions.Write || task.Config.ExecutionWorkspace.Permissions.Processes {
		t.Fatal("presentation changed authority")
	}
}

func TestResponseContextKeepsRequestAndEvidenceSeparate(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	cfg := taskConfig()
	cfg.ExecutionRequestVersion = 1
	cfg.ContextWindow = 8192
	cfg.MaxTokens = 512
	task, err := r.Create("Tell me a story about café 日本語.", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call", Function: api.FunctionCall{Name: "workspace_read", Arguments: `{"path":"story.txt"}`}}}}, api.ChatMessage{Role: "tool", ToolCallID: "call", Content: "Rejected. Use workspace_list to read directories."})
	messages, err := r.prepareContext(task, []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "workspace_read"}}})
	if err != nil {
		t.Fatal(err)
	}
	if messages[len(messages)-1].StringContent() != task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1].StringContent() || messages[1].StringContent() != task.Prompt {
		t.Fatal("original request/evidence changed")
	}
	index := messages[len(messages)-3].StringContent()
	for _, want := range []string{task.Prompt, "workspace_read", "not a demonstration", "No local workspace is attached", "Do not return repair instructions"} {
		if !strings.Contains(index, want) {
			t.Fatal("missing response context", want)
		}
	}
	if strings.Contains(index, "workspace_process_start") {
		t.Fatal("invented capability")
	}
}

func TestWorkspaceAttachmentIsNotAnOperationResult(t *testing.T) {
	cfg := taskConfig()
	cfg.ExecutionRequestVersion = 1
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("List the selected folder", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "tool", Name: "request_workspace_access", Content: `{"status":"workspace_attached","operation_executed":false}`})
	before := task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1]
	text := responseContext(task, nil)
	if !strings.Contains(text, "NOT a directory listing") || !strings.Contains(text, task.Prompt) || task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1].StringContent() != before.StringContent() {
		t.Fatal("attachment lost purpose or altered evidence")
	}
	task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1].Name = "workspace_list"
	if strings.Contains(responseContext(task, nil), "has just been attached") {
		t.Fatal("applied setup guidance after an actual observation")
	}
}
