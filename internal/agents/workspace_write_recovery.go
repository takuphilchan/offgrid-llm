package agents

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// A model/tool error string cannot settle a mutation. Only the current call's
// authenticated, persisted rejected host receipt permits a fresh proposal.
func (r *Runner) repairWorkspaceWritePrecondition(task *Task, descriptor capabilities.Descriptor, failure *ToolFailure) (bool, error) {
	expected, ok := execution.Descriptor(descriptor.Name)
	if !task.Config.TaskFirst || task.Config.ExecutionRequestVersion != execution.AccessRequestVersion || !ok || descriptor != expected || execution.Operation(descriptor.Name) != "file_write" || failure.Code != "execution_write_precondition" || failure.Recovery == "" {
		return false, nil
	}
	saved, found := r.manager.GetTask(task.ID)
	if !found || saved.Actor != task.Actor || saved.Status != TaskRunning || saved.Checkpoint == nil {
		return false, nil
	}
	cp, proof := task.Checkpoint, saved.Checkpoint
	if cp == nil || cp.ExecutingCall == "" || cp.ExecutingCall != proof.ExecutingCall || !validExecutionIntent(saved, proof.ExecutionIntent) || proof.ExecutionIntent.Tool != descriptor.Name || proof.ExecutionReceipt == nil || proof.ExecutionReceipt.Receipt.State != "rejected" {
		return false, nil
	}
	var reason struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(proof.ExecutionReceipt.Receipt.Result, &reason) != nil || (reason.Error != "execution_target_exists" && reason.Error != "execution_revision_changed") {
		return false, nil
	}
	cp.ExecutingCall, cp.ReadOnlyCall, cp.ReadOnlyCapability = "", "", nil
	cp.ExecutionIntent, cp.ExecutionReceipt = nil, nil
	cp.PreEffectRepair, cp.PreEffectRepairTool = true, descriptor.Name
	recovery := failure.Recovery
	if descriptor.Name == execution.InputWriteTool {
		recovery += " Repair workspace_write_from_input itself: after reading the target, keep the same input, after, before and path selectors and supply the observed revision. Do not substitute workspace_write or retype the selected text: that can lose Unicode or trailing newlines and does not resolve this exact-input operation."
	}
	return true, r.rejectProposal(task, &ToolAuthorizationError{Code: failure.Code, Message: failure.Message}, recovery)
}
