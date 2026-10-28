package server

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestExecutionCompletionPendingIdentifiesCurrentProcess(t *testing.T) {
	ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
	step := func(name, session, handle, state, receiptState string, exit *int) agents.Step {
		result, _ := json.Marshal(execution.ProcessResult{Handle: handle, State: state, ExitCode: exit})
		data, _ := json.Marshal(map[string]any{"host": ref.Host, "session": session, "receipt": execution.Receipt{State: receiptState, Result: result}})
		return agents.Step{Type: "action", ToolName: name, ToolResult: string(data)}
	}
	for _, session := range []string{"session", "foreign"} {
		task := &agents.Task{Config: agents.AgentConfig{Execution: &ref}, Steps: []agents.Step{step("workspace_process_start", session, "owned", "running", "dispatched", nil)}}
		var pending *agents.ExecutionPending
		if !errors.As((&browserRunTools{}).ValidateExecutionCompletion(task), &pending) || pending.Handle != "owned" {
			t.Fatal("missing pending state")
		}
		if (pending.Reference == ref) != (session == ref.Session) {
			t.Fatal("foreign process promoted to current session")
		}
		zero := 0
		task.Steps = append(task.Steps, step("workspace_process_wait", session, "owned", "exited", "complete", &zero))
		if err := (&browserRunTools{}).ValidateExecutionCompletion(task); err != nil {
			t.Fatal("recorded terminal outcome rejected", err)
		}
	}
	// A forged observation without a start cannot introduce an owned handle.
	task := &agents.Task{Config: agents.AgentConfig{Execution: &ref}, Steps: []agents.Step{step("workspace_process_wait", ref.Session, "unknown", "running", "complete", nil)}}
	var pending *agents.ExecutionPending
	if err := (&browserRunTools{}).ValidateExecutionCompletion(task); err == nil || errors.As(err, &pending) {
		t.Fatal("unknown handle scheduled")
	}
}

func TestEditAccessCannotCompleteAfterOnlyReading(t *testing.T) {
	ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
	grant, _ := json.Marshal(map[string]any{"execution": ref, "status": "workspace_attached", "requested_access": "edit", "permissions": execution.Permissions{Read: true, Write: true, Processes: true}})
	task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, ExecutionRequestVersion: execution.AccessRequestVersion, Execution: &ref}, Steps: []agents.Step{{Type: "input_resolved", ToolName: "request_workspace_access", ToolResult: string(grant)}}}
	var pending *agents.ExecutionPending
	if err := (&browserRunTools{}).ValidateExecutionCompletion(task); !errors.As(err, &pending) || pending.Kind != "workspace_edit" || pending.Reference != ref || pending.Handle != "" {
		t.Fatal("edit without a write reported complete", err)
	}
	read, _ := json.Marshal(map[string]any{"host": ref.Host, "session": ref.Session, "receipt": execution.Receipt{State: "complete"}})
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "workspace_read", ToolResult: string(read)})
	if err := (&browserRunTools{}).ValidateExecutionCompletion(task); !errors.As(err, &pending) {
		t.Fatal("reading did not leave edit outstanding", err)
	}
	written, _ := json.Marshal(execution.WriteResult{Location: execution.Location{Host: ref.Host}, Verified: true, Bytes: 5})
	for _, state := range []string{"rejected", "complete"} {
		view, _ := json.Marshal(map[string]any{"host": ref.Host, "session": ref.Session, "receipt": execution.Receipt{State: state, Result: written}})
		task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "workspace_write", ToolResult: string(view)})
		err := (&browserRunTools{}).ValidateExecutionCompletion(task)
		if (err == nil) != (state == "complete") {
			t.Fatal("incorrect write outcome admission", state, err)
		}
		task.Steps = task.Steps[:len(task.Steps)-1]
	}
	// A different host's receipt and a revoked/different execution identity
	// cannot satisfy the original edit request.
	view, _ := json.Marshal(map[string]any{"host": "foreign", "session": ref.Session, "receipt": execution.Receipt{State: "complete", Result: written}})
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "workspace_write", ToolResult: string(view)})
	if err := (&browserRunTools{}).ValidateExecutionCompletion(task); !errors.As(err, &pending) {
		t.Fatal("foreign write satisfied edit", err)
	}
	task.Steps[0].ToolResult = strings.Replace(string(grant), `"edit"`, `"read"`, 1)
	if err := (&browserRunTools{}).ValidateExecutionCompletion(task); err != nil {
		t.Fatal("read request treated as edit merely because the client granted broader authority", err)
	}
}
