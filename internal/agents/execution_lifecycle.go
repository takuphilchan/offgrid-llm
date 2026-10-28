package agents

import (
	"context"
	"time"
)

// stopOwnedExecution does not depend on another model completion. It closes
// dispatch for the exact job and waits for owned-tree termination evidence.
// A disconnect/time limit remains visibly uncertain, never acknowledged as stop.
func (r *Runner) stopOwnedExecution(task *Task) (*Task, error) {
	if task == nil || task.Config.Execution == nil {
		return task, nil
	}
	if !HasOwnedExecutionProcesses(task) {
		return task, nil
	}
	controller, ok := r.tools.(interface {
		StopOwnedExecution(context.Context, *Task) error
	})
	if !ok {
		return r.executionStopUncertain(task)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := controller.StopOwnedExecution(ctx, task); err != nil {
		return r.executionStopUncertain(task)
	}
	return task, nil
}

func HasOwnedExecutionProcesses(task *Task) bool {
	if task == nil || task.Config.Execution == nil {
		return false
	}
	for _, step := range task.Steps {
		if step.Type == "action" && step.ToolName == "workspace_process_start" {
			return true
		}
	}
	if cp := task.Checkpoint; cp != nil && cp.ExecutingCall != "" {
		for _, call := range cp.Calls {
			if call.ID == cp.ExecutingCall && call.Function.Name == "workspace_process_start" {
				return true
			}
		}
	}
	return false
}

func (r *Runner) executionStopUncertain(task *Task) (*Task, error) {
	updated, err := r.manager.updateTask(task.ID, func(current *Task) error {
		current.Status, current.ErrorCode, current.Error = TaskUncertain, "execution_stop_unconfirmed", "Local executor stopping is not confirmed. Stop the host locally and inspect owned process receipts before retrying."
		current.CompletedAt = nil
		return nil
	})
	if err == nil {
		r.observe(updated)
	}
	return updated, err
}
