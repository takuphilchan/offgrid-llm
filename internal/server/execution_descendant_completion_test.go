package server

import (
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestExecutionCompletionRequiresDescendantProofBeyondLeaderExit(t *testing.T) {
	zero := 0
	for _, proof := range []*execution.ProcessDescendants{nil, {Version: 1, State: "unknown"}, {Version: 2, State: "settled"}, {Version: 1, State: "invalid"}, {Version: 1, State: "settled"}} {
		process, _ := json.Marshal(execution.ProcessResult{Handle: "owned", State: "exited", ExitCode: &zero, Descendants: proof})
		result, _ := json.Marshal(map[string]any{"receipt": execution.Receipt{State: "complete", Result: process}})
		task := &agents.Task{Steps: []agents.Step{{Type: "action", ToolName: "workspace_process_start", ToolResult: string(result)}}}
		err := (&browserRunTools{}).ValidateExecutionCompletion(task)
		valid := proof != nil && proof.Version == 1 && proof.State == "settled"
		if (err == nil) != valid {
			t.Fatal("leader exit became verified task result", proof, err)
		}
	}
}

func TestExecutionCompletionRequiresSameStartedOwnerAndCompleteReceipt(t *testing.T) {
	zero := 0
	step := func(name, host, session, receiptState, state string) agents.Step {
		process, _ := json.Marshal(execution.ProcessResult{Handle: "same-handle", State: state, ExitCode: &zero, Descendants: &execution.ProcessDescendants{Version: 1, State: "settled"}})
		result, _ := json.Marshal(map[string]any{"host": host, "session": session, "receipt": execution.Receipt{State: receiptState, Result: process}})
		return agents.Step{Type: "action", ToolName: name, ToolResult: string(result)}
	}
	for _, tc := range []struct{ name, host, session, receipt string }{
		{"foreign-host", "other-host", "session", "complete"},
		{"foreign-session", "host", "other-session", "complete"},
		{"uncertain-receipt", "host", "session", "uncertain"},
		{"dispatched-receipt", "host", "session", "dispatched"},
		{"rejected-receipt", "host", "session", "rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &agents.Task{Steps: []agents.Step{step("workspace_process_start", "host", "session", "dispatched", "running"), step("workspace_process_wait", tc.host, tc.session, tc.receipt, "exited")}}
			if (&browserRunTools{}).ValidateExecutionCompletion(task) == nil {
				t.Fatal("foreign or unconfirmed observation completed the started process")
			}
		})
	}
	task := &agents.Task{Steps: []agents.Step{
		step("workspace_process_start", "host", "session", "dispatched", "running"),
		step("workspace_process_start", "other-host", "other-session", "dispatched", "running"),
		step("workspace_process_wait", "other-host", "other-session", "complete", "exited"),
	}}
	if (&browserRunTools{}).ValidateExecutionCompletion(task) == nil {
		t.Fatal("second owner's reused handle completed the first owner's process")
	}
}
