package agents

import (
	"bytes"
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// A typed copy may repair an ordinary write only when it performed that same
// path/content operation, with authenticated persisted read-back. This does not
// allow ordinary model-generated content to replace a pending typed copy.
func resolveInputWriteRepair(task, saved *Task, descriptor capabilities.Descriptor) {
	if task == nil || saved == nil || task.Checkpoint == nil || saved.Checkpoint == nil || !task.Config.TaskFirst || task.Config.ExecutionRequestVersion != execution.AccessRequestVersion {
		return
	}
	cp, proof := task.Checkpoint, saved.Checkpoint
	expected, _ := execution.Descriptor(execution.InputWriteTool)
	if !cp.PreEffectRepair || cp.PreEffectRepairTool != "workspace_write" || descriptor != expected || task.ID != saved.ID || task.Actor != saved.Actor || cp.ExecutingCall == "" || cp.ExecutingCall != proof.ExecutingCall || !validExecutionIntent(saved, proof.ExecutionIntent) || proof.ExecutionIntent.Tool != execution.InputWriteTool || proof.ExecutionReceipt == nil {
		return
	}
	intent := proof.ExecutionIntent.Request
	if !execution.WriteReceiptMatches(intent, proof.ExecutionReceipt.Receipt) || task.Config.Execution == nil || saved.Config.Execution == nil || *task.Config.Execution != *saved.Config.Execution {
		return
	}
	var written execution.FileWritePayload
	if json.Unmarshal(intent.Arguments, &written) != nil {
		return
	}
	for i := len(task.Steps) - 1; i >= 0; i-- {
		step := task.Steps[i]
		// A directory/attachment transition cannot silently reinterpret the
		// original relative destination, even on the same host.
		if step.ToolName == "workspace_change_directory" || step.Type == "input_resolved" {
			return
		}
		if step.Type != "rejected" || step.ToolName != "workspace_write" {
			continue
		}
		var rejection struct {
			Code    string `json:"error"`
			Effects string `json:"effects"`
		}
		var original struct {
			Path    string  `json:"path"`
			Content *string `json:"content"`
		}
		if json.Unmarshal([]byte(step.ToolResult), &rejection) != nil || rejection.Code != "execution_write_precondition" || rejection.Effects != "none" || json.Unmarshal([]byte(step.ToolArgs), &original) != nil || original.Content == nil || original.Path != written.Path || !bytes.Equal([]byte(*original.Content), written.Data) {
			return
		}
		originalDescriptor, _ := execution.Descriptor("workspace_write")
		resolvePreEffectRepair(cp, "workspace_write", originalDescriptor)
		return
	}
}
