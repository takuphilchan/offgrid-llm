package agents

import (
	"context"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func validatePartialSynthesis(task *Task) error {
	if task.Checkpoint != nil && task.Checkpoint.PartialSynthesis && (task.WorkID == "" || task.ParentID != "" || task.Origin == nil || task.Delegation != nil || task.Result == "" || task.Config.ToolAdapter != nil || task.Status == TaskCompleted) {
		return ErrRunStorage
	}
	return nil
}

func (r *Runner) partialSynthesisFailure(ctx context.Context, task *Task, err error) (*Task, error) {
	if !task.Checkpoint.PartialSynthesis || ctx.Err() != nil {
		return nil, err
	}
	return r.fail(task, "Partial-result synthesis was unavailable. The recorded branch outcomes remain available: "+err.Error())
}

// Save a deterministic outcome before admitting optional synthesis. A failed
// model/context reservation, interrupted generation or restart cannot erase it.
// The caller holds both locks and has already settled every child operation.
func preparePartialSynthesis(parent *Task, children map[string]*Task) bool {
	if parent.WorkID == "" || parent.Config.ToolAdapter != nil || parent.Checkpoint == nil || parent.Delegation == nil {
		return false
	}
	useful := false
	for _, outcome := range childOutcomes(parent, children) {
		// Superseded dependencies require a new instruction decision. They must
		// not be summarized as a current result by this automatic stage.
		if outcome.Stale {
			return false
		}
		useful = useful || outcome.Status == TaskCompleted && outcome.Output != ""
	}
	if !useful {
		return false
	}
	probe, err := copyTask(parent)
	if err != nil || reserveModelBudget(probe) != nil {
		return false
	}
	parent.Checkpoint.PartialSynthesis = true
	parent.Checkpoint.Messages = append(parent.Checkpoint.Messages, api.ChatMessage{Role: "system", Content: "Some worker branches failed. Summarize the useful independent results for the user and state the missing outcomes and limitations. Use task_history to retrieve full saved results if needed. Do not claim overall completion, verification or integration. This stage cannot run tools other than saved evidence retrieval, create workers or retry failed work. It consumes the current work's remaining allowance. The recorded branch status remains partial regardless of your wording."})
	parent.Delegation = nil
	parent.Status, parent.SchedulerReady, parent.CompletedAt = TaskPending, true, nil
	return true
}

func (r *Runner) finishPartialSynthesis(task *Task, answer string) (*Task, error) {
	settled, err := r.stopOwnedExecution(task)
	if err != nil || settled.Status == TaskUncertain {
		return settled, err
	}
	// Keep the recorded outcome and exact retrieval references visible even if
	// model prose incorrectly describes a failed branch as completed.
	task.Result = answer + "\n\nRecorded branch outcomes:\n" + task.Result
	endPartialSynthesis(task)
	finishTask(task, TaskFailed, "Some delegated work could not complete. Useful independent results are retained below.")
	task.ErrorCode = "delegation_partial"
	if err := r.persist(task); err != nil {
		return nil, err
	}
	return task, nil
}

func endPartialSynthesis(task *Task) {
	if task.Checkpoint == nil || !task.Checkpoint.PartialSynthesis {
		return
	}
	task.Checkpoint.PartialSynthesis = false
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "system", Content: "The prior partial-result synthesis stage has ended. Its recorded failures and findings remain evidence. Any explicit continuation or correction follows the current work instructions within the remaining allowance and live authority."})
}
