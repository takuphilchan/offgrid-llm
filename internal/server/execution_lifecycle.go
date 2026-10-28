package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func (b *browserRunTools) StopOwnedExecution(ctx context.Context, task *agents.Task) error {
	if !agents.HasOwnedExecutionProcesses(task) {
		return nil
	}
	if b.server.executionHosts == nil {
		return execution.ErrHostUnavailable
	}
	return b.server.executionHosts.CancelIfAdmitted(ctx, task.Actor, b.server.workspaceID, task.ID, *task.Config.Execution)
}

// ValidateExecutionCompletion checks recorded host results, not the model's
// assertion. A start receipt proves dispatch only; an observed terminal result
// is required before this root job can finish. Failed exit codes remain visible
// evidence and may be honestly reported, never rewritten into successful tests.
func (b *browserRunTools) ValidateExecutionCompletion(task *agents.Task) error {
	if err := agents.ValidateInputWrites(task); err != nil {
		return err
	}
	states := map[string]string{}
	references := map[string]execution.Reference{}
	var order []string
	for _, step := range task.Steps {
		if step.Type != "action" || !strings.HasPrefix(step.ToolName, "workspace_process_") {
			continue
		}
		var result struct {
			Receipt execution.Receipt `json:"receipt"`
			Host    string            `json:"host"`
			Session string            `json:"session"`
		}
		if json.Unmarshal([]byte(step.ToolResult), &result) != nil {
			return fmt.Errorf("Process completion is unverified: inspect the owned process before finishing")
		}
		var process execution.ProcessResult
		if json.Unmarshal(result.Receipt.Result, &process) != nil || process.Handle == "" {
			return fmt.Errorf("Process completion is unverified: inspect the owned process before finishing")
		}
		if step.ToolName != "workspace_process_start" {
			if _, known := states[process.Handle]; !known {
				return fmt.Errorf("Process result does not match a recorded start")
			}
		} else {
			if _, known := states[process.Handle]; !known {
				order = append(order, process.Handle)
			}
			if (result.Receipt.State == "dispatched" || result.Receipt.State == "complete") && task.Config.Execution != nil && result.Host == task.Config.Execution.Host && result.Session == task.Config.Execution.Session {
				references[process.Handle] = *task.Config.Execution
			}
		}
		if process.State != "running" && process.ExitCode == nil {
			return fmt.Errorf("Process exit evidence is incomplete")
		}
		states[process.Handle] = process.State
	}
	for _, handle := range order {
		state := states[handle]
		if state == "running" {
			return &agents.ExecutionPending{Handle: handle, Reference: references[handle]}
		}
		if state != "exited" && state != "failed" && state != "cancelled" && state != "timed_out" {
			return fmt.Errorf("A workspace command is still running or unverified. Inspect/wait for its actual exit before finishing; do not restart it")
		}
	}
	if grantStep := pendingEditAccess(task); grantStep >= 0 && !verifiedWorkspaceWrite(task, grantStep) {
		return &agents.ExecutionPending{Kind: "workspace_edit", Reference: *task.Config.Execution}
	}
	return nil
}

// An explicit edit-access request is an execution intent. The model may still
// decline or fail, but it cannot report completion after receiving that grant
// and doing only reads. This checks typed service steps, not model prose.
func pendingEditAccess(task *agents.Task) int {
	if task == nil || !task.Config.TaskFirst || task.Config.ExecutionRequestVersion != execution.AccessRequestVersion || task.Config.Execution == nil {
		return -1
	}
	for i := len(task.Steps) - 1; i >= 0; i-- {
		step := task.Steps[i]
		if step.Type != "input_resolved" || step.ToolName != "request_workspace_access" {
			continue
		}
		var resolved struct {
			Execution       execution.Reference   `json:"execution"`
			Permissions     execution.Permissions `json:"permissions"`
			Status          string                `json:"status"`
			RequestedAccess string                `json:"requested_access"`
		}
		if json.Unmarshal([]byte(step.ToolResult), &resolved) == nil && resolved.Status == "workspace_attached" && resolved.Execution == *task.Config.Execution && resolved.RequestedAccess == "edit" && resolved.Permissions.Write {
			return i
		}
	}
	return -1
}

func verifiedWorkspaceWrite(task *agents.Task, after int) bool {
	for _, step := range task.Steps[after+1:] {
		if step.Type != "action" || (step.ToolName != "workspace_write" && step.ToolName != execution.InputWriteTool) {
			continue
		}
		var result struct {
			Host    string            `json:"host"`
			Session string            `json:"session"`
			Receipt execution.Receipt `json:"receipt"`
		}
		var written execution.WriteResult
		if json.Unmarshal([]byte(step.ToolResult), &result) == nil && result.Host == task.Config.Execution.Host && result.Session == task.Config.Execution.Session && result.Receipt.State == "complete" && json.Unmarshal(result.Receipt.Result, &written) == nil && written.Host == result.Host && written.Verified {
			return true
		}
	}
	return false
}
