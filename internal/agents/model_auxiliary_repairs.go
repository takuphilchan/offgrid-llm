package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Historical repair counters also enforce local lifetime limits and answer-only
// dispatch. They cannot describe unresolved debt after a verified correction.
type AuxiliaryModelRepairs struct {
	Version    int            `json:"version"`
	Rejections int            `json:"rejections,omitempty"`
	Active     map[string]int `json:"active,omitempty"`
	Resolved   map[string]int `json:"resolved,omitempty"`
}

func newAuxiliaryModelRepairs() *AuxiliaryModelRepairs {
	return &AuxiliaryModelRepairs{Version: 1, Active: map[string]int{}}
}

func validateStoredModelCorrections(task *Task) error {
	if task.Checkpoint == nil {
		return nil
	}
	if _, err := auxiliaryModelCorrections(task); err != nil {
		return err
	}
	if b := task.Checkpoint.ModelBudget; b != nil {
		if task.ParentID != "" || b.Switches < len(task.Checkpoint.ModelSwitches) {
			return fmt.Errorf("invalid model budget owner or switch count")
		}
		if _, err := b.corrections(); err != nil {
			return err
		}
	}
	return nil
}

func auxiliaryRepairCategory(category string) bool {
	switch category {
	case "skill_answer", "delegation_answer", "workflow_completion", "workspace_completion", "delegation_admission", "legacy_ambiguous":
		return true
	}
	if strings.HasPrefix(category, "metadata:") {
		return isRuntimeTool(strings.TrimPrefix(category, "metadata:"))
	}
	return strings.HasPrefix(category, "workspace_read:") && execution.FileReadTool(strings.TrimPrefix(category, "workspace_read:"))
}

func auxiliaryModelCorrections(task *Task) (int, error) {
	a := task.Checkpoint.AuxiliaryModelRepairs
	if a == nil {
		if legacyAuxiliaryAmbiguous(task) {
			return maxProposalRepairs, nil
		}
		return 0, nil
	}
	if a.Version != 1 || a.Rejections < 0 || a.Rejections > 1000000 || len(a.Active) > 40 || len(a.Resolved) > 40 {
		return 0, fmt.Errorf("invalid auxiliary model repair state")
	}
	total := 0
	for category, count := range a.Active {
		if !auxiliaryRepairCategory(category) || count < 1 || count > 100 {
			return 0, fmt.Errorf("invalid auxiliary model repair category")
		}
		total += count
	}
	for category, count := range a.Resolved {
		if !auxiliaryRepairCategory(category) || count < 1 || count > 1000000 {
			return 0, fmt.Errorf("invalid auxiliary model resolution witness")
		}
	}
	return total, nil
}

func legacyAuxiliaryAmbiguous(task *Task) bool {
	if task.Status == TaskCompleted {
		return false
	}
	cp := task.Checkpoint
	if cp.SkillAnswerRepairs > 0 || cp.DelegationAnswerRepairs > 0 || cp.WorkspaceCompletionRepairs > 0 || cp.CompletionRepairs > 0 || cp.WorkspaceReadFailures > 0 {
		return true
	}
	for _, step := range task.Steps {
		if step.Type == "completion_rejected" || step.Type == "response_contract_rejected" || step.Type == "delegation_answer_rejected" || step.Type == "rejected" && step.ToolName == "delegate_tasks" {
			return true
		}
		if step.Type == "task_metadata" {
			var result struct {
				Error string `json:"error"`
			}
			if json.Unmarshal([]byte(step.ToolResult), &result) == nil && result.Error == "invalid_task_operation" {
				return true
			}
		}
	}
	return false
}

func ensureAuxiliaryModelRepairs(task *Task) {
	if task.Checkpoint.AuxiliaryModelRepairs != nil {
		return
	}
	a := newAuxiliaryModelRepairs()
	if legacyAuxiliaryAmbiguous(task) {
		a.Active["legacy_ambiguous"] = maxProposalRepairs
	}
	task.Checkpoint.AuxiliaryModelRepairs = a
}

