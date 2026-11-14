package agents

import (
	"encoding/json"
	"time"
)

// r.mu is held. Acceptance fences the whole affected graph in one durable
// transaction before signalling cancellation. No model slot is needed.
func (r *Runner) steerDelegationLocked(snapshot *Task, actor, requestID, digest, instruction string) (*Task, error) {
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()
	current := r.manager.tasks[snapshot.ID]
	if current == nil || current.Actor != actor {
		return nil, ErrTaskNotFound
	}
	for _, previous := range append(append([]Instruction(nil), current.Instructions...), current.PendingInstructions...) {
		if previous.RequestID == requestID {
			if previous.Digest != digest {
				return nil, ErrRunConflict
			}
			return copyTask(current)
		}
	}
	if !CanSteerTask(current) || current.Delegation == nil {
		return nil, ErrRunConflict
	}
	parent, err := copyTask(current)
	if err != nil {
		return nil, err
	}
	parent.ControlEpoch++
	parent.PendingInstructions = append(parent.PendingInstructions, Instruction{requestID, digest, instruction, time.Now().UTC()})
	parent.Delegation.CorrectionPaused = parent.Delegation.CorrectionPaused || parent.Status == TaskInterrupted
	parent.Status = TaskChildren
	parent.ErrorCode, parent.Error = "correction_pending", "Correction accepted. Waiting for delegated operations to settle."
	batch := []*Task{parent}
	for _, link := range parent.Delegation.Children {
		child := r.manager.tasks[link.ID]
		if child == nil || child.Actor != actor || child.ParentID != parent.ID || child.DeletedAt != nil {
			return nil, ErrRunConflict
		}
		switch child.Status {
		case TaskCompleted, TaskCancelled, TaskFailed, TaskUncertain:
			continue
		}
		next, err := copyTask(child)
		if err != nil {
			return nil, err
		}
		next.ControlEpoch++
		next.Status, next.ErrorCode, next.Error = TaskInterrupted, "correction_pending", "Parent correction fenced this assignment."
		next.PendingApproval = nil
		if uncertainEffect(next.Checkpoint) {
			next.Status, next.ErrorCode, next.Error = TaskUncertain, "uncertain_outcome", "A correction arrived during a tool operation. Reconcile its actual outcome before continuing."
		}
		releaseInterruptedRead(next.Checkpoint)
		batch = append(batch, next)
	}
	if err := r.manager.saveTaskBatch(batch); err != nil {
		return nil, err
	}
	for _, item := range batch {
		r.manager.tasks[item.ID] = item
	}
	for _, link := range parent.Delegation.Children {
		if cancel := r.active[link.ID]; cancel != nil {
			cancel()
		}
	}
	return copyTask(parent)
}

// Both locks are held. Keep results as stale evidence; do not synthesize success
// from a graph prepared before the accepted correction or reopen its budget.
func (r *Runner) applyDelegationCorrectionLocked(parent *Task, children map[string]*Task) error {
	if _, active := r.active[parent.ID]; active {
		return ErrRunConflict
	}
	for _, child := range children {
		if _, active := r.active[child.ID]; active {
			return ErrRunConflict
		}
		if child.Status == TaskUncertain || (child.Checkpoint != nil && child.Checkpoint.ExecutingCall != "") {
			return ErrRunConflict
		}
	}
	paused := parent.Delegation.CorrectionPaused
	outcomes, _ := json.Marshal(childOutcomes(parent, children))
	appendDelegationOutcome(parent, "Superseded assignments. Retained results are evidence from earlier instructions, not completion of the corrected request: "+string(outcomes))
	parent.Checkpoint.WorkerAnswerCommand, parent.PendingInput = nil, nil
	parent.Checkpoint.Calls = nil
	parent.Delegation = nil
	for _, instruction := range parent.PendingInstructions {
		if err := applyInstruction(parent, instruction); err != nil {
			return err
		}
	}
	parent.PendingInstructions = nil
	parent.Status, parent.SchedulerReady, parent.ErrorCode, parent.Error = TaskPending, true, "", ""
	if paused {
		parent.Status, parent.SchedulerReady, parent.ErrorCode, parent.Error = TaskInterrupted, false, "task_paused", "Correction saved. Resume explicitly to continue."
	}
	batch := []*Task{parent}
	for _, child := range children {
		if child.Status != TaskInterrupted && child.Status != TaskPending && child.Status != TaskInput {
			continue
		}
		next, err := copyTask(child)
		if err != nil {
			return err
		}
		finishTask(next, TaskCancelled, "Assignment superseded by a parent correction. Retained evidence is not current completion.")
		next.ErrorCode = "instruction_superseded"
		for index := range parent.ChildHistory {
			if parent.ChildHistory[index].ID == next.ID {
				parent.ChildHistory[index].Status = next.Status
				parent.ChildHistory[index].Phase = ""
			}
		}
		batch = append(batch, next)
	}
	if err := r.manager.saveTaskBatch(batch); err != nil {
		return err
	}
	for _, item := range batch {
		r.manager.tasks[item.ID] = item
	}
	return nil
}
