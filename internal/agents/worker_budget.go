package agents

import "github.com/takuphilchan/offgrid-llm/internal/execution"

// ChildBudgetSettlement records the irreversible release of a finished run's
// unused allocation. Retained slots cover calls and conservative token/time
// charges, including generations whose outcome was never measured.
type ChildBudgetSettlement struct {
	Allocated int `json:"allocated"`
	Retained  int `json:"retained"`
}

func retainedChildBudget(parent, child *Task) (int, bool) {
	if child == nil || child.Checkpoint == nil || !terminalControlTask(child) || child.ProcessStopPending || child.ControlPending || child.Checkpoint.ExecutingCall != "" {
		return 0, false
	}
	cp := child.Checkpoint
	allocated := child.Config.MaxIterations
	if allocated < 1 || cp.Iteration < 0 || cp.BudgetedModelCalls < 0 || cp.ReservedChildIterations != 0 || cp.ChargedCompletionTokens < 0 || cp.ChargedInferenceNS < 0 || child.Config.MaxTokens != parent.Config.MaxTokens || child.Config.TimeoutPerStep != parent.Config.TimeoutPerStep {
		return 0, false
	}
	retained := max(cp.Iteration, cp.BudgetedModelCalls)
	if process, valid := processBudgetDecisions(cp.ProcessBudget); valid {
		if cp.ProcessBudget != nil {
			unit, ok := toolBudgetUnit(max(int64(parent.Config.TimeoutPerStep), int64(execution.DefaultProcessTimeout)), maxToolsPerDecision)
			starts := maxToolsPerDecision
			if pinned := parent.Checkpoint.ProcessBudget; pinned != nil {
				unit, starts = pinned.PerDecisionNS, pinned.StartsPerDecision
			}
			if !ok || cp.ProcessBudget.PerDecisionNS != unit || cp.ProcessBudget.StartsPerDecision != starts {
				return 0, false
			}
		}
		retained = max(retained, process)
	} else {
		return 0, false
	}
	if tools := cp.ToolBudget; tools != nil {
		perCall := int64(parent.Config.TimeoutPerStep)
		perDecision := maxToolsPerDecision
		if parent.Checkpoint.ToolBudget != nil {
			perCall = parent.Checkpoint.ToolBudget.PerCallNS
			perDecision = parent.Checkpoint.ToolBudget.CallsPerDecision
		}
		unit, ok := toolBudgetUnit(perCall, perDecision)
		if !ok || tools.Version != 1 || tools.CallsPerDecision != perDecision || tools.PerCallNS != perCall || tools.Attempts < 0 || tools.ChargedNS < 0 {
			return 0, false
		}
		used := tools.ChargedNS/unit + int64(boolInt(tools.ChargedNS%unit != 0))
		if used > int64(allocated) {
			return 0, false
		}
		retained = max(retained, int(used), tools.Attempts/perDecision+boolInt(tools.Attempts%perDecision != 0))
	}
	if input := cp.InputBudget; input != nil {
		window := parent.Config.ContextWindow
		if parent.Checkpoint.InputBudget != nil {
			window = parent.Checkpoint.InputBudget.Window
		}
		if window < 1 || input.Window < 1 || input.Window > window || input.ChargedTokens < 0 {
			return 0, false
		}
		used := input.ChargedTokens/int64(window) + int64(boolInt(input.ChargedTokens%int64(window) != 0))
		if used > int64(allocated) {
			return 0, false
		}
		retained = max(retained, int(used))
	}
	// A completed run necessarily generated an answer. Missing attempt evidence
	// in a started legacy/corrupt checkpoint cannot be treated as zero usage.
	if retained == 0 && (child.Status == TaskCompleted || child.StartedAt != nil) {
		return 0, false
	}
	for _, cost := range [][2]int64{{cp.ChargedCompletionTokens, int64(parent.Config.MaxTokens)}, {cp.ChargedInferenceNS, int64(parent.Config.TimeoutPerStep)}} {
		if cost[1] <= 0 {
			return 0, false
		}
		slots := cost[0]/cost[1] + int64(boolInt(cost[0]%cost[1] != 0))
		if slots > int64(allocated) {
			return 0, false
		}
		retained = max(retained, int(slots))
	}
	return retained, retained <= allocated
}

// Caller holds runner/manager locks and owns a detached parent snapshot. The
// receipt and reduced counter must be saved together, before allocating again.
// Legacy graphs keep their admitted accounting contract.
func (r *Runner) settleChildBudgetsLocked(parent *Task) bool {
	if parent.WorkID == "" || parent.Checkpoint == nil {
		return false
	}
	changed := false
	cp := parent.Checkpoint
	for _, link := range parent.ChildHistory {
		if _, settled := cp.ChildBudgetSettlements[link.ID]; settled || r.active[link.ID] != nil {
			continue
		}
		child := r.manager.tasks[link.ID]
		if child == nil || child.ParentID != parent.ID || child.Actor != parent.Actor || child.Worker == nil || child.Worker.WorkID != parent.WorkID {
			continue
		}
		retained, ok := retainedChildBudget(parent, child)
		if !ok || cp.ReservedChildIterations < child.Config.MaxIterations {
			continue
		}
		if cp.ChildBudgetSettlements == nil {
			cp.ChildBudgetSettlements = map[string]ChildBudgetSettlement{}
		}
		cp.ChildBudgetSettlements[child.ID] = ChildBudgetSettlement{Allocated: child.Config.MaxIterations, Retained: retained}
		cp.ReservedChildIterations -= child.Config.MaxIterations - retained
		changed = true
	}
	return changed
}
