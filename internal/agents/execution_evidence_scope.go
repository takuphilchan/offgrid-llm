package agents

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Walk only service-persisted attachment transitions backwards. Historical
// references validate receipts; they never authorize a new host operation.
func executionReferenceAtStep(task *Task, index int) (execution.Reference, bool) {
	if task.Config.Execution == nil {
		return execution.Reference{}, false
	}
	ref := *task.Config.Execution
	if task.Config.ExecutionRequestVersion != execution.AccessRequestVersion {
		return ref, true
	}
	for i := len(task.Steps) - 1; i > index; i-- {
		step := task.Steps[i]
		if step.Type != "input_resolved" || step.ToolName != execution.AccessRequestTool {
			continue
		}
		var transition struct {
			Status   string               `json:"status"`
			Current  execution.Reference  `json:"execution"`
			Previous *execution.Reference `json:"previous_execution"`
		}
		if json.Unmarshal([]byte(step.ToolResult), &transition) != nil || transition.Status != "workspace_attached" || transition.Current != ref || transition.Previous == nil || transition.Previous.Validate() != nil || transition.Previous.Host != ref.Host {
			return execution.Reference{}, false
		}
		ref = *transition.Previous
	}
	return ref, true
}
