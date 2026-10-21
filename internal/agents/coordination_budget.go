package agents

import "fmt"

// restoreChildReservations repairs legacy or incomplete parent counters from
// the pinned child jobs. A restored parent must never regain decisions already
// allocated to children, even when its checkpoint predates this counter.
func restoreChildReservations(tasks []*Task) map[string]bool {
	byID := make(map[string]*Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	changed := make(map[string]bool)
	for _, parent := range tasks {
		if parent.Checkpoint == nil || len(parent.ChildHistory) == 0 || parent.Config.MaxIterations < 1 {
			continue
		}
		allocated := 0
		for _, link := range parent.ChildHistory {
			child := byID[link.ID]
			if child == nil || child.Config.MaxIterations < 1 ||
				child.Config.MaxIterations > parent.Config.MaxIterations-allocated {
				// An invalid or oversized saved child budget cannot grant the
				// parent extra decisions. Exhaust the root budget instead.
				allocated = parent.Config.MaxIterations
				break
			}
			allocated += child.Config.MaxIterations
		}
		if parent.Checkpoint.ReservedChildIterations < allocated {
			parent.Checkpoint.ReservedChildIterations = allocated
			changed[parent.ID] = true
		}
	}
	return changed
}

// childDecisionBudget allocates from the one durable root budget, regardless
// of whether the user selected reusable profiles. Keep two parent decisions
// after delegation: one for checking child evidence or performing a required
// parent-owned action, and one for the final synthesis. A child never consumes
// the parent's last chance to verify before answering.
func childDecisionBudget(task *Task, children int) (int, error) {
	if task == nil || task.Checkpoint == nil || children < 1 {
		return 0, fmt.Errorf("Delegation requires a root checkpoint and children")
	}
	const parentVerificationAndAnswer = 2
	used := max(task.Checkpoint.Iteration, task.Checkpoint.BudgetedModelCalls)
	if task.Config.MaxTokens > 0 {
		perCall := int64(task.Config.MaxTokens)
		used = max(used, int(task.Checkpoint.ChargedCompletionTokens/perCall)+boolInt(task.Checkpoint.ChargedCompletionTokens%perCall != 0))
	}
	if task.Config.TimeoutPerStep > 0 {
		perStep := int64(task.Config.TimeoutPerStep)
		used = max(used, int(task.Checkpoint.ChargedInferenceNS/perStep)+boolInt(task.Checkpoint.ChargedInferenceNS%perStep != 0))
	}
	perChild := (task.Config.MaxIterations - used - task.Checkpoint.ReservedChildIterations - parentVerificationAndAnswer) / children
	if perChild < 1 {
		return 0, fmt.Errorf("Insufficient aggregate iteration budget for delegation and the parent answer")
	}
	return perChild, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
