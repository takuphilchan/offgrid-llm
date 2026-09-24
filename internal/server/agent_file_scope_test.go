package server

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestForeignFilesystemNeedsLocalAccess(t *testing.T) {
	for _, test := range []struct{ path, os, want string }{
		{`D:\`, "linux", "File Explorer"}, {`d:notes`, "darwin", "File Explorer"},
		{`\\server\share`, "linux", "File Explorer"}, {"file:///D:/notes", "linux", "File Explorer"},
		{"/Users/alice/Documents", "linux", "Finder"}, {"/home/alice", "windows", "Files"},
		{`D:\`, "windows", ""}, {"/tmp/report", "linux", ""}, {"notes/report.txt", "darwin", ""},
	} {
		if got := foreignFileApplication(test.path, test.os); got != test.want {
			t.Errorf("%q on %s: %s", test.path, test.os, got)
		}
	}
}

func TestMisroutedFileCallBecomesSavedConsentNotUnknownOutcome(t *testing.T) {
	s := taskFirstServer(t)
	registry := agents.NewToolRegistry()
	b := &browserRunTools{RunTools: registry, server: s}
	path := `D:\`
	if runtime.GOOS == "windows" {
		path = "/Users/alice/Documents"
	}
	args, _ := json.Marshal(map[string]string{"path": path})
	s.agentRunner = agents.NewRunner(s.agentManager, b, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "list_files", Arguments: string(args)}}}}}}}, nil
	})
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 8192
	task, _ := s.agentRunner.Create("List the folders on my computer", "model", "local-admin", cfg)
	task, err := s.agentRunner.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.Status != agents.TaskInput || task.PendingInput == nil || task.PendingInput.Mode != "app" || task.Checkpoint.ExecutingCall != "" || len(task.Steps) != 0 {
		t.Fatalf("host input was executed: %+v %v", task, err)
	}
	d, _ := b.Capability("list_files")
	execution := agents.ToolExecution{RunID: task.ID, Actor: task.Actor, ExpectedCapability: &d}
	if err = registry.DisableTool("list_files"); err != nil {
		t.Fatal(err)
	}
	if err = b.Authorize(context.Background(), "list_files", args, execution); err == nil {
		t.Fatal("disabled tool bypassed scope")
	}
	if _, ok := err.(*agents.InputRequired); ok {
		t.Fatal("disabled tool requested consent")
	}
}
