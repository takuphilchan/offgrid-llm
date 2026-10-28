package agents

import (
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// A trusted rejected read is evidence for another model decision, not a retry
// command. It cannot settle an outstanding mutation or create new authority.
func (r *Runner) repairWorkspaceReadFailure(task *Task, descriptor capabilities.Descriptor, failure *ToolFailure) (bool, error) {
	expected, ok := execution.Descriptor(descriptor.Name)
	if !ok || descriptor != expected || !execution.FileReadTool(descriptor.Name) || failure.Code != "execution_rejected" || failure.Recovery == "" || task.Checkpoint.ExecutionIntent != nil {
		return false, nil
	}
	cp := task.Checkpoint
	cp.WorkspaceReadFailures++ // Persisted root allowance; successes do not refund it.
	for _, pending := range cp.Calls {
		result := `{"error":"tool_proposal_abandoned","effects":"none","verified":false,"message":"Not executed because an earlier read failed. Decide again from the actual request and recorded evidence."}`
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: pending.ID, Name: pending.Function.Name, Content: result})
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "rejected", ToolName: pending.Function.Name, ToolArgs: pending.Function.Arguments, ToolResult: result, Timestamp: time.Now().UTC()})
	}
	cp.Calls = nil
	setRunPhase(task, "repairing_tool", descriptor.Name)
	if cp.WorkspaceReadFailures > maxProposalRepairs {
		task.Status, task.ErrorCode = TaskInterrupted, "workspace_read_repair_limit"
		task.Error = "Paused after repeated rejected workspace reads. No file changes were made by those reads; earlier actions remain recorded. Review the requested path and selected folder before continuing."
		setRunPhase(task, "interrupted", "")
	}
	return true, r.persist(task)
}
