package agents

import "context"

func (r *Runner) persist(task *Task) error {
	return r.persistModelDecision(task, false)
}

func (r *Runner) persistModelDecision(task *Task, rejected bool) error {
	return r.persistModelState(task, rejected, false)
}

// Only a successfully accepted result resolves a rejected model decision.
// Queuing a well-formed envelope or clearing execution state is not progress.
func (r *Runner) persistModelProgress(task *Task) error {
	return r.persistModelState(task, false, true)
}

func (r *Runner) persistModelState(task *Task, rejected, accepted bool) error {
	ensureAuxiliaryModelRepairs(task)
	snapshot, err := copyTask(task)
	if err != nil {
		return err
	}
	steering, inbox := false, false
	updated, err := r.manager.updateModelGraph(task.ID, rejected, func(current *Task) (bool, error) {
		if current.Status != TaskRunning {
			return false, context.Canceled
		}
		_, err := auxiliaryModelCorrections(snapshot)
		if err != nil {
			return false, err
		}
		_, err = auxiliaryModelCorrections(current)
		if err != nil {
			return false, err
		}
		if hasConversationHold(current) && current.Checkpoint.ExecutingCall == "" {
			retainHeldModelSettlement(current.Checkpoint, snapshot.Checkpoint)
			parkConversationTask(current)
			inbox = true
			return false, nil
		}
		// A correction admitted before a new dispatch/final answer wins that
		// boundary. Do not erase it with a worker's older snapshot. In-flight
		// results are still recorded first; they must never be replayed.
		if len(current.PendingInstructions) > 0 && current.Checkpoint.ExecutingCall == "" && (auxiliaryModelRejection(current, snapshot) || snapshot.Status != TaskRunning || snapshot.Checkpoint.ExecutingCall != "" || len(snapshot.Checkpoint.Calls) > 0) {
			// Reject the superseded proposal, not the inference cost already
			// incurred. Persist the charge in the same transaction as the inbox
			// check so restart/steering cannot recover a spent parent budget.
			retainHeldModelSettlement(current.Checkpoint, snapshot.Checkpoint)
			steering = true
			return false, nil
		}
		snapshot.PendingInstructions = append([]Instruction(nil), current.PendingInstructions...)
		snapshot.WorkerMessageRequests = current.WorkerMessageRequests
		snapshot.ConversationHold = current.ConversationHold
		snapshot.ControlEpoch = max(snapshot.ControlEpoch, current.ControlEpoch)
		// Host adapters own process charges independently of a model snapshot.
		if current.Checkpoint != nil && current.Checkpoint.ProcessBudget != nil && snapshot.Checkpoint != nil {
			snapshot.Checkpoint.ProcessBudget = current.Checkpoint.ProcessBudget
		}
		if current.Checkpoint != nil && current.Checkpoint.StorageBudget != nil && snapshot.Checkpoint != nil {
			snapshot.Checkpoint.StorageBudget = current.Checkpoint.StorageBudget
		}
		if snapshot.Checkpoint != nil && current.Checkpoint != nil && snapshot.Checkpoint.ExecutingCall != "" && snapshot.Checkpoint.ExecutingCall == current.Checkpoint.ExecutingCall {
			snapshot.Checkpoint.ComputerReceipt = current.Checkpoint.ComputerReceipt
			snapshot.Checkpoint.ExecutionIntent, snapshot.Checkpoint.ExecutionReceipt = current.Checkpoint.ExecutionIntent, current.Checkpoint.ExecutionReceipt
		}
		if snapshot.Checkpoint != nil && snapshot.Checkpoint.ExecutingCall == "" {
			snapshot.Checkpoint.ComputerReceipt, snapshot.Checkpoint.ExecutingCapability = nil, nil
			snapshot.Checkpoint.ExecutionIntent, snapshot.Checkpoint.ExecutionReceipt = nil, nil
		}
		if accepted {
			snapshot.Checkpoint.DecisionRepairs = 0
			if snapshot.Status == TaskCompleted {
				resolveFinalAuxiliaryModelRepairs(snapshot)
			}
		}
		*current = *snapshot
		return accepted, nil
	})
	if err == nil {
		if (rejected || accepted) && !inbox && !steering {
			// The execution loop retains this checkpoint pointer. Publish the
			// transaction-selected model without invalidating that live reference.
			checkpoint := task.Checkpoint
			*checkpoint = *updated.Checkpoint
			*task = *updated
			task.Checkpoint = checkpoint
		}
		if task.Checkpoint != nil && updated.Checkpoint != nil {
			task.Checkpoint.ModelBudget = updated.Checkpoint.ModelBudget
			task.Checkpoint.ProcessBudget = updated.Checkpoint.ProcessBudget
			task.Checkpoint.StorageBudget = updated.Checkpoint.StorageBudget
		}
		r.observe(updated)
		if inbox {
			return errConversationInputPending
		}
		if steering {
			return errSteeringPending
		}
	}
	return err
}