// Called only at a trusted rejection, before incrementing historical counters.
// No switch is requested here: adapter switching has a separate typed gate.
func chargeAuxiliaryModelRepair(task *Task, category string) {
	ensureAuxiliaryModelRepairs(task)
	a := task.Checkpoint.AuxiliaryModelRepairs
	if a.Active == nil {
		a.Active = map[string]int{}
	}
	a.Active[category]++
	a.Rejections++
}

func resolveAuxiliaryModelRepair(task *Task, category string) {
	ensureAuxiliaryModelRepairs(task)
	a := task.Checkpoint.AuxiliaryModelRepairs
	if count := a.Active[category]; count > 0 {
		if a.Resolved == nil {
			a.Resolved = map[string]int{}
		}
		a.Resolved[category] += count
		delete(a.Active, category)
	}
}

func resolveFinalAuxiliaryModelRepairs(task *Task) {
	ensureAuxiliaryModelRepairs(task)
	for category := range task.Checkpoint.AuxiliaryModelRepairs.Active {
		resolveAuxiliaryModelRepair(task, category)
	}
}

// Check against canonical state inside the task transaction. Generic accepted
// progress is not permission to erase auxiliary debt from an older snapshot.
// Only the matching hooks above advance resolution witnesses.
func validateAuxiliaryModelTransition(current, next *Task) error {
	prior, after := current.Checkpoint.AuxiliaryModelRepairs, next.Checkpoint.AuxiliaryModelRepairs
	if prior == nil {
		prior = newAuxiliaryModelRepairs()
		if legacyAuxiliaryAmbiguous(current) {
			prior.Active["legacy_ambiguous"] = maxProposalRepairs
		}
	} else if after == nil {
		return ErrRunConflict
	}
	if after == nil {
		after = newAuxiliaryModelRepairs()
	}
	if after.Rejections < prior.Rejections {
		return ErrRunConflict
	}
	for category, count := range prior.Resolved {
		if after.Resolved[category] < count {
			return ErrRunConflict
		}
	}
	for category, count := range prior.Active {
		if decrease := count - after.Active[category]; decrease > 0 && after.Resolved[category]-prior.Resolved[category] < decrease {
			return ErrRunConflict
		}
	}
	return nil
}

func auxiliaryModelRejection(current, next *Task) bool {
	a := next.Checkpoint.AuxiliaryModelRepairs
	if a == nil {
		return false
	}
	prior := current.Checkpoint.AuxiliaryModelRepairs
	if prior == nil && a.Rejections > 0 || prior != nil && a.Rejections > prior.Rejections {
		return true
	}
	for category, count := range a.Active {
		old := 0
		if prior != nil {
			old = prior.Active[category]
		} else if category == "legacy_ambiguous" && legacyAuxiliaryAmbiguous(current) {
			old = maxProposalRepairs
		}
		if count > old {
			return true
		}
	}
	return false
}

// Only trusted execution identities and the service's receipt validator can
// settle completion debt. A successful wait may still report a running process.
func (r *Runner) resolveAuxiliaryExecutionProgress(task *Task, descriptor capabilities.Descriptor) {
	expected, known := execution.Descriptor(descriptor.Name)
	if !known || expected != descriptor {
		return
	}
	if execution.FileReadTool(descriptor.Name) {
		resolveAuxiliaryModelRepair(task, "workspace_read:"+descriptor.Name)
	}
	a := task.Checkpoint.AuxiliaryModelRepairs
	if a == nil || a.Active["workspace_completion"] == 0 {
		return
	}
	switch descriptor.Name {
	case "workspace_write", "workspace_write_from_input", "workspace_process_wait", "workspace_process_output", "workspace_process_stop":
		if validator, ok := r.tools.(interface{ ValidateExecutionCompletion(*Task) error }); ok {
			_ = validateAuxiliaryExecutionCompletion(task, validator)
		}
	}
}

func validateAuxiliaryExecutionCompletion(task *Task, validator interface{ ValidateExecutionCompletion(*Task) error }) error {
	if err := validator.ValidateExecutionCompletion(task); err != nil {
		return err
	}
	resolveAuxiliaryModelRepair(task, "workspace_completion")
	return nil
}

type runtimeArgumentError struct{ cause error }

func (e *runtimeArgumentError) Error() string { return e.cause.Error() }
func (e *runtimeArgumentError) Unwrap() error { return e.cause }
