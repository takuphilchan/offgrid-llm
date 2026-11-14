package agents

import (
	"errors"
)

var errConversationInputPending = errors.New("conversation input is awaiting incorporation")

type ConversationHold struct {
	InputIDs         []string   `json:"input_ids"`
	ResumeStatus     TaskStatus `json:"resume_status"`
	ResumeReady      bool       `json:"resume_ready"`
	RecoveryRequired bool       `json:"recovery_required,omitempty"`
}

func hasConversationHold(t *Task) bool {
	return t != nil && t.ConversationHold != nil && len(t.ConversationHold.InputIDs) > 0
}

func validateConversationHold(t *Task) error {
	if t.ConversationHold == nil {
		if t.Status == TaskConversationInput {
			return ErrRunStorage
		}
		return nil
	}
	hold := t.ConversationHold
	if t.Checkpoint == nil || len(hold.InputIDs) < 1 || len(hold.InputIDs) > 32 {
		return ErrRunStorage
	}
	seen := map[string]bool{}
	for _, id := range hold.InputIDs {
		if !validTaskID(id) || seen[id] {
			return ErrRunStorage
		}
		seen[id] = true
	}
	switch hold.ResumeStatus {
	case TaskPending, TaskRunning, TaskChildren, TaskWaiting, TaskInput, TaskInterrupted, TaskUncertain:
	default:
		return ErrRunStorage
	}
	if t.Status == TaskConversationInput && t.Checkpoint.ExecutingCall != "" {
		return ErrRunStorage
	}
	return nil
}

func holdForConversationInput(t *Task, id string) error {
	if t.Checkpoint == nil {
		return ErrRunConflict
	}
	if t.ConversationHold == nil {
		t.ConversationHold = &ConversationHold{ResumeStatus: t.Status, ResumeReady: t.SchedulerReady}
	}
	for _, prior := range t.ConversationHold.InputIDs {
		if prior == id {
			return nil
		}
	}
	if len(t.ConversationHold.InputIDs) >= 32 {
		return ErrRunConflict
	}
	t.ConversationHold.InputIDs = append(t.ConversationHold.InputIDs, id)
	t.ControlEpoch++
	t.SchedulerReady = false
	if t.Status == TaskPending {
		t.Status = TaskConversationInput
	}
	return nil
}

// Resolving one input cannot release another conversation's hold or override a
// user control/recovery boundary. Call only after active dispatch has settled.
func releaseConversationInput(t *Task, id string) {
	if !hasConversationHold(t) {
		return
	}
	hold := t.ConversationHold
	remaining := make([]string, 0, len(hold.InputIDs))
	for _, input := range hold.InputIDs {
		if input != id {
			remaining = append(remaining, input)
		}
	}
	if len(remaining) == len(hold.InputIDs) {
		return
	}
	t.ControlEpoch++
	if len(remaining) > 0 {
		hold.InputIDs = remaining
		return
	}
	t.ConversationHold = nil
	if t.Status != TaskConversationInput {
		return
	}
	if t.ControlAction != "" || t.ControlPending || hold.RecoveryRequired || t.ComputerSessionExpired {
		t.Status, t.SchedulerReady = TaskInterrupted, false
		return
	}
	t.Status, t.SchedulerReady = hold.ResumeStatus, hold.ResumeReady
	if t.Status == TaskRunning {
		t.Status, t.SchedulerReady = TaskPending, true
	}
	if t.ParentID != "" {
		t.SchedulerReady = false // Dependency admission belongs to the parent.
	}
}

// Park only at a safe boundary. Already dispatched operations retain their
// original receipt path. Yielding a slot does not cancel owned processes.
func parkConversationTask(t *Task) {
	if hasConversationHold(t) && t.Status == TaskRunning && t.Checkpoint != nil && t.Checkpoint.ExecutingCall == "" {
		t.Status = TaskConversationInput
		t.SchedulerReady = false
	}
}

func (r *Runner) conversationInputBoundary(task *Task) error {
	r.manager.mu.RLock()
	held := hasConversationHold(r.manager.tasks[task.ID])
	r.manager.mu.RUnlock()
	if held {
		return errConversationInputPending
	}
	return nil
}

func (r *Runner) yieldConversationInput(id string) (*Task, error) {
	return r.manager.updateTask(id, func(current *Task) error {
		if !hasConversationHold(current) {
			return ErrRunConflict
		}
		if current.Checkpoint == nil || current.Checkpoint.ExecutingCall != "" {
			return ErrRunConflict
		}
		parkConversationTask(current)
		return nil
	})
}

// Rejecting a stale proposal must retain measured settlement of a dispatched
// generation. A reservation that did not commit is not a dispatched call.
func retainHeldModelSettlement(current, proposed *Checkpoint) {
	retainToolSettlement(current, proposed)
	if current == nil || proposed == nil || current.BudgetedModelCalls != proposed.BudgetedModelCalls {
		return
	}
	current.Iteration = max(current.Iteration, proposed.Iteration)
	current.ChargedCompletionTokens = proposed.ChargedCompletionTokens
	current.ChargedInferenceNS = proposed.ChargedInferenceNS
	current.MeasuredCompletionTokens = proposed.MeasuredCompletionTokens
	current.MeasuredInferenceNS = proposed.MeasuredInferenceNS
	current.UnmeasuredModelCalls = proposed.UnmeasuredModelCalls
	current.InputBudget = proposed.InputBudget
	current.ContextPreparations = proposed.ContextPreparations
	current.MeasuredContextPreparationNS = proposed.MeasuredContextPreparationNS
}
