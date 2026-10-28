package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestExecutionProgressSeparatesCurrentRequestAndKnownProcessResults(t *testing.T) {
	task := &Task{Prompt: "Run tests; leave files unchanged", Config: AgentConfig{Execution: &execution.Reference{Session: "session"}, ExecutionWorkspace: &execution.Resource{}, ConversationContext: []api.ChatMessage{{Role: "user", Content: "List files"}}}}
	add := func(name, session, handle, state string, exit *int) {
		p, _ := json.Marshal(execution.ProcessResult{Handle: handle, State: state, ExitCode: exit})
		receiptState := "complete"
		if name == "workspace_process_start" {
			receiptState = "dispatched"
		}
		data, _ := json.Marshal(map[string]any{"session": session, "operation": execution.Operation(name), "receipt": execution.Receipt{State: receiptState, Result: p}})
		task.Steps = append(task.Steps, Step{Type: "action", ToolName: name, ToolResult: string(data)})
	}
	zero := 0
	add("workspace_process_start", "session", "owned", "running", nil)
	add("workspace_process_wait", "session", "owned", "exited", &zero)
	add("workspace_process_wait", "session", "invented", "exited", &zero)
	add("workspace_process_start", "foreign", "private", "running", nil)
	before, _ := json.Marshal(task)
	view := executionProgressContext(task)
	for _, want := range []string{`"handle":"owned"`, `"state":"exited"`, `"exit_code":0`} {
		if !strings.Contains(view, want) {
			t.Fatal(view)
		}
	}
	for _, absent := range []string{"List files", task.Prompt, "invented", "private"} {
		if strings.Contains(view, absent) {
			t.Fatal("unrelated evidence entered progress", view)
		}
	}
	after, _ := json.Marshal(task)
	if string(before) != string(after) {
		t.Fatal("projection changed durable state")
	}
	task.Config.Execution = nil
	if executionProgressContext(task) != "" {
		t.Fatal("legacy task gained execution state")
	}
}

func TestExecutionProgressDistinguishesNoStartFromUnresolvedAttempts(t *testing.T) {
	task := &Task{Config: AgentConfig{Execution: &execution.Reference{Session: "session"}, ExecutionWorkspace: &execution.Resource{Permissions: execution.Permissions{Processes: true}}, ConversationContext: []api.ChatMessage{{Role: "assistant", Content: "Tests passed with exit code 0"}}}}
	const absent = "No workspace command has been started in this turn"
	if view := executionProgressContext(task); !strings.Contains(view, absent) || strings.Contains(view, "Tests passed") {
		t.Fatal("conversation prose became process evidence", view)
	}
	for _, kind := range []string{"action", "rejected", "interrupted"} {
		task.Steps = []Step{{Type: kind, ToolName: "workspace_process_start", ToolResult: "unknown"}}
		if strings.Contains(executionProgressContext(task), absent) {
			t.Fatal("unresolved attempt was described as not started", kind)
		}
	}
	// An old attempt outside the bounded display is still an attempt. Never
	// encourage replay just because it no longer fits in the progress summary.
	task.Steps = append(task.Steps, make([]Step, 129)...)
	if strings.Contains(executionProgressContext(task), absent) {
		t.Fatal("old attempt lost")
	}
	for _, name := range []string{"workspace_list", "workspace_read", "workspace_write", execution.InputWriteTool} {
		task.Steps = []Step{{Type: "rejected", ToolName: name}}
		if !strings.Contains(executionProgressContext(task), absent) {
			t.Fatal("file attempt became a command receipt", name)
		}
	}
	task.Config.ExecutionWorkspace.Permissions.Processes = false
	if strings.Contains(executionProgressContext(task), "workspace_process_start") {
		t.Fatal("unavailable command was suggested")
	}
}
