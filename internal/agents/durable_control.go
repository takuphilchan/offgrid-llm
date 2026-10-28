package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (r *Runner) Stop(id, actor, action, approvalID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.manager.updateTask(id, func(task *Task) error {
		if task.Actor != actor {
			return ErrTaskNotFound
		}
		if action == "deny" {
			if task.Status != TaskWaiting || task.PendingApproval == nil || task.PendingApproval.ID != approvalID {
				return ErrRunConflict
			}
		} else if action != "cancel" {
			return ErrRunConflict
		}
		if task.Status == TaskCompleted || task.Status == TaskFailed || task.Status == TaskCancelled {
			return ErrRunConflict
		}
		finishTask(task, TaskCancelled, "Stopped by user.")
		if HasOwnedExecutionProcesses(task) {
			task.Status, task.ErrorCode, task.Error, task.CompletedAt = TaskInterrupted, "execution_stopping", "Stopping owned local processes; termination is not yet confirmed.", nil
		}
		if uncertainEffect(task.Checkpoint) {
			task.Status, task.Error = TaskUncertain, "Cancelled during a tool call. Its external outcome must be checked."
			if HasOwnedExecutionProcesses(task) {
				task.ErrorCode = "execution_effects_unknown"
			}
		}
		releaseInterruptedRead(task.Checkpoint)
		return nil
	})
	if err == nil {
		if cancel := r.active[id]; cancel != nil {
			cancel()
		}
		if cascadeErr := r.stopChildrenLocked(task, true); cascadeErr != nil {
			return task, cascadeErr
		}
		task, err = r.stopOwnedExecution(task)
		if err != nil {
			return task, err
		}
		if task.Status == TaskInterrupted && task.ErrorCode == "execution_stopping" {
			task, err = r.manager.updateTask(id, func(current *Task) error {
				finishTask(current, TaskCancelled, "Stopped by user. Owned process termination confirmed; earlier effects are not undone.")
				current.ErrorCode = ""
				return nil
			})
			if err != nil {
				return task, err
			}
		}
		r.observe(task)
	}
	return task, err
}

// Reconcile records an actor's report, not independent verification. It never
// re-executes the original call or automatically resumes the task.
func (r *Runner) Reconcile(id, actor, callID, result string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, active := r.active[id]; active {
		return nil, ErrRunConflict
	}
	task, err := r.manager.updateTask(id, func(task *Task) error {
		if task.Actor != actor {
			return ErrTaskNotFound
		}
		cp := task.Checkpoint
		if task.Status != TaskUncertain || cp == nil || len(cp.Calls) == 0 || cp.ExecutingCall != callID || cp.Calls[0].ID != callID || callID == "" || result == "" {
			return ErrRunConflict
		}
		call := cp.Calls[0]
		report, _ := json.Marshal(map[string]any{"provenance": "human_reported", "verified": false, "report": result, "guidance": "This is a user report, not independent tool evidence. Do not repeat the original action. Verify outstanding outcomes through permitted read-only observation before claiming success."})
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(report)})
		cp.Calls, cp.ExecutingCall = cp.Calls[1:], ""
		cp.ExecutionIntent, cp.ExecutionReceipt = nil, nil
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "reconciled", Provenance: "human_reported", ToolName: call.Function.Name, ToolArgs: call.Function.Arguments, ToolResult: result, Timestamp: time.Now().UTC()})
		task.Status, task.Error, task.CompletedAt = TaskInterrupted, "Outcome recorded. Resume explicitly to continue.", nil
		return nil
	})
	if err == nil {
		r.observe(task)
	}
	return task, err
}
