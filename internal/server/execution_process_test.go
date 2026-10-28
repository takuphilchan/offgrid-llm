package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestProcessAuthorizationUsesRunnerApprovalNotModelArguments(t *testing.T) {
	for _, tc := range []struct {
		args              string
		approved, allowed bool
	}{
		{`{"program":"go","arguments":["test","./..."]}`, false, true},
		{`{"program":"sh","arguments":["-c","echo hi"]}`, false, false},
		{`{"program":"sh","arguments":["-c","echo hi"]}`, true, true},
	} {
		err := authorizeExecutionInvocation("workspace_process_start", json.RawMessage(tc.args), agents.ToolExecution{Approved: tc.approved})
		if (err == nil) != tc.allowed {
			t.Fatal(tc, err)
		}
		if !tc.allowed && !errors.Is(err, capabilities.ErrApprovalRequired) {
			t.Fatal(err)
		}
	}
	for _, args := range []string{`{"program":"go","arguments":[],"approved":true}`, `{"program":"go","program":"sh"}`} {
		if authorizeExecutionInvocation("workspace_process_start", []byte(args), agents.ToolExecution{Approved: true}) == nil {
			t.Fatal("invalid invocation approved")
		}
	}
	if execution.Permitted("workspace_process_start", execution.Permissions{Read: true, Write: true}) {
		t.Fatal("file consent granted commands")
	}
}

func TestProcessCompletionRequiresTerminalHostEvidence(t *testing.T) {
	tools := &browserRunTools{}
	step := func(name, state string, exit *int) agents.Step {
		process, _ := json.Marshal(execution.ProcessResult{Handle: "owned", State: state, ExitCode: exit})
		result, _ := json.Marshal(map[string]any{"receipt": execution.Receipt{State: "complete", Result: process}})
		return agents.Step{Type: "action", ToolName: name, ToolResult: string(result)}
	}
	task := &agents.Task{Steps: []agents.Step{step("workspace_process_start", "running", nil)}}
	if tools.ValidateExecutionCompletion(task) == nil {
		t.Fatal("start counted as finished")
	}
	exit := 7
	task.Steps = append(task.Steps, step("workspace_process_wait", "failed", &exit))
	if err := tools.ValidateExecutionCompletion(task); err != nil {
		t.Fatal("honest failed exit could not be reported", err)
	}
	if err := tools.StopOwnedExecution(context.Background(), &agents.Task{}); err != nil {
		t.Fatal(err)
	}
}
