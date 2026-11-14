package agents

import "fmt"

// restoreChildReservations repairs legacy or incomplete parent counters from
// the pinned child jobs and committed settlement receipts. Recovery never
// creates refunds; absent or inconsistent evidence retains the allocation.
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
		allocated := parent.Checkpoint.InheritedChildIterations
		if allocated < 0 || allocated > parent.Config.MaxIterations {
			allocated = parent.Config.MaxIterations
		}
		for _, link := range parent.ChildHistory {
			child := byID[link.ID]
			if child == nil || child.Config.MaxIterations < 1 {
				// An invalid or oversized saved child budget cannot grant the
				// parent extra decisions. Exhaust the root budget instead.
				allocated = parent.Config.MaxIterations
				break
			}
			charge := child.Config.MaxIterations
			if receipt, ok := parent.Checkpoint.ChildBudgetSettlements[child.ID]; ok && parent.WorkID != "" {
				retained, valid := retainedChildBudget(parent, child)
				if valid && child.ParentID == parent.ID && child.Actor == parent.Actor && child.Worker != nil && child.Worker.WorkID == parent.WorkID && receipt.Allocated == child.Config.MaxIterations && receipt.Retained >= retained && receipt.Retained <= receipt.Allocated {
					charge = receipt.Retained
				}
			}
			if charge > parent.Config.MaxIterations-allocated {
				allocated = parent.Config.MaxIterations
				break
			}
			allocated += charge
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
	parentVerificationAndAnswer := 2
	if task.WorkID != "" {
		parentVerificationAndAnswer = max(2, (task.Config.MaxIterations+9)/10)
	}
	used := max(task.Checkpoint.Iteration, task.Checkpoint.BudgetedModelCalls)
	if process, valid := processBudgetDecisions(task.Checkpoint.ProcessBudget); valid {
		used = max(used, process)
	} else {
		return 0, fmt.Errorf("Process budget is inconsistent")
	}
	if tools := task.Checkpoint.ToolBudget; tools != nil {
		unit, ok := toolBudgetUnit(tools.PerCallNS, tools.CallsPerDecision)
		if !ok || tools.Version != 1 || tools.Attempts < 0 || tools.ChargedNS < 0 {
			return 0, fmt.Errorf("Tool runtime budget is inconsistent")
		}
		used = max(used, tools.Attempts/tools.CallsPerDecision+boolInt(tools.Attempts%tools.CallsPerDecision != 0))
		used = max(used, int(tools.ChargedNS/unit)+boolInt(tools.ChargedNS%unit != 0))
	}
	if input := task.Checkpoint.InputBudget; input != nil {
		if input.Window <= 0 || input.ChargedTokens < 0 {
			return 0, fmt.Errorf("Input-token budget is inconsistent")
		}
		used = max(used, int(input.ChargedTokens/int64(input.Window))+boolInt(input.ChargedTokens%int64(input.Window) != 0))
	}
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
	// Canonical workers draw a bounded initial slice, leaving a shared remainder
	// for follow-ups. Legacy admitted graphs retain their pinned allocation rules.
	if task.WorkID != "" {
		perChild = min(perChild, resolvedWorkerPolicy(task).InitialModelCalls)
	}
	return perChild, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
